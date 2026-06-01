"""gRPC server — implements all 19 RPCs from prediction.proto."""
from __future__ import annotations

import threading
from concurrent import futures

import grpc

from src.utils.logger import get_logger

log = get_logger("grpc_server")

_backtest_running = threading.Event()


def _get_pb():
    """Lazy import of generated proto modules to avoid import-time errors."""
    try:
        from src.proto.prediction import prediction_pb2, prediction_pb2_grpc
    except ImportError:
        # Fallback path when running inside Docker (proto generated to /app/src/proto)
        import importlib
        prediction_pb2 = importlib.import_module("src.proto.prediction.prediction_pb2")
        prediction_pb2_grpc = importlib.import_module("src.proto.prediction.prediction_pb2_grpc")
    return prediction_pb2, prediction_pb2_grpc


class PredictionServicer:
    """Implements the PredictionService gRPC interface."""

    # -------------------------------------------------------------------
    # Background: fire-and-forget
    # -------------------------------------------------------------------

    def TriggerCrawler(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerCrawler")
        threading.Thread(target=_bg_crawl_vn30, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="Stock crawler started in background")

    def TriggerNasdaqCrawler(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerNasdaqCrawler")
        threading.Thread(target=_bg_crawl_nasdaq, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="NASDAQ crawler started in background")

    def TriggerCryptoCrawler(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerCryptoCrawler")
        threading.Thread(target=_bg_crawl_crypto, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="Crypto crawler started in background")

    def TriggerFuelCrawler(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerFuelCrawler")
        threading.Thread(target=_bg_crawl_fuel, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="Fuel crawler started in background")

    def TriggerStockHistory(self, request, context):
        pb2, _ = _get_pb()
        days = request.days or 365
        log.info("grpc.TriggerStockHistory", days=days)
        threading.Thread(target=_bg_stock_history, args=(days,), daemon=True).start()
        return pb2.TriggerResponse(success=True, message=f"Historical stock crawl started (days={days})")

    def TriggerGoldHistory(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerGoldHistory")
        threading.Thread(target=_bg_gold_history, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="Gold history import started in background")

    def TriggerGoldPredict(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerGoldPredict")
        threading.Thread(target=_bg_predict_gold, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="Gold prediction started in background")

    def TriggerNasdaqPredict(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerNasdaqPredict")
        threading.Thread(target=_bg_predict_nasdaq, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="NASDAQ prediction started in background")

    def TriggerCryptoPredict(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerCryptoPredict")
        threading.Thread(target=_bg_predict_crypto, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="Crypto prediction started in background")

    def TriggerFuelPredict(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerFuelPredict")
        threading.Thread(target=_bg_predict_fuel, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="Fuel prediction started in background")

    # -------------------------------------------------------------------
    # Backtest (background with concurrency guard)
    # -------------------------------------------------------------------

    def TriggerHistoricalBacktest(self, request, context):
        pb2, _ = _get_pb()
        if _backtest_running.is_set():
            return pb2.TriggerResponse(success=False, error="backtest already running")

        train_window = request.train_window or 30
        step_size = request.step_size or 6
        market_key = request.market_key or "VN30"

        log.info("grpc.TriggerHistoricalBacktest", market=market_key, train_window=train_window, step_size=step_size)

        _backtest_running.set()
        threading.Thread(
            target=_bg_backtest,
            args=(train_window, step_size, market_key),
            daemon=True,
        ).start()

        return pb2.TriggerResponse(success=True, message="Historical backtest started in background")

    # -------------------------------------------------------------------
    # Synchronous
    # -------------------------------------------------------------------

    def TriggerGoldCrawler(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerGoldCrawler")
        try:
            from src.crawlers.gold import GoldCrawler
            saved = GoldCrawler().crawl()
            return pb2.TriggerResponse(success=True, message=f"Gold crawler completed: {saved} prices saved")
        except Exception as exc:
            log.error("grpc.TriggerGoldCrawler.error", error=str(exc))
            return pb2.TriggerResponse(success=False, error=str(exc))

    def TriggerReconcile(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerReconcile")
        try:
            from src.orchestrator.training import reconcile_predictions
            updated = reconcile_predictions()
            return pb2.TriggerResponse(success=True, message=f"Reconcile completed: {updated} predictions updated")
        except Exception as exc:
            log.error("grpc.TriggerReconcile.error", error=str(exc))
            return pb2.TriggerResponse(success=False, error=str(exc))

    def TriggerPredict(self, request, context):
        """Runs training + all market predictions synchronously."""
        pb2, _ = _get_pb()
        log.info("grpc.TriggerPredict")
        try:
            from src.orchestrator.runner import run_all_markets
            from src.orchestrator.training import train_all_algorithms
            train_all_algorithms()
            total = run_all_markets()
            return pb2.TriggerResponse(success=True, message=f"Training and prediction completed: {total} predictions")
        except Exception as exc:
            log.error("grpc.TriggerPredict.error", error=str(exc))
            return pb2.TriggerResponse(success=False, error=str(exc))

    def TriggerTrain(self, request, context):
        pb2, _ = _get_pb()
        algo = request.algorithm or ""
        log.info("grpc.TriggerTrain", algorithm=algo)
        try:
            from src.orchestrator import training
            if training.get_training_status()["is_training"]:
                return pb2.TriggerTrainResponse(success=False, error="training already in progress")

            if algo:
                success, session_id = training.train_single_algorithm(algo)
            else:
                # Start in background, return immediately with session ID
                import uuid
                session_id = str(uuid.uuid4())
                threading.Thread(target=training.train_all_algorithms, daemon=True).start()
                success = True

            return pb2.TriggerTrainResponse(
                success=success,
                session_id=session_id,
                message="Training started",
            )
        except Exception as exc:
            log.error("grpc.TriggerTrain.error", error=str(exc))
            return pb2.TriggerTrainResponse(success=False, error=str(exc))

    def TriggerStockCrawl(self, request, context):
        pb2, _ = _get_pb()
        symbol = request.symbol
        log.info("grpc.TriggerStockCrawl", symbol=symbol)
        try:
            from src.crawlers.vn30 import VN30Crawler
            saved = VN30Crawler().crawl_single(symbol)
            return pb2.StockCrawlResponse(
                success=True,
                symbol=symbol,
                message=f"Stock {symbol} crawled: {saved} price(s) saved",
            )
        except Exception as exc:
            log.error("grpc.TriggerStockCrawl.error", symbol=symbol, error=str(exc))
            return pb2.StockCrawlResponse(success=False, symbol=symbol, error=str(exc))

    def TriggerStockPredict(self, request, context):
        pb2, _ = _get_pb()
        symbol = request.symbol
        log.info("grpc.TriggerStockPredict", symbol=symbol)
        try:
            from datetime import datetime, timedelta
            from decimal import Decimal

            from src.algorithms.registry import build_algorithms
            from src.database import repository as repo

            stock = repo.get_stock_by_symbol(symbol)
            if not stock:
                return pb2.StockPredictResponse(success=False, symbol=symbol, error=f"stock {symbol} not found")

            prices_asc = repo.get_stock_prices_asc(stock.id, limit=270)
            if len(prices_asc) < 20:
                return pb2.StockPredictResponse(
                    success=False, symbol=symbol,
                    error=f"insufficient data: {len(prices_asc)} prices",
                )

            price_list = [float(p.close_price) for p in prices_asc]
            vol_list = [float(p.volume or 0) for p in prices_asc]
            current = price_list[-1]
            now = datetime.utcnow()
            target = now + timedelta(days=1)

            algos = build_algorithms()
            count = 0
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
                except Exception as exc2:
                    log.debug("grpc.StockPredict.algo_failed", symbol=symbol, algo=key, error=str(exc2))

            return pb2.StockPredictResponse(
                success=True,
                symbol=symbol,
                predictions_count=count,
                message=f"Generated {count} predictions for {symbol}",
            )
        except Exception as exc:
            log.error("grpc.TriggerStockPredict.error", symbol=symbol, error=str(exc))
            return pb2.StockPredictResponse(success=False, symbol=symbol, error=str(exc))

    def GetTrainingStatus(self, request, context):
        pb2, _ = _get_pb()
        try:
            from src.orchestrator.training import get_training_status
            st = get_training_status()
            return pb2.TrainingStatusResponse(
                is_training=st["is_training"],
                last_trained=st["last_trained"],
                progress=st["progress"],
                current_phase=st["current_phase"],
                total_algorithms=st["total_algorithms"],
                done_algorithms=st["done_algorithms"],
            )
        except Exception as exc:
            log.error("grpc.GetTrainingStatus.error", error=str(exc))
            return pb2.TrainingStatusResponse(is_training=False, current_phase="error")


# -------------------------------------------------------------------
# Background worker functions
# -------------------------------------------------------------------

def _bg_crawl_vn30():
    try:
        from src.crawlers.vn30 import VN30Crawler
        saved = VN30Crawler().crawl()
        log.info("bg.vn30.done", saved=saved)
    except Exception as exc:
        log.error("bg.vn30.error", error=str(exc))


def _bg_crawl_nasdaq():
    try:
        from src.crawlers.nasdaq import NasdaqCrawler
        saved = NasdaqCrawler().crawl()
        log.info("bg.nasdaq.done", saved=saved)
    except Exception as exc:
        log.error("bg.nasdaq.error", error=str(exc))


def _bg_crawl_crypto():
    try:
        from src.crawlers.crypto import CryptoCrawler
        saved = CryptoCrawler().crawl()
        log.info("bg.crypto.done", saved=saved)
    except Exception as exc:
        log.error("bg.crypto.error", error=str(exc))


def _bg_crawl_fuel():
    try:
        from src.crawlers.fuel import FuelCrawler
        saved = FuelCrawler().crawl()
        log.info("bg.fuel.done", saved=saved)
    except Exception as exc:
        log.error("bg.fuel.error", error=str(exc))


def _bg_stock_history(days: int):
    try:
        from src.crawlers.vn30 import VN30Crawler
        saved = VN30Crawler().crawl_history(days=days)
        log.info("bg.stock_history.done", saved=saved)
    except Exception as exc:
        log.error("bg.stock_history.error", error=str(exc))


def _bg_gold_history():
    try:
        from src.crawlers.gold import GoldCrawler
        saved = GoldCrawler().crawl_history()
        log.info("bg.gold_history.done", saved=saved)
    except Exception as exc:
        log.error("bg.gold_history.error", error=str(exc))


def _bg_predict_gold():
    try:
        from src.orchestrator.runner import run_for_market
        n = run_for_market("GOLD")
        log.info("bg.predict_gold.done", count=n)
    except Exception as exc:
        log.error("bg.predict_gold.error", error=str(exc))


def _bg_predict_nasdaq():
    try:
        from src.orchestrator.runner import run_for_market
        n = run_for_market("NASDAQ100")
        log.info("bg.predict_nasdaq.done", count=n)
    except Exception as exc:
        log.error("bg.predict_nasdaq.error", error=str(exc))


def _bg_predict_crypto():
    try:
        from src.orchestrator.runner import run_for_market
        n = run_for_market("CRYPTO")
        log.info("bg.predict_crypto.done", count=n)
    except Exception as exc:
        log.error("bg.predict_crypto.error", error=str(exc))


def _bg_predict_fuel():
    try:
        from src.orchestrator.runner import run_for_market
        n = run_for_market("FUEL")
        log.info("bg.predict_fuel.done", count=n)
    except Exception as exc:
        log.error("bg.predict_fuel.error", error=str(exc))


def _bg_backtest(train_window: int, step_size: int, market_key: str):
    try:
        from src.orchestrator.training import run_historical_backtest
        result = run_historical_backtest(train_window, step_size, market_key)
        log.info("bg.backtest.done", **result)
    except Exception as exc:
        log.error("bg.backtest.error", error=str(exc))
    finally:
        _backtest_running.clear()


# -------------------------------------------------------------------
# Server lifecycle
# -------------------------------------------------------------------

_grpc_server: grpc.Server | None = None


def start_grpc_server(port: int) -> None:
    global _grpc_server
    pb2, pb2_grpc = _get_pb()

    servicer = PredictionServicer()
    _grpc_server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    pb2_grpc.add_PredictionServiceServicer_to_server(servicer, _grpc_server)
    _grpc_server.add_insecure_port(f"[::]:{port}")
    _grpc_server.start()
    log.info("grpc.server.started", port=port)


def stop_grpc_server() -> None:
    global _grpc_server
    if _grpc_server:
        log.info("grpc.server.stopping")
        _grpc_server.stop(grace=5)
        _grpc_server = None
