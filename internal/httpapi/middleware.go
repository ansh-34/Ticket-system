package httpapi

import (
	"context"
	"net/http"
	"strings"
)

// ctxKey is an unexported type to avoid context key collisions.
type ctxKey string

const userIDKey ctxKey = "userID"

// authMiddleware enforces a valid "Authorization: Bearer <token>" header and
// stores the authenticated user ID in the request context.
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		prefix := "Bearer "
		if !strings.HasPrefix(header, prefix) {
			writeError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
		userID, err := s.tokens.Verify(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

// userIDFromContext returns the authenticated user ID set by authMiddleware.
func userIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}
