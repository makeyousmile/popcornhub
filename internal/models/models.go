package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// User represents a user document in the "users" collection.
type User struct {
	ID           bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string        `bson:"name" json:"name"`
	Email        string        `bson:"email" json:"email"`
	PasswordHash string        `bson:"password_hash" json:"-"`
	AvatarURL    string        `bson:"avatar_url" json:"avatar_url"`
	CreatedAt    time.Time     `bson:"created_at" json:"created_at"`
}

// GroupRole represents a member's role in a group.
type GroupRole string

const (
	RoleOwner  GroupRole = "owner"
	RoleAdmin  GroupRole = "admin"
	RoleMember GroupRole = "member"
)

// GroupMember represents an embedded member document in a group.
type GroupMember struct {
	UserID   bson.ObjectID `bson:"user_id" json:"user_id"`
	Role     GroupRole     `bson:"role" json:"role"`
	JoinedAt time.Time     `bson:"joined_at" json:"joined_at"`
}

// GroupMemberDetail contains populated user details for members.
type GroupMemberDetail struct {
	UserID    bson.ObjectID `json:"user_id"`
	Name      string        `json:"name"`
	Email     string        `json:"email"`
	AvatarURL string        `json:"avatar_url"`
	Role      GroupRole     `json:"role"`
	JoinedAt  time.Time     `json:"joined_at"`
}

// Group represents a group document in the "groups" collection.
type Group struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name       string        `bson:"name" json:"name"`
	InviteCode string        `bson:"invite_code" json:"invite_code"`
	OwnerID    bson.ObjectID `bson:"owner_id" json:"owner_id"`
	Members    []GroupMember `bson:"members" json:"members"`
	CreatedAt  time.Time     `bson:"created_at" json:"created_at"`
}

// MediaMetadata represents metadata about a media item.
type MediaMetadata struct {
	Genre []string `bson:"genre" json:"genre"`
	Year  int      `bson:"year" json:"year"`
}

// MediaItem represents a movie or show in the "media_catalog" collection.
type MediaItem struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Title       string        `bson:"title" json:"title"`
	StreamURL   string        `bson:"stream_url" json:"stream_url"`
	DurationSec float64       `bson:"duration_sec" json:"duration_sec"`
	PosterURL   string        `bson:"poster_url" json:"poster_url"`
	Metadata    MediaMetadata `bson:"metadata" json:"metadata"`
}

// PlaybackState represents current playback position and status.
type PlaybackState struct {
	IsPlaying    bool          `bson:"is_playing" json:"is_playing"`
	PositionSec  float64       `bson:"position_sec" json:"position_sec"`
	UpdatedAt    time.Time     `bson:"updated_at" json:"updated_at"`
	LastActionBy bson.ObjectID `bson:"last_action_by" json:"last_action_by"`
}

// WatchRoom represents an active or persistent watch room in "watch_rooms".
type WatchRoom struct {
	ID            bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	GroupID       bson.ObjectID   `bson:"group_id" json:"group_id"`
	MediaID       bson.ObjectID   `bson:"media_id" json:"media_id"`
	PlaybackState PlaybackState   `bson:"playback_state" json:"playback_state"`
	ActiveUsers   []bson.ObjectID `bson:"active_users" json:"active_users"`
	CreatedAt     time.Time       `bson:"created_at" json:"created_at"`
}

// PresenceStatus represents user's presence state.
type PresenceStatus string

const (
	StatusIdle     PresenceStatus = "idle"
	StatusWatching PresenceStatus = "watching"
)

// UserPresence represents dynamic presence stored in Redis and transmitted over WebSockets.
type UserPresence struct {
	UserID     string         `json:"user_id"`
	UserName   string         `json:"user_name"`
	AvatarURL  string         `json:"avatar_url"`
	GroupID    string         `json:"group_id,omitempty"`
	Status     PresenceStatus `json:"status"`
	MediaID    string         `json:"media_id,omitempty"`
	MediaTitle string         `json:"media_title,omitempty"`
	RoomID     string         `json:"room_id,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at"`
}
