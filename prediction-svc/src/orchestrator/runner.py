"""Orchestrator — runs predictions for all markets or a specific market.

Markets:
  GOLD     — gold instruments (XAU, BTMC/sjc, BTMC/nhan_tron) × 6 algorithms
  NASDAQ100 — 15 NASDAQ symbols × 6 algorithms
  CRYPTO   — BTC, ETH, SOL × 6 algorithms
  SP500    — 16 S&P 500 symbols × 6 algorithms

Per-symbol mode (additive):
  When ``settings.per_symbol_enabled`` is True, each market runner also writes
  per-symbol predictions with algorithm_name suffixed by ``PS_SUFFIX`` ("__ps").
  Per-symbol predictions are produced ONLY for symbols that have pre-trained
  per-symbol instances in the registry (cold-start symbols are skipped).
"""
from __future__ import annotations

import os
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timedelta
from decimal import Decimal
from typing import Callable

from src.algorithms.registry import PS_SUFFIX, get_algos_for_market, get_trained_algos_for_symbol
from src.config import get_settings
from src.crawlers.crypto import COINS as CRYPTO_COINS
from src.crawlers.nasdaq import NASDAQ_SYMBOLS
from src.crawlers.sp500 import SP500_SYMBOLS
from src.database import repository as repo
from src.utils.logger import get_logger
from src.utils.market_calendar import is_intraday_open, next_trading_day
from src.telemetry import metrics as _metrics

log = get_logger("orchestrator")

GOLD_INSTRUMENTS = [
    ("XAU", "spot"),
    ("BTMC", "sjc"),
    ("BTMC", "nhan_tron"),
]

EmitFn = Callable[[str, str, float], None]  # (level, msg, progress 0-1)


def _noop(level: str, msg: str, progress: float = 0.0) -> None:
    pass


def _ps_workers() -> int:
    """Thread-pool worker count for per-symbol prediction pass."""
    settings = get_settings()
    n = settings.per_symbol_workers or (os.cpu_count() or 4)
    return max(1, min(8, n))


def _apply_ensemble_weights(algos: dict, dir_acc: dict) -> None:
    """Feed per-algorithm direction accuracy into the ensemble so it weights
    its bases by skill instead of averaging everyone equally. No-op if the
    ensemble or accuracy data is missing (it falls back to equal weighting)."""
    ens = algos.get("ensemble")
    if ens is not None and hasattr(ens, "set_weights"):
        ens.set_weights(dir_acc)


def run_all_markets() -> int:
    """Run predictions for ALL markets. Returns total prediction count."""
    total = 0
    for market_key in ("GOLD", "NASDAQ100", "CRYPTO", "SP500"):
        try:
            n = run_for_market(market_key)
            total += n
            log.info("orchestrator.market_done", market=market_key, predictions=n)
        except Exception as exc:
            log.error("orchestrator.market_failed", market=market_key, error=str(exc))
    return total


def run_for_market(
    market_key: str, force: bool = False, emit: EmitFn | None = None, run_sim: bool = True
) -> int:
    """Run predictions for a single market. Returns prediction count.

    force=True bypasses the market-calendar check.
    emit(level, msg, progress) is called with live progress updates.
    run_sim=False skips the inline bot step — used by the Kafka predict-consumer,
    which lets the separate simulation-consumer own that stage (event-driven mode).
    """
    _emit = emit or _noop
    mk = market_key.upper()

    if not force:
        from src.utils.market_calendar import is_market_open
        if not is_market_open(mk):
            log.info("orchestrator.skip.market_closed", market=mk)
            _emit("warn", f"{mk}: market closed, skipping", 1.0)
            return 0

    algos = get_algos_for_market(mk)

    if mk == "GOLD":
        count = _predict_gold(algos, _emit)
    elif mk == "NASDAQ100":
        count = _predict_nasdaq(algos, _emit)
    elif mk == "CRYPTO":
        count = _predict_crypto(algos, _emit)
    elif mk == "SP500":
        count = _predict_sp500(algos, _emit)
    else:
        raise ValueError(f"Unknown market key: {market_key!r}")

    if run_sim:
        _trigger_sim_step(mk)
    return count


# ---------------------------------------------------------------------------
# Market-specific prediction runners
# ---------------------------------------------------------------------------

def _transformer_intraday_prices(registry_market: str, ident: str, now: datetime) -> list[float]:
    """Hourly intraday history for transformer_nn (horizon-matched input).

    The system predicts +1h; transformer_nn is trained on hourly bars, so its
    inference input must be hourly too. ident: symbol (NASDAQ/SP500), coin_id
    (CRYPTO) or source (GOLD). Returns [] on any error → caller falls back to
    the daily series.
    """
    try:
        if registry_market == "NASDAQ100":
            rows = repo.get_nasdaq_intraday_asc_as_of(ident, now, limit=400)
            return [float(r.close_price) for r in rows if r.close_price is not None]
        if registry_market == "SP500":
            rows = repo.get_sp500_intraday_asc_as_of(ident, now, limit=400)
            return [float(r.close_price) for r in rows if r.close_price is not None]
        if registry_market == "CRYPTO":
            rows = repo.get_crypto_intraday_asc_as_of(ident, now, limit=400)
            return [float(r.price) for r in rows if r.price is not None]
        if registry_market == "GOLD":
            rows = repo.get_gold_intraday_asc_as_of(ident, now, limit=400)
            out = []
            for r in rows:
                if r.sell_price is not None:
                    out.append(float(r.sell_price))
                elif r.buy_price is not None:
                    out.append(float(r.buy_price))
            return out
    except Exception:
        return []
    return []


def _predict_gold(algos: dict, emit: EmitFn) -> int:
    instruments = GOLD_INSTRUMENTS
    total_ops = len(instruments) * len(algos)
    done_ops = 0

    log.info("predict.gold.start", instruments=len(instruments), algorithms=len(algos))
    emit("info", f"Gold: {len(instruments)} instruments x {len(algos)} algorithms", 0.0)

    dir_acc = repo.get_direction_accuracy("GOLD")
    _apply_ensemble_weights(algos, dir_acc)
    for _algo_key, _acc_val in dir_acc.items():
        _metrics.set_direction_accuracy("GOLD", _algo_key, _acc_val)
    count = 0
    now = datetime.now()
    target = now + timedelta(hours=1)

    for inst_idx, (source, product_type) in enumerate(instruments):
        label = f"{source}/{product_type}"
        emit("info", f"[{inst_idx + 1}/{len(instruments)}] {label} - computing...", done_ops / max(total_ops, 1))

        prices_asc = repo.get_gold_prices_asc(source, product_type, limit=270)
        if len(prices_asc) < 20:
            emit("warn", f"  {label}: insufficient data ({len(prices_asc)} points)", done_ops / max(total_ops, 1))
            done_ops += len(algos)
            continue

        price_list = [float(p.buy_price) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                p_input = price_list
                if key == "transformer_nn":
                    ip = _transformer_intraday_prices("GOLD", source, now)
                    if len(ip) >= 110:
                        p_input = ip
                result = algo.predict(p_input)
                if getattr(result, "skip_write", False):
                    done_ops += 1
                    emit("info", f"  {label} / {key}: skipped (no conviction)", done_ops / max(total_ops, 1))
                    continue
                algo_acc_pct = dir_acc.get(key, None)
                algo_acc = Decimal(str(round(algo_acc_pct / 100, 4))) if algo_acc_pct is not None else None
                repo.create_gold_prediction(
                    source=source,
                    product_type=product_type,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.01")),
                    current_price=Decimal(str(result.current_price)).quantize(Decimal("0.01")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    accuracy=algo_acc,
                    status="pending",
                )
                count += 1
                _metrics.record_prediction("GOLD", key)
                done_ops += 1
                emit("ok", f"  {label} / {key}: {result.predicted_price:,.2f}", done_ops / max(total_ops, 1))
            except Exception as exc:
                done_ops += 1
                emit("warn", f"  {label} / {key}: error - {exc}", done_ops / max(total_ops, 1))
                log.error(
                    "algo.predict.failed",
                    market="GOLD",
                    algo=key,
                    source=source,
                    product_type=product_type,
                    error=str(exc),
                    exc_info=True,
                )

    log.info("predict.gold.done", count=count)

    # Per-symbol pass (additive — only when enabled and instances are pre-trained)
    if get_settings().per_symbol_enabled:
        count += _predict_gold_per_symbol(now, target)

    emit("ok", f"Gold complete: {count} predictions saved", 1.0)
    return count


def _predict_gold_per_symbol(now: datetime, target: datetime) -> int:
    """Write per-symbol predictions for GOLD instruments."""
    def _work(source: str, product_type: str) -> int:
        symbol_label = f"{source}_{product_type}"
        ps_algos = get_trained_algos_for_symbol("GOLD", symbol_label)
        if not ps_algos:
            return 0  # cold-start — skip

        prices_asc = repo.get_gold_prices_asc(source, product_type, limit=270)
        if len(prices_asc) < 20:
            return 0

        price_list = [float(p.buy_price) for p in prices_asc]
        current = price_list[-1]
        local_count = 0

        for key, algo in ps_algos.items():
            try:
                result = algo.predict(price_list)
                if getattr(result, "skip_write", False):
                    continue
                repo.create_gold_prediction(
                    source=source,
                    product_type=product_type,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.01")),
                    current_price=Decimal(str(result.current_price)).quantize(Decimal("0.01")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=f"{key}{PS_SUFFIX}",
                    prediction_date=now,
                    target_date=target,
                    accuracy=None,
                    status="pending",
                )
                local_count += 1
            except Exception as exc:
                log.error(
                    "algo.predict.failed",
                    market="GOLD",
                    algo=key,
                    source=source,
                    product_type=product_type,
                    error=str(exc),
                    exc_info=True,
                )
        return local_count

    total = 0
    with ThreadPoolExecutor(max_workers=_ps_workers()) as pool:
        futures = [
            pool.submit(_work, source, product_type)
            for source, product_type in GOLD_INSTRUMENTS
        ]
        for future in as_completed(futures):
            try:
                total += future.result()
            except Exception as exc:
                log.warning("predict.gold.ps.worker_failed", error=str(exc))

    if total:
        log.info("predict.gold.ps.done", count=total)
    return total


def _predict_nasdaq(algos: dict, emit: EmitFn) -> int:
    # Guard: only predict during US market hours (9:30 AM – 4:00 PM ET, weekday, no holiday)
    if not is_intraday_open("NASDAQ100"):
        log.info("predict.nasdaq.skip.not_intraday", reason="outside US session hours")
        emit("warn", "NASDAQ: outside US session hours, skipping predict", 1.0)
        return 0

    symbols = repo.get_nasdaq_symbols() or NASDAQ_SYMBOLS
    total_ops = len(symbols) * len(algos)
    done_ops = 0

    log.info("predict.nasdaq.start", symbols=len(symbols), algorithms=len(algos))
    emit("info", f"NASDAQ: {len(symbols)} symbols x {len(algos)} algorithms", 0.0)

    dir_acc = repo.get_direction_accuracy("NASDAQ100")
    _apply_ensemble_weights(algos, dir_acc)
    for _algo_key, _acc_val in dir_acc.items():
        _metrics.set_direction_accuracy("NASDAQ100", _algo_key, _acc_val)
    count = 0
    now = datetime.now()
    target = now + timedelta(hours=1)

    for sym_idx, symbol in enumerate(symbols):
        emit("info", f"[{sym_idx + 1}/{len(symbols)}] {symbol} - computing...", done_ops / max(total_ops, 1))

        prices_asc = repo.get_nasdaq_prices_asc(symbol, limit=270)
        if len(prices_asc) < 20:
            emit("warn", f"  {symbol}: insufficient data ({len(prices_asc)} points)", done_ops / max(total_ops, 1))
            done_ops += len(algos)
            continue

        price_list = [float(p.close_price) for p in prices_asc]
        vol_list = [float(p.volume or 0) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                # Symbol context for symbol-aware algos (transformer_nn joins fundamentals)
                algo._context_symbol = symbol
                p_input, v_input = price_list, vol_list
                if key == "transformer_nn":
                    ip = _transformer_intraday_prices("NASDAQ100", symbol, now)
                    if len(ip) >= 110:
                        p_input, v_input = ip, None
                result = algo.predict(p_input, v_input)
                if getattr(result, "skip_write", False):
                    done_ops += 1
                    emit("info", f"  {symbol} / {key}: skipped (no conviction)", done_ops / max(total_ops, 1))
                    continue
                algo_acc_pct = dir_acc.get(key, None)
                algo_acc = Decimal(str(round(algo_acc_pct / 100, 4))) if algo_acc_pct is not None else None
                repo.create_nasdaq_prediction(
                    symbol=symbol,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.0001")),
                    current_price=Decimal(str(result.current_price)).quantize(Decimal("0.0001")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    accuracy=algo_acc,
                    status="pending",
                )
                count += 1
                _metrics.record_prediction("NASDAQ100", key)
                done_ops += 1
                emit("ok", f"  {symbol} / {key}: {result.predicted_price:,.4f}", done_ops / max(total_ops, 1))
            except Exception as exc:
                done_ops += 1
                emit("warn", f"  {symbol} / {key}: error - {exc}", done_ops / max(total_ops, 1))
                log.error(
                    "algo.predict.failed",
                    market="NASDAQ100",
                    algo=key,
                    symbol=symbol,
                    error=str(exc),
                    exc_info=True,
                )

    log.info("predict.nasdaq.done", count=count)

    # Per-symbol pass (additive — only when enabled and instances are pre-trained)
    if get_settings().per_symbol_enabled:
        symbols_snap = repo.get_nasdaq_symbols() or NASDAQ_SYMBOLS
        count += _predict_nasdaq_per_symbol(now, target, symbols_snap)

    emit("ok", f"NASDAQ complete: {count} predictions saved", 1.0)
    return count


def _predict_nasdaq_per_symbol(now: datetime, target: datetime, symbols: list[str]) -> int:
    """Write per-symbol predictions for NASDAQ100 symbols."""
    def _work(symbol: str) -> int:
        ps_algos = get_trained_algos_for_symbol("NASDAQ100", symbol)
        if not ps_algos:
            return 0

        prices_asc = repo.get_nasdaq_prices_asc(symbol, limit=270)
        if len(prices_asc) < 20:
            return 0

        price_list = [float(p.close_price) for p in prices_asc]
        vol_list = [float(p.volume or 0) for p in prices_asc]
        current = price_list[-1]
        local_count = 0

        for key, algo in ps_algos.items():
            try:
                result = algo.predict(price_list, vol_list)
                if getattr(result, "skip_write", False):
                    continue
                repo.create_nasdaq_prediction(
                    symbol=symbol,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.0001")),
                    current_price=Decimal(str(result.current_price)).quantize(Decimal("0.0001")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=f"{key}{PS_SUFFIX}",
                    prediction_date=now,
                    target_date=target,
                    accuracy=None,
                    status="pending",
                )
                local_count += 1
            except Exception as exc:
                log.error(
                    "algo.predict.failed",
                    market="NASDAQ100",
                    algo=key,
                    symbol=symbol,
                    error=str(exc),
                    exc_info=True,
                )
        return local_count

    total = 0
    with ThreadPoolExecutor(max_workers=_ps_workers()) as pool:
        futures = {pool.submit(_work, sym): sym for sym in symbols}
        for future in as_completed(futures):
            try:
                total += future.result()
            except Exception as exc:
                log.warning("predict.nasdaq.ps.worker_failed", error=str(exc))

    if total:
        log.info("predict.nasdaq.ps.done", count=total)
    return total


def _predict_crypto(algos: dict, emit: EmitFn) -> int:
    coins = CRYPTO_COINS
    total_ops = len(coins) * len(algos)
    done_ops = 0

    log.info("predict.crypto.start", coins=len(coins), algorithms=len(algos))
    emit("info", f"Crypto: {len(coins)} coins x {len(algos)} algorithms", 0.0)

    dir_acc = repo.get_direction_accuracy("CRYPTO")
    _apply_ensemble_weights(algos, dir_acc)
    for _algo_key, _acc_val in dir_acc.items():
        _metrics.set_direction_accuracy("CRYPTO", _algo_key, _acc_val)
    count = 0
    now = datetime.now()
    # CRYPTO 24/7 — target = now + 1h (GOLD-style hourly prediction)
    target = now + timedelta(hours=1)

    for coin_idx, (coin_id, symbol) in enumerate(coins):
        emit("info", f"[{coin_idx + 1}/{len(coins)}] {symbol} - computing...", done_ops / max(total_ops, 1))

        prices_asc = repo.get_crypto_prices_asc(coin_id, limit=270)
        if len(prices_asc) < 20:
            emit("warn", f"  {symbol}: insufficient data ({len(prices_asc)} points)", done_ops / max(total_ops, 1))
            done_ops += len(algos)
            continue

        price_list = [float(p.close_price) for p in prices_asc]
        vol_list = [float(p.volume24h or 0) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                p_input, v_input = price_list, vol_list
                if key == "transformer_nn":
                    ip = _transformer_intraday_prices("CRYPTO", coin_id, now)
                    if len(ip) >= 110:
                        p_input, v_input = ip, None
                result = algo.predict(p_input, v_input)
                if getattr(result, "skip_write", False):
                    done_ops += 1
                    emit("info", f"  {symbol} / {key}: skipped (no conviction)", done_ops / max(total_ops, 1))
                    continue
                algo_acc_pct = dir_acc.get(key, None)
                algo_acc = Decimal(str(round(algo_acc_pct / 100, 4))) if algo_acc_pct is not None else None
                repo.create_crypto_prediction(
                    coin_id=coin_id,
                    symbol=symbol,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.01")),
                    current_price=Decimal(str(result.current_price)).quantize(Decimal("0.01")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    accuracy=algo_acc,
                    status="pending",
                )
                count += 1
                _metrics.record_prediction("CRYPTO", key)
                done_ops += 1
                emit("ok", f"  {symbol} / {key}: {result.predicted_price:,.2f}", done_ops / max(total_ops, 1))
            except Exception as exc:
                done_ops += 1
                emit("warn", f"  {symbol} / {key}: error - {exc}", done_ops / max(total_ops, 1))
                log.error(
                    "algo.predict.failed",
                    market="CRYPTO",
                    algo=key,
                    symbol=symbol,
                    coin=coin_id,
                    error=str(exc),
                    exc_info=True,
                )

    log.info("predict.crypto.done", count=count)

    # Per-symbol pass (additive — only when enabled and instances are pre-trained)
    if get_settings().per_symbol_enabled:
        count += _predict_crypto_per_symbol(now, target, list(CRYPTO_COINS))

    emit("ok", f"Crypto complete: {count} predictions saved", 1.0)
    return count


def _predict_crypto_per_symbol(now: datetime, target: datetime, coins: list[tuple[str, str]]) -> int:
    """Write per-symbol predictions for Crypto coins."""
    def _work(coin_id: str, symbol: str) -> int:
        # symbol_label for crypto is the coin symbol (BTC/ETH/SOL)
        ps_algos = get_trained_algos_for_symbol("CRYPTO", symbol)
        if not ps_algos:
            return 0

        prices_asc = repo.get_crypto_prices_asc(coin_id, limit=270)
        if len(prices_asc) < 20:
            return 0

        price_list = [float(p.close_price) for p in prices_asc]
        vol_list = [float(p.volume24h or 0) for p in prices_asc]
        current = price_list[-1]
        local_count = 0

        for key, algo in ps_algos.items():
            try:
                result = algo.predict(price_list, vol_list)
                if getattr(result, "skip_write", False):
                    continue
                repo.create_crypto_prediction(
                    coin_id=coin_id,
                    symbol=symbol,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.01")),
                    current_price=Decimal(str(result.current_price)).quantize(Decimal("0.01")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=f"{key}{PS_SUFFIX}",
                    prediction_date=now,
                    target_date=target,
                    accuracy=None,
                    status="pending",
                )
                local_count += 1
            except Exception as exc:
                log.error(
                    "algo.predict.failed",
                    market="CRYPTO",
                    algo=key,
                    symbol=symbol,
                    coin=coin_id,
                    error=str(exc),
                    exc_info=True,
                )
        return local_count

    total = 0
    with ThreadPoolExecutor(max_workers=_ps_workers()) as pool:
        futures = {pool.submit(_work, cid, sym): sym for cid, sym in coins}
        for future in as_completed(futures):
            try:
                total += future.result()
            except Exception as exc:
                log.warning("predict.crypto.ps.worker_failed", error=str(exc))

    if total:
        log.info("predict.crypto.ps.done", count=total)
    return total


def _predict_sp500(algos: dict, emit: EmitFn) -> int:
    # Guard: only predict during US market hours (9:30 AM – 4:00 PM ET, weekday, no holiday)
    if not is_intraday_open("SP500"):
        log.info("predict.sp500.skip.not_intraday", reason="outside US session hours")
        emit("warn", "S&P 500: outside US session hours, skipping predict", 1.0)
        return 0

    symbols = repo.get_sp500_symbols() or SP500_SYMBOLS
    total_ops = len(symbols) * len(algos)
    done_ops = 0

    log.info("predict.sp500.start", symbols=len(symbols), algorithms=len(algos))
    emit("info", f"S&P 500: {len(symbols)} symbols x {len(algos)} algorithms", 0.0)

    dir_acc = repo.get_direction_accuracy("SP500")
    _apply_ensemble_weights(algos, dir_acc)
    for _algo_key, _acc_val in dir_acc.items():
        _metrics.set_direction_accuracy("SP500", _algo_key, _acc_val)
    count = 0
    now = datetime.now()
    target = now + timedelta(hours=1)

    for sym_idx, symbol in enumerate(symbols):
        emit("info", f"[{sym_idx + 1}/{len(symbols)}] {symbol} - computing...", done_ops / max(total_ops, 1))

        prices_asc = repo.get_sp500_prices_asc(symbol, limit=270)
        if len(prices_asc) < 20:
            emit("warn", f"  {symbol}: insufficient data ({len(prices_asc)} points)", done_ops / max(total_ops, 1))
            done_ops += len(algos)
            continue

        price_list = [float(p.close_price) for p in prices_asc]
        vol_list = [float(p.volume or 0) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                # Symbol context for symbol-aware algos (transformer_nn joins fundamentals)
                algo._context_symbol = symbol
                p_input, v_input = price_list, vol_list
                if key == "transformer_nn":
                    ip = _transformer_intraday_prices("SP500", symbol, now)
                    if len(ip) >= 110:
                        p_input, v_input = ip, None
                result = algo.predict(p_input, v_input)
                if getattr(result, "skip_write", False):
                    done_ops += 1
                    emit("info", f"  {symbol} / {key}: skipped (no conviction)", done_ops / max(total_ops, 1))
                    continue
                algo_acc_pct = dir_acc.get(key, None)
                algo_acc = Decimal(str(round(algo_acc_pct / 100, 4))) if algo_acc_pct is not None else None
                repo.create_sp500_prediction(
                    symbol=symbol,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.0001")),
                    current_price=Decimal(str(result.current_price)).quantize(Decimal("0.0001")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    accuracy=algo_acc,
                    status="pending",
                )
                count += 1
                _metrics.record_prediction("SP500", key)
                done_ops += 1
                emit("ok", f"  {symbol} / {key}: {result.predicted_price:,.4f}", done_ops / max(total_ops, 1))
            except Exception as exc:
                done_ops += 1
                emit("warn", f"  {symbol} / {key}: error - {exc}", done_ops / max(total_ops, 1))
                log.error(
                    "algo.predict.failed",
                    market="SP500",
                    algo=key,
                    symbol=symbol,
                    error=str(exc),
                    exc_info=True,
                )

    log.info("predict.sp500.done", count=count)

    # Per-symbol pass (additive — only when enabled and instances are pre-trained)
    if get_settings().per_symbol_enabled:
        sp_symbols_snap = repo.get_sp500_symbols() or SP500_SYMBOLS
        count += _predict_sp500_per_symbol(now, target, sp_symbols_snap)

    emit("ok", f"S&P 500 complete: {count} predictions saved", 1.0)
    return count


def _predict_sp500_per_symbol(now: datetime, target: datetime, symbols: list[str]) -> int:
    """Write per-symbol predictions for SP500 symbols."""
    def _work(symbol: str) -> int:
        ps_algos = get_trained_algos_for_symbol("SP500", symbol)
        if not ps_algos:
            return 0

        prices_asc = repo.get_sp500_prices_asc(symbol, limit=270)
        if len(prices_asc) < 20:
            return 0

        price_list = [float(p.close_price) for p in prices_asc]
        vol_list = [float(p.volume or 0) for p in prices_asc]
        current = price_list[-1]
        local_count = 0

        for key, algo in ps_algos.items():
            try:
                result = algo.predict(price_list, vol_list)
                if getattr(result, "skip_write", False):
                    continue
                repo.create_sp500_prediction(
                    symbol=symbol,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.0001")),
                    current_price=Decimal(str(result.current_price)).quantize(Decimal("0.0001")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=f"{key}{PS_SUFFIX}",
                    prediction_date=now,
                    target_date=target,
                    accuracy=None,
                    status="pending",
                )
                local_count += 1
            except Exception as exc:
                log.error(
                    "algo.predict.failed",
                    market="SP500",
                    algo=key,
                    symbol=symbol,
                    error=str(exc),
                    exc_info=True,
                )
        return local_count

    total = 0
    with ThreadPoolExecutor(max_workers=_ps_workers()) as pool:
        futures = {pool.submit(_work, sym): sym for sym in symbols}
        for future in as_completed(futures):
            try:
                total += future.result()
            except Exception as exc:
                log.warning("predict.sp500.ps.worker_failed", error=str(exc))

    if total:
        log.info("predict.sp500.ps.done", count=total)
    return total


def _trigger_sim_step(market_key: str) -> None:
    """Trigger simulation step for bots of this market after predictions complete."""
    try:
        from src.simulation.engine import SimulationEngine
        SimulationEngine().run_live_step_for_market(market_key)
    except Exception as exc:
        log.warning("orchestrator.sim_step_failed", market=market_key, error=str(exc))
