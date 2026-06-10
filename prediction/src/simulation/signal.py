"""SignalGenerator — reads predictions from DB for a given market/algorithm/date."""
from __future__ import annotations

import sqlalchemy
from dataclasses import dataclass
from datetime import date
from decimal import Decimal
from typing import Literal

from src.utils.logger import get_logger

log = get_logger("simulation.signal")


@dataclass
class TradeSignal:
    symbol: str
    action: Literal["BUY", "SELL", "HOLD"]
    signal_strength: float   # % change predicted
    confidence: float
    predicted_price: float
    current_price: float
    prediction_date: date


class SignalGenerator:
    """Reads predictions from DB and generates trade signals."""

    # Market → symbol field name in the prediction table
    # For VN30, the symbol comes via JOIN with stocks table
    MARKET_TO_TABLE = {
        "VN30": "predictions",
        "GOLD": "gold_predictions",
        "NASDAQ": "nasdaq_predictions",
        "SP500": "sp500_predictions",
        "CRYPTO": "crypto_predictions",
    }

    def get_signals(self, market: str, algorithm: str, for_date: date,
                    buy_threshold: float, sell_threshold: float,
                    min_confidence: float) -> list[TradeSignal]:
        """
        Returns list of TradeSignal for the given market/algorithm/date.
        Uses prediction rows where prediction_date matches for_date.
        """
        from datetime import datetime
        from src.database.connection import session_scope

        signals = []

        with session_scope() as session:
            rows = self._query_predictions(session, market, algorithm, for_date)

            for row in rows:
                try:
                    predicted = float(row["predicted_price"])
                    current = float(row["current_price"])
                    if current == 0:
                        continue

                    confidence = float(row["confidence"] or 0)
                    signal_strength = (predicted - current) / current * 100

                    if signal_strength > buy_threshold and confidence > min_confidence:
                        action = "BUY"
                    elif signal_strength < -sell_threshold and confidence > min_confidence:
                        action = "SELL"
                    else:
                        action = "HOLD"

                    signals.append(TradeSignal(
                        symbol=row["symbol"],
                        action=action,
                        signal_strength=signal_strength,
                        confidence=confidence,
                        predicted_price=predicted,
                        current_price=current,
                        prediction_date=for_date,
                    ))
                except Exception as exc:
                    log.warning("signal.row_error", error=str(exc))
                    continue

        return signals

    def _query_predictions(self, session, market: str, algorithm: str, for_date: date) -> list[dict]:
        """Raw SQL query for predictions on a given date."""
        from datetime import datetime

        date_start = datetime.combine(for_date, datetime.min.time())
        date_end = datetime.combine(for_date, datetime.max.time())

        if market == "VN30":
            result = session.execute(
                sqlalchemy.text("""
                    SELECT s.symbol, p.predicted_price, p.current_price, p.confidence
                    FROM predictions p
                    JOIN stocks s ON p.stock_id = s.id
                    WHERE p.algorithm_name = :algo
                      AND p.prediction_date BETWEEN :d_start AND :d_end
                      AND s.is_vn30 = 1
                      AND p.deleted_at IS NULL
                """),
                {"algo": algorithm, "d_start": date_start, "d_end": date_end}
            ).fetchall()
            return [{"symbol": r[0], "predicted_price": r[1], "current_price": r[2], "confidence": r[3]} for r in result]

        elif market == "GOLD":
            result = session.execute(
                sqlalchemy.text("""
                    SELECT CONCAT(source, '_', product_type) as symbol,
                           predicted_price, current_price, confidence
                    FROM gold_predictions
                    WHERE algorithm_name = :algo
                      AND prediction_date BETWEEN :d_start AND :d_end
                      AND deleted_at IS NULL
                """),
                {"algo": algorithm, "d_start": date_start, "d_end": date_end}
            ).fetchall()
            return [{"symbol": r[0], "predicted_price": r[1], "current_price": r[2], "confidence": r[3]} for r in result]

        elif market == "NASDAQ":
            result = session.execute(
                sqlalchemy.text("""
                    SELECT symbol, predicted_price, current_price, confidence
                    FROM nasdaq_predictions
                    WHERE algorithm_name = :algo
                      AND prediction_date BETWEEN :d_start AND :d_end
                      AND deleted_at IS NULL
                """),
                {"algo": algorithm, "d_start": date_start, "d_end": date_end}
            ).fetchall()
            return [{"symbol": r[0], "predicted_price": r[1], "current_price": r[2], "confidence": r[3]} for r in result]

        elif market == "SP500":
            result = session.execute(
                sqlalchemy.text("""
                    SELECT symbol, predicted_price, current_price, confidence
                    FROM sp500_predictions
                    WHERE algorithm_name = :algo
                      AND prediction_date BETWEEN :d_start AND :d_end
                      AND deleted_at IS NULL
                """),
                {"algo": algorithm, "d_start": date_start, "d_end": date_end}
            ).fetchall()
            return [{"symbol": r[0], "predicted_price": r[1], "current_price": r[2], "confidence": r[3]} for r in result]

        elif market == "CRYPTO":
            result = session.execute(
                sqlalchemy.text("""
                    SELECT symbol, predicted_price, current_price, confidence
                    FROM crypto_predictions
                    WHERE algorithm_name = :algo
                      AND prediction_date BETWEEN :d_start AND :d_end
                      AND deleted_at IS NULL
                """),
                {"algo": algorithm, "d_start": date_start, "d_end": date_end}
            ).fetchall()
            return [{"symbol": r[0], "predicted_price": r[1], "current_price": r[2], "confidence": r[3]} for r in result]

        return []

    def get_current_prices(self, market: str, symbols: list[str], for_date: date) -> dict[str, float]:
        """Returns {symbol: price} for all positions that need SL/TP checking."""
        from datetime import datetime, timedelta
        from src.database.connection import session_scope

        if not symbols:
            return {}

        prices = {}

        with session_scope() as session:
            # Look back up to 7 days to find most recent price
            date_start = datetime.combine(for_date - timedelta(days=7), datetime.min.time())
            date_end = datetime.combine(for_date, datetime.max.time())

            if market == "VN30":
                for symbol in symbols:
                    result = session.execute(
                        sqlalchemy.text("""
                            SELECT sp.close_price FROM stock_prices sp
                            JOIN stocks s ON sp.stock_id = s.id
                            WHERE s.symbol = :sym
                              AND sp.trading_date BETWEEN :d_start AND :d_end
                              AND sp.deleted_at IS NULL
                            ORDER BY sp.trading_date DESC LIMIT 1
                        """),
                        {"sym": symbol, "d_start": date_start, "d_end": date_end}
                    ).fetchone()
                    if result:
                        prices[symbol] = float(result[0])

            elif market == "GOLD":
                for symbol in symbols:
                    # symbol is "source_product_type"
                    parts = symbol.split("_", 1)
                    if len(parts) == 2:
                        src, ptype = parts[0], parts[1]
                        result = session.execute(
                            sqlalchemy.text("""
                                SELECT sell_price FROM gold_prices
                                WHERE source = :src AND product_type = :ptype
                                  AND trading_date BETWEEN :d_start AND :d_end
                                ORDER BY trading_date DESC LIMIT 1
                            """),
                            {"src": src, "ptype": ptype, "d_start": date_start, "d_end": date_end}
                        ).fetchone()
                        if result:
                            prices[symbol] = float(result[0])

            elif market in ("NASDAQ", "SP500", "CRYPTO"):
                table = {"NASDAQ": "nasdaq_prices", "SP500": "sp500_prices", "CRYPTO": "crypto_prices"}[market]
                for symbol in symbols:
                    result = session.execute(
                        sqlalchemy.text(f"""
                            SELECT close_price FROM {table}
                            WHERE symbol = :sym
                              AND trading_date BETWEEN :d_start AND :d_end
                            ORDER BY trading_date DESC LIMIT 1
                        """),
                        {"sym": symbol, "d_start": date_start, "d_end": date_end}
                    ).fetchone()
                    if result:
                        prices[symbol] = float(result[0])

        return prices
