package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/domain/identity"
	"github.com/haksolot/kors/shared/korsctx"
)

type AuthMiddleware struct {
	IdentityRepo  identity.Repository
	JWKSCache     *JWKSCache
	IdentityCache *IdentityCache
}

func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		
		// Legacy system bypass & Dev Mock Service bypass
		appEnv := os.Getenv("APP_ENV")
		jwksEndpoint := os.Getenv("JWKS_ENDPOINT")
		
		if appEnv == "development" && jwksEndpoint == "" {
			if tokenString == "system" {
				idObj, _ := m.IdentityRepo.GetByExternalID(r.Context(), "system")
				if idObj != nil {
					ctx := korsctx.WithIdentity(r.Context(), idObj.ID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				ctx := korsctx.WithIdentity(r.Context(), uuid.Nil)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if strings.HasPrefix(tokenString, "mock-") {
				extID := strings.TrimPrefix(tokenString, "mock-")
				idObj, _ := m.IdentityRepo.GetByExternalID(r.Context(), extID)
				if idObj != nil {
					ctx := korsctx.WithIdentity(r.Context(), idObj.ID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		}

		// Real JWT Parsing with crypto validation
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			kid, _ := token.Header["kid"].(string)
			if m.JWKSCache == nil {
				return nil, fmt.Errorf("JWKS cache not configured")
			}
			return m.JWKSCache.GetKey(r.Context(), kid)
		})

		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		externalID, _ := claims["sub"].(string)

		if m.IdentityCache != nil {
			if c, ok := m.IdentityCache.Get(externalID); ok {
				ctx := korsctx.WithIdentity(r.Context(), c)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		idObj, _ := m.IdentityRepo.GetByExternalID(r.Context(), externalID)
		if idObj != nil {
			if m.IdentityCache != nil {
				m.IdentityCache.Set(externalID, idObj.ID)
			}
			ctx := korsctx.WithIdentity(r.Context(), idObj.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		next.ServeHTTP(w, r)
	})
}
