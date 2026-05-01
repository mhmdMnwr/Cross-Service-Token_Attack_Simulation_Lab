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

// PDUSession represents a PDU session context.
type PDUSession struct {
	SessionID string `json:"sessionId"`
	SUPI      string `json:"supi"`
	DNN       string `json:"dnn"`
	SST       int    `json:"sst"`
	SD        string `json:"sd"`
	State     string `json:"state"`
	CreatedAt string `json:"createdAt"`
	UPFAddr   string `json:"upfAddress"`
	IPAddr    string `json:"ueIpAddress"`
}

// Handler holds SMF dependencies.
type Handler struct {
	signingKey  []byte
	sessions    map[string]*PDUSession
	sessionMu   sync.RWMutex
	sessionCounter int
	udmEndpoint string
	udmToken    string
}

// NewHandler creates a new SMF handler.
func NewHandler(signingKey string) *Handler {
	return &Handler{
		signingKey: []byte(signingKey),
		sessions:   make(map[string]*PDUSession),
	}
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

		if requiredScope != "" && !strings.Contains(claims.Scope, requiredScope) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("insufficient scope: required '%s', got '%s'", requiredScope, claims.Scope),
			})
			return
		}

		c.Set("nfClaims", claims)
		c.Next()
	}
}

// CreateSMContext handles POST /nsmf-pdusession/v1/sm-contexts
func (h *Handler) CreateSMContext(c *gin.Context) {
	claims, _ := c.Get("nfClaims")
	nfClaims := claims.(*NFClaims)

	var req struct {
		SUPI string `json:"supi"`
		DNN  string `json:"dnn"`
		SST  int    `json:"sst"`
		SD   string `json:"sd"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	h.sessionMu.Lock()
	h.sessionCounter++
	sessionID := fmt.Sprintf("pdu-session-%04d", h.sessionCounter)
	session := &PDUSession{
		SessionID: sessionID,
		SUPI:      req.SUPI,
		DNN:       req.DNN,
		SST:       req.SST,
		SD:        req.SD,
		State:     "ACTIVE",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		UPFAddr:   "192.168.100.1",
		IPAddr:    fmt.Sprintf("10.60.0.%d", h.sessionCounter),
	}
	h.sessions[sessionID] = session
	h.sessionMu.Unlock()

	log.WithFields(log.Fields{
		"sessionId":  sessionID,
		"supi":       req.SUPI,
		"dnn":        req.DNN,
		"callerNF":   nfClaims.Subject,
		"callerType": nfClaims.NFType,
	}).Info("PDU session created")

	c.JSON(http.StatusCreated, session)
}

// Health handles GET /health
func (h *Handler) Health(c *gin.Context) {
	h.sessionMu.RLock()
	count := len(h.sessions)
	h.sessionMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"status":        "healthy",
		"service":       "SMF",
		"activeSessions": count,
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
