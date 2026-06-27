"""Version stamping — reads build-time env vars and writes /versions/prediction-svc.json."""
from __future__ import annotations

import json
import os
from datetime import datetime

from src.utils.logger import get_logger

_log = get_logger("version")

_VERSION_DIR = "/versions"
_VERSION_FILE = f"{_VERSION_DIR}/prediction-svc.json"


def stamp_version() -> None:
    """Read GIT_SHA / BUILD_TIME / GIT_DIRTY, write version file, emit startup log line.

    If the /versions directory is not mounted or the write fails for any reason
    the function logs a WARNING and returns — it must never crash the service.
    """
    git_sha = os.environ.get("GIT_SHA", "unknown")
    build_time = os.environ.get("BUILD_TIME", "unknown")
    git_dirty = os.environ.get("GIT_DIRTY", "unknown")

    # (c) Log exactly one startup line.
    _log.info("version", git_sha=git_sha, build_time=build_time, dirty=git_dirty)

    # (a) Write JSON version file.
    started_at = datetime.now().isoformat()

    payload = {
        "service": "prediction-svc",
        "git_sha": git_sha,
        "build_time": build_time,
        "dirty": git_dirty,
        "started_at": started_at,
    }

    try:
        os.makedirs(_VERSION_DIR, exist_ok=True)
        with open(_VERSION_FILE, "w", encoding="utf-8") as fh:
            json.dump(payload, fh)
    except Exception as exc:  # noqa: BLE001
        _log.warning("version.file.write_failed", path=_VERSION_FILE, error=str(exc))
