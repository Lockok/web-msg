package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/google/uuid"
)

func TestAuth_MissingToken(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-secret")

	handler := Auth(jwtManager)(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			rec.Code,
		)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-secret")

	handler := Auth(jwtManager)(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d",
			http.StatusUnauthorized, rec.Code)
	}
}

func TestAuth_ValidToken(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-secret")
	expectedID := uuid.New()

	token, err := jwtManager.Generate(expectedID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	handler := Auth(jwtManager)(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		userID, ok := GetUserID(r.Context())
		if !ok {
			t.Error("user ID not found in context")
			return
		}

		if userID != expectedID {
			t.Errorf("expected user ID %s, got %s",
				expectedID, userID)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d",
			http.StatusOK, rec.Code)
	}
}
