package main

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/stumacwastaken/alerts-ingester/internal/alerts"
	"github.com/stumacwastaken/alerts-ingester/internal/alerts/data"
	"github.com/stumacwastaken/alerts-ingester/internal/server"
	_ "modernc.org/sqlite"
)

type config struct {
	host               string
	port               string
	logLevel           string
	dbConnectionString string
	serviceURL         string
	syncInterval       time.Duration
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conf, err := configFromEnvVars()
	if err != nil {
		slog.Default().Error("failed to fetch configuration", slog.Any("err", err))
		os.Exit(1)
	}

	logger := newLogger(os.Stdout, conf.logLevel)

	db, err := sql.Open("sqlite", conf.dbConnectionString)

	if err != nil {
		panic(err)
	}

	defer db.Close()
	if err := db.Ping(); err != nil {
		logger.Error("failed to ping database. Exiting", slog.Any("error", err))
		os.Exit(1)
	}

	alertsService := alerts.NewService(
		logger,
		data.NewStore(db),
		alerts.NewDemoSyncer(logger, conf.serviceURL),
	)

	go alertsService.SyncPeriodically(ctx, conf.syncInterval)

	s := server.New(
		&server.Config{
			Host: conf.host,
			Port: conf.port,
		},
		logger,
		db, // needed to check db connectivity in health handler
		alertsService,
	)
	if err := s.Start(ctx); err != nil {
		logger.Error("failed to start server, exiting", slog.Any("error", err))
		os.Exit(1)
	}

	<-ctx.Done()
	logger.Info("shutdown notice received, shutting down system...")
	db.Close()
	logger.Info("goodbye!")
}

func newLogger(w io.Writer, logLevel string) *slog.Logger {
	lvl := new(slog.LevelVar)
	switch strings.ToLower(logLevel) {
	case "debug":
		lvl.Set(slog.LevelDebug)
	case "warn":
		lvl.Set(slog.LevelWarn)
	case "error":
		lvl.Set(slog.LevelError)
	default:
		lvl.Set(slog.LevelInfo)
	}
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	logger := slog.New(slog.NewJSONHandler(w, opts))
	return logger
}

// in a more productionalized world I'd have used viper or koanf. But this is
// quick and easy.
func configFromEnvVars() (config, error) {

	d, err := time.ParseDuration(os.Getenv("INGESTER_SYNC_INTERVAL"))
	if err != nil {
		return config{}, err
	}
	conf := config{
		host:               os.Getenv("INGESTER_HOST"),
		port:               os.Getenv("INGESTER_PORT"),
		logLevel:           os.Getenv("INGESTER_LOG_LEVEL"),
		dbConnectionString: os.Getenv("INGESTER_DB_CONNECTION_STRING"),
		serviceURL:         os.Getenv("INGESTER_ALERTS_SERVICE_URL"),
		syncInterval:       d,
	}

	if conf.host == "" || conf.port == "" || conf.dbConnectionString == "" || conf.syncInterval == 0 {
		return conf, errors.New("invalid configuration")
	}
	return conf, nil
}
