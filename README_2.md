# Alerts Ingestion Service

A demo application for ingesting alerts from a third-party alerts API.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Dependencies](#dependencies)
- [Setup](#setup)
- [API Endpoints](#api-endpoints)
- [Configuration](#configuration)
- [Design Decisions](#design-decisions)
- [Testing](#testing)
- [Future Considerations](#future-considerations)

---

## Quick Start

**Full demo (recommended):**

```bash
make demo
```

This brings up the mock alerts server, runs database migrations, and starts the ingester. Verify with:

```bash
curl http://localhost:9999/health
```

**Development mode:**

```bash
make dev
```

Only starts the alerts server. Run migrations manually (see [Setup](#setup)).

> **Note:** If Docker networks fail on startup (common with different profiles), try:
> `docker system prune` or `docker networks prune`

---

## Dependencies

| Tool | Purpose |
|------|---------|
| [Golang 1.25+](https://go.dev/dl/) | Runtime |
| [Docker Compose](https://docs.docker.com/compose) | Container orchestration |
| [sqlc](https://sqlc.dev) | Type-safe query generation from SQL |
| [go-migrate](https://github.com/golang-migrate/migrate) | Database migrations |
| [SQLite](https://www.sqlite.org/) | Database |

**Why these choices?**

- **sqlc** — Generates Go code from schema and queries; avoids ORM overhead and row-scan typos. See [sqlc.yaml](./sqlc.yaml).
- **go-migrate** — Manages schema via `db/migrations/`.
- **SQLite** — Simple for development; suitable for demos (see [Turso](https://turso.tech) for production use cases).
- **modernc.org/sqlite** — Pure-Go SQLite driver (no CGO).
- **go-retryablehttp** — Retries on 5xx, 429, etc. when calling the alerts API.

---

## Setup

From scratch:

1. Install Golang and the tools above.
2. Run migrations: `make migrate-up` (creates `db/` and schema).
3. (Optional) Regenerate sqlc code: `make gen-queries` (generated code is checked in).
4. Build: `make build`
5. Start mock server: `make dev`
6. Run the binary or launch from your IDE.
7. Configure via [local.env](./local.env).

---

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/alerts` | List ingested alerts (paginated) |
| GET | `/sync` | Trigger an immediate sync |
| GET | `/health` | Service health status |

### GET /alerts

Returns ingested alerts with pagination.

**Query parameters:**

| Param | Default | Description |
|-------|---------|-------------|
| `page` | 1 | Page number |
| `limit` | 100 | Page size (max 1000) |

- Limit > 1000 returns an error.
- Negative `page` defaults to 1.
- Response includes `alerts`, `pages`, and `page_size`.

### GET /sync

Triggers an immediate sync from the third-party API.

- **201** — Sync job scheduled.
- **429** — Sync already in progress.

### GET /health

Returns health status:

| Field | Description |
|-------|-------------|
| `status` | `ok` \| `degraded` \| `down` |
| `database_connected` | DB connectivity |
| `last_successful_sync` | Timestamp of last successful sync |
| `recent_errors` | Failed jobs from last 2 hours |

**Status logic:**

- `down` — DB unreachable or all of last 10 syncs failed.
- `degraded` — Some failures in last 10 syncs.
- `ok` — No failures.

---

## Configuration

### Ingester (`local.env` / `demo.env`)

```env
INGESTER_HOST=127.0.0.1
INGESTER_PORT=9999
INGESTER_LOG_LEVEL=DEBUG
INGESTER_DB_CONNECTION_STRING=db/alerts.db
INGESTER_SYNC_INTERVAL=3m
INGESTER_ALERTS_SERVICE_URL=http://localhost:9200/alerts
```

| Variable | Description |
|----------|-------------|
| `INGESTER_HOST` | Listen host |
| `INGESTER_PORT` | Listen port |
| `INGESTER_LOG_LEVEL` | Log level (slog) |
| `INGESTER_DB_CONNECTION_STRING` | SQLite path |
| `INGESTER_SYNC_INTERVAL` | Sync interval (Go duration) |
| `INGESTER_ALERTS_SERVICE_URL` | Upstream alerts API URL |

### Mock Server

```env
PORT=9200
HOST=0.0.0.0
ERROR_RATE=0.25
```

| Variable | Description |
|----------|-------------|
| `ERROR_RATE` | 0–1; probability of returning 400, 429, or 500 |
| — | ~10% of requests sleep 4s (simulates slow responses) |

---

## Design Decisions

### Architecture

- **Flat structure** — `cmd/`, `internal/api/`, `internal/alerts/`, `db/migrations/`.
- **Layered flow** — API → alerts service → data layer.
- **HTTP only** — TLS expected at load balancer / ingress.

Request flow: API layer → alerts service → data layer (or sync) → response.

### Enrichment

Each alert is enriched with:

1. **IP address** — Random IPv4.
2. **Enrichment type** — Threat feed source (CrowdStrike, VirusTotal, OTX, RF, or Censys for private IPs).

### Sync Runner

- Ticker-based periodic sync.
- Mutex prevents concurrent syncs (in-process only; would need changes for distributed).
- Skips tick if last sync was within 1 minute (rate limiting). `/sync` does not apply this check.

### Syncer Interface

The alerts service accepts a `Syncer` interface so different upstream APIs can be plugged in without changing core logic.

### Error Handling

- [APIError](internal/api/errors.go) maps errors to HTTP status codes.
- [AlertError](internal/alerts/alerts.go) implements `StatusCode()` for HTTP responses.
- Health endpoint does not surface these; failures there return a generic 500

### Database

- **alerts** — Normalized alert records.
- **alert_fetch_history** — Sync job history (success/failure, timestamps).

Alerts reference `alert_fetch_history` to track which sync produced them.

### Observability

- Logging only (no Prometheus/OpenTelemetry).
- Middleware logs request method, path, status, and duration.

---

## Testing

No automated tests. Manual checks used:

1. **Startup** — Logs show initial sync.
2. **GET /alerts** — Pagination, `limit=9000` errors, `page=-1` defaults to 1.
3. **GET /sync** — Repeated calls return 429 when sync is running.
4. **GET /health** — Status, last sync, recent errors.

---

## Future Considerations

- Basic auth between ingester and upstream (API key or OAuth).
- Prometheus/OpenTelemetry metrics.
- Additional query params (ordering, filters).

---

## Problems?

If you run into issues, please reach out.
