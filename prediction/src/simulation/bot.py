"""TradingBot — state machine for one market×algorithm bot."""
from __future__ import annotations

from dataclasses import dataclass
from datetime import date
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

    def step(self, sim_date: date) -> list[Trade]:
        """Run one simulation day: SL/TP check then process new signals."""
        trades = []

        # Get current prices for SL/TP check
        closed_this_step: set[str] = set()
        if self.portfolio.positions:
            current_prices = self.signal_gen.get_current_prices(
                self.config.market,
                list(self.portfolio.positions.keys()),
                sim_date,
            )
            sl_tp_trades = self.portfolio.check_stop_loss_take_profit(current_prices, sim_date)
            trades.extend(sl_tp_trades)
            # Block same-step re-entry: a stale prediction signal on a symbol
            # that just hit SL/TP would re-open the position at the same bad price.
            closed_this_step = {t.symbol for t in sl_tp_trades}

        # Get new signals
        signals = self.signal_gen.get_signals(
            market=self.config.market,
            algorithm=self.config.algorithm,
            for_date=sim_date,
            buy_threshold=self.config.buy_threshold,
            sell_threshold=self.config.sell_threshold,
            min_confidence=self.config.min_confidence,
        )

        for signal in signals:
            if signal.action == "BUY":
                if signal.symbol in closed_this_step:
                    continue
                trade = self.portfolio.buy(
                    symbol=signal.symbol,
                    price=signal.current_price,
                    trade_date=sim_date,
                    signal_strength=signal.signal_strength,
                    confidence=signal.confidence,
                )
                if trade:
                    trades.append(trade)
            elif signal.action == "SELL":
                trade = self.portfolio.sell(
                    symbol=signal.symbol,
                    price=signal.current_price,
                    trade_date=sim_date,
                    close_reason="signal",
                )
                if trade:
                    trades.append(trade)
            else:
                trades.append(Trade(
                    symbol=signal.symbol,
                    action="HOLD",
                    quantity=0,
                    price=signal.current_price,
                    trade_value=0,
                    trade_date=sim_date,
                    signal_strength=signal.signal_strength,
                    confidence=signal.confidence,
                ))

        return trades

    def get_snapshot(self, sim_date: date) -> dict:
        # Use entry prices as fallback current prices
        current_prices = {}
        if self.portfolio.positions:
            current_prices = self.signal_gen.get_current_prices(
                self.config.market,
                list(self.portfolio.positions.keys()),
                sim_date,
            )
        return self.portfolio.snapshot(current_prices, sim_date)
