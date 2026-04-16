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

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/database"
	pethttp "github.com/rafaelsoares/alfredo/internal/petcare/adapters/primary/http"
	petmw "github.com/rafaelsoares/alfredo/internal/petcare/adapters/primary/http/middleware"
	petcaresqlite "github.com/rafaelsoares/alfredo/internal/petcare/adapters/secondary/sqlite"
	petsvc "github.com/rafaelsoares/alfredo/internal/petcare/service"
)

type Config struct {
	DB       *sql.DB
	Calendar app.CalendarPort
	Telegram app.TelegramPort
	APIKey   string
	Location *time.Location
	Logger   *zap.Logger
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

	petRepo := petcaresqlite.NewPetRepository(cfg.DB)
	vaccineRepo := petcaresqlite.NewVaccineRepository(cfg.DB)
	treatmentRepo := petcaresqlite.NewTreatmentRepository(cfg.DB)
	doseRepo := petcaresqlite.NewDoseRepository(cfg.DB)
	txRunner := petcaresqlite.NewTxRunner(cfg.DB)

	appointmentRepo := petcaresqlite.NewAppointmentRepository(cfg.DB)

	petService := petsvc.NewPetService(petRepo)
	vaccineService := petsvc.NewVaccineService(vaccineRepo, petRepo)
	treatmentService := petsvc.NewTreatmentService(treatmentRepo)
	doseService := petsvc.NewDoseService(doseRepo)
	appointmentService := petsvc.NewAppointmentService(appointmentRepo)

	petUC := app.NewPetUseCase(petService, txRunner, cfg.Calendar, logger)
	vaccineUC := app.NewVaccineUseCase(vaccineService, petService, txRunner, cfg.Calendar, cfg.Telegram, cfg.Location.String(), logger)
	treatmentUC := app.NewTreatmentUseCase(treatmentService, doseService, petService, txRunner, cfg.Calendar, cfg.Telegram, cfg.Location.String(), logger)
	appointmentUC := app.NewAppointmentUseCase(appointmentService, petService, cfg.Calendar, cfg.Telegram, cfg.Location.String(), logger)

	healthAgg := app.NewHealthAggregator(map[string]app.HealthPinger{
		"sqlite": database.NewChecker(cfg.DB),
	})

	healthHandler := pethttp.NewHealthHTTPHandler(healthAgg)
	petHandler := pethttp.NewPetHandler(petUC)
	vaccineHandler := pethttp.NewVaccineHandler(vaccineUC, cfg.Location)
	treatmentHandler := pethttp.NewTreatmentHandler(treatmentUC, cfg.Location)
	appointmentHandler := pethttp.NewAppointmentHandler(appointmentUC, cfg.Location)

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
	petHandler.Register(protected)
	vaccineHandler.Register(protected)
	treatmentHandler.Register(protected)
	appointmentHandler.Register(protected)

	return e, nil
}
