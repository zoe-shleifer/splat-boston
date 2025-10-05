# Quick Start Guide

## Running the Server Locally

### Step 1: Start Redis

In one terminal window:

```bash
redis-server
```

Or if you want it to run in the background:

```bash
redis-server --daemonize yes
```

### Step 2: Build and Run the Server

In another terminal window:

```bash
# Build the server
go build -o bin/server ./cmd/server

# Run the server
./bin/server
```

Or run directly without building:

```bash
go run ./cmd/server
```

The server will start on `http://localhost:8080` by default.

### Step 3: Test the Server

#### Health Check

```bash
curl http://localhost:8080/healthz
```

Expected response: `OK`

#### Get a Chunk (Empty State)

```bash
curl -i "http://localhost:8080/state/chunk?cx=0&cy=0"
```

Expected response:
- Status: 200 OK
- Headers: `X-Seq: 0`, `Content-Type: application/octet-stream`
- Body: 32KB of binary data (all zeros initially)

#### Paint a Tile

```bash
curl -X POST http://localhost:8080/paint \
  -H "Content-Type: application/json" \
  -d '{
    "lat": 42.3601,
    "lon": -71.0589,
    "cx": 0,
    "cy": 0,
    "o": 0,
    "color": 5,
    "turnstileToken": ""
  }'
```

Expected response:
```json
{"ok":true,"seq":1,"ts":1234567890}
```

#### Paint Another Tile (Different Color)

```bash
curl -X POST http://localhost:8080/paint \
  -H "Content-Type: application/json" \
  -d '{
    "lat": 42.3601,
    "lon": -71.0589,
    "cx": 0,
    "cy": 0,
    "o": 1,
    "color": 3,
    "turnstileToken": ""
  }'
```

#### Test Cooldown (Should Fail with 429)

Try painting again immediately:

```bash
curl -X POST http://localhost:8080/paint \
  -H "Content-Type: application/json" \
  -d '{
    "lat": 42.3601,
    "lon": -71.0589,
    "cx": 0,
    "cy": 0,
    "o": 2,
    "color": 7,
    "turnstileToken": ""
  }'
```

Expected: HTTP 429 (Too Many Requests) with "cooldown" error

#### Test Geofence (Should Fail with 403)

Try painting outside Boston area:

```bash
curl -X POST http://localhost:8080/paint \
  -H "Content-Type: application/json" \
  -d '{
    "lat": 40.7128,
    "lon": -74.0060,
    "cx": 0,
    "cy": 0,
    "o": 3,
    "color": 2,
    "turnstileToken": ""
  }'
```

Expected: HTTP 403 (Forbidden) with "geofence" error

### Step 4: Test WebSocket

You can test WebSocket connections using `websocat` or a simple HTML page.

#### Install websocat (optional)

```bash
brew install websocat  # macOS
```

#### Connect to WebSocket

```bash
websocat "ws://localhost:8080/sub?cx=0&cy=0"
```

Then in another terminal, paint a tile (as shown above). You should see real-time delta messages like:

```json
{"seq":2,"o":1,"color":3,"ts":1234567890}
```

### Step 5: Monitor Redis

You can inspect the Redis data:

```bash
# Connect to Redis CLI
redis-cli

# View all keys
KEYS *

# Get chunk sequence
GET chunk:0:0:seq

# Get chunk bits (binary data)
GET chunk:0:0:bits

# Check cooldowns
KEYS cool:*
```

## Configuration

Set environment variables to configure the server:

```bash
export BIND_ADDR=:8080
export REDIS_URL=redis://localhost:6379
export PAINT_COOLDOWN_MS=5000
export GEOFENCE_RADIUS_M=300
export SPEED_MAX_KMH=150
export ENABLE_TURNSTILE=false
export WS_WRITE_BUFFER=1048576
export WS_PING_INTERVAL_S=20

./bin/server
```

## Using Docker Compose

The easiest way to run everything:

```bash
docker-compose up
```

This will start both Redis and the Go server automatically.

## Testing with a Simple HTML Page

Create `test.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>Paint Test</title>
</head>
<body>
    <h1>Paint Canvas Test</h1>
    <button onclick="paint()">Paint Tile</button>
    <button onclick="subscribe()">Subscribe to Updates</button>
    <pre id="output"></pre>

    <script>
        let ws;
        const output = document.getElementById('output');

        function log(msg) {
            output.textContent += msg + '\n';
        }

        async function paint() {
            const response = await fetch('http://localhost:8080/paint', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    lat: 42.3601,
                    lon: -71.0589,
                    cx: 0,
                    cy: 0,
                    o: Math.floor(Math.random() * 1000),
                    color: Math.floor(Math.random() * 9),
                    turnstileToken: ''
                })
            });
            const data = await response.json();
            log('Paint response: ' + JSON.stringify(data));
        }

        function subscribe() {
            ws = new WebSocket('ws://localhost:8080/sub?cx=0&cy=0');
            ws.onmessage = (event) => {
                log('Delta received: ' + event.data);
            };
            ws.onopen = () => log('WebSocket connected');
            ws.onclose = () => log('WebSocket disconnected');
        }
    </script>
</body>
</html>
```

Open it in your browser and test!

## Troubleshooting

### Server won't start

- Check if Redis is running: `redis-cli ping`
- Check if port 8080 is free: `lsof -i :8080`

### Paint returns 500

- Check Redis connection
- Check server logs for errors

### WebSocket won't connect

- Make sure the server is running
- Check browser console for errors
- Verify the WebSocket URL format

## Next Steps

- Try painting multiple tiles
- Test concurrent connections
- Monitor Redis memory usage
- Add Turnstile token for production
- Deploy to production with Cloudflare

