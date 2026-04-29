# Chapter 8: Integrations

AuraGo connects with numerous external services to extend its capabilities. This chapter covers all available integrations and their configuration.

## Setting Up Integrations via the Web UI

The easiest and recommended way to configure integrations is through the AuraGo Web UI:

1. Open the AuraGo Web UI in your browser.
2. Go to **Menu → Config → Integrations**.
3. Find the desired integration in the list.
4. Toggle **Enabled**.
5. Fill in the required fields (URL, Host, Username, etc.).
6. Store credentials securely in the **Vault** (not in `config.yaml`!).
7. Click **Save** and restart AuraGo if prompted.

> 💡 **Tip:** Most integrations support a **Read-only** toggle. Enable it first to test safely.

---

## Telegram Bot Setup

Telegram is the recommended mobile interface for AuraGo, supporting text, voice, and images.

### Step 1: Create a Bot with BotFather
1. Open Telegram and search for **@BotFather**.
2. Send `/newbot` and follow the prompts.
3. Copy the **HTTP API Token**.

### Step 2: Get Your User ID
Search for **@userinfobot** in Telegram and start it—it will reply with your ID. Alternatively, set `telegram_user_id: 0`, start AuraGo, message your bot, and check the logs for your ID.

### Web UI Setup
1. Open **Config → Integrations → Telegram**.
2. Enable the integration.
3. Paste the **Bot Token** and your **Telegram User ID**.
4. Save and restart AuraGo.
5. Send `/start` to your bot to test.

> ⚠️ **Security Warning:** Never share your bot token.

### YAML Reference
```yaml
telegram:
    bot_token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
    telegram_user_id: 123456789
```

---

## Discord Bot Setup

AuraGo can participate in Discord servers and respond to mentions.

### Step 1: Create a Discord Application
1. Go to the [Discord Developer Portal](https://discord.com/developers/applications) and create a new application.
2. Go to **Bot → Add Bot**.
3. Enable **Message Content Intent**.

### Step 2: Invite the Bot
1. Go to **OAuth2 → URL Generator**.
2. Select scopes `bot` and `applications.commands`.
3. Select permissions: Send Messages, Read Messages/View Channels, Read Message History, Attach Files, Use Slash Commands.
4. Open the generated URL and add the bot to your server.

### Step 3: Get IDs
Enable **Developer Mode** in Discord (User Settings → Advanced), then right-click your server name to copy the **Server ID** and a channel to copy the **Channel ID**.

### Web UI Setup
1. Open **Config → Integrations → Discord**.
2. Enable the integration.
3. Paste the **Bot Token**, **Guild ID**, **Channel ID**, and your **User ID**.
4. Save and restart.

> 💡 **Tip:** Enable `readonly` for a monitoring-only bot.

### YAML Reference
```yaml
discord:
    enabled: true
    bot_token: "YOUR_BOT_TOKEN"
    guild_id: "123456789012345678"
    allowed_user_id: "987654321098765432"
    default_channel_id: "123456789012345678"
```

---

## Email Configuration

AuraGo can read emails via IMAP and send via SMTP.

### Web UI Setup
1. Open **Config → Integrations → Email**.
2. Enable the integration.
3. Enter your IMAP/SMTP host and port.
4. Fill in your **Username** and store the **Password** in the Vault.
5. Toggle **Watch Enabled** to let AuraGo poll for new emails.
6. Save and restart.

For Gmail, you must create an **App Password** (not your regular password) at [Google Account Security](https://myaccount.google.com/security) → 2-Step Verification → App passwords.

### YAML Reference
```yaml
email:
    enabled: true
    imap_host: imap.gmail.com
    imap_port: 993
    smtp_host: smtp.gmail.com
    smtp_port: 587
    username: "your.email@gmail.com"
    from_address: "your.email@gmail.com"
    watch_enabled: true
```

---

## Home Assistant Integration

Control your smart home devices through AuraGo.

### Web UI Setup
1. In Home Assistant, go to **Profile → Long-Lived Access Tokens → Create Token**. Name it "AuraGo" and copy the token.
2. In AuraGo, open **Config → Integrations → Home Assistant**.
3. Enable the integration.
4. Enter your Home Assistant URL (e.g., `http://homeassistant.local:8123`).
5. Store the **Access Token** in the Vault.
6. Save and restart.

> 💡 **Tip:** Enable `readonly` for monitoring-only access.

### YAML Reference
```yaml
home_assistant:
    enabled: true
    url: http://homeassistant.local:8123
    access_token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9..."
    allowed_services:
      - "light.turn_on"
      - "light.turn_off"
    blocked_services:
      - "lock.unlock"
```

`allowed_services` is an explicit allowlist for `call_service`; leave it empty to block service calls while still allowing state reads.

---

## Docker Integration

Manage Docker containers, images, and networks.

### Web UI Setup
1. Open **Config → Integrations → Docker**.
2. Enable the integration.
3. Set the **Docker Host** (`unix:///var/run/docker.sock` on Linux/macOS, `npipe:////./pipe/docker_engine` on Windows, or a remote `tcp://` address).
4. Save and restart.

> ⚠️ **Warning:** Docker integration grants significant system access. Use `readonly` for restricted environments.

### YAML Reference
```yaml
docker:
    enabled: true
    host: "unix:///var/run/docker.sock"
```

---

## Proxmox Integration

Manage VMs and LXC containers on Proxmox VE.

### Web UI Setup
1. In Proxmox, go to **Datacenter → Permissions → API Tokens** and create a token for a user (e.g., `root@pam`).
2. In AuraGo, open **Config → Integrations → Proxmox**.
3. Enable the integration and enter the **URL**, **Token ID**, and **Node** name.
4. Store the token secret in the Vault.
5. Save and restart.

### YAML Reference
```yaml
proxmox:
    enabled: true
    readonly: false
    allow_destructive: false
    url: "https://proxmox.local:8006"
    token_id: "root@pam!aurago"
    node: "pve"
```

---

## Webhooks

Receive HTTP events from external services.

### Web UI Setup
1. Open **Config → Integrations → Webhooks**.
2. Enable the integration.
3. Create a new webhook in the Web UI, choose a preset (generic, GitHub, GitLab, etc.), and copy the generated URL.
4. Paste the URL into the external service (e.g., GitHub repository settings).

### YAML Reference
```yaml
webhooks:
    enabled: true
    max_payload_size: 65536
    rate_limit: 60
```

---

## Budget Tracking

Track and control LLM API costs.

### Web UI Setup
1. Open **Config → Integrations → Budget**.
2. Enable the integration.
3. Set your **Daily Limit** and **Enforcement Mode** (`warn`, `partial`, or `full`).
4. Save and restart.

### YAML Reference
```yaml
budget:
    enabled: true
    daily_limit_usd: 5.00
    enforcement: warn
```

---

## Google Workspace

Connect AuraGo to Gmail, Google Calendar, and Google Drive.

### Web UI Setup
1. Open **Config → Integrations → Google Workspace**.
2. Enable the integration and select the services you want (Gmail, Calendar, Drive, Docs, Sheets).
3. Enter your **OAuth Client ID**.
4. Save and restart.
5. Trigger a Google operation in chat; AuraGo will provide an authorization URL. Approve it in your browser and submit the redirected URL back to AuraGo.

### YAML Reference
```yaml
google_workspace:
    enabled: true
    gmail: true
    calendar: true
    drive: true
    client_id: "YOUR_CLIENT_ID"
```

---

## WebDAV/Koofr Setup

Access cloud storage through WebDAV-compatible services.

### Web UI Setup
1. Open **Config → Integrations → WebDAV** (or **Koofr**).
2. Enable the integration.
3. Enter the **URL**, **Username**, and store the **Password/App Password** in the Vault.
4. Save and restart.

### YAML Reference

**WebDAV:**
```yaml
webdav:
    enabled: true
    readonly: false
    auth_type: basic
    url: "https://cloud.example.com/remote.php/dav/files/username/"
    username: "your_username"
```

**Koofr:**
```yaml
koofr:
    enabled: true
    readonly: false
    username: "your_username"
    base_url: "https://app.koofr.net"
```

> 🔒 Passwords are stored in the Vault.

---

## TrueNAS Integration

Manage ZFS pools, datasets, and shares.

### Web UI Setup
1. In TrueNAS, go to **System → API Keys** and create a key named "AuraGo".
2. In AuraGo, open **Config → Integrations → TrueNAS**.
3. Enable the integration, enter the **Host**, **Port**, and enable **HTTPS**.
4. Store the API key in the Vault.
5. Save and restart.

### YAML Reference
```yaml
truenas:
    enabled: true
    host: "truenas.local"
    port: 443
    use_https: true
```

---

## Tailscale Integration

Manage your Tailscale VPN network.

### Web UI Setup
1. Go to [Tailscale Admin Console](https://login.tailscale.com/admin/settings/keys) and generate an API key.
2. In AuraGo, open **Config → Integrations → Tailscale**.
3. Enable the integration and enter your **Tailnet** name.
4. Store the API key in the Vault.
5. Save and restart.

### YAML Reference
```yaml
tailscale:
    enabled: true
    tailnet: "your-tailnet"
```

---

## FritzBox Integration

Control AVM Fritz!Box routers via TR-064.

### Web UI Setup
1. Open **Config → Integrations → FritzBox**.
2. Enable the integration.
3. Enter the **Host**, **Port**, **Username**, and store the **Password** in the Vault.
4. Toggle the sub-systems you want (Network, Smart Home, Telephony, etc.).
5. Save and restart.

### YAML Reference
```yaml
fritzbox:
    enabled: true
    host: "fritz.box"
    username: "admin"
```

---

## AdGuard Home Integration

Manage DNS filtering and blocking.

### Web UI Setup
1. Open **Config → Integrations → AdGuard**.
2. Enable the integration.
3. Enter the **URL** and **Username**.
4. Store the **Password** in the Vault.
5. Save and restart.

### YAML Reference
```yaml
adguard:
    enabled: true
    url: "http://adguard.local"
    username: "admin"
```

---

## n8n Integration

Connect with n8n workflow automation. AuraGo can expose scoped API access to n8n workflows, execute selected tools, create isolated chat sessions, search/store memory, and create or manage missions from workflows.

### Web UI Setup
1. Open **Config → Integrations → n8n**.
2. Enable the integration.
3. Enter your **Webhook Base URL** for outbound calls from AuraGo to n8n.
4. Configure allowed scopes and tools. Use the narrowest scopes that your workflow needs.
5. Enable HMAC/token checks and rate limits for public endpoints.
6. Save and restart.

AuraGo also provides an official n8n community node: `@antibyte/n8n-nodes-aurago`.

### Scopes and capabilities

| Scope | Allows |
|-------|--------|
| `n8n:chat` | Start or continue isolated AuraGo chat sessions from n8n |
| `n8n:tools` | Execute explicitly allowed AuraGo tools from workflows |
| `n8n:memory` | Search or store memory entries through workflow logic |
| `n8n:missions` | Create, update, trigger, or inspect Mission Control tasks |
| `n8n:admin` | Administrative operations; keep disabled unless required |

Use `readonly: true` for workflows that should only read state. `scopes` and `allowed_tools` are explicit allowlists; leave either empty to disable that capability.

### YAML Reference
```yaml
n8n:
    enabled: true
    readonly: false
    webhook_base_url: "https://n8n.yourdomain.com"
    allowed_events: ["message", "tool_result"]
    require_token: true
    allowed_tools: ["query_memory", "manage_missions"]
    rate_limit_rps: 10
    scopes: ["n8n:chat", "n8n:tools", "n8n:memory"]
```

---

## Telnyx Integration

Send/receive SMS and make voice calls.

### Web UI Setup
1. Open **Config → Integrations → Telnyx**.
2. Enable the integration.
3. Enter your **Phone Number**, **Messaging Profile ID**, and **Connection ID**.
4. Save and restart.

### YAML Reference
```yaml
telnyx:
    enabled: true
    phone_number: "+1234567890"
    messaging_profile_id: "PROFILE_ID"
    allowed_numbers:
      - "+1234567890"
```

`allowed_numbers` is an explicit E.164 allowlist for inbound calls/SMS and outbound notifications. Leave it empty to block Telnyx traffic until numbers are configured.

---

## VirusTotal Integration

Scan files and URLs for malware.

### Web UI Setup
1. Sign up at [VirusTotal](https://www.virustotal.com) and copy your API key.
2. Open **Config → Integrations → VirusTotal**.
3. Enable the integration and paste the API key (or store it in the Vault).
4. Save and restart.

### YAML Reference
```yaml
virustotal:
    enabled: true
    api_key: "your-api-key"
```

---

## Brave Search Integration

Enable web search via Brave Search API.

### Web UI Setup
1. Sign up at [Brave Search API](https://api.search.brave.com) and generate a key.
2. Open **Config → Integrations → Brave Search**.
3. Enable the integration and paste the API key.
4. Save and restart.

### YAML Reference
```yaml
brave_search:
    enabled: true
    api_key: "BS..."
```

---

## MCP (Model Context Protocol)

Connect external MCP servers (client) or expose AuraGo itself as an MCP server.

### Web UI Setup
1. Open **Config → Integrations → MCP**.
2. Enable the client and/or server as needed.
3. Add allowed tools and server commands.
4. Save and restart.

### MCP Client
Allows the agent to use tools from external MCP servers.

```yaml
mcp:
    enabled: true
    allowed_tools: ["fetch", "filesystem"]
    servers:
        - name: "fetch-server"
          command: "uvx"
          args: ["mcp-server-fetch"]
          allowed_tools: ["fetch"]
          allow_destructive: false
```

### MCP Server
Exposes AuraGo tools to external clients.

```yaml
mcp_server:
    enabled: true
    port: 8089
    allowed_tools:
        - "execute_shell"
        - "filesystem"
```

`allowed_tools` is an explicit server-side allowlist. Leave it empty to expose no AuraGo tools; `vscode_debug_bridge` applies its own limited debugging preset.

---

## Jellyfin Integration

Manage your Jellyfin media server.

### Web UI Setup
1. Open **Config → Integrations → Jellyfin**.
2. Enable the integration.
3. Enter the **Host**, **Port**, and enable **HTTPS** if needed.
4. Store the API key in the Vault.
5. Save and restart.

### YAML Reference
```yaml
jellyfin:
    enabled: true
    host: "jellyfin.local"
    port: 8096
```

---

## Image Generation Integration

Generate images via AI providers.

### Web UI Setup
1. Open **Config → Integrations → Image Generation**.
2. Enable the integration.
3. Select a **Provider** and **Model**.
4. Save and restart.

### YAML Reference
```yaml
image_generation:
    enabled: true
    provider: "openai"
    model: "dall-e-3"
```

---

## Video Generation Integration

Generate short videos from text prompts or image guidance. Supported providers include MiniMax Hailuo and Google Veo, depending on your configured API credentials and model availability.

### Capabilities

| Capability | Description |
|------------|-------------|
| Text-to-video | Generate a video directly from a prompt |
| Image-to-video | Use a first frame as guidance |
| First/last frame guidance | Provider-supported start/end frame control |
| Reference images | Provider-supported image references |
| Media Registry | Generated MP4 files are saved and registered automatically |
| Daily limits | Limit cost and provider usage with `max_daily` |

### Web UI Setup
1. Open **Config → Integrations → Video Generation**.
2. Enable the integration.
3. Select a **Provider** (`minimax` or `google`) and model.
4. Set duration, resolution, aspect ratio, and daily limits.
5. Store provider credentials in the Vault.
6. Save and restart.

### YAML Reference
```yaml
video_generation:
    enabled: true
    provider: "minimax"
    model: "hailuo-02"
    duration_seconds: 6
    resolution: "720p"
    aspect_ratio: "16:9"
    max_daily: 5
```

Use the `generate_video` tool in chat. Existing video files can be sent back to the user with `send_video`, which renders them as inline video players in the Web UI.

---

## Netlify Integration

Deploy static sites on Netlify.

### Web UI Setup
1. Open **Config → Integrations → Netlify**.
2. Enable the integration.
3. Enter your **Team Slug** and optional **Default Site ID**.
4. Save and restart.

### YAML Reference
```yaml
netlify:
    enabled: true
    team_slug: "your-team"
```

---

## Paperless NGX Integration

Access your document archive.

### Web UI Setup
1. Open **Config → Integrations → Paperless NGX**.
2. Enable the integration.
3. Enter the **URL**.
4. Save and restart.

### YAML Reference
```yaml
paperless_ngx:
    enabled: true
    url: "http://paperless.local:8000"
```

---

## LLM Guardian Integration

Content safety and policy enforcement.

### Web UI Setup
1. Open **Config → Integrations → LLM Guardian**.
2. Enable the integration.
3. Choose a **Provider**, **Model**, and **Default Level**.
4. Save and restart.

### YAML Reference
```yaml
llm_guardian:
    enabled: true
    default_level: "medium"
```

---

## Remote Control Integration

Receive commands from other AuraGo instances.

### Web UI Setup
1. Open **Config → Integrations → Remote Control**.
2. Enable the integration.
3. Set the **Discovery Port** and **Allowed Paths**.
4. Save and restart.

### YAML Reference
```yaml
remote_control:
    enabled: true
    discovery_port: 8092
    allowed_paths:
      - "/home/aurago"
```

`allowed_paths` is an explicit allowlist for remote file operations. Leave it empty to block remote file reads, writes, and directory listings.

---

## Sandbox Integration

Run isolated tool executions.

### Web UI Setup
1. Open **Config → Integrations → Sandbox**.
2. Enable the integration.
3. Choose a **Backend** (e.g., `docker`) and set the timeout.
4. Save and restart.

### YAML Reference
```yaml
sandbox:
    enabled: true
    backend: docker
```

---

## Skill Manager Integration

Upload and enable custom Python skills.

### Web UI Setup
1. Open **Config → Integrations → Skill Manager**.
2. Enable the integration.
3. Configure upload limits and scanning options.
4. Save and restart.

### YAML Reference
```yaml
tools:
    skill_manager:
        enabled: true
        allow_uploads: true
```

---

## AI Gateway Integration

Route and monitor AI traffic through Cloudflare AI Gateway.

### Web UI Setup
1. Open **Config → Integrations → AI Gateway**.
2. Enable the integration.
3. Enter your **Account ID** and **Gateway ID**.
4. Optionally enable `log_requests` for detailed logging.
5. Save and restart.

### YAML Reference
```yaml
ai_gateway:
    enabled: true
    account_id: "YOUR_ACCOUNT_ID"
    gateway_id: "YOUR_GATEWAY_ID"
    log_requests: false
```

---

## Notifications Integration

Send push notifications via ntfy or Pushover.

### Web UI Setup
1. Open **Config → Integrations → Notifications**.
2. Enable ntfy and/or Pushover.
3. Enter the URL, topic, or user key as needed.
4. Save and restart.

### YAML Reference
```yaml
notifications:
    ntfy:
        enabled: true
        url: "https://ntfy.sh"
        topic: "aurago"
```

---

## Web Push / PWA Notifications

AuraGo also supports browser Web Push for the installable PWA. This is separate from ntfy and Pushover: browsers subscribe through VAPID keys, subscriptions are stored in `data/push.db`, and AuraGo can deliver local browser notifications without an external notification provider.

### API Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /api/push/vapid-pubkey` | Retrieve the public VAPID key |
| `POST /api/push/subscribe` | Register the browser push subscription |
| `POST /api/push/unsubscribe` | Remove the current browser subscription |
| `GET /api/push/status` | Check push availability and subscription state |

Web Push requires HTTPS or `localhost`, because browsers block service-worker push subscriptions on insecure origins.

---

## Chromecast Integration

Stream media and TTS to Chromecast devices.

### Web UI Setup
1. Open **Config → Integrations → Chromecast**.
2. Enable the integration.
3. Set the **TTS Port**.
4. Save and restart.

### YAML Reference
```yaml
chromecast:
    enabled: true
    tts_port: 8090
```

---

## S3 Storage Integration

Access Amazon S3 or compatible object storage.

### Web UI Setup
1. Open **Config → Integrations → S3**.
2. Enable the integration.
3. Enter the **Endpoint**, **Region**, **Bucket**, and **Access Key**.
4. Store the **Secret Key** in the Vault.
5. Save and restart.

### YAML Reference
```yaml
s3:
    enabled: true
    readonly: false
    endpoint: "s3.amazonaws.com"
    region: "us-east-1"
    bucket: "my-bucket"
    use_path_style: false
    insecure: false
```

> 🔒 Access key and secret key are stored in the Vault as `s3_access_key` and `s3_secret_key`.

---

## SQL Connections Integration

Connect to external SQL databases (PostgreSQL, MySQL/MariaDB, SQLite).

### Web UI Setup
1. Open **Config → Integrations → SQL Connections**.
2. Enable the integration.
3. Add a connection with **Driver**, **Host**, **Port**, and **Database**.
4. Store credentials in the Vault.
5. Adjust `max_result_rows` and timeouts as needed.
6. Save and restart.

> 💡 **Security:** Use dedicated read-only users when possible.

### YAML Reference
```yaml
sql_connections:
    enabled: true
    max_result_rows: 1000
    connections:
        - name: "production"
          driver: "postgres"
          host: "db.example.com"
          port: 5432
          database: "aurago"
          username: "aurago_readonly"
          password_vault_key: "sql_prod_password"
          read_only: true
          max_pool_size: 5
          connect_timeout: 10
          query_timeout: 30
```

---

## OneDrive Integration

Access Microsoft OneDrive files via Microsoft Graph API.

### Web UI Setup
1. Open **Config → Integrations → OneDrive**.
2. Enable the integration.
3. Enter your Azure AD **Client ID** and **Tenant ID**.
4. Start OAuth2 authentication from the Web UI.
5. Save and restart.

### YAML Reference
```yaml
onedrive:
    enabled: true
    client_id: "YOUR_CLIENT_ID"
    tenant_id: "common"
    client_secret_vault_key: "onedrive_client_secret"
    graph_scopes:
        - "Files.Read"
        - "Files.ReadWrite"
    upload_folder: "AuraGo"
```

---

## Homepage Integration

Deploy a personal start-page dashboard.

### Web UI Setup
1. Open **Config → Integrations → Homepage**.
2. Enable the integration.
3. Configure deployment host, user, and target path.
4. Optionally enable the local webserver (`allow_local_server`).
5. Save and restart.

### YAML Reference
```yaml
homepage:
    enabled: true
    deploy_host: "server.example.com"
    deploy_user: "deploy"
    deploy_path: "/var/www/homepage"
    webserver_config_path: "/etc/nginx/sites-available/homepage"
    allow_deploy: true
    allow_local_server: false
```

---

## Cloudflare Tunnel Integration

Expose AuraGo securely via Cloudflare Tunnel.

### Web UI Setup
1. Open **Config → Integrations → Cloudflare Tunnel**.
2. Enable the integration.
3. Enter your **Account ID**, **Tunnel ID**, and **Tunnel Name**.
4. Choose which services to expose (Web UI, Homepage).
5. Save and restart.

AuraGo can manage the `cloudflared` container automatically.

### YAML Reference
```yaml
cloudflare_tunnel:
    enabled: true
    mode: auto
    auth_method: token
    account_id: "YOUR_ACCOUNT_ID"
    tunnel_name: "aurago"
    tunnel_id: "YOUR_TUNNEL_ID"
    expose_web_ui: true
    expose_homepage: false
    loopback_port: 18080
    metrics_port: 0
    log_level: info
```

> 🔒 The connector token is stored in the Vault as `cloudflare_tunnel_token`.

---

## Rocket.Chat Integration

Send messages to Rocket.Chat channels.

### Web UI Setup
1. Open **Config → Integrations → Rocket.Chat**.
2. Enable the integration.
3. Enter the **URL**, **User ID**, and **Channel**.
4. Save and restart.

### YAML Reference
```yaml
rocketchat:
    enabled: true
    url: "https://chat.example.com"
    channel: "general"
```

---

## Fallback LLM Integration

Failover to a secondary LLM provider.

### Web UI Setup
1. Open **Config → Integrations → Fallback LLM**.
2. Enable the integration.
3. Enter the **API Key**, **Base URL**, and **Model**.
4. Save and restart.

### YAML Reference
```yaml
fallback_llm:
    enabled: true
    api_key: "YOUR_API_KEY"
    model: "llama3.1"
```

---

## Co-Agents Integration

Spawn specialist sub-agents.

### Web UI Setup
1. Open **Config → Integrations → Co-Agents**.
2. Enable the integration.
3. Choose which specialists to activate (Researcher, Coder, Designer, Security, Writer).
4. Save and restart.

### YAML Reference
```yaml
co_agents:
    enabled: true
    max_concurrent: 3
```

---

## Mission Preparation Integration

Pre-analyse missions before execution. The Mission Preparation service asks an LLM to create structured run guidance before a scheduled or manual mission starts.

It can generate essential tools, step plans, likely pitfalls, decision points, preload hints, and a confidence score. Prepared guidance is cached by mission checksum and invalidated when the mission changes. When enabled, scheduled missions can be prepared automatically before execution.

### Web UI Setup
1. Open **Config → Integrations → Mission Preparation**.
2. Enable the integration.
3. Set timeout and confidence thresholds.
4. Save and restart.

### YAML Reference
```yaml
mission_preparation:
    enabled: true
    timeout_seconds: 120
    auto_prepare_scheduled: true
    min_confidence: 0.6
```

---

## TTS / Whisper Integration

Text-to-speech and speech-to-text.

### Web UI Setup
1. Open **Config → Integrations → TTS / Whisper**.
2. Enable the desired providers.
3. Enter API keys or configure local Piper options.
4. Save and restart.

### YAML Reference
```yaml
tts:
    provider: google
    language: en
    cache_retention_hours: 24
    cache_max_files: 100
    piper:
        voice: "en_US-lessac-high"
        container_port: 10200
whisper:
    provider: openai
```

---

## Ollama Integration

Manage local LLMs.

### Web UI Setup
1. Open **Config → Integrations → Ollama**.
2. Enable the integration.
3. Enter the **URL** of your Ollama instance.
4. Optionally enable **Managed Instance** to let AuraGo run Ollama in Docker.
5. Save and restart.

### Managed Ollama

When managed mode is enabled, AuraGo controls an `aurago_ollama_managed` Docker container. It can keep model data in a persistent volume, detect available GPU support (NVIDIA, AMD, Intel where supported by Docker), apply memory limits, pull configured default models, and recreate the container from the Web UI or API.

Use `GET /api/ollama/managed/status` to inspect the container and `POST /api/ollama/managed/recreate` to rebuild it after configuration changes.

### YAML Reference
```yaml
ollama:
    enabled: true
    url: "http://localhost:11434"
    managed:
      enabled: true
      image: "ollama/ollama:latest"
      default_models: ["llama3.1"]
```

---

## Media Registry Integration

Register and track media assets.

### Web UI Setup
1. Open **Config → Integrations → Media Registry**.
2. Enable the integration.
3. Save and restart.

### YAML Reference
```yaml
media_registry:
    enabled: true
```

---

## GitHub Integration

Interact with GitHub repositories and webhooks.

### Web UI Setup
1. Open **Config → Integrations → Webhooks** to set up GitHub webhooks.
2. For API access, store a personal access token in the Vault.
3. Save and restart.

### YAML Reference
```yaml
webhooks:
    enabled: true
```

---

## MeshCentral Integration

Remote device management.

### Web UI Setup
1. Open **Config → Integrations → MeshCentral**.
2. Enable the integration.
3. Enter your **Server URL** and credentials.
4. Save and restart.

### YAML Reference
```yaml
meshcentral:
    enabled: true
    url: "https://meshcentral.local"
```

---

## Ansible Integration

Infrastructure automation via Ansible.

### Web UI Setup
1. Open **Config → Integrations → Ansible**.
2. Enable the integration.
3. Enter the **API URL** and **API Token**.
4. Save and restart.

### YAML Reference
```yaml
ansible:
    enabled: true
    api_url: "http://ansible-api:5000"
```

## A2A Protocol Integration

AuraGo supports the Google A2A (Agent-to-Agent) protocol for communication between AI agents. It can publish an Agent Card so other A2A clients know its name, capabilities, endpoints, and authentication requirements. AuraGo can also act as an A2A client and register remote agents for delegated tasks.

A2A is useful when multiple autonomous agents need to exchange tasks without sharing a single chat session. AuraGo supports REST, JSON-RPC, and gRPC bindings where enabled, plus streaming and push notifications.

### Web UI Setup
1. Open **Config → Integrations → A2A**.
2. Enable the server and configure the agent card.
3. Add remote agents for cross-agent collaboration.
4. Save and restart.

### YAML Reference
```yaml
a2a:
  server:
    enabled: true
    port: 0
    base_path: "/a2a"
    agent_name: "AuraGo"
    streaming: true
    push_notifications: true
    bindings:
      rest: true
      json_rpc: true
      grpc: true
      grpc_port: 50051
  client:
    enabled: true
    remote_agents: []
  auth:
    api_key_enabled: true
    bearer_enabled: true
```

The public agent card stays unauthenticated for discovery. All other A2A endpoints require at least one configured auth method; store the API key or bearer secret in the Vault before exposing the server.

## Music Generation Integration

AI music generation via supported providers.

### Web UI Setup
1. Open **Config → Integrations → Music Generation**.
2. Enable the integration and set the provider and daily limits.
3. Save and restart.

### YAML Reference
```yaml
music_generation:
  enabled: true
  provider: ""
  model: ""
  max_daily: 10
```

## LDAP / Active Directory Integration

Query and authenticate against LDAP or Active Directory. The `ldap` tool can search users and groups, retrieve details, list groups, and authenticate credentials. Write operations are blocked unless LDAP read-only mode is disabled.

### Web UI Setup
1. Open **Config → Integrations → LDAP**.
2. Enable the integration.
3. Enter the **Server URL**, **Base DN**, and Bind DN.
4. Store the Bind password in the Vault.
5. Enable TLS or LDAPS where available.
6. Use the test button before allowing non-read-only operations.

### YAML Reference
```yaml
ldap:
  enabled: true
  url: "ldap://ldap.example.com:389"
  base_dn: "dc=example,dc=com"
  bind_dn: "cn=admin,dc=example,dc=com"
  use_tls: true
  insecure_skip_verify: false
  readonly: true
```

---

## Firewall Integration

Linux firewall monitoring and management (iptables/ufw).

### Web UI Setup
1. Open **Config → Integrations → Firewall**.
2. Enable the integration and choose the mode.
3. Save and restart.

### YAML Reference
```yaml
firewall:
  enabled: true
  mode: "readonly"
  poll_interval_seconds: 60
```

## Invasion Control Integration

Remote deployment system for AuraGo worker instances (eggs) across nests.

### Web UI Setup
1. Open **Config → Invasion Control**.
2. Manage nests and deploy eggs.
3. Monitor status and send tasks remotely.

### YAML Reference
```yaml
invasion_control:
  enabled: true
  readonly: false
```

## Document Creator (Gotenberg) Integration

PDF creation and document conversion. Supports the built-in Maroto backend or an external Gotenberg sidecar.

### Web UI Setup
1. Open **Config → Integrations → Document Creator**.
2. Choose the backend (maroto or gotenberg).
3. Configure the output directory and sidecar URL if needed.
4. Save and restart.

### YAML Reference
```yaml
tools:
  document_creator:
    enabled: true
    backend: "maroto"
    output_dir: "data/documents"
    gotenberg:
      url: "http://gotenberg:3000"
      timeout: 120
```

---

## Security Proxy

Protection layer for publicly reachable AuraGo instances with rate limiting, IP filtering, and geo-blocking. AuraGo manages the proxy as a Caddy-based Docker container, reloads the generated configuration, and exposes logs and lifecycle actions via API.

### Web UI Setup
1. Open **Config → Integrations → Security Proxy**.
2. Enable the proxy.
3. Configure rate limiting (requests per minute).
4. Optionally define allowed/blocked IPs or countries.
5. Save and restart.

### YAML Reference
```yaml
security_proxy:
    enabled: true
    domain: "aurago.example.com"
    rate_limiting:
        enabled: true
        requests_per_minute: 60
    ip_filter:
        enabled: false
        allowed_ips: []
        blocked_ips: []
    geo_blocking:
        enabled: false
        blocked_countries: []
```

### Runtime API

| Endpoint | Purpose |
|----------|---------|
| `GET /api/proxy/status` | Current proxy/container status |
| `POST /api/proxy/start` | Start the managed proxy container |
| `POST /api/proxy/stop` | Stop the proxy |
| `POST /api/proxy/destroy` | Remove the managed proxy container |
| `POST /api/proxy/reload` | Regenerate and reload the Caddy configuration |
| `GET /api/proxy/logs` | Fetch recent proxy logs |

---

## Egg Mode (Invasion Control)

Connect multiple AuraGo instances into a distributed nest (cluster). Individual instances are called "eggs".

### Web UI Setup
1. Open **Config → Integrations → Egg Mode**.
2. Enable **Egg Mode**.
3. Enter the **Master URL** of the main instance.
4. Optionally set **Egg ID** and **Nest ID**.
5. Save and restart.

### YAML Reference
```yaml
egg_mode:
    enabled: false
    master_url: "https://master.aurago.local:8088"
    egg_id: "egg-01"
    nest_id: "nest-main"
    api_key_vault_key: "egg_api_key"
```

---

## Testing Integrations

### Health Check Commands
Test individual integrations via API:
```bash
curl http://localhost:8088/api/health/telegram
curl http://localhost:8088/api/health/discord
curl http://localhost:8088/api/health/email
curl http://localhost:8088/api/health/homeassistant
curl http://localhost:8088/api/health/docker
```

### Integration Status in Web UI
The dashboard shows status indicators:
- 🟢 Green: Connected and working
- 🟡 Yellow: Configured but not connected
- 🔴 Red: Error or disabled

### Debugging Integration Issues
1. **Check logs:** `tail -f log/aurago_$(date +%Y-%m-%d).log`
2. **Verify configuration:** `./aurago -validate-config`
3. **Test with verbose output:** `./aurago -debug`

### Common Issues
| Issue | Solution |
|-------|----------|
| Telegram bot not responding | Check `telegram_user_id` matches your account |
| Discord connection fails | Verify bot token and intents are enabled |
| Email authentication fails | Use app password, not regular password |
| Home Assistant 401 error | Regenerate access token |
| Docker permission denied | Add user to docker group or use sudo |
| Webhook not receiving | Check firewall and URL format |

---

## Next Steps

Now that your integrations are set up:
1. **[Security](14-security.md)** – Secure your AuraGo installation
2. **[Advanced Usage](15-coagents.md)** – Workflows, co-agents, and automation
3. **[Troubleshooting](16-troubleshooting.md)** – Solve common issues
