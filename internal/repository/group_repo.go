package repository

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"popcornhub/internal/models"
)

var (
	ErrGroupNotFound      = errors.New("group not found")
	ErrGroupAlreadyExists = errors.New("group with this invite code already exists")
	ErrAlreadyMember      = errors.New("user is already a member of this group")
)

type GroupRepo struct {
	collection *mongo.Collection
}

func NewGroupRepo(db *mongo.Database) *GroupRepo {
	return &GroupRepo{
		collection: db.Collection("groups"),
	}
}

func (r *GroupRepo) Create(ctx context.Context, group *models.Group) error {
	res, err := r.collection.InsertOne(ctx, group)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrGroupAlreadyExists
		}
		return err
	}
	group.ID = res.InsertedID.(bson.ObjectID)
	return nil
}

func (r *GroupRepo) FindByID(ctx context.Context, id bson.ObjectID) (*models.Group, error) {
	var group models.Group
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&group)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return &group, nil
}

func (r *GroupRepo) FindByInviteCode(ctx context.Context, code string) (*models.Group, error) {
	var group models.Group
	err := r.collection.FindOne(ctx, bson.M{"invite_code": code}).Decode(&group)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return &group, nil
}

func (r *GroupRepo) FindByUserID(ctx context.Context, userID bson.ObjectID) ([]models.Group, error) {
	cursor, err := r.collection.Find(ctx, bson.M{
		"members.user_id": userID,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var groups []models.Group
	if err := cursor.All(ctx, &groups); err != nil {
		return nil, err
	}
	if groups == nil {
		groups = []models.Group{}
	}
	return groups, nil
}

func (r *GroupRepo) AddMember(ctx context.Context, groupID bson.ObjectID, member models.GroupMember) error {
	// Atomic update: only push if user is not already in members array
	filter := bson.M{
		"_id":             groupID,
		"members.user_id": bson.M{"$ne": member.UserID},
	}
	update := bson.M{
		"$push": bson.M{"members": member},
	}
	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		// Either group does not exist or user is already a member
		isMem, checkErr := r.IsMember(ctx, groupID, member.UserID)
		if checkErr == nil && isMem {
			return ErrAlreadyMember
		}
		return ErrGroupNotFound
	}
	return nil
}

func (r *GroupRepo) IsMember(ctx context.Context, groupID, userID bson.ObjectID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"_id":             groupID,
		"members.user_id": userID,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
