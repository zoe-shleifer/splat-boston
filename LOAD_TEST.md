# Load Testing Guide

This guide explains how to run load tests on your Splat Boston backend to simulate multiple concurrent users painting across different locations.

## Overview

The load test script (`load-test.js`) simulates real users:
- 🎨 Painting tiles at regular intervals
- 📡 Subscribing to WebSocket updates for their chunks
- 📍 Distributed across multiple locations in Greater Boston
- 🔄 Testing both single-chunk and multi-chunk scenarios

## Prerequisites

1. **Install dependencies**:
```bash
npm install
```

2. **Start your backend**:
```bash
# Terminal 1: Start Redis
redis-server

# Terminal 2: Start Go backend
go run ./cmd/server
```

## Viewing the Load Test in Real-Time

Before running the load test, open the viewer in your browser:

```bash
# macOS
open load-test-viewer.html

# Linux
xdg-open load-test-viewer.html

# Windows
start load-test-viewer.html
```

Or simply open `load-test-viewer.html` in your browser.

The viewer will:
- 🗺️ Display the map with painted tiles in real-time
- 📊 Show live statistics (paints/sec, active chunks, total tiles)
- 📝 Log all paint events as they happen
- 🔄 Auto-subscribe to visible chunks as you pan/zoom

**Usage:**
1. Open the viewer in your browser
2. Adjust the API URL if needed (default: `http://localhost:8080`)
3. Click **Connect**
4. In another terminal, run your load test
5. Watch the tiles appear in real-time! 🎨

## Running Load Tests

### Quick Test (10 users, 1 minute)
```bash
npm run load-test
```

### Light Test (5 users, 30 seconds)
```bash
npm run load-test:light
```

### Heavy Test (50 users, 2 minutes)
```bash
npm run load-test:heavy
```

### Visible Blocks Test (10x10 blocks, highly visible)
```bash
npm run load-test:visible
```
This mode paints large 10×10 blocks of tiles, making them very easy to see on the map!

### Custom Configuration

You can customize the test with environment variables:

```bash
NUM_USERS=20 \
PAINT_INTERVAL_MS=5000 \
TEST_DURATION_MS=90000 \
BLOCK_SIZE=8 \
API_URL=http://localhost:8080 \
WS_URL=ws://localhost:8080 \
node load-test.js
```

## Configuration Parameters

| Variable | Default | Description |
|----------|---------|-------------|
| `NUM_USERS` | 10 | Number of simulated users |
| `PAINT_INTERVAL_MS` | 6000 | Time between paint attempts per user (ms) |
| `TEST_DURATION_MS` | 60000 | Total test duration (ms) |
| `BLOCK_SIZE` | 5 | Size of contiguous blocks to paint (NxN tiles) |
| `API_URL` | http://localhost:8080 | Backend API URL |
| `WS_URL` | ws://localhost:8080 | WebSocket URL |

## Block Painting Behavior

Each simulated user paints **contiguous blocks** of tiles instead of random scattered tiles:

- **Default**: 5×5 blocks (25 tiles per block)
- **Block pattern**: Users paint tiles row-by-row within each block
- **Color consistency**: Each user uses the same color for all their tiles
- **Multiple blocks**: After completing a block, users start a new block nearby
- **Highly visible**: Blocks make painted areas much easier to see on the map

Example with `BLOCK_SIZE=5`:
```
█████  User paints a 5×5 block
█████  = 25 tiles total
█████  Each tile painted one-by-one
█████  in 25 sequential paint requests
█████
```

To make blocks **even more visible**, increase the block size:
```bash
BLOCK_SIZE=10 node load-test.js  # 10×10 = 100 tiles per block!
BLOCK_SIZE=20 node load-test.js  # 20×20 = 400 tiles per block!!
```

## Test Scenarios

The script tests users across multiple Greater Boston locations:

### Single-Chunk Scenarios
- **Downtown Boston** (chunk ~343, 612)
  - Boston Common
  - Chinatown
  - Financial District

### Multi-Chunk Scenarios
- **Cambridge** (chunk ~342, 611)
  - Harvard Square
  - Central Square
  - Kendall Square

- **South Boston** (chunk ~344, 613)
  - South Boston
  - Seaport

- **Back Bay** (chunk ~343, 611)
  - Copley Square
  - Fenway

- **Somerville** (chunk ~342, 610)
  - Davis Square
  - Union Square

Users are distributed evenly across all locations, so with 10 users you'll have activity in multiple chunks simultaneously.

## Metrics Reported

### Paint Statistics
- Total paint requests
- Successful paints
- Failed paints
- Cooldown rejections (429)
- Geofence rejections (403)

### WebSocket Statistics
- Active connections
- Messages received
- Connection errors

### Latency Metrics
- Min, Average, Max latency
- p50, p95, p99 percentiles

### Error Summary
- Breakdown of all error types

## Example Output

```
============================================================
Splat Boston Load Test
============================================================
API URL: http://localhost:8080
WS URL: ws://localhost:8080
Number of users: 10
Paint interval: 6000ms
Test duration: 60000ms
============================================================

✓ Backend is healthy

Starting 10 users...

[User 1] Starting at Boston Common (42.3601, -71.0589)
[User 1] Chunk: (343, 612)
[User 1] WebSocket connected to chunk (343, 612)
[User 1] Paint success: offset=12543, color=5, seq=1, latency=15ms
...

Stopping all users...

============================================================
Load Test Results
============================================================
Total paint requests: 95
Successful paints: 82 (86.3%)
Failed paints: 0
Cooldown rejections: 13
Geofence rejections: 0

WebSocket connections: 10
WebSocket messages received: 820
WebSocket errors: 0

Paint Latencies:
  Min: 8ms
  Avg: 15.3ms
  p50: 14ms
  p95: 25ms
  p99: 32ms
  Max: 45ms

============================================================
```

## Interpreting Results

### Good Performance Indicators
- ✅ p99 latency < 30ms
- ✅ Paint success rate > 85% (excluding cooldowns)
- ✅ No WebSocket errors
- ✅ All users receive delta updates

### Warning Signs
- ⚠️ p99 latency > 50ms
- ⚠️ High failure rate (not due to cooldowns)
- ⚠️ WebSocket connection errors
- ⚠️ Missing delta updates

### Critical Issues
- 🚨 p99 latency > 100ms
- 🚨 Paint failures > 10%
- 🚨 Backend crashes
- 🚨 Redis connection errors

## Testing Scenarios

### 1. Single Chunk Load
Test many users in the same location:
```bash
# Edit load-test.js and comment out all but one location
NUM_USERS=20 node load-test.js
```

### 2. Multi-Chunk Load
Test users spread across Boston (default behavior):
```bash
NUM_USERS=30 node load-test.js
```

### 3. Rapid Fire
Test rate limiting and cooldowns:
```bash
NUM_USERS=10 PAINT_INTERVAL_MS=2000 TEST_DURATION_MS=30000 node load-test.js
```

### 4. Stress Test
Push the system to its limits:
```bash
NUM_USERS=100 PAINT_INTERVAL_MS=3000 TEST_DURATION_MS=300000 node load-test.js
```

### 5. WebSocket Stress
Test WebSocket broadcast performance:
```bash
# All users in same chunk = every user gets every update
NUM_USERS=50 PAINT_INTERVAL_MS=6000 TEST_DURATION_MS=120000 node load-test.js
```

## Remote Server Testing

To test your deployed server:

```bash
API_URL=http://157.245.11.177:8080 \
WS_URL=ws://157.245.11.177:8080 \
NUM_USERS=20 \
node load-test.js
```

## Troubleshooting

### "Backend is not reachable"
- Check that your Go server is running
- Verify the correct port (default: 8080)
- Check firewall settings

### "WebSocket connection failed"
- Ensure WebSocket endpoint is accessible
- Check for CORS issues
- Verify WebSocket URL format

### "High cooldown rejections"
- This is expected - cooldown is 5 seconds
- Lower `PAINT_INTERVAL_MS` to see more cooldowns
- Check Redis cooldown keys: `redis-cli KEYS cool:*`

### "Geofence rejections"
- Verify locations are within Greater Boston
- Check `GEOFENCE_RADIUS_M` setting (default: 300m)
- Review backend geofence logic

## Next Steps

After load testing:

1. **Analyze bottlenecks** - Check which operations are slowest
2. **Scale Redis** - Add Redis clustering if needed
3. **Optimize WebSocket** - Consider connection pooling
4. **Add monitoring** - Set up Prometheus/Grafana for metrics
5. **Test at scale** - Run longer tests with more users

## Production Recommendations

Based on load test results, configure production:

- **Concurrent users**: Plan for 10x your peak load test
- **Redis memory**: Allocate based on chunks × 32KB
- **Rate limits**: Set based on acceptable p99 latency
- **Cloudflare**: Add Turnstile and WAF rules
- **Monitoring**: Alert on p99 > 50ms or error rate > 1%

