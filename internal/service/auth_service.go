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
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrPasswordTooShort   = errors.New("password must be at least 6 characters")
)

func isValidEmail(email string) bool {
	if len(email) < 5 || len(email) > 120 {
		return false
	}
	at := strings.IndexByte(email, '@')
	if at <= 0 || at >= len(email)-1 {
		return false
	}
	dot := strings.LastIndexByte(email[at:], '.')
	return dot > 0 && dot < len(email[at:])-1
}

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

	if input.Email == "" || input.Name == "" || input.Password == "" {
		return nil, ErrEmptyFields
	}

	if len(input.Password) < 6 {
		return nil, ErrPasswordTooShort
	}

	if len([]rune(input.Name)) > 60 || len([]rune(input.Email)) > 120 {
		return nil, errors.New("name or email is too long")
	}

	if !isValidEmail(input.Email) {
		return nil, ErrInvalidEmail
	}

	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	avatar := strings.TrimSpace(input.AvatarURL)
	if avatar == "" || (!strings.HasPrefix(avatar, "http://") && !strings.HasPrefix(avatar, "https://")) {
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
