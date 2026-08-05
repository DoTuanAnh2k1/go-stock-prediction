"""Run-once CLI entrypoint cho k8s Job/CronJob (Cách 2 — k8s-native).

Chạy MỘT job (crawl pipeline HOẶC train/reconcile/fundamentals/transformer/meta…)
đúng một lượt rồi THOÁT — KHÔNG khởi động gRPC server, KHÔNG APScheduler.
Pod của Job/CronJob dùng entrypoint này là "worker" tự làm việc thật, khác với
việc gọi trigger endpoint (Cách 1). Xem deploy/k8s/pipeline/.

Tái dùng nguyên các job_* trong src.scheduler.jobs (không nhân đôi logic) — đây
chính là cách "unify" scheduler: mọi lịch chạy (k8s CronJob lẫn cron container ở
docker-compose) đều gọi vào cùng JOB_FUNCTIONS, thay cho APScheduler in-app.
Init tối thiểu giống main.py: config → timezone (ICT) → logger → DB.

Dùng:
    python -m src.jobs_cli <market>        # alias: gold | nasdaq | crypto | sp500  → crawler_<market>
    python -m src.jobs_cli <job_key>       # bất kỳ key trong JOB_FUNCTIONS:
                                           #   train_gold, train_nasdaq, train_crypto, train_sp500,
                                           #   train_meta, crawler_fundamentals, train_transformer,
                                           #   daily_reconcile, ...
    python -m src.jobs_cli --list          # in ra mọi job_key hợp lệ

Exit code:
    0 = chạy xong (Job Complete)
    1 = lỗi bất ngờ (DB down, import fail, ... → Job Failed → backoffLimit retry)
    2 = sai tham số
"""
from __future__ import annotations

import os
import sys
import time

# Timezone (ICT-at-rest): set TZ TRƯỚC mọi project import, để bất kỳ datetime.now()
# nào (kể cả lúc import) đều theo giờ container — mirror main.py.
os.environ.setdefault("TZ", "Asia/Ho_Chi_Minh")
try:
    time.tzset()
except AttributeError:
    pass  # Windows không có tzset

from src.config import get_settings
from src.database.connection import init_db
from src.scheduler.jobs import JOB_FUNCTIONS
from src.utils.logger import get_logger, init_logger

# Alias thân thiện (khớp quy ước gold/nasdaq/crypto/sp500) -> job_key trong JOB_FUNCTIONS.
# Ngoài alias, chấp nhận trực tiếp bất kỳ job_key nào có trong JOB_FUNCTIONS.
_MARKET_JOB = {
    "gold": "crawler_gold",
    "nasdaq": "crawler_nasdaq",
    "crypto": "crawler_crypto",
    "sp500": "crawler_sp500",
}


def _resolve_job_key(arg: str) -> str | None:
    """Map CLI arg -> job_key. Ưu tiên alias market; sau đó nhận key trực tiếp."""
    if arg in _MARKET_JOB:
        return _MARKET_JOB[arg]
    if arg in JOB_FUNCTIONS:
        return arg
    return None


def main(argv: list[str]) -> int:
    if len(argv) == 1 and argv[0] in ("--list", "-l"):
        keys = sorted(set(_MARKET_JOB) | set(JOB_FUNCTIONS))
        sys.stdout.write("valid jobs:\n  " + "\n  ".join(keys) + "\n")
        return 0

    if len(argv) != 1 or _resolve_job_key(argv[0]) is None:
        valid = sorted(set(_MARKET_JOB) | set(JOB_FUNCTIONS))
        sys.stderr.write(
            "usage: python -m src.jobs_cli <job_key|gold|nasdaq|crypto|sp500>\n"
            "valid: " + ", ".join(valid) + "\n"
        )
        return 2

    arg = argv[0]
    job_key = _resolve_job_key(arg)

    cfg = get_settings()
    init_logger(cfg.log_level)
    log = get_logger("jobs_cli")
    init_db()  # chỉ cần DB cho job — KHÔNG gRPC, KHÔNG scheduler

    log.info("jobs_cli.start", arg=arg, job_key=job_key)
    try:
        JOB_FUNCTIONS[job_key]()  # job_crawl_* (pipeline) hoặc job_train_* / job_*
        log.info("jobs_cli.done", job_key=job_key)
        return 0
    except Exception as exc:
        log.error("jobs_cli.error", job_key=job_key, error=str(exc))
        return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
