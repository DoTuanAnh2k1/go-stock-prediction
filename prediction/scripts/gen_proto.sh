#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# gen_proto.sh — Generate Python protobuf stubs for local development.
#
# Run from the prediction/ directory:
#   ./scripts/gen_proto.sh
#
# Or from the repo root:
#   bash prediction/scripts/gen_proto.sh
#
# Requirements:
#   pip install grpcio-tools==1.70.0 protobuf==5.29.4
# ---------------------------------------------------------------------------
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
REPO_ROOT="$(dirname "$SERVICE_DIR")"

PROTO_SRC="${REPO_ROOT}/proto"
PROTO_FILE="prediction/prediction.proto"
OUT_DIR="${SERVICE_DIR}/src/proto"

echo "Repo root : ${REPO_ROOT}"
echo "Proto src : ${PROTO_SRC}"
echo "Output dir: ${OUT_DIR}"

# Ensure output directories exist
mkdir -p "${OUT_DIR}/prediction"

# Generate Python + gRPC stubs
python3 -m grpc_tools.protoc \
    -I"${PROTO_SRC}" \
    --python_out="${OUT_DIR}" \
    --grpc_python_out="${OUT_DIR}" \
    "${PROTO_SRC}/${PROTO_FILE}"

# Create __init__.py files so the generated code is importable as a package
touch "${OUT_DIR}/__init__.py"
touch "${OUT_DIR}/prediction/__init__.py"

# Fix relative import in generated grpc file:
#   protoc generates:  from prediction import prediction_pb2
#   We need:           from src.proto.prediction import prediction_pb2
GRPC_FILE="${OUT_DIR}/prediction/prediction_pb2_grpc.py"
if [ -f "${GRPC_FILE}" ]; then
    sed -i 's/from prediction import prediction_pb2/from src.proto.prediction import prediction_pb2/' "${GRPC_FILE}"
    echo "Fixed import in ${GRPC_FILE}"
fi

echo ""
echo "Proto generation complete. Files written to:"
ls -1 "${OUT_DIR}/prediction/"
