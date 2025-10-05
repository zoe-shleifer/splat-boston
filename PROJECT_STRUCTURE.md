# Project Structure

This repository contains both the **frontend** (React/TypeScript) and **backend** (Go) for Splat Boston.

## Directory Layout

```
splat-boston/
├── cmd/                    # Go backend entry points
│   └── server/            # Main HTTP/WebSocket server
│
├── internal/              # Go backend packages (private)
│   ├── api/              # HTTP request handlers
│   ├── bits/             # 4-bit nibble packing utilities
│   ├── geo/              # Coordinate conversion & geofencing
│   ├── rate/             # Rate limiting & cooldown logic
│   ├── redis/            # Redis client with Lua scripts
│   ├── turnstile/        # Cloudflare Turnstile verification
│   └── ws/               # WebSocket hub for real-time updates
│
├── src/                   # Frontend (React/TypeScript)
│   ├── App.tsx           # Main React app
│   ├── App.css           # Styles
│   ├── backend.md        # Backend specification
│   └── ...
│
├── public/                # Frontend static assets
│   ├── index.html
│   ├── tiles/            # GeoJSON tiles (4384 files)
│   └── ...
│
├── docs/                  # Documentation
│   └── BACKEND.md        # Backend architecture overview
│
├── data/                  # Runtime data
│   └── boston_mask.bin   # Geofence mask (generated)
│
├── bin/                   # Compiled binaries (gitignored)
│   └── server            # Built Go server
│
├── node_modules/          # npm dependencies (gitignored)
│
├── go.mod, go.sum        # Go dependencies
├── package.json          # npm configuration
├── tsconfig.json         # TypeScript configuration
├── Dockerfile            # Container image for Go server
├── docker-compose.yml    # Local development stack
├── Makefile              # Build automation
├── README.md             # Project overview
├── QUICKSTART.md         # Quick start guide
├── test-client.html      # Browser-based test client
└── test_runner.sh        # Test automation script
```

## Tech Stack

### Backend (Go)
- **Runtime**: Go 1.22+
- **Database**: Redis 7+ (for state storage)
- **WebSocket**: gorilla/websocket
- **Key Features**:
  - Atomic 4-bit color storage with Lua scripts
  - Real-time WebSocket delta broadcasting
  - GPS geofencing (300m radius)
  - Anti-teleportation (150 km/h speed limit)
  - Rate limiting (5s cooldown per IP)
  - Cloudflare Turnstile bot protection

### Frontend (React)
- **Runtime**: Node.js / npm
- **Framework**: React with TypeScript
- **Mapping**: Leaflet with custom canvas renderer
- **State**: React hooks
- **Key Features**:
  - Interactive map of Greater Boston
  - Real-time collaborative painting
  - GPS-based geofencing
  - 8-color palette

## Development Workflow

### Backend Development

```bash
# Install Go dependencies
go mod download

# Run tests
go test ./internal/...

# Build server
go build -o bin/server ./cmd/server

# Run server (requires Redis)
./bin/server
```

### Frontend Development

```bash
# Install npm dependencies
npm install

# Start development server
npm start

# Build for production
npm run build
```

### Full Stack Development

```bash
# Using Docker Compose (easiest)
docker-compose up

# Or manually:
# Terminal 1: Start Redis
redis-server

# Terminal 2: Start Go server
./bin/server

# Terminal 3: Start React dev server
npm start
```

## Testing

### Backend Tests

```bash
# All tests
go test ./internal/...

# Specific packages
go test ./internal/geo/...
go test ./internal/bits/...
go test ./internal/rate/...

# With coverage
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out

# With race detection
go test -race ./internal/...

# Benchmarks
go test -bench=. ./internal/...
```

### Integration Tests

```bash
# Run the full test suite (requires Redis)
./test_runner.sh
```

### Manual Testing

```bash
# Open the test client
open test-client.html

# Or use curl
curl http://localhost:8080/healthz
```

## Deployment

See [README.md](README.md#-production-deployment) for production deployment instructions.

## Documentation

- **[README.md](README.md)** - Project overview, features, and quick start
- **[QUICKSTART.md](QUICKSTART.md)** - Detailed local development guide
- **[src/backend.md](src/backend.md)** - Complete backend specification
- **[docs/BACKEND.md](docs/BACKEND.md)** - Backend architecture overview

## License

[Your License]

