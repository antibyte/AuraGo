#!/usr/bin/env python3
"""Analyze remaining untranslated values and categorize them."""
import json
import re
import io
from pathlib import Path

with open("disposable/untranslated_values.json", "r", encoding="utf-8") as f:
    data = json.load(f)

values = data["values"]

# Categories
BRAND_NAMES = {
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
    "Fritz!Box:", "Gotenberg URL", "Home Assistant", "Host / IP",
    "Jellyfin Host", "Ollama URL", "SMB:", "TR-064 Port",
    "TrueNAS Host", "WebDAV URL", "Cloudflare Tunnel",
    "Docker Host", "Docker Image", "Egg Port",
    "IMAP Host", "IMAP Port", "SMTP Host", "SMTP Port",
    "OAuth2 Client ID", "OAuth2 Client Secret", "SSH Private Key",
    "HTTP Port", "HTTPS Port", "GPU Backend",
    "Quality of Service (QoS)", "Text-to-Speech", "Whisper speech-to-text",
    "Time Machine support (Mac)", "Native Function Calling ⚠️",
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

# Short UI labels that are definitely user-facing
SHORT_UI_LABELS = {
    "ACTIVE", "Agent Status", "Budget", "Budget & Tokens", "Burst",
    "Cache Rate", "Cache hits", "Coder", "Collection", "Collections",
    "Command", "Communication", "Compression", "Daemon", "Daemons",
    "Database", "Datasets", "Date", "Designer", "Details",
    "Direct", "Disable", "Disk", "Download", "Download PNG", "Download SVG",
    "E-Mail", "Edges", "Email", "Embeddings", "Embeddings ✗",
    "Enable", "Enabled", "Error", "Feedback", "Folder",
    "Format", "Format Opt.", "Full", "Graph Edges", "Graph Nodes",
    "Headers", "Highlights", "Homepage", "Hostname", "Idle",
    "Images", "Immediate", "Inbox", "Inbox Watcher", "Indexer",
    "Infrastructure", "Journal", "KG Explorer", "Label", "Limits",
    "Local", "Local (Ollama)", "Locked", "Log only", "Logging",
    "Login", "Logs", "Maintenance", "Manual", "Master Key",
    "Max Tools", "Media", "Medium", "Medium risk", "Memory Health",
    "Messages", "Messenger", "Mode", "Navigation", "Neutral",
    "Never", "Nodes", "Notes", "Notification", "Notifications",
    "Notify (SSE only)", "Offline", "Online", "Open", "Optional",
    "Parameters", "Performance", "Permanent", "Permissions",
    "Permissions (Scopes)", "Personality V2", "Plan", "Pools",
    "Preset", "Profiling", "Prompt", "Providers", "Public",
    "Query Models", "RAG Batch", "Rate Limiting", "Recoveries",
    "Researcher", "Rollback", "Run now", "Scopes", "Secrets",
    "Send", "Session", "Shed Rate", "Show free models only",
    "Skill Code", "Skill Details", "Skills", "Snapshots",
    "Speech", "Stable", "Start", "Start Scrub", "Start Tunnel",
    "Status", "Stop", "Test", "Time", "Tokens", "Topics",
    "Total", "Total Runs", "Tracked", "Transfer Vault", "Tunnel",
    "Types", "Update Available", "Updates", "Upload", "Upload Skill",
    "Uptime", "User", "Valence", "Vault", "Vault Keys", "Version",
    "View Log", "Vision", "Watch Folder", "Watch enabled",
    "Web Config & Login", "Web Config UI", "Web Scraper",
    "d ago", "h ago", "just now", "m ago", "optional",
    "not required", "alias (e.g. repo)", "e.g. GitHub Push",
    "e.g. X-Hub-Signature-256", "e.g. github-push",
    "source (e.g. repository.full_name)", "Comma-separated values",
    "user, agent", "x executed",
}

# Config section labels
CONFIG_SECTIONS = {
    "Agent & AI", "Budget & Tokens", "Co-Agents", "Domain & TLS",
    "Environment Variables", "LLM Provider", "Logging",
    "Prompts & Personas", "Server & System", "Smart Home",
    "Web Config & Login", "Web Config UI",
}

# Config descriptions (longer text that should be translated)
CONFIG_DESC_PATTERNS = [
    "Allow HTTP", "Allow temporary", "Allows the agent",
    "Automatically adjusts", "Automatically shortens", "Automatically install",
    "Base personality", "Browse all available", "Changing the embeddings",
    "Choose a safe rollout", "Choose preset", "Cloudflare Account ID for",
    "Comma-separated list", "Connection timeout", "Container backend",
    "Default Quality of Service", "Default bucket name", "Default channel",
    "Default node name", "Disable TLS certificate", "Display name",
    "Enable Cloudflare AI Gateway", "Enable TLS/SSL", "Enable it to let",
    "Enable path-style", "Enable the built-in MCP", "Empty = default",
    "Empty = same as username", "Enables HTTPS via Let's",
    "Enables a debugging-focused MCP", "Enables the Paperless",
    "Enables the S3-compatible", "Enables the dedicated Helper",
    "Enables/disables agent debug", "Existing long-term memory",
    "Expose AuraGo securely", "Expose Homepage over Tailscale",
    "Full: reflection", "Funnel additionally", "Funnel only applies",
    "Generate a bearer token", "Generate a token here",
    "Generate images from text", "GitHub username",
    "Half-life for usage", "Helper LLM", "Homepage exposure",
    "Homepage flows may", "How many times the agent",
    "How many times the same response", "How many times the same tool",
    "How often the agent checks", "How quickly personality",
    "How sensitive the persona", "How strongly external events",
    "How the persona responds", "HTTPS over Tailscale",
    "Inside homepage tool calls", "Internal (OpenRouter",
    "Keeps the sandbox MCP", "Learns from usage history",
    "Legacy compatibility field", "List of tool names",
    "Loopback HTTP port", "Lowercase, numbers, hyphens",
    "Maximum number of automatic", "Maximum number of messages",
    "Maximum number of tool manuals", "Maximum number of tools",
    "Maximum requests per second", "Maximum wait time for HTTP",
    "Maximum wait time for a background", "Memory analysis uses",
    "Microsoft OneDrive cloud", "Minimum messages in history",
    "Normal: reflection", "Number of consecutive identical",
    "Number of pre-warmed", "OAuth2 authorization endpoint",
    "OAuth2 token exchange endpoint", "Optional model override",
    "OpenAI-compatible endpoint", "OpenRouter recommended",
    "Opens a plain HTTP port", "Password for MQTT",
    "Path to the CA certificate", "Path to the client certificate",
    "Path to the client private key", "Prefers tools with higher",
    "Provider from provider management", "Provider type determines",
    "Publishes the Homepage", "Read-only mode for MeshCentral",
    "Registers a new SSH server", "Removes stale tool transition",
    "Require Bearer token", "Requirements for HTTPS",
    "S3 endpoint URL", "Scraped web pages are summarized",
    "Secret Access Key for S3", "Set the absolute host path",
    "Skip TLS certificate verification. WARNING",
    "Skip setup", "Skip ✕", "Smart Home", "Space-separated OAuth2",
    "Stores the sudo password", "Stored conversation memories",
    "Temporarily raises the homepage", "The Helper LLM is disabled",
    "The Helper LLM is enabled", "The LLM Guardian always",
    "The configuration is already saved", "The embeddings reset",
    "The name or slug", "The request timed out",
    "This change will likely", "Time for the daily maintenance",
    "Tool guides, documentation", "Tool names always offered",
    "Tool responses exceeding", "URL of the Paperless",
    "Unique identifier", "Update AuraGo to the latest",
    "Use `/personality", "Use this snippet",
    "Uses the central Helper LLM", "Uses the provider's native",
    "Usually 993", "Valence", "Wait time after the main",
    "Wait time between retry", "When active, n8n",
    "When enabled, MCP clients",
    "When enabled, scraped content",
    "When enabled, the agent can only list",
    "When enabled, the agent can only search",
    "Your Cloudflare Account ID",
    "n8n Webhook Base URL", "n8n can connect to AuraGo",
    "n8n workflow automation", "ntfy topic name",
    "admin password", "auth.enabled",
    "custom = keep manual flags",
    "ℹ️ /credits is only available",
    "⚠️ **System Warnings**", "⚠️ Budget warning",
    "⚠️ Note:", "⚠️ Warnings system",
    "⛔ Budget exceeded",
]

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
    elif any(val.startswith(p) for p in CONFIG_DESC_PATTERNS):
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
for val, count in sorted(user_facing.items(), key=lambda x: -x[1]):
    w(f"  [{count:3d}] {val[:100]}")
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
for val, count in sorted(short_ui.items(), key=lambda x: -x[1]):
    w(f"  [{count:3d}] {val[:100]}")
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
for val, count in sorted(brand.items(), key=lambda x: -x[1]):
    w(f"  [{count:3d}] {val[:100]}")
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
with open("disposable/analysis_report_v2.txt", "w", encoding="utf-8") as f:
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
with open("disposable/categorized_untranslated_v2.json", "w", encoding="utf-8") as f:
    json.dump(categorized, f, ensure_ascii=False, indent=2)

print(f"Report written to disposable/analysis_report_v2.txt")
print(f"Data written to disposable/categorized_untranslated_v2.json")
print(f"Translatable: {translatable} values ({translatable_occ} occ)")
print(f"Do NOT translate: {len(brand) + len(technical) + len(already_german) + len(placeholder)} values")
