package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"prostaff-riot-gateway/internal/webutils"
)

type contextKey string

const ServiceContextKey contextKey = "service"

// InternalAuth returns middleware that validates JWTs from internal ProStaff services.
func InternalAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				webutils.ErrorJSON(w, errors.New("authorization header required"), http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				webutils.ErrorJSON(w, errors.New("invalid authorization header format"), http.StatusUnauthorized)
				return
			}

			claims, err := ValidateServiceToken(parts[1], secret)
			if err != nil {
				webutils.ErrorJSON(w, ErrInvalidToken, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ServiceContextKey, claims.Service)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
