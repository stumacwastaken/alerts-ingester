CREATE TABLE IF NOT EXISTS alerts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL, -- this could be pulled out into its own table if we know the sources in advance
    severity TEXT NOT NULL,
    description TEXT,
    ip_address TEXT, -- ip address in text format because sqlite.
    enrichment_type TEXT,
    source_created_at TIMESTAMP NOT NULL, -- source of truth is from the alerts api
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP -- when we create the record in our system
)