-- registry_db — service registry/discovery (owned by service-mgt). Not a hypertable.
-- Split out of the former shared database.sql for database-per-service isolation.

CREATE TABLE IF NOT EXISTS service_instances (
    id            BIGSERIAL     PRIMARY KEY,
    service_name  VARCHAR(64)   NOT NULL,
    instance_id   VARCHAR(128)  NOT NULL UNIQUE,
    address       VARCHAR(255)  NOT NULL,
    port          INTEGER       NOT NULL,
    metadata      JSONB,
    status        VARCHAR(16)   NOT NULL DEFAULT 'UP',
    ttl_seconds   INTEGER       NOT NULL DEFAULT 30,
    last_seen     TIMESTAMP     NOT NULL DEFAULT NOW(),
    registered_at TIMESTAMP     NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP     NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_service_instances_name
    ON service_instances(service_name, status);

CREATE INDEX IF NOT EXISTS idx_service_instances_status
    ON service_instances(status);
