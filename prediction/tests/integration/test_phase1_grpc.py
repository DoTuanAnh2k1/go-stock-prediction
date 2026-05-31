"""Phase 1 integration tests — verify all 19 gRPC RPCs return valid responses.

These tests require the Python prediction service to be running on localhost:8119
with proto stubs generated (run `make proto` first).

Run:
    make test-phase1
    # or
    pytest tests/integration/test_phase1_grpc.py -v
"""
from __future__ import annotations

import pytest

# All tests in this module are integration tests that need a live gRPC server.
pytestmark = pytest.mark.integration


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _empty(pb2):
    return pb2.Empty()


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

def test_server_starts_and_responds(grpc_stub):
    """GetTrainingStatus must return a response with no exception."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
    assert resp is not None


def test_get_training_status(grpc_stub):
    """GetTrainingStatus returns correct schema with is_training=False."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
    assert resp.is_training is False
    assert resp.total_algorithms >= 5  # MA + EMA + LSTM + ARIMA + LightGBM + Ensemble


def test_trigger_crawler(grpc_stub):
    """TriggerCrawler acknowledges the request."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerCrawler(prediction_pb2.Empty())
    assert resp.success is True


def test_trigger_gold_crawler(grpc_stub):
    """TriggerGoldCrawler returns success (may fail crawl but not RPC)."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerGoldCrawler(prediction_pb2.Empty())
    # success can be False if the external site is unreachable, but RPC must return
    assert resp is not None
    assert hasattr(resp, "success")
    assert hasattr(resp, "message")


def test_trigger_predict(grpc_stub):
    """TriggerPredict returns a TriggerResponse."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerPredict(prediction_pb2.Empty())
    assert resp is not None
    assert hasattr(resp, "success")


def test_trigger_nasdaq_crawler(grpc_stub):
    """TriggerNasdaqCrawler acknowledges the request."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerNasdaqCrawler(prediction_pb2.Empty())
    assert resp.success is True


def test_trigger_crypto_crawler(grpc_stub):
    """TriggerCryptoCrawler acknowledges the request."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerCryptoCrawler(prediction_pb2.Empty())
    assert resp.success is True


def test_trigger_fuel_crawler(grpc_stub):
    """TriggerFuelCrawler acknowledges the request."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerFuelCrawler(prediction_pb2.Empty())
    assert resp.success is True


def test_trigger_train_all(grpc_stub):
    """TriggerTrain with empty algorithm trains all — returns non-empty session_id."""
    from src.proto.prediction import prediction_pb2

    req = prediction_pb2.TriggerTrainRequest(algorithm="")
    resp = grpc_stub.TriggerTrain(req)
    assert resp.success is True
    assert resp.session_id != ""


def test_trigger_train_single(grpc_stub):
    """TriggerTrain with a specific algorithm name succeeds."""
    from src.proto.prediction import prediction_pb2

    req = prediction_pb2.TriggerTrainRequest(algorithm="moving_average")
    resp = grpc_stub.TriggerTrain(req)
    assert resp.success is True


def test_trigger_stock_crawl(grpc_stub):
    """TriggerStockCrawl echoes the symbol in the response."""
    from src.proto.prediction import prediction_pb2

    req = prediction_pb2.StockRequest(symbol="VCB")
    resp = grpc_stub.TriggerStockCrawl(req)
    assert resp.symbol == "VCB"
    assert hasattr(resp, "success")


def test_trigger_stock_predict(grpc_stub):
    """TriggerStockPredict echoes the symbol in the response."""
    from src.proto.prediction import prediction_pb2

    req = prediction_pb2.StockRequest(symbol="VCB")
    resp = grpc_stub.TriggerStockPredict(req)
    assert resp.symbol == "VCB"
    assert hasattr(resp, "predictions_count")


def test_trigger_reconcile(grpc_stub):
    """TriggerReconcile returns a TriggerResponse."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerReconcile(prediction_pb2.Empty())
    assert resp is not None
    assert hasattr(resp, "success")


def test_trigger_historical_backtest(grpc_stub):
    """TriggerHistoricalBacktest returns a TriggerResponse."""
    from src.proto.prediction import prediction_pb2

    req = prediction_pb2.BacktestRequest(train_window=30, step_size=6, market_key="VN30")
    resp = grpc_stub.TriggerHistoricalBacktest(req)
    assert resp is not None
    assert hasattr(resp, "success")


def test_trigger_gold_predict(grpc_stub):
    """TriggerGoldPredict acknowledges the request."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerGoldPredict(prediction_pb2.Empty())
    assert resp.success is True


def test_trigger_nasdaq_predict(grpc_stub):
    """TriggerNasdaqPredict acknowledges the request."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerNasdaqPredict(prediction_pb2.Empty())
    assert resp.success is True


def test_trigger_crypto_predict(grpc_stub):
    """TriggerCryptoPredict acknowledges the request."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerCryptoPredict(prediction_pb2.Empty())
    assert resp.success is True


def test_trigger_fuel_predict(grpc_stub):
    """TriggerFuelPredict acknowledges the request."""
    from src.proto.prediction import prediction_pb2

    resp = grpc_stub.TriggerFuelPredict(prediction_pb2.Empty())
    assert resp.success is True


def test_trigger_stock_history(grpc_stub):
    """TriggerStockHistory with days=30 returns success=True."""
    from src.proto.prediction import prediction_pb2

    req = prediction_pb2.StockHistoryRequest(days=30)
    resp = grpc_stub.TriggerStockHistory(req)
    assert resp.success is True
