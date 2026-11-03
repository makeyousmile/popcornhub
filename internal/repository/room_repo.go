package repository

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"popcornhub/internal/models"
)

var ErrRoomNotFound = errors.New("watch room not found")

type RoomRepo struct {
	collection *mongo.Collection
}

func NewRoomRepo(db *mongo.Database) *RoomRepo {
	return &RoomRepo{
		collection: db.Collection("watch_rooms"),
	}
}

func (r *RoomRepo) Create(ctx context.Context, room *models.WatchRoom) error {
	res, err := r.collection.InsertOne(ctx, room)
	if err != nil {
		return err
	}
	room.ID = res.InsertedID.(bson.ObjectID)
	return nil
}

func (r *RoomRepo) FindByID(ctx context.Context, id bson.ObjectID) (*models.WatchRoom, error) {
	var room models.WatchRoom
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&room)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}
	return &room, nil
}

func (r *RoomRepo) FindActiveByGroupID(ctx context.Context, groupID bson.ObjectID) (*models.WatchRoom, error) {
	var room models.WatchRoom
	// Most recently updated or created room for this group
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})
	err := r.collection.FindOne(ctx, bson.M{"group_id": groupID}, opts).Decode(&room)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}
	return &room, nil
}

func (r *RoomRepo) UpdatePlaybackState(ctx context.Context, roomID bson.ObjectID, state models.PlaybackState) error {
	filter := bson.M{"_id": roomID}
	update := bson.M{
		"$set": bson.M{
			"playback_state": state,
		},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *RoomRepo) AddActiveUser(ctx context.Context, roomID, userID bson.ObjectID) error {
	filter := bson.M{"_id": roomID}
	update := bson.M{
		"$addToSet": bson.M{"active_users": userID},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *RoomRepo) RemoveActiveUser(ctx context.Context, roomID, userID bson.ObjectID) error {
	filter := bson.M{"_id": roomID}
	update := bson.M{
		"$pull": bson.M{"active_users": userID},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}
