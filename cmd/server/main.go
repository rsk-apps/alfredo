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

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/config"
	pethttp "github.com/rafaelsoares/alfredo/internal/petcare/adapters/primary/http"
	petmw "github.com/rafaelsoares/alfredo/internal/petcare/adapters/primary/http/middleware"
	"github.com/rafaelsoares/alfredo/internal/petcare/adapters/secondary/sqlite"
	petsvc "github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/webhook"
)

var version = "dev"

func main() {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// 2. Guard: refuse to start without an API key
	if cfg.Auth.APIKey == "" {
		log.Fatal("APP_AUTH_API_KEY must be set — server refuses to start without authentication")
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

	// 4. Open SQLite (runs migration 001)
	db, err := sqlite.Open(cfg.Database.Path)
	if err != nil {
		zapLogger.Fatal("sqlite open failed", zap.Error(err))
	}
	defer db.Close() //nolint:errcheck

	// 5. Webhook emitter (no-op when URL is empty)
	emitter := webhook.New(cfg.Webhook.BaseURL, "petcare", zapLogger)

	// 6. Pet-care repositories
	petRepo := sqlite.NewPetRepository(db)
	vaccineRepo := sqlite.NewVaccineRepository(db)
	dbChecker := sqlite.NewChecker(db)

	// 7. Pet-care services (pure CRUD — no side-effects)
	petService := petsvc.NewPetService(petRepo)
	vaccineService := petsvc.NewVaccineService(vaccineRepo, petRepo)

	// 8. Use Cases (orchestrate domain + webhook emission)
	petUC := app.NewPetUseCase(petService, emitter)
	vaccineUC := app.NewVaccineUseCase(vaccineService, petService, emitter, zapLogger)

	// 9. Health aggregator
	healthAgg := app.NewHealthAggregator(map[string]app.HealthPinger{
		"sqlite": dbChecker,
	})
	healthHandler := pethttp.NewHealthHTTPHandler(healthAgg)

	// 10. HTTP handlers
	petHandler := pethttp.NewPetHandler(petUC)
	vaccineHandler := pethttp.NewVaccineHandler(vaccineUC)

	// 11. Echo instance with global middleware
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(petmw.RequestLogger(zapLogger))
	e.Use(echomw.Recover())

	// 12. Public routes — no auth (health checks from Traefik/Docker)
	public := e.Group("/api/v1")
	public.GET("/health", healthHandler.Health)

	// 13. Protected routes — API key required
	protected := e.Group("/api/v1")
	protected.Use(petmw.APIKeyAuth(cfg.Auth.APIKey))
	petHandler.Register(protected)
	vaccineHandler.Register(protected)

	// 14. Start server with graceful shutdown
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	zapLogger.Info("alfredo starting", zap.String("addr", addr), zap.String("version", version))

	srv := &http.Server{
		Addr:    addr,
		Handler: e,
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
