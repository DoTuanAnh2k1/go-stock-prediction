"""Phase 1 end-to-end integration tests — Go API Backend → gRPC → Python service.

These tests send HTTP requests to the Go API Backend (localhost:8118) which
internally calls the Python prediction service via gRPC (localhost:8119).
They verify the full request chain is operational after replacing the Go
prediction service with the Python one.

Requirements:
  - Go API Backend running on localhost:8118
  - Python prediction service running on localhost:8119
  - MySQL DB accessible and seeded

Run:
    make test-phase1
    # or
    pytest tests/integration/test_phase1_api_backend.py -v
"""
from __future__ import annotations

import pytest
import requests

pytestmark = pytest.mark.integration


# ---------------------------------------------------------------------------
# Auth
# ---------------------------------------------------------------------------

def test_api_auth_login(api_base_url):
    """POST /api/auth/login returns a JWT token."""
    resp = requests.post(
        f"{api_base_url}/api/auth/login",
        json={"username": "admin", "password": "admin123"},
        timeout=10,
    )
    assert resp.status_code == 200
    data = resp.json()
    assert "token" in data
    assert data["token"] != ""


# ---------------------------------------------------------------------------
# Training status (calls gRPC GetTrainingStatus)
# ---------------------------------------------------------------------------

def test_api_training_status(api_base_url, auth_headers):
    """GET /api/training/status returns 200 with is_training field."""
    resp = requests.get(f"{api_base_url}/api/training/status", headers=auth_headers, timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert "is_training" in data


# ---------------------------------------------------------------------------
# Trigger endpoints (all require JWT)
# ---------------------------------------------------------------------------

def test_api_trigger_crawler(api_base_url, auth_headers):
    """POST /api/trigger/crawler returns 200."""
    resp = requests.post(f"{api_base_url}/api/trigger/crawler", headers=auth_headers, timeout=15)
    assert resp.status_code == 200


def test_api_trigger_predict(api_base_url, auth_headers):
    """POST /api/trigger/predict returns 200."""
    resp = requests.post(f"{api_base_url}/api/trigger/predict", headers=auth_headers, timeout=30)
    assert resp.status_code == 200


def test_api_trigger_gold_crawler(api_base_url, auth_headers):
    """POST /api/trigger/gold-crawler returns 200."""
    resp = requests.post(f"{api_base_url}/api/trigger/gold-crawler", headers=auth_headers, timeout=30)
    assert resp.status_code == 200


def test_api_trigger_reconcile(api_base_url, auth_headers):
    """POST /api/trigger/reconcile returns 200."""
    resp = requests.post(f"{api_base_url}/api/trigger/reconcile", headers=auth_headers, timeout=30)
    assert resp.status_code == 200


def test_api_trigger_gold_predict(api_base_url, auth_headers):
    """POST /api/trigger/gold-predict returns 200."""
    resp = requests.post(f"{api_base_url}/api/trigger/gold-predict", headers=auth_headers, timeout=15)
    assert resp.status_code == 200


def test_api_trigger_nasdaq_crawler(api_base_url, auth_headers):
    """POST /api/trigger/nasdaq-crawler returns 200."""
    resp = requests.post(f"{api_base_url}/api/trigger/nasdaq-crawler", headers=auth_headers, timeout=15)
    assert resp.status_code == 200


def test_api_trigger_crypto_crawler(api_base_url, auth_headers):
    """POST /api/trigger/crypto-crawler returns 200."""
    resp = requests.post(f"{api_base_url}/api/trigger/crypto-crawler", headers=auth_headers, timeout=15)
    assert resp.status_code == 200


def test_api_trigger_fuel_crawler(api_base_url, auth_headers):
    """POST /api/trigger/fuel-crawler returns 200."""
    resp = requests.post(f"{api_base_url}/api/trigger/fuel-crawler", headers=auth_headers, timeout=15)
    assert resp.status_code == 200


def test_api_trigger_train(api_base_url, auth_headers):
    """POST /api/trigger/train with JSON body returns 200."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/train",
        headers=auth_headers,
        json={"algorithm": "moving_average"},
        timeout=60,
    )
    assert resp.status_code == 200


def test_api_trigger_stock_history(api_base_url, auth_headers):
    """POST /api/trigger/stock-history with body days=30 returns 200."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/stock-history",
        headers=auth_headers,
        json={"days": 30},
        timeout=15,
    )
    assert resp.status_code == 200


def test_api_trigger_historical_backtest(api_base_url, auth_headers):
    """POST /api/trigger/historical-backtest returns 202 Accepted."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/historical-backtest?train_window=30&step_size=6",
        headers=auth_headers,
        timeout=15,
    )
    # 202 = accepted (background task started); 409 = already running (also valid)
    assert resp.status_code in (200, 202, 409)


def test_api_trigger_stock_crawl(api_base_url):
    """POST /api/stocks/VCB/crawl returns 200 (public endpoint, no auth needed)."""
    resp = requests.post(f"{api_base_url}/api/stocks/VCB/crawl", timeout=30)
    assert resp.status_code == 200


# ---------------------------------------------------------------------------
# Schedules
# ---------------------------------------------------------------------------

def test_api_schedules_list(api_base_url, auth_headers):
    """GET /api/schedules returns list of >= 5 cron schedule items."""
    resp = requests.get(f"{api_base_url}/api/schedules", headers=auth_headers, timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert isinstance(data, list)
    assert len(data) >= 5
    # Verify structure of first item
    if data:
        item = data[0]
        assert "job_key" in item
        assert "cron_expression" in item
        assert "enabled" in item
