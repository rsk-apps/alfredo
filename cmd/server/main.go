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

	agentclaude "github.com/rafaelsoares/alfredo/internal/agent/adapters/secondary/claude"
	agentnoop "github.com/rafaelsoares/alfredo/internal/agent/adapters/secondary/noop"
	agentport "github.com/rafaelsoares/alfredo/internal/agent/port"
	agentservice "github.com/rafaelsoares/alfredo/internal/agent/service"
	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/config"
	"github.com/rafaelsoares/alfredo/internal/database"
	"github.com/rafaelsoares/alfredo/internal/gcalendar"
	"github.com/rafaelsoares/alfredo/internal/httpserver"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

var version = "dev"

func initCalendarAdapter(cfg *config.Config, zapLogger *zap.Logger) (app.CalendarPort, error) {
	if cfg.GCalendar.ClientID != "" && cfg.GCalendar.ClientSecret != "" && cfg.GCalendar.RefreshToken != "" {
		adapter, err := gcalendar.NewAdapter(context.Background(), gcalendar.AdapterConfig{
			ClientID:     cfg.GCalendar.ClientID,
			ClientSecret: cfg.GCalendar.ClientSecret,
			RefreshToken: cfg.GCalendar.RefreshToken,
		})
		if err != nil {
			return nil, err
		}
		zapLogger.Info("gcalendar adapter enabled", zap.String("mode", "google"))
		return adapter, nil
	}
	zapLogger.Warn("gcalendar noop adapter enabled",
		zap.String("mode", "noop"),
		zap.Bool("client_id_set", cfg.GCalendar.ClientID != ""),
		zap.Bool("client_secret_set", cfg.GCalendar.ClientSecret != ""),
		zap.Bool("refresh_token_set", cfg.GCalendar.RefreshToken != ""),
	)
	return gcalendar.NewNoopAdapter(zapLogger), nil
}

func initTelegramAdapter(cfg *config.Config, zapLogger *zap.Logger) (app.TelegramPort, error) {
	if cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
		adapter, err := telegram.NewAdapter(telegram.AdapterConfig{
			BotToken: cfg.Telegram.BotToken,
			ChatID:   cfg.Telegram.ChatID,
		})
		if err != nil {
			return nil, err
		}
		zapLogger.Info("telegram adapter enabled", zap.String("mode", "telegram"))
		return adapter, nil
	}
	zapLogger.Warn("telegram noop adapter enabled",
		zap.String("mode", "noop"),
		zap.Bool("bot_token_set", cfg.Telegram.BotToken != ""),
		zap.Bool("chat_id_set", cfg.Telegram.ChatID != ""),
	)
	return telegram.NewNoopAdapter(zapLogger), nil
}

func initAgentLLM(cfg *config.Config, zapLogger *zap.Logger) (agentport.LLMClient, error) {
	if cfg.Agent.AnthropicAPIKey != "" {
		adapter, err := agentclaude.NewAdapter(agentclaude.Config{
			APIKey:      cfg.Agent.AnthropicAPIKey,
			Model:       cfg.Agent.Model,
			CallTimeout: time.Duration(cfg.Agent.CallTimeoutSeconds) * time.Second,
		})
		if err != nil {
			return nil, err
		}
		zapLogger.Info("agent llm adapter enabled", zap.String("mode", "claude"), zap.String("model", cfg.Agent.Model))
		return adapter, nil
	}
	zapLogger.Warn("agent noop llm adapter enabled", zap.String("mode", "noop"))
	return agentnoop.NewAdapter(zapLogger), nil
}

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

	// 5. Calendar adapter
	calendarAdapter, err := initCalendarAdapter(cfg, zapLogger)
	if err != nil {
		zapLogger.Fatal("gcalendar init failed", zap.Error(err))
	}

	// 6. Telegram adapter
	telegramAdapter, err := initTelegramAdapter(cfg, zapLogger)
	if err != nil {
		zapLogger.Fatal("telegram init failed", zap.Error(err))
	}

	// 7. Agent LLM adapter
	agentLLM, err := initAgentLLM(cfg, zapLogger)
	if err != nil {
		zapLogger.Fatal("agent init failed", zap.Error(err))
	}

	e, err := httpserver.New(httpserver.Config{
		DB:       db,
		Calendar: calendarAdapter,
		Telegram: telegramAdapter,
		AgentLLM: agentLLM,
		AgentRouterConfig: agentservice.RouterConfig{
			MaxIterations:          cfg.Agent.MaxIterations,
			MaxOutputTokensPerCall: cfg.Agent.MaxOutputTokens,
			TotalTimeout:           time.Duration(cfg.Agent.TotalTimeoutSeconds) * time.Second,
			CallTimeout:            time.Duration(cfg.Agent.CallTimeoutSeconds) * time.Second,
		},
		APIKey:   cfg.Auth.APIKey,
		Location: loc,
		Logger:   zapLogger,
	})
	if err != nil {
		zapLogger.Fatal("server wiring failed", zap.Error(err))
	}

	// 8. Start server with graceful shutdown
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
