#!/usr/bin/env python3
"""Analyze remaining untranslated values and categorize them."""
import json
from pathlib import Path

data = json.loads(Path("disposable/untranslated_values.json").read_text(encoding="utf-8"))
values = data["values"]

# Categories
USER_FACING = {}     # Should be translated - visible in UI
CONFIG_DESC = {}     # Config descriptions - nice to have but long
TECHNICAL = {}       # Technical identifiers - keep English
BRAND_NAMES = {}     # Brand/product names - keep English
GERMAN_ALREADY = {}  # Already in German (German-only values)
PLACEHOLDERS = {}    # Placeholder/example values

brand_names = {
    "adguard", "a2a", "acme", "ansible", "aurago", "aws", "backblaze",
    "bearer", "bootstrap", "brave", "caddy", "cloudflare", "daemon",
    "devcontainer", "discord", "docker", "dockerfile", "email",
    "fritzbox", "fritz!box", "gcp", "gemini", "github", "gmail",
    "google", "gotenberg", "homeassistant", "imap", "jellyfin",
    "kimicode", "koofr", "let's encrypt", "letsencrypt", "llm",
    "mac", "magicdns", "meshcentral", "minio", "mqtt", "n8n",
    "ntfy", "oauth", "ollama", "onedrive", "openai", "openrouter",
    "paperless", "paperless-ngx", "piper", "podman", "proxmox",
    "rocket.chat", "rocketchat", "s3", "sandbox", "slack", "smb",
    "smtp", "ssh", "starttls", "sudo", "supervisor", "tailscale",
    "telegram", "telnyx", "truenas", "vault", "vscode", "webdav",
    "webhook", "whisper", "wireguard", "workers", "zfs", "ntfs",
    "gmail", "sendgrid", "netlify", "brave search"
}

for val, count in sorted(values.items(), key=lambda x: -x[1]):
    v = val.strip()
    vl = v.lower()
    
    # Skip German-only values
    has_ger = any(c in v for c in "äöüßÜÖÄ")
    if has_ger:
        GERMAN_ALREADY[v] = count
        continue
    
    # Skip IP addresses, ports, technical placeholders
    import re
    if re.sub(r'[^a-zA-Z0-9]', '', v).isdigit() or re.sub(r'[^a-zA-Z0-9]', '', v) == "":
        TECHNICAL[v] = count
        continue
    
    # Skip code snippets and technical patterns
    if any(p in vl for p in ["docker build", "docker-compose", "pip install", "go build",
                              "curl ", "wget ", "chmod", "systemctl", "apt-get",
                              "sk-or-", "sk-ant-", "sk-...", "192.168.", "127.0.0",
                              "content-type", "application/json", "grpc", "http://",
                              "https://", "hooks.slack.com", "llm-sandbox",
                              "aurago_ansible", "aurago-ansible", "home/#",
                              "execute_sudo", "execute_remote", "concept",
                              "x-hub-signature", "github-push", "repository.full_name",
                              "xml detected", "json format", "raw json",
                              "function call...", "tool call...", "api format...",
                              "requesting proper", "requesting raw",
                              "requesting native", "requesting corrected"]):
        TECHNICAL[v] = count
        continue
    
    # Skip very long config descriptions (>100 chars)
    if len(v) > 120:
        CONFIG_DESC[v] = count
        continue
    
    # Skip brand names / product names (short, well-known)
    if vl in brand_names or v in brand_names:
        BRAND_NAMES[v] = count
        continue
    
    # Skip example/placeholder values
    if any(p in vl for p in ["e.g.", "example", "192.168", "my_skill", "tank/",
                              "my-site", "repo\"", "source (e.g"]):
        if len(v) < 60:
            PLACEHOLDERS[v] = count
            continue
    
    # Everything else is potentially user-facing
    USER_FACING[v] = count

import sys, io
out = io.StringIO()
def p(s=""): out.write(s + "\n")

p("=" * 70)
p(f"ANALYSIS OF {len(values)} REMAINING UNTRANSLATED VALUES")
p(f"Total occurrences: {sum(values.values())}")
p("=" * 70)

p(f"\n1. USER-FACING (should translate): {len(USER_FACING)} values, {sum(USER_FACING.values())} occurrences")
for v, c in sorted(USER_FACING.items(), key=lambda x: -x[1]):
    p(f"   [{c:3d}] {v[:120]}")

p(f"\n2. CONFIG DESCRIPTIONS (long, nice-to-have): {len(CONFIG_DESC)} values, {sum(CONFIG_DESC.values())} occurrences")
for v, c in sorted(CONFIG_DESC.items(), key=lambda x: -x[1]):
    p(f"   [{c:3d}] {v[:120]}...")

p(f"\n3. TECHNICAL (keep English): {len(TECHNICAL)} values, {sum(TECHNICAL.values())} occurrences")
for v, c in sorted(TECHNICAL.items(), key=lambda x: -x[1]):
    p(f"   [{c:3d}] {v[:120]}")

p(f"\n4. BRAND/PRODUCT NAMES (keep English): {len(BRAND_NAMES)} values, {sum(BRAND_NAMES.values())} occurrences")
for v, c in sorted(BRAND_NAMES.items(), key=lambda x: -x[1]):
    p(f"   [{c:3d}] {v[:120]}")

p(f"\n5. ALREADY GERMAN: {len(GERMAN_ALREADY)} values, {sum(GERMAN_ALREADY.values())} occurrences")
for v, c in sorted(GERMAN_ALREADY.items(), key=lambda x: -x[1]):
    p(f"   [{c:3d}] {v[:120]}")

p(f"\n6. PLACEHOLDERS/EXAMPLES: {len(PLACEHOLDERS)} values, {sum(PLACEHOLDERS.values())} occurrences")
for v, c in sorted(PLACEHOLDERS.items(), key=lambda x: -x[1]):
    p(f"   [{c:3d}] {v[:120]}")

# Save report
Path("disposable/analysis_report.txt").write_text(out.getvalue(), encoding="utf-8")

# Save categorized data
output = {"user_facing": USER_FACING, "config_descriptions": CONFIG_DESC,
          "technical": TECHNICAL, "brand_names": BRAND_NAMES,
          "german_already": GERMAN_ALREADY, "placeholders": PLACEHOLDERS}
Path("disposable/categorized_untranslated.json").write_text(
    json.dumps(output, ensure_ascii=False, indent=2), encoding="utf-8"
)

# Print summary only (ASCII-safe)
print(f"Report saved to disposable/analysis_report.txt")
print(f"Data saved to disposable/categorized_untranslated.json")
print(f"User-facing: {len(USER_FACING)} ({sum(USER_FACING.values())} occ)")
print(f"Config desc: {len(CONFIG_DESC)} ({sum(CONFIG_DESC.values())} occ)")
print(f"Technical:   {len(TECHNICAL)} ({sum(TECHNICAL.values())} occ)")
print(f"Brand names: {len(BRAND_NAMES)} ({sum(BRAND_NAMES.values())} occ)")
print(f"German alrd: {len(GERMAN_ALREADY)} ({sum(GERMAN_ALREADY.values())} occ)")
print(f"Placeholders:{len(PLACEHOLDERS)} ({sum(PLACEHOLDERS.values())} occ)")
