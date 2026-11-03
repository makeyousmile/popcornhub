package service

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"popcornhub/internal/models"
	"popcornhub/internal/repository"
)

type MediaService struct {
	mediaRepo *repository.MediaRepo
}

func NewMediaService(mediaRepo *repository.MediaRepo) *MediaService {
	return &MediaService{
		mediaRepo: mediaRepo,
	}
}

func (s *MediaService) List(ctx context.Context) ([]models.MediaItem, error) {
	return s.mediaRepo.List(ctx)
}

func (s *MediaService) GetByID(ctx context.Context, id bson.ObjectID) (*models.MediaItem, error) {
	return s.mediaRepo.FindByID(ctx, id)
}

func (s *MediaService) Create(ctx context.Context, item *models.MediaItem) error {
	return s.mediaRepo.Create(ctx, item)
}
