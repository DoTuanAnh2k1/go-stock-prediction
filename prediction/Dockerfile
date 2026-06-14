# ---------------------------------------------------------------------------
# Stage 1: Generate protobuf Python files
# ---------------------------------------------------------------------------
FROM python:3.12-slim AS proto-builder

RUN pip install --no-cache-dir "grpcio-tools>=1.71.0" "protobuf>=6.33.5"

WORKDIR /app

# Copy proto file from repo root (build context must be the repo root)
COPY api/proto/prediction/prediction.proto api/proto/prediction/prediction.proto

RUN mkdir -p src/proto/prediction && \
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
        src/proto/prediction/prediction_pb2_grpc.py

# ---------------------------------------------------------------------------
# Stage 2: Runtime
# ---------------------------------------------------------------------------
FROM python:3.12-slim

# System deps for lxml and potential native extensions
RUN apt-get update && apt-get install -y --no-install-recommends \
    libxml2 \
    libxslt1.1 \
    default-mysql-client \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Install Python dependencies (split for layer caching)
COPY prediction/pyproject.toml pyproject.toml
RUN pip install --no-cache-dir -e ".[dev,ml]"

# Copy pre-generated proto stubs (committed to repo alongside prediction.proto)
# To regenerate: python -m grpc_tools.protoc -Iapi/proto --python_out=prediction/src/proto
#                --grpc_python_out=prediction/src/proto api/proto/prediction/prediction.proto
#                then fix import: sed -i 's/from prediction import/from src.proto.prediction import/' ...
COPY prediction/src/proto src/proto/

# Copy application source and tests
COPY prediction/src/ src/
COPY prediction/tests/ tests/

# Run as non-root user
RUN useradd -m -u 1000 appuser && chown -R appuser:appuser /app
USER appuser

EXPOSE 8119

CMD ["python", "-m", "src.main"]
