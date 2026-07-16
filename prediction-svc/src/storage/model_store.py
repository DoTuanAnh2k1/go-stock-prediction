"""Model checkpoint store — abstraction over local filesystem and S3/MinIO.

Backend chosen via env MODEL_STORE_BACKEND:
  local  (default) — read/write directly from RL_MODEL_DIR.
  s3              — read-through cache: local RL_MODEL_DIR acts as ephemeral
                    cache (emptyDir in k8s); on load, file is downloaded from
                    S3 if not present locally; on save, file is written locally
                    then uploaded to S3.

S3 env vars (required when backend=s3):
  S3_ENDPOINT   = http://minio.stock:9000
  S3_BUCKET     = models
  S3_ACCESS_KEY
  S3_SECRET_KEY

Usage pattern (minimal invasiveness — local backend = identical to old code):

    from src.storage.model_store import get_store
    store = get_store()

    # Before torch.load(path, ...):
    local_path = store.ensure_local(path)       # may download from S3
    data = torch.load(local_path, ...)

    # After torch.save({...}, path):
    store.upload_if_remote(path)                # no-op for local backend

Fail-safe: S3 errors are caught and logged; the local file is still used.
"""
from __future__ import annotations

import os
from pathlib import Path

from src.utils.logger import get_logger

log = get_logger("model_store")

# ---------------------------------------------------------------------------
# Backend detection
# ---------------------------------------------------------------------------

_BACKEND_LOCAL = "local"
_BACKEND_S3 = "s3"


def _backend() -> str:
    return os.environ.get("MODEL_STORE_BACKEND", _BACKEND_LOCAL).lower().strip()


# ---------------------------------------------------------------------------
# S3 client (lazy — import boto3 only when needed)
# ---------------------------------------------------------------------------

def _s3_client():
    """Build a boto3 S3 client from env vars. Raises on misconfiguration."""
    import boto3  # noqa: PLC0415  (lazy import — not a mandatory dep for local backend)
    endpoint = os.environ.get("S3_ENDPOINT", "http://minio.stock:9000")
    bucket = os.environ.get("S3_BUCKET", "models")
    access_key = os.environ.get("S3_ACCESS_KEY", "")
    secret_key = os.environ.get("S3_SECRET_KEY", "")
    client = boto3.client(
        "s3",
        endpoint_url=endpoint,
        aws_access_key_id=access_key,
        aws_secret_access_key=secret_key,
        # MinIO does not require region; set a placeholder so boto3 is happy.
        region_name="us-east-1",
    )
    return client, bucket


def _s3_key(local_path: str) -> str:
    """Derive S3 object key from local path (strip leading RL_MODEL_DIR prefix)."""
    model_dir = os.environ.get("RL_MODEL_DIR", "/models").rstrip("/")
    local = str(local_path)
    if local.startswith(model_dir + "/"):
        return local[len(model_dir) + 1:]
    # Fallback: use just the filename.
    return Path(local).name


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

class ModelStore:
    """Checkpoint store.  All methods are fail-safe (never crash the service)."""

    def __init__(self, backend: str) -> None:
        self._backend = backend
        if backend not in (_BACKEND_LOCAL, _BACKEND_S3):
            log.warning("model_store.unknown_backend", backend=backend, fallback=_BACKEND_LOCAL)
            self._backend = _BACKEND_LOCAL
        log.info("model_store.init", backend=self._backend)

    # ------------------------------------------------------------------
    # Core helpers
    # ------------------------------------------------------------------

    def ensure_local(self, path: str) -> str:
        """Return a local path that is guaranteed to exist (downloading from
        S3 if needed).  For local backend — returns *path* unchanged.

        Callers can pass the result directly to torch.load() / pickle.load().
        """
        if self._backend == _BACKEND_LOCAL:
            return path

        # S3 backend: download on cache miss
        if os.path.exists(path):
            return path

        try:
            client, bucket = _s3_client()
            key = _s3_key(path)
            os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
            log.info("model_store.s3.download", key=key, bucket=bucket, local=path)
            client.download_file(bucket, key, path)
        except Exception as exc:
            log.warning("model_store.s3.download.failed", path=path, error=str(exc))

        return path  # caller must handle FileNotFoundError if still missing

    def upload_if_remote(self, path: str) -> None:
        """Upload *path* to S3 (after it has been written locally).
        No-op for local backend.
        """
        if self._backend == _BACKEND_LOCAL:
            return

        if not os.path.exists(path):
            log.warning("model_store.upload.file_missing", path=path)
            return

        try:
            client, bucket = _s3_client()
            key = _s3_key(path)
            log.info("model_store.s3.upload", key=key, bucket=bucket, local=path)
            client.upload_file(path, bucket, key)
        except Exception as exc:
            log.warning("model_store.s3.upload.failed", path=path, error=str(exc))

    # ------------------------------------------------------------------
    # Convenience: raw bytes I/O (alternative higher-level API)
    # ------------------------------------------------------------------

    def save_bytes(self, key: str, data: bytes) -> None:
        """Write bytes to local RL_MODEL_DIR/<key> and upload if S3."""
        model_dir = os.environ.get("RL_MODEL_DIR", "/models")
        local_path = os.path.join(model_dir, key)
        os.makedirs(os.path.dirname(local_path), exist_ok=True)
        with open(local_path, "wb") as fh:
            fh.write(data)
        self.upload_if_remote(local_path)

    def load_bytes(self, key: str) -> bytes | None:
        """Load bytes from local cache (downloading from S3 if missing).
        Returns None when not found anywhere.
        """
        model_dir = os.environ.get("RL_MODEL_DIR", "/models")
        local_path = os.path.join(model_dir, key)
        local_path = self.ensure_local(local_path)
        if not os.path.exists(local_path):
            return None
        with open(local_path, "rb") as fh:
            return fh.read()


# ---------------------------------------------------------------------------
# Singleton
# ---------------------------------------------------------------------------

_store: ModelStore | None = None


def get_store() -> ModelStore:
    global _store
    if _store is None:
        _store = ModelStore(backend=_backend())
    return _store
