.PHONY: test build run clean install deps docker-build docker-up

# Build the server
build:
	go build -o bin/server ./cmd/server

# Run the server
run:
	go run ./cmd/server

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run all tests
test:
	./test_runner.sh

# Run specific test packages
test-bits:
	go test -v ./internal/bits/...

test-geo:
	go test -v ./internal/geo/...

test-rate:
	go test -v ./internal/rate/...

test-redis:
	go test -v ./internal/redis/...

test-turnstile:
	go test -v ./internal/turnstile/...

test-ws:
	go test -v ./internal/ws/...

test-api:
	go test -v ./internal/api/...

test-integration:
	go test -v ./internal/integration/...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out -o coverage.html

# Run tests with race detection
test-race:
	go test -race ./internal/...

# Run benchmarks
bench:
	go test -bench=. -benchmem ./internal/...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Docker build
docker-build:
	docker build -t splat-boston:latest .

# Docker compose up
docker-up:
	docker-compose up -d

# Docker compose down
docker-down:
	docker-compose down

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Install tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

