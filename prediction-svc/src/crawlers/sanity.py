"""Ingest sanity guards — reject transient tick garbage before it hits the DB.

Two public APIs:

check_update(market, key, new_price, last_price)
    Gate for daily-live upserts.  Uses a two-strike "persistence confirmation"
    model: a price that moves outside the market-aware band is held in
    _pending and only accepted on the *second* consecutive crawl that reports
    the same out-of-band level (confirming a real split/gap rather than a
    transient bad tick).

batch_outlier_mask(prices, market)
    Gate for intraday bar sequences.  Flags isolated spikes using bilateral
    neighbor comparison — a bar is flagged only when it deviates from BOTH
    the previous bar AND the next bar beyond the threshold, which means real
    step-changes (splits, genuine gaps) are never flagged because the
    post-split bars are consistent with each other.
"""
from __future__ import annotations

import os
from typing import Optional

from src.utils.logger import get_logger

log = get_logger("crawl.sanity")

# ---------------------------------------------------------------------------
# Market-aware deviation thresholds
# ---------------------------------------------------------------------------

_DEFAULT_MAX_DEVIATION: dict[str, float] = {
    "GOLD": 0.30,
    "NASDAQ": 0.40,
    "NASDAQ100": 0.40,
    "SP500": 0.40,
    "CRYPTO": 0.80,
}

_FALLBACK_DEVIATION = 0.40


def max_deviation(market: str) -> float:
    """Return the maximum allowed single-update relative deviation for *market*.

    Reads ``CRAWL_MAX_TICK_DEVIATION_<MARKET>`` (uppercase) from the environment
    first; falls back to the hard-coded defaults.  Unknown markets default to 0.40.
    """
    upper = market.upper()
    env_key = f"CRAWL_MAX_TICK_DEVIATION_{upper}"
    env_val = os.environ.get(env_key)
    if env_val is not None:
        try:
            return float(env_val)
        except ValueError:
            log.warning(
                "crawl.sanity.bad_env_threshold",
                env_key=env_key,
                value=env_val,
            )
    return _DEFAULT_MAX_DEVIATION.get(upper, _FALLBACK_DEVIATION)


# ---------------------------------------------------------------------------
# Module-level pending state — "persistence confirmation" buffer
# ---------------------------------------------------------------------------

# key → candidate out-of-band price waiting for confirmation.
# Use a dict so state can be inspected and reset in tests.
_pending: dict[str, float] = {}


# ---------------------------------------------------------------------------
# check_update — daily-live gate
# ---------------------------------------------------------------------------

def check_update(
    market: str,
    key: str,
    new_price: float,
    last_price: Optional[float],
    *,
    rel_tol: float = 0.02,
) -> tuple[bool, str]:
    """Decide whether *new_price* should be persisted for the given (market, key).

    The *key* should incorporate the market to avoid cross-market collisions,
    e.g. ``f"{market}:{symbol}"``.

    Parameters
    ----------
    market:
        Market identifier string (case-insensitive), e.g. ``"NASDAQ"``.
    key:
        Unique identifier for the time-series being updated.
    new_price:
        The incoming price from the latest crawl.
    last_price:
        The most recent price already stored in the DB.  Pass ``None`` (or a
        non-positive value) for symbols that have no existing row (bootstrap).
    rel_tol:
        Two candidate values are considered "the same" when their relative
        difference is within this tolerance.  Default 0.02 (2%).

    Returns
    -------
    (accept: bool, reason: str)
        ``accept=True``  → persist the update as usual.
        ``accept=False`` → skip the write; the caller should log a warning.

    Reason strings
    --------------
    ``"nonpositive"``       — new_price <= 0 (invalid data)
    ``"bootstrap"``         — no prior price; accept unconditionally
    ``"in_band"``           — deviation within threshold; normal update
    ``"confirmed"``         — out-of-band but matches a pending candidate → real move
    ``"suspect_rejected"``  — out-of-band first occurrence; held pending
    """
    # Guard: zero or negative price is always bad data.
    if new_price <= 0:
        _pending.pop(key, None)
        return False, "nonpositive"

    # Bootstrap: no prior price exists; accept unconditionally.
    if last_price is None or last_price <= 0:
        _pending.pop(key, None)
        return True, "bootstrap"

    thr = max_deviation(market)
    dev = abs(new_price - last_price) / last_price

    # In-band: normal everyday movement.
    if dev <= thr:
        _pending.pop(key, None)
        return True, "in_band"

    # Out-of-band: check persistence confirmation.
    pending_val = _pending.get(key)
    if pending_val is not None:
        # Compare new_price with the previously-cached candidate.
        candidate_dev = abs(new_price - pending_val) / pending_val
        if candidate_dev <= rel_tol:
            # Two consecutive crawls agree on this out-of-band level → real move.
            _pending.pop(key, None)
            return True, "confirmed"

    # First occurrence (or new candidate differs from old pending): hold in buffer.
    _pending[key] = new_price
    return False, "suspect_rejected"


# ---------------------------------------------------------------------------
# batch_outlier_mask — intraday sequence gate
# ---------------------------------------------------------------------------

def batch_outlier_mask(prices: list[float], market: str) -> list[bool]:
    """Return a boolean mask flagging isolated spike bars.

    A bar at index *i* is flagged ``True`` only when:
      - it has valid (> 0) neighbours on **both** sides, AND
      - its relative deviation from the *previous* bar exceeds the threshold, AND
      - its relative deviation from the *next* bar exceeds the threshold.

    This bilateral test is deliberate and critical.  Consider a 4:1 stock
    split at index j where prices go ``[762, 762, 194, 194, ...]``:
      - Index j (194): prev=762 → big deviation, but next=194 → small deviation
        → NOT flagged (correct: this is the real new level).
      - A transient garbage bar ``[193, 772, 197]``: prev=193, next=197
        → both deviations large → flagged (correct: isolated spike).

    Endpoints (i=0 or i=n-1) and bars with a zero/None neighbour return
    ``False`` (insufficient context → no false positive).
    """
    n = len(prices)
    mask = [False] * n
    if n < 3:
        return mask

    thr = max_deviation(market)

    for i in range(1, n - 1):
        p = prices[i]
        prev = prices[i - 1]
        nxt = prices[i + 1]

        # Need positive values on both sides to compute deviations.
        if not (p > 0 and prev > 0 and nxt > 0):
            continue

        dev_prev = abs(p - prev) / prev
        dev_next = abs(p - nxt) / nxt

        if dev_prev > thr and dev_next > thr:
            mask[i] = True

    return mask
