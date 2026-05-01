package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
)

// NFClaims mirrors the NRF token claims structure.
type NFClaims struct {
	jwt.RegisteredClaims
	Scope  string `json:"scope"`
	NFType string `json:"nfType"`
}

// Handler holds UDM dependencies.
type Handler struct {
	db         *SubscriberDB
	signingKey []byte
}

// NewHandler creates a new UDM handler.
func NewHandler(db *SubscriberDB, signingKey string) *Handler {
	return &Handler{db: db, signingKey: []byte(signingKey)}
}

// TokenAuthMiddleware validates the NRF-issued JWT and checks the required scope.
func (h *Handler) TokenAuthMiddleware(requiredScope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractBearerToken(c)
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		claims, err := h.validateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token: " + err.Error()})
			return
		}

		// Enforce scope check
		if !strings.Contains(claims.Scope, requiredScope) {
			log.WithFields(log.Fields{
				"requiredScope": requiredScope,
				"tokenScope":   claims.Scope,
				"caller":       claims.Subject,
			}).Warn("Scope mismatch — access denied")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("insufficient scope: required '%s', got '%s'", requiredScope, claims.Scope),
			})
			return
		}

		c.Set("nfClaims", claims)
		c.Next()
	}
}

// GetAMData handles GET /nudm-sdm/v1/:supi/am-data
func (h *Handler) GetAMData(c *gin.Context) {
	supi := c.Param("supi")
	claims, _ := c.Get("nfClaims")
	nfClaims := claims.(*NFClaims)

	log.WithFields(log.Fields{
		"supi":       supi,
		"callerNF":   nfClaims.Subject,
		"callerType": nfClaims.NFType,
		"scope":      nfClaims.Scope,
	}).Info("Subscriber AM data requested")

	subscriber, ok := h.db.GetSubscriber(supi)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscriber not found", "supi": supi})
		return
	}

	c.JSON(http.StatusOK, subscriber)
}

// Health handles GET /health
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":      "healthy",
		"service":     "UDM",
		"subscribers": len(h.db.subscribers),
	})
}

func (h *Handler) validateToken(tokenString string) (*NFClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &NFClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return h.signingKey, nil
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

func extractBearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
