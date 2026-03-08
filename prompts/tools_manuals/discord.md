# Discord Tool Manual

Integrated Discord bot. The agent can send/receive messages and list channels.

> The bot responds when **mentioned** (`@BotName message`) or in **DMs**.  
> All fetched messages are scanned by the Guardian for prompt injection.

---

## send_discord — Send a message to a Discord channel

```json
{
  "action": "send_discord",
  "channel_id": "1234567890",
  "message": "Hello from AuraGo!"
}
```

| Parameter  | Type   | Required | Description                                                    |
|------------|--------|----------|----------------------------------------------------------------|
| channel_id | string | no       | Target channel ID. Defaults to `default_channel_id` from config |
| message    | string | **yes**  | Message text. `content` and `body` also accepted               |

### Response
```json
{"status": "success", "message": "Message sent to Discord channel 1234567890"}
```

> Long messages (>2000 chars) are automatically split at newline boundaries.

---

## fetch_discord — Read recent messages from a Discord channel

```json
{
  "action": "fetch_discord",
  "channel_id": "1234567890",
  "limit": 10
}
```

| Parameter  | Type   | Required | Default | Description                                          |
|------------|--------|----------|---------|------------------------------------------------------|
| channel_id | string | no       | config  | Channel ID to fetch from                             |
| limit      | int    | no       | 10      | Number of most recent messages to retrieve (max 100) |

### Response
```json
{
  "status": "success",
  "count": 3,
  "data": [
    {
      "author": "username",
      "content": "message text",
      "timestamp": "2026-02-28T10:30:00Z",
      "id": "1234567890123456"
    }
  ]
}
```

---

## list_discord_channels — List text channels in the configured guild

```json
{
  "action": "list_discord_channels"
}
```

No parameters required. Uses `guild_id` from config.yaml.

### Response
```json
{
  "status": "success",
  "count": 5,
  "data": [
    {"id": "1234567890", "name": "general"},
    {"id": "1234567891", "name": "bot-commands"}
  ]
}
```

---

## Configuration (config.yaml)

```yaml
discord:
  enabled: true
  bot_token: "your-discord-bot-token"
  guild_id: "your-server-id"           # Required for list_discord_channels
  allowed_user_id: "your-discord-user-id"  # Leave empty to allow all users
  default_channel_id: "channel-id"     # Fallback for send/fetch without explicit channel_id
```

## Bot Setup
1. Go to https://discord.com/developers/applications
2. Create a new Application → Bot → copy the token
3. Enable **MESSAGE CONTENT INTENT** under Bot → Privileged Gateway Intents
4. Invite URL: `https://discord.com/api/oauth2/authorize?client_id=YOUR_APP_ID&permissions=3072&scope=bot`
   - `3072` = Send Messages + Read Message History
5. Paste the bot token into `config.yaml`

## Behavior
- The bot only responds when **@mentioned** in a server channel or messaged via **DM**
- If `allowed_user_id` is set, only that Discord user can interact with the agent
- Messages from other users are silently ignored (logged as warnings)
- Slash commands (`/help`, etc.) work the same as in Telegram and the Web UI
