# Chapter 6: Tools

AuraGo's **100+ built-in tools** transform it from a chatbot into an autonomous agent.

---

## Tool Categories Overview

| Category | Tools | Danger Zone |
|----------|-------|-------------|
| **🗂️ Filesystem** | Read, write, edit, search, hashline editor | Yes |
| **🌐 Web & APIs** | Search (DDG, Brave, Wikipedia), HTTP, scraping, screenshots, mDNS/UPnP/WHOIS/DNS | No (partial) |
| **🌐 Web & Sites** | Homepage scaffold, build, deploy, registry, Netlify, Vercel | No (`homepage.enabled`) |
| **🐳 Docker** | Containers, images, networks, sidecars | Yes |
| **🖥️ Proxmox** | VMs, LXCs, snapshots | Yes |
| **🏠 Smart Home** | Home Assistant, MQTT, Wake-on-LAN, Frigate, AdGuard, Fritz!Box, 3D printer | Yes (partial) |
| **☁️ Cloud** | Google Workspace, WebDAV, GitHub, S3, OneDrive, Koofr, Netlify, Vercel | No (partial) |
| **📧 Communication** | Email, Telegram, Discord, Telnyx, Rocket.Chat, MQTT, MeshCentral | No |
| **🎬 Media Generation** | Images, music, videos, TTS, vision, Piper, Supertonic, media registry | No (provider limits apply) |
| **🔧 System** | Metrics, processes, cron, sandbox, background tasks, daemon skills | Partial |
| **🧠 Memory** | Memory, notes, knowledge graph, cheatsheets, core memory | No |
| **🌐 Network** | Ping, port scan, mDNS, UPnP, WHOIS, DNS lookup | No |
| **🖥️ Remote** | SSH, Invasion Control, MeshCentral, Ansible sidecar | Yes |
| **📝 Documents** | PDF Creator/Extractor, Paperless NGX, JSON/YAML/XML/TOML editors | No |
| **🎬 Media Conversion** | FFmpeg, ImageMagick, video download, YouTube player, transcription | No |
| **🛒 Skills & Python** | Skill Manager, manifest, Python tool bridge, daemon skills | No (partial) |
| **🔐 Security & Vault** | Vault, AES-256-GCM, secret injection, LLM Guardian, SSH key manager | No |
| **🖥️ Virtual Desktop** | Code Studio, Pixel, Zipper, app launcher, SSH/SFTP/VNC, office (Pixel/Calc/Writer) | No (partial) |
| **🚀 Missions & Co-Agents** | Mission Control v2, co-agent dispatcher, A2A, handoff | No (partial) |
| **🔌 MCP & Composio** | Model Context Protocol client/server, Compos.io, AI Gateway | No (`agent.allow_mcp`) |
| **🛡️ Inventory & Wake-on-LAN** | SSH device inventory, WOL, SSH key manager | Yes (passive access) |
| **📦 Browser Automation** | Headless Chrome, forms, screenshots, PDF | No (partial) |
| **⏰ Heartbeat & Background** | Scheduled wake-ups, background tasks, heartbeat service | No |
| **🧰 Editors** | JSON, YAML, XML, TOML, office (document, workbook), hashline | No |

---

## New Platform Features

The current version includes several powerful extensions:

| Feature | Description |
|---------|-------------|
| **LLM Guardian** | Risk scanning of tool calls and external content before execution |
| **Adaptive Tools** | Token optimization through context-aware tool selection |
| **Document Creator** | PDF creation for invoices, reports |
| **PDF Extractor** | Text extraction with optional LLM summarization |
| **Video Generation** | AI text-to-video and image-to-video with MiniMax Hailuo or Google Veo |
| **Media Registry** | Search, tag, and reuse generated images, audio, music, and videos |
| **MCP Client/Server** | Model Context Protocol for interoperability |
| **Invasion Control** | Distributed orchestration across multiple hosts |
| **Homepage / site projects** | Docker dev workspace, focused tools, project registry & history |
| **Sudo Execution** | Vault-backed credential handling for privileged commands |

---

## Configuration of Key Tools

> 💡 **Web UI first:** Open **Menu → Config** and use the sidebar to configure tools and integrations. The YAML blocks below are for advanced or headless setups.

### 1. Filesystem & Shell

### Web UI Setup
1. Open **Config → Danger Zone**.
2. Enable **Allow Shell Execution**, **Allow Python Execution**, and **Allow Filesystem Write** as needed.
3. Save changes.

### YAML Reference
```yaml
agent:
  allow_shell: true              # Allow shell commands
  allow_python: true             # Allow Python execution
  allow_filesystem_write: true   # Allow file write access
```

### 2. Docker

### Web UI Setup
1. Open **Config → Integrations → Docker**.
2. Enable the integration and set the Docker host.
3. Optionally enable **Read-only** for safe testing.
4. Save and restart if prompted.

### YAML Reference
```yaml
docker:
  enabled: true
  host: "unix:///var/run/docker.sock"
  # Or for remote Docker:
  # host: "tcp://docker-host:2376"
```

### 3. Proxmox

### Web UI Setup
1. Open **Config → Integrations → Proxmox**.
2. Enable the integration and enter URL, token ID, and node.
3. Store the token secret in the **Vault**.
4. Save and restart if prompted.

### YAML Reference
```yaml
proxmox:
  enabled: true
  url: "https://proxmox.example.com:8006"
  token_id: "root@pam!aurago"
  token_secret: ""              # Stored in vault
  node: "pve"
  insecure: false               # true = accept insecure TLS
```

### 4. Home Assistant

### Web UI Setup
1. Open **Config → Integrations → Home Assistant**.
2. Enable the integration and enter the instance URL.
3. Store the access token in the **Vault**.
4. Toggle **Read-only** if you only need state queries.
5. Save and restart if prompted.

### YAML Reference
```yaml
home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"
  # AccessToken stored in vault
  readonly: false               # true = read-only mode
```

### 5. Google Workspace (OAuth2)

### Web UI Setup
1. Open **Config → Integrations → Google Workspace**.
2. Enable the integration and complete OAuth2 in the Web UI.
3. Select Gmail, Calendar, and Drive permissions as needed.
4. Save changes.

### YAML Reference
```yaml
google_workspace:
  enabled: true
```

Authentication via OAuth2 in the Web UI vault menu.

### 6. Email

### Web UI Setup
1. Open **Config → Integrations → Email**.
2. Enable the integration and enter IMAP/SMTP settings.
3. Store credentials in the **Vault**.
4. Enable **Inbox Watch** if you want automatic polling.
5. Save and restart if prompted.

### YAML Reference
```yaml
email:
  enabled: true
  imap_host: "imap.gmail.com"
  imap_port: 993
  smtp_host: "smtp.gmail.com"
  smtp_port: 587
  username: "your.email@gmail.com"
  watch_enabled: true
  watch_interval_seconds: 120
```

### 7. Web Search

### Web UI Setup
1. DuckDuckGo search is enabled by default (no API key required).
2. For Brave Search, open **Config → Integrations → Brave Search**, enable it, and add your API key.
3. Save changes.

### YAML Reference
```yaml
# DuckDuckGo (no API key required)
# Enabled by default

# Brave Search (optional, better results)
brave_search:
  enabled: true
  api_key: "BS..."
  country: "US"
  lang: "en"
```

---

### 8. Media Generation

### Web UI Setup
1. Open **Config → Media & Content → Image Generation** (and **Music Generation** / **Video Generation** as needed).
2. Enable each generator and select the provider and model.
3. Set daily limits if required.
4. Save changes.

### YAML Reference
```yaml
image_generation:
  enabled: true
  provider: "openai"

music_generation:
  enabled: true
  provider: "minimax"
  max_daily: 10

video_generation:
  enabled: true
  provider: "minimax"      # or "google"
  model: "hailuo-02"       # provider-specific
  max_daily: 5
```

The related tools are `generate_image`, `generate_music`, and `generate_video`. Generated files are saved locally and registered in the Media Registry so they can be searched, tagged, sent back to chat, or reused later.

### 9. Homepage & static sites

Build and deploy marketing sites or personal homepages inside AuraGo’s dedicated homepage workspace (`data/homepage/` by default), not `agent_workspace/workdir/`.

### Web UI Setup
1. Open **Config → Integrations → Homepage**.
2. Enable the integration; set workspace path and registry DB path if needed.
3. Optionally enable **Allow local server** when Docker is unavailable (limited workflows).
4. Save. Enable **Netlify** and/or **Vercel** integrations for one-shot deploy tools.
5. Optional: **Virtual Desktop → Homepage Studio** for a guided UI on the same workspace.

### Focused agent tools (preferred)

| Tool | Role |
|------|------|
| `homepage_project` | Workspace lifecycle, `init_project`, `exec`, `install_deps` |
| `homepage_file` | `read_file`, `write_file`, `edit_file`, `list_files` in the homepage workspace |
| `homepage_deploy` | `build`, `dev`, local publish, `deploy_netlify`, `deploy_vercel`, tunnel |
| `homepage_quality` | `lint`, `check_js`, `lighthouse`, `screenshot`, image optimization |
| `homepage_git` | `git_init`, `commit`, `status`, `diff`, `log`, `rollback` |
| `homepage_registry` | Project catalog, deploy/edit logs, **project history** (`list_history`, `add_history`) |

The legacy combined tool `homepage` still accepts older `operation` values for compatibility.

**Rules:** Use `homepage_file` for site sources — not generic `filesystem`. Do not run `/workspace/...` via `execute_shell`; use `homepage_project` `exec` or deploy tools. Read `list_history` before large edits; write `add_history` after meaningful changes. Global design guardrail: `prompts/rules/homepage/DESIGN.md`.

### YAML Reference
```yaml
homepage:
  enabled: true
  allow_local_server: false
```

See [Integrations](08-integrations.md#homepage-and-site-projects) and [Internal Tools](22-internal-tools.md#homepage--static-site-tool-family).


---

## Adaptive Tools — Intelligent Filtering

**Save tokens, stay focused.** The Adaptive Tools system filters tools based on conversation context:

### Web UI Setup
1. Open **Config → Optimizations**.
2. Under **Tool Optimization**, enable **Adaptive Tools** and adjust `max_tools` / `max_total_tools`.
3. Save changes.

### YAML Reference
```yaml
agent:
  adaptive_tools:
    enabled: true
    max_tools: 10               # Adaptive/preferred tool cap
    max_total_tools: 20         # Final native schema cap
    max_schema_tokens: 6500     # Rough schema-token cap (0 = unlimited)
    provider_profiles_enabled: true
    session_tool_retention_turns: 8
    
    # Always available (don't filter):
    always_include:
      - "filesystem"
      - "query_memory"
      - "manage_memory"
      - "execute_shell"
```

| Aspect | Without Adaptive | With Adaptive |
|--------|------------------|---------------|
| **Tokens** | 50+ tools in prompt | Relevant tools within a total budget |
| **Cost** | Higher | Lower |
| **Accuracy** | LLM overwhelmed | Precise tool selection |

The `max_tools` setting caps adaptive/preferred tools only. AuraGo first keeps hard-required recovery tools, then the small soft always-include core and recent session tools, then adaptive tools. Tools such as `ddg_search`, `api_request`, `docker`, `execute_python`, `file_editor`, `manage_missions`, and Virtual Desktop helpers are normally pulled in by intent, channel, recent use, or `discover_tools`.

The final native schema budget is controlled by `max_total_tools` where it can be applied. Provider profiles are only stability overlays; regular chat, bots, missions, background tasks, and desktop sessions all use the same budget path.

---

## Read-Only vs. Read-Write

Many tools support a read-only mode:

### Web UI Setup
1. Open the integration in **Config** (e.g. **Config → Integrations → Docker**).
2. Enable **Read-only** on that integration's form.
3. Save changes.

### YAML Reference
```yaml
docker:
  enabled: true
  readonly: true        # list/inspect/logs only

home_assistant:
  enabled: true
  readonly: true        # get_states only; blocks call_service
```

---

## Danger Zone

The Danger Zone controls potentially dangerous operations:

### Web UI Setup
1. Open **Config → Danger Zone**.
2. Toggle capability gates (`allow_shell`, `allow_python`, `allow_mcp`, etc.) as needed.
3. For web scraping, open **Config → Agent Tools → Web Scraper** and enable it there.
4. Save changes.

### YAML Reference
```yaml
agent:
  allow_shell: true              # Shell commands
  allow_python: true             # Python execution
  allow_filesystem_write: true   # File write access
  allow_network_requests: true   # HTTP requests
  allow_remote_shell: true       # SSH to remote devices
  allow_self_update: true        # Self-updates
  allow_mcp: true                # MCP protocol (also requires mcp.enabled)
  allow_package_manager: false   # also requires package_manager.enabled (**Config → Danger Zone**)

tools:
  web_scraper:
    enabled: true                # replaces deprecated allow_web_scraper (**Config → Agent Tools → Web Scraper**)
```

> ⚠️ **Tip:** In production environments, only enable what you really need.

---

## Examples in Chat

### Filesystem
```
You: List all files in the project folder
Agent: 📁 Project folder:
       ├── main.go
       ├── go.mod
       └── config.yaml

You: Create a file notes.txt with "Important ideas"
Agent: ✅ File created: notes.txt
```

### Docker
```
You: Show all running containers
Agent: 🐳 Running containers:
       ┌─────────┬────────┬─────────────┐
       │ NAME    │ STATUS │ PORTS       │
       ├─────────┼────────┼─────────────┤
       │ nginx   │ Up 2h  │ 80→8080/tcp │
       │ postgres│ Up 2h  │ 5432/tcp    │
       └─────────┴────────┴─────────────┘

You: Restart the redis container
Agent: 🔄 Restarting container redis...
       ✅ Success (3.2s)
```

### Home Assistant
```
You: Turn off the living room light
Agent: 🏠 Smart Home:
       ✅ Living Room Light: OFF

You: What's the temperature in the bedroom?
Agent: 🌡️ Bedroom Sensor:
       Temperature: 21.5°C
       Humidity: 45%
```

### Web Search
```
You: Search for Go best practices
Agent: 🔍 Searching...
       Found: 5 results
       1. Go Code Review Comments
       2. Effective Go
       ...
```

### Video Generation
```
You: Create a short 16:9 video of a sunrise over a futuristic home lab
Agent: 🎬 Generating video with the configured provider...
       ✅ Video created and registered in the media registry

You: Send the generated video to me
Agent: ▶️ Video attached with an inline player
```

---

## Network Tools

Diagnostic tools for network scanning and monitoring.

### Configuration

### Web UI Setup
1. Open **Config → Agent Tools → Network Tools**.
2. Enable **Network Ping**, **Network Scan**, **UPnP Scan**, **Web Capture**, and **Form Automation** as needed.
3. Save changes.

### YAML Reference
```yaml
tools:
  network_ping:
    enabled: true                 # ICMP ping and port scanner
  network_scan:
    enabled: true                 # mDNS/Bonjour discovery
  upnp_scan:
    enabled: true                 # UPnP/SSDP device discovery
```

### Available Tools

| Tool | Description |
|------|-------------|
| `network_ping` | ICMP ping to a host |
| `port_scanner` | TCP port scan on a host |
| `mdns_scan` | Find mDNS/Bonjour devices on the LAN |
| `upnp_scan` | Find UPnP/SSDP devices on the LAN |

### Examples in Chat

```
Ping google.com
Scan ports on 192.168.1.1
Find all devices on the local network
Which UPnP devices are available?
```

---

## Web Capture & Form Automation

Screenshots, PDF generation, and browser automation.

### Configuration

### Web UI Setup
1. Open **Config → Agent Tools → Network Tools**.
2. Enable **Web Capture** and optionally **Form Automation**.
3. Save changes.

### YAML Reference
```yaml
tools:
  web_capture:
    enabled: true                 # Screenshots and PDF from web pages
  form_automation:
    enabled: false                # Form automation (requires web_capture)
```

### Available Tools

| Tool | Description |
|------|-------------|
| `web_capture` | Create a screenshot or PDF of a web page |
| `web_performance_audit` | Measure Core Web Vitals (TTFB, FCP, LCP) |
| `form_automation` | Fill and submit forms automatically |

### Requirements

- Headless Chromium (started automatically when needed)
- More RAM for large pages

### Examples in Chat

```
Take a screenshot of google.com
Save the documentation as PDF
Fill out the contact form on example.com
```

---

## PDF Extractor

Extract text from PDF documents with optional LLM summarization.

### Configuration

### Web UI Setup
1. Open **Config → Agent Tools → Information Tools**.
2. Configure **PDF Extractor** (enable summary mode if desired).
3. Save changes.

### YAML Reference
```yaml
tools:
  pdf_extractor:
    enabled: true
    summary_mode: false           # When true, summarise extracted text via LLM
```

### Available Tools

| Tool | Description |
|------|-------------|
| `pdf_extractor` | Extract text and metadata from PDF files |

### Examples in Chat

```
Extract the text from report.pdf
Summarise the key points from invoice.pdf
```

---

## Media Conversion & Video Download

Conversion of audio, video, and image files as well as video download from platforms like YouTube.

### Configuration

### Web UI Setup
1. Open **Config → Media & Content → Media Conversion** for FFmpeg/ImageMagick paths.
2. Open **Config → Media & Content → Video Download** for download mode and transcription options.
3. Enable **Send YouTube Video** under **Config → Tools** if needed.
4. Save changes.

### YAML Reference
```yaml
tools:
  media_conversion:
    enabled: true
    ffmpeg_path: ""
    imagemagick_path: ""
  video_download:
    enabled: true
    mode: "docker"                # docker or native
    download_dir: "data/downloads"
    allow_transcribe: false
  send_youtube_video:
    enabled: true
```

### Available Tools

| Tool | Description |
|------|-------------|
| `media_conversion` | Convert files between formats (FFmpeg/ImageMagick) |
| `video_download` | Download videos from YouTube and other platforms |
| `send_youtube_video` | Send YouTube videos as embedded players |

### Requirements

- FFmpeg and/or ImageMagick (system-wide or paths configured)
- For video download: yt-dlp (in Docker container or system-wide)

### Examples in Chat

```
Convert video.mp4 to audio.mp3
Download the YouTube video
Send me the YouTube video as an embedded player
```

---

## Creating Custom Tools

AuraGo can create new Python tools at runtime:

```
You: Create a tool that converts temperatures
Agent: 🛠️ Creating temperature converter...
       ✅ Created: temperature_converter.py
       
You: What's 25°C in Fahrenheit?
Agent: 🌡️ 25°C = 77°F
```

Created tools are saved to `agent_workspace/tools/` and available immediately.

---

## Summary

| Category | Highlights |
|----------|------------|
| **100+ Tools** | For nearly any home lab task |
| **Security** | Read-only mode, Danger Zone, LLM Guardian |
| **Flexibility** | Dynamic tool creation at runtime |
| **Efficiency** | Adaptive Tools save tokens |
| **Media** | Generate and deliver images, music, audio, documents, and videos |

> 💡 **Remember:** AuraGo's strength lies in combining tools. A single chat can chain shell commands, web searches, Docker operations, and email notifications — fully automatically!

---

**Next Steps**

- **[Configuration](07-configuration.md)** — Fine-tune tools
- **[Integrations](08-integrations.md)** — Connect external services
