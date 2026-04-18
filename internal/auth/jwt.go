package auth

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid or expired token")

// ServiceClaims are the JWT claims used by internal ProStaff services.
type ServiceClaims struct {
	Service string `json:"service"`
	jwt.RegisteredClaims
}

// ValidateServiceToken parses and validates a JWT issued by an internal service.
func ValidateServiceToken(tokenString, secret string) (*ServiceClaims, error) {
	claims := &ServiceClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
