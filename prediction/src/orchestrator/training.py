"""Training state management, training pipeline, and reconcile logic."""
from __future__ import annotations

import threading
import uuid
from datetime import datetime, timedelta
from decimal import Decimal
from typing import Optional

from src.algorithms.registry import build_algorithms
from src.database import repository as repo
from src.utils.logger import get_logger

log = get_logger("training")

_lock = threading.Lock()

# Training state
_is_training = False
_last_trained: Optional[datetime] = None
_progress = 0.0
_current_phase = "idle"
_done_algorithms = 0

def _count_algorithms() -> int:
    try:
        from src.algorithms.registry import build_algorithms
        return len(build_algorithms())
    except Exception:
        return 6  # fallback: 6 registered algorithms

_total_algorithms = _count_algorithms()


def get_training_status() -> dict:
    with _lock:
        return {
            "is_training": _is_training,
            "last_trained": _last_trained.isoformat() + "Z" if _last_trained else "",
            "progress": _progress,
            "current_phase": _current_phase,
            "total_algorithms": _total_algorithms,
            "done_algorithms": _done_algorithms,
        }


def train_all_algorithms() -> tuple[bool, str]:
    """Train all algorithms. Returns (success, session_id).

    This runs synchronously (for cron jobs). For manual trigger, caller
    should run in a background thread.
    """
    global _is_training, _last_trained, _progress, _current_phase, _total_algorithms, _done_algorithms

    with _lock:
        if _is_training:
            return False, ""
        _is_training = True
        _progress = 0.0
        _current_phase = "Initializing"
        _done_algorithms = 0

    algos = build_algorithms()

    with _lock:
        _total_algorithms = len(algos)

    session_id = str(uuid.uuid4())
    started_at = datetime.utcnow()

    try:
        stocks = repo.get_vn30_stocks()
        total_success = 0
        total_error = 0

        for idx, (key, algo) in enumerate(algos.items()):
            with _lock:
                _current_phase = f"Training {key} ({idx+1}/{len(algos)})"
                _progress = idx / len(algos) * 100

            log.info("training.algo.start", algo=key, stocks=len(stocks))
            success = 0
            error = 0
            algo_started = datetime.utcnow()

            for stock in stocks:
                prices_asc = repo.get_stock_prices_asc(stock.id, limit=200)
                if len(prices_asc) < 20:
                    error += 1
                    continue
                try:
                    price_list = [float(p.close_price) for p in prices_asc]
                    vol_list = [float(p.volume or 0) for p in prices_asc]
                    algo.predict(price_list, vol_list)
                    success += 1
                except Exception:
                    error += 1

            duration_ms = int((datetime.utcnow() - algo_started).total_seconds() * 1000)
            accuracy = Decimal(str(round(success / max(1, success + error), 4)))

            try:
                repo.create_training_log(
                    session_id=session_id,
                    algorithm_name=key,
                    market_key="vn30",
                    total_stocks=len(stocks),
                    success_count=success,
                    error_count=error,
                    accuracy=accuracy,
                    duration_ms=duration_ms,
                    started_at=algo_started,
                    completed_at=datetime.utcnow(),
                )
            except Exception as exc:
                log.warning("training.log.failed", error=str(exc))

            total_success += success
            total_error += error

            with _lock:
                _done_algorithms = idx + 1
                _progress = _done_algorithms / len(algos) * 100

            log.info("training.algo.done", algo=key, success=success, error=error)

        repo.create_sync_log(
            source="ML Training",
            success_count=total_success,
            error_count=total_error,
            duration_ms=int((datetime.utcnow() - started_at).total_seconds() * 1000),
        )

    except Exception as exc:
        log.error("training.failed", error=str(exc))
    finally:
        with _lock:
            _is_training = False
            _last_trained = datetime.utcnow()
            _progress = 100.0
            _current_phase = "idle"

    return True, session_id


def train_single_algorithm(algorithm_name: str) -> tuple[bool, str]:
    """Train a single named algorithm."""
    global _is_training, _last_trained

    with _lock:
        if _is_training:
            return False, ""
        _is_training = True

    algos = build_algorithms()
    algo = algos.get(algorithm_name)

    if not algo:
        with _lock:
            _is_training = False
        raise ValueError(f"Unknown algorithm: {algorithm_name}")

    session_id = str(uuid.uuid4())
    started_at = datetime.utcnow()

    try:
        stocks = repo.get_vn30_stocks()
        success = 0
        error = 0

        for stock in stocks:
            prices_asc = repo.get_stock_prices_asc(stock.id, limit=200)
            if len(prices_asc) < 20:
                error += 1
                continue
            try:
                price_list = [float(p.close_price) for p in prices_asc]
                vol_list = [float(p.volume or 0) for p in prices_asc]
                algo.predict(price_list, vol_list)
                success += 1
            except Exception:
                error += 1

        duration_ms = int((datetime.utcnow() - started_at).total_seconds() * 1000)
        accuracy = Decimal(str(round(success / max(1, success + error), 4)))

        repo.create_training_log(
            session_id=session_id,
            algorithm_name=algorithm_name,
            market_key="vn30",
            total_stocks=len(stocks),
            success_count=success,
            error_count=error,
            accuracy=accuracy,
            duration_ms=duration_ms,
            started_at=started_at,
            completed_at=datetime.utcnow(),
        )

    finally:
        with _lock:
            _is_training = False
            _last_trained = datetime.utcnow()

    return True, session_id


def reconcile_predictions() -> int:
    """Fill actual_price and compute accuracy for past pending predictions.

    Stock predictions: match close_price from stock_prices within ±3 days.
    Gold predictions: match sell_price from gold_prices within ±1 day.
    accuracy = max(0, 1 - |actual - predicted| / actual)
    status: accuracy >= 0.70 → 'confirmed', else 'wrong'
    """
    log.info("reconcile.start")
    total_updated = 0

    # --- Stock predictions ---
    try:
        pending = repo.get_pending_predictions(days_back=10)
        log.info("reconcile.stock.pending", count=len(pending))

        for pred in pending:
            stock_prices = repo.get_stock_prices_range_asc(
                pred.stock_id,
                pred.target_date - timedelta(days=3),
                pred.target_date + timedelta(days=3),
            )
            if not stock_prices:
                continue

            # Find closest price to target_date
            closest = min(stock_prices, key=lambda p: abs((p.trading_date - pred.target_date).total_seconds()))
            actual = Decimal(str(closest.close_price))

            if actual == 0:
                continue

            predicted = Decimal(str(pred.predicted_price))
            accuracy = max(Decimal("0"), Decimal("1") - abs(actual - predicted) / actual)
            accuracy = accuracy.quantize(Decimal("0.0001"))
            status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

            repo.update_prediction_actual(pred.id, actual, accuracy, status)
            total_updated += 1

    except Exception as exc:
        log.error("reconcile.stock.error", error=str(exc))

    # --- Gold predictions ---
    try:
        gold_pending = repo.get_pending_gold_predictions(days_back=5)
        log.info("reconcile.gold.pending", count=len(gold_pending))

        for pred in gold_pending:
            gold_prices = repo.get_gold_prices_asc(pred.source, pred.product_type, limit=10)
            if not gold_prices:
                continue

            # Find closest
            target = pred.target_date
            closest = min(gold_prices, key=lambda p: abs((p.trading_date - target).total_seconds()))
            actual = Decimal(str(closest.sell_price or closest.buy_price))

            if actual == 0:
                continue

            predicted = Decimal(str(pred.predicted_price))
            accuracy = max(Decimal("0"), Decimal("1") - abs(actual - predicted) / actual)
            accuracy = accuracy.quantize(Decimal("0.0001"))
            status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

            repo.update_gold_prediction_actual(pred.id, actual, accuracy, status)
            total_updated += 1

    except Exception as exc:
        log.error("reconcile.gold.error", error=str(exc))

    log.info("reconcile.done", updated=total_updated)
    return total_updated


def run_historical_backtest(train_window: int, step_size: int, market_key: str = "") -> dict:
    """Walk-forward backtesting.

    Returns stats dict with total_predictions, stocks_processed, etc.
    """
    from src.algorithms.registry import build_algorithms

    mk = market_key.upper() if market_key else "VN30"
    algos = build_algorithms()

    if train_window <= 0:
        train_window = 30
    if step_size <= 0:
        step_size = 6

    log.info("backtest.start", market=mk, train_window=train_window, step_size=step_size)
    started_at = datetime.utcnow()
    total_preds = 0
    items_processed = 0

    if mk in ("", "VN30"):
        total_preds, items_processed = _backtest_vn30(algos, train_window, step_size)
    elif mk == "GOLD":
        total_preds, items_processed = _backtest_gold(algos, train_window, step_size)
    elif mk == "NASDAQ100":
        total_preds, items_processed = _backtest_nasdaq(algos, train_window, step_size)
    elif mk == "CRYPTO":
        total_preds, items_processed = _backtest_crypto(algos, train_window, step_size)
    elif mk == "FUEL":
        total_preds, items_processed = _backtest_fuel(algos, train_window, step_size)

    duration_ms = int((datetime.utcnow() - started_at).total_seconds() * 1000)
    log.info("backtest.done", market=mk, predictions=total_preds, items=items_processed, duration_ms=duration_ms)
    return {"total_predictions": total_preds, "stocks_processed": items_processed, "duration_ms": duration_ms}


def _walk_forward_prices(prices_asc: list, algos: dict, train_window: int, step_size: int,
                          save_fn) -> int:
    """Generic walk-forward over a price series."""
    BATCH_SIZE = 200
    MAX_HISTORY = 270
    total = 0
    n = len(prices_asc)
    today = datetime.utcnow().replace(hour=0, minute=0, second=0, microsecond=0)

    for key, algo in algos.items():
        batch = []
        for window_end in range(train_window, n, step_size):
            hist_len = min(window_end, MAX_HISTORY)
            hist_start = window_end - hist_len
            price_list = [float(p) for p in prices_asc[hist_start:window_end]]

            fold_end = min(window_end + step_size, n)
            for t in range(window_end, fold_end):
                try:
                    result = algo.predict(price_list)
                    actual_price = Decimal(str(prices_asc[t]))
                    predicted_price = Decimal(str(result.predicted_price)).quantize(Decimal("0.01"))
                    accuracy = max(Decimal("0"), Decimal("1") - abs(actual_price - predicted_price) / actual_price)
                    accuracy = accuracy.quantize(Decimal("0.0001"))
                    status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

                    record = save_fn(
                        algo_name=key,
                        predicted_price=predicted_price,
                        current_price=Decimal(str(prices_asc[window_end - 1])).quantize(Decimal("0.01")),
                        confidence=Decimal(str(round(result.confidence, 4))),
                        actual_price=actual_price,
                        accuracy=accuracy,
                        status=status,
                        idx=t,
                        window_end=window_end,
                    )
                    if record:
                        batch.append(record)
                        if len(batch) >= BATCH_SIZE:
                            repo.bulk_create_predictions(batch)
                            total += len(batch)
                            batch = []
                except Exception:
                    pass

        if batch:
            repo.bulk_create_predictions(batch)
            total += len(batch)

    return total


def _backtest_vn30(algos: dict, train_window: int, step_size: int) -> tuple[int, int]:
    today = datetime.utcnow().replace(hour=0, minute=0, second=0, microsecond=0)
    repo.delete_predictions_before(today)

    stocks = repo.get_vn30_stocks()
    total = 0
    items = 0

    for stock in stocks:
        prices_asc = repo.get_stock_prices_asc(stock.id, limit=500)
        if len(prices_asc) < train_window + 1:
            continue

        price_floats = [float(p.close_price) for p in prices_asc]
        dates = [p.trading_date for p in prices_asc]

        items += 1
        for key, algo in algos.items():
            batch: list[dict] = []
            n = len(price_floats)
            for window_end in range(train_window, n, step_size):
                hist_end = min(window_end, 270)
                hist_start = window_end - hist_end
                price_list = price_floats[hist_start:window_end]

                fold_end = min(window_end + step_size, n)
                for t in range(window_end, fold_end):
                    try:
                        result = algo.predict(price_list)
                        actual = Decimal(str(price_floats[t])).quantize(Decimal("0.01"))
                        predicted = Decimal(str(result.predicted_price)).quantize(Decimal("0.01"))
                        accuracy = max(Decimal("0"), Decimal("1") - abs(actual - predicted) / actual)
                        accuracy = accuracy.quantize(Decimal("0.0001"))
                        status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

                        batch.append(dict(
                            stock_id=stock.id,
                            predicted_price=predicted,
                            current_price=Decimal(str(price_floats[window_end - 1])).quantize(Decimal("0.01")),
                            confidence=Decimal(str(round(result.confidence, 4))),
                            algorithm_name=key,
                            prediction_date=dates[window_end - 1],
                            target_date=dates[t],
                            actual_price=actual,
                            accuracy=accuracy,
                            status=status,
                        ))

                        if len(batch) >= 200:
                            repo.bulk_create_predictions(batch)
                            total += len(batch)
                            batch = []
                    except Exception:
                        pass

            if batch:
                repo.bulk_create_predictions(batch)
                total += len(batch)

    return total, items


def _backtest_gold(algos: dict, train_window: int, step_size: int) -> tuple[int, int]:
    total = 0
    items = 0
    for source, product_type in [("XAU", "spot"), ("BTMC", "sjc"), ("BTMC", "nhan_tron")]:
        prices_asc = repo.get_gold_prices_asc(source, product_type, limit=500)
        if len(prices_asc) < train_window + 1:
            continue
        items += 1
        price_floats = [float(p.buy_price) for p in prices_asc]
        dates = [p.trading_date for p in prices_asc]
        n = len(price_floats)

        for key, algo in algos.items():
            batch: list = []
            for window_end in range(train_window, n, step_size):
                hist_end = min(window_end, 270)
                hist_start = window_end - hist_end
                price_list = price_floats[hist_start:window_end]
                fold_end = min(window_end + step_size, n)
                for t in range(window_end, fold_end):
                    try:
                        result = algo.predict(price_list)
                        actual = Decimal(str(price_floats[t])).quantize(Decimal("0.01"))
                        predicted = Decimal(str(result.predicted_price)).quantize(Decimal("0.01"))
                        accuracy = max(Decimal("0"), Decimal("1") - abs(actual - predicted) / actual)
                        accuracy = accuracy.quantize(Decimal("0.0001"))
                        status = "confirmed" if float(accuracy) >= 0.70 else "wrong"
                        from src.database.models import GoldPrediction
                        batch.append(dict(
                            source=source, product_type=product_type,
                            predicted_price=predicted, current_price=Decimal(str(price_floats[window_end - 1])).quantize(Decimal("0.01")),
                            confidence=Decimal(str(round(result.confidence, 4))), algorithm_name=key,
                            prediction_date=dates[window_end - 1], target_date=dates[t],
                            actual_price=actual, accuracy=accuracy, status=status,
                        ))
                        if len(batch) >= 200:
                            _bulk_gold_predictions(batch)
                            total += len(batch)
                            batch = []
                    except Exception:
                        pass
            if batch:
                _bulk_gold_predictions(batch)
                total += len(batch)
    return total, items


def _bulk_gold_predictions(batch: list) -> None:
    from src.database.connection import session_scope
    from src.database.models import GoldPrediction
    with session_scope() as session:
        session.bulk_save_objects([GoldPrediction(**r) for r in batch])


def _backtest_nasdaq(algos: dict, train_window: int, step_size: int) -> tuple[int, int]:
    from src.crawlers.nasdaq import NASDAQ_SYMBOLS
    symbols = repo.get_nasdaq_symbols() or NASDAQ_SYMBOLS
    total = 0
    items = 0
    for symbol in symbols:
        prices_asc = repo.get_nasdaq_prices_asc(symbol, limit=500)
        if len(prices_asc) < train_window + 1:
            continue
        items += 1
        price_floats = [float(p.close_price) for p in prices_asc]
        dates = [p.trading_date for p in prices_asc]
        n = len(price_floats)
        for key, algo in algos.items():
            batch = []
            for window_end in range(train_window, n, step_size):
                hist_end = min(window_end, 270)
                price_list = price_floats[window_end - hist_end:window_end]
                for t in range(window_end, min(window_end + step_size, n)):
                    try:
                        result = algo.predict(price_list)
                        actual = Decimal(str(price_floats[t])).quantize(Decimal("0.0001"))
                        predicted = Decimal(str(result.predicted_price)).quantize(Decimal("0.0001"))
                        accuracy = max(Decimal("0"), Decimal("1") - abs(actual - predicted) / actual)
                        accuracy = accuracy.quantize(Decimal("0.0001"))
                        status = "confirmed" if float(accuracy) >= 0.70 else "wrong"
                        batch.append(dict(
                            symbol=symbol, predicted_price=predicted,
                            current_price=Decimal(str(price_floats[window_end - 1])).quantize(Decimal("0.0001")),
                            confidence=Decimal(str(round(result.confidence, 4))), algorithm_name=key,
                            prediction_date=dates[window_end - 1], target_date=dates[t],
                            actual_price=actual, accuracy=accuracy, status=status,
                        ))
                        if len(batch) >= 200:
                            _bulk_nasdaq_predictions(batch)
                            total += len(batch)
                            batch = []
                    except Exception:
                        pass
            if batch:
                _bulk_nasdaq_predictions(batch)
                total += len(batch)
    return total, items


def _bulk_nasdaq_predictions(batch: list) -> None:
    from src.database.connection import session_scope
    from src.database.models import NasdaqPrediction
    with session_scope() as session:
        session.bulk_save_objects([NasdaqPrediction(**r) for r in batch])


def _backtest_crypto(algos: dict, train_window: int, step_size: int) -> tuple[int, int]:
    from src.crawlers.crypto import COINS
    total = 0
    items = 0
    for coin_id, symbol in COINS:
        prices_asc = repo.get_crypto_prices_asc(coin_id, limit=500)
        if len(prices_asc) < train_window + 1:
            continue
        items += 1
        price_floats = [float(p.close_price) for p in prices_asc]
        dates = [p.trading_date for p in prices_asc]
        n = len(price_floats)
        for key, algo in algos.items():
            batch = []
            for window_end in range(train_window, n, step_size):
                hist_end = min(window_end, 270)
                price_list = price_floats[window_end - hist_end:window_end]
                for t in range(window_end, min(window_end + step_size, n)):
                    try:
                        result = algo.predict(price_list)
                        actual = Decimal(str(price_floats[t])).quantize(Decimal("0.01"))
                        predicted = Decimal(str(result.predicted_price)).quantize(Decimal("0.01"))
                        accuracy = max(Decimal("0"), Decimal("1") - abs(actual - predicted) / actual)
                        accuracy = accuracy.quantize(Decimal("0.0001"))
                        status = "confirmed" if float(accuracy) >= 0.70 else "wrong"
                        batch.append(dict(
                            coin_id=coin_id, symbol=symbol, predicted_price=predicted,
                            current_price=Decimal(str(price_floats[window_end - 1])).quantize(Decimal("0.01")),
                            confidence=Decimal(str(round(result.confidence, 4))), algorithm_name=key,
                            prediction_date=dates[window_end - 1], target_date=dates[t],
                            actual_price=actual, accuracy=accuracy, status=status,
                        ))
                        if len(batch) >= 200:
                            _bulk_crypto_predictions(batch)
                            total += len(batch)
                            batch = []
                    except Exception:
                        pass
            if batch:
                _bulk_crypto_predictions(batch)
                total += len(batch)
    return total, items


def _bulk_crypto_predictions(batch: list) -> None:
    from src.database.connection import session_scope
    from src.database.models import CryptoPrediction
    with session_scope() as session:
        session.bulk_save_objects([CryptoPrediction(**r) for r in batch])


def _backtest_fuel(algos: dict, train_window: int, step_size: int) -> tuple[int, int]:
    total = 0
    items = 0
    for product_type in FUEL_PRODUCTS:
        prices_asc = repo.get_fuel_prices_asc(product_type, limit=500)
        if len(prices_asc) < train_window + 1:
            continue
        items += 1
        price_floats = [float(p.price) for p in prices_asc]
        dates = [p.trading_date for p in prices_asc]
        n = len(price_floats)
        for key, algo in algos.items():
            batch = []
            for window_end in range(train_window, n, step_size):
                hist_end = min(window_end, 270)
                price_list = price_floats[window_end - hist_end:window_end]
                for t in range(window_end, min(window_end + step_size, n)):
                    try:
                        result = algo.predict(price_list)
                        actual = Decimal(str(price_floats[t])).quantize(Decimal("0.001"))
                        predicted = Decimal(str(result.predicted_price)).quantize(Decimal("0.001"))
                        accuracy = max(Decimal("0"), Decimal("1") - abs(actual - predicted) / actual)
                        accuracy = accuracy.quantize(Decimal("0.0001"))
                        status = "confirmed" if float(accuracy) >= 0.70 else "wrong"
                        batch.append(dict(
                            product_type=product_type, predicted_price=predicted,
                            current_price=Decimal(str(price_floats[window_end - 1])).quantize(Decimal("0.001")),
                            confidence=Decimal(str(round(result.confidence, 4))), algorithm_name=key,
                            prediction_date=dates[window_end - 1], target_date=dates[t],
                            actual_price=actual, accuracy=accuracy, status=status,
                        ))
                        if len(batch) >= 200:
                            _bulk_fuel_predictions(batch)
                            total += len(batch)
                            batch = []
                    except Exception:
                        pass
            if batch:
                _bulk_fuel_predictions(batch)
                total += len(batch)
    return total, items


def _bulk_fuel_predictions(batch: list) -> None:
    from src.database.connection import session_scope
    from src.database.models import FuelPrediction
    with session_scope() as session:
        session.bulk_save_objects([FuelPrediction(**r) for r in batch])


FUEL_PRODUCTS = ["ron95_iii", "e5_ron92", "do_005s", "kerosene"]
