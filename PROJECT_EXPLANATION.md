# 5G SBA Cross-Service Token Attack — Complete Project Explanation

---

## 1. Who Discovered This Attack?

The attack was discovered by a joint research team from **Northeastern University (USA)** and the **University of Padova (Italy)**:

| Researcher | Affiliation | Role |
|---|---|---|
| **Anqi Chen*** | Northeastern University | Co-first author |
| **Riccardo Preatoni*** | University of Padova | Co-first author |
| **Alessandro Brighente** | University of Padova | Researcher |
| **Mauro Conti** | University of Padua & Örebro University | Senior researcher |
| **Cristina Nita-Rotaru** | Northeastern University | Senior researcher |

> *\* = Equal contribution (co-first authors)*

The paper was published on **arXiv (arXiv:2509.08992v1)** on **September 10, 2025**, in the Computer Science — Cryptography and Security category [cs.CR].

---

## 2. Why Were They Looking For This?

### The Problem They Identified

5G networks made a revolutionary change: instead of a monolithic core network (like 4G), 5G uses a **Service-Based Architecture (SBA)** where the core network is broken into small independent microservices called **Network Functions (NFs)**. These NFs communicate via HTTP-based REST APIs called **Service-Based Interfaces (SBIs)**.

Since NFs are deployed in **cloud infrastructure** (AWS, Azure, private clouds), they face cloud-specific security threats:
- **Container escape attacks** (CVE-2024-21626, CVE-2019-5736)
- **Privilege escalation** in Kubernetes/Docker environments
- **Insider threats** from cloud operators
- **Lateral movement** between compromised containers

The researchers noticed that **very few works** had studied attacks directly against the 5G core — most previous research focused on attacks from user devices (UEs) or the radio access network (RAN). They wanted to study what happens when **an attacker compromises an NF from inside the cloud** and tries to abuse the access control system.

### Why free5GC?

They chose **free5GC v4.0.0** because:
- It is the **most mature** open-source 5G Core implementation
- It is the **only one** that implements OAuth 2.0 access control
- It conforms to **3GPP Release 17** specifications
- It is hosted by the **Linux Foundation** and widely adopted in research
- Open5GS and OAI (the other open-source options) do **not** implement OAuth access control at all

---

## 3. How Did They Find the Vulnerability?

### Manual Discovery (First)

The researchers first found the Cross-Service Token Attack **manually** by inspecting the source code and testing the OAuth flow:

1. They set up a full free5GC deployment using Docker Compose
2. They obtained a valid access token from the NRF with scope `nudr-dr` (for UDR data access)
3. They then tried using that **same token** to access a completely **different service** — the NRF's `nnrf-disc` (discovery) service
4. **It worked!** The request was accepted despite the scope mismatch

### Root Cause — The Bug in free5GC Code

They traced the bug to the file `openapi/oauth/oauth.go` in free5GC. Here is the buggy code:

```go
func VerifyOAuth(authorization, serviceName, certPath string) error {
    ...
    token, err := jwt.ParseWithClaims(...)
    if err != nil {
        return errors.Wrapf(err, "verify OAuth parse")
    }
    if !verifyScope(token.Claims.(*models.AccessTokenClaims).Scope, serviceName) {
        return errors.Wrapf(err, "verify OAuth scope")  // BUG IS HERE
    }
    return nil
}
```

**The bug:** When `verifyScope()` returns `false` (meaning the scope does NOT match), the code does `return errors.Wrapf(err, "verify OAuth scope")`. But at this point, `err` is still `nil` from the successful `jwt.ParseWithClaims()` call. So `errors.Wrapf(nil, "verify OAuth scope")` returns `nil` — which means **"no error, access granted!"**

The fix should have been:
```go
if !verifyScope(...) {
    return fmt.Errorf("verify OAuth scope: scope mismatch")  // Use a NEW error
}
```

This is a classic Go programming mistake — **reusing an error variable** instead of creating a new one.

### Automated Discovery (Second — FivGeeFuzz)

After finding the bug manually, the researchers built **FivGeeFuzz**, a grammar-based fuzzing tool that:
- Takes 48 OpenAPI specifications from 3GPP Release 17
- Uses Microsoft's **RESTler** fuzzer to automatically generate test sequences
- Tests all 10 NFs in free5GC systematically
- Found the Cross-Service Token Attack **again** automatically, plus **additional bugs**

---

## 4. How Did They Test It?

### Test Environment
- **Server:** 32 virtual CPUs, 64 GB RAM, Ubuntu 22.04 LTS, x86_64
- **Target:** free5GC v4.0.0, deployed via Docker Compose
- **NFs tested:** AMF, AUSF, CHF, NEF, NRF, NSSF, PCF, SMF, UDM, UDR (10 NFs)
- **Tool:** RESTler v9.3.1 + custom preprocessing pipeline
- **Specifications:** 48 OpenAPI YAML files from 3GPP TS 29.x (Release 17)

### What They Did
1. Deployed the full free5GC stack in Docker
2. Connected RESTler to the free5GC Docker network
3. Pre-generated a valid access token for authentication
4. Ran fuzzing campaigns against all 48 API specifications across 10 NFs
5. Manually validated and reproduced every bug found

### Results Found

| Category | Count | Description |
|---|---|---|
| Cross-Service Token Attack | 1 | Scope validation bypass (affects ALL NFs) |
| Runtime Panics (500 errors) | Multiple | UDM, AUSF, and other NFs crash on malformed input |
| Total vulnerabilities | Multiple | Confirmed in free5GC v4.0.0 AND v4.0.1 |

---

## 5. Why Is This Attack Important?

### Severity: **CRITICAL**

The Cross-Service Token Attack is extremely severe because:

1. **Affects ALL NFs** — Every single NF in free5GC uses the same buggy `VerifyOAuth()` function from the shared `openapi` library
2. **Complete access control bypass** — An attacker with ANY valid token can access ANY service
3. **Real-world deployment risk** — free5GC is used by real operators:
   - **Fujitsu** uses free5GC for 5G core R&D in Japan
   - **o2 Telefónica** references it for network deployment
   - **Platform9** runs free5GC on production Kubernetes
   - **Boost Mobile** uses open-source 5G core technology
4. **Cascading impact** — One compromised NF can steal all subscriber data, authentication keys, session information, and billing records

### What an Attacker Can Do
- **Harvest IMSIs** — Get the identity of every subscriber on the network
- **Track locations** — Use Location Area Codes to follow people physically
- **Decrypt traffic** — Steal authentication vectors (Kausf) to derive encryption keys
- **Clone SIMs** — Extract permanent keys (K) and OPc values to clone subscriber identities
- **Billing fraud** — Create unauthorized PDU sessions, redirect charges
- **Deny service** — Crash NFs using the runtime panic vulnerabilities

---

## 6. Does This Affect Real Devices / Real Networks?

### Yes, potentially

The vulnerability exists in **free5GC**, which is deployed in real-world scenarios:

| Deployment | Status |
|---|---|
| Research labs worldwide | ✅ Directly affected |
| Fujitsu's 5G development (Japan) | ⚠️ Potentially affected if using free5GC OAuth |
| Platform9 managed Kubernetes | ⚠️ Potentially affected |
| Any operator using free5GC-based core | ⚠️ Potentially affected |
| Open5GS / OAI deployments | ❌ Not affected (no OAuth implemented) |
| Commercial 5G cores (Nokia, Ericsson, etc.) | ❓ Unknown — proprietary code, may have similar bugs |

**Important nuances:**
- The bug is in the **software implementation**, not in the 3GPP **standard** itself
- The 3GPP standard correctly specifies scope validation — free5GC just implemented it wrong
- Commercial 5G core implementations are proprietary, so we can't verify if they have similar bugs
- TLS is optional in 5G SBI (per spec), making the attack surface larger in some deployments

---

## 7. The Attack Explained (Step by Step)

### Normal (Legitimate) Flow

```
AMF ──── "I need UDM access" ────► NRF
         scope=nudm-sdm

NRF ──── Token{scope=nudm-sdm} ──► AMF
         (signed JWT)

AMF ──── Token{scope=nudm-sdm} ──► UDM
         GET /nudm-sdm/v1/{supi}/am-data

UDM ──── Verifies scope matches ──► ✅ Access granted
         nudm-sdm == nudm-sdm
```

### Attack Flow (Cross-Service Token Attack)

```
ATTACKER ── "I need disc access" ──► NRF
             scope=nnrf-disc

NRF ──────── Token{scope=nnrf-disc} ──► ATTACKER
             (legitimate signed token)

ATTACKER ── Token{scope=nnrf-disc} ──► UDM
             GET /nudm-sdm/v1/{supi}/am-data
             (scope MISMATCH!)

UDM ──────── verifyScope() returns false
             BUT err is nil from parsing ──► ✅ Access GRANTED!
             (Bug: returns nil instead of error)

ATTACKER ◄── Gets subscriber data ──── UDM
             (permanent keys, IMSI, billing info)
```

### Why the Bug Makes This Work

The key insight is:
1. Token **signature** is valid (NRF really did sign it) ✅
2. Token **scope** does NOT match the requested service ❌
3. But the scope check **result is silently ignored** due to the Go error handling bug
4. So the request is treated as fully authorized

---

## 8. How Our Simulation Works

### Architecture

Our lab recreates the 5G SBA with 6 Docker containers on a shared network:

```
┌─────────────────────── Docker Network: 5g-sbi-net ───────────────────────┐
│                                                                          │
│  ┌──────────┐                                                            │
│  │   NRF    │ ← OAuth2 server, NF registry, service discovery           │
│  │  :8000   │                                                            │
│  └────┬─────┘                                                            │
│       │ Issues JWT tokens                                                │
│       │                                                                  │
│  ┌────┴─────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                │
│  │   AMF    │  │   AUSF   │  │   UDM    │  │   SMF    │                │
│  │  :8003   │  │  :8002   │  │  :8001   │  │  :8004   │                │
│  │ /debug/  │  │ /debug/  │  │ Subscriber│  │  PDU     │                │
│  │ token-   │  │ token-   │  │ Database │  │ Sessions │                │
│  │ store    │  │ store    │  │          │  │          │                │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘                │
│                                                                          │
│  ┌──────────────┐                                                        │
│  │  ATTACKER    │ ← Rogue NF that executes the 6-step attack            │
│  │  :8005       │                                                        │
│  └──────────────┘                                                        │
└──────────────────────────────────────────────────────────────────────────┘
```

### How Each Service Works

**NRF (Network Repository Function) — Port 8000**
- Acts as the OAuth 2.0 authorization server
- Issues JWT tokens with HS256 signing
- Maintains an in-memory registry of all registered NFs
- Provides service discovery (find NF by type)
- **Vulnerability simulated:** Does NOT validate if the requesting NF type should be allowed the requested scope

**UDM (Unified Data Management) — Port 8001**
- Stores subscriber profiles (5 fake subscribers with real-looking data)
- Each profile includes: IMSI, name, phone number, permanent key, OPc, billing plan, location codes
- Properly enforces scope checks (requires `nudm-sdm` scope)
- Registers with NRF on startup

**AUSF (Authentication Server Function) — Port 8002**
- Handles UE authentication requests
- Calls UDM internally to get authentication vectors
- **Caches its UDM token in memory** (attack surface!)
- Has a `/debug/token-store` endpoint that validates tokens but does NOT check scope

**AMF (Access and Mobility Management) — Port 8003**
- Manages UE registrations and context
- Obtains tokens for both AUSF and UDM access
- **Caches multiple tokens in memory** (bigger attack surface!)
- Has a `/debug/token-store` endpoint with the same weak scope enforcement

**SMF (Session Management Function) — Port 8004**
- Handles PDU session establishment
- Obtains UDM tokens for subscriber data queries
- Properly enforces scope checks

**ATTACKER (Rogue NF) — Port 8005**
- Runs the complete 6-step attack automatically
- Registers as a fake "AMF" with the NRF
- Steals tokens from AMF/AUSF debug endpoints
- Uses stolen tokens to extract subscriber data
- Prints a structured report of everything stolen

### The 6 Attack Steps in Our Simulation

| Step | Action | What It Demonstrates |
|---|---|---|
| 1. Registration | Attacker registers as "AMF" with NRF | NRF accepts any NF registration without strong validation |
| 2. NF Discovery | Queries NRF for all NF endpoints | Any registered NF can map the entire network topology |
| 3. Token Finding | Probes AMF/AUSF `/debug/token-store` | Weak scope enforcement lets any token access debug endpoints |
| 4. Token Reuse | Uses stolen UDM token to query subscriber data | Stolen tokens work on services they weren't intended for |
| 5. Lateral Movement | Uses stolen AUSF token to get auth vectors | Demonstrates cascading access across multiple services |
| 6. Exfiltration | Prints all stolen data | Shows the full impact of the attack |

---

## 9. How We Built This Simulation

### Technology Stack

| Component | Technology | Why |
|---|---|---|
| Services | **Go** | Same language as free5GC, realistic simulation |
| HTTP routing | **gin-gonic/gin** | Fast, widely-used Go web framework |
| JWT tokens | **golang-jwt/jwt/v5** | Industry-standard JWT library |
| Logging | **sirupsen/logrus** | Structured logging with fields |
| Containers | **Docker + Docker Compose** | Isolate each NF, simulate network |
| Traffic scripts | **Python** (requests + rich) | Easy to write, colorful output |

### Design Decisions

1. **HTTP/1.1 instead of HTTP/2** — Simplified for demo purposes (real 5G SBI uses HTTP/2, but the vulnerability is protocol-independent)

2. **HS256 instead of RS256 for JWT** — Simpler setup (shared secret instead of key pairs), but the attack works identically with either

3. **In-memory data stores** — No database needed, everything resets on restart, keeps the lab simple

4. **Intentionally weak `/debug/token-store`** — This simulates the real bug where scope checks fail silently. In the real free5GC, the scope check failure is the bug; in our simulation, we simply skip the scope check on debug endpoints

5. **NRF doesn't validate NF-type-to-scope mapping** — This mirrors a real architectural weakness: should a rogue "AMF" be allowed to request `nudm-sdm` scope?

### File Structure Explained

```
5g-token-attack-lab/
├── docker-compose.yml     ← Orchestrates all 6 services + network
├── Makefile               ← Quick commands (make up, make attack, etc.)
├── README.md              ← Step-by-step exploitation guide
│
├── nrf/                   ← OAuth2 + NF Registry
│   ├── main.go            ← Server setup, routes, startup
│   ├── handlers.go        ← HTTP handlers (register, discover, issue token)
│   ├── token.go           ← JWT signing/validation logic
│   ├── Dockerfile          
│   └── go.mod             
│
├── udm/                   ← Subscriber Data Store
│   ├── main.go            ← Server setup, NRF registration
│   ├── handlers.go        ← Token validation, subscriber data API
│   ├── subscriber_db.go   ← 5 fake subscriber profiles
│   ├── Dockerfile
│   └── go.mod
│
├── ausf/                  ← Authentication Server
│   ├── main.go            ← Server setup, token caching, NRF registration
│   ├── handlers.go        ← UE auth, debug token store (VULNERABLE)
│   ├── Dockerfile
│   └── go.mod
│
├── amf/                   ← Access & Mobility Management
│   ├── main.go            ← Server setup, multi-token caching
│   ├── handlers.go        ← UE context, debug token store (VULNERABLE)
│   ├── session_store.go   ← UE session management
│   ├── Dockerfile
│   └── go.mod
│
├── smf/                   ← Session Management
│   ├── main.go            ← Server setup, PDU session handling
│   ├── handlers.go        ← Session creation, token validation
│   ├── Dockerfile
│   └── go.mod
│
├── attacker/              ← Rogue NF
│   ├── main.go            ← Attack orchestrator (runs 6 steps)
│   ├── steps.go           ← Each attack step as a named function
│   ├── Dockerfile
│   └── go.mod
│
└── traffic/               ← Python helper scripts
    ├── legit_traffic.py   ← Simulates 3 UEs registering + PDU sessions
    ├── capture.py         ← Live token store monitor (rich tables)
    └── requirements.txt
```

---

## 10. How to Present This as a Project Presentation

### Suggested Slide Structure (20-25 minutes)

**Slide 1: Title**
- "Cross-Service Token Finding Attacks in 5G Core Networks"
- Your names, course, date

**Slide 2: What is 5G SBA? (2 min)**
- 5G core = microservices (NFs) communicating via REST APIs
- Draw the architecture: UE → RAN → AMF → AUSF → UDM → NRF
- Emphasize: deployed on cloud infrastructure (Docker/Kubernetes)

**Slide 3: Access Control in 5G — OAuth 2.0 (2 min)**
- NFs authenticate using JWT tokens from NRF
- Token contains: scope (what service you can access), nfType, expiry
- Show a decoded JWT example

**Slide 4: The Research Team & Motivation (1 min)**
- Northeastern University + University of Padova
- Most previous attacks targeted UE/RAN, very few studied the core itself
- free5GC = only open-source 5G core with OAuth 2.0

**Slide 5: The Vulnerability — The Bug (3 min)**
- Show the buggy Go code from free5GC
- Explain: `err` stays `nil` when scope check fails
- Impact: ANY token works on ANY service

**Slide 6: Attack Diagram (2 min)**
- Show the normal flow vs. attack flow
- Color-code: green = legitimate, red = attack
- Highlight the scope mismatch that should be rejected but isn't

**Slide 7: What Can the Attacker Steal? (2 min)**
- Subscriber identities (IMSI), phone numbers
- Permanent encryption keys (K, OPc)
- Authentication vectors
- Location data, billing information

**Slide 8: Our Lab — Architecture (2 min)**
- Show the Docker network diagram with 6 services
- Explain each NF's role briefly
- Technologies used: Go, Docker, JWT

**Slide 9: Live Demo — Starting the Lab (3 min)**
- `make up` → show services starting
- `make health` → show all healthy
- `python3 traffic/legit_traffic.py` → show normal traffic flow

**Slide 10: Live Demo — Running the Attack (5 min)**
- `make attack` → show each step executing
- Step 1: Registration (show the JWT)
- Step 3: Token stealing (highlight the vulnerability)
- Step 4: Data exfiltration (show subscriber profiles)
- Step 6: Summary report

**Slide 11: Real-World Impact (2 min)**
- Affects free5GC which is used by Fujitsu, Platform9, researchers worldwide
- Confirmed in v4.0.0 AND v4.0.1
- TLS is optional in 5G spec, making this worse
- Could affect commercial networks with similar bugs

**Slide 12: Defenses & Mitigations (2 min)**
- Fix the scope validation bug (use a new error variable)
- Enforce mTLS on all SBI interfaces
- Bind tokens to NF instance certificates
- Short token expiry + rotation
- NRF should validate NF-type-to-scope authorization

**Slide 13: Conclusion & Q&A (1 min)**
- 5G SBA opens new attack surfaces
- A single Go error handling bug compromises the entire access control
- Our lab demonstrates the full attack chain reproducibly
- Questions?

### Tips for the Presentation

1. **Start with impact** — "What if someone could read every text message and track every phone on a 5G network?" hooks the audience

2. **Show the bug first, then the fix** — Make the audience see how a 2-line mistake breaks everything

3. **Demo is king** — The colorized terminal output from the attacker is visually impressive. Run it live if possible

4. **Have a backup video** — Record `make attack` output as a backup in case Docker doesn't cooperate during the presentation

5. **Know the Go code** — Be ready to explain what gin, JWT, and logrus do if asked

6. **Anticipate questions:**
   - "Is this fixed?" → The paper was published in Sept 2025, free5GC may have patched it
   - "Does this work on real 5G?" → It works on free5GC deployments; commercial cores are proprietary
   - "Why Go?" → free5GC is written in Go, so we match their stack
   - "Could TLS prevent this?" → TLS protects transport, not the scope validation logic

---

## Quick Reference — Key Commands

```bash
# Start the lab
make up

# Check all services are healthy  
make health

# Generate legitimate traffic (populates token stores)
make traffic

# Watch token stores in real-time
make capture

# Run the attack
make attack

# View service logs
make logs

# Clean up everything
make clean
```
