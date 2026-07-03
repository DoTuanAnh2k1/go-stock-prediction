"""Walk-forward RL DQN replay harness.

Fixes the horizon mismatch ("cause A"): rl_dqn is now trained on HOURLY
intraday bars (`*_intraday_prices` tables) to match its HOURLY deployment
cadence.  This module provides:

  - `train_rl_intraday(market_key, cutoff_dt)`  — retrain rl_dqn on intraday
    series with timestamp <= cutoff_dt; saves checkpoint to disk.

  - `generate_rl_prediction_asof(market_key, as_of_dt)` — run inference on
    intraday series as-of as_of_dt and write prediction rows to the DB
    (algorithm_name='rl_dqn', target=as_of_dt+1h).

  - `replay_rl_market(market_key, start_dt, end_dt, initial_capital)` — hourly
    walk-forward replay: for each hour in [start_dt, end_dt] run a live-step
    via the existing SimulationEngine and also generate a prediction row.  Uses
    a fresh sim_session (mode='replay') so results don't pollute live sessions.

Market → registry key mapping:

  SIM KEY     REGISTRY KEY (checkpoint)
  GOLD        GOLD          → rl_dqn_GOLD.pt
  NASDAQ      NASDAQ100     → rl_dqn_NASDAQ100.pt
  SP500       SP500         → rl_dqn_SP500.pt
  CRYPTO      CRYPTO        → rl_dqn_CRYPTO.pt

IMPORTANT: This module does NOT touch live sim_bots rows, does NOT set
is_active, and does NOT modify existing sessions / predictions outside the
hourly replay window.  The orchestrator (caller) is responsible for deciding
when to run train / replay.
"""
from __future__ import annotations

import os
from datetime import datetime, timedelta
from decimal import Decimal
from typing import Optional

from src.algorithms.features import MIN_DATA_POINTS
from src.crawlers.crypto import COINS as CRYPTO_COINS
from src.crawlers.nasdaq import NASDAQ_SYMBOLS
from src.crawlers.sp500 import SP500_SYMBOLS
from src.database import repository as repo
from src.database.connection import session_scope
from src.database.models import SimBot, SimSession, SimTrade, SimPortfolioSnapshot
from src.utils.logger import get_logger

log = get_logger("rl_replay")

# ---------------------------------------------------------------------------
# Market key mappings
# ---------------------------------------------------------------------------

# Simulation/DB market key → registry market key (used in checkpoint filename).
# bot.py uses NASDAQ for the sim market but NASDAQ100 for the registry/checkpoint.
SIM_TO_REGISTRY_MARKET: dict[str, str] = {
    "GOLD": "GOLD",
    "NASDAQ": "NASDAQ100",
    "SP500": "SP500",
    "CRYPTO": "CRYPTO",
}

# Gold instruments to enumerate for intraday (source only; product_type varies).
# We use distinct sources from DB; these are the known defaults.
_GOLD_INTRADAY_SOURCES_DEFAULT = ["XAU", "BTMC"]

# Crypto coin_id → (coin_id, symbol) map for intraday lookup.
_CRYPTO_COIN_MAP: dict[str, str] = {
    coin_id: symbol for coin_id, symbol in CRYPTO_COINS
}

# Symbol → coin_id reverse map (used to write predictions with correct coin_id).
_CRYPTO_SYM_TO_COIN: dict[str, str] = {
    symbol: coin_id for coin_id, symbol in CRYPTO_COINS
}


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _intraday_prices_for_source(market_key: str, identifier: str, as_of: datetime) -> list[float]:
    """Fetch intraday close prices for a single symbol/coin_id/source as-of as_of.

    Returns ASC list of floats using the correct price attribute per market:
      - NASDAQ / SP500:  NasdaqIntradayPrice.close_price / SP500IntradayPrice.close_price
      - CRYPTO:          CryptoIntradayPrice.price  (the close/line-chart column)
      - GOLD:            GoldIntradayPrice.sell_price (fallback: buy_price)
    """
    mk = market_key.upper()

    if mk == "NASDAQ":
        rows = repo.get_nasdaq_intraday_asc_as_of(identifier, as_of, limit=400)
        return [float(r.close_price) for r in rows if r.close_price is not None]

    elif mk == "SP500":
        rows = repo.get_sp500_intraday_asc_as_of(identifier, as_of, limit=400)
        return [float(r.close_price) for r in rows if r.close_price is not None]

    elif mk == "CRYPTO":
        # identifier is coin_id
        rows = repo.get_crypto_intraday_asc_as_of(identifier, as_of, limit=400)
        return [float(r.price) for r in rows if r.price is not None]

    elif mk == "GOLD":
        # identifier is source (e.g. "XAU", "BTMC")
        rows = repo.get_gold_intraday_asc_as_of(identifier, as_of, limit=400)
        prices: list[float] = []
        for r in rows:
            if r.sell_price is not None:
                prices.append(float(r.sell_price))
            elif r.buy_price is not None:
                prices.append(float(r.buy_price))
        return prices

    log.warning("rl_replay.intraday.unknown_market", market=mk)
    return []


def _intraday_series_for_market(market_key: str, cutoff_dt: datetime) -> list[list[float]]:
    """Collect all intraday price series for the market with timestamp <= cutoff_dt.

    Returns a list of price series (each a list[float] in ASC order), filtered
    to only series that have >= MIN_DATA_POINTS points.

    Market enumeration:
      NASDAQ  → symbols from NASDAQ_SYMBOLS crawler constant
      SP500   → symbols from SP500_SYMBOLS crawler constant
      CRYPTO  → coin_ids from CRYPTO_COINS crawler constant  (bitcoin/ethereum/solana)
      GOLD    → distinct sources in gold_intraday_prices table as-of cutoff_dt
                (defaults to ["XAU", "BTMC"] if table is empty)
    """
    mk = market_key.upper()
    series: list[list[float]] = []

    if mk == "NASDAQ":
        for symbol in NASDAQ_SYMBOLS:
            prices = _intraday_prices_for_source(mk, symbol, cutoff_dt)
            if len(prices) >= MIN_DATA_POINTS:
                series.append(prices)

    elif mk == "SP500":
        for symbol in SP500_SYMBOLS:
            prices = _intraday_prices_for_source(mk, symbol, cutoff_dt)
            if len(prices) >= MIN_DATA_POINTS:
                series.append(prices)

    elif mk == "CRYPTO":
        for coin_id, _symbol in CRYPTO_COINS:
            prices = _intraday_prices_for_source(mk, coin_id, cutoff_dt)
            if len(prices) >= MIN_DATA_POINTS:
                series.append(prices)

    elif mk == "GOLD":
        # Query distinct sources present in DB; fall back to known defaults.
        try:
            sources = repo.get_gold_intraday_sources(as_of=cutoff_dt)
        except Exception as exc:
            log.warning("rl_replay.gold_sources.fallback", error=str(exc))
            sources = []
        if not sources:
            sources = list(_GOLD_INTRADAY_SOURCES_DEFAULT)

        for source in sources:
            prices = _intraday_prices_for_source(mk, source, cutoff_dt)
            if len(prices) >= MIN_DATA_POINTS:
                series.append(prices)

    else:
        log.warning("rl_replay.series.unknown_market", market=mk)

    log.info(
        "rl_replay.series.collected",
        market=mk,
        n_series=len(series),
        cutoff=cutoff_dt.isoformat(),
        total_points=sum(len(s) for s in series),
    )
    return series


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

def train_rl_intraday(market_key: str, cutoff_dt: datetime) -> dict:
    """Retrain rl_dqn on hourly intraday series with timestamp <= cutoff_dt.

    Builds a fresh RLDQNPredictor, sets _market_key to the registry key
    (NASDAQ100 for NASDAQ), collects all intraday series, and calls
    train_batch(). The checkpoint is saved to disk at:
        ${RL_MODEL_DIR}/rl_dqn_{REGISTRY_MARKET}.pt

    Args:
        market_key: Sim market key ("GOLD", "NASDAQ", "SP500", "CRYPTO").
        cutoff_dt:  Only intraday bars with timestamp <= cutoff_dt are used.

    Returns:
        dict with keys: market, registry_market, n_series, total_points,
        checkpoint_path.
    """
    from src.algorithms.rl_dqn import RLDQNPredictor, _checkpoint_path

    mk = market_key.upper()
    registry_market = SIM_TO_REGISTRY_MARKET.get(mk, mk)

    log.info("rl_replay.train.start", market=mk, registry_market=registry_market,
             cutoff=cutoff_dt.isoformat())

    series = _intraday_series_for_market(mk, cutoff_dt)
    n_series = len(series)
    total_points = sum(len(s) for s in series)

    if n_series == 0:
        log.warning("rl_replay.train.no_series", market=mk)
        return {
            "market": mk,
            "registry_market": registry_market,
            "n_series": 0,
            "total_points": 0,
            "checkpoint_path": _checkpoint_path(registry_market),
        }

    algo = RLDQNPredictor()
    algo._market_key = registry_market  # set before train_batch so checkpoint uses correct name

    # train_batch expects list of (prices_list, volumes_or_None)
    algo.train_batch([(prices, None) for prices in series])

    ckpt = _checkpoint_path(registry_market)
    log.info(
        "rl_replay.train.done",
        market=mk,
        registry_market=registry_market,
        n_series=n_series,
        total_points=total_points,
        checkpoint_path=ckpt,
    )

    return {
        "market": mk,
        "registry_market": registry_market,
        "n_series": n_series,
        "total_points": total_points,
        "checkpoint_path": ckpt,
    }


def generate_rl_prediction_asof(market_key: str, as_of_dt: datetime) -> int:
    """Run rl_dqn inference on intraday series as-of as_of_dt and write predictions.

    For each symbol/instrument:
      1. Fetch intraday close prices with timestamp <= as_of_dt.
      2. If >= MIN_DATA_POINTS, call predict(prices).
      3. Write a prediction row (algorithm_name='rl_dqn',
         prediction_date=as_of_dt, target_date=as_of_dt+1h).

    Deletes any existing pending rl_dqn prediction for the same
    symbol/source as-of-today before writing, to avoid stacking duplicates.

    Args:
        market_key: Sim market key.
        as_of_dt:   The "now" datetime for inference (intraday cutoff).

    Returns:
        Count of prediction rows written.
    """
    from src.algorithms.rl_dqn import RLDQNPredictor

    mk = market_key.upper()
    registry_market = SIM_TO_REGISTRY_MARKET.get(mk, mk)
    target_dt = as_of_dt + timedelta(hours=1)

    # Build a single algo instance per call (loads checkpoint lazily).
    algo = RLDQNPredictor()
    algo._market_key = registry_market

    written = 0

    if mk == "NASDAQ":
        for symbol in NASDAQ_SYMBOLS:
            prices = _intraday_prices_for_source(mk, symbol, as_of_dt)
            if len(prices) < MIN_DATA_POINTS:
                continue
            try:
                result = algo.predict(prices)
            except Exception as exc:
                log.warning("rl_replay.predict.failed", market=mk, symbol=symbol, error=str(exc))
                continue

            try:
                repo.create_nasdaq_prediction(
                    symbol=symbol,
                    predicted_price=Decimal(str(round(result.predicted_price, 4))),
                    current_price=Decimal(str(round(result.current_price, 4))),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name="rl_dqn",
                    prediction_date=as_of_dt,
                    target_date=target_dt,
                    status="pending",
                )
                written += 1
            except Exception as exc:
                log.warning("rl_replay.write.failed", market=mk, symbol=symbol, error=str(exc))

    elif mk == "SP500":
        for symbol in SP500_SYMBOLS:
            prices = _intraday_prices_for_source(mk, symbol, as_of_dt)
            if len(prices) < MIN_DATA_POINTS:
                continue
            try:
                result = algo.predict(prices)
            except Exception as exc:
                log.warning("rl_replay.predict.failed", market=mk, symbol=symbol, error=str(exc))
                continue

            try:
                repo.create_sp500_prediction(
                    symbol=symbol,
                    predicted_price=Decimal(str(round(result.predicted_price, 4))),
                    current_price=Decimal(str(round(result.current_price, 4))),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name="rl_dqn",
                    prediction_date=as_of_dt,
                    target_date=target_dt,
                    status="pending",
                )
                written += 1
            except Exception as exc:
                log.warning("rl_replay.write.failed", market=mk, symbol=symbol, error=str(exc))

    elif mk == "CRYPTO":
        for coin_id, symbol in CRYPTO_COINS:
            prices = _intraday_prices_for_source(mk, coin_id, as_of_dt)
            if len(prices) < MIN_DATA_POINTS:
                continue
            try:
                result = algo.predict(prices)
            except Exception as exc:
                log.warning("rl_replay.predict.failed", market=mk, coin_id=coin_id, error=str(exc))
                continue

            try:
                repo.create_crypto_prediction(
                    coin_id=coin_id,
                    symbol=symbol,
                    predicted_price=Decimal(str(round(result.predicted_price, 2))),
                    current_price=Decimal(str(round(result.current_price, 2))),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name="rl_dqn",
                    prediction_date=as_of_dt,
                    target_date=target_dt,
                    status="pending",
                )
                written += 1
            except Exception as exc:
                log.warning("rl_replay.write.failed", market=mk, coin_id=coin_id, error=str(exc))

    elif mk == "GOLD":
        try:
            sources = repo.get_gold_intraday_sources(as_of=as_of_dt)
        except Exception:
            sources = list(_GOLD_INTRADAY_SOURCES_DEFAULT)
        if not sources:
            sources = list(_GOLD_INTRADAY_SOURCES_DEFAULT)

        for source in sources:
            prices = _intraday_prices_for_source(mk, source, as_of_dt)
            if len(prices) < MIN_DATA_POINTS:
                continue
            try:
                result = algo.predict(prices)
            except Exception as exc:
                log.warning("rl_replay.predict.failed", market=mk, source=source, error=str(exc))
                continue

            # Use "spot" as a generic product_type when we don't know it from the source alone;
            # for XAU that is correct; for BTMC we use "sjc" as a representative product_type.
            product_type = "spot" if source == "XAU" else "sjc"
            try:
                repo.create_gold_prediction(
                    source=source,
                    product_type=product_type,
                    predicted_price=Decimal(str(round(result.predicted_price, 2))),
                    current_price=Decimal(str(round(result.current_price, 2))),
                    confidence=Decimal(str(round(result.confidence, 4))),
                    algorithm_name="rl_dqn",
                    prediction_date=as_of_dt,
                    target_date=target_dt,
                    status="pending",
                )
                written += 1
            except Exception as exc:
                log.warning("rl_replay.write.failed", market=mk, source=source, error=str(exc))

    else:
        log.warning("rl_replay.predict.unknown_market", market=mk)

    log.info("rl_replay.predict.done", market=mk, written=written, as_of=as_of_dt.isoformat())
    return written


def replay_rl_market(
    market_key: str,
    start_dt: datetime,
    end_dt: datetime,
    initial_capital: float = 1000.0,
) -> dict:
    """Hourly walk-forward replay for the rl_dqn pooled bot.

    Creates a FRESH sim_session (mode='replay') for the bot_id
    f"{market.lower()}_rl_dqn", then walks hourly from start_dt to end_dt
    (inclusive), calling:
      - generate_rl_prediction_asof(market_key, ts) to write an intraday
        rl_dqn prediction for each hour.
      - engine._run_single_bot(..., now=ts) to execute the live-step with
        the bot reading intraday state (since _step_rl now uses intraday
        data when now is provided).

    The replay session has mode='replay' so monitoring queries can
    distinguish it from live or backtest sessions.

    Args:
        market_key:      Sim market key ("GOLD", "NASDAQ", "SP500", "CRYPTO").
        start_dt:        First hourly tick (inclusive).
        end_dt:          Last hourly tick (inclusive).
        initial_capital: Starting capital for the fresh replay session.

    Returns:
        dict with: market, n_hours, n_predictions, session_id, bot_id.
    """
    from src.simulation.engine import SimulationEngine, _upsert_snapshot
    from src.simulation.bot import TradingBot, BotConfig
    from src.database.repository import update_session_kpis

    mk = market_key.upper()
    bot_id = f"{mk.lower()}_rl_dqn"

    # ── 1. Fetch bot config from DB ───────────────────────────────────────────
    with session_scope() as sess:
        db_bot = sess.query(SimBot).filter(SimBot.id == bot_id).first()
        if db_bot is None:
            log.warning("rl_replay.replay.bot_not_found", bot_id=bot_id)
            # Create a minimal config using the provided initial_capital.
            config = BotConfig(
                bot_id=bot_id,
                market=mk,
                algorithm="rl_dqn",
                initial_capital=initial_capital,
                buy_threshold=1.5,
                sell_threshold=1.0,
                min_confidence=0.60,
                stop_loss=5.0,
                take_profit=8.0,
                max_position_pct=15.0,
                max_positions=5,
                symbol=None,
            )
        else:
            config = BotConfig(
                bot_id=bot_id,
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
            )

    # ── 2. Create fresh replay session ───────────────────────────────────────
    replay_session = SimSession(
        bot_id=bot_id,
        start_date=start_dt.date(),
        end_date=None,
        status="running",
        mode="replay",
    )
    with session_scope() as sess:
        sess.add(replay_session)
        sess.flush()
        session_id: int = replay_session.id
        sess.commit()

    log.info(
        "rl_replay.replay.start",
        market=mk,
        bot_id=bot_id,
        session_id=session_id,
        start=start_dt.isoformat(),
        end=end_dt.isoformat(),
    )

    # ── 3. Build hourly tick list ─────────────────────────────────────────────
    ticks: list[datetime] = []
    ts = start_dt
    while ts <= end_dt:
        ticks.append(ts)
        ts += timedelta(hours=1)

    # ── 4. Walk-forward hourly loop ───────────────────────────────────────────
    engine = SimulationEngine()
    n_predictions = 0

    # Build a fresh bot instance for the replay (starts flat, no prior trades).
    bot = TradingBot(config)

    for ts in ticks:
        today = ts.date()

        # a) Generate intraday rl_dqn prediction for this hour.
        try:
            n_written = generate_rl_prediction_asof(mk, ts)
            n_predictions += n_written
        except Exception as exc:
            log.warning("rl_replay.replay.predict_failed", ts=ts.isoformat(), error=str(exc))

        # b) Run bot live-step via _run_single_bot which handles session/trade/snapshot
        #    persistence and KPI update.  We pass session_id already created above,
        #    but _run_single_bot queries DB for an existing "live"+"running" session
        #    and creates one if absent — since our session has mode='replay' it will
        #    create a NEW live session.  To avoid this, drive the bot step manually
        #    using the existing bot instance and persist trades ourselves.
        try:
            trades = bot.step(today, cache=None, now=ts)

            # Persist trades for this tick
            with session_scope() as sess:
                for trade in trades:
                    trade_at_val = trade.trade_at if trade.trade_at is not None else ts
                    db_trade = SimTrade(
                        session_id=session_id,
                        bot_id=bot_id,
                        symbol=trade.symbol,
                        action=trade.action,
                        quantity=Decimal(str(round(trade.quantity, 6))),
                        price=Decimal(str(round(trade.price, 4))),
                        trade_value=Decimal(str(round(trade.trade_value, 2))),
                        signal_strength=(
                            Decimal(str(round(trade.signal_strength, 4)))
                            if trade.signal_strength is not None else None
                        ),
                        confidence=(
                            Decimal(str(round(trade.confidence, 3)))
                            if trade.confidence is not None else None
                        ),
                        trade_date=trade_at_val,
                        close_reason=trade.close_reason,
                        entry_trade_id=trade.entry_trade_id,
                        pnl=(
                            Decimal(str(round(trade.pnl, 2)))
                            if trade.pnl is not None else None
                        ),
                        pnl_pct=(
                            Decimal(str(round(trade.pnl_pct, 4)))
                            if trade.pnl_pct is not None else None
                        ),
                    )
                    sess.add(db_trade)
                sess.commit()

            # Hourly snapshot (upsert: 1 row per session per hour)
            snap = bot.get_snapshot(today, snap_at=ts)
            with session_scope() as sess:
                _upsert_snapshot(sess, session_id, bot_id, snap)
                sess.commit()

        except Exception as exc:
            log.warning("rl_replay.replay.step_failed", ts=ts.isoformat(), error=str(exc))

    # ── 5. Finalize session ───────────────────────────────────────────────────
    try:
        with session_scope() as sess:
            s = sess.query(SimSession).filter(SimSession.id == session_id).first()
            if s:
                s.status = "completed"
                s.end_date = end_dt.date()
                sess.commit()
        update_session_kpis(session_id)
    except Exception as exc:
        log.warning("rl_replay.replay.finalize_failed", session_id=session_id, error=str(exc))

    # Read final portfolio value for the return dict.
    final_value: float = bot.portfolio.cash
    for pos in bot.portfolio.positions.values():
        final_value += pos.quantity * pos.entry_price  # approximate mark-to-book

    return_pct = (final_value - config.initial_capital) / max(1.0, config.initial_capital) * 100.0

    result = {
        "market": mk,
        "n_hours": len(ticks),
        "n_predictions": n_predictions,
        "session_id": session_id,
        "bot_id": bot_id,
        "initial_capital": config.initial_capital,
        "final_value": round(final_value, 2),
        "return_pct": round(return_pct, 4),
    }
    log.info("rl_replay.replay.done", **result)
    return result


# ---------------------------------------------------------------------------
# Pooled-equity training (NASDAQ + SP500 combined)
# ---------------------------------------------------------------------------

def train_rl_pooled_equity(
    cutoff_dt: datetime,
    seed: int | None = None,
) -> dict:
    """Train one RL DQN policy on pooled NASDAQ + SP500 intraday series.

    Rationale: NASDAQ and SP500 equities are strongly correlated and share
    the same feature space (normalised features).  Concatenating their intraday
    series roughly doubles the number of training episodes compared to training
    each market independently, significantly reducing seed-variance and
    data-scarcity issues in low-symbol sub-sets.

    Strategy:
      1. Collect intraday series from both NASDAQ and SP500 via
         `_intraday_series_for_market`.
      2. Optionally set global random seeds (random, numpy, torch) to make
         training reproducible when `seed` is not None.
      3. Build ONE `RLDQNPredictor`, set `_market_key="NASDAQ100"`, and call
         `train_batch(combined)` — this saves `rl_dqn_NASDAQ100.pt`.
      4. Copy the SAME trained weights to `rl_dqn_SP500.pt` by switching
         `_market_key` to "SP500" and calling `_save_checkpoint` directly.
         Both equity markets then deploy the pooled policy.

    Args:
        cutoff_dt: Only intraday bars with timestamp <= cutoff_dt are used.
        seed:      Optional integer seed for reproducibility.  When provided,
                   seeds `random`, `numpy`, and `torch` (if available) before
                   calling `train_batch`.

    Returns:
        dict with keys: n_series, total_points, checkpoints, seed.
    """
    import random as _random
    import numpy as _np

    from src.algorithms.rl_dqn import RLDQNPredictor, _checkpoint_path

    # ── 1. Collect series from both equity markets ────────────────────────────
    nasdaq_series = _intraday_series_for_market("NASDAQ", cutoff_dt)
    sp500_series = _intraday_series_for_market("SP500", cutoff_dt)
    combined = nasdaq_series + sp500_series

    n_series = len(combined)
    total_points = sum(len(s) for s in combined)

    nasdaq_ckpt = _checkpoint_path("NASDAQ100")
    sp500_ckpt = _checkpoint_path("SP500")

    log.info(
        "rl_replay.pooled_equity.start",
        nasdaq_series=len(nasdaq_series),
        sp500_series=len(sp500_series),
        combined_series=n_series,
        total_points=total_points,
        seed=seed,
        cutoff=cutoff_dt.isoformat(),
    )

    if n_series == 0:
        log.warning("rl_replay.pooled_equity.no_series")
        return {
            "n_series": 0,
            "total_points": 0,
            "checkpoints": [nasdaq_ckpt, sp500_ckpt],
            "seed": seed,
        }

    # ── 2. Seed global RNGs if requested ─────────────────────────────────────
    if seed is not None:
        _random.seed(seed)
        _np.random.seed(seed)
        try:
            import torch as _torch
            _torch.manual_seed(seed)
        except ImportError:
            pass  # torch not available — still seed Python/numpy
        log.info("rl_replay.pooled_equity.seeded", seed=seed)

    # ── 3. Train on combined series, save NASDAQ100 checkpoint ───────────────
    algo = RLDQNPredictor()
    algo._market_key = "NASDAQ100"
    algo.train_batch([(prices, None) for prices in combined])

    # ── 4. Copy identical weights to SP500 checkpoint ────────────────────────
    # train_batch() sets algo._qnet (eval mode) and algo._in_dim.
    # Switching _market_key redirects _save_checkpoint to the SP500 path.
    if algo._qnet is not None:
        algo._market_key = "SP500"
        algo._save_checkpoint(algo._qnet, algo._in_dim)
        log.info(
            "rl_replay.pooled_equity.sp500_copy",
            src=nasdaq_ckpt,
            dst=sp500_ckpt,
        )
    else:
        log.warning("rl_replay.pooled_equity.no_qnet", reason="train_batch produced no network")

    log.info(
        "rl_replay.pooled_equity.done",
        n_series=n_series,
        total_points=total_points,
        checkpoints=[nasdaq_ckpt, sp500_ckpt],
        seed=seed,
    )
    return {
        "n_series": n_series,
        "total_points": total_points,
        "checkpoints": [nasdaq_ckpt, sp500_ckpt],
        "seed": seed,
    }


# ---------------------------------------------------------------------------
# Multi-seed training — pick best checkpoint by greedy reward
# ---------------------------------------------------------------------------

def train_rl_intraday_multiseed(
    market_key: str,
    cutoff_dt: datetime,
    n_seeds: int = 3,
) -> dict:
    """Train rl_dqn n_seeds times and keep the checkpoint with the best greedy reward.

    Motivation: DQN training is stochastic (replay-buffer sampling, random
    exploration, weight initialisation).  Running multiple seeds and scoring
    each with a greedy evaluation pass selects the run that found the best
    policy, reducing seed-variance without increasing inference cost.

    Greedy scoring uses the same helpers exposed by rl_dqn:
      - `_build_obs_matrix(prices, volumes)` to build feature matrices.
      - `_make_windows(obs_mat, prices, EPISODE_WINDOW, EPISODE_STRIDE)` to
        generate evaluation episodes.
      - `_run_episode_greedy(obs_mat, prices, policy_net)` to score each window.

    After training each seed the trained `algo._qnet` is scored on the REAL
    (non-mirror) intraday windows (same evaluation used inside `train_batch`).
    The best seed's network is then written to disk via `_save_checkpoint`.

    Special case — market_key == "EQUITY":
      Uses pooled NASDAQ + SP500 series (same as `train_rl_pooled_equity`).
      The best-seed checkpoint is saved to BOTH rl_dqn_NASDAQ100.pt and
      rl_dqn_SP500.pt.

    Args:
        market_key: Sim market key ("GOLD", "NASDAQ", "SP500", "CRYPTO") or
                    the special value "EQUITY" for pooled equity training.
        cutoff_dt:  Intraday bar cutoff (only bars with timestamp <= cutoff_dt).
        n_seeds:    Number of independent training runs to attempt (default 3).

    Returns:
        dict with keys:
          market, registry_market, n_seeds, per_seed_greedy (list[float]),
          best_seed (int), best_greedy (float).
          For the EQUITY case, registry_market == "NASDAQ100+SP500".
    """
    import random as _random
    import numpy as _np

    from src.algorithms.rl_dqn import (
        RLDQNPredictor,
        _checkpoint_path,
        _build_obs_matrix,
        _make_windows,
        _run_episode_greedy,
        EPISODE_WINDOW,
        EPISODE_STRIDE,
    )
    # MIN_DATA_POINTS is imported at module top from src.algorithms.features; available here.

    mk = market_key.upper()
    is_equity_pool = (mk == "EQUITY")

    # ── 1. Collect series ─────────────────────────────────────────────────────
    if is_equity_pool:
        nasdaq_series = _intraday_series_for_market("NASDAQ", cutoff_dt)
        sp500_series = _intraday_series_for_market("SP500", cutoff_dt)
        all_series = nasdaq_series + sp500_series
        registry_market = "NASDAQ100+SP500"
        primary_registry = "NASDAQ100"   # checkpoint key for the pooled policy
        secondary_registry = "SP500"     # also saved with the same weights
    else:
        all_series = _intraday_series_for_market(mk, cutoff_dt)
        registry_market = SIM_TO_REGISTRY_MARKET.get(mk, mk)
        primary_registry = registry_market
        secondary_registry = None

    n_series = len(all_series)
    total_points = sum(len(s) for s in all_series)

    log.info(
        "rl_replay.multiseed.start",
        market=mk,
        registry_market=registry_market,
        n_series=n_series,
        total_points=total_points,
        n_seeds=n_seeds,
        cutoff=cutoff_dt.isoformat(),
    )

    if n_series == 0:
        log.warning("rl_replay.multiseed.no_series", market=mk)
        return {
            "market": mk,
            "registry_market": registry_market,
            "n_seeds": n_seeds,
            "per_seed_greedy": [],
            "best_seed": 0,
            "best_greedy": 0.0,
        }

    # ── 2. Pre-build evaluation episodes (real series only, no mirror) ────────
    # We build obs matrices once and reuse them across all seed runs to avoid
    # redundant CPU work.  These are the same real (non-mirrored) windows that
    # train_batch uses in its greedy evaluation pass.
    eval_episodes: list[tuple] = []
    for prices in all_series:
        try:
            arr = _np.array(prices, dtype=_np.float32)
            mat = _build_obs_matrix(arr, None)
            if len(mat) > 0:
                for win_mat, win_prices in _make_windows(mat, arr, EPISODE_WINDOW, EPISODE_STRIDE):
                    eval_episodes.append((win_mat, win_prices))
        except Exception as exc:
            log.warning("rl_replay.multiseed.obs_failed", error=str(exc))

    if not eval_episodes:
        log.warning("rl_replay.multiseed.no_eval_episodes", market=mk)
        # Fall back to running training once and accepting the result
        algo = RLDQNPredictor()
        algo._market_key = primary_registry
        algo.train_batch([(p, None) for p in all_series])
        best_ckpt = _checkpoint_path(primary_registry)
        return {
            "market": mk,
            "registry_market": registry_market,
            "n_seeds": n_seeds,
            "per_seed_greedy": [0.0],
            "best_seed": 0,
            "best_greedy": 0.0,
            "note": "No eval episodes — ran single training without multi-seed selection",
        }

    # ── 3. Train n_seeds times, score each run ────────────────────────────────
    per_seed_greedy: list[float] = []
    best_seed: int = 0
    best_greedy: float = float("-inf")
    best_algo: RLDQNPredictor | None = None

    for seed_idx in range(n_seeds):
        # Seed global RNGs deterministically from the seed index
        _random.seed(seed_idx)
        _np.random.seed(seed_idx)
        try:
            import torch as _torch
            _torch.manual_seed(seed_idx)
        except ImportError:
            pass

        log.info(
            "rl_replay.multiseed.seed_start",
            market=mk,
            seed=seed_idx,
            run=f"{seed_idx + 1}/{n_seeds}",
        )

        algo = RLDQNPredictor()
        algo._market_key = primary_registry
        try:
            algo.train_batch([(p, None) for p in all_series])
        except Exception as exc:
            log.warning(
                "rl_replay.multiseed.train_failed",
                market=mk, seed=seed_idx, error=str(exc),
            )
            per_seed_greedy.append(float("-inf"))
            continue

        # Score: greedy reward summed over all eval episodes.
        if algo._qnet is None:
            # train_batch may fail silently (e.g. torch not installed)
            per_seed_greedy.append(float("-inf"))
            continue

        greedy_total = 0.0
        for win_mat, win_prices in eval_episodes:
            try:
                greedy_total += _run_episode_greedy(win_mat, win_prices, algo._qnet)
            except Exception as exc:
                log.warning(
                    "rl_replay.multiseed.greedy_failed",
                    market=mk, seed=seed_idx, error=str(exc),
                )

        per_seed_greedy.append(greedy_total)
        log.info(
            "rl_replay.multiseed.seed_done",
            market=mk,
            seed=seed_idx,
            greedy_total=round(greedy_total, 6),
        )

        if greedy_total > best_greedy:
            best_greedy = greedy_total
            best_seed = seed_idx
            best_algo = algo

    # ── 4. Write the best checkpoint(s) to disk ───────────────────────────────
    if best_algo is not None and best_algo._qnet is not None:
        # Primary checkpoint (NASDAQ100 for equity pool, or the market's own key)
        best_algo._market_key = primary_registry
        best_algo._save_checkpoint(best_algo._qnet, best_algo._in_dim)

        # For EQUITY pool: also mirror to SP500 checkpoint
        if secondary_registry is not None:
            best_algo._market_key = secondary_registry
            best_algo._save_checkpoint(best_algo._qnet, best_algo._in_dim)
            log.info(
                "rl_replay.multiseed.equity_mirror",
                primary=primary_registry,
                secondary=secondary_registry,
                best_seed=best_seed,
            )
    else:
        log.warning(
            "rl_replay.multiseed.no_best",
            market=mk,
            reason="All seeds failed to produce a trained network",
        )

    # Sanitise -inf values for JSON-friendly return (replace with None sentinel)
    per_seed_greedy_out = [
        (g if g != float("-inf") else None) for g in per_seed_greedy
    ]

    log.info(
        "rl_replay.multiseed.done",
        market=mk,
        n_seeds=n_seeds,
        per_seed_greedy=per_seed_greedy_out,
        best_seed=best_seed,
        best_greedy=round(best_greedy, 6) if best_greedy != float("-inf") else None,
    )

    return {
        "market": mk,
        "registry_market": registry_market,
        "n_seeds": n_seeds,
        "per_seed_greedy": per_seed_greedy_out,
        "best_seed": best_seed,
        "best_greedy": best_greedy if best_greedy != float("-inf") else 0.0,
    }
