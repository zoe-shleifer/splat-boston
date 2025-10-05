# Frontend Backend Integration Guide

This document explains how the frontend integrates with the Go backend.

## Architecture Overview

The frontend is a React application that communicates with the Go backend via:
1. **REST API** - for fetching chunk data and sending paint requests
2. **WebSocket** - for real-time delta updates

## Key Components

### 1. Coordinate Conversion (`src/utils/coords.ts`)

Converts between WGS84 lat/lon and Web Mercator tile coordinates, matching the backend implementation in `internal/geo/coords.go`.

- `latLonToTileXY(lat, lon)` - Convert GPS coordinates to tile indices
- `chunkOf(x, y)` - Get chunk coordinates from tile indices
- `offsetOf(x, y)` - Get offset within chunk (0-65535)
- `tileXYToLatLon(x, y)` - Convert tile indices back to GPS coordinates

### 2. Nibble Utilities (`src/utils/nibbles.ts`)

Handles 4-bit color packing/unpacking for chunk data:
- Each tile uses 4 bits (0-15) for color
- Two tiles per byte (high nibble = even offset, low nibble = odd offset)
- Chunk size: 256×256 tiles = 65,536 tiles = 32KB binary data

### 3. API Client (`src/api/client.ts`)

REST API communication:
- `fetchChunk(cx, cy)` - Fetch 32KB chunk data from `GET /state/chunk?cx=&cy=`
- `paintTile(request)` - Send paint request to `POST /paint`
- `healthCheck()` - Check backend health at `GET /healthz`

### 4. WebSocket Client (`src/api/websocket.ts`)

Real-time delta updates:
- `ChunkWebSocket` - Single chunk WebSocket connection
- `ChunkWebSocketManager` - Manages multiple chunk subscriptions
- Automatically reconnects on disconnect
- Receives delta updates: `{seq, o, color, ts}`

## Data Flow

### Initial Load
1. User grants location access
2. Frontend converts GPS to tile coordinates
3. Frontend calculates chunk coordinates
4. Frontend fetches chunk data via `GET /state/chunk?cx=&cy=`
5. Frontend parses 32KB binary data and renders painted tiles
6. Frontend subscribes to WebSocket for that chunk

### Painting a Tile
1. User clicks on map
2. Frontend calculates tile coordinates and offset
3. Frontend sends `POST /paint` with:
   - User's GPS location
   - Chunk coordinates (cx, cy)
   - Tile offset within chunk (o)
   - Color index (1-8)
   - Turnstile token (empty for now)
4. Backend validates and paints tile
5. Backend broadcasts delta via WebSocket
6. Frontend receives delta and updates local state

### Real-time Updates
1. Another user paints a tile in the same chunk
2. Backend broadcasts delta to all WebSocket subscribers
3. Frontend receives delta: `{seq, o, color, ts}`
4. Frontend updates chunk data and re-renders tile

## Color Palette

8 colors (indices 1-8), matching backend:
1. Red (#FF0000)
2. Orange (#FFA500)
3. Yellow (#FFFF00)
4. Green (#00FF00)
5. Cyan (#00FFFF)
6. Blue (#0000FF)
7. Magenta (#FF00FF)
8. White (#FFFFFF)

Index 0 = unpainted (transparent)

## Configuration

Set environment variables in `.env.local`:

```bash
# Backend API URL (defaults to http://localhost:8080)
REACT_APP_API_URL=http://localhost:8080

# WebSocket URL (defaults to ws://localhost:8080)
REACT_APP_WS_URL=ws://localhost:8080
```

## Running the Full Stack

### Terminal 1: Start Redis
```bash
redis-server
```

### Terminal 2: Start Go Backend
```bash
go run ./cmd/server
```

### Terminal 3: Start React Frontend
```bash
npm start
```

The app will open at `http://localhost:3000` and connect to the backend at `http://localhost:8080`.

## Features

### Implemented
✅ GPS-based geofencing (300m radius)
✅ Chunk loading and caching
✅ Real-time WebSocket updates
✅ Nibble-packed chunk data
✅ Paint cooldown error handling
✅ Speed limit error handling
✅ Connection status indicator

### TODO
- [ ] Cloudflare Turnstile integration
- [ ] Multi-chunk viewport management
- [ ] Offline support with local cache
- [ ] Paint history/undo
- [ ] User statistics

## Error Handling

The frontend handles these backend errors:
- **429 Too Many Requests** - Cooldown period (5 seconds)
- **403 Forbidden** - Outside geofence or speed limit exceeded
- **401 Unauthorized** - Turnstile verification failed
- **400 Bad Request** - Invalid parameters

Errors are displayed in the UI for 5 seconds, then automatically cleared.

## Performance Considerations

- Chunk data is cached locally (32KB per chunk)
- WebSocket reconnects automatically with exponential backoff
- Only paints tiles with color > 0 (skips empty tiles)
- Optimistic UI updates (local state updated before server confirmation)

## Debugging

Open browser console to see:
- Chunk loading logs
- WebSocket connection status
- Paint request/response
- Delta updates
- Error messages

## Testing

1. Open the app in two browser windows
2. Grant location access in both
3. Paint a tile in one window
4. Watch the tile appear in the other window in real-time
5. Try painting again immediately (should fail with cooldown error)

