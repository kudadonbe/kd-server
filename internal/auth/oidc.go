package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// ErrOIDCInvalid indicates an external-IdP token could not be trusted.
var ErrOIDCInvalid = errors.New("auth: invalid idp token")

// OIDCClaims are the verified claims of an external-IdP token. Raw exposes the
// full claim set so callers can read provider-specific claims (role, national
// ID) named in the tenant's provider config.
type OIDCClaims struct {
	Subject string
	Email   string
	Raw     map[string]any
}

// OIDCVerifier verifies external-IdP (RS256) browser tokens against a provider's
// JWKS, enforcing issuer and audience.
type OIDCVerifier interface {
	VerifyOIDC(ctx context.Context, issuer, audience, jwksURL, token string) (*OIDCClaims, error)
}

// JWKSVerifier implements OIDCVerifier. JWKS key sets are fetched once per URL
// and cached with background rotation, so verification is a local operation
// after warm-up.
type JWKSVerifier struct {
	mu   sync.Mutex
	sets map[string]keyfunc.Keyfunc
}

// NewJWKSVerifier builds an empty verifier; key sets load lazily on first use.
func NewJWKSVerifier() *JWKSVerifier {
	return &JWKSVerifier{sets: make(map[string]keyfunc.Keyfunc)}
}

func (v *JWKSVerifier) keyfuncFor(jwksURL string) (keyfunc.Keyfunc, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if kf, ok := v.sets[jwksURL]; ok {
		return kf, nil
	}
	// NewDefault uses a background context so rotation outlives any one request.
	kf, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("auth: jwks load: %w", err)
	}
	v.sets[jwksURL] = kf
	return kf, nil
}

// VerifyOIDC verifies the token's RS256 signature against the provider's JWKS,
// then checks issuer, audience, and expiry. It returns the verified claims.
func (v *JWKSVerifier) VerifyOIDC(_ context.Context, issuer, audience, jwksURL, token string) (*OIDCClaims, error) {
	kf, err := v.keyfuncFor(jwksURL)
	if err != nil {
		return nil, err
	}

	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, kf.Keyfunc,
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("auth: verify idp: %w", ErrOIDCInvalid)
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, ErrOIDCInvalid
	}
	email, _ := claims["email"].(string)
	return &OIDCClaims{Subject: sub, Email: email, Raw: claims}, nil
}

// PeekToken reads a JWT's issuer and algorithm WITHOUT verifying it, so the
// caller can route to the right verifier (HS256 = kd-server, RS256 = external
// IdP) and select the tenant provider by issuer.
func PeekToken(token string) (issuer, alg string, err error) {
	parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return "", "", err
	}
	claims, _ := parsed.Claims.(jwt.MapClaims)
	issuer, _ = claims["iss"].(string)
	alg, _ = parsed.Header["alg"].(string)
	return issuer, alg, nil
}
