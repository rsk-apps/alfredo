package httpserver

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	agenthttp "github.com/rafaelsoares/alfredo/internal/agent/adapters/primary/http"
	agentnoop "github.com/rafaelsoares/alfredo/internal/agent/adapters/secondary/noop"
	agentsqlite "github.com/rafaelsoares/alfredo/internal/agent/adapters/secondary/sqlite"
	agentport "github.com/rafaelsoares/alfredo/internal/agent/port"
	agentservice "github.com/rafaelsoares/alfredo/internal/agent/service"
	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/database"
	healthhttp "github.com/rafaelsoares/alfredo/internal/health/adapters/primary/http"
	healthsql "github.com/rafaelsoares/alfredo/internal/health/adapters/secondary/sqlite"
	healthsvc "github.com/rafaelsoares/alfredo/internal/health/service"
	pethttp "github.com/rafaelsoares/alfredo/internal/petcare/adapters/primary/http"
	petmw "github.com/rafaelsoares/alfredo/internal/petcare/adapters/primary/http/middleware"
	petcaresqlite "github.com/rafaelsoares/alfredo/internal/petcare/adapters/secondary/sqlite"
	petsvc "github.com/rafaelsoares/alfredo/internal/petcare/service"
)

type Config struct {
	DB                *sql.DB
	Calendar          app.CalendarPort
	Telegram          app.TelegramPort
	AgentLLM          agentport.LLMClient
	AgentRouterConfig agentservice.RouterConfig
	APIKey            string
	Location          *time.Location
	Logger            *zap.Logger
}

func New(cfg Config) (*echo.Echo, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("server db is required")
	}
	if cfg.Calendar == nil {
		return nil, fmt.Errorf("server calendar is required")
	}
	if cfg.Telegram == nil {
		return nil, fmt.Errorf("server telegram is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("server api key is required")
	}
	if cfg.Location == nil {
		return nil, fmt.Errorf("server location is required")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	agentLLM := cfg.AgentLLM
	if agentLLM == nil {
		agentLLM = agentnoop.NewAdapter(logger)
	}

	petRepo := petcaresqlite.NewPetRepository(cfg.DB)
	vaccineRepo := petcaresqlite.NewVaccineRepository(cfg.DB)
	treatmentRepo := petcaresqlite.NewTreatmentRepository(cfg.DB)
	doseRepo := petcaresqlite.NewDoseRepository(cfg.DB)
	observationRepo := petcaresqlite.NewObservationRepository(cfg.DB)
	txRunner := petcaresqlite.NewTxRunner(cfg.DB)

	appointmentRepo := petcaresqlite.NewAppointmentRepository(cfg.DB)
	supplyRepo := petcaresqlite.NewSupplyRepository(cfg.DB)
	healthRepo := healthsql.NewProfileRepository(cfg.DB)
	metricRepo := healthsql.NewMetricRepository(cfg.DB)
	workoutRepo := healthsql.NewWorkoutRepository(cfg.DB)
	rawImportRepo := healthsql.NewRawImportRepository(cfg.DB)

	petService := petsvc.NewPetService(petRepo)
	vaccineService := petsvc.NewVaccineService(vaccineRepo, petRepo)
	treatmentService := petsvc.NewTreatmentService(treatmentRepo)
	doseService := petsvc.NewDoseService(doseRepo)
	appointmentService := petsvc.NewAppointmentService(appointmentRepo)
	observationService := petsvc.NewObservationService(observationRepo)
	supplyService := petsvc.NewSupplyService(supplyRepo)
	healthProfileService := healthsvc.NewProfileService(healthRepo)
	metricService := healthsvc.NewMetricService(metricRepo, rawImportRepo)
	workoutService := healthsvc.NewWorkoutService(workoutRepo, rawImportRepo)

	petUC := app.NewPetUseCase(petService, txRunner, cfg.Calendar, logger)
	vaccineUC := app.NewVaccineUseCase(vaccineService, petService, txRunner, cfg.Calendar, cfg.Telegram, cfg.Location.String(), logger)
	treatmentUC := app.NewTreatmentUseCase(treatmentService, doseService, petService, txRunner, cfg.Calendar, cfg.Telegram, cfg.Location.String(), logger)
	appointmentUC := app.NewAppointmentUseCase(appointmentService, petService, cfg.Calendar, cfg.Telegram, cfg.Location.String(), logger)
	observationUC := app.NewObservationUseCase(observationService, petService, cfg.Telegram, cfg.Location.String(), logger)
	supplyUC := app.NewSupplyUseCase(supplyService, petService)
	summaryUC := app.NewSummaryUseCase(petService, vaccineService, treatmentService, appointmentService, observationService, supplyService, cfg.Location)
	agentInvocationRepo := agentsqlite.NewInvocationRepository(cfg.DB)
	agentRouter := agentservice.NewRouter(agentLLM, agentInvocationRepo, cfg.AgentRouterConfig, logger)
	agentUC := app.NewAgentUseCase(agentRouter, petUC, vaccineUC, treatmentUC, observationUC, appointmentUC, supplyUC, summaryUC, cfg.Telegram, cfg.Location, logger)

	healthAgg := app.NewHealthAggregator(map[string]app.HealthPinger{
		"sqlite": database.NewChecker(cfg.DB),
	})

	healthHandler := pethttp.NewHealthHTTPHandler(healthAgg)
	healthProfileHandler := healthhttp.NewProfileHandler(healthProfileService)
	metricHandler := healthhttp.NewMetricHandler(metricService)
	workoutHandler := healthhttp.NewWorkoutHandler(workoutService)
	petHandler := pethttp.NewPetHandler(petUC)
	summaryHandler := pethttp.NewSummaryHandler(summaryUC, cfg.Location)
	vaccineHandler := pethttp.NewVaccineHandler(vaccineUC, cfg.Location)
	treatmentHandler := pethttp.NewTreatmentHandler(treatmentUC, cfg.Location)
	appointmentHandler := pethttp.NewAppointmentHandler(appointmentUC, cfg.Location)
	observationHandler := pethttp.NewObservationHandler(observationUC, cfg.Location)
	supplyHandler := pethttp.NewSupplyHandler(supplyUC)
	siriHandler := agenthttp.NewSiriHandler(agentUC)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(petmw.RequestLogger(logger))
	e.Use(echomw.Recover())
	e.Use(echomw.BodyLimit("1M"))
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		msg := "internal_error"
		var he *echo.HTTPError
		if errors.As(err, &he) {
			code = he.Code
			msg = http.StatusText(he.Code)
		}
		if !c.Response().Committed {
			_ = c.JSON(code, map[string]string{"error": msg})
		}
	}

	public := e.Group("/api/v1")
	public.GET("/health", healthHandler.Health)

	protected := e.Group("/api/v1")
	protected.Use(petmw.APIKeyAuth(cfg.APIKey))
	healthProfileHandler.Register(protected)
	metricHandler.Register(protected)
	workoutHandler.Register(protected)
	summaryHandler.Register(protected)
	petHandler.Register(protected)
	vaccineHandler.Register(protected)
	treatmentHandler.Register(protected)
	appointmentHandler.Register(protected)
	observationHandler.Register(protected)
	supplyHandler.Register(protected)
	siriHandler.Register(protected)

	return e, nil
}
