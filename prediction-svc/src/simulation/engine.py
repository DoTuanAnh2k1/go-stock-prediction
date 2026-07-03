"""SimulationEngine — orchestrates backtest and live simulation."""
from __future__ import annotations

import os
import threading
import concurrent.futures
from datetime import date, datetime, timedelta
from decimal import Decimal
from typing import Optional

import sqlalchemy
from sqlalchemy.dialects.postgresql import insert as pg_insert

from src.database.connection import session_scope
from src.database.models import SimBot, SimSession, SimTrade, SimPortfolioSnapshot
from src.database.repository import update_session_kpis
from src.simulation.bot import TradingBot, BotConfig
from src.simulation.signal import SignalGenerator
from src.utils.logger import get_logger

log = get_logger("simulation.engine")

_backtest_lock = threading.Lock()


def _upsert_snapshot(session, session_id: int, bot_id: str, snap: dict) -> None:
    """Insert-or-update the portfolio snapshot.

    Live-step can run multiple times per day (hourly for some markets). To avoid
    duplicate rows we upsert on:
    - ``(session_id, snapshot_at)`` when snapshot_at is provided (hourly live-step
      path) — one row per session per exact datetime, so each hourly run creates
      its own row rather than overwriting.
    - ``(session_id, snapshot_date)`` as fallback when snapshot_at is None
      (backtest path and legacy daily live-step) — one row per session per day,
      latest state wins.

    NOTE: ``session.merge()`` was used previously with the intent of upserting,
    but merge keys on the primary key (id, snapshot_date); since each snapshot is
    built without an id it always INSERTed, accumulating duplicate daily rows.
    """
    # snapshot_at is part of the unique index and must be non-null. Live hourly
    # steps pass the real datetime (one row per session per hour). The backtest /
    # legacy daily path omits it, so we fall back to midnight of snapshot_date,
    # yielding exactly one row per session per day.
    snap_at: Optional[datetime] = snap.get("snapshot_at")
    if snap_at is None:
        snap_at = datetime.combine(snap["snapshot_date"], datetime.min.time())

    values = {
        "session_id": session_id,
        "bot_id": bot_id,
        "snapshot_date": snap["snapshot_date"],
        "snapshot_at": snap_at,
        "cash_balance": Decimal(str(round(snap["cash_balance"], 2))),
        "positions_value": Decimal(str(round(snap["positions_value"], 2))),
        "total_value": Decimal(str(round(snap["total_value"], 2))),
        "total_return_pct": Decimal(str(round(snap["total_return_pct"], 4))),
        "open_positions": snap["open_positions"],
    }

    # Timescale requires the partition column (snapshot_date) to be part of any
    # unique index, so the conflict target is (session_id, snapshot_at,
    # snapshot_date). Because snapshot_at is derived from snapshot_date this is
    # effectively "one row per session per snapshot_at" (per-hour live / per-day
    # backtest).
    stmt = pg_insert(SimPortfolioSnapshot.__table__).values(**values)
    stmt = stmt.on_conflict_do_update(
        index_elements=["session_id", "snapshot_at", "snapshot_date"],
        set_={
            "bot_id": stmt.excluded.bot_id,
            "cash_balance": stmt.excluded.cash_balance,
            "positions_value": stmt.excluded.positions_value,
            "total_value": stmt.excluded.total_value,
            "total_return_pct": stmt.excluded.total_return_pct,
            "open_positions": stmt.excluded.open_positions,
        },
    )

    session.execute(stmt)


class StepDataCache:
    """Read-only cache of prediction rows and current prices built ONCE per
    market live-step and shared across all bots of that market.

    Building this cache before dispatching bot tasks eliminates N duplicate DB
    queries (one per bot) for the same data.  The cache is immutable after
    construction — safe to share across threads.

    Attributes
    ----------
    predictions : dict[str, list[dict]]
        Keyed by ``algorithm_name``; each value is the list of row dicts
        ``{symbol, predicted_price, current_price, confidence}`` for today.
    current_prices : dict[str, float]
        Keyed by symbol; most-recent close/sell price as-of today.
    """

    def __init__(
        self,
        market: str,
        algorithms: list[str],
        for_date: date,
    ) -> None:
        self._sg = SignalGenerator()
        self.predictions: dict[str, list[dict]] = {}
        self.current_prices: dict[str, float] = {}

        # Fetch predictions for each distinct algorithm used by this market's bots.
        for algo in algorithms:
            try:
                rows = self._sg.fetch_predictions(market, algo, for_date)
                self.predictions[algo] = rows
            except Exception as exc:
                log.warning("sim.cache.pred_fetch_failed", market=market, algo=algo, error=str(exc))
                self.predictions[algo] = []

        # Derive the full set of symbols present in prediction rows and fetch
        # one batched current-price lookup for those symbols.
        all_symbols: set[str] = set()
        for rows in self.predictions.values():
            for row in rows:
                sym = row.get("symbol")
                if sym:
                    all_symbols.add(sym)

        if all_symbols:
            try:
                prices = self._sg.get_current_prices(market, list(all_symbols), for_date)
                self.current_prices.update(prices)
            except Exception as exc:
                log.warning("sim.cache.price_fetch_failed", market=market, error=str(exc))

# Normalize orchestrator market keys → bot market field values
_MARKET_KEY_NORM: dict[str, str] = {
    "NASDAQ100": "NASDAQ",
}


def _get_simulation_dates(market: str, algorithm: str, start_date: date, end_date: date) -> list[date]:
    """Get all dates that have predictions for this market/algorithm in the given range."""
    market_to_table = {
        "GOLD": "gold_predictions",
        "NASDAQ": "nasdaq_predictions",
        "SP500": "sp500_predictions",
        "CRYPTO": "crypto_predictions",
    }
    table = market_to_table.get(market, "gold_predictions")

    start_dt = datetime.combine(start_date, datetime.min.time())
    end_dt = datetime.combine(end_date, datetime.max.time())

    with session_scope() as session:
        result = session.execute(
            sqlalchemy.text(f"""
                SELECT DISTINCT DATE(prediction_date) as pred_date
                FROM {table}
                WHERE algorithm_name = :algo
                  AND prediction_date BETWEEN :d_start AND :d_end
                ORDER BY pred_date ASC
            """),
            {"algo": algorithm, "d_start": start_dt, "d_end": end_dt}
        ).fetchall()

        return [r[0] for r in result if r[0] is not None]


def _restore_portfolio_state(bot, db_trades: list, initial_capital: float) -> None:
    """Restore bot portfolio state from prior trades in the live session."""
    from src.simulation.portfolio import Position

    if not db_trades:
        return

    cash = initial_capital
    # Per-symbol: queue of unmatched BUYs (FIFO matching)
    open_buys: dict[str, list] = {}  # symbol → list of buy dicts

    for t in db_trades:  # ordered by id ASC = insertion order
        tv = float(t["trade_value"])
        symbol = t["symbol"]

        if t["action"] == "BUY":
            cash -= tv
            td = t["trade_date"]
            open_buys.setdefault(symbol, []).append({
                'quantity': float(t["quantity"]),
                'price': float(t["price"]),
                'date': td.date() if hasattr(td, 'date') else td,
                'entry_at': td if hasattr(td, 'date') else None,
                'id': t["id"],
                'trade_value': tv,
            })
        elif t["action"] == "SELL":
            cash += tv
            # FIFO: match against earliest open buy
            if symbol in open_buys and open_buys[symbol]:
                open_buys[symbol].pop(0)
                if not open_buys[symbol]:
                    del open_buys[symbol]

    # Restore positions: one per symbol (first open buy), refund duplicates to cash
    for symbol, buys in open_buys.items():
        if not buys:
            continue
        b = buys[0]  # first open buy
        if symbol not in bot.portfolio.positions:
            bot.portfolio.positions[symbol] = Position(
                symbol=symbol,
                quantity=b['quantity'],
                entry_price=b['price'],
                entry_date=b['date'],
                entry_trade_id=b['id'],
                entry_at=b.get('entry_at'),
            )
        # Refund extra duplicate buys (Bug artifacts) back to cash
        for extra in buys[1:]:
            cash += extra['trade_value']

    bot.portfolio.cash = max(0.0, cash)

    # Set counter to avoid in-memory ID conflicts
    if db_trades:
        max_id = max(t["id"] for t in db_trades)
        bot.portfolio._trade_id_counter = max_id

    log.debug(
        "sim.portfolio.restored",
        positions=list(bot.portfolio.positions.keys()),
        cash=round(bot.portfolio.cash, 2),
    )


class SimulationEngine:
    """Runs backtest or live simulation for trading bots."""

    def run_backtest(self, bot_id: str, start_date: date, end_date: date) -> dict:
        """
        Walk-forward backtest: iterate over each day with predictions,
        run bot.step(), persist trades and snapshots.
        Returns summary dict.
        """
        with session_scope() as session:
            db_bot = session.query(SimBot).filter(SimBot.id == bot_id).first()
            if not db_bot:
                raise ValueError(f"Bot {bot_id!r} not found")

            config = BotConfig(
                bot_id=db_bot.id,
                market=db_bot.market,
                algorithm=db_bot.algorithm,
                initial_capital=float(db_bot.initial_capital),
                buy_threshold=float(db_bot.buy_threshold),
                sell_threshold=float(db_bot.sell_threshold),
                min_confidence=float(db_bot.min_confidence),
                stop_loss=float(db_bot.stop_loss),
                take_profit=float(db_bot.take_profit),
                max_position_pct=float(db_bot.max_position_pct),
                max_positions=int(db_bot.max_positions),
                symbol=db_bot.symbol,
                trailing_stop=bool(getattr(db_bot, "trailing_stop", False) or False),
            )

        bot = TradingBot(config)

        # Create simulation session
        session_obj = SimSession(
            bot_id=bot_id,
            start_date=start_date,
            end_date=None,
            status="running",
            mode="backtest",
        )
        with session_scope() as session:
            session.add(session_obj)
            session.flush()
            session_id = session_obj.id
            session.commit()

        log.info("sim.backtest.start", bot_id=bot_id, start=str(start_date), end=str(end_date))

        try:
            sim_dates = _get_simulation_dates(config.market, config.algorithm, start_date, end_date)

            if not sim_dates:
                log.warning("sim.backtest.no_data", bot_id=bot_id)
                self._finalize_session(session_id, end_date, "completed")
                return {"bot_id": bot_id, "session_id": session_id, "total_days": 0}

            for sim_date in sim_dates:
                trades = bot.step(sim_date)

                # Persist trades
                trade_datetime = datetime(sim_date.year, sim_date.month, sim_date.day)
                with session_scope() as session:
                    for trade in trades:
                        db_trade = SimTrade(
                            session_id=session_id,
                            bot_id=bot_id,
                            symbol=trade.symbol,
                            action=trade.action,
                            quantity=Decimal(str(round(trade.quantity, 6))),
                            price=Decimal(str(round(trade.price, 4))),
                            trade_value=Decimal(str(round(trade.trade_value, 2))),
                            signal_strength=Decimal(str(round(trade.signal_strength, 4))) if trade.signal_strength is not None else None,
                            confidence=Decimal(str(round(trade.confidence, 3))) if trade.confidence is not None else None,
                            trade_date=trade_datetime,
                            close_reason=trade.close_reason,
                            entry_trade_id=trade.entry_trade_id,
                            pnl=Decimal(str(round(trade.pnl, 2))) if trade.pnl is not None else None,
                            pnl_pct=Decimal(str(round(trade.pnl_pct, 4))) if trade.pnl_pct is not None else None,
                        )
                        session.add(db_trade)
                        session.flush()
                    session.commit()

                # Daily snapshot (upsert: exactly one row per session per day)
                # Backtest path: snap_at=None → upsert on (session_id, snapshot_date)
                snap = bot.get_snapshot(sim_date, snap_at=None)
                with session_scope() as session:
                    _upsert_snapshot(session, session_id, bot_id, snap)
                    session.commit()

            self._finalize_session(session_id, end_date, "completed")
            log.info("sim.backtest.done", bot_id=bot_id, session_id=session_id, days=len(sim_dates))

            # Log KPIs after backtest completes
            try:
                from src.simulation.metrics import PerformanceMetrics

                with session_scope() as session:
                    snaps = (
                        session.query(SimPortfolioSnapshot)
                        .filter(SimPortfolioSnapshot.session_id == session_id)
                        .order_by(SimPortfolioSnapshot.snapshot_date.asc())
                        .all()
                    )
                    sell_rows = (
                        session.query(SimTrade)
                        .filter(
                            SimTrade.session_id == session_id,
                            SimTrade.action == "SELL",
                        )
                        .all()
                    )
                    # Build entry_date lookup: entry_trade_id → trade_date of the BUY
                    entry_ids = [
                        t.entry_trade_id for t in sell_rows if t.entry_trade_id is not None
                    ]
                    entry_date_map: dict[int, date] = {}
                    if entry_ids:
                        buy_rows = (
                            session.query(SimTrade.id, SimTrade.trade_date)
                            .filter(SimTrade.id.in_(entry_ids))
                            .all()
                        )
                        entry_date_map = {r.id: r.trade_date for r in buy_rows}

                    snap_dicts = [
                        {
                            "snapshot_date": s.snapshot_date,
                            "total_value": float(s.total_value),
                            "total_return_pct": float(s.total_return_pct or 0),
                        }
                        for s in snaps
                    ]
                    trade_dicts = [
                        {
                            "pnl": float(t.pnl) if t.pnl is not None else None,
                            "pnl_pct": float(t.pnl_pct) if t.pnl_pct is not None else None,
                            "trade_date": t.trade_date,
                            "entry_date": entry_date_map.get(t.entry_trade_id)
                            if t.entry_trade_id is not None
                            else None,
                        }
                        for t in sell_rows
                    ]

                with session_scope() as session:
                    db_bot = session.query(SimBot).filter(SimBot.id == bot_id).first()
                    initial = float(db_bot.initial_capital) if db_bot else 0.0

                kpis = PerformanceMetrics.compute(snap_dicts, trade_dicts, initial)
                log.info(
                    "sim.backtest.kpis",
                    bot_id=bot_id,
                    session_id=session_id,
                    total_return_pct=round(kpis.total_return_pct, 2),
                    sharpe=round(kpis.sharpe_ratio, 3),
                    max_dd=round(kpis.max_drawdown_pct, 2),
                    win_rate=round(kpis.win_rate_pct, 1),
                    total_trades=kpis.total_trades,
                )
            except Exception as kpi_exc:
                log.warning("sim.backtest.kpis_failed", bot_id=bot_id, error=str(kpi_exc))

            return {"bot_id": bot_id, "session_id": session_id, "total_days": len(sim_dates)}

        except Exception as exc:
            log.error("sim.backtest.error", bot_id=bot_id, error=str(exc))
            self._finalize_session(session_id, end_date, "paused")
            raise

    def _finalize_session(self, session_id: int, end_date: date, status: str):
        with session_scope() as session:
            s = session.query(SimSession).filter(SimSession.id == session_id).first()
            if s:
                s.status = status
                s.end_date = end_date
                session.commit()
        # Compute and store pre-aggregated KPI metrics whenever a session completes.
        if status == "completed":
            try:
                update_session_kpis(session_id)
            except Exception as kpi_exc:
                log.warning("sim.kpi.finalize_failed", session_id=session_id, error=str(kpi_exc))

    def run_all_bots_backtest(self, start_date: date, end_date: date) -> int:
        """Backtest all active bots in parallel. Returns count of completed.

        Each bot is dispatched to a ThreadPoolExecutor worker.  run_backtest()
        opens its own session_scope() for every DB operation — no shared session
        state between threads.  Worker count follows per_symbol_workers setting
        (0 = os.cpu_count()), capped at 8.
        """
        from src.config import get_settings

        with session_scope() as session:
            bots = session.query(SimBot).filter(SimBot.is_active == True).all()
            bot_ids = [b.id for b in bots]

        cfg = get_settings()
        workers = max(1, min(8, cfg.per_symbol_workers or (os.cpu_count() or 4)))
        log.info("sim.all_bots.start", total=len(bot_ids), workers=workers)

        completed = 0
        completed_lock = threading.Lock()

        def _run_one(bot_id: str) -> None:
            self.run_backtest(bot_id, start_date, end_date)

        with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as pool:
            futures = {pool.submit(_run_one, bid): bid for bid in bot_ids}
            for fut in concurrent.futures.as_completed(futures):
                bot_id = futures[fut]
                try:
                    fut.result()
                    with completed_lock:
                        completed += 1
                except Exception as exc:
                    log.error(
                        "sim.all_bots.bot_failed",
                        bot_id=bot_id,
                        error=str(exc),
                        exc_info=True,
                    )

        log.info("sim.all_bots.done", total=len(bot_ids), completed=completed)
        return completed

    def run_live_step_for_market(self, market_key: str) -> None:
        """Run one live simulation step for all active bots of a specific market.

        Called immediately after predictions for that market are written to DB.
        market_key can be orchestrator keys like "NASDAQ100" — will be normalized.

        For NASDAQ/SP500: skips if outside US session hours (is_intraday_open guard),
        matching the predict guard so bots only trade when fresh prices exist.
        GOLD and CRYPTO always run (no intraday guard).
        """
        from src.config import get_settings
        from src.utils.market_calendar import is_intraday_open

        normalized = _MARKET_KEY_NORM.get(market_key.upper(), market_key.upper())

        # Guard: NASDAQ/SP500 only trade during US session hours (9:30 AM – 4:00 PM ET)
        if not is_intraday_open(normalized):
            log.info(
                "sim.live_step.skip.not_intraday",
                market=normalized,
                reason="outside US session hours",
            )
            return

        now = datetime.now()  # VN local time (TZ=Asia/Ho_Chi_Minh)
        today = now.date()

        with session_scope() as session:
            bots = session.query(SimBot).filter(
                SimBot.is_active == True,
                SimBot.market == normalized,
            ).all()
            bot_configs = []
            for db_bot in bots:
                bot_configs.append((db_bot.id, BotConfig(
                    bot_id=db_bot.id,
                    market=db_bot.market,
                    algorithm=db_bot.algorithm,
                    initial_capital=float(db_bot.initial_capital),
                    buy_threshold=float(db_bot.buy_threshold),
                    sell_threshold=float(db_bot.sell_threshold),
                    min_confidence=float(db_bot.min_confidence),
                    stop_loss=float(db_bot.stop_loss),
                    take_profit=float(db_bot.take_profit),
                    max_position_pct=float(db_bot.max_position_pct),
                    max_positions=int(db_bot.max_positions),
                    symbol=db_bot.symbol,
                    trailing_stop=bool(getattr(db_bot, "trailing_stop", False) or False),
                )))

        if not bot_configs:
            log.debug("sim.live_step.no_bots", market=normalized)
            return

        # Build a step-scoped data cache: fetch predictions once per distinct
        # algorithm and current prices once per market-wide symbol set.
        # This eliminates O(N_bots) duplicate DB queries for the same data.
        distinct_algos = list({cfg.algorithm for _, cfg in bot_configs})
        cache = StepDataCache(market=normalized, algorithms=distinct_algos, for_date=today)

        settings = get_settings()
        raw_workers = settings.per_symbol_workers or (os.cpu_count() or 4)
        n_workers = max(1, min(16, raw_workers))

        log.info("sim.live_step.market_start", market=normalized, bots=len(bot_configs), workers=n_workers)
        self._run_bot_steps(bot_configs, today, now=now, cache=cache, n_workers=n_workers)
        log.info("sim.live_step.market_done", market=normalized)

    def _run_single_bot(self, bot_id: str, config: BotConfig, today, now: datetime, cache=None) -> None:
        """Execute one live-step for a single bot.  Designed to be called from a
        thread pool; each invocation opens its OWN session_scope() blocks so
        SQLAlchemy sessions are never shared across threads.

        ``now`` carries the full datetime for hourly trade_at / snapshot_at columns.
        """
        try:
            with session_scope() as session:
                live_session = session.query(SimSession).filter(
                    SimSession.bot_id == bot_id,
                    SimSession.mode == "live",
                    SimSession.status == "running",
                ).order_by(SimSession.id.desc()).first()

                if not live_session:
                    live_session = SimSession(
                        bot_id=bot_id,
                        start_date=today,
                        status="running",
                        mode="live",
                    )
                    session.add(live_session)
                    session.flush()
                session_id = live_session.id
                session.commit()

            bot = TradingBot(config)

            # Restore portfolio state from existing live session trades
            with session_scope() as _sess:
                prior_trades = [
                    {
                        "id": t.id,
                        "symbol": t.symbol,
                        "action": t.action,
                        "quantity": t.quantity,
                        "price": t.price,
                        "trade_value": t.trade_value,
                        "trade_date": t.trade_date,
                    }
                    for t in _sess.query(SimTrade).filter(
                        SimTrade.session_id == session_id
                    ).order_by(SimTrade.id.asc()).all()
                ]
            if prior_trades:
                _restore_portfolio_state(bot, prior_trades, config.initial_capital)

            # Pass now so bot propagates trade_at to each Trade object
            trades = bot.step(today, cache=cache, now=now)

            with session_scope() as session:
                for trade in trades:
                    # trade_at: prefer trade.trade_at (set by bot when now is passed);
                    # fall back to now for safety.
                    trade_at_val = trade.trade_at if trade.trade_at is not None else now
                    db_trade = SimTrade(
                        session_id=session_id,
                        bot_id=bot_id,
                        symbol=trade.symbol,
                        action=trade.action,
                        quantity=Decimal(str(round(trade.quantity, 6))),
                        price=Decimal(str(round(trade.price, 4))),
                        trade_value=Decimal(str(round(trade.trade_value, 2))),
                        signal_strength=Decimal(str(round(trade.signal_strength, 4))) if trade.signal_strength is not None else None,
                        confidence=Decimal(str(round(trade.confidence, 3))) if trade.confidence is not None else None,
                        trade_date=trade_at_val,
                        close_reason=trade.close_reason,
                        entry_trade_id=trade.entry_trade_id,
                        pnl=Decimal(str(round(trade.pnl, 2))) if trade.pnl is not None else None,
                        pnl_pct=Decimal(str(round(trade.pnl_pct, 4))) if trade.pnl_pct is not None else None,
                    )
                    session.add(db_trade)
                session.commit()

            # Snapshot with hourly snap_at so upsert creates 1 row per hour
            snap = bot.get_snapshot(today, snap_at=now)
            with session_scope() as session:
                _upsert_snapshot(session, session_id, bot_id, snap)
                session.commit()

            # Refresh KPI columns after each live step so leaderboard/monitoring
            # queries can read directly from sim_sessions without aggregating.
            try:
                update_session_kpis(session_id)
            except Exception as kpi_exc:
                log.warning("sim.kpi.live_step_failed", bot_id=bot_id, session_id=session_id, error=str(kpi_exc))

        except Exception as exc:
            log.error("sim.live_step.bot_error", bot_id=bot_id, error=str(exc))

    def _run_bot_steps(self, bot_configs: list, today, now: Optional[datetime] = None,
                       cache=None, n_workers: int = 1) -> None:
        """Execute one simulation step for a list of (bot_id, BotConfig) pairs.

        When n_workers > 1 bots are dispatched to a ThreadPoolExecutor so that
        the (now large) per-symbol fleet can be processed in parallel.  The
        StepDataCache (if provided) is read-only and safe to share across threads.
        When n_workers == 1 (or cache is None — backtest path) the loop is
        sequential for backwards compatibility.

        ``now`` is the live datetime used for trade_at / snapshot_at columns.
        When None (backtest path), it defaults to datetime.now() inside each bot.
        """
        step_now = now if now is not None else datetime.now()

        if n_workers <= 1:
            # Sequential path — preserves exact legacy behaviour for backtest /
            # small fleets / callers that don't pass n_workers.
            for bot_id, config in bot_configs:
                self._run_single_bot(bot_id, config, today, step_now, cache=cache)
        else:
            with concurrent.futures.ThreadPoolExecutor(max_workers=n_workers) as executor:
                futures = {
                    executor.submit(self._run_single_bot, bot_id, config, today, step_now, cache): bot_id
                    for bot_id, config in bot_configs
                }
                for future in concurrent.futures.as_completed(futures):
                    bot_id = futures[future]
                    try:
                        future.result()
                    except Exception as exc:
                        # _run_single_bot already logs internally; catch here as
                        # a safety net so one bad future can't prevent others.
                        log.error("sim.live_step.bot_error", bot_id=bot_id, error=str(exc))

    def run_live_step(self):
        """Called by fallback cron — runs one simulation step for ALL active bots."""
        from datetime import date as date_type
        today = date_type.today()

        from src.utils.market_calendar import is_market_open

        with session_scope() as session:
            bots = session.query(SimBot).filter(SimBot.is_active == True).all()
            bot_configs = []
            for db_bot in bots:
                # Skip bots for closed markets (NASDAQ/SP500 on weekends & US holidays)
                if not is_market_open(db_bot.market):
                    log.debug("sim.live_step.skip_closed", bot_id=db_bot.id, market=db_bot.market)
                    continue
                bot_configs.append((db_bot.id, BotConfig(
                    bot_id=db_bot.id,
                    market=db_bot.market,
                    algorithm=db_bot.algorithm,
                    initial_capital=float(db_bot.initial_capital),
                    buy_threshold=float(db_bot.buy_threshold),
                    sell_threshold=float(db_bot.sell_threshold),
                    min_confidence=float(db_bot.min_confidence),
                    stop_loss=float(db_bot.stop_loss),
                    take_profit=float(db_bot.take_profit),
                    max_position_pct=float(db_bot.max_position_pct),
                    max_positions=int(db_bot.max_positions),
                    symbol=db_bot.symbol,
                    trailing_stop=bool(getattr(db_bot, "trailing_stop", False) or False),
                )))

        now = datetime.now()
        log.info("sim.live_step.all_start", bots=len(bot_configs))
        # Fallback cron path: run sequentially (cache=None) to keep behaviour
        # identical to the pre-threading implementation.
        self._run_bot_steps(bot_configs, today, now=now, cache=None, n_workers=1)
        log.info("sim.live_step.all_done")

    def reset_active_bots(self) -> int:
        """Close all existing live sessions and create fresh ones for all active bots.

        Closes any 'running' or 'paused' live sessions, then creates a new
        'running' live session per active bot. Call this when bots stop trading
        due to stale session state.

        Returns count of bots successfully reset.
        """
        from datetime import date as date_type
        today = date_type.today()

        with session_scope() as session:
            bots = session.query(SimBot).filter(SimBot.is_active == True).all()
            bot_ids = [b.id for b in bots]

        count = 0
        for bot_id in bot_ids:
            try:
                stale_ids: list[int] = []
                with session_scope() as session:
                    stale = session.query(SimSession).filter(
                        SimSession.bot_id == bot_id,
                        SimSession.mode == "live",
                        SimSession.status.in_(["running", "paused"]),
                    ).all()
                    for s in stale:
                        stale_ids.append(s.id)
                        s.status = "completed"
                        s.end_date = today
                    session.commit()

                # Compute KPIs for each freshly-closed session
                for sid in stale_ids:
                    try:
                        update_session_kpis(sid)
                    except Exception as kpi_exc:
                        log.warning("sim.kpi.reset_failed", session_id=sid, error=str(kpi_exc))

                with session_scope() as session:
                    new_sess = SimSession(
                        bot_id=bot_id,
                        start_date=today,
                        status="running",
                        mode="live",
                    )
                    session.add(new_sess)
                    session.commit()

                count += 1
            except Exception as exc:
                log.error("sim.reset.bot_error", bot_id=bot_id, error=str(exc))

        log.info("sim.reset.done", count=count, total=len(bot_ids))
        return count
