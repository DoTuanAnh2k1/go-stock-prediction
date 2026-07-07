"""apply_pending_splits — idempotent history adjustment for recorded stock splits.

Called once after each full symbol crawl loop in NasdaqCrawler and SP500Crawler.
For each unapplied split we make the stored INTRADAY series continuous across the
split by dividing the pre-split segment by the ratio.

Why only intraday (and NOT daily / predictions):
- Yahoo's DAILY endpoint returns split-ADJUSTED history (old closes are already
  divided by the ratio). Dividing them again would DOUBLE-adjust and corrupt the
  daily-live table that the simulation reads. So daily is left untouched — it is
  already correct and self-heals on the next history crawl.
- Yahoo's INTRADAY (1h) endpoint returns UNADJUSTED prices, so bars backfilled
  before the split sit at pre-split scale while bars crawled after the split sit
  at post-split scale. The boundary between the two segments is NOT the ex-date
  (it depends on when each bar was crawled), so we DETECT it as the point where
  consecutive closes drop by ~ratio, then divide everything before it.
- Predictions are left untouched: pending pre-split predictions are transient
  (reconcile within ~1h) and the date boundary is unreliable — not worth the risk.

Design decisions / limitations:
- Closed cross-split trades (already-reconciled sim_trades with action=SELL) are
  NOT re-realized. Their PnL was computed in split-unadjusted prices and stays
  recorded as-is. Only open positions (held through split) are corrected at
  restore time via _restore_portfolio_state in engine.py.
- Position restore uses entry_date vs split_date (DATE granularity): a position
  opened ON the ex-date itself may be mis-classified; positions held for days
  across the split (the common case) are correct.
- Volume adjustment is skipped (only used for relative VWMA weighting).
- Gold and Crypto never have splits: this module only processes NASDAQ / SP500.
- Each split is wrapped in a single transaction (all-or-nothing per split).
"""
from __future__ import annotations

from datetime import datetime, time, timedelta

from sqlalchemy import text

from src.database.connection import session_scope
from src.database import repository as repo
from src.utils.logger import get_logger

log = get_logger("orchestrator.splits")

# Map repo market_key → (daily_table, intraday_table, pred_table)
_MARKET_TABLES: dict[str, tuple[str, str, str]] = {
    "NASDAQ": (
        "nasdaq_prices",
        "nasdaq_intraday_prices",
        "nasdaq_predictions",
    ),
    "SP500": (
        "sp500_prices",
        "sp500_intraday_prices",
        "sp500_predictions",
    ),
}


def _apply_one_split(split: dict) -> None:
    """Divide historical OHLC price rows by ratio for a single split.

    ``split`` is a dict from ``list_unapplied_splits()`` with keys:
        id, market_key, symbol, split_date (datetime.date), ratio (float)

    Rows are adjusted in a single transaction.  ``mark_split_applied`` is called
    inside the same transaction so applied_at is committed atomically with the
    price updates.
    """
    split_id = split["id"]
    market_key = split["market_key"].upper()
    symbol = split["symbol"]
    split_date = split["split_date"]          # datetime.date
    ratio = split["ratio"]                    # float

    if ratio <= 0:
        log.warning(
            "split.apply.skip.bad_ratio",
            split_id=split_id, symbol=symbol, ratio=ratio
        )
        return

    tables = _MARKET_TABLES.get(market_key)
    if tables is None:
        log.warning(
            "split.apply.skip.unknown_market",
            split_id=split_id, market=market_key, symbol=symbol
        )
        return

    _daily_tbl, intraday_tbl, _pred_tbl = tables

    # Search window around the ex-date for the unadjusted→adjusted transition.
    # The boundary is where consecutive closes drop by ~ratio (the split cliff);
    # it is NOT necessarily the ex-date midnight, so we detect it from the data.
    win_start = datetime.combine(split_date - timedelta(days=5), time.min)
    win_end = datetime.combine(split_date + timedelta(days=2), time.min)
    drop_floor = ratio * 0.75   # a genuine split drop; no real 1h move is this large

    with session_scope() as session:
        rows = session.execute(
            text(f"""
                SELECT timestamp, close_price FROM {intraday_tbl}
                WHERE symbol = :sym
                  AND timestamp BETWEEN :ws AND :we
                  AND close_price > 0
                ORDER BY timestamp ASC
            """),
            {"sym": symbol, "ws": win_start, "we": win_end},
        ).fetchall()

        # Find the LAST big drop (prev/cur >= ratio*0.75) → the split boundary.
        boundary_ts = None
        for i in range(1, len(rows)):
            prev = float(rows[i - 1].close_price)
            cur = float(rows[i].close_price)
            if cur > 0 and prev / cur >= drop_floor:
                boundary_ts = rows[i].timestamp

        if boundary_ts is None:
            # No split cliff found (series already consistent, or no data in window).
            # Mark applied so we don't rescan every crawl; adjust nothing.
            session.execute(
                text("UPDATE stock_splits SET applied_at = :now WHERE id = :sid"),
                {"now": datetime.now(), "sid": split_id},
            )
            log.warning(
                "split.apply.no_transition",
                split_id=split_id, market=market_key, symbol=symbol,
                split_date=str(split_date), ratio=ratio,
            )
            return

        # Divide every pre-boundary bar by ratio → whole series at post-split scale.
        upd = session.execute(
            text(f"""
                UPDATE {intraday_tbl}
                SET
                    open_price  = ROUND(open_price  / :ratio, 6),
                    high_price  = ROUND(high_price  / :ratio, 6),
                    low_price   = ROUND(low_price   / :ratio, 6),
                    close_price = ROUND(close_price / :ratio, 6)
                WHERE symbol    = :sym
                  AND timestamp < :bts
            """),
            {"ratio": ratio, "sym": symbol, "bts": boundary_ts},
        )
        rows_intraday = upd.rowcount

        session.execute(
            text("UPDATE stock_splits SET applied_at = :now WHERE id = :sid"),
            {"now": datetime.now(), "sid": split_id},
        )

    log.info(
        "split.applied",
        split_id=split_id,
        market=market_key,
        symbol=symbol,
        split_date=str(split_date),
        ratio=ratio,
        boundary=str(boundary_ts),
        rows_intraday=rows_intraday,
    )


def apply_pending_splits() -> int:
    """Apply all unapplied splits and return count applied.

    Idempotent: splits with applied_at already set are not fetched.
    Each split is applied in its own transaction — a failure on one split
    does not block the others.
    """
    pending = repo.list_unapplied_splits()
    if not pending:
        return 0

    log.info("split.apply.start", count=len(pending))
    applied = 0
    for split in pending:
        try:
            _apply_one_split(split)
            applied += 1
        except Exception as exc:
            log.error(
                "split.apply.error",
                split_id=split.get("id"),
                symbol=split.get("symbol"),
                error=str(exc),
            )

    log.info("split.apply.done", applied=applied, skipped=len(pending) - applied)
    return applied
