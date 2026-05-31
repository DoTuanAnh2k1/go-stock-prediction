"""SQLAlchemy ORM models — mirrors the Go GORM structs exactly."""
from __future__ import annotations

from datetime import datetime, date
from decimal import Decimal
from typing import Optional

from sqlalchemy import (
    BigInteger, Boolean, Column, DateTime, Date, ForeignKey, Index,
    Integer, Numeric, String, Text, UniqueConstraint,
)
from sqlalchemy.orm import relationship

from src.database.connection import Base


class Exchange(Base):
    __tablename__ = "exchanges"

    id = Column(Integer, primary_key=True, autoincrement=True)
    code = Column(String(10), nullable=False, unique=True)
    name = Column(String(100), nullable=False)
    timezone = Column(String(50), default="Asia/Ho_Chi_Minh")
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    deleted_at = Column(DateTime)

    stocks = relationship("Stock", back_populates="exchange")


class Stock(Base):
    __tablename__ = "stocks"

    id = Column(Integer, primary_key=True, autoincrement=True)
    symbol = Column(String(10), nullable=False, unique=True)
    company_name = Column(String(200), nullable=False)
    exchange_id = Column(Integer, ForeignKey("exchanges.id"), nullable=False)
    is_vn30 = Column(Boolean, default=False)
    is_vn100 = Column(Boolean, default=False)
    listing_date = Column(DateTime)
    sector = Column(String(100))
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    deleted_at = Column(DateTime)

    exchange = relationship("Exchange", back_populates="stocks")
    prices = relationship("StockPrice", back_populates="stock")
    predictions = relationship("Prediction", back_populates="stock")


class StockPrice(Base):
    __tablename__ = "stock_prices"
    __table_args__ = (
        UniqueConstraint("stock_id", "trading_date", name="uq_stock_price_date"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    stock_id = Column(Integer, ForeignKey("stocks.id"), nullable=False)
    trading_date = Column(DateTime, nullable=False)
    open_price = Column(Numeric(15, 2), nullable=False)
    high_price = Column(Numeric(15, 2), nullable=False)
    low_price = Column(Numeric(15, 2), nullable=False)
    close_price = Column(Numeric(15, 2), nullable=False)
    volume = Column(BigInteger, nullable=False)
    value = Column(Numeric(20, 2))
    change = Column(Numeric(15, 2))
    change_percent = Column(Numeric(5, 4))
    foreign_buy = Column(BigInteger, default=0)
    foreign_sell = Column(BigInteger, default=0)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    deleted_at = Column(DateTime)

    stock = relationship("Stock", back_populates="prices")


class Prediction(Base):
    __tablename__ = "predictions"

    id = Column(Integer, primary_key=True, autoincrement=True)
    stock_id = Column(Integer, ForeignKey("stocks.id"), nullable=False)
    predicted_price = Column(Numeric(15, 2), nullable=False)
    current_price = Column(Numeric(15, 2), nullable=False)
    confidence = Column(Numeric(5, 4))
    algorithm_name = Column(String(50), nullable=False)
    prediction_date = Column(DateTime, nullable=False)
    target_date = Column(DateTime, nullable=False)
    actual_price = Column(Numeric(15, 2))
    accuracy = Column(Numeric(5, 4))
    status = Column(String(20), default="pending")
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    deleted_at = Column(DateTime)

    stock = relationship("Stock", back_populates="predictions")


class GoldPrice(Base):
    __tablename__ = "gold_prices"
    __table_args__ = (
        UniqueConstraint("source", "product_type", "trading_date", name="uq_gold_price"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    source = Column(String(50), nullable=False)
    product_type = Column(String(50), nullable=False)
    trading_date = Column(DateTime, nullable=False)
    buy_price = Column(Numeric(20, 2), nullable=False)
    sell_price = Column(Numeric(20, 2))
    currency = Column(String(3), nullable=False, default="VND")
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


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
    status = Column(String(20), default="pending")
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
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
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
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
    status = Column(String(20), default="pending")
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    deleted_at = Column(DateTime)


class CryptoPrice(Base):
    __tablename__ = "crypto_prices"
    __table_args__ = (
        UniqueConstraint("coin_id", "trading_date", name="idx_crypto_coin_date"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    coin_id = Column(String(50), nullable=False)
    symbol = Column(String(10), nullable=False)
    close_price = Column(Numeric(20, 2), nullable=False)
    market_cap = Column(Numeric(30, 2))
    volume24h = Column("volume24h", Numeric(30, 2))
    trading_date = Column(Date, nullable=False)
    currency = Column(String(3), nullable=False, default="USD")
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
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
    status = Column(String(20), default="pending")
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    deleted_at = Column(DateTime)


class FuelPrice(Base):
    __tablename__ = "fuel_prices"
    __table_args__ = (
        UniqueConstraint("product_type", "trading_date", name="idx_fuel_product_date"),
    )

    id = Column(Integer, primary_key=True, autoincrement=True)
    product_type = Column(String(30), nullable=False)
    price = Column(Numeric(10, 3), nullable=False)
    trading_date = Column(Date, nullable=False)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    deleted_at = Column(DateTime)


class FuelPrediction(Base):
    __tablename__ = "fuel_predictions"

    id = Column(Integer, primary_key=True, autoincrement=True)
    product_type = Column(String(30), nullable=False)
    algorithm_name = Column(String(100), nullable=False)
    predicted_price = Column(Numeric(10, 3), nullable=False)
    current_price = Column(Numeric(10, 3), nullable=False)
    confidence = Column(Numeric(5, 4))
    prediction_date = Column(DateTime, nullable=False)
    target_date = Column(DateTime, nullable=False)
    actual_price = Column(Numeric(10, 3))
    accuracy = Column(Numeric(5, 4))
    status = Column(String(20), default="pending")
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
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
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    deleted_at = Column(DateTime)
    market_key = Column(String(50), default="vn30")


class SyncLog(Base):
    __tablename__ = "sync_logs"

    id = Column(Integer, primary_key=True, autoincrement=True)
    sync_date = Column(DateTime, nullable=False)
    success_count = Column(Integer, default=0)
    error_count = Column(Integer, default=0)
    duration_ms = Column(BigInteger, default=0)
    source = Column(String(100))
    error_message = Column(Text)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    deleted_at = Column(DateTime)


class CronSchedule(Base):
    __tablename__ = "cron_schedules"

    id = Column(Integer, primary_key=True, autoincrement=True)
    job_key = Column(String(100), nullable=False, unique=True)
    job_name = Column(String(200))
    cron_expression = Column(String(100), nullable=False)
    enabled = Column(Boolean, default=True)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


class User(Base):
    __tablename__ = "users"

    id = Column(Integer, primary_key=True, autoincrement=True)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    deleted_at = Column(DateTime)
    username = Column(String(50), nullable=False, unique=True)
    password_hash = Column(Text, nullable=False)
    role = Column(String(191), nullable=False, default="user")
