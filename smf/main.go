package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	handler := NewHandler(signingKey)

	// Register with NRF and obtain tokens
	go func() {
		registerWithNRF(nrfURI)

		// Obtain UDM token for subscriber data queries
		udmToken := obtainToken(nrfURI, "smf-001", "nudm-sdm", "SMF")
		if udmToken != "" {
			handler.udmToken = udmToken
			log.Info("✓ UDM token obtained")
		}

		// Discover UDM endpoint
		handler.udmEndpoint = discoverNF(nrfURI, udmToken, "UDM")
	}()

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
		}).Info("SMF request processed")
	})

	// PDU session management — requires nsmf-pdusession scope
	pdu := r.Group("/nsmf-pdusession/v1")
	pdu.Use(handler.TokenAuthMiddleware("nsmf-pdusession"))
	pdu.POST("/sm-contexts", handler.CreateSMContext)

	// Health (unprotected)
	r.GET("/health", handler.Health)

	log.Info("╔══════════════════════════════════════════════╗")
	log.Info("║   SMF — Session Management Function         ║")
	log.Info("║   PDU Session Establishment                 ║")
	log.Info("║   Listening on :8004                        ║")
	log.Info("╚══════════════════════════════════════════════╝")

	if err := r.Run(":8004"); err != nil {
		log.Fatalf("SMF failed to start: %v", err)
	}
}

func registerWithNRF(nrfURI string) {
	profile := map[string]interface{}{
		"nfType":     "SMF",
		"host":       "smf",
		"port":       8004,
		"nfServices": []string{"nsmf-pdusession"},
	}

	body, _ := json.Marshal(profile)
	regURL := fmt.Sprintf("%s/nnrf-nfm/v1/nf-instances/smf-001", nrfURI)

	for i := 0; i < 30; i++ {
		req, err := http.NewRequest("PUT", regURL, bytes.NewReader(body))
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.WithError(err).Warn("NRF not ready, retrying...")
			time.Sleep(2 * time.Second)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
			log.Info("✓ Successfully registered with NRF as SMF")
			return
		}
		time.Sleep(2 * time.Second)
	}
	log.Error("✗ Failed to register with NRF after 30 attempts")
}

func obtainToken(nrfURI, nfInstanceID, scope, nfType string) string {
	data := url.Values{
		"grant_type":   {"client_credentials"},
		"nfInstanceId": {nfInstanceID},
		"scope":        {scope},
		"nfType":       {nfType},
	}

	for i := 0; i < 10; i++ {
		resp, err := http.PostForm(fmt.Sprintf("%s/oauth2/token", nrfURI), data)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			if token, ok := result["access_token"].(string); ok {
				return token
			}
		}
		time.Sleep(2 * time.Second)
	}
	log.Error("Failed to obtain token from NRF")
	return ""
}

func discoverNF(nrfURI, token, targetNFType string) string {
	discURL := fmt.Sprintf("%s/nnrf-disc/v1/nf-instances?target-nf-type=%s", nrfURI, targetNFType)

	req, err := http.NewRequest("GET", discURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		NFInstances []struct {
			Host string `json:"host"`
			Port int    `json:"port"`
		} `json:"nfInstances"`
	}

	if err := json.Unmarshal(body, &result); err != nil || len(result.NFInstances) == 0 {
		return ""
	}

	endpoint := fmt.Sprintf("http://%s:%d", result.NFInstances[0].Host, result.NFInstances[0].Port)
	log.WithField("endpoint", endpoint).Infof("Discovered %s via NRF", targetNFType)
	return endpoint
}
