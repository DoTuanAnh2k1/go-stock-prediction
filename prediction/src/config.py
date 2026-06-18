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
