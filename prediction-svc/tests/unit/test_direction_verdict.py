"""Unit tests for repository.direction_verdict — the frozen-actual reconcile guard."""
from decimal import Decimal

from src.database.repository import direction_verdict


def test_frozen_actual_is_unscorable():
    # actual price never moved from entry -> unscorable, must return None so the
    # caller leaves the prediction pending instead of marking every algorithm wrong.
    assert direction_verdict(Decimal("5"), Decimal("0")) is None
    assert direction_verdict(Decimal("-5"), Decimal("0")) is None
    assert direction_verdict(Decimal("0"), Decimal("0")) is None


def test_correct_directions():
    # predicted up, actual up
    assert direction_verdict(Decimal("2"), Decimal("3")) is True
    # predicted down, actual down
    assert direction_verdict(Decimal("-2"), Decimal("-3")) is True


def test_wrong_directions():
    # predicted up, actual down
    assert direction_verdict(Decimal("2"), Decimal("-3")) is False
    # predicted down, actual up
    assert direction_verdict(Decimal("-2"), Decimal("3")) is False


def test_flat_prediction_never_counts_as_correct():
    # a zero-change prediction is not a directional call -> False when actual moves
    assert direction_verdict(Decimal("0"), Decimal("3")) is False
    assert direction_verdict(Decimal("0"), Decimal("-3")) is False
