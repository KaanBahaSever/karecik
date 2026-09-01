package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenTTL is how long an issued token stays valid.
const TokenTTL = 30 * 24 * time.Hour

// Claims carries the data we embed in the token.
type Claims struct {
	UserID     string `json:"uid"`
	BusinessID string `json:"bid"`
	jwt.RegisteredClaims
}

// GenerateToken issues a signed JWT holding the user and business identifiers.
func GenerateToken(secret string, userID, businessID uuid.UUID) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:     userID.String(),
		BusinessID: businessID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "karecik",
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken validates a token and returns the claims inside it.
func ParseToken(secret, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
