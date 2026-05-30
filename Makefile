.PHONY: test test-unit test-integration test-coverage build test-db-up test-db-down vet

build:
	go build -o go-stock-prediction ./cmd/app

vet:
	go vet ./...

test:
	go test ./... -v -count=1

test-unit:
	go test ./... -v -count=1 -short

test-integration:
	go test ./... -v -count=1 -run Integration

test-coverage:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | grep total

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
