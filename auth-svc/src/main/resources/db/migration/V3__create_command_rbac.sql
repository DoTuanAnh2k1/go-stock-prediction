-- Flyway V3: Command RBAC — command catalog, command groups, user assignments.
-- Mirrors the market-group RBAC (V1). user_id has no FK to users for the same
-- reason as V1 (users table owned by GORM/Go; migration order not guaranteed).

-- Handler catalog: source of truth is cli-svc code, upserted on cli-svc startup.
CREATE TABLE IF NOT EXISTS cli_handlers (
    handler_key  VARCHAR(80)  PRIMARY KEY,
    display_name VARCHAR(120) NOT NULL,
    verb         VARCHAR(10)  NOT NULL,
    resource     VARCHAR(40)  NOT NULL,
    arg_schema   JSONB        NOT NULL DEFAULT '[]',
    enabled      BOOLEAN      NOT NULL DEFAULT true
);

-- Command = admin-declared: a handler + fixed args + display name.
CREATE TABLE IF NOT EXISTS commands (
    id           BIGSERIAL    PRIMARY KEY,
    name         VARCHAR(120) NOT NULL UNIQUE,
    description  TEXT,
    handler_key  VARCHAR(80)  NOT NULL REFERENCES cli_handlers(handler_key),
    args         JSONB        NOT NULL DEFAULT '{}',
    enabled      BOOLEAN      NOT NULL DEFAULT true,
    created_at   TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS command_groups (
    id          BIGSERIAL    PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS command_group_commands (
    group_id   BIGINT NOT NULL REFERENCES command_groups(id) ON DELETE CASCADE,
    command_id BIGINT NOT NULL REFERENCES commands(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, command_id)
);

CREATE TABLE IF NOT EXISTS user_command_groups (
    user_id  BIGINT NOT NULL,
    group_id BIGINT NOT NULL REFERENCES command_groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_ucg_user_id ON user_command_groups(user_id);
CREATE INDEX IF NOT EXISTS idx_cgc_command_id ON command_group_commands(command_id);
