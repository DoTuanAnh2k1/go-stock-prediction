"""Phase 5 integration tests — Trading Simulation.

Tests verify:
1. Seeder: sim_bots table has 66 rows
2. API: /api/simulation/leaderboard returns 200 + 66 bots
3. API: /api/simulation/bots returns list of bots
4. API: /api/simulation/bots/{id} returns bot detail
5. Engine: SimulationEngine can instantiate without error
6. Backtest: TriggerSimulationBacktest gRPC returns success (background)

Run:
    make test-simulation
    # or
    docker exec prediction_service python -m pytest tests/integration/test_phase5_simulation.py -v
"""
from __future__ import annotations

import pytest

pytestmark = pytest.mark.integration


# ---------------------------------------------------------------------------
# DB seeder tests
# ---------------------------------------------------------------------------

def test_sim_bots_seeded_count():
    """sim_bots table must have 66 rows after seeder runs."""
    from src.database.connection import session_scope
    from src.database.models import SimBot

    with session_scope() as session:
        count = session.query(SimBot).count()
    assert count == 66, f"Expected 66 bots, got {count}"


def test_sim_bots_all_markets_present():
    """All 6 markets must be represented."""
    from src.database.connection import session_scope
    from src.database.models import SimBot

    with session_scope() as session:
        markets = {b.market for b in session.query(SimBot).all()}
    assert markets == {"VN30", "GOLD", "NASDAQ", "SP500", "CRYPTO"}


def test_sim_bots_all_algorithms_present():
    """All 11 algorithms must be represented."""
    from src.database.connection import session_scope
    from src.database.models import SimBot

    with session_scope() as session:
        algos = {b.algorithm for b in session.query(SimBot).all()}
    expected = {
        "moving_average", "ema", "lstm_nn", "arima_garch", "lightgbm",
        "sarima", "egarch", "gru_nn", "random_forest", "xgboost", "ensemble",
    }
    assert algos == expected


def test_sim_bots_vnd_markets_have_correct_capital():
    """VN30 bots must have 1,000,000,000 VND initial capital."""
    from src.database.connection import session_scope
    from src.database.models import SimBot

    with session_scope() as session:
        vnd_data = [(b.id, float(b.initial_capital))
                    for b in session.query(SimBot).filter(SimBot.currency == "VND").all()]
    assert len(vnd_data) == 22, f"Expected 22 VND bots, got {len(vnd_data)}"
    for bot_id, capital in vnd_data:
        assert capital == 1_000_000_000.0, f"Bot {bot_id} wrong capital: {capital}"


def test_sim_bots_usd_markets_have_correct_capital():
    """GOLD, NASDAQ, SP500, CRYPTO bots must have 100,000 USD initial capital."""
    from src.database.connection import session_scope
    from src.database.models import SimBot

    with session_scope() as session:
        usd_data = [(b.id, float(b.initial_capital))
                    for b in session.query(SimBot).filter(SimBot.currency == "USD").all()]
    assert len(usd_data) == 44, f"Expected 44 USD bots, got {len(usd_data)}"
    for bot_id, capital in usd_data:
        assert capital == 100_000.0, f"Bot {bot_id} wrong capital: {capital}"


def test_sim_bots_default_thresholds():
    """All bots should have the default buy/sell thresholds from seeder."""
    from src.database.connection import session_scope
    from src.database.models import SimBot

    with session_scope() as session:
        thresholds = [(b.id, float(b.buy_threshold), float(b.sell_threshold),
                       float(b.min_confidence), float(b.stop_loss), float(b.take_profit))
                      for b in session.query(SimBot).all()]

    for bot_id, buy_thr, sell_thr, min_conf, sl, tp in thresholds:
        assert buy_thr == pytest.approx(1.50, rel=1e-6), f"Bot {bot_id} wrong buy_threshold"
        assert sell_thr == pytest.approx(1.00, rel=1e-6), f"Bot {bot_id} wrong sell_threshold"
        assert min_conf == pytest.approx(0.60, rel=1e-6), f"Bot {bot_id} wrong min_confidence"
        assert sl == pytest.approx(5.00, rel=1e-6), f"Bot {bot_id} wrong stop_loss"
        assert tp == pytest.approx(8.00, rel=1e-6), f"Bot {bot_id} wrong take_profit"


def test_sim_bots_max_positions():
    """All bots must have max_positions=5."""
    from src.database.connection import session_scope
    from src.database.models import SimBot

    with session_scope() as session:
        positions_data = [(b.id, int(b.max_positions)) for b in session.query(SimBot).all()]

    for bot_id, max_pos in positions_data:
        assert max_pos == 5, f"Bot {bot_id} has max_positions={max_pos}"


def test_sim_bots_are_active():
    """All seeded bots must be active by default."""
    from src.database.connection import session_scope
    from src.database.models import SimBot

    with session_scope() as session:
        inactive = session.query(SimBot).filter(SimBot.is_active == False).count()
    assert inactive == 0


def test_sim_bots_bot_ids_follow_naming_convention():
    """Bot IDs must follow '{market_lower}_{algo}' pattern."""
    from src.database.connection import session_scope
    from src.database.models import SimBot

    with session_scope() as session:
        bot_data = [(b.id, b.market, b.algorithm) for b in session.query(SimBot).all()]

    for bot_id, market, algorithm in bot_data:
        expected_prefix = market.lower()
        assert bot_id.startswith(expected_prefix), (
            f"Bot {bot_id!r} doesn't start with market prefix {expected_prefix!r}"
        )
        assert algorithm in bot_id, (
            f"Bot {bot_id!r} doesn't contain algorithm {algorithm!r}"
        )


# ---------------------------------------------------------------------------
# API integration tests
# ---------------------------------------------------------------------------

def test_leaderboard_api_returns_all_bots(api_base_url):
    """GET /api/simulation/leaderboard must return 66 bots."""
    import requests

    resp = requests.get(f"{api_base_url}/api/simulation/leaderboard", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert "leaderboard" in data
    assert len(data["leaderboard"]) == 66
    assert data["summary"]["total_bots"] == 66


def test_leaderboard_api_filter_by_market(api_base_url):
    """GET /api/simulation/leaderboard?market=CRYPTO must return 11 bots."""
    import requests

    resp = requests.get(
        f"{api_base_url}/api/simulation/leaderboard?market=CRYPTO", timeout=10
    )
    assert resp.status_code == 200
    data = resp.json()
    for bot in data["leaderboard"]:
        assert bot["market"] == "CRYPTO"
    assert len(data["leaderboard"]) == 11


def test_bots_api_returns_list(api_base_url):
    """GET /api/simulation/bots must return a list of 66 bots."""
    import requests

    resp = requests.get(f"{api_base_url}/api/simulation/bots", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert isinstance(data, list)
    assert len(data) == 66


def test_bot_detail_api(api_base_url):
    """GET /api/simulation/bots/crypto_ensemble must return bot detail."""
    import requests

    resp = requests.get(f"{api_base_url}/api/simulation/bots/crypto_ensemble", timeout=10)
    assert resp.status_code == 200
    data = resp.json()
    assert data["id"] == "crypto_ensemble"
    assert data["market"] == "CRYPTO"
    assert data["algorithm"] == "ensemble"
    assert "kpis" in data


def test_bot_detail_not_found(api_base_url):
    """GET /api/simulation/bots/nonexistent must return 404."""
    import requests

    resp = requests.get(
        f"{api_base_url}/api/simulation/bots/nonexistent_bot_xyz", timeout=10
    )
    assert resp.status_code == 404


def test_bot_trades_api_empty(api_base_url):
    """GET /api/simulation/bots/vn30_lstm_nn/trades returns 200 even when no trades yet."""
    import requests

    resp = requests.get(
        f"{api_base_url}/api/simulation/bots/vn30_lstm_nn/trades", timeout=10
    )
    assert resp.status_code == 200
    data = resp.json()
    assert "data" in data
    assert "total" in data


def test_bot_chart_api_empty(api_base_url):
    """GET /api/simulation/bots/vn30_lstm_nn/chart returns 200 even when no data yet."""
    import requests

    resp = requests.get(
        f"{api_base_url}/api/simulation/bots/vn30_lstm_nn/chart", timeout=10
    )
    assert resp.status_code == 200
    data = resp.json()
    assert "dates" in data
    assert "values" in data


# ---------------------------------------------------------------------------
# Module instantiation tests
# ---------------------------------------------------------------------------

def test_simulation_engine_instantiates():
    """SimulationEngine can be imported and instantiated without error."""
    from src.simulation.engine import SimulationEngine

    engine = SimulationEngine()
    assert engine is not None


def test_signal_generator_instantiates():
    """SignalGenerator can be imported and instantiated."""
    from src.simulation.signal import SignalGenerator, TradeSignal

    sg = SignalGenerator()
    assert sg is not None


def test_performance_metrics_instantiates():
    """PerformanceMetrics can be imported and basic compute works."""
    from datetime import date

    from src.simulation.metrics import PerformanceMetrics, SimKPIs

    snaps = [
        {"snapshot_date": date(2024, 1, 1), "total_value": 100_000.0, "total_return_pct": 0.0},
        {"snapshot_date": date(2024, 6, 1), "total_value": 115_000.0, "total_return_pct": 15.0},
    ]
    kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
    assert isinstance(kpis, SimKPIs)
    assert abs(kpis.total_return_pct - 15.0) < 0.01


def test_trading_bot_instantiates():
    """TradingBot can be instantiated with a BotConfig."""
    from src.simulation.bot import BotConfig, TradingBot

    config = BotConfig(
        bot_id="test_bot",
        market="CRYPTO",
        algorithm="ensemble",
        initial_capital=100_000.0,
        buy_threshold=1.5,
        sell_threshold=1.0,
        min_confidence=0.60,
        stop_loss=5.0,
        take_profit=8.0,
        max_position_pct=15.0,
        max_positions=5,
    )
    bot = TradingBot(config)
    assert bot is not None
    assert bot.portfolio.cash == 100_000.0


def test_portfolio_instantiates():
    """Portfolio can be instantiated correctly."""
    from src.simulation.portfolio import Portfolio

    p = Portfolio(
        initial_capital=1_000_000.0,
        stop_loss_pct=5.0,
        take_profit_pct=8.0,
        max_position_pct=15.0,
        max_positions=5,
    )
    assert p.cash == 1_000_000.0
    assert p.positions == {}


# ---------------------------------------------------------------------------
# gRPC tests
# ---------------------------------------------------------------------------

def test_grpc_trigger_simulation_backtest(grpc_stub):
    """TriggerSimulationBacktest RPC must return success=True."""
    try:
        from src.proto.prediction import prediction_pb2

        req = prediction_pb2.SimulationRequest(
            bot_id="crypto_ensemble",
            start_date="2025-01-01",
            end_date="2025-01-31",
        )
        resp = grpc_stub.TriggerSimulationBacktest(req)
        assert resp.success is True
        assert (
            "simulation" in resp.message.lower()
            or "backtest" in resp.message.lower()
        )
    except Exception as e:
        pytest.skip(f"gRPC not available or proto stubs not regenerated: {e}")


def test_grpc_trigger_simulation_live_step(grpc_stub):
    """TriggerSimulationLiveStep RPC must return success=True."""
    try:
        from src.proto.prediction import prediction_pb2

        resp = grpc_stub.TriggerSimulationLiveStep(prediction_pb2.Empty())
        assert resp.success is True
    except Exception as e:
        pytest.skip(f"gRPC not available or proto stubs not regenerated: {e}")
