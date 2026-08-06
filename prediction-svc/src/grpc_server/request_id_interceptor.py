"""gRPC server interceptor — propagates or mints X-Request-ID per RPC.

CONTRACT (shared across all 6 services):
  - Inbound metadata key : ``x-request-id`` (lowercase)
  - structlog field name  : ``request_id``
  - Value format          : UUID v4 hex (no dashes)

For every incoming RPC the interceptor:
  1. Reads ``x-request-id`` from ``handler_call_details.invocation_metadata``.
  2. Mints ``uuid.uuid4().hex`` when the key is absent.
  3. Calls ``structlog.contextvars.bind_contextvars(request_id=rid)`` before
     delegating to the real handler — so the id appears on every log line
     produced during that handler's execution.
  4. Clears the binding in a ``finally`` block via
     ``structlog.contextvars.unbind_contextvars("request_id")``.

All four gRPC handler shapes are handled:
  - unary_unary   → ``grpc.unary_unary_rpc_method_handler``
  - unary_stream  → ``grpc.unary_stream_rpc_method_handler``
  - stream_unary  → ``grpc.stream_unary_rpc_method_handler``
  - stream_stream → ``grpc.stream_stream_rpc_method_handler``

The ``request_deserializer`` and ``response_serializer`` from the original
handler are forwarded unchanged to the replacement handler so the gRPC
framework can still (de)serialise messages correctly.
"""
from __future__ import annotations

import uuid
from typing import Any

import grpc
import structlog.contextvars

_METADATA_KEY = "x-request-id"


def _mint_request_id(invocation_metadata) -> str:
    """Return the inbound request-id or a freshly minted UUID hex string."""
    for key, value in (invocation_metadata or []):
        if key == _METADATA_KEY:
            v = (value or "").strip()
            if v:
                return v
    return uuid.uuid4().hex


class RequestIdInterceptor(grpc.ServerInterceptor):
    """Bind a correlation ``request_id`` into structlog contextvars for each RPC.

    Register via ``grpc.server(..., interceptors=[RequestIdInterceptor()])``.
    """

    def intercept_service(self, continuation, handler_call_details):
        original_handler = continuation(handler_call_details)
        if original_handler is None:
            return original_handler

        rid = _mint_request_id(handler_call_details.invocation_metadata)

        # Reconstruct the handler with a wrapped callable that binds/unbinds
        # the request_id in structlog contextvars.  We must cover all four
        # handler shapes; grpc.RpcMethodHandler is a named-tuple so we use the
        # matching factory functions instead of _replace to keep the semantics
        # identical to what grpcio expects.

        if original_handler.unary_unary is not None:
            fn = original_handler.unary_unary

            def _unary_unary(request: Any, context: grpc.ServicerContext) -> Any:
                structlog.contextvars.bind_contextvars(request_id=rid)
                try:
                    return fn(request, context)
                finally:
                    structlog.contextvars.unbind_contextvars("request_id")

            return grpc.unary_unary_rpc_method_handler(
                _unary_unary,
                request_deserializer=original_handler.request_deserializer,
                response_serializer=original_handler.response_serializer,
            )

        if original_handler.unary_stream is not None:
            fn = original_handler.unary_stream

            def _unary_stream(request: Any, context: grpc.ServicerContext):
                structlog.contextvars.bind_contextvars(request_id=rid)
                try:
                    yield from fn(request, context)
                finally:
                    structlog.contextvars.unbind_contextvars("request_id")

            return grpc.unary_stream_rpc_method_handler(
                _unary_stream,
                request_deserializer=original_handler.request_deserializer,
                response_serializer=original_handler.response_serializer,
            )

        if original_handler.stream_unary is not None:
            fn = original_handler.stream_unary

            def _stream_unary(request_iterator: Any, context: grpc.ServicerContext) -> Any:
                structlog.contextvars.bind_contextvars(request_id=rid)
                try:
                    return fn(request_iterator, context)
                finally:
                    structlog.contextvars.unbind_contextvars("request_id")

            return grpc.stream_unary_rpc_method_handler(
                _stream_unary,
                request_deserializer=original_handler.request_deserializer,
                response_serializer=original_handler.response_serializer,
            )

        if original_handler.stream_stream is not None:
            fn = original_handler.stream_stream

            def _stream_stream(request_iterator: Any, context: grpc.ServicerContext):
                structlog.contextvars.bind_contextvars(request_id=rid)
                try:
                    yield from fn(request_iterator, context)
                finally:
                    structlog.contextvars.unbind_contextvars("request_id")

            return grpc.stream_stream_rpc_method_handler(
                _stream_stream,
                request_deserializer=original_handler.request_deserializer,
                response_serializer=original_handler.response_serializer,
            )

        # Fallback: handler shape not recognised — return unmodified.
        return original_handler
