# 5G SBA Cross-Service Token Finding Attack Lab

A self-contained research testbed for reproducing **Cross-Service Token Finding Attacks** on the 5G Service-Based Architecture (SBA). This lab simulates how a compromised or rogue Network Function (NF) can exploit OAuth 2.0 token management weaknesses to steal tokens, impersonate legitimate NFs, and exfiltrate subscriber data.

> **⚠️ EDUCATIONAL PURPOSE ONLY** — This lab is designed for security research and understanding 5G SBA vulnerabilities. Do not use these techniques against production networks.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    5G SBA Lab (Docker Network: 5g-sbi-net)      │
│                                                                 │
│   ┌─────────┐    OAuth2 Token    ┌─────────┐                   │
│   │   NRF   │◄──────────────────►│ ATTACKER│  ← Rogue NF       │
│   │ :8000   │   NF Registration  │ :8005   │                   │
│   └────┬────┘                    └────┬────┘                   │
│        │ Token Issuance               │ Token Theft            │
│   ┌────┼──────────────────────────────┼──────┐                 │
│   │    ▼              │               ▼      │                 │
│   │ ┌──────┐    ┌──────┐    ┌──────┐    ┌──────┐              │
│   │ │ AMF  │───►│ AUSF │───►│ UDM  │    │ SMF  │              │
│   │ │:8003 │    │:8002 │    │:8001 │    │:8004 │              │
│   │ └──────┘    └──────┘    └──────┘    └──────┘              │
│   │  /debug/     /debug/    Subscriber   PDU                   │
│   │  token-store token-store  Data       Sessions              │
│   └──────────────────────────────────────────────┘             │
└─────────────────────────────────────────────────────────────────┘
```

**Key vulnerability:** The NRF issues OAuth 2.0 tokens without validating whether the requesting NF type is authorized for the requested scope. Debug endpoints on AMF/AUSF expose cached tokens without scope enforcement.

**Protocol:** HTTP/1.1 (simplified from HTTP/2 for demo purposes)

**Reference specs:** 3GPP TS 33.501 (5G security), TS 29.510 (NRF services), free5GC architecture

---

## 1. Prerequisites

- **Docker** ≥ 20.10 & **Docker Compose** ≥ 2.0
- **Python 3.8+** with pip (for traffic scripts)
- **curl** (for manual testing)
- ~2 GB disk space for Docker images

```bash
# Verify prerequisites
docker --version
docker-compose --version
python3 --version
```

---

## 2. Build & Start the Lab

```bash
# Clone and enter the project
cd 5g-token-attack-lab

# Build and start all 5G core NFs (NRF, UDM, AUSF, AMF, SMF)
docker-compose up --build -d

# Or use the Makefile
make up
```

Wait ~10 seconds for all services to register with NRF. Verify health:

```bash
make health

# Or manually:
curl -s http://localhost:8000/health | python3 -m json.tool
curl -s http://localhost:8001/health | python3 -m json.tool
curl -s http://localhost:8002/health | python3 -m json.tool
curl -s http://localhost:8003/health | python3 -m json.tool
curl -s http://localhost:8004/health | python3 -m json.tool
```

Expected output for NRF:
```json
{
    "registeredNFs": 4,
    "status": "healthy",
    "service": "NRF"
}
```

---

## 3. Generate Legitimate Traffic

In a **second terminal**, generate normal UE traffic to populate token stores:

```bash
# Install Python dependencies and run traffic generator
make traffic

# Or manually:
cd traffic
pip install -r requirements.txt
python3 legit_traffic.py
```

This simulates 3 UEs (Alice, Bob, Charlie) performing:
- UE registration through AMF → AUSF → UDM
- PDU session establishment through AMF → SMF → UDM
- 30 seconds of continuous traffic to keep token stores warm

You'll see ASCII flow diagrams for each packet hop:

```
  12:00:01.234    UE ──▶ AMF    N1: Registration Request  imsi-001010000000001
  12:00:01.250   AMF ──▶ AUSF   Nausf: UE Authentication  (internal)
  12:00:01.267  AUSF ──▶ UDM    Nudm: Get Auth Vectors    (internal)
  12:00:01.280   UDM ──▶ AUSF   Nudm: Auth Vectors Resp   5G-AKA
  12:00:01.295  AUSF ──▶ AMF    Nausf: Auth Result         SUCCESS
  12:00:01.310   AMF ──▶ UE     N1: Registration Accept    imsi-001010000000001
```

---

## 4. Watch Token Stores Fill Up

In a **third terminal**, start the live token monitor:

```bash
make capture

# Or manually:
cd traffic
python3 capture.py
```

This polls AMF and AUSF `/debug/token-store` endpoints every 2 seconds and displays a live table:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ 🔍 5G SBI Token Store Monitor — Cycle 5 — 12:01:30                        │
├─────────┬──────────────────────────────────┬────────────────┬──────────────┤
│ Service │ Token (truncated)                │ Scope          │ Issued To    │
├─────────┼──────────────────────────────────┼────────────────┼──────────────┤
│ AMF     │ eyJhbGciOiJIUz...XVkbS1zZG0     │ nausf-auth     │ amf-001      │
│ AMF     │ eyJhbGciOiJIUz...bmF1c2YtYX      │ nudm-sdm       │ amf-001      │
│ AUSF    │ eyJhbGciOiJIUz...c2RtIiwibn      │ nudm-sdm       │ ausf-001     │
└─────────┴──────────────────────────────────┴────────────────┴──────────────┘
```

---

## 5. Run the Attack

Once token stores are populated, launch the rogue NF:

```bash
make attack

# Or manually:
docker-compose --profile attack run --rm attacker
```

The attacker executes 6 steps automatically with ~1 second delays between each.

---

## 6. Interpreting the Output

### Step 1: Registration
The rogue NF registers with NRF as a spoofed "AMF" and obtains a legitimate access token.

**Example decoded JWT:**
```
┌── JWT Claims ──────────────────────────────────┐
│  sub          → rogue-amf-666
│  scope        → nnrf-disc
│  nfType       → AMF
│  iss          → NRF
│  exp          → 1.7145e+09  (1 hour from now)
│  iat          → 1.7145e+09
│  jti          → rogue-amf-666-1714500000000000000
└────────────────────────────────────────────────┘
```

- **sub**: NF instance ID — the attacker chose this freely
- **scope**: What services this token grants access to
- **nfType**: The NF type the attacker claims to be (spoofed)
- **exp/iat**: Token validity window (1 hour)

### Step 2: NF Discovery
Uses the NRF token to enumerate all registered NFs and their endpoints — mapping the entire 5G core topology.

### Step 3: Cross-Service Token Finding
Probes AMF and AUSF `/debug/token-store` endpoints. These validate the token signature but **don't enforce scope**, allowing any valid token holder to read cached tokens belonging to other NFs.

### Step 4: Token Reuse
Uses a stolen UDM-scoped token (originally issued to AMF or AUSF) to query subscriber data directly. Example extracted profile:

```json
{
  "supi": "imsi-001010000000001",
  "subscriberName": "Alice Johnson",
  "msisdn": "+1-555-0101",
  "permanentKey": "465B5CE8B199B49FAA5F0A2EE238A6BC",
  "opc": "E8ED289DEBA952E4283B54E88E6183CA",
  "billingPlan": "PREMIUM_UNLIMITED",
  "locationAreaCodes": ["LAC-001", "LAC-002", "LAC-003"]
}
```

### Step 5: Lateral Movement
Uses stolen AUSF tokens to forge authentication requests and extract authentication vectors (RAND, AUTN, XRES, Kausf).

### Step 6: Summary
A structured report of everything the attacker collected.

---

## 7. What the Attacker Can Do With This

### IMSI Harvesting → Location Tracking
Extracted `locationAreaCodes` from subscriber profiles enable tracking a user's physical location across cell towers.

### Stolen Session Keys → Decrypt User Traffic
Authentication vectors contain `Kausf`, from which an attacker can derive:
- **Kseaf** → **Kamf** → **KNASenc** (NAS encryption key)
- This enables decryption of all user-plane and control-plane traffic

### Billing Fraud via SMF Session Manipulation
With SMF-scoped tokens, an attacker can:
- Create unauthorized PDU sessions for free data
- Modify existing session parameters (QoS, APN)
- Attribute data usage to other subscribers

### Persistent Access via Token Reuse
Stolen tokens remain valid until expiry (1 hour in this lab). During this window the attacker can:
- Continuously query subscriber data
- Impersonate any NF the token was issued to
- Register additional rogue NFs for redundancy

### Identity Cloning
With `permanentKey` and `OPc` values, an attacker can clone a subscriber's SIM credentials onto another device.

---

## 8. Mitigations (Defense Side)

### Token Binding to NF Instance
Bind tokens to the requesting NF's TLS certificate or IP address. Reject tokens presented from a different source than the original requester.

### mTLS on All SBI Interfaces
Require mutual TLS authentication between all NFs. Each NF presents a certificate issued by the operator's PKI, preventing rogue NFs from communicating on the SBI.

### Strict Scope Enforcement at Every NF
Every NF must validate that:
1. The token is signed by a trusted NRF
2. The token scope matches the requested service
3. The `nfType` in the token is authorized for that scope
4. Debug/management endpoints require separate admin credentials

### Short Token Expiry + Token Rotation
- Reduce token lifetime from 1 hour to 5 minutes
- Implement token rotation with refresh tokens
- Use one-time-use tokens where possible
- Monitor for token reuse from unexpected NF instances

### NRF Scope-to-NF-Type Validation
The NRF should enforce a policy matrix: only AMF can request `nausf-auth` scope, only AUSF/AMF can request `nudm-sdm` scope, etc. This prevents a rogue NF of type X from obtaining tokens for services it should never access.

### Network Segmentation
Deploy NFs in isolated network segments with firewall rules limiting which NFs can communicate with each other, matching the expected service graph.

---

## Service Ports

| Service  | Port | Description                        |
|----------|------|------------------------------------|
| NRF      | 8000 | OAuth2 + NF Registry + Discovery   |
| UDM      | 8001 | Subscriber Data Management         |
| AUSF     | 8002 | UE Authentication                  |
| AMF      | 8003 | Access & Mobility Management       |
| SMF      | 8004 | Session Management                 |
| Attacker | 8005 | Rogue NF (on-demand via profile)   |

## Makefile Targets

| Target    | Command                          |
|-----------|----------------------------------|
| `make up` | Start the 5G core network        |
| `make attack` | Run the attack simulation    |
| `make traffic` | Generate legitimate UE traffic |
| `make capture` | Watch token stores live       |
| `make logs` | View all service logs           |
| `make health` | Check health of all services  |
| `make clean` | Stop and remove everything     |

## Libraries Used (Go)

- `github.com/gin-gonic/gin` — HTTP routing
- `github.com/golang-jwt/jwt/v5` — JWT signing & validation
- `github.com/sirupsen/logrus` — Structured logging
# Cross-Service-Token_Attack_Simulation_Lab
