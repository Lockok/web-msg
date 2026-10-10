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

func TestJWTManager_ValidateInvalidClaims(t *testing.T) {
	future := jwt.NewNumericDate(time.Now().Add(time.Hour))

	tests := []struct {
		name      string
		subject   string
		expiresAt *jwt.NumericDate
		method    jwt.SigningMethod
	}{
		{
			name:    "missing expiration",
			subject: uuid.New().String(),
			method:  jwt.SigningMethodHS256,
		},
		{
			name:      "missing subject",
			expiresAt: future,
			method:    jwt.SigningMethodHS256,
		},
		{
			name:      "invalid subject",
			subject:   "not-a-uuid",
			expiresAt: future,
			method:    jwt.SigningMethodHS256,
		},
		{
			name:      "nil UUID subject",
			subject:   uuid.Nil.String(),
			expiresAt: future,
			method:    jwt.SigningMethodHS256,
		},
		{
			name:      "unexpected signing method",
			subject:   uuid.New().String(),
			expiresAt: future,
			method:    jwt.SigningMethodHS384,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewJWTManager("test-secret")

			claims := Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   tt.subject,
					ExpiresAt: tt.expiresAt,
				},
			}

			token := jwt.NewWithClaims(tt.method, claims)

			tokenString, err := token.SignedString(manager.secret)
			if err != nil {
				t.Fatalf("sign token: %v", err)
			}

			userID, err := manager.Validate(tokenString)
			if err == nil {
				t.Fatal("expected validation error")
			}

			if userID != uuid.Nil {
				t.Fatal("must not return a user ID for an invalid token")
			}
		})
	}
}

func TestJWTManager_GenerateNilUserID(t *testing.T) {
	manager := NewJWTManager("test-secret")

	tokenString, err := manager.Generate(uuid.Nil)
	if err == nil {
		t.Fatal("expected error for an empty user ID")
	}

	if tokenString != "" {
		t.Fatal("must not generate a token for an empty user ID")
	}
}
