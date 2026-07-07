"""SQLAlchemy ORM models — mirrors the Go GORM structs exactly."""
from __future__ import annotations

from datetime import datetime
from decimal import Decimal

from sqlalchemy import (
    BigInteger,
    Boolean,
    Column,
    Date,
    DateTime,
    Integer,
    Numeric,
    String,
    Text,
    UniqueConstraint,
)
from sqlalchemy.dialects.postgresql import JSONB

from src.database.connection import Base


class GoldPrice(Base):
    __tablename__ = "gold_prices"
    __table_args__ = (
        UniqueConstraint("source", "product_type", "trading_date", name="uq_gold_price"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    source = Column(String(50), nullable=False)
    product_type = Column(String(50), nullable=False)
    trading_date = Column(DateTime, nullable=False)
    open_price = Column(Numeric(15, 2), nullable=True)   # OHLC — only XAU source; VN sources stay NULL
    high_price = Column(Numeric(15, 2), nullable=True)
    low_price = Column(Numeric(15, 2), nullable=True)
    buy_price = Column(Numeric(20, 2), nullable=False)
    sell_price = Column(Numeric(20, 2))
    currency = Column(String(3), nullable=False, default="VND")
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)


class GoldPrediction(Base):
    __tablename__ = "gold_predictions"

    id = Column(Integer, primary_key=True, autoincrement=True)
    source = Column(String(50), nullable=False)
    product_type = Column(String(50), nullable=False)
    predicted_price = Column(Numeric(20, 2), nullable=False)
    current_price = Column(Numeric(20, 2), nullable=False)
    confidence = Column(Numeric(5, 4))
    algorithm_name = Column(String(50), nullable=False)
    prediction_date = Column(DateTime, nullable=False)
    target_date = Column(DateTime, nullable=False)
    actual_price = Column(Numeric(20, 2))
    accuracy = Column(Numeric(5, 4))
    direction_correct = Column(Boolean, nullable=True)
    status = Column(String(20), default="pending")
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)
    deleted_at = Column(DateTime)


class NasdaqPrice(Base):
    __tablename__ = "nasdaq_prices"
    __table_args__ = (
        UniqueConstraint("symbol", "trading_date", name="idx_nasdaq_symbol_date"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    symbol = Column(String(10), nullable=False)
    company_name = Column(String(200))
    open_price = Column(Numeric(15, 4))
    high_price = Column(Numeric(15, 4))
    low_price = Column(Numeric(15, 4))
    close_price = Column(Numeric(15, 4), nullable=False)
    volume = Column(BigInteger)
    trading_date = Column(Date, nullable=False)
    currency = Column(String(3), nullable=False, default="USD")
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)
    deleted_at = Column(DateTime)


class NasdaqPrediction(Base):
    __tablename__ = "nasdaq_predictions"

    id = Column(Integer, primary_key=True, autoincrement=True)
    symbol = Column(String(10), nullable=False)
    algorithm_name = Column(String(100), nullable=False)
    predicted_price = Column(Numeric(15, 4), nullable=False)
    current_price = Column(Numeric(15, 4), nullable=False)
    confidence = Column(Numeric(5, 4))
    prediction_date = Column(DateTime, nullable=False)
    target_date = Column(DateTime, nullable=False)
    actual_price = Column(Numeric(15, 4))
    accuracy = Column(Numeric(5, 4))
    direction_correct = Column(Boolean, nullable=True)
    status = Column(String(20), default="pending")
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)
    deleted_at = Column(DateTime)


class SP500Price(Base):
    __tablename__ = "sp500_prices"
    __table_args__ = (
        UniqueConstraint("symbol", "trading_date", name="idx_sp500_symbol_date"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    symbol = Column(String(10), nullable=False)
    company_name = Column(String(200))
    open_price = Column(Numeric(15, 4))
    high_price = Column(Numeric(15, 4))
    low_price = Column(Numeric(15, 4))
    close_price = Column(Numeric(15, 4), nullable=False)
    volume = Column(BigInteger)
    trading_date = Column(Date, nullable=False)
    currency = Column(String(3), nullable=False, default="USD")
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)
    deleted_at = Column(DateTime)


class SP500Prediction(Base):
    __tablename__ = "sp500_predictions"

    id = Column(Integer, primary_key=True, autoincrement=True)
    symbol = Column(String(10), nullable=False)
    algorithm_name = Column(String(100), nullable=False)
    predicted_price = Column(Numeric(15, 4), nullable=False)
    current_price = Column(Numeric(15, 4), nullable=False)
    confidence = Column(Numeric(5, 4))
    prediction_date = Column(DateTime, nullable=False)
    target_date = Column(DateTime, nullable=False)
    actual_price = Column(Numeric(15, 4))
    accuracy = Column(Numeric(5, 4))
    direction_correct = Column(Boolean, nullable=True)
    status = Column(String(20), default="pending")
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)
    deleted_at = Column(DateTime)


class CryptoPrice(Base):
    __tablename__ = "crypto_prices"
    __table_args__ = (
        UniqueConstraint("coin_id", "trading_date", name="idx_crypto_coin_date"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    coin_id = Column(String(50), nullable=False)
    symbol = Column(String(10), nullable=False)
    open_price = Column(Numeric(20, 2), nullable=True)   # OHLC candlestick
    high_price = Column(Numeric(20, 2), nullable=True)
    low_price = Column(Numeric(20, 2), nullable=True)
    close_price = Column(Numeric(20, 2), nullable=False)
    market_cap = Column(Numeric(30, 2))
    volume24h = Column("volume24h", Numeric(30, 2))
    trading_date = Column(Date, nullable=False)
    currency = Column(String(3), nullable=False, default="USD")
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)
    deleted_at = Column(DateTime)


class CryptoPrediction(Base):
    __tablename__ = "crypto_predictions"

    id = Column(Integer, primary_key=True, autoincrement=True)
    coin_id = Column(String(50), nullable=False)
    symbol = Column(String(10), nullable=False)
    algorithm_name = Column(String(100), nullable=False)
    predicted_price = Column(Numeric(20, 2), nullable=False)
    current_price = Column(Numeric(20, 2), nullable=False)
    confidence = Column(Numeric(5, 4))
    prediction_date = Column(DateTime, nullable=False)
    target_date = Column(DateTime, nullable=False)
    actual_price = Column(Numeric(20, 2))
    accuracy = Column(Numeric(5, 4))
    direction_correct = Column(Boolean, nullable=True)
    status = Column(String(20), default="pending")
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)
    deleted_at = Column(DateTime)


class TrainingLog(Base):
    __tablename__ = "training_logs"

    id = Column(Integer, primary_key=True, autoincrement=True)
    session_id = Column(String(100), nullable=False)
    algorithm_name = Column(String(100), nullable=False)
    total_stocks = Column(Integer, default=0)
    success_count = Column(Integer, default=0)
    error_count = Column(Integer, default=0)
    accuracy = Column(Numeric(10, 4))
    duration_ms = Column(BigInteger, default=0)
    error_details = Column(Text)
    started_at = Column(DateTime)
    completed_at = Column(DateTime)
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)
    deleted_at = Column(DateTime)
    market_key = Column(String(50), default="gold")


class SyncLog(Base):
    __tablename__ = "sync_logs"

    id = Column(Integer, primary_key=True, autoincrement=True)
    sync_date = Column(DateTime, nullable=False)
    success_count = Column(Integer, default=0)
    error_count = Column(Integer, default=0)
    duration_ms = Column(BigInteger, default=0)
    source = Column(String(100))
    error_message = Column(Text)
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)
    deleted_at = Column(DateTime)


class CronSchedule(Base):
    __tablename__ = "cron_schedules"

    id = Column(Integer, primary_key=True, autoincrement=True)
    job_key = Column(String(100), nullable=False, unique=True)
    job_name = Column(String(200))
    cron_expression = Column(String(100), nullable=False)
    enabled = Column(Boolean, default=True)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)


class User(Base):
    __tablename__ = "users"

    id = Column(Integer, primary_key=True, autoincrement=True)
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)
    deleted_at = Column(DateTime)
    username = Column(String(50), nullable=False, unique=True)
    password_hash = Column(Text, nullable=False)
    role = Column(String(191), nullable=False, default="user")


class SimBot(Base):
    __tablename__ = "sim_bots"

    id = Column(String(50), primary_key=True)
    market = Column(String(20), nullable=False, index=True)
    algorithm = Column(String(50), nullable=False)
    symbol = Column(String(30), nullable=True)  # NULL = pooled per-market bot; set = per-symbol bot
    display_name = Column(String(100), nullable=False)
    initial_capital = Column(Numeric(20, 2), nullable=False)
    currency = Column(String(5), nullable=False)
    buy_threshold = Column(Numeric(5, 2), default=Decimal("1.50"))
    sell_threshold = Column(Numeric(5, 2), default=Decimal("1.00"))
    min_confidence = Column(Numeric(4, 2), default=Decimal("0.60"))
    stop_loss = Column(Numeric(5, 2), default=Decimal("5.00"))
    take_profit = Column(Numeric(5, 2), default=Decimal("8.00"))
    max_position_pct = Column(Numeric(5, 2), default=Decimal("15.00"))
    max_positions = Column(Integer, default=5)
    is_active = Column(Boolean, default=True)
    trailing_stop = Column(Boolean, nullable=False, default=False)
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)


class SimSession(Base):
    __tablename__ = "sim_sessions"

    id = Column(BigInteger, primary_key=True, autoincrement=True)
    bot_id = Column(String(50), nullable=False, index=True)
    start_date = Column(DateTime, nullable=False)
    end_date = Column(DateTime)
    status = Column(String(20), default="running")  # running, completed, paused
    mode = Column(String(20), default="backtest")   # backtest, live
    created_at = Column(DateTime, default=datetime.now)
    # Pre-computed KPIs (set by update_session_kpis after each session end)
    total_trades     = Column(Integer,    nullable=False, default=0)
    wins             = Column(Integer,    nullable=False, default=0)
    losses           = Column(Integer,    nullable=False, default=0)
    breakeven        = Column(Integer,    nullable=False, default=0)
    total_pnl        = Column(Numeric(20, 2), nullable=False, default=0)
    total_return_pct = Column(Numeric(8, 4),  nullable=True)
    win_rate         = Column(Numeric(5, 4),  nullable=True)
    profit_factor    = Column(Numeric(8, 4),  nullable=True)
    max_drawdown_pct = Column(Numeric(8, 4),  nullable=True)
    kpi_updated_at   = Column(DateTime,  nullable=True)


class SimTrade(Base):
    __tablename__ = "sim_trades"

    id = Column(BigInteger, primary_key=True, autoincrement=True)
    session_id = Column(BigInteger, nullable=False, index=True)
    bot_id = Column(String(50), nullable=False, index=True)
    symbol = Column(String(20), nullable=False)
    action = Column(String(5), nullable=False)      # BUY, SELL
    quantity = Column(Numeric(20, 6), nullable=False)
    price = Column(Numeric(20, 4), nullable=False)
    trade_value = Column(Numeric(20, 2), nullable=False)
    signal_strength = Column(Numeric(8, 4))
    confidence = Column(Numeric(4, 3))
    trade_date = Column(DateTime, nullable=False, index=True)
    close_reason = Column(String(20))               # signal, stop_loss, take_profit
    entry_trade_id = Column(BigInteger)
    pnl = Column(Numeric(20, 2))
    pnl_pct = Column(Numeric(8, 4))
    created_at = Column(DateTime, default=datetime.now)
    # Hourly granularity: full datetime of the trade (mirrors trade_date for new rows)
    trade_at = Column(DateTime, nullable=True)


class SimPortfolioSnapshot(Base):
    __tablename__ = "sim_portfolio_snapshots"
    __table_args__ = (
        UniqueConstraint("session_id", "snapshot_date", name="uk_sim_snap"),
    )

    id = Column(BigInteger, primary_key=True, autoincrement=True)
    session_id = Column(BigInteger, nullable=False, index=True)
    bot_id = Column(String(50), nullable=False)
    snapshot_date = Column(Date, nullable=False)
    # Hourly granularity: full datetime for upsert key (1 row per session per hour)
    snapshot_at = Column(DateTime, nullable=True)
    cash_balance = Column(Numeric(20, 2), nullable=False)
    positions_value = Column(Numeric(20, 2), nullable=False)
    total_value = Column(Numeric(20, 2), nullable=False)
    total_return_pct = Column(Numeric(8, 4))
    open_positions = Column(Integer, default=0)


# ---------------------------------------------------------------------------
# Intraday price tables (hourly bars, last 24-48h)
# ---------------------------------------------------------------------------

class NasdaqIntradayPrice(Base):
    __tablename__ = "nasdaq_intraday_prices"
    __table_args__ = (
        UniqueConstraint("symbol", "timestamp", name="idx_nasdaq_intraday_symbol_ts"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    symbol = Column(String(20), nullable=False)
    timestamp = Column(DateTime, nullable=False)
    open_price = Column(Numeric(20, 6))
    high_price = Column(Numeric(20, 6))
    low_price = Column(Numeric(20, 6))
    close_price = Column(Numeric(20, 6))
    volume = Column(BigInteger)
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)


class SP500IntradayPrice(Base):
    __tablename__ = "sp500_intraday_prices"
    __table_args__ = (
        UniqueConstraint("symbol", "timestamp", name="idx_sp500_intraday_symbol_ts"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    symbol = Column(String(20), nullable=False)
    timestamp = Column(DateTime, nullable=False)
    open_price = Column(Numeric(20, 6))
    high_price = Column(Numeric(20, 6))
    low_price = Column(Numeric(20, 6))
    close_price = Column(Numeric(20, 6))
    volume = Column(BigInteger)
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)


class CryptoIntradayPrice(Base):
    __tablename__ = "crypto_intraday_prices"
    __table_args__ = (
        UniqueConstraint("coin_id", "timestamp", name="idx_crypto_intraday_coin_ts"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    coin_id = Column(String(50), nullable=False)
    timestamp = Column(DateTime, nullable=False)
    open_price = Column(Numeric(30, 8), nullable=True)   # OHLC candlestick
    high_price = Column(Numeric(30, 8), nullable=True)
    low_price = Column(Numeric(30, 8), nullable=True)
    price = Column(Numeric(30, 8))                       # close / line-chart price
    market_cap = Column(Numeric(30, 2))
    volume = Column(Numeric(30, 2))
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)


class GoldIntradayPrice(Base):
    __tablename__ = "gold_intraday_prices"
    __table_args__ = (
        UniqueConstraint("source", "product_type", "timestamp", name="idx_gold_intraday_src_prod_ts"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    source = Column(String(50), nullable=False)
    product_type = Column(String(50), nullable=False)
    timestamp = Column(DateTime, nullable=False)
    open_price = Column(Numeric(15, 2), nullable=True)   # OHLC — only XAU source; VN sources stay NULL
    high_price = Column(Numeric(15, 2), nullable=True)
    low_price = Column(Numeric(15, 2), nullable=True)
    buy_price = Column(Numeric(15, 2))
    sell_price = Column(Numeric(15, 2))
    currency = Column(String(3), nullable=False)
    created_at = Column(DateTime, default=datetime.now)
    updated_at = Column(DateTime, default=datetime.now, onupdate=datetime.now)


class StockFundamental(Base):
    """Latest fundamental snapshot per (market, symbol) — yfinance, weekly upsert.

    Only NASDAQ / SP500 symbols have data; other markets have no rows.
    Consumed by transformer_nn as static context features.
    """
    __tablename__ = "stock_fundamentals"

    id = Column(BigInteger, primary_key=True, autoincrement=True)
    market_key = Column(String(16), nullable=False)
    symbol = Column(String(20), nullable=False)
    pe_ratio = Column(Numeric(20, 4), nullable=True)
    forward_pe = Column(Numeric(20, 4), nullable=True)
    price_to_book = Column(Numeric(20, 4), nullable=True)
    eps_ttm = Column(Numeric(20, 4), nullable=True)
    revenue_growth = Column(Numeric(12, 6), nullable=True)
    earnings_growth = Column(Numeric(12, 6), nullable=True)
    profit_margin = Column(Numeric(12, 6), nullable=True)
    debt_to_equity = Column(Numeric(20, 4), nullable=True)
    dividend_yield = Column(Numeric(12, 6), nullable=True)
    beta = Column(Numeric(12, 6), nullable=True)
    market_cap = Column(Numeric(30, 2), nullable=True)
    fetched_at = Column(DateTime, nullable=False, default=datetime.now)


class StockSplit(Base):
    """Authoritative stock-split register for NASDAQ / SP500 symbols.

    One row per (market_key, symbol, split_date).  ``applied_at`` is NULL until
    ``apply_pending_splits()`` has back-adjusted the historical price tables.
    """
    __tablename__ = "stock_splits"
    __table_args__ = (
        UniqueConstraint("market_key", "symbol", "split_date", name="uq_stock_splits_sym_date"),
    )

    id = Column(BigInteger, primary_key=True, autoincrement=True)
    market_key = Column(String(20), nullable=False)         # 'NASDAQ' | 'SP500'
    symbol = Column(String(20), nullable=False)
    split_date = Column(Date, nullable=False)                # ex-date (ICT wallclock)
    ratio = Column(Numeric(12, 6), nullable=False)           # numerator / denominator
    numerator = Column(Numeric(12, 6), nullable=False)
    denominator = Column(Numeric(12, 6), nullable=False)
    applied_at = Column(DateTime, nullable=True)             # NULL = not yet applied
    created_at = Column(DateTime, nullable=False, default=datetime.now)


class PipelineReport(Base):
    """One row per pipeline run — records outcome of crawl→train→predict pipeline."""
    __tablename__ = "pipeline_reports"

    id = Column(BigInteger, primary_key=True, autoincrement=True)
    pipeline_key = Column(String(50), nullable=False)
    market = Column(String(20), nullable=False)
    status = Column(String(20), nullable=False)          # success | partial | failed | skipped
    started_at = Column(DateTime, nullable=False)
    finished_at = Column(DateTime, nullable=True)
    duration_ms = Column(BigInteger, nullable=True)
    crawled_count = Column(Integer, nullable=False, default=0)
    predictions_count = Column(Integer, nullable=False, default=0)
    trained = Column(Boolean, nullable=False, default=False)
    steps = Column(JSONB, nullable=True)                 # list[{label, status, detail}]
    error = Column(Text, nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.now)


