"""Run-once CLI entrypoint cho k8s Job/CronJob (Cách 2 — k8s-native).

Chạy pipeline MỘT market (crawl → [train mỗi lần thứ 10] → predict → reconcile)
đúng một lượt rồi THOÁT — KHÔNG khởi động gRPC server, KHÔNG APScheduler.
Pod của Job/CronJob dùng entrypoint này là "worker" tự làm việc thật, khác với
việc gọi trigger endpoint (Cách 1). Xem deploy/k8s/pipeline/.

Tái dùng nguyên các job_crawl_* trong src.scheduler.jobs (không nhân đôi logic).
Init tối thiểu giống main.py: config → timezone (ICT) → logger → DB.

Dùng:
    python -m src.jobs_cli <market>        # market: gold | nasdaq | crypto | sp500

Exit code:
    0 = chạy xong (Job Complete)
    1 = lỗi bất ngờ (DB down, import fail, ... → Job Failed → backoffLimit retry)
    2 = sai tham số
"""
from __future__ import annotations

import os
import sys
import time

# Tên market (CLI, khớp quy ước gold/nasdaq/crypto/sp500) -> job_key trong JOB_FUNCTIONS
_MARKET_JOB = {
    "gold": "crawler_gold",
    "nasdaq": "crawler_nasdaq",
    "crypto": "crawler_crypto",
    "sp500": "crawler_sp500",
}


def main(argv: list[str]) -> int:
    if len(argv) != 1 or argv[0] not in _MARKET_JOB:
        sys.stderr.write(f"usage: python -m src.jobs_cli <{'|'.join(_MARKET_JOB)}>\n")
        return 2
    market = argv[0]

    # 1. Config
    from src.config import get_settings
    cfg = get_settings()

    # 2. Timezone (ICT-at-rest — mirror main.py: luôn datetime.now theo giờ container)
    os.environ.setdefault("TZ", "Asia/Ho_Chi_Minh")
    try:
        time.tzset()
    except AttributeError:
        pass  # Windows

    # 3. Logger
    from src.utils.logger import get_logger, init_logger
    init_logger(cfg.log_level)
    log = get_logger("jobs_cli")

    # 4. DB (KHÔNG gRPC, KHÔNG scheduler — chỉ cần DB cho pipeline)
    from src.database.connection import init_db
    init_db()

    job_key = _MARKET_JOB[market]
    log.info("jobs_cli.start", market=market, job_key=job_key)
    try:
        from src.scheduler.jobs import JOB_FUNCTIONS
        JOB_FUNCTIONS[job_key]()   # = job_crawl_<market>: _run_pipeline + intraday
        log.info("jobs_cli.done", market=market)
        return 0
    except Exception as exc:
        log.error("jobs_cli.error", market=market, error=str(exc))
        return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
