package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/config"
	"github.com/rafaelsoares/alfredo/internal/database"
	"github.com/rafaelsoares/alfredo/internal/gcalendar"
	"github.com/rafaelsoares/alfredo/internal/httpserver"
)

var version = "dev"

func main() {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// 2. Guard: refuse to start without a sufficiently strong API key
	if cfg.Auth.APIKey == "" {
		log.Fatal("APP_AUTH_API_KEY must be set — server refuses to start without authentication")
	}
	if len(cfg.Auth.APIKey) < 32 {
		log.Fatal("APP_AUTH_API_KEY must be at least 32 characters — use a cryptographically random value")
	}

	// 3. Init Zap logger with configurable level
	lvl, err := zapcore.ParseLevel(cfg.Log.Level)
	if err != nil {
		lvl = zapcore.InfoLevel
	}
	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(lvl)
	zapLogger, err := zapCfg.Build()
	if err != nil {
		log.Fatalf("logger error: %v", err)
	}
	defer zapLogger.Sync() //nolint:errcheck

	// 4. Open SQLite (runs all migrations)
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		zapLogger.Fatal("sqlite open failed", zap.Error(err))
	}
	defer db.Close() //nolint:errcheck

	loc, err := time.LoadLocation(cfg.App.Timezone)
	if err != nil {
		zapLogger.Fatal("timezone load failed", zap.String("timezone", cfg.App.Timezone), zap.Error(err))
	}

	// 5. Calendar adapter (no-op when credentials are absent)
	var calendarAdapter app.CalendarPort
	if cfg.GCalendar.ClientID != "" && cfg.GCalendar.ClientSecret != "" && cfg.GCalendar.RefreshToken != "" {
		calendarAdapter, err = gcalendar.NewAdapter(context.Background(), gcalendar.AdapterConfig{
			ClientID:     cfg.GCalendar.ClientID,
			ClientSecret: cfg.GCalendar.ClientSecret,
			RefreshToken: cfg.GCalendar.RefreshToken,
		})
		if err != nil {
			zapLogger.Fatal("gcalendar init failed", zap.Error(err))
		}
		zapLogger.Info("gcalendar adapter enabled", zap.String("mode", "google"))
	} else {
		calendarAdapter = gcalendar.NewNoopAdapter(zapLogger)
		zapLogger.Warn("gcalendar noop adapter enabled",
			zap.String("mode", "noop"),
			zap.Bool("client_id_set", cfg.GCalendar.ClientID != ""),
			zap.Bool("client_secret_set", cfg.GCalendar.ClientSecret != ""),
			zap.Bool("refresh_token_set", cfg.GCalendar.RefreshToken != ""),
		)
	}

	e, err := httpserver.New(httpserver.Config{
		DB:       db,
		Calendar: calendarAdapter,
		APIKey:   cfg.Auth.APIKey,
		Location: loc,
		Logger:   zapLogger,
	})
	if err != nil {
		zapLogger.Fatal("server wiring failed", zap.Error(err))
	}

	// 14. Start server with graceful shutdown
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	zapLogger.Info("alfredo starting", zap.String("addr", addr), zap.String("version", version))

	srv := &http.Server{
		Addr:         addr,
		Handler:      e,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zapLogger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zapLogger.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		zapLogger.Error("shutdown error", zap.Error(err))
	}
}
