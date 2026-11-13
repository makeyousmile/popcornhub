package auth

import (
	"testing"
	"time"
)

func TestHashPassword_And_CheckPassword(t *testing.T) {
	password := "superSecret123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !CheckPassword(password, hash) {
		t.Fatalf("expected password to match hash")
	}

	if CheckPassword("wrongPassword", hash) {
		t.Fatalf("expected wrong password to fail")
	}
}

func TestGenerateTokenPair_And_Validation(t *testing.T) {
	secret := "test-secret-key-12345"
	userID := "60d5ec49f1b2c8b1f8e4e1a1"
	email := "ivan@example.com"
	name := "Иван"

	tokens, err := GenerateTokenPair(secret, userID, email, name, 5*time.Minute, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("tokens should not be empty")
	}

	// Validate access token
	accessClaims, err := ValidateAccessToken(secret, tokens.AccessToken)
	if err != nil {
		t.Fatalf("failed to validate access token: %v", err)
	}
	if accessClaims.UserID != userID || accessClaims.Email != email || accessClaims.Name != name {
		t.Fatalf("access claims mismatch: %+v", accessClaims)
	}

	// Validate refresh token
	refreshClaims, err := ValidateRefreshToken(secret, tokens.RefreshToken)
	if err != nil {
		t.Fatalf("failed to validate refresh token: %v", err)
	}
	if refreshClaims.UserID != userID || refreshClaims.TokenType != "refresh" {
		t.Fatalf("refresh claims mismatch: %+v", refreshClaims)
	}

	// Tampered secret test
	_, err = ValidateAccessToken("wrong-secret", tokens.AccessToken)
	if err == nil {
		t.Fatalf("expected validation with wrong secret to fail")
	}

	// Refresh token must NOT be accepted as access token
	_, err = ValidateAccessToken(secret, tokens.RefreshToken)
	if err == nil {
		t.Fatalf("expected ValidateAccessToken to reject refresh token")
	}
}
