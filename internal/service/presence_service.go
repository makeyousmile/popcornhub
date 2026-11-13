package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"popcornhub/internal/models"
)

const (
	presencePrefix   = "popcornhub:presence:user:"
	presenceTTL      = 45 * time.Second
	PresenceChannel  = "popcornhub:presence:events"
)

type PresenceService struct {
	rdb *redis.Client
}

func NewPresenceService(rdb *redis.Client) *PresenceService {
	return &PresenceService{rdb: rdb}
}

func (s *PresenceService) SetPresence(ctx context.Context, p models.UserPresence) error {
	p.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	key := presencePrefix + p.UserID
	if err := s.rdb.Set(ctx, key, data, presenceTTL).Err(); err != nil {
		return fmt.Errorf("failed to set presence in redis: %w", err)
	}

	// Publish to Redis pub/sub
	if err := s.rdb.Publish(ctx, PresenceChannel, data).Err(); err != nil {
		slog.Warn("Failed to publish presence event", "error", err)
	}

	return nil
}

func (s *PresenceService) Heartbeat(ctx context.Context, userID string) error {
	key := presencePrefix + userID
	return s.rdb.Expire(ctx, key, presenceTTL).Err()
}

func (s *PresenceService) RemovePresence(ctx context.Context, userID, userName, avatarURL string) error {
	key := presencePrefix + userID
	_ = s.rdb.Del(ctx, key).Err()

	// Notify that user is idle / offline
	p := models.UserPresence{
		UserID:    userID,
		UserName:  userName,
		AvatarURL: avatarURL,
		Status:    models.StatusIdle,
		UpdatedAt: time.Now().UTC(),
	}
	data, _ := json.Marshal(p)
	_ = s.rdb.Publish(ctx, PresenceChannel, data).Err()

	return nil
}

func (s *PresenceService) GetPresence(ctx context.Context, userID string) (*models.UserPresence, error) {
	key := presencePrefix + userID
	data, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var p models.UserPresence
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *PresenceService) GetGroupPresence(ctx context.Context, memberUserIDs []string, groupID string) ([]models.UserPresence, error) {
	if len(memberUserIDs) == 0 {
		return []models.UserPresence{}, nil
	}

	keys := make([]string, len(memberUserIDs))
	for i, id := range memberUserIDs {
		keys[i] = presencePrefix + id
	}

	results, err := s.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	presences := make([]models.UserPresence, 0, len(results))
	for _, res := range results {
		if res == nil {
			continue
		}
		str, ok := res.(string)
		if !ok {
			continue
		}
		var p models.UserPresence
		if err := json.Unmarshal([]byte(str), &p); err == nil {
			if groupID != "" && p.GroupID != "" && p.GroupID != groupID {
				// Do not leak media viewing activity from another group
				p.Status = models.StatusIdle
				p.MediaID = ""
				p.MediaTitle = ""
				p.RoomID = ""
			}
			presences = append(presences, p)
		}
	}

	return presences, nil
}

func (s *PresenceService) Subscribe(ctx context.Context) *redis.PubSub {
	return s.rdb.Subscribe(ctx, PresenceChannel)
}
