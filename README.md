# Splat Boston - r/place-style Location-Gated Pixel Canvas

A production-lean backend for a location-gated pixel canvas where players can paint only near their current GPS location.

## Architecture

- **Go API/WebSocket server** - Handles HTTP requests and real-time WebSocket connections
- **Redis** - Stores canvas state with atomic 4-bit writes and acts as source of truth
- **Cloudflare** - TLS, caching, Turnstile bot protection, WAF/rate limits

## Features

- 🎨 8-color palette with 4-bit color storage
- 📍 GPS geofencing - paint only near your location
- ⚡ Real-time updates via WebSocket
- 🛡️ Rate limiting and cooldown (5s per paint)
- 🚀 Speed clamp to prevent teleportation
- 🤖 Cloudflare Turnstile for bot protection
- 🗺️ Covers Greater Boston with ~25M tiles (10m × 10m)

## Quick Start

### Prerequisites

- Go 1.22+
- Redis 7+
- (Optional) Cloudflare account for Turnstile

### Installation

```bash
# Clone the repository
git clone <your-repo-url>
cd splat-boston

# Install dependencies
make deps

# Start Redis (if not already running)
redis-server

# Build the server
make build

# Run the server
make run
```

### Configuration

Configure via environment variables:

```bash
export BIND_ADDR=:8080
export REDIS_URL=redis://localhost:6379
export BOSTON_MASK_PATH=./data/boston_mask.bin
export PAINT_COOLDOWN_MS=5000
export GEOFENCE_RADIUS_M=300
export SPEED_MAX_KMH=150
export ENABLE_TURNSTILE=false
export TURNSTILE_SECRET=your_secret_key
export WS_WRITE_BUFFER=1048576
export WS_PING_INTERVAL_S=20
```

## API Endpoints

### GET /state/chunk?cx=&cy=

Returns the current chunk snapshot (32KB bitpacked colors).

**Response Headers:**
- `X-Seq`: Snapshot sequence number
- `Content-Type`: application/octet-stream
- `Cache-Control`: public, max-age=2, stale-while-revalidate=8

### POST /paint

Submit a paint request.

**Request Body:**
```json
{
  "lat": 42.3601,
  "lon": -71.0589,
  "cx": 343,
  "cy": 612,
  "o": 12345,
  "color": 3,
  "turnstileToken": "CF-challenge-token"
}
```

**Response:**
```json
{
  "ok": true,
  "seq": 102394,
  "ts": 1730075401
}
```

**Status Codes:**
- `200 OK` - Paint successful
- `400 Bad Request` - Invalid input
- `401 Unauthorized` - Turnstile failed
- `403 Forbidden` - Geofence/speed limit exceeded
- `429 Too Many Requests` - Cooldown active
- `500 Internal Server Error` - Server error

### WS /sub?cx=&cy=

Subscribe to real-time deltas for a chunk.

**Server → Client Messages:**
```json
{
  "seq": 102393,
  "o": 12345,
  "color": 3,
  "ts": 1730075401
}
```

### GET /healthz

Health check endpoint. Returns 200 OK if Redis is healthy.

## Testing

### Run All Tests

```bash
make test
```

Or use the test runner script:

```bash
./test_runner.sh
```

### Run Specific Test Suites

```bash
make test-bits        # Nibble packing tests
make test-geo         # Coordinate and geofence tests
make test-rate        # Rate limiting tests
make test-redis       # Redis operations (requires Redis)
make test-api         # API handler tests
make test-ws          # WebSocket hub tests
make test-integration # Integration tests (requires Redis)
```

### Run with Coverage

```bash
make test-coverage
open coverage.html
```

### Run with Race Detection

```bash
make test-race
```

### Run Benchmarks

```bash
make bench
```

## Project Structure

```
.
├── cmd/
│   └── server/            # Main HTTP/WS service
├── internal/
│   ├── api/               # HTTP handlers
│   ├── bits/              # Nibble read/write utils
│   ├── geo/               # Projection, haversine, masks
│   ├── rate/              # Rate limiting and cooldown
│   ├── redis/             # Redis client and Lua scripts
│   ├── turnstile/         # Cloudflare Turnstile verification
│   └── ws/                # WebSocket hub
├── data/
│   └── boston_mask.bin    # Geofence mask (if available)
├── Makefile
├── go.mod
├── go.sum
└── test_runner.sh
```

## Data Model

### World Model

- **Projection:** Web-Mercator (EPSG:3857) over WGS84 input lat/lon
- **Tile:** 10m × 10m
- **Chunk:** 256 × 256 tiles (65,536 tiles)
- **Palette:** 9 entries (index 0 = unpainted) → 4 bits per tile

### Redis Keys

- `chunk:{cx}:{cy}:bits` - 32 KiB binary string (65,536 tiles × 4 bits)
- `chunk:{cx}:{cy}:seq` - Monotonic sequence counter
- `cool:{ip}` - Cooldown timestamp

### Coordinate Conversion

```go
// Convert lat/lon to tile coordinates
x, y := geo.LatLonToTileXY(lat, lon)

// Get chunk coordinates
cx, cy := geo.ChunkOf(x, y)

// Get offset within chunk
offset := geo.OffsetOf(x, y)
```

## Performance

### Target SLOs

- p99 `/paint` < 30ms @ single region
- p99 WS broadcast < 20ms to active subscribers

### Benchmarks

Run benchmarks with:

```bash
make bench
```

## Production Deployment

### Docker

Build and run with Docker:

```bash
make docker-build
make docker-up
```

Or use docker-compose:

```bash
docker-compose up -d
```

### Environment Setup

1. Set up Redis (managed service or self-hosted)
2. Configure Cloudflare:
   - Add DNS record (proxied/orange cloud)
   - Enable Turnstile (get site key + secret)
   - Configure WAF rules for `/paint` rate limiting
   - Set cache rules for `/state/chunk`
3. Deploy Go server with environment variables
4. Monitor health via `/healthz`

## Security

- **Turnstile:** Bot protection on `/paint` endpoint
- **Rate Limiting:** 1 paint per 5s per IP
- **Speed Clamp:** Rejects speeds > 150 km/h
- **Geofence:** 300m radius from GPS location
- **IP + Cookie:** Dual cooldown mechanism

## License

[Your License]

## Contributing

[Your contributing guidelines]

## Support

[Your support information]

