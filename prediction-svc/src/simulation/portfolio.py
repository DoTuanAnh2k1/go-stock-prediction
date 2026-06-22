"""Portfolio — manages cash, positions, and tracks trades."""
from __future__ import annotations

from dataclasses import dataclass, field
from datetime import date, datetime
from typing import Optional


@dataclass
class Position:
    symbol: str
    quantity: float
    entry_price: float
    entry_date: date
    entry_trade_id: Optional[int] = None


@dataclass
class Trade:
    symbol: str
    action: str           # BUY or SELL
    quantity: float
    price: float
    trade_value: float
    trade_date: date
    signal_strength: Optional[float] = None
    confidence: Optional[float] = None
    close_reason: Optional[str] = None
    entry_trade_id: Optional[int] = None
    pnl: Optional[float] = None
    pnl_pct: Optional[float] = None
    trade_at: Optional[datetime] = None   # full datetime for hourly granularity


class Portfolio:
    def __init__(self, initial_capital: float, stop_loss_pct: float,
                 take_profit_pct: float, max_position_pct: float, max_positions: int):
        self.initial_capital = initial_capital
        self.cash = initial_capital
        self.positions: dict[str, Position] = {}
        self.stop_loss_pct = stop_loss_pct        # e.g. 5.0 → -5%
        self.take_profit_pct = take_profit_pct    # e.g. 8.0 → +8%
        self.max_position_pct = max_position_pct  # e.g. 15.0
        self.max_positions = max_positions
        self._trade_id_counter = 0

    def _next_id(self) -> int:
        self._trade_id_counter += 1
        return self._trade_id_counter

    def buy(self, symbol: str, price: float, trade_date: date,
            signal_strength: float = None, confidence: float = None,
            trade_at: Optional[datetime] = None) -> Optional[Trade]:
        """Execute a BUY if within position limits."""
        if price <= 0:
            return None
        if len(self.positions) >= self.max_positions:
            return None
        if symbol in self.positions:
            return None  # already holding this symbol

        # Position size: min(max_position_pct% of total, cash / remaining_slots)
        total_value = self.total_value({symbol: price})
        max_by_pct = total_value * (self.max_position_pct / 100.0)
        remaining_slots = max(1, self.max_positions - len(self.positions))
        max_by_slots = self.cash / remaining_slots
        position_cash = min(max_by_pct, max_by_slots, self.cash)

        if position_cash <= 0:
            return None

        quantity = position_cash / price
        trade_value = quantity * price

        trade_id = self._next_id()
        self.positions[symbol] = Position(
            symbol=symbol,
            quantity=quantity,
            entry_price=price,
            entry_date=trade_date,
            entry_trade_id=trade_id,
        )
        self.cash -= trade_value

        return Trade(
            symbol=symbol, action="BUY", quantity=quantity, price=price,
            trade_value=trade_value, trade_date=trade_date,
            signal_strength=signal_strength, confidence=confidence,
            trade_at=trade_at,
        )

    def sell(self, symbol: str, price: float, trade_date: date,
             close_reason: str = "signal",
             trade_at: Optional[datetime] = None) -> Optional[Trade]:
        """Execute a SELL for all of a position."""
        if symbol not in self.positions:
            return None

        pos = self.positions.pop(symbol)
        trade_value = pos.quantity * price
        self.cash += trade_value

        pnl = (price - pos.entry_price) * pos.quantity
        pnl_pct = (price - pos.entry_price) / pos.entry_price * 100 if pos.entry_price > 0 else 0.0

        return Trade(
            symbol=symbol, action="SELL", quantity=pos.quantity, price=price,
            trade_value=trade_value, trade_date=trade_date,
            close_reason=close_reason,
            entry_trade_id=pos.entry_trade_id,
            pnl=pnl, pnl_pct=pnl_pct,
            trade_at=trade_at,
        )

    def check_stop_loss_take_profit(self, current_prices: dict[str, float], trade_date: date,
                                     trade_at: Optional[datetime] = None) -> list[Trade]:
        """Auto-sell positions that hit SL or TP."""
        trades = []
        for symbol in list(self.positions.keys()):
            price = current_prices.get(symbol)
            if price is None or price <= 0:
                continue

            pos = self.positions[symbol]
            change_pct = (price - pos.entry_price) / pos.entry_price * 100

            if change_pct <= -self.stop_loss_pct:
                trade = self.sell(symbol, price, trade_date, close_reason="stop_loss", trade_at=trade_at)
                if trade:
                    trades.append(trade)
            elif change_pct >= self.take_profit_pct:
                trade = self.sell(symbol, price, trade_date, close_reason="take_profit", trade_at=trade_at)
                if trade:
                    trades.append(trade)

        return trades

    def total_value(self, current_prices: dict[str, float]) -> float:
        positions_value = sum(
            pos.quantity * current_prices.get(sym, pos.entry_price)
            for sym, pos in self.positions.items()
        )
        return self.cash + positions_value

    def snapshot(self, current_prices: dict[str, float], snap_date: date,
                 snap_at: Optional[datetime] = None) -> dict:
        """Build a portfolio snapshot dict.

        snap_date — the calendar date (DATE type, hypertable partition column).
        snap_at   — the full datetime for hourly granularity (new upsert key).
                    Defaults to None for backtest paths that don't need sub-day resolution.
        """
        positions_value = sum(
            pos.quantity * current_prices.get(sym, pos.entry_price)
            for sym, pos in self.positions.items()
        )
        total = self.cash + positions_value
        total_return_pct = (total - self.initial_capital) / self.initial_capital * 100 if self.initial_capital > 0 else 0.0
        return {
            "snapshot_date": snap_date,
            "snapshot_at": snap_at,
            "cash_balance": self.cash,
            "positions_value": positions_value,
            "total_value": total,
            "total_return_pct": total_return_pct,
            "open_positions": len(self.positions),
        }
