package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"popcornhub/internal/auth"
	"popcornhub/internal/config"
	"popcornhub/internal/models"
	"popcornhub/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmptyFields        = errors.New("name, email and password are required")
)

type AuthService struct {
	userRepo *repository.UserRepo
	cfg      *config.Config
}

func NewAuthService(userRepo *repository.UserRepo, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		cfg:      cfg,
	}
}

type RegisterInput struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	AvatarURL string `json:"avatar_url"`
}

type AuthResponse struct {
	User   *models.User    `json:"user"`
	Tokens *auth.TokenPair `json:"tokens"`
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	input.Name = strings.TrimSpace(input.Name)

	if input.Email == "" || input.Name == "" || len(input.Password) < 4 {
		return nil, ErrEmptyFields
	}

	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	avatar := input.AvatarURL
	if avatar == "" {
		avatar = "https://api.dicebear.com/7.x/bottts/svg?seed=" + input.Name
	}

	user := &models.User{
		Name:         input.Name,
		Email:        input.Email,
		PasswordHash: hash,
		AvatarURL:    avatar,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	tokens, err := auth.GenerateTokenPair(
		s.cfg.JWTSecret,
		user.ID.Hex(),
		user.Email,
		user.Name,
		s.cfg.JWTAccessExpiry,
		s.cfg.JWTRefreshExpiry,
	)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:   user,
		Tokens: tokens,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !auth.CheckPassword(password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	tokens, err := auth.GenerateTokenPair(
		s.cfg.JWTSecret,
		user.ID.Hex(),
		user.Email,
		user.Name,
		s.cfg.JWTAccessExpiry,
		s.cfg.JWTRefreshExpiry,
	)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:   user,
		Tokens: tokens,
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*auth.TokenPair, error) {
	claims, err := auth.ValidateRefreshToken(s.cfg.JWTSecret, refreshToken)
	if err != nil {
		return nil, err
	}

	userObjID, err := bson.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return nil, auth.ErrInvalidToken
	}

	user, err := s.userRepo.FindByID(ctx, userObjID)
	if err != nil {
		return nil, auth.ErrInvalidToken
	}

	return auth.GenerateTokenPair(
		s.cfg.JWTSecret,
		user.ID.Hex(),
		user.Email,
		user.Name,
		s.cfg.JWTAccessExpiry,
		s.cfg.JWTRefreshExpiry,
	)
}

func (s *AuthService) GetUser(ctx context.Context, userID bson.ObjectID) (*models.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}
