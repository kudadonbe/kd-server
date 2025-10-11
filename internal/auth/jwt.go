package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken indicates the provided JWT cannot be trusted.
var ErrInvalidToken = errors.New("auth: invalid token")

// Claims represents the fields expected within kd-server tokens.
type Claims struct {
	Tenant string `json:"tenant"`
	jwt.RegisteredClaims
}

// Verifier validates JWTs and returns the parsed claims.
type Verifier interface {
	Verify(ctx context.Context, token string) (*Claims, error)
}

// HMACVerifier verifies JWTs signed with an HMAC secret.
type HMACVerifier struct {
	secret []byte
	now    func() time.Time
}

// NewHMACVerifier creates a verifier that accepts HS256 signed tokens.
func NewHMACVerifier(secret string) (*HMACVerifier, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("auth: secret is required")
	}
	return &HMACVerifier{
		secret: []byte(secret),
		now:    time.Now,
	}, nil
}

// Verify ensures the token is valid, not expired, and matches the expected schema.
func (v *HMACVerifier) Verify(_ context.Context, token string) (*Claims, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrInvalidToken
	}

	claims := &Claims{}
	parsedToken, err := jwt.ParseWithClaims(
		token,
		claims,
		func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, ErrInvalidToken
			}
			return v.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
		jwt.WithTimeFunc(v.now),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: verify: %w", ErrInvalidToken)
	}

	if !parsedToken.Valid {
		return nil, ErrInvalidToken
	}

	if strings.TrimSpace(claims.Tenant) == "" {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
