#!/usr/bin/env python3
"""
capture.py — Polls /debug/token-store on AMF and AUSF every 2 seconds
and displays a live table of all cached tokens using the rich library.

Usage:
    python3 capture.py
"""

import json
import time
import sys
import base64
import requests
from datetime import datetime

try:
    from rich.console import Console
    from rich.table import Table
    from rich.live import Live
    from rich.panel import Panel
    from rich.text import Text
except ImportError:
    print("Install rich: pip install rich")
    sys.exit(1)

NRF  = "http://localhost:8000"
AMF  = "http://localhost:8003"
AUSF = "http://localhost:8002"

console = Console()


def obtain_monitor_token():
    """Get a token from NRF for monitoring."""
    try:
        resp = requests.post(f"{NRF}/oauth2/token", data={
            "grant_type": "client_credentials",
            "nfInstanceId": "monitor-001",
            "scope": "monitoring",
            "nfType": "SCP",
        }, timeout=5)
        if resp.status_code == 200:
            return resp.json().get("access_token", "")
    except:
        pass
    return ""


def decode_jwt_payload(token):
    """Decode the payload of a JWT without verification."""
    try:
        parts = token.split(".")
        if len(parts) != 3:
            return {}
        padding = 4 - len(parts[1]) % 4
        payload = base64.urlsafe_b64decode(parts[1] + "=" * padding)
        return json.loads(payload)
    except:
        return {}


def fetch_token_store(url, token):
    """Fetch the token store from an NF's debug endpoint."""
    try:
        resp = requests.get(
            f"{url}/debug/token-store",
            headers={"Authorization": f"Bearer {token}"},
            timeout=3
        )
        if resp.status_code == 200:
            return resp.json()
    except:
        pass
    return None


def build_table(amf_data, ausf_data, cycle):
    """Build a rich table with all cached tokens."""
    table = Table(
        title=f"🔍 5G SBI Token Store Monitor — Cycle {cycle} — {datetime.now().strftime('%H:%M:%S')}",
        show_lines=True,
        title_style="bold cyan",
    )

    table.add_column("Service", style="bold yellow", width=8)
    table.add_column("Token (truncated)", style="dim", width=40)
    table.add_column("Scope", style="bold green", width=18)
    table.add_column("Issued To", style="cyan", width=14)
    table.add_column("NF Type", style="magenta", width=8)
    table.add_column("Expires", style="red", width=22)

    sources = [("AMF", amf_data), ("AUSF", ausf_data)]

    for service, data in sources:
        if data is None:
            table.add_row(service, "[dim]not responding[/dim]", "-", "-", "-", "-")
            continue

        tokens = data.get("cachedTokens", [])
        if not tokens:
            table.add_row(service, "[dim]no cached tokens[/dim]", "-", "-", "-", "-")
            continue

        for ct in tokens:
            raw = ct.get("token", "")
            truncated = raw[:20] + "..." + raw[-15:] if len(raw) > 38 else raw
            table.add_row(
                service,
                truncated,
                ct.get("scope", "?"),
                ct.get("issuedTo", "?"),
                ct.get("nfType", "?"),
                ct.get("expiresAt", "?")[:22],
            )

    return table


def main():
    console.print(Panel(
        "[bold red]5G SBI Token Store Monitor[/bold red]\n"
        "[dim]Polling AMF and AUSF /debug/token-store every 2 seconds[/dim]\n"
        "[dim]Press Ctrl+C to stop[/dim]",
        title="capture.py",
        border_style="cyan",
    ))

    # Get a monitoring token
    console.print("[yellow]Obtaining monitoring token from NRF...[/yellow]")
    token = obtain_monitor_token()
    if not token:
        console.print("[red]Failed to obtain token from NRF. Is the lab running?[/red]")
        sys.exit(1)
    console.print("[green]✓ Monitoring token obtained[/green]\n")

    cycle = 0
    try:
        with Live(console=console, refresh_per_second=1) as live:
            while True:
                cycle += 1
                amf_data = fetch_token_store(AMF, token)
                ausf_data = fetch_token_store(AUSF, token)
                table = build_table(amf_data, ausf_data, cycle)
                live.update(table)
                time.sleep(2)
    except KeyboardInterrupt:
        console.print("\n[yellow]Monitoring stopped.[/yellow]")


if __name__ == "__main__":
    main()
