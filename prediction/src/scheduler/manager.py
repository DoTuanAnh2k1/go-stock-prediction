"""APScheduler-based cron job manager with DB-backed schedules.

Cron format in DB: 6 fields (seconds, minutes, hours, day, month, weekday)
APScheduler CronTrigger: 5 fields (minutes, hours, day, month, day_of_week)
Conversion: strip the first field (seconds).
"""
from __future__ import annotations

import threading
import time
from collections.abc import Callable

from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.triggers.cron import CronTrigger

from src.database import repository as repo
from src.utils.logger import get_logger

log = get_logger("scheduler")

_scheduler: BackgroundScheduler | None = None
_job_functions: dict[str, Callable] = {}
_lock = threading.Lock()


# Default schedules (mirrors Go constants)
DEFAULT_SCHEDULES = [
    ("crawler_stock", "Crawl cổ phiếu VN30 (hàng ngày)", "0 0 12 * * *", True),
    ("crawler_sp500", "Pipeline S&P 500 (mỗi giờ, phút 30)", "0 30 * * * *", True),
    ("crawler_gold", "Pipeline Gold (mỗi giờ, phút 0)", "0 0 * * * *", True),
    ("gold_predict", "Dự đoán vàng (disabled — trong pipeline)", "0 0 11 * * *", False),
    ("crawler_nasdaq", "Pipeline NASDAQ (mỗi giờ, phút 15)", "0 15 * * * *", True),
    ("crawler_crypto", "Pipeline Crypto (mỗi giờ, phút 45)", "0 45 * * * *", True),
    ("weekly_training", "Huấn luyện mô hình (Chủ nhật 9AM)", "0 0 9 * * 0", False),
    # Per-market training jobs — staggered on Sunday to avoid overlap
    ("train_vn30",   "Training VN30 (Chủ nhật 2AM)",          "0 0 2 * * 0", True),
    ("train_gold",   "Training Gold (Chủ nhật 3AM)",           "0 0 3 * * 0", True),
    ("train_nasdaq", "Training NASDAQ (Chủ nhật 4AM)",         "0 0 4 * * 0", True),
    ("train_crypto", "Training Crypto (Chủ nhật 5AM)",         "0 0 5 * * 0", True),
    ("train_sp500",  "Training S&P 500 (Chủ nhật 7AM)",        "0 0 7 * * 0", True),
    ("daily_prediction", "Dự đoán tất cả thị trường (mỗi giờ)", "0 0 */1 * * *", False),
    ("predict_vn30",   "Dự đoán VN30 (3PM ngày thường)",          "0 0 15 * * 1-5",  True),
    ("predict_nasdaq", "Dự đoán NASDAQ (disabled — trong pipeline)",  "0 30 23 * * 1-5", False),
    ("predict_crypto", "Dự đoán Crypto (disabled — trong pipeline)", "0 0 */6 * * *",   False),
    ("predict_sp500",  "Dự đoán S&P 500 (disabled — trong pipeline)", "0 0 13 * * 1-5",  False),
    ("daily_reconcile", "Reconcile dự đoán (6AM hàng ngày)", "0 0 6 * * *", True),
    ("daily_backup", "Backup database (3AM hàng ngày)", "0 0 3 * * *", True),
]


def parse_6field_cron(expr: str) -> dict:
    """Convert 6-field cron (Go format) to APScheduler CronTrigger kwargs.

    Go robfig/cron: <second> <minute> <hour> <day> <month> <weekday>
    APScheduler:    <minute> <hour> <day> <month> <day_of_week>

    We strip the first field (seconds) and map the rest.
    """
    parts = expr.strip().split()
    if len(parts) == 6:
        # 6-field format: drop seconds field
        _, minute, hour, day, month, weekday = parts
    elif len(parts) == 5:
        # Already 5-field format
        minute, hour, day, month, weekday = parts
    else:
        raise ValueError(f"Invalid cron expression: {expr!r}")

    # Convert Go weekday notation (0=Sunday) to APScheduler (0=Monday)
    # APScheduler uses: mon=0..sun=6 OR mon,tue,...,sun
    # Go uses: 0=SUN, 1=MON, ..., 6=SAT
    # "1-5" in Go = Mon-Fri = "mon-fri" in APScheduler notation
    # We pass it as-is — APScheduler handles "1-5" as "mon-fri" (0-based Monday=0)
    # Actually APScheduler's day_of_week: 0=mon, 6=sun; Go: 0=sun, 1=mon...6=sat
    # Map: go_weekday → aps_weekday: sun(0)→6, mon(1)→0, ..., sat(6)→5
    # For "1-5" (Mon-Fri in Go) → "0-4" in APScheduler
    # Handle common patterns:
    if weekday != "*":
        def _convert_day(d: str) -> str:
            mapping = {"0": "6", "1": "0", "2": "1", "3": "2", "4": "3", "5": "4", "6": "5",
                       "SUN": "6", "MON": "0", "TUE": "1", "WED": "2", "THU": "3", "FRI": "4", "SAT": "5"}
            # Try numeric or named
            if "-" in d:
                a, b = d.split("-", 1)
                return f"{_convert_day(a)}-{_convert_day(b)}"
            return mapping.get(d.upper(), d)

        weekday = _convert_day(weekday)

    return dict(minute=minute, hour=hour, day=day, month=month, day_of_week=weekday)


def _wait_for_db(max_retries: int = 10, delay: float = 3.0) -> None:
    """Wait for DB to become available, retrying with backoff."""
    for attempt in range(1, max_retries + 1):
        try:
            repo.get_all_cron_schedules()
            return
        except Exception as exc:
            if attempt == max_retries:
                log.error("scheduler.db.unavailable", attempts=max_retries, error=str(exc))
                raise
            log.warning("scheduler.db.waiting", attempt=attempt, retry_in=delay, error=str(exc))
            time.sleep(delay)


def init_scheduler(job_functions: dict[str, Callable]) -> None:
    """Initialize APScheduler and register all jobs from DB."""
    global _scheduler, _job_functions

    _job_functions = job_functions

    _scheduler = BackgroundScheduler(timezone="Asia/Ho_Chi_Minh")

    # Wait for DB to be ready before seeding/loading
    _wait_for_db()

    # Seed default schedules to DB
    for job_key, job_name, cron_expr, enabled in DEFAULT_SCHEDULES:
        try:
            repo.upsert_cron_schedule(job_key, job_name, cron_expr, enabled)
        except Exception as exc:
            log.warning("scheduler.seed.error", job_key=job_key, error=str(exc))

    # Load all schedules from DB and register jobs
    schedules = repo.get_all_cron_schedules()
    for schedule in schedules:
        if schedule.enabled and schedule.job_key in job_functions:
            _add_job(schedule.job_key, schedule.cron_expression, job_functions[schedule.job_key])
        # Pre-populate _schedule_state so the first _watch_schedule_changes poll
        # doesn't treat every job as "changed" and reset their next fire times.
        _schedule_state[schedule.job_key] = {
            "cron": schedule.cron_expression,
            "enabled": schedule.enabled,
        }

    # Watch for changes every 60 seconds
    _scheduler.add_job(
        _watch_schedule_changes,
        trigger=CronTrigger(minute="*"),
        id="__schedule_watcher__",
        replace_existing=True,
        max_instances=1,
    )

    _scheduler.start()
    log.info("scheduler.started", jobs=len(_scheduler.get_jobs()))


def _add_job(job_key: str, cron_expr: str, fn: Callable) -> bool:
    try:
        trigger_kwargs = parse_6field_cron(cron_expr)
        _scheduler.add_job(
            fn,
            trigger=CronTrigger(**trigger_kwargs),
            id=job_key,
            replace_existing=True,
            max_instances=1,
            misfire_grace_time=300,
        )
        log.info("scheduler.job.added", job_key=job_key, cron=cron_expr)
        return True
    except Exception as exc:
        log.error("scheduler.job.add_failed", job_key=job_key, error=str(exc))
        return False


def _remove_job(job_key: str) -> None:
    try:
        _scheduler.remove_job(job_key)
        log.info("scheduler.job.removed", job_key=job_key)
    except Exception:
        pass


# Pre-populate from DEFAULT_SCHEDULES so the first _watch_schedule_changes
# poll (which fires at the next whole minute after startup) never sees "old: {}"
# for jobs that were already registered. The DB-sourced values loaded in
# init_scheduler() will overwrite these defaults, so the watcher only fires
# for genuine changes made after startup.
_schedule_state: dict[str, dict] = {
    job_key: {"cron": cron_expr, "enabled": enabled}
    for job_key, _job_name, cron_expr, enabled in DEFAULT_SCHEDULES
}


def _watch_schedule_changes() -> None:
    """Poll DB for schedule changes and reschedule jobs as needed."""
    global _schedule_state

    try:
        schedules = repo.get_all_cron_schedules()
        for schedule in schedules:
            key = schedule.job_key
            prev = _schedule_state.get(key, {})
            current = {"cron": schedule.cron_expression, "enabled": schedule.enabled}

            if prev == current:
                continue

            log.info("scheduler.schedule_changed", job_key=key, old=prev, new=current)

            if schedule.enabled and key in _job_functions:
                _add_job(key, schedule.cron_expression, _job_functions[key])
            else:
                _remove_job(key)

            _schedule_state[key] = current

    except Exception as exc:
        log.warning("scheduler.watch.error", error=str(exc))


def shutdown_scheduler() -> None:
    global _scheduler
    if _scheduler and _scheduler.running:
        _scheduler.shutdown(wait=False)
        log.info("scheduler.stopped")
