package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// NFClaims represents the JWT claims for an NF access token.
type NFClaims struct {
	jwt.RegisteredClaims
	Scope  string `json:"scope"`
	NFType string `json:"nfType"`
}

// TokenService handles JWT signing and validation.
type TokenService struct {
	signingKey []byte
}

// NewTokenService creates a new TokenService with the given signing key.
func NewTokenService(key string) *TokenService {
	return &TokenService{signingKey: []byte(key)}
}

// IssueToken creates a signed JWT for the given NF.
func (ts *TokenService) IssueToken(nfInstanceID, scope, nfType string) (string, error) {
	now := time.Now()
	claims := NFClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   nfInstanceID,
			Issuer:    "NRF",
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        fmt.Sprintf("%s-%d", nfInstanceID, now.UnixNano()),
		},
		Scope:  scope,
		NFType: nfType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(ts.signingKey)
}

// ValidateToken parses and validates a JWT, returning the claims.
func (ts *TokenService) ValidateToken(tokenString string) (*NFClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &NFClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return ts.signingKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*NFClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
