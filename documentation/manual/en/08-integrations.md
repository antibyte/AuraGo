# Chapter 8: Integrations

AuraGo connects with numerous external services to extend its capabilities. This chapter covers all available integrations and their configuration.

## Table of Contents

1. [Integration Overview](#integration-overview)
2. [Telegram Bot Setup](#telegram-bot-setup)
3. [Discord Bot Setup](#discord-bot-setup)
4. [Email Configuration](#email-configuration)
5. [Home Assistant Integration](#home-assistant-integration)
6. [Docker Integration](#docker-integration)
7. [Webhooks](#webhooks)
8. [Budget Tracking](#budget-tracking)
9. [Google Workspace](#google-workspace)
10. [WebDAV/Koofr Setup](#webdavkoofr-setup)
11. [Additional Integrations Coverage](#additional-integrations-coverage-beyond-core-setup)
12. [Testing Integrations](#testing-integrations)

---

## Integration Overview

AuraGo supports multiple communication channels and service integrations:

### Communication Interfaces

| Integration | Type | Features |
|-------------|------|----------|
| **Web UI** | Built-in | Full-featured chat interface with dashboard |
| **Telegram** | Bot | Text, voice messages, images, inline commands |
| **Discord** | Bot | Server integration, channel management |
| **Email** | IMAP/SMTP | Email reading and sending |
| **Rocket.Chat** | Bot | Enterprise chat integration |

### Service Integrations

| Integration | Purpose |
|-------------|---------|
| **Home Assistant** | Smart home control |
| **Docker** | Container management |
| **Proxmox** | VM and container management |
| **Google Workspace** | Gmail, Calendar, Drive, Docs |
| **WebDAV/Koofr** | Cloud storage access |
| **MeshCentral** | Remote device management |
| **Ollama** | Local LLM management |
| **Tailscale** | VPN network management |
| **Ansible** | Infrastructure automation |

### Monitoring & Notifications

| Integration | Purpose |
|-------------|---------|
| **Webhooks** | Incoming HTTP events |
| **Budget** | Cost tracking and limits |
| **Ntfy** | Push notifications |
| **Pushover** | Mobile push notifications |

---

## Telegram Bot Setup

Telegram is the recommended mobile interface for AuraGo, supporting text, voice, and images.

### Step 1: Create a Bot with BotFather

1. Open Telegram and search for **@BotFather** (verified with blue checkmark)
2. Start a chat and send `/newbot`
3. Follow the prompts:
   - **Name:** Your bot's display name (e.g., "AuraGo Assistant")
   - **Username:** Unique username ending in "bot" (e.g., "aurago_assistant_bot")
4. Copy the **HTTP API Token** provided (format: `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`)

### Step 2: Get Your User ID

AuraGo only responds to authorized users. Find your Telegram user ID:

**Option A: Via @userinfobot**
1. Search for **@userinfobot** in Telegram
2. Start the bot - it will reply with your ID

**Option B: Silent Discovery (AuraGo feature)**
1. Set `telegram_user_id: 0` in config
2. Start AuraGo
3. Message your bot
4. Check AuraGo logs for your printed ID
5. Update config with the ID

### Step 3: Configure AuraGo

```yaml
telegram:
    bot_token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
    telegram_user_id: 123456789
    max_concurrent_workers: 5
```

| Setting | Description |
|---------|-------------|
| `bot_token` | Your bot's API token from BotFather |
| `telegram_user_id` | Your numeric Telegram user ID |
| `max_concurrent_workers` | Parallel request handling (default: 5) |

### Step 4: Test the Connection

1. Restart AuraGo
2. Send `/start` to your bot
3. You should receive a welcome message

> 💡 **Tip:** The bot supports:
> - Text messages with full conversation context
> - Voice messages (automatic transcription)
> - Images (vision analysis)
> - Inline commands (`/status`, `/memory`, `/tools`)

> ⚠️ **Security Warning:** Never share your bot token. Anyone with the token can control your bot and access your AuraGo instance.

---

## Discord Bot Setup

Discord integration allows AuraGo to participate in servers and respond to mentions.

### Step 1: Create a Discord Application

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Click **New Application**
3. Name it (e.g., "AuraGo Bot")
4. Go to **Bot** section in the left sidebar
5. Click **Add Bot**
6. Under **Privileged Gateway Intents**, enable:
   - **Message Content Intent** (required to read messages)

### Step 2: Get Your Bot Token

1. In the Bot section, click **Reset Token**
2. Copy the token (format: long string of letters and numbers)
3. **Never share this token**

### Step 3: Invite Bot to Server

1. Go to **OAuth2** → **URL Generator**
2. Select scopes:
   - `bot`
   - `applications.commands`
3. Select bot permissions:
   - Send Messages
   - Read Messages/View Channels
   - Read Message History
   - Attach Files
   - Use Slash Commands
4. Copy the generated URL and open it in a browser
5. Select your server and authorize

### Step 4: Get Required IDs

**Guild ID (Server ID):**
1. Enable Developer Mode: User Settings → Advanced → Developer Mode
2. Right-click your server name → Copy Server ID

**Channel ID:**
1. Right-click a channel → Copy Channel ID

**Your User ID:**
1. Right-click your username → Copy User ID

### Step 5: Configure AuraGo

```yaml
discord:
    enabled: true
    bot_token: "YOUR_BOT_TOKEN_HERE"
    guild_id: "123456789012345678"
    allowed_user_id: "987654321098765432"
    default_channel_id: "123456789012345678"
    readonly: false
```

| Setting | Description |
|---------|-------------|
| `enabled` | Enable Discord integration |
| `bot_token` | Bot token from Developer Portal |
| `guild_id` | Server ID where bot operates |
| `allowed_user_id` | Only respond to this user (security) |
| `default_channel_id` | Default channel for notifications |
| `readonly` | If true, only read messages, never send |

### Step 6: Test

1. Restart AuraGo
2. In Discord, mention your bot: `@AuraGo Bot hello`
3. The bot should respond in the channel

> 💡 **Tip:** Use `readonly: true` to create a monitoring-only bot that observes but never responds.

---

## Email Configuration

AuraGo can read emails via IMAP and send via SMTP, enabling email-based workflows.

### Gmail Setup

```yaml
email:
    enabled: true
    imap_host: imap.gmail.com
    imap_port: 993
    smtp_host: smtp.gmail.com
    smtp_port: 587
    username: "your.email@gmail.com"
    password: "your-app-password"  # Not your regular password!
    from_address: "your.email@gmail.com"
    watch_enabled: true
    watch_interval_seconds: 120
    watch_folder: INBOX
```

### Creating a Gmail App Password

1. Enable 2-Factor Authentication on your Google account
2. Go to [Google Account Security](https://myaccount.google.com/security)
3. Click **2-Step Verification**
4. Scroll to **App passwords**
5. Select **Mail** and your device
6. Copy the 16-character password

### Other Email Providers

| Provider | IMAP Host | SMTP Host | Notes |
|----------|-----------|-----------|-------|
| **Outlook** | outlook.office365.com | smtp.office365.com | Use app password |
| **Yahoo** | imap.mail.yahoo.com | smtp.mail.yahoo.com | Generate app password |
| **ProtonMail** | 127.0.0.1 | 127.0.0.1 | Requires ProtonMail Bridge |
| **Self-hosted** | Your server | Your server | Standard ports |

### Multi-Account Support (Modern Config)

```yaml
email_accounts:
  - id: personal
    name: "Personal Gmail"
    imap_host: imap.gmail.com
    imap_port: 993
    smtp_host: smtp.gmail.com
    smtp_port: 587
    username: "personal@gmail.com"
    from_address: "personal@gmail.com"
    watch_enabled: true
    watch_interval_seconds: 60
    
  - id: work
    name: "Work Outlook"
    imap_host: outlook.office365.com
    imap_port: 993
    smtp_host: smtp.office365.com
    smtp_port: 587
    username: "name@company.com"
    from_address: "name@company.com"
    watch_enabled: false
```

### Email Watching

When `watch_enabled: true`, AuraGo periodically checks for new emails and can:
- Notify you of important messages
- Summarize email threads
- Trigger actions based on email content

> ⚠️ **Warning:** Storing email passwords in config files is less secure. For production, use environment variables or the vault system.

---

## Home Assistant Integration

Control your smart home devices through AuraGo with Home Assistant integration.

### Setup

1. In Home Assistant:
   - Go to **Profile** (bottom left)
   - Scroll to **Long-Lived Access Tokens**
   - Click **Create Token**
   - Name it "AuraGo" and copy the token

2. Find your Home Assistant URL:
   - Local: `http://homeassistant.local:8123` or `http://192.168.1.100:8123`
   - Remote: Your Nabu Casa URL or reverse proxy URL

3. Configure AuraGo:

```yaml
home_assistant:
    enabled: true
    url: http://homeassistant.local:8123
    access_token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9..."
    readonly: false
```

### Available Actions

| Action | Example Command |
|--------|-----------------|
| List entities | "What devices do I have?" |
| Get state | "Is the living room light on?" |
| Control devices | "Turn off all lights" |
| Read sensors | "What's the temperature in the bedroom?" |
| Call services | "Start the vacuum cleaner" |

> 💡 **Tip:** Use `readonly: true` for monitoring-only access without device control.

> ⚠️ **Security:** Store your access token in the vault for better security:
> ```
> vault set home_assistant_token "eyJ0eXAiOiJKV1Qi..."
> ```

---

## Docker Integration

AuraGo can manage Docker containers, images, and networks.

### Configuration

```yaml
docker:
    enabled: true
    host: unix:///var/run/docker.sock
    readonly: false
```

### Connection Methods

| Host Value | Use Case |
|------------|----------|
| `unix:///var/run/docker.sock` | Local Docker (Linux/macOS) |
| `tcp://localhost:2375` | Remote Docker (insecure) |
| `tcp://localhost:2376` | Remote Docker with TLS |
| `npipe:////./pipe/docker_engine` | Local Docker (Windows) |

### Docker on Remote Hosts

For secure remote Docker access:

1. Enable TLS on Docker daemon
2. Mount client certificates to AuraGo
3. Configure host with TLS:

```yaml
docker:
    enabled: true
    host: tcp://docker-host:2376
    readonly: false
```

### Available Operations

| Operation | Description |
|-----------|-------------|
| List containers | Show running and stopped containers |
| Start/Stop/Restart | Container lifecycle management |
| View logs | Stream container logs |
| Inspect | Detailed container information |
| Execute commands | Run commands inside containers |
| Image management | Pull, list, remove images |
| Network management | Create and manage networks |
| Volume management | List and manage volumes |

> ⚠️ **Warning:** Docker integration grants significant system access. Consider `readonly: true` for restricted environments.

### Docker Compose Example

```yaml
# docker-compose.yml for AuraGo with Docker access
services:
  aurago:
    image: aurago:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./data:/app/data
      - ./config.yaml:/app/config.yaml
```

---

## Webhooks

Webhooks allow external services to send events to AuraGo via HTTP POST requests.

### Built-in Presets

AuraGo includes presets for common webhook formats:

| Preset | Service | Features |
|--------|---------|----------|
| `generic_json` | Any service | Accepts any JSON payload |
| `github` | GitHub | Validates HMAC signatures, extracts repo/action/user |
| `gitlab` | GitLab | Token validation, project extraction |
| `home_assistant` | Home Assistant | Entity state changes |
| `uptime` | Uptime Kuma, Hetrix | Monitor alerts |
| `plain_text` | Generic | Plain text or simple JSON |

### Configuration

```yaml
webhooks:
    enabled: true
    readonly: false
    max_payload_size: 65536
    rate_limit: 60
```

### Creating a Webhook

Webhooks are created dynamically via the API or Web UI:

```bash
# Create webhook via API
curl -X POST http://localhost:8088/api/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "GitHub Deployments",
    "slug": "github-deploy",
    "format": "github",
    "delivery": {
      "mode": "message",
      "priority": "immediate"
    }
  }'
```

### Webhook URL Format

```
http://your-aurago:8088/api/webhooks/{slug}?token={token}
```

### Delivery Modes

| Mode | Description |
|------|-------------|
| `message` | Send to agent as user message |
| `notify` | Send SSE notification to UI only |
| `silent` | Log only, no notification |

### Example: GitHub Webhook

1. In AuraGo Web UI, go to Settings → Webhooks
2. Create new webhook:
   - Name: "GitHub Push Events"
   - Slug: `github-push`
   - Preset: GitHub
   - Delivery Mode: Message
3. Copy the generated URL
4. In GitHub repository:
   - Settings → Webhooks → Add webhook
   - Payload URL: Your AuraGo webhook URL
   - Content type: `application/json`
   - Secret: (optional, for HMAC validation)
   - Events: Push

Now AuraGo will receive and process GitHub push events.

> 💡 **Tip:** Use the `notify` mode for high-frequency events that don't need agent processing (like CI status updates).

---

## Budget Tracking

Track and control LLM API costs with the budget system.

### Configuration

```yaml
budget:
    enabled: true
    daily_limit_usd: 5.00
    enforcement: warn
    reset_hour: 0
    warning_threshold: 0.8
    default_cost:
        input_per_million: 1.00
        output_per_million: 3.00
    models:
      - name: gpt-4o
        input_per_million: 2.50
        output_per_million: 10.00
      - name: llama-3.1-8b
        input_per_million: 0
        output_per_million: 0
```

### Enforcement Modes

| Mode | Behavior |
|------|----------|
| `warn` | Log warning when limit exceeded (default) |
| `partial` | Block expensive models, allow cheap ones |
| `full` | Block all LLM requests when limit exceeded |

### Cost Tracking

Costs are calculated based on:
- Input tokens × input cost per million
- Output tokens × output cost per million

Set `input_per_million: 0` for free models (like Ollama local models).

### Budget Commands

```
You: What's my budget status?
AuraGo: Today's usage: $2.34 of $5.00 (47%). Remaining: $2.66.
```

> 💡 **Tip:** Set `warning_threshold: 0.8` to get notified at 80% of your daily limit.

---

## Google Workspace

Connect AuraGo to Gmail, Google Calendar, and Google Drive.

### Prerequisites

1. Enable in config:
```yaml
agent:
    enable_google_workspace: true
```

2. Create OAuth credentials (see detailed guide in `documentation/google_setup.md`)

### Authentication Flow

1. Trigger a Google operation: "Check my email"
2. AuraGo provides an authorization URL
3. Open the URL in your browser and approve access
4. Copy the redirected URL (starts with `http://localhost`)
5. Submit to AuraGo: `google_workspace submit_auth_url "paste_url_here"`

### Available Services

| Service | Actions |
|---------|---------|
| **Gmail** | List, read, search, send, label emails |
| **Calendar** | List events, create meetings, check availability |
| **Drive** | List files, search, read documents |
| **Docs** | Read document content |

### Example Commands

```
"Summarize my unread emails from today"
"Schedule a meeting with John tomorrow at 3pm"
"Find the Q4 report in my Drive"
"What's on my calendar for next week?"
```

> ⚠️ **Security:** OAuth tokens are stored in the encrypted vault. No `token.json` files are kept on disk.

---

## WebDAV/Koofr Setup

Access cloud storage through WebDAV-compatible services.

### WebDAV Configuration

```yaml
webdav:
    enabled: true
    url: "https://cloud.example.com/remote.php/dav/files/username/"
    username: "your_username"
    password: "your_app_password"
    readonly: false
```

### Nextcloud/ownCloud URL

Find your WebDAV URL in the web interface:
1. Files app → Settings (bottom left)
2. Copy the WebDAV URL

Example: `https://cloud.example.com/remote.php/dav/files/username/`

### Koofr Configuration

```yaml
koofr:
    enabled: true
    username: "your@email.com"
    app_password: "your-app-password"
    base_url: https://app.koofr.net
    readonly: false
```

### Creating a Koofr App Password

1. Go to Koofr → Account → Password
2. Click **Add password** under "App passwords"
3. Name it "AuraGo" and copy the password

### Available Operations

| Operation | Description |
|-----------|-------------|
| List | Browse directories |
| Read | Download and read files |
| Write | Upload or overwrite files |
| Mkdir | Create directories |
| Delete | Remove files/folders |
| Move | Rename or move items |
| Info | Get metadata |

> 💡 **Tip:** Always use app-specific passwords instead of your main account password.

> ⚠️ **Security:** Ensure WebDAV URLs use `https://` for encrypted connections.

---

## Additional Integrations Coverage (beyond core setup)

The current platform includes additional integrations/features that should be considered in production rollouts:

| Integration/Feature | Typical use case | Key config blocks |
|---|---|---|
| Cloudflare Tunnel + AI Gateway | secure public exposure and AI traffic routing | `cloudflare_tunnel`, `ai_gateway` |
| AdGuard / FRITZ!Box / MQTT | home network and smart-home connectivity | `adguard`, `fritzbox`, `mqtt` |
| Paperless NGX + Media Registry + Homepage | document/media/site registry workflows | `paperless_ngx`, `media_registry`, `homepage` |
| Netlify | static site deployment workflows | `netlify` |
| S3 + OneDrive + WebDAV/Koofr | multi-backend cloud storage access | `s3`, `onedrive`, `webdav`, `koofr` |
| Telnyx + Rocket.Chat | telephony and self-hosted chat channels | `telnyx`, `rocketchat` |
| Image generation / TTS / Whisper | multimodal generation and speech pipelines | `image_generation`, `tts`, `whisper` |
| MCP server mode | expose AuraGo capabilities to external MCP clients | `mcp_server` |
| LLM Guardian | policy and risk controls across tool/doc workflows | `llm_guardian` |

> Keep integrations in read-only mode first (`read_only`/`readonly`) and unlock write operations incrementally after validation.

---

## Testing Integrations

### Health Check Commands

Test individual integrations:

```bash
# Test Telegram
curl http://localhost:8088/api/health/telegram

# Test Discord
curl http://localhost:8088/api/health/discord

# Test Email
curl http://localhost:8088/api/health/email

# Test Home Assistant
curl http://localhost:8088/api/health/homeassistant

# Test Docker
curl http://localhost:8088/api/health/docker
```

### Integration Status in Web UI

The dashboard shows status indicators:
- 🟢 Green: Connected and working
- 🟡 Yellow: Configured but not connected
- 🔴 Red: Error or disabled

### Debugging Integration Issues

1. **Check logs:**
   ```bash
   tail -f log/aurago_$(date +%Y-%m-%d).log
   ```

2. **Verify configuration:**
   ```bash
   ./aurago -validate-config
   ```

3. **Test with verbose output:**
   ```bash
   ./aurago -debug
   ```

### Common Issues

| Issue | Solution |
|-------|----------|
| Telegram bot not responding | Check `telegram_user_id` matches your account |
| Discord connection fails | Verify bot token and intents are enabled |
| Email authentication fails | Use app password, not regular password |
| Home Assistant 401 error | Regenerate access token |
| Docker permission denied | Add user to docker group or use sudo |
| Webhook not receiving | Check firewall and URL format |

### Integration Testing Checklist

Before relying on any integration:

- [ ] Configuration saved in `config.yaml`
- [ ] AuraGo restarted after config change
- [ ] Health check endpoint returns success
- [ ] Test message/event received successfully
- [ ] Error handling verified (wrong credentials, etc.)
- [ ] Logs show no errors

---

## Next Steps

Now that your integrations are set up:

1. **[Security](14-security.md)** – Secure your AuraGo installation
2. **[Advanced Usage](15-coagents.md)** – Workflows, co-agents, and automation
3. **[Troubleshooting](16-troubleshooting.md)** – Solve common issues

> 💡 **Pro Tip:** Start with one integration at a time. Test thoroughly before adding more. This makes debugging much easier.
