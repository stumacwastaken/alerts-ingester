package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/stumacwastaken/alerts-ingester/internal/alerts"
)

type Status struct {
	Status             string         `json:"status"`
	DBConnectivity     bool           `json:"database_connected"`
	LastSuccessfulSync time.Time      `json:"last_successful_sync"`
	RecentErrors       []*RecentError `json:"recent_errors"`
}
type RecentError struct {
	Description string    `json:"description"`
	Time        time.Time `json:"time"`
}

type HealthHandler struct {
	db        *sql.DB
	log       *slog.Logger
	alertsSvc *alerts.Service
}

var errInternalServer = InternalServerError("failed to fetch health data!")

func NewHealthHandler(db *sql.DB, log *slog.Logger, alertsSvc *alerts.Service) *HealthHandler {
	return &HealthHandler{
		db:        db,
		log:       log,
		alertsSvc: alertsSvc,
	}
}

// HealthHandler returns a simple JSON response to indicate the service is running.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // could add a timeout here
	res, err := h.generateHealthReport(ctx)
	if err != nil {
		ResponseFromError(w, r, errInternalServer)
		return
	}

	marshalled, err := json.Marshal(res)
	if err != nil {
		ResponseFromError(w, r, errInternalServer)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(marshalled)
}

func (h *HealthHandler) generateHealthReport(ctx context.Context) (*Status, error) {
	status := &Status{
		RecentErrors: []*RecentError{},
	}
	// will technically hang without a context timeout. But this is a demo
	if err := h.db.Ping(); err != nil {
		status.DBConnectivity = false
	} else {
		status.DBConnectivity = true
	}

	recentErrs, err := h.alertsSvc.FailureLog(ctx, 10, 2)
	if err != nil {
		return nil, err
	}

	for _, dtoErr := range recentErrs {
		recErr := &RecentError{
			Description: dtoErr.Reason.String, // I'm ok with an "" empty string here
			Time:        dtoErr.ScannedAt,
		}
		status.RecentErrors = append(status.RecentErrors, recErr)
	}

	lastSync, err := h.alertsSvc.LastSuccessfulSync(ctx)
	if err != nil {
		return nil, err
	}

	status.LastSuccessfulSync = lastSync.ScannedAt

	// service status will attempt to hit the db for sync history,
	// if the db is down it'll return "down"
	status.Status = h.alertsSvc.ServiceStatus(ctx)

	return status, nil
}
