"""TradingBot — state machine for one market×algorithm bot."""
from __future__ import annotations

from dataclasses import dataclass, field
from datetime import date, datetime, timedelta
from typing import Optional

from src.simulation.portfolio import Portfolio, Trade
from src.simulation.signal import SignalGenerator


@dataclass
class BotConfig:
    bot_id: str
    market: str
    algorithm: str
    initial_capital: float
    buy_threshold: float
    sell_threshold: float
    min_confidence: float
    stop_loss: float
    take_profit: float
    max_position_pct: float
    max_positions: int
    # Per-symbol scoping: None = pooled per-market bot (legacy behaviour).
    # Non-None = per-symbol bot; only this symbol's signals and positions are
    # processed.  The algorithm value will carry the "__ps" suffix in this case.
    symbol: str | None = None


# Map simulation market key → registry market key used by RL DQN
# SignalGenerator / seeder use "GOLD", "NASDAQ", "SP500", "CRYPTO"
# Registry uses "GOLD", "NASDAQ100", "SP500", "CRYPTO"
_SIM_TO_REGISTRY_MARKET: dict[str, str] = {
    "GOLD": "GOLD",
    "NASDAQ": "NASDAQ100",
    "SP500": "SP500",
    "CRYPTO": "CRYPTO",
}


class TradingBot:
    def __init__(self, config: BotConfig):
        self.config = config
        self.portfolio = Portfolio(
            initial_capital=config.initial_capital,
            stop_loss_pct=config.stop_loss,
            take_profit_pct=config.take_profit,
            max_position_pct=config.max_position_pct,
            max_positions=config.max_positions,
        )
        self.signal_gen = SignalGenerator()

    def step(self, sim_date: date, cache=None, now: Optional[datetime] = None) -> list[Trade]:
        """Run one simulation step: SL/TP check then process new signals.

        cache (optional StepDataCache from engine): when provided, DB queries
        for predictions and current prices are served from the pre-fetched cache
        instead of issuing new round-trips.  When None (default / backtest path)
        every call issues its own DB queries — behaviour is unchanged.

        now: the full datetime of this step (for hourly trade_at/snapshot_at).
             When None (backtest path), trade_at/snapshot_at will be left None.
        """
        trades = []

        # Get current prices for SL/TP check (all bots, including RL).
        # For per-symbol bots only check the one tracked symbol to save a DB
        # round-trip (correctness: positions only ever contain that symbol anyway).
        closed_this_step: set[str] = set()
        if self.portfolio.positions:
            sl_symbols = list(self.portfolio.positions.keys())
            if self.config.symbol is not None:
                # Restrict to our symbol; other symbols can't be in positions
                sl_symbols = [s for s in sl_symbols if s == self.config.symbol]

            if sl_symbols:
                if cache is not None and hasattr(cache, "current_prices"):
                    current_prices = {s: cache.current_prices[s] for s in sl_symbols if s in cache.current_prices}
                else:
                    current_prices = self.signal_gen.get_current_prices(
                        self.config.market,
                        sl_symbols,
                        sim_date,
                    )
                sl_tp_trades = self.portfolio.check_stop_loss_take_profit(
                    current_prices, sim_date, trade_at=now
                )
                trades.extend(sl_tp_trades)
                # Block same-step re-entry
                closed_this_step = {t.symbol for t in sl_tp_trades}

        # Detect RL branch: includes both the pooled ("rl_dqn") and per-symbol
        # ("rl_dqn__ps") variants so that the policy path is used in both cases.
        base_key = self.config.algorithm.split("__")[0]
        is_rl = base_key == "rl_dqn"

        if is_rl:
            # ------------------------------------------------------------------
            # RL native branch: policy directly decides BUY/SELL/HOLD per symbol
            # ------------------------------------------------------------------
            rl_trades = self._step_rl(sim_date, closed_this_step, now=now)
            trades.extend(rl_trades)
        else:
            # ------------------------------------------------------------------
            # Standard threshold branch
            # ------------------------------------------------------------------
            cached_rows: list[dict] | None = None
            if cache is not None and hasattr(cache, "predictions"):
                cached_rows = cache.predictions.get(self.config.algorithm)

            signals = self.signal_gen.get_signals(
                market=self.config.market,
                algorithm=self.config.algorithm,
                for_date=sim_date,
                buy_threshold=self.config.buy_threshold,
                sell_threshold=self.config.sell_threshold,
                min_confidence=self.config.min_confidence,
                cached_rows=cached_rows,
            )

            # Per-symbol scoping: discard signals not for our target symbol.
            if self.config.symbol is not None:
                signals = [s for s in signals if s.symbol == self.config.symbol]

            for signal in signals:
                # Use live price from cache for execution; signal.current_price is
                # the price recorded at prediction time and may be stale by the time
                # the bot step runs (e.g. price crashed after prediction was written).
                exec_price = (
                    cache.current_prices.get(signal.symbol, signal.current_price)
                    if cache is not None and hasattr(cache, "current_prices")
                    else signal.current_price
                )
                if signal.action == "BUY":
                    if signal.symbol in closed_this_step:
                        continue
                    trade = self.portfolio.buy(
                        symbol=signal.symbol,
                        price=exec_price,
                        trade_date=sim_date,
                        signal_strength=signal.signal_strength,
                        confidence=signal.confidence,
                        trade_at=now,
                    )
                    if trade:
                        trades.append(trade)
                elif signal.action == "SELL":
                    trade = self.portfolio.sell(
                        symbol=signal.symbol,
                        price=exec_price,
                        trade_date=sim_date,
                        close_reason="signal",
                        trade_at=now,
                    )
                    if trade:
                        trade.signal_strength = signal.signal_strength
                        trade.confidence = signal.confidence
                        trades.append(trade)
                else:
                    trades.append(Trade(
                        symbol=signal.symbol,
                        action="HOLD",
                        quantity=0,
                        price=exec_price,
                        trade_value=0.0,
                        trade_date=sim_date,
                        signal_strength=signal.signal_strength,
                        confidence=signal.confidence,
                        trade_at=now,
                    ))

        return trades

    def _step_rl(self, sim_date: date, closed_this_step: set[str],
                 now: Optional[datetime] = None) -> list[Trade]:
        """RL native step: for each tradable symbol, build observation and call policy.act()."""
        from src.algorithms.registry import get_algos_for_market, get_algos_for_symbol, PS_SUFFIX
        from src.database import repository as repo
        import numpy as np
        from src.algorithms.features import build_enhanced_features, MIN_DATA_POINTS

        trades: list[Trade] = []

        # Resolve registry market key (NASDAQ100 vs NASDAQ)
        registry_market = _SIM_TO_REGISTRY_MARKET.get(self.config.market, self.config.market)

        # Get the RL DQN policy:
        # - Per-symbol bot ("rl_dqn__ps"): load per-symbol checkpoint via
        #   get_algos_for_symbol so rl_dqn_{market}_{symbol}.pt is used.
        # - Pooled bot ("rl_dqn"): keep existing get_algos_for_market behaviour.
        try:
            if self.config.symbol is not None:
                algos = get_algos_for_symbol(registry_market, self.config.symbol)
            else:
                algos = get_algos_for_market(registry_market)
            policy = algos.get("rl_dqn")
            if policy is None:
                return trades
        except Exception:
            return trades

        # Enumerate symbols and prices as-of sim_date.
        # Per-symbol bot: restrict to the single tracked symbol.
        symbol_prices = self._get_symbols_and_prices_as_of(sim_date, repo)
        if self.config.symbol is not None:
            symbol_prices = {k: v for k, v in symbol_prices.items() if k == self.config.symbol}

        as_of_dt = datetime.combine(sim_date, datetime.max.time())

        for symbol, current_price in symbol_prices.items():
            if current_price <= 0:
                continue

            # Fetch price history as-of sim_date for feature building
            prices_list = self._get_price_history_as_of(symbol, as_of_dt, repo)
            if len(prices_list) < MIN_DATA_POINTS:
                continue

            # Build enhanced features
            try:
                arr = np.array(prices_list, dtype=np.float32)
                vol_arr = None  # volumes not tracked per-symbol in simulation
                feats, _ = build_enhanced_features(arr, vol_arr)
                if not feats:
                    continue
                last_feat = np.array(feats[-1], dtype=np.float32)
            except Exception:
                continue

            # Build position state for this symbol
            pos = self.portfolio.positions.get(symbol)
            if pos is not None:
                holding_flag = 1.0
                unrealized_pnl = (current_price / pos.entry_price - 1.0) if pos.entry_price > 0 else 0.0
                days_held = max(0, (sim_date - pos.entry_date).days)
                days_held_norm = min(days_held / 30.0, 1.0)
            else:
                holding_flag = 0.0
                unrealized_pnl = 0.0
                days_held_norm = 0.0

            pos_state = np.array([holding_flag, unrealized_pnl, days_held_norm], dtype=np.float32)
            obs = np.concatenate([last_feat, pos_state])

            # Get policy action (greedy, ε=0)
            try:
                action = policy.act(obs)
            except Exception:
                action = 0  # hold on error

            # Confidence from softmax
            try:
                confidence = policy._softmax_confidence(obs, action)
            except Exception:
                confidence = 0.40

            # Execute action
            if action == 1:  # BUY
                if symbol in closed_this_step:
                    continue
                trade = self.portfolio.buy(
                    symbol=symbol,
                    price=current_price,
                    trade_date=sim_date,
                    signal_strength=0.0,
                    confidence=confidence,
                    trade_at=now,
                )
                if trade:
                    trades.append(trade)
            elif action == 2:  # SELL
                trade = self.portfolio.sell(
                    symbol=symbol,
                    price=current_price,
                    trade_date=sim_date,
                    close_reason="rl_signal",
                    trade_at=now,
                )
                if trade:
                    trade.confidence = confidence
                    trades.append(trade)
            else:  # HOLD
                trades.append(Trade(
                    symbol=symbol,
                    action="HOLD",
                    quantity=0,
                    price=current_price,
                    trade_value=0.0,
                    trade_date=sim_date,
                    signal_strength=0.0,
                    confidence=confidence,
                    trade_at=now,
                ))

        return trades

    def _get_symbols_and_prices_as_of(self, sim_date: date, repo) -> dict[str, float]:
        """Return {symbol: price} for all tradable symbols of this market, as-of sim_date."""
        from datetime import datetime, timedelta
        from src.database.connection import session_scope
        import sqlalchemy

        market = self.config.market
        prices: dict[str, float] = {}

        as_of = datetime.combine(sim_date, datetime.max.time())
        date_start = datetime.combine(sim_date - timedelta(days=7), datetime.min.time())

        with session_scope() as session:
            if market == "GOLD":
                # Use the instruments defined in training
                instruments = [("XAU", "spot"), ("BTMC", "sjc"), ("BTMC", "nhan_tron")]
                for source, product_type in instruments:
                    result = session.execute(
                        sqlalchemy.text("""
                            SELECT sell_price FROM gold_prices
                            WHERE source = :src AND product_type = :ptype
                              AND trading_date BETWEEN :d_start AND :d_end
                            ORDER BY trading_date DESC LIMIT 1
                        """),
                        {"src": source, "ptype": product_type, "d_start": date_start, "d_end": as_of}
                    ).fetchone()
                    if result and result[0]:
                        symbol = f"{source}_{product_type}"
                        prices[symbol] = float(result[0])

            elif market == "NASDAQ":
                result = session.execute(
                    sqlalchemy.text("""
                        SELECT DISTINCT symbol FROM nasdaq_prices
                        WHERE deleted_at IS NULL
                    """)
                ).fetchall()
                syms = [r[0] for r in result]
                for sym in syms:
                    row = session.execute(
                        sqlalchemy.text("""
                            SELECT close_price FROM nasdaq_prices
                            WHERE symbol = :sym AND deleted_at IS NULL
                              AND trading_date BETWEEN :d_start AND :d_end
                            ORDER BY trading_date DESC LIMIT 1
                        """),
                        {"sym": sym, "d_start": date_start, "d_end": as_of}
                    ).fetchone()
                    if row and row[0]:
                        prices[sym] = float(row[0])

            elif market == "SP500":
                result = session.execute(
                    sqlalchemy.text("""
                        SELECT DISTINCT symbol FROM sp500_prices
                        WHERE deleted_at IS NULL
                    """)
                ).fetchall()
                syms = [r[0] for r in result]
                for sym in syms:
                    row = session.execute(
                        sqlalchemy.text("""
                            SELECT close_price FROM sp500_prices
                            WHERE symbol = :sym AND deleted_at IS NULL
                              AND trading_date BETWEEN :d_start AND :d_end
                            ORDER BY trading_date DESC LIMIT 1
                        """),
                        {"sym": sym, "d_start": date_start, "d_end": as_of}
                    ).fetchone()
                    if row and row[0]:
                        prices[sym] = float(row[0])

            elif market == "CRYPTO":
                result = session.execute(
                    sqlalchemy.text("""
                        SELECT DISTINCT coin_id, symbol FROM crypto_prices
                        WHERE deleted_at IS NULL
                    """)
                ).fetchall()
                for coin_id, sym in result:
                    row = session.execute(
                        sqlalchemy.text("""
                            SELECT close_price FROM crypto_prices
                            WHERE coin_id = :cid AND deleted_at IS NULL
                              AND trading_date BETWEEN :d_start AND :d_end
                            ORDER BY trading_date DESC LIMIT 1
                        """),
                        {"cid": coin_id, "d_start": date_start, "d_end": as_of}
                    ).fetchone()
                    if row and row[0]:
                        # Use coin symbol as the trading identifier
                        prices[sym] = float(row[0])

        return prices

    def _get_price_history_as_of(self, symbol: str, as_of_dt: datetime, repo, limit: int = 270) -> list[float]:
        """Fetch price history for feature building as-of as_of_dt."""
        market = self.config.market

        try:
            if market == "GOLD":
                # symbol is "source_product_type"
                parts = symbol.split("_", 1)
                if len(parts) != 2:
                    return []
                source, product_type = parts[0], parts[1]
                rows = repo.get_gold_prices_asc_as_of(source, product_type, as_of_dt, limit=limit)
                return [float(r.buy_price) for r in rows]

            elif market == "NASDAQ":
                rows = repo.get_nasdaq_prices_asc_as_of(symbol, as_of_dt, limit=limit)
                return [float(r.close_price) for r in rows]

            elif market == "SP500":
                rows = repo.get_sp500_prices_asc_as_of(symbol, as_of_dt, limit=limit)
                return [float(r.close_price) for r in rows]

            elif market == "CRYPTO":
                # symbol is the coin symbol (BTC/ETH/SOL); need coin_id
                # coin_id and symbol mapping from crawlers
                coin_map = {"BTC": "bitcoin", "ETH": "ethereum", "SOL": "solana"}
                coin_id = coin_map.get(symbol, symbol.lower())
                rows = repo.get_crypto_prices_asc_as_of(coin_id, as_of_dt, limit=limit)
                return [float(r.close_price) for r in rows]
        except Exception:
            return []

        return []

    def get_snapshot(self, sim_date: date, snap_at: Optional[datetime] = None) -> dict:
        """Build portfolio snapshot. snap_at carries the full datetime for hourly upsert."""
        # Use entry prices as fallback current prices
        current_prices = {}
        if self.portfolio.positions:
            current_prices = self.signal_gen.get_current_prices(
                self.config.market,
                list(self.portfolio.positions.keys()),
                sim_date,
            )
        return self.portfolio.snapshot(current_prices, sim_date, snap_at=snap_at)
