package repository

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"popcornhub/internal/models"
)

var ErrMediaNotFound = errors.New("media item not found")

type MediaRepo struct {
	collection *mongo.Collection
}

func NewMediaRepo(db *mongo.Database) *MediaRepo {
	return &MediaRepo{
		collection: db.Collection("media_catalog"),
	}
}

func (r *MediaRepo) List(ctx context.Context) ([]models.MediaItem, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var list []models.MediaItem
	if err := cursor.All(ctx, &list); err != nil {
		return nil, err
	}
	if list == nil {
		list = []models.MediaItem{}
	}
	return list, nil
}

func (r *MediaRepo) FindByID(ctx context.Context, id bson.ObjectID) (*models.MediaItem, error) {
	var item models.MediaItem
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&item)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrMediaNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *MediaRepo) Create(ctx context.Context, item *models.MediaItem) error {
	res, err := r.collection.InsertOne(ctx, item)
	if err != nil {
		return err
	}
	item.ID = res.InsertedID.(bson.ObjectID)
	return nil
}
