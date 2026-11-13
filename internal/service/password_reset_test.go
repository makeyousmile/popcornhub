package service

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"popcornhub/internal/config"
	"popcornhub/internal/models"
)

func TestEmailValidation(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"alice.bob@sub.domain.org", true},
		{"invalid", false},
		{"@domain.com", false},
		{"user@", false},
		{"user@domain", false},
		{"", false},
	}

	for _, tc := range tests {
		got := isValidEmail(tc.email)
		if got != tc.valid {
			t.Errorf("isValidEmail(%q) = %v; expected %v", tc.email, got, tc.valid)
		}
	}
}

func TestPasswordResetTokenExpiry(t *testing.T) {
	now := time.Now().UTC()
	expired := &models.PasswordReset{
		ExpiresAt: now.Add(-1 * time.Minute),
	}
	if !now.After(expired.ExpiresAt) {
		t.Errorf("expected expired token to be detected as expired")
	}

	valid := &models.PasswordReset{
		ExpiresAt: now.Add(15 * time.Minute),
	}
	if now.After(valid.ExpiresAt) {
		t.Errorf("expected valid token not to be expired")
	}
}

func TestEmailServiceLoggingWithoutSMTP(t *testing.T) {
	cfg := &config.Config{
		SMTPHost:   "",
		AppBaseURL: "http://localhost:8085",
	}
	emailService := NewEmailService(cfg)
	err := emailService.SendPasswordReset("test@example.com", "Test User", "http://localhost:8085/?reset_token=abc123")
	if err != nil {
		t.Fatalf("expected nil error when SMTP is not configured, got %v", err)
	}
}
