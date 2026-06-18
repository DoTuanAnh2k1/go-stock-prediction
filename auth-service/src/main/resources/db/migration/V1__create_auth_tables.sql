-- Flyway V1: create market groups tables (users table owned by GORM/Go API)
-- Note: user_id has no FK to users because GORM AutoMigrate may run after Flyway

CREATE TABLE IF NOT EXISTS market_groups (
    id          BIGSERIAL    PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS market_group_markets (
    group_id    BIGINT      NOT NULL REFERENCES market_groups(id) ON DELETE CASCADE,
    market_key  VARCHAR(20) NOT NULL,
    PRIMARY KEY (group_id, market_key)
);

CREATE TABLE IF NOT EXISTS user_market_groups (
    user_id   BIGINT NOT NULL,
    group_id  BIGINT NOT NULL REFERENCES market_groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_umg_user_id ON user_market_groups(user_id);
