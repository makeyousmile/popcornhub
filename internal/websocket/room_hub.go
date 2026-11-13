package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"

	"popcornhub/internal/auth"
	"popcornhub/internal/config"
	"popcornhub/internal/metrics"
	"popcornhub/internal/models"
	"popcornhub/internal/repository"
	"popcornhub/internal/service"
)

type RoomClient struct {
	hub       *RoomHub
	room      *LocalRoom
	conn      *websocket.Conn
	userID    bson.ObjectID
	userName  string
	avatarURL string
	send      chan []byte
	closed    bool
	mu        sync.Mutex
}

type LocalRoom struct {
	RoomID       bson.ObjectID
	GroupID      bson.ObjectID
	MediaID      bson.ObjectID
	MediaTitle   string
	durationSec  float64
	mu           sync.RWMutex
	isPlaying    bool
	positionSec  float64
	updatedAt    time.Time
	lastActionBy bson.ObjectID
	clients      map[*RoomClient]bool
	pubsub       *redis.PubSub
	stopChan     chan struct{}
	isClosed     bool
}

func (lr *LocalRoom) CurrentPosition() (float64, bool) {
	lr.mu.RLock()
	defer lr.mu.RUnlock()

	if !lr.isPlaying {
		return lr.positionSec, false
	}

	elapsed := time.Since(lr.updatedAt).Seconds()
	pos := lr.positionSec + elapsed
	if lr.durationSec > 0 && pos > lr.durationSec {
		pos = lr.durationSec
	}
	return pos, true
}

type RoomHub struct {
	mu              sync.RWMutex
	rooms           map[string]*LocalRoom // roomID (hex) -> LocalRoom
	roomRepo        *repository.RoomRepo
	groupRepo       *repository.GroupRepo
	mediaRepo       *repository.MediaRepo
	userRepo        *repository.UserRepo
	presenceService *service.PresenceService
	rdb             *redis.Client
	cfg             *config.Config
	limiter         *UserRateLimiter
}

func NewRoomHub(
	roomRepo *repository.RoomRepo,
	groupRepo *repository.GroupRepo,
	mediaRepo *repository.MediaRepo,
	userRepo *repository.UserRepo,
	presenceService *service.PresenceService,
	rdb *redis.Client,
	cfg *config.Config,
) *RoomHub {
	return &RoomHub{
		rooms:           make(map[string]*LocalRoom),
		roomRepo:        roomRepo,
		groupRepo:       groupRepo,
		mediaRepo:       mediaRepo,
		userRepo:        userRepo,
		presenceService: presenceService,
		rdb:             rdb,
		cfg:             cfg,
		limiter:         NewUserRateLimiter(time.Duration(cfg.RateLimitIntervalMs) * time.Millisecond),
	}
}

func (h *RoomHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Path param room_id from r.PathValue("id") (net/http.ServeMux in Go 1.22+)
	roomIDStr := r.PathValue("id")
	if roomIDStr == "" {
		roomIDStr = r.URL.Query().Get("room_id")
	}

	roomOID, err := bson.ObjectIDFromHex(roomIDStr)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

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

	userOID, err := bson.ObjectIDFromHex(claims.UserID)
	if err != nil {
		http.Error(w, "invalid user id in token", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	roomDoc, err := h.roomRepo.FindByID(ctx, roomOID)
	if err != nil {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	// Verify group membership
	isMem, err := h.groupRepo.IsMember(ctx, roomDoc.GroupID, userOID)
	if err != nil || !isMem {
		http.Error(w, "forbidden: not a group member", http.StatusForbidden)
		return
	}

	// Fetch user details for avatar
	userDoc, _ := h.userRepo.FindByID(ctx, userOID)
	avatar := ""
	if userDoc != nil {
		avatar = userDoc.AvatarURL
	}

	// Fetch media title and duration for presence & sync
	mediaTitle := ""
	var durationSec float64
	mediaDoc, _ := h.mediaRepo.FindByID(ctx, roomDoc.MediaID)
	if mediaDoc != nil {
		mediaTitle = mediaDoc.Title
		durationSec = mediaDoc.DurationSec
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade room websocket", "error", err)
		return
	}

	localRoom := h.getOrCreateLocalRoom(roomDoc, mediaTitle, durationSec)

	client := &RoomClient{
		hub:       h,
		room:      localRoom,
		conn:      conn,
		userID:    userOID,
		userName:  claims.Name,
		avatarURL: avatar,
		send:      make(chan []byte, 64),
	}

	h.registerClient(localRoom, client)
	metrics.ActiveWebSockets.WithLabelValues("room").Inc()

	// Update DB active users and presence
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.roomRepo.AddActiveUser(bgCtx, roomOID, userOID)
		_ = h.presenceService.SetPresence(bgCtx, models.UserPresence{
			UserID:     claims.UserID,
			UserName:   claims.Name,
			AvatarURL:  avatar,
			GroupID:    roomDoc.GroupID.Hex(),
			Status:     models.StatusWatching,
			MediaID:    roomDoc.MediaID.Hex(),
			MediaTitle: mediaTitle,
			RoomID:     roomOID.Hex(),
			UpdatedAt:  time.Now().UTC(),
		})
	}()

	// Broadcast USER_JOINED system notification
	joinMsg := WSMessage{
		Type:            ActionUserJoined,
		InitiatedByID:   client.userID.Hex(),
		InitiatedByName: client.userName,
		ServerTime:      NowServerTime(),
		Message:         fmt.Sprintf("%s присоединился к просмотру", client.userName),
	}
	h.broadcastToRedis(localRoom.RoomID.Hex(), joinMsg)

	// Send initial SYNC_STATE directly to the new client
	currPos, isPlaying := localRoom.CurrentPosition()
	activeList := localRoom.GetActiveUsersList()
	syncMsg := WSMessage{
		Type:            ActionSyncState,
		PositionSec:     currPos,
		IsPlaying:       isPlaying,
		InitiatedByID:   client.userID.Hex(),
		InitiatedByName: "server",
		ServerTime:      NowServerTime(),
		ActiveUsers:     activeList,
	}
	data, _ := json.Marshal(syncMsg)
	client.Send(data)

	go client.writePump()
	go client.readPump()
}

func (h *RoomHub) getOrCreateLocalRoom(doc *models.WatchRoom, mediaTitle string, durationSec float64) *LocalRoom {
	h.mu.Lock()
	defer h.mu.Unlock()

	roomKey := doc.ID.Hex()
	if lr, ok := h.rooms[roomKey]; ok {
		lr.mu.RLock()
		closed := lr.isClosed
		lr.mu.RUnlock()
		if !closed {
			return lr
		}
	}

	lr := &LocalRoom{
		RoomID:       doc.ID,
		GroupID:      doc.GroupID,
		MediaID:      doc.MediaID,
		MediaTitle:   mediaTitle,
		durationSec:  durationSec,
		isPlaying:    doc.PlaybackState.IsPlaying,
		positionSec:  doc.PlaybackState.PositionSec,
		updatedAt:    doc.PlaybackState.UpdatedAt,
		lastActionBy: doc.PlaybackState.LastActionBy,
		clients:      make(map[*RoomClient]bool),
		stopChan:     make(chan struct{}),
	}

	// Subscribe to Redis Pub/Sub for cross-instance broadcasts
	channel := "popcornhub:room:" + roomKey
	lr.pubsub = h.rdb.Subscribe(context.Background(), channel)
	go h.listenRedisRoom(lr)

	h.rooms[roomKey] = lr
	return lr
}

func (h *RoomHub) listenRedisRoom(lr *LocalRoom) {
	ch := lr.pubsub.Channel()
	for {
		select {
		case <-lr.stopChan:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var wsMsg WSMessage
			if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err == nil {
				if wsMsg.Type == ActionPlay || wsMsg.Type == ActionPause || wsMsg.Type == ActionSeek {
					lr.mu.Lock()
					actionTime := time.Now().UTC()
					if wsMsg.ServerTime > 0 {
						actionTime = time.UnixMilli(wsMsg.ServerTime).UTC()
					}
					switch wsMsg.Type {
					case ActionPlay:
						lr.isPlaying = true
						lr.positionSec = wsMsg.PositionSec
						lr.updatedAt = actionTime
					case ActionPause:
						lr.isPlaying = false
						lr.positionSec = wsMsg.PositionSec
						lr.updatedAt = actionTime
					case ActionSeek:
						lr.positionSec = wsMsg.PositionSec
						lr.updatedAt = actionTime
					}
					lr.mu.Unlock()
				}
			}
			lr.broadcastLocal([]byte(msg.Payload))
		}
	}
}

func (lr *LocalRoom) broadcastLocal(data []byte) {
	lr.mu.RLock()
	defer lr.mu.RUnlock()

	for c := range lr.clients {
		c.Send(data)
	}
}

func (lr *LocalRoom) GetActiveUsersList() []RoomUserInfo {
	lr.mu.RLock()
	defer lr.mu.RUnlock()

	seen := make(map[string]bool)
	users := make([]RoomUserInfo, 0, len(lr.clients))
	for c := range lr.clients {
		uid := c.userID.Hex()
		if !seen[uid] {
			seen[uid] = true
			users = append(users, RoomUserInfo{
				UserID:    uid,
				UserName:  c.userName,
				AvatarURL: c.avatarURL,
			})
		}
	}
	return users
}

func (h *RoomHub) registerClient(room *LocalRoom, c *RoomClient) {
	room.mu.Lock()
	room.clients[c] = true
	room.mu.Unlock()
}

func (h *RoomHub) unregisterClient(room *LocalRoom, c *RoomClient) {
	room.mu.Lock()
	delete(room.clients, c)
	remaining := len(room.clients)
	room.mu.Unlock()

	metrics.ActiveWebSockets.WithLabelValues("room").Dec()

	// Broadcast USER_LEFT
	leftMsg := WSMessage{
		Type:            ActionUserLeft,
		InitiatedByID:   c.userID.Hex(),
		InitiatedByName: c.userName,
		ServerTime:      NowServerTime(),
		Message:         fmt.Sprintf("%s покинул просмотр", c.userName),
	}
	h.broadcastToRedis(room.RoomID.Hex(), leftMsg)

	// Update DB and presence
	go func(roomOID, userOID bson.ObjectID, uid, uname string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.roomRepo.RemoveActiveUser(bgCtx, roomOID, userOID)
		_ = h.presenceService.SetPresence(bgCtx, models.UserPresence{
			UserID:    uid,
			UserName:  uname,
			Status:    models.StatusIdle,
			UpdatedAt: time.Now().UTC(),
		})
	}(room.RoomID, c.userID, c.userID.Hex(), c.userName)

	// If no local clients left in room, clean up safely
	if remaining == 0 {
		h.mu.Lock()
		room.mu.Lock()
		if len(room.clients) == 0 && !room.isClosed {
			room.isClosed = true
			close(room.stopChan)
			_ = room.pubsub.Close()
			delete(h.rooms, room.RoomID.Hex())
		}
		room.mu.Unlock()
		h.mu.Unlock()
	}
}

func (h *RoomHub) broadcastToRedis(roomID string, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	channel := "popcornhub:room:" + roomID
	err = h.rdb.Publish(context.Background(), channel, data).Err()
	if err != nil {
		slog.Warn("Failed to publish room event to Redis, fallback to local broadcast", "error", err, "room_id", roomID)
		h.mu.RLock()
		lr := h.rooms[roomID]
		h.mu.RUnlock()
		if lr != nil {
			lr.broadcastLocal(data)
		}
	}
}

func (c *RoomClient) Send(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.send <- data:
	default:
		// Client is slow, drop packet to avoid blocking others
	}
}

func (c *RoomClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.send)
		_ = c.conn.Close()
	}
}

func (c *RoomClient) readPump() {
	defer func() {
		c.hub.unregisterClient(c.room, c)
		c.Close()
	}()

	c.conn.SetReadLimit(8192)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, rawMsg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var msg WSMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			continue
		}

		c.handleIncomingAction(msg)
	}
}

func (c *RoomClient) handleIncomingAction(msg WSMessage) {
	now := time.Now().UTC()
	serverTime := NowServerTime()

	switch msg.Type {
	case ActionPlay, ActionPause, ActionSeek:
		// 1. Rate Limiting check (Token Bucket: max 1 per 1.5s per user)
		if !c.hub.limiter.Allow(c.userID.Hex()) {
			limitMsg := WSMessage{
				Type:       ActionRateLimited,
				ServerTime: serverTime,
				Message:    "Слишком частые действия! Подождите 1.5 секунды.",
			}
			data, _ := json.Marshal(limitMsg)
			c.Send(data)
			return
		}

		metrics.RoomActionsTotal.WithLabelValues(msg.Type).Inc()

		// Validate & clamp position
		if msg.PositionSec < 0 {
			msg.PositionSec = 0
		}
		if c.room.durationSec > 0 && msg.PositionSec > c.room.durationSec {
			msg.PositionSec = c.room.durationSec
		}

		// 2. Collaborative Democratic Playback State Update
		c.room.mu.Lock()
		switch msg.Type {
		case ActionPlay:
			c.room.isPlaying = true
			c.room.positionSec = msg.PositionSec
			c.room.updatedAt = now
			c.room.lastActionBy = c.userID
		case ActionPause:
			c.room.isPlaying = false
			c.room.positionSec = msg.PositionSec
			c.room.updatedAt = now
			c.room.lastActionBy = c.userID
		case ActionSeek:
			c.room.positionSec = msg.PositionSec
			c.room.updatedAt = now
			c.room.lastActionBy = c.userID
		}
		currentPos := c.room.positionSec
		isPlaying := c.room.isPlaying
		c.room.mu.Unlock()

		// 3. Persist state asynchronously in MongoDB
		go func(pos float64, playing bool, actBy bson.ObjectID) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = c.hub.roomRepo.UpdatePlaybackState(ctx, c.room.RoomID, models.PlaybackState{
				IsPlaying:    playing,
				PositionSec:  pos,
				UpdatedAt:    now,
				LastActionBy: actBy,
			})
		}(currentPos, isPlaying, c.userID)

		// 4. Broadcast to all users via Redis Pub/Sub
		broadcastMsg := WSMessage{
			Type:            msg.Type,
			PositionSec:     currentPos,
			IsPlaying:       isPlaying,
			InitiatedByID:   c.userID.Hex(),
			InitiatedByName: c.userName,
			ServerTime:      serverTime,
		}
		c.hub.broadcastToRedis(c.room.RoomID.Hex(), broadcastMsg)

	case ActionSyncState:
		// Client requested current synchronization
		pos, isPlaying := c.room.CurrentPosition()
		reply := WSMessage{
			Type:            ActionSyncState,
			PositionSec:     pos,
			IsPlaying:       isPlaying,
			InitiatedByID:   c.userID.Hex(),
			InitiatedByName: c.userName,
			ServerTime:      serverTime,
			ActiveUsers:     c.room.GetActiveUsersList(),
		}
		data, _ := json.Marshal(reply)
		c.Send(data)

	case ActionChatMessage:
		msg.Text = strings.TrimSpace(msg.Text)
		if msg.Text == "" {
			return
		}
		if len([]rune(msg.Text)) > 500 {
			msg.Text = string([]rune(msg.Text)[:500])
		}
		metrics.RoomActionsTotal.WithLabelValues("chat").Inc()

		chatBroadcast := WSMessage{
			Type:            ActionChatMessage,
			InitiatedByID:   c.userID.Hex(),
			InitiatedByName: c.userName,
			Text:            msg.Text,
			ServerTime:      serverTime,
			Payload: map[string]string{
				"avatar_url": c.avatarURL,
			},
		}
		c.hub.broadcastToRedis(c.room.RoomID.Hex(), chatBroadcast)

	case ActionReaction:
		msg.Emoji = strings.TrimSpace(msg.Emoji)
		if msg.Emoji == "" {
			return
		}
		if len([]rune(msg.Emoji)) > 8 {
			msg.Emoji = string([]rune(msg.Emoji)[:8])
		}
		metrics.RoomActionsTotal.WithLabelValues("reaction").Inc()

		reactionBroadcast := WSMessage{
			Type:            ActionReaction,
			InitiatedByID:   c.userID.Hex(),
			InitiatedByName: c.userName,
			Emoji:           msg.Emoji,
			ServerTime:      serverTime,
		}
		c.hub.broadcastToRedis(c.room.RoomID.Hex(), reactionBroadcast)

	case ActionHeartbeat:
		// Keepalive
	}
}

func (c *RoomClient) writePump() {
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
