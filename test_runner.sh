#!/bin/bash

# Test runner script for the r/place-style backend
# This script runs all unit tests and integration tests

set -e

echo "🧪 Running r/place-style backend tests..."
echo "========================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go to run tests."
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | cut -d' ' -f3 | cut -d'.' -f2)
if [ "$GO_VERSION" -lt 19 ]; then
    print_warning "Go version 1.19+ is recommended for best test performance"
fi

# Set test environment variables
export TEST_ENV=true
export REDIS_URL=redis://localhost:6379/1
export BIND_ADDR=:8080
export BOSTON_MASK_PATH=./data/boston_mask.bin
export PALETTE=000000,FF0000,FFA500,FFFF00,00FF00,00FFFF,0000FF,FF00FF,FFFFFF
export PAINT_COOLDOWN_MS=5000
export GEOFENCE_RADIUS_M=300
export SPEED_MAX_KMH=150
export ENABLE_TURNSTILE=false
export TURNSTILE_SECRET=test_secret
export WS_WRITE_BUFFER=1048576
export WS_PING_INTERVAL_S=20

print_status "Setting up test environment..."

# Create test data directory if it doesn't exist
mkdir -p data

# Check if Redis is available for integration tests
if command -v redis-cli &> /dev/null; then
    if redis-cli ping &> /dev/null; then
        print_success "Redis is available for integration tests"
        RUN_INTEGRATION_TESTS=true
    else
        print_warning "Redis is not running. Integration tests will be skipped."
        RUN_INTEGRATION_TESTS=false
    fi
else
    print_warning "Redis CLI not found. Integration tests will be skipped."
    RUN_INTEGRATION_TESTS=false
fi

# Run tests
print_status "Running unit tests..."

# Test coordinate math
print_status "Testing coordinate math functions..."
go test -v ./internal/geo/... -run TestLatLonToTileXY
go test -v ./internal/geo/... -run TestChunkOf
go test -v ./internal/geo/... -run TestOffsetOf
go test -v ./internal/geo/... -run TestCoordinateRoundTrip
print_success "Coordinate math tests passed"

# Test bit operations
print_status "Testing bit manipulation and nibble packing..."
go test -v ./internal/bits/... -run TestNibblePacking
go test -v ./internal/bits/... -run TestNibbleOverwrite
go test -v ./internal/bits/... -run TestNibbleBounds
go test -v ./internal/bits/... -run TestChunkFullCycle
print_success "Bit operations tests passed"

# Test geofence and mask operations
print_status "Testing geofence and mask operations..."
go test -v ./internal/geo/... -run TestMaskOperations
go test -v ./internal/geo/... -run TestHaversineDistance
go test -v ./internal/geo/... -run TestGeofenceRadius
go test -v ./internal/geo/... -run TestSpeedClamp
print_success "Geofence and mask tests passed"

# Test Redis operations
if [ "$RUN_INTEGRATION_TESTS" = true ]; then
    print_status "Testing Redis operations and Lua scripts..."
    go test -v ./internal/redis/... -run TestRedisPaintScript
    go test -v ./internal/redis/... -run TestRedisPaintOverwrite
    go test -v ./internal/redis/... -run TestRedisChunkInitialization
    go test -v ./internal/redis/... -run TestRedisSequenceIncrement
    go test -v ./internal/redis/... -run TestRedisCooldown
    print_success "Redis operations tests passed"
else
    print_warning "Skipping Redis tests (Redis not available)"
fi

# Test API handlers
print_status "Testing API handlers..."
go test -v ./internal/api/... -run TestGetChunk
go test -v ./internal/api/... -run TestPostPaint
go test -v ./internal/api/... -run TestPostPaintTurnstile
go test -v ./internal/api/... -run TestPostPaintGeofence
go test -v ./internal/api/... -run TestPostPaintCooldown
print_success "API handler tests passed"

# Test WebSocket hub
print_status "Testing WebSocket hub functionality..."
go test -v ./internal/ws/... -run TestHubBasicOperations
go test -v ./internal/ws/... -run TestHubPublish
go test -v ./internal/ws/... -run TestRoomSubscriberManagement
go test -v ./internal/ws/... -run TestRoomBroadcast
go test -v ./internal/ws/... -run TestWebSocketConnection
print_success "WebSocket hub tests passed"

# Test rate limiting
print_status "Testing cooldown and rate limiting..."
go test -v ./internal/rate/... -run TestCooldownLimiter
go test -v ./internal/rate/... -run TestSpeedLimiter
go test -v ./internal/rate/... -run TestRateLimiter
go test -v ./internal/rate/... -run TestCombinedLimiters
print_success "Rate limiting tests passed"

# Test Turnstile verification
print_status "Testing Turnstile verification..."
go test -v ./internal/turnstile/... -run TestTurnstileVerification
go test -v ./internal/turnstile/... -run TestTurnstileVerificationErrorCodes
go test -v ./internal/turnstile/... -run TestTurnstileVerificationConcurrency
print_success "Turnstile verification tests passed"

# Test integration workflow
if [ "$RUN_INTEGRATION_TESTS" = true ]; then
    print_status "Testing end-to-end paint workflow..."
    go test -v ./internal/integration/... -run TestPaintWorkflow
    go test -v ./internal/integration/... -run TestPaintWorkflowCooldown
    go test -v ./internal/integration/... -run TestPaintWorkflowGeofence
    go test -v ./internal/integration/... -run TestPaintWorkflowTurnstile
    go test -v ./internal/integration/... -run TestPaintWorkflowSequence
    go test -v ./internal/integration/... -run TestPaintWorkflowWebSocket
    print_success "Integration tests passed"
else
    print_warning "Skipping integration tests (Redis not available)"
fi

# Run performance benchmarks
print_status "Running performance benchmarks..."
go test -bench=. ./internal/geo/... -benchmem
go test -bench=. ./internal/bits/... -benchmem
go test -bench=. ./internal/rate/... -benchmem
go test -bench=. ./internal/turnstile/... -benchmem
if [ "$RUN_INTEGRATION_TESTS" = true ]; then
    go test -bench=. ./internal/redis/... -benchmem
    go test -bench=. ./internal/integration/... -benchmem
fi
print_success "Performance benchmarks completed"

# Run race detection tests
print_status "Running race detection tests..."
go test -race ./internal/geo/...
go test -race ./internal/bits/...
go test -race ./internal/rate/...
go test -race ./internal/turnstile/...
go test -race ./internal/ws/...
if [ "$RUN_INTEGRATION_TESTS" = true ]; then
    go test -race ./internal/redis/...
    go test -race ./internal/integration/...
fi
print_success "Race detection tests passed"

# Run coverage analysis
print_status "Running test coverage analysis..."
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out -o coverage.html
print_success "Coverage report generated: coverage.html"

# Run all tests together
print_status "Running comprehensive test suite..."
go test -v ./internal/... -timeout=30s
print_success "All tests passed!"

echo ""
echo "🎉 Test suite completed successfully!"
echo "========================================"
echo "📊 Coverage report: coverage.html"
echo "🔍 Race detection: All tests passed"
echo "⚡ Performance benchmarks: Completed"
echo "🧪 Unit tests: All passed"
if [ "$RUN_INTEGRATION_TESTS" = true ]; then
    echo "🔗 Integration tests: All passed"
else
    echo "⚠️  Integration tests: Skipped (Redis not available)"
fi
echo ""
echo "✅ Ready for production deployment!"
