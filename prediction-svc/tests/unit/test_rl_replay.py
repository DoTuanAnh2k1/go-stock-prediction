"""Unit tests for src.orchestrator.rl_replay.

Verifies:
  - Module imports successfully (no DB, no torch required).
  - SIM_TO_REGISTRY_MARKET mapping is complete and correct.
  - Market key normalisation is consistent with bot.py's _SIM_TO_REGISTRY_MARKET.
  - _intraday_series_for_market / train_rl_intraday / generate_rl_prediction_asof /
    replay_rl_market are importable and have the correct signatures.

No DB or Docker is required.  All assertions are pure-logic.
"""
from __future__ import annotations

import inspect
import pytest


# ---------------------------------------------------------------------------
# Module import
# ---------------------------------------------------------------------------

def test_rl_replay_imports():
    """rl_replay must import without any DB / torch dependency at import time."""
    from src.orchestrator import rl_replay  # noqa: F401


# ---------------------------------------------------------------------------
# Mapping correctness
# ---------------------------------------------------------------------------

def test_sim_to_registry_market_keys():
    """SIM_TO_REGISTRY_MARKET must cover all 4 simulation markets."""
    from src.orchestrator.rl_replay import SIM_TO_REGISTRY_MARKET

    required_sim_keys = {"GOLD", "NASDAQ", "SP500", "CRYPTO"}
    assert required_sim_keys == set(SIM_TO_REGISTRY_MARKET.keys()), (
        f"Missing sim market keys: {required_sim_keys - set(SIM_TO_REGISTRY_MARKET.keys())}"
    )


def test_sim_to_registry_market_values():
    """Registry keys must match the checkpoint filename convention used by rl_dqn."""
    from src.orchestrator.rl_replay import SIM_TO_REGISTRY_MARKET

    # Checkpoint path = rl_dqn_{REGISTRY_KEY}.pt
    # Known existing checkpoints: rl_dqn_GOLD.pt, rl_dqn_NASDAQ100.pt,
    #   rl_dqn_SP500.pt, rl_dqn_CRYPTO.pt
    assert SIM_TO_REGISTRY_MARKET["NASDAQ"] == "NASDAQ100", (
        "NASDAQ must map to NASDAQ100 (checkpoint is rl_dqn_NASDAQ100.pt)"
    )
    assert SIM_TO_REGISTRY_MARKET["GOLD"] == "GOLD"
    assert SIM_TO_REGISTRY_MARKET["SP500"] == "SP500"
    assert SIM_TO_REGISTRY_MARKET["CRYPTO"] == "CRYPTO"


def test_replay_mapping_consistent_with_bot():
    """rl_replay.SIM_TO_REGISTRY_MARKET must match bot._SIM_TO_REGISTRY_MARKET."""
    from src.orchestrator.rl_replay import SIM_TO_REGISTRY_MARKET as replay_map
    from src.simulation.bot import _SIM_TO_REGISTRY_MARKET as bot_map

    # Both mappings must agree on all keys present in bot_map.
    for sim_key, registry_key in bot_map.items():
        assert replay_map.get(sim_key) == registry_key, (
            f"Mismatch for {sim_key!r}: replay={replay_map.get(sim_key)!r} "
            f"bot={registry_key!r}"
        )


# ---------------------------------------------------------------------------
# Public API signatures
# ---------------------------------------------------------------------------

def test_intraday_series_for_market_callable():
    """_intraday_series_for_market must be importable with the expected signature."""
    from src.orchestrator.rl_replay import _intraday_series_for_market

    sig = inspect.signature(_intraday_series_for_market)
    params = list(sig.parameters.keys())
    assert "market_key" in params
    assert "cutoff_dt" in params


def test_train_rl_intraday_signature():
    """train_rl_intraday must accept (market_key, cutoff_dt) and return dict."""
    from src.orchestrator.rl_replay import train_rl_intraday

    sig = inspect.signature(train_rl_intraday)
    params = list(sig.parameters.keys())
    assert "market_key" in params
    assert "cutoff_dt" in params


def test_generate_rl_prediction_asof_signature():
    """generate_rl_prediction_asof must accept (market_key, as_of_dt) and return int."""
    from src.orchestrator.rl_replay import generate_rl_prediction_asof

    sig = inspect.signature(generate_rl_prediction_asof)
    params = list(sig.parameters.keys())
    assert "market_key" in params
    assert "as_of_dt" in params


def test_replay_rl_market_signature():
    """replay_rl_market must accept (market_key, start_dt, end_dt, initial_capital)."""
    from src.orchestrator.rl_replay import replay_rl_market

    sig = inspect.signature(replay_rl_market)
    params = list(sig.parameters.keys())
    assert "market_key" in params
    assert "start_dt" in params
    assert "end_dt" in params
    assert "initial_capital" in params


# ---------------------------------------------------------------------------
# MIN_DATA_POINTS re-export
# ---------------------------------------------------------------------------

def test_min_data_points_imported():
    """rl_replay must import MIN_DATA_POINTS from features (not redefine it)."""
    from src.orchestrator.rl_replay import MIN_DATA_POINTS
    from src.algorithms.features import MIN_DATA_POINTS as features_min

    assert MIN_DATA_POINTS == features_min, (
        "rl_replay.MIN_DATA_POINTS must equal features.MIN_DATA_POINTS"
    )


# ---------------------------------------------------------------------------
# Bot intraday helpers exist
# ---------------------------------------------------------------------------

def test_bot_intraday_helpers_exist():
    """TradingBot must have the two new intraday helper methods."""
    from src.simulation.bot import TradingBot

    assert hasattr(TradingBot, "_get_symbols_and_prices_intraday_as_of"), (
        "TradingBot must have _get_symbols_and_prices_intraday_as_of"
    )
    assert hasattr(TradingBot, "_get_price_history_intraday_as_of"), (
        "TradingBot must have _get_price_history_intraday_as_of"
    )


def test_bot_intraday_helper_signatures():
    """New intraday helpers must accept the expected parameters."""
    from src.simulation.bot import TradingBot

    sig1 = inspect.signature(TradingBot._get_symbols_and_prices_intraday_as_of)
    assert "now" in sig1.parameters

    sig2 = inspect.signature(TradingBot._get_price_history_intraday_as_of)
    assert "symbol" in sig2.parameters
    assert "now" in sig2.parameters
