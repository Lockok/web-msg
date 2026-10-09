package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestJWTManager_Generate(t *testing.T) {
	manager := NewJWTManager("test-secret")

	userID := uuid.New()

	tokenString, err := manager.Generate(userID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if tokenString == "" {
		t.Fatal("expected token, got empty string")
	}
}

func TestJWTManager_Validate(t *testing.T) {
	manager := NewJWTManager("test-secret")
	userID := uuid.New()

	tokenString, err := manager.Generate(userID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	gotUserID, err := manager.Validate(tokenString)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}

	if gotUserID != userID {
		t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
	}
}

func TestJWTManager_ValidateWrongSecret(t *testing.T) {
	manager := NewJWTManager("secret-one")
	otherManager := NewJWTManager("secret-two")

	tokenString, err := otherManager.Generate(uuid.New())
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	_, err = manager.Validate(tokenString)
	if err == nil {
		t.Fatal("expected error for token signed with another secret")
	}
}

func TestJWTManager_ValidateExpiredToken(t *testing.T) {
	manager := NewJWTManager("test-secret")

	claims := Claims{
		UserID: uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(manager.secret)
	if err != nil {
		t.Fatalf("generate expired token: %v", err)
	}

	_, err = manager.Validate(tokenString)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}
