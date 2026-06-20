# ---------------------------------------------------------------------------
# Makefile — Python Prediction Service
# ---------------------------------------------------------------------------
# Run from prediction/ directory.

.PHONY: help proto install run lint test-unit test-integration test-phase1 test-phase2 test-phase3 test-phase4 test-phase5 test-simulation clean

# Default target
help:
	@echo "Available targets:"
	@echo "  make proto             Generate protobuf Python stubs"
	@echo "  make install           Install dependencies (including dev extras)"
	@echo "  make run               Start the prediction service"
	@echo "  make lint              Run ruff linter"
	@echo "  make test-unit         Run unit tests only"
	@echo "  make test-integration  Run all integration tests"
	@echo "  make test-phase1       Run Phase 1 gRPC + API backend integration tests"
	@echo "  make test-phase2       Run Phase 2 crawler integration tests"
	@echo "  make test-phase3       Run Phase 3 algorithm unit + prediction integration tests"
	@echo "  make test-phase4       Run Phase 4 training + scheduler integration tests"
	@echo "  make test-phase5       Run Phase 5 full regression + performance benchmarks"
	@echo "  make test-simulation   Run trading simulation tests (unit + integration)"
	@echo "  make clean             Remove generated proto files and __pycache__"

# ---------------------------------------------------------------------------
# Proto generation
# ---------------------------------------------------------------------------
proto:
	@echo "Generating protobuf stubs..."
	bash scripts/gen_proto.sh

# ---------------------------------------------------------------------------
# Dependencies
# ---------------------------------------------------------------------------
install:
	pip install -e ".[dev]"

# ---------------------------------------------------------------------------
# Run service
# ---------------------------------------------------------------------------
run:
	python -m src.main

# ---------------------------------------------------------------------------
# Linting
# ---------------------------------------------------------------------------
lint:
	ruff check src/ tests/

# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------
test-unit:
	pytest tests/unit/ -v

test-integration:
	pytest tests/integration/ -v

test-phase1:
	docker exec prediction-svc python -m pytest tests/integration/test_phase1_grpc.py tests/integration/test_phase1_api_backend.py -v

test-phase2:
	docker exec prediction-svc python -m pytest tests/integration/test_phase2_crawlers.py tests/integration/test_phase2_api_backend.py -v

test-phase3:
	docker exec prediction-svc python -m pytest tests/unit/ tests/integration/test_phase3_predictions.py tests/integration/test_phase3_api_backend.py -v

test-phase4:
	docker exec prediction-svc python -m pytest tests/unit/ tests/integration/test_phase4_training.py tests/integration/test_phase4_api_backend.py -v

test-phase5:
	docker cp tests/integration/test_phase5_full_regression.py prediction-svc:/app/tests/integration/
	docker exec prediction-svc python -m pytest tests/integration/test_phase5_full_regression.py -v

test-simulation:
	docker exec prediction-svc python -m pytest tests/unit/test_simulation_portfolio.py tests/unit/test_simulation_metrics.py tests/unit/test_simulation_signal.py tests/integration/test_phase5_simulation.py -v

# ---------------------------------------------------------------------------
# Clean
# ---------------------------------------------------------------------------
clean:
	find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	find . -name "*.pyc" -delete 2>/dev/null || true
	rm -f src/proto/prediction/prediction_pb2.py
	rm -f src/proto/prediction/prediction_pb2_grpc.py
	@echo "Clean complete."
