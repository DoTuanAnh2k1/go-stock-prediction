"""Query RPC dispatch — server-side handler for the generic Query(QueryRequest) RPC.

Each handler queries the database via SQLAlchemy (reusing existing models from
src.database.models) and returns a plain Python object that _serialize() converts
to a JSON-safe dict whose keys match the Go json tags exactly so that Go's
json.Unmarshal can decode result_json into the corresponding Go structs without
any field mapping.

Serialization contract
----------------------
* Decimal / price columns  → JSON string  e.g. "2650.50"
* datetime columns         → RFC3339 string with ICT offset "+07:00"
  The DB stores values as TIMESTAMP WITHOUT TIME ZONE in ICT wallclock, so we
  just append "+07:00" to the naive isoformat — no conversion needed.
* *nullable* fields        → None → JSON null
* bool, int                → as-is
* date columns             → "YYYY-MM-DD" string (trading_date, split_date)
"""
from __future__ import annotations

import json
from datetime import datetime, date
from decimal import Decimal
from typing import Any

from sqlalchemy import func, distinct, text

from src.database.connection import session_scope
from src.database.models import (
    CronSchedule,
    CryptoIntradayPrice,
    CryptoPrediction,
    CryptoPrice,
    GoldIntradayPrice,
    GoldPrediction,
    GoldPrice,
    NasdaqIntradayPrice,
    NasdaqPrediction,
    NasdaqPrice,
    PipelineReport,
    SimBot,
    SimPortfolioSnapshot,
    SimSession,
    SimTrade,
    SP500IntradayPrice,
    SP500Prediction,
    SP500Price,
    SyncLog,
    TrainingLog,
)
from src.utils.logger import get_logger

log = get_logger("query_handlers")

_ICT_OFFSET = "+07:00"

# ---------------------------------------------------------------------------
# Serialization helpers
# ---------------------------------------------------------------------------

def _fmt_dt(v: datetime | None) -> str | None:
    """Format a naive ICT datetime as RFC3339 with +07:00 offset."""
    if v is None:
        return None
    # isoformat gives "2026-08-06T09:00:00" or "2026-08-06T09:00:00.123456"
    # Truncate microseconds to seconds for cleanliness, then append offset.
    s = v.strftime("%Y-%m-%dT%H:%M:%S")
    return s + _ICT_OFFSET


def _fmt_date(v: date | datetime | None) -> str | None:
    if v is None:
        return None
    if isinstance(v, datetime):
        return v.strftime("%Y-%m-%d")
    return v.isoformat()  # date.isoformat() → "YYYY-MM-DD"


def _fmt_dec(v: Decimal | None) -> str | None:
    """Serialize Decimal to string; Go shopspring/decimal accepts string."""
    if v is None:
        return None
    return str(v)


def _serialize_model(obj: Any, fields: list[tuple[str, str]]) -> dict:
    """Build a JSON-safe dict from a SQLAlchemy model instance.

    fields = [(json_key, attr_name), ...]
    Type conversion is inferred from the Python type of the attribute value.
    """
    out: dict = {}
    for json_key, attr in fields:
        val = getattr(obj, attr, None)
        out[json_key] = _coerce(val)
    return out


def _coerce(val: Any) -> Any:
    """Convert a single value to a JSON-safe scalar."""
    if val is None:
        return None
    if isinstance(val, Decimal):
        return str(val)
    if isinstance(val, datetime):
        return _fmt_dt(val)
    if isinstance(val, date):
        return _fmt_date(val)
    if isinstance(val, bool):
        return val
    if isinstance(val, (int, float, str)):
        return val
    # Fallback: try str
    return str(val)


def _parse_dt(s: str | None) -> datetime | None:
    if not s:
        return None
    # Accept ISO dates "2026-01-01" or datetimes, with or without a timezone
    # offset (grpcstore sends RFC3339 "+07:00"). DB stores naive ICT wallclock,
    # so drop any tzinfo (the wallclock is already ICT).
    s = s.strip().replace(" ", "T")
    try:
        dt = datetime.fromisoformat(s)  # handles "+07:00", "Z", and offset-less
        return dt.replace(tzinfo=None) if dt.tzinfo is not None else dt
    except ValueError:
        pass
    for fmt in ("%Y-%m-%dT%H:%M:%S", "%Y-%m-%dT%H:%M", "%Y-%m-%d"):
        try:
            return datetime.strptime(s, fmt)
        except ValueError:
            continue
    raise ValueError(f"Cannot parse datetime: {s!r}")


def _page_result(items: list, total: int) -> dict:
    return {"items": items, "total": total}


# ---------------------------------------------------------------------------
# Field maps — (json_key, model_attr_name) — must match Go json tags exactly
# ---------------------------------------------------------------------------

_GOLD_PRICE_FIELDS = [
    ("id", "id"), ("source", "source"), ("product_type", "product_type"),
    ("trading_date", "trading_date"), ("open_price", "open_price"),
    ("high_price", "high_price"), ("low_price", "low_price"),
    ("buy_price", "buy_price"), ("sell_price", "sell_price"),
    ("currency", "currency"), ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_GOLD_PREDICTION_FIELDS = [
    ("id", "id"), ("source", "source"), ("product_type", "product_type"),
    ("predicted_price", "predicted_price"), ("current_price", "current_price"),
    ("confidence", "confidence"), ("algorithm_name", "algorithm_name"),
    ("prediction_date", "prediction_date"), ("target_date", "target_date"),
    ("actual_price", "actual_price"), ("accuracy", "accuracy"),
    ("direction_correct", "direction_correct"), ("status", "status"),
    ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_GOLD_INTRADAY_FIELDS = [
    ("id", "id"), ("source", "source"), ("product_type", "product_type"),
    ("timestamp", "timestamp"), ("open_price", "open_price"),
    ("high_price", "high_price"), ("low_price", "low_price"),
    ("buy_price", "buy_price"), ("sell_price", "sell_price"),
    ("currency", "currency"), ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_NASDAQ_PRICE_FIELDS = [
    ("id", "id"), ("symbol", "symbol"), ("company_name", "company_name"),
    ("open_price", "open_price"), ("high_price", "high_price"),
    ("low_price", "low_price"), ("close_price", "close_price"),
    ("volume", "volume"), ("trading_date", "trading_date"),
    ("currency", "currency"), ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_NASDAQ_PREDICTION_FIELDS = [
    ("id", "id"), ("symbol", "symbol"), ("algorithm_name", "algorithm_name"),
    ("predicted_price", "predicted_price"), ("current_price", "current_price"),
    ("confidence", "confidence"), ("prediction_date", "prediction_date"),
    ("target_date", "target_date"), ("actual_price", "actual_price"),
    ("accuracy", "accuracy"), ("direction_correct", "direction_correct"),
    ("status", "status"), ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_NASDAQ_INTRADAY_FIELDS = [
    ("id", "id"), ("symbol", "symbol"), ("timestamp", "timestamp"),
    ("open_price", "open_price"), ("high_price", "high_price"),
    ("low_price", "low_price"), ("close_price", "close_price"),
    ("volume", "volume"), ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_CRYPTO_PRICE_FIELDS = [
    ("id", "id"), ("coin_id", "coin_id"), ("symbol", "symbol"),
    ("open_price", "open_price"), ("high_price", "high_price"),
    ("low_price", "low_price"), ("close_price", "close_price"),
    ("market_cap", "market_cap"), ("volume_24h", "volume24h"),
    ("trading_date", "trading_date"), ("currency", "currency"),
    ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_CRYPTO_PREDICTION_FIELDS = [
    ("id", "id"), ("coin_id", "coin_id"), ("symbol", "symbol"),
    ("algorithm_name", "algorithm_name"),
    ("predicted_price", "predicted_price"), ("current_price", "current_price"),
    ("confidence", "confidence"), ("prediction_date", "prediction_date"),
    ("target_date", "target_date"), ("actual_price", "actual_price"),
    ("accuracy", "accuracy"), ("direction_correct", "direction_correct"),
    ("status", "status"), ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_CRYPTO_INTRADAY_FIELDS = [
    ("id", "id"), ("coin_id", "coin_id"), ("timestamp", "timestamp"),
    ("open_price", "open_price"), ("high_price", "high_price"),
    ("low_price", "low_price"), ("price", "price"),
    ("market_cap", "market_cap"), ("volume", "volume"),
    ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_SP500_PRICE_FIELDS = [
    ("id", "id"), ("symbol", "symbol"), ("company_name", "company_name"),
    ("open_price", "open_price"), ("high_price", "high_price"),
    ("low_price", "low_price"), ("close_price", "close_price"),
    ("volume", "volume"), ("trading_date", "trading_date"),
    ("currency", "currency"), ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_SP500_PREDICTION_FIELDS = [
    ("id", "id"), ("symbol", "symbol"), ("algorithm_name", "algorithm_name"),
    ("predicted_price", "predicted_price"), ("current_price", "current_price"),
    ("confidence", "confidence"), ("prediction_date", "prediction_date"),
    ("target_date", "target_date"), ("actual_price", "actual_price"),
    ("accuracy", "accuracy"), ("direction_correct", "direction_correct"),
    ("status", "status"), ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_SP500_INTRADAY_FIELDS = [
    ("id", "id"), ("symbol", "symbol"), ("timestamp", "timestamp"),
    ("open_price", "open_price"), ("high_price", "high_price"),
    ("low_price", "low_price"), ("close_price", "close_price"),
    ("volume", "volume"), ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_SIM_BOT_FIELDS = [
    ("id", "id"), ("market", "market"), ("algorithm", "algorithm"),
    ("symbol", "symbol"), ("display_name", "display_name"),
    ("initial_capital", "initial_capital"), ("currency", "currency"),
    ("buy_threshold", "buy_threshold"), ("sell_threshold", "sell_threshold"),
    ("min_confidence", "min_confidence"), ("stop_loss", "stop_loss"),
    ("take_profit", "take_profit"), ("max_position_pct", "max_position_pct"),
    ("max_positions", "max_positions"), ("is_active", "is_active"),
    ("trailing_stop", "trailing_stop"),
    ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_SIM_SESSION_FIELDS = [
    ("id", "id"), ("bot_id", "bot_id"), ("start_date", "start_date"),
    ("end_date", "end_date"), ("status", "status"), ("mode", "mode"),
    ("created_at", "created_at"),
    ("total_trades", "total_trades"), ("wins", "wins"), ("losses", "losses"),
    ("breakeven", "breakeven"), ("total_pnl", "total_pnl"),
    ("total_return_pct", "total_return_pct"), ("win_rate", "win_rate"),
    ("profit_factor", "profit_factor"), ("max_drawdown_pct", "max_drawdown_pct"),
    ("kpi_updated_at", "kpi_updated_at"),
]

_SIM_TRADE_FIELDS = [
    ("id", "id"), ("session_id", "session_id"), ("bot_id", "bot_id"),
    ("symbol", "symbol"), ("action", "action"), ("quantity", "quantity"),
    ("price", "price"), ("trade_value", "trade_value"),
    ("signal_strength", "signal_strength"), ("confidence", "confidence"),
    ("trade_date", "trade_date"), ("trade_at", "trade_at"),
    ("close_reason", "close_reason"), ("entry_trade_id", "entry_trade_id"),
    ("pnl", "pnl"), ("pnl_pct", "pnl_pct"), ("created_at", "created_at"),
]

_SIM_SNAPSHOT_FIELDS = [
    ("id", "id"), ("session_id", "session_id"), ("bot_id", "bot_id"),
    ("snapshot_date", "snapshot_date"), ("snapshot_at", "snapshot_at"),
    ("cash_balance", "cash_balance"), ("positions_value", "positions_value"),
    ("total_value", "total_value"), ("total_return_pct", "total_return_pct"),
    ("open_positions", "open_positions"),
]

_TRAINING_LOG_FIELDS = [
    ("id", "id"), ("session_id", "session_id"), ("algorithm_name", "algorithm_name"),
    ("market_key", "market_key"), ("total_stocks", "total_stocks"),
    ("success_count", "success_count"), ("error_count", "error_count"),
    ("accuracy", "accuracy"), ("duration_ms", "duration_ms"),
    ("error_details", "error_details"), ("started_at", "started_at"),
    ("completed_at", "completed_at"), ("created_at", "created_at"),
    ("updated_at", "updated_at"),
]

_CRON_SCHEDULE_FIELDS = [
    # Go CronSchedule has no json tags on ID/JobKey but the field names are used
    # as lowercase by the API, which reads from the struct directly.
    # Go field: ID uint, JobKey string, JobName string, CronExpression string,
    #           Enabled bool, UpdatedAt time.Time
    ("id", "id"), ("job_key", "job_key"), ("job_name", "job_name"),
    ("cron_expression", "cron_expression"), ("enabled", "enabled"),
    ("updated_at", "updated_at"),
]

_SYNC_LOG_FIELDS = [
    ("id", "id"), ("sync_date", "sync_date"), ("success_count", "success_count"),
    ("error_count", "error_count"), ("duration_ms", "duration_ms"),
    ("source", "source"), ("error_message", "error_message"),
    ("created_at", "created_at"), ("updated_at", "updated_at"),
]

_PIPELINE_REPORT_FIELDS = [
    ("id", "id"), ("pipeline_key", "pipeline_key"), ("market", "market"),
    ("status", "status"), ("started_at", "started_at"),
    ("finished_at", "finished_at"), ("duration_ms", "duration_ms"),
    ("crawled_count", "crawled_count"), ("predictions_count", "predictions_count"),
    ("trained", "trained"), ("steps", "steps"), ("error", "error"),
    ("created_at", "created_at"),
]


def _ser_gold_price(obj):
    return _serialize_model(obj, _GOLD_PRICE_FIELDS)

def _ser_gold_pred(obj):
    return _serialize_model(obj, _GOLD_PREDICTION_FIELDS)

def _ser_gold_intraday(obj):
    return _serialize_model(obj, _GOLD_INTRADAY_FIELDS)

def _ser_nasdaq_price(obj):
    return _serialize_model(obj, _NASDAQ_PRICE_FIELDS)

def _ser_nasdaq_pred(obj):
    return _serialize_model(obj, _NASDAQ_PREDICTION_FIELDS)

def _ser_nasdaq_intraday(obj):
    return _serialize_model(obj, _NASDAQ_INTRADAY_FIELDS)

def _ser_crypto_price(obj):
    return _serialize_model(obj, _CRYPTO_PRICE_FIELDS)

def _ser_crypto_pred(obj):
    return _serialize_model(obj, _CRYPTO_PREDICTION_FIELDS)

def _ser_crypto_intraday(obj):
    return _serialize_model(obj, _CRYPTO_INTRADAY_FIELDS)

def _ser_sp500_price(obj):
    return _serialize_model(obj, _SP500_PRICE_FIELDS)

def _ser_sp500_pred(obj):
    return _serialize_model(obj, _SP500_PREDICTION_FIELDS)

def _ser_sp500_intraday(obj):
    return _serialize_model(obj, _SP500_INTRADAY_FIELDS)

def _ser_sim_bot(obj):
    return _serialize_model(obj, _SIM_BOT_FIELDS)

def _ser_sim_session(obj):
    return _serialize_model(obj, _SIM_SESSION_FIELDS)

def _ser_sim_trade(obj):
    return _serialize_model(obj, _SIM_TRADE_FIELDS)

def _ser_sim_snapshot(obj):
    d = _serialize_model(obj, _SIM_SNAPSHOT_FIELDS)
    # snapshot_date may be a date object (Date column) — keep as "YYYY-MM-DD"
    sd = getattr(obj, "snapshot_date", None)
    if sd is not None and isinstance(sd, (date, datetime)):
        d["snapshot_date"] = _fmt_date(sd)
    return d

def _ser_training_log(obj):
    return _serialize_model(obj, _TRAINING_LOG_FIELDS)

def _ser_cron_schedule(obj):
    return _serialize_model(obj, _CRON_SCHEDULE_FIELDS)

def _ser_sync_log(obj):
    return _serialize_model(obj, _SYNC_LOG_FIELDS)

def _ser_pipeline_report(obj):
    d = _serialize_model(obj, _PIPELINE_REPORT_FIELDS)
    # steps is already a dict/list (JSONB) — keep as-is for JSON embedding
    raw = getattr(obj, "steps", None)
    if raw is None:
        d["steps"] = None
    else:
        # SQLAlchemy returns JSONB as a Python dict/list already
        d["steps"] = raw
    return d


# ---------------------------------------------------------------------------
# Prediction page sort whitelist
# ---------------------------------------------------------------------------

_PRED_SORT_WHITELIST = {"prediction_date", "target_date", "accuracy", "confidence"}
_TRAINING_SORT_WHITELIST = {"started_at", "accuracy", "duration_ms"}


def _safe_sort(sort_by: str, whitelist: set, default: str) -> str:
    return sort_by if sort_by in whitelist else default


def _safe_dir(sort_dir: str) -> str:
    return "ASC" if (sort_dir or "").lower() == "asc" else "DESC"


# ---------------------------------------------------------------------------
# Handler implementations
# ---------------------------------------------------------------------------

def _h_GetGoldPricesByDateRange(p: dict) -> Any:
    source = p.get("source", "")
    product_type = p.get("product_type", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(GoldPrice).filter(
            GoldPrice.trading_date.between(from_dt, to_dt),
            GoldPrice.deleted_at.is_(None),
        ).order_by(GoldPrice.trading_date.desc())
        if source:
            q = q.filter(GoldPrice.source == source)
        if product_type:
            q = q.filter(GoldPrice.product_type == product_type)
        return [_ser_gold_price(r) for r in q.all()]


def _h_GetLatestGoldPrices(_p: dict) -> Any:
    with session_scope() as s:
        sub = (
            s.query(
                GoldPrice.source,
                GoldPrice.product_type,
                func.max(GoldPrice.trading_date).label("max_date"),
            )
            .filter(GoldPrice.deleted_at.is_(None))
            .group_by(GoldPrice.source, GoldPrice.product_type)
            .subquery()
        )
        rows = (
            s.query(GoldPrice)
            .join(
                sub,
                (GoldPrice.source == sub.c.source)
                & (GoldPrice.product_type == sub.c.product_type)
                & (GoldPrice.trading_date == sub.c.max_date),
            )
            .filter(GoldPrice.deleted_at.is_(None))
            .all()
        )
        return [_ser_gold_price(r) for r in rows]


def _h_GetGoldIntradayByRange(p: dict) -> Any:
    source = p.get("source", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(GoldIntradayPrice).filter(
            GoldIntradayPrice.timestamp.between(from_dt, to_dt),
        ).order_by(GoldIntradayPrice.timestamp.desc())
        if source:
            q = q.filter(GoldIntradayPrice.source == source)
        return [_ser_gold_intraday(r) for r in q.all()]


def _h_GetNasdaqPricesByDateRange(p: dict) -> Any:
    symbol = p.get("symbol", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(NasdaqPrice).filter(
            NasdaqPrice.symbol == symbol,
            NasdaqPrice.trading_date.between(from_dt, to_dt),
            NasdaqPrice.deleted_at.is_(None),
        ).order_by(NasdaqPrice.trading_date.desc())
        return [_ser_nasdaq_price(r) for r in q.all()]


def _h_GetLatestNasdaqPrice(p: dict) -> Any:
    symbol = p.get("symbol", "")
    with session_scope() as s:
        r = (
            s.query(NasdaqPrice)
            .filter(NasdaqPrice.symbol == symbol, NasdaqPrice.deleted_at.is_(None))
            .order_by(NasdaqPrice.trading_date.desc())
            .first()
        )
        return _ser_nasdaq_price(r) if r else None


def _h_GetNasdaqSymbols(_p: dict) -> Any:
    with session_scope() as s:
        rows = s.query(distinct(NasdaqPrice.symbol)).filter(
            NasdaqPrice.deleted_at.is_(None)
        ).all()
        return [r[0] for r in rows]


def _h_GetNasdaqIntradayByRange(p: dict) -> Any:
    symbol = p.get("symbol", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(NasdaqIntradayPrice).filter(
            NasdaqIntradayPrice.symbol == symbol,
            NasdaqIntradayPrice.timestamp.between(from_dt, to_dt),
        ).order_by(NasdaqIntradayPrice.timestamp.desc())
        return [_ser_nasdaq_intraday(r) for r in q.all()]


def _h_GetCryptoPricesByDateRange(p: dict) -> Any:
    coin_id = p.get("coin_id", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(CryptoPrice).filter(
            CryptoPrice.coin_id == coin_id,
            CryptoPrice.trading_date.between(from_dt, to_dt),
            CryptoPrice.deleted_at.is_(None),
        ).order_by(CryptoPrice.trading_date.desc())
        return [_ser_crypto_price(r) for r in q.all()]


def _h_GetLatestCryptoPrice(p: dict) -> Any:
    coin_id = p.get("coin_id", "")
    with session_scope() as s:
        r = (
            s.query(CryptoPrice)
            .filter(CryptoPrice.coin_id == coin_id, CryptoPrice.deleted_at.is_(None))
            .order_by(CryptoPrice.trading_date.desc())
            .first()
        )
        return _ser_crypto_price(r) if r else None


def _h_GetCryptoCoins(_p: dict) -> Any:
    with session_scope() as s:
        sub = (
            s.query(
                CryptoPrice.coin_id,
                func.max(CryptoPrice.trading_date).label("max_date"),
            )
            .filter(CryptoPrice.deleted_at.is_(None))
            .group_by(CryptoPrice.coin_id)
            .subquery()
        )
        rows = (
            s.query(CryptoPrice)
            .join(
                sub,
                (CryptoPrice.coin_id == sub.c.coin_id)
                & (CryptoPrice.trading_date == sub.c.max_date),
            )
            .filter(CryptoPrice.deleted_at.is_(None))
            .all()
        )
        return [_ser_crypto_price(r) for r in rows]


def _h_GetCryptoIntradayByRange(p: dict) -> Any:
    coin_id = p.get("coin_id", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(CryptoIntradayPrice).filter(
            CryptoIntradayPrice.coin_id == coin_id,
            CryptoIntradayPrice.timestamp.between(from_dt, to_dt),
        ).order_by(CryptoIntradayPrice.timestamp.desc())
        return [_ser_crypto_intraday(r) for r in q.all()]


def _h_GetSP500PricesByDateRange(p: dict) -> Any:
    symbol = p.get("symbol", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(SP500Price).filter(
            SP500Price.symbol == symbol,
            SP500Price.trading_date.between(from_dt, to_dt),
            SP500Price.deleted_at.is_(None),
        ).order_by(SP500Price.trading_date.desc())
        return [_ser_sp500_price(r) for r in q.all()]


def _h_GetLatestSP500Price(p: dict) -> Any:
    symbol = p.get("symbol", "")
    with session_scope() as s:
        r = (
            s.query(SP500Price)
            .filter(SP500Price.symbol == symbol, SP500Price.deleted_at.is_(None))
            .order_by(SP500Price.trading_date.desc())
            .first()
        )
        return _ser_sp500_price(r) if r else None


def _h_GetSP500Symbols(_p: dict) -> Any:
    with session_scope() as s:
        rows = s.query(distinct(SP500Price.symbol)).filter(
            SP500Price.deleted_at.is_(None)
        ).all()
        return [r[0] for r in rows]


def _h_GetSP500IntradayByRange(p: dict) -> Any:
    symbol = p.get("symbol", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(SP500IntradayPrice).filter(
            SP500IntradayPrice.symbol == symbol,
            SP500IntradayPrice.timestamp.between(from_dt, to_dt),
        ).order_by(SP500IntradayPrice.timestamp.desc())
        return [_ser_sp500_intraday(r) for r in q.all()]


# ---------------------------------------------------------------------------
# Prediction handlers
# ---------------------------------------------------------------------------

def _h_GetGoldPredictions(p: dict) -> Any:
    source = p.get("source", "")
    product_type = p.get("product_type", "")
    algorithm = p.get("algorithm", "")
    limit = int(p.get("limit", 0))
    with session_scope() as s:
        q = s.query(GoldPrediction).filter(
            GoldPrediction.deleted_at.is_(None)
        ).order_by(GoldPrediction.prediction_date.desc())
        if source:
            q = q.filter(GoldPrediction.source == source)
        if product_type:
            q = q.filter(GoldPrediction.product_type == product_type)
        if algorithm:
            q = q.filter(GoldPrediction.algorithm_name == algorithm)
        if limit > 0:
            q = q.limit(limit)
        return [_ser_gold_pred(r) for r in q.all()]


def _h_GetLatestGoldPredictions(_p: dict) -> Any:
    with session_scope() as s:
        sub = (
            s.query(
                GoldPrediction.source,
                GoldPrediction.product_type,
                GoldPrediction.algorithm_name,
                func.max(GoldPrediction.prediction_date).label("max_date"),
            )
            .filter(GoldPrediction.deleted_at.is_(None))
            .group_by(GoldPrediction.source, GoldPrediction.product_type, GoldPrediction.algorithm_name)
            .subquery()
        )
        rows = (
            s.query(GoldPrediction)
            .join(
                sub,
                (GoldPrediction.source == sub.c.source)
                & (GoldPrediction.product_type == sub.c.product_type)
                & (GoldPrediction.algorithm_name == sub.c.algorithm_name)
                & (GoldPrediction.prediction_date == sub.c.max_date),
            )
            .filter(GoldPrediction.deleted_at.is_(None))
            .order_by(GoldPrediction.prediction_date.desc())
            .all()
        )
        return [_ser_gold_pred(r) for r in rows]


def _h_GetLatestConfirmedGoldPredictions(_p: dict) -> Any:
    """MAX(id) per (source, product_type, algorithm_name) — most recent row regardless of status."""
    with session_scope() as s:
        sub = (
            s.query(func.max(GoldPrediction.id).label("max_id"))
            .filter(GoldPrediction.deleted_at.is_(None))
            .group_by(GoldPrediction.source, GoldPrediction.product_type, GoldPrediction.algorithm_name)
            .subquery()
        )
        rows = (
            s.query(GoldPrediction)
            .join(sub, GoldPrediction.id == sub.c.max_id)
            .filter(GoldPrediction.deleted_at.is_(None))
            .order_by(GoldPrediction.prediction_date.desc())
            .all()
        )
        return [_ser_gold_pred(r) for r in rows]


def _h_GetGoldPredictionsByDateRange(p: dict) -> Any:
    source = p.get("source", "")
    product_type = p.get("product_type", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(GoldPrediction).filter(
            GoldPrediction.prediction_date.between(from_dt, to_dt),
            GoldPrediction.deleted_at.is_(None),
        ).order_by(GoldPrediction.prediction_date.asc())
        if source:
            q = q.filter(GoldPrediction.source == source)
        if product_type:
            q = q.filter(GoldPrediction.product_type == product_type)
        return [_ser_gold_pred(r) for r in q.all()]


def _h_GetGoldPredictionsPage(p: dict) -> Any:
    page = max(1, int(p.get("page", 1)))
    limit = max(1, int(p.get("limit", 50)))
    search = p.get("search", "")
    algorithm = p.get("algorithm", "")
    status = p.get("status", "")
    sort_by = _safe_sort(p.get("sort_by", ""), _PRED_SORT_WHITELIST, "prediction_date")
    sort_dir = _safe_dir(p.get("sort_dir", ""))
    with session_scope() as s:
        q = s.query(GoldPrediction).filter(GoldPrediction.deleted_at.is_(None))
        if search:
            like = f"%{search}%"
            q = q.filter(
                (GoldPrediction.source.ilike(like)) | (GoldPrediction.product_type.ilike(like))
            )
        if algorithm:
            q = q.filter(GoldPrediction.algorithm_name == algorithm)
        if status == "confirmed":
            q = q.filter(GoldPrediction.actual_price.isnot(None))
        elif status == "pending":
            q = q.filter(GoldPrediction.actual_price.is_(None))
        total = q.count()
        col = getattr(GoldPrediction, sort_by)
        order_col = col.asc() if sort_dir == "ASC" else col.desc()
        rows = q.order_by(order_col).offset((page - 1) * limit).limit(limit).all()
        return _page_result([_ser_gold_pred(r) for r in rows], total)


# ---- NASDAQ Predictions ----

def _h_GetNasdaqPredictions(p: dict) -> Any:
    symbol = p.get("symbol", "")
    algorithm = p.get("algorithm", "")
    limit = int(p.get("limit", 0))
    with session_scope() as s:
        q = s.query(NasdaqPrediction).filter(
            NasdaqPrediction.deleted_at.is_(None)
        ).order_by(NasdaqPrediction.prediction_date.desc())
        if symbol:
            q = q.filter(NasdaqPrediction.symbol == symbol)
        if algorithm:
            q = q.filter(NasdaqPrediction.algorithm_name == algorithm)
        if limit > 0:
            q = q.limit(limit)
        return [_ser_nasdaq_pred(r) for r in q.all()]


def _h_GetLatestNasdaqPredictions(_p: dict) -> Any:
    with session_scope() as s:
        sub = (
            s.query(
                NasdaqPrediction.symbol,
                NasdaqPrediction.algorithm_name,
                func.max(NasdaqPrediction.prediction_date).label("max_date"),
            )
            .filter(NasdaqPrediction.deleted_at.is_(None))
            .group_by(NasdaqPrediction.symbol, NasdaqPrediction.algorithm_name)
            .subquery()
        )
        rows = (
            s.query(NasdaqPrediction)
            .join(
                sub,
                (NasdaqPrediction.symbol == sub.c.symbol)
                & (NasdaqPrediction.algorithm_name == sub.c.algorithm_name)
                & (NasdaqPrediction.prediction_date == sub.c.max_date),
            )
            .filter(NasdaqPrediction.deleted_at.is_(None))
            .order_by(NasdaqPrediction.prediction_date.desc())
            .all()
        )
        return [_ser_nasdaq_pred(r) for r in rows]


def _h_GetLatestConfirmedNasdaqPredictions(_p: dict) -> Any:
    with session_scope() as s:
        sub = (
            s.query(func.max(NasdaqPrediction.id).label("max_id"))
            .filter(NasdaqPrediction.deleted_at.is_(None))
            .group_by(NasdaqPrediction.symbol, NasdaqPrediction.algorithm_name)
            .subquery()
        )
        rows = (
            s.query(NasdaqPrediction)
            .join(sub, NasdaqPrediction.id == sub.c.max_id)
            .filter(NasdaqPrediction.deleted_at.is_(None))
            .order_by(NasdaqPrediction.prediction_date.desc())
            .all()
        )
        return [_ser_nasdaq_pred(r) for r in rows]


def _h_GetNasdaqPredictionsByDateRange(p: dict) -> Any:
    symbol = p.get("symbol", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(NasdaqPrediction).filter(
            NasdaqPrediction.prediction_date.between(from_dt, to_dt),
            NasdaqPrediction.deleted_at.is_(None),
        ).order_by(NasdaqPrediction.prediction_date.asc())
        if symbol:
            q = q.filter(NasdaqPrediction.symbol == symbol)
        return [_ser_nasdaq_pred(r) for r in q.all()]


def _h_GetNasdaqPredictionsPage(p: dict) -> Any:
    page = max(1, int(p.get("page", 1)))
    limit = max(1, int(p.get("limit", 50)))
    search = p.get("search", "")
    algorithm = p.get("algorithm", "")
    status = p.get("status", "")
    sort_by = _safe_sort(p.get("sort_by", ""), _PRED_SORT_WHITELIST, "prediction_date")
    sort_dir = _safe_dir(p.get("sort_dir", ""))
    with session_scope() as s:
        q = s.query(NasdaqPrediction).filter(NasdaqPrediction.deleted_at.is_(None))
        if search:
            q = q.filter(NasdaqPrediction.symbol.ilike(f"%{search}%"))
        if algorithm:
            q = q.filter(NasdaqPrediction.algorithm_name == algorithm)
        if status == "confirmed":
            q = q.filter(NasdaqPrediction.actual_price.isnot(None))
        elif status == "pending":
            q = q.filter(NasdaqPrediction.actual_price.is_(None))
        total = q.count()
        col = getattr(NasdaqPrediction, sort_by)
        order_col = col.asc() if sort_dir == "ASC" else col.desc()
        rows = q.order_by(order_col).offset((page - 1) * limit).limit(limit).all()
        return _page_result([_ser_nasdaq_pred(r) for r in rows], total)


# ---- Crypto Predictions ----

def _h_GetCryptoPredictions(p: dict) -> Any:
    coin_id = p.get("coin_id", "")
    algorithm = p.get("algorithm", "")
    limit = int(p.get("limit", 0))
    with session_scope() as s:
        q = s.query(CryptoPrediction).filter(
            CryptoPrediction.deleted_at.is_(None)
        ).order_by(CryptoPrediction.prediction_date.desc())
        if coin_id:
            q = q.filter(CryptoPrediction.coin_id == coin_id)
        if algorithm:
            q = q.filter(CryptoPrediction.algorithm_name == algorithm)
        if limit > 0:
            q = q.limit(limit)
        return [_ser_crypto_pred(r) for r in q.all()]


def _h_GetLatestCryptoPredictions(_p: dict) -> Any:
    with session_scope() as s:
        sub = (
            s.query(func.max(CryptoPrediction.id).label("max_id"))
            .filter(CryptoPrediction.deleted_at.is_(None))
            .group_by(CryptoPrediction.coin_id, CryptoPrediction.algorithm_name)
            .subquery()
        )
        rows = (
            s.query(CryptoPrediction)
            .join(sub, CryptoPrediction.id == sub.c.max_id)
            .filter(CryptoPrediction.deleted_at.is_(None))
            .order_by(CryptoPrediction.prediction_date.desc())
            .all()
        )
        return [_ser_crypto_pred(r) for r in rows]


def _h_GetLatestConfirmedCryptoPredictions(_p: dict) -> Any:
    # Go implementation uses MAX(id) — same as GetLatestCryptoPredictions
    return _h_GetLatestCryptoPredictions(_p)


def _h_GetCryptoPredictionsByDateRange(p: dict) -> Any:
    coin_id = p.get("coin_id", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(CryptoPrediction).filter(
            CryptoPrediction.prediction_date.between(from_dt, to_dt),
            CryptoPrediction.deleted_at.is_(None),
        ).order_by(CryptoPrediction.prediction_date.asc())
        if coin_id:
            q = q.filter(CryptoPrediction.coin_id == coin_id)
        return [_ser_crypto_pred(r) for r in q.all()]


def _h_GetCryptoPredictionsPage(p: dict) -> Any:
    page = max(1, int(p.get("page", 1)))
    limit = max(1, int(p.get("limit", 50)))
    search = p.get("search", "")
    algorithm = p.get("algorithm", "")
    status = p.get("status", "")
    sort_by = _safe_sort(p.get("sort_by", ""), _PRED_SORT_WHITELIST, "prediction_date")
    sort_dir = _safe_dir(p.get("sort_dir", ""))
    with session_scope() as s:
        q = s.query(CryptoPrediction).filter(CryptoPrediction.deleted_at.is_(None))
        if search:
            like = f"%{search}%"
            q = q.filter(
                (CryptoPrediction.coin_id.ilike(like)) | (CryptoPrediction.symbol.ilike(like))
            )
        if algorithm:
            q = q.filter(CryptoPrediction.algorithm_name == algorithm)
        if status == "confirmed":
            q = q.filter(CryptoPrediction.actual_price.isnot(None))
        elif status == "pending":
            q = q.filter(CryptoPrediction.actual_price.is_(None))
        total = q.count()
        col = getattr(CryptoPrediction, sort_by)
        order_col = col.asc() if sort_dir == "ASC" else col.desc()
        rows = q.order_by(order_col).offset((page - 1) * limit).limit(limit).all()
        return _page_result([_ser_crypto_pred(r) for r in rows], total)


# ---- SP500 Predictions ----

def _h_GetSP500Predictions(p: dict) -> Any:
    symbol = p.get("symbol", "")
    algorithm = p.get("algorithm", "")
    limit = int(p.get("limit", 0))
    with session_scope() as s:
        q = s.query(SP500Prediction).filter(
            SP500Prediction.deleted_at.is_(None)
        ).order_by(SP500Prediction.prediction_date.desc())
        if symbol:
            q = q.filter(SP500Prediction.symbol == symbol)
        if algorithm:
            q = q.filter(SP500Prediction.algorithm_name == algorithm)
        if limit > 0:
            q = q.limit(limit)
        return [_ser_sp500_pred(r) for r in q.all()]


def _h_GetLatestSP500Predictions(_p: dict) -> Any:
    with session_scope() as s:
        sub = (
            s.query(
                SP500Prediction.symbol,
                SP500Prediction.algorithm_name,
                func.max(SP500Prediction.prediction_date).label("max_date"),
            )
            .filter(SP500Prediction.deleted_at.is_(None))
            .group_by(SP500Prediction.symbol, SP500Prediction.algorithm_name)
            .subquery()
        )
        rows = (
            s.query(SP500Prediction)
            .join(
                sub,
                (SP500Prediction.symbol == sub.c.symbol)
                & (SP500Prediction.algorithm_name == sub.c.algorithm_name)
                & (SP500Prediction.prediction_date == sub.c.max_date),
            )
            .filter(SP500Prediction.deleted_at.is_(None))
            .order_by(SP500Prediction.prediction_date.desc())
            .all()
        )
        return [_ser_sp500_pred(r) for r in rows]


def _h_GetLatestConfirmedSP500Predictions(_p: dict) -> Any:
    with session_scope() as s:
        sub = (
            s.query(func.max(SP500Prediction.id).label("max_id"))
            .filter(SP500Prediction.deleted_at.is_(None))
            .group_by(SP500Prediction.symbol, SP500Prediction.algorithm_name)
            .subquery()
        )
        rows = (
            s.query(SP500Prediction)
            .join(sub, SP500Prediction.id == sub.c.max_id)
            .filter(SP500Prediction.deleted_at.is_(None))
            .order_by(SP500Prediction.prediction_date.desc())
            .all()
        )
        return [_ser_sp500_pred(r) for r in rows]


def _h_GetSP500PredictionsByDateRange(p: dict) -> Any:
    symbol = p.get("symbol", "")
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        q = s.query(SP500Prediction).filter(
            SP500Prediction.prediction_date.between(from_dt, to_dt),
            SP500Prediction.deleted_at.is_(None),
        ).order_by(SP500Prediction.prediction_date.asc())
        if symbol:
            q = q.filter(SP500Prediction.symbol == symbol)
        return [_ser_sp500_pred(r) for r in q.all()]


def _h_GetSP500PredictionsPage(p: dict) -> Any:
    page = max(1, int(p.get("page", 1)))
    limit = max(1, int(p.get("limit", 50)))
    search = p.get("search", "")
    algorithm = p.get("algorithm", "")
    status = p.get("status", "")
    sort_by = _safe_sort(p.get("sort_by", ""), _PRED_SORT_WHITELIST, "prediction_date")
    sort_dir = _safe_dir(p.get("sort_dir", ""))
    with session_scope() as s:
        q = s.query(SP500Prediction).filter(SP500Prediction.deleted_at.is_(None))
        if search:
            q = q.filter(SP500Prediction.symbol.ilike(f"%{search}%"))
        if algorithm:
            q = q.filter(SP500Prediction.algorithm_name == algorithm)
        if status == "confirmed":
            q = q.filter(SP500Prediction.actual_price.isnot(None))
        elif status == "pending":
            q = q.filter(SP500Prediction.actual_price.is_(None))
        total = q.count()
        col = getattr(SP500Prediction, sort_by)
        order_col = col.asc() if sort_dir == "ASC" else col.desc()
        rows = q.order_by(order_col).offset((page - 1) * limit).limit(limit).all()
        return _page_result([_ser_sp500_pred(r) for r in rows], total)


# ---------------------------------------------------------------------------
# Simulation handlers
# ---------------------------------------------------------------------------

def _h_GetAllSimBots(_p: dict) -> Any:
    with session_scope() as s:
        rows = s.query(SimBot).all()
        return [_ser_sim_bot(r) for r in rows]


def _h_GetSimBotByID(p: dict) -> Any:
    bot_id = p.get("id", "")
    with session_scope() as s:
        r = s.query(SimBot).filter(SimBot.id == bot_id).first()
        return _ser_sim_bot(r) if r else None


def _h_GetSimBotsByMarketAlgo(p: dict) -> Any:
    market = p.get("market", "")
    algorithm = p.get("algorithm", "")
    with session_scope() as s:
        rows = s.query(SimBot).filter(
            SimBot.market == market,
            SimBot.algorithm == algorithm,
        ).order_by(SimBot.id.asc()).all()
        return [_ser_sim_bot(r) for r in rows]


def _h_GetLatestSimSession(p: dict) -> Any:
    bot_id = p.get("bot_id", "")
    with session_scope() as s:
        r = (
            s.query(SimSession)
            .filter(SimSession.bot_id == bot_id)
            .order_by(SimSession.created_at.desc())
            .first()
        )
        return _ser_sim_session(r) if r else None


def _h_GetBestSimSessionForChart(p: dict) -> Any:
    bot_id = p.get("bot_id", "")
    with session_scope() as s:
        row = s.execute(
            text("""
                SELECT s.* FROM sim_sessions s
                INNER JOIN (
                    SELECT session_id, COUNT(*) AS cnt
                    FROM sim_portfolio_snapshots
                    WHERE bot_id = :bot_id
                    GROUP BY session_id
                    ORDER BY cnt DESC
                    LIMIT 1
                ) best ON s.id = best.session_id
            """),
            {"bot_id": bot_id},
        ).fetchone()
        if row is None:
            return None
        # Map row to SimSession manually
        r = s.query(SimSession).filter(SimSession.id == row.id).first()
        return _ser_sim_session(r) if r else None


def _h_GetLatestLiveSimSession(p: dict) -> Any:
    bot_id = p.get("bot_id", "")
    with session_scope() as s:
        r = (
            s.query(SimSession)
            .filter(
                SimSession.bot_id == bot_id,
                SimSession.mode == "live",
                SimSession.status == "running",
            )
            .order_by(SimSession.id.desc())
            .first()
        )
        return _ser_sim_session(r) if r else None


def _h_GetSimTrades(p: dict) -> Any:
    session_id = int(p.get("session_id", 0))
    offset = int(p.get("offset", 0))
    limit = int(p.get("limit", 100))
    exclude_hold = bool(p.get("exclude_hold", False))
    with session_scope() as s:
        q = s.query(SimTrade).filter(SimTrade.session_id == session_id)
        if exclude_hold:
            q = q.filter(SimTrade.action != "HOLD")
        total = q.count()
        rows = q.order_by(SimTrade.trade_date.asc()).offset(offset).limit(limit).all()
        return _page_result([_ser_sim_trade(r) for r in rows], total)


def _h_GetSimPortfolioSnapshots(p: dict) -> Any:
    session_id = int(p.get("session_id", 0))
    with session_scope() as s:
        rows = (
            s.query(SimPortfolioSnapshot)
            .filter(SimPortfolioSnapshot.session_id == session_id)
            .order_by(SimPortfolioSnapshot.snapshot_date.asc())
            .all()
        )
        return [_ser_sim_snapshot(r) for r in rows]


def _h_GetAllSessionsWithSnapCount(_p: dict) -> Any:
    with session_scope() as s:
        rows = s.execute(text("""
            SELECT s.id, s.bot_id, s.start_date, s.end_date, s.status, s.mode,
                   s.created_at, s.total_trades, s.wins, s.losses, s.breakeven,
                   s.total_pnl, s.total_return_pct, s.win_rate, s.profit_factor,
                   s.max_drawdown_pct, s.kpi_updated_at,
                   COALESCE(sc.cnt, 0) AS snap_count
            FROM sim_sessions s
            LEFT JOIN (
                SELECT session_id, COUNT(*) AS cnt
                FROM sim_portfolio_snapshots
                GROUP BY session_id
            ) sc ON s.id = sc.session_id
            ORDER BY s.bot_id ASC, s.id DESC
        """)).fetchall()
        result = []
        for r in rows:
            d = {
                "id": r.id,
                "bot_id": r.bot_id,
                "start_date": _fmt_dt(r.start_date),
                "end_date": _fmt_dt(r.end_date),
                "status": r.status,
                "mode": r.mode,
                "created_at": _fmt_dt(r.created_at),
                "total_trades": r.total_trades,
                "wins": r.wins,
                "losses": r.losses,
                "breakeven": r.breakeven,
                "total_pnl": str(r.total_pnl) if r.total_pnl is not None else "0",
                "total_return_pct": str(r.total_return_pct) if r.total_return_pct is not None else None,
                "win_rate": str(r.win_rate) if r.win_rate is not None else None,
                "profit_factor": str(r.profit_factor) if r.profit_factor is not None else None,
                "max_drawdown_pct": str(r.max_drawdown_pct) if r.max_drawdown_pct is not None else None,
                "kpi_updated_at": _fmt_dt(r.kpi_updated_at),
                "snap_count": r.snap_count,
            }
            result.append(d)
        return result


def _h_GetSessionTradeStatsBatch(p: dict) -> Any:
    session_ids = [int(x) for x in p.get("session_ids", [])]
    if not session_ids:
        return {}
    with session_scope() as s:
        rows = s.execute(
            text("""
                SELECT
                    session_id,
                    COUNT(CASE WHEN action = 'SELL' THEN 1 END)                                           AS total_trades,
                    COUNT(CASE WHEN action = 'SELL' AND COALESCE(pnl_pct, pnl) > 0 THEN 1 END)           AS wins,
                    COUNT(CASE WHEN action = 'SELL' AND COALESCE(pnl_pct, pnl) < 0 THEN 1 END)           AS losses,
                    COUNT(CASE WHEN action = 'SELL' AND COALESCE(pnl_pct, pnl) = 0 THEN 1 END)           AS breakeven,
                    COALESCE(SUM(CASE WHEN action = 'SELL' THEN COALESCE(pnl, 0) END), 0)                AS total_pnl,
                    COALESCE(SUM(CASE WHEN action = 'SELL' AND pnl > 0 THEN pnl ELSE 0 END), 0)          AS win_pnl,
                    COALESCE(SUM(CASE WHEN action = 'SELL' AND pnl < 0 THEN ABS(pnl) ELSE 0 END), 0)     AS loss_pnl
                FROM sim_trades
                WHERE session_id = ANY(:ids)
                GROUP BY session_id
            """),
            {"ids": session_ids},
        ).fetchall()
        result = {}
        for r in rows:
            result[str(r.session_id)] = {
                "TotalTrades": r.total_trades,
                "Wins": r.wins,
                "Losses": r.losses,
                "Breakeven": r.breakeven,
                "TotalPnl": float(r.total_pnl) if r.total_pnl is not None else 0.0,
                "WinPnl": float(r.win_pnl) if r.win_pnl is not None else 0.0,
                "LossPnl": float(r.loss_pnl) if r.loss_pnl is not None else 0.0,
            }
        return result


def _h_GetLastSnapshotsBatch(p: dict) -> Any:
    session_ids = [int(x) for x in p.get("session_ids", [])]
    if not session_ids:
        return {}
    with session_scope() as s:
        rows = s.execute(
            text("""
                SELECT sp.*
                FROM sim_portfolio_snapshots sp
                INNER JOIN (
                    SELECT session_id, MAX(snapshot_date) AS last_date
                    FROM sim_portfolio_snapshots
                    WHERE session_id = ANY(:ids)
                    GROUP BY session_id
                ) latest ON sp.session_id = latest.session_id
                    AND sp.snapshot_date = latest.last_date
            """),
            {"ids": session_ids},
        ).fetchall()
        result = {}
        for r in rows:
            result[str(r.session_id)] = {
                "id": r.id,
                "session_id": r.session_id,
                "bot_id": r.bot_id,
                "snapshot_date": _fmt_date(r.snapshot_date),
                "snapshot_at": _fmt_dt(r.snapshot_at),
                "cash_balance": str(r.cash_balance) if r.cash_balance is not None else None,
                "positions_value": str(r.positions_value) if r.positions_value is not None else None,
                "total_value": str(r.total_value) if r.total_value is not None else None,
                "total_return_pct": str(r.total_return_pct) if r.total_return_pct is not None else None,
                "open_positions": r.open_positions,
            }
        return result


def _h_GetLeaderboardEntries(_p: dict) -> Any:
    with session_scope() as s:
        rows = s.execute(text("""
            SELECT
                s.id              AS session_id,
                b.id              AS bot_id,
                b.market,
                b.algorithm,
                b.display_name,
                b.initial_capital,
                b.currency,
                b.is_active,
                b.buy_threshold,
                b.sell_threshold,
                b.min_confidence,
                b.stop_loss,
                b.take_profit,
                s.start_date,
                s.end_date,
                s.mode,
                s.status,
                s.total_trades,
                s.wins,
                s.losses,
                s.breakeven,
                s.total_pnl,
                s.total_return_pct,
                s.win_rate,
                s.profit_factor,
                s.max_drawdown_pct,
                snap.total_value  AS current_value
            FROM sim_bots b
            LEFT JOIN LATERAL (
                SELECT ss.*
                FROM sim_sessions ss
                WHERE ss.bot_id = b.id
                ORDER BY
                    (ss.mode = 'live' AND ss.status = 'running') DESC,
                    (SELECT COUNT(*) FROM sim_portfolio_snapshots WHERE session_id = ss.id) DESC,
                    ss.id DESC
                LIMIT 1
            ) s ON true
            LEFT JOIN LATERAL (
                SELECT ps.total_value
                FROM sim_portfolio_snapshots ps
                WHERE ps.session_id = s.id
                ORDER BY ps.snapshot_date DESC
                LIMIT 1
            ) snap ON true
            ORDER BY s.total_return_pct DESC NULLS LAST
        """)).fetchall()
        result = []
        for r in rows:
            result.append({
                "session_id": r.session_id,
                "bot_id": r.bot_id,
                "market": r.market,
                "algorithm": r.algorithm,
                "display_name": r.display_name,
                "initial_capital": str(r.initial_capital) if r.initial_capital is not None else None,
                "currency": r.currency,
                "is_active": r.is_active,
                "buy_threshold": str(r.buy_threshold) if r.buy_threshold is not None else None,
                "sell_threshold": str(r.sell_threshold) if r.sell_threshold is not None else None,
                "min_confidence": str(r.min_confidence) if r.min_confidence is not None else None,
                "stop_loss": str(r.stop_loss) if r.stop_loss is not None else None,
                "take_profit": str(r.take_profit) if r.take_profit is not None else None,
                "start_date": _fmt_dt(r.start_date),
                "end_date": _fmt_dt(r.end_date),
                "mode": r.mode,
                "status": r.status,
                "total_trades": r.total_trades,
                "wins": r.wins,
                "losses": r.losses,
                "breakeven": r.breakeven,
                "total_pnl": str(r.total_pnl) if r.total_pnl is not None else "0",
                "total_return_pct": str(r.total_return_pct) if r.total_return_pct is not None else None,
                "win_rate": str(r.win_rate) if r.win_rate is not None else None,
                "profit_factor": str(r.profit_factor) if r.profit_factor is not None else None,
                "max_drawdown_pct": str(r.max_drawdown_pct) if r.max_drawdown_pct is not None else None,
                "current_value": str(r.current_value) if r.current_value is not None else None,
            })
        return result


def _h_UpdateSimBotConfig(p: dict) -> Any:
    bot_data = p.get("bot", {})
    bot_id = bot_data.get("id", "")
    with session_scope() as s:
        bot = s.query(SimBot).filter(SimBot.id == bot_id).first()
        if bot is None:
            raise ValueError(f"SimBot not found: {bot_id!r}")
        # Update config columns (same as Go Save() — full replace of mutable fields)
        for field in [
            "market", "algorithm", "symbol", "display_name", "initial_capital",
            "currency", "buy_threshold", "sell_threshold", "min_confidence",
            "stop_loss", "take_profit", "max_position_pct", "max_positions",
            "is_active", "trailing_stop",
        ]:
            if field in bot_data:
                val = bot_data[field]
                # Convert numeric strings to Decimal for Numeric columns
                if field in {"initial_capital", "buy_threshold", "sell_threshold",
                             "min_confidence", "stop_loss", "take_profit", "max_position_pct"}:
                    val = Decimal(str(val)) if val is not None else None
                setattr(bot, field, val)
        bot.updated_at = datetime.now()
    return {}


# ---------------------------------------------------------------------------
# Training handlers
# ---------------------------------------------------------------------------

def _h_GetTrainingLogByID(p: dict) -> Any:
    log_id = int(p.get("id", 0))
    with session_scope() as s:
        r = s.query(TrainingLog).filter(TrainingLog.id == log_id).first()
        return _ser_training_log(r) if r else None


def _h_GetTrainingLogsBySessionID(p: dict) -> Any:
    session_id = p.get("session_id", "")
    with session_scope() as s:
        rows = (
            s.query(TrainingLog)
            .filter(TrainingLog.session_id == session_id)
            .order_by(TrainingLog.algorithm_name.asc())
            .all()
        )
        return [_ser_training_log(r) for r in rows]


def _h_GetTrainingSessions(p: dict) -> Any:
    limit = int(p.get("limit", 20))
    offset = int(p.get("offset", 0))
    with session_scope() as s:
        # Count distinct sessions
        total_row = s.execute(
            text("SELECT COUNT(DISTINCT session_id) AS cnt FROM training_logs")
        ).fetchone()
        total = total_row.cnt if total_row else 0

        # Get distinct session IDs ordered by most recent
        sid_rows = s.execute(
            text("""
                SELECT session_id
                FROM training_logs
                GROUP BY session_id
                ORDER BY MAX(started_at) DESC
                LIMIT :lim OFFSET :off
            """),
            {"lim": limit, "off": offset},
        ).fetchall()
        session_ids = [r.session_id for r in sid_rows]
        if not session_ids:
            return _page_result([], total)

        rows = (
            s.query(TrainingLog)
            .filter(TrainingLog.session_id.in_(session_ids))
            .order_by(TrainingLog.started_at.desc(), TrainingLog.algorithm_name.asc())
            .all()
        )
        return _page_result([_ser_training_log(r) for r in rows], total)


def _h_GetLatestTrainingLogByAlgorithm(_p: dict) -> Any:
    with session_scope() as s:
        sub = (
            s.query(
                TrainingLog.algorithm_name,
                func.max(TrainingLog.id).label("max_id"),
            )
            .group_by(TrainingLog.algorithm_name)
            .subquery()
        )
        rows = (
            s.query(TrainingLog)
            .join(sub, TrainingLog.id == sub.c.max_id)
            .all()
        )
        return [_ser_training_log(r) for r in rows]


def _h_GetTrainingMetricsAggregate(_p: dict) -> Any:
    with session_scope() as s:
        total_sessions_row = s.execute(
            text("SELECT COUNT(DISTINCT session_id) AS cnt FROM training_logs")
        ).fetchone()
        total_sessions = total_sessions_row.cnt if total_sessions_row else 0

        raw = s.execute(text("""
            SELECT
                AVG(duration_ms)                                              AS avg_duration_ms,
                SUM(success_count)                                            AS total_success,
                SUM(error_count)                                              AS total_error,
                AVG(CASE WHEN total_stocks > 0
                         THEN CAST(success_count AS DECIMAL(10,4)) / total_stocks
                         ELSE 1 END)                                          AS avg_data_quality
            FROM training_logs
        """)).fetchone()

        avg_duration = float(raw.avg_duration_ms or 0)
        total_success = float(raw.total_success or 0)
        total_error = float(raw.total_error or 0)
        data_quality = float(raw.avg_data_quality or 0)
        total_algo = total_success + total_error
        success_rate = (total_success / total_algo) if total_algo > 0 else 0.0

        total_preds_row = s.execute(text("""
            SELECT (
                SELECT COUNT(*) FROM gold_predictions   WHERE deleted_at IS NULL
            ) + (
                SELECT COUNT(*) FROM nasdaq_predictions WHERE deleted_at IS NULL
            ) + (
                SELECT COUNT(*) FROM sp500_predictions  WHERE deleted_at IS NULL
            ) + (
                SELECT COUNT(*) FROM crypto_predictions WHERE deleted_at IS NULL
            ) AS total
        """)).fetchone()
        total_predictions = int(total_preds_row.total or 0)

        return {
            "TotalSessions": total_sessions,
            "TotalPredictions": total_predictions,
            "AvgDurationMs": avg_duration,
            "SuccessRate": success_rate,
            "DataQuality": data_quality,
        }


def _h_GetTrainingSessionsByMarket(p: dict) -> Any:
    market_key = p.get("market_key", "")
    page = max(1, int(p.get("page", 1)))
    limit = max(1, int(p.get("limit", 20)))
    algorithm = p.get("algorithm", "")
    sort_by = _safe_sort(p.get("sort_by", ""), _TRAINING_SORT_WHITELIST, "started_at")
    sort_dir = _safe_dir(p.get("sort_dir", ""))
    with session_scope() as s:
        q = s.query(TrainingLog).filter(TrainingLog.market_key == market_key)
        if algorithm:
            q = q.filter(TrainingLog.algorithm_name == algorithm)
        total = q.count()
        col = getattr(TrainingLog, sort_by)
        order_col = col.asc() if sort_dir == "ASC" else col.desc()
        rows = q.order_by(order_col).offset((page - 1) * limit).limit(limit).all()
        return _page_result([_ser_training_log(r) for r in rows], total)


# ---------------------------------------------------------------------------
# Schedules / sync / pipeline / monitoring / direction / session
# ---------------------------------------------------------------------------

def _h_GetAllCronSchedules(_p: dict) -> Any:
    with session_scope() as s:
        rows = s.query(CronSchedule).order_by(CronSchedule.job_key.asc()).all()
        return [_ser_cron_schedule(r) for r in rows]


def _h_GetCronScheduleByKey(p: dict) -> Any:
    job_key = p.get("job_key", "")
    with session_scope() as s:
        r = s.query(CronSchedule).filter(CronSchedule.job_key == job_key).first()
        return _ser_cron_schedule(r) if r else None


def _h_UpsertCronSchedule(p: dict) -> Any:
    sched = p.get("schedule", {})
    job_key = sched.get("job_key", "")
    with session_scope() as s:
        existing = s.query(CronSchedule).filter(CronSchedule.job_key == job_key).first()
        if existing:
            existing.job_name = sched.get("job_name", existing.job_name)
            existing.cron_expression = sched.get("cron_expression", existing.cron_expression)
            existing.enabled = sched.get("enabled", existing.enabled)
            existing.updated_at = datetime.now()
        else:
            new_sched = CronSchedule(
                job_key=job_key,
                job_name=sched.get("job_name", ""),
                cron_expression=sched.get("cron_expression", ""),
                enabled=sched.get("enabled", True),
                updated_at=datetime.now(),
            )
            s.add(new_sched)
    return {}


def _h_GetLatestSyncLogs(p: dict) -> Any:
    limit = int(p.get("limit", 20))
    with session_scope() as s:
        rows = (
            s.query(SyncLog)
            .filter(SyncLog.deleted_at.is_(None))
            .order_by(SyncLog.sync_date.desc())
            .limit(limit)
            .all()
        )
        return [_ser_sync_log(r) for r in rows]


def _h_GetDirectionAccuracy(p: dict) -> Any:
    market = (p.get("market", "") or "").upper()
    table_map = {
        "GOLD": "gold_predictions",
        "NASDAQ": "nasdaq_predictions",
        "CRYPTO": "crypto_predictions",
        "SP500": "sp500_predictions",
    }
    table = table_map.get(market)
    if not table:
        raise ValueError(f"Unknown market: {market!r}")
    with session_scope() as s:
        rows = s.execute(text(f"""
            SELECT
                algorithm_name AS algorithm,
                COUNT(*) AS total,
                COUNT(*) FILTER (WHERE direction_correct = true) AS correct
            FROM {table}
            WHERE direction_correct IS NOT NULL
              AND deleted_at IS NULL
            GROUP BY algorithm_name
            ORDER BY algorithm_name
        """)).fetchall()
        return [
            {"algorithm": r.algorithm, "total": r.total, "correct": r.correct}
            for r in rows
        ]


def _h_GetMarketCrawlStats(p: dict) -> Any:
    market = (p.get("market", "") or "").upper()
    intraday_map = {
        "GOLD":   "gold_intraday_prices",
        "NASDAQ": "nasdaq_intraday_prices",
        "CRYPTO": "crypto_intraday_prices",
        "SP500":  "sp500_intraday_prices",
    }
    daily_table_map = {
        "GOLD": "gold_prices",
        "NASDAQ": "nasdaq_prices",
        "CRYPTO": "crypto_prices",
        "SP500": "sp500_prices",
    }
    daily_table = daily_table_map.get(market)
    intraday_table = intraday_map.get(market)
    if not daily_table:
        raise ValueError(f"Unknown market: {market!r}")

    now = datetime.now()
    today_start = now.replace(hour=0, minute=0, second=0, microsecond=0)

    with session_scope() as s:
        # last daily crawl
        r = s.execute(text(
            f"SELECT created_at AS ts FROM {daily_table} WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT 1"
        )).fetchone()
        last_daily_at = _fmt_dt(r.ts) if r else None

        # daily count today
        r2 = s.execute(text(
            f"SELECT COUNT(*) AS cnt FROM {daily_table} WHERE deleted_at IS NULL AND created_at >= :ts"
        ), {"ts": today_start}).fetchone()
        daily_today = r2.cnt if r2 else 0

        # last intraday crawl
        r3 = s.execute(text(
            f"SELECT created_at AS ts FROM {intraday_table} ORDER BY created_at DESC LIMIT 1"
        )).fetchone()
        last_intraday_at = _fmt_dt(r3.ts) if r3 else None

        # intraday count today
        r4 = s.execute(text(
            f"SELECT COUNT(*) AS cnt FROM {intraday_table} WHERE created_at >= :ts"
        ), {"ts": today_start}).fetchone()
        intraday_today = r4.cnt if r4 else 0

        return {
            "last_daily_at": last_daily_at,
            "last_intraday_at": last_intraday_at,
            "daily_today": daily_today,
            "intraday_today": intraday_today,
        }


def _h_GetMarketPredStats(p: dict) -> Any:
    market = (p.get("market", "") or "").upper()
    table_map = {
        "GOLD": "gold_predictions",
        "NASDAQ": "nasdaq_predictions",
        "CRYPTO": "crypto_predictions",
        "SP500": "sp500_predictions",
    }
    table = table_map.get(market)
    if not table:
        raise ValueError(f"Unknown market: {market!r}")

    now = datetime.now()
    today_start = now.replace(hour=0, minute=0, second=0, microsecond=0)

    with session_scope() as s:
        rows = s.execute(
            text(f"""
                SELECT
                    algorithm_name,
                    SUM(CASE WHEN created_at >= :today THEN 1 ELSE 0 END) AS today_count,
                    MAX(created_at) AS last_predict_at
                FROM {table}
                WHERE deleted_at IS NULL
                GROUP BY algorithm_name
                ORDER BY algorithm_name
            """),
            {"today": today_start},
        ).fetchall()
        return [
            {
                "algorithm_name": r.algorithm_name,
                "today_count": r.today_count,
                "last_predict_at": _fmt_dt(r.last_predict_at),
            }
            for r in rows
        ]


def _h_GetSessionDirAccuracy(p: dict) -> Any:
    market = (p.get("market", "") or "").upper()
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    table_map = {
        "GOLD": "gold_predictions",
        "NASDAQ": "nasdaq_predictions",
        "CRYPTO": "crypto_predictions",
        "SP500": "sp500_predictions",
    }
    table = table_map.get(market)
    if not table:
        raise ValueError(f"Unknown market: {market!r}")
    with session_scope() as s:
        rows = s.execute(
            text(f"""
                SELECT
                    algorithm_name AS algorithm,
                    COUNT(*) AS total,
                    COUNT(*) FILTER (WHERE direction_correct = true) AS correct
                FROM {table}
                WHERE direction_correct IS NOT NULL
                  AND deleted_at IS NULL
                  AND prediction_date >= :from_dt
                  AND prediction_date < :to_dt
                GROUP BY algorithm_name
                ORDER BY algorithm_name
            """),
            {"from_dt": from_dt, "to_dt": to_dt},
        ).fetchall()
        result = []
        for r in rows:
            acc = (r.correct / r.total) if r.total > 0 else 0.0
            result.append({
                "algorithm": r.algorithm,
                "total": r.total,
                "correct": r.correct,
                "accuracy": acc,
            })
        return result


def _h_GetSessionBotTrades(p: dict) -> Any:
    market = (p.get("market", "") or "").upper()
    from_dt = _parse_dt(p.get("from"))
    to_dt = _parse_dt(p.get("to"))
    with session_scope() as s:
        rows = s.execute(
            text("""
                SELECT
                    b.id                                                     AS bot_id,
                    b.display_name,
                    b.algorithm,
                    b.currency,
                    CAST(b.initial_capital AS DECIMAL(20,2))                AS initial_capital,
                    COALESCE(CAST(latest.total_value AS DECIMAL(20,2)),
                             CAST(b.initial_capital AS DECIMAL(20,2)))      AS current_value,
                    COALESCE(latest.total_return_pct, 0)                    AS total_return_pct,
                    COUNT(t.id)                                             AS trades,
                    SUM(CASE WHEN t.pnl > 0 THEN 1 ELSE 0 END)             AS wins,
                    SUM(CASE WHEN t.pnl < 0 THEN 1 ELSE 0 END)             AS losses,
                    SUM(CASE WHEN t.pnl = 0 THEN 1 ELSE 0 END)             AS breakeven,
                    COALESCE(SUM(t.pnl), 0)                                 AS session_pnl
                FROM sim_bots b
                LEFT JOIN (
                    SELECT s1.bot_id,
                           s1.total_value,
                           s1.total_return_pct
                    FROM sim_portfolio_snapshots s1
                    WHERE s1.snapshot_date = (
                        SELECT MAX(s2.snapshot_date)
                        FROM sim_portfolio_snapshots s2
                        WHERE s2.bot_id = s1.bot_id
                    )
                ) latest ON latest.bot_id = b.id
                LEFT JOIN sim_trades t ON t.bot_id = b.id
                    AND t.action = 'SELL'
                    AND t.pnl IS NOT NULL
                    AND t.trade_date >= :from_dt
                    AND t.trade_date < :to_dt
                WHERE b.market = :market
                GROUP BY b.id, b.display_name, b.algorithm, b.currency,
                         b.initial_capital, latest.total_value, latest.total_return_pct
                ORDER BY session_pnl DESC
            """),
            {"from_dt": from_dt, "to_dt": to_dt, "market": market},
        ).fetchall()
        result = []
        for r in rows:
            trades = r.trades or 0
            wins = r.wins or 0
            win_rate = (wins / trades) if trades > 0 else 0.0
            result.append({
                "bot_id": r.bot_id,
                "display_name": r.display_name,
                "algorithm": r.algorithm,
                "currency": r.currency,
                "initial_capital": float(r.initial_capital) if r.initial_capital is not None else 0.0,
                "current_value": float(r.current_value) if r.current_value is not None else 0.0,
                "session_pnl": float(r.session_pnl) if r.session_pnl is not None else 0.0,
                "total_return_pct": float(r.total_return_pct) if r.total_return_pct is not None else 0.0,
                "trades": trades,
                "wins": wins,
                "losses": r.losses or 0,
                "breakeven": r.breakeven or 0,
                "win_rate": win_rate,
            })
        return result


def _h_GetPipelineReports(p: dict) -> Any:
    pipeline_key = p.get("pipeline_key", "")
    limit = int(p.get("limit", 50))
    with session_scope() as s:
        q = s.query(PipelineReport).order_by(PipelineReport.created_at.desc()).limit(limit)
        if pipeline_key:
            q = s.query(PipelineReport).filter(
                PipelineReport.pipeline_key == pipeline_key
            ).order_by(PipelineReport.created_at.desc()).limit(limit)
        return [_ser_pipeline_report(r) for r in q.all()]


def _h_GetDistinctPipelineKeys(_p: dict) -> Any:
    with session_scope() as s:
        rows = s.query(distinct(PipelineReport.pipeline_key)).order_by(
            PipelineReport.pipeline_key
        ).all()
        return [r[0] for r in rows]


def _h_Ping(_p: dict) -> Any:
    return []


# ---------------------------------------------------------------------------
# Dispatch table
# ---------------------------------------------------------------------------

_HANDLERS: dict = {
    # Prices
    "GetGoldPricesByDateRange": _h_GetGoldPricesByDateRange,
    "GetLatestGoldPrices": _h_GetLatestGoldPrices,
    "GetGoldIntradayByRange": _h_GetGoldIntradayByRange,
    "GetNasdaqPricesByDateRange": _h_GetNasdaqPricesByDateRange,
    "GetLatestNasdaqPrice": _h_GetLatestNasdaqPrice,
    "GetNasdaqSymbols": _h_GetNasdaqSymbols,
    "GetNasdaqIntradayByRange": _h_GetNasdaqIntradayByRange,
    "GetCryptoPricesByDateRange": _h_GetCryptoPricesByDateRange,
    "GetLatestCryptoPrice": _h_GetLatestCryptoPrice,
    "GetCryptoCoins": _h_GetCryptoCoins,
    "GetCryptoIntradayByRange": _h_GetCryptoIntradayByRange,
    "GetSP500PricesByDateRange": _h_GetSP500PricesByDateRange,
    "GetLatestSP500Price": _h_GetLatestSP500Price,
    "GetSP500Symbols": _h_GetSP500Symbols,
    "GetSP500IntradayByRange": _h_GetSP500IntradayByRange,
    # Gold predictions
    "GetGoldPredictions": _h_GetGoldPredictions,
    "GetLatestGoldPredictions": _h_GetLatestGoldPredictions,
    "GetLatestConfirmedGoldPredictions": _h_GetLatestConfirmedGoldPredictions,
    "GetGoldPredictionsByDateRange": _h_GetGoldPredictionsByDateRange,
    "GetGoldPredictionsPage": _h_GetGoldPredictionsPage,
    # NASDAQ predictions
    "GetNasdaqPredictions": _h_GetNasdaqPredictions,
    "GetLatestNasdaqPredictions": _h_GetLatestNasdaqPredictions,
    "GetLatestConfirmedNasdaqPredictions": _h_GetLatestConfirmedNasdaqPredictions,
    "GetNasdaqPredictionsByDateRange": _h_GetNasdaqPredictionsByDateRange,
    "GetNasdaqPredictionsPage": _h_GetNasdaqPredictionsPage,
    # Crypto predictions
    "GetCryptoPredictions": _h_GetCryptoPredictions,
    "GetLatestCryptoPredictions": _h_GetLatestCryptoPredictions,
    "GetLatestConfirmedCryptoPredictions": _h_GetLatestConfirmedCryptoPredictions,
    "GetCryptoPredictionsByDateRange": _h_GetCryptoPredictionsByDateRange,
    "GetCryptoPredictionsPage": _h_GetCryptoPredictionsPage,
    # SP500 predictions
    "GetSP500Predictions": _h_GetSP500Predictions,
    "GetLatestSP500Predictions": _h_GetLatestSP500Predictions,
    "GetLatestConfirmedSP500Predictions": _h_GetLatestConfirmedSP500Predictions,
    "GetSP500PredictionsByDateRange": _h_GetSP500PredictionsByDateRange,
    "GetSP500PredictionsPage": _h_GetSP500PredictionsPage,
    # Simulation
    "GetAllSimBots": _h_GetAllSimBots,
    "GetSimBotByID": _h_GetSimBotByID,
    "GetSimBotsByMarketAlgo": _h_GetSimBotsByMarketAlgo,
    "GetLatestSimSession": _h_GetLatestSimSession,
    "GetBestSimSessionForChart": _h_GetBestSimSessionForChart,
    "GetLatestLiveSimSession": _h_GetLatestLiveSimSession,
    "GetSimTrades": _h_GetSimTrades,
    "GetSimPortfolioSnapshots": _h_GetSimPortfolioSnapshots,
    "GetAllSessionsWithSnapCount": _h_GetAllSessionsWithSnapCount,
    "GetSessionTradeStatsBatch": _h_GetSessionTradeStatsBatch,
    "GetLastSnapshotsBatch": _h_GetLastSnapshotsBatch,
    "GetLeaderboardEntries": _h_GetLeaderboardEntries,
    "UpdateSimBotConfig": _h_UpdateSimBotConfig,
    # Training
    "GetTrainingLogByID": _h_GetTrainingLogByID,
    "GetTrainingLogsBySessionID": _h_GetTrainingLogsBySessionID,
    "GetTrainingSessions": _h_GetTrainingSessions,
    "GetLatestTrainingLogByAlgorithm": _h_GetLatestTrainingLogByAlgorithm,
    "GetTrainingMetricsAggregate": _h_GetTrainingMetricsAggregate,
    "GetTrainingSessionsByMarket": _h_GetTrainingSessionsByMarket,
    # Schedules / sync / pipeline / monitoring / direction / session
    "GetAllCronSchedules": _h_GetAllCronSchedules,
    "GetCronScheduleByKey": _h_GetCronScheduleByKey,
    "UpsertCronSchedule": _h_UpsertCronSchedule,
    "GetLatestSyncLogs": _h_GetLatestSyncLogs,
    "GetDirectionAccuracy": _h_GetDirectionAccuracy,
    "GetMarketCrawlStats": _h_GetMarketCrawlStats,
    "GetMarketPredStats": _h_GetMarketPredStats,
    "GetSessionDirAccuracy": _h_GetSessionDirAccuracy,
    "GetSessionBotTrades": _h_GetSessionBotTrades,
    "GetPipelineReports": _h_GetPipelineReports,
    "GetDistinctPipelineKeys": _h_GetDistinctPipelineKeys,
    "Ping": _h_Ping,
}


def dispatch(method: str, params_json: str) -> tuple[str, str]:
    """Execute method and return (result_json, error_str).

    result_json is "" on error; error_str is "" on success.
    """
    handler = _HANDLERS.get(method)
    if handler is None:
        return "", f"unknown method: {method!r}"

    try:
        params: dict = json.loads(params_json) if params_json.strip() else {}
    except json.JSONDecodeError as exc:
        return "", f"invalid params_json: {exc}"

    try:
        result = handler(params)
        return json.dumps(result, ensure_ascii=False, default=_json_fallback), ""
    except Exception as exc:
        log.error("query_handler.error", method=method, error=str(exc))
        return "", str(exc)


def _json_fallback(obj: Any) -> Any:
    """json.dumps default= fallback for any remaining non-serializable type."""
    if isinstance(obj, Decimal):
        return str(obj)
    if isinstance(obj, datetime):
        return _fmt_dt(obj)
    if isinstance(obj, date):
        return _fmt_date(obj)
    raise TypeError(f"Object of type {type(obj).__name__} is not JSON serializable")
