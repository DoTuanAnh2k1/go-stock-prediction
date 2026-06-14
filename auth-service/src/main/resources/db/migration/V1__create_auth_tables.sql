-- Flyway V1: tạo 3 bảng mới cho market groups
-- Bảng users đã tồn tại (tạo bởi Go GORM), không touch

CREATE TABLE IF NOT EXISTS market_groups (
    id          BIGINT       NOT NULL AUTO_INCREMENT,
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    created_at  DATETIME     NOT NULL,
    updated_at  DATETIME     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_market_groups_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS market_group_markets (
    group_id    BIGINT      NOT NULL,
    market_key  VARCHAR(20) NOT NULL,
    PRIMARY KEY (group_id, market_key),
    CONSTRAINT fk_mgm_group FOREIGN KEY (group_id)
        REFERENCES market_groups(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_market_groups (
    user_id   BIGINT NOT NULL,
    group_id  BIGINT NOT NULL,
    PRIMARY KEY (user_id, group_id),
    CONSTRAINT fk_umg_user  FOREIGN KEY (user_id)
        REFERENCES users(id)         ON DELETE CASCADE,
    CONSTRAINT fk_umg_group FOREIGN KEY (group_id)
        REFERENCES market_groups(id) ON DELETE CASCADE,
    INDEX idx_umg_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
