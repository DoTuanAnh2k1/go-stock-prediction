"""PerformanceMetrics — computes KPIs from portfolio snapshots and closed trades.

KPIs computed:
  - total_return_pct          (final_value - initial) / initial × 100
  - annualized_return_pct     (1 + total_return)^(365/days) - 1
  - sharpe_ratio               mean(daily_returns) / std(daily_returns) × √252
  - max_drawdown_pct           max((peak - trough) / peak)
  - win_rate_pct               winning_trades / total_closed_trades × 100
  - profit_factor              sum(profit) / abs(sum(loss))
  - total_trades               count of closed (SELL) trades
  - avg_trade_duration_days    mean(exit_date - entry_date in days)
  - best_trade_pct             max(pnl_pct) among SELL trades
  - worst_trade_pct            min(pnl_pct) among SELL trades
"""
from __future__ import annotations

import math
from dataclasses import dataclass, field
from datetime import date
from typing import Optional


@dataclass
class SimKPIs:
    total_return_pct: float = 0.0
    annualized_return_pct: float = 0.0
    sharpe_ratio: float = 0.0
    max_drawdown_pct: float = 0.0
    win_rate_pct: float = 0.0
    profit_factor: float = 0.0
    total_trades: int = 0
    avg_trade_duration_days: float = 0.0
    best_trade_pct: float = 0.0
    worst_trade_pct: float = 0.0


class PerformanceMetrics:
    """Compute KPIs from a list of daily portfolio snapshots and closed trades."""

    @staticmethod
    def compute(
        snapshots: list[dict],
        sell_trades: list[dict],
        initial_capital: float,
    ) -> SimKPIs:
        """
        Args:
            snapshots: list of dicts ordered by date ASC, each with:
                - snapshot_date: date
                - total_value: float
                - total_return_pct: float (optional — recomputed if absent)
            sell_trades: list of dicts for SELL trades only, each with:
                - pnl: float | None
                - pnl_pct: float | None
                - trade_date: date | None        (exit date)
                - entry_date: date | None        (when the BUY happened)
            initial_capital: float

        Returns:
            SimKPIs dataclass with all 10 metrics.
        """
        kpis = SimKPIs()

        if not snapshots:
            return kpis

        # ── total_return_pct ─────────────────────────────────────────────────
        final_value = float(snapshots[-1]["total_value"])
        if initial_capital > 0:
            kpis.total_return_pct = (final_value - initial_capital) / initial_capital * 100

        # ── annualized_return_pct ────────────────────────────────────────────
        first_date = snapshots[0]["snapshot_date"]
        last_date = snapshots[-1]["snapshot_date"]
        if isinstance(first_date, str):
            from datetime import datetime
            first_date = datetime.strptime(first_date, "%Y-%m-%d").date()
            last_date = datetime.strptime(last_date, "%Y-%m-%d").date()
        days = (last_date - first_date).days
        if days > 0:
            kpis.annualized_return_pct = (
                math.pow(1 + kpis.total_return_pct / 100, 365.0 / days) - 1
            ) * 100

        # ── sharpe_ratio ─────────────────────────────────────────────────────
        if len(snapshots) >= 2:
            values = [float(s["total_value"]) for s in snapshots]
            daily_returns = [
                (values[i] - values[i - 1]) / values[i - 1]
                for i in range(1, len(values))
                if values[i - 1] > 0
            ]
            if len(daily_returns) >= 2:
                mean_r = sum(daily_returns) / len(daily_returns)
                variance = sum((r - mean_r) ** 2 for r in daily_returns) / (len(daily_returns) - 1)
                std_r = math.sqrt(variance) if variance > 0 else 0.0
                if std_r > 0:
                    kpis.sharpe_ratio = mean_r / std_r * math.sqrt(252)

        # ── max_drawdown_pct ─────────────────────────────────────────────────
        peak = float(snapshots[0]["total_value"])
        max_dd = 0.0
        for snap in snapshots:
            v = float(snap["total_value"])
            if v > peak:
                peak = v
            if peak > 0:
                dd = (peak - v) / peak * 100
                if dd > max_dd:
                    max_dd = dd
        kpis.max_drawdown_pct = -max_dd  # negative convention

        # ── sell-trade metrics ───────────────────────────────────────────────
        closed = [t for t in sell_trades if t.get("pnl") is not None]
        kpis.total_trades = len(closed)

        if closed:
            profits = [float(t["pnl"]) for t in closed if float(t["pnl"]) > 0]
            losses = [float(t["pnl"]) for t in closed if float(t["pnl"]) <= 0]

            kpis.win_rate_pct = len(profits) / len(closed) * 100

            total_profit = sum(profits)
            total_loss = abs(sum(losses)) if losses else 0.0
            kpis.profit_factor = total_profit / total_loss if total_loss > 0 else float("inf")

            pnl_pcts = [float(t["pnl_pct"]) for t in closed if t.get("pnl_pct") is not None]
            if pnl_pcts:
                kpis.best_trade_pct = max(pnl_pcts)
                kpis.worst_trade_pct = min(pnl_pcts)

            # avg_trade_duration_days: use trade_date (exit) - entry_date (buy)
            durations = []
            for t in closed:
                exit_date = t.get("trade_date")
                entry_date = t.get("entry_date")
                if exit_date and entry_date:
                    if isinstance(exit_date, str):
                        from datetime import datetime
                        exit_date = datetime.strptime(exit_date, "%Y-%m-%d").date()
                        entry_date = datetime.strptime(entry_date, "%Y-%m-%d").date()
                    durations.append((exit_date - entry_date).days)
            if durations:
                kpis.avg_trade_duration_days = sum(durations) / len(durations)

        return kpis
