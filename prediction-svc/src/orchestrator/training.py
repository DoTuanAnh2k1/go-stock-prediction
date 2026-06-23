"""Training state management, training pipeline, and reconcile logic."""
from __future__ import annotations

import os
import threading
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timedelta
from decimal import Decimal

from src.algorithms.base import PredictionAlgorithm
from src.algorithms.registry import (
    PER_SYMBOL_ALGO_MIN_POINTS,
    _DEFAULT_PS_MIN_POINTS,
    build_algorithms,
    build_algorithms_for_symbol,
    get_algos_for_market,
    set_algos_for_market,
    set_algos_for_symbol,
)
from src.config import get_settings
from src.crawlers.crypto import COINS as CRYPTO_COINS
from src.crawlers.nasdaq import NASDAQ_SYMBOLS
from src.crawlers.sp500 import SP500_SYMBOLS
from src.database import repository as repo
from src.utils.logger import get_logger

log = get_logger("training")

GOLD_INSTRUMENTS = [("XAU", "spot"), ("BTMC", "sjc"), ("BTMC", "nhan_tron")]


# ---------------------------------------------------------------------------
# Per-symbol helpers
# ---------------------------------------------------------------------------

def _resolve_workers(default_cap: int = 8) -> int:
    """Resolve the thread-pool worker count from settings.

    Uses ``settings.per_symbol_workers`` when > 0, otherwise ``os.cpu_count()``.
    Always clamped to [1, default_cap].
    """
    settings = get_settings()
    n = settings.per_symbol_workers or (os.cpu_count() or 4)
    return max(1, min(default_cap, n))


def _collect_symbol_series_for_market(
    mk: str,
) -> list[tuple[str, list[float], list[float] | None]]:
    """Return (symbol_label, prices_asc, vols_or_None) per symbol for *mk*.

    symbol_label conventions (must match runner + seeder):
      - GOLD:     f"{source}_{product_type}"   e.g. "XAU_spot", "BTMC_sjc", "BTMC_nhan_tron"
      - NASDAQ100: the ticker symbol            e.g. "AAPL", "MSFT"
      - SP500:     the ticker symbol            e.g. "SPY", "JPM"
      - CRYPTO:    the coin symbol              e.g. "BTC", "ETH", "SOL"

    Only symbols with >= settings.per_symbol_min_points data points are included;
    data-starved symbols are silently dropped here so we never even build instances
    for them.
    """
    settings = get_settings()
    min_pts = settings.per_symbol_min_points
    result: list[tuple[str, list[float], list[float] | None]] = []

    if mk == "GOLD":
        for source, product_type in GOLD_INSTRUMENTS:
            prices = repo.get_gold_prices_asc(source, product_type, limit=500)
            if len(prices) < min_pts:
                continue
            price_list = [float(p.buy_price) for p in prices]
            symbol_label = f"{source}_{product_type}"
            result.append((symbol_label, price_list, None))

    elif mk == "NASDAQ100":
        symbols = repo.get_nasdaq_symbols() or NASDAQ_SYMBOLS
        for symbol in symbols:
            prices = repo.get_nasdaq_prices_asc(symbol, limit=500)
            if len(prices) < min_pts:
                continue
            price_list = [float(p.close_price) for p in prices]
            vol_list = [float(p.volume or 0) for p in prices]
            result.append((symbol, price_list, vol_list))

    elif mk == "CRYPTO":
        for coin_id, symbol in CRYPTO_COINS:
            prices = repo.get_crypto_prices_asc(coin_id, limit=500)
            if len(prices) < min_pts:
                continue
            price_list = [float(p.close_price) for p in prices]
            vol_list = [float(p.volume24h or 0) for p in prices]
            result.append((symbol, price_list, vol_list))

    elif mk == "SP500":
        sp_symbols = repo.get_sp500_symbols() or SP500_SYMBOLS
        for symbol in sp_symbols:
            prices = repo.get_sp500_prices_asc(symbol, limit=500)
            if len(prices) < min_pts:
                continue
            price_list = [float(p.close_price) for p in prices]
            vol_list = [float(p.volume or 0) for p in prices]
            result.append((symbol, price_list, vol_list))

    else:
        log.warning("training.per_symbol.market.unknown", market=mk)

    return result


def _train_symbol_worker(
    mk: str,
    symbol: str,
    prices: list[float],
    vols: list[float] | None,
    global_min_pts: int,
) -> tuple[str, int, int]:
    """Worker function: train per-symbol instances for a single (market, symbol).

    Builds fresh instances, trains each algo only if data meets both the global
    min-points requirement AND the per-algo minimum.  Drops algos that are
    data-starved.  Persists surviving (trained or analytically-ready) instances
    via ``set_algos_for_symbol``.

    Returns (symbol, trained_count, skipped_count) for the caller to log.
    """
    # Stateless algos that predict analytically (no persistent model) — keep
    # them if data meets their per-algo min-point threshold, even though
    # is_trained() returns False.
    STATELESS_ALGO_KEYS = frozenset({
        "moving_average", "ema", "arima_garch", "sarima", "egarch",
    })

    instances = build_algorithms_for_symbol(mk, symbol)
    n_pts = len(prices)
    surviving: dict[str, PredictionAlgorithm] = {}
    trained_count = 0
    skipped_count = 0

    for key, algo in instances.items():
        algo_min = max(
            PER_SYMBOL_ALGO_MIN_POINTS.get(key, _DEFAULT_PS_MIN_POINTS),
            global_min_pts,
        )
        if n_pts < algo_min:
            skipped_count += 1
            continue  # data-starved: drop from per-symbol cache

        try:
            algo.train_batch([(prices, vols)])
        except Exception as exc:
            log.warning(
                "training.per_symbol.algo.failed",
                market=mk, symbol=symbol, algo=key, error=str(exc),
            )
            # For stateless algos keep them anyway (they don't rely on train_batch)
            if key not in STATELESS_ALGO_KEYS:
                skipped_count += 1
                continue

        # Keep if trained successfully OR is a stateless/analytical algo
        if algo.is_trained() or key in STATELESS_ALGO_KEYS:
            surviving[key] = algo
            trained_count += 1
        else:
            skipped_count += 1

    set_algos_for_symbol(mk, symbol, surviving)
    return symbol, trained_count, skipped_count


def train_per_symbol_for_market(mk: str) -> None:
    """Train per-symbol algorithm instances for all symbols of *mk*.

    Runs in a thread pool (capped by _resolve_workers).  Each worker builds
    fresh instances so there are no shared-state races between threads.
    DB access in each worker is session-per-call (repository pattern) so it
    is thread-safe with the pool size configured in connection.py.

    This function is called automatically from ``train_for_market`` when
    ``settings.per_symbol_enabled`` is True.  It can also be invoked directly
    for targeted refreshes.
    """
    settings = get_settings()
    global_min_pts = settings.per_symbol_min_points

    symbol_series = _collect_symbol_series_for_market(mk)
    if not symbol_series:
        log.info("training.per_symbol.no_symbols", market=mk)
        return

    log.info("training.per_symbol.start", market=mk, symbols=len(symbol_series))
    workers = _resolve_workers(8)

    with ThreadPoolExecutor(max_workers=workers) as pool:
        futures = {
            pool.submit(_train_symbol_worker, mk, sym, prices, vols, global_min_pts): sym
            for sym, prices, vols in symbol_series
        }
        for future in as_completed(futures):
            sym = futures[future]
            try:
                _sym, trained, skipped = future.result()
                log.info(
                    "training.per_symbol.symbol.done",
                    market=mk, symbol=_sym,
                    trained=trained, skipped=skipped,
                )
            except Exception as exc:
                log.warning(
                    "training.per_symbol.symbol.error",
                    market=mk, symbol=sym, error=str(exc),
                )

    log.info("training.per_symbol.done", market=mk, symbols=len(symbol_series))


def _collect_all_training_series() -> list[tuple[list[float], list[float], str]]:
    """Collect (price_list, vol_list, market_label) for all markets."""
    series = []

    # GOLD
    for source, product_type in GOLD_INSTRUMENTS:
        prices = repo.get_gold_prices_asc(source, product_type, limit=200)
        if len(prices) >= 20:
            series.append((
                [float(p.buy_price) for p in prices],
                [],
                f"gold/{source}/{product_type}",
            ))

    # NASDAQ
    symbols = repo.get_nasdaq_symbols() or NASDAQ_SYMBOLS
    for symbol in symbols:
        prices = repo.get_nasdaq_prices_asc(symbol, limit=200)
        if len(prices) >= 20:
            series.append((
                [float(p.close_price) for p in prices],
                [float(p.volume or 0) for p in prices],
                f"nasdaq/{symbol}",
            ))

    # CRYPTO
    for coin_id, symbol in CRYPTO_COINS:
        prices = repo.get_crypto_prices_asc(coin_id, limit=200)
        if len(prices) >= 20:
            series.append((
                [float(p.close_price) for p in prices],
                [],
                f"crypto/{symbol}",
            ))

    # SP500
    sp_symbols = repo.get_sp500_symbols() or SP500_SYMBOLS
    for symbol in sp_symbols:
        prices = repo.get_sp500_prices_asc(symbol, limit=200)
        if len(prices) >= 20:
            series.append((
                [float(p.close_price) for p in prices],
                [float(p.volume or 0) for p in prices],
                f"sp500/{symbol}",
            ))

    return series


def _collect_series_for_market(mk: str) -> list[tuple[list[float], list[float], str]]:
    """Collect (price_list, vol_list, label) for a single market.

    Args:
        mk: Market key in UPPER case — one of GOLD, NASDAQ100, CRYPTO, SP500.

    Returns:
        List of (price_list, vol_list, label) tuples where price_list is ASC order.
    """
    series: list[tuple[list[float], list[float], str]] = []

    if mk == "GOLD":
        for source, product_type in GOLD_INSTRUMENTS:
            prices = repo.get_gold_prices_asc(source, product_type, limit=200)
            if len(prices) >= 20:
                series.append((
                    [float(p.buy_price) for p in prices],
                    [],
                    f"gold/{source}/{product_type}",
                ))

    elif mk == "NASDAQ100":
        symbols = repo.get_nasdaq_symbols() or NASDAQ_SYMBOLS
        for symbol in symbols:
            prices = repo.get_nasdaq_prices_asc(symbol, limit=200)
            if len(prices) >= 20:
                series.append((
                    [float(p.close_price) for p in prices],
                    [float(p.volume or 0) for p in prices],
                    f"nasdaq/{symbol}",
                ))

    elif mk == "CRYPTO":
        for coin_id, symbol in CRYPTO_COINS:
            prices = repo.get_crypto_prices_asc(coin_id, limit=200)
            if len(prices) >= 20:
                series.append((
                    [float(p.close_price) for p in prices],
                    [],
                    f"crypto/{symbol}",
                ))

    elif mk == "SP500":
        sp_symbols = repo.get_sp500_symbols() or SP500_SYMBOLS
        for symbol in sp_symbols:
            prices = repo.get_sp500_prices_asc(symbol, limit=200)
            if len(prices) >= 20:
                series.append((
                    [float(p.close_price) for p in prices],
                    [float(p.volume or 0) for p in prices],
                    f"sp500/{symbol}",
                ))

    else:
        log.warning("training.market.unknown", market=mk)

    return series

_lock = threading.Lock()

# Training state
_is_training = False
_last_trained: datetime | None = None
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


def train_for_market(market_key: str) -> tuple[bool, str]:
    """Train all algorithms for a specific market. Returns (success, session_id).

    Builds fresh algorithm instances for the market, calls algo.train_batch()
    once per algorithm with ALL price series for that market combined. This ensures
    stateful algorithms (LSTM, GRU, LightGBM, RandomForest, XGBoost) build a single
    model from the full dataset rather than overwriting with each series.

    Trained instances are stored in the registry so subsequent predict() calls
    run inference-only.
    """
    mk = market_key.upper()
    session_id = str(uuid.uuid4())
    started_at = datetime.now()

    series = _collect_series_for_market(mk)
    if not series:
        log.warning("training.market.no_data", market=mk)
        return False, session_id

    # Build fresh instances exclusively for this market
    algos = build_algorithms(market_key=mk)

    # Convert to (prices, volumes) tuples — drop label
    series_data = [
        (price_list, vol_list if vol_list else None)
        for price_list, vol_list, _label in series
    ]

    total_success = 0
    total_error = 0

    log.info("training.market.start", market=mk, series=len(series), algorithms=len(algos))

    for key, algo in algos.items():
        algo_started = datetime.now()
        try:
            algo.train_batch(series_data)
            success = len(series_data)
            error = 0
        except Exception as exc:
            log.warning("training.market.algo.failed", market=mk, algo=key, error=str(exc))
            success = 0
            error = len(series_data)

        duration_ms = int((datetime.now() - algo_started).total_seconds() * 1000)
        accuracy = Decimal(str(round(success / max(1, success + error), 4)))

        try:
            repo.create_training_log(
                session_id=session_id,
                algorithm_name=key,
                market_key=mk.lower(),
                total_stocks=len(series),
                success_count=success,
                error_count=error,
                accuracy=accuracy,
                duration_ms=duration_ms,
                started_at=algo_started,
                completed_at=datetime.now(),
            )
        except Exception as exc:
            log.warning("training.log.failed", error=str(exc))

        total_success += success
        total_error += error
        log.info("training.market.algo.done", market=mk, algo=key, trained=algo.is_trained())

    # Persist trained instances in the registry
    set_algos_for_market(mk, algos)

    # Per-symbol training (additive — runs only when enabled)
    if get_settings().per_symbol_enabled:
        try:
            train_per_symbol_for_market(mk)
        except Exception as exc:
            log.warning("training.per_symbol.hook.failed", market=mk, error=str(exc))

    try:
        repo.create_sync_log(
            source=f"ML Training ({mk})",
            success_count=total_success,
            error_count=total_error,
            duration_ms=int((datetime.now() - started_at).total_seconds() * 1000),
        )
    except Exception as exc:
        log.warning("training.sync_log.failed", error=str(exc))

    log.info("training.market.done", market=mk, success=total_success, error=total_error)
    return True, session_id


def train_all_algorithms() -> tuple[bool, str]:
    """Train all algorithms for all markets. Returns (success, session_id).

    Iterates over every known market and calls train_for_market() for each.
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

    markets = ["GOLD", "NASDAQ100", "CRYPTO", "SP500"]
    session_id = str(uuid.uuid4())
    started_at = datetime.now()

    # Update total algorithm count for status reporting
    try:
        sample_algos = build_algorithms()
        with _lock:
            _total_algorithms = len(sample_algos) * len(markets)
    except Exception:
        with _lock:
            _total_algorithms = len(markets)

    total_success = 0
    total_error = 0

    try:
        for idx, mk in enumerate(markets):
            with _lock:
                _current_phase = f"Training {mk} ({idx+1}/{len(markets)})"
                _progress = idx / len(markets) * 100

            log.info("training.market.begin", market=mk)
            try:
                success, _sid = train_for_market(mk)
                if success:
                    total_success += 1
                else:
                    total_error += 1
            except Exception as exc:
                log.error("training.market.error", market=mk, error=str(exc))
                total_error += 1

            with _lock:
                _done_algorithms = (idx + 1)
                _progress = _done_algorithms / len(markets) * 100

        repo.create_sync_log(
            source="ML Training (ALL)",
            success_count=total_success,
            error_count=total_error,
            duration_ms=int((datetime.now() - started_at).total_seconds() * 1000),
        )

    except Exception as exc:
        log.error("training.failed", error=str(exc))
    finally:
        with _lock:
            _is_training = False
            _last_trained = datetime.now()
            _progress = 100.0
            _current_phase = "idle"

    return True, session_id


def train_single_algorithm(algorithm_name: str) -> tuple[bool, str]:
    """Train a single named algorithm — per-market (market-aware).

    Builds/uses the market-scoped cached instance for each market and trains
    only the requested algorithm on that market's series. This keeps the
    algorithm's ``_market_key`` correct, so market-aware/stateful models
    (e.g. rl_dqn) write properly scoped per-market checkpoints, and it does
    NOT clobber the trained state of the other cached algorithms.
    """
    global _is_training, _last_trained

    with _lock:
        if _is_training:
            return False, ""
        _is_training = True

    session_id = str(uuid.uuid4())
    found = False

    try:
        for mk in ("GOLD", "NASDAQ100", "CRYPTO", "SP500"):
            # Cached, market-scoped instances (_market_key already set to mk).
            algos = get_algos_for_market(mk)
            algo = algos.get(algorithm_name)
            if not algo:
                continue
            found = True

            series = _collect_series_for_market(mk)
            if not series:
                log.warning("training.single.no_data", algo=algorithm_name, market=mk)
                continue

            series_data = [
                (price_list, vol_list if vol_list else None)
                for price_list, vol_list, _label in series
            ]

            algo_started = datetime.now()
            try:
                # Trains the cached instance in place; market-aware models save
                # their per-market checkpoint here (e.g. rl_dqn_{mk}.pt).
                algo.train_batch(series_data)
                success = len(series_data)
                error = 0
            except Exception as exc:
                log.warning("training.single.algo.failed", algo=algorithm_name, market=mk, error=str(exc))
                success = 0
                error = len(series_data)

            # Re-store the cache (other algorithms untouched).
            set_algos_for_market(mk, algos)

            duration_ms = int((datetime.now() - algo_started).total_seconds() * 1000)
            accuracy = Decimal(str(round(success / max(1, success + error), 4)))
            try:
                repo.create_training_log(
                    session_id=session_id,
                    algorithm_name=algorithm_name,
                    market_key=mk.lower(),
                    total_stocks=len(series_data),
                    success_count=success,
                    error_count=error,
                    accuracy=accuracy,
                    duration_ms=duration_ms,
                    started_at=algo_started,
                    completed_at=datetime.now(),
                )
            except Exception as exc:
                log.warning("training.log.failed", error=str(exc))

        if not found:
            raise ValueError(f"Unknown algorithm: {algorithm_name}")

    finally:
        with _lock:
            _is_training = False
            _last_trained = datetime.now()

    return True, session_id


def _to_date(value):
    """Normalise a date/datetime to a plain ``date`` for safe comparison.

    DB columns return datetime in some markets (e.g. crypto_prices.trading_date)
    and date in others. ``datetime`` is a subclass of ``date``, so an
    ``isinstance(x, date)`` guard does NOT prevent a ``datetime >= date``
    comparison, which raises TypeError. Always normalise both sides first.
    """
    if isinstance(value, datetime):
        return value.date()
    return value


def reconcile_predictions(only_market: str | None = None) -> int:
    """Fill actual_price and compute accuracy for past pending predictions.

    only_market: when set (e.g. "CRYPTO", "NASDAQ100"), reconcile ONLY that
    market's predictions. Each market's hourly pipeline calls this right after it
    crawls fresh prices, so a +1h prediction is scored ~1h later on that market's
    own cadence. None = reconcile every market (daily catch-all safety net).

    Stock predictions: match close_price from stock_prices within ±3 days.
    Gold predictions: match sell_price from gold_prices within ±1 day.
    accuracy = max(0, 1 - |actual - predicted| / actual)
    status: accuracy >= 0.70 → 'confirmed', else 'wrong'
    """
    # Normalize the optional market filter (NASDAQ100 → NASDAQ to match branches).
    _norm = {"NASDAQ100": "NASDAQ", "NASDAQ": "NASDAQ", "GOLD": "GOLD",
             "SP500": "SP500", "CRYPTO": "CRYPTO"}
    wanted = _norm.get(only_market.upper()) if only_market else None

    def _want(m: str) -> bool:
        return wanted is None or wanted == m

    log.info("reconcile.start", market=wanted or "ALL")
    total_updated = 0

    # --- Backfill direction_correct for already-reconciled predictions ---
    # Records that have actual_price but direction_correct=NULL were reconciled before
    # this field was introduced; compute it from existing data without re-fetching prices.
    for market, backfill_fn in [
        ("gold", repo.backfill_direction_correct_gold),
        ("nasdaq", repo.backfill_direction_correct_nasdaq),
        ("sp500", repo.backfill_direction_correct_sp500),
        ("crypto", repo.backfill_direction_correct_crypto),
    ]:
        if not _want(market.upper()):
            continue
        try:
            n = backfill_fn()
            if n:
                log.info("reconcile.backfill.done", market=market, updated=n)
                total_updated += n
        except Exception as exc:
            log.error("reconcile.backfill.error", market=market, error=str(exc))

    # --- Gold predictions ---
    try:
        gold_pending = repo.get_pending_gold_predictions(days_back=5) if _want("GOLD") else []
        log.info("reconcile.gold.pending", count=len(gold_pending))

        for pred in gold_pending:
            gold_prices = repo.get_gold_prices_asc(pred.source, pred.product_type, limit=60)
            if not gold_prices:
                continue

            # Find closest
            target = pred.target_date
            closest = min(gold_prices, key=lambda p: abs((p.trading_date - target).total_seconds()))
            actual = Decimal(str(closest.sell_price or closest.buy_price))

            if actual == 0:
                continue

            predicted = Decimal(str(pred.predicted_price))
            current = Decimal(str(pred.current_price))
            accuracy = max(Decimal("0"), Decimal("1") - abs(actual - predicted) / actual)
            accuracy = accuracy.quantize(Decimal("0.0001"))
            status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

            pred_diff = predicted - current
            actual_diff = actual - current
            if actual_diff == 0:
                direction_correct: bool | None = (pred_diff == 0)
            elif pred_diff != 0:
                direction_correct = (pred_diff > 0) == (actual_diff > 0)
            else:
                direction_correct = False

            repo.update_gold_prediction_actual(pred.id, actual, accuracy, status, direction_correct)
            total_updated += 1

    except Exception as exc:
        log.error("reconcile.gold.error", error=str(exc))

    # --- NASDAQ predictions ---
    # GOLD-style: reconcile when target_date <= now (prediction has matured);
    # use latest live price as actual (no longer waiting for next trading day).
    try:
        nasdaq_pending = repo.get_pending_nasdaq_predictions(days_back=10) if _want("NASDAQ") else []
        log.info("reconcile.nasdaq.pending", count=len(nasdaq_pending))

        now_ts = datetime.now()
        for pred in nasdaq_pending:
            # Only reconcile predictions whose target time has already passed
            if pred.target_date > now_ts:
                continue

            prices = repo.get_nasdaq_prices_asc(pred.symbol, limit=60)
            if not prices:
                continue

            # actual = latest live price (most recent row in ASC list = last element)
            actual = Decimal(str(prices[-1].close_price))
            if actual == 0:
                continue

            predicted = Decimal(str(pred.predicted_price))
            current = Decimal(str(pred.current_price))
            accuracy = max(Decimal("0"), Decimal("1") - abs(actual - predicted) / actual)
            accuracy = accuracy.quantize(Decimal("0.0001"))
            status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

            pred_diff = predicted - current
            actual_diff = actual - current
            if actual_diff == 0:
                direction_correct: bool | None = (pred_diff == 0)
            elif pred_diff != 0:
                direction_correct = (pred_diff > 0) == (actual_diff > 0)
            else:
                direction_correct = False

            repo.update_nasdaq_prediction_actual(pred.id, actual, accuracy, status, direction_correct)
            total_updated += 1

    except Exception as exc:
        log.error("reconcile.nasdaq.error", error=str(exc))

    # --- SP500 predictions ---
    # GOLD-style: reconcile when target_date <= now; actual = latest live price.
    try:
        sp500_pending = repo.get_pending_sp500_predictions(days_back=10) if _want("SP500") else []
        log.info("reconcile.sp500.pending", count=len(sp500_pending))

        now_ts = datetime.now()
        for pred in sp500_pending:
            # Only reconcile predictions whose target time has already passed
            if pred.target_date > now_ts:
                continue

            prices = repo.get_sp500_prices_asc(pred.symbol, limit=60)
            if not prices:
                continue

            # actual = latest live price
            actual = Decimal(str(prices[-1].close_price))
            if actual == 0:
                continue

            predicted = Decimal(str(pred.predicted_price))
            current = Decimal(str(pred.current_price))
            accuracy = max(Decimal("0"), Decimal("1") - abs(actual - predicted) / actual)
            accuracy = accuracy.quantize(Decimal("0.0001"))
            status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

            pred_diff = predicted - current
            actual_diff = actual - current
            if actual_diff == 0:
                direction_correct: bool | None = (pred_diff == 0)
            elif pred_diff != 0:
                direction_correct = (pred_diff > 0) == (actual_diff > 0)
            else:
                direction_correct = False

            repo.update_sp500_prediction_actual(pred.id, actual, accuracy, status, direction_correct)
            total_updated += 1

    except Exception as exc:
        log.error("reconcile.sp500.error", error=str(exc))

    # --- Crypto predictions ---
    # GOLD-style: reconcile when target_date <= now; actual = latest live price.
    # Crypto 24/7 — no day-boundary filtering needed.
    try:
        crypto_pending = repo.get_pending_crypto_predictions(days_back=10) if _want("CRYPTO") else []
        log.info("reconcile.crypto.pending", count=len(crypto_pending))

        now_ts = datetime.now()
        for pred in crypto_pending:
            # Only reconcile predictions whose target time has already passed
            if pred.target_date > now_ts:
                continue

            prices = repo.get_crypto_prices_asc(pred.coin_id, limit=60)
            if not prices:
                continue

            # actual = latest live price
            actual = Decimal(str(prices[-1].close_price))
            if actual == 0:
                continue

            predicted = Decimal(str(pred.predicted_price))
            current = Decimal(str(pred.current_price))
            accuracy = max(Decimal("0"), Decimal("1") - abs(actual - predicted) / actual)
            accuracy = accuracy.quantize(Decimal("0.0001"))
            status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

            pred_diff = predicted - current
            actual_diff = actual - current
            if actual_diff == 0:
                direction_correct: bool | None = (pred_diff == 0)
            elif pred_diff != 0:
                direction_correct = (pred_diff > 0) == (actual_diff > 0)
            else:
                direction_correct = False

            repo.update_crypto_prediction_actual(pred.id, actual, accuracy, status, direction_correct)
            total_updated += 1

    except Exception as exc:
        log.error("reconcile.crypto.error", error=str(exc))

    log.info("reconcile.done", updated=total_updated)
    return total_updated


def run_historical_backtest(train_window: int, step_size: int, market_key: str = "") -> dict:
    """Walk-forward backtesting.

    Returns stats dict with total_predictions, stocks_processed, etc.
    """
    from src.algorithms.registry import build_algorithms

    mk = market_key.upper() if market_key else ""
    algos = build_algorithms()

    if train_window <= 0:
        train_window = 30
    if step_size <= 0:
        step_size = 6

    log.info("backtest.start", market=mk, train_window=train_window, step_size=step_size)
    started_at = datetime.now()
    total_preds = 0
    items_processed = 0

    if mk == "GOLD":
        total_preds, items_processed = _backtest_gold(algos, train_window, step_size)
    elif mk == "NASDAQ100":
        total_preds, items_processed = _backtest_nasdaq(algos, train_window, step_size)
    elif mk == "CRYPTO":
        total_preds, items_processed = _backtest_crypto(algos, train_window, step_size)
    elif mk == "SP500":
        total_preds, items_processed = _backtest_sp500(algos, train_window, step_size)
    elif mk == "ALL":
        for _mkey, _fn in [
            ("GOLD",      lambda: _backtest_gold(algos, train_window, step_size)),
            ("NASDAQ100", lambda: _backtest_nasdaq(algos, train_window, step_size)),
            ("CRYPTO",    lambda: _backtest_crypto(algos, train_window, step_size)),
            ("SP500",     lambda: _backtest_sp500(algos, train_window, step_size)),
        ]:
            try:
                preds, items = _fn()
                total_preds += preds
                items_processed += items
                log.info("backtest.market_done", market=_mkey, predictions=preds, items=items)
            except Exception as exc:
                log.error("backtest.market_failed", market=_mkey, error=str(exc))
    else:
        raise ValueError(f"Unknown market key for backtest: {market_key!r}. Valid values: GOLD, NASDAQ100, CRYPTO, SP500, ALL")

    duration_ms = int((datetime.now() - started_at).total_seconds() * 1000)
    log.info("backtest.done", market=mk, predictions=total_preds, items=items_processed, duration_ms=duration_ms)
    return {"total_predictions": total_preds, "stocks_processed": items_processed, "duration_ms": duration_ms}


def _walk_forward_prices(prices_asc: list, algos: dict, train_window: int, step_size: int,
                          save_fn) -> int:
    """Generic walk-forward over a price series."""
    BATCH_SIZE = 200
    MAX_HISTORY = 270
    total = 0
    n = len(prices_asc)

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


def _backtest_sp500(algos: dict, train_window: int, step_size: int) -> tuple[int, int]:
    from src.crawlers.sp500 import SP500_SYMBOLS
    symbols = repo.get_sp500_symbols() or SP500_SYMBOLS
    total = 0
    items = 0
    for symbol in symbols:
        prices_asc = repo.get_sp500_prices_asc(symbol, limit=500)
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
                            _bulk_sp500_predictions(batch)
                            total += len(batch)
                            batch = []
                    except Exception:
                        pass
            if batch:
                _bulk_sp500_predictions(batch)
                total += len(batch)
    return total, items


def _bulk_sp500_predictions(batch: list) -> None:
    from src.database.connection import session_scope
    from src.database.models import SP500Prediction
    with session_scope() as session:
        session.bulk_save_objects([SP500Prediction(**r) for r in batch])
