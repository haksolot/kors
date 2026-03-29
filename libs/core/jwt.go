package core

import (
	"context"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Standard role names as defined in MES_REQUIREMENTS.md §12.
const (
	RoleOperator          = "kors-operator"
	RoleSupervisor        = "kors-supervisor"
	RoleProductionManager = "kors-prod-manager"
	RoleQualityManager    = "kors-quality-manager"
	RoleAdmin             = "kors-admin"
)

// Claims holds the extracted fields from a validated JWT.
type Claims struct {
	Subject string
	Email   string
	Roles   []string
}

// HasRole returns true if the user has the specified role.
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole returns true if the user has at least one of the specified roles.
func (c *Claims) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if c.HasRole(role) {
			return true
		}
	}
	return false
}

// JWTValidator validates JWTs against a JWKS endpoint.
// Create one at startup and reuse it — it caches the JWKS internally.
type JWTValidator struct {
	cache   *jwk.Cache
	jwksURL string
	noop    bool // dev mode: skip real validation
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

// NewNoopJWTValidator returns a JWTValidator that accepts any token and returns
// a fixed dev-mode Claims. For local development only — never use in production.
func NewNoopJWTValidator() *JWTValidator {
	return &JWTValidator{noop: true}
}

// ValidateJWT parses and validates the token string, returning the extracted claims.
// Returns an error if the token is invalid, expired, or cannot be verified.
// Never trust the user ID from the request — always use Claims.Subject from this function.
func (v *JWTValidator) ValidateJWT(ctx context.Context, tokenStr string) (*Claims, error) {
	if v.noop {
		return &Claims{
			Subject: "00000000-0000-0000-0000-000000000001",
			Email:   "dev@kors.local",
			Roles:   []string{"kors-admin", "kors-quality", "kors-operateur"},
		}, nil
	}
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
