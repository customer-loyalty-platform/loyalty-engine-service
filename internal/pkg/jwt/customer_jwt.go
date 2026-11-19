package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CustomerClaims struct {
	MemberID      string `json:"member_id"`
	AccountNumber string `json:"account_number,omitempty"`
	TierID        string `json:"tier_id,omitempty"`
	jwt.RegisteredClaims
}

type TokenValidator struct {
	secretKey []byte
}

func NewTokenValidator(secret string) *TokenValidator {
	return &TokenValidator{
		secretKey: []byte(secret),
	}
}

func (tv *TokenValidator) ValidateToken(tokenString string) (*CustomerClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomerClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return tv.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*CustomerClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// GenerateTestToken provides a convenient helper for local testing and development
func (tv *TokenValidator) GenerateTestToken(memberID string, accountNumber string, duration time.Duration) (string, error) {
	claims := CustomerClaims{
		MemberID:      memberID,
		AccountNumber: accountNumber,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   memberID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(tv.secretKey)
}

