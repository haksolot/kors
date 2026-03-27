package core

import (
	"context"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Claims holds the extracted fields from a validated JWT.
type Claims struct {
	Subject string
	Email   string
	Roles   []string
}

// JWTValidator validates JWTs against a JWKS endpoint.
// Create one at startup and reuse it — it caches the JWKS internally.
type JWTValidator struct {
	cache *jwk.Cache
	jwksURL string
}

// NewJWTValidator creates a validator that fetches and caches the JWKS from jwksURL.
// The cache refreshes automatically. Call this once at service startup.
func NewJWTValidator(ctx context.Context, jwksURL string) (*JWTValidator, error) {
	cache := jwk.NewCache(ctx)

	if err := cache.Register(jwksURL, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
		return nil, fmt.Errorf("NewJWTValidator: register JWKS %s: %w", jwksURL, err)
	}

	// Initial fetch to fail fast if the JWKS endpoint is unreachable.
	if _, err := cache.Refresh(ctx, jwksURL); err != nil {
		return nil, fmt.Errorf("NewJWTValidator: initial JWKS fetch from %s: %w", jwksURL, err)
	}

	return &JWTValidator{cache: cache, jwksURL: jwksURL}, nil
}

// ValidateJWT parses and validates the token string, returning the extracted claims.
// Returns an error if the token is invalid, expired, or cannot be verified.
// Never trust the user ID from the request — always use Claims.Subject from this function.
func (v *JWTValidator) ValidateJWT(ctx context.Context, tokenStr string) (*Claims, error) {
	keySet, err := v.cache.Get(ctx, v.jwksURL)
	if err != nil {
		return nil, fmt.Errorf("ValidateJWT: get JWKS: %w", err)
	}

	token, err := jwt.Parse([]byte(tokenStr),
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("ValidateJWT: parse token: %w", err)
	}

	claims := &Claims{
		Subject: token.Subject(),
	}

	if email, ok := token.Get("email"); ok {
		if s, ok := email.(string); ok {
			claims.Email = s
		}
	}

	if roles, ok := token.Get("realm_access"); ok {
		if ra, ok := roles.(map[string]any); ok {
			if r, ok := ra["roles"].([]any); ok {
				for _, role := range r {
					if s, ok := role.(string); ok {
						claims.Roles = append(claims.Roles, s)
					}
				}
			}
		}
	}

	return claims, nil
}
