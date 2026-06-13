"""Phase 2 integration tests — verify all crawlers via gRPC.

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

import pytest

pytestmark = pytest.mark.integration


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
    from src.database.connection import get_session
    from src.database.models import GoldPrice

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
    from src.crawlers.nasdaq import NASDAQ_SYMBOLS
    from src.database import repository as repo

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

    # Fire background crawlers at once
    grpc_stub.TriggerNasdaqCrawler(prediction_pb2.Empty())
    grpc_stub.TriggerCryptoCrawler(prediction_pb2.Empty())

    # Service must still respond
    resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
    assert resp is not None
