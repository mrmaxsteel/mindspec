package viz

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"
)

// Message types sent over WebSocket.
const (
	MsgSnapshot = "snapshot"
	MsgUpdate   = "update"
	MsgStats    = "stats"
)

// WSMessage is the envelope for all WebSocket messages.
type WSMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// StatsData is the payload for stats messages.
type StatsData struct {
	EventsPerSec float64 `json:"eventsPerSec"`
	ErrorCount   int     `json:"errorCount"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
	Connected    bool    `json:"connected"`
	Paused       bool    `json:"paused"`
	Capped       bool    `json:"capped"`
	Dropped      int64   `json:"dropped"`
	Sampling     bool    `json:"sampling"`
	Mode         string  `json:"mode"` // "live" or "replay"
}

// Hub manages WebSocket clients and broadcasts messages.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	dropped    atomic.Int64
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop. Blocks until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for c := range h.clients {
				close(c.send)
				delete(h.clients, c)
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					// Client can't keep up — drop and count
					h.dropped.Add(1)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case h.broadcast <- data:
	default:
		h.dropped.Add(1)
	}
}

// Dropped returns the total count of dropped messages.
func (h *Hub) Dropped() int64 {
	return h.dropped.Load()
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Client represents a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// NewClient creates a new client and registers it with the hub.
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	c := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 64),
	}
	hub.register <- c
	return c
}

// WritePump drains the send channel and writes to the WebSocket.
func (c *Client) WritePump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := c.conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// ReadPump reads from the WebSocket for disconnect detection.
func (c *Client) ReadPump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
	}()

	for {
		_, _, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
	}
}

// SendDirect sends a message directly to this client (e.g., initial snapshot).
func (c *Client) SendDirect(msg WSMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	select {
	case c.send <- data:
		return nil
	default:
		return fmt.Errorf("client send buffer full")
	}
}
