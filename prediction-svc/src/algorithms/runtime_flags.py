"""Runtime feature flags — global toggles shared across algorithm instances.

These flags are intentionally module-level globals protected by a lock so they
can be safely read from many threads (e.g. ThreadPoolExecutor in rebuild.py)
while being written once on the main thread before the pool is started.

Current flags
-------------
optuna_enabled : bool  (default True)
    When False, all Optuna hyperparameter searches in lightgbm_model.py and
    xgboost_model.py are skipped and the functions return default params
    immediately.  The model is still trained with those defaults — only the
    search phase is bypassed.

    rebuild_and_replay sets this to False in fast-mode (default) and restores
    it in a finally-block so normal pipeline runs are never affected.
"""
from __future__ import annotations

import threading

_lock = threading.Lock()
_optuna_enabled: bool = True


def set_optuna_enabled(v: bool) -> None:
    """Set the global Optuna-enabled flag (thread-safe write)."""
    global _optuna_enabled
    with _lock:
        _optuna_enabled = v


def is_optuna_enabled() -> bool:
    """Return the current Optuna-enabled flag (thread-safe read)."""
    return _optuna_enabled
