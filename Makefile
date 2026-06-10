.PHONY: test test-unit test-integration test-coverage build test-db-up test-db-down vet swagger test-phase5

build:
	cd api && go build -o api-server ./cmd

vet:
	cd api && go vet ./...

test:
	cd api && go test ./... -v -count=1

test-unit:
	cd api && go test ./... -v -count=1 -short

test-integration:
	cd api && go test ./... -v -count=1 -run Integration

test-coverage:
	cd api && go test ./... -coverprofile=coverage.out -count=1
	cd api && go tool cover -html=coverage.out -o coverage.html
	cd api && go tool cover -func=coverage.out | grep total

test-db-up:
	docker compose -f docker-compose.test.yml up -d
	@echo "Waiting for test MySQL to be ready..."
	@for i in $$(seq 1 30); do \
		docker compose -f docker-compose.test.yml exec -T test-mysql mysqladmin ping -h localhost -u root -ptest123 --silent 2>/dev/null && break; \
		sleep 1; \
	done
	@echo "Test DB is ready on port 3307"

test-db-down:
	docker compose -f docker-compose.test.yml down -v

# Phase 5 full regression — Python prediction service + API Backend end-to-end
test-phase5:
	cd prediction && make test-phase5

reset:
	docker compose down
	docker compose up -d --build

up:
	docker compose up -d

swagger:
	cd api && swag init -g cmd/main.go -o docs/
	@echo "Swagger docs generated at api/docs/"
	@echo "UI available at http://localhost:8118/swagger/"
