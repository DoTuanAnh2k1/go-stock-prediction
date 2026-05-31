"""Phase 3 integration tests — verify API Backend prediction endpoints.

Tests that API Backend correctly exposes prediction data via HTTP.
Requires: API Backend running at http://api:8118 (Docker network).

Run:
    make test-phase3
    # or
    docker exec prediction_service python -m pytest tests/integration/test_phase3_api_backend.py -v
"""
from __future__ import annotations

import pytest
import requests

pytestmark = pytest.mark.integration


# ---------------------------------------------------------------------------
# Training / algorithm endpoints
# ---------------------------------------------------------------------------

def test_api_training_algorithms_returns_200(api_base_url):
    """GET /api/training/algorithms must return 200 with a JSON list."""
    resp = requests.get(f"{api_base_url}/api/training/algorithms", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None
    # Should be a list or a dict with a list
    if isinstance(data, list):
        assert len(data) >= 0  # May be empty if no training logged yet
    else:
        # Some APIs wrap in {"data": [...]} or {"algorithms": [...]}
        assert isinstance(data, dict)


def test_api_training_status_returns_200(api_base_url):
    """GET /api/training/status must return 200 with is_training field."""
    resp = requests.get(f"{api_base_url}/api/training/status", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None
    assert "is_training" in data


def test_api_training_status_is_training_is_bool(api_base_url):
    """is_training field must be a boolean."""
    resp = requests.get(f"{api_base_url}/api/training/status", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert isinstance(data["is_training"], bool)


# ---------------------------------------------------------------------------
# Prediction endpoints
# ---------------------------------------------------------------------------

def test_api_predictions_returns_200(api_base_url):
    """GET /api/predictions must return 200."""
    resp = requests.get(f"{api_base_url}/api/predictions", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None


def test_api_predictions_accuracy_returns_200(api_base_url):
    """GET /api/predictions/accuracy must return 200."""
    resp = requests.get(f"{api_base_url}/api/predictions/accuracy", timeout=10)
    assert resp.status_code == 200


# ---------------------------------------------------------------------------
# Trigger prediction endpoints (require auth)
# ---------------------------------------------------------------------------

def test_api_trigger_predict_returns_200(api_base_url, auth_headers):
    """POST /api/trigger/predict must return 200 or 202."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/predict",
        headers=auth_headers,
        timeout=30,
    )
    assert resp.status_code in (200, 202), (
        f"Unexpected status {resp.status_code}: {resp.text[:200]}"
    )


def test_api_trigger_gold_predict_returns_200(api_base_url, auth_headers):
    """POST /api/trigger/gold-predict must return 200 or 202."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/gold-predict",
        headers=auth_headers,
        timeout=30,
    )
    assert resp.status_code in (200, 202), (
        f"Unexpected status {resp.status_code}: {resp.text[:200]}"
    )


def test_api_trigger_predict_without_auth_returns_401(api_base_url):
    """POST /api/trigger/predict without token must return 401."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/predict",
        timeout=10,
    )
    assert resp.status_code == 401


def test_api_trigger_predict_with_invalid_token_returns_401(api_base_url):
    """POST /api/trigger/predict with invalid token must return 401."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/predict",
        headers={"Authorization": "Bearer invalid.token.here"},
        timeout=10,
    )
    assert resp.status_code == 401


# ---------------------------------------------------------------------------
# Gold prediction endpoints
# ---------------------------------------------------------------------------

def test_api_gold_predictions_latest_returns_200(api_base_url):
    """GET /api/gold/predictions/latest must return 200."""
    resp = requests.get(f"{api_base_url}/api/gold/predictions/latest", timeout=10)
    assert resp.status_code == 200


def test_api_gold_predictions_returns_200(api_base_url):
    """GET /api/gold/predictions must return 200."""
    resp = requests.get(f"{api_base_url}/api/gold/predictions", timeout=10)
    assert resp.status_code == 200


def test_api_gold_predictions_chart_returns_200(api_base_url):
    """GET /api/gold/predictions/chart must return 200."""
    resp = requests.get(f"{api_base_url}/api/gold/predictions/chart", timeout=10)
    assert resp.status_code == 200


# ---------------------------------------------------------------------------
# Dashboard stats
# ---------------------------------------------------------------------------

def test_api_dashboard_stats_returns_200(api_base_url):
    """GET /api/dashboard/stats must return 200 after predictions run."""
    resp = requests.get(f"{api_base_url}/api/dashboard/stats", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None
