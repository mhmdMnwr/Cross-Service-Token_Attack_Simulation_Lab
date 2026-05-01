#!/usr/bin/env python3
"""
legit_traffic.py — Simulates 3 UEs registering and creating PDU sessions
through the 5G core network. Populates token stores in AMF and AUSF
before the attacker runs.

Usage:
    python3 legit_traffic.py
"""

import json
import time
import sys
import requests
from datetime import datetime

# ── Service Endpoints (host-mapped ports) ──
NRF  = "http://localhost:8000"
UDM  = "http://localhost:8001"
AUSF = "http://localhost:8002"
AMF  = "http://localhost:8003"
SMF  = "http://localhost:8004"

# ── Color helpers ──
RED    = "\033[91m"
GREEN  = "\033[92m"
YELLOW = "\033[93m"
BLUE   = "\033[94m"
CYAN   = "\033[96m"
BOLD   = "\033[1m"
RESET  = "\033[0m"

def ts():
    return datetime.now().strftime("%H:%M:%S.%f")[:-3]

def banner(msg):
    print(f"\n{BOLD}{BLUE}{'═'*60}")
    print(f"  {msg}")
    print(f"{'═'*60}{RESET}")

def flow(src, dst, action, detail=""):
    arrow = f"{GREEN}──▶{RESET}"
    print(f"  {ts()}  {CYAN}{src:>6}{RESET} {arrow} {CYAN}{dst:<6}{RESET}  {action}  {YELLOW}{detail}{RESET}")

def ok(msg):
    print(f"  {GREEN}[✓]{RESET} {msg}")

def err(msg):
    print(f"  {RED}[✗]{RESET} {msg}")


def obtain_token(nf_id, scope, nf_type):
    """Request an access token from NRF."""
    try:
        resp = requests.post(f"{NRF}/oauth2/token", data={
            "grant_type": "client_credentials",
            "nfInstanceId": nf_id,
            "scope": scope,
            "nfType": nf_type,
        }, timeout=5)
        if resp.status_code == 200:
            return resp.json().get("access_token", "")
    except Exception as e:
        err(f"Token request failed: {e}")
    return ""


def simulate_ue_registration(ue_num, supi, amf_token):
    """Simulate a UE registration through AMF."""
    banner(f"UE #{ue_num} REGISTRATION — {supi}")

    # Step 1: UE → AMF: Initial registration
    flow("UE", "AMF", "N1: Registration Request", supi)

    try:
        resp = requests.post(
            f"{AMF}/namf-comm/v1/ue-registrations",
            json={"supi": supi},
            headers={"Authorization": f"Bearer {amf_token}"},
            timeout=5
        )
        if resp.status_code in (200, 201):
            flow("AMF", "AUSF", "Nausf: UE Authentication", "(internal)")
            flow("AUSF", "UDM", "Nudm: Get Auth Vectors", "(internal)")
            flow("UDM", "AUSF", "Nudm: Auth Vectors Response", "5G-AKA")
            flow("AUSF", "AMF", "Nausf: Auth Result", "SUCCESS")
            flow("AMF", "UE", "N1: Registration Accept", supi)
            ok(f"UE {supi} registered successfully")
        else:
            err(f"Registration failed: {resp.status_code} — {resp.text}")
    except Exception as e:
        err(f"Registration request failed: {e}")

    print()


def simulate_pdu_session(ue_num, supi, smf_token):
    """Simulate a PDU session establishment through SMF."""
    flow("UE", "AMF", "N1: PDU Session Request", f"DNN=internet")
    flow("AMF", "SMF", "Nsmf: Create SM Context", supi)

    try:
        resp = requests.post(
            f"{SMF}/nsmf-pdusession/v1/sm-contexts",
            json={"supi": supi, "dnn": "internet", "sst": 1, "sd": "000001"},
            headers={"Authorization": f"Bearer {smf_token}"},
            timeout=5
        )
        if resp.status_code in (200, 201):
            session = resp.json()
            flow("SMF", "UDM", "Nudm: Get Session Data", "(internal)")
            flow("UDM", "SMF", "Nudm: Session Data Response", "")
            flow("SMF", "AMF", "Nsmf: SM Context Created", session.get("sessionId", ""))
            flow("AMF", "UE", "N1: PDU Session Accept", f"IP={session.get('ueIpAddress', '?')}")
            ok(f"PDU session established: {session.get('sessionId', '?')}")
        else:
            err(f"PDU session failed: {resp.status_code}")
    except Exception as e:
        err(f"PDU session request failed: {e}")


def wait_for_services():
    """Wait until all core NFs are healthy."""
    print(f"\n{BOLD}Waiting for 5G core services to be ready...{RESET}")
    services = {"NRF": NRF, "UDM": UDM, "AUSF": AUSF, "AMF": AMF, "SMF": SMF}

    for name, url in services.items():
        for attempt in range(30):
            try:
                resp = requests.get(f"{url}/health", timeout=2)
                if resp.status_code == 200:
                    ok(f"{name} is healthy")
                    break
            except:
                pass
            if attempt == 29:
                err(f"{name} is not responding after 60s")
                return False
            time.sleep(2)
    return True


def main():
    print(f"\n{BOLD}{CYAN}")
    print("  ╔══════════════════════════════════════════════════════════╗")
    print("  ║     5G LEGITIMATE TRAFFIC GENERATOR                     ║")
    print("  ║     Simulating UE registrations & PDU sessions          ║")
    print("  ╚══════════════════════════════════════════════════════════╝")
    print(RESET)

    if not wait_for_services():
        print(f"\n{RED}Some services are not ready. Exiting.{RESET}")
        sys.exit(1)

    # Obtain tokens for traffic simulation
    print(f"\n{BOLD}Obtaining service tokens...{RESET}")
    amf_token = obtain_token("traffic-gen-001", "namf-comm", "AMF")
    smf_token = obtain_token("traffic-gen-001", "nsmf-pdusession", "AMF")

    if not amf_token or not smf_token:
        err("Failed to obtain required tokens")
        sys.exit(1)
    ok("Tokens obtained for AMF and SMF access")

    # Simulate 3 UEs
    ues = [
        ("imsi-001010000000001", "Alice"),
        ("imsi-001010000000002", "Bob"),
        ("imsi-001010000000003", "Charlie"),
    ]

    for i, (supi, name) in enumerate(ues, 1):
        simulate_ue_registration(i, supi, amf_token)
        time.sleep(2)
        simulate_pdu_session(i, supi, smf_token)
        time.sleep(2)

    # Continue generating traffic for 30 seconds
    banner("CONTINUOUS TRAFFIC — keeping token stores warm (30s)")
    end_time = time.time() + 30
    cycle = 0
    while time.time() < end_time:
        cycle += 1
        remaining = int(end_time - time.time())
        print(f"  {ts()}  Cycle {cycle} — {remaining}s remaining", end="\r")
        time.sleep(5)

    print(f"\n\n{GREEN}{BOLD}Traffic generation complete!{RESET}")
    print(f"Token stores in AMF and AUSF should now be populated.")
    print(f"Run {CYAN}python3 capture.py{RESET} to inspect them,")
    print(f"or  {RED}make attack{RESET} to run the attack.\n")


if __name__ == "__main__":
    main()
