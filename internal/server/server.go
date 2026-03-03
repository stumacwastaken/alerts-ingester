package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/stumacwastaken/alerts-ingester/internal/alerts"
	"github.com/stumacwastaken/alerts-ingester/internal/api"
)

// Server represents the HTTP server for the alerts ingester.
type Server struct {
	host       string
	port       string
	log        *slog.Logger
	httpServer *http.Server
}

type Config struct {
	Host string
	Port string
}

// New creates a new Server instance.
func New(cfg *Config, log *slog.Logger, db *sql.DB, alerts *alerts.Service) *Server {
	router := createRouter(log, db, alerts)
	// if we have multiple (read: more than 2) middlewares we should probably have
	// a setMiddlewares() func that accepts a slice of middleware funcs. Even this
	// is hurting me a bit.
	wrappedRouter := LogResultMiddleware(log, JSONContentType(router))
	httpServer := &http.Server{
		Addr:    net.JoinHostPort(cfg.Host, cfg.Port),
		Handler: wrappedRouter,
	}

	server := &Server{
		host:       cfg.Host,
		port:       cfg.Port,
		log:        log,
		httpServer: httpServer,
	}

	return server
}

func (s *Server) Start(ctx context.Context) error {
	s.log.Debug("starting server...")
	srvErr := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("error starting http server", slog.Any("error", err))
			srvErr <- err
		}
	}()

	select {
	case err := <-srvErr:
		return err
	case <-ctx.Done():
		s.log.Info("shutdown received, shutting down server")

		shutDownctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := s.httpServer.Shutdown(shutDownctx); err != nil {
			s.log.Error("error shutting down server properly", slog.Any("error", err))
			return err
		}
	}
	return nil
}

func createRouter(log *slog.Logger, db *sql.DB, alerts *alerts.Service) http.Handler {
	mux := http.NewServeMux()

	alertAPI := api.NewAlertsAPI(log, alerts)

	mux.Handle("GET /health", api.NewHealthHandler(db, log, alerts))
	mux.HandleFunc("GET /alerts", alertAPI.GetAlerts)
	mux.HandleFunc("GET /sync", alertAPI.Sync)
	return mux
}

func JSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// can be used as a way to output metrics to useful things like prometheus
func LogResultMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		rw := newResponseWriter(w)
		next.ServeHTTP(rw, r)

		duration := time.Since(startTime)
		log.Debug("http request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status_code", rw.statusCode),
			slog.Int64("duration_ms", duration.Milliseconds()),
		)
	})
}

// to capture status codes and whatnot
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}
