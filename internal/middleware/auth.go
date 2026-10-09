package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/google/uuid"
)

const userIDKey = "user_id"

func Auth(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler{
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			parts := strings.Fields(header)

			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			userID, err := jwtManager.Validate(parts[1])
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDKey).(uuid.UUID)
	return userID, ok
}