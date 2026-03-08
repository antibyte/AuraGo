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
4. Copy that ID and update `config.yaml`.

## Step 3: Configure AuraGo

Add your credentials to the `telegram` section in `config.yaml`:

```yaml
telegram:
  bot_token: "YOUR_BOT_TOKEN_HERE"
  telegram_user_id: 987654321  # Your numeric ID
```
Restart AuraGo for the changes to take effect.

## Step 4: Usage

Once AuraGo is running:
- Send a message to start a conversation.
