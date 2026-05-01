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

	sessionStore := NewSessionStore()
	handler := NewHandler(signingKey, sessionStore)

	// Register with NRF and obtain tokens
	go func() {
		registerWithNRF(nrfURI)

		// Obtain AUSF token
		ausfToken := obtainToken(nrfURI, "amf-001", "nausf-auth", "AMF")
		if ausfToken != "" {
			handler.ausfToken = ausfToken
			handler.AddCachedToken(ausfToken, "nausf-auth", "amf-001", "AMF",
				time.Now().Add(1*time.Hour))
			log.Info("✓ AUSF token obtained and cached")
		}

		// Obtain UDM token
		udmToken := obtainToken(nrfURI, "amf-001", "nudm-sdm", "AMF")
		if udmToken != "" {
			handler.udmToken = udmToken
			handler.AddCachedToken(udmToken, "nudm-sdm", "amf-001", "AMF",
				time.Now().Add(1*time.Hour))
			log.Info("✓ UDM token obtained and cached")
		}

		// Discover peer NFs
		handler.ausfEndpoint = discoverNF(nrfURI, ausfToken, "AUSF")
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
		}).Info("AMF request processed")
	})

	// UE context management — requires namf-comm scope
	comm := r.Group("/namf-comm/v1")
	comm.Use(handler.TokenAuthMiddleware("namf-comm", true))
	comm.POST("/ue-contexts/:ueId/n1-n2-messages", handler.N1N2Message)

	// UE registration (simplified, used by traffic generator)
	reg := r.Group("/namf-comm/v1")
	reg.Use(handler.TokenAuthMiddleware("namf-comm", true))
	reg.POST("/ue-registrations", handler.RegisterUE)

	// VULNERABLE debug endpoint — validates token but does NOT check scope
	debug := r.Group("/debug")
	debug.Use(handler.TokenAuthMiddleware("", false))
	debug.GET("/token-store", handler.DebugTokenStore)

	// Health (unprotected)
	r.GET("/health", handler.Health)

	log.Info("╔══════════════════════════════════════════════╗")
	log.Info("║   AMF — Access and Mobility Management      ║")
	log.Info("║   UE Registration & Context Management      ║")
	log.Info("║   Listening on :8003                        ║")
	log.Info("╚══════════════════════════════════════════════╝")

	if err := r.Run(":8003"); err != nil {
		log.Fatalf("AMF failed to start: %v", err)
	}
}

func registerWithNRF(nrfURI string) {
	profile := map[string]interface{}{
		"nfType":     "AMF",
		"host":       "amf",
		"port":       8003,
		"nfServices": []string{"namf-comm"},
	}

	body, _ := json.Marshal(profile)
	regURL := fmt.Sprintf("%s/nnrf-nfm/v1/nf-instances/amf-001", nrfURI)

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
			log.Info("✓ Successfully registered with NRF as AMF")
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
