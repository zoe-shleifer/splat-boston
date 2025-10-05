 **Go + Redis + Cloudflare**, no auth, Boston-only geofence, 8-color palette, and OMCB-style chunking/bit-packing.

---

# r/place-style “near-me” canvas — Backend (Go + Redis)

A tiny, production-lean backend for a location-gated pixel canvas:

* **One Go API/WS server**
* **One Redis** (atomic 4-bit writes, fan-out source of truth)
* **Cloudflare** in front (TLS, caching, Turnstile, WAF/rate limits)
* **No auth, no SQL** in v0

> Target: ~25M tiles (10m × 10m) covering Greater Boston. Players can paint only **near their current GPS**. Snapshots are cached; deltas stream over WS.


## 1) Architecture

```
[ Client (Leaflet + Canvas) ]
        |  GET /state/chunk?cx,cy (CDN cached 1–2s)
        |  WS /sub?cx,cy  (deltas)
        |  POST /paint (Turnstile token, lat/lon)
        v
[ Cloudflare ]
  - TLS, CDN, WAF, Bot Fight Mode
  - Turnstile verify on /paint
        v
[ Go server ]
  - Reads/writes Redis
  - Geofence + cooldown
  - Fan-out deltas per chunk
        v
[ Redis ]
  - chunk:{cx}:{cy}:bits (bitstring of 4-bit colors)
  - chunk:{cx}:{cy}:seq  (monotonic)
  - cool:{ip} / cool:{ip}:{cid} (rate-limit)
```

---

## 2) World Model

* **Projection:** Web-Mercator (EPSG:3857) over WGS84 input lat/lon.
* **Tile:** 10m × 10m.
* **Chunk:** 256 × 256 tiles (65,536 tiles).
* **Indexing:**

  * Global tile `(x,y)` → chunk coords `cx = x >> 8`, `cy = y >> 8`
  * Offset within chunk: `o = ((y & 255) << 8) | (x & 255)` (0..65535)
* **Palette:** 9 entries (index 0 = unpainted) → 4 bits per tile.

### Geofence (playable mask)

* Precompute a **1-bit per tile** mask for Greater Boston (same grid).
* File: `boston_mask.bin` packed MSB→LSB, row-major over `(x,y)` in bounding index domain. Server rejects paints when mask bit = 0.

> See §7 for mask generation.

---

## 3) Data Layout (Redis)

* `chunk:{cx}:{cy}:bits` → **binary string** length = 32 KiB (65,536 tiles × 4 bits / 8 = 32,768 bytes).
* `chunk:{cx}:{cy}:seq` → **INT** (monotonic; starts at 0).
* `chunk:{cx}:{cy}:rb` (optional) → small ringbuffer/list of `{seq,o,color,ts}` for quick catch-up.
* `cool:{ip}` or `cool:{ip}:{cid}` → last paint timestamp.

Memory rough-order: 25M tiles ≈ 12.5 MB raw bitstrings (+ Redis overhead).

---

## 4) API (Minimal)

### `GET /state/chunk?cx=&cy=`

Returns current chunk snapshot.

* **Headers**:

  * `X-Seq: <uint64>` — snapshot sequence for cache keying
  * `Content-Type: application/octet-stream`
* **Body**: 32 KiB bitpacked colors (two tiles per byte, hi-nibble = even `o`, lo-nibble = odd `o`).
* **Cache**: Suggest edge TTL **1–2s** + `stale-while-revalidate`.

### `WS /sub?cx=&cy=`

Stream **ordered deltas** for a chunk.

* **Server → Client** messages (JSON, ≤ 24B typical):

  ```json
  { "seq": 102393, "o": 12345, "color": 3, "ts": 1730075401 }
  ```
* Client should ignore any delta with `seq <= lastAppliedSeq`.

### `POST /paint`

Body:

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

Responses:

* `200 OK` → `{ "ok": true, "seq": 102394, "ts": 1730075401 }`
* `400` → invalid input / outside mask
* `401` → Turnstile failed (prod)
* `403` → geofence radius exceeded / speed clamp hit
* `409` → conflicting current color (if you enforce “no-op” guard)
* `429` → cooldown
* `500` → internal

---

## 5) Server Configuration

Environment variables (with sane defaults):

```
BIND_ADDR=:8080
REDIS_URL=redis://localhost:6379
BOSTON_MASK_PATH=./data/boston_mask.bin       # 1b/tile mask file
PALETTE=000000,FF0000,FFA500,FFFF00,00FF00,00FFFF,0000FF,FF00FF,FFFFFF
PAINT_COOLDOWN_MS=5000                        # 1 paint / 5s per subject
GEOFENCE_RADIUS_M=300                         # max distance from (lat,lon) to tile center
SPEED_MAX_KMH=150                             # anti-teleport heuristic
ENABLE_TURNSTILE=false                        # true in prod
TURNSTILE_SECRET=                             # from Cloudflare
WS_WRITE_BUFFER=1048576
WS_PING_INTERVAL_S=20
```

---

## 6) Redis: Atomic 4-bit Paint (Lua)

Write one tile (4-bit color) + bump sequence; return new `{seq,ts,prevColor}`.

**Key layout per request**

* `k_bits = "chunk:{cx}:{cy}:bits"`
* `k_seq  = "chunk:{cx}:{cy}:seq"`

**Args**

* `o` (0..65535), `color` (0..8), `nowTs`

```lua
-- file: lua/paint.lua
-- KEYS[1]=k_bits, KEYS[2]=k_seq
-- ARGV[1]=o, ARGV[2]=color, ARGV[3]=nowTs

local o = tonumber(ARGV[1])
local color = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local byteIdx = math.floor((o * 4) / 8)
local nibbleIsHigh = (o % 2) == 0

local cur = redis.call('GETRANGE', KEYS[1], byteIdx, byteIdx)
if cur == false or #cur == 0 then
  -- initialize 32 KiB if absent
  redis.call('SETRANGE', KEYS[1], 32767, string.char(0))
  cur = string.char(0)
end

local b = string.byte(cur)
local prev
if nibbleIsHigh then
  prev = bit.rshift(bit.band(b, 0xF0), 4)
  b = bit.bor(bit.band(b, 0x0F), bit.lshift(color, 4))
else
  prev = bit.band(b, 0x0F)
  b = bit.bor(bit.band(b, 0xF0), color)
end

redis.call('SETRANGE', KEYS[1], byteIdx, string.char(b))
local seq = redis.call('INCR', KEYS[2])

return { seq, now, prev }
```

**Go call example**

```go
res, err := rdb.Eval(ctx, paintScript, []string{kBits, kSeq},
    o, color, time.Now().Unix(),
).Result()
```

---

## 7) Boston Mask Generation

Input: a **GeoJSON polygon** of Greater Boston playable area.

Steps (one-off tool provided under `./cmd/maskgen`):

```bash
# Build mask generator
go build -o bin/maskgen ./cmd/maskgen

# Define the global tile-space bounding box you care about (minx..maxx/miny..maxy in *tile indices*)
# You can derive these from the polygon's lat/lon bounds via the same proj and 10m grid.

# Generate bit-packed mask file (1 = allowed)
bin/maskgen \
  -geojson ./data/greater_boston.geojson \
  -out ./data/boston_mask.bin \
  -tileSizeMeters 10 \
  -chunkSize 256 \
  -proj webmercator \
  -bounds "minx,miny,maxx,maxy"
```

The server loads `boston_mask.bin` into memory on boot. To test a position:

* Convert `(lat,lon)` → `(x,y)` tile index (same function you’ll use on the client).
* Compute bit index → accept/reject.

---

## 8) Coordinate Math (WGS84 → 10m Web-Mercator tiles)

```go
// WebMercator meters
const earthRadius = 6378137.0
const originShift = math.Pi * earthRadius
const tileMeters  = 10.0 // grid resolution

func latLonToTileXY(lat, lon float64) (x, y int64) {
    // Clamp latitude to Mercator
    lat = math.Max(math.Min(lat, 85.05112878), -85.05112878)
    mx := lon * originShift / 180.0
    my := math.Log(math.Tan((90.0+lat)*math.Pi/360.0)) * earthRadius
    // Shift to [0, 2*originShift], then quantize to 10m tiles
    tx := int64(math.Floor((mx + originShift) / tileMeters))
    ty := int64(math.Floor((originShift - my) / tileMeters)) // top-down
    return tx, ty
}

func chunkOf(x, y int64) (cx, cy int64) { return x >> 8, y >> 8 }
func offsetOf(x, y int64) int {
    return int(((y & 255) << 8) | (x & 255))
}
```

---

## 9) Cooldown & Geofence

* **Cooldown:** 1 paint / 5s per `(IP[,cid])`.

  * `SET cool:{ip} <now> EX 5 NX` pattern or store last-ts and compare; return `429`.
* **Geofence radius:** Haversine from `(lat,lon)` to tile center ≤ `GEOFENCE_RADIUS_M`.
* **Speed clamp:** store `(lat,lon,ts)` last paint and reject if implied speed > `SPEED_MAX_KMH`.

---

## 10) Server Layout

```
.
├── cmd/
│   ├── server/            # main HTTP/WS service
│   └── maskgen/           # one-off GeoJSON→bitmask tool
├── internal/
│   ├── api/               # handlers: /state/chunk, /paint, /sub
│   ├── bits/              # nibble read/write utils
│   ├── geo/               # proj, haversine, masks
│   ├── redis/             # scripts, keys, ringbuffer (optional)
│   └── ws/                # hub: per-(cx,cy) broadcasters
├── lua/
│   └── paint.lua
├── data/
│   ├── greater_boston.geojson
│   └── boston_mask.bin
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

---

## 11) WebSocket Hub (concept)

* One **room key per chunk**: `"cx:cy"`.
* Each room keeps a list of conns; inbound paint pushes `{seq,o,color,ts}` into the room channel.
* Idle cleanup after N minutes without subscribers.

Sketch:

```go
type Delta struct{ Seq uint64; O uint16; Color uint8; Ts int64 }
type Room struct {
    subs map[*Conn]struct{}
    ch   chan Delta
}

type Hub struct {
    mu    sync.RWMutex
    rooms map[string]*Room
}

func (h *Hub) Publish(cx, cy int64, d Delta) {
    key := fmt.Sprintf("%d:%d", cx, cy)
    h.mu.RLock(); r := h.rooms[key]; h.mu.RUnlock()
    if r == nil { return }
    select { case r.ch <- d: default: } // drop on backpressure
}
```

---

## 12) Edge Caching Strategy

* `GET /state/chunk?cx,cy`:

  * Compute `X-Seq` header from Redis.
  * Add `Cache-Control: public, max-age=2, stale-while-revalidate=8`.
  * **(Optional)** also provide `/tiles/{z}/{x}/{y}.png?rev={X-Seq}` if serving raster tiles; clients overlay WS deltas until CDN catches up.

---

## 13) Cloudflare (prod)

* **DNS + Proxy** (orange cloud).
* **Turnstile**: Invisible or managed widget on “Paint” button.

  * Server verifies token each `/paint` with `TURNSTILE_SECRET`.
* **WAF rules**:

  * Rate-limit `/paint`: e.g. **12/min, burst 3** per IP.
  * Challenge suspicious ASNs on `/paint` only.
  * Cache `/state/chunk` **by header `X-Seq` or query `rev`** if using raster.
* **Bot Fight Mode**: on (monitor → enforce after burn-in).

---

## 14) Docker & Compose

**Dockerfile**

```dockerfile
FROM golang:1.22 AS build
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /server ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=build /server /server
COPY data/boston_mask.bin /data/boston_mask.bin
ENV BIND_ADDR=:8080 BOSTON_MASK_PATH=/data/boston_mask.bin
EXPOSE 8080
ENTRYPOINT ["/server"]
```

**docker-compose.yml**

```yaml
version: "3.9"
services:
  redis:
    image: redis:7-alpine
    command: ["redis-server", "--appendonly", "no"]
    ports: ["6379:6379"]
  server:
    build: .
    environment:
      REDIS_URL: redis://redis:6379
      ENABLE_TURNSTILE: "true"
      TURNSTILE_SECRET: ${TURNSTILE_SECRET:?set-it}
    depends_on: [redis]
    ports: ["8080:8080"]
```

---

## 15) Observability

* **Access logs**: method, path, ip, cf-ray, cf-country, status, ms.
* **Counters**: paints accepted/rejected (per reason), WS conns per chunk, Redis RTT.
* **SLO**:

  * p99 `/paint` < 30 ms @ single region
  * p99 WS broadcast < 20 ms to active subs

---

## 16) Testing

* **Unit**: bits/nibble packing, coord math, mask hits.
* **Integration**: start Redis; paint nack/ack; snapshot reflects delta.
* **Replay**: synthetic random painters (Poisson) staying within radius; verify cooldown and speed clamp.

```bash
go test ./...
```

---

## 17) Failure Modes & Recovery

* **Server restart**: state persists in Redis; clients refresh snapshot.
* **Redis loss** (v0): state lost. To mitigate, run a **snapshotter** process every N seconds to write all chunk bitstrings to disk/S3; on boot, load latest snapshot.
* **Hot chunk floods**: room backpressure drops deltas; snapshot fetch fills gaps.

---

## 18) Security (no auth)

* Turnstile for `/paint` + WAF rate limits.
* **IP+cookie cooldown** (`cid` = random 128-bit cookie from server on first visit).
* **Speed clamp** vs GPS spoof.
* Reject absurd GPS accuracy claims; ignore client-reported accuracy entirely if you prefer.

---

## 19) Client Contract (for frontend team)

* On viewport enter:

  1. `GET /state/chunk?cx,cy` → paint into an offscreen buffer (PNG tile or raw buffer).
  2. `WS /sub?cx,cy` → apply deltas where `delta.seq > lastSeq`.
* For painting:

  * Compute `(x,y,cx,cy,o)` for clicked pixel.
  * Send `/paint` with **Turnstile token** and **lat/lon**.
  * On `200`, optimistic-apply locally; server WS will confirm with same `seq`.

---

## 20) Extensibility

* Add **ringbuffer** `chunk:{cx}:{cy}:rb` (list length ~100) to let a reconnecting client catch up deltas newer than its lastSeq without refetching snapshot.
* Add **Postgres** later for moderation/events (append-only paint log).
* Replace raster with **MVT** tiles if/when needed.

---

## 21) Appendix — Reference Handlers (sketches)

**GET /state/chunk**

```go
func (h *Handler) GetChunk(w http.ResponseWriter, r *http.Request) {
    cx, cy := parseXY(r)
    kBits := fmt.Sprintf("chunk:%d:%d:bits", cx, cy)
    kSeq  := fmt.Sprintf("chunk:%d:%d:seq",  cx, cy)

    seq, _ := h.rdb.Get(ctx, kSeq).Uint64()
    buf, err := h.rdb.GetRange(ctx, kBits, 0, 32767).Bytes()
    if err == redis.Nil || len(buf) == 0 {
        buf = make([]byte, 32768) // blank
    }
    w.Header().Set("Content-Type", "application/octet-stream")
    w.Header().Set("X-Seq", strconv.FormatUint(seq, 10))
    w.Header().Set("Cache-Control", "public, max-age=2, stale-while-revalidate=8")
    w.WriteHeader(200)
    w.Write(buf)
}
```

**POST /paint**

```go
type PaintReq struct {
  Lat float64 `json:"lat"`
  Lon float64 `json:"lon"`
  Cx  int64   `json:"cx"`
  Cy  int64   `json:"cy"`
  O   int     `json:"o"`
  Color uint8 `json:"color"`
  TurnstileToken string `json:"turnstileToken"`
}

func (h *Handler) PostPaint(w http.ResponseWriter, r *http.Request) {
  var req PaintReq
  if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w,"bad json",400); return }

  if h.cfg.EnableTurnstile {
    if !verifyTurnstile(h.cfg.TurnstileSecret, req.TurnstileToken, getIP(r)) {
      http.Error(w,"turnstile",401); return
    }
  }

  // cooldown + speed clamp + geofence
  if err := h.guard.Check(r.Context(), getIP(r), req); err != nil {
    writeErr(w, err); return
  }

  // atomic paint
  kBits := fmt.Sprintf("chunk:%d:%d:bits", req.Cx, req.Cy)
  kSeq  := fmt.Sprintf("chunk:%d:%d:seq",  req.Cx, req.Cy)
  res, err := h.rdb.EvalSha(r.Context(), h.lua.PaintSHA, []string{kBits,kSeq},
      req.O, req.Color, time.Now().Unix(),
  ).Result()
  if err != nil { http.Error(w,"redis",500); return }

  arr := res.([]interface{})
  seq := toUint64(arr[0])
  ts  := toInt64(arr[1])

  // broadcast
  h.hub.Publish(req.Cx, req.Cy, Delta{Seq: seq, O: uint16(req.O), Color: req.Color, Ts: ts})

  json.NewEncoder(w).Encode(map[string]any{"ok":true,"seq":seq,"ts":ts})
}
```

---

## 22) Production Checklist

* [ ] Cloudflare DNS proxied, certs green.
* [ ] Turnstile site key (frontend) + secret key (backend).
* [ ] WAF rules: `/paint` rate-limit; allow `GET /state/chunk`; no-cache bypass for WS.
* [ ] Redis: managed (AOF off is fine for v0), alerts on memory, CPU.
* [ ] Health endpoint `/healthz` (Redis ping).
* [ ] Dashboards: paints/s, 4xx by reason, WS conns, Redis RTT, p50/p95 lat.
* [ ] Runbook for “hot chunk flood”: temporarily raise cooldown, reduce WS fan-out buffer (drop policy), bump CDN TTL to 3–5s.

