"""Unit tests for orchestrator/rebuild.py.

No DB, no network, no Docker required — all external calls are monkeypatched.
Covers:
  1. _compute_direction_correct pure-logic edge cases.
  2. rebuild_and_replay integration with all DB/engine calls mocked — verifies
     the returned stats dict structure, cutoff/predict_from filtering behaviour,
     that direction_correct is set on every saved record, and that
     train_meta_all receives max_train_date=cutoff.
  3. Cutoff/predict_from filter correctness using a tiny synthetic price series.
  4. predict_from_date defaulting and clamping logic.
"""
from __future__ import annotations

from datetime import date, timedelta
from decimal import Decimal
from typing import Any
from unittest.mock import MagicMock, call, patch

import pytest

from src.orchestrator.rebuild import _compute_direction_correct


# ---------------------------------------------------------------------------
# 1. _compute_direction_correct — pure logic
# ---------------------------------------------------------------------------

class TestComputeDirectionCorrect:

    def test_up_prediction_matches_up_movement(self):
        predicted = Decimal("105.00")
        current   = Decimal("100.00")
        actual    = Decimal("103.00")
        assert _compute_direction_correct(predicted, current, actual) is True

    def test_up_prediction_wrong_on_down_movement(self):
        predicted = Decimal("105.00")
        current   = Decimal("100.00")
        actual    = Decimal("97.00")
        assert _compute_direction_correct(predicted, current, actual) is False

    def test_down_prediction_matches_down_movement(self):
        predicted = Decimal("95.00")
        current   = Decimal("100.00")
        actual    = Decimal("98.00")
        assert _compute_direction_correct(predicted, current, actual) is True

    def test_down_prediction_wrong_on_up_movement(self):
        predicted = Decimal("95.00")
        current   = Decimal("100.00")
        actual    = Decimal("102.00")
        assert _compute_direction_correct(predicted, current, actual) is False

    def test_flat_actual_flat_prediction_returns_true(self):
        """When actual == current AND predicted == current: both flat -> correct."""
        v = Decimal("100.00")
        assert _compute_direction_correct(v, v, v) is True

    def test_flat_actual_non_flat_prediction_returns_false(self):
        """When actual == current but predicted differs: wrong (can't be right about direction)."""
        current   = Decimal("100.00")
        predicted = Decimal("102.00")
        actual    = Decimal("100.00")
        assert _compute_direction_correct(predicted, current, actual) is False

    def test_predicted_flat_actual_up_returns_false(self):
        """Predicted no-change but price actually rose -> wrong direction."""
        current   = Decimal("100.00")
        predicted = Decimal("100.00")
        actual    = Decimal("103.00")
        assert _compute_direction_correct(predicted, current, actual) is False

    def test_both_move_same_large_magnitude(self):
        predicted = Decimal("150.00")
        current   = Decimal("100.00")
        actual    = Decimal("120.00")
        assert _compute_direction_correct(predicted, current, actual) is True

    @pytest.mark.parametrize("predicted,current,actual,expected", [
        (Decimal("101"), Decimal("100"), Decimal("101"), True),
        (Decimal("101"), Decimal("100"), Decimal("99"),  False),
        (Decimal("99"),  Decimal("100"), Decimal("99"),  True),
        (Decimal("99"),  Decimal("100"), Decimal("101"), False),
        (Decimal("100"), Decimal("100"), Decimal("100"), True),
        (Decimal("100"), Decimal("100"), Decimal("101"), False),
    ])
    def test_parametrized(self, predicted, current, actual, expected):
        assert _compute_direction_correct(predicted, current, actual) == expected


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_price_row(price: float, d: date):
    """Minimal row object suitable for both GoldPrice and NasdaqPrice shapes."""
    obj = MagicMock()
    obj.buy_price   = price   # gold
    obj.close_price = price   # nasdaq/crypto/sp500
    obj.trading_date = d
    return obj


def _build_price_series(n: int = 50, start: date = date(2025, 1, 1)):
    """Build n synthetic price rows (ascending dates, monotone price)."""
    rows = []
    for i in range(n):
        rows.append(_make_price_row(100.0 + i * 0.5, start + timedelta(days=i)))
    return rows


# ---------------------------------------------------------------------------
# 2. rebuild_and_replay — integration-level with all DB mocked
# ---------------------------------------------------------------------------

class TestRebuildAndReplay:
    """Monkeypatches all external calls; only tests the orchestration logic."""

    def _make_algo(self, key: str = "mock_algo", price: float = 101.0):
        """Return a MagicMock that implements the PredictionAlgorithm interface."""
        from src.algorithms.base import PredictionResult
        algo = MagicMock()
        algo.get_key.return_value = key
        result = PredictionResult(
            algorithm_name=key,
            predicted_price=price,
            current_price=100.0,
            confidence=0.75,
        )
        algo.predict.return_value = result
        return algo

    def _patch_all(self, monkeypatch, price_rows, cutoff: date):
        """Apply all required monkeypatches for a single rebuild_and_replay call."""
        from src.orchestrator import rebuild as rb

        # build_algorithms -> one algo, GOLD market
        algo = self._make_algo()
        monkeypatch.setattr(rb, "build_algorithms", lambda: {"mock_algo": algo})

        # repo calls -- return our synthetic price series
        monkeypatch.setattr(rb.repo, "get_gold_prices_asc",   lambda *a, **kw: price_rows)
        monkeypatch.setattr(rb.repo, "get_nasdaq_prices_asc", lambda *a, **kw: price_rows)
        monkeypatch.setattr(rb.repo, "get_nasdaq_symbols",    lambda: ["AAPL"])
        monkeypatch.setattr(rb.repo, "get_crypto_prices_asc", lambda *a, **kw: price_rows)
        monkeypatch.setattr(rb.repo, "get_sp500_prices_asc",  lambda *a, **kw: price_rows)
        monkeypatch.setattr(rb.repo, "get_sp500_symbols",     lambda: ["SPY"])

        # Bulk insert functions -- capture written records
        saved: dict[str, list] = {
            "gold": [], "nasdaq": [], "crypto": [], "sp500": []
        }

        def capture_gold(batch):
            saved["gold"].extend(batch)

        def capture_nasdaq(batch):
            saved["nasdaq"].extend(batch)

        def capture_crypto(batch):
            saved["crypto"].extend(batch)

        def capture_sp500(batch):
            saved["sp500"].extend(batch)

        monkeypatch.setattr(rb, "_bulk_gold_predictions",   capture_gold)
        monkeypatch.setattr(rb, "_bulk_nasdaq_predictions", capture_nasdaq)
        monkeypatch.setattr(rb, "_bulk_crypto_predictions", capture_crypto)
        monkeypatch.setattr(rb, "_bulk_sp500_predictions",  capture_sp500)

        # Wipe, meta, sim -- side-effect free
        monkeypatch.setattr(rb, "_wipe_predictions_and_sim", lambda: None)
        monkeypatch.setattr(rb, "train_meta_all",            lambda **kw: None)

        # Patch the thin adapter so we never touch SimulationEngine internals
        sim_engine = MagicMock()
        monkeypatch.setattr(rb, "_run_sim_backtest", lambda s, e: 7)

        return saved, algo, sim_engine

    # --- basic contract ---

    def test_returns_expected_keys(self, monkeypatch):
        from src.orchestrator.rebuild import rebuild_and_replay

        price_rows = _build_price_series(50)
        cutoff = price_rows[35].trading_date   # keep rows after index 35
        saved, algo, sim_engine = self._patch_all(monkeypatch, price_rows, cutoff)

        result = rebuild_and_replay(cutoff, step_size=1)

        assert set(result.keys()) == {
            "predictions_generated", "markets", "meta_trained",
            "bots_replayed", "cutoff_date", "predict_from_date", "duration_ms",
        }
        assert result["markets"] == ["GOLD", "NASDAQ100", "CRYPTO", "SP500"]
        assert result["meta_trained"] is True
        assert result["bots_replayed"] == 7
        assert result["cutoff_date"] == str(cutoff)
        assert isinstance(result["duration_ms"], int)
        assert result["duration_ms"] >= 0

    def test_predict_from_date_default_is_cutoff_minus_30(self, monkeypatch):
        """When predict_from_date is None, default = cutoff - 30 days."""
        from src.orchestrator.rebuild import rebuild_and_replay

        price_rows = _build_price_series(70)
        cutoff = price_rows[50].trading_date
        expected_predict_from = cutoff - timedelta(days=30)
        self._patch_all(monkeypatch, price_rows, cutoff)

        result = rebuild_and_replay(cutoff, step_size=1)
        assert result["predict_from_date"] == str(expected_predict_from)

    def test_predict_from_date_explicit(self, monkeypatch):
        """Explicit predict_from_date is returned as-is in the result."""
        from src.orchestrator.rebuild import rebuild_and_replay

        price_rows = _build_price_series(70)
        cutoff = price_rows[50].trading_date
        predict_from = price_rows[20].trading_date
        self._patch_all(monkeypatch, price_rows, cutoff)

        result = rebuild_and_replay(cutoff, predict_from_date=predict_from, step_size=1)
        assert result["predict_from_date"] == str(predict_from)

    def test_predict_from_date_clamped_to_cutoff_when_beyond(self, monkeypatch):
        """predict_from_date > cutoff is clamped to cutoff."""
        from src.orchestrator.rebuild import rebuild_and_replay

        price_rows = _build_price_series(70)
        cutoff = price_rows[40].trading_date
        predict_from_beyond = cutoff + timedelta(days=10)  # beyond cutoff
        self._patch_all(monkeypatch, price_rows, cutoff)

        result = rebuild_and_replay(
            cutoff, predict_from_date=predict_from_beyond, step_size=1
        )
        # Clamped to cutoff
        assert result["predict_from_date"] == str(cutoff)

    def test_step_size_zero_defaults_to_three(self, monkeypatch):
        from src.orchestrator.rebuild import rebuild_and_replay

        price_rows = _build_price_series(40)
        cutoff = price_rows[30].trading_date
        self._patch_all(monkeypatch, price_rows, cutoff)

        # step_size=0 -> internally defaults to 3; must not raise
        result = rebuild_and_replay(cutoff, step_size=0)
        assert result["predictions_generated"] >= 0

    def test_meta_trained_false_on_exception(self, monkeypatch):
        from src.orchestrator import rebuild as rb
        from src.orchestrator.rebuild import rebuild_and_replay

        price_rows = _build_price_series(40)
        cutoff = price_rows[30].trading_date
        self._patch_all(monkeypatch, price_rows, cutoff)

        # Force train_meta_all to raise
        monkeypatch.setattr(rb, "train_meta_all", MagicMock(side_effect=RuntimeError("meta fail")))

        result = rebuild_and_replay(cutoff, step_size=1)
        assert result["meta_trained"] is False
        assert result["predictions_generated"] >= 0   # rest still ran

    def test_wipe_is_called(self, monkeypatch):
        from src.orchestrator import rebuild as rb
        from src.orchestrator.rebuild import rebuild_and_replay

        price_rows = _build_price_series(40)
        cutoff = price_rows[30].trading_date
        self._patch_all(monkeypatch, price_rows, cutoff)

        wipe_mock = MagicMock()
        monkeypatch.setattr(rb, "_wipe_predictions_and_sim", wipe_mock)

        rebuild_and_replay(cutoff, step_size=1)
        wipe_mock.assert_called_once()

    def test_sim_engine_backtest_called_with_cutoff_plus_one(self, monkeypatch):
        from src.orchestrator import rebuild as rb
        from src.orchestrator.rebuild import rebuild_and_replay

        price_rows = _build_price_series(40)
        cutoff = price_rows[30].trading_date
        self._patch_all(monkeypatch, price_rows, cutoff)

        # Override with a capturing stub to verify the date arguments
        call_args: list = []
        monkeypatch.setattr(rb, "_run_sim_backtest", lambda s, e: call_args.append((s, e)) or 3)

        rebuild_and_replay(cutoff, step_size=1)
        assert len(call_args) == 1
        sim_start, sim_end = call_args[0]
        assert sim_start == cutoff + timedelta(days=1)
        assert sim_end == date.today()

    def test_train_meta_all_receives_max_train_date(self, monkeypatch):
        """train_meta_all must be called with max_train_date=cutoff_date."""
        from src.orchestrator import rebuild as rb
        from src.orchestrator.rebuild import rebuild_and_replay

        price_rows = _build_price_series(50)
        cutoff = price_rows[35].trading_date
        self._patch_all(monkeypatch, price_rows, cutoff)

        meta_calls: list[dict] = []
        monkeypatch.setattr(rb, "train_meta_all", lambda **kw: meta_calls.append(kw))

        rebuild_and_replay(cutoff, step_size=1)
        assert len(meta_calls) == 1
        assert meta_calls[0].get("max_train_date") == cutoff


# ---------------------------------------------------------------------------
# 3. Cutoff / predict_from filter correctness — synthetic price series
# ---------------------------------------------------------------------------

class TestPredictFromFilter:
    """Verify that predictions are saved only for target_date >= predict_from."""

    def test_gold_predictions_only_from_predict_from(self, monkeypatch):
        """
        Price series of 50 rows; predict_from = row[40].trading_date.
        With step_size=1 and train_window=30: window_end from 30..49.
        For window_end=30..39: dates[window_end] < predict_from -> skip.
        For window_end=40..49: dates[window_end] >= predict_from -> save.
        Expect 10 predictions (one algo, one instrument).
        """
        from src.orchestrator.rebuild import _rebuild_backtest_gold
        from src.algorithms.base import PredictionResult
        from src.orchestrator import rebuild as rb

        N = 50
        start = date(2025, 1, 1)
        price_rows = _build_price_series(N, start)
        predict_from = price_rows[40].trading_date
        cutoff = price_rows[49].trading_date  # cutoff = last row (not used by filter now)

        algo = MagicMock()
        result = PredictionResult(
            algorithm_name="ma",
            predicted_price=price_rows[-1].buy_price,
            current_price=price_rows[-1].buy_price,
            confidence=0.8,
        )
        algo.predict.return_value = result
        algos = {"ma": algo}

        monkeypatch.setattr(rb, "GOLD_INSTRUMENTS", [("XAU", "spot")])
        monkeypatch.setattr(rb.repo, "get_gold_prices_asc", lambda *a, **kw: price_rows)

        saved_batches: list[dict] = []
        monkeypatch.setattr(rb, "_bulk_gold_predictions", lambda b: saved_batches.extend(b))

        count = _rebuild_backtest_gold(algos, step_size=1, cutoff=cutoff, predict_from=predict_from)

        assert count == len(saved_batches)

        # Every saved record must have target_date >= predict_from
        for rec in saved_batches:
            target_d = rec["target_date"]
            if not isinstance(target_d, date):
                target_d = target_d.date()
            assert target_d >= predict_from, (
                f"found target_date {target_d} < predict_from {predict_from}"
            )

        # window_end=40..49 -> 10 predictions (step_size=1, fold=[t] for each)
        assert count == 10

    def test_gold_saves_meta_train_window_predictions(self, monkeypatch):
        """Predictions in [predict_from..cutoff] must also be saved (meta-train data)."""
        from src.orchestrator.rebuild import _rebuild_backtest_gold
        from src.algorithms.base import PredictionResult
        from src.orchestrator import rebuild as rb

        N = 50
        start = date(2025, 1, 1)
        price_rows = _build_price_series(N, start)
        predict_from = price_rows[30].trading_date  # start saving right at train_window boundary
        cutoff = price_rows[40].trading_date        # meta-train: 30..40; bot-trade: 41..49

        algo = MagicMock()
        algo.predict.return_value = MagicMock(predicted_price=100.0, confidence=0.7)
        algos = {"ma": algo}

        monkeypatch.setattr(rb, "GOLD_INSTRUMENTS", [("XAU", "spot")])
        monkeypatch.setattr(rb.repo, "get_gold_prices_asc", lambda *a, **kw: price_rows)

        saved_batches: list[dict] = []
        monkeypatch.setattr(rb, "_bulk_gold_predictions", lambda b: saved_batches.extend(b))

        count = _rebuild_backtest_gold(algos, step_size=1, cutoff=cutoff, predict_from=predict_from)

        assert count > 0, "expected predictions in meta-train window to be saved"

        # Rows with target_date <= cutoff must be present (meta-train material)
        meta_train_rows = [
            r for r in saved_batches
            if (r["target_date"] if isinstance(r["target_date"], date)
                else r["target_date"].date()) <= cutoff
        ]
        assert len(meta_train_rows) > 0, (
            "no meta-train-window predictions saved; meta classifier will be data-starved"
        )

    def test_direction_correct_set_on_all_saved_records(self, monkeypatch):
        """Every saved record must have direction_correct as a bool (not None by default)."""
        from src.orchestrator.rebuild import _rebuild_backtest_nasdaq
        from src.algorithms.base import PredictionResult
        from src.orchestrator import rebuild as rb

        N = 40
        start = date(2025, 3, 1)
        price_rows = _build_price_series(N, start)
        predict_from = price_rows[30].trading_date  # save last 10 rows
        cutoff = price_rows[39].trading_date

        algo = MagicMock()
        # Predict slightly above current -> direction UP; actual is also UP -> True
        algo.predict.side_effect = lambda prices: PredictionResult(
            algorithm_name="ema",
            predicted_price=prices[-1] + 1.0,
            current_price=prices[-1],
            confidence=0.7,
        )
        algos = {"ema": algo}

        monkeypatch.setattr(rb.repo, "get_nasdaq_symbols",    lambda: ["MSFT"])
        monkeypatch.setattr(rb.repo, "get_nasdaq_prices_asc", lambda *a, **kw: price_rows)

        saved_batches: list[dict] = []
        monkeypatch.setattr(rb, "_bulk_nasdaq_predictions", lambda b: saved_batches.extend(b))

        count = _rebuild_backtest_nasdaq(algos, step_size=1, cutoff=cutoff, predict_from=predict_from)

        assert count == len(saved_batches) > 0
        for rec in saved_batches:
            assert "direction_correct" in rec, "direction_correct missing from saved record"
            assert isinstance(rec["direction_correct"], bool), (
                f"direction_correct must be bool, got {type(rec['direction_correct'])}"
            )

    def test_no_predictions_when_predict_from_after_all_dates(self, monkeypatch):
        """When predict_from is after all price dates, nothing is saved."""
        from src.orchestrator.rebuild import _rebuild_backtest_gold
        from src.algorithms.base import PredictionResult
        from src.orchestrator import rebuild as rb

        N = 45
        start = date(2025, 1, 1)
        price_rows = _build_price_series(N, start)
        # predict_from after the very last date
        predict_from = price_rows[-1].trading_date + timedelta(days=1)
        cutoff = predict_from  # doesn't matter

        algo = MagicMock()
        algo.predict.return_value = PredictionResult(
            algorithm_name="ma",
            predicted_price=100.0,
            current_price=100.0,
            confidence=0.7,
        )
        algos = {"ma": algo}

        monkeypatch.setattr(rb, "GOLD_INSTRUMENTS", [("XAU", "spot")])
        monkeypatch.setattr(rb.repo, "get_gold_prices_asc", lambda *a, **kw: price_rows)

        saved_batches: list[dict] = []
        monkeypatch.setattr(rb, "_bulk_gold_predictions", lambda b: saved_batches.extend(b))

        count = _rebuild_backtest_gold(algos, step_size=1, cutoff=cutoff, predict_from=predict_from)
        assert count == 0
        assert saved_batches == []

    def test_all_predictions_when_predict_from_before_series_start(self, monkeypatch):
        """When predict_from is before all target dates, every fold is saved."""
        from src.orchestrator.rebuild import _rebuild_backtest_gold
        from src.algorithms.base import PredictionResult
        from src.orchestrator import rebuild as rb

        N = 35
        start = date(2025, 5, 1)
        price_rows = _build_price_series(N, start)
        predict_from = date(2020, 1, 1)  # before the series even starts
        cutoff = date(2020, 1, 1)        # doesn't affect filter

        algo = MagicMock()
        algo.predict.return_value = PredictionResult(
            algorithm_name="ma",
            predicted_price=100.0,
            current_price=100.0,
            confidence=0.7,
        )
        algos = {"ma": algo}

        monkeypatch.setattr(rb, "GOLD_INSTRUMENTS", [("XAU", "spot")])
        monkeypatch.setattr(rb.repo, "get_gold_prices_asc", lambda *a, **kw: price_rows)

        saved_batches: list[dict] = []
        monkeypatch.setattr(rb, "_bulk_gold_predictions", lambda b: saved_batches.extend(b))

        count = _rebuild_backtest_gold(algos, step_size=1, cutoff=cutoff, predict_from=predict_from)
        # With N=35 and train_window=30, valid t values are 30..34 -> 5 rows
        assert count == 5
        assert len(saved_batches) == 5

    def test_sp500_uses_predict_from_filter(self, monkeypatch):
        """SP500 predictions must also respect predict_from, not the old cutoff-only filter."""
        from src.orchestrator.rebuild import _rebuild_backtest_sp500
        from src.algorithms.base import PredictionResult
        from src.orchestrator import rebuild as rb

        N = 50
        start = date(2025, 4, 1)
        price_rows = _build_price_series(N, start)
        predict_from = price_rows[42].trading_date
        cutoff = price_rows[49].trading_date

        algo = MagicMock()
        algo.predict.return_value = PredictionResult(
            algorithm_name="ma",
            predicted_price=100.0,
            current_price=100.0,
            confidence=0.7,
        )
        algos = {"ma": algo}

        monkeypatch.setattr(rb.repo, "get_sp500_symbols",    lambda: ["SPY"])
        monkeypatch.setattr(rb.repo, "get_sp500_prices_asc", lambda *a, **kw: price_rows)

        saved_batches: list[dict] = []
        monkeypatch.setattr(rb, "_bulk_sp500_predictions", lambda b: saved_batches.extend(b))

        count = _rebuild_backtest_sp500(
            algos, step_size=1, cutoff=cutoff, predict_from=predict_from
        )

        assert count == len(saved_batches)
        for rec in saved_batches:
            target_d = rec["target_date"]
            if not isinstance(target_d, date):
                target_d = target_d.date()
            assert target_d >= predict_from, (
                f"SP500 target_date {target_d} < predict_from {predict_from}"
            )
        # window_end=42..49 -> 8 predictions
        assert count == 8
