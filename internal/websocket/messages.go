package websocket

import "time"

// Action types
const (
	ActionPlay         = "PLAY"
	ActionPause        = "PAUSE"
	ActionSeek         = "SEEK"
	ActionSyncState    = "SYNC_STATE"
	ActionChatMessage  = "CHAT_MESSAGE"
	ActionReaction     = "REACTION"
	ActionUserJoined   = "USER_JOINED"
	ActionUserLeft     = "USER_LEFT"
	ActionRateLimited  = "RATE_LIMITED"
	ActionHeartbeat    = "HEARTBEAT"
	ActionRoomUsers    = "ROOM_USERS"
	ActionSystemNotice = "SYSTEM_NOTICE"
)

// WSMessage represents the inbound/outbound WebSocket message protocol.
type WSMessage struct {
	Type            string      `json:"type"`
	PositionSec     float64     `json:"position_sec,omitempty"`
	IsPlaying       bool        `json:"is_playing,omitempty"`
	InitiatedByID   string      `json:"initiated_by_id,omitempty"`
	InitiatedByName string      `json:"initiated_by_name,omitempty"`
	ServerTime      int64       `json:"server_time"` // Unix ms
	Text            string      `json:"text,omitempty"`
	Emoji           string      `json:"emoji,omitempty"`
	Message         string      `json:"message,omitempty"`
	ActiveUsers     interface{} `json:"active_users,omitempty"`
	Payload         interface{} `json:"payload,omitempty"`
}

// RoomUserInfo holds active participant info in room.
type RoomUserInfo struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	AvatarURL string `json:"avatar_url"`
}

func NowServerTime() int64 {
	return time.Now().UnixMilli()
}
