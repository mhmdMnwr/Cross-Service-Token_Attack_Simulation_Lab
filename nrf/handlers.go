package main

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// NFProfile represents a registered Network Function.
type NFProfile struct {
	NFInstanceID string   `json:"nfInstanceId"`
	NFType       string   `json:"nfType"`
	NFStatus     string   `json:"nfStatus"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	Services     []string `json:"nfServices"`
	RegisteredAt string   `json:"registeredAt"`
}

// NFRegistry is an in-memory store of registered NF profiles.
type NFRegistry struct {
	mu  sync.RWMutex
	nfs map[string]*NFProfile
}

// NewNFRegistry creates an empty registry.
func NewNFRegistry() *NFRegistry {
	return &NFRegistry{nfs: make(map[string]*NFProfile)}
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	tokenService *TokenService
	registry     *NFRegistry
}

// NewHandler creates a new Handler.
func NewHandler(ts *TokenService, reg *NFRegistry) *Handler {
	return &Handler{tokenService: ts, registry: reg}
}

// RequestLogger is gin middleware that logs every request.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.WithFields(log.Fields{
			"method":   c.Request.Method,
			"path":     c.Request.URL.Path,
			"status":   c.Writer.Status(),
			"latency":  time.Since(start).String(),
			"clientIP": c.ClientIP(),
		}).Info("SBI request processed")
	}
}

// RegisterNF handles PUT /nnrf-nfm/v1/nf-instances/:nfInstanceId
func (h *Handler) RegisterNF(c *gin.Context) {
	nfInstanceID := c.Param("nfInstanceId")

	var profile NFProfile
	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid NF profile: " + err.Error()})
		return
	}

	profile.NFInstanceID = nfInstanceID
	profile.NFStatus = "REGISTERED"
	profile.RegisteredAt = time.Now().UTC().Format(time.RFC3339)

	h.registry.mu.Lock()
	h.registry.nfs[nfInstanceID] = &profile
	h.registry.mu.Unlock()

	log.WithFields(log.Fields{
		"nfInstanceId": nfInstanceID,
		"nfType":       profile.NFType,
		"host":         profile.Host,
		"port":         profile.Port,
	}).Info("NF registered successfully")

	c.JSON(http.StatusCreated, profile)
}

// DeregisterNF handles DELETE /nnrf-nfm/v1/nf-instances/:nfInstanceId
func (h *Handler) DeregisterNF(c *gin.Context) {
	nfInstanceID := c.Param("nfInstanceId")

	h.registry.mu.Lock()
	delete(h.registry.nfs, nfInstanceID)
	h.registry.mu.Unlock()

	log.WithField("nfInstanceId", nfInstanceID).Info("NF deregistered")
	c.Status(http.StatusNoContent)
}

// ListNFs handles GET /nnrf-nfm/v1/nf-instances
func (h *Handler) ListNFs(c *gin.Context) {
	h.registry.mu.RLock()
	defer h.registry.mu.RUnlock()

	profiles := make([]*NFProfile, 0, len(h.registry.nfs))
	for _, p := range h.registry.nfs {
		profiles = append(profiles, p)
	}

	c.JSON(http.StatusOK, gin.H{"nfInstances": profiles})
}

// DiscoverNFs handles GET /nnrf-disc/v1/nf-instances?target-nf-type=X
func (h *Handler) DiscoverNFs(c *gin.Context) {
	// Validate token
	tokenString := extractBearerToken(c)
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
		return
	}
	claims, err := h.tokenService.ValidateToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token: " + err.Error()})
		return
	}

	targetNFType := c.Query("target-nf-type")
	log.WithFields(log.Fields{
		"caller":       claims.Subject,
		"callerType":   claims.NFType,
		"targetNFType": targetNFType,
	}).Info("NF discovery request")

	h.registry.mu.RLock()
	defer h.registry.mu.RUnlock()

	var results []*NFProfile
	for _, p := range h.registry.nfs {
		if targetNFType == "" || strings.EqualFold(p.NFType, targetNFType) {
			results = append(results, p)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"validityPeriod": 3600,
		"nfInstances":    results,
	})
}

// IssueToken handles POST /oauth2/token
// Accepts: grant_type=client_credentials, scope, nfInstanceId, nfType
func (h *Handler) IssueToken(c *gin.Context) {
	grantType := c.PostForm("grant_type")
	if grantType != "client_credentials" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported grant_type"})
		return
	}

	nfInstanceID := c.PostForm("nfInstanceId")
	scope := c.PostForm("scope")
	nfType := c.PostForm("nfType")

	if nfInstanceID == "" || scope == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nfInstanceId and scope are required"})
		return
	}

	// VULNERABILITY: NRF does not validate whether the requesting NF type
	// is authorized for the requested scope. Any registered NF can request
	// any scope. This is the root cause of the cross-service token attack.
	tokenString, err := h.tokenService.IssueToken(nfInstanceID, scope, nfType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	log.WithFields(log.Fields{
		"nfInstanceId": nfInstanceID,
		"scope":        scope,
		"nfType":       nfType,
	}).Warn("Access token issued (no scope-to-NF-type validation!)")

	c.JSON(http.StatusOK, gin.H{
		"access_token": tokenString,
		"token_type":   "Bearer",
		"expires_in":   3600,
		"scope":        scope,
	})
}

// IntrospectToken handles POST /oauth2/introspect
func (h *Handler) IntrospectToken(c *gin.Context) {
	tokenString := c.PostForm("token")
	if tokenString == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	claims, err := h.tokenService.ValidateToken(tokenString)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"active": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"active":  true,
		"sub":     claims.Subject,
		"scope":   claims.Scope,
		"nfType":  claims.NFType,
		"iss":     claims.Issuer,
		"exp":     claims.ExpiresAt.Unix(),
		"iat":     claims.IssuedAt.Unix(),
		"jti":     claims.ID,
	})
}

// Health handles GET /health
func (h *Handler) Health(c *gin.Context) {
	h.registry.mu.RLock()
	count := len(h.registry.nfs)
	h.registry.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"status":      "healthy",
		"service":     "NRF",
		"registeredNFs": count,
	})
}

// extractBearerToken extracts the token from the Authorization header.
func extractBearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
