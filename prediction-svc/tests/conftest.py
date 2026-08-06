"""pytest fixtures shared across test suites.

Integration tests run inside the prediction Docker container:
  make test-phase1     # runs via docker exec
  make test-phase2     # runs via docker exec

From inside the container:
  - gRPC: localhost:8119  (same container, always accessible)
  - API:  http://api:8118 (Docker network, override via TEST_API_URL)
  - DB:   initialised automatically via init_db_fixture

Environment variables:
  TEST_API_URL  - override API base URL (default: http://api:8118)
"""
from __future__ import annotations

import os
import time

import pytest
import requests

# ---------------------------------------------------------------------------
# DB initialisation (needed when running tests fresh via docker exec)
# ---------------------------------------------------------------------------


@pytest.fixture(scope="session", autouse=True)
def init_db_fixture():
    """Initialise the SQLAlchemy session factory before any test accesses DB."""
    try:
        from src.database.connection import init_db
        init_db()
    except Exception:
        pass  # DB tests will fail with clear errors if init fails


# ---------------------------------------------------------------------------
# gRPC fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(scope="session")
def grpc_channel():
    """Open a gRPC channel to the running prediction service."""
    import grpc

    channel = grpc.insecure_channel("localhost:8119")
    try:
        grpc.channel_ready_future(channel).result(timeout=5)
    except grpc.FutureTimeoutError:
        pass
    yield channel
    channel.close()


@pytest.fixture(scope="session")
def grpc_stub(grpc_channel):
    """Return a PredictionService gRPC stub."""
    from src.proto.prediction import prediction_pb2_grpc

    return prediction_pb2_grpc.PredictionServiceStub(grpc_channel)


# ---------------------------------------------------------------------------
# HTTP (API Backend) fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(scope="session")
def api_base_url() -> str:
    """Base URL for API Backend.

    When running via `docker exec` inside the prediction container, the API
    backend is reachable at http://api:8118 (Docker network).
    Override with TEST_API_URL env var for local development.
    """
    return os.environ.get("TEST_API_URL", "http://api:8118")


@pytest.fixture(scope="session")
def admin_token(api_base_url: str) -> str:
    """POST /api/auth/login and return the JWT token."""
    url = f"{api_base_url}/api/auth/login"
    # Credentials come from the same env the auth-svc seeder reads — no hardcoded admin.
    payload = {
        "username": os.environ.get("SUPER_ADMIN_USERNAME", "chon"),
        "password": os.environ.get("SUPER_ADMIN_PASSWORD", ""),
    }

    for attempt in range(3):
        try:
            resp = requests.post(url, json=payload, timeout=10)
            if resp.status_code == 200:
                data = resp.json()
                token = data.get("token", "")
                if token:
                    return token
        except requests.RequestException:
            pass
        if attempt < 2:
            time.sleep(2)

    return ""


@pytest.fixture(scope="session")
def auth_headers(admin_token: str) -> dict:
    return {"Authorization": f"Bearer {admin_token}"}
