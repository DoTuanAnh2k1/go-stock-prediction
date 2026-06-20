"""Seed sim_bots table with all trading bots (4 markets × 11 algorithms × 10 variants)."""
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
    ("GOLD", "Gold", Decimal("1000"), "USD"),
    ("NASDAQ", "NASDAQ", Decimal("1000"), "USD"),
    ("SP500", "S&P 500", Decimal("1000"), "USD"),
    ("CRYPTO", "Crypto", Decimal("1000"), "USD"),
]

# Variant configs: (suffix, label, buy, sell, conf, sl, tp)
# suffix="" is the original/default bot (no suffix on ID).
VARIANTS = [
    ("",    "Default",         Decimal("0.50"), Decimal("0.30"), Decimal("0.40"), Decimal("5.00"),  Decimal("8.00")),
    ("_v2", "Conservative",    Decimal("1.00"), Decimal("0.80"), Decimal("0.60"), Decimal("5.00"),  Decimal("10.00")),
    ("_v3", "Aggressive",      Decimal("0.30"), Decimal("0.20"), Decimal("0.30"), Decimal("3.00"),  Decimal("5.00")),
    ("_v4", "High Confidence", Decimal("0.50"), Decimal("0.30"), Decimal("0.70"), Decimal("5.00"),  Decimal("8.00")),
    ("_v5", "Trend Follow",    Decimal("1.50"), Decimal("0.50"), Decimal("0.50"), Decimal("7.00"),  Decimal("15.00")),
    ("_v6", "Tight Exit",      Decimal("0.50"), Decimal("0.30"), Decimal("0.40"), Decimal("3.00"),  Decimal("5.00")),
    ("_v7", "Wide Exit",       Decimal("0.50"), Decimal("0.30"), Decimal("0.40"), Decimal("8.00"),  Decimal("15.00")),
    ("_v8", "Momentum",        Decimal("0.80"), Decimal("0.50"), Decimal("0.55"), Decimal("6.00"),  Decimal("12.00")),
    ("_v9", "Scalping",        Decimal("0.20"), Decimal("0.20"), Decimal("0.30"), Decimal("2.00"),  Decimal("3.00")),
    ("_v10","Swing",           Decimal("2.00"), Decimal("1.00"), Decimal("0.65"), Decimal("10.00"), Decimal("20.00")),
]


def seed_bots() -> int:
    """Insert sim_bots rows if they don't already exist. Returns count inserted."""
    inserted = 0

    with session_scope() as session:
        for market_key, market_display, initial_capital, currency in MARKETS:
            for algo_key, algo_display in ALGORITHMS:
                for suffix, variant_label, buy, sell, conf, sl, tp in VARIANTS:
                    bot_id = f"{market_key.lower()}_{algo_key}{suffix}"
                    if suffix:
                        display_name = f"{market_display} — {algo_display} ({variant_label})"
                    else:
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
                            buy_threshold=buy,
                            sell_threshold=sell,
                            min_confidence=conf,
                            stop_loss=sl,
                            take_profit=tp,
                            max_position_pct=Decimal("15.00"),
                            max_positions=5,
                            is_active=True,
                        )
                        session.add(bot)
                        inserted += 1

        session.commit()

    log.info("sim.seeder.done", inserted=inserted)
    return inserted
