.PHONY: test test-unit test-integration test-coverage build test-db-up test-db-down vet swagger test-phase5 up reset

# docker compose wrapper — compose file lives in deploy/, .env stays at repo root
COMPOSE = docker compose --env-file .env -f deploy/docker-compose.yaml
COMPOSE_TEST = docker compose -f deploy/docker-compose.test.yml

build:
	cd api-svc && go build -o api-server ./cmd

vet:
	cd api-svc && go vet ./...

test:
	cd api-svc && go test ./... -v -count=1

test-unit:
	cd api-svc && go test ./... -v -count=1 -short

test-integration:
	cd api-svc && go test ./... -v -count=1 -run Integration

test-coverage:
	cd api-svc && go test ./... -coverprofile=coverage.out -count=1
	cd api-svc && go tool cover -html=coverage.out -o coverage.html
	cd api-svc && go tool cover -func=coverage.out | grep total

test-db-up:
	$(COMPOSE_TEST) up -d
	@echo "Waiting for test MySQL to be ready..."
	@for i in $$(seq 1 30); do \
		$(COMPOSE_TEST) exec -T test-mysql mysqladmin ping -h localhost -u root -ptest123 --silent 2>/dev/null && break; \
		sleep 1; \
	done
	@echo "Test DB is ready on port 3307"

test-db-down:
	$(COMPOSE_TEST) down -v

# Phase 5 full regression — Python prediction service + API Backend end-to-end
test-phase5:
	cd prediction-svc && make test-phase5

reset:
	$(COMPOSE) down
	$(COMPOSE) up -d --build

up:
	$(COMPOSE) up -d

swagger:
	cd api-svc && swag init -g cmd/main.go -o docs/
	@echo "Swagger docs generated at api-svc/docs/"
	@echo "UI available at http://localhost:8118/swagger/"
