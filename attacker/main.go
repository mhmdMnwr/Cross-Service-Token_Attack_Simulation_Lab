package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Printf("\n%s%s", colorBold, colorRed)
	fmt.Println("  ╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║    5G SBA CROSS-SERVICE TOKEN FINDING ATTACK SIMULATION     ║")
	fmt.Println("  ║                                                              ║")
	fmt.Println("  ║    Rogue NF → NRF Registration → Token Theft → Data Theft   ║")
	fmt.Println("  ╚══════════════════════════════════════════════════════════════╝")
	fmt.Print(colorReset)

	nrfURI := os.Getenv("NRF_URI")
	if nrfURI == "" {
		nrfURI = "http://nrf:8000"
	}

	// Wait for services to be ready
	info("Waiting 5 seconds for all NFs to register and cache tokens...")
	time.Sleep(5 * time.Second)

	ctx := &AttackContext{
		NRFURI: nrfURI,
	}

	steps := []struct {
		name string
		fn   func(*AttackContext) error
	}{
		{"Registration", Step1_Registration},
		{"NF Discovery", Step2_NFDiscovery},
		{"Cross-Service Token Finding", Step3_CrossServiceTokenFinding},
		{"Token Reuse", Step4_TokenReuse},
		{"Lateral Movement", Step5_LateralMovement},
	}

	for _, step := range steps {
		if err := step.fn(ctx); err != nil {
			fmt.Printf("  %s[✗] Step '%s' failed: %v%s\n", colorRed, step.name, err, colorReset)
		}
		time.Sleep(1 * time.Second)
	}

	Step6_ExfiltrationSummary(ctx)
}
