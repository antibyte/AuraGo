# Telegram Bot Setup Guide (UI & Communication Layer)

This guide explains how to set up the Telegram Bot interface for AuraGo. Telegram serves as the primary Chat UI, allowing remote interaction, tool authorization (like passing OAuth URLs), and asynchronous notifications.

## Step 1: Create the Bot via BotFather

To interact with the Telegram API, you need to register a new bot and obtain a secret token.

1. Open Telegram on your phone or desktop.
2. Search for the official **@BotFather** (verified blue checkmark).
3. Start a chat and send `/newbot`.
4. Follow the prompts:
   - **Name:** e.g., `AuraGo Agent`.
   - **Username:** e.g., `aurago_agent_bot`.
5. Once created, BotFather will give you an **HTTP API Token**. Keep this secret.

## Step 2: Whitelist Your User ID (Security)

AuraGo is a personal agent. You MUST restrict access so it only responds to you.

### Option A: Manual Discovery
1. Search for **@userinfobot** in Telegram.
2. It will reply with your `Id` (e.g., `987654321`).

### Option B: Silent ID Discovery (AuraGo Feature)
1. Set `telegram_user_id: 0` in `config.yaml`.
2. Start AuraGo and message your new bot.
3. AuraGo will block the message but print your ID to the system console/logs.
4. Copy that ID into the Telegram settings and save.

## Step 3: Configure AuraGo

Open **Configuration → Telegram**, enter the bot token and your numeric user ID,
then save. The token is stored in the encrypted Vault; never put it in YAML.
The non-secret settings in `config.yaml` look like this:

```yaml
telegram:
  telegram_user_id: 987654321  # Your numeric ID
  max_concurrent_workers: 5
  readonly: false              # true = the bot answers, but the agent may not send
```
Saving Telegram settings takes effect without restarting AuraGo. Token, user ID
and worker-limit changes cancel the old poller and its active messages before a
replacement starts. Active messages are not automatically replayed. The update
cursor is preserved for the same verified bot — during this server process, and
across restarts through a cursor file in the data directory (`telegram_offsets.json`).
Switching to a different bot starts its own cursor.

Removing the token or enabling Egg Mode stops Telegram. An invalid replacement
token appears as unavailable in the existing integration status; AuraGo does not
reactivate the previous token. Correct the token in Configuration and save again.

## Read-only mode

With `telegram.readonly: true` the bot still answers incoming messages, but the
agent may not send on its own: `send_telegram` and the `telegram` notification
channel are refused with a clear message. This is the counterpart of
`discord.readonly`.

## Message length

Telegram accepts 4096 characters per message, counted after the Markdown
formatting is parsed. A longer message is refused before it is sent, with the
measured length — shorten the text instead of retrying. If Telegram reports
flood control (HTTP 429), AuraGo waits for the announced time and repeats the
send at most twice; a message that was accepted is never sent twice.

## Step 4: Usage

Once AuraGo is running:
- Send a message to start a conversation.
