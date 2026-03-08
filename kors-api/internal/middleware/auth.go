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
}

func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		
		// Legacy system bypass
		if tokenString == "system" {
			// Try to find the system identity in DB
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

		// Dev Mock Service bypass: "mock-<external_id>"
		if strings.HasPrefix(tokenString, "mock-") {
			extID := strings.TrimPrefix(tokenString, "mock-")
			idObj, _ := m.IdentityRepo.GetByExternalID(r.Context(), extID)
			if idObj != nil {
				ctx := korsctx.WithIdentity(r.Context(), idObj.ID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Real JWT Parsing (Unverified for local dev simplicity, but extracts sub)
		token, _, _ := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			// sub contains the Keycloak Client ID or User ID
			externalID, _ := claims["sub"].(string)
			
			// We find the identity in KORS matching this Keycloak ID
			idObj, _ := m.IdentityRepo.GetByExternalID(r.Context(), externalID)
			if idObj != nil {
				ctx := korsctx.WithIdentity(r.Context(), idObj.ID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
