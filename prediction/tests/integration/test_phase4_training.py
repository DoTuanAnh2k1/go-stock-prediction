"""Phase 4 integration tests — training pipeline, reconcile, and historical backtest.

Tests TriggerTrain (all + single), GetTrainingStatus, TriggerReconcile, and
TriggerHistoricalBacktest via gRPC directly.

Run:
    make test-phase4
    # or
    docker exec prediction_service python -m pytest tests/integration/test_phase4_training.py -v
"""
from __future__ import annotations

import time

import pytest

pytestmark = pytest.mark.integration


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _wait_until_idle(grpc_stub, max_wait: int = 60) -> None:
    """Wait until no training is in progress (up to max_wait seconds)."""
    from src.proto.prediction import prediction_pb2

    for _ in range(max_wait // 2):
        status = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
        if not status.is_training:
            return
        time.sleep(2)


# ---------------------------------------------------------------------------
# GetTrainingStatus — field contracts
# ---------------------------------------------------------------------------

def test_get_training_status_has_required_fields(grpc_stub):
    """GetTrainingStatus must return all 6 required fields."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
    assert resp is not None

    # 1. is_training — bool
    assert resp.is_training is True or resp.is_training is False

    # 2. last_trained — string (may be empty if never trained)
    assert hasattr(resp, "last_trained")
    assert isinstance(resp.last_trained, str)

    # 3. progress — numeric (protobuf float/double)
    assert hasattr(resp, "progress")

    # 4. current_phase — string
    assert hasattr(resp, "current_phase")
    assert isinstance(resp.current_phase, str)

    # 5. total_algorithms — int
    assert hasattr(resp, "total_algorithms")

    # 6. done_algorithms — int
    assert hasattr(resp, "done_algorithms")


def test_get_training_status_total_algorithms_at_least_6(grpc_stub):
    """total_algorithms must be >= 6 (MA, EMA, LSTM, ARIMA, LightGBM, Ensemble)."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
    assert resp.total_algorithms >= 6


def test_get_training_status_progress_range(grpc_stub):
    """progress must be in [0, 100] at any point."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
    assert 0.0 <= resp.progress <= 100.0


# ---------------------------------------------------------------------------
# TriggerTrain — all algorithms (background)
# ---------------------------------------------------------------------------

def test_trigger_train_all_returns_success(grpc_stub):
    """TriggerTrain() with no algorithm must return success=True or report in-progress.

    Wait for any previous training to finish before triggering.
    """
    from src.proto.prediction import prediction_pb2

    _wait_until_idle(grpc_stub, max_wait=60)

    resp = grpc_stub.TriggerTrain(prediction_pb2.TriggerTrainRequest())
    assert resp is not None

    # Either training was kicked off successfully, or another was already in-progress.
    # Both are valid outcomes.
    assert resp.success is True or "already in progress" in (resp.error or "").lower(), (
        f"Unexpected failure: success={resp.success}, error={resp.error!r}"
    )


def test_trigger_train_all_has_session_id(grpc_stub):
    """If TriggerTrain() succeeds, session_id must be non-empty."""
    from src.proto.prediction import prediction_pb2

    _wait_until_idle(grpc_stub, max_wait=60)

    resp = grpc_stub.TriggerTrain(prediction_pb2.TriggerTrainRequest())
    if resp.success:
        assert resp.session_id != "", "Expected non-empty session_id when success=True"


# ---------------------------------------------------------------------------
# TriggerTrain — single algorithm
# ---------------------------------------------------------------------------

def test_trigger_train_single_moving_average(grpc_stub):
    """TriggerTrain(algorithm='moving_average') must return success=True or in-progress."""
    from src.proto.prediction import prediction_pb2

    _wait_until_idle(grpc_stub, max_wait=60)

    resp = grpc_stub.TriggerTrain(
        prediction_pb2.TriggerTrainRequest(algorithm="moving_average")
    )
    assert resp is not None

    # Accept: success or already-in-progress guard
    assert resp.success is True or "already in progress" in (resp.error or "").lower(), (
        f"Unexpected failure: success={resp.success}, error={resp.error!r}"
    )


def test_trigger_train_unknown_algorithm_returns_error(grpc_stub):
    """TriggerTrain with an unknown algorithm must return success=False with an error."""
    from src.proto.prediction import prediction_pb2

    _wait_until_idle(grpc_stub, max_wait=60)

    resp = grpc_stub.TriggerTrain(
        prediction_pb2.TriggerTrainRequest(algorithm="nonexistent_algo_xyz_999")
    )
    assert resp is not None
    assert resp.success is False
    assert resp.error != "", "Expected non-empty error for unknown algorithm"


# ---------------------------------------------------------------------------
# TriggerReconcile
# ---------------------------------------------------------------------------

def test_trigger_reconcile_returns_success(grpc_stub):
    """TriggerReconcile must return success=True."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerReconcile(prediction_pb2.Empty())
    assert resp is not None
    assert resp.success is True, f"Reconcile failed: {resp.error!r}"


def test_trigger_reconcile_message_mentions_count(grpc_stub):
    """Reconcile message must mention 'reconcile' or 'prediction'."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerReconcile(prediction_pb2.Empty())
    assert resp.success is True
    msg = (resp.message or "").lower()
    assert "reconcile" in msg or "prediction" in msg, (
        f"Unexpected message: {resp.message!r}"
    )


def test_trigger_reconcile_idempotent(grpc_stub):
    """Three consecutive reconcile calls must all succeed."""
    from src.proto.prediction import prediction_pb2

    for i in range(3):
        resp = grpc_stub.TriggerReconcile(prediction_pb2.Empty())
        assert resp.success is True, f"Reconcile call {i+1} failed: {resp.error!r}"


# ---------------------------------------------------------------------------
# TriggerHistoricalBacktest
# ---------------------------------------------------------------------------

def test_trigger_historical_backtest_gold_success(grpc_stub):
    """TriggerHistoricalBacktest with GOLD market must return success=True.

    If a backtest is already running (from a previous test), that is acceptable.
    """
    from src.proto.prediction import prediction_pb2

    req = prediction_pb2.BacktestRequest(
        train_window=30,
        step_size=6,
        market_key="GOLD",
    )
    resp = grpc_stub.TriggerHistoricalBacktest(req)
    assert resp is not None

    # Either started successfully, or concurrency guard kicked in
    assert resp.success is True or "already running" in (resp.error or "").lower(), (
        f"Unexpected failure: success={resp.success}, error={resp.error!r}"
    )


def test_trigger_historical_backtest_concurrent_guard(grpc_stub):
    """Calling TriggerHistoricalBacktest twice quickly should not crash the service.

    The second call may return 'already running' or succeed if the first
    finished immediately (empty DB — nothing to backtest).
    Both outcomes are valid.
    """
    from src.proto.prediction import prediction_pb2

    req = prediction_pb2.BacktestRequest(
        train_window=30,
        step_size=6,
        market_key="GOLD",
    )

    resp1 = grpc_stub.TriggerHistoricalBacktest(req)
    assert resp1 is not None

    # Fire second call immediately
    resp2 = grpc_stub.TriggerHistoricalBacktest(req)
    assert resp2 is not None

    # At least one must respond without crashing
    # If both succeed — empty DB, backtest finished instantly before second call
    # If second fails with "already running" — concurrency guard worked
    if resp1.success and not resp2.success:
        assert "already running" in (resp2.error or "").lower(), (
            f"Second call failed with unexpected error: {resp2.error!r}"
        )


def test_trigger_historical_backtest_gold_market(grpc_stub):
    """TriggerHistoricalBacktest with GOLD market must return success=True or already running."""
    from src.proto.prediction import prediction_pb2

    # Allow previous backtest to finish (up to 10s)
    time.sleep(2)

    req = prediction_pb2.BacktestRequest(
        train_window=30,
        step_size=6,
        market_key="GOLD",
    )
    resp = grpc_stub.TriggerHistoricalBacktest(req)
    assert resp is not None
    assert resp.success is True or "already running" in (resp.error or "").lower(), (
        f"Unexpected failure: success={resp.success}, error={resp.error!r}"
    )


# ---------------------------------------------------------------------------
# Service resilience
# ---------------------------------------------------------------------------

def test_service_alive_after_all_operations(grpc_stub):
    """After all training/reconcile/backtest calls, GetTrainingStatus must still respond."""
    from src.proto.prediction import prediction_pb2

    # Fire a few ops
    grpc_stub.TriggerReconcile(prediction_pb2.Empty())
    grpc_stub.TriggerHistoricalBacktest(
        prediction_pb2.BacktestRequest(train_window=30, step_size=6, market_key="GOLD")
    )

    # Service must still respond
    resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
    assert resp is not None
    assert resp.is_training is True or resp.is_training is False
