.PHONY: test test-unit test-integration test-coverage build

build:
	go build -o go-stock-prediction ./cmd/app

test:
	go test ./... -v -count=1

test-unit:
	go test ./... -v -count=1 -short

test-integration:
	go test ./... -v -count=1 -tags integration

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | grep total
