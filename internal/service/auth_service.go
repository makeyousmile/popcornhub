package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
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
	userRepo     *repository.UserRepo
	resetRepo    *repository.PasswordResetRepo
	emailService *EmailService
	cfg          *config.Config
}

func NewAuthService(
	userRepo *repository.UserRepo,
	resetRepo *repository.PasswordResetRepo,
	emailService *EmailService,
	cfg *config.Config,
) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		resetRepo:    resetRepo,
		emailService: emailService,
		cfg:          cfg,
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

func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if !isValidEmail(email) {
		return ErrInvalidEmail
	}

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			// Avoid user enumeration
			slog.Info("Password reset requested for non-existent email", "email", email)
			return nil
		}
		return err
	}

	// Invalidate previous tokens
	if err := s.resetRepo.InvalidateExistingForUser(ctx, user.ID); err != nil {
		slog.Warn("Failed to invalidate old reset tokens", "error", err)
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate secure token: %w", err)
	}
	rawToken := hex.EncodeToString(tokenBytes)

	tokenHashBytes := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(tokenHashBytes[:])

	reset := &models.PasswordReset{
		UserID:    user.ID,
		Email:     user.Email,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		CreatedAt: time.Now().UTC(),
		Used:      false,
	}

	if err := s.resetRepo.Create(ctx, reset); err != nil {
		return fmt.Errorf("failed to save password reset record: %w", err)
	}

	baseURL := strings.TrimRight(s.cfg.AppBaseURL, "/")
	resetURL := fmt.Sprintf("%s/?reset_token=%s", baseURL, rawToken)

	if err := s.emailService.SendPasswordReset(user.Email, user.Name, resetURL); err != nil {
		slog.Error("Failed to send password reset email", "error", err)
	}

	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return errors.New("reset token is required")
	}

	if len(newPassword) < 6 {
		return ErrPasswordTooShort
	}

	tokenHashBytes := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(tokenHashBytes[:])

	reset, err := s.resetRepo.FindActiveByTokenHash(ctx, tokenHash)
	if err != nil {
		return err
	}

	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, reset.UserID, newHash); err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}

	if err := s.resetRepo.MarkUsed(ctx, reset.ID); err != nil {
		slog.Warn("Failed to mark reset token as used", "error", err)
	}

	slog.Info("Password successfully reset", "user_id", reset.UserID.Hex())
	return nil
}
