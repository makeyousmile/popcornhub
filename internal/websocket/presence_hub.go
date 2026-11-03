package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"popcornhub/internal/auth"
	"popcornhub/internal/config"
	"popcornhub/internal/metrics"
	"popcornhub/internal/models"
	"popcornhub/internal/service"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for dev/API flexibility
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type PresenceClient struct {
	hub       *PresenceHub
	conn      *websocket.Conn
	userID    string
	userName  string
	avatarURL string
	send      chan []byte
	closed    bool
	mu        sync.Mutex
}

type PresenceHub struct {
	clients         map[string]map[*PresenceClient]bool // userID -> set of clients
	mu              sync.RWMutex
	presenceService *service.PresenceService
	cfg             *config.Config
}

func NewPresenceHub(presenceService *service.PresenceService, cfg *config.Config) *PresenceHub {
	h := &PresenceHub{
		clients:         make(map[string]map[*PresenceClient]bool),
		presenceService: presenceService,
		cfg:             cfg,
	}

	go h.listenRedisEvents()

	return h
}

func (h *PresenceHub) listenRedisEvents() {
	ctx := context.Background()
	pubsub := h.presenceService.Subscribe(ctx)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		var presence models.UserPresence
		if err := json.Unmarshal([]byte(msg.Payload), &presence); err != nil {
			continue
		}

		// Broadcast presence event to all connected clients
		h.broadcastToAll([]byte(msg.Payload))
	}
}

func (h *PresenceHub) broadcastToAll(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, userClients := range h.clients {
		for c := range userClients {
			c.Send(data)
		}
	}
}

func (h *PresenceHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		tokenStr = r.Header.Get("Sec-WebSocket-Protocol")
	}
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ValidateAccessToken(h.cfg.JWTSecret, tokenStr)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade presence websocket", "error", err)
		return
	}

	client := &PresenceClient{
		hub:      h,
		conn:     conn,
		userID:   claims.UserID,
		userName: claims.Name,
		send:     make(chan []byte, 64),
	}

	h.register(client)
	metrics.ActiveWebSockets.WithLabelValues("presence").Inc()

	// Set initial idle presence in Redis
	ctx := context.Background()
	_ = h.presenceService.SetPresence(ctx, models.UserPresence{
		UserID:    claims.UserID,
		UserName:  claims.Name,
		Status:    models.StatusIdle,
		UpdatedAt: time.Now().UTC(),
	})

	go client.writePump()
	go client.readPump()
}

func (h *PresenceHub) register(c *PresenceClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[c.userID]; !ok {
		h.clients[c.userID] = make(map[*PresenceClient]bool)
	}
	h.clients[c.userID][c] = true
}

func (h *PresenceHub) unregister(c *PresenceClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if userClients, ok := h.clients[c.userID]; ok {
		delete(userClients, c)
		if len(userClients) == 0 {
			delete(h.clients, c.userID)
			// User has disconnected all presence tabs
			go func(uid, uname string) {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = h.presenceService.RemovePresence(ctx, uid, uname, "")
			}(c.userID, c.userName)
		}
	}
	metrics.ActiveWebSockets.WithLabelValues("presence").Dec()
}

func (c *PresenceClient) Send(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.send <- data:
	default:
		// Drop message if buffer full
	}
}

func (c *PresenceClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.send)
		_ = c.conn.Close()
	}
}

func (c *PresenceClient) readPump() {
	defer func() {
		c.hub.unregister(c)
		c.Close()
	}()

	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var raw map[string]interface{}
		if err := json.Unmarshal(message, &raw); err == nil {
			msgType, _ := raw["type"].(string)
			if msgType == ActionHeartbeat {
				// Client heartbeat every 15s
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				_ = c.hub.presenceService.Heartbeat(ctx, c.userID)
				cancel()
			} else if msgType == "SET_STATUS" {
				st, _ := raw["status"].(string)
				mid, _ := raw["media_id"].(string)
				mt, _ := raw["media_title"].(string)
				rid, _ := raw["room_id"].(string)
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				_ = c.hub.presenceService.SetPresence(ctx, models.UserPresence{
					UserID:     c.userID,
					UserName:   c.userName,
					Status:     models.PresenceStatus(st),
					MediaID:    mid,
					MediaTitle: mt,
					RoomID:     rid,
					UpdatedAt:  time.Now().UTC(),
				})
				cancel()
			}
		}
	}
}

func (c *PresenceClient) writePump() {
	ticker := time.NewTicker(20 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
