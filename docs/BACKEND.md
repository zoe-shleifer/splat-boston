# Backend Architecture

This document has been moved for better organization.

For complete backend documentation, see: [src/backend.md](../src/backend.md)

## Quick Links

- **[QUICKSTART.md](../QUICKSTART.md)** - How to run the server locally
- **[README.md](../README.md)** - Project overview and features
- **[API Documentation](../src/backend.md#4-api-minimal)** - Complete API reference

## Project Structure

```
.
├── cmd/
│   └── server/           # Main HTTP/WebSocket server
├── internal/
│   ├── api/              # HTTP handlers
│   ├── bits/             # Nibble packing utilities
│   ├── geo/              # Coordinate conversion & geofencing
│   ├── rate/             # Rate limiting & cooldown
│   ├── redis/            # Redis client with Lua scripts
│   ├── turnstile/        # Cloudflare Turnstile verification
│   └── ws/               # WebSocket hub
├── docs/                 # Documentation
├── data/                 # Runtime data (masks, etc.)
└── src/                  # Frontend (React)
```

## Running Tests

```bash
# Run all tests
go test ./internal/...

# Run specific package tests
go test ./internal/geo/...
go test ./internal/bits/...

# Run with coverage
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out
```

## Development

See [QUICKSTART.md](../QUICKSTART.md) for local development setup.

