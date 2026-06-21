"""Python Prediction Service — entry point.

Startup sequence (mirrors Go cmd/prediction/main.go):
1. Load config
2. Set timezone
3. Init logger
4. Init DB
5. Start gRPC server
6. Init scheduler
7. Startup data sync (5s delay, crawl once)
8. Wait for SIGTERM/SIGINT → graceful shutdown
"""
from __future__ import annotations

import os
import signal
import sys
import threading
import time

# ---------------------------------------------------------------------------
# 1. Config
# ---------------------------------------------------------------------------
from src.config import get_settings

cfg = get_settings()

# ---------------------------------------------------------------------------
# 2. Timezone (set TZ env var; Python datetime uses it)
# ---------------------------------------------------------------------------
os.environ.setdefault("TZ", "Asia/Ho_Chi_Minh")
try:
    time.tzset()
except AttributeError:
    pass  # Windows doesn't have tzset

# ---------------------------------------------------------------------------
# 3. Logger
# ---------------------------------------------------------------------------
from src.utils.logger import get_logger, init_logger

init_logger(cfg.log_level)
log = get_logger("main")

log.info("service.starting", grpc_port=cfg.grpc_server_port)

# ---------------------------------------------------------------------------
# 4. Database
# ---------------------------------------------------------------------------
from src.database.connection import init_db

init_db()
log.info("database.ready")

# ---------------------------------------------------------------------------
# 5. gRPC server
# ---------------------------------------------------------------------------
from src.grpc_server.server import start_grpc_server, stop_grpc_server

start_grpc_server(cfg.grpc_server_port)
log.info("grpc.ready", port=cfg.grpc_server_port)

# ---------------------------------------------------------------------------
# 5b. Service registry (optional — gated by SERVICE_MGT_ENABLED)
# ---------------------------------------------------------------------------
from src.registry_client import start as registry_start, stop as registry_stop

registry_start(
    enabled=cfg.service_mgt_enabled,
    target=cfg.registry_grpc_target,
    service_name="prediction-svc",
    address="prediction-svc",
    port=cfg.grpc_server_port,
)

# ---------------------------------------------------------------------------
# 6. Scheduler
# ---------------------------------------------------------------------------
from src.scheduler.jobs import JOB_FUNCTIONS
from src.scheduler.manager import init_scheduler, shutdown_scheduler

init_scheduler(JOB_FUNCTIONS)
log.info("scheduler.ready")

# ---------------------------------------------------------------------------
# 7. Startup data sync
# ---------------------------------------------------------------------------

def _startup_sync():
    time.sleep(5)
    log.info("startup.sync.begin")

    # Seed simulation bots
    try:
        from src.simulation.seeder import seed_bots
        inserted = seed_bots()
        log.info("startup.sync.sim_bots.done", inserted=inserted)
    except Exception as exc:
        log.warning("startup.sync.sim_bots.error", error=str(exc))


def _startup_reconcile():
    time.sleep(10)
    log.info("startup.reconcile.begin")
    try:
        from src.orchestrator.training import reconcile_predictions
        updated = reconcile_predictions()
        log.info("startup.reconcile.done", updated=updated)
    except Exception as exc:
        log.warning("startup.reconcile.error", error=str(exc))


threading.Thread(target=_startup_sync, daemon=True).start()
threading.Thread(target=_startup_reconcile, daemon=True).start()

# ---------------------------------------------------------------------------
# 8. Wait for shutdown signal
# ---------------------------------------------------------------------------
_stop_event = threading.Event()


def _handle_signal(signum, frame):
    log.info("service.stopping", signal=signum)
    _stop_event.set()


signal.signal(signal.SIGTERM, _handle_signal)
signal.signal(signal.SIGINT, _handle_signal)

log.info("service.ready")
_stop_event.wait()

log.info("service.shutdown")
registry_stop()
stop_grpc_server()
shutdown_scheduler()
log.info("service.stopped")
sys.exit(0)
