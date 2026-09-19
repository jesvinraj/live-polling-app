package config

import (
	"os"
	"testing"
)

func TestJWTSecretInReleaseMode(t *testing.T) {
	// 1. In release mode without JWT_SECRET, LoadConfig must panic
	_ = os.Setenv("GIN_MODE", "release")
	_ = os.Setenv("JWT_SECRET", "")
	defer func() {
		_ = os.Unsetenv("GIN_MODE")
		_ = os.Unsetenv("JWT_SECRET")
	}()

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = LoadConfig()
	}()

	if !panicked {
		t.Error("expected LoadConfig to panic when GIN_MODE=release and JWT_SECRET is empty")
	}

	// 2. In release mode with short JWT_SECRET (< 32 chars), LoadConfig must panic
	_ = os.Setenv("JWT_SECRET", "short-secret-under-32")
	panicked = false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = LoadConfig()
	}()

	if !panicked {
		t.Error("expected LoadConfig to panic when GIN_MODE=release and JWT_SECRET is shorter than 32 chars")
	}

	// 3. In release mode with valid JWT_SECRET (>= 32 chars), LoadConfig must succeed
	validSecret := "a-secure-production-jwt-secret-key-that-is-at-least-32-chars-long"
	_ = os.Setenv("JWT_SECRET", validSecret)
	panicked = false
	var cfg *Config
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		cfg = LoadConfig()
	}()

	if panicked || cfg == nil {
		t.Error("expected LoadConfig to succeed with valid 32+ character JWT_SECRET in release mode")
	} else if cfg.JWTSecret != validSecret {
		t.Errorf("expected JWTSecret %s, got %s", validSecret, cfg.JWTSecret)
	}

	// 4. In debug mode with empty JWT_SECRET, fallback must be used without panic
	_ = os.Setenv("GIN_MODE", "debug")
	_ = os.Setenv("JWT_SECRET", "")
	cfg = LoadConfig()
	if cfg.JWTSecret == "" {
		t.Error("expected debug fallback JWTSecret, got empty string")
	}
}
