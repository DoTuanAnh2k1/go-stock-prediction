"""SignalGenerator — reads predictions from DB for a given market/algorithm/date."""
from __future__ import annotations

import sqlalchemy
from dataclasses import dataclass
from datetime import date
from decimal import Decimal
from typing import Literal

from src.utils.logger import get_logger

log = get_logger("simulation.signal")


def _derive_signals(
    rows: list[dict],
    buy_threshold: float,
    sell_threshold: float,
    min_confidence: float,
    for_date: date,
) -> list["TradeSignal"]:
    """Convert raw prediction rows into TradeSignal objects applying thresholds."""
    signals: list[TradeSignal] = []
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

    # Market → table name for prediction queries
    MARKET_TO_TABLE = {
        "GOLD": "gold_predictions",
        "NASDAQ": "nasdaq_predictions",
        "SP500": "sp500_predictions",
        "CRYPTO": "crypto_predictions",
    }

    def get_signals(
        self,
        market: str,
        algorithm: str,
        for_date: date,
        buy_threshold: float,
        sell_threshold: float,
        min_confidence: float,
        cached_rows: list[dict] | None = None,
    ) -> list[TradeSignal]:
        """
        Returns list of TradeSignal for the given market/algorithm/date.
        Uses prediction rows where prediction_date matches for_date.

        If cached_rows is provided the DB query is skipped and signal derivation
        runs directly on the supplied rows (same threshold logic applies).  This
        allows the engine to pre-fetch predictions once per distinct algorithm and
        share them across many bots without duplicating DB round-trips.
        """
        if cached_rows is not None:
            return _derive_signals(cached_rows, buy_threshold, sell_threshold, min_confidence, for_date)

        from src.database.connection import session_scope

        with session_scope() as session:
            rows = self._query_predictions(session, market, algorithm, for_date)

        return _derive_signals(rows, buy_threshold, sell_threshold, min_confidence, for_date)

    def fetch_predictions(self, market: str, algorithm: str, for_date: date) -> list[dict]:
        """Fetch raw prediction rows for (market, algorithm, for_date).

        Returns the same dict shape used by get_signals:
          {symbol, predicted_price, current_price, confidence}

        Intended for the engine's StepDataCache to call once per distinct
        algorithm and share the result across bots.
        """
        from src.database.connection import session_scope

        with session_scope() as session:
            return self._query_predictions(session, market, algorithm, for_date)

    def _query_predictions(self, session, market: str, algorithm: str, for_date: date) -> list[dict]:
        """Raw SQL query for predictions on a given date."""
        from datetime import datetime

        date_start = datetime.combine(for_date, datetime.min.time())
        date_end = datetime.combine(for_date, datetime.max.time())

        if market == "GOLD":
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

            if market == "GOLD":
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
