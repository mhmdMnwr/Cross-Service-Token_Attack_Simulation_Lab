package main

import (
	"os"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})
	log.SetLevel(log.InfoLevel)

	signingKey := os.Getenv("JWT_SIGNING_KEY")
	if signingKey == "" {
		signingKey = "5g-sba-lab-secret-key-2024"
	}

	tokenService := NewTokenService(signingKey)
	registry := NewNFRegistry()
	handler := NewHandler(tokenService, registry)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestLogger())

	// NF Management (NFM) endpoints
	r.PUT("/nnrf-nfm/v1/nf-instances/:nfInstanceId", handler.RegisterNF)
	r.DELETE("/nnrf-nfm/v1/nf-instances/:nfInstanceId", handler.DeregisterNF)
	r.GET("/nnrf-nfm/v1/nf-instances", handler.ListNFs)

	// NF Discovery (DISC) endpoints
	r.GET("/nnrf-disc/v1/nf-instances", handler.DiscoverNFs)

	// OAuth2 endpoints
	r.POST("/oauth2/token", handler.IssueToken)
	r.POST("/oauth2/introspect", handler.IntrospectToken)

	// Health check
	r.GET("/health", handler.Health)

	log.Info("╔══════════════════════════════════════════════╗")
	log.Info("║   NRF — Network Repository Function         ║")
	log.Info("║   OAuth2 Authorization Server               ║")
	log.Info("║   Listening on :8000                        ║")
	log.Info("╚══════════════════════════════════════════════╝")

	if err := r.Run(":8000"); err != nil {
		log.Fatalf("NRF failed to start: %v", err)
	}
}
