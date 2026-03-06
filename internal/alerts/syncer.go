package alerts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/stumacwastaken/alerts-ingester/internal/alerts/data"
)

type retryKey struct{}

func WithRetryCount() (context.Context, *int) {
	count := 0
	return context.WithValue(context.Background(), retryKey{}, &count), &count
}

type alertsResponse struct {
	Alerts []*alertResponse `json:"alerts"`
}

type alertResponse struct {
	Source      string    `json:"source"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type DemoSyncer struct {
	url           string
	repeatAttempt int
	log           *slog.Logger
	client        *retryablehttp.Client
}

func NewDemoSyncer(log *slog.Logger, url string) *DemoSyncer {
	client := retryablehttp.NewClient()
	client.RetryMax = 3 // could be pulled out into something configurable
	client.Logger = log
	client.RequestLogHook = func(logger retryablehttp.Logger, req *http.Request, retryNum int) {
		if p, ok := req.Context().Value(retryKey{}).(*int); ok {
			*p = retryNum
		}
	}

	return &DemoSyncer{
		url:    url,
		log:    log,
		client: client,
	}
}

func (d *DemoSyncer) ServiceName() string {
	return "demo"
}

func (d *DemoSyncer) Sync(ctx context.Context, since time.Time) ([]*data.Alert, int, error) {
	childCtx, retryCount := WithRetryCount()
	req, err := retryablehttp.NewRequestWithContext(childCtx, http.MethodGet, d.url, nil)
	if err != nil {
		return nil, *retryCount, err
	}
	q := req.URL.Query()
	q.Add("since", since.Format(time.RFC3339))
	req.URL.RawQuery = q.Encode()

	res, err := d.client.Do(req)
	if err != nil {
		return nil, *retryCount, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			d.log.Error(
				"failed to close request body in syncer, possible memory leak",
				slog.Any("error", err),
			)
		}
	}()
	// There shouldn't be a reason why this would happen, but it's a cheap
	// guard just in case.
	if res.StatusCode > 300 || res.StatusCode < 200 {
		return nil, *retryCount, fmt.Errorf("status code Not 200, got %d", res.StatusCode)
	}

	// this could become a problem with really large bodies, would
	// need a custom decoder to parse though a giant array.
	var result alertsResponse
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, *retryCount, err
	}

	return alertsResponseToDataAlert(result.Alerts), *retryCount, nil
}

func alertsResponseToDataAlert(response []*alertResponse) []*data.Alert {
	dataAlerts := make([]*data.Alert, len(response))
	for i, alert := range response {
		al := &data.Alert{
			Source:          alert.Source,
			Severity:        alert.Severity,
			Description:     sql.NullString{String: alert.Description, Valid: true}, // leaky sql abstraction
			SourceCreatedAt: alert.CreatedAt,
		}
		dataAlerts[i] = al
	}
	return dataAlerts
}
