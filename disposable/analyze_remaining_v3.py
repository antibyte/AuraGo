#!/usr/bin/env python3
"""Analyze remaining untranslated values and categorize them - v3 with expanded blacklist."""
import json
import re
import io
from pathlib import Path

with open("disposable/untranslated_values.json", "r", encoding="utf-8") as f:
    data = json.load(f)

values = data["values"]

# Brand names and product names - DO NOT TRANSLATE
BRAND_NAMES = {
    # Original brands
    "AdGuard", "AdGuard Home", "AuraGo", "Brave Search", "Cloudflare Tunnel",
    "Docker", "Docker API", "Docker Host", "Docker Image", "Docker (Local)", "Docker (Remote)",
    "ElevenLabs", "Fritz!Box", "FritzBox", "Gotenberg", "Home Assistant",
    "Jellyfin", "Kimi Code", "Koofr", "MeshCentral", "Ollama", "Ollama (local)",
    "OneDrive", "Paperless-ngx", "Proxmox", "Rocket.Chat", "Tailscale",
    "Tailscale Funnel", "Telegram", "TrueNAS", "WebDAV", "Webhook", "Webhooks",
    "WireGuard", "Piper TTS", "Whisper API (OpenAI)", "Google (Gemini)",
    "Google Workspace", "GitHub Repositories", "Gotenberg (Docker sidecar)",
    "Microsoft OneDrive cloud storage integration", "Koofr cloud storage",
    "WebDAV cloud sync", "Rocket.Chat bot integration", "Telegram bot integration",
    "Discord bot integration", "Smart home integration", "n8n workflow automation integration",
    "Alibaba Coding Plan", "MiniMax Coding Plan", "Z AI GLM Coding Plan",
    "Dev Container", "VS Code debug bridge", "Time Machine",
    "GZIP", "Basic Auth", "Bearer Token", "Bearer Token (Vault)", "Bearer token",
    "HMAC Secret", "Personal Access Token", "Connector Token",
    "Python", "Shell", "Ansible", "Bootstrap", "Coder", "Coding",
    "MCP Server", "Agent", "Helper LLM", "LLM Guardian", "AI Gateway",
    "Invasion Control", "Background", "Daemon", "Daemons",
    "SSE Clients", "Circuit Breaker", "Security Proxy", "SSH Tunnel",
    "Remote Shell (execute_remote_shell)", "Login Guard",
    "A2A Server", "ACME Email", "ACME Email (Let's Encrypt)",
    "AuraGo Interface", "AuraGo – Configuration", "AuraGo – Dashboard",
    "AuraGo – Mission Control", "AuraGo - Invasion Control", "AuraGo - Mission Control",
    "Dansk", "Deutsch", "English", "Español", "Français", "Italiano",
    "Nederlands", "Norsk", "Polski", "Português", "Svenska", "Čeština",
    "Ελληνικά", "हिन्दी", "中文", "日本語",
    "AuraGo API Token for n8n", "Paperless-ngx URL",
    "AdGuard Home URL", "Cloudflare Account ID", "Egg Port",
    "Fritz!Box:", "Gotenberg URL", "Host / IP",
    "Jellyfin Host", "Ollama URL", "SMB:", "TR-064 Port",
    "TrueNAS Host", "WebDAV URL", "Cloudflare Tunnel",
    "Docker Host", "Docker Image", "Egg Port",
    "IMAP Host", "IMAP Port", "SMTP Host", "SMTP Port",
    "OAuth2 Client ID", "OAuth2 Client Secret", "SSH Private Key",
    "HTTP Port", "HTTPS Port", "GPU Backend",
    "Quality of Service (QoS)", "Text-to-Speech", "Whisper speech-to-text",
    "Time Machine support (Mac)", "Native Function Calling ⚠️",
    # New additions - technical acronyms and brand names
    "OpenRouter", "OpenAI", "Anthropic", "Chromecast", "Discord", "GitHub",
    "Netlify", "VirusTotal", "n8n", "MQTT", "TTS", "STT",
    "A2A", "ZLE", "PLANS", "INVASION", "MISSIONS", "DASHBOARD",
    "SSH", "IP", "MAC", "CPU", "RAM", "Web", "OK", "No",
    "Model", "Models", "Provider", "Type", "Tags", "Name",
    "Description", "Host", "Port", "Output", "Input", "Context",
    "Auth", "Sandbox", "Offline", "Online", "Status", "Total",
    "Prompt", "Plan", "Tunnel", "Feedback", "Error", "Budget",
    "Journal", "Manual", "Pools", "Tokens", "Permanent", "Disk",
    "Highlights", "Embeddings", "Neutral", "Media", "Vision", "Test",
    "Folder", "Chat", "Containers", "Dashboard", "Missions", "Plans",
    "Invasion", "Fail-Safe", "Password", "Nest", "Tools {{count}}",
    "Net ↓", "Net ↑", "{{n}} Repositories", "Fallbacks", "Parse {{count}}",
    "Policy {{count}}", "Custom", "Mobile", "Actions", "Topic:",
    "❌ error", "run", "Name *", "Name (optional)",
    " deploy", "Deployment", "Off", "ago", "30d", "7d", "1h", "24h", "6h",
    "Multimodal (Gemini / LLM)", "Fallback {{pct}}%", "Webhook:",
    "Firewall", "Error:", "${{cost}} / ${{limit}}",
}

ALREADY_GERMAN = {
    "(Co-Agents + Vision/STT gesperrt bei Überschreitung)",
    "(alles gesperrt bei Überschreitung)",
    "(nur Warnung)",
}

TECHNICAL = {
    "192.168.1.100", "192.168.1.50", "application/json",
    "requests, pyyaml", "root", "my_skill", "debug",
    "images", "running", "runs", "used", "tokens",
    "commit(s)", "images",
    "XML detected in native mode, requesting function call...",
    "ping failed: {0} — ensure the process has permission to send ICMP packets (root / CAP_NET_RAW on Linux)",
    "admin password is required", "admin password must be a string",
    "admin password must be at least 8 characters long",
    "auth.enabled must be a boolean",
    "UnicodeEncodeError",
}

PLACEHOLDERS = {
    "https://hooks.slack.com/services/...",
    "(free only)",
}

# Short UI labels that are definitely user-facing (these SHOULD be translated)
SHORT_UI_LABELS = {
    "ACTIVE", "Agent Status", "Budget", "Budget & Tokens", "Burst",
    "Cache Rate", "Cache hits", "Collection", "Collections",
    "Comma-separated values", "Command", "Communication",
    "Compression", "Database", "Datasets", "Date", "Designer",
    "Details", "Direct", "Disable", "Download", "Download PNG", "Download SVG",
    "E-Mail", "Edges", "Email", "Enabled", "Feedback",
    "Folder", "Full", "Graph Edges", "Graph Nodes",
    "Headers", "High", "Hostname", "Idle", "Images",
    "Infrastructure", "Inbox", "Indexer", "Interval", "KG Explorer",
    "Limits", "Locked", "Log only", "Logging", "Logs", "Login",
    "Master Key", "Max Tools", "Medium", "Memory", "Memory Health",
    "Message", "Messages", "Mode", "Navigation", "Never",
    "Neutral", "Nodes", "Notes", "Notification", "Notifications",
    "Open", "Optional", "Parameters", "Performance", "Personality V2",
    "Permissions", "Permissions (Scopes)", "Preset", "Profiling",
    "Providers", "Public", "Query Models", "RAG Batch",
    "Rate Limiting", "Recoveries", "Researcher", "Retrieval {{count}}",
    "Rollback", "Run now", "Scopes", "Secrets", "Session",
    "Shed Rate", "Skill Code", "Skill Details", "Skills", "Speech",
    "Stable", "Start", "Start Scrub", "Start Tunnel", "Stop",
    "Streaming (SSE)", "Tasks", "Time", "Topics", "Tracked",
    "Transfer Vault", "Types", "Update", "Update Available",
    "Updates", "Upload", "Upload Skill", "Uptime", "User",
    "Valence", "Vault", "Vault Keys", "Version", "View Log",
    "Watch Folder", "Watch enabled", "Web Config & Login", "Web Scraper",
    "d ago", "h ago", "just now", "m ago", "optional", "user, agent", "x executed",
}

# Emoji-prefixed UI labels
EMOJI_UI = {
    "↯ Reset in {{hours}}h {{minutes}}m", "↻ Restart", "⏳ Checking…",
    "⏹ Stop", "▶ Start", "⚠️ Danger Zone", "⚡ Trigger",
    "⬆️ Install Update", "⬇ Download", "🆓 This model is free",
    "🌅 **System Briefing:**", "🌐 Browse", "🎤 Speech-to-Text (Whisper)",
    "🎨 Optional", "🎯 Mission Control", "🎵 Audio", "🏡 Smart Home",
    "💳 ${{amount}}", "📄 Documents", "📄 Logs", "📊 Token & Context",
    "🔍 Filter…", "🔍 debug", "🔐 OAuth2 Authorization Code",
    "🔐 Secrets", "🔑 API Key", "🔒 Lockout (min)", "🖥️ System",
    "🖱️ Manual", "🖼️ Images", "🛡️ Permissions", "🟡 Medium",
    "🥚 Hatch", "🪝 Webhook",
    "✓", "✔ set",
}

# Config section headers
CONFIG_SECTIONS = {
    "Co-Agents", "Agent & AI", "Server & System", "Smart Home",
    "Environment Variables", "LLM Provider", "Prompts & Personas", "Domain & TLS",
}

# Now categorize
user_facing = {}
config_desc = {}
short_ui = {}
brand = {}
technical = {}
already_german = {}
placeholder = {}
emoji_ui = {}
config_section = {}

for val, count in values.items():
    if val in ALREADY_GERMAN:
        already_german[val] = count
    elif val in BRAND_NAMES or val in TECHNICAL or val in PLACEHOLDERS:
        if val in BRAND_NAMES:
            brand[val] = count
        elif val in TECHNICAL:
            technical[val] = count
        else:
            placeholder[val] = count
    elif val in SHORT_UI_LABELS:
        short_ui[val] = count
    elif val in EMOJI_UI:
        emoji_ui[val] = count
    elif val in CONFIG_SECTIONS:
        config_section[val] = count
    elif any(val.startswith(p) for p in ["⚠️", "ℹ️", "⛔", "🔔"]):
        config_desc[val] = count
    elif len(val) > 30:
        # Long descriptions are likely config descriptions
        config_desc[val] = count
    else:
        user_facing[val] = count

# Write report
out = io.StringIO()
def w(s=""):
    out.write(s + "\n")

w("=" * 80)
w("ANALYSIS OF REMAINING UNTRANSLATED VALUES")
w("=" * 80)
w(f"Total unique values: {len(values)}")
w(f"Total occurrences: {sum(values.values())}")
w()

w(f"1. USER-FACING UI LABELS: {len(user_facing)} values ({sum(user_facing.values())} occ)")
w("-" * 60)
for val, count in sorted(user_facing.items(), key=lambda x: -x[1])[:30]:
    w(f"  [{count:3d}] {val[:100]}")
if len(user_facing) > 30:
    w(f"  ... and {len(user_facing)-30} more")
w()

w(f"2. CONFIG DESCRIPTIONS: {len(config_desc)} values ({sum(config_desc.values())} occ)")
w("-" * 60)
for val, count in sorted(config_desc.items(), key=lambda x: -x[1])[:30]:
    w(f"  [{count:3d}] {val[:100]}")
if len(config_desc) > 30:
    w(f"  ... and {len(config_desc)-30} more")
w()

w(f"3. SHORT UI LABELS: {len(short_ui)} values ({sum(short_ui.values())} occ)")
w("-" * 60)
for val, count in sorted(short_ui.items(), key=lambda x: -x[1])[:30]:
    w(f"  [{count:3d}] {val[:100]}")
if len(short_ui) > 30:
    w(f"  ... and {len(short_ui)-30} more")
w()

w(f"4. EMOJI UI LABELS: {len(emoji_ui)} values ({sum(emoji_ui.values())} occ)")
w("-" * 60)
for val, count in sorted(emoji_ui.items(), key=lambda x: -x[1]):
    w(f"  [{count:3d}] {val[:100]}")
w()

w(f"5. CONFIG SECTIONS: {len(config_section)} values ({sum(config_section.values())} occ)")
w("-" * 60)
for val, count in sorted(config_section.items(), key=lambda x: -x[1]):
    w(f"  [{count:3d}] {val[:100]}")
w()

w(f"6. BRAND NAMES (DO NOT TRANSLATE): {len(brand)} values ({sum(brand.values())} occ)")
w("-" * 60)
for val, count in sorted(brand.items(), key=lambda x: -x[1])[:30]:
    w(f"  [{count:3d}] {val[:100]}")
if len(brand) > 30:
    w(f"  ... and {len(brand)-30} more")
w()

w(f"7. TECHNICAL (DO NOT TRANSLATE): {len(technical)} values ({sum(technical.values())} occ)")
w("-" * 60)
for val, count in sorted(technical.items(), key=lambda x: -x[1]):
    w(f"  [{count:3d}] {val[:100]}")
w()

w(f"8. ALREADY GERMAN: {len(already_german)} values ({sum(already_german.values())} occ)")
w("-" * 60)
for val, count in sorted(already_german.items(), key=lambda x: -x[1]):
    w(f"  [{count:3d}] {val[:100]}")
w()

w(f"9. PLACEHOLDERS: {len(placeholder)} values ({sum(placeholder.values())} occ)")
w("-" * 60)
for val, count in sorted(placeholder.items(), key=lambda x: -x[1]):
    w(f"  [{count:3d}] {val[:100]}")
w()

w()
w("=" * 80)
w("SUMMARY: SHOULD BE TRANSLATED")
w("=" * 80)
translatable = len(user_facing) + len(config_desc) + len(short_ui) + len(emoji_ui) + len(config_section)
translatable_occ = sum(user_facing.values()) + sum(config_desc.values()) + sum(short_ui.values()) + sum(emoji_ui.values()) + sum(config_section.values())
w(f"Translatable: {translatable} values ({translatable_occ} occurrences)")
w(f"Do NOT translate: {len(brand) + len(technical) + len(already_german) + len(placeholder)} values ({sum(brand.values()) + sum(technical.values()) + sum(already_german.values()) + sum(placeholder.values())} occurrences)")

# Write to file
with open("disposable/analysis_report_v3.txt", "w", encoding="utf-8") as f:
    f.write(out.getvalue())

# Write categorized data as JSON
categorized = {
    "user_facing": user_facing,
    "config_descriptions": config_desc,
    "short_ui_labels": short_ui,
    "emoji_ui_labels": emoji_ui,
    "config_sections": config_section,
    "brand_names": brand,
    "technical": technical,
    "already_german": already_german,
    "placeholders": placeholder,
}
with open("disposable/categorized_untranslated_v3.json", "w", encoding="utf-8") as f:
    json.dump(categorized, f, ensure_ascii=False, indent=2)

print(f"Report written to disposable/analysis_report_v3.txt")
print(f"Data written to disposable/categorized_untranslated_v3.json")
print(f"Translatable: {translatable} values ({translatable_occ} occurrences)")
print(f"Do NOT translate: {len(brand) + len(technical) + len(already_german) + len(placeholder)} values")
