package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

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

// CachedToken represents a token stored in the AUSF's memory.
type CachedToken struct {
	Token     string `json:"token"`
	Scope     string `json:"scope"`
	IssuedTo  string `json:"issuedTo"`
	NFType    string `json:"nfType"`
	ExpiresAt string `json:"expiresAt"`
	CachedAt  string `json:"cachedAt"`
}

// Handler holds AUSF dependencies.
type Handler struct {
	signingKey  []byte
	tokenStore  []CachedToken
	tokenMu     sync.RWMutex
	udmEndpoint string
	udmToken    string
}

// NewHandler creates a new AUSF handler.
func NewHandler(signingKey string) *Handler {
	return &Handler{
		signingKey: []byte(signingKey),
		tokenStore: make([]CachedToken, 0),
	}
}

// AddCachedToken adds a token to the internal store.
func (h *Handler) AddCachedToken(token, scope, issuedTo, nfType string, expiresAt time.Time) {
	h.tokenMu.Lock()
	defer h.tokenMu.Unlock()
	h.tokenStore = append(h.tokenStore, CachedToken{
		Token:     token,
		Scope:     scope,
		IssuedTo:  issuedTo,
		NFType:    nfType,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		CachedAt:  time.Now().UTC().Format(time.RFC3339),
	})
}

// TokenAuthMiddleware validates the JWT but does NOT check scope.
// This is intentionally weak for the debug endpoint.
func (h *Handler) TokenAuthMiddleware(requiredScope string, enforceScope bool) gin.HandlerFunc {
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

		if enforceScope && !strings.Contains(claims.Scope, requiredScope) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("insufficient scope: required '%s', got '%s'", requiredScope, claims.Scope),
			})
			return
		}

		c.Set("nfClaims", claims)
		c.Next()
	}
}

// UEAuthentication handles POST /nausf-auth/v1/ue-authentications
func (h *Handler) UEAuthentication(c *gin.Context) {
	claims, _ := c.Get("nfClaims")
	nfClaims := claims.(*NFClaims)

	var req struct {
		SUPI       string `json:"supiOrSuci"`
		ServingNet string `json:"servingNetworkName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	log.WithFields(log.Fields{
		"supi":       req.SUPI,
		"callerNF":   nfClaims.Subject,
		"callerType": nfClaims.NFType,
	}).Info("UE authentication request received")

	// Simulate calling UDM for authentication vectors
	authVector := map[string]string{
		"rand":  "AABBCCDD11223344AABBCCDD11223344",
		"autn":  "FFEEDDCCBBAA99887766554433221100",
		"xres":  "1122334455667788",
		"kausf": "A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6",
	}

	c.JSON(http.StatusCreated, gin.H{
		"authType":           "5G_AKA",
		"supiOrSuci":         req.SUPI,
		"authenticationVector": authVector,
		"_links": map[string]interface{}{
			"5g-aka": map[string]string{
				"href": fmt.Sprintf("/nausf-auth/v1/ue-authentications/%s/5g-aka-confirmation", req.SUPI),
			},
		},
	})
}

// DebugTokenStore handles GET /debug/token-store
// VULNERABILITY: This endpoint only validates the token signature but does NOT
// check the scope. Any NF with a valid NRF token can access cached tokens.
func (h *Handler) DebugTokenStore(c *gin.Context) {
	claims, _ := c.Get("nfClaims")
	nfClaims := claims.(*NFClaims)

	log.WithFields(log.Fields{
		"callerNF":   nfClaims.Subject,
		"callerType": nfClaims.NFType,
		"scope":      nfClaims.Scope,
	}).Warn("⚠ Debug token store accessed — tokens exposed!")

	h.tokenMu.RLock()
	defer h.tokenMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"service":     "AUSF",
		"tokenCount":  len(h.tokenStore),
		"cachedTokens": h.tokenStore,
	})
}

// Health handles GET /health
func (h *Handler) Health(c *gin.Context) {
	h.tokenMu.RLock()
	count := len(h.tokenStore)
	h.tokenMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"status":       "healthy",
		"service":      "AUSF",
		"cachedTokens": count,
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
