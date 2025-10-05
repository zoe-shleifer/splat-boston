#!/bin/bash

# Setup script for Splat Boston development environment
# Checks dependencies and sets up the project

set -e

echo "🚀 Setting up Splat Boston development environment..."

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Check Go
echo ""
echo "Checking Go installation..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | cut -d' ' -f3)
    print_success "Go found: $GO_VERSION"
    
    # Check version
    GO_VERSION_NUM=$(echo $GO_VERSION | sed 's/go//' | cut -d'.' -f1,2)
    if (( $(echo "$GO_VERSION_NUM >= 1.22" | bc -l) )); then
        print_success "Go version is 1.22 or higher"
    else
        print_warning "Go version 1.22+ is recommended (you have $GO_VERSION)"
    fi
else
    print_error "Go not found. Please install Go 1.22+ from https://golang.org/"
    exit 1
fi

# Check Redis
echo ""
echo "Checking Redis installation..."
if command -v redis-server &> /dev/null; then
    REDIS_VERSION=$(redis-server --version | cut -d'=' -f2 | cut -d' ' -f1)
    print_success "Redis found: v$REDIS_VERSION"
else
    print_error "Redis not found. Please install Redis:"
    echo "  macOS: brew install redis"
    echo "  Linux: apt-get install redis-server"
    exit 1
fi

# Check if Redis is running
echo ""
echo "Checking Redis status..."
if redis-cli ping &> /dev/null; then
    print_success "Redis is running"
else
    print_warning "Redis is not running. Start it with: redis-server"
fi

# Check Node.js (optional, for frontend)
echo ""
echo "Checking Node.js installation (optional for frontend)..."
if command -v node &> /dev/null; then
    NODE_VERSION=$(node --version)
    print_success "Node.js found: $NODE_VERSION"
else
    print_warning "Node.js not found (needed only for frontend development)"
fi

# Install Go dependencies
echo ""
echo "Installing Go dependencies..."
go mod download
print_success "Go dependencies installed"

# Build the server
echo ""
echo "Building Go server..."
go build -o bin/server ./cmd/server
print_success "Server built successfully"

# Install frontend dependencies (optional)
if command -v npm &> /dev/null && [ -f "package.json" ]; then
    echo ""
    read -p "Install frontend dependencies? (requires npm, ~478MB) (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        npm install
        print_success "Frontend dependencies installed"
    fi
fi

# Create data directory
echo ""
echo "Setting up data directory..."
mkdir -p data
print_success "Data directory created"

echo ""
echo "🎉 Setup complete!"
echo ""
echo "Next steps:"
echo "  1. Start Redis:     redis-server"
echo "  2. Start server:    ./bin/server"
echo "  3. Test server:     curl http://localhost:8080/healthz"
echo "  4. Open test client: open test-client.html"
echo ""
echo "For more information, see:"
echo "  - QUICKSTART.md for detailed instructions"
echo "  - README.md for project overview"
echo "  - PROJECT_STRUCTURE.md for code organization"

