"""Cron job definitions registered with the scheduler."""
from __future__ import annotations

from src.utils.logger import get_logger

log = get_logger("jobs")


def job_crawl_vn30() -> None:
    from src.crawlers.vn30 import VN30Crawler
    try:
        crawler = VN30Crawler()
        saved = crawler.crawl()
        log.info("job.vn30.done", saved=saved)
    except Exception as exc:
        log.error("job.vn30.error", error=str(exc))


def job_crawl_gold() -> None:
    from src.crawlers.gold import GoldCrawler
    try:
        crawler = GoldCrawler()
        saved = crawler.crawl()
        log.info("job.gold.done", saved=saved)
    except Exception as exc:
        log.error("job.gold.error", error=str(exc))


def job_crawl_nasdaq() -> None:
    from src.crawlers.nasdaq import NasdaqCrawler
    try:
        crawler = NasdaqCrawler()
        saved = crawler.crawl()
        log.info("job.nasdaq.done", saved=saved)
    except Exception as exc:
        log.error("job.nasdaq.error", error=str(exc))


def job_crawl_crypto() -> None:
    from src.crawlers.crypto import CryptoCrawler
    try:
        crawler = CryptoCrawler()
        saved = crawler.crawl()
        log.info("job.crypto.done", saved=saved)
    except Exception as exc:
        log.error("job.crypto.error", error=str(exc))


def job_crawl_fuel() -> None:
    from src.crawlers.fuel import FuelCrawler
    try:
        crawler = FuelCrawler()
        saved = crawler.crawl()
        log.info("job.fuel.done", saved=saved)
    except Exception as exc:
        log.error("job.fuel.error", error=str(exc))


def job_weekly_training() -> None:
    from src.orchestrator.training import train_all_algorithms
    try:
        success, session_id = train_all_algorithms()
        log.info("job.training.done", success=success, session_id=session_id)
    except Exception as exc:
        log.error("job.training.error", error=str(exc))


def job_daily_prediction() -> None:
    from src.orchestrator.runner import run_all_markets
    try:
        total = run_all_markets()
        log.info("job.prediction.done", total=total)
    except Exception as exc:
        log.error("job.prediction.error", error=str(exc))


def job_daily_reconcile() -> None:
    from src.orchestrator.training import reconcile_predictions
    try:
        updated = reconcile_predictions()
        log.info("job.reconcile.done", updated=updated)
    except Exception as exc:
        log.error("job.reconcile.error", error=str(exc))


def job_crawl_sp500() -> None:
    from src.crawlers.sp500 import SP500Crawler
    try:
        crawler = SP500Crawler()
        saved = crawler.crawl()
        log.info("job.sp500.done", saved=saved)
    except Exception as exc:
        log.error("job.sp500.error", error=str(exc))


def job_gold_predict() -> None:
    from src.orchestrator.runner import run_for_market
    try:
        count = run_for_market("GOLD")
        log.info("job.gold_predict.done", count=count)
    except Exception as exc:
        log.error("job.gold_predict.error", error=str(exc))


def job_simulation_daily() -> None:
    from src.simulation.engine import SimulationEngine
    try:
        engine = SimulationEngine()
        engine.run_live_step()
        log.info("job.simulation.done")
    except Exception as exc:
        log.error("job.simulation.error", error=str(exc))


def job_predict_vn30() -> None:
    from src.orchestrator.runner import run_for_market
    try:
        count = run_for_market("VN30")
        log.info("job.predict_vn30.done", count=count)
    except Exception as exc:
        log.error("job.predict_vn30.error", error=str(exc))


def job_predict_nasdaq() -> None:
    from src.orchestrator.runner import run_for_market
    try:
        count = run_for_market("NASDAQ100")
        log.info("job.predict_nasdaq.done", count=count)
    except Exception as exc:
        log.error("job.predict_nasdaq.error", error=str(exc))


def job_predict_crypto() -> None:
    from src.orchestrator.runner import run_for_market
    try:
        count = run_for_market("CRYPTO")
        log.info("job.predict_crypto.done", count=count)
    except Exception as exc:
        log.error("job.predict_crypto.error", error=str(exc))


def job_predict_fuel() -> None:
    from src.orchestrator.runner import run_for_market
    try:
        count = run_for_market("FUEL")
        log.info("job.predict_fuel.done", count=count)
    except Exception as exc:
        log.error("job.predict_fuel.error", error=str(exc))


def job_predict_sp500() -> None:
    from src.orchestrator.runner import run_for_market
    try:
        count = run_for_market("SP500")
        log.info("job.predict_sp500.done", count=count)
    except Exception as exc:
        log.error("job.predict_sp500.error", error=str(exc))


# Job registry — maps job_key → callable
JOB_FUNCTIONS = {
    "crawler_stock": job_crawl_vn30,
    "crawler_gold": job_crawl_gold,
    "crawler_nasdaq": job_crawl_nasdaq,
    "crawler_crypto": job_crawl_crypto,
    "crawler_fuel": job_crawl_fuel,
    "crawler_sp500": job_crawl_sp500,
    "weekly_training": job_weekly_training,
    "daily_prediction": job_daily_prediction,
    "daily_reconcile": job_daily_reconcile,
    "gold_predict": job_gold_predict,
    "simulation_daily": job_simulation_daily,
    "predict_vn30": job_predict_vn30,
    "predict_nasdaq": job_predict_nasdaq,
    "predict_crypto": job_predict_crypto,
    "predict_fuel": job_predict_fuel,
    "predict_sp500": job_predict_sp500,
}
