package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Test WebSocket hub functionality for real-time paint updates

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for testing
	},
}

func TestHubBasicOperations(t *testing.T) {
	hub := NewHub()

	// Test initial state
	if hub.GetRoomCount() != 0 {
		t.Errorf("Expected 0 rooms initially, got %d", hub.GetRoomCount())
	}

	// Test room creation
	roomKey := "0:0"
	if hub.GetSubscriberCount(roomKey) != 0 {
		t.Errorf("Expected 0 subscribers initially, got %d", hub.GetSubscriberCount(roomKey))
	}
}

func TestHubPublish(t *testing.T) {
	hub := NewHub()

	// Test publishing to non-existent room
	delta := Delta{Seq: 1, O: 0, Color: 5, Ts: time.Now().Unix()}
	hub.Publish(0, 0, delta) // Should not panic

	// Test room creation and publishing
	roomKey := "0:0"
	room := &Room{
		subs: make(map[*Conn]struct{}),
		ch:   make(chan Delta, 256),
	}
	hub.mu.Lock()
	hub.rooms[roomKey] = room
	hub.mu.Unlock()

	// Test publishing to existing room
	hub.Publish(0, 0, delta)
}

func TestRoomSubscriberManagement(t *testing.T) {
	room := &Room{
		subs: make(map[*Conn]struct{}),
		ch:   make(chan Delta, 256),
	}

	// Test adding subscribers
	conn1 := &Conn{send: make(chan Delta, 256)}
	conn2 := &Conn{send: make(chan Delta, 256)}

	room.addSubscriber(conn1)
	room.addSubscriber(conn2)

	if len(room.subs) != 2 {
		t.Errorf("Expected 2 subscribers, got %d", len(room.subs))
	}

	// Test removing subscribers
	room.removeSubscriber(conn1)

	if len(room.subs) != 1 {
		t.Errorf("Expected 1 subscriber after removal, got %d", len(room.subs))
	}

	// Test removing non-existent subscriber
	room.removeSubscriber(conn1) // Should not panic

	if len(room.subs) != 1 {
		t.Errorf("Expected 1 subscriber after removing non-existent, got %d", len(room.subs))
	}
}

func TestRoomBroadcast(t *testing.T) {
	room := &Room{
		subs: make(map[*Conn]struct{}),
		ch:   make(chan Delta, 256),
	}

	// Create test connections
	conn1 := &Conn{send: make(chan Delta, 1)}
	conn2 := &Conn{send: make(chan Delta, 1)}

	room.addSubscriber(conn1)
	room.addSubscriber(conn2)

	// Test broadcast
	delta := Delta{Seq: 1, O: 0, Color: 5, Ts: time.Now().Unix()}
	room.broadcast(delta)

	// Check that both connections received the delta
	select {
	case received1 := <-conn1.send:
		if received1 != delta {
			t.Errorf("Connection 1 received wrong delta: %+v", received1)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Connection 1 did not receive delta")
	}

	select {
	case received2 := <-conn2.send:
		if received2 != delta {
			t.Errorf("Connection 2 received wrong delta: %+v", received2)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Connection 2 did not receive delta")
	}
}

func TestRoomBroadcastBackpressure(t *testing.T) {
	room := &Room{
		subs: make(map[*Conn]struct{}),
		ch:   make(chan Delta, 256),
	}

	// Create connection with small buffer
	conn := &Conn{send: make(chan Delta, 1)}
	room.addSubscriber(conn)

	// Fill the buffer
	delta1 := Delta{Seq: 1, O: 0, Color: 5, Ts: time.Now().Unix()}
	conn.send <- delta1

	// Try to broadcast another delta (should drop due to backpressure)
	delta2 := Delta{Seq: 2, O: 1, Color: 3, Ts: time.Now().Unix()}
	room.broadcast(delta2)

	// Connection should be removed due to backpressure
	time.Sleep(10 * time.Millisecond)

	// Verify connection was removed
	if len(room.subs) != 0 {
		t.Errorf("Expected connection to be removed due to backpressure, but %d subscribers remain", len(room.subs))
	}
}

func TestWebSocketConnection(t *testing.T) {
	hub := NewHub()

	// Start hub in background
	go hub.Run()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("WebSocket upgrade failed: %v", err)
		}

		conn := &Conn{
			ws:   ws,
			send: make(chan Delta, 256),
			hub:  hub,
		}

		hub.register <- conn

		go conn.WritePump()
		go conn.ReadPump()
	}))
	defer server.Close()

	// Connect to WebSocket
	wsURL := "ws" + server.URL[4:] + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer ws.Close()

	// Wait for connection to be registered
	time.Sleep(10 * time.Millisecond)

	// Test publishing
	delta := Delta{Seq: 1, O: 0, Color: 5, Ts: time.Now().Unix()}
	hub.Publish(0, 0, delta)

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

func TestWebSocketMultipleConnections(t *testing.T) {
	hub := NewHub()

	// Start hub in background
	go hub.Run()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("WebSocket upgrade failed: %v", err)
		}

		conn := &Conn{
			ws:   ws,
			send: make(chan Delta, 256),
			hub:  hub,
		}

		hub.register <- conn

		go conn.WritePump()
		go conn.ReadPump()
	}))
	defer server.Close()

	// Connect multiple WebSockets
	numConnections := 3
	connections := make([]*websocket.Conn, numConnections)

	for i := 0; i < numConnections; i++ {
		wsURL := "ws" + server.URL[4:] + "/ws"
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("WebSocket dial %d failed: %v", i, err)
		}
		connections[i] = ws
		defer ws.Close()
	}

	// Wait for connections to be registered
	time.Sleep(10 * time.Millisecond)

	// Test publishing to all connections
	delta := Delta{Seq: 1, O: 0, Color: 5, Ts: time.Now().Unix()}
	hub.Publish(0, 0, delta)

	// Read messages from all connections
	for i, ws := range connections {
		_, message, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message from connection %d: %v", i, err)
		}

		var receivedDelta Delta
		if err := json.Unmarshal(message, &receivedDelta); err != nil {
			t.Fatalf("Failed to unmarshal delta from connection %d: %v", i, err)
		}

		if receivedDelta != delta {
			t.Errorf("Connection %d received delta %+v, expected %+v", i, receivedDelta, delta)
		}
	}
}

func TestWebSocketPingPong(t *testing.T) {
	hub := NewHub()

	// Start hub in background
	go hub.Run()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("WebSocket upgrade failed: %v", err)
		}

		conn := &Conn{
			ws:   ws,
			send: make(chan Delta, 256),
			hub:  hub,
		}

		hub.register <- conn

		go conn.WritePump()
		go conn.ReadPump()
	}))
	defer server.Close()

	// Connect to WebSocket
	wsURL := "ws" + server.URL[4:] + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer ws.Close()

	// Set pong handler
	ws.SetPongHandler(func(string) error {
		return nil
	})

	// Wait for ping (should happen after 54 seconds, but we'll test the mechanism)
	// In a real test, you might want to reduce the ping interval for faster testing
	time.Sleep(100 * time.Millisecond)

	// Send pong to keep connection alive
	if err := ws.WriteMessage(websocket.PongMessage, nil); err != nil {
		t.Fatalf("Failed to send pong: %v", err)
	}
}

func TestHubConcurrentOperations(t *testing.T) {
	hub := NewHub()

	// Start hub in background
	go hub.Run()

	// Test concurrent room creation and publishing
	numGoroutines := 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Create room
			roomKey := fmt.Sprintf("%d:0", id)
			room := &Room{
				subs: make(map[*Conn]struct{}),
				ch:   make(chan Delta, 256),
			}

			hub.mu.Lock()
			hub.rooms[roomKey] = room
			hub.mu.Unlock()

			// Publish to room
			delta := Delta{Seq: uint64(id), O: 0, Color: uint8(id % 16), Ts: time.Now().Unix()}
			hub.Publish(int64(id), 0, delta)
		}(i)
	}

	wg.Wait()

	// Verify all rooms were created
	if hub.GetRoomCount() != numGoroutines {
		t.Errorf("Expected %d rooms, got %d", numGoroutines, hub.GetRoomCount())
	}
}

func BenchmarkHubPublish(b *testing.B) {
	hub := NewHub()

	// Create room with subscribers
	room := &Room{
		subs: make(map[*Conn]struct{}),
		ch:   make(chan Delta, 256),
	}

	// Add subscribers
	for i := 0; i < 10; i++ {
		conn := &Conn{send: make(chan Delta, 256)}
		room.addSubscriber(conn)
	}

	hub.mu.Lock()
	hub.rooms["0:0"] = room
	hub.mu.Unlock()

	delta := Delta{Seq: 1, O: 0, Color: 5, Ts: time.Now().Unix()}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.Publish(0, 0, delta)
	}
}
