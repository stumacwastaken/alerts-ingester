package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/stumacwastaken/alerts-ingester/internal/alerts"
	"github.com/stumacwastaken/alerts-ingester/internal/alerts/data"
)

type AlertAPI struct {
	log      *slog.Logger
	alertSvc *alerts.Service
}

func NewAlertsAPI(log *slog.Logger, alertSvc *alerts.Service) *AlertAPI {
	return &AlertAPI{
		log:      log,
		alertSvc: alertSvc,
	}
}

type AlertItem struct {
	Source          string    `json:"source"`
	Severity        string    `json:"severity"`
	Description     string    `json:"description"`
	SourceCreatedAt time.Time `json:"alert_time"`
	IP              string    `json:"ip"`
	AlertType       string    `json:"enrichment_type"`
}

type AlertsResponse struct {
	Alerts    []*AlertItem `json:"alerts"`
	PageCount int          `json:"pages"`
	PageSize  int          `json:"page_size"`
}

func (a *AlertAPI) GetAlerts(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	query := req.URL.Query()
	limit := 100
	page := 1
	if val, err := strconv.ParseInt(query.Get("limit"), 10, 64); err != nil {
		a.log.Debug("error parsing limit from query params, using default",
			slog.Any("error", err), slog.Int("defaultValue", limit))
	} else {
		limit = int(val)
	}

	if val, err := strconv.ParseInt(query.Get("page"), 10, 64); err != nil {
		a.log.Debug("error parsing page from query params, using default",
			slog.Any("error", err), slog.Int("defaultValue", page))
	} else {
		page = int(val)
	}

	res, pages, err := a.alertSvc.SyncedAlerts(ctx, limit, page)
	if err != nil {
		ResponseFromError(w, req, err)
		return
	}
	alerts := alertDTOsToAlertItems(res)
	response := &AlertsResponse{
		Alerts:    alerts,
		PageCount: pages,
		PageSize:  limit,
	}

	marshalled, err := json.Marshal(response)
	if err != nil {
		a.log.Error("failed to marshal alerts response", slog.Any("error", err))
		ResponseFromError(w, req, err)
		return
	}

	w.WriteHeader(http.StatusOK)

	_, err = w.Write(marshalled)
	if err != nil {
		a.log.Error("failed to write alerts response", slog.Any("error", err))
		ResponseFromError(w, req, err)
		return
	}
}

func (a *AlertAPI) Sync(w http.ResponseWriter, req *http.Request) {
	err := a.alertSvc.RunSync(req.Context())
	if err != nil {
		ResponseFromError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"result":"job scheduled"}`))
}

func alertDTOsToAlertItems(alerts []data.Alert) []*AlertItem {
	alertItems := make([]*AlertItem, len(alerts))
	for i, dto := range alerts {
		alert := &AlertItem{
			Source:          dto.Source,
			Severity:        dto.Severity,
			Description:     dto.Description.String,
			SourceCreatedAt: dto.SourceCreatedAt,
			IP:              dto.IpAddress.String,
			AlertType:       dto.EnrichmentType.String,
		}
		alertItems[i] = alert
	}
	return alertItems
}
