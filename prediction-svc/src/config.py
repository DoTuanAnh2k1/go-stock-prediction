"""Configuration module — loads settings from environment variables / .env file."""
from __future__ import annotations

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    # gRPC
    grpc_server_port: int = 8119

    # Database — PostgreSQL
    postgres_host: str = "localhost"
    postgres_port: int = 5432
    postgres_user: str = "postgres"
    postgres_password: str = "123"
    postgres_db: str = "go_stock_prediction"
    postgres_debug: bool = False

    # Deprecated MySQL vars — kept so docker-compose .env files with MYSQL_* don't error
    mysql_host: str = "localhost"
    mysql_port: int = 3306

    # Logging
    log_level: str = "INFO"

    # Backup
    backup_dir: str = "/backups"

    # RL DQN model checkpoints
    rl_model_dir: str = "/models"

    # Service registry (service-mgt)
    service_mgt_enabled: bool = False
    registry_grpc_target: str = "service-mgt:8121"

    # Per-symbol models (additive, coexists with pooled per-market models)
    # When enabled: each (symbol × algorithm) gets its own trained model, its
    # predictions are written with the "__ps" algorithm_name suffix, and a
    # per-symbol bot fleet is seeded. Disabled = legacy pooled behaviour only.
    per_symbol_enabled: bool = False
    # Minimum price points a symbol must have for a given algorithm family to be
    # trained per-symbol. Data-starved (symbol, algo) pairs are skipped.
    per_symbol_min_points: int = 80
    # Thread-pool worker count for per-symbol training and bot live-steps.
    # 0 → auto (os.cpu_count()). Bounded internally to a sane max.
    per_symbol_workers: int = 0

    class Config:
        env_file = ".env"
        case_sensitive = False
        extra = "ignore"  # ignore unknown env vars from Go service


_settings: Settings | None = None


def get_settings() -> Settings:
    global _settings
    if _settings is None:
        _settings = Settings()
    return _settings
