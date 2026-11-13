package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"popcornhub/internal/models"
)

var (
	ErrResetTokenNotFound = errors.New("reset token not found or already used")
	ErrResetTokenExpired  = errors.New("reset token has expired")
)

type PasswordResetRepo struct {
	collection *mongo.Collection
}

func NewPasswordResetRepo(db *mongo.Database) *PasswordResetRepo {
	return &PasswordResetRepo{
		collection: db.Collection("password_resets"),
	}
}

func (r *PasswordResetRepo) Create(ctx context.Context, reset *models.PasswordReset) error {
	res, err := r.collection.InsertOne(ctx, reset)
	if err != nil {
		return err
	}
	reset.ID = res.InsertedID.(bson.ObjectID)
	return nil
}

func (r *PasswordResetRepo) InvalidateExistingForUser(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.collection.UpdateMany(ctx,
		bson.M{"user_id": userID, "used": false},
		bson.M{"$set": bson.M{"used": true}},
	)
	return err
}

func (r *PasswordResetRepo) FindActiveByTokenHash(ctx context.Context, tokenHash string) (*models.PasswordReset, error) {
	var reset models.PasswordReset
	filter := bson.M{
		"token_hash": tokenHash,
		"used":       false,
	}
	err := r.collection.FindOne(ctx, filter).Decode(&reset)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrResetTokenNotFound
		}
		return nil, err
	}

	if time.Now().UTC().After(reset.ExpiresAt) {
		return nil, ErrResetTokenExpired
	}

	return &reset, nil
}

func (r *PasswordResetRepo) MarkUsed(ctx context.Context, id bson.ObjectID) error {
	_, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"used": true}},
	)
	return err
}
