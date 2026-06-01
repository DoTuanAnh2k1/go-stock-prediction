"""SQLAlchemy database connection and session management."""
from __future__ import annotations

from collections.abc import Generator
from contextlib import contextmanager

from sqlalchemy import create_engine
from sqlalchemy.orm import DeclarativeBase, Session, sessionmaker

from src.config import get_settings
from src.utils.logger import get_logger

log = get_logger("database")

_engine = None
_SessionLocal = None


class Base(DeclarativeBase):
    pass


def get_engine():
    global _engine
    if _engine is None:
        raise RuntimeError("Database not initialized. Call init_db() first.")
    return _engine


def init_db() -> None:
    """Initialize the database engine and session factory."""
    global _engine, _SessionLocal

    cfg = get_settings()

    connection_string = (
        f"mysql+pymysql://{cfg.mysql_user}:{cfg.mysql_password}"
        f"@{cfg.mysql_host}:{cfg.mysql_port}/{cfg.mysql_db_name}"
        f"?charset=utf8mb4"
    )

    _engine = create_engine(
        connection_string,
        echo=cfg.mysql_debug,
        pool_pre_ping=True,
        pool_size=10,
        max_overflow=20,
        pool_recycle=3600,
    )

    _SessionLocal = sessionmaker(bind=_engine, autocommit=False, autoflush=False)

    log.info("database.connected", host=cfg.mysql_host, db=cfg.mysql_db_name)


def get_session() -> Session:
    """Return a new SQLAlchemy session. Caller is responsible for closing."""
    if _SessionLocal is None:
        raise RuntimeError("Database not initialized.")
    return _SessionLocal()


@contextmanager
def session_scope() -> Generator[Session, None, None]:
    """Provide a transactional scope around a series of operations."""
    session = get_session()
    try:
        yield session
        session.commit()
    except Exception:
        session.rollback()
        raise
    finally:
        session.close()
