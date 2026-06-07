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

`allowed_user_id` is required for inbound Discord control. Leave it empty to block user messages until a specific Discord user ID is configured.

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

## AgentMail Integration

API-based email inboxes via [AgentMail](https://agentmail.to). Separate from the legacy IMAP/SMTP `email` integration — existing `fetch_email` and `send_email` tools keep their current behavior.

### Web UI Setup
1. Open **Config → Integrations → AgentMail**.
2. Enable the integration.
3. Enter your **Inbox ID** or enable **Auto Create Inbox**.
4. Optionally enable **Relay to Agent** to wake AuraGo on new messages.
5. Store the API key in the Vault (`agentmail_api_key`).
6. Save and restart.

### YAML Reference
```yaml
agentmail:
    enabled: true
    readonly: false
    inbox_id: ""
    auto_create_inbox: false
    username: ""
    domain: ""
    display_name: ""
    use_websocket: true
    poll_interval_seconds: 120
    relay_to_agent: false
    relay_cheatsheet_id: ""
    max_attachment_mb: 10
    base_url: https://api.agentmail.to
    websocket_url: wss://ws.agentmail.to/v0
```

> 🔒 The API key is stored in the Vault as `agentmail_api_key`, not in `config.yaml`.

Use the `agentmail` tool in chat for inbox management, messages, drafts, labels, and replies.

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

`allowed_services` is an optional allowlist for `call_service`; leave it empty to allow all services unless they are listed in `blocked_services`.

---

## MQTT Integration

Connect to an MQTT broker for IoT devices and smart-home automation. AuraGo can subscribe to topics, buffer messages, relay events to the agent, and publish availability status via Last Will and Testament (LWT).

### Web UI Setup
1. Open **Config → Integrations → MQTT**.
2. Enable the integration.
3. Enter the **Broker URL** (e.g., `tcp://localhost:1883` or `mqtts://broker:8883`).
4. Add **Topics** to subscribe to.
5. Optionally enable **Relay to Agent** or **Availability** publishing.
6. Store credentials in the Vault if needed.
7. Save and restart.

### YAML Reference
```yaml
mqtt:
    enabled: true
    broker: "tcp://localhost:1883"
    client_id: aurago
    username: ""
    topics:
      - "home/+/sensors"
    qos: 0
    relay_to_agent: false
    connect_timeout: 15
    clean_session: true
    trigger_min_interval_seconds: 0
    tls:
        enabled: false
        ca_file: ""
        cert_file: ""
        key_file: ""
        insecure_skip_verify: false
    buffer:
        max_messages: 500
        max_age_hours: 0
        max_payload_bytes: 262144
    availability:
        enabled: false
        topic: aurago/status
        online_payload: online
        offline_payload: offline
        qos: 1
        retain: true
```

`trigger_min_interval_seconds` limits how often MQTT events can start missions (0 = disabled).

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

## Package Manager Integration

Structured OS package management via the `package_manager` tool. Supports apt, dnf, yum, pacman, zypper, apk, brew, winget, choco, and scoop.

> ⚠️ **Security:** Package management grants significant system access. Enable only when needed and prefer `readonly` for monitoring.

### Requirements

Both toggles must be enabled:

1. `package_manager.enabled: true`
2. `agent.allow_package_manager: true`

### Web UI Setup
1. Open **Config → Integrations → Package Manager**.
2. Enable the integration.
3. Configure **Read-only**, **Auto Detect**, and operation permissions (install/remove/upgrade).
4. Open **Config → Agent** and enable **Allow Package Manager**.
5. Save and restart.

### YAML Reference
```yaml
package_manager:
    enabled: true
    readonly: false
    auto_detect: true
    override: ""
    allow_install: true
    allow_remove: true
    allow_upgrade: true

agent:
    allow_package_manager: true
```

`override` forces a specific manager (e.g., `apt`, `brew`, `winget`); leave empty for auto-detection from PATH.

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
3. Add local stdio commands or network server URLs, optional headers, and optional per-server tool limits.
4. Use **Test connection** to run initialize and tool discovery before saving.
5. Save and restart.

### MCP Client
Allows the agent to use tools from external MCP servers.

```yaml
mcp:
    enabled: true
    servers:
        - name: "fetch-server"
          transport: stdio
          command: "uvx"
          args: ["mcp-server-fetch"]
          allowed_tools: []  # optional allowlist; empty means all discovered non-destructive tools
          allow_destructive: false

        - name: "remote-tools"
          transport: streamable_http # stdio | streamable_http | sse | websocket
          url: "https://example.com/mcp"
          headers:
              Authorization: "Bearer {{remote-mcp-token}}"
          allowed_tools: []
          allow_destructive: false
```

When `transport` is omitted, AuraGo keeps the old behavior and starts the server as a local stdio process. Network transports require a URL; header values can reference MCP vault secrets with `{{alias}}`.

For MCP clients, `allowed_tools` is optional per server. Leave it empty or omit it to allow all discovered non-destructive tools; add tool names only when you want to restrict execution and routing to that subset.

### MCP Server
Exposes AuraGo tools to external clients.

```yaml
mcp_server:
    enabled: true
    require_auth: true
    allowed_tools:
        - "execute_shell"
        - "filesystem"
    vscode_debug_bridge: false
```

> The MCP server shares the main HTTP server — there is no separate `port` setting. MCP client access also requires `agent.allow_mcp: true`.

`allowed_tools` is an explicit server-side allowlist. Leave it empty to expose no AuraGo tools; `vscode_debug_bridge` applies its own limited debugging preset.

---

## Composio Integration

Connect AuraGo to [Composio](https://composio.dev) toolkits (GitHub, Slack, Gmail, and hundreds more) via the native `composio_call` tool.

### Web UI Setup
1. Sign up at [Composio](https://composio.dev) and create an API key.
2. Open **Config → Integrations → Composio**.
3. Enable the integration.
4. Set **User ID** and configure toolkit policies.
5. Store the API key in the Vault (`composio_api_key`).
6. Connect accounts in the Composio dashboard for the configured toolkits.
7. Save and restart.

### YAML Reference
```yaml
composio:
    enabled: true
    base_url: https://backend.composio.dev/api/v3.1
    user_id: aurago-default
    read_only: true
    allow_destructive: false
    allow_natural_language_input: false
    request_timeout_seconds: 60
    cache_ttl_seconds: 300
    max_result_bytes: 262144
    toolkits: []
    # - slug: github
    #   enabled: true
    #   read_only: true
    #   allow_destructive: false
    #   allowed_tool_slugs: []
    #   blocked_tool_slugs: []
```

> 🔒 The API key is stored in the Vault as `composio_api_key`, not in `config.yaml`.

By default, `read_only: true` blocks mutating actions. Set `allow_destructive: true` only when delete/remove/revoke operations are explicitly required. Per-toolkit `allowed_tool_slugs` and `blocked_tool_slugs` provide fine-grained control.

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

### AgoDesk / AgoChat Desktop Companion

AuraGo can pair with the **AgoDesk** desktop client over WebSocket. When a device is connected, the agent can send proactive messages via `send_agodesk_chat` and execute remote desktop commands through the AgoDesk protocol.

**Prerequisites:**
- `remote_control.enabled: true`
- AgoDesk client connected and paired (`/api/agodesk/ws`)

**Setup:**
1. Install and open the AgoDesk desktop client.
2. Pair with AuraGo (one-time `pairing_token` or stored `device_id` + `shared_key_proof`).
3. The connected device appears in the agent context as a **REACHABLE CHAT CHANNEL** with its `device_id`.

**Agent tool:** `send_agodesk_chat` — send proactive text to a paired device.

```json
{
  "action": "send_agodesk_chat",
  "device_id": "dev-abc123",
  "message": "Your backup finished successfully."
}
```

> 📖 Full protocol reference: [`documentation/agodesk_backend_protocol.md`](../../agodesk_backend_protocol.md) (pairing flow, capabilities, desktop commands).

**API:** `GET /api/remote/devices` lists connected RemoteHub/AgoDesk devices.

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

## Python Tool Bridge

Allows **Python skills** to call selected native AuraGo tools over an internal HTTP bridge (`POST /api/internal/tool-bridge/`). Disabled by default for security.

### Web UI Setup
1. Open **Config → Tools → Python Tool Bridge**.
2. Enable the bridge.
3. Add tool names to the **allowed_tools** whitelist (empty = no tools callable).
4. Optionally allow named SQL connections for `sql_query` via the bridge.
5. Save and restart.

### YAML Reference
```yaml
tools:
    python_tool_bridge:
        enabled: false
        allowed_tools: []              # explicit whitelist required, e.g. ["api_request", "sql_query"]
        allowed_sql_connections: []    # SQL connection names; empty = block SQL bridge calls
```

Skills declare bridge usage in their manifest via `internal_tools`. The agent does not call the bridge directly — skills invoke it at runtime.

> ⚠️ **Security:** Only whitelist tools a skill truly needs. Never add `get_secret`, `execute_shell`, or vault tools unless you fully trust the skill code.

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

## Daemon Skills

Long-running Python skills that stay active in the background and can wake the agent on events (e.g. file watchers, polling loops). Managed via the `manage_daemon` tool and the dashboard **Daemon Skills** card.

### Web UI Setup
1. Open **Config → Tools → Daemon Skills**.
2. Enable daemon skills (opt-in, disabled by default).
3. Configure concurrency and wake-up cost limits.
4. Upload or enable a skill with `daemon: true` in its manifest.
5. Save and restart.

### YAML Reference
```yaml
tools:
    daemon_skills:
        enabled: false
        max_concurrent_daemons: 5
        global_rate_limit_secs: 60
        max_wakeups_per_hour: 6
        max_budget_per_hour: 0.50
```

### Agent Tool: `manage_daemon`

| Operation | Description |
|-----------|-------------|
| `list` | List all running daemons |
| `status` | Get status for a specific daemon (`skill_id` required) |
| `start` / `stop` | Start or stop a daemon by skill ID |
| `reenable` | Re-enable an auto-disabled daemon |
| `refresh` | Rescan skills from disk and reconcile running daemons |

> ⚠️ **Cost control:** `max_wakeups_per_hour` and `max_budget_per_hour` act as circuit breakers to prevent runaway LLM costs from frequent daemon wake-ups.

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

Manage GitHub repositories, issues, pull requests, branches, files, commits, and workflow runs via the native `github` tool. Incoming GitHub webhooks are configured separately.

### Web UI Setup
1. Open **Config → Integrations → GitHub**.
2. Enable the integration and enter the default **owner** (username or organisation).
3. For GitHub Enterprise, set **base_url** (e.g. `https://github.example.com/api/v3`).
4. Store a personal access token in the Vault (`github_token`).
5. Optionally enable **read-only** mode to block create/update/delete operations.
6. For incoming webhooks, open **Config → Integrations → Webhooks**.
7. Save and restart.

### YAML Reference
```yaml
github:
    enabled: false
    readonly: false
    owner: ""
    default_private: false
    base_url: ""                  # GitHub Enterprise API base URL (optional)
```

### Agent Tool: `github`

| Operation | Description |
|-----------|-------------|
| `list_repos`, `search_repos` | List or search repositories |
| `create_repo`, `delete_repo`, `get_repo` | Repository lifecycle |
| `list_issues`, `create_issue`, `close_issue` | Issue management |
| `list_pull_requests`, `list_branches` | PR and branch listing |
| `get_file`, `create_or_update_file`, `list_commits` | File and commit access |
| `list_workflow_runs` | CI/CD workflow runs |
| `list_projects`, `track_project`, `untrack_project` | Local project tracking |

> 💡 **Vault:** Store the token as `github_token`. Never put API tokens in `config.yaml`.

### Webhooks (separate)

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

## YepAPI Integration

Unified API for SEO, SERP, web scraping, and social media data (YouTube, TikTok, Instagram, Amazon).

### Web UI Setup
1. Open **Config → Integrations → YepAPI**.
2. Enable the integration.
3. Store the API key in the Vault (key: `yepapi_api_key`).
4. Configure per-service enable/disable as needed.
5. Save and restart.

### Capabilities

| Service | Capabilities |
|---------|--------------|
| **SEO** | Keyword research, domain overview, competitor analysis, backlinks, on-page analysis, trends |
| **SERP** | Google/Bing/Yahoo/Baidu search, Maps, Images, News, YouTube, Autocomplete |
| **Scraping** | Standard, JavaScript-rendered, stealth, screenshots, AI extraction |
| **YouTube** | Video search, transcripts, comments, channels, playlists, shorts |
| **TikTok** | Video/user search, profiles, posts, comments, music, challenges |
| **Instagram** | User search, profiles, posts, reels, stories, comments, hashtags |
| **Amazon** | Product search, ASIN lookup, reviews, deals, best sellers |

### YAML Reference
```yaml
yepapi:
    enabled: true
    seo:
        enabled: true
    serp:
        enabled: true
    scraping:
        enabled: true
    youtube:
        enabled: true
    tiktok:
        enabled: true
    instagram:
        enabled: true
    amazon:
        enabled: true
```

---

## Inventory System

Device registry with SSH credential management and Wake-on-LAN support.

### Web UI Setup
1. Open **Config → Integrations → Inventory**.
2. Enable the integration.
3. Configure `inventory_path` if using a custom database location.
4. Enable **Wake-on-LAN** if needed.
5. Save and restart.

### YAML Reference
```yaml
sqlite:
    inventory_path: ./data/inventory.db

tools:
    inventory:
        enabled: true
    wol:
        enabled: true
```

### Key Features
- **Device Registry**: Store devices (servers, VMs, Docker, network devices) with IP, port, SSH credentials
- **Wake-on-LAN**: Send magic packets to wake devices via stored MAC addresses
- **Credential Security**: SSH passwords and private keys stored in encrypted Vault
- **Tag-based Organization**: Group and search devices by tags

Use `register_device`, `query_inventory`, and `wake_on_lan` tools in chat.

---

## Heartbeat System

Background wake-up scheduler for autonomous status checks at configurable intervals.

### Web UI Setup
1. Open **Config → Integrations → Heartbeat**.
2. Enable the integration.
3. Configure **Day Window** (default: 08:00–22:00, every 1h) and **Night Window** (default: 22:00–08:00, every 4h).
4. Toggle what to check: Tasks, Appointments, Emails.
5. Optionally add a custom prompt for heartbeat wake-ups.
6. Save and restart.

### YAML Reference
```yaml
heartbeat:
    enabled: true
    check_tasks: true
    check_appointments: true
    check_emails: true
    additional_prompt: "Alert me only for critical issues."
    day_time_window:
        start: "08:00"
        end: "22:00"
        interval: "1h"
    night_time_window:
        start: "22:00"
        end: "08:00"
        interval: "4h"
```

### Key Features
- **Time-aware guidance**: Different wake-up priorities based on time of day (morning check, midday review, evening summary, night quiet mode)
- **Overlap protection**: Prevents concurrent heartbeat executions
- **State persistence**: Persists last-run time to survive process restarts
- **Custom prompts**: Append user-defined instructions to every heartbeat wake-up

---

## Knowledge Graph Extraction

LLM-based entity and relationship extraction from conversations and files.

### Web UI Setup
1. Open **Config → Integrations → Knowledge Graph**.
2. Enable the integration.
3. Configure **Auto Extraction** for nightly batch entity extraction.
4. Toggle **Prompt Injection** to include KG context in system prompts.
5. Set limits for prompt node count and character count.
6. Save and restart.

### YAML Reference
```yaml
tools:
    knowledge_graph:
        enabled: true
        readonly: false
        auto_extraction: true
        prompt_injection: true
        max_prompt_nodes: 5
        max_prompt_chars: 800
        retrieval_fusion: true
```

### Key Features
- **Entity Types**: person, device, service, software, location, project, concept, event
- **Relations**: runs_on, owns, manages, uses, depends_on, connected_to, related_to, part_of, deployed_on, located_in
- **Confidence Scoring**: Heuristic quality scoring (0.0–1.0) per extraction
- **Cross-enrichment**: RAG ↔ Knowledge Graph bidirectional fusion

---

## Obsidian Integration

Connect AuraGo to your Obsidian vault for notes and knowledge management.

### Web UI Setup
1. Open **Config → Integrations → Obsidian**.
2. Enable the integration.
3. Enter the **Local REST API** host/port and store `obsidian_api_key` in the Vault.
4. Save and restart.

### YAML Reference
```yaml
obsidian:
    enabled: true
    host: "127.0.0.1"
    port: 27124
    use_https: true
    readonly: false
```

---

## Uptime Kuma Integration

Monitor service availability with Uptime Kuma.

### Web UI Setup
1. Open **Config → Integrations → Uptime Kuma**.
2. Enable the integration.
3. Enter the **Base URL**.
4. Store the API key in the Vault.
5. Save and restart.

### YAML Reference
```yaml
uptime_kuma:
    enabled: true
    base_url: "https://uptime-kuma.example.com:3001"
```

---

## Vercel Integration

Deploy web projects directly to Vercel.

### Web UI Setup
1. Open **Config → Integrations → Vercel**.
2. Enable the integration.
3. Enter the **Team Slug**.
4. Store the API token in the Vault.
5. Save and restart.

### YAML Reference
```yaml
vercel:
    enabled: true
    team_slug: "my-team"
```

---

## Browser Automation

Headless browser automation for forms, screenshots, and web interactions.

### Web UI Setup
1. Open **Config → Integrations → Browser Automation**.
2. Enable the integration.
3. Configure **Headless Mode** and screenshot directory.
4. Save and restart.

The Browser Automation sidecar requires `AURAGO_BROWSER_AUTOMATION_TOKEN` by default. AuraGo injects it automatically for managed sidecars; set it explicitly when running the sidecar manually. Use `AURAGO_BROWSER_AUTOMATION_ALLOW_UNAUTH=1` only for isolated local development.

### YAML Reference
```yaml
browser_automation:
    enabled: true
    headless: true
    screenshots_dir: "browser_screenshots"
```

---

## Output Compression

Reduces token consumption by filtering and deduplicating tool outputs before they enter the LLM context.

### Web UI Setup
1. Open **Config → Agent → Output Compression**.
2. Enable the integration.
3. Adjust thresholds (minimum characters, preserve errors).
4. Toggle compression for Shell, Python, API outputs, and advanced filters.
5. Save and restart.

### Advanced Modes

| Mode | Default | Use case |
|------|---------|----------|
| `repetitive_substitution` | disabled | Replaces repeated long phrases in log-like output with a small dictionary. It skips errors, diffs, code/source reads, JSON documents, and exact-copy-sensitive tools. |
| `toon_json` | disabled | Converts known homogeneous API arrays to a compact TOON-style representation when it saves enough tokens. It skips `api_request` and file-read outputs. |

### YAML Reference
```yaml
agent:
    output_compression:
        enabled: true
        min_chars: 500
        preserve_errors: true
        shell_compression: true
        python_compression: true
        api_compression: true
        repetitive_substitution:
            enabled: false
            lzw_enabled: true
            ltsc_lite_enabled: false
            min_phrase_chars: 15
            min_occurrences: 3
            min_savings_percent: 15
            max_input_chars: 50000
            max_dictionary_entries: 16
        toon_json:
            enabled: false
            min_savings_percent: 10
            max_rows: 200
```

---

## 3D Printer Integration

Monitor and control 3D printers via the `three_d_printer` tool. Supports **Elegoo Centauri Carbon** (SDCP WebSocket) and **Klipper/Moonraker** (HTTP API).

### Web UI Setup
1. Open **Config → Integrations → 3D Printers**.
2. Enable the integration.
3. Add printers under **Elegoo Centauri Carbon** or **Klipper** with `id`, `name`, and `url`.
4. Set `readonly: true` for monitoring-only access (blocks start/pause/cancel).
5. Test: `GET /api/3d-printers/test`.

### YAML Reference
```yaml
three_d_printers:
    enabled: false
    readonly: true
    default_printer: ""
    elegoo_centauri_carbon:
        enabled: false
        printers:
          - id: "lab-printer"
            name: "Elegoo Centauri Carbon"
            url: "ws://192.168.1.50/websocket"
            mainboard_id: ""
            timeout_seconds: 10
    klipper:
        enabled: false
        printers:
          - id: "voron"
            name: "Voron 2.4"
            url: "http://192.168.1.60:7125"
            api_key: ""
            timeout_seconds: 10
            webcam_name: ""
```

**Camera APIs:**
- `GET /api/3d-printers/{printer_id}/camera/snapshot`
- `GET /api/3d-printers/{printer_id}/camera/stream`

**Agent tool:** `three_d_printer` — operations: `list_printers`, `status`, `files`, `camera_snapshot`, `show_live_stream`, `start_print`, `pause_print`, `resume_print`, `cancel_print` (write ops require `readonly: false`).

---

## Frigate Integration

Video surveillance and NVR management via Frigate.

**Web UI:** Config → Integrations → Frigate → Enter URL and API token. Optional: Enable event relay, review relay, and media storage.

### YAML Reference
```yaml
frigate:
  enabled: true
  readonly: false
  url: "https://frigate.local:8971"
  internal_port: false
  insecure: false
  default_camera: ""
  event_relay: false
  review_relay: false
  store_media: false
  mqtt_topic_prefix: "frigate"
```

## Grafana Integration

Monitoring and observability via Grafana.

**Web UI:** Config → Integrations → Grafana → Enter base URL. Store API key in the Vault.

### YAML Reference
```yaml
grafana:
  enabled: true
  base_url: "http://grafana.local:3000"
  readonly: false
  insecure_ssl: false
  request_timeout: 30
```

## Manifest Integration

[Manifest](https://manifest.build) is an OpenAI-compatible LLM gateway with a managed dashboard. AuraGo can run Manifest as a **managed Docker sidecar** (Manifest + Postgres) or connect to an external hosted instance.

### Web UI Setup
1. Open **Config → Integrations → Manifest**.
2. Enable the integration and choose **managed** or **external** mode.
3. For managed mode: AuraGo starts `manifestdotbuild/manifest` and a Postgres container automatically.
4. Store secrets in the Vault (never in `config.yaml`): `manifest_api_key`, `manifest_postgres_password`, `manifest_better_auth_secret`.
5. Test via **Config → Integrations → Manifest → Test Connection** (`GET /api/manifest/test`).

### Provider routing
Add a provider entry with `type: manifest` to route LLM calls through Manifest:

```yaml
providers:
  - id: manifest-main
    type: manifest
    name: Manifest Gateway
    base_url: http://127.0.0.1:2099    # managed local URL
    api_key: ""                         # vault: manifest_api_key
    model: gpt-4o
```

### YAML Reference
```yaml
manifest:
    enabled: false
    auto_start: true
    mode: managed                       # managed | external
    url: "http://127.0.0.1:2099"        # managed dashboard/API URL
    external_base_url: "https://app.manifest.build/v1"
    host: "127.0.0.1"
    port: 2099
    host_port: 2099
    image: manifestdotbuild/manifest:5
    container_name: aurago_manifest
    network_name: aurago_manifest
    postgres_container_name: aurago_manifest_postgres
    postgres_image: postgres:15-alpine
    postgres_user: manifest
    postgres_database: manifest
```

Manifest also appears in the Virtual Desktop **Software Store** catalog. Tailscale exposure: `tailscale.tsnet.expose_manifest`.

---

## Dograh Integration

[Dograh](https://dograh.com) is a voice/telephony AI platform. AuraGo can deploy a **managed multi-container stack** (API, UI, Postgres, Redis, MinIO, optional coturn) and bridge it via MCP — there is no native `dograh` agent tool.

### Web UI Setup
1. Open **Config → Integrations → Dograh**.
2. Enable the integration (`dograh.enabled: true`).
3. AuraGo starts the managed stack when `auto_start: true`.
4. Store API keys in the Vault: `dograh_api_key`, `dograh_super_api_key`, `dograh_encryption_key`, `dograh_postgres_password`, `dograh_minio_secret_key`.
5. Test: `GET /api/dograh/test`, status: `GET /api/dograh/status`.

### MCP bridge
Dograh exposes MCP tools to AuraGo via the standard `mcp_call` tool when configured as an MCP server/client bridge. AuraGo can also accept inbound Dograh MCP connections on `/mcp` when `mcp_server.enabled: true`.

### YAML Reference
```yaml
dograh:
    enabled: false
    auto_start: true
    mode: managed
    readonly: true                      # blocks resource mutations from AuraGo helpers
    allow_test_calls: false
    api_url: "http://127.0.0.1:8000"
    ui_url: "http://127.0.0.1:3010"
    api_port: 8000
    ui_port: 3010
    telemetry_enabled: false
    turn_enabled: false                 # optional coturn sidecar for WebRTC
```

> 📖 See also `prompts/tools_manuals/manifest.md` for Manifest sidecar details.

---

## Space Agent Integration

Managed Docker sidecar for the Space Agent — a standalone AuraGo instance for isolated tasks.

**Web UI:** Config → Integrations → Space Agent → Configure repository URL, host, port, and HTTPS.

### YAML Reference
```yaml
space_agent:
  enabled: true
  auto_start: true
  repo_url: ""
  git_ref: ""
  container_name: ""
  image: ""
  host: ""
  port: 0
  https_enabled: false
  https_port: 0
  customware_path: ""
  data_path: ""
  admin_user: ""
  public_url: ""
```

## Virtual Desktop Integration

Workspace-backed browser desktop for local apps, generated apps, file work, Code Studio, and managed Docker software.

**Web UI:** Config → Integrations → Virtual Desktop → Enable the workspace, configure agent control, and adjust Code Studio limits.

### Related Tool Toggles

The `virtual_desktop` tool is exposed only when both `tools.virtual_desktop.enabled` and `virtual_desktop.allow_agent_control` are true. Office document/workbook tools also require the virtual desktop to be enabled.

### Desktop Software Store

The Software Store installs AuraGo-managed Docker apps into the virtual desktop environment. Current catalog entries include Node-RED, Dozzle, code-server, Beszel, RomM, OliveTin, Manifest, and Termix. Termix starts with a `guacd` companion container so its Web UI can manage SSH, RDP, VNC, and Telnet sessions.

### YAML Reference
```yaml
tools:
  virtual_desktop:
    enabled: false
  office_document:
    enabled: false
    readonly: false
  office_workbook:
    enabled: false
    readonly: false

virtual_desktop:
  enabled: false
  readonly: false
  allow_agent_control: false
  allow_generated_apps: true
  allow_python_jobs: false
  workspace_dir: agent_workspace/virtual_desktop
  max_file_size_mb: 50
  control_level: confirm_destructive
  max_ws_clients: 8
  code_studio:
    enabled: true
    image: ghcr.io/antibyte/aurago-code-studio:latest
    auto_start: false
    auto_stop_minutes: 30
    max_memory_mb: 4096
    max_cpu_cores: 2
```

## Shell Sandbox Integration

Linux Landlock-based sandbox for shell commands. Restricts filesystem access, CPU time, and memory for shell operations.

**Web UI:** Config → Integrations → Shell Sandbox → Enable and configure limits.

> 💡 Linux only. On failure, an unsafe fallback can be allowed via `allow_unsafe_fallback`.

### YAML Reference
```yaml
shell_sandbox:
  enabled: false
  allow_unsafe_fallback: false
  max_memory_mb: 1024
  max_cpu_seconds: 30
  max_processes: 50
  max_file_size_mb: 100
  allowed_paths:
    - path: "/tmp"
      readonly: false
```

## Media Conversion Integration

Media conversion via FFmpeg and ImageMagick. Converts audio, video, and image files between formats.

**Web UI:** Config → Integrations → Media Conversion → Configure paths to FFmpeg/ImageMagick.

### YAML Reference
```yaml
tools:
  media_conversion:
    enabled: true
    readonly: false
    ffmpeg_path: ""
    imagemagick_path: ""
    timeout_seconds: 0
    max_file_size_mb: 0
```

## Video Download Integration

Video download via yt-dlp (YouTube and other platforms). Supports Docker and native mode.

**Web UI:** Config → Integrations → Video Download → Configure mode (docker/native) and download directory.

### YAML Reference
```yaml
tools:
  video_download:
    enabled: true
    readonly: false
    allow_download: true
    allow_transcribe: false
    mode: "docker"
    yt_dlp_path: ""
    download_dir: ""
    max_file_size_mb: 0
    timeout_seconds: 0
    default_format: ""
    max_search_results: 0
    container_image: ""
    auto_pull: false
```

## Send YouTube Video Integration

Enables sending YouTube videos as embedded players in chat messages.

**Web UI:** Config → Integrations → Send YouTube Video → Enable.

### YAML Reference
```yaml
tools:
  send_youtube_video:
    enabled: true
```

## GolangciLint Integration

Code quality checks for Go code via golangci-lint.

**Web UI:** Config → Integrations → GolangciLint → Enable.

### YAML Reference
```yaml
golangci_lint:
  enabled: true
```

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
