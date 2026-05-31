"""Configuration module — loads settings from environment variables / .env file."""
from __future__ import annotations

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    # gRPC
    grpc_server_port: int = 8119

    # Database
    db_driver: str = "mysql"
    mysql_host: str = "localhost"
    mysql_port: int = 3306
    mysql_user: str = "root"
    mysql_password: str = "123"
    mysql_db_name: str = "go_stock_prediction"
    mysql_debug: bool = False

    # Logging
    log_level: str = "INFO"

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
