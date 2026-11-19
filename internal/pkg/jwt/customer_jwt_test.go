package jwt

import (
	"testing"
	"time"
)

func TestTokenValidator(t *testing.T) {
	secret := "test-secret-key-12345"
	validator := NewTokenValidator(secret)

	memberID := "usr-123456"
	accountNumber := "ACC-7890"

	token, err := validator.GenerateTestToken(memberID, accountNumber, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	claims, err := validator.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.MemberID != memberID {
		t.Errorf("Expected memberID %s, got %s", memberID, claims.MemberID)
	}

	if claims.AccountNumber != accountNumber {
		t.Errorf("Expected accountNumber %s, got %s", accountNumber, claims.AccountNumber)
	}
}

func TestInvalidToken(t *testing.T) {
	validator := NewTokenValidator("secret-a")
	invalidToken := "invalid.jwt.token"

	_, err := validator.ValidateToken(invalidToken)
	if err == nil {
		t.Error("Expected error for invalid token, got nil")
	}
}

