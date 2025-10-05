package ws

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Delta represents a paint update message
type Delta struct {
	Seq   uint64 `json:"seq"`
	O     uint16 `json:"o"`
	Color uint8  `json:"color"`
	Ts    int64  `json:"ts"`
}

// Conn represents a WebSocket connection
type Conn struct {
	ws     *websocket.Conn
	send   chan Delta
	hub    *Hub
	roomID string
}

// readPump reads messages from the WebSocket connection
func (c *Conn) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.ws.Close()
	}()

	c.ws.SetReadLimit(512)
	c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log error
			}
			break
		}
	}
}

// writePump writes messages to the WebSocket connection
func (c *Conn) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case delta, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.ws.WriteJSON(delta); err != nil {
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Room represents a chat room for a specific chunk
type Room struct {
	subs map[*Conn]struct{}
	ch   chan Delta
	mu   sync.RWMutex
}

// addSubscriber adds a subscriber to the room
func (r *Room) addSubscriber(conn *Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs[conn] = struct{}{}
}

// removeSubscriber removes a subscriber from the room
func (r *Room) removeSubscriber(conn *Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subs, conn)
}

// broadcast sends a delta to all subscribers in the room
func (r *Room) broadcast(delta Delta) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for conn := range r.subs {
		select {
		case conn.send <- delta:
		default:
			// Drop on backpressure
			close(conn.send)
			delete(r.subs, conn)
		}
	}
}

// Hub manages WebSocket connections and rooms
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]*Room

	register   chan *Conn
	unregister chan *Conn
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]*Room),
		register:   make(chan *Conn),
		unregister: make(chan *Conn),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			roomKey := conn.roomID
			room, exists := h.rooms[roomKey]
			if !exists {
				room = &Room{
					subs: make(map[*Conn]struct{}),
					ch:   make(chan Delta, 256),
				}
				h.rooms[roomKey] = room
			}
			h.mu.Unlock()

			room.addSubscriber(conn)

		case conn := <-h.unregister:
			h.mu.Lock()
			roomKey := conn.roomID
			if room, exists := h.rooms[roomKey]; exists {
				room.removeSubscriber(conn)
				if len(room.subs) == 0 {
					delete(h.rooms, roomKey)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Publish publishes a delta to a specific chunk's room
func (h *Hub) Publish(cx, cy int64, delta Delta) {
	key := fmt.Sprintf("%d:%d", cx, cy)
	h.mu.RLock()
	room, exists := h.rooms[key]
	h.mu.RUnlock()

	if !exists {
		return
	}

	room.broadcast(delta)
}

// GetRoomCount returns the number of active rooms
func (h *Hub) GetRoomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// GetSubscriberCount returns the number of subscribers in a room
func (h *Hub) GetSubscriberCount(roomKey string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if room, exists := h.rooms[roomKey]; exists {
		room.mu.RLock()
		defer room.mu.RUnlock()
		return len(room.subs)
	}
	return 0
}

// RegisterConn registers a new connection with a room ID
func (h *Hub) RegisterConn(ws *websocket.Conn, cx, cy int64) *Conn {
	conn := &Conn{
		ws:     ws,
		send:   make(chan Delta, 256),
		hub:    h,
		roomID: fmt.Sprintf("%d:%d", cx, cy),
	}

	h.register <- conn

	return conn
}
