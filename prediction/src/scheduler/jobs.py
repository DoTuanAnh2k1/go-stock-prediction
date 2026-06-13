"""Cron job definitions registered with the scheduler."""
from __future__ import annotations

from typing import Callable

from src.utils.logger import get_logger

log = get_logger("jobs")

# Crawl counter per market — triggers retraining every 10 crawls
_crawl_counts: dict[str, int] = {}


def _run_pipeline(market_key: str, crawl_fn: Callable) -> None:
    """Crawl → [train every 10th crawl] → predict pipeline.

    Steps:
    1. Run crawl_fn() to fetch and persist new data.
    2. Increment per-market crawl counter; on every 10th crawl, trigger
       train_for_market() to rebuild models from the latest data.
    3. Run run_for_market() to generate fresh predictions.

    Aborts the full pipeline if crawl fails (no point predicting stale data).
    Skips entirely if the market is closed (weekend/holiday for NASDAQ/SP500).
    """
    # Step 0: Skip closed markets (NASDAQ/SP500 cuối tuần & ngày lễ US)
    from src.utils.market_calendar import is_market_open
    if not is_market_open(market_key):
        log.info("pipeline.skip.market_closed", market=market_key)
        return

    # Step 1: Crawl
    try:
        saved = crawl_fn()
        log.info("pipeline.crawl.done", market=market_key, saved=saved)
    except Exception as exc:
        log.error("pipeline.crawl.error", market=market_key, error=str(exc))
        return  # Abort if crawl fails

    # Step 2: Increment counter and train every 10th crawl
    _crawl_counts[market_key] = _crawl_counts.get(market_key, 0) + 1
    crawl_num = _crawl_counts[market_key]
    log.info("pipeline.crawl_count", market=market_key, count=crawl_num)

    if crawl_num % 10 == 0:
        log.info("pipeline.training.start", market=market_key, crawl=crawl_num)
        try:
            from src.orchestrator.training import train_for_market
            success, sid = train_for_market(market_key)
            log.info("pipeline.training.done", market=market_key, success=success, session_id=sid)
        except Exception as exc:
            log.warning("pipeline.training.error", market=market_key, error=str(exc))

    # Step 3: Predict (run_for_market also triggers sim step internally)
    try:
        from src.orchestrator.runner import run_for_market
        n = run_for_market(market_key)
        log.info("pipeline.predict.done", market=market_key, predictions=n)
    except Exception as exc:
        log.error("pipeline.predict.error", market=market_key, error=str(exc))


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


def job_backup_database() -> None:
    """Create a compressed mysqldump backup of the database."""
    import subprocess
    import os
    import datetime

    from src.config import get_settings
    settings = get_settings()

    backup_dir = os.environ.get("BACKUP_DIR", "/backups")
    os.makedirs(backup_dir, exist_ok=True)

    ts = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
    filename = f"backup_{ts}.sql.gz"
    filepath_out = os.path.join(backup_dir, filename)

    try:
        dump_cmd = [
            "mysqldump",
            f"--host={settings.mysql_host}",
            f"--port={settings.mysql_port}",
            f"--user={settings.mysql_user}",
            f"--password={settings.mysql_password}",
            "--single-transaction",
            "--routines",
            "--triggers",
            settings.mysql_db_name,
        ]

        with open(filepath_out, "wb") as f:
            dump_proc = subprocess.Popen(dump_cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            gzip_proc = subprocess.Popen(["gzip", "-c"], stdin=dump_proc.stdout, stdout=f, stderr=subprocess.PIPE)
            dump_proc.stdout.close()
            gzip_proc.wait()
            dump_proc.wait()

            if dump_proc.returncode != 0:
                _, stderr_data = dump_proc.communicate()
                raise RuntimeError(f"mysqldump failed (rc={dump_proc.returncode}): {stderr_data.decode()}")

        size = os.path.getsize(filepath_out)
        log.info("job.backup.done", filename=filename, size_bytes=size)

        # Keep only the 10 most recent backups
        _cleanup_old_backups(backup_dir, keep=10)

    except Exception as exc:
        log.error("job.backup.error", error=str(exc))
        if os.path.exists(filepath_out):
            os.remove(filepath_out)


def _cleanup_old_backups(backup_dir: str, keep: int = 10) -> None:
    """Remove old backups, keeping only the `keep` most recent files."""
    try:
        files = sorted(
            [f for f in os.listdir(backup_dir) if f.startswith("backup_") and f.endswith(".sql.gz")],
            reverse=True,
        )
        for old_file in files[keep:]:
            os.remove(os.path.join(backup_dir, old_file))
            log.info("job.backup.cleanup", removed=old_file)
    except Exception as exc:
        log.warning("job.backup.cleanup.error", error=str(exc))


def job_simulation_daily() -> None:
    """Run one live simulation step for all active trading bots."""
    from src.simulation.engine import SimulationEngine
    log.info("job.simulation_daily.start")
    try:
        engine = SimulationEngine()
        engine.run_live_step()
        log.info("job.simulation_daily.done")
    except Exception as exc:
        log.error("job.simulation_daily.error", error=str(exc))


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
    "daily_backup": job_backup_database,
    "simulation_daily": job_simulation_daily,
}
