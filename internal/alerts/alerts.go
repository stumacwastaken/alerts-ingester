package alerts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/netip"
	"sync"
	"time"

	"github.com/stumacwastaken/alerts-ingester/internal/alerts/data"
)

var (
	ErrFailedFetch        = errors.New("failed to fetch values")
	ErrSyncAlreadyRunning = &AlertError{Code: 429, Msg: "sync already running"}
	ErrSyncRunFailedStart = &AlertError{Code: 500, Msg: "sync failed to start"}
)

type AlertError struct {
	Code int // http status codes ideally
	Msg  string
}

func (e *AlertError) Error() string {
	return fmt.Sprintf(`{"error": "%s", "code": %d}`, e.Msg, e.Code)
}

func (e *AlertError) StatusCode() int {
	return e.Code
}

type Alert struct {
	Source         string
	Severity       string
	Description    string
	CreatedAt      time.Time
	IPAddress      string
	EnrichmentType string
}

type Service struct {
	queries       *data.Store
	log           *slog.Logger
	syncer        Syncer
	interval      time.Duration
	syncRunningMu sync.Mutex
	syncRunning   bool
	swg           *sync.WaitGroup
}

func NewService(log *slog.Logger, queries *data.Store, swg *sync.WaitGroup, syncer Syncer) *Service {
	return &Service{
		queries: queries,
		log:     log,
		syncer:  syncer,
		swg:     swg,
	}
}

type Syncer interface {
	ServiceName() string
	Sync(ctx context.Context, timestamp time.Time) ([]*data.Alert, int, error) // responsible for handling its own retries
}

// Will periodically reach out and try to sync the data. If it's within x seconds
// of a previous sync will hold off.
func (s *Service) SyncPeriodically(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.log.Info("starting periodic sync of alerts service", slog.Duration("interval", interval))
	lastSync, err := s.queries.LastSuccessfulSync(ctx)
	if err != nil {
		s.log.Error("failed to fetch last sync", slog.Any("error", err))
	}

	var lastScanTime time.Time
	if lastSync.ScannedAt.IsZero() {
		lastScanTime = time.Now().UTC()
	} else {
		lastScanTime = lastSync.ScannedAt
	}
	if err := s.RunSync(context.Background(), WithTimeStamp(lastScanTime)); err != nil {
		s.log.Warn("failed to sync on startup",
			slog.String("serviceName", s.syncer.ServiceName()),
			slog.Any("error", err),
		)
	}

	for {
		select {
		case <-ctx.Done():
			s.log.Info("stopping periodic sync interval")
			return nil
		case t := <-ticker.C:
			s.log.Debug("interval hit", slog.Time("on", t))
			lastSync, err := s.queries.LastSuccessfulSync(ctx)
			if err != nil {
				s.log.Error("failed to fetch last sync", slog.Any("error", err))
			}
			if t.Sub(lastSync.ScannedAt).Abs() < 1*time.Minute {
				s.log.Info("most recent scan was within 1 minute of curren time, skipping interval")
			}
			err = s.RunSync(context.Background(), WithTimeStamp(lastSync.ScannedAt))
			if err != nil {
				s.log.Warn("failed to run periodic sync", slog.Any("error", err))
				continue
			}
		}
	}
}

type runSyncOpts struct {
	timestamp time.Time
}

type RunSyncFn func(*runSyncOpts)

func WithTimeStamp(t time.Time) RunSyncFn {
	return func(rso *runSyncOpts) {
		rso.timestamp = t
	}
}

func (s *Service) RunSync(ctx context.Context, opts ...RunSyncFn) error {
	if s.syncRunning {
		return ErrSyncAlreadyRunning
	}
	s.syncRunningMu.Lock()
	s.syncRunning = true
	rso := &runSyncOpts{}
	for _, opt := range opts {
		opt(rso)
	}

	if rso.timestamp.IsZero() {
		lastSync, err := s.queries.LastSuccessfulSync(ctx)
		if err != nil {
			s.log.Error("failed to fetch last sync", slog.Any("error", err))
			return ErrSyncRunFailedStart
		}

		rso.timestamp = time.Now().UTC()
		if !lastSync.ScannedAt.IsZero() {
			rso.timestamp = lastSync.ScannedAt
		}
	}

	go func() {
		if err := s.runSync(context.Background(), rso.timestamp); err != nil {
			s.log.Warn("failed to sync",
				slog.String("serviceName", s.syncer.ServiceName()),
				slog.Any("error", err),
			)
		}
		s.syncRunning = false
		s.syncRunningMu.Unlock()
	}()
	return nil
}

// run the syncer and enrich the data if successful
func (s *Service) runSync(ctx context.Context, since time.Time) error {
	s.swg.Add(1)
	defer s.swg.Done()

	startTime := time.Now().UTC()
	s.log.Debug("running sync for alert service", slog.String("serviceName", s.syncer.ServiceName()))
	res, retries, err := s.syncer.Sync(ctx, since)
	if err != nil {
		s.log.Warn("failed to sync alert service",
			slog.String("serviceName", s.syncer.ServiceName()), slog.Any("error", err),
		)
		_, srErr := s.queries.AddScanResult(ctx, data.AddScanResultParams{
			Reattempts: sql.NullInt64{Int64: int64(retries), Valid: true},
			Success:    false,
			Reason:     sql.NullString{String: err.Error(), Valid: true},
			ScannedAt:  startTime,
		})

		if srErr != nil {
			s.log.Warn("failed to add scan result history entry", slog.Any("error", err))
			// this would be a great place have a prometheus update and ring
			// someone's pager.
		}
		return err
	}

	return s.queries.ExecInTx(ctx, func(q *data.Queries) error {
		alertFetchHistoryID, err := q.AddScanResult(ctx, data.AddScanResultParams{
			Reattempts: sql.NullInt64{Int64: int64(retries), Valid: true},
			Success:    true,
			ScannedAt:  startTime,
		})
		if err != nil {
			return err
		}

		err = enrichData(res)
		if err != nil {
			// save the data, but kick off an alert (or a log in this case) to indicate
			// the failure of the enrichment process for later processing.
			s.log.Error(
				"Failed to enrich batch of alerts",
				slog.Int64("enrichmentID", alertFetchHistoryID),
				slog.Any("error", err),
			)
		}
		for _, a := range res {
			entryErr := q.AddAlert(ctx, data.AddAlertParams{
				Source:          a.Source,
				Severity:        a.Severity,
				Description:     a.Description,
				SourceCreatedAt: a.SourceCreatedAt,
				EnrichmentType:  a.EnrichmentType,
				IpAddress:       a.IpAddress,
				FetchHistoryID:  sql.NullInt64{Int64: alertFetchHistoryID, Valid: true},
			})

			if entryErr != nil {
				s.log.Error("failed to insert alert data into table", slog.Any("error", err))
				return err
			}
		}
		s.log.Debug("finished sync job")
		return nil
	})
}

// Not mapping the alertFetchHistory value to something that's not a dto is
// kind of leaky, but honestly this is a small project. If this were a monolith
// or modulith it should probably be wrapped up to avoid balls of spaghetti.
// This is a pretty anemic method to begin with.
func (s *Service) FailureLog(ctx context.Context, count, hours int) ([]data.AlertFetchHistory, error) {
	hourCount := fmt.Sprintf("-%d hours", hours)
	res, err := s.queries.RecentScanErrors(ctx, data.RecentScanErrorsParams{
		Datetime: hourCount,
		Limit:    int64(count),
	})
	if err != nil {
		return nil, fmt.Errorf("%w when fetching failure log. details: %w",
			ErrFailedFetch, err)
	}
	return res, nil
}

func (s *Service) LastSuccessfulSync(ctx context.Context) (data.AlertFetchHistory, error) {
	res, err := s.queries.LastSuccessfulSync(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return data.AlertFetchHistory{}, nil
		}
		return res, fmt.Errorf("%w when fetching last successful sync. details: %w",
			ErrFailedFetch, err)
	}
	return res, nil
}

func (s *Service) SyncedAlerts(ctx context.Context, limit, page int) ([]data.Alert, int, error) {
	if page < 1 {
		page = 1 // no negative pages
	}

	if limit > 1000 { // basic sanity.
		return nil, 0, &AlertError{Msg: "limit too large! maximum of 1000", Code: 400}
	}
	total, err := s.queries.CountAlerts(ctx)
	if err != nil {
		s.log.Warn("failed to fetch alert count", slog.Any("error", err))
		return nil, 0, ErrFailedFetch
	}

	offset := (page - 1) * limit
	if int64(offset) > total {
		return nil, 0, &AlertError{Msg: "offset incorrect. ", Code: 400}
	}

	res, err := s.queries.Alerts(ctx, data.AlertsParams{
		Limit:  int64(limit),
		Offset: int64(offset), // offset can trigger full table scans. But it's fine for a small project
	})
	if err != nil {
		s.log.Warn("failed to fetch alerts", slog.Any("error", err))
		return nil, 0, ErrFailedFetch
	}

	pageCount := int(total) / limit
	return res, pageCount, nil

}

func (s *Service) ServiceStatus(ctx context.Context) string { // string return would be better as a type
	res, err := s.queries.LastSyncs(ctx, 10)
	status := "down"

	if err != nil {
		s.log.Error("failed to fetch last syncs, status set to down", slog.Any("error", err))
		// if we can't query for last syncs make it down
		return status
	}
	failureCount := 0

	for _, entry := range res {
		if !entry.Success {
			failureCount++
		}
	}
	// if no failures return ok
	if failureCount == 0 {
		return "ok"
	}

	if failureCount > 0 {
		status = "degraded"
	}
	if failureCount == len(res) {
		status = "down"
	}
	return status
}

// enriches to add an IP address and something random. If we were actually requiring
// a real IP address and data type then this would be a call out to some external
// service (or a database that this service has access to). Given that I have
// neither please accept these stubs.
func enrichData(alerts []*data.Alert) error {
	for _, alert := range alerts {
		ip := randomIPv4()
		alert.IpAddress = sql.NullString{String: ip.String(), Valid: true}
		alert.EnrichmentType = sql.NullString{String: randomEnrichmentSource(ip), Valid: true}
	}
	return nil
}

// generate a random enrichment type (like say if we were also consuming other
// threat feeds) based off the IP address. If it's private we can say the threat
// source is from Censys.
// quick note: This data could be normalized in the database.
func randomEnrichmentSource(ip netip.Addr) string {
	randomRegion := []string{"CrowdStrike", "VirusTotal", "OTX", "RF"}

	if ip.IsPrivate() {
		return "Censys"
	}
	return randomRegion[rand.IntN(len(randomRegion))]

}

func randomIPv4() netip.Addr {
	addr := rand.Uint32()

	ip := netip.AddrFrom4([4]byte{
		byte(addr >> 24),
		byte(addr >> 16),
		byte(addr >> 8),
		byte(addr),
	})

	return ip
}
