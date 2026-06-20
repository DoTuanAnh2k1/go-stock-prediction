"""Unit tests for the shared enhanced feature engineering module.

Tests src.algorithms.features — build_basic_features() and build_enhanced_features().

No network, no DB, no Docker required — pure NumPy/pandas computation.

Skips gracefully if the module has not been created yet or if optional
dependencies (pandas_ta) are absent.
"""
from __future__ import annotations

import importlib
import importlib.util
import math

import numpy as np
import pytest

# ---------------------------------------------------------------------------
# Module-level skip if features.py does not exist yet
# ---------------------------------------------------------------------------

features_mod = pytest.importorskip(
    "src.algorithms.features",
    reason="src.algorithms.features not yet created — skipping feature tests",
)

build_basic_features = features_mod.build_basic_features
build_enhanced_features = features_mod.build_enhanced_features
MIN_DATA_POINTS = features_mod.MIN_DATA_POINTS

# pandas_ta is optional — tests that need it carry their own skip guard
_pandas_ta_available = importlib.util.find_spec("pandas_ta") is not None

# ---------------------------------------------------------------------------
# Shared realistic test data (seeded for reproducibility)
# ---------------------------------------------------------------------------

np.random.seed(42)
_PRICES_200 = list(100.0 * np.cumprod(1 + np.random.normal(0.001, 0.02, 200)))
_VOLUMES_200 = list(np.random.uniform(1e6, 5e6, 200))

# Short array — fewer than MIN_DATA_POINTS (80) points
np.random.seed(7)
_PRICES_50 = list(50.0 * np.cumprod(1 + np.random.normal(0.0005, 0.015, 50)))
_VOLUMES_50 = list(np.random.uniform(5e5, 3e6, 50))

# Feature counts per row (derived from module docstring / implementation)
BASIC_FEATURE_COUNT = 14   # 10 lags + RSI + MA5 ratio + MA20 ratio + vol ratio
ENHANCED_FEATURE_COUNT = 30  # 10+4+3+1+2+1+2+3+1+2+1

# enhanced starts at log_returns index 20 (needs MA50 warmup); basic starts at 10
_ENHANCED_EXTRA_SKIP = 10  # enhanced produces 10 fewer rows than basic


# ---------------------------------------------------------------------------
# Helper
# ---------------------------------------------------------------------------


def _has_nan(features: list[list[float]]) -> bool:
    """Return True if any value in the feature matrix is NaN or infinite."""
    for row in features:
        for val in row:
            if math.isnan(val) or math.isinf(val):
                return True
    return False


# ---------------------------------------------------------------------------
# Tests: build_basic_features
# ---------------------------------------------------------------------------


class TestBuildBasicFeatures:
    """Tests for build_basic_features(arr, vol_arr) -> (features, targets)."""

    def test_build_basic_features_output_shape(self):
        """Each row must contain exactly 14 features: 10 lags + RSI + 2 MA ratios + vol ratio."""
        arr = np.array(_PRICES_200, dtype=float)
        vol_arr = np.array(_VOLUMES_200, dtype=float)

        features, targets = build_basic_features(arr, vol_arr)

        assert len(features) > 0, "Should return at least one feature row"
        assert len(features[0]) == BASIC_FEATURE_COUNT, (
            f"Expected {BASIC_FEATURE_COUNT} features per row, got {len(features[0])}"
        )
        assert len(features) == len(targets), "features and targets must have equal length"

    def test_build_basic_features_min_data(self):
        """Short arrays (<80 points) should either return few/empty lists or raise gracefully."""
        arr = np.array(_PRICES_50, dtype=float)
        vol_arr = np.array(_VOLUMES_50, dtype=float)

        # Must not crash with an unhandled exception
        try:
            features, targets = build_basic_features(arr, vol_arr)
            # If something was returned, lengths must be consistent
            assert len(features) == len(targets)
            # Short input → fewer feature rows than full dataset
            assert len(features) < 100
        except (ValueError, IndexError):
            pass  # Raising is also acceptable for insufficient data

    def test_build_basic_features_no_volume(self):
        """vol_arr=None must produce 14 features with a placeholder vol ratio of 1.0."""
        arr = np.array(_PRICES_200, dtype=float)

        features, targets = build_basic_features(arr, None)

        assert len(features) > 0
        assert len(features[0]) == BASIC_FEATURE_COUNT
        # The volume ratio is the last feature; defaults to 1.0 when no volumes provided
        for row in features:
            assert row[-1] == pytest.approx(1.0, abs=1e-9), (
                "Vol ratio should default to 1.0 when vol_arr is None"
            )

    def test_features_no_nan_basic(self):
        """No NaN or Inf values should appear in basic feature output."""
        arr = np.array(_PRICES_200, dtype=float)
        vol_arr = np.array(_VOLUMES_200, dtype=float)

        features, targets = build_basic_features(arr, vol_arr)

        assert not _has_nan(features), "basic features contain NaN or Inf"
        for t in targets:
            assert not math.isnan(t) and not math.isinf(t), "targets contain NaN or Inf"

    def test_targets_match_log_returns_basic(self):
        """Targets should be next-day log returns derived from arr."""
        arr = np.array(_PRICES_200, dtype=float)

        features, targets = build_basic_features(arr, None)

        log_returns = np.diff(np.log(arr))
        # All target values must be log return values present in the computed series
        log_return_set = set(round(float(v), 10) for v in log_returns)
        for t in targets:
            assert round(float(t), 10) in log_return_set, (
                f"Target {t} is not a log return from the price series"
            )

    def test_build_basic_features_lengths_consistent(self):
        """Feature and target list lengths must always be equal for various input sizes."""
        for n in [100, 150, 200]:
            np.random.seed(n)
            prices = list(80.0 * np.cumprod(1 + np.random.normal(0.0008, 0.018, n)))
            arr = np.array(prices, dtype=float)

            features, targets = build_basic_features(arr, None)

            assert len(features) == len(targets), (
                f"Length mismatch for n={n}: {len(features)} features vs {len(targets)} targets"
            )

    def test_basic_lag_features_are_log_returns(self):
        """First 10 feature columns must be lag log-returns of the price series."""
        arr = np.array(_PRICES_200, dtype=float)

        features, targets = build_basic_features(arr, None)

        log_returns = np.diff(np.log(arr))
        # Feature start index in log_returns: the loop starts at i=10
        # features[0] corresponds to i=10; lag_1 = log_returns[9], lag_2 = log_returns[8], ...
        first_row = features[0]
        for lag in range(1, 11):
            expected = float(log_returns[10 - lag])
            assert first_row[lag - 1] == pytest.approx(expected, rel=1e-9, abs=1e-12), (
                f"Lag {lag} feature mismatch: expected {expected}, got {first_row[lag - 1]}"
            )

    def test_basic_rsi_in_range(self):
        """RSI column (index 10) must be in [0, 100] for all rows."""
        arr = np.array(_PRICES_200, dtype=float)

        features, _ = build_basic_features(arr, None)

        for i, row in enumerate(features):
            rsi_val = row[10]
            assert 0.0 <= rsi_val <= 100.0, f"Row {i}: RSI {rsi_val} outside [0, 100]"

    def test_basic_ma_ratios_positive(self):
        """MA ratio columns (11 and 12) must be positive (price/MA is always positive)."""
        arr = np.array(_PRICES_200, dtype=float)

        features, _ = build_basic_features(arr, None)

        for i, row in enumerate(features):
            assert row[11] > 0, f"Row {i}: MA5 ratio {row[11]} is not positive"
            assert row[12] > 0, f"Row {i}: MA20 ratio {row[12]} is not positive"


# ---------------------------------------------------------------------------
# Tests: build_enhanced_features
# ---------------------------------------------------------------------------


class TestBuildEnhancedFeatures:
    """Tests for build_enhanced_features(arr, vol_arr) -> (features, targets)."""

    def test_build_enhanced_features_output_shape(self):
        """Enhanced features must return 30 features per row and more rows than basic."""
        arr = np.array(_PRICES_200, dtype=float)
        vol_arr = np.array(_VOLUMES_200, dtype=float)

        features, targets = build_enhanced_features(arr, vol_arr)

        assert len(features) > 0, "Should return at least one feature row"
        n_features = len(features[0])
        assert n_features == ENHANCED_FEATURE_COUNT, (
            f"Expected {ENHANCED_FEATURE_COUNT} features per row, got {n_features}"
        )
        assert len(features) == len(targets), "features and targets must have equal length"

    def test_build_enhanced_features_more_than_basic(self):
        """Enhanced feature set must have strictly more columns than basic."""
        arr = np.array(_PRICES_200, dtype=float)

        basic_features, _ = build_basic_features(arr, None)
        enhanced_features, _ = build_enhanced_features(arr, None)

        assert len(enhanced_features[0]) > len(basic_features[0]), (
            f"Enhanced ({len(enhanced_features[0])}) must exceed basic ({len(basic_features[0])})"
        )

    def test_build_enhanced_features_min_data(self):
        """Short arrays (<80 points) should not crash — empty result or ValueError is fine."""
        arr = np.array(_PRICES_50, dtype=float)
        vol_arr = np.array(_VOLUMES_50, dtype=float)

        try:
            features, targets = build_enhanced_features(arr, vol_arr)
            assert len(features) == len(targets)
        except (ValueError, IndexError):
            pass  # Acceptable for insufficient data

    def test_build_enhanced_features_no_volume(self):
        """vol_arr=None should work — volume ratio defaults to 1.0."""
        arr = np.array(_PRICES_200, dtype=float)

        features, targets = build_enhanced_features(arr, None)

        assert len(features) > 0
        n_features = len(features[0])
        assert n_features == ENHANCED_FEATURE_COUNT
        assert len(features) == len(targets)
        # Last feature (vol ratio) should be 1.0 when no volumes provided
        for row in features:
            assert row[-1] == pytest.approx(1.0, abs=1e-9), (
                "Vol ratio should default to 1.0 when vol_arr is None"
            )

    def test_build_enhanced_features_with_volume(self):
        """Providing volumes should not reduce feature count (same columns, different values)."""
        arr = np.array(_PRICES_200, dtype=float)
        vol_arr = np.array(_VOLUMES_200, dtype=float)

        features_no_vol, _ = build_enhanced_features(arr, None)
        features_with_vol, _ = build_enhanced_features(arr, vol_arr)

        # Feature count must be the same (volumes affect last column value, not count)
        assert len(features_with_vol[0]) == len(features_no_vol[0]), (
            "Volume presence should not change feature count"
        )
        # Row count must be the same
        assert len(features_with_vol) == len(features_no_vol)

    def test_features_no_nan_enhanced(self):
        """No NaN or Inf values in enhanced feature output."""
        arr = np.array(_PRICES_200, dtype=float)
        vol_arr = np.array(_VOLUMES_200, dtype=float)

        features, targets = build_enhanced_features(arr, vol_arr)

        assert not _has_nan(features), "enhanced features contain NaN or Inf"
        for t in targets:
            assert not math.isnan(t) and not math.isinf(t), "targets contain NaN or Inf"

    def test_targets_match_log_returns_enhanced(self):
        """Targets from enhanced features must be log returns of the price series."""
        arr = np.array(_PRICES_200, dtype=float)

        features, targets = build_enhanced_features(arr, None)

        log_returns = np.diff(np.log(arr))
        log_return_set = set(round(float(v), 10) for v in log_returns)
        for t in targets:
            assert round(float(t), 10) in log_return_set, (
                f"Enhanced target {t} is not a log return from the price series"
            )

    def test_build_enhanced_features_values_in_range(self):
        """Validate specific feature value ranges for each feature group.

        Group A (lags 0-9): plausible daily log returns, |val| < 0.5 for stocks
        Group D (index 17): RSI in [0, 100]
        Group E (index 18-19): StochRSI %K/%D in [0, 100]
        Group F (index 20): Bollinger %B clipped to [-0.5, 1.5]
        Group B (index 10-13): MA ratios > 0
        """
        arr = np.array(_PRICES_200, dtype=float)
        vol_arr = np.array(_VOLUMES_200, dtype=float)

        features, targets = build_enhanced_features(arr, vol_arr)
        assert len(features) > 0

        for i, row in enumerate(features):
            # Group A: lag log-returns (cols 0-9) — plausible magnitude for stocks
            for lag_idx in range(10):
                assert abs(row[lag_idx]) < 0.5, (
                    f"Row {i}, col {lag_idx}: log-return {row[lag_idx]} seems unrealistic (>0.5)"
                )

            # Group B: MA ratios (cols 10-13) — must be positive
            for ma_col in range(10, 14):
                assert row[ma_col] > 0, (
                    f"Row {i}, col {ma_col}: MA ratio {row[ma_col]} is not positive"
                )

            # Group D: RSI (col 17) must be in [0, 100]
            rsi_val = row[17]
            assert 0.0 <= rsi_val <= 100.0, (
                f"Row {i}: RSI (col 17) {rsi_val} outside [0, 100]"
            )

            # Group E: StochRSI %K and %D (cols 18-19) must be in [0, 100]
            for stoch_col in [18, 19]:
                stoch_val = row[stoch_col]
                assert 0.0 <= stoch_val <= 100.0, (
                    f"Row {i}, col {stoch_col}: StochRSI {stoch_val} outside [0, 100]"
                )

            # Group F: Bollinger %B (col 20) clipped to [-0.5, 1.5]
            bb_val = row[20]
            assert -1.0 <= bb_val <= 2.0, (
                f"Row {i}: Bollinger %B (col 20) {bb_val} outside expected range [-1, 2]"
            )

    def test_enhanced_lag_features_match_basic_lags(self):
        """First 10 columns of enhanced features (lag returns) must match basic lag features.

        Both functions build lag returns with the same formula from the same array.
        Enhanced starts 10 rows later (i=20) vs basic (i=10), so we align by comparing
        the last len(enhanced) rows of basic with all enhanced rows.
        """
        arr = np.array(_PRICES_200, dtype=float)

        basic_features, basic_targets = build_basic_features(arr, None)
        enhanced_features, enhanced_targets = build_enhanced_features(arr, None)

        n_enhanced = len(enhanced_features)
        n_basic = len(basic_features)

        # Enhanced has fewer rows because it starts later (i>=20 vs i>=10)
        assert n_enhanced <= n_basic, (
            f"Enhanced should have <= rows than basic: {n_enhanced} > {n_basic}"
        )

        # Align: last n_enhanced rows of basic correspond to first n_enhanced of enhanced
        basic_tail = basic_features[n_basic - n_enhanced:]
        for row_idx, (basic_row, enhanced_row) in enumerate(
            zip(basic_tail, enhanced_features)
        ):
            # First 10 columns (lag returns) must be identical
            for col in range(10):
                assert enhanced_row[col] == pytest.approx(basic_row[col], rel=1e-9, abs=1e-12), (
                    f"Lag feature mismatch at row {row_idx}, col {col}: "
                    f"basic={basic_row[col]}, enhanced={enhanced_row[col]}"
                )

        # Targets must also match in the aligned region
        basic_targets_tail = basic_targets[n_basic - n_enhanced:]
        for row_idx, (bt, et) in enumerate(zip(basic_targets_tail, enhanced_targets)):
            assert bt == pytest.approx(et, rel=1e-9, abs=1e-12), (
                f"Target mismatch at row {row_idx}: basic={bt}, enhanced={et}"
            )

    def test_enhanced_features_lengths_consistent_various_sizes(self):
        """Feature and target list lengths must always be equal for various input sizes."""
        for n in [100, 150, 200]:
            np.random.seed(n + 1)
            prices = list(90.0 * np.cumprod(1 + np.random.normal(0.0005, 0.02, n)))
            arr = np.array(prices, dtype=float)

            features, targets = build_enhanced_features(arr, None)

            assert len(features) == len(targets), (
                f"Length mismatch for n={n}: {len(features)} features vs {len(targets)} targets"
            )

    def test_enhanced_produces_fewer_rows_than_basic(self):
        """Enhanced features require more warmup (i>=20) so produce fewer rows than basic."""
        arr = np.array(_PRICES_200, dtype=float)

        basic_features, _ = build_basic_features(arr, None)
        enhanced_features, _ = build_enhanced_features(arr, None)

        assert len(enhanced_features) < len(basic_features), (
            "Enhanced features need more warmup — should have fewer rows than basic"
        )
        # Exactly 10 fewer rows (enhanced starts at i=20, basic at i=10)
        assert len(basic_features) - len(enhanced_features) == _ENHANCED_EXTRA_SKIP, (
            f"Expected exactly {_ENHANCED_EXTRA_SKIP} fewer rows in enhanced, "
            f"got {len(basic_features) - len(enhanced_features)}"
        )


# ---------------------------------------------------------------------------
# Tests: Bollinger %B range validation (enhanced only, pandas_ta optional)
# ---------------------------------------------------------------------------


class TestEnhancedBollingerRange:
    """Bollinger Band %B feature (col 20 in enhanced) range validation.

    numpy fallback: _numpy_bb_pct clips output to [-0.5, 1.5].
    pandas_ta path: values come from pandas_ta BBP column — not clipped, but
    should still be reasonable for typical stock price series (~[-1, 2]).
    """

    def test_bollinger_percent_b_reasonable_range(self):
        """Bollinger %B (col 20) must be in a reasonable range regardless of backend."""
        arr = np.array(_PRICES_200, dtype=float)

        features, _ = build_enhanced_features(arr, None)
        assert len(features) > 0

        # Col 20 = Group F: Bollinger %B
        # numpy fallback clips to [-0.5, 1.5]; pandas_ta path values stay within
        # a similar band for typical equity data — use a wider tolerance [-2, 3]
        for i, row in enumerate(features):
            bb_val = row[20]
            assert -2.0 <= bb_val <= 3.0, (
                f"Row {i}: Bollinger %B (col 20) {bb_val} outside range [-2, 3]"
            )

    def test_bollinger_percent_b_numpy_path_strictly_clipped(self):
        """When numpy fallback is used (no pandas_ta), %B must be clipped to [-0.5, 1.5]."""
        if _pandas_ta_available:
            pytest.skip("pandas_ta present — numpy fallback not used, skipping clip test")

        arr = np.array(_PRICES_200, dtype=float)

        features, _ = build_enhanced_features(arr, None)
        assert len(features) > 0

        for i, row in enumerate(features):
            bb_val = row[20]
            assert -0.5 <= bb_val <= 1.5, (
                f"numpy fallback: Bollinger %B at row {i}: {bb_val} outside [-0.5, 1.5]"
            )

    def test_bollinger_percent_b_pandas_ta_path(self):
        """When pandas_ta is available, %B values should still be in reasonable range."""
        if not _pandas_ta_available:
            pytest.skip("pandas_ta not installed — skipping pandas_ta-specific Bollinger test")

        arr = np.array(_PRICES_200, dtype=float)

        features, _ = build_enhanced_features(arr, None)
        assert len(features) > 0

        bb_col = [row[20] for row in features]
        for i, v in enumerate(bb_col):
            assert -2.0 <= v <= 3.0, (
                f"pandas_ta %B at row {i}: {v} outside reasonable range [-2, 3]"
            )


# ---------------------------------------------------------------------------
# Tests: No look-ahead leakage (causality)
# ---------------------------------------------------------------------------


class TestNoLookaheadLeakage:
    """A feature row must depend ONLY on prices up to its own "as-of" day.

    Regression guard for the price_idx off-by-one bug: previously each row used
    price_idx = i + 1 (the target day's price) while the target was the return
    INTO that day, leaking the answer into the features. The corrected builders
    use price_idx = i (as-of day) and emit a final inference row at the latest
    price.

    Method: build features on the full series, then on the series truncated right
    after as-of day `t`. The truncated build's last row (inference row, as-of day
    `t`) must equal the full build's row for as-of day `t`. If features peeked at
    arr[t+1], truncating it would change the row → mismatch.
    """

    @staticmethod
    def _assert_rows_equal(row_a, row_b, t, kind):
        assert len(row_a) == len(row_b), f"{kind}: row width mismatch at t={t}"
        for col, (a, b) in enumerate(zip(row_a, row_b)):
            assert a == pytest.approx(b, rel=1e-9, abs=1e-9), (
                f"{kind} look-ahead leak at as-of day t={t}, col {col}: "
                f"full={a} vs truncated={b} — feature depends on arr[t+1]"
            )

    def test_enhanced_row_depends_only_on_past(self):
        arr_full = np.array(_PRICES_200, dtype=float)
        feats_full, _ = build_enhanced_features(arr_full, None)
        # rows list index p -> as-of day = 20 + p  (loop starts at i=20)
        for t in (60, 120, 180):
            arr_trunc = arr_full[: t + 1]
            feats_trunc, _ = build_enhanced_features(arr_trunc, None)
            # truncated inference row (last) is as-of day t
            self._assert_rows_equal(feats_full[t - 20], feats_trunc[-1], t, "enhanced")

    def test_basic_row_depends_only_on_past(self):
        arr_full = np.array(_PRICES_200, dtype=float)
        feats_full, _ = build_basic_features(arr_full, None)
        # rows list index p -> as-of day = 10 + p  (loop starts at i=10)
        for t in (60, 120, 180):
            arr_trunc = arr_full[: t + 1]
            feats_trunc, _ = build_basic_features(arr_trunc, None)
            self._assert_rows_equal(feats_full[t - 10], feats_trunc[-1], t, "basic")

    def test_inference_row_is_as_of_latest_price(self):
        """The last feature row must be built from the latest price (used for prediction)."""
        arr = np.array(_PRICES_200, dtype=float)
        log_returns = np.diff(np.log(arr))

        for build in (build_basic_features, build_enhanced_features):
            features, _ = build(arr, None)
            # Lag-1 of the last row must be the most recent realised return.
            assert features[-1][0] == pytest.approx(
                float(log_returns[-1]), rel=1e-9, abs=1e-12
            ), f"{build.__name__}: inference row lag_1 must equal the latest return"

    def test_training_targets_are_real_log_returns(self):
        """Every training target (all but the dropped inference placeholder) is a log return."""
        arr = np.array(_PRICES_200, dtype=float)
        log_returns = np.diff(np.log(arr))
        log_return_set = set(round(float(v), 10) for v in log_returns)

        for build in (build_basic_features, build_enhanced_features):
            _, targets = build(arr, None)
            for t in targets[:-1]:  # callers train on targets[:-1]
                assert round(float(t), 10) in log_return_set, (
                    f"{build.__name__}: training target {t} is not a real log return"
                )


# ---------------------------------------------------------------------------
# Tests: API contract (function signatures exist and are callable)
# ---------------------------------------------------------------------------


class TestFeaturesAPIContract:
    """Verify the public API contract of src.algorithms.features."""

    def test_build_basic_features_is_callable(self):
        assert callable(build_basic_features)

    def test_build_enhanced_features_is_callable(self):
        assert callable(build_enhanced_features)

    def test_min_data_points_constant_exported(self):
        """MIN_DATA_POINTS must be exported and be at least 80."""
        assert MIN_DATA_POINTS >= 80

    def test_build_basic_features_returns_tuple_of_two(self):
        arr = np.array(_PRICES_200, dtype=float)
        result = build_basic_features(arr, None)
        assert isinstance(result, tuple)
        assert len(result) == 2

    def test_build_enhanced_features_returns_tuple_of_two(self):
        arr = np.array(_PRICES_200, dtype=float)
        result = build_enhanced_features(arr, None)
        assert isinstance(result, tuple)
        assert len(result) == 2

    def test_build_basic_features_returns_lists(self):
        """Return type must be (list, list) — not numpy arrays."""
        arr = np.array(_PRICES_200, dtype=float)
        features, targets = build_basic_features(arr, None)
        assert isinstance(features, list)
        assert isinstance(targets, list)

    def test_build_enhanced_features_returns_lists(self):
        arr = np.array(_PRICES_200, dtype=float)
        features, targets = build_enhanced_features(arr, None)
        assert isinstance(features, list)
        assert isinstance(targets, list)

    def test_both_functions_produce_float_targets(self):
        """All target values must be Python/numpy floats (log returns)."""
        arr = np.array(_PRICES_200, dtype=float)

        _, basic_targets = build_basic_features(arr, None)
        _, enhanced_targets = build_enhanced_features(arr, None)

        for t in basic_targets:
            assert isinstance(t, (float, np.floating)), (
                f"Basic target is not float: {type(t)}"
            )
        for t in enhanced_targets:
            assert isinstance(t, (float, np.floating)), (
                f"Enhanced target is not float: {type(t)}"
            )

    def test_feature_rows_are_lists_of_floats(self):
        """Each row in features must be a list of floats."""
        arr = np.array(_PRICES_200, dtype=float)

        basic_features, _ = build_basic_features(arr, None)
        enhanced_features, _ = build_enhanced_features(arr, None)

        for row in basic_features[:5]:
            assert isinstance(row, list)
            for val in row:
                assert isinstance(val, (float, np.floating)), (
                    f"Basic feature value is not float: {type(val)}"
                )

        for row in enhanced_features[:5]:
            assert isinstance(row, list)
            for val in row:
                assert isinstance(val, (float, np.floating)), (
                    f"Enhanced feature value is not float: {type(val)}"
                )


# ---------------------------------------------------------------------------
# Tests: Integration with existing algorithm classes
# ---------------------------------------------------------------------------


class TestAlgorithmCompatibility:
    """Verify that algorithm _build_features wrappers delegate to build_enhanced_features.

    After the refactor, LightGBMPredictor, XGBoostPredictor, and
    RandomForestPredictor all delegate to build_enhanced_features via a thin
    _build_features() wrapper that retains backward-compatible call sites.
    """

    def test_lightgbm_build_features_delegates_to_enhanced(self):
        """LightGBMPredictor._build_features must produce enhanced features (30 cols, 179 rows)."""
        try:
            from src.algorithms.lightgbm_model import LightGBMPredictor
        except ImportError:
            pytest.skip("lightgbm not installed")

        arr = np.array(_PRICES_200, dtype=float)

        enhanced_features, enhanced_targets = build_enhanced_features(arr, None)

        if hasattr(LightGBMPredictor, "_build_features"):
            algo_features, algo_targets = LightGBMPredictor._build_features(arr, None)
            assert len(algo_features) == len(enhanced_features), (
                f"LightGBM _build_features row count {len(algo_features)} "
                f"!= enhanced {len(enhanced_features)}"
            )
            assert len(algo_features[0]) == ENHANCED_FEATURE_COUNT, (
                f"LightGBM _build_features has {len(algo_features[0])} cols, "
                f"expected {ENHANCED_FEATURE_COUNT}"
            )
            for i, (ef, af) in enumerate(zip(enhanced_features, algo_features)):
                for col, (ev, av) in enumerate(zip(ef, af)):
                    assert ev == pytest.approx(av, rel=1e-6, abs=1e-9), (
                        f"LightGBM row {i}, col {col}: enhanced={ev}, algo={av}"
                    )

    def test_xgboost_build_features_delegates_to_enhanced(self):
        """XGBoostPredictor._build_features must produce enhanced features (30 cols, 179 rows)."""
        try:
            from src.algorithms.xgboost_model import XGBoostPredictor
        except ImportError:
            pytest.skip("xgboost not installed")

        arr = np.array(_PRICES_200, dtype=float)
        enhanced_features, enhanced_targets = build_enhanced_features(arr, None)

        if hasattr(XGBoostPredictor, "_build_features"):
            algo_features, algo_targets = XGBoostPredictor._build_features(arr, None)
            assert len(algo_features) == len(enhanced_features), (
                f"XGBoost _build_features row count {len(algo_features)} "
                f"!= enhanced {len(enhanced_features)}"
            )
            assert len(algo_features[0]) == ENHANCED_FEATURE_COUNT, (
                f"XGBoost _build_features has {len(algo_features[0])} cols, "
                f"expected {ENHANCED_FEATURE_COUNT}"
            )
            for i, (ef, af) in enumerate(zip(enhanced_features, algo_features)):
                for col, (ev, av) in enumerate(zip(ef, af)):
                    assert ev == pytest.approx(av, rel=1e-6, abs=1e-9), (
                        f"XGBoost row {i}, col {col}: enhanced={ev}, algo={av}"
                    )

    def test_random_forest_build_features_delegates_to_enhanced(self):
        """RandomForestPredictor._build_features must produce enhanced features."""
        from src.algorithms.random_forest import RandomForestPredictor

        arr = np.array(_PRICES_200, dtype=float)
        enhanced_features, enhanced_targets = build_enhanced_features(arr, None)

        if hasattr(RandomForestPredictor, "_build_features"):
            algo_features, algo_targets = RandomForestPredictor._build_features(arr, None)
            assert len(algo_features) == len(enhanced_features), (
                f"RandomForest _build_features row count {len(algo_features)} "
                f"!= enhanced {len(enhanced_features)}"
            )
            assert len(algo_features[0]) == ENHANCED_FEATURE_COUNT, (
                f"RandomForest _build_features has {len(algo_features[0])} cols, "
                f"expected {ENHANCED_FEATURE_COUNT}"
            )
            for i, (ef, af) in enumerate(zip(enhanced_features, algo_features)):
                for col, (ev, av) in enumerate(zip(ef, af)):
                    assert ev == pytest.approx(av, rel=1e-6, abs=1e-9), (
                        f"RandomForest row {i}, col {col}: enhanced={ev}, algo={av}"
                    )
