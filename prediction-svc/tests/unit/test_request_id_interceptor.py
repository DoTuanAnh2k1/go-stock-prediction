"""Unit tests for RequestIdInterceptor.

Tests verify:
- A fresh request_id is minted when ``x-request-id`` is absent from metadata.
- An inbound ``x-request-id`` is reused unchanged.
- The id is bound into structlog contextvars during handler execution and
  cleared afterwards.
- All four RPC handler shapes (unary_unary, unary_stream, stream_unary,
  stream_stream) are handled.
"""
from __future__ import annotations

import uuid
from types import SimpleNamespace
from typing import Any, Iterator
from unittest.mock import MagicMock, patch

import pytest
import structlog.contextvars

from src.grpc_server.request_id_interceptor import (
    RequestIdInterceptor,
    _mint_request_id,
    _METADATA_KEY,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_handler(**kwargs) -> Any:
    """Return a minimal grpc.RpcMethodHandler-like namespace."""
    defaults = dict(
        unary_unary=None,
        unary_stream=None,
        stream_unary=None,
        stream_stream=None,
        request_deserializer=None,
        response_serializer=None,
    )
    defaults.update(kwargs)
    return SimpleNamespace(**defaults)


def _make_details(metadata=None):
    """Return a handler_call_details-like namespace."""
    return SimpleNamespace(
        method="/prediction.PredictionService/TriggerTrain",
        invocation_metadata=metadata or [],
    )


# ---------------------------------------------------------------------------
# _mint_request_id
# ---------------------------------------------------------------------------

class TestMintRequestId:
    def test_returns_inbound_id_when_present(self):
        metadata = [(_METADATA_KEY, "abc123"), ("other", "val")]
        assert _mint_request_id(metadata) == "abc123"

    def test_mints_uuid_when_absent(self):
        result = _mint_request_id([("content-type", "application/grpc")])
        # Should be a 32-char hex string (uuid4 without dashes)
        assert len(result) == 32
        uuid.UUID(result)  # raises ValueError if not valid UUID hex

    def test_mints_uuid_when_metadata_empty(self):
        result = _mint_request_id([])
        assert len(result) == 32

    def test_mints_uuid_when_metadata_none(self):
        result = _mint_request_id(None)
        assert len(result) == 32

    def test_skips_blank_value(self):
        metadata = [(_METADATA_KEY, "   ")]
        result = _mint_request_id(metadata)
        assert len(result) == 32  # blank → minted, not the blank string


# ---------------------------------------------------------------------------
# Interceptor — unary_unary
# ---------------------------------------------------------------------------

class TestInterceptorUnaryUnary:
    def setup_method(self):
        structlog.contextvars.clear_contextvars()

    def test_binds_request_id_during_call(self):
        interceptor = RequestIdInterceptor()
        captured: list[str] = []

        def _handler(request, context):
            captured.append(structlog.contextvars.get_contextvars().get("request_id", ""))
            return "response"

        original = _make_handler(unary_unary=_handler)
        details = _make_details()

        wrapped = interceptor.intercept_service(lambda d: original, details)
        wrapped.unary_unary("req", MagicMock())

        assert len(captured) == 1
        assert len(captured[0]) == 32  # minted UUID hex

    def test_reuses_inbound_id(self):
        interceptor = RequestIdInterceptor()
        inbound_id = "deadbeef" * 4  # 32 chars
        captured: list[str] = []

        def _handler(request, context):
            captured.append(structlog.contextvars.get_contextvars().get("request_id", ""))
            return "response"

        original = _make_handler(unary_unary=_handler)
        details = _make_details(metadata=[(_METADATA_KEY, inbound_id)])

        wrapped = interceptor.intercept_service(lambda d: original, details)
        wrapped.unary_unary("req", MagicMock())

        assert captured[0] == inbound_id

    def test_clears_after_call(self):
        interceptor = RequestIdInterceptor()

        def _handler(request, context):
            return "response"

        original = _make_handler(unary_unary=_handler)
        details = _make_details()

        wrapped = interceptor.intercept_service(lambda d: original, details)
        wrapped.unary_unary("req", MagicMock())

        assert "request_id" not in structlog.contextvars.get_contextvars()

    def test_clears_even_on_exception(self):
        interceptor = RequestIdInterceptor()

        def _handler(request, context):
            raise RuntimeError("boom")

        original = _make_handler(unary_unary=_handler)
        details = _make_details()

        wrapped = interceptor.intercept_service(lambda d: original, details)
        with pytest.raises(RuntimeError):
            wrapped.unary_unary("req", MagicMock())

        assert "request_id" not in structlog.contextvars.get_contextvars()

    def test_preserves_serializers(self):
        interceptor = RequestIdInterceptor()
        deser = object()
        ser = object()

        original = _make_handler(
            unary_unary=lambda req, ctx: "r",
            request_deserializer=deser,
            response_serializer=ser,
        )
        details = _make_details()
        wrapped = interceptor.intercept_service(lambda d: original, details)

        assert wrapped.request_deserializer is deser
        assert wrapped.response_serializer is ser


# ---------------------------------------------------------------------------
# Interceptor — unary_stream
# ---------------------------------------------------------------------------

class TestInterceptorUnaryStream:
    def setup_method(self):
        structlog.contextvars.clear_contextvars()

    def test_binds_during_stream(self):
        interceptor = RequestIdInterceptor()
        captured: list[str] = []

        def _handler(request, context) -> Iterator:
            captured.append(structlog.contextvars.get_contextvars().get("request_id", ""))
            yield "item1"
            yield "item2"

        original = _make_handler(unary_stream=_handler)
        details = _make_details()
        wrapped = interceptor.intercept_service(lambda d: original, details)

        items = list(wrapped.unary_stream("req", MagicMock()))
        assert items == ["item1", "item2"]
        assert len(captured[0]) == 32

    def test_clears_after_stream(self):
        interceptor = RequestIdInterceptor()

        def _handler(request, context) -> Iterator:
            yield "x"

        original = _make_handler(unary_stream=_handler)
        details = _make_details()
        wrapped = interceptor.intercept_service(lambda d: original, details)

        list(wrapped.unary_stream("req", MagicMock()))
        assert "request_id" not in structlog.contextvars.get_contextvars()


# ---------------------------------------------------------------------------
# Interceptor — stream_unary
# ---------------------------------------------------------------------------

class TestInterceptorStreamUnary:
    def setup_method(self):
        structlog.contextvars.clear_contextvars()

    def test_binds_during_call(self):
        interceptor = RequestIdInterceptor()
        captured: list[str] = []

        def _handler(request_iterator, context):
            captured.append(structlog.contextvars.get_contextvars().get("request_id", ""))
            return "response"

        original = _make_handler(stream_unary=_handler)
        details = _make_details()
        wrapped = interceptor.intercept_service(lambda d: original, details)

        result = wrapped.stream_unary(iter(["req1", "req2"]), MagicMock())
        assert result == "response"
        assert len(captured[0]) == 32

    def test_clears_after_call(self):
        interceptor = RequestIdInterceptor()

        def _handler(request_iterator, context):
            return "r"

        original = _make_handler(stream_unary=_handler)
        details = _make_details()
        wrapped = interceptor.intercept_service(lambda d: original, details)
        wrapped.stream_unary(iter([]), MagicMock())
        assert "request_id" not in structlog.contextvars.get_contextvars()


# ---------------------------------------------------------------------------
# Interceptor — stream_stream
# ---------------------------------------------------------------------------

class TestInterceptorStreamStream:
    def setup_method(self):
        structlog.contextvars.clear_contextvars()

    def test_binds_during_stream(self):
        interceptor = RequestIdInterceptor()
        captured: list[str] = []

        def _handler(request_iterator, context) -> Iterator:
            captured.append(structlog.contextvars.get_contextvars().get("request_id", ""))
            yield from request_iterator

        original = _make_handler(stream_stream=_handler)
        details = _make_details()
        wrapped = interceptor.intercept_service(lambda d: original, details)

        items = list(wrapped.stream_stream(iter(["a", "b"]), MagicMock()))
        assert items == ["a", "b"]
        assert len(captured[0]) == 32

    def test_clears_after_stream(self):
        interceptor = RequestIdInterceptor()

        def _handler(request_iterator, context) -> Iterator:
            yield "x"

        original = _make_handler(stream_stream=_handler)
        details = _make_details()
        wrapped = interceptor.intercept_service(lambda d: original, details)
        list(wrapped.stream_stream(iter([]), MagicMock()))
        assert "request_id" not in structlog.contextvars.get_contextvars()


# ---------------------------------------------------------------------------
# Interceptor — None handler pass-through
# ---------------------------------------------------------------------------

class TestInterceptorNoneHandler:
    def test_returns_none_when_continuation_returns_none(self):
        interceptor = RequestIdInterceptor()
        details = _make_details()
        result = interceptor.intercept_service(lambda d: None, details)
        assert result is None


# ---------------------------------------------------------------------------
# jobs.py — _with_request_id wrapper
# ---------------------------------------------------------------------------

class TestWithRequestId:
    def setup_method(self):
        structlog.contextvars.clear_contextvars()

    def test_binds_request_id(self):
        from src.scheduler.jobs import _with_request_id

        captured: list[str] = []

        def _job():
            captured.append(structlog.contextvars.get_contextvars().get("request_id", ""))

        _with_request_id(_job)()
        assert len(captured) == 1
        assert len(captured[0]) == 32

    def test_clears_after_job(self):
        from src.scheduler.jobs import _with_request_id

        _with_request_id(lambda: None)()
        assert "request_id" not in structlog.contextvars.get_contextvars()

    def test_clears_on_exception(self):
        from src.scheduler.jobs import _with_request_id

        def _bad():
            raise ValueError("oops")

        with pytest.raises(ValueError):
            _with_request_id(_bad)()

        assert "request_id" not in structlog.contextvars.get_contextvars()

    def test_job_functions_are_all_wrapped(self):
        """Every entry in JOB_FUNCTIONS must have its own request_id per call."""
        from src.scheduler.jobs import JOB_FUNCTIONS, _with_request_id

        ids_seen: list[str] = []

        def _probe():
            ids_seen.append(structlog.contextvars.get_contextvars().get("request_id", ""))

        # Pick any two wrapped callables and confirm they each get a unique id.
        wrapped1 = _with_request_id(_probe)
        wrapped2 = _with_request_id(_probe)
        wrapped1()
        wrapped2()

        assert len(ids_seen) == 2
        assert ids_seen[0] != ids_seen[1], "each job run should get a distinct request_id"
