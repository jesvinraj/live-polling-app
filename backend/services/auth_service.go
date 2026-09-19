package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"live-polling-tool/config"
	"live-polling-tool/database"
	"live-polling-tool/models"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type JWTClaims struct {
	UserID   string `json:"userId"`
	Email    string `json:"email"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type AuthService struct {
	db  *database.MongoInstance
	cfg *config.Config
}

func NewAuthService(db *database.MongoInstance, cfg *config.Config) *AuthService {
	return &AuthService{
		db:  db,
		cfg: cfg,
	}
}

func (s *AuthService) Signup(ctx context.Context, email, username, password string) (*models.AuthResponse, error) {
	// Check if user with email already exists
	var existing models.User
	err := s.db.Users.FindOne(ctx, bson.M{"email": email}).Decode(&existing)
	if err == nil {
		return nil, errors.New("a user with this email already exists")
	} else if err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("database query error: %w", err)
	}

	// Check if user with username already exists
	err = s.db.Users.FindOne(ctx, bson.M{"username": username}).Decode(&existing)
	if err == nil {
		return nil, errors.New("this username is already taken")
	} else if err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("database query error: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().UTC()
	newUser := models.User{
		ID:           primitive.NewObjectID(),
		Email:        email,
		Username:     username,
		PasswordHash: string(hashedPassword),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err = s.db.Users.InsertOne(ctx, newUser)
	if err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	token, err := s.GenerateToken(newUser.ID.Hex(), newUser.Email, newUser.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth token: %w", err)
	}

	return &models.AuthResponse{
		Token: token,
		User:  newUser.ToResponse(),
	}, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*models.AuthResponse, error) {
	var user models.User
	err := s.db.Users.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid email or password")
		}
		return nil, fmt.Errorf("database query error: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	token, err := s.GenerateToken(user.ID.Hex(), user.Email, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth token: %w", err)
	}

	return &models.AuthResponse{
		Token: token,
		User:  user.ToResponse(),
	}, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, idStr string) (*models.UserResponse, error) {
	objID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return nil, errors.New("invalid user id")
	}

	var user models.User
	err = s.db.Users.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("database query error: %w", err)
	}

	resp := user.ToResponse()
	return &resp, nil
}

func (s *AuthService) GenerateToken(userID, email, username string) (string, error) {
	expirationTime := time.Now().Add(time.Duration(s.cfg.JWTExpirationHours) * time.Hour)
	claims := &JWTClaims{
		UserID:   userID,
		Email:    email,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "live-polling-tool",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid or expired token")
	}

	return claims, nil
}
