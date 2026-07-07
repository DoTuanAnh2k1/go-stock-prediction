"""Unit tests for src/crawlers/sanity.py — no DB, no Docker required."""
from __future__ import annotations

import os

import pytest

from src.crawlers import sanity


# ---------------------------------------------------------------------------
# Fixture: reset module-level _pending state between tests to prevent leakage.
# ---------------------------------------------------------------------------

@pytest.fixture(autouse=True)
def clear_pending():
    """Wipe the pending-confirmation buffer before each test."""
    sanity._pending.clear()
    yield
    sanity._pending.clear()


# ===========================================================================
# max_deviation
# ===========================================================================

class TestMaxDeviation:
    def test_known_markets(self):
        assert sanity.max_deviation("GOLD") == pytest.approx(0.30)
        assert sanity.max_deviation("NASDAQ") == pytest.approx(0.40)
        assert sanity.max_deviation("NASDAQ100") == pytest.approx(0.40)
        assert sanity.max_deviation("SP500") == pytest.approx(0.40)
        assert sanity.max_deviation("CRYPTO") == pytest.approx(0.80)

    def test_unknown_market_defaults(self):
        assert sanity.max_deviation("FOREX") == pytest.approx(0.40)
        assert sanity.max_deviation("") == pytest.approx(0.40)

    def test_case_insensitive(self):
        assert sanity.max_deviation("gold") == sanity.max_deviation("GOLD")
        assert sanity.max_deviation("crypto") == sanity.max_deviation("CRYPTO")

    def test_env_override(self, monkeypatch):
        monkeypatch.setenv("CRAWL_MAX_TICK_DEVIATION_NASDAQ", "0.55")
        assert sanity.max_deviation("NASDAQ") == pytest.approx(0.55)

    def test_bad_env_falls_back_to_default(self, monkeypatch):
        monkeypatch.setenv("CRAWL_MAX_TICK_DEVIATION_GOLD", "not_a_float")
        assert sanity.max_deviation("GOLD") == pytest.approx(0.30)


# ===========================================================================
# check_update — daily-live gate
# ===========================================================================

class TestCheckUpdate:
    """Tests for sanity.check_update(market, key, new_price, last_price)."""

    # -----------------------------------------------------------------------
    # Nonpositive price is always rejected immediately.
    # -----------------------------------------------------------------------

    def test_nonpositive_zero(self):
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:CRWD", 0.0, 194.0)
        assert accept is False
        assert reason == "nonpositive"

    def test_nonpositive_negative(self):
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:AAPL", -5.0, 100.0)
        assert accept is False
        assert reason == "nonpositive"

    # -----------------------------------------------------------------------
    # Bootstrap: last_price absent or non-positive — accept unconditionally.
    # -----------------------------------------------------------------------

    def test_bootstrap_no_prior(self):
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:CRWD", 194.0, None)
        assert accept is True
        assert reason == "bootstrap"

    def test_bootstrap_zero_prior(self):
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:CRWD", 194.0, 0.0)
        assert accept is True
        assert reason == "bootstrap"

    def test_bootstrap_negative_prior(self):
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:CRWD", 194.0, -1.0)
        assert accept is True
        assert reason == "bootstrap"

    def test_bootstrap_clears_pending(self):
        # Pre-inject a pending value then trigger bootstrap — pending must be cleared.
        sanity._pending["NASDAQ:CRWD"] = 999.0
        accept, _ = sanity.check_update("NASDAQ", "NASDAQ:CRWD", 194.0, None)
        assert accept is True
        assert "NASDAQ:CRWD" not in sanity._pending

    # -----------------------------------------------------------------------
    # In-band: normal everyday price change — always accepted.
    # -----------------------------------------------------------------------

    def test_in_band_small_move(self):
        # +2% on NASDAQ (threshold 40%) — clearly in band.
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:AAPL", 102.0, 100.0)
        assert accept is True
        assert reason == "in_band"

    def test_in_band_at_threshold_boundary(self):
        # Exactly at the NASDAQ 40% threshold → in_band (<=, not strict <).
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:AAPL", 140.0, 100.0)
        assert accept is True
        assert reason == "in_band"

    def test_in_band_clears_pending(self):
        sanity._pending["NASDAQ:AAPL"] = 999.0
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:AAPL", 102.0, 100.0)
        assert accept is True
        assert reason == "in_band"
        assert "NASDAQ:AAPL" not in sanity._pending

    # -----------------------------------------------------------------------
    # Garbage tick: CRWD spike scenario (last=193, new=772 → 300% jump).
    # -----------------------------------------------------------------------

    def test_garbage_tick_first_reject(self):
        # NASDAQ threshold 40%; dev=(772-193)/193 ≈ 300% → way out of band.
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:CRWD", 772.74, 193.0)
        assert accept is False
        assert reason == "suspect_rejected"
        # Candidate is cached for next crawl.
        assert sanity._pending.get("NASDAQ:CRWD") == pytest.approx(772.74)

    def test_garbage_tick_next_crawl_reverts(self):
        # After the garbage tick is rejected, the next crawl brings back a normal
        # value near 193 → should be accepted as in_band (dev ≈ 1%).
        sanity._pending["NASDAQ:CRWD"] = 772.74  # simulate prior reject
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:CRWD", 195.0, 193.0)
        assert accept is True
        assert reason == "in_band"
        assert "NASDAQ:CRWD" not in sanity._pending

    # -----------------------------------------------------------------------
    # Split confirmation: last=762 → new=194 (4:1 split, ~75% drop).
    # Two consecutive crawls at ≈194 → second one must be "confirmed".
    # -----------------------------------------------------------------------

    def test_split_first_crawl_rejected(self):
        # First crawl after split: dev=(762-194)/762 ≈ 74.5% → NASDAQ 40% threshold exceeded.
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:CRWD", 194.0, 762.0)
        assert accept is False
        assert reason == "suspect_rejected"
        assert sanity._pending.get("NASDAQ:CRWD") == pytest.approx(194.0)

    def test_split_second_crawl_confirmed(self):
        # Simulate first crawl already cached 194.0 as pending.
        sanity._pending["NASDAQ:CRWD"] = 194.0
        # Second crawl returns 194.5 — within 2% rel_tol of the pending 194.0.
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:CRWD", 194.5, 762.0)
        assert accept is True
        assert reason == "confirmed"
        assert "NASDAQ:CRWD" not in sanity._pending

    def test_split_confirm_at_exact_same_price(self):
        sanity._pending["NASDAQ:CRWD"] = 194.0
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:CRWD", 194.0, 762.0)
        assert accept is True
        assert reason == "confirmed"

    def test_split_third_candidate_stays_pending(self):
        # If after first reject a different out-of-band price arrives (price changed
        # between crawls), the new price should become the updated pending candidate.
        sanity._pending["NASDAQ:CRWD"] = 194.0  # first reject was 194
        # Second crawl arrives with a very different out-of-band value (220) → new pending.
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:CRWD", 220.0, 762.0)
        assert accept is False
        assert reason == "suspect_rejected"
        assert sanity._pending.get("NASDAQ:CRWD") == pytest.approx(220.0)

    # -----------------------------------------------------------------------
    # Market-specific thresholds.
    # -----------------------------------------------------------------------

    def test_crypto_high_threshold_passes_50pct_move(self):
        # 50% move on CRYPTO (threshold=0.80) should pass.
        accept, reason = sanity.check_update("CRYPTO", "CRYPTO:bitcoin", 150.0, 100.0)
        assert accept is True
        assert reason == "in_band"

    def test_crypto_high_threshold_blocks_over_80pct(self):
        # 85% move on CRYPTO (threshold=0.80) — out of band.
        accept, reason = sanity.check_update("CRYPTO", "CRYPTO:bitcoin", 185.0, 100.0)
        assert accept is False
        assert reason == "suspect_rejected"

    def test_nasdaq_threshold_blocks_50pct(self):
        # 50% move on NASDAQ (threshold=0.40) — out of band.
        accept, reason = sanity.check_update("NASDAQ", "NASDAQ:AAPL", 150.0, 100.0)
        assert accept is False
        assert reason == "suspect_rejected"

    def test_gold_threshold_blocks_35pct(self):
        # 35% move on GOLD (threshold=0.30) — out of band.
        accept, reason = sanity.check_update("GOLD", "GOLD:XAU:spot", 135.0, 100.0)
        assert accept is False
        assert reason == "suspect_rejected"

    def test_gold_threshold_passes_25pct(self):
        # 25% move on GOLD (threshold=0.30) — in band.
        accept, reason = sanity.check_update("GOLD", "GOLD:XAU:spot", 125.0, 100.0)
        assert accept is True
        assert reason == "in_band"

    # -----------------------------------------------------------------------
    # Different keys don't interfere with each other.
    # -----------------------------------------------------------------------

    def test_keys_are_isolated(self):
        # Reject CRWD but not AAPL — pending should not bleed across keys.
        sanity.check_update("NASDAQ", "NASDAQ:CRWD", 772.0, 193.0)
        assert "NASDAQ:AAPL" not in sanity._pending

    def test_market_prefix_prevents_collision(self):
        # Same symbol in different markets must not share state.
        sanity.check_update("NASDAQ", "NASDAQ:AAPL", 772.0, 193.0)
        assert "SP500:AAPL" not in sanity._pending


# ===========================================================================
# batch_outlier_mask — intraday spike filter
# ===========================================================================

class TestBatchOutlierMask:
    """Tests for sanity.batch_outlier_mask(prices, market)."""

    # -----------------------------------------------------------------------
    # Core isolation test: garbage bar surrounded by normal values.
    # [193, 193, 772, 197, 195] — index 2 is an isolated spike.
    # -----------------------------------------------------------------------

    def test_isolated_spike_flagged(self):
        prices = [193.0, 193.0, 772.0, 197.0, 195.0]
        mask = sanity.batch_outlier_mask(prices, "NASDAQ")
        assert len(mask) == 5
        assert mask[2] is True   # spike
        assert mask[0] is False
        assert mask[1] is False
        assert mask[3] is False
        assert mask[4] is False

    # -----------------------------------------------------------------------
    # Split scenario: prices go from pre-split level to post-split level.
    # [760, 762, 761, 194, 193, 195] — NO bar should be flagged because the
    # step-change is sustained: 761→194 has large dev from prev, but 194→193
    # is tiny (next is similar) → NOT a bilateral spike.
    # -----------------------------------------------------------------------

    def test_split_boundary_not_flagged(self):
        prices = [760.0, 762.0, 761.0, 194.0, 193.0, 195.0]
        mask = sanity.batch_outlier_mask(prices, "NASDAQ")
        # None of these should be flagged — the transition is real.
        assert all(m is False for m in mask), f"Unexpected flags: {mask}"

    # -----------------------------------------------------------------------
    # Monotone steady series — nothing should be flagged.
    # -----------------------------------------------------------------------

    def test_steady_uptrend_all_false(self):
        prices = [100.0, 101.0, 102.0, 103.0, 104.0]
        mask = sanity.batch_outlier_mask(prices, "NASDAQ")
        assert all(m is False for m in mask)

    def test_flat_series_all_false(self):
        prices = [200.0, 200.0, 200.0, 200.0]
        mask = sanity.batch_outlier_mask(prices, "SP500")
        assert all(m is False for m in mask)

    # -----------------------------------------------------------------------
    # Endpoints are always False (insufficient bilateral context).
    # -----------------------------------------------------------------------

    def test_endpoints_never_flagged(self):
        # Even if first/last value is extreme, can't judge without two neighbours.
        prices = [9999.0, 100.0, 101.0, 9999.0]
        mask = sanity.batch_outlier_mask(prices, "NASDAQ")
        assert mask[0] is False
        assert mask[-1] is False

    # -----------------------------------------------------------------------
    # Edge cases: very short lists.
    # -----------------------------------------------------------------------

    def test_empty_list(self):
        mask = sanity.batch_outlier_mask([], "NASDAQ")
        assert mask == []

    def test_single_element(self):
        mask = sanity.batch_outlier_mask([100.0], "NASDAQ")
        assert mask == [False]

    def test_two_elements(self):
        mask = sanity.batch_outlier_mask([100.0, 9999.0], "NASDAQ")
        assert mask == [False, False]

    def test_three_elements_spike_in_middle(self):
        # [100, 9999, 100] — index 1 spikes on NASDAQ (9900% dev from neighbours).
        mask = sanity.batch_outlier_mask([100.0, 9999.0, 100.0], "NASDAQ")
        assert mask[0] is False
        assert mask[1] is True
        assert mask[2] is False

    # -----------------------------------------------------------------------
    # Zero neighbour — not enough data to judge, so False.
    # -----------------------------------------------------------------------

    def test_zero_neighbour_not_flagged(self):
        prices = [0.0, 9999.0, 100.0]
        mask = sanity.batch_outlier_mask(prices, "NASDAQ")
        # index 1: prev=0 → skip (not enough info) → False
        assert mask[1] is False

    # -----------------------------------------------------------------------
    # Market threshold differences.
    # -----------------------------------------------------------------------

    def test_crypto_threshold_higher(self):
        # 70% jump within CRYPTO threshold (0.80) → NOT flagged bilaterally.
        prices = [100.0, 170.0, 100.0]
        # Both sides deviate 70% — CRYPTO threshold 80% is NOT exceeded → False.
        mask = sanity.batch_outlier_mask(prices, "CRYPTO")
        assert mask[1] is False  # 70% < 80% threshold

    def test_crypto_spike_above_threshold(self):
        # 85% bilateral deviation — exceeds CRYPTO 80% threshold → flagged.
        prices = [100.0, 185.0, 100.0]
        mask = sanity.batch_outlier_mask(prices, "CRYPTO")
        assert mask[1] is True

    def test_nasdaq_spike_above_40pct(self):
        # 50% bilateral deviation — exceeds NASDAQ 40% threshold.
        prices = [100.0, 150.0, 100.0]
        mask = sanity.batch_outlier_mask(prices, "NASDAQ")
        assert mask[1] is True

    # -----------------------------------------------------------------------
    # Longer CRWD-realistic sequence: one garbage bar in the middle.
    # -----------------------------------------------------------------------

    def test_realistic_crwd_sequence(self):
        # Simulated intraday CRWD sequence around split date with one garbage tick.
        # Normal post-split prices ~193-197, with one transient 772 bar.
        prices = [193.5, 194.0, 193.8, 772.74, 194.2, 193.9, 194.5]
        mask = sanity.batch_outlier_mask(prices, "NASDAQ")
        # Only the 772.74 bar (index 3) should be flagged.
        expected = [False, False, False, True, False, False, False]
        assert mask == expected

    # -----------------------------------------------------------------------
    # Return type: list of bool, same length as input.
    # -----------------------------------------------------------------------

    def test_return_length_matches_input(self):
        for n in range(10):
            prices = [float(x + 1) for x in range(n)]
            mask = sanity.batch_outlier_mask(prices, "NASDAQ")
            assert len(mask) == n

    def test_return_type_is_bool(self):
        prices = [100.0, 200.0, 100.0]
        mask = sanity.batch_outlier_mask(prices, "NASDAQ")
        for m in mask:
            assert isinstance(m, bool)
