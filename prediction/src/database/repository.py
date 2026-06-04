"""Repository layer — all DB queries go through this class."""
from __future__ import annotations

from datetime import datetime, timedelta
from decimal import Decimal

from sqlalchemy.orm import Session

from src.database.connection import get_session, session_scope
from src.database.models import (
    CronSchedule,
    CryptoPrediction,
    CryptoPrice,
    Exchange,
    FuelPrediction,
    FuelPrice,
    GoldPrediction,
    GoldPrice,
    NasdaqPrediction,
    NasdaqPrice,
    Prediction,
    SP500Prediction,
    SP500Price,
    Stock,
    StockPrice,
    SyncLog,
    TrainingLog,
)
from src.utils.logger import get_logger

log = get_logger("repository")

HOSE_EXCHANGE_CODE = "HOSE"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _get_or_create_exchange(session: Session, name: str, code: str) -> Exchange:
    ex = session.query(Exchange).filter_by(code=code).first()
    if ex is None:
        ex = Exchange(name=name, code=code)
        session.add(ex)
        session.flush()
    return ex


def _get_or_create_stock(session: Session, symbol: str, exchange_id: int) -> Stock:
    stock = session.query(Stock).filter_by(symbol=symbol).first()
    if stock is None:
        stock = Stock(symbol=symbol, company_name=symbol, exchange_id=exchange_id)
        session.add(stock)
        session.flush()
    return stock


# ---------------------------------------------------------------------------
# VN30 / Stock
# ---------------------------------------------------------------------------

def upsert_stock_price(
    symbol: str,
    trading_date: datetime,
    open_price: Decimal,
    high_price: Decimal,
    low_price: Decimal,
    close_price: Decimal,
    volume: int,
    value: Decimal,
    change: Decimal,
    change_percent: Decimal,
) -> None:
    with session_scope() as session:
        exchange = _get_or_create_exchange(session, "Ho Chi Minh Stock Exchange", HOSE_EXCHANGE_CODE)
        stock = _get_or_create_stock(session, symbol, exchange.id)

        existing = (
            session.query(StockPrice)
            .filter_by(stock_id=stock.id, trading_date=trading_date)
            .first()
        )
        if existing:
            existing.open_price = open_price
            existing.high_price = high_price
            existing.low_price = low_price
            existing.close_price = close_price
            existing.volume = volume
            existing.value = value
            existing.change = change
            existing.change_percent = change_percent
            existing.updated_at = datetime.utcnow()
        else:
            sp = StockPrice(
                stock_id=stock.id,
                trading_date=trading_date,
                open_price=open_price,
                high_price=high_price,
                low_price=low_price,
                close_price=close_price,
                volume=volume,
                value=value,
                change=change,
                change_percent=change_percent,
            )
            session.add(sp)


def get_vn30_stocks() -> list[Stock]:
    with get_session() as session:
        return session.query(Stock).all()


def get_stock_by_symbol(symbol: str) -> Stock | None:
    session = get_session()
    try:
        return session.query(Stock).filter_by(symbol=symbol).first()
    finally:
        session.close()


def get_stock_prices_asc(stock_id: int, limit: int = 270) -> list[StockPrice]:
    """Return stock prices in ASC order (oldest first). DB stores DESC."""
    session = get_session()
    try:
        rows = (
            session.query(StockPrice)
            .filter(
                StockPrice.stock_id == stock_id,
                StockPrice.deleted_at.is_(None),
            )
            .order_by(StockPrice.trading_date.desc())
            .limit(limit)
            .all()
        )
        return list(reversed(rows))
    finally:
        session.close()


def get_stock_prices_range_asc(stock_id: int, from_date: datetime, to_date: datetime) -> list[StockPrice]:
    session = get_session()
    try:
        rows = (
            session.query(StockPrice)
            .filter(
                StockPrice.stock_id == stock_id,
                StockPrice.trading_date >= from_date,
                StockPrice.trading_date <= to_date,
                StockPrice.deleted_at.is_(None),
            )
            .order_by(StockPrice.trading_date.desc())
            .all()
        )
        return list(reversed(rows))
    finally:
        session.close()


def create_prediction(
    stock_id: int,
    predicted_price: Decimal,
    current_price: Decimal,
    confidence: Decimal,
    algorithm_name: str,
    prediction_date: datetime,
    target_date: datetime,
    actual_price: Decimal | None = None,
    accuracy: Decimal | None = None,
    status: str = "pending",
) -> None:
    with session_scope() as session:
        pred = Prediction(
            stock_id=stock_id,
            predicted_price=predicted_price,
            current_price=current_price,
            confidence=confidence,
            algorithm_name=algorithm_name,
            prediction_date=prediction_date,
            target_date=target_date,
            actual_price=actual_price,
            accuracy=accuracy,
            status=status,
        )
        session.add(pred)


def bulk_create_predictions(predictions: list[dict]) -> int:
    """Insert multiple prediction rows; returns count inserted."""
    if not predictions:
        return 0
    with session_scope() as session:
        objs = [Prediction(**p) for p in predictions]
        session.bulk_save_objects(objs)
        return len(objs)


def delete_predictions_before(target_date: datetime) -> None:
    with session_scope() as session:
        session.query(Prediction).filter(
            Prediction.target_date < target_date,
            Prediction.deleted_at.is_(None),
        ).delete(synchronize_session=False)


def get_pending_predictions(days_back: int = 7) -> list[Prediction]:
    session = get_session()
    try:
        cutoff = datetime.now() - timedelta(days=days_back)
        return (
            session.query(Prediction)
            .filter(
                Prediction.status == "pending",
                Prediction.target_date <= datetime.now(),
                Prediction.target_date >= cutoff,
                Prediction.deleted_at.is_(None),
            )
            .all()
        )
    finally:
        session.close()


def update_prediction_actual(pred_id: int, actual_price: Decimal, accuracy: Decimal, status: str) -> None:
    with session_scope() as session:
        session.query(Prediction).filter_by(id=pred_id).update(
            {"actual_price": actual_price, "accuracy": accuracy, "status": status, "updated_at": datetime.utcnow()}
        )


# ---------------------------------------------------------------------------
# Gold
# ---------------------------------------------------------------------------

def upsert_gold_price(
    source: str,
    product_type: str,
    trading_date: datetime,
    buy_price: Decimal,
    sell_price: Decimal,
    currency: str,
) -> None:
    with session_scope() as session:
        existing = (
            session.query(GoldPrice)
            .filter_by(source=source, product_type=product_type, trading_date=trading_date)
            .first()
        )
        if existing:
            existing.buy_price = buy_price
            existing.sell_price = sell_price
            existing.updated_at = datetime.utcnow()
        else:
            session.add(
                GoldPrice(
                    source=source,
                    product_type=product_type,
                    trading_date=trading_date,
                    buy_price=buy_price,
                    sell_price=sell_price,
                    currency=currency,
                )
            )


def get_gold_prices_asc(source: str, product_type: str, limit: int = 270) -> list[GoldPrice]:
    session = get_session()
    try:
        rows = (
            session.query(GoldPrice)
            .filter_by(source=source, product_type=product_type)
            .order_by(GoldPrice.trading_date.desc())
            .limit(limit)
            .all()
        )
        return list(reversed(rows))
    finally:
        session.close()


def create_gold_prediction(
    source: str,
    product_type: str,
    predicted_price: Decimal,
    current_price: Decimal,
    confidence: Decimal,
    algorithm_name: str,
    prediction_date: datetime,
    target_date: datetime,
    actual_price: Decimal | None = None,
    accuracy: Decimal | None = None,
    status: str = "pending",
) -> None:
    with session_scope() as session:
        session.add(
            GoldPrediction(
                source=source,
                product_type=product_type,
                predicted_price=predicted_price,
                current_price=current_price,
                confidence=confidence,
                algorithm_name=algorithm_name,
                prediction_date=prediction_date,
                target_date=target_date,
                actual_price=actual_price,
                accuracy=accuracy,
                status=status,
            )
        )


def get_pending_gold_predictions(days_back: int = 3) -> list[GoldPrediction]:
    session = get_session()
    try:
        cutoff = datetime.now() - timedelta(days=days_back)
        return (
            session.query(GoldPrediction)
            .filter(
                GoldPrediction.status == "pending",
                GoldPrediction.target_date <= datetime.now(),
                GoldPrediction.target_date >= cutoff,
                GoldPrediction.deleted_at.is_(None),
            )
            .all()
        )
    finally:
        session.close()


def update_gold_prediction_actual(pred_id: int, actual_price: Decimal, accuracy: Decimal, status: str) -> None:
    with session_scope() as session:
        session.query(GoldPrediction).filter_by(id=pred_id).update(
            {"actual_price": actual_price, "accuracy": accuracy, "status": status, "updated_at": datetime.utcnow()}
        )


# ---------------------------------------------------------------------------
# NASDAQ
# ---------------------------------------------------------------------------

def upsert_nasdaq_price(
    symbol: str,
    trading_date,
    open_price: Decimal,
    high_price: Decimal,
    low_price: Decimal,
    close_price: Decimal,
    volume: int,
    currency: str = "USD",
) -> None:
    with session_scope() as session:
        existing = (
            session.query(NasdaqPrice)
            .filter_by(symbol=symbol, trading_date=trading_date)
            .first()
        )
        if existing:
            existing.open_price = open_price
            existing.high_price = high_price
            existing.low_price = low_price
            existing.close_price = close_price
            existing.volume = volume
            existing.updated_at = datetime.utcnow()
        else:
            session.add(
                NasdaqPrice(
                    symbol=symbol,
                    trading_date=trading_date,
                    open_price=open_price,
                    high_price=high_price,
                    low_price=low_price,
                    close_price=close_price,
                    volume=volume,
                    currency=currency,
                )
            )


def get_nasdaq_prices_asc(symbol: str, limit: int = 270) -> list[NasdaqPrice]:
    session = get_session()
    try:
        rows = (
            session.query(NasdaqPrice)
            .filter(NasdaqPrice.symbol == symbol, NasdaqPrice.deleted_at.is_(None))
            .order_by(NasdaqPrice.trading_date.desc())
            .limit(limit)
            .all()
        )
        return list(reversed(rows))
    finally:
        session.close()


def get_nasdaq_symbols() -> list[str]:
    session = get_session()
    try:
        rows = (
            session.query(NasdaqPrice.symbol)
            .filter(NasdaqPrice.deleted_at.is_(None))
            .distinct()
            .all()
        )
        return [r.symbol for r in rows]
    finally:
        session.close()


def create_nasdaq_prediction(
    symbol: str,
    predicted_price: Decimal,
    current_price: Decimal,
    confidence: Decimal,
    algorithm_name: str,
    prediction_date: datetime,
    target_date: datetime,
    actual_price: Decimal | None = None,
    accuracy: Decimal | None = None,
    status: str = "pending",
) -> None:
    with session_scope() as session:
        session.add(
            NasdaqPrediction(
                symbol=symbol,
                predicted_price=predicted_price,
                current_price=current_price,
                confidence=confidence,
                algorithm_name=algorithm_name,
                prediction_date=prediction_date,
                target_date=target_date,
                actual_price=actual_price,
                accuracy=accuracy,
                status=status,
            )
        )


# ---------------------------------------------------------------------------
# Crypto
# ---------------------------------------------------------------------------

def upsert_crypto_price(
    coin_id: str,
    symbol: str,
    trading_date,
    close_price: Decimal,
    market_cap: Decimal | None,
    volume_24h: Decimal | None,
    currency: str = "USD",
) -> None:
    with session_scope() as session:
        existing = (
            session.query(CryptoPrice)
            .filter_by(coin_id=coin_id, trading_date=trading_date)
            .first()
        )
        if existing:
            existing.close_price = close_price
            existing.market_cap = market_cap
            existing.volume24h = volume_24h
            existing.updated_at = datetime.utcnow()
        else:
            session.add(
                CryptoPrice(
                    coin_id=coin_id,
                    symbol=symbol,
                    trading_date=trading_date,
                    close_price=close_price,
                    market_cap=market_cap,
                    volume24h=volume_24h,
                    currency=currency,
                )
            )


def get_crypto_prices_asc(coin_id: str, limit: int = 270) -> list[CryptoPrice]:
    session = get_session()
    try:
        rows = (
            session.query(CryptoPrice)
            .filter(CryptoPrice.coin_id == coin_id, CryptoPrice.deleted_at.is_(None))
            .order_by(CryptoPrice.trading_date.desc())
            .limit(limit)
            .all()
        )
        return list(reversed(rows))
    finally:
        session.close()


def create_crypto_prediction(
    coin_id: str,
    symbol: str,
    predicted_price: Decimal,
    current_price: Decimal,
    confidence: Decimal,
    algorithm_name: str,
    prediction_date: datetime,
    target_date: datetime,
    actual_price: Decimal | None = None,
    accuracy: Decimal | None = None,
    status: str = "pending",
) -> None:
    with session_scope() as session:
        session.add(
            CryptoPrediction(
                coin_id=coin_id,
                symbol=symbol,
                predicted_price=predicted_price,
                current_price=current_price,
                confidence=confidence,
                algorithm_name=algorithm_name,
                prediction_date=prediction_date,
                target_date=target_date,
                actual_price=actual_price,
                accuracy=accuracy,
                status=status,
            )
        )


# ---------------------------------------------------------------------------
# Fuel
# ---------------------------------------------------------------------------

def upsert_fuel_price(product_type: str, trading_date, price: Decimal) -> None:
    with session_scope() as session:
        existing = (
            session.query(FuelPrice)
            .filter_by(product_type=product_type, trading_date=trading_date)
            .first()
        )
        if existing:
            existing.price = price
            existing.updated_at = datetime.utcnow()
        else:
            session.add(FuelPrice(product_type=product_type, trading_date=trading_date, price=price))


def get_fuel_prices_asc(product_type: str, limit: int = 270) -> list[FuelPrice]:
    session = get_session()
    try:
        rows = (
            session.query(FuelPrice)
            .filter(FuelPrice.product_type == product_type, FuelPrice.deleted_at.is_(None))
            .order_by(FuelPrice.trading_date.desc())
            .limit(limit)
            .all()
        )
        return list(reversed(rows))
    finally:
        session.close()


def create_fuel_prediction(
    product_type: str,
    predicted_price: Decimal,
    current_price: Decimal,
    confidence: Decimal,
    algorithm_name: str,
    prediction_date: datetime,
    target_date: datetime,
    actual_price: Decimal | None = None,
    accuracy: Decimal | None = None,
    status: str = "pending",
) -> None:
    with session_scope() as session:
        session.add(
            FuelPrediction(
                product_type=product_type,
                predicted_price=predicted_price,
                current_price=current_price,
                confidence=confidence,
                algorithm_name=algorithm_name,
                prediction_date=prediction_date,
                target_date=target_date,
                actual_price=actual_price,
                accuracy=accuracy,
                status=status,
            )
        )


# ---------------------------------------------------------------------------
# S&P 500
# ---------------------------------------------------------------------------

def upsert_sp500_price(
    symbol: str,
    trading_date,
    open_price: Decimal,
    high_price: Decimal,
    low_price: Decimal,
    close_price: Decimal,
    volume: int,
    currency: str = "USD",
) -> None:
    with session_scope() as session:
        existing = (
            session.query(SP500Price)
            .filter_by(symbol=symbol, trading_date=trading_date)
            .first()
        )
        if existing:
            existing.open_price = open_price
            existing.high_price = high_price
            existing.low_price = low_price
            existing.close_price = close_price
            existing.volume = volume
            existing.updated_at = datetime.utcnow()
        else:
            session.add(
                SP500Price(
                    symbol=symbol,
                    trading_date=trading_date,
                    open_price=open_price,
                    high_price=high_price,
                    low_price=low_price,
                    close_price=close_price,
                    volume=volume,
                    currency=currency,
                )
            )


def get_sp500_prices_asc(symbol: str, limit: int = 270) -> list[SP500Price]:
    session = get_session()
    try:
        rows = (
            session.query(SP500Price)
            .filter(SP500Price.symbol == symbol, SP500Price.deleted_at.is_(None))
            .order_by(SP500Price.trading_date.desc())
            .limit(limit)
            .all()
        )
        return list(reversed(rows))
    finally:
        session.close()


def get_sp500_symbols() -> list[str]:
    session = get_session()
    try:
        rows = (
            session.query(SP500Price.symbol)
            .filter(SP500Price.deleted_at.is_(None))
            .distinct()
            .all()
        )
        return [r.symbol for r in rows]
    finally:
        session.close()


def create_sp500_prediction(
    symbol: str,
    predicted_price: Decimal,
    current_price: Decimal,
    confidence: Decimal,
    algorithm_name: str,
    prediction_date: datetime,
    target_date: datetime,
    actual_price: Decimal | None = None,
    accuracy: Decimal | None = None,
    status: str = "pending",
) -> None:
    with session_scope() as session:
        session.add(
            SP500Prediction(
                symbol=symbol,
                predicted_price=predicted_price,
                current_price=current_price,
                confidence=confidence,
                algorithm_name=algorithm_name,
                prediction_date=prediction_date,
                target_date=target_date,
                actual_price=actual_price,
                accuracy=accuracy,
                status=status,
            )
        )


# ---------------------------------------------------------------------------
# Cron schedules
# ---------------------------------------------------------------------------

def get_all_cron_schedules() -> list[CronSchedule]:
    session = get_session()
    try:
        return session.query(CronSchedule).all()
    finally:
        session.close()


def get_cron_schedule(job_key: str) -> CronSchedule | None:
    session = get_session()
    try:
        return session.query(CronSchedule).filter_by(job_key=job_key).first()
    finally:
        session.close()


def upsert_cron_schedule(job_key: str, job_name: str, cron_expression: str, enabled: bool = True) -> None:
    with session_scope() as session:
        existing = session.query(CronSchedule).filter_by(job_key=job_key).first()
        if existing:
            changed = False
            if existing.cron_expression != cron_expression:
                existing.cron_expression = cron_expression
                changed = True
            if existing.job_name != job_name:
                existing.job_name = job_name
                changed = True
            if existing.enabled != enabled:
                existing.enabled = enabled
                changed = True
            if changed:
                existing.updated_at = datetime.utcnow()
            return
        session.add(
            CronSchedule(
                job_key=job_key,
                job_name=job_name,
                cron_expression=cron_expression,
                enabled=enabled,
            )
        )


# ---------------------------------------------------------------------------
# Training / Sync logs
# ---------------------------------------------------------------------------

def create_training_log(
    session_id: str,
    algorithm_name: str,
    market_key: str,
    total_stocks: int,
    success_count: int,
    error_count: int,
    accuracy: Decimal,
    duration_ms: int,
    started_at: datetime,
    completed_at: datetime,
) -> None:
    with session_scope() as session:
        session.add(
            TrainingLog(
                session_id=session_id,
                algorithm_name=algorithm_name,
                market_key=market_key,
                total_stocks=total_stocks,
                success_count=success_count,
                error_count=error_count,
                accuracy=accuracy,
                duration_ms=duration_ms,
                started_at=started_at,
                completed_at=completed_at,
            )
        )


def create_sync_log(
    source: str,
    success_count: int,
    error_count: int,
    duration_ms: int,
    error_message: str = "",
) -> None:
    with session_scope() as session:
        session.add(
            SyncLog(
                sync_date=datetime.utcnow(),
                success_count=success_count,
                error_count=error_count,
                duration_ms=duration_ms,
                source=source,
                error_message=error_message or None,
            )
        )
