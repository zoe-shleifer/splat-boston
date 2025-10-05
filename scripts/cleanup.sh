#!/bin/bash

# Cleanup script for Splat Boston project
# Removes build artifacts, temporary files, and cleans up dependencies

set -e

echo "🧹 Cleaning up Splat Boston..."

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Clean Go build artifacts
echo ""
echo "Cleaning Go build artifacts..."
if [ -d "bin" ]; then
    rm -rf bin/
    print_success "Removed bin/ directory"
fi

if [ -f "coverage.out" ]; then
    rm -f coverage.out coverage.html
    print_success "Removed coverage files"
fi

find . -name "*.test" -type f -delete 2>/dev/null && print_success "Removed test binaries" || true

# Clean Redis dumps
echo ""
echo "Cleaning Redis dumps..."
find . -name "*.rdb" -type f -delete 2>/dev/null && print_success "Removed Redis dump files" || true

# Clean logs
echo ""
echo "Cleaning log files..."
find . -name "*.log" -type f -delete 2>/dev/null && print_success "Removed log files" || true

# Clean OS files
echo ""
echo "Cleaning OS-specific files..."
find . -name ".DS_Store" -type f -delete 2>/dev/null && print_success "Removed .DS_Store files" || true

# Clean IDE files
echo ""
echo "Cleaning IDE files..."
rm -rf .idea .vscode 2>/dev/null && print_success "Removed IDE directories" || true
find . -name "*.swp" -o -name "*.swo" -type f -delete 2>/dev/null && print_success "Removed vim swap files" || true

# Format Go code
echo ""
echo "Formatting Go code..."
if command -v go &> /dev/null; then
    go fmt ./...
    print_success "Formatted Go code"
else
    print_warning "Go not found, skipping formatting"
fi

# Tidy Go modules
echo ""
echo "Tidying Go modules..."
if command -v go &> /dev/null; then
    go mod tidy
    print_success "Tidied Go modules"
else
    print_warning "Go not found, skipping module tidy"
fi

# Optional: Clean node_modules (commented out by default)
# Uncomment if you want to clean frontend dependencies
# echo ""
# echo "Cleaning node_modules..."
# if [ -d "node_modules" ]; then
#     read -p "Remove node_modules/ (478MB)? This will require 'npm install' later. (y/N) " -n 1 -r
#     echo
#     if [[ $REPLY =~ ^[Yy]$ ]]; then
#         rm -rf node_modules/
#         print_success "Removed node_modules/"
#     fi
# fi

# Optional: Clean npm cache
# echo ""
# echo "Cleaning npm cache..."
# if command -v npm &> /dev/null; then
#     npm cache clean --force
#     print_success "Cleaned npm cache"
# fi

echo ""
echo "🎉 Cleanup complete!"
echo ""
echo "To rebuild:"
echo "  Go backend:  go build -o bin/server ./cmd/server"
echo "  Frontend:    npm install && npm run build"

