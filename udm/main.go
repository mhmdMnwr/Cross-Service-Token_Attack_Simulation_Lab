package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})

	signingKey := os.Getenv("JWT_SIGNING_KEY")
	if signingKey == "" {
		signingKey = "5g-sba-lab-secret-key-2024"
	}

	nrfURI := os.Getenv("NRF_URI")
	if nrfURI == "" {
		nrfURI = "http://nrf:8000"
	}

	db := NewSubscriberDB()
	handler := NewHandler(db, signingKey)

	// Register with NRF
	go registerWithNRF(nrfURI)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Request logging
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.WithFields(log.Fields{
			"method":  c.Request.Method,
			"path":    c.Request.URL.Path,
			"status":  c.Writer.Status(),
			"latency": time.Since(start).String(),
		}).Info("UDM request processed")
	})

	// Protected endpoints — require nudm-sdm scope
	sdm := r.Group("/nudm-sdm/v1")
	sdm.Use(handler.TokenAuthMiddleware("nudm-sdm"))
	sdm.GET("/:supi/am-data", handler.GetAMData)

	// Health (unprotected)
	r.GET("/health", handler.Health)

	log.Info("╔══════════════════════════════════════════════╗")
	log.Info("║   UDM — Unified Data Management             ║")
	log.Info("║   Subscriber Data Store                     ║")
	log.Info("║   Listening on :8001                        ║")
	log.Info("╚══════════════════════════════════════════════╝")

	if err := r.Run(":8001"); err != nil {
		log.Fatalf("UDM failed to start: %v", err)
	}
}

func registerWithNRF(nrfURI string) {
	profile := map[string]interface{}{
		"nfType":     "UDM",
		"host":       "udm",
		"port":       8001,
		"nfServices": []string{"nudm-sdm"},
	}

	body, _ := json.Marshal(profile)
	url := fmt.Sprintf("%s/nnrf-nfm/v1/nf-instances/udm-001", nrfURI)

	for i := 0; i < 30; i++ {
		resp, err := http.NewRequest("PUT", url, bytes.NewReader(body))
		if err != nil {
			log.WithError(err).Warn("Failed to create NRF registration request")
			time.Sleep(2 * time.Second)
			continue
		}
		resp.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		r, err := client.Do(resp)
		if err != nil {
			log.WithError(err).Warn("NRF not ready, retrying...")
			time.Sleep(2 * time.Second)
			continue
		}
		r.Body.Close()

		if r.StatusCode == http.StatusCreated || r.StatusCode == http.StatusOK {
			log.Info("✓ Successfully registered with NRF as UDM")
			return
		}

		log.WithField("status", r.StatusCode).Warn("NRF registration returned unexpected status")
		time.Sleep(2 * time.Second)
	}

	log.Error("✗ Failed to register with NRF after 30 attempts")
}
