# ===========================================================================
# go-stock-prediction — root Makefile
# ---------------------------------------------------------------------------
# Microservice stack: api-svc (Go) · prediction-svc (Python) · auth-svc (Java)
#                     gateway-svc (Rust) · web-svc (React) · cli-svc (Go)
# Run all targets from the repo root. `.env` lives here; compose file is in deploy/.
# `make help` lists everything.
# ===========================================================================

.DEFAULT_GOAL := help

# --- docker compose wrappers (compose files in deploy/, .env at repo root) ---
COMPOSE      = docker compose --env-file .env -f deploy/docker-compose.yaml
COMPOSE_TEST = docker compose -f deploy/docker-compose.test.yml

# --- Version stamping: baked into every image at build time via build args. ---
# Exported so `docker compose build` interpolation picks them up (shell env wins
# over .env). Each service writes /versions/<svc>.json + a startup `version ...`
# log line; api-svc aggregates them at GET /api/version. `make versions` reads it.
# GIT_DIRTY=true when the working tree has ANY uncommitted/untracked change.
export GIT_SHA    := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
export BUILD_TIME := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GIT_DIRTY  := $(shell test -z "$$(git status --porcelain 2>/dev/null)" && echo false || echo true)

.PHONY: help \
        up down reset restart rebuild ps logs \
        build test test-unit test-integration test-coverage test-phase5 \
        api-build api-test api-test-unit api-test-integration api-test-coverage api-vet api-swagger swagger vet \
        pred-install pred-run pred-lint pred-test pred-test-unit pred-test-phase5 \
        auth-build auth-test \
        gateway-build gateway-test gateway-check gateway-fmt \
        web-install web-build web-dev web-preview \
        cli-build cli-test \
        proto-go proto-auth proto-py proto \
        test-db-up test-db-down \
        stats stats-watch \
        versions versions-logs

# ---------------------------------------------------------------------------
help:
	@echo "go-stock-prediction — make targets"
	@echo ""
	@echo "Docker stack (deploy/docker-compose.yaml):"
	@echo "  make up               Start the full stack (detached)"
	@echo "  make down             Stop the full stack"
	@echo "  make reset            down + up --build (rebuild all images)"
	@echo "  make rebuild SVC=...  Rebuild + restart one service (e.g. SVC=api-svc)"
	@echo "  make restart SVC=...  Restart one service without rebuild"
	@echo "  make ps               Show container status"
	@echo "  make logs [SVC=...]   Tail logs (all, or one service)"
	@echo ""
	@echo "api-svc (Go, :8118):"
	@echo "  make api-build        Build api-server binary"
	@echo "  make api-test         Run all Go tests"
	@echo "  make api-test-unit    Run -short tests only"
	@echo "  make api-test-coverage  Generate coverage.html"
	@echo "  make api-vet          go vet"
	@echo "  make api-swagger      Regenerate Swagger docs"
	@echo ""
	@echo "prediction-svc (Python, gRPC :8119):"
	@echo "  make pred-install     pip install -e .[dev]"
	@echo "  make pred-run         Run the service locally"
	@echo "  make pred-lint        ruff check"
	@echo "  make pred-test        pytest unit + integration (delegates to prediction-svc/Makefile)"
	@echo "  make pred-test-unit   pytest unit only"
	@echo "  make pred-test-phase5 Full regression in container"
	@echo ""
	@echo "auth-svc (Java/Maven, gRPC :8120):"
	@echo "  make auth-build       mvn package (skip tests)"
	@echo "  make auth-test        mvn test"
	@echo ""
	@echo "gateway-svc (Rust/Cargo, :80/:443):"
	@echo "  make gateway-build    cargo build --release"
	@echo "  make gateway-test     cargo test"
	@echo "  make gateway-check    cargo check + clippy"
	@echo "  make gateway-fmt      cargo fmt"
	@echo ""
	@echo "web-svc (React/Vite, :3000):"
	@echo "  make web-install      npm install"
	@echo "  make web-build        tsc -b + vite build"
	@echo "  make web-dev          vite dev server"
	@echo ""
	@echo "cli-svc (Go SSH, :2345):"
	@echo "  make cli-build        Build cli-server binary"
	@echo "  make cli-test         Run cli-svc Go tests"
	@echo ""
	@echo "Proto regeneration:"
	@echo "  make proto-go         Go stubs (prediction + auth)"
	@echo "  make proto-auth       Go auth stubs only"
	@echo "  make proto-py         Python prediction stubs"
	@echo ""
	@echo "Resource metrics (scripts/svc-metrics.sh):"
	@echo "  make stats            One-shot CPU/RAM snapshot sorted by memory desc + TOTAL"
	@echo "  make stats-watch      Auto-refresh every 3 s (log-safe; Ctrl-C to stop)"
	@echo "  bash scripts/svc-metrics.sh --json    NDJSON lines for scripting"
	@echo ""
	@echo "Version stamping (GIT_SHA/BUILD_TIME/GIT_DIRTY baked at build):"
	@echo "  make versions         Query GET /api/version (running stack) — per-service SHA + drift"
	@echo "  make versions-logs    Fallback: grep the 'version ...' startup log line per container"
	@echo ""
	@echo "Aggregate / legacy aliases:"
	@echo "  make build  make test  make test-unit  make swagger  make vet  make test-phase5"

# ===========================================================================
# Docker stack
# ===========================================================================
up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

reset:
	$(COMPOSE) down
	$(COMPOSE) up -d --build

ps:
	$(COMPOSE) ps

# Tail logs — all services, or one via `make logs SVC=api-svc`
logs:
	$(COMPOSE) logs -f --tail=200 $(SVC)

# Rebuild + restart a single service: `make rebuild SVC=api-svc`
rebuild:
	@test -n "$(SVC)" || (echo "Usage: make rebuild SVC=<service>"; exit 1)
	$(COMPOSE) up -d --build $(SVC)

# Restart a single service without rebuild: `make restart SVC=api-svc`
restart:
	@test -n "$(SVC)" || (echo "Usage: make restart SVC=<service>"; exit 1)
	$(COMPOSE) restart $(SVC)

# ===========================================================================
# api-svc — Go HTTP API Backend (:8118)
# ===========================================================================
api-build:
	cd api-svc && go build -o api-server ./cmd

api-vet:
	cd api-svc && go vet ./...

api-test:
	cd api-svc && go test ./... -v -count=1

api-test-unit:
	cd api-svc && go test ./... -v -count=1 -short

api-test-integration:
	cd api-svc && go test ./... -v -count=1 -run Integration

api-test-coverage:
	cd api-svc && go test ./... -coverprofile=coverage.out -count=1
	cd api-svc && go tool cover -html=coverage.out -o coverage.html
	cd api-svc && go tool cover -func=coverage.out | grep total

api-swagger:
	cd api-svc && swag init -g cmd/main.go -o docs/ --parseInternal --parseDependency
	@echo "Swagger docs generated at api-svc/docs/"
	@echo "UI available at http://localhost:8118/swagger/"

# ===========================================================================
# prediction-svc — Python gRPC service (:8119)
# ===========================================================================
pred-install:
	cd prediction-svc && make install

pred-run:
	cd prediction-svc && make run

pred-lint:
	cd prediction-svc && make lint

pred-test:
	cd prediction-svc && make test-unit && cd prediction-svc && make test-integration

pred-test-unit:
	cd prediction-svc && make test-unit

pred-test-phase5:
	cd prediction-svc && make test-phase5

# ===========================================================================
# auth-svc — Java Spring Boot Auth Service (gRPC :8120)
# ===========================================================================
auth-build:
	cd auth-svc && mvn -q package -DskipTests

auth-test:
	cd auth-svc && mvn -q test

# ===========================================================================
# gateway-svc — Rust Axum Gateway (:80 / :443)
# ===========================================================================
gateway-build:
	cd gateway-svc && cargo build --release

gateway-test:
	cd gateway-svc && cargo test

gateway-check:
	cd gateway-svc && cargo check && cargo clippy --all-targets -- -D warnings

gateway-fmt:
	cd gateway-svc && cargo fmt

# ===========================================================================
# web-svc — React + Vite SPA (:3000)
# ===========================================================================
web-install:
	cd web-svc && npm install

web-build:
	cd web-svc && npm run build

web-dev:
	cd web-svc && npm run dev

web-preview:
	cd web-svc && npm run preview

# ===========================================================================
# cli-svc — Go SSH CLI Service (:2345)
# ===========================================================================
cli-build:
	cd cli-svc && go build -o cli-server .

cli-test:
	cd cli-svc && go test ./... -count=1

# ===========================================================================
# Proto regeneration (see CLAUDE.md — never hand-edit generated stubs)
# ===========================================================================
# Go stubs for both prediction.proto and auth.proto
proto-go: proto-auth
	cd api-svc && protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/prediction/prediction.proto

proto-auth:
	cd api-svc && protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/auth/auth.proto

# Python prediction stubs (delegates to prediction-svc/Makefile -> scripts/gen_proto.sh)
proto-py:
	cd prediction-svc && make proto

# ===========================================================================
# Test DB helper (legacy — docker-compose.test.yml)
# ===========================================================================
test-db-up:
	$(COMPOSE_TEST) up -d
	@echo "Waiting for test DB to be ready..."
	@for i in $$(seq 1 30); do \
		$(COMPOSE_TEST) exec -T test-mysql mysqladmin ping -h localhost -u root -ptest123 --silent 2>/dev/null && break; \
		sleep 1; \
	done
	@echo "Test DB is ready on port 3307"

test-db-down:
	$(COMPOSE_TEST) down -v

# ===========================================================================
# Backward-compatible aliases (old short names still work)
# ===========================================================================
build:        api-build
vet:          api-vet
test:         api-test
test-unit:    api-test-unit
test-integration: api-test-integration
test-coverage: api-test-coverage
swagger:      api-swagger
proto:        proto-go
test-phase5:  pred-test-phase5

# ===========================================================================
# Resource metrics — per-container CPU/RAM snapshot (scripts/svc-metrics.sh)
# ===========================================================================
# One-shot snapshot sorted by memory usage descending + TOTAL line.
stats:
	bash scripts/svc-metrics.sh

# Auto-refresh every 3 seconds (log-safe loop, not interactive docker stats).
stats-watch:
	bash scripts/svc-metrics.sh --watch 3

# ===========================================================================
# Version stamping — what code is actually running?
# ===========================================================================
# Hit the aggregated endpoint through the gateway (api-svc has no published port).
# /api/version is public (no auth). Pretty-prints with jq if available, else raw.
# Falls back to scraping container startup logs if the endpoint is unreachable.
versions:
	@echo "GET /api/version (via gateway):"
	@curl -fsSk https://localhost/api/version 2>/dev/null | (jq . 2>/dev/null || cat) \
		|| curl -fsS http://localhost/api/version 2>/dev/null | (jq . 2>/dev/null || cat) \
		|| { echo "endpoint unreachable — falling back to container logs:"; $(MAKE) --no-print-directory versions-logs; }

# Binary-proof fallback: every service logs one `version ...` line at startup.
versions-logs:
	@for s in api-svc prediction-svc auth-svc gateway-svc web-svc; do \
		printf "%-16s" "$$s"; \
		docker logs $$s 2>&1 | grep -m1 'version git_sha=' || echo "(no version log — see /versions/$$s.json)"; \
	done
