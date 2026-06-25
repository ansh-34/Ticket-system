package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned when a token is missing, malformed, expired, or
// signed with an unexpected algorithm/key.
var ErrInvalidToken = errors.New("invalid or expired token")

// TokenManager issues and verifies HS256 JWTs.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewTokenManager builds a TokenManager. ttl is the token lifetime.
func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl, now: time.Now}
}

// Generate returns a signed JWT whose subject is the user ID.
func (m *TokenManager) Generate(userID, email string) (string, error) {
	now := m.now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Verify validates the token signature and expiry and returns the user ID
// (subject claim).
func (m *TokenManager) Verify(tokenString string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		// Reject any token not signed with the expected HMAC algorithm.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}
