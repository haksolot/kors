package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/domain/identity"
	"github.com/haksolot/kors/shared/korsctx"
)

type AuthMiddleware struct {
	IdentityRepo identity.Repository
	JWKSEndpoint string
}

func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// No auth header, continue as guest or fail later in usecase
			next.ServeHTTP(w, r)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		
		// For now, if token is "system", use our system identity (Convenience for tests)
		if tokenString == "system" {
			sysID := uuid.Nil // Our 0000... ID
			ctx := korsctx.WithIdentity(r.Context(), sysID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Real JWT Logic (Skeleton - would use JWKS in production)
		token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			externalID, _ := claims["sub"].(string)
			if externalID != "" {
				// Find or create identity
				idObj, _ := m.IdentityRepo.GetByExternalID(r.Context(), externalID)
				if idObj != nil {
					ctx := korsctx.WithIdentity(r.Context(), idObj.ID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
