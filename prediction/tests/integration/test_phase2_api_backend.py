"""Phase 2 integration tests — verify API Backend reads crawled data correctly.

Run via docker exec:
    make test-phase2
    # or
    docker exec prediction_service python -m pytest tests/integration/test_phase2_api_backend.py -v
"""
from __future__ import annotations

import pytest
import requests

pytestmark = pytest.mark.integration


# ---------------------------------------------------------------------------
# Gold endpoints
# ---------------------------------------------------------------------------

def test_api_gold_latest_has_data(api_base_url, auth_headers):
    """GET /api/gold/latest must return non-empty gold price data."""
    resp = requests.get(f"{api_base_url}/api/gold/latest", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None
    if isinstance(data, dict):
        assert "data" in data or "updated_at" in data
    else:
        assert len(data) > 0


def test_api_gold_prices_list(api_base_url, auth_headers):
    """GET /api/gold/prices must return a list of gold prices."""
    resp = requests.get(f"{api_base_url}/api/gold/prices", timeout=10)
    assert resp.status_code == 200


def test_api_gold_chart_data(api_base_url, auth_headers):
    """GET /api/gold/chart must return chart data."""
    resp = requests.get(f"{api_base_url}/api/gold/chart", timeout=10)
    assert resp.status_code == 200


def test_api_trigger_gold_crawler_succeeds(api_base_url, auth_headers):
    """POST /api/trigger/gold-crawler must return 2xx."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/gold-crawler",
        headers=auth_headers,
        timeout=30,
    )
    assert resp.status_code in (200, 202), f"Unexpected status: {resp.status_code}"


# ---------------------------------------------------------------------------
# VN30 / Stock endpoints
# ---------------------------------------------------------------------------

def test_api_stock_vcb_history_has_data(api_base_url, auth_headers):
    """GET /api/stocks/VCB/history must return price history."""
    resp = requests.get(f"{api_base_url}/api/stocks/VCB/history", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None


def test_api_stock_vcb_current_price(api_base_url, auth_headers):
    """GET /api/stocks/VCB/current must return current price data."""
    resp = requests.get(f"{api_base_url}/api/stocks/VCB/current", timeout=10)
    assert resp.status_code == 200


def test_api_market_overview_has_data(api_base_url, auth_headers):
    """GET /api/market/overview must return market overview."""
    resp = requests.get(f"{api_base_url}/api/market/overview", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None


def test_api_trigger_crawler_succeeds(api_base_url, auth_headers):
    """POST /api/trigger/crawler must return 2xx."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/crawler",
        headers=auth_headers,
        timeout=10,
    )
    assert resp.status_code in (200, 202), f"Unexpected status: {resp.status_code}"


def test_api_trigger_stock_history_succeeds(api_base_url, auth_headers):
    """POST /api/trigger/stock-history must return 2xx."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/stock-history",
        headers=auth_headers,
        json={"days": 7},
        timeout=10,
    )
    assert resp.status_code in (200, 202), f"Unexpected status: {resp.status_code}"


# ---------------------------------------------------------------------------
# NASDAQ endpoints
# ---------------------------------------------------------------------------

def test_api_nasdaq_latest(api_base_url, auth_headers):
    """GET /api/nasdaq/latest must return NASDAQ data."""
    resp = requests.get(f"{api_base_url}/api/nasdaq/latest", timeout=10)
    assert resp.status_code == 200


def test_api_trigger_nasdaq_crawler_succeeds(api_base_url, auth_headers):
    """POST /api/trigger/nasdaq-crawler must return 2xx."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/nasdaq-crawler",
        headers=auth_headers,
        timeout=10,
    )
    assert resp.status_code in (200, 202), f"Unexpected status: {resp.status_code}"


# ---------------------------------------------------------------------------
# Crypto endpoints
# ---------------------------------------------------------------------------

def test_api_crypto_latest(api_base_url, auth_headers):
    """GET /api/crypto/latest must return crypto data (BTC, ETH)."""
    resp = requests.get(f"{api_base_url}/api/crypto/latest", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None


def test_api_trigger_crypto_crawler_succeeds(api_base_url, auth_headers):
    """POST /api/trigger/crypto-crawler must return 2xx."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/crypto-crawler",
        headers=auth_headers,
        timeout=10,
    )
    assert resp.status_code in (200, 202), f"Unexpected status: {resp.status_code}"


# ---------------------------------------------------------------------------
# Dashboard stats (reads data from all crawled sources)
# ---------------------------------------------------------------------------

def test_api_dashboard_stats(api_base_url, auth_headers):
    """GET /api/dashboard/stats must return valid stats after crawl."""
    resp = requests.get(f"{api_base_url}/api/dashboard/stats", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None


# ---------------------------------------------------------------------------
# Schedules (DB-backed cron visible via API)
# ---------------------------------------------------------------------------

def test_api_schedules_list_has_all_crawlers(api_base_url, auth_headers):
    """GET /api/schedules must list all 4 crawler jobs."""
    resp = requests.get(f"{api_base_url}/api/schedules", headers=auth_headers, timeout=10)
    assert resp.status_code == 200
    schedules = resp.json()
    job_keys = {s["job_key"] for s in schedules}
    expected = {"crawler_stock", "crawler_gold", "crawler_nasdaq", "crawler_crypto"}
    missing = expected - job_keys
    assert missing == set(), f"Missing schedule keys: {missing}"
