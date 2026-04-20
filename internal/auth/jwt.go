package auth

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

const GatewayAudience = "prostaff-riot-gateway"

var ErrInvalidToken = errors.New("invalid or expired token")

type ServiceClaims struct {
	Service string `json:"service"`
	jwt.RegisteredClaims
}

// ValidateServiceToken rejects tokens whose aud != GatewayAudience, preventing
// user-facing JWTs from being reused as service tokens even if the secret is shared.
func ValidateServiceToken(tokenString, secret string) (*ServiceClaims, error) {
	claims := &ServiceClaims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(secret), nil
		},
		jwt.WithAudience(GatewayAudience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
