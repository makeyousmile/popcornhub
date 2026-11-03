package service

import (
	"strings"
	"testing"
)

func TestGenerateInviteCode(t *testing.T) {
	code, err := generateInviteCode("Семья")
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	parts := strings.Split(code, "-")
	if len(parts) != 2 {
		t.Fatalf("expected format PREFIX-NUMBER, got %s", code)
	}

	if len(parts[1]) != 4 {
		t.Fatalf("expected 4 digit number, got %s", parts[1])
	}
}
