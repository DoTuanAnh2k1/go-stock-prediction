"""Vietnamese number format parser.

Handles formats like:
  "1.234.567"   -> 1234567
  "1,234,567"   -> 1234567
  "1.234,50"    -> 1234.50
  "30,000"      -> 30000
"""
from __future__ import annotations

import re
from decimal import Decimal, InvalidOperation


def parse_vnd_price(raw: str) -> Decimal:
    """Parse a Vietnamese price string to Decimal.

    Strips all dots and commas used as thousands separators.
    Handles: "155.500.000", "155,500,000", "1.234,56" patterns.

    Raises ValueError for empty / zero / dash values.
    """
    cleaned = raw.strip()
    if not cleaned or cleaned in ("-", "—", "N/A", ""):
        raise ValueError(f"empty price string: {raw!r}")

    # Remove space separators
    cleaned = cleaned.replace(" ", "")

    # Detect decimal separator:
    # If the last separator is a comma and there are exactly 2 digits after it,
    # treat comma as decimal separator, dots as thousands sep.
    # Otherwise treat all separators as thousands separators.
    last_comma = cleaned.rfind(",")
    last_dot = cleaned.rfind(".")

    if last_comma > last_dot:
        # Comma is last → could be decimal separator (e.g. "1.234,56")
        after_comma = cleaned[last_comma + 1 :]
        if len(after_comma) == 2:
            # Decimal comma format
            integer_part = cleaned[:last_comma].replace(".", "").replace(",", "")
            decimal_part = after_comma
            cleaned = f"{integer_part}.{decimal_part}"
        else:
            # Comma as thousands separator
            cleaned = cleaned.replace(",", "").replace(".", "")
    else:
        # Dot is last or no separators → dots and commas are thousands separators
        cleaned = cleaned.replace(",", "").replace(".", "")

    # Remove any remaining non-numeric chars except leading minus and single dot
    cleaned = re.sub(r"[^\d.\-]", "", cleaned)

    if not cleaned or cleaned in ("", "-", "."):
        raise ValueError(f"cannot parse price: {raw!r}")

    try:
        value = Decimal(cleaned)
    except InvalidOperation as exc:
        raise ValueError(f"invalid decimal: {cleaned!r} (from {raw!r})") from exc

    if value == Decimal(0):
        raise ValueError(f"zero price: {raw!r}")

    return value


def safe_parse_vnd(raw: str, default: Decimal = Decimal(0)) -> Decimal:
    """Like parse_vnd_price but returns default on any error."""
    try:
        return parse_vnd_price(raw)
    except (ValueError, TypeError):
        return default
