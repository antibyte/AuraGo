# Discord Setup

AuraGo can talk to you through Discord: you write in a channel or a DM, the agent
answers, and it can send on its own through the agent tools.

This guide covers the setup, the new configuration keys, and the limits that apply
to what the agent may do on Discord.

---

## Step 1: Create the bot

1. Open the [Discord Developer Portal](https://discord.com/developers/applications)
   and create a new application.
2. Under **Bot**, add a bot and copy its token. The token is a secret — AuraGo
   stores it in the encrypted Vault, never in `config.yaml`.
3. Under **Bot → Privileged Gateway Intents**, enable **Message Content Intent**.
   Without it Discord does not deliver message text and AuraGo reports
   *"enable the Message Content Intent for the bot in the Discord Developer Portal"*.
4. Invite the bot to your server with the permissions it needs:
   * **View Channels**, **Send Messages**, **Read Message History** for the
     channels it should work in.
   * **Send Messages** in the channels it may post to on its own.
5. To have AuraGo list the channels of a server, note the server (guild) ID. Enable
   **Developer Mode** in Discord (User Settings → Advanced), then right-click the
   server name → *Copy Server ID*.

---

## Step 2: Configure AuraGo

Open **Configuration → Discord** and fill in:

| Key | Meaning |
|---|---|
| `enabled` | Turns the Discord integration on. |
| `bot_token` | The bot token from the Developer Portal. Stored in the Vault. |
| `allowed_user_id` | Your Discord user ID. Only this user can command the agent. Empty blocks all inbound messages. |
| `default_channel_id` | The channel AuraGo answers in and posts to when nothing else is specified. |
| `guild_id` | The server ID used by the channel-listing tool. |
| `readonly` | When `true`, the agent may read but not send. |
| `allowed_channel_ids` | The channels the agent may use — see below. |
| `max_concurrent_workers` | How many messages may run an agent loop at the same time. |

The non-secret keys in `config.yaml` look like this:

```yaml
discord:
  enabled: true
  allowed_user_id: "123456789012345678"
  default_channel_id: "987654321098765432"
  guild_id: "112233445566778899"
  readonly: false
  allowed_channel_ids:
    - "987654321098765432"
  max_concurrent_workers: 5
```

Saving Discord settings takes effect without restarting AuraGo: the bot is
stopped and started again with the new configuration.

---

## Step 3: Allowed channels

`allowed_channel_ids` restricts **which channels the agent may read from and
write to** when it acts on its own — for example through the `send_discord` and
`fetch_discord` agent tools.

* **Empty (default)** — AuraGo behaves as before: the agent may use every channel
  the bot can reach. On startup AuraGo logs a hint that this is unrestricted.
* **Set** — only the listed channels plus `default_channel_id` may be used. A
  request for any other channel is refused with a message that names the allowed
  channels and the key to change. The channel list the agent sees is filtered to
  the same set.

This key is deliberately opt-in: an installation that has never set it keeps
working exactly as before.

> The person-detection alerts (AuraGo's camera/presence notifications) always go
> to `default_channel_id` and are not affected by this list.

---

## Step 4: Read-only mode

With `readonly: true` the agent may still read Discord messages, but it may not
send: `send_discord` and the `discord` notification channel are refused.
Incoming messages are still answered — the bot itself always replies.

---

## Step 5: Concurrency

`max_concurrent_workers` limits how many Discord messages may run an agent loop
at the same time (default `5`). If all slots are busy, AuraGo waits briefly and
then tells the sender that it is busy instead of starting yet another run. Values
of `0` or less fall back to `5`.

---

## Step 6: Verify

* **Configuration → Discord → Test** sends a message to the default channel.
* `GET /api/health/discord` reports the runtime state. It answers `503` while the
  integration is enabled but not connected, and `200` otherwise. The body shows
  `enabled`, `connected`, `status`, `token_present`, `read_only`,
  `guild_configured`, `default_channel_configured` and `allowed_user_configured`
  — no secrets.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| *„disallowed intent" / 4014* | Message Content Intent is off | Enable it in the Developer Portal (Bot → Privileged Gateway Intents) |
| *„unauthorized" / 401* | The bot token was reset | Copy the new token into the AuraGo Vault |
| The bot connects but answers nothing | `allowed_user_id` is empty or wrong | Set it to your numeric user ID (Developer Mode → right-click your name) |
| The bot answers in the wrong place | `default_channel_id` is empty | Set it to the channel ID you want |
| The agent asks for a channel it may not use | `allowed_channel_ids` is set and does not contain it | Add the channel ID to the list |
| No channel listing | `guild_id` is empty | Set the server ID (Developer Mode → Copy Server ID) |
| *„busy"* | All worker slots are taken | Raise `max_concurrent_workers` |

---

See also: [telegram_setup.md](telegram_setup.md) for the Telegram side, and
[configuration.md](configuration.md) for the complete key reference.
