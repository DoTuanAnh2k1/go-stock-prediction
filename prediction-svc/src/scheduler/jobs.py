"""Cron job definitions registered with the scheduler."""
from __future__ import annotations

from datetime import datetime
from typing import Callable

from src.utils.logger import get_logger

log = get_logger("jobs")

# Maps market_key → pipeline_key (matches cron_schedules job_key in DB)
PIPELINE_KEY_MAP: dict[str, str] = {
    "GOLD": "crawler_gold",
    "NASDAQ100": "crawler_nasdaq",
    "SP500": "crawler_sp500",
    "CRYPTO": "crawler_crypto",
}


def _run_pipeline(market_key: str, crawl_fn: Callable) -> None:
    """Crawl → [train every 10th crawl] → predict pipeline.

    Steps:
    1. Run crawl_fn() to fetch and persist new data.
    2. Increment per-market crawl counter; on every 10th crawl, trigger
       train_for_market() to rebuild models from the latest data.
    3. Run run_for_market() to generate fresh predictions.

    Aborts the full pipeline if crawl fails (no point predicting stale data).
    Skips entirely if the market is closed (weekend/holiday for NASDAQ/SP500).
    A PipelineReport row is written to DB at the end of every branch.
    """
    from src.database.repository import (
        create_pipeline_report,
        delete_old_pipeline_reports,
        increment_crawl_count,
    )
    from src.utils.market_calendar import is_market_open

    pipeline_key = PIPELINE_KEY_MAP.get(market_key, market_key.lower())
    started_at = datetime.now()
    steps: list[dict] = []
    crawled_count = 0
    predictions_count = 0
    trained = False
    status = "success"
    error_msg: str | None = None

    # Step 0: Skip closed markets (NASDAQ/SP500 cuối tuần & ngày lễ US)
    if not is_market_open(market_key):
        log.info("pipeline.skip.market_closed", market=market_key)
        steps.append({"label": "Skip", "status": "skipped", "detail": "market closed"})
        finished_at = datetime.now()
        try:
            create_pipeline_report(
                pipeline_key=pipeline_key,
                market=market_key,
                status="skipped",
                started_at=started_at,
                finished_at=finished_at,
                duration_ms=int((finished_at - started_at).total_seconds() * 1000),
                crawled_count=0,
                predictions_count=0,
                trained=False,
                steps=steps,
                error=None,
            )
            delete_old_pipeline_reports(7)
        except Exception as exc:
            log.warning("pipeline.report.write.error", market=market_key, error=str(exc))
        return

    # Step 1: Crawl
    try:
        saved = crawl_fn()
        crawled_count = saved if isinstance(saved, int) else 0
        log.info("pipeline.crawl.done", market=market_key, saved=saved)
        steps.append({"label": "Crawl", "status": "success", "detail": f"saved {saved}"})
    except Exception as exc:
        log.error("pipeline.crawl.error", market=market_key, error=str(exc))
        error_msg = str(exc)
        steps.append({"label": "Crawl", "status": "failed", "detail": str(exc)})
        finished_at = datetime.now()
        try:
            create_pipeline_report(
                pipeline_key=pipeline_key,
                market=market_key,
                status="failed",
                started_at=started_at,
                finished_at=finished_at,
                duration_ms=int((finished_at - started_at).total_seconds() * 1000),
                crawled_count=0,
                predictions_count=0,
                trained=False,
                steps=steps,
                error=error_msg,
            )
            delete_old_pipeline_reports(7)
        except Exception as rep_exc:
            log.warning("pipeline.report.write.error", market=market_key, error=str(rep_exc))
        return  # Abort if crawl fails

    # Step 2: Increment counter (DB-backed, survives restarts) and train every 10th crawl
    try:
        crawl_num = increment_crawl_count(market_key)
    except Exception as exc:
        log.warning("pipeline.crawl_count.error", market=market_key, error=str(exc))
        crawl_num = 0  # 0 % 10 != 0 → skip training this run, never crash the pipeline
    log.info("pipeline.crawl_count", market=market_key, count=crawl_num)

    if crawl_num % 10 == 0:
        log.info("pipeline.training.start", market=market_key, crawl=crawl_num)
        try:
            from src.orchestrator.training import train_for_market
            success, sid = train_for_market(market_key)
            trained = True
            log.info("pipeline.training.done", market=market_key, success=success, session_id=sid)
            steps.append({"label": "Train", "status": "success", "detail": f"session_id={sid}"})
        except Exception as exc:
            log.warning("pipeline.training.error", market=market_key, error=str(exc))
            steps.append({"label": "Train", "status": "failed", "detail": str(exc)})
            # Training failure → pipeline continues but marks partial
            if status == "success":
                status = "partial"

    # Step 3: Predict (run_for_market also triggers sim step internally)
    try:
        from src.orchestrator.runner import run_for_market
        n = run_for_market(market_key)
        predictions_count = n if isinstance(n, int) else 0
        log.info("pipeline.predict.done", market=market_key, predictions=n)
        steps.append({"label": "Predict", "status": "success", "detail": f"{n} predictions"})
    except Exception as exc:
        log.error("pipeline.predict.error", market=market_key, error=str(exc))
        steps.append({"label": "Predict", "status": "failed", "detail": str(exc)})
        if error_msg is None:
            error_msg = str(exc)
        # Crawl succeeded but predict failed → partial if training ran, failed otherwise
        status = "partial" if trained or crawled_count > 0 else "failed"

    # Write report — wrap entirely so DB errors never break the pipeline
    try:
        finished_at = datetime.now()
        create_pipeline_report(
            pipeline_key=pipeline_key,
            market=market_key,
            status=status,
            started_at=started_at,
            finished_at=finished_at,
            duration_ms=int((finished_at - started_at).total_seconds() * 1000),
            crawled_count=crawled_count,
            predictions_count=predictions_count,
            trained=trained,
            steps=steps,
            error=error_msg,
        )
        delete_old_pipeline_reports(7)
    except Exception as exc:
        log.warning("pipeline.report.write.error", market=market_key, error=str(exc))


def job_crawl_gold() -> None:
    from src.crawlers.gold import GoldCrawler
    crawler = GoldCrawler()
    _run_pipeline("GOLD", lambda: crawler.crawl())
    try:
        crawler.crawl_intraday()
    except Exception as exc:
        log.warning("job.crawl_gold.intraday.error", error=str(exc))


def job_crawl_nasdaq() -> None:
    from src.crawlers.nasdaq import NasdaqCrawler
    crawler = NasdaqCrawler()
    _run_pipeline("NASDAQ100", lambda: crawler.crawl())
    try:
        crawler.crawl_intraday()
    except Exception as exc:
        log.warning("job.crawl_nasdaq.intraday.error", error=str(exc))


def job_crawl_crypto() -> None:
    from src.crawlers.crypto import CryptoCrawler
    crawler = CryptoCrawler()
    _run_pipeline("CRYPTO", lambda: crawler.crawl())
    try:
        crawler.crawl_intraday()
    except Exception as exc:
        log.warning("job.crawl_crypto.intraday.error", error=str(exc))


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
    crawler = SP500Crawler()
    _run_pipeline("SP500", lambda: crawler.crawl())
    try:
        crawler.crawl_intraday()
    except Exception as exc:
        log.warning("job.crawl_sp500.intraday.error", error=str(exc))


def job_gold_predict() -> None:
    from src.orchestrator.runner import run_for_market
    try:
        count = run_for_market("GOLD")
        log.info("job.gold_predict.done", count=count)
    except Exception as exc:
        log.error("job.gold_predict.error", error=str(exc))


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


def job_predict_sp500() -> None:
    from src.orchestrator.runner import run_for_market
    try:
        count = run_for_market("SP500")
        log.info("job.predict_sp500.done", count=count)
    except Exception as exc:
        log.error("job.predict_sp500.error", error=str(exc))


# ---------------------------------------------------------------------------
# Per-market training jobs
# ---------------------------------------------------------------------------

def job_train_gold() -> None:
    from src.orchestrator.training import train_for_market
    try:
        success, sid = train_for_market("GOLD")
        log.info("job.train_gold.done", success=success, session_id=sid)
    except Exception as exc:
        log.error("job.train_gold.error", error=str(exc))


def job_train_nasdaq() -> None:
    from src.orchestrator.training import train_for_market
    try:
        success, sid = train_for_market("NASDAQ100")
        log.info("job.train_nasdaq.done", success=success, session_id=sid)
    except Exception as exc:
        log.error("job.train_nasdaq.error", error=str(exc))


def job_train_crypto() -> None:
    from src.orchestrator.training import train_for_market
    try:
        success, sid = train_for_market("CRYPTO")
        log.info("job.train_crypto.done", success=success, session_id=sid)
    except Exception as exc:
        log.error("job.train_crypto.error", error=str(exc))


def job_train_sp500() -> None:
    from src.orchestrator.training import train_for_market
    try:
        success, sid = train_for_market("SP500")
        log.info("job.train_sp500.done", success=success, session_id=sid)
    except Exception as exc:
        log.error("job.train_sp500.error", error=str(exc))


# Note: the database backup job lives in the Go API service
# (api/pkg/server/backup_scheduler.go). The API owns the daily_backup schedule
# and runs mysqldump itself, so the prediction service no longer performs backups.


# Job registry — maps job_key → callable
JOB_FUNCTIONS = {
    "crawler_gold": job_crawl_gold,
    "crawler_nasdaq": job_crawl_nasdaq,
    "crawler_crypto": job_crawl_crypto,
    "crawler_sp500": job_crawl_sp500,
    "weekly_training": job_weekly_training,
    "daily_prediction": job_daily_prediction,
    "daily_reconcile": job_daily_reconcile,
    "gold_predict": job_gold_predict,
    "predict_nasdaq": job_predict_nasdaq,
    "predict_crypto": job_predict_crypto,
    "predict_sp500": job_predict_sp500,
    # Per-market training jobs
    "train_gold": job_train_gold,
    "train_nasdaq": job_train_nasdaq,
    "train_crypto": job_train_crypto,
    "train_sp500": job_train_sp500,
}
