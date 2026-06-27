# ---------------------------------------------------------------------------
# Stage 1: Generate protobuf Python files
# ---------------------------------------------------------------------------
FROM python:3.12-slim AS proto-builder

RUN pip install --no-cache-dir "grpcio-tools>=1.71.0" "protobuf>=6.33.5"

WORKDIR /app

# Copy proto files from repo root (build context must be the repo root).
# Source lives in api-svc/proto and service-mgt/proto; staged at api/proto inside the image for protoc.
COPY api-svc/proto/prediction/prediction.proto api/proto/prediction/prediction.proto
COPY service-mgt/proto/registry/registry.proto api/proto/registry/registry.proto

RUN mkdir -p src/proto/prediction src/proto/registry && \
    # Generate prediction stubs
    python -m grpc_tools.protoc \
        -Iapi/proto \
        --python_out=src/proto \
        --grpc_python_out=src/proto \
        api/proto/prediction/prediction.proto && \
    touch src/__init__.py && \
    touch src/proto/__init__.py && \
    touch src/proto/prediction/__init__.py && \
    # Fix relative imports in generated grpc file to use absolute package path
    sed -i 's/from prediction import prediction_pb2/from src.proto.prediction import prediction_pb2/' \
        src/proto/prediction/prediction_pb2_grpc.py && \
    # Generate registry stubs
    python -m grpc_tools.protoc \
        -Iapi/proto \
        --python_out=src/proto \
        --grpc_python_out=src/proto \
        api/proto/registry/registry.proto && \
    touch src/proto/registry/__init__.py && \
    # Fix relative imports in generated registry grpc file
    sed -i 's/from registry import registry_pb2/from src.proto.registry import registry_pb2/' \
        src/proto/registry/registry_pb2_grpc.py

# ---------------------------------------------------------------------------
# Stage 2: Runtime
# ---------------------------------------------------------------------------
FROM python:3.12-slim

ARG GIT_SHA=unknown
ARG BUILD_TIME=unknown
ARG GIT_DIRTY=unknown
ENV GIT_SHA=$GIT_SHA BUILD_TIME=$BUILD_TIME GIT_DIRTY=$GIT_DIRTY

# System deps for lxml and potential native extensions
RUN apt-get update && apt-get install -y --no-install-recommends \
    libxml2 \
    libxslt1.1 \
    default-mysql-client \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Install Python dependencies (split for layer caching)
COPY prediction-svc/pyproject.toml pyproject.toml
RUN pip install --no-cache-dir -e ".[dev,ml]"

# Copy pre-generated proto stubs (committed to repo alongside prediction.proto)
# To regenerate: python -m grpc_tools.protoc -Iapi-svc/proto --python_out=prediction-svc/src/proto
#                --grpc_python_out=prediction-svc/src/proto api-svc/proto/prediction/prediction.proto
#                then fix import: sed -i 's/from prediction import/from src.proto.prediction import/' ...
COPY prediction-svc/src/proto src/proto/

# Copy application source and tests
COPY prediction-svc/src/ src/
COPY prediction-svc/tests/ tests/

# Create models directory with correct permissions before switching to non-root user
RUN useradd -m -u 1000 appuser && \
    mkdir -p /models && \
    chown -R appuser:appuser /app /models
USER appuser

EXPOSE 8119

CMD ["python", "-m", "src.main"]
