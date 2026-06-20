"""Shared feature builder for tree-based ML algorithms.

Provides two functions:
  - build_basic_features  — original 14-feature set (backward compatible)
  - build_enhanced_features — expanded ~30-feature set using pandas-ta when available

Both functions accept:
  arr     : np.ndarray  — close prices (length N)
  vol_arr : np.ndarray | None — volumes (length N, or None)

Both return (features_list, targets_list) where each element corresponds to
one day's feature row / next-day log return target.

Minimum data required: MIN_DATA_POINTS = 80 (callers are responsible for
enforcing this before calling).
"""
from __future__ import annotations

import numpy as np

MIN_DATA_POINTS = 80

# Try importing pandas-ta at module level — graceful fallback if absent
try:
    import pandas as pd
    import pandas_ta as ta  # noqa: F401  (imported for availability check)
    _PANDAS_TA_AVAILABLE = True
except ImportError:
    _PANDAS_TA_AVAILABLE = False


# ---------------------------------------------------------------------------
# Basic feature builder (original 14 features — kept for backward compat)
# ---------------------------------------------------------------------------

def build_basic_features(
    arr: np.ndarray,
    vol_arr: np.ndarray | None,
) -> tuple[list, list]:
    """Original feature set: lag returns 1-10, RSI(14), MA5/MA20 ratios, vol ratio.

    Identical to the old _build_features() in lightgbm/xgboost/random_forest.
    Returns (features_list, targets_list).
    """
    log_returns = np.diff(np.log(arr))
    features, targets = [], []

    # Iterate over price index `i` (the "as-of" day). The row is built using
    # ONLY prices up to and including arr[i]; the target is the next-day return
    # log(arr[i+1]/arr[i]) == log_returns[i]. The final iteration (i == len(arr)-1)
    # is the inference row — as-of the latest price, with no realised target yet.
    for i in range(10, len(log_returns) + 1):
        row: list[float] = []

        # --- Lag returns: 1..10 (all strictly past, end at log_returns[i-1]) ---
        for lag in range(1, 11):
            row.append(float(log_returns[i - lag]))

        # --- RSI(14) — as-of day i (inclusive of arr[i], no future) ---
        if i >= 14:
            subset = arr[i - 14 : i + 1]
            deltas = np.diff(subset)
            gains = np.where(deltas > 0, deltas, 0.0)
            losses = np.where(deltas < 0, -deltas, 0.0)
            avg_g = float(np.mean(gains)) if len(gains) > 0 else 1e-9
            avg_l = float(np.mean(losses)) if len(losses) > 0 else 1e-9
            rsi = 100.0 - 100.0 / (1.0 + avg_g / (avg_l + 1e-9))
        else:
            rsi = 50.0
        row.append(float(rsi))

        # --- MA ratios: price / MA5, price / MA20 (price_now = arr[i], not arr[i+1]) ---
        price_now = float(arr[i])
        ma5 = float(np.mean(arr[max(0, i - 4) : i + 1])) if i >= 4 else price_now
        ma20 = float(np.mean(arr[max(0, i - 19) : i + 1])) if i >= 19 else price_now
        row.append(price_now / ma5 if ma5 > 0 else 1.0)
        row.append(price_now / ma20 if ma20 > 0 else 1.0)

        # --- Volume ratio: current vol / avg vol(10) — as-of day i ---
        if vol_arr is not None and len(vol_arr) > i:
            vol_now = float(vol_arr[i])
            vol_avg = float(np.mean(vol_arr[max(0, i - 9) : i + 1]))
            row.append(vol_now / vol_avg if vol_avg > 0 else 1.0)
        else:
            row.append(1.0)

        features.append(row)
        if i < len(log_returns):
            targets.append(float(log_returns[i]))
        else:
            # Inference row: no next-day return exists yet. Placeholder kept as a
            # real log return so callers that drop the last pair via [:-1] are
            # unaffected; features[-1] is used for prediction only.
            targets.append(float(log_returns[-1]))

    return features, targets


# ---------------------------------------------------------------------------
# Enhanced feature builder (~30 features, with pandas-ta)
# ---------------------------------------------------------------------------

def build_enhanced_features(
    arr: np.ndarray,
    vol_arr: np.ndarray | None,
) -> tuple[list, list]:
    """Expanded feature set inspired by FreqAI / quantitative finance practice.

    Features (up to ~30):
      Group A — Lag returns (10 features)
        lag_1 … lag_10        — log returns lagged 1-10 days

      Group B — Moving-average ratios (4 features)
        price / MA5, price / MA10, price / MA20, price / MA50

      Group C — Multi-timeframe returns (3 features)
        5-day log return, 10-day log return, 20-day log return

      Group D — RSI (1 feature, via pandas_ta or numpy fallback)
        RSI(14)

      Group E — Stochastic RSI (2 features, pandas_ta or fallback)
        stochrsi_k, stochrsi_d

      Group F — Bollinger %B (1 feature, pandas_ta or fallback)
        bb_pct_b  — position within Bollinger Bands [0, 1]

      Group G — MACD features (2 features, pandas_ta or fallback)
        macd_line_norm    — MACD line / price
        macd_hist_norm    — MACD histogram / price

      Group H — Volatility (3 features)
        rolling_std_5d, rolling_std_10d, rolling_std_20d  — of log returns

      Group I — Rate of Change (1 feature, pandas_ta or fallback)
        roc_10  — 10-day rate of change (%)

      Group J — Momentum (2 features)
        momentum_5   — (price_t - price_{t-5}) / price_{t-5}
        momentum_10  — (price_t - price_{t-10}) / price_{t-10}

      Group K — Volume ratio (1 feature)
        vol_ratio  — current vol / avg vol(10)

    Total: 10 + 4 + 3 + 1 + 2 + 1 + 2 + 3 + 1 + 2 + 1 = 30 features

    Falls back gracefully to numpy-only computation when pandas-ta is absent.
    Returns (features_list, targets_list).
    """
    if _PANDAS_TA_AVAILABLE:
        return _build_enhanced_with_pandas_ta(arr, vol_arr)
    return _build_enhanced_numpy_fallback(arr, vol_arr)


# ---------------------------------------------------------------------------
# Pandas-ta implementation
# ---------------------------------------------------------------------------

def _build_enhanced_with_pandas_ta(
    arr: np.ndarray,
    vol_arr: np.ndarray | None,
) -> tuple[list, list]:
    import pandas as pd
    import pandas_ta as ta  # noqa: F811

    close = pd.Series(arr, dtype=float)
    log_returns = np.diff(np.log(arr))

    # Pre-compute pandas_ta indicators on the full series -----------------
    # RSI(14)
    rsi_series = ta.rsi(close, length=14)
    if rsi_series is None:
        rsi_series = pd.Series([50.0] * len(close))
    rsi_series = rsi_series.fillna(50.0)

    # Stochastic RSI — returns DataFrame with columns STOCHRSIk_14_14_3_3 etc.
    stochrsi_df = ta.stochrsi(close, length=14, rsi_length=14, k=3, d=3)
    if stochrsi_df is not None and not stochrsi_df.empty:
        stoch_k = stochrsi_df.iloc[:, 0].fillna(50.0)
        stoch_d = stochrsi_df.iloc[:, 1].fillna(50.0)
    else:
        stoch_k = pd.Series([50.0] * len(close))
        stoch_d = pd.Series([50.0] * len(close))

    # Bollinger Bands — returns DataFrame with columns BBL, BBM, BBU, BBB, BBP
    bb_df = ta.bbands(close, length=20, std=2.0)
    if bb_df is not None and not bb_df.empty:
        # %B column is typically the 4th column (index 4) or named BBP_...
        pct_b_col = [c for c in bb_df.columns if c.startswith("BBP")]
        if pct_b_col:
            bb_pct_b = bb_df[pct_b_col[0]].fillna(0.5)
        else:
            bb_pct_b = bb_df.iloc[:, 4].fillna(0.5) if bb_df.shape[1] > 4 else pd.Series([0.5] * len(close))
    else:
        bb_pct_b = pd.Series([0.5] * len(close))

    # MACD — returns DataFrame with MACD_, MACDh_, MACDs_ columns
    macd_df = ta.macd(close, fast=12, slow=26, signal=9)
    if macd_df is not None and not macd_df.empty:
        macd_line_s = macd_df.iloc[:, 0].fillna(0.0)   # MACD line
        macd_hist_s = macd_df.iloc[:, 1].fillna(0.0)   # histogram
    else:
        macd_line_s = pd.Series([0.0] * len(close))
        macd_hist_s = pd.Series([0.0] * len(close))

    # ROC(10)
    roc_series = ta.roc(close, length=10)
    if roc_series is None:
        roc_series = pd.Series([0.0] * len(close))
    roc_series = roc_series.fillna(0.0)

    # Convert to numpy for fast indexing
    rsi_np = rsi_series.to_numpy(dtype=float)
    stoch_k_np = stoch_k.to_numpy(dtype=float)
    stoch_d_np = stoch_d.to_numpy(dtype=float)
    bb_pct_b_np = bb_pct_b.to_numpy(dtype=float)
    macd_line_np = macd_line_s.to_numpy(dtype=float)
    macd_hist_np = macd_hist_s.to_numpy(dtype=float)
    roc_np = roc_series.to_numpy(dtype=float)

    features: list = []
    targets: list = []

    # Iterate over price index via `i`; price_idx == i is the "as-of" day so every
    # feature uses only arr[:i+1]. Target is log_returns[i] (return i -> i+1). The
    # last iteration (i == len(log_returns)) is the inference row (no target yet).
    for i in range(20, len(log_returns) + 1):  # start at 20 to allow MA20 and multiframe returns
        price_idx = i   # index into arr — as-of day (NO look-ahead to arr[i+1])
        price_now = float(arr[price_idx])

        row: list[float] = []

        # A: Lag returns 1..10
        for lag in range(1, 11):
            row.append(float(log_returns[i - lag]))

        # B: MA ratios (price / MA5/10/20/50)
        for window, min_i in [(5, 4), (10, 9), (20, 19), (50, 49)]:
            if price_idx >= window:
                ma = float(np.mean(arr[price_idx - window : price_idx]))
            else:
                ma = price_now
            row.append(price_now / ma if ma > 0 else 1.0)

        # C: Multi-timeframe log returns (5d, 10d, 20d)
        for back in (5, 10, 20):
            if price_idx >= back:
                ref = float(arr[price_idx - back])
                mt_ret = np.log(price_now / ref) if ref > 0 else 0.0
            else:
                mt_ret = 0.0
            row.append(float(mt_ret))

        # D: RSI(14) — index aligned to arr
        row.append(float(_safe_get(rsi_np, price_idx, 50.0)))

        # E: StochRSI %K, %D
        row.append(float(_safe_get(stoch_k_np, price_idx, 50.0)))
        row.append(float(_safe_get(stoch_d_np, price_idx, 50.0)))

        # F: Bollinger %B
        row.append(float(_safe_get(bb_pct_b_np, price_idx, 0.5)))

        # G: MACD line/hist normalised by price
        ml = float(_safe_get(macd_line_np, price_idx, 0.0))
        mh = float(_safe_get(macd_hist_np, price_idx, 0.0))
        row.append(ml / price_now if price_now > 0 else 0.0)
        row.append(mh / price_now if price_now > 0 else 0.0)

        # H: Rolling std of log returns (5d, 10d, 20d)
        for back in (5, 10, 20):
            start = max(0, i - back)
            chunk = log_returns[start : i]
            row.append(float(np.std(chunk)) if len(chunk) > 1 else 0.0)

        # I: ROC(10) — percent
        row.append(float(_safe_get(roc_np, price_idx, 0.0)))

        # J: Momentum 5, 10
        for back in (5, 10):
            if price_idx >= back:
                ref = float(arr[price_idx - back])
                row.append((price_now - ref) / ref if ref > 0 else 0.0)
            else:
                row.append(0.0)

        # K: Volume ratio
        if vol_arr is not None and len(vol_arr) > price_idx:
            vol_now = float(vol_arr[price_idx])
            vol_avg = float(np.mean(vol_arr[max(0, price_idx - 10) : price_idx]))
            row.append(vol_now / vol_avg if vol_avg > 0 else 1.0)
        else:
            row.append(1.0)

        features.append(row)
        if i < len(log_returns):
            targets.append(float(log_returns[i]))
        else:
            targets.append(float(log_returns[-1]))  # inference-row placeholder (dropped by callers)

    return features, targets


def _safe_get(arr: np.ndarray, idx: int, default: float) -> float:
    """Safely index into numpy array, returning default if out of bounds or NaN."""
    if idx < 0 or idx >= len(arr):
        return default
    val = float(arr[idx])
    return val if not np.isnan(val) else default


# ---------------------------------------------------------------------------
# Pure-numpy fallback (no pandas-ta) for enhanced features
# ---------------------------------------------------------------------------

def _build_enhanced_numpy_fallback(
    arr: np.ndarray,
    vol_arr: np.ndarray | None,
) -> tuple[list, list]:
    """Enhanced features using only numpy — no pandas-ta dependency.

    Approximates the same feature groups with pure numpy:
      A: Lag returns 1-10
      B: MA5/10/20/50 ratios
      C: Multi-timeframe returns 5/10/20d
      D: RSI(14)
      E: StochRSI %K/%D (approximate via 14-period RSI range normalization)
      F: Bollinger %B (20-period SMA ± 2*std)
      G: MACD line/hist normalised
      H: Rolling std 5/10/20d
      I: ROC(10)
      J: Momentum 5/10
      K: Volume ratio
    """
    log_returns = np.diff(np.log(arr))
    features: list = []
    targets: list = []

    # price_idx == i is the "as-of" day (no look-ahead). Last iteration is the
    # inference row. Mirrors _build_enhanced_with_pandas_ta exactly.
    for i in range(20, len(log_returns) + 1):
        price_idx = i
        price_now = float(arr[price_idx])
        row: list[float] = []

        # A: Lag returns 1..10
        for lag in range(1, 11):
            row.append(float(log_returns[i - lag]))

        # B: MA ratios
        for window in (5, 10, 20, 50):
            if price_idx >= window:
                ma = float(np.mean(arr[price_idx - window : price_idx]))
            else:
                ma = price_now
            row.append(price_now / ma if ma > 0 else 1.0)

        # C: Multi-timeframe log returns
        for back in (5, 10, 20):
            if price_idx >= back:
                ref = float(arr[price_idx - back])
                mt_ret = float(np.log(price_now / ref)) if ref > 0 else 0.0
            else:
                mt_ret = 0.0
            row.append(mt_ret)

        # D: RSI(14)
        rsi = _numpy_rsi(arr, price_idx, period=14)
        row.append(float(rsi))

        # E: Stochastic RSI approximation
        #    Compute RSI over rolling 14-period window and normalise
        stoch_k, stoch_d = _numpy_stochrsi_approx(arr, price_idx, rsi_period=14, stoch_period=14)
        row.append(float(stoch_k))
        row.append(float(stoch_d))

        # F: Bollinger %B
        bb_pct = _numpy_bb_pct(arr, price_idx, period=20)
        row.append(float(bb_pct))

        # G: MACD
        macd_line, macd_hist = _numpy_macd(arr, price_idx, fast=12, slow=26, signal=9)
        row.append(macd_line / price_now if price_now > 0 else 0.0)
        row.append(macd_hist / price_now if price_now > 0 else 0.0)

        # H: Rolling std
        for back in (5, 10, 20):
            start = max(0, i - back)
            chunk = log_returns[start : i]
            row.append(float(np.std(chunk)) if len(chunk) > 1 else 0.0)

        # I: ROC(10)
        if price_idx >= 10:
            ref10 = float(arr[price_idx - 10])
            roc = (price_now - ref10) / ref10 * 100.0 if ref10 > 0 else 0.0
        else:
            roc = 0.0
        row.append(float(roc))

        # J: Momentum 5, 10
        for back in (5, 10):
            if price_idx >= back:
                ref = float(arr[price_idx - back])
                row.append((price_now - ref) / ref if ref > 0 else 0.0)
            else:
                row.append(0.0)

        # K: Volume ratio
        if vol_arr is not None and len(vol_arr) > price_idx:
            vol_now = float(vol_arr[price_idx])
            vol_avg = float(np.mean(vol_arr[max(0, price_idx - 10) : price_idx]))
            row.append(vol_now / vol_avg if vol_avg > 0 else 1.0)
        else:
            row.append(1.0)

        features.append(row)
        if i < len(log_returns):
            targets.append(float(log_returns[i]))
        else:
            targets.append(float(log_returns[-1]))  # inference-row placeholder (dropped by callers)

    return features, targets


# ---------------------------------------------------------------------------
# Numpy helpers for indicator approximations
# ---------------------------------------------------------------------------

def _numpy_rsi(arr: np.ndarray, end_idx: int, period: int = 14) -> float:
    """Compute RSI(period) using end_idx as the last price index."""
    start = max(0, end_idx - period)
    subset = arr[start : end_idx + 1]
    if len(subset) < 2:
        return 50.0
    deltas = np.diff(subset)
    gains = np.where(deltas > 0, deltas, 0.0)
    losses = np.where(deltas < 0, -deltas, 0.0)
    avg_g = float(np.mean(gains))
    avg_l = float(np.mean(losses))
    if avg_l < 1e-12:
        return 100.0 if avg_g > 0 else 50.0
    rs = avg_g / avg_l
    return float(100.0 - 100.0 / (1.0 + rs))


def _numpy_stochrsi_approx(
    arr: np.ndarray,
    end_idx: int,
    rsi_period: int = 14,
    stoch_period: int = 14,
) -> tuple[float, float]:
    """Approximate StochRSI %K/%D via a rolling RSI series."""
    total_needed = rsi_period + stoch_period
    start = max(0, end_idx - total_needed)
    sub = arr[start : end_idx + 1]
    if len(sub) < rsi_period + 2:
        return 50.0, 50.0

    # Build a mini RSI series over sub
    rsi_vals: list[float] = []
    for j in range(rsi_period, len(sub)):
        chunk = sub[: j + 1]
        rsi_vals.append(_numpy_rsi(chunk, len(chunk) - 1, rsi_period))

    if len(rsi_vals) < stoch_period:
        return 50.0, 50.0

    window = rsi_vals[-stoch_period:]
    rsi_now = rsi_vals[-1]
    rsi_low = min(window)
    rsi_high = max(window)
    denom = rsi_high - rsi_low
    stoch_k = (rsi_now - rsi_low) / denom * 100.0 if denom > 0 else 50.0
    # %D = 3-period SMA of %K (approximate)
    k_series = []
    for j in range(-3, 0):
        rv = rsi_vals[j]
        rl = min(rsi_vals[max(0, j - stoch_period) : j + 1] or [rv])
        rh = max(rsi_vals[max(0, j - stoch_period) : j + 1] or [rv])
        d2 = rh - rl
        k_series.append((rv - rl) / d2 * 100.0 if d2 > 0 else 50.0)
    stoch_d = float(np.mean(k_series))

    return float(np.clip(stoch_k, 0.0, 100.0)), float(np.clip(stoch_d, 0.0, 100.0))


def _numpy_bb_pct(arr: np.ndarray, end_idx: int, period: int = 20) -> float:
    """Bollinger %B using end_idx as the last price index."""
    start = max(0, end_idx - period + 1)
    window = arr[start : end_idx + 1]
    if len(window) < 2:
        return 0.5
    sma = float(np.mean(window))
    std = float(np.std(window))
    if std < 1e-12:
        return 0.5
    price = float(arr[end_idx])
    upper = sma + 2.0 * std
    lower = sma - 2.0 * std
    denom = upper - lower
    pct = (price - lower) / denom if denom > 0 else 0.5
    return float(np.clip(pct, -0.5, 1.5))  # allow slight overshoot


def _numpy_ema_scalar(prices: np.ndarray, period: int) -> float:
    if len(prices) < period:
        return float(np.mean(prices)) if len(prices) > 0 else 0.0
    k = 2.0 / (period + 1)
    ema = float(np.mean(prices[:period]))
    for p in prices[period:]:
        ema = float(p) * k + ema * (1 - k)
    return ema


def _numpy_macd(
    arr: np.ndarray,
    end_idx: int,
    fast: int = 12,
    slow: int = 26,
    signal: int = 9,
) -> tuple[float, float]:
    """Compute MACD line and histogram at end_idx."""
    needed = slow + signal
    start = max(0, end_idx - needed - 10)  # extra buffer for EMA warmup
    sub = arr[start : end_idx + 1]
    if len(sub) < slow:
        return 0.0, 0.0

    ema_fast = _numpy_ema_scalar(sub, fast)
    ema_slow = _numpy_ema_scalar(sub, slow)
    macd_line = ema_fast - ema_slow

    # Build a short MACD series for the signal line
    macd_vals: list[float] = []
    for j in range(slow - 1, len(sub)):
        ef = _numpy_ema_scalar(sub[: j + 1], fast)
        es = _numpy_ema_scalar(sub[: j + 1], slow)
        macd_vals.append(ef - es)

    if len(macd_vals) < signal:
        return float(macd_line), 0.0

    sig_line = _numpy_ema_scalar(np.array(macd_vals), signal)
    histogram = macd_line - sig_line
    return float(macd_line), float(histogram)
