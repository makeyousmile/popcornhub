package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"popcornhub/internal/models"
	"popcornhub/internal/repository"
)

var (
	ErrNotGroupMember = errors.New("user is not a member of this group")
	ErrGroupNameEmpty = errors.New("group name cannot be empty")
)

type GroupService struct {
	groupRepo *repository.GroupRepo
	userRepo  *repository.UserRepo
}

func NewGroupService(groupRepo *repository.GroupRepo, userRepo *repository.UserRepo) *GroupService {
	return &GroupService{
		groupRepo: groupRepo,
		userRepo:  userRepo,
	}
}

func (s *GroupService) CreateGroup(ctx context.Context, ownerID bson.ObjectID, name string) (*models.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrGroupNameEmpty
	}

	inviteCode, err := generateInviteCode(name)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	group := &models.Group{
		Name:       name,
		InviteCode: inviteCode,
		OwnerID:    ownerID,
		Members: []models.GroupMember{
			{
				UserID:   ownerID,
				Role:     models.RoleOwner,
				JoinedAt: now,
			},
		},
		CreatedAt: now,
	}

	if err := s.groupRepo.Create(ctx, group); err != nil {
		return nil, err
	}

	return group, nil
}

func (s *GroupService) JoinGroupByCode(ctx context.Context, userID bson.ObjectID, inviteCode string) (*models.Group, error) {
	inviteCode = strings.TrimSpace(strings.ToUpper(inviteCode))
	group, err := s.groupRepo.FindByInviteCode(ctx, inviteCode)
	if err != nil {
		return nil, err
	}

	member := models.GroupMember{
		UserID:   userID,
		Role:     models.RoleMember,
		JoinedAt: time.Now().UTC(),
	}

	if err := s.groupRepo.AddMember(ctx, group.ID, member); err != nil {
		return nil, err
	}

	// Return updated group
	return s.groupRepo.FindByID(ctx, group.ID)
}

func (s *GroupService) GetUserGroups(ctx context.Context, userID bson.ObjectID) ([]models.Group, error) {
	return s.groupRepo.FindByUserID(ctx, userID)
}

func (s *GroupService) GetGroupDetails(ctx context.Context, userID, groupID bson.ObjectID) (*models.Group, error) {
	isMem, err := s.groupRepo.IsMember(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !isMem {
		return nil, ErrNotGroupMember
	}

	return s.groupRepo.FindByID(ctx, groupID)
}

func (s *GroupService) GetGroupMembers(ctx context.Context, userID, groupID bson.ObjectID) ([]models.GroupMemberDetail, error) {
	isMem, err := s.groupRepo.IsMember(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !isMem {
		return nil, ErrNotGroupMember
	}

	group, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, err
	}

	userIDs := make([]bson.ObjectID, len(group.Members))
	for i, m := range group.Members {
		userIDs[i] = m.UserID
	}

	users, err := s.userRepo.FindByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	userMap := make(map[bson.ObjectID]models.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	details := make([]models.GroupMemberDetail, len(group.Members))
	for i, m := range group.Members {
		u := userMap[m.UserID]
		details[i] = models.GroupMemberDetail{
			UserID:    m.UserID,
			Name:      u.Name,
			Email:     u.Email,
			AvatarURL: u.AvatarURL,
			Role:      m.Role,
			JoinedAt:  m.JoinedAt,
		}
	}

	return details, nil
}

func generateInviteCode(groupName string) (string, error) {
	prefix := "POP"
	var letters []rune
	for _, r := range strings.ToUpper(strings.TrimSpace(groupName)) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			letters = append(letters, r)
		}
	}
	if len(letters) >= 3 {
		prefix = string(letters[:3])
	} else {
		// If unicode/cyrillic, take first 3 runes cleanly
		cleanRunes := []rune(strings.ToUpper(strings.TrimSpace(groupName)))
		if len(cleanRunes) >= 3 {
			prefix = string(cleanRunes[:3])
		}
	}

	n, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		return "", err
	}
	codeNum := n.Int64() + 1000

	return fmt.Sprintf("%s-%04d", prefix, codeNum), nil
}
