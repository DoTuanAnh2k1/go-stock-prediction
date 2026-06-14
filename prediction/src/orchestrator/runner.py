"""Orchestrator — runs predictions for all markets or a specific market.

Markets:
  GOLD     — gold instruments (XAU, BTMC/sjc, BTMC/nhan_tron) × 6 algorithms
  NASDAQ100 — 15 NASDAQ symbols × 6 algorithms
  CRYPTO   — BTC, ETH, SOL × 6 algorithms
  SP500    — 16 S&P 500 symbols × 6 algorithms
"""
from __future__ import annotations

from datetime import datetime, timedelta
from decimal import Decimal
from typing import Callable

from src.algorithms.registry import get_algos_for_market
from src.crawlers.crypto import COINS as CRYPTO_COINS
from src.crawlers.nasdaq import NASDAQ_SYMBOLS
from src.crawlers.sp500 import SP500_SYMBOLS
from src.database import repository as repo
from src.utils.logger import get_logger

log = get_logger("orchestrator")

GOLD_INSTRUMENTS = [
    ("XAU", "spot"),
    ("BTMC", "sjc"),
    ("BTMC", "nhan_tron"),
]

EmitFn = Callable[[str, str, float], None]  # (level, msg, progress 0-1)


def _noop(level: str, msg: str, progress: float = 0.0) -> None:
    pass


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


def run_for_market(market_key: str, force: bool = False, emit: EmitFn | None = None) -> int:
    """Run predictions for a single market. Returns prediction count.

    force=True bypasses the market-calendar check.
    emit(level, msg, progress) is called with live progress updates.
    """
    _emit = emit or _noop
    mk = market_key.upper()

    if not force:
        from src.utils.market_calendar import is_market_open
        if not is_market_open(mk):
            log.info("orchestrator.skip.market_closed", market=mk)
            _emit("warn", f"{mk}: thị trường đóng cửa, bỏ qua", 1.0)
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

    _trigger_sim_step(mk)
    return count


# ---------------------------------------------------------------------------
# Market-specific prediction runners
# ---------------------------------------------------------------------------

def _predict_gold(algos: dict, emit: EmitFn) -> int:
    instruments = GOLD_INSTRUMENTS
    total_ops = len(instruments) * len(algos)
    done_ops = 0

    log.info("predict.gold.start", instruments=len(instruments), algorithms=len(algos))
    emit("info", f"Gold: {len(instruments)} công cụ × {len(algos)} thuật toán", 0.0)

    dir_acc = repo.get_direction_accuracy("GOLD")
    count = 0
    now = datetime.now()
    target = now + timedelta(hours=1)

    for inst_idx, (source, product_type) in enumerate(instruments):
        label = f"{source}/{product_type}"
        emit("info", f"[{inst_idx + 1}/{len(instruments)}] {label} — đang tính...", done_ops / max(total_ops, 1))

        prices_asc = repo.get_gold_prices_asc(source, product_type, limit=270)
        if len(prices_asc) < 20:
            emit("warn", f"  {label}: không đủ dữ liệu ({len(prices_asc)} điểm)", done_ops / max(total_ops, 1))
            done_ops += len(algos)
            continue

        price_list = [float(p.buy_price) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                result = algo.predict(price_list)
                algo_acc_pct = dir_acc.get(key, None)
                algo_acc = Decimal(str(round(algo_acc_pct / 100, 4))) if algo_acc_pct is not None else None
                repo.create_gold_prediction(
                    source=source,
                    product_type=product_type,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.01")),
                    current_price=Decimal(str(current)).quantize(Decimal("0.01")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    accuracy=algo_acc,
                    status="pending",
                )
                count += 1
                done_ops += 1
                emit("ok", f"  {label} · {key}: {result.predicted_price:,.2f}", done_ops / max(total_ops, 1))
            except Exception as exc:
                done_ops += 1
                emit("warn", f"  {label} · {key}: lỗi — {exc}", done_ops / max(total_ops, 1))
                log.warning("predict.gold.algo_failed", source=source, product_type=product_type, algo=key, error=str(exc))

    log.info("predict.gold.done", count=count)
    emit("ok", f"Gold hoàn thành: {count} dự đoán đã lưu", 1.0)
    return count


def _predict_nasdaq(algos: dict, emit: EmitFn) -> int:
    symbols = repo.get_nasdaq_symbols() or NASDAQ_SYMBOLS
    total_ops = len(symbols) * len(algos)
    done_ops = 0

    log.info("predict.nasdaq.start", symbols=len(symbols), algorithms=len(algos))
    emit("info", f"NASDAQ: {len(symbols)} cổ phiếu × {len(algos)} thuật toán", 0.0)

    dir_acc = repo.get_direction_accuracy("NASDAQ100")
    count = 0
    now = datetime.now()
    target = now + timedelta(hours=1)

    for sym_idx, symbol in enumerate(symbols):
        emit("info", f"[{sym_idx + 1}/{len(symbols)}] {symbol} — đang tính...", done_ops / max(total_ops, 1))

        prices_asc = repo.get_nasdaq_prices_asc(symbol, limit=270)
        if len(prices_asc) < 20:
            emit("warn", f"  {symbol}: không đủ dữ liệu ({len(prices_asc)} điểm)", done_ops / max(total_ops, 1))
            done_ops += len(algos)
            continue

        price_list = [float(p.close_price) for p in prices_asc]
        vol_list = [float(p.volume or 0) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                result = algo.predict(price_list, vol_list)
                algo_acc_pct = dir_acc.get(key, None)
                algo_acc = Decimal(str(round(algo_acc_pct / 100, 4))) if algo_acc_pct is not None else None
                repo.create_nasdaq_prediction(
                    symbol=symbol,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.0001")),
                    current_price=Decimal(str(current)).quantize(Decimal("0.0001")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    accuracy=algo_acc,
                    status="pending",
                )
                count += 1
                done_ops += 1
                emit("ok", f"  {symbol} · {key}: {result.predicted_price:,.4f}", done_ops / max(total_ops, 1))
            except Exception as exc:
                done_ops += 1
                emit("warn", f"  {symbol} · {key}: lỗi — {exc}", done_ops / max(total_ops, 1))
                log.warning("predict.nasdaq.algo_failed", symbol=symbol, algo=key, error=str(exc))

    log.info("predict.nasdaq.done", count=count)
    emit("ok", f"NASDAQ hoàn thành: {count} dự đoán đã lưu", 1.0)
    return count


def _predict_crypto(algos: dict, emit: EmitFn) -> int:
    coins = CRYPTO_COINS
    total_ops = len(coins) * len(algos)
    done_ops = 0

    log.info("predict.crypto.start", coins=len(coins), algorithms=len(algos))
    emit("info", f"Crypto: {len(coins)} coin × {len(algos)} thuật toán", 0.0)

    dir_acc = repo.get_direction_accuracy("CRYPTO")
    count = 0
    now = datetime.now()
    target = now + timedelta(hours=1)

    for coin_idx, (coin_id, symbol) in enumerate(coins):
        emit("info", f"[{coin_idx + 1}/{len(coins)}] {symbol} — đang tính...", done_ops / max(total_ops, 1))

        prices_asc = repo.get_crypto_prices_asc(coin_id, limit=270)
        if len(prices_asc) < 20:
            emit("warn", f"  {symbol}: không đủ dữ liệu ({len(prices_asc)} điểm)", done_ops / max(total_ops, 1))
            done_ops += len(algos)
            continue

        price_list = [float(p.close_price) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                result = algo.predict(price_list)
                algo_acc_pct = dir_acc.get(key, None)
                algo_acc = Decimal(str(round(algo_acc_pct / 100, 4))) if algo_acc_pct is not None else None
                repo.create_crypto_prediction(
                    coin_id=coin_id,
                    symbol=symbol,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.01")),
                    current_price=Decimal(str(current)).quantize(Decimal("0.01")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    accuracy=algo_acc,
                    status="pending",
                )
                count += 1
                done_ops += 1
                emit("ok", f"  {symbol} · {key}: {result.predicted_price:,.2f}", done_ops / max(total_ops, 1))
            except Exception as exc:
                done_ops += 1
                emit("warn", f"  {symbol} · {key}: lỗi — {exc}", done_ops / max(total_ops, 1))
                log.warning("predict.crypto.algo_failed", coin=coin_id, algo=key, error=str(exc))

    log.info("predict.crypto.done", count=count)
    emit("ok", f"Crypto hoàn thành: {count} dự đoán đã lưu", 1.0)
    return count


def _predict_sp500(algos: dict, emit: EmitFn) -> int:
    symbols = repo.get_sp500_symbols() or SP500_SYMBOLS
    total_ops = len(symbols) * len(algos)
    done_ops = 0

    log.info("predict.sp500.start", symbols=len(symbols), algorithms=len(algos))
    emit("info", f"S&P 500: {len(symbols)} cổ phiếu × {len(algos)} thuật toán", 0.0)

    dir_acc = repo.get_direction_accuracy("SP500")
    count = 0
    now = datetime.now()
    target = now + timedelta(hours=1)

    for sym_idx, symbol in enumerate(symbols):
        emit("info", f"[{sym_idx + 1}/{len(symbols)}] {symbol} — đang tính...", done_ops / max(total_ops, 1))

        prices_asc = repo.get_sp500_prices_asc(symbol, limit=270)
        if len(prices_asc) < 20:
            emit("warn", f"  {symbol}: không đủ dữ liệu ({len(prices_asc)} điểm)", done_ops / max(total_ops, 1))
            done_ops += len(algos)
            continue

        price_list = [float(p.close_price) for p in prices_asc]
        vol_list = [float(p.volume or 0) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                result = algo.predict(price_list, vol_list)
                algo_acc_pct = dir_acc.get(key, None)
                algo_acc = Decimal(str(round(algo_acc_pct / 100, 4))) if algo_acc_pct is not None else None
                repo.create_sp500_prediction(
                    symbol=symbol,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.0001")),
                    current_price=Decimal(str(current)).quantize(Decimal("0.0001")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    accuracy=algo_acc,
                    status="pending",
                )
                count += 1
                done_ops += 1
                emit("ok", f"  {symbol} · {key}: {result.predicted_price:,.4f}", done_ops / max(total_ops, 1))
            except Exception as exc:
                done_ops += 1
                emit("warn", f"  {symbol} · {key}: lỗi — {exc}", done_ops / max(total_ops, 1))
                log.warning("predict.sp500.algo_failed", symbol=symbol, algo=key, error=str(exc))

    log.info("predict.sp500.done", count=count)
    emit("ok", f"S&P 500 hoàn thành: {count} dự đoán đã lưu", 1.0)
    return count


def _trigger_sim_step(market_key: str) -> None:
    """Trigger simulation step for bots of this market after predictions complete."""
    try:
        from src.simulation.engine import SimulationEngine
        SimulationEngine().run_live_step_for_market(market_key)
    except Exception as exc:
        log.warning("orchestrator.sim_step_failed", market=market_key, error=str(exc))
