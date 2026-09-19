package utils

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,30}$`)

func ValidateSignupInput(email, username, password string) (string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	if email == "" {
		return "", "", errors.New("email is required")
	}

	_, err := mail.ParseAddress(email)
	if err != nil {
		return "", "", errors.New("invalid email address format")
	}

	if username == "" {
		return "", "", errors.New("username is required")
	}

	if !usernameRegex.MatchString(username) {
		return "", "", errors.New("username must be between 3 and 30 characters and contain only letters, numbers, underscores, or hyphens")
	}

	if len(password) < 8 {
		return "", "", errors.New("password must be at least 8 characters long")
	}

	if len(password) > 72 {
		return "", "", errors.New("password cannot exceed 72 characters")
	}

	hasLetter := false
	hasNumber := false
	for _, ch := range password {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			hasLetter = true
		} else if ch >= '0' && ch <= '9' {
			hasNumber = true
		}
	}

	if !hasLetter || !hasNumber {
		return "", "", errors.New("password must contain both letters and numbers")
	}

	return email, username, nil
}

func ValidateLoginInput(email, password string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", errors.New("email is required")
	}

	if password == "" {
		return "", errors.New("password is required")
	}

	return email, nil
}

func ValidatePollInput(question string, rawOptions []string, closesAt *time.Time) (string, []string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", nil, errors.New("poll question is required")
	}

	if len(question) < 5 {
		return "", nil, errors.New("poll question must be at least 5 characters long")
	}

	if len(question) > 250 {
		return "", nil, errors.New("poll question cannot exceed 250 characters")
	}

	if len(rawOptions) < 2 {
		return "", nil, errors.New("a poll must have at least 2 options")
	}

	if len(rawOptions) > 10 {
		return "", nil, errors.New("a poll cannot have more than 10 options")
	}

	var cleanedOptions []string
	seen := make(map[string]bool)

	for _, opt := range rawOptions {
		trimmed := strings.TrimSpace(opt)
		if trimmed == "" {
			return "", nil, errors.New("poll options cannot be empty")
		}
		if len(trimmed) > 100 {
			return "", nil, errors.New("poll options cannot exceed 100 characters")
		}
		lower := strings.ToLower(trimmed)
		if seen[lower] {
			return "", nil, errors.New("duplicate poll options are not allowed")
		}
		seen[lower] = true
		cleanedOptions = append(cleanedOptions, trimmed)
	}

	if closesAt != nil {
		if closesAt.Before(time.Now().UTC().Add(1 * time.Minute)) {
			return "", nil, errors.New("poll closing time must be at least 1 minute in the future")
		}
	}

	return question, cleanedOptions, nil
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ValidateVoterToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("voter token is required")
	}
	if len(token) != 36 || !uuidRegex.MatchString(token) {
		return "", errors.New("voter token must be a valid UUID v4 format")
	}
	return strings.ToLower(token), nil
}


