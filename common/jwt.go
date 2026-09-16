package common

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// BearerPrefix is prepended to the JWT in the authorization header.
	BearerPrefix = "Bearer "
	// MetadataKey is the gRPC metadata key carrying the authorization header.
	MetadataKey = "authorization"
)

// Claims is the JWT payload issued by the auth-service on login. The token is
// signed with a secret shared by every service, which allows any service to
// verify it statelessly without a round-trip to the auth-service.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// SignJWT issues a signed HS256 token for the given identity.
func SignJWT(secret, userID, email, role string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseJWT verifies the token signature and expiry and returns its claims.
func ParseJWT(secret, tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}