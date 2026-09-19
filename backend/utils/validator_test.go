package utils

import (
	"testing"
	"time"
)


func TestValidateSignupInput(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		username    string
		password    string
		expectError bool
	}{
		{
			name:        "Valid input",
			email:       "user@example.com",
			username:    "valid_user123",
			password:    "password123",
			expectError: false,
		},
		{
			name:        "Empty email",
			email:       "",
			username:    "valid_user",
			password:    "password123",
			expectError: true,
		},
		{
			name:        "Invalid email format",
			email:       "not-an-email",
			username:    "valid_user",
			password:    "password123",
			expectError: true,
		},
		{
			name:        "Username too short",
			email:       "user@example.com",
			username:    "ab",
			password:    "password123",
			expectError: true,
		},
		{
			name:        "Username with invalid characters",
			email:       "user@example.com",
			username:    "user!@#",
			password:    "password123",
			expectError: true,
		},
		{
			name:        "Password too short",
			email:       "user@example.com",
			username:    "valid_user",
			password:    "short",
			expectError: true,
		},
		{
			name:        "Password with letters only",
			email:       "user@example.com",
			username:    "valid_user",
			password:    "onlyletters",
			expectError: true,
		},
		{
			name:        "Password with numbers only",
			email:       "user@example.com",
			username:    "valid_user",
			password:    "12345678",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEmail, cleanUsername, err := ValidateSignupInput(tt.email, tt.username, tt.password)
			if tt.expectError && err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
			if !tt.expectError && err != nil {
				t.Errorf("did not expect error for %s, got %v", tt.name, err)
			}
			if !tt.expectError {
				if cleanEmail != tt.email || cleanUsername != tt.username {
					t.Errorf("sanitization mismatch: email=%s, username=%s", cleanEmail, cleanUsername)
				}
			}
		})
	}
}

func TestValidateLoginInput(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		password    string
		expectError bool
	}{
		{
			name:        "Valid login input",
			email:       "user@example.com",
			password:    "password123",
			expectError: false,
		},
		{
			name:        "Empty email",
			email:       "",
			password:    "password123",
			expectError: true,
		},
		{
			name:        "Empty password",
			email:       "user@example.com",
			password:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateLoginInput(tt.email, tt.password)
			if tt.expectError && err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
			if !tt.expectError && err != nil {
				t.Errorf("did not expect error for %s, got %v", tt.name, err)
			}
		})
	}
}

func TestValidatePollInput(t *testing.T) {
	futureTime := time.Now().UTC().Add(1 * time.Hour)
	pastTime := time.Now().UTC().Add(-1 * time.Hour)

	tests := []struct {
		name        string
		question    string
		options     []string
		closesAt    *time.Time
		expectError bool
	}{
		{
			name:        "Valid poll with 2 options",
			question:    "What is your favorite programming language?",
			options:     []string{"Go", "TypeScript"},
			closesAt:    &futureTime,
			expectError: false,
		},
		{
			name:        "Question too short",
			question:    "Why?",
			options:     []string{"Option 1", "Option 2"},
			closesAt:    nil,
			expectError: true,
		},
		{
			name:        "Less than 2 options",
			question:    "What is your favorite language?",
			options:     []string{"Go"},
			closesAt:    nil,
			expectError: true,
		},
		{
			name:        "More than 10 options",
			question:    "Pick a number from one to eleven",
			options:     []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
			closesAt:    nil,
			expectError: true,
		},
		{
			name:        "Duplicate options (case-insensitive)",
			question:    "What do you prefer?",
			options:     []string{"Tea", "Coffee", "tea"},
			closesAt:    nil,
			expectError: true,
		},
		{
			name:        "Empty option text",
			question:    "What is your favorite color?",
			options:     []string{"Red", "  "},
			closesAt:    nil,
			expectError: true,
		},
		{
			name:        "Closing time in past",
			question:    "Will this expire in the past?",
			options:     []string{"Yes", "No"},
			closesAt:    &pastTime,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanQ, cleanOpts, err := ValidatePollInput(tt.question, tt.options, tt.closesAt)
			if tt.expectError && err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
			if !tt.expectError && err != nil {
				t.Errorf("did not expect error for %s, got %v", tt.name, err)
			}
			if !tt.expectError {
				if len(cleanOpts) != len(tt.options) || cleanQ == "" {
					t.Errorf("unexpected output for %s: q=%s, opts=%v", tt.name, cleanQ, cleanOpts)
				}
			}
		})
	}
}

