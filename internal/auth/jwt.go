package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTManager struct {
	secret []byte
}

func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
	}
}

type Claims struct {
	jwt.RegisteredClaims
}

func (m *JWTManager) Generate(userID uuid.UUID) (string, error) {
	if userID == uuid.Nil {
		return "", errors.New("user ID must not be empty")
	}

	now := time.Now()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(m.secret)
}

func (m *JWTManager) Validate(tokenString string) (uuid.UUID, error) {
	userID, _, err := m.ValidateWithExpiry(tokenString)
	return userID, err
}

func (m *JWTManager) ValidateWithExpiry(tokenString string) (uuid.UUID, time.Time, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		return uuid.Nil, time.Time{}, err
	}

	if !token.Valid {
		return uuid.Nil, time.Time{}, errors.New("invalid token")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, time.Time{}, errors.New("invalid token subject")
	}

	if userID == uuid.Nil {
		return uuid.Nil, time.Time{}, errors.New("token subject must not be empty")
	}

	if claims.ExpiresAt == nil {
		return uuid.Nil, time.Time{}, errors.New(
			"token expiration must be present",
		)
	}

	return userID, claims.ExpiresAt.Time, nil
}
