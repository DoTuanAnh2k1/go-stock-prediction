"""Repository layer — all DB queries go through this class."""
from __future__ import annotations

from datetime import datetime, timedelta
from decimal import Decimal
from typing import Optional

from sqlalchemy import or_, text

from src.database.connection import get_session, session_scope
from src.database.models import (
    CronSchedule,
    CryptoPrediction,
    CryptoIntradayPrice,
    CryptoPrice,
    GoldIntradayPrice,
    GoldPrediction,
    GoldPrice,
    NasdaqIntradayPrice,
    NasdaqPrediction,
    NasdaqPrice,
    PipelineReport,
    SP500IntradayPrice,
    SP500Prediction,
    SP500Price,
    StockFundamental,
    StockSplit,
    SyncLog,
    TrainingLog,
)
from src.crawlers import sanity
from src.telemetry import metrics as _metrics
from src.utils.logger import get_logger

log = get_logger("repository")


def direction_verdict(pred_diff: Decimal, actual_diff: Decimal) -> bool | None:
    """Direction-accuracy verdict for a matured prediction.

    Returns None when the outcome is *unscorable* because the actual price never
    moved from the entry price (``actual_diff == 0``). That happens when reconcile
    runs before a fresh price bar exists — the market closed past the target hour,
    or a crawl lag left the daily-live row frozen at the value captured when the
    prediction was made. Scoring direction against zero movement would mark EVERY
    algorithm wrong (``sign(actual_diff) == 0`` can never match ``sign(pred_diff)``),
    so callers must leave the prediction pending instead of recording a false 'wrong'.

    Once real movement is observed it returns True when the predicted direction
    matches the actual direction, False otherwise (a flat prediction, ``pred_diff
    == 0``, never counts as a correct directional call).
    """
    if actual_diff == 0:
        return None
    if pred_diff == 0:
        return False
    return (pred_diff > 0) == (actual_diff > 0)


# ---------------------------------------------------------------------------
# Gold
# ---------------------------------------------------------------------------

def delete_pending_gold_predictions_for_symbol(source: str, product_type: str, algorithm_name: str) -> None:
    with session_scope() as session:
        session.query(GoldPrediction).filter(
            GoldPrediction.source == source,
            GoldPrediction.product_type == product_type,
            GoldPrediction.algorithm_name == algorithm_name,
            GoldPrediction.actual_price.is_(None),
            GoldPrediction.deleted_at.is_(None),
        ).delete(synchronize_session=False)


def delete_pending_nasdaq_predictions_for_symbol(symbol: str, algorithm_name: str) -> None:
    with session_scope() as session:
        session.query(NasdaqPrediction).filter(
            NasdaqPrediction.symbol == symbol,
            NasdaqPrediction.algorithm_name == algorithm_name,
            NasdaqPrediction.actual_price.is_(None),
            NasdaqPrediction.deleted_at.is_(None),
        ).delete(synchronize_session=False)


def delete_pending_crypto_predictions_for_symbol(coin_id: str, algorithm_name: str) -> None:
    with session_scope() as session:
        session.query(CryptoPrediction).filter(
            CryptoPrediction.coin_id == coin_id,
            CryptoPrediction.algorithm_name == algorithm_name,
            CryptoPrediction.actual_price.is_(None),
            CryptoPrediction.deleted_at.is_(None),
        ).delete(synchronize_session=False)


def delete_pending_sp500_predictions_for_symbol(symbol: str, algorithm_name: str) -> None:
    with session_scope() as session:
        session.query(SP500Prediction).filter(
            SP500Prediction.symbol == symbol,
            SP500Prediction.algorithm_name == algorithm_name,
            SP500Prediction.actual_price.is_(None),
            SP500Prediction.deleted_at.is_(None),
        ).delete(synchronize_session=False)


def upsert_gold_price(
    source: str,
    product_type: str,
    trading_date: datetime,
    buy_price: Decimal,
    sell_price: Decimal,
    currency: str,
    open_price: Decimal | None = None,
    high_price: Decimal | None = None,
    low_price: Decimal | None = None,
) -> None:
    """Upsert a daily gold price row.

    ``open_price``, ``high_price``, ``low_price`` are populated for the XAU/XAU_VND
    sources (Yahoo Finance already returns OHLC).  VN sources (SJC, BTMC, etc.)
    pass None — those columns remain NULL in the DB and candlestick toggle is
    disabled on the frontend for those rows.
    """
    # --- Sanity check: reject transient bad ticks on the daily-live row ---
    _sanity_key = f"GOLD:{source}:{product_type}"
    _new_close = float(sell_price)
    try:
        with session_scope() as _ss:
            _last_row = (
                _ss.query(GoldPrice.sell_price)
                .filter_by(source=source, product_type=product_type)
                .order_by(GoldPrice.trading_date.desc())
                .limit(1)
                .first()
            )
        _last_close = float(_last_row.sell_price) if _last_row is not None else None
    except Exception:
        # If the DB lookup fails (e.g. not yet initialised), skip the guard
        # and let the write proceed — bootstrap behaviour.
        _last_close = None
    _accept, _reason = sanity.check_update("GOLD", _sanity_key, _new_close, _last_close)
    if not _accept:
        log.warning(
            "crawl.sanity.reject",
            market="GOLD",
            key=_sanity_key,
            new=_new_close,
            last=_last_close,
            reason=_reason,
        )
        _metrics.record_crawl("GOLD", "reject")
        return

    with session_scope() as session:
        existing = (
            session.query(GoldPrice)
            .filter_by(source=source, product_type=product_type, trading_date=trading_date)
            .first()
        )
        if existing:
            existing.buy_price = buy_price
            existing.sell_price = sell_price
            # Only overwrite OHLC when the caller supplies a value (don't clobber
            # an existing OHLC row just because a VN-source update arrives later)
            if open_price is not None:
                existing.open_price = open_price
            if high_price is not None:
                existing.high_price = high_price
            if low_price is not None:
                existing.low_price = low_price
            existing.updated_at = datetime.now()
        else:
            session.add(
                GoldPrice(
                    source=source,
                    product_type=product_type,
                    trading_date=trading_date,
                    open_price=open_price,
                    high_price=high_price,
                    low_price=low_price,
                    buy_price=buy_price,
                    sell_price=sell_price,
                    currency=currency,
                )
            )
    _metrics.record_crawl("GOLD", "saved")


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


def get_gold_prices_asc_as_of(
    source: str, product_type: str, as_of: "datetime", limit: int = 270
) -> "list[GoldPrice]":
    """Return up to `limit` gold prices with trading_date <= as_of, in ASC order."""
    session = get_session()
    try:
        rows = (
            session.query(GoldPrice)
            .filter(
                GoldPrice.source == source,
                GoldPrice.product_type == product_type,
                GoldPrice.trading_date <= as_of,
            )
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
                GoldPrediction.deleted_at.is_(None),
                GoldPrediction.target_date <= datetime.now(),
                GoldPrediction.target_date >= cutoff,
                or_(
                    GoldPrediction.actual_price.is_(None),
                    GoldPrediction.direction_correct.is_(None),
                ),
            )
            .all()
        )
    finally:
        session.close()


def update_gold_prediction_actual(
    pred_id: int, actual_price: Decimal, accuracy: Decimal, status: str, direction_correct: bool | None = None
) -> None:
    with session_scope() as session:
        session.query(GoldPrediction).filter_by(id=pred_id).update(
            {
                "actual_price": actual_price,
                "accuracy": accuracy,
                "status": status,
                "direction_correct": direction_correct,
                "updated_at": datetime.now(),
            }
        )


def backfill_direction_correct_gold() -> int:
    """Compute direction_correct for gold predictions that have actual_price but no direction_correct."""
    with session_scope() as session:
        rows = (
            session.query(GoldPrediction)
            .filter(
                GoldPrediction.actual_price.isnot(None),
                GoldPrediction.direction_correct.is_(None),
                GoldPrediction.deleted_at.is_(None),
            )
            .all()
        )
        updated = 0
        for pred in rows:
            pred_diff = Decimal(str(pred.predicted_price)) - Decimal(str(pred.current_price))
            actual_diff = Decimal(str(pred.actual_price)) - Decimal(str(pred.current_price))
            verdict = direction_verdict(pred_diff, actual_diff)
            if verdict is None:
                # Frozen/stale actual (== entry price): unscorable — leave direction_correct
                # NULL instead of marking every algorithm wrong.
                continue
            pred.direction_correct = verdict
            updated += 1
        return updated


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
    # --- Sanity check: reject transient bad ticks on the daily-live row ---
    _sanity_key = f"NASDAQ:{symbol}"
    _new_close = float(close_price)
    try:
        with session_scope() as _ss:
            _last_row = (
                _ss.query(NasdaqPrice.close_price)
                .filter(NasdaqPrice.symbol == symbol, NasdaqPrice.deleted_at.is_(None))
                .order_by(NasdaqPrice.trading_date.desc())
                .limit(1)
                .first()
            )
        _last_close = float(_last_row.close_price) if _last_row is not None else None
    except Exception:
        _last_close = None
    _accept, _reason = sanity.check_update("NASDAQ", _sanity_key, _new_close, _last_close)
    if not _accept:
        log.warning(
            "crawl.sanity.reject",
            market="NASDAQ",
            key=_sanity_key,
            new=_new_close,
            last=_last_close,
            reason=_reason,
        )
        _metrics.record_crawl("NASDAQ100", "reject")
        return

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
            existing.updated_at = datetime.now()
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
    _metrics.record_crawl("NASDAQ100", "saved")


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


def get_nasdaq_prices_asc_as_of(
    symbol: str, as_of: "datetime", limit: int = 270
) -> "list[NasdaqPrice]":
    """Return up to `limit` NASDAQ prices with trading_date <= as_of, in ASC order."""
    session = get_session()
    try:
        rows = (
            session.query(NasdaqPrice)
            .filter(
                NasdaqPrice.symbol == symbol,
                NasdaqPrice.deleted_at.is_(None),
                NasdaqPrice.trading_date <= as_of,
            )
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


def get_pending_nasdaq_predictions(days_back: int = 7) -> list[NasdaqPrediction]:
    session = get_session()
    try:
        cutoff = datetime.now() - timedelta(days=days_back)
        return (
            session.query(NasdaqPrediction)
            .filter(
                NasdaqPrediction.deleted_at.is_(None),
                NasdaqPrediction.target_date <= datetime.now(),
                NasdaqPrediction.target_date >= cutoff,
                or_(
                    NasdaqPrediction.actual_price.is_(None),
                    NasdaqPrediction.direction_correct.is_(None),
                ),
            )
            .all()
        )
    finally:
        session.close()


def update_nasdaq_prediction_actual(
    pred_id: int, actual_price: Decimal, accuracy: Decimal, status: str, direction_correct: bool | None = None
) -> None:
    with session_scope() as session:
        session.query(NasdaqPrediction).filter_by(id=pred_id).update(
            {
                "actual_price": actual_price,
                "accuracy": accuracy,
                "status": status,
                "direction_correct": direction_correct,
                "updated_at": datetime.now(),
            }
        )


def backfill_direction_correct_nasdaq() -> int:
    """Compute direction_correct for NASDAQ predictions that have actual_price but no direction_correct."""
    with session_scope() as session:
        rows = (
            session.query(NasdaqPrediction)
            .filter(
                NasdaqPrediction.actual_price.isnot(None),
                NasdaqPrediction.direction_correct.is_(None),
                NasdaqPrediction.deleted_at.is_(None),
            )
            .all()
        )
        updated = 0
        for pred in rows:
            pred_diff = Decimal(str(pred.predicted_price)) - Decimal(str(pred.current_price))
            actual_diff = Decimal(str(pred.actual_price)) - Decimal(str(pred.current_price))
            verdict = direction_verdict(pred_diff, actual_diff)
            if verdict is None:
                # Frozen/stale actual (== entry price): unscorable — leave direction_correct
                # NULL instead of marking every algorithm wrong.
                continue
            pred.direction_correct = verdict
            updated += 1
        return updated


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
    open_price: Decimal | None = None,
    high_price: Decimal | None = None,
    low_price: Decimal | None = None,
) -> None:
    """Upsert a daily crypto price row.

    ``open_price``, ``high_price``, ``low_price`` come from CoinGecko ``/ohlc``
    endpoint.  They are optional so that callers that only know the close price
    (e.g. legacy simple/price path) can still call this without changes.
    When updating an existing row, OHLC is only written when the caller supplies
    a non-None value — avoiding accidental NULL-clobbering during plain close
    refreshes.
    """
    # --- Sanity check: reject transient bad ticks on the daily-live row ---
    _sanity_key = f"CRYPTO:{coin_id}"
    _new_close = float(close_price)
    try:
        with session_scope() as _ss:
            _last_row = (
                _ss.query(CryptoPrice.close_price)
                .filter(CryptoPrice.coin_id == coin_id, CryptoPrice.deleted_at.is_(None))
                .order_by(CryptoPrice.trading_date.desc())
                .limit(1)
                .first()
            )
        _last_close = float(_last_row.close_price) if _last_row is not None else None
    except Exception:
        _last_close = None
    _accept, _reason = sanity.check_update("CRYPTO", _sanity_key, _new_close, _last_close)
    if not _accept:
        log.warning(
            "crawl.sanity.reject",
            market="CRYPTO",
            key=_sanity_key,
            new=_new_close,
            last=_last_close,
            reason=_reason,
        )
        _metrics.record_crawl("CRYPTO", "reject")
        return

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
            if open_price is not None:
                existing.open_price = open_price
            if high_price is not None:
                existing.high_price = high_price
            if low_price is not None:
                existing.low_price = low_price
            existing.updated_at = datetime.now()
        else:
            session.add(
                CryptoPrice(
                    coin_id=coin_id,
                    symbol=symbol,
                    trading_date=trading_date,
                    open_price=open_price,
                    high_price=high_price,
                    low_price=low_price,
                    close_price=close_price,
                    market_cap=market_cap,
                    volume24h=volume_24h,
                    currency=currency,
                )
            )
    _metrics.record_crawl("CRYPTO", "saved")


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


def get_crypto_prices_asc_as_of(
    coin_id: str, as_of: "datetime", limit: int = 270
) -> "list[CryptoPrice]":
    """Return up to `limit` Crypto prices with trading_date <= as_of, in ASC order."""
    session = get_session()
    try:
        rows = (
            session.query(CryptoPrice)
            .filter(
                CryptoPrice.coin_id == coin_id,
                CryptoPrice.deleted_at.is_(None),
                CryptoPrice.trading_date <= as_of,
            )
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


def get_pending_crypto_predictions(days_back: int = 7) -> list[CryptoPrediction]:
    session = get_session()
    try:
        cutoff = datetime.now() - timedelta(days=days_back)
        return (
            session.query(CryptoPrediction)
            .filter(
                CryptoPrediction.deleted_at.is_(None),
                CryptoPrediction.target_date <= datetime.now(),
                CryptoPrediction.target_date >= cutoff,
                or_(
                    CryptoPrediction.actual_price.is_(None),
                    CryptoPrediction.direction_correct.is_(None),
                ),
            )
            .all()
        )
    finally:
        session.close()


def update_crypto_prediction_actual(
    pred_id: int, actual_price: Decimal, accuracy: Decimal, status: str, direction_correct: bool | None = None
) -> None:
    with session_scope() as session:
        session.query(CryptoPrediction).filter_by(id=pred_id).update(
            {
                "actual_price": actual_price,
                "accuracy": accuracy,
                "status": status,
                "direction_correct": direction_correct,
                "updated_at": datetime.now(),
            }
        )


def backfill_direction_correct_crypto() -> int:
    """Compute direction_correct for crypto predictions that have actual_price but no direction_correct."""
    with session_scope() as session:
        rows = (
            session.query(CryptoPrediction)
            .filter(
                CryptoPrediction.actual_price.isnot(None),
                CryptoPrediction.direction_correct.is_(None),
                CryptoPrediction.deleted_at.is_(None),
            )
            .all()
        )
        updated = 0
        for pred in rows:
            pred_diff = Decimal(str(pred.predicted_price)) - Decimal(str(pred.current_price))
            actual_diff = Decimal(str(pred.actual_price)) - Decimal(str(pred.current_price))
            verdict = direction_verdict(pred_diff, actual_diff)
            if verdict is None:
                # Frozen/stale actual (== entry price): unscorable — leave direction_correct
                # NULL instead of marking every algorithm wrong.
                continue
            pred.direction_correct = verdict
            updated += 1
        return updated


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
    # --- Sanity check: reject transient bad ticks on the daily-live row ---
    _sanity_key = f"SP500:{symbol}"
    _new_close = float(close_price)
    try:
        with session_scope() as _ss:
            _last_row = (
                _ss.query(SP500Price.close_price)
                .filter(SP500Price.symbol == symbol, SP500Price.deleted_at.is_(None))
                .order_by(SP500Price.trading_date.desc())
                .limit(1)
                .first()
            )
        _last_close = float(_last_row.close_price) if _last_row is not None else None
    except Exception:
        _last_close = None
    _accept, _reason = sanity.check_update("SP500", _sanity_key, _new_close, _last_close)
    if not _accept:
        log.warning(
            "crawl.sanity.reject",
            market="SP500",
            key=_sanity_key,
            new=_new_close,
            last=_last_close,
            reason=_reason,
        )
        _metrics.record_crawl("SP500", "reject")
        return

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
            existing.updated_at = datetime.now()
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
    _metrics.record_crawl("SP500", "saved")


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


def get_sp500_prices_asc_as_of(
    symbol: str, as_of: "datetime", limit: int = 270
) -> "list[SP500Price]":
    """Return up to `limit` S&P 500 prices with trading_date <= as_of, in ASC order."""
    session = get_session()
    try:
        rows = (
            session.query(SP500Price)
            .filter(
                SP500Price.symbol == symbol,
                SP500Price.deleted_at.is_(None),
                SP500Price.trading_date <= as_of,
            )
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


def get_pending_sp500_predictions(days_back: int = 7) -> list[SP500Prediction]:
    session = get_session()
    try:
        cutoff = datetime.now() - timedelta(days=days_back)
        return (
            session.query(SP500Prediction)
            .filter(
                SP500Prediction.deleted_at.is_(None),
                SP500Prediction.target_date <= datetime.now(),
                SP500Prediction.target_date >= cutoff,
                or_(
                    SP500Prediction.actual_price.is_(None),
                    SP500Prediction.direction_correct.is_(None),
                ),
            )
            .all()
        )
    finally:
        session.close()


def update_sp500_prediction_actual(
    pred_id: int, actual_price: Decimal, accuracy: Decimal, status: str, direction_correct: bool | None = None
) -> None:
    with session_scope() as session:
        session.query(SP500Prediction).filter_by(id=pred_id).update(
            {
                "actual_price": actual_price,
                "accuracy": accuracy,
                "status": status,
                "direction_correct": direction_correct,
                "updated_at": datetime.now(),
            }
        )


def backfill_direction_correct_sp500() -> int:
    """Compute direction_correct for SP500 predictions that have actual_price but no direction_correct."""
    with session_scope() as session:
        rows = (
            session.query(SP500Prediction)
            .filter(
                SP500Prediction.actual_price.isnot(None),
                SP500Prediction.direction_correct.is_(None),
                SP500Prediction.deleted_at.is_(None),
            )
            .all()
        )
        updated = 0
        for pred in rows:
            pred_diff = Decimal(str(pred.predicted_price)) - Decimal(str(pred.current_price))
            actual_diff = Decimal(str(pred.actual_price)) - Decimal(str(pred.current_price))
            verdict = direction_verdict(pred_diff, actual_diff)
            if verdict is None:
                # Frozen/stale actual (== entry price): unscorable — leave direction_correct
                # NULL instead of marking every algorithm wrong.
                continue
            pred.direction_correct = verdict
            updated += 1
        return updated


# ---------------------------------------------------------------------------
# Intraday prices
# ---------------------------------------------------------------------------

def upsert_nasdaq_intraday(record: NasdaqIntradayPrice) -> None:
    with session_scope() as session:
        existing = (
            session.query(NasdaqIntradayPrice)
            .filter_by(symbol=record.symbol, timestamp=record.timestamp)
            .first()
        )
        if existing:
            existing.open_price = record.open_price
            existing.high_price = record.high_price
            existing.low_price = record.low_price
            existing.close_price = record.close_price
            existing.volume = record.volume
            existing.updated_at = datetime.now()
        else:
            new = NasdaqIntradayPrice(
                symbol=record.symbol,
                timestamp=record.timestamp,
                open_price=record.open_price,
                high_price=record.high_price,
                low_price=record.low_price,
                close_price=record.close_price,
                volume=record.volume,
            )
            session.add(new)


def upsert_sp500_intraday(record: SP500IntradayPrice) -> None:
    with session_scope() as session:
        existing = (
            session.query(SP500IntradayPrice)
            .filter_by(symbol=record.symbol, timestamp=record.timestamp)
            .first()
        )
        if existing:
            existing.open_price = record.open_price
            existing.high_price = record.high_price
            existing.low_price = record.low_price
            existing.close_price = record.close_price
            existing.volume = record.volume
            existing.updated_at = datetime.now()
        else:
            new = SP500IntradayPrice(
                symbol=record.symbol,
                timestamp=record.timestamp,
                open_price=record.open_price,
                high_price=record.high_price,
                low_price=record.low_price,
                close_price=record.close_price,
                volume=record.volume,
            )
            session.add(new)


def upsert_crypto_intraday(record: CryptoIntradayPrice) -> None:
    """Upsert an intraday crypto price bar.

    Persists OHLC fields (``open_price``, ``high_price``, ``low_price``) when
    present on the record — these come from the CoinGecko ``/ohlc?days=1``
    endpoint merged with the market_chart data.  Falls back gracefully when
    OHLC attributes are absent (None).
    """
    with session_scope() as session:
        existing = (
            session.query(CryptoIntradayPrice)
            .filter_by(coin_id=record.coin_id, timestamp=record.timestamp)
            .first()
        )
        if existing:
            existing.price = record.price
            existing.market_cap = record.market_cap
            existing.volume = record.volume
            # Only overwrite OHLC when the record carries them
            if getattr(record, "open_price", None) is not None:
                existing.open_price = record.open_price
            if getattr(record, "high_price", None) is not None:
                existing.high_price = record.high_price
            if getattr(record, "low_price", None) is not None:
                existing.low_price = record.low_price
            existing.updated_at = datetime.now()
        else:
            new = CryptoIntradayPrice(
                coin_id=record.coin_id,
                timestamp=record.timestamp,
                open_price=getattr(record, "open_price", None),
                high_price=getattr(record, "high_price", None),
                low_price=getattr(record, "low_price", None),
                price=record.price,
                market_cap=record.market_cap,
                volume=record.volume,
            )
            session.add(new)


def upsert_gold_intraday(record: GoldIntradayPrice) -> None:
    """Upsert an intraday gold price bar.

    Persists OHLC fields (``open_price``, ``high_price``, ``low_price``) when
    present on the record — these come from Yahoo Finance hourly OHLC data for
    the XAU/XAU_VND sources.  VN-source records carry None for OHLC.
    """
    with session_scope() as session:
        existing = (
            session.query(GoldIntradayPrice)
            .filter_by(source=record.source, product_type=record.product_type, timestamp=record.timestamp)
            .first()
        )
        if existing:
            existing.buy_price = record.buy_price
            existing.sell_price = record.sell_price
            # Only overwrite OHLC when the record carries them
            if getattr(record, "open_price", None) is not None:
                existing.open_price = record.open_price
            if getattr(record, "high_price", None) is not None:
                existing.high_price = record.high_price
            if getattr(record, "low_price", None) is not None:
                existing.low_price = record.low_price
            existing.updated_at = datetime.now()
        else:
            new = GoldIntradayPrice(
                source=record.source,
                product_type=record.product_type,
                timestamp=record.timestamp,
                open_price=getattr(record, "open_price", None),
                high_price=getattr(record, "high_price", None),
                low_price=getattr(record, "low_price", None),
                buy_price=record.buy_price,
                sell_price=record.sell_price,
                currency=record.currency,
            )
            session.add(new)



# ---------------------------------------------------------------------------
# Intraday as-of fetchers
# ---------------------------------------------------------------------------

def get_nasdaq_intraday_asc_as_of(
    symbol: str, as_of: "datetime", limit: int = 400
) -> "list[NasdaqIntradayPrice]":
    """Return up to `limit` NASDAQ intraday bars with timestamp <= as_of, in ASC order."""
    session = get_session()
    try:
        rows = (
            session.query(NasdaqIntradayPrice)
            .filter(
                NasdaqIntradayPrice.symbol == symbol,
                NasdaqIntradayPrice.timestamp <= as_of,
            )
            .order_by(NasdaqIntradayPrice.timestamp.desc())
            .limit(limit)
            .all()
        )
        return list(reversed(rows))
    finally:
        session.close()


def get_sp500_intraday_asc_as_of(
    symbol: str, as_of: "datetime", limit: int = 400
) -> "list[SP500IntradayPrice]":
    """Return up to `limit` S&P 500 intraday bars with timestamp <= as_of, in ASC order."""
    session = get_session()
    try:
        rows = (
            session.query(SP500IntradayPrice)
            .filter(
                SP500IntradayPrice.symbol == symbol,
                SP500IntradayPrice.timestamp <= as_of,
            )
            .order_by(SP500IntradayPrice.timestamp.desc())
            .limit(limit)
            .all()
        )
        return list(reversed(rows))
    finally:
        session.close()


def get_crypto_intraday_asc_as_of(
    coin_id: str, as_of: "datetime", limit: int = 400
) -> "list[CryptoIntradayPrice]":
    """Return up to `limit` crypto intraday bars with timestamp <= as_of, in ASC order.

    Note: CryptoIntradayPrice uses the `price` column (not `close_price`) as the
    close/line-chart price.
    """
    session = get_session()
    try:
        rows = (
            session.query(CryptoIntradayPrice)
            .filter(
                CryptoIntradayPrice.coin_id == coin_id,
                CryptoIntradayPrice.timestamp <= as_of,
            )
            .order_by(CryptoIntradayPrice.timestamp.desc())
            .limit(limit)
            .all()
        )
        return list(reversed(rows))
    finally:
        session.close()


def get_gold_intraday_asc_as_of(
    source: str, as_of: "datetime", limit: int = 400
) -> "list[GoldIntradayPrice]":
    """Return up to `limit` gold intraday bars for the given source with timestamp <= as_of, in ASC order.

    Aggregates across all product_types for the source; callers use sell_price
    (or buy_price as fallback) as the close price for RL feature building.
    Returns rows ordered ASC by timestamp; multiple rows may share the same
    timestamp when multiple product_types exist — caller deduplication by taking
    the last row per timestamp is recommended if needed.
    """
    session = get_session()
    try:
        rows = (
            session.query(GoldIntradayPrice)
            .filter(
                GoldIntradayPrice.source == source,
                GoldIntradayPrice.timestamp <= as_of,
            )
            .order_by(GoldIntradayPrice.timestamp.desc())
            .limit(limit)
            .all()
        )
        return list(reversed(rows))
    finally:
        session.close()


def get_gold_intraday_sources(as_of: "datetime | None" = None) -> list[str]:
    """Return distinct source values present in gold_intraday_prices.

    Optionally filtered to sources that have at least one bar with
    timestamp <= as_of.
    """
    session = get_session()
    try:
        q = session.query(GoldIntradayPrice.source).distinct()
        if as_of is not None:
            q = q.filter(GoldIntradayPrice.timestamp <= as_of)
        rows = q.all()
        return [r.source for r in rows]
    finally:
        session.close()


# ---------------------------------------------------------------------------
# Direction accuracy
# ---------------------------------------------------------------------------

_MARKET_MODEL_MAP: dict[str, tuple] = {
    "GOLD": (GoldPrediction, "algorithm_name"),
    "NASDAQ100": (NasdaqPrediction, "algorithm_name"),
    "CRYPTO": (CryptoPrediction, "algorithm_name"),
    "SP500": (SP500Prediction, "algorithm_name"),
}


def get_direction_accuracy(market_key: str) -> dict[str, float]:
    """Return per-algorithm direction accuracy (%) for reconciled predictions in a market.

    Only rows where direction_correct IS NOT NULL are counted (i.e. reconciled rows).

    Returns:
        dict mapping algorithm_name -> accuracy_pct (0.0–100.0), e.g.
        {"moving_average": 62.5, "lstm": 58.0, ...}
    """
    mk = market_key.upper()
    entry = _MARKET_MODEL_MAP.get(mk)
    if entry is None:
        return {}

    model_cls, algo_col = entry
    session = get_session()
    try:
        rows = (
            session.query(
                getattr(model_cls, algo_col),
                getattr(model_cls, "direction_correct"),
            )
            .filter(getattr(model_cls, "direction_correct").isnot(None))
            .all()
        )

        # Aggregate in Python — group by algorithm
        totals: dict[str, int] = {}
        corrects: dict[str, int] = {}
        for algo_name, direction_correct in rows:
            totals[algo_name] = totals.get(algo_name, 0) + 1
            if direction_correct:
                corrects[algo_name] = corrects.get(algo_name, 0) + 1

        result: dict[str, float] = {}
        for algo_name, total in totals.items():
            result[algo_name] = round(corrects.get(algo_name, 0) / total * 100, 2)
        return result
    finally:
        session.close()


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
                existing.updated_at = datetime.now()
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
                sync_date=datetime.now(),
                success_count=success_count,
                error_count=error_count,
                duration_ms=duration_ms,
                source=source,
                error_message=error_message or None,
            )
        )


# ---------------------------------------------------------------------------
# Pipeline reports
# ---------------------------------------------------------------------------

def create_pipeline_report(
    pipeline_key: str,
    market: str,
    status: str,
    started_at: datetime,
    finished_at: datetime | None,
    duration_ms: int,
    crawled_count: int,
    predictions_count: int,
    trained: bool,
    steps: list[dict],
    error: str | None = None,
) -> None:
    """Insert one pipeline run report row.  Never raises — logs warning on failure."""
    try:
        with session_scope() as session:
            session.add(
                PipelineReport(
                    pipeline_key=pipeline_key,
                    market=market,
                    status=status,
                    started_at=started_at,
                    finished_at=finished_at,
                    duration_ms=duration_ms,
                    crawled_count=crawled_count,
                    predictions_count=predictions_count,
                    trained=trained,
                    steps=steps,
                    error=error,
                    created_at=datetime.now(),
                )
            )
    except Exception as exc:
        log.warning("pipeline_report.create.error", error=str(exc))


def delete_old_pipeline_reports(days: int = 7) -> int:
    """Delete pipeline_reports older than *days* days.  Returns deleted row count.  Never raises."""
    try:
        cutoff = datetime.now() - timedelta(days=days)
        with session_scope() as session:
            deleted = (
                session.query(PipelineReport)
                .filter(PipelineReport.created_at < cutoff)
                .delete(synchronize_session=False)
            )
        log.debug("pipeline_report.cleanup", deleted=deleted, cutoff=cutoff.isoformat())
        return deleted
    except Exception as exc:
        log.warning("pipeline_report.cleanup.error", error=str(exc))
        return 0


def increment_crawl_count(market_key: str) -> int:
    """Atomically increment and return the per-market crawl counter.

    Persisted in `pipeline_crawl_counters` so the "train every 10th crawl"
    logic survives prediction-svc restarts (replaces the old in-memory dict).
    Uses a single upsert + RETURNING so concurrent callers stay consistent.
    """
    with session_scope() as session:
        row = session.execute(
            text("""
                INSERT INTO pipeline_crawl_counters (market_key, crawl_count, updated_at)
                VALUES (:mk, 1, :now)
                ON CONFLICT (market_key) DO UPDATE
                    SET crawl_count = pipeline_crawl_counters.crawl_count + 1,
                        updated_at  = :now
                RETURNING crawl_count
            """),
            {"mk": market_key, "now": datetime.now()},
        ).fetchone()
        return int(row[0]) if row else 0


# ---------------------------------------------------------------------------
# Stock fundamentals (yfinance snapshots — transformer_nn static features)
# ---------------------------------------------------------------------------

FUNDAMENTAL_FIELDS = [
    "pe_ratio", "forward_pe", "price_to_book", "eps_ttm",
    "revenue_growth", "earnings_growth", "profit_margin",
    "debt_to_equity", "dividend_yield", "beta", "market_cap",
]


def upsert_stock_fundamental(market_key: str, symbol: str, fields: dict) -> None:
    """Upsert the latest fundamental snapshot for (market_key, symbol).

    `fields` may contain any subset of FUNDAMENTAL_FIELDS; unknown keys are
    ignored, missing keys are stored as NULL.
    """
    values = {k: fields.get(k) for k in FUNDAMENTAL_FIELDS}
    cols = ", ".join(FUNDAMENTAL_FIELDS)
    placeholders = ", ".join(f":{k}" for k in FUNDAMENTAL_FIELDS)
    updates = ", ".join(f"{k} = :{k}" for k in FUNDAMENTAL_FIELDS)
    with session_scope() as session:
        session.execute(
            text(f"""
                INSERT INTO stock_fundamentals (market_key, symbol, {cols}, fetched_at)
                VALUES (:mk, :sym, {placeholders}, :now)
                ON CONFLICT (market_key, symbol) DO UPDATE
                    SET {updates}, fetched_at = :now
            """),
            {"mk": market_key.upper(), "sym": symbol, "now": datetime.now(), **values},
        )


def get_fundamentals_map(market_key: str) -> dict[str, dict]:
    """Return {symbol: {field: float | None}} for a market. Empty dict on error.

    Values are converted to float (Numeric → float) so callers can feed them
    straight into feature vectors.
    """
    try:
        with session_scope() as session:
            rows = (
                session.query(StockFundamental)
                .filter(StockFundamental.market_key == market_key.upper())
                .all()
            )
            out: dict[str, dict] = {}
            for r in rows:
                out[r.symbol] = {
                    k: (float(getattr(r, k)) if getattr(r, k) is not None else None)
                    for k in FUNDAMENTAL_FIELDS
                }
            return out
    except Exception as exc:
        log.warning("fundamentals.map.error", market=market_key, error=str(exc))
        return {}


# ---------------------------------------------------------------------------
# Simulation KPI aggregation
# ---------------------------------------------------------------------------

def update_session_kpis(session_id: int) -> None:
    """Compute and store pre-aggregated KPI metrics in sim_sessions.

    Reads sim_trades (SELL rows) and sim_portfolio_snapshots for the given
    session_id, computes trade stats + max drawdown, then UPDATEs sim_sessions
    in a single statement. Safe to call multiple times (idempotent).

    Uses datetime.now() — container TZ is Asia/Ho_Chi_Minh (ICT).
    """
    with session_scope() as session:
        # ── 1. Pull initial_capital from sim_bots via sim_sessions ──────────
        capital_row = session.execute(
            text("""
                SELECT b.initial_capital
                FROM sim_sessions s
                JOIN sim_bots b ON b.id = s.bot_id
                WHERE s.id = :sid
            """),
            {"sid": session_id},
        ).fetchone()

        if capital_row is None:
            log.warning("sim.kpi.session_not_found", session_id=session_id)
            return

        initial_capital = float(capital_row[0]) if capital_row[0] else 0.0

        # ── 2. Trade stats from SELL rows ────────────────────────────────────
        trade_row = session.execute(
            text("""
                SELECT
                    COUNT(*)                                                AS total_trades,
                    COALESCE(SUM(CASE WHEN pnl > 0  THEN 1 ELSE 0 END), 0) AS wins,
                    COALESCE(SUM(CASE WHEN pnl < 0  THEN 1 ELSE 0 END), 0) AS losses,
                    COALESCE(SUM(CASE WHEN pnl = 0 OR pnl IS NULL THEN 1 ELSE 0 END), 0) AS breakeven,
                    COALESCE(SUM(pnl), 0)                                  AS total_pnl,
                    COALESCE(SUM(CASE WHEN pnl > 0 THEN pnl ELSE 0 END), 0) AS gross_win,
                    COALESCE(SUM(CASE WHEN pnl < 0 THEN ABS(pnl) ELSE 0 END), 0) AS gross_loss
                FROM sim_trades
                WHERE session_id = :sid
                  AND action = 'SELL'
            """),
            {"sid": session_id},
        ).fetchone()

        total_trades = int(trade_row[0]) if trade_row else 0
        wins         = int(trade_row[1]) if trade_row else 0
        losses       = int(trade_row[2]) if trade_row else 0
        breakeven    = int(trade_row[3]) if trade_row else 0
        total_pnl    = float(trade_row[4]) if trade_row else 0.0
        gross_win    = float(trade_row[5]) if trade_row else 0.0
        gross_loss   = float(trade_row[6]) if trade_row else 0.0

        win_rate: Optional[float] = (wins / total_trades) if total_trades > 0 else None
        profit_factor: Optional[float] = (gross_win / gross_loss) if gross_loss > 0 else None
        total_return_pct: Optional[float] = (
            (total_pnl / initial_capital * 100.0) if initial_capital > 0 else None
        )

        # ── 3. Max drawdown from portfolio snapshots (peak-to-trough on total_value) ─
        snap_rows = session.execute(
            text("""
                SELECT total_value
                FROM sim_portfolio_snapshots
                WHERE session_id = :sid
                ORDER BY snapshot_date ASC
            """),
            {"sid": session_id},
        ).fetchall()

        max_drawdown_pct: Optional[float] = None
        if snap_rows:
            peak = float(snap_rows[0][0])
            max_dd = 0.0
            for row in snap_rows:
                val = float(row[0])
                if val > peak:
                    peak = val
                if peak > 0:
                    dd = (peak - val) / peak * 100.0
                    if dd > max_dd:
                        max_dd = dd
            max_drawdown_pct = max_dd

        # ── 4. UPDATE sim_sessions with all computed KPIs ────────────────────
        session.execute(
            text("""
                UPDATE sim_sessions
                SET
                    total_trades     = :total_trades,
                    wins             = :wins,
                    losses           = :losses,
                    breakeven        = :breakeven,
                    total_pnl        = :total_pnl,
                    total_return_pct = :total_return_pct,
                    win_rate         = :win_rate,
                    profit_factor    = :profit_factor,
                    max_drawdown_pct = :max_drawdown_pct,
                    kpi_updated_at   = :now
                WHERE id = :sid
            """),
            {
                "total_trades":     total_trades,
                "wins":             wins,
                "losses":           losses,
                "breakeven":        breakeven,
                "total_pnl":        round(total_pnl, 2),
                "total_return_pct": round(total_return_pct, 4) if total_return_pct is not None else None,
                "win_rate":         round(win_rate, 4) if win_rate is not None else None,
                "profit_factor":    round(profit_factor, 4) if profit_factor is not None else None,
                "max_drawdown_pct": round(max_drawdown_pct, 4) if max_drawdown_pct is not None else None,
                "now":              datetime.now(),
                "sid":              session_id,
            },
        )

    log.debug(
        "sim.kpi.updated",
        session_id=session_id,
        total_trades=total_trades,
        wins=wins,
        losses=losses,
        total_pnl=round(total_pnl, 2),
        win_rate=round(win_rate, 4) if win_rate is not None else None,
        max_drawdown_pct=round(max_drawdown_pct, 4) if max_drawdown_pct is not None else None,
    )


# ---------------------------------------------------------------------------
# Stock splits
# ---------------------------------------------------------------------------

def record_split(
    market_key: str,
    symbol: str,
    split_date,          # datetime.date
    numerator: Decimal,
    denominator: Decimal,
) -> int | None:
    """Insert a new split row.  Returns the new row id on first insert, None if already recorded.

    ratio = numerator / denominator (e.g. 4:1 forward split → 4.0).
    Uses INSERT ... ON CONFLICT DO NOTHING so duplicate crawler runs are idempotent.
    """
    if denominator == 0:
        log.warning("split.record.zero_denominator", symbol=symbol, market=market_key)
        return None
    ratio = Decimal(str(numerator)) / Decimal(str(denominator))
    with session_scope() as session:
        row = session.execute(
            text("""
                INSERT INTO stock_splits
                    (market_key, symbol, split_date, ratio, numerator, denominator, created_at)
                VALUES
                    (:mk, :sym, :sd, :ratio, :num, :den, :now)
                ON CONFLICT (market_key, symbol, split_date) DO NOTHING
                RETURNING id
            """),
            {
                "mk": market_key.upper(),
                "sym": symbol,
                "sd": split_date,
                "ratio": ratio,
                "num": numerator,
                "den": denominator,
                "now": datetime.now(),
            },
        ).fetchone()
        return int(row[0]) if row else None


def list_unapplied_splits() -> list[dict]:
    """Return all split rows where applied_at IS NULL, ordered by split_date ASC."""
    session = get_session()
    try:
        rows = (
            session.query(StockSplit)
            .filter(StockSplit.applied_at.is_(None))
            .order_by(StockSplit.split_date.asc())
            .all()
        )
        return [
            {
                "id": r.id,
                "market_key": r.market_key,
                "symbol": r.symbol,
                "split_date": r.split_date,
                "ratio": float(r.ratio),
                "numerator": float(r.numerator),
                "denominator": float(r.denominator),
            }
            for r in rows
        ]
    finally:
        session.close()


def list_splits_for_symbol(market_key: str, symbol: str) -> list[dict]:
    """Return all recorded splits for (market_key, symbol) in ascending split_date order.

    Includes both applied and unapplied rows — callers filter by split_date relative
    to position entry_date.  Designed to be called once per symbol on restore; the
    small result set makes in-Python filtering fast.
    """
    session = get_session()
    try:
        rows = (
            session.query(StockSplit)
            .filter(
                StockSplit.market_key == market_key.upper(),
                StockSplit.symbol == symbol,
            )
            .order_by(StockSplit.split_date.asc())
            .all()
        )
        return [
            {
                "id": r.id,
                "split_date": r.split_date,
                "ratio": float(r.ratio),
            }
            for r in rows
        ]
    finally:
        session.close()


def mark_split_applied(split_id: int) -> None:
    """Set applied_at = NOW() for the given split row."""
    with session_scope() as session:
        session.query(StockSplit).filter(StockSplit.id == split_id).update(
            {"applied_at": datetime.now()}
        )
