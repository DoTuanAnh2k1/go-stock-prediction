-- Flyway V1: create users + market group tables.
-- Database-per-service: auth-svc owns auth_db 100% (api-svc is DB-less), so Flyway
-- creates `users` here (base columns; V2 adds profile fields). Previously `users`
-- was created by database.sql/GORM — that gap only surfaced on a fresh auth_db.
CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL    PRIMARY KEY,
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMP,
    username      VARCHAR(50)  NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(255) NOT NULL DEFAULT 'user'
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username   ON users(username);
CREATE INDEX        IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

-- Note: user_id has no FK to users to avoid migration-order coupling.

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
