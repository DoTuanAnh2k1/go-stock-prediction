"""Phase 5 integration tests — full regression suite.

Covers the complete daily workflow:
  1. All 5 crawlers
  2. All-market prediction
  3. Training pipeline
  4. Reconcile
  5. Dashboard/API reads
  6. Performance benchmarks
  7. Scheduler health

Run:
    make test-phase5
    # or
    docker exec prediction_service python -m pytest tests/integration/test_phase5_full_regression.py -v
"""
from __future__ import annotations

import time

import pytest
import requests

pytestmark = pytest.mark.integration

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

CRAWLER_ENDPOINTS = [
    "crawler",
    "gold-crawler",
    "nasdaq-crawler",
    "crypto-crawler",
    "fuel-crawler",
]


def _trigger(api_base_url: str, auth_headers: dict, endpoint: str, **kwargs) -> requests.Response:
    return requests.post(f"{api_base_url}/api/trigger/{endpoint}", headers=auth_headers, timeout=60, **kwargs)


# ===========================================================================
# 1. Auth smoke test
# ===========================================================================

class TestAuthSmoke:
    def test_login_returns_token(self, api_base_url):
        resp = requests.post(
            f"{api_base_url}/api/auth/login",
            json={"username": "admin", "password": "admin123"},
            timeout=10,
        )
        assert resp.status_code == 200
        data = resp.json()
        assert "token" in data and data["token"] != ""

    def test_me_with_valid_token(self, api_base_url, auth_headers):
        resp = requests.get(f"{api_base_url}/api/auth/me", headers=auth_headers, timeout=10)
        assert resp.status_code == 200
        data = resp.json()
        assert data.get("username") == "admin"

    def test_trigger_without_token_returns_401(self, api_base_url):
        for endpoint in CRAWLER_ENDPOINTS[:2]:
            resp = requests.post(f"{api_base_url}/api/trigger/{endpoint}", timeout=10)
            assert resp.status_code == 401, f"{endpoint} should require auth"


# ===========================================================================
# 2. All crawlers respond
# ===========================================================================

class TestAllCrawlersRespond:
    """Verify every crawler endpoint returns 200 or 202 from the API Backend.

    Crawlers run in background → HTTP 202 Accepted is the expected success code.
    """

    def test_vn30_crawler(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "crawler")
        assert resp.status_code in (200, 202), resp.text[:300]

    def test_gold_crawler(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "gold-crawler")
        assert resp.status_code in (200, 202), resp.text[:300]

    def test_nasdaq_crawler(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "nasdaq-crawler")
        assert resp.status_code in (200, 202), resp.text[:300]

    def test_crypto_crawler(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "crypto-crawler")
        assert resp.status_code in (200, 202), resp.text[:300]

    def test_fuel_crawler(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "fuel-crawler")
        assert resp.status_code in (200, 202), resp.text[:300]


# ===========================================================================
# 3. gRPC contract — all RPCs registered
# ===========================================================================

class TestGRPCContractAllRPCs:
    """Verify every RPC in the proto is registered and responds (not UNIMPLEMENTED)."""

    def test_get_training_status(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
        assert resp is not None
        assert isinstance(resp.is_training, bool)

    def test_trigger_crawler(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerCrawler(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_gold_crawler(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerGoldCrawler(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_nasdaq_crawler(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerNasdaqCrawler(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_crypto_crawler(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerCryptoCrawler(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_fuel_crawler(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerFuelCrawler(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_predict(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerPredict(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_gold_predict(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerGoldPredict(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_nasdaq_predict(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerNasdaqPredict(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_crypto_predict(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerCryptoPredict(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_fuel_predict(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerFuelPredict(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_reconcile(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerReconcile(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_stock_history(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerStockHistory(prediction_pb2.StockHistoryRequest(days=7))
        assert resp.success is True

    def test_trigger_gold_history(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerGoldHistory(prediction_pb2.Empty())
        assert resp.success is True

    def test_trigger_train_all(self, grpc_stub):
        from src.proto.prediction import prediction_pb2

        # Wait for any existing training to finish
        for _ in range(30):
            status = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
            if not status.is_training:
                break
            time.sleep(2)

        resp = grpc_stub.TriggerTrain(prediction_pb2.TriggerTrainRequest())
        assert resp is not None
        assert resp.success is True or "already in progress" in (resp.error or "").lower()

    def test_trigger_stock_crawl(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerStockCrawl(prediction_pb2.StockRequest(symbol="VCB"))
        assert resp is not None
        # success is True if stock data was saved; False may mean VCB not in DB yet
        assert isinstance(resp.success, bool)

    def test_trigger_stock_predict(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerStockPredict(prediction_pb2.StockRequest(symbol="VCB"))
        assert resp is not None
        assert isinstance(resp.success, bool)

    def test_trigger_historical_backtest(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerHistoricalBacktest(
            prediction_pb2.BacktestRequest(train_window=30, step_size=6, market_key="VN30")
        )
        assert resp is not None
        assert resp.success is True or "already running" in (resp.error or "").lower()


# ===========================================================================
# 4. API Backend endpoints — full read coverage
# ===========================================================================

class TestAPIBackendReadEndpoints:
    """All GET endpoints must respond with 200 and valid JSON."""

    def test_dashboard_stats(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/dashboard/stats", timeout=10)
        assert resp.status_code == 200
        data = resp.json()
        assert data is not None

    def test_training_status(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/training/status", timeout=10)
        assert resp.status_code == 200
        data = resp.json()
        assert "is_training" in data

    def test_training_history(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/training/history", timeout=10)
        assert resp.status_code == 200

    def test_training_algorithms(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/training/algorithms", timeout=10)
        assert resp.status_code == 200
        data = resp.json()
        assert isinstance(data, list)
        assert len(data) >= 6, f"Expected >= 6 algorithms (MA, EMA, LSTM, ARIMA, LightGBM, Ensemble), got {len(data)}"

    def test_training_algorithms_has_all_required_keys(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/training/algorithms", timeout=10)
        assert resp.status_code == 200
        data = resp.json()
        keys = {a.get("key") or a.get("name") for a in data}
        for required in ("moving_average", "lstm_nn", "arima_garch"):
            assert any(required in str(k) for k in keys), f"{required} missing from /api/training/algorithms"

    def test_training_metrics(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/training/metrics", timeout=10)
        assert resp.status_code == 200

    def test_predictions_list(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/predictions", timeout=10)
        assert resp.status_code == 200

    def test_predictions_accuracy(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/predictions/accuracy", timeout=10)
        assert resp.status_code == 200

    def test_predictions_accuracy_trend(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/predictions/accuracy-trend", timeout=10)
        assert resp.status_code == 200

    def test_market_overview(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/market/overview", timeout=10)
        assert resp.status_code == 200

    def test_gold_latest(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/gold/latest", timeout=10)
        assert resp.status_code == 200

    def test_gold_prices(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/gold/prices", timeout=10)
        assert resp.status_code == 200

    def test_gold_predictions_list(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/gold/predictions", timeout=10)
        assert resp.status_code == 200

    def test_schedules_list(self, api_base_url, auth_headers):
        resp = requests.get(f"{api_base_url}/api/schedules", headers=auth_headers, timeout=10)
        assert resp.status_code == 200
        data = resp.json()
        assert isinstance(data, list)
        assert len(data) >= 8, f"Expected >= 8 schedules, got {len(data)}"

    def test_schedules_have_required_fields(self, api_base_url, auth_headers):
        resp = requests.get(f"{api_base_url}/api/schedules", headers=auth_headers, timeout=10)
        assert resp.status_code == 200
        for s in resp.json():
            assert "job_key" in s
            assert "cron_expression" in s
            assert "enabled" in s

    def test_algorithms_comparison(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/algorithms/comparison", timeout=10)
        assert resp.status_code == 200

    def test_algorithms_backtest_requires_symbol(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/algorithms/backtest", timeout=10)
        assert resp.status_code == 400

    def test_algorithms_backtest_with_valid_symbol(self, api_base_url):
        resp = requests.get(f"{api_base_url}/api/algorithms/backtest?symbol=VCB", timeout=30)
        # Returns 200 with results or 400 if insufficient data — never 500
        assert resp.status_code in (200, 400), f"Unexpected {resp.status_code}: {resp.text[:300]}"

    def test_users_list_requires_admin(self, api_base_url, auth_headers):
        resp = requests.get(f"{api_base_url}/api/users", headers=auth_headers, timeout=10)
        assert resp.status_code == 200

    def test_health_check(self, api_base_url):
        resp = requests.get(f"{api_base_url}/health", timeout=10)
        assert resp.status_code == 200


# ===========================================================================
# 5. Trigger endpoints end-to-end (POST → gRPC → Python)
# ===========================================================================

class TestTriggerEndpoints:
    def test_trigger_predict_returns_200(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "predict")
        assert resp.status_code == 200, resp.text[:300]

    def test_trigger_reconcile_returns_200(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "reconcile")
        assert resp.status_code == 200, resp.text[:300]

    def test_trigger_train_returns_202_or_409(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "train")
        assert resp.status_code in (202, 409), resp.text[:300]

    def test_trigger_historical_backtest_returns_202_or_409(self, api_base_url, auth_headers):
        resp = requests.post(
            f"{api_base_url}/api/trigger/historical-backtest?train_window=30&step_size=6",
            headers=auth_headers,
            timeout=30,
        )
        assert resp.status_code in (202, 409), resp.text[:300]

    def test_trigger_gold_crawler_returns_2xx(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "gold-crawler")
        assert resp.status_code in (200, 202), resp.text[:300]

    def test_trigger_nasdaq_crawler_returns_2xx(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "nasdaq-crawler")
        assert resp.status_code in (200, 202), resp.text[:300]

    def test_trigger_crypto_crawler_returns_2xx(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "crypto-crawler")
        assert resp.status_code in (200, 202), resp.text[:300]

    def test_trigger_fuel_crawler_returns_2xx(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "fuel-crawler")
        assert resp.status_code in (200, 202), resp.text[:300]

    def test_trigger_gold_predict_returns_2xx(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "gold-predict")
        assert resp.status_code in (200, 202), resp.text[:300]

    def test_trigger_nasdaq_predict_returns_2xx(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "nasdaq-predict")
        assert resp.status_code in (200, 202), resp.text[:300]

    def test_trigger_crypto_predict_returns_2xx(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "crypto-predict")
        assert resp.status_code in (200, 202), resp.text[:300]

    def test_trigger_fuel_predict_returns_2xx(self, api_base_url, auth_headers):
        resp = _trigger(api_base_url, auth_headers, "fuel-predict")
        assert resp.status_code in (200, 202), resp.text[:300]


# ===========================================================================
# 6. Schedule CRUD
# ===========================================================================

class TestScheduleCRUD:
    def test_list_schedules(self, api_base_url, auth_headers):
        resp = requests.get(f"{api_base_url}/api/schedules", headers=auth_headers, timeout=10)
        assert resp.status_code == 200

    def test_update_schedule_no_op(self, api_base_url, auth_headers):
        """Update a schedule with its current values (no-op) — must return 200."""
        list_resp = requests.get(f"{api_base_url}/api/schedules", headers=auth_headers, timeout=10)
        schedules = list_resp.json()
        assert len(schedules) > 0

        first = schedules[0]
        put_resp = requests.put(
            f"{api_base_url}/api/schedules/{first['job_key']}",
            headers=auth_headers,
            json={"cron_expression": first["cron_expression"], "enabled": first["enabled"]},
            timeout=10,
        )
        assert put_resp.status_code == 200

    def test_schedule_update_without_auth_returns_401(self, api_base_url):
        resp = requests.put(
            f"{api_base_url}/api/schedules/crawler_daily",
            json={"cron_expression": "0 0 12 * * *", "enabled": True},
            timeout=10,
        )
        assert resp.status_code == 401

    def test_all_expected_jobs_present(self, api_base_url, auth_headers):
        resp = requests.get(f"{api_base_url}/api/schedules", headers=auth_headers, timeout=10)
        assert resp.status_code == 200
        keys = {s["job_key"] for s in resp.json()}
        # Python service seeds these job keys (may differ from old Go keys)
        expected = {
            "crawler_stock",
            "crawler_gold",
            "daily_prediction",
            "daily_reconcile",
            "weekly_training",
        }
        missing = expected - keys
        assert not missing, f"Missing schedule job keys: {missing}. Available: {keys}"


# ===========================================================================
# 7. Performance benchmarks
# ===========================================================================

class TestPerformanceBenchmarks:
    """Response time and throughput requirements."""

    def test_dashboard_stats_under_3s(self, api_base_url):
        start = time.time()
        resp = requests.get(f"{api_base_url}/api/dashboard/stats", timeout=10)
        elapsed = time.time() - start
        assert resp.status_code == 200
        assert elapsed < 3.0, f"dashboard/stats took {elapsed:.1f}s (limit: 3s)"

    def test_training_status_under_5s(self, api_base_url):
        start = time.time()
        resp = requests.get(f"{api_base_url}/api/training/status", timeout=10)
        elapsed = time.time() - start
        assert resp.status_code == 200
        assert elapsed < 5.0, f"training/status took {elapsed:.1f}s (limit: 5s)"

    def test_grpc_get_training_status_under_2s(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        start = time.time()
        resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
        elapsed = time.time() - start
        assert resp is not None
        assert elapsed < 2.0, f"GetTrainingStatus took {elapsed:.2f}s (limit: 2s)"

    def test_single_stock_predict_under_60s(self, grpc_stub):
        """TriggerStockPredict for one symbol must complete within 60 seconds."""
        from src.proto.prediction import prediction_pb2
        start = time.time()
        resp = grpc_stub.TriggerStockPredict(prediction_pb2.StockRequest(symbol="VCB"))
        elapsed = time.time() - start
        assert resp is not None
        assert elapsed < 60.0, f"TriggerStockPredict took {elapsed:.1f}s (limit: 60s)"

    def test_gold_predict_under_30s(self, grpc_stub):
        from src.proto.prediction import prediction_pb2
        start = time.time()
        resp = grpc_stub.TriggerGoldPredict(prediction_pb2.Empty())
        elapsed = time.time() - start
        assert resp is not None
        assert elapsed < 30.0, f"TriggerGoldPredict took {elapsed:.1f}s (limit: 30s)"

    def test_concurrent_grpc_calls_dont_crash(self, grpc_stub):
        """Fire 5 simultaneous gRPC status calls — service must respond to all."""
        import threading

        from src.proto.prediction import prediction_pb2

        results = []
        errors = []

        def call():
            try:
                resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
                results.append(resp)
            except Exception as e:
                errors.append(e)

        threads = [threading.Thread(target=call) for _ in range(5)]
        for t in threads:
            t.start()
        for t in threads:
            t.join(timeout=10)

        assert len(errors) == 0, f"Concurrent calls raised errors: {errors}"
        assert len(results) == 5, f"Expected 5 responses, got {len(results)}"


# ===========================================================================
# 8. Service resilience
# ===========================================================================

class TestServiceResilience:
    def test_service_alive_after_all_ops(self, grpc_stub):
        """After a mix of operations, GetTrainingStatus must still respond."""
        from src.proto.prediction import prediction_pb2

        grpc_stub.TriggerReconcile(prediction_pb2.Empty())
        grpc_stub.TriggerGoldPredict(prediction_pb2.Empty())
        grpc_stub.TriggerCryptoCrawler(prediction_pb2.Empty())

        resp = grpc_stub.GetTrainingStatus(prediction_pb2.Empty())
        assert resp is not None
        assert resp.is_training is True or resp.is_training is False

    def test_unknown_stock_predict_does_not_crash(self, grpc_stub):
        """Predicting an unknown symbol must return gracefully, not crash the service."""
        from src.proto.prediction import prediction_pb2
        resp = grpc_stub.TriggerStockPredict(
            prediction_pb2.StockRequest(symbol="NONEXISTENT_SYMBOL_XYZ99")
        )
        # success may be True (empty result) or False — just must not raise
        assert resp is not None

    def test_api_backend_still_responds_after_stress(self, api_base_url, auth_headers):
        """API Backend must respond normally after all trigger operations."""
        resp = requests.get(f"{api_base_url}/api/training/status", timeout=10)
        assert resp.status_code == 200
