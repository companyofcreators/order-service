package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Client represents a single WebSocket connection.
type Client struct {
	UserID uuid.UUID
	conn   *websocket.Conn
	hub    *Hub
	send   chan []byte
}

// readPump reads messages from the WebSocket connection.
// Clients only receive order events; we listen for pong/close.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Hub maintains the set of active clients and broadcasts order events to them.
type Hub struct {
	clients    map[uuid.UUID]map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	log        *slog.Logger
}

// OrderEvent is the JSON payload sent over WebSocket to clients.
type OrderEvent struct {
	Type  string          `json:"type"`
	Order json.RawMessage `json:"order,omitempty"`
}

// NewHub creates a new Hub and starts its run loop.
func NewHub(log *slog.Logger) *Hub {
	h := &Hub{
		clients:    make(map[uuid.UUID]map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		log:        log,
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[client.UserID]; !ok {
				h.clients[client.UserID] = make(map[*Client]struct{})
			}
			h.clients[client.UserID][client] = struct{}{}
			h.mu.Unlock()
			h.log.Info("websocket client connected", "user_id", client.UserID.String())

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.UserID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.UserID)
					}
				}
			}
			h.mu.Unlock()
			h.log.Info("websocket client disconnected", "user_id", client.UserID.String())
		}
	}
}

// RegisterClient upgrades an HTTP connection to WebSocket and registers the client.
func (h *Hub) RegisterClient(userID uuid.UUID, conn *websocket.Conn) *Client {
	client := &Client{
		UserID: userID,
		conn:   conn,
		hub:    h,
		send:   make(chan []byte, 64),
	}
	h.register <- client

	go client.writePump()
	go client.readPump()

	// Send welcome message.
	welcome, _ := json.Marshal(OrderEvent{Type: "order.connected"})
	select {
	case client.send <- welcome:
	default:
	}

	return client
}

// Broadcast sends a JSON message to all connected clients.
func (h *Hub) Broadcast(eventType string, orderData json.RawMessage) {
	msg := OrderEvent{Type: eventType, Order: orderData}

	data, err := json.Marshal(msg)
	if err != nil {
		h.log.Error("failed to marshal order event", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, clients := range h.clients {
		for client := range clients {
			select {
			case client.send <- data:
			default:
				h.log.Warn("client send buffer full, disconnecting", "user_id", client.UserID.String())
				go func(c *Client) { h.unregister <- c }(client)
			}
		}
	}
}
