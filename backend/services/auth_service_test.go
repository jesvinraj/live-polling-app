package services

import (
	"testing"
	"time"

	"live-polling-tool/config"
)

func TestTokenGenerationAndValidation(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:          "test-secret-key-that-is-long-enough-32-chars",
		JWTExpirationHours: 1,
	}

	authService := NewAuthService(nil, cfg)

	userID := "507f1f77bcf86cd799439011"
	email := "test@example.com"
	username := "testuser"

	token, err := authService.GenerateToken(userID, email, username)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := authService.ValidateToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected userID %s, got %s", userID, claims.UserID)
	}

	if claims.Email != email {
		t.Errorf("expected email %s, got %s", email, claims.Email)
	}

	if claims.Username != username {
		t.Errorf("expected username %s, got %s", username, claims.Username)
	}
}

func TestInvalidTokenValidation(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:          "test-secret-key-that-is-long-enough-32-chars",
		JWTExpirationHours: 1,
	}

	authService := NewAuthService(nil, cfg)

	// Test malformed token
	_, err := authService.ValidateToken("invalid.token.string")
	if err == nil {
		t.Error("expected error for malformed token, got nil")
	}

	// Test token with wrong secret
	otherCfg := &config.Config{
		JWTSecret:          "different-secret-key-32-chars-long",
		JWTExpirationHours: 1,
	}
	otherService := NewAuthService(nil, otherCfg)
	otherToken, _ := otherService.GenerateToken("123", "a@b.com", "user")

	_, err = authService.ValidateToken(otherToken)
	if err == nil {
		t.Error("expected signature validation error, got nil")
	}
}

func TestExpiredTokenValidation(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:          "test-secret-key-that-is-long-enough-32-chars",
		JWTExpirationHours: -1, // Expired 1 hour ago
	}

	authService := NewAuthService(nil, cfg)
	token, err := authService.GenerateToken("123", "a@b.com", "user")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Sleep 10ms to ensure expiry is in the past
	time.Sleep(10 * time.Millisecond)

	_, err = authService.ValidateToken(token)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}
