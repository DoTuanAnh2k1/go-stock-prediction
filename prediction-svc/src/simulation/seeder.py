"""Seed sim_bots table with trading bots.

Pooled fleet (always seeded):
  4 markets × 11 algorithms × 10 variants + 4 RL DQN bots = 444 bots.

Per-symbol fleet (seeded only when settings.per_symbol_enabled is true):
  For each (market, symbol): 11 algorithms × 10 variants + 1 RL DQN bot.
  Markets and symbol counts:
    GOLD (3) + NASDAQ (15) + SP500 (16) + CRYPTO (3) = 37 symbols
  Total per-symbol bots: 37 × (11 × 10 + 1) = 37 × 111 = 4,107 bots.
"""
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
    from src.algorithms.registry import PS_SUFFIX
    from src.config import get_settings

    inserted = 0

    with session_scope() as session:
        # -----------------------------------------------------------------------
        # Standard bots: 4 markets × 11 algorithms × 10 variants
        # -----------------------------------------------------------------------
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

        # -----------------------------------------------------------------------
        # RL DQN bots: 1 per market (4 total, no variants — policy decides all)
        # buy/sell threshold are irrelevant for RL native branch but stored for
        # schema compatibility.  SL/TP still acts as a hard safety guard.
        # -----------------------------------------------------------------------
        for market_key, market_display, initial_capital, currency in MARKETS:
            bot_id = f"{market_key.lower()}_rl_dqn"
            display_name = f"{market_display} — RL DQN"
            existing = session.query(SimBot).filter(SimBot.id == bot_id).first()
            if existing is None:
                bot = SimBot(
                    id=bot_id,
                    market=market_key,
                    algorithm="rl_dqn",
                    display_name=display_name,
                    initial_capital=initial_capital,
                    currency=currency,
                    buy_threshold=Decimal("0.50"),   # unused by RL native branch
                    sell_threshold=Decimal("0.30"),  # unused by RL native branch
                    min_confidence=Decimal("0.40"),  # unused by RL native branch
                    stop_loss=Decimal("5.00"),        # SL guard still active
                    take_profit=Decimal("8.00"),      # TP guard still active
                    max_position_pct=Decimal("15.00"),
                    max_positions=5,
                    is_active=True,
                )
                session.add(bot)
                inserted += 1

        # -----------------------------------------------------------------------
        # Per-symbol bots (only when per_symbol_enabled is true)
        # For each (market, symbol): 11 algo × 10 variants + 1 RL DQN
        # algorithm column carries the "__ps" suffix; symbol column is set.
        # -----------------------------------------------------------------------
        if get_settings().per_symbol_enabled:
            inserted += _seed_per_symbol_bots(session)

        session.commit()

    log.info("sim.seeder.done", inserted=inserted)
    return inserted


# ---------------------------------------------------------------------------
# Symbols per market — defined here using crawler constants so the seeder is
# deterministic and does NOT need a live DB to discover symbols.
# ---------------------------------------------------------------------------

_GOLD_SYMBOLS = ["XAU_spot", "BTMC_sjc", "BTMC_nhan_tron"]


def _get_per_symbol_markets() -> list[tuple[str, str, Decimal, str, list[str]]]:
    """Return (market_key, market_display, initial_capital, currency, symbols) list."""
    from src.crawlers.nasdaq import NASDAQ_SYMBOLS
    from src.crawlers.sp500 import SP500_SYMBOLS
    from src.crawlers.crypto import COINS

    crypto_symbols = [symbol for _coin_id, symbol in COINS]  # ["BTC", "ETH", "SOL"]

    return [
        ("GOLD",   "Gold",    Decimal("1000"), "USD", _GOLD_SYMBOLS),
        ("NASDAQ", "NASDAQ",  Decimal("1000"), "USD", list(NASDAQ_SYMBOLS)),
        ("SP500",  "S&P 500", Decimal("1000"), "USD", list(SP500_SYMBOLS)),
        ("CRYPTO", "Crypto",  Decimal("1000"), "USD", crypto_symbols),
    ]


def _seed_per_symbol_bots(session) -> int:
    """Seed per-symbol bots into the already-open session.  Returns rows inserted."""
    from src.algorithms.registry import PS_SUFFIX

    ps_algo_suffix = PS_SUFFIX  # "__ps"
    inserted = 0

    per_symbol_markets = _get_per_symbol_markets()

    for market_key, market_display, initial_capital, currency, symbols in per_symbol_markets:
        for symbol in symbols:
            sym_slug = symbol.lower().replace("/", "_")

            # --- Standard per-symbol bots: 11 algorithms × 10 variants ---
            for algo_key, algo_display in ALGORITHMS:
                ps_algorithm = f"{algo_key}{ps_algo_suffix}"  # e.g. "lightgbm__ps"

                for suffix, variant_label, buy, sell, conf, sl, tp in VARIANTS:
                    # bot_id format: {market}_{sym_slug}_{algo}__ps{variant_suffix}
                    # e.g. "gold_xau_spot_lightgbm__ps", "nasdaq_aapl_lstm_nn__ps_v2"
                    raw_id = f"{market_key.lower()}_{sym_slug}_{algo_key}{ps_algo_suffix}{suffix}"
                    # Truncate to 50 chars to match SimBot PK column length
                    bot_id = raw_id[:50]

                    if suffix:
                        display_name = f"{market_display} {symbol} — {algo_display} ({variant_label})"
                    else:
                        display_name = f"{market_display} {symbol} — {algo_display}"

                    existing = session.query(SimBot).filter(SimBot.id == bot_id).first()
                    if existing is None:
                        bot = SimBot(
                            id=bot_id,
                            market=market_key,
                            algorithm=ps_algorithm,
                            symbol=symbol,
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

            # --- RL DQN per-symbol bot: 1 per (market, symbol) ---
            rl_ps_algorithm = f"rl_dqn{ps_algo_suffix}"  # "rl_dqn__ps"
            rl_bot_id = f"{market_key.lower()}_{sym_slug}_rl_dqn{ps_algo_suffix}"[:50]
            rl_display = f"{market_display} {symbol} — RL DQN"

            existing = session.query(SimBot).filter(SimBot.id == rl_bot_id).first()
            if existing is None:
                bot = SimBot(
                    id=rl_bot_id,
                    market=market_key,
                    algorithm=rl_ps_algorithm,
                    symbol=symbol,
                    display_name=rl_display,
                    initial_capital=initial_capital,
                    currency=currency,
                    buy_threshold=Decimal("0.50"),   # unused by RL native branch
                    sell_threshold=Decimal("0.30"),  # unused by RL native branch
                    min_confidence=Decimal("0.40"),  # unused by RL native branch
                    stop_loss=Decimal("5.00"),        # SL guard still active
                    take_profit=Decimal("8.00"),      # TP guard still active
                    max_position_pct=Decimal("15.00"),
                    max_positions=5,
                    is_active=True,
                )
                session.add(bot)
                inserted += 1

    return inserted
