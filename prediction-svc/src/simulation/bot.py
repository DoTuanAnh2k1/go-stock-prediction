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
    # Trailing stop-loss: when True, SL is computed off the highest intraday
    # price observed since entry instead of the fixed entry price (see
    # Portfolio.trailing_stop / TradingBot._get_peak_prices_since_entry).
    trailing_stop: bool = False


# Cross-section threshold multiplier for meta_stack adaptive threshold.
# threshold = max(0.5 + delta, mu_p + K * sigma_p)
# K=1.0 means "buy only symbols whose p is at least 1 std-dev above the
# cross-section mean" — prevents buying the least-bad symbol when the whole
# market looks bearish.  The absolute floor (0.5+delta) ensures we never
# buy a symbol the model rates as likely to fall.
_META_ADAPTIVE_K: float = 1.0

# RL BUY confidence floor: softmax over 3 Q-values gives 1/3 ≈ 0.333 for a
# totally indifferent policy.  Requiring a modest margin above uniform before
# opening a NEW position filters out coin-flip entries (fee churn) without
# blocking exits — SELL is never gated so risk can always be closed.
_RL_BUY_CONF_FLOOR: float = 0.38

# Conviction branch (transformer_nn): P(up) entry floors validated by the
# 28/6→2/7 walk-forward trading replay. Variant thresholds map on top via
# delta = buy_threshold/100 (same convention as meta_stack), but the absolute
# floor/ceiling always applies: never buy below P(up)=0.52, never exit-signal
# above P(up)=0.48. Rationale: hourly |Δprice| predictions (~0.1-0.3%) can
# never clear the %-magnitude thresholds designed for daily moves, so
# transformer bots decide on the direction head's P(up) instead.
_CONVICTION_BUY_FLOOR: float = 0.52
_CONVICTION_SELL_CEIL: float = 0.48

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
            trailing_stop=config.trailing_stop,
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

                peak_prices: Optional[dict[str, float]] = None
                if self.config.trailing_stop:
                    step_now = now if now is not None else datetime.now()
                    peak_prices = self._get_peak_prices_since_entry(
                        sl_symbols, step_now, current_prices=current_prices
                    )

                sl_tp_trades = self.portfolio.check_stop_loss_take_profit(
                    current_prices, sim_date, trade_at=now, peak_prices=peak_prices
                )
                trades.extend(sl_tp_trades)
                # Block same-step re-entry
                closed_this_step = {t.symbol for t in sl_tp_trades}

        # Detect native branches: pooled and per-symbol ("__ps") variants both
        # route to the same branch.
        base_key = self.config.algorithm.split("__")[0]
        is_rl = base_key == "rl_dqn"
        is_meta = base_key == "meta_stack"
        is_conviction = base_key == "transformer_nn"

        if is_rl:
            # ------------------------------------------------------------------
            # RL native branch: policy directly decides BUY/SELL/HOLD per symbol
            # ------------------------------------------------------------------
            rl_trades = self._step_rl(sim_date, closed_this_step, now=now)
            trades.extend(rl_trades)
        elif is_meta:
            # ------------------------------------------------------------------
            # Meta-stacking branch: P(up|x_t) → conviction-sized BUY/HOLD/SELL
            # ------------------------------------------------------------------
            meta_trades = self._step_meta(sim_date, closed_this_step, now=now)
            trades.extend(meta_trades)
        elif is_conviction:
            # ------------------------------------------------------------------
            # Conviction branch (transformer_nn): direction-head P(up) decides
            # ------------------------------------------------------------------
            conv_trades = self._step_conviction(sim_date, closed_this_step, now=now)
            trades.extend(conv_trades)
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

    def _get_symbols_and_prices_intraday_as_of(self, now: datetime) -> dict[str, float]:
        """Return {symbol: price} for all tradable symbols as-of `now`, using INTRADAY tables.

        Used by the RL branch when `now` is provided (hourly deployment).  Falls back
        to None prices for symbols with no recent intraday bar.
        """
        from src.database import repository as repo
        from src.database.connection import session_scope
        import sqlalchemy

        market = self.config.market
        prices: dict[str, float] = {}
        # Look back 48h to account for gaps in intraday data
        date_start = now - timedelta(hours=48)

        with session_scope() as session:
            if market == "GOLD":
                instruments = [("XAU", "spot"), ("BTMC", "sjc"), ("BTMC", "nhan_tron")]
                for source, product_type in instruments:
                    result = session.execute(
                        sqlalchemy.text("""
                            SELECT sell_price, buy_price FROM gold_intraday_prices
                            WHERE source = :src
                              AND timestamp BETWEEN :d_start AND :d_end
                            ORDER BY timestamp DESC LIMIT 1
                        """),
                        {"src": source, "d_start": date_start, "d_end": now}
                    ).fetchone()
                    if result:
                        price_val = result[0] if result[0] is not None else result[1]
                        if price_val:
                            symbol = f"{source}_{product_type}"
                            prices[symbol] = float(price_val)

            elif market == "NASDAQ":
                result = session.execute(
                    sqlalchemy.text("""
                        SELECT DISTINCT symbol FROM nasdaq_intraday_prices
                    """)
                ).fetchall()
                syms = [r[0] for r in result]
                for sym in syms:
                    row = session.execute(
                        sqlalchemy.text("""
                            SELECT close_price FROM nasdaq_intraday_prices
                            WHERE symbol = :sym
                              AND timestamp BETWEEN :d_start AND :d_end
                            ORDER BY timestamp DESC LIMIT 1
                        """),
                        {"sym": sym, "d_start": date_start, "d_end": now}
                    ).fetchone()
                    if row and row[0] is not None:
                        prices[sym] = float(row[0])

            elif market == "SP500":
                result = session.execute(
                    sqlalchemy.text("""
                        SELECT DISTINCT symbol FROM sp500_intraday_prices
                    """)
                ).fetchall()
                syms = [r[0] for r in result]
                for sym in syms:
                    row = session.execute(
                        sqlalchemy.text("""
                            SELECT close_price FROM sp500_intraday_prices
                            WHERE symbol = :sym
                              AND timestamp BETWEEN :d_start AND :d_end
                            ORDER BY timestamp DESC LIMIT 1
                        """),
                        {"sym": sym, "d_start": date_start, "d_end": now}
                    ).fetchone()
                    if row and row[0] is not None:
                        prices[sym] = float(row[0])

            elif market == "CRYPTO":
                result = session.execute(
                    sqlalchemy.text("""
                        SELECT DISTINCT coin_id FROM crypto_intraday_prices
                    """)
                ).fetchall()
                for (coin_id,) in result:
                    row = session.execute(
                        sqlalchemy.text("""
                            SELECT price FROM crypto_intraday_prices
                            WHERE coin_id = :cid
                              AND timestamp BETWEEN :d_start AND :d_end
                            ORDER BY timestamp DESC LIMIT 1
                        """),
                        {"cid": coin_id, "d_start": date_start, "d_end": now}
                    ).fetchone()
                    if row and row[0] is not None:
                        # Use coin symbol (BTC/ETH/SOL) as trading identifier
                        coin_sym_map = {"bitcoin": "BTC", "ethereum": "ETH", "solana": "SOL"}
                        sym = coin_sym_map.get(coin_id, coin_id.upper())
                        prices[sym] = float(row[0])

        return prices

    def _get_price_history_intraday_as_of(
        self, symbol: str, now: datetime, limit: int = 400
    ) -> list[float]:
        """Fetch INTRADAY price history for feature building as-of `now`.

        Returns ASC list of floats using the correct price column per market.
        Falls back to empty list on any error.
        """
        from src.database import repository as repo

        market = self.config.market
        try:
            if market == "GOLD":
                # symbol is "source_product_type" (same convention as daily path)
                parts = symbol.split("_", 1)
                if len(parts) != 2:
                    return []
                source = parts[0]
                rows = repo.get_gold_intraday_asc_as_of(source, now, limit=limit)
                prices: list[float] = []
                for r in rows:
                    if r.sell_price is not None:
                        prices.append(float(r.sell_price))
                    elif r.buy_price is not None:
                        prices.append(float(r.buy_price))
                return prices

            elif market == "NASDAQ":
                rows = repo.get_nasdaq_intraday_asc_as_of(symbol, now, limit=limit)
                return [float(r.close_price) for r in rows if r.close_price is not None]

            elif market == "SP500":
                rows = repo.get_sp500_intraday_asc_as_of(symbol, now, limit=limit)
                return [float(r.close_price) for r in rows if r.close_price is not None]

            elif market == "CRYPTO":
                # symbol is BTC/ETH/SOL; map to coin_id for intraday lookup
                coin_map = {"BTC": "bitcoin", "ETH": "ethereum", "SOL": "solana"}
                coin_id = coin_map.get(symbol, symbol.lower())
                rows = repo.get_crypto_intraday_asc_as_of(coin_id, now, limit=limit)
                return [float(r.price) for r in rows if r.price is not None]
        except Exception:
            return []

        return []

    def _get_peak_prices_since_entry(
        self,
        symbols: list[str],
        now: datetime,
        current_prices: Optional[dict[str, float]] = None,
    ) -> dict[str, float]:
        """Return {symbol: peak intraday price since entry} for trailing-stop bots.

        Statelessly recomputed every step via SQL MAX(price) on the market's
        intraday table, bounded by [position.entry_at (or entry_date @ 00:00),
        now]. On any error or when no intraday rows exist for the window,
        falls back per-symbol to max(entry_price, current price if known) so
        the trailing stop degrades gracefully to the entry-anchored stop.
        """
        from src.database.connection import session_scope
        import sqlalchemy

        market = self.config.market
        peaks: dict[str, float] = {}

        for symbol in symbols:
            pos = self.portfolio.positions.get(symbol)
            if pos is None:
                continue

            entry_at = pos.entry_at
            if entry_at is None:
                entry_at = datetime.combine(pos.entry_date, datetime.min.time())

            fallback_price = pos.entry_price
            if current_prices and symbol in current_prices:
                fallback_price = max(fallback_price, current_prices[symbol])

            peak_val: Optional[float] = None
            try:
                with session_scope() as session:
                    if market == "GOLD":
                        parts = symbol.split("_", 1)
                        source = parts[0] if parts else symbol
                        row = session.execute(
                            sqlalchemy.text("""
                                SELECT MAX(COALESCE(sell_price, buy_price)) FROM gold_intraday_prices
                                WHERE source = :src
                                  AND timestamp BETWEEN :d_start AND :d_end
                            """),
                            {"src": source, "d_start": entry_at, "d_end": now}
                        ).fetchone()
                        if row and row[0] is not None:
                            peak_val = float(row[0])

                    elif market == "NASDAQ":
                        row = session.execute(
                            sqlalchemy.text("""
                                SELECT MAX(close_price) FROM nasdaq_intraday_prices
                                WHERE symbol = :sym
                                  AND timestamp BETWEEN :d_start AND :d_end
                            """),
                            {"sym": symbol, "d_start": entry_at, "d_end": now}
                        ).fetchone()
                        if row and row[0] is not None:
                            peak_val = float(row[0])

                    elif market == "SP500":
                        row = session.execute(
                            sqlalchemy.text("""
                                SELECT MAX(close_price) FROM sp500_intraday_prices
                                WHERE symbol = :sym
                                  AND timestamp BETWEEN :d_start AND :d_end
                            """),
                            {"sym": symbol, "d_start": entry_at, "d_end": now}
                        ).fetchone()
                        if row and row[0] is not None:
                            peak_val = float(row[0])

                    elif market == "CRYPTO":
                        coin_map = {"BTC": "bitcoin", "ETH": "ethereum", "SOL": "solana"}
                        coin_id = coin_map.get(symbol, symbol.lower())
                        row = session.execute(
                            sqlalchemy.text("""
                                SELECT MAX(price) FROM crypto_intraday_prices
                                WHERE coin_id = :cid
                                  AND timestamp BETWEEN :d_start AND :d_end
                            """),
                            {"cid": coin_id, "d_start": entry_at, "d_end": now}
                        ).fetchone()
                        if row and row[0] is not None:
                            peak_val = float(row[0])
            except Exception:
                peak_val = None

            if peak_val is None or peak_val <= 0:
                peak_val = fallback_price

            peaks[symbol] = peak_val

        return peaks

    def _step_rl(self, sim_date: date, closed_this_step: set[str],
                 now: Optional[datetime] = None) -> list[Trade]:
        """RL native step: for each tradable symbol, build observation and call policy.act().

        When `now` is provided (hourly live-step / replay), uses INTRADAY price tables
        (`*_intraday_prices`) for both the current price lookup and the feature history.
        When `now` is None (backtest path), falls back to the existing daily tables.
        This fixes the horizon mismatch: the model was trained on daily data but deployed
        hourly — now both training (via rl_replay) and inference use the same hourly cadence.
        """
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

        # Choose data source: intraday (hourly) when now is provided, else daily (backtest).
        use_intraday = now is not None

        if use_intraday:
            # Intraday path: fetch symbol → price from *_intraday_prices as-of now.
            symbol_prices = self._get_symbols_and_prices_intraday_as_of(now)
            as_of_dt = now
        else:
            # Daily/backtest path: existing behaviour.
            symbol_prices = self._get_symbols_and_prices_as_of(sim_date, repo)
            as_of_dt = datetime.combine(sim_date, datetime.max.time())

        # Per-symbol bot: restrict to the single tracked symbol.
        if self.config.symbol is not None:
            symbol_prices = {k: v for k, v in symbol_prices.items() if k == self.config.symbol}

        for symbol, current_price in symbol_prices.items():
            if current_price <= 0:
                continue

            # Fetch price history for feature building using the same data source.
            if use_intraday:
                prices_list = self._get_price_history_intraday_as_of(symbol, as_of_dt)
            else:
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

            # Low-conviction BUY → treat as HOLD (anti-churn; exits untouched)
            if action == 1 and confidence < _RL_BUY_CONF_FLOOR:
                action = 0

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

    def _step_conviction(self, sim_date: date, closed_this_step: set[str],
                         now: Optional[datetime] = None) -> list[Trade]:
        """Conviction step for transformer_nn: direction-head P(up) decides.

        Rationale: transformer_nn predicts the NEXT HOUR, so its |Δprice| is
        ~0.1-0.3% — it can never clear the %-magnitude buy/sell thresholds
        the standard branch uses (designed for daily-scale moves). Instead the
        dual head's calibra-ish P(up) is the signal, meta_stack style:

          delta_buy  = buy_threshold  / 100      (variant dial)
          delta_sell = sell_threshold / 100
          BUY   when p_up >= max(0.5 + delta_buy, _CONVICTION_BUY_FLOOR)
          SELL  when holding and p_up <= min(0.5 - delta_sell, _CONVICTION_SELL_CEIL)
          otherwise HOLD (incl. p_up=None: no checkpoint / legacy head / error)

        Data source mirrors _step_rl: intraday tables when `now` is given
        (hourly live-step), daily tables in backtest. SL/TP hard guard
        (including trailing for _v11/_v12) already ran in step().
        """
        from src.algorithms.registry import get_algos_for_market, get_algos_for_symbol
        from src.algorithms.transformer_model import MIN_DATA_POINTS_TF
        from src.database import repository as repo

        trades: list[Trade] = []
        registry_market = _SIM_TO_REGISTRY_MARKET.get(self.config.market, self.config.market)

        try:
            if self.config.symbol is not None:
                algos = get_algos_for_symbol(registry_market, self.config.symbol)
            else:
                algos = get_algos_for_market(registry_market)
            model = algos.get("transformer_nn")
            if model is None:
                return trades
        except Exception:
            return trades

        use_intraday = now is not None
        if use_intraday:
            symbol_prices = self._get_symbols_and_prices_intraday_as_of(now)
            as_of_dt = now
        else:
            symbol_prices = self._get_symbols_and_prices_as_of(sim_date, repo)
            as_of_dt = datetime.combine(sim_date, datetime.max.time())

        if self.config.symbol is not None:
            symbol_prices = {k: v for k, v in symbol_prices.items()
                             if k == self.config.symbol}

        buy_th = max(0.5 + self.config.buy_threshold / 100.0, _CONVICTION_BUY_FLOOR)
        sell_th = min(0.5 - self.config.sell_threshold / 100.0, _CONVICTION_SELL_CEIL)

        for symbol, current_price in symbol_prices.items():
            if current_price <= 0:
                continue

            if use_intraday:
                prices_list = self._get_price_history_intraday_as_of(symbol, as_of_dt)
            else:
                prices_list = self._get_price_history_as_of(symbol, as_of_dt, repo)
            if len(prices_list) < MIN_DATA_POINTS_TF:
                continue

            model._context_symbol = symbol
            p_up = model.predict_direction_proba(prices_list)
            if p_up is None:
                continue   # no trained dual-head model → hold

            confidence = min(0.85, max(0.35, 0.35 + abs(p_up - 0.5)))

            if p_up >= buy_th:
                if symbol in closed_this_step:
                    continue
                trade = self.portfolio.buy(
                    symbol=symbol,
                    price=current_price,
                    trade_date=sim_date,
                    signal_strength=round((p_up - 0.5) * 100.0, 4),
                    confidence=confidence,
                    trade_at=now,
                )
                if trade:
                    trades.append(trade)
            elif p_up <= sell_th and symbol in self.portfolio.positions:
                trade = self.portfolio.sell(
                    symbol=symbol,
                    price=current_price,
                    trade_date=sim_date,
                    close_reason="conviction_signal",
                    trade_at=now,
                )
                if trade:
                    trade.confidence = confidence
                    trades.append(trade)

        return trades

    def _step_meta(self, sim_date: date, closed_this_step: set[str],
                   now: Optional[datetime] = None) -> list[Trade]:
        """Meta-stacking native step: P(up|x_t) → conviction-sized position.

        Policy — adaptive threshold (2-pass):
          delta = buy_threshold / 100        (reuse existing config)
          p = P(up | x_t)                    (trained model or fallback vote)

          Pass 1: compute p for ALL tradable symbols of this step.
          Pass 2: derive adaptive threshold then decide per symbol:

            threshold = max(0.5 + delta, mu_p + K * sigma_p)   [K = _META_ADAPTIVE_K]

            When only 1 symbol is available (per-symbol bot or single-symbol
            market): sigma_p is undefined → threshold = 0.5 + delta (absolute
            floor only; K is not applied).

          p > threshold:                   BUY  with size = clamp((p−0.5)/0.5, 0..1) × max_position_pct
          p < 0.5 − delta  AND holding:   SELL
          otherwise:                       HOLD

        Floor semantics: the absolute floor (0.5+delta) guarantees we NEVER buy
        a symbol the model rates as likely to fall, even when it is the "least
        bad" of a down-sweep step.  If no symbol clears the threshold → hold cash.
        That is correct behaviour.

        SL/TP hard guard runs before this method (in step()), same as all bots.
        """
        import numpy as np
        from src.database import repository as repo
        from src.simulation.meta_stack import get_meta_model

        trades: list[Trade] = []

        # Load or retrieve cached meta model for this market (+ optional symbol)
        is_per_symbol = self.config.symbol is not None
        meta = get_meta_model(
            self.config.market,
            self.config.symbol if is_per_symbol else None,
        )

        # Enumerate tradable symbols and current prices as-of sim_date
        symbol_prices = self._get_symbols_and_prices_as_of(sim_date, repo)
        if is_per_symbol:
            symbol_prices = {k: v for k, v in symbol_prices.items()
                             if k == self.config.symbol}

        as_of_dt = datetime.combine(sim_date, datetime.max.time())
        delta = self.config.buy_threshold / 100.0  # confidence dead-band

        # ------------------------------------------------------------------
        # Pass 1: compute p for all tradable symbols
        # ------------------------------------------------------------------
        # symbol_data: {symbol: (p, current_price)}
        symbol_data: dict[str, tuple[float, float]] = {}

        for symbol, current_price in symbol_prices.items():
            if current_price <= 0:
                continue

            # Price history for sigma/momentum features
            prices_list = self._get_price_history_as_of(symbol, as_of_dt, repo)

            # Build feature vector (AS-OF as_of_dt, no leakage)
            try:
                x = meta.build_features_for_inference(symbol, as_of_dt, prices_list)
                if x is None:
                    continue
            except Exception:
                continue

            # Get P(up)
            try:
                p = meta.predict_proba(x)
            except Exception:
                p = 0.5  # safest neutral assumption on any error

            symbol_data[symbol] = (p, current_price)

        if not symbol_data:
            return trades

        # ------------------------------------------------------------------
        # Pass 2: adaptive threshold, then decide per symbol
        # ------------------------------------------------------------------
        all_p = [p for p, _ in symbol_data.values()]

        if len(all_p) > 1:
            # Cross-section available: adaptive threshold
            mu_p = float(np.mean(all_p))
            sigma_p = float(np.std(all_p))
            threshold = max(0.5 + delta, mu_p + _META_ADAPTIVE_K * sigma_p)
        else:
            # Single symbol — no cross-section; use absolute floor only
            threshold = 0.5 + delta

        for symbol, (p, current_price) in symbol_data.items():
            has_position = symbol in self.portfolio.positions

            if p > threshold:
                # BUY: size scaled by conviction
                if symbol in closed_this_step:
                    continue
                conviction = min(1.0, max(0.0, (p - 0.5) / 0.5))
                pos_pct = conviction * self.config.max_position_pct
                trade = self.portfolio.buy(
                    symbol=symbol,
                    price=current_price,
                    trade_date=sim_date,
                    signal_strength=round(p - 0.5, 4),
                    confidence=round(p, 4),
                    trade_at=now,
                    position_pct=pos_pct,
                )
                if trade:
                    trades.append(trade)

            elif p < 0.5 - delta and has_position:
                # SELL: close existing position
                trade = self.portfolio.sell(
                    symbol=symbol,
                    price=current_price,
                    trade_date=sim_date,
                    close_reason="meta_signal",
                    trade_at=now,
                )
                if trade:
                    trade.confidence = round(p, 4)
                    trades.append(trade)

            else:
                # HOLD
                trades.append(Trade(
                    symbol=symbol,
                    action="HOLD",
                    quantity=0,
                    price=current_price,
                    trade_value=0.0,
                    trade_date=sim_date,
                    signal_strength=round(p - 0.5, 4),
                    confidence=round(p, 4),
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
