"""Orchestrator — runs predictions for all markets or a specific market.

Markets:
  VN30     — 30 Vietnamese stocks × 6 algorithms
  GOLD     — gold instruments (XAU, BTMC/sjc, BTMC/nhan_tron) × 6 algorithms
  NASDAQ100 — 15 NASDAQ symbols × 6 algorithms
  CRYPTO   — BTC, ETH × 6 algorithms
  FUEL     — ron95_iii, e5_ron92, do_005s, kerosene × 6 algorithms
"""
from __future__ import annotations

from datetime import datetime, timedelta
from decimal import Decimal

from src.algorithms.registry import build_algorithms
from src.crawlers.crypto import COINS as CRYPTO_COINS
from src.crawlers.nasdaq import NASDAQ_SYMBOLS
from src.database import repository as repo
from src.utils.logger import get_logger

log = get_logger("orchestrator")

GOLD_INSTRUMENTS = [
    ("XAU", "spot"),
    ("BTMC", "sjc"),
    ("BTMC", "nhan_tron"),
]

FUEL_PRODUCTS = ["ron95_iii", "e5_ron92", "do_005s", "kerosene"]


def run_all_markets() -> int:
    """Run predictions for ALL markets. Returns total prediction count."""
    total = 0
    for market_key in ("VN30", "GOLD", "NASDAQ100", "CRYPTO", "FUEL"):
        try:
            n = run_for_market(market_key)
            total += n
            log.info("orchestrator.market_done", market=market_key, predictions=n)
        except Exception as exc:
            log.error("orchestrator.market_failed", market=market_key, error=str(exc))
    return total


def run_for_market(market_key: str) -> int:
    """Run predictions for a single market. Returns prediction count."""
    mk = market_key.upper()
    algos = build_algorithms()

    if mk in ("VN30", ""):
        return _predict_vn30(algos)
    elif mk == "GOLD":
        return _predict_gold(algos)
    elif mk == "NASDAQ100":
        return _predict_nasdaq(algos)
    elif mk == "CRYPTO":
        return _predict_crypto(algos)
    elif mk == "FUEL":
        return _predict_fuel(algos)
    else:
        raise ValueError(f"Unknown market key: {market_key!r}")


# ---------------------------------------------------------------------------
# Market-specific prediction runners
# ---------------------------------------------------------------------------

def _predict_vn30(algos: dict) -> int:
    stocks = repo.get_vn30_stocks()
    log.info("predict.vn30.start", stocks=len(stocks), algorithms=len(algos))
    count = 0
    now = datetime.utcnow()
    target = now + timedelta(days=1)

    for stock in stocks:
        prices_asc = repo.get_stock_prices_asc(stock.id, limit=270)
        if len(prices_asc) < 20:
            log.debug("predict.vn30.insufficient", symbol=stock.symbol, points=len(prices_asc))
            continue

        price_list = [float(p.close_price) for p in prices_asc]
        vol_list = [float(p.volume or 0) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                result = algo.predict(price_list, vol_list)
                repo.create_prediction(
                    stock_id=stock.id,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.01")),
                    current_price=Decimal(str(current)).quantize(Decimal("0.01")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    status="pending",
                )
                count += 1
            except Exception as exc:
                log.debug("predict.vn30.algo_failed", symbol=stock.symbol, algo=key, error=str(exc))

    log.info("predict.vn30.done", count=count)
    return count


def _predict_gold(algos: dict) -> int:
    log.info("predict.gold.start", instruments=len(GOLD_INSTRUMENTS), algorithms=len(algos))
    count = 0
    now = datetime.utcnow()
    target = now + timedelta(days=1)

    for source, product_type in GOLD_INSTRUMENTS:
        prices_asc = repo.get_gold_prices_asc(source, product_type, limit=270)
        if len(prices_asc) < 20:
            log.debug("predict.gold.insufficient", source=source, product_type=product_type, points=len(prices_asc))
            continue

        price_list = [float(p.buy_price) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                result = algo.predict(price_list)
                repo.create_gold_prediction(
                    source=source,
                    product_type=product_type,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.01")),
                    current_price=Decimal(str(current)).quantize(Decimal("0.01")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    status="pending",
                )
                count += 1
            except Exception as exc:
                log.debug("predict.gold.algo_failed", source=source, product_type=product_type, algo=key, error=str(exc))

    log.info("predict.gold.done", count=count)
    return count


def _predict_nasdaq(algos: dict) -> int:
    symbols = repo.get_nasdaq_symbols() or NASDAQ_SYMBOLS
    log.info("predict.nasdaq.start", symbols=len(symbols), algorithms=len(algos))
    count = 0
    now = datetime.utcnow()
    target = now + timedelta(days=1)

    for symbol in symbols:
        prices_asc = repo.get_nasdaq_prices_asc(symbol, limit=270)
        if len(prices_asc) < 20:
            log.debug("predict.nasdaq.insufficient", symbol=symbol, points=len(prices_asc))
            continue

        price_list = [float(p.close_price) for p in prices_asc]
        vol_list = [float(p.volume or 0) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                result = algo.predict(price_list, vol_list)
                repo.create_nasdaq_prediction(
                    symbol=symbol,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.0001")),
                    current_price=Decimal(str(current)).quantize(Decimal("0.0001")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    status="pending",
                )
                count += 1
            except Exception as exc:
                log.debug("predict.nasdaq.algo_failed", symbol=symbol, algo=key, error=str(exc))

    log.info("predict.nasdaq.done", count=count)
    return count


def _predict_crypto(algos: dict) -> int:
    log.info("predict.crypto.start", coins=len(CRYPTO_COINS), algorithms=len(algos))
    count = 0
    now = datetime.utcnow()
    target = now + timedelta(days=1)

    for coin_id, symbol in CRYPTO_COINS:
        prices_asc = repo.get_crypto_prices_asc(coin_id, limit=270)
        if len(prices_asc) < 20:
            log.debug("predict.crypto.insufficient", coin=coin_id, points=len(prices_asc))
            continue

        price_list = [float(p.close_price) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                result = algo.predict(price_list)
                repo.create_crypto_prediction(
                    coin_id=coin_id,
                    symbol=symbol,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.01")),
                    current_price=Decimal(str(current)).quantize(Decimal("0.01")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    status="pending",
                )
                count += 1
            except Exception as exc:
                log.debug("predict.crypto.algo_failed", coin=coin_id, algo=key, error=str(exc))

    log.info("predict.crypto.done", count=count)
    return count


def _predict_fuel(algos: dict) -> int:
    log.info("predict.fuel.start", products=len(FUEL_PRODUCTS), algorithms=len(algos))
    count = 0
    now = datetime.utcnow()
    target = now + timedelta(days=1)

    for product_type in FUEL_PRODUCTS:
        prices_asc = repo.get_fuel_prices_asc(product_type, limit=270)
        if len(prices_asc) < 20:
            log.debug("predict.fuel.insufficient", product=product_type, points=len(prices_asc))
            continue

        price_list = [float(p.price) for p in prices_asc]
        current = price_list[-1]

        for key, algo in algos.items():
            try:
                result = algo.predict(price_list)
                repo.create_fuel_prediction(
                    product_type=product_type,
                    predicted_price=Decimal(str(result.predicted_price)).quantize(Decimal("0.001")),
                    current_price=Decimal(str(current)).quantize(Decimal("0.001")),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name=key,
                    prediction_date=now,
                    target_date=target,
                    status="pending",
                )
                count += 1
            except Exception as exc:
                log.debug("predict.fuel.algo_failed", product=product_type, algo=key, error=str(exc))

    log.info("predict.fuel.done", count=count)
    return count
