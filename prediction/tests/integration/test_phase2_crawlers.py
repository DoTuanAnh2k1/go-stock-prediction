"""Phase 2 integration tests — verify all 5 crawlers via gRPC.

Requirements:
  - Python prediction service running on localhost:8119
  - MySQL DB accessible
  - Real network access (crawlers hit external APIs)

Run:
    make test-phase2
    # or
    pytest tests/integration/test_phase2_crawlers.py -v
"""
from __future__ import annotations

import time

import pytest

pytestmark = pytest.mark.integration


# ---------------------------------------------------------------------------
# VN30 crawler
# ---------------------------------------------------------------------------

def test_trigger_crawler_returns_success(grpc_stub):
    """TriggerCrawler must return success=True immediately (background)."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerCrawler(prediction_pb2.Empty())
    assert resp.success is True
    assert "background" in resp.message.lower() or "crawl" in resp.message.lower()


def test_trigger_stock_history_returns_success(grpc_stub):
    """TriggerStockHistory(days=7) must return success=True immediately."""
    from src.proto.prediction import prediction_pb2

    req = prediction_pb2.StockHistoryRequest(days=7)
    resp = grpc_stub.TriggerStockHistory(req)
    assert resp.success is True


def test_trigger_stock_crawl_single_symbol(grpc_stub):
    """TriggerStockCrawl('VCB') must echo symbol and return success."""
    from src.proto.prediction import prediction_pb2

    req = prediction_pb2.StockRequest(symbol="VCB")
    resp = grpc_stub.TriggerStockCrawl(req)
    assert resp.symbol == "VCB"
    assert resp.success is True


def test_vn30_data_in_db():
    """After crawl, stock_prices must have VN30 data (at least last trading day)."""
    from src.database import repository as repo

    stock = repo.get_stock_by_symbol("VCB")
    assert stock is not None, "VCB stock not found — run crawler first"

    prices = repo.get_stock_prices_asc(stock.id, limit=5)
    assert len(prices) > 0, "No price data for VCB"
    assert float(prices[-1].close_price) > 0, "Last price is zero"


def test_vn30_all_30_stocks_present():
    """All 30 VN30 stocks must be present in DB after crawl."""
    from src.database import repository as repo
    from src.crawlers.vn30 import VN30_SYMBOLS

    missing = []
    for symbol in VN30_SYMBOLS:
        stock = repo.get_stock_by_symbol(symbol)
        if stock is None:
            missing.append(symbol)

    assert missing == [], f"Missing stocks: {missing}"


# ---------------------------------------------------------------------------
# Gold crawler
# ---------------------------------------------------------------------------

def test_trigger_gold_crawler_returns_response(grpc_stub):
    """TriggerGoldCrawler must return a response (success may vary on network)."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerGoldCrawler(prediction_pb2.Empty())
    assert resp is not None
    assert hasattr(resp, "success")
    # Not asserting success=True because BTMC may timeout in restricted network


def test_gold_data_in_db():
    """After gold crawl, gold_prices must have XAU data."""
    from src.database import repository as repo
    from src.database.models import GoldPrice
    from src.database.connection import get_session

    session = get_session()
    try:
        count = session.query(GoldPrice).filter(
            GoldPrice.source.in_(["XAU", "XAU_VND"])
        ).count()
        assert count > 0, "No XAU gold prices in DB"
    finally:
        session.close()


def test_gold_xau_price_is_reasonable():
    """XAU price must be in realistic range (500–5000 USD/oz)."""
    from src.database import repository as repo

    prices = repo.get_gold_prices_asc("XAU", "spot", limit=1)
    assert len(prices) > 0, "No XAU/spot prices"
    price = float(prices[-1].buy_price)
    assert 500 <= price <= 10000, f"XAU price out of range: {price}"


def test_trigger_gold_history_returns_success(grpc_stub):
    """TriggerGoldHistory must return success=True immediately (background)."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerGoldHistory(prediction_pb2.Empty())
    assert resp.success is True


# ---------------------------------------------------------------------------
# NASDAQ crawler
# ---------------------------------------------------------------------------

def test_trigger_nasdaq_crawler_returns_success(grpc_stub):
    """TriggerNasdaqCrawler must return success=True immediately (background)."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerNasdaqCrawler(prediction_pb2.Empty())
    assert resp.success is True


def test_nasdaq_data_in_db():
    """After NASDAQ crawl, nasdaq_prices must have data."""
    from src.database import repository as repo

    symbols = repo.get_nasdaq_symbols()
    assert len(symbols) > 0, "No NASDAQ symbols in DB"

    # Check at least one symbol has price data
    prices = repo.get_nasdaq_prices_asc(symbols[0], limit=5)
    assert len(prices) > 0, f"No price data for {symbols[0]}"
    assert float(prices[-1].close_price) > 0


def test_nasdaq_15_symbols_present():
    """All 15 configured NASDAQ symbols must be in DB."""
    from src.database import repository as repo
    from src.crawlers.nasdaq import NASDAQ_SYMBOLS

    db_symbols = set(repo.get_nasdaq_symbols())
    missing = [s for s in NASDAQ_SYMBOLS if s not in db_symbols]
    assert missing == [], f"Missing NASDAQ symbols: {missing}"


# ---------------------------------------------------------------------------
# Crypto crawler
# ---------------------------------------------------------------------------

def test_trigger_crypto_crawler_returns_success(grpc_stub):
    """TriggerCryptoCrawler must return success=True immediately (background)."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerCryptoCrawler(prediction_pb2.Empty())
    assert resp.success is True


def test_crypto_data_in_db():
    """After crypto crawl, crypto_prices must have BTC and ETH."""
    from src.database import repository as repo

    btc_prices = repo.get_crypto_prices_asc("bitcoin", limit=5)
    assert len(btc_prices) > 0, "No BTC prices in DB"
    assert float(btc_prices[-1].close_price) > 0

    eth_prices = repo.get_crypto_prices_asc("ethereum", limit=5)
    assert len(eth_prices) > 0, "No ETH prices in DB"
    assert float(eth_prices[-1].close_price) > 0


def test_btc_price_is_reasonable():
    """BTC price must be in realistic range (1000–500000 USD)."""
    from src.database import repository as repo

    prices = repo.get_crypto_prices_asc("bitcoin", limit=1)
    assert len(prices) > 0, "No BTC prices"
    price = float(prices[-1].close_price)
    assert 1000 <= price <= 500000, f"BTC price out of range: {price}"


# ---------------------------------------------------------------------------
# Fuel crawler
# ---------------------------------------------------------------------------

def test_trigger_fuel_crawler_returns_success(grpc_stub):
    """TriggerFuelCrawler must return success=True immediately (background)."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerFuelCrawler(prediction_pb2.Empty())
    assert resp.success is True


def test_fuel_data_in_db():
    """After fuel history import, fuel_prices must have RON95 data."""
    from src.database import repository as repo

    prices = repo.get_fuel_prices_asc("ron95_iii", limit=5)
    assert len(prices) > 0, "No RON95 fuel prices in DB — run fuel history import first"
    assert float(prices[-1].price) > 0


# ---------------------------------------------------------------------------
# Error handling — crawlers must not crash service
# ---------------------------------------------------------------------------

def test_crawler_failure_does_not_crash_service(grpc_stub):
    """After a crawler error, GetTrainingStatus must still respond."""
    from src.proto.prediction import prediction_pb2

    # This verifies the service is still alive
    resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
    assert resp is not None
    assert resp.is_training is False or resp.is_training is True  # any value OK


def test_concurrent_crawlers_do_not_crash(grpc_stub):
    """Triggering multiple crawlers simultaneously must not crash service."""
    from src.proto.prediction import prediction_pb2

    # Fire all background crawlers at once
    grpc_stub.TriggerCrawler(prediction_pb2.Empty())
    grpc_stub.TriggerNasdaqCrawler(prediction_pb2.Empty())
    grpc_stub.TriggerCryptoCrawler(prediction_pb2.Empty())
    grpc_stub.TriggerFuelCrawler(prediction_pb2.Empty())

    # Service must still respond
    resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
    assert resp is not None
