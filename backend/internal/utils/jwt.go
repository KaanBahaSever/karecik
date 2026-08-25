package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenTTL, uretilen token'in gecerlilik suresi.
const TokenTTL = 30 * 24 * time.Hour

// Claims, token icinde tasidigimiz veriler.
type Claims struct {
	UserID     string `json:"uid"`
	BusinessID string `json:"bid"`
	jwt.RegisteredClaims
}

// GenerateToken, kullanici ve isletme kimligini iceren imzali bir JWT uretir.
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

// ParseToken, token'i dogrular ve icindeki Claims'i dondurur.
func ParseToken(secret, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("beklenmeyen imzalama yontemi")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("gecersiz token")
	}
	return claims, nil
}
