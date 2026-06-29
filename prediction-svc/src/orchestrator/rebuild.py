"""Rebuild & Replay pipeline.

Wipes all prediction rows and simulation session data (keeping sim_bots),
then replays a walk-forward backtest so every algorithm produces perfectly
out-of-sample, aligned predictions.  After the predictions are written,
meta models are retrained (on the pre-cutoff slice only) and bots replay
trading on the fresh predictions via SimulationEngine.

Public API
----------
rebuild_and_replay(cutoff_date, predict_from_date=None, step_size=3) -> dict
    Orchestrates all five phases and returns a stats dict.

    Two time boundaries separate the pipeline into distinct roles:

      predict_from_date  – first date included in saved predictions.
                           Default: cutoff_date - 14 days.
      cutoff_date        – last date in the META TRAIN window.
                           Predictions with target_date <= cutoff_date feed
                           the meta classifier.  Bot trading starts only from
                           cutoff_date + 1 forward.

    Timeline example (predict_from=2026-06-01, cutoff=2026-06-13):
      [2026-06-01 .. 2026-06-13]  → base predictions saved + used to TRAIN meta
      [2026-06-14 .. today]       → base predictions saved + BOT TRADING window

_compute_direction_correct(predicted, current, actual) -> bool | None
    Pure helper — exported so the unit-test can validate it in isolation.
"""
from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import date, datetime, timedelta
from decimal import Decimal

from src.algorithms.registry import build_algorithms
from src.algorithms.runtime_flags import is_optuna_enabled, set_optuna_enabled
from src.crawlers.crypto import COINS as CRYPTO_COINS
from src.crawlers.nasdaq import NASDAQ_SYMBOLS
from src.crawlers.sp500 import SP500_SYMBOLS
from src.database import repository as repo
from src.orchestrator.training import (
    GOLD_INSTRUMENTS,
    _bulk_crypto_predictions,
    _bulk_gold_predictions,
    _bulk_nasdaq_predictions,
    _bulk_sp500_predictions,
    _resolve_workers,
    _to_date,
    train_meta_all,
)
from src.utils.logger import get_logger

log = get_logger("rebuild")

_TRAIN_WINDOW = 30
_MAX_HISTORY = 270
_BATCH_SIZE = 200


# ---------------------------------------------------------------------------
# Pure helpers
# ---------------------------------------------------------------------------

def _compute_direction_correct(
    predicted: Decimal,
    current: Decimal,
    actual: Decimal,
) -> bool | None:
    """Return whether the predicted direction matches the actual movement.

    Returns
    -------
    True   predicted direction == actual direction
    False  predicted direction != actual direction
    None   price did not move (actual == current) and predicted == current
           (flat-on-flat treated as correct); False otherwise when flat
    """
    pred_diff = predicted - current
    actual_diff = actual - current
    if actual_diff == 0:
        return bool(pred_diff == 0)
    if pred_diff != 0:
        return bool((pred_diff > 0) == (actual_diff > 0))
    return False


# ---------------------------------------------------------------------------
# Wipe helpers
# ---------------------------------------------------------------------------

def _wipe_predictions_and_sim() -> None:
    """DELETE all prediction + simulation rows, keeping sim_bots intact.

    Uses separate session_scope() per table to avoid TimescaleDB hypertable
    transaction conflicts (CREATE UNIQUE INDEX must live outside DML transactions).
    """
    from src.database.connection import session_scope
    from sqlalchemy import text

    for table in (
        "gold_predictions",
        "nasdaq_predictions",
        "sp500_predictions",
        "crypto_predictions",
    ):
        with session_scope() as session:
            session.execute(text(f"DELETE FROM {table}"))  # noqa: S608
            session.commit()

    for table in ("sim_trades", "sim_portfolio_snapshots", "sim_sessions"):
        with session_scope() as session:
            session.execute(text(f"DELETE FROM {table}"))  # noqa: S608
            session.commit()


# ---------------------------------------------------------------------------
# Per-market walk-forward with cutoff filter — symbol-level parallelism
# ---------------------------------------------------------------------------

def _rebuild_backtest_gold(
    algos: dict,
    step_size: int,
    cutoff: date,
    predict_from: date,
) -> int:
    """Walk-forward GOLD predictions; save only rows where target_date >= predict_from.

    Each (source, product_type) instrument is processed in a separate thread.
    Thread safety: _bulk_gold_predictions opens its own session_scope() per
    call; repo.get_gold_prices_asc() creates its own session via get_session().
    The algos dict is shared read-only (predict-only, no weight updates).

    predict_from controls which predictions are persisted:
      - target_date >= predict_from  → saved (includes both meta-train and bot-trade windows)
      - target_date <  predict_from  → discarded (warm-up period before the window)
    cutoff is forwarded only for logging; the actual meta/bot split is done
    in rebuild_and_replay by passing max_train_date=cutoff to train_meta_all().
    """
    instruments = list(GOLD_INSTRUMENTS)
    workers = _resolve_workers()
    log.info("rebuild.predict.workers", market="GOLD", n=workers, instruments=len(instruments))

    def _instrument_task(source_product: tuple) -> int:
        source, product_type = source_product
        task_total = 0
        prices_asc = repo.get_gold_prices_asc(source, product_type, limit=500)
        if len(prices_asc) < _TRAIN_WINDOW + 1:
            return 0

        price_floats = [float(p.buy_price) for p in prices_asc]
        dates = [p.trading_date for p in prices_asc]
        n = len(price_floats)

        for key, algo in algos.items():
            batch: list[dict] = []
            for window_end in range(_TRAIN_WINDOW, n, step_size):
                hist_end = min(window_end, _MAX_HISTORY)
                price_list = price_floats[window_end - hist_end:window_end]
                fold_end = min(window_end + step_size, n)

                for t in range(window_end, fold_end):
                    # Save predictions whose TARGET date is on or after predict_from
                    # (covers both meta-train window and bot-trade window)
                    if _to_date(dates[t]) < predict_from:
                        continue
                    try:
                        result = algo.predict(price_list)
                        actual = Decimal(str(price_floats[t])).quantize(Decimal("0.01"))
                        predicted = Decimal(str(result.predicted_price)).quantize(Decimal("0.01"))
                        current = Decimal(str(price_floats[window_end - 1])).quantize(Decimal("0.01"))
                        accuracy = max(
                            Decimal("0"),
                            Decimal("1") - abs(actual - predicted) / actual,
                        ).quantize(Decimal("0.0001"))
                        status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

                        batch.append(dict(
                            source=source,
                            product_type=product_type,
                            predicted_price=predicted,
                            current_price=current,
                            confidence=Decimal(str(round(result.confidence, 4))),
                            algorithm_name=key,
                            prediction_date=dates[window_end - 1],
                            target_date=dates[t],
                            actual_price=actual,
                            accuracy=accuracy,
                            status=status,
                            direction_correct=_compute_direction_correct(predicted, current, actual),
                        ))
                        if len(batch) >= _BATCH_SIZE:
                            _bulk_gold_predictions(batch)
                            task_total += len(batch)
                            batch = []
                    except Exception:
                        log.error(
                            "rebuild.gold.predict_failed",
                            algo=key,
                            source=source,
                            product_type=product_type,
                            idx=t,
                            exc_info=True,
                        )
            if batch:
                _bulk_gold_predictions(batch)
                task_total += len(batch)
                batch = []

        return task_total

    total = 0
    with ThreadPoolExecutor(max_workers=workers) as pool:
        futures = {pool.submit(_instrument_task, inst): inst for inst in instruments}
        for fut in as_completed(futures):
            inst = futures[fut]
            try:
                total += fut.result()
            except Exception:
                log.error("rebuild.gold.instrument_failed", instrument=inst, exc_info=True)

    return total


def _rebuild_backtest_nasdaq(
    algos: dict,
    step_size: int,
    cutoff: date,
    predict_from: date,
) -> int:
    """Walk-forward NASDAQ predictions; save only rows where target_date >= predict_from.

    Each symbol is processed in a separate thread.
    Thread safety: _bulk_nasdaq_predictions opens its own session_scope(); repo
    functions create their own sessions. algos dict is shared read-only.
    """
    symbols = repo.get_nasdaq_symbols() or NASDAQ_SYMBOLS
    workers = _resolve_workers()
    log.info("rebuild.predict.workers", market="NASDAQ100", n=workers, symbols=len(symbols))

    def _symbol_task(symbol: str) -> int:
        task_total = 0
        prices_asc = repo.get_nasdaq_prices_asc(symbol, limit=500)
        if len(prices_asc) < _TRAIN_WINDOW + 1:
            return 0

        price_floats = [float(p.close_price) for p in prices_asc]
        dates = [p.trading_date for p in prices_asc]
        n = len(price_floats)

        for key, algo in algos.items():
            batch: list[dict] = []
            for window_end in range(_TRAIN_WINDOW, n, step_size):
                hist_end = min(window_end, _MAX_HISTORY)
                price_list = price_floats[window_end - hist_end:window_end]
                fold_end = min(window_end + step_size, n)

                for t in range(window_end, fold_end):
                    if _to_date(dates[t]) < predict_from:
                        continue
                    try:
                        result = algo.predict(price_list)
                        actual = Decimal(str(price_floats[t])).quantize(Decimal("0.0001"))
                        predicted = Decimal(str(result.predicted_price)).quantize(Decimal("0.0001"))
                        current = Decimal(str(price_floats[window_end - 1])).quantize(Decimal("0.0001"))
                        accuracy = max(
                            Decimal("0"),
                            Decimal("1") - abs(actual - predicted) / actual,
                        ).quantize(Decimal("0.0001"))
                        status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

                        batch.append(dict(
                            symbol=symbol,
                            predicted_price=predicted,
                            current_price=current,
                            confidence=Decimal(str(round(result.confidence, 4))),
                            algorithm_name=key,
                            prediction_date=dates[window_end - 1],
                            target_date=dates[t],
                            actual_price=actual,
                            accuracy=accuracy,
                            status=status,
                            direction_correct=_compute_direction_correct(predicted, current, actual),
                        ))
                        if len(batch) >= _BATCH_SIZE:
                            _bulk_nasdaq_predictions(batch)
                            task_total += len(batch)
                            batch = []
                    except Exception:
                        log.error(
                            "rebuild.nasdaq.predict_failed",
                            algo=key,
                            symbol=symbol,
                            idx=t,
                            exc_info=True,
                        )
            if batch:
                _bulk_nasdaq_predictions(batch)
                task_total += len(batch)
                batch = []

        return task_total

    total = 0
    with ThreadPoolExecutor(max_workers=workers) as pool:
        futures = {pool.submit(_symbol_task, sym): sym for sym in symbols}
        for fut in as_completed(futures):
            sym = futures[fut]
            try:
                total += fut.result()
            except Exception:
                log.error("rebuild.nasdaq.symbol_failed", symbol=sym, exc_info=True)

    return total


def _rebuild_backtest_crypto(
    algos: dict,
    step_size: int,
    cutoff: date,
    predict_from: date,
) -> int:
    """Walk-forward CRYPTO predictions; save only rows where target_date >= predict_from.

    Each coin is processed in a separate thread.
    Thread safety: _bulk_crypto_predictions opens its own session_scope(); repo
    functions create their own sessions. algos dict is shared read-only.
    """
    coins = list(CRYPTO_COINS)
    workers = _resolve_workers()
    log.info("rebuild.predict.workers", market="CRYPTO", n=workers, coins=len(coins))

    def _coin_task(coin: tuple) -> int:
        coin_id, symbol = coin
        task_total = 0
        prices_asc = repo.get_crypto_prices_asc(coin_id, limit=500)
        if len(prices_asc) < _TRAIN_WINDOW + 1:
            return 0

        price_floats = [float(p.close_price) for p in prices_asc]
        dates = [p.trading_date for p in prices_asc]
        n = len(price_floats)

        for key, algo in algos.items():
            batch: list[dict] = []
            for window_end in range(_TRAIN_WINDOW, n, step_size):
                hist_end = min(window_end, _MAX_HISTORY)
                price_list = price_floats[window_end - hist_end:window_end]
                fold_end = min(window_end + step_size, n)

                for t in range(window_end, fold_end):
                    if _to_date(dates[t]) < predict_from:
                        continue
                    try:
                        result = algo.predict(price_list)
                        actual = Decimal(str(price_floats[t])).quantize(Decimal("0.01"))
                        predicted = Decimal(str(result.predicted_price)).quantize(Decimal("0.01"))
                        current = Decimal(str(price_floats[window_end - 1])).quantize(Decimal("0.01"))
                        accuracy = max(
                            Decimal("0"),
                            Decimal("1") - abs(actual - predicted) / actual,
                        ).quantize(Decimal("0.0001"))
                        status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

                        batch.append(dict(
                            coin_id=coin_id,
                            symbol=symbol,
                            predicted_price=predicted,
                            current_price=current,
                            confidence=Decimal(str(round(result.confidence, 4))),
                            algorithm_name=key,
                            prediction_date=dates[window_end - 1],
                            target_date=dates[t],
                            actual_price=actual,
                            accuracy=accuracy,
                            status=status,
                            direction_correct=_compute_direction_correct(predicted, current, actual),
                        ))
                        if len(batch) >= _BATCH_SIZE:
                            _bulk_crypto_predictions(batch)
                            task_total += len(batch)
                            batch = []
                    except Exception:
                        log.error(
                            "rebuild.crypto.predict_failed",
                            algo=key,
                            coin_id=coin_id,
                            idx=t,
                            exc_info=True,
                        )
            if batch:
                _bulk_crypto_predictions(batch)
                task_total += len(batch)
                batch = []

        return task_total

    total = 0
    with ThreadPoolExecutor(max_workers=workers) as pool:
        futures = {pool.submit(_coin_task, coin): coin for coin in coins}
        for fut in as_completed(futures):
            coin = futures[fut]
            try:
                total += fut.result()
            except Exception:
                log.error("rebuild.crypto.coin_failed", coin=coin, exc_info=True)

    return total


def _rebuild_backtest_sp500(
    algos: dict,
    step_size: int,
    cutoff: date,
    predict_from: date,
) -> int:
    """Walk-forward SP500 predictions; save only rows where target_date >= predict_from.

    Each symbol is processed in a separate thread.
    Thread safety: _bulk_sp500_predictions opens its own session_scope(); repo
    functions create their own sessions. algos dict is shared read-only.
    """
    symbols = repo.get_sp500_symbols() or SP500_SYMBOLS
    workers = _resolve_workers()
    log.info("rebuild.predict.workers", market="SP500", n=workers, symbols=len(symbols))

    def _symbol_task(symbol: str) -> int:
        task_total = 0
        prices_asc = repo.get_sp500_prices_asc(symbol, limit=500)
        if len(prices_asc) < _TRAIN_WINDOW + 1:
            return 0

        price_floats = [float(p.close_price) for p in prices_asc]
        dates = [p.trading_date for p in prices_asc]
        n = len(price_floats)

        for key, algo in algos.items():
            batch: list[dict] = []
            for window_end in range(_TRAIN_WINDOW, n, step_size):
                hist_end = min(window_end, _MAX_HISTORY)
                price_list = price_floats[window_end - hist_end:window_end]
                fold_end = min(window_end + step_size, n)

                for t in range(window_end, fold_end):
                    if _to_date(dates[t]) < predict_from:
                        continue
                    try:
                        result = algo.predict(price_list)
                        actual = Decimal(str(price_floats[t])).quantize(Decimal("0.0001"))
                        predicted = Decimal(str(result.predicted_price)).quantize(Decimal("0.0001"))
                        current = Decimal(str(price_floats[window_end - 1])).quantize(Decimal("0.0001"))
                        accuracy = max(
                            Decimal("0"),
                            Decimal("1") - abs(actual - predicted) / actual,
                        ).quantize(Decimal("0.0001"))
                        status = "confirmed" if float(accuracy) >= 0.70 else "wrong"

                        batch.append(dict(
                            symbol=symbol,
                            predicted_price=predicted,
                            current_price=current,
                            confidence=Decimal(str(round(result.confidence, 4))),
                            algorithm_name=key,
                            prediction_date=dates[window_end - 1],
                            target_date=dates[t],
                            actual_price=actual,
                            accuracy=accuracy,
                            status=status,
                            direction_correct=_compute_direction_correct(predicted, current, actual),
                        ))
                        if len(batch) >= _BATCH_SIZE:
                            _bulk_sp500_predictions(batch)
                            task_total += len(batch)
                            batch = []
                    except Exception:
                        log.error(
                            "rebuild.sp500.predict_failed",
                            algo=key,
                            symbol=symbol,
                            idx=t,
                            exc_info=True,
                        )
            if batch:
                _bulk_sp500_predictions(batch)
                task_total += len(batch)
                batch = []

        return task_total

    total = 0
    with ThreadPoolExecutor(max_workers=workers) as pool:
        futures = {pool.submit(_symbol_task, sym): sym for sym in symbols}
        for fut in as_completed(futures):
            sym = futures[fut]
            try:
                total += fut.result()
            except Exception:
                log.error("rebuild.sp500.symbol_failed", symbol=sym, exc_info=True)

    return total


# ---------------------------------------------------------------------------
# Thin adapter — exists so tests can monkeypatch without touching SimulationEngine
# ---------------------------------------------------------------------------

def _run_sim_backtest(sim_start: date, sim_end: date) -> int:
    """Instantiate SimulationEngine and run backtest for all active bots.

    Isolated in its own function so the unit-test suite can monkeypatch it at
    module level (src.orchestrator.rebuild._run_sim_backtest) without needing
    to wrestle with lazy-import scoping inside rebuild_and_replay.
    """
    from src.simulation.engine import SimulationEngine
    return SimulationEngine().run_all_bots_backtest(sim_start, sim_end)


# ---------------------------------------------------------------------------
# Main orchestrator
# ---------------------------------------------------------------------------

def rebuild_and_replay(
    cutoff_date: date,
    predict_from_date: date | None = None,
    step_size: int = 3,
    fast: bool = True,
) -> dict:
    """Wipe predictions + sim data and replay out-of-sample from cutoff.

    Phase 1 — Wipe: DELETE all prediction rows and simulation session rows
      (sim_bots kept intact so bot configs survive).
    Phase 2 — Walk-forward: all 12 algorithms × all 4 markets, expanding
      window capped at 270 days.  Saves predictions where target_date >=
      predict_from_date (covers both the meta-train window and the bot-trade
      window).  direction_correct is set inline — no separate reconcile pass.
    Phase 3 — train_meta_all(max_train_date=cutoff_date): retrain
      MetaStackModel using only prediction rows whose prediction_date <=
      cutoff_date, so the meta classifier never sees bot-trade-window data
      during training.
    Phase 4 — Bot replay: SimulationEngine.run_all_bots_backtest() from
      cutoff+1d → today. Bot trades are constrained to dates that have
      prediction rows.

    Parameters
    ----------
    cutoff_date:
        Last date of the META TRAIN window.  Predictions with target_date <=
        cutoff_date are used to train the meta classifier.  Bot trading starts
        from cutoff_date + 1 forward.
    predict_from_date:
        First date (inclusive) for which predictions are saved.  Defaults to
        cutoff_date - 30 days when None.  Must be <= cutoff_date.
    step_size:
        Walk-forward fold size in days (default 3).
    fast:
        When True (default), Optuna hyperparameter search is disabled for
        LightGBM and XGBoost during the walk-forward phase.  Both models are
        still trained with their default hyperparameters — only the 30-trial
        search is bypassed.  This gives a ~4-5x speedup for the rebuild.
        The flag is restored in a finally-block so live pipeline runs are
        never affected (their is_optuna_enabled() remains True).

    Returns
    -------
    dict with keys: predictions_generated, markets, meta_trained,
                    bots_replayed, cutoff_date, predict_from_date, duration_ms.
    """
    if step_size <= 0:
        step_size = 3

    if predict_from_date is None:
        predict_from_date = cutoff_date - timedelta(days=30)

    # Clamp: predict_from must not exceed cutoff
    if predict_from_date > cutoff_date:
        predict_from_date = cutoff_date

    started_at = datetime.now()
    trade_from = cutoff_date + timedelta(days=1)

    log.info(
        "rebuild.window",
        predict_from=str(predict_from_date),
        cutoff=str(cutoff_date),
        trade_from=str(trade_from),
        step_size=step_size,
        fast_mode=fast,
    )

    # --- Fast-mode: disable Optuna for the duration of the rebuild ---
    _optuna_was_enabled = is_optuna_enabled()
    if fast:
        set_optuna_enabled(False)
        log.info("rebuild.fast_mode", optuna_enabled=False)

    try:
        return _rebuild_core(
            cutoff_date=cutoff_date,
            predict_from_date=predict_from_date,
            step_size=step_size,
            trade_from=trade_from,
            started_at=started_at,
        )
    finally:
        if fast:
            set_optuna_enabled(_optuna_was_enabled)
            log.info("rebuild.fast_mode.restored", optuna_enabled=_optuna_was_enabled)


def _rebuild_core(
    cutoff_date: date,
    predict_from_date: date,
    step_size: int,
    trade_from: date,
    started_at: datetime,
) -> dict:
    """Internal core — called by rebuild_and_replay inside the fast-mode try/finally."""
    # --- Phase 1: Wipe ---
    log.info("rebuild.wipe.start")
    _wipe_predictions_and_sim()
    log.info("rebuild.wipe.done")

    # --- Phase 2 + 3: Walk-forward with direction_correct ---
    algos = build_algorithms()
    total_preds = 0

    # Use default-argument capture to avoid late-binding issues with lambdas
    _market_jobs: list[tuple[str, object]] = [
        ("GOLD",      lambda pf=predict_from_date: _rebuild_backtest_gold(algos, step_size, cutoff_date, pf)),
        ("NASDAQ100", lambda pf=predict_from_date: _rebuild_backtest_nasdaq(algos, step_size, cutoff_date, pf)),
        ("CRYPTO",    lambda pf=predict_from_date: _rebuild_backtest_crypto(algos, step_size, cutoff_date, pf)),
        ("SP500",     lambda pf=predict_from_date: _rebuild_backtest_sp500(algos, step_size, cutoff_date, pf)),
    ]

    for market_key, fn in _market_jobs:
        try:
            n = fn()
            total_preds += n
            log.info("rebuild.predict.market_done", market=market_key, predictions=n)
        except Exception as exc:
            log.error(
                "rebuild.predict.market_error",
                market=market_key,
                error=str(exc),
                exc_info=True,
            )

    log.info("rebuild.predict.done", total_predictions=total_preds)
    # direction_correct was set inline during the walk-forward — nothing extra needed
    log.info("rebuild.direction.done", note="set per-record during walk-forward")

    # --- Phase 4: Retrain meta models — restricted to meta-train window ---
    meta_trained = False
    try:
        train_meta_all(max_train_date=cutoff_date)
        meta_trained = True
        log.info("rebuild.meta.done", max_train_date=str(cutoff_date))
    except Exception as exc:
        log.error("rebuild.meta.error", error=str(exc), exc_info=True)

    # --- Phase 5: Replay bot trading ---
    bots_replayed = 0
    try:
        sim_start = trade_from
        sim_end = date.today()
        bots_replayed = _run_sim_backtest(sim_start, sim_end)
        log.info("rebuild.replay.done", bots=bots_replayed)
    except Exception as exc:
        log.error("rebuild.replay.error", error=str(exc), exc_info=True)

    duration_ms = int((datetime.now() - started_at).total_seconds() * 1000)

    return {
        "predictions_generated": total_preds,
        "markets": ["GOLD", "NASDAQ100", "CRYPTO", "SP500"],
        "meta_trained": meta_trained,
        "bots_replayed": bots_replayed,
        "cutoff_date": str(cutoff_date),
        "predict_from_date": str(predict_from_date),
        "duration_ms": duration_ms,
    }
