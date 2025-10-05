package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"

	"splat-boston/internal/geo"
	"splat-boston/internal/rate"
	redisclient "splat-boston/internal/redis"
	"splat-boston/internal/turnstile"
	"splat-boston/internal/ws"
)

// PaintRequest represents a paint request
type PaintRequest struct {
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	Cx             int64   `json:"cx"`
	Cy             int64   `json:"cy"`
	O              int     `json:"o"`
	Color          uint8   `json:"color"`
	TurnstileToken string  `json:"turnstileToken"`
}

// PaintResponse represents a paint response
type PaintResponse struct {
	Ok  bool   `json:"ok"`
	Seq uint64 `json:"seq"`
	Ts  int64  `json:"ts"`
}

// Config holds the server configuration
type Config struct {
	EnableTurnstile bool
	TurnstileSecret string
	GeofenceRadiusM float64
	SpeedMaxKmh     float64
	PaintCooldownMs int
	WSWriteBuffer   int
	WSPingIntervalS int
}

// Handler handles HTTP requests
type Handler struct {
	rdb             *redisclient.Client
	hub             *ws.Hub
	config          Config
	turnstileClient *turnstile.TurnstileClient
	cooldownLimiter *rate.Limiter
	speedLimiter    *rate.SpeedLimiter
	mask            *geo.Mask
	upgrader        websocket.Upgrader
}

// NewHandler creates a new API handler
func NewHandler(rdb *redisclient.Client, hub *ws.Hub, config Config, mask *geo.Mask) *Handler {
	h := &Handler{
		rdb:             rdb,
		hub:             hub,
		config:          config,
		cooldownLimiter: rate.NewLimiter(),
		speedLimiter:    rate.NewSpeedLimiter(config.SpeedMaxKmh),
		mask:            mask,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
			WriteBufferSize: config.WSWriteBuffer,
		},
	}

	if config.EnableTurnstile {
		h.turnstileClient = turnstile.NewTurnstileClient(config.TurnstileSecret)
	}

	return h
}

// GetChunk handles GET /state/chunk?cx=&cy=
func (h *Handler) GetChunk(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	cxStr := r.URL.Query().Get("cx")
	cyStr := r.URL.Query().Get("cy")

	if cxStr == "" || cyStr == "" {
		http.Error(w, "Missing cx or cy parameter", 400)
		return
	}

	cx, err := strconv.ParseInt(cxStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid cx parameter", 400)
		return
	}

	cy, err := strconv.ParseInt(cyStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid cy parameter", 400)
		return
	}

	// Get sequence number
	seq, err := h.rdb.GetChunkSeq(cx, cy)
	if err != nil && err != redis.Nil {
		http.Error(w, "Redis error", 500)
		return
	}

	// Get chunk bits
	buf, err := h.rdb.GetChunkBits(cx, cy)
	if err == redis.Nil || len(buf) == 0 {
		buf = make([]byte, 32768) // blank chunk
	} else if err != nil {
		http.Error(w, "Redis error", 500)
		return
	}

	// Ensure we have 32KB
	if len(buf) < 32768 {
		newBuf := make([]byte, 32768)
		copy(newBuf, buf)
		buf = newBuf
	}

	// Set headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Seq", fmt.Sprintf("%d", seq))
	w.Header().Set("Cache-Control", "public, max-age=2, stale-while-revalidate=8")
	w.WriteHeader(200)
	w.Write(buf)
}

// PostPaint handles POST /paint
func (h *Handler) PostPaint(w http.ResponseWriter, r *http.Request) {
	var req PaintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	// Verify Turnstile if enabled
	if h.config.EnableTurnstile {
		if req.TurnstileToken == "" {
			http.Error(w, "turnstile", 401)
			return
		}

		ip := getIP(r)
		resp, err := h.turnstileClient.Verify(context.Background(), req.TurnstileToken, ip)
		if err != nil || !resp.Success {
			http.Error(w, "turnstile", 401)
			return
		}
	}

	ip := getIP(r)

	// Cooldown disabled for development
	// cooldownDuration := time.Duration(h.config.PaintCooldownMs) * time.Millisecond
	// if h.cooldownLimiter.CheckCooldown(ip, cooldownDuration) {
	// 	http.Error(w, "cooldown", 429)
	// 	return
	// }

	// Check geofence (simplified - just check lat/lon bounds for Boston area)
	if req.Lat < 42.0 || req.Lat > 43.0 || req.Lon < -72.0 || req.Lon > -70.0 {
		http.Error(w, "geofence", 403)
		return
	}

	// Check speed limit
	if !h.speedLimiter.CheckSpeed(ip, req.Lat, req.Lon) {
		http.Error(w, "speed limit exceeded", 403)
		return
	}

	// Check mask if available
	if h.mask != nil {
		x, y := geo.LatLonToTileXY(req.Lat, req.Lon)
		if !h.mask.IsTileAllowed(x, y) {
			http.Error(w, "outside mask", 403)
			return
		}
	}

	// Validate color range
	if req.Color > 15 {
		http.Error(w, "invalid color", 400)
		return
	}

	// Paint tile
	seq, ts, _, err := h.rdb.PaintTile(req.Cx, req.Cy, req.O, req.Color)
	if err != nil {
		http.Error(w, "redis", 500)
		return
	}

	// Cooldown disabled for development
	// h.cooldownLimiter.SetCooldown(ip)

	// Broadcast delta
	h.hub.Publish(req.Cx, req.Cy, ws.Delta{
		Seq:   seq,
		O:     uint16(req.O),
		Color: req.Color,
		Ts:    ts,
	})

	// Return response
	response := PaintResponse{
		Ok:  true,
		Seq: seq,
		Ts:  ts,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleWebSocket handles WebSocket connections for /sub?cx=&cy=
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	cxStr := r.URL.Query().Get("cx")
	cyStr := r.URL.Query().Get("cy")

	if cxStr == "" || cyStr == "" {
		http.Error(w, "Missing cx or cy parameter", 400)
		return
	}

	cx, err := strconv.ParseInt(cxStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid cx parameter", 400)
		return
	}

	cy, err := strconv.ParseInt(cyStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid cy parameter", 400)
		return
	}

	// Upgrade connection
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Register connection
	conn := h.hub.RegisterConn(ws, cx, cy)

	// Start pumps
	go conn.WritePump()
	go conn.ReadPump()
}

func getIP(r *http.Request) string {
	// Check for Cloudflare headers
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}

	// Check for X-Forwarded-For
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}
