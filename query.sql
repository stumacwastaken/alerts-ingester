-- name: Alerts :many
SELECT * FROM alerts
ORDER BY source_created_at DESC, id DESC
  LIMIT ?
  OFFSET ?;


-- name: CountAlerts :one
SELECT COUNT(*) FROM alerts;

-- name: AddAlert :exec
INSERT INTO alerts (
    source, 
    severity, 
    description, 
    source_created_at,
    enrichment_type,
    ip_address,
    fetch_history_id
) VALUES (
    ?, ?, ?, ?, ?, ?, ?
);

-- name: AddScanResult :one
INSERT into alert_fetch_history (
    reattempts,
    success,
    reason,
    scanned_at
) VALUES (
    ?, ?, ?, ?
) RETURNING id;

-- name: RecentScanErrors :many
SELECT * FROM alert_fetch_history
    WHERE success == 0
      -- Take the first 19 characters (YYYY-MM-DD HH:MM:SS) because sqlite can't understand rfc3339
      AND datetime(substr(scanned_at, 1, 19)) >= datetime('now', ?) 
    ORDER BY scanned_at DESC
    LIMIT ?;

-- name: LastSuccessfulSync :one
SELECT * FROM alert_fetch_history
   WHERE success == 1
   ORDER BY datetime(substr(scanned_at, 1, 19)) DESC
   LIMIT 1;

-- name: LastSyncs :many
SELECT * from alert_fetch_history
    ORDER BY datetime(substr(scanned_at, 1, 19)) DESC
    LIMIT ?;
