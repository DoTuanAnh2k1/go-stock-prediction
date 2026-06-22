"""gRPC server — implements all 19 RPCs from prediction.proto."""
from __future__ import annotations

import queue
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
        log.info("grpc.TriggerCrawler.removed")
        return pb2.TriggerResponse(success=False, message="stock crawler removed")

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

    def TriggerCryptoHistory(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerCryptoHistory")
        threading.Thread(target=_bg_crypto_history, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="Crypto history import started in background")

    def TriggerStockHistory(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerStockHistory.removed")
        return pb2.TriggerResponse(success=False, message="stock history removed")

    def TriggerGoldHistory(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerGoldHistory")
        threading.Thread(target=_bg_gold_history, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="Gold history import started in background")

    def TriggerGoldPredict(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerGoldPredict")
        try:
            from src.orchestrator.runner import run_for_market
            n = run_for_market("GOLD", force=True)
            log.info("grpc.TriggerGoldPredict.done", count=n)
            return pb2.TriggerResponse(success=True, message=f"Gold prediction completed: {n} predictions saved")
        except Exception as exc:
            log.error("grpc.TriggerGoldPredict.error", error=str(exc))
            return pb2.TriggerResponse(success=False, error=str(exc))

    def TriggerNasdaqPredict(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerNasdaqPredict")
        try:
            from src.orchestrator.runner import run_for_market
            n = run_for_market("NASDAQ100", force=True)
            log.info("grpc.TriggerNasdaqPredict.done", count=n)
            return pb2.TriggerResponse(success=True, message=f"NASDAQ prediction completed: {n} predictions saved")
        except Exception as exc:
            log.error("grpc.TriggerNasdaqPredict.error", error=str(exc))
            return pb2.TriggerResponse(success=False, error=str(exc))

    def TriggerCryptoPredict(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerCryptoPredict")
        try:
            from src.orchestrator.runner import run_for_market
            n = run_for_market("CRYPTO", force=True)
            log.info("grpc.TriggerCryptoPredict.done", count=n)
            return pb2.TriggerResponse(success=True, message=f"Crypto prediction completed: {n} predictions saved")
        except Exception as exc:
            log.error("grpc.TriggerCryptoPredict.error", error=str(exc))
            return pb2.TriggerResponse(success=False, error=str(exc))

    def TriggerSP500Crawler(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerSP500Crawler")
        threading.Thread(target=_bg_crawl_sp500, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="S&P 500 crawler started in background")

    def TriggerSP500Predict(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerSP500Predict")
        try:
            from src.orchestrator.runner import run_for_market
            n = run_for_market("SP500", force=True)
            log.info("grpc.TriggerSP500Predict.done", count=n)
            return pb2.TriggerResponse(success=True, message=f"SP500 prediction completed: {n} predictions saved")
        except Exception as exc:
            log.error("grpc.TriggerSP500Predict.error", error=str(exc))
            return pb2.TriggerResponse(success=False, error=str(exc))

    # -------------------------------------------------------------------
    # Backtest (background with concurrency guard)
    # -------------------------------------------------------------------

    def TriggerHistoricalBacktest(self, request, context):
        pb2, _ = _get_pb()
        if _backtest_running.is_set():
            return pb2.TriggerResponse(success=False, error="backtest already running")

        train_window = request.train_window or 30
        step_size = request.step_size or 6
        market_key = request.market_key or "GOLD"

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
            crawler = GoldCrawler()
            saved = crawler.crawl()
            try:
                crawler.crawl_intraday()
            except Exception as exc:
                log.warning("grpc.gold.intraday.error", error=str(exc))
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
        """Runs all market predictions in background (training is separate via TriggerTrain)."""
        pb2, _ = _get_pb()
        log.info("grpc.TriggerPredict")
        threading.Thread(target=_bg_predict_all, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="Prediction started in background for all markets")

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
        log.info("grpc.TriggerStockCrawl.removed", symbol=symbol)
        return pb2.StockCrawlResponse(success=False, symbol=symbol, message="stock crawler removed")

    def TriggerStockPredict(self, request, context):
        pb2, _ = _get_pb()
        symbol = request.symbol
        log.info("grpc.TriggerStockPredict.removed", symbol=symbol)
        return pb2.StockPredictResponse(success=False, symbol=symbol, message="stock predict removed")

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

    def TriggerSimulationBacktest(self, request, context):
        pb2, _ = _get_pb()
        bot_id = request.bot_id or ""
        start_date_str = request.start_date or "2024-01-01"
        end_date_str = request.end_date or ""
        log.info("grpc.TriggerSimulationBacktest", bot_id=bot_id,
                 start=start_date_str, end=end_date_str)
        threading.Thread(
            target=_bg_simulation_backtest,
            args=(bot_id, start_date_str, end_date_str),
            daemon=True
        ).start()
        msg = f"Simulation backtest started for bot_id={bot_id or 'ALL'}"
        return pb2.TriggerResponse(success=True, message=msg)

    def TriggerSimulationLiveStep(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.TriggerSimulationLiveStep")
        threading.Thread(target=_bg_simulation_live_step, daemon=True).start()
        return pb2.TriggerResponse(success=True, message="Simulation live step started")

    def ResetSimBots(self, request, context):
        pb2, _ = _get_pb()
        log.info("grpc.ResetSimBots")
        try:
            from src.simulation.engine import SimulationEngine
            count = SimulationEngine().reset_active_bots()
            return pb2.TriggerResponse(success=True, message=f"Reset {count} active bots with fresh live sessions")
        except Exception as exc:
            log.error("grpc.ResetSimBots.error", error=str(exc))
            return pb2.TriggerResponse(success=False, error=str(exc))

    # -------------------------------------------------------------------
    # Streaming pipeline predict RPCs
    # -------------------------------------------------------------------

    def StreamGoldPredict(self, request, context):
        yield from _stream_predict("GOLD", context)

    def StreamNasdaqPredict(self, request, context):
        yield from _stream_predict("NASDAQ100", context)

    def StreamCryptoPredict(self, request, context):
        yield from _stream_predict("CRYPTO", context)

    def StreamSP500Predict(self, request, context):
        yield from _stream_predict("SP500", context)


# -------------------------------------------------------------------
# Background worker functions
# -------------------------------------------------------------------

def _bg_crawl_nasdaq():
    try:
        from src.crawlers.nasdaq import NasdaqCrawler
        crawler = NasdaqCrawler()
        saved = crawler.crawl()
        log.info("bg.nasdaq.done", saved=saved)
        crawler.crawl_intraday()
    except Exception as exc:
        log.error("bg.nasdaq.error", error=str(exc))


def _bg_crawl_crypto():
    try:
        from src.crawlers.crypto import CryptoCrawler
        crawler = CryptoCrawler()
        saved = crawler.crawl()
        log.info("bg.crypto.done", saved=saved)
        crawler.crawl_intraday()
    except Exception as exc:
        log.error("bg.crypto.error", error=str(exc))


def _bg_crypto_history():
    try:
        from src.crawlers.crypto import CryptoCrawler
        saved = CryptoCrawler().crawl_history()
        log.info("bg.crypto_history.done", saved=saved)
    except Exception as exc:
        log.error("bg.crypto_history.error", error=str(exc))


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
        n = run_for_market("GOLD", force=True)
        log.info("bg.predict_gold.done", count=n)
    except Exception as exc:
        log.error("bg.predict_gold.error", error=str(exc))


def _bg_predict_nasdaq():
    try:
        from src.orchestrator.runner import run_for_market
        n = run_for_market("NASDAQ100", force=True)
        log.info("bg.predict_nasdaq.done", count=n)
    except Exception as exc:
        log.error("bg.predict_nasdaq.error", error=str(exc))


def _bg_predict_crypto():
    try:
        from src.orchestrator.runner import run_for_market
        n = run_for_market("CRYPTO", force=True)
        log.info("bg.predict_crypto.done", count=n)
    except Exception as exc:
        log.error("bg.predict_crypto.error", error=str(exc))


def _bg_crawl_sp500():
    try:
        from src.crawlers.sp500 import SP500Crawler
        crawler = SP500Crawler()
        saved = crawler.crawl()
        log.info("bg.sp500.done", saved=saved)
        crawler.crawl_intraday()
    except Exception as exc:
        log.error("bg.sp500.error", error=str(exc))


def _bg_predict_sp500():
    try:
        from src.orchestrator.runner import run_for_market
        n = run_for_market("SP500", force=True)
        log.info("bg.predict_sp500.done", count=n)
    except Exception as exc:
        log.error("bg.predict_sp500.error", error=str(exc))


def _bg_predict_all():
    try:
        from src.orchestrator.runner import run_all_markets
        total = run_all_markets()
        log.info("bg.predict_all.done", count=total)
    except Exception as exc:
        log.error("bg.predict_all.error", error=str(exc))


def _bg_backtest(train_window: int, step_size: int, market_key: str):
    try:
        from src.orchestrator.training import run_historical_backtest
        result = run_historical_backtest(train_window, step_size, market_key)
        log.info("bg.backtest.done", **result)
    except Exception as exc:
        log.error("bg.backtest.error", error=str(exc))
    finally:
        _backtest_running.clear()


def _bg_simulation_backtest(bot_id: str, start_date_str: str, end_date_str: str):
    from datetime import date, datetime
    from src.simulation.engine import SimulationEngine
    try:
        start = datetime.strptime(start_date_str, "%Y-%m-%d").date()
        end = datetime.strptime(end_date_str, "%Y-%m-%d").date() if end_date_str else date.today()
        engine = SimulationEngine()
        if bot_id:
            engine.run_backtest(bot_id, start, end)
        else:
            engine.run_all_bots_backtest(start, end)
    except Exception as exc:
        log.error("sim.backtest.error", error=str(exc))


def _bg_simulation_live_step():
    from src.simulation.engine import SimulationEngine
    try:
        SimulationEngine().run_live_step()
    except Exception as exc:
        log.error("sim.live_step.error", error=str(exc))


# -------------------------------------------------------------------
# Streaming predict helper
# -------------------------------------------------------------------

_SENTINEL = object()


def _stream_predict(market_key: str, context):
    """Run run_for_market in a thread, yield PipelineLogEvents via a queue."""
    pb2, _ = _get_pb()
    q: queue.Queue = queue.Queue(maxsize=500)

    def emit(level: str, msg: str, progress: float = 0.0) -> None:
        if not context.is_active():
            return
        try:
            q.put_nowait(pb2.PipelineLogEvent(level=level, msg=msg, progress=progress))
        except queue.Full:
            pass

    def run() -> None:
        try:
            from src.orchestrator.runner import run_for_market
            n = run_for_market(market_key, force=True, emit=emit)
            q.put(pb2.PipelineLogEvent(level="ok", msg=f"Complete: {n} predictions", progress=1.0, done=True))
        except Exception as exc:
            log.error("stream_predict.error", market=market_key, error=str(exc))
            q.put(pb2.PipelineLogEvent(level="error", msg=str(exc), progress=1.0, done=True, error=str(exc)))
        finally:
            q.put(_SENTINEL)

    threading.Thread(target=run, daemon=True).start()

    while context.is_active():
        try:
            item = q.get(timeout=30)
        except queue.Empty:
            # heartbeat to keep connection alive
            yield pb2.PipelineLogEvent(level="info", msg="...", progress=0.0)
            continue
        if item is _SENTINEL:
            break
        yield item


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
