"""Phase 4 integration tests — API Backend training, reconcile, backtest, schedules.

Tests the HTTP API endpoints exposed by the Go API Backend that proxy to the
Python prediction service via gRPC.

Run:
    make test-phase4
    # or
    docker exec prediction_service python -m pytest tests/integration/test_phase4_api_backend.py -v
"""
from __future__ import annotations

import pytest
import requests

pytestmark = pytest.mark.integration


# ---------------------------------------------------------------------------
# GET /api/training/status
# ---------------------------------------------------------------------------

def test_api_training_status_returns_200(api_base_url):
    """GET /api/training/status must return 200 with a JSON body."""
    resp = requests.get(f"{api_base_url}/api/training/status", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None


def test_api_training_status_has_is_training(api_base_url):
    """GET /api/training/status must include is_training field."""
    resp = requests.get(f"{api_base_url}/api/training/status", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert "is_training" in data


def test_api_training_status_is_training_is_bool(api_base_url):
    """is_training field must be a boolean (not string, not int)."""
    resp = requests.get(f"{api_base_url}/api/training/status", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert isinstance(data["is_training"], bool), (
        f"Expected bool, got {type(data['is_training']).__name__}: {data['is_training']!r}"
    )


def test_api_training_status_has_progress(api_base_url):
    """GET /api/training/status must include progress field."""
    resp = requests.get(f"{api_base_url}/api/training/status", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    # Go handler returns: is_training, last_trained, next_training, current_phase, progress
    assert "progress" in data or "current_phase" in data, (
        f"Expected progress or current_phase in response, got: {list(data.keys())}"
    )


# ---------------------------------------------------------------------------
# POST /api/trigger/train
# ---------------------------------------------------------------------------

def test_api_trigger_train_returns_202(api_base_url, auth_headers):
    """POST /api/trigger/train (train all) must return 202 Accepted or 409 if already running."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/train",
        headers=auth_headers,
        timeout=30,
    )
    assert resp.status_code in (202, 409), (
        f"Unexpected status {resp.status_code}: {resp.text[:300]}"
    )


def test_api_trigger_train_single_algorithm(api_base_url, auth_headers):
    """POST /api/trigger/train with body {"algorithm": "moving_average"} must return 202 or 409."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/train",
        headers=auth_headers,
        json={"algorithm": "moving_average"},
        timeout=30,
    )
    assert resp.status_code in (202, 409), (
        f"Unexpected status {resp.status_code}: {resp.text[:300]}"
    )


def test_api_trigger_train_without_auth_returns_401(api_base_url):
    """POST /api/trigger/train without JWT must return 401."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/train",
        timeout=10,
    )
    assert resp.status_code == 401


# ---------------------------------------------------------------------------
# POST /api/trigger/reconcile
# ---------------------------------------------------------------------------

def test_api_trigger_reconcile_returns_200(api_base_url, auth_headers):
    """POST /api/trigger/reconcile must return 200."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/reconcile",
        headers=auth_headers,
        timeout=30,
    )
    assert resp.status_code == 200, (
        f"Unexpected status {resp.status_code}: {resp.text[:300]}"
    )


def test_api_trigger_reconcile_without_auth_returns_401(api_base_url):
    """POST /api/trigger/reconcile without JWT must return 401."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/reconcile",
        timeout=10,
    )
    assert resp.status_code == 401


# ---------------------------------------------------------------------------
# POST /api/trigger/historical-backtest
# ---------------------------------------------------------------------------

def test_api_trigger_historical_backtest_returns_202_or_409(api_base_url, auth_headers):
    """POST /api/trigger/historical-backtest must return 202 (started) or 409 (already running)."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/historical-backtest?train_window=30&step_size=6",
        headers=auth_headers,
        timeout=30,
    )
    assert resp.status_code in (202, 409), (
        f"Unexpected status {resp.status_code}: {resp.text[:300]}"
    )


def test_api_trigger_historical_backtest_without_auth_returns_401(api_base_url):
    """POST /api/trigger/historical-backtest without JWT must return 401."""
    resp = requests.post(
        f"{api_base_url}/api/trigger/historical-backtest?train_window=30&step_size=6",
        timeout=10,
    )
    assert resp.status_code == 401


# ---------------------------------------------------------------------------
# GET /api/schedules
# ---------------------------------------------------------------------------

def test_api_schedules_list_returns_200(api_base_url, auth_headers):
    """GET /api/schedules must return 200 with a non-empty list."""
    resp = requests.get(
        f"{api_base_url}/api/schedules",
        headers=auth_headers,
        timeout=10,
    )
    assert resp.status_code == 200
    data = resp.json()
    assert isinstance(data, list), f"Expected list, got {type(data).__name__}"
    assert len(data) >= 8, (
        f"Expected >= 8 schedules (5 crawlers + 3 predict), got {len(data)}"
    )


def test_api_schedules_have_required_fields(api_base_url, auth_headers):
    """Each schedule entry must have job_key, cron_expression, and enabled fields."""
    resp = requests.get(
        f"{api_base_url}/api/schedules",
        headers=auth_headers,
        timeout=10,
    )
    assert resp.status_code == 200
    schedules = resp.json()
    assert len(schedules) > 0

    for sched in schedules:
        assert "job_key" in sched, f"Missing job_key in: {sched}"
        assert "cron_expression" in sched, f"Missing cron_expression in: {sched}"
        assert "enabled" in sched, f"Missing enabled in: {sched}"


def test_api_schedules_update_returns_200(api_base_url, auth_headers):
    """PUT /api/schedules/{key} must return 200 when updating with a valid cron expression."""
    # First get the list to find a valid key
    list_resp = requests.get(
        f"{api_base_url}/api/schedules",
        headers=auth_headers,
        timeout=10,
    )
    assert list_resp.status_code == 200
    schedules = list_resp.json()
    assert len(schedules) > 0

    first_key = schedules[0]["job_key"]
    original_cron = schedules[0]["cron_expression"]
    original_enabled = schedules[0]["enabled"]

    # Update with same values (no-op update — safe to run in tests)
    update_resp = requests.put(
        f"{api_base_url}/api/schedules/{first_key}",
        headers=auth_headers,
        json={"cron_expression": original_cron, "enabled": original_enabled},
        timeout=10,
    )
    assert update_resp.status_code == 200, (
        f"PUT /api/schedules/{first_key} returned {update_resp.status_code}: {update_resp.text[:300]}"
    )


def test_api_schedules_without_auth_returns_401(api_base_url):
    """GET /api/schedules without JWT must return 401."""
    resp = requests.get(
        f"{api_base_url}/api/schedules",
        timeout=10,
    )
    assert resp.status_code == 401


# ---------------------------------------------------------------------------
# GET /api/training/history
# ---------------------------------------------------------------------------

def test_api_training_history_returns_200(api_base_url):
    """GET /api/training/history must return 200."""
    resp = requests.get(f"{api_base_url}/api/training/history", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None


# ---------------------------------------------------------------------------
# GET /api/training/metrics
# ---------------------------------------------------------------------------

def test_api_training_metrics_returns_200(api_base_url):
    """GET /api/training/metrics must return 200."""
    resp = requests.get(f"{api_base_url}/api/training/metrics", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data is not None
