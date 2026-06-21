"""Service registry client — register/heartbeat/deregister with service-mgt.

Usage:
    from src.registry_client import start, stop

    # On startup (after gRPC server is up):
    start(enabled=cfg.service_mgt_enabled, target=cfg.registry_grpc_target,
          service_name="prediction-svc", address="prediction-svc", port=8119)

    # On shutdown:
    stop()

All operations are best-effort: a dead registry never crashes the service.
Controlled by SERVICE_MGT_ENABLED env var (default False).
"""
from __future__ import annotations

import threading
import time

import grpc

from src.proto.registry import registry_pb2, registry_pb2_grpc
from src.utils.logger import get_logger

log = get_logger("registry_client")

# ---------------------------------------------------------------------------
# Module-level state (singleton)
# ---------------------------------------------------------------------------
_channel: grpc.Channel | None = None
_stub: registry_pb2_grpc.RegistryStub | None = None
_instance_id: str | None = None
_stop_event: threading.Event = threading.Event()
_heartbeat_thread: threading.Thread | None = None
# Registration params saved so the heartbeat loop can re-register after a
# lost lease (registry restart) or a failed boot-time Register.
_reg_params: dict | None = None

# Heartbeat interval in seconds (well below typical TTL of 30s)
_HEARTBEAT_INTERVAL = 10


def _heartbeat_loop(ttl: int) -> None:
    """Background daemon thread — send Heartbeat every _HEARTBEAT_INTERVAL seconds.

    Re-registers automatically when instance_id is lost (e.g. registry restart
    invalidates the lease, or boot-time Register failed).
    """
    global _instance_id

    while not _stop_event.wait(timeout=_HEARTBEAT_INTERVAL):
        current_id = _instance_id
        if current_id is None:
            # Not registered (lost lease or failed boot) — try to (re-)register.
            if _reg_params is not None:
                _instance_id = _do_register(**_reg_params)
            continue
        try:
            resp = _stub.Heartbeat(registry_pb2.HeartbeatRequest(instance_id=current_id))
            if resp.ok:
                log.debug("registry.heartbeat.ok", instance_id=current_id)
            else:
                log.warning("registry.heartbeat.not_ok", instance_id=current_id)
                _instance_id = None
        except grpc.RpcError as exc:
            code = exc.code()
            log.warning(
                "registry.heartbeat.error",
                instance_id=current_id,
                code=str(code),
                detail=exc.details(),
            )
            # Clear so the next tick attempts a fresh register (handled outside this loop)
            _instance_id = None


def _do_register(
    service_name: str,
    address: str,
    port: int,
    ttl: int,
) -> str | None:
    """Call Register RPC and return the assigned instance_id, or None on error."""
    try:
        resp = _stub.Register(
            registry_pb2.RegisterRequest(
                service_name=service_name,
                address=address,
                port=port,
                ttl_seconds=ttl,
            )
        )
        log.info(
            "registry.registered",
            service=service_name,
            instance_id=resp.instance_id,
            lease_ttl=resp.lease_ttl_seconds,
        )
        return resp.instance_id
    except grpc.RpcError as exc:
        log.warning(
            "registry.register.error",
            service=service_name,
            code=str(exc.code()),
            detail=exc.details(),
        )
        return None
    except Exception as exc:
        log.warning("registry.register.unexpected_error", error=str(exc))
        return None


def start(
    enabled: bool,
    target: str,
    service_name: str,
    address: str,
    port: int,
    ttl: int = 30,
) -> None:
    """Open channel, register, and start the heartbeat background thread.

    Safe to call even when enabled=False — logs once and returns immediately.
    A failed Register does NOT raise; the service continues without registration.
    """
    global _channel, _stub, _instance_id, _stop_event, _heartbeat_thread, _reg_params

    if not enabled:
        log.info("registry.disabled")
        return

    log.info("registry.starting", target=target, service=service_name, address=address, port=port)

    _channel = grpc.insecure_channel(target)
    _stub = registry_pb2_grpc.RegistryStub(_channel)

    # Save params so the heartbeat loop can re-register after a lost lease.
    _reg_params = {"service_name": service_name, "address": address, "port": port, "ttl": ttl}

    # Initial registration (best-effort)
    _instance_id = _do_register(service_name, address, port, ttl)

    # Reset stop event (may have been set from a previous call in tests)
    _stop_event.clear()

    # Start heartbeat daemon thread
    _heartbeat_thread = threading.Thread(
        target=_heartbeat_loop,
        args=(ttl,),
        daemon=True,
        name="registry-heartbeat",
    )
    _heartbeat_thread.start()
    log.info("registry.heartbeat.started", interval_s=_HEARTBEAT_INTERVAL)


def stop() -> None:
    """Signal the heartbeat thread to stop, deregister best-effort, close channel."""
    global _channel, _stub, _instance_id, _heartbeat_thread

    _stop_event.set()

    current_id = _instance_id
    if current_id is not None and _stub is not None:
        try:
            _stub.Deregister(registry_pb2.DeregisterRequest(instance_id=current_id))
            log.info("registry.deregistered", instance_id=current_id)
        except Exception as exc:
            log.warning("registry.deregister.error", instance_id=current_id, error=str(exc))

    _instance_id = None

    if _channel is not None:
        try:
            _channel.close()
        except Exception:
            pass
        _channel = None

    _stub = None
    log.info("registry.stopped")
