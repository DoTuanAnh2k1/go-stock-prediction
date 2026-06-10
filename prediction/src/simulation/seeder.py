"""Seed sim_bots table with all 66 trading bots (6 markets × 11 algorithms)."""
from __future__ import annotations

from decimal import Decimal

from src.database.connection import session_scope
from src.database.models import SimBot
from src.utils.logger import get_logger

log = get_logger("simulation.seeder")

ALGORITHMS = [
    ("moving_average", "Moving Average"),
    ("ema", "EMA/MACD"),
    ("lstm_nn", "LSTM"),
    ("arima_garch", "ARIMA-GARCH"),
    ("lightgbm", "LightGBM"),
    ("sarima", "SARIMA"),
    ("egarch", "EGARCH"),
    ("gru_nn", "GRU"),
    ("random_forest", "Random Forest"),
    ("xgboost", "XGBoost"),
    ("ensemble", "Ensemble"),
]

MARKETS = [
    ("VN30", "VN30", Decimal("1000000000"), "VND"),
    ("GOLD", "Gold", Decimal("100000"), "USD"),
    ("NASDAQ", "NASDAQ", Decimal("100000"), "USD"),
    ("SP500", "S&P 500", Decimal("100000"), "USD"),
    ("CRYPTO", "Crypto", Decimal("100000"), "USD"),
]


def seed_bots() -> int:
    """Insert sim_bots rows if they don't already exist. Returns count inserted."""
    inserted = 0

    with session_scope() as session:
        for market_key, market_display, initial_capital, currency in MARKETS:
            for algo_key, algo_display in ALGORITHMS:
                bot_id = f"{market_key.lower()}_{algo_key}"
                display_name = f"{market_display} — {algo_display}"

                existing = session.query(SimBot).filter(SimBot.id == bot_id).first()
                if existing is None:
                    bot = SimBot(
                        id=bot_id,
                        market=market_key,
                        algorithm=algo_key,
                        display_name=display_name,
                        initial_capital=initial_capital,
                        currency=currency,
                        buy_threshold=Decimal("1.50"),
                        sell_threshold=Decimal("1.00"),
                        min_confidence=Decimal("0.60"),
                        stop_loss=Decimal("5.00"),
                        take_profit=Decimal("8.00"),
                        max_position_pct=Decimal("15.00"),
                        max_positions=5,
                        is_active=True,
                    )
                    session.add(bot)
                    inserted += 1

        session.commit()

    log.info("sim.seeder.done", inserted=inserted)
    return inserted
