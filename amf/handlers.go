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

// CachedToken represents a token stored in the AMF's memory.
type CachedToken struct {
	Token     string `json:"token"`
	Scope     string `json:"scope"`
	IssuedTo  string `json:"issuedTo"`
	NFType    string `json:"nfType"`
	ExpiresAt string `json:"expiresAt"`
	CachedAt  string `json:"cachedAt"`
}

// Handler holds AMF dependencies.
type Handler struct {
	signingKey    []byte
	sessionStore  *SessionStore
	tokenStore    []CachedToken
	tokenMu       sync.RWMutex
	ausfEndpoint  string
	udmEndpoint   string
	ausfToken     string
	udmToken      string
}

// NewHandler creates a new AMF handler.
func NewHandler(signingKey string, ss *SessionStore) *Handler {
	return &Handler{
		signingKey:   []byte(signingKey),
		sessionStore: ss,
		tokenStore:   make([]CachedToken, 0),
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

// TokenAuthMiddleware validates the JWT. If enforceScope is false, any valid token is accepted.
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

// N1N2Message handles POST /namf-comm/v1/ue-contexts/:ueId/n1-n2-messages
func (h *Handler) N1N2Message(c *gin.Context) {
	ueID := c.Param("ueId")
	claims, _ := c.Get("nfClaims")
	nfClaims := claims.(*NFClaims)

	log.WithFields(log.Fields{
		"ueId":       ueID,
		"callerNF":   nfClaims.Subject,
		"callerType": nfClaims.NFType,
	}).Info("N1N2 message received — processing UE context")

	// Create or update session
	session := h.sessionStore.CreateSession(ueID)

	// Record the tokens used during this session
	if h.ausfToken != "" {
		h.sessionStore.AddTokenToSession(ueID, "nausf-auth", h.ausfToken)
	}
	if h.udmToken != "" {
		h.sessionStore.AddTokenToSession(ueID, "nudm-sdm", h.udmToken)
	}

	c.JSON(http.StatusOK, gin.H{
		"ueId":    ueID,
		"status":  "REGISTERED",
		"session": session,
		"message": "UE context created, authentication and subscription data retrieved",
	})
}

// RegisterUE handles POST /namf-comm/v1/ue-registrations
// Simplified UE registration that triggers the full AMF→AUSF→UDM chain
func (h *Handler) RegisterUE(c *gin.Context) {
	var req struct {
		SUPI string `json:"supi"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	log.WithField("supi", req.SUPI).Info("UE registration initiated")

	session := h.sessionStore.CreateSession(req.SUPI)

	// Store tokens used for this registration
	if h.ausfToken != "" {
		h.sessionStore.AddTokenToSession(req.SUPI, "nausf-auth", h.ausfToken)
	}
	if h.udmToken != "" {
		h.sessionStore.AddTokenToSession(req.SUPI, "nudm-sdm", h.udmToken)
	}

	c.JSON(http.StatusCreated, gin.H{
		"supi":    req.SUPI,
		"status":  session.State,
		"message": "UE registered successfully",
	})
}

// DebugTokenStore handles GET /debug/token-store
// VULNERABILITY: This endpoint validates the token signature but does NOT
// check the scope. Any NF with a valid NRF token can read all cached tokens.
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
		"service":      "AMF",
		"tokenCount":   len(h.tokenStore),
		"cachedTokens": h.tokenStore,
		"sessions":     h.sessionStore.ListSessions(),
	})
}

// Health handles GET /health
func (h *Handler) Health(c *gin.Context) {
	h.tokenMu.RLock()
	tokenCount := len(h.tokenStore)
	h.tokenMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"status":       "healthy",
		"service":      "AMF",
		"cachedTokens": tokenCount,
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
