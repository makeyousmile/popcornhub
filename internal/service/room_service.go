package service

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"popcornhub/internal/models"
	"popcornhub/internal/repository"
)

type RoomService struct {
	roomRepo  *repository.RoomRepo
	groupRepo *repository.GroupRepo
	mediaRepo *repository.MediaRepo
}

func NewRoomService(roomRepo *repository.RoomRepo, groupRepo *repository.GroupRepo, mediaRepo *repository.MediaRepo) *RoomService {
	return &RoomService{
		roomRepo:  roomRepo,
		groupRepo: groupRepo,
		mediaRepo: mediaRepo,
	}
}

type RoomWithMedia struct {
	Room  *models.WatchRoom `json:"room"`
	Media *models.MediaItem `json:"media"`
	Group *models.Group     `json:"group"`
}

func (s *RoomService) GetOrCreateRoom(ctx context.Context, userID, groupID, mediaID bson.ObjectID) (*models.WatchRoom, error) {
	isMem, err := s.groupRepo.IsMember(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !isMem {
		return nil, ErrNotGroupMember
	}

	// Verify media exists
	_, err = s.mediaRepo.FindByID(ctx, mediaID)
	if err != nil {
		return nil, err
	}

	// Look for existing active room for this group
	existing, err := s.roomRepo.FindActiveByGroupID(ctx, groupID)
	if err == nil && existing != nil {
		// If same media, return existing room
		if existing.MediaID == mediaID {
			return existing, nil
		}
	} else if err != nil && !errors.Is(err, repository.ErrRoomNotFound) {
		return nil, err
	}

	// Create new room
	now := time.Now().UTC()
	room := &models.WatchRoom{
		GroupID: groupID,
		MediaID: mediaID,
		PlaybackState: models.PlaybackState{
			IsPlaying:    false,
			PositionSec:  0,
			UpdatedAt:    now,
			LastActionBy: userID,
		},
		ActiveUsers: []bson.ObjectID{userID},
		CreatedAt:   now,
	}

	if err := s.roomRepo.Create(ctx, room); err != nil {
		return nil, err
	}

	return room, nil
}

func (s *RoomService) GetRoomDetails(ctx context.Context, userID, roomID bson.ObjectID) (*RoomWithMedia, error) {
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return nil, err
	}

	isMem, err := s.groupRepo.IsMember(ctx, room.GroupID, userID)
	if err != nil {
		return nil, err
	}
	if !isMem {
		return nil, ErrNotGroupMember
	}

	group, err := s.groupRepo.FindByID(ctx, room.GroupID)
	if err != nil {
		return nil, err
	}

	media, err := s.mediaRepo.FindByID(ctx, room.MediaID)
	if err != nil {
		return nil, err
	}

	return &RoomWithMedia{
		Room:  room,
		Media: media,
		Group: group,
	}, nil
}
