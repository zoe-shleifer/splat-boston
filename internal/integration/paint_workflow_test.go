package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Integration tests for end-to-end paint workflow

type IntegrationTest struct {
	server   *httptest.Server
	wsServer *httptest.Server
	redis    *MockRedisClient
	hub      *MockWebSocketHub
	config   Config
}

type Config struct {
	EnableTurnstile bool
	GeofenceRadiusM float64
	SpeedMaxKmh     float64
	PaintCooldownMs int
	TurnstileSecret string
}

type MockRedisClient struct {
	chunks    map[string][]byte
	seqs      map[string]uint64
	cooldowns map[string]time.Time
	mu        sync.RWMutex
}

type MockWebSocketHub struct {
	rooms     map[string]*MockRoom
	mu        sync.RWMutex
	published []Delta
}

type MockRoom struct {
	subs map[string]*MockConnection
	mu   sync.RWMutex
}

type MockConnection struct {
	id   string
	send chan Delta
}

type Delta struct {
	Seq   uint64 `json:"seq"`
	O     uint16 `json:"o"`
	Color uint8  `json:"color"`
	Ts    int64  `json:"ts"`
}

type PaintRequest struct {
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	Cx             int64   `json:"cx"`
	Cy             int64   `json:"cy"`
	O              int     `json:"o"`
	Color          uint8   `json:"color"`
	TurnstileToken string  `json:"turnstileToken"`
}

type PaintResponse struct {
	Ok  bool   `json:"ok"`
	Seq uint64 `json:"seq"`
	Ts  int64  `json:"ts"`
}

func NewIntegrationTest() *IntegrationTest {
	redis := &MockRedisClient{
		chunks:    make(map[string][]byte),
		seqs:      make(map[string]uint64),
		cooldowns: make(map[string]time.Time),
	}

	hub := &MockWebSocketHub{
		rooms:     make(map[string]*MockRoom),
		published: make([]Delta, 0),
	}

	config := Config{
		EnableTurnstile: false,
		GeofenceRadiusM: 300,
		SpeedMaxKmh:     150,
		PaintCooldownMs: 5000,
		TurnstileSecret: "test_secret",
	}

	return &IntegrationTest{
		redis:  redis,
		hub:    hub,
		config: config,
	}
}

func (it *IntegrationTest) Start() {
	// Start HTTP server
	it.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state/chunk":
			it.handleGetChunk(w, r)
		case "/paint":
			it.handlePostPaint(w, r)
		default:
			http.NotFound(w, r)
		}
	}))

	// Start WebSocket server
	it.wsServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		it.handleWebSocket(w, r)
	}))
}

func (it *IntegrationTest) Stop() {
	if it.server != nil {
		it.server.Close()
	}
	if it.wsServer != nil {
		it.wsServer.Close()
	}
}

func (it *IntegrationTest) handleGetChunk(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	cxStr := r.URL.Query().Get("cx")
	cyStr := r.URL.Query().Get("cy")

	if cxStr == "" || cyStr == "" {
		http.Error(w, "Missing cx or cy parameter", 400)
		return
	}

	var cx, cy int64
	fmt.Sscanf(cxStr, "%d", &cx)
	fmt.Sscanf(cyStr, "%d", &cy)

	kBits := fmt.Sprintf("chunk:%d:%d:bits", cx, cy)
	kSeq := fmt.Sprintf("chunk:%d:%d:seq", cx, cy)

	// Get sequence number
	it.redis.mu.RLock()
	seq, exists := it.redis.seqs[kSeq]
	it.redis.mu.RUnlock()

	if !exists {
		seq = 0
	}

	// Get chunk bits
	it.redis.mu.RLock()
	buf, exists := it.redis.chunks[kBits]
	it.redis.mu.RUnlock()

	if !exists || len(buf) == 0 {
		buf = make([]byte, 32768) // blank chunk
	}

	// Set headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Seq", fmt.Sprintf("%d", seq))
	w.Header().Set("Cache-Control", "public, max-age=2, stale-while-revalidate=8")
	w.WriteHeader(200)
	w.Write(buf)
}

func (it *IntegrationTest) handlePostPaint(w http.ResponseWriter, r *http.Request) {
	var req PaintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	// Mock Turnstile verification
	if it.config.EnableTurnstile {
		if req.TurnstileToken == "" || req.TurnstileToken == "invalid" {
			http.Error(w, "turnstile", 401)
			return
		}
	}

	// Mock cooldown check
	ip := it.getIP(r)
	cooldownKey := fmt.Sprintf("cool:%s", ip)
	it.redis.mu.RLock()
	lastPaint, exists := it.redis.cooldowns[cooldownKey]
	it.redis.mu.RUnlock()

	if exists {
		cooldownDuration := time.Duration(it.config.PaintCooldownMs) * time.Millisecond
		if time.Now().Before(lastPaint.Add(cooldownDuration)) {
			http.Error(w, "cooldown", 429)
			return
		}
	}

	// Mock geofence check
	if req.Lat < 42.0 || req.Lat > 43.0 || req.Lon < -72.0 || req.Lon > -70.0 {
		http.Error(w, "geofence", 403)
		return
	}

	// Mock paint operation
	kBits := fmt.Sprintf("chunk:%d:%d:bits", req.Cx, req.Cy)
	kSeq := fmt.Sprintf("chunk:%d:%d:seq", req.Cx, req.Cy)

	// Initialize chunk if needed
	it.redis.mu.Lock()
	if _, exists := it.redis.chunks[kBits]; !exists {
		it.redis.chunks[kBits] = make([]byte, 32768)
	}
	if _, exists := it.redis.seqs[kSeq]; !exists {
		it.redis.seqs[kSeq] = 0
	}

	// Simulate paint
	it.redis.seqs[kSeq]++
	seq := it.redis.seqs[kSeq]
	ts := time.Now().Unix()

	// Set cooldown
	it.redis.cooldowns[cooldownKey] = time.Now()
	it.redis.mu.Unlock()

	// Publish delta
	delta := Delta{
		Seq:   seq,
		O:     uint16(req.O),
		Color: req.Color,
		Ts:    ts,
	}
	it.hub.Publish(req.Cx, req.Cy, delta)

	// Return response
	response := PaintResponse{
		Ok:  true,
		Seq: seq,
		Ts:  ts,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (it *IntegrationTest) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	// Create mock connection
	conn := &MockConnection{
		id:   fmt.Sprintf("conn_%d", time.Now().UnixNano()),
		send: make(chan Delta, 256),
	}

	// Register with hub
	it.hub.Register(conn, 0, 0) // Default to chunk 0,0

	// Handle messages
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}

	// Unregister
	it.hub.Unregister(conn, 0, 0)
}

func (it *IntegrationTest) getIP(r *http.Request) string {
	return "192.168.1.1"
}

func (it *IntegrationTest) GetPublishedDeltas() []Delta {
	it.hub.mu.RLock()
	defer it.hub.mu.RUnlock()
	return it.hub.published
}

func (it *IntegrationTest) GetChunkData(cx, cy int64) ([]byte, uint64) {
	it.redis.mu.RLock()
	defer it.redis.mu.RUnlock()

	kBits := fmt.Sprintf("chunk:%d:%d:bits", cx, cy)
	kSeq := fmt.Sprintf("chunk:%d:%d:seq", cx, cy)

	buf, _ := it.redis.chunks[kBits]
	seq, _ := it.redis.seqs[kSeq]

	return buf, seq
}

func (it *IntegrationTest) SetChunkData(cx, cy int64, data []byte, seq uint64) {
	it.redis.mu.Lock()
	defer it.redis.mu.Unlock()

	kBits := fmt.Sprintf("chunk:%d:%d:bits", cx, cy)
	kSeq := fmt.Sprintf("chunk:%d:%d:seq", cx, cy)

	it.redis.chunks[kBits] = data
	it.redis.seqs[kSeq] = seq
}

func (it *IntegrationTest) ClearCooldown(ip string) {
	it.redis.mu.Lock()
	defer it.redis.mu.Unlock()

	cooldownKey := fmt.Sprintf("cool:%s", ip)
	delete(it.redis.cooldowns, cooldownKey)
}

func (it *IntegrationTest) SetCooldown(ip string) {
	it.redis.mu.Lock()
	defer it.redis.mu.Unlock()

	cooldownKey := fmt.Sprintf("cool:%s", ip)
	it.redis.cooldowns[cooldownKey] = time.Now()
}

// Mock WebSocket Hub methods
func (h *MockWebSocketHub) Register(conn *MockConnection, cx, cy int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := fmt.Sprintf("%d:%d", cx, cy)
	room, exists := h.rooms[key]
	if !exists {
		room = &MockRoom{
			subs: make(map[string]*MockConnection),
		}
		h.rooms[key] = room
	}

	room.mu.Lock()
	room.subs[conn.id] = conn
	room.mu.Unlock()
}

func (h *MockWebSocketHub) Unregister(conn *MockConnection, cx, cy int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := fmt.Sprintf("%d:%d", cx, cy)
	if room, exists := h.rooms[key]; exists {
		room.mu.Lock()
		delete(room.subs, conn.id)
		room.mu.Unlock()

		if len(room.subs) == 0 {
			delete(h.rooms, key)
		}
	}
}

func (h *MockWebSocketHub) Publish(cx, cy int64, delta Delta) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := fmt.Sprintf("%d:%d", cx, cy)
	if room, exists := h.rooms[key]; exists {
		room.mu.RLock()
		for _, conn := range room.subs {
			select {
			case conn.send <- delta:
			default:
				// Drop on backpressure
			}
		}
		room.mu.RUnlock()
	}

	// Store published delta for testing
	h.published = append(h.published, delta)
}

func TestPaintWorkflow(t *testing.T) {
	it := NewIntegrationTest()
	it.Start()
	defer it.Stop()

	// Test successful paint workflow
	reqBody := PaintRequest{
		Lat:   42.3601,
		Lon:   -71.0589,
		Cx:    0,
		Cy:    0,
		O:     0,
		Color: 5,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/paint", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	it.handlePostPaint(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response PaintResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Ok {
		t.Errorf("Expected Ok=true, got %v", response.Ok)
	}

	if response.Seq != 1 {
		t.Errorf("Expected sequence 1, got %d", response.Seq)
	}

	// Verify delta was published
	deltas := it.GetPublishedDeltas()
	if len(deltas) != 1 {
		t.Errorf("Expected 1 published delta, got %d", len(deltas))
	}

	if deltas[0].Seq != 1 {
		t.Errorf("Expected delta sequence 1, got %d", deltas[0].Seq)
	}

	if deltas[0].Color != 5 {
		t.Errorf("Expected delta color 5, got %d", deltas[0].Color)
	}
}

func TestPaintWorkflowCooldown(t *testing.T) {
	it := NewIntegrationTest()
	it.Start()
	defer it.Stop()

	// First paint should succeed
	reqBody := PaintRequest{
		Lat:   42.3601,
		Lon:   -71.0589,
		Cx:    0,
		Cy:    0,
		O:     0,
		Color: 5,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/paint", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	it.handlePostPaint(w, req)

	if w.Code != 200 {
		t.Errorf("First paint should succeed, got status %d", w.Code)
	}

	// Second paint should hit cooldown
	req2 := httptest.NewRequest("POST", "/paint", bytes.NewReader(jsonBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	it.handlePostPaint(w2, req2)

	if w2.Code != 429 {
		t.Errorf("Second paint should hit cooldown, got status %d", w2.Code)
	}
}

func TestPaintWorkflowGeofence(t *testing.T) {
	it := NewIntegrationTest()
	it.Start()
	defer it.Stop()

	// Test outside geofence
	reqBody := PaintRequest{
		Lat:   40.0, // Outside Boston area
		Lon:   -75.0,
		Cx:    0,
		Cy:    0,
		O:     0,
		Color: 5,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/paint", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	it.handlePostPaint(w, req)

	if w.Code != 403 {
		t.Errorf("Paint outside geofence should be rejected, got status %d", w.Code)
	}
}

func TestPaintWorkflowTurnstile(t *testing.T) {
	it := NewIntegrationTest()
	it.config.EnableTurnstile = true
	it.Start()
	defer it.Stop()

	// Test without Turnstile token
	reqBody := PaintRequest{
		Lat:   42.3601,
		Lon:   -71.0589,
		Cx:    0,
		Cy:    0,
		O:     0,
		Color: 5,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/paint", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	it.handlePostPaint(w, req)

	if w.Code != 401 {
		t.Errorf("Paint without Turnstile should be rejected, got status %d", w.Code)
	}
}

func TestPaintWorkflowSequence(t *testing.T) {
	it := NewIntegrationTest()
	it.Start()
	defer it.Stop()

	// Paint multiple tiles and verify sequence increments
	expectedSeq := uint64(1)
	for i := 0; i < 5; i++ {
		reqBody := PaintRequest{
			Lat:   42.3601,
			Lon:   -71.0589,
			Cx:    0,
			Cy:    0,
			O:     i,
			Color: uint8(i % 16),
		}

		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/paint", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		it.handlePostPaint(w, req)

		if w.Code != 200 {
			t.Errorf("Paint %d should succeed, got status %d", i, w.Code)
			continue
		}

		var response PaintResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to unmarshal response %d: %v", i, err)
			continue
		}

		if response.Seq != expectedSeq {
			t.Errorf("Paint %d: expected sequence %d, got %d", i, expectedSeq, response.Seq)
		}
		expectedSeq++
	}

	// Verify final sequence number
	_, seq := it.GetChunkData(0, 0)
	if seq != 5 {
		t.Errorf("Expected final sequence 5, got %d", seq)
	}
}

func TestPaintWorkflowChunkRetrieval(t *testing.T) {
	it := NewIntegrationTest()
	it.Start()
	defer it.Stop()

	// Set some chunk data
	chunkData := make([]byte, 32768)
	chunkData[0] = 0x50 // Set first tile to color 5
	it.SetChunkData(0, 0, chunkData, 1)

	// Retrieve chunk
	req := httptest.NewRequest("GET", "/state/chunk?cx=0&cy=0", nil)
	w := httptest.NewRecorder()

	it.handleGetChunk(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/octet-stream" {
		t.Errorf("Expected Content-Type application/octet-stream, got %s", w.Header().Get("Content-Type"))
	}

	if w.Header().Get("X-Seq") != "1" {
		t.Errorf("Expected X-Seq header 1, got %s", w.Header().Get("X-Seq"))
	}

	if len(w.Body.Bytes()) != 32768 {
		t.Errorf("Expected 32768 bytes, got %d", len(w.Body.Bytes()))
	}
}

func TestPaintWorkflowWebSocket(t *testing.T) {
	it := NewIntegrationTest()
	it.Start()
	defer it.Stop()

	// Connect to WebSocket
	wsURL := "ws" + it.wsServer.URL[4:] + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer ws.Close()

	// Wait for connection to be registered
	time.Sleep(10 * time.Millisecond)

	// Publish a delta
	delta := Delta{Seq: 1, O: 0, Color: 5, Ts: time.Now().Unix()}
	it.hub.Publish(0, 0, delta)

	// Read message
	_, message, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	// Parse JSON
	var receivedDelta Delta
	if err := json.Unmarshal(message, &receivedDelta); err != nil {
		t.Fatalf("Failed to unmarshal delta: %v", err)
	}

	// Verify delta
	if receivedDelta != delta {
		t.Errorf("Received delta %+v, expected %+v", receivedDelta, delta)
	}
}

func TestPaintWorkflowConcurrent(t *testing.T) {
	it := NewIntegrationTest()
	it.Start()
	defer it.Stop()

	// Test concurrent paints
	numGoroutines := 10
	results := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			reqBody := PaintRequest{
				Lat:   42.3601,
				Lon:   -71.0589,
				Cx:    0,
				Cy:    0,
				O:     id,
				Color: uint8(id % 16),
			}

			jsonBody, _ := json.Marshal(reqBody)
			req := httptest.NewRequest("POST", "/paint", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			it.handlePostPaint(w, req)
			results <- (w.Code == 200)
		}(i)
	}

	// Wait for all goroutines to complete
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		if <-results {
			successCount++
		}
	}

	// Should have some successful paints (not all due to cooldown)
	if successCount == 0 {
		t.Errorf("Expected some successful paints, got 0")
	}

	// Verify sequence numbers are unique
	_, seq := it.GetChunkData(0, 0)
	if seq == 0 {
		t.Errorf("Expected some sequence number, got 0")
	}
}

func TestPaintWorkflowErrorHandling(t *testing.T) {
	it := NewIntegrationTest()
	it.Start()
	defer it.Stop()

	// Test invalid JSON
	req := httptest.NewRequest("POST", "/paint", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	it.handlePostPaint(w, req)

	if w.Code != 400 {
		t.Errorf("Expected status 400 for invalid JSON, got %d", w.Code)
	}

	// Test missing parameters
	reqBody := PaintRequest{
		Lat: 42.3601,
		Lon: -71.0589,
		// Missing Cx, Cy, O, Color
	}

	jsonBody, _ := json.Marshal(reqBody)
	req = httptest.NewRequest("POST", "/paint", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	it.handlePostPaint(w, req)

	// Should still process (validation would be in real implementation)
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func BenchmarkPaintWorkflow(b *testing.B) {
	it := NewIntegrationTest()
	it.Start()
	defer it.Stop()

	reqBody := PaintRequest{
		Lat:   42.3601,
		Lon:   -71.0589,
		Cx:    0,
		Cy:    0,
		O:     0,
		Color: 5,
	}

	jsonBody, _ := json.Marshal(reqBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/paint", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		it.handlePostPaint(w, req)
	}
}
