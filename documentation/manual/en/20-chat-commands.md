# Chat Commands

AuraGo supports **Slash Commands** – commands that can be entered directly in chat to perform specific actions. All commands start with a forward slash `/`.

> 📅 **Updated:** March 2026

---

## Overview of All Commands

| Command | Description | Availability |
|---------|-------------|--------------|
| `/help` | Shows all available commands with short descriptions | Always |
| `/reset` | Clears chat history and short-term memory | Always |
| `/stop` | Interrupts the current agent action | Always |
| `/restart` | Restarts the AuraGo server | Always |
| `/debug [on/off]` | Enables/Disables debug mode | Always |
| `/personality [name]` | Lists or switches personalities | Always |
| `/budget [en]` | Shows current budget status | If budget tracking enabled |
| `/sudopwd <password>` | Stores sudo password in vault | Always |
| `/voice [on/off]` | Toggles voice output (TTS) | Always |
| `/warnings` | Lists active system warnings | Always |
| `/addssh` | Registers a new SSH server | Always |
| `/credits` | Shows OpenRouter credits and usage | OpenRouter only |

---

## Detailed Descriptions

### `/help`
Displays a list of all registered commands with their short descriptions.

**Example:**
```
/help
```

**Output:**
```
📜 **Available Commands:**

• /reset: Clears the current chat history (Short-Term Memory).
• /stop: Interrupts the current agent action.
• /help: Shows this help.
...
```

---

### `/reset`
Clears the current chat history and short-term memory (STM). This is useful for a "fresh start" without affecting long-term storage or other data.

**Example:**
```
/reset
```

**Output:**
```
🧹 Chat history and short-term memory have been cleared.
```

> ⚠️ **Note:** Long-term memory (LTM) and the Knowledge Graph remain intact.

---

### `/stop`
Interrupts the currently running agent action. This is helpful when the agent gets into an infinite loop or performs an unwanted action.

**Example:**
```
/stop
```

**Output:**
```
🛑 AuraGo has been instructed to interrupt the current action.
```

---

### `/restart`
Restarts the AuraGo server. This can be useful after making configuration changes that require a restart.

**Example:**
```
/restart
```

**Output:**
```
🔄 AuraGo is restarting...
```

> ⚠️ **Note:** The restart occurs asynchronously after a short 1-second delay.

---

### `/debug [on/off]`
Enables or disables the agent's debug mode. In debug mode, more detailed error messages are activated in the system prompt.

**Syntax:**
```
/debug [on|off|1|0|true|false]
```

**Examples:**
```
/debug on      # Enable debug mode
/debug off     # Disable debug mode
/debug         # Toggle (switch)
```

**Output (enabled):**
```
🔍 **Agent Debug Mode enabled.** The agent now reports errors with detailed information.
```

**Output (disabled):**
```
🔇 **Agent Debug Mode disabled.** The agent behaves normally.
```

---

### `/personality [name]`
Lists all available personalities or switches to a specific personality.

**Syntax:**
```
/personality [name]
```

**Examples:**
```
/personality           # Lists all personalities
/personality default   # Switches to default personality
/personality tech      # Switches to tech personality
```

**Output (list):**
```
🎭 **Available Personalities:**

• default ✅ (active)
• tech
• creative
• professional

Use `/personality <name>` to switch.
```

**Output (switch):**
```
🎭 Personality switched to **tech**. The change is permanent.
```

> ℹ️ Personalities are stored as Markdown files in the `prompts/personalities/` directory.

---

### `/budget [en]`
Displays the current budget status, including daily costs, limits, and models used.

**Syntax:**
```
/budget [en|english]
```

**Parameters:**
- `en` (optional) – Displays output in English

**Examples:**
```
/budget        # German
/budget en     # English
```

**Output:**
```
💰 **Budget Status (2026-03-28)**

Today's Costs: $0.42 / $5.00 (8.4%)
Models Used:
  • google/gemini-2.0-flash-001: $0.32
  • anthropic/claude-3.5-sonnet: $0.10
```

> ℹ️ Budget tracking must be enabled in the configuration (`budget.enabled: true`).

---

### `/voice [on|off]`
Toggles voice output (TTS). When no argument is given, the current state is toggled.

**Syntax:**
```
/voice [on|off|1|0|true|false]
```

**Examples:**
```
/voice on      # Enable voice output
/voice off     # Disable voice output
/voice         # Toggle
```

---

### `/warnings`
Displays all active system warnings from the warnings registry with severity level (critical/warning/info).

**Example:**
```
/warnings
```

---

### `/sudopwd <password>`
Securely stores the sudo password in the vault for the `execute_sudo` tool. The password is stored encrypted with AES-256-GCM.

**Syntax:**
```
/sudopwd <password>
/sudopwd --clear
```

**Parameters:**
- `password` – The sudo password
- `--clear` – Deletes the stored password

**Examples:**
```
/sudopwd mySecurePassword123
/sudopwd --clear
```

**Output:**
```
✅ Sudo password successfully stored in vault.
⚠️ Note: `agent.sudo_enabled` is still disabled. Enable it in the config.
```

> 🔒 **Security:** The password is never stored or logged in plain text.

---

### `/addssh`
Registers a new SSH server in the inventory and securely stores credentials in the vault.

**Syntax:**
```
/addssh host=NAME user=USER [ip=IP] [pass=PASS|keypath=PATH] [port=22] [tags=tag1,tag2]
```

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `host` | Yes | Server hostname |
| `user` | Yes | SSH username |
| `ip` | No | IP address (if different from hostname) |
| `pass` | Conditional | Password (either pass or keypath) |
| `keypath` | Conditional | Path to SSH key |
| `port` | No | SSH port (default: 22) |
| `tags` | No | Comma-separated tags |

**Examples:**
```
/addssh host=server1 user=root pass=secret
/addssh host=server2 user=admin keypath=/home/user/.ssh/id_rsa port=2222 tags=production,web
```

**Output:**
```
✅ Server server1 successfully registered with ID: abc-123-def
```

---

### `/credits`
Displays the current OpenRouter credit balance and usage. Only available when OpenRouter is configured as the LLM provider.

**Example:**
```
/credits
```

**Output:**
```
💳 **OpenRouter Credits**

Balance: $25.43
Used today: $0.42
Remaining: $25.01
```

> ℹ️ This command is only available when an OpenRouter provider is configured (primary LLM, helper LLM, or providers list).

---

## Programmatic Usage

Commands can also be triggered programmatically via the API:

```bash
# Via Chat-Completion API with special prefix
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{"role": "user", "content": "/reset"}]
  }'
```

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Command not recognized | Make sure you start with `/` and have no leading spaces |
| `/budget` shows no data | Budget tracking must be enabled in `config.yaml` |
| `/credits` not working | Only available when using OpenRouter as provider |
| `/addssh` reports errors | Check if `host` and `user` are specified, plus `pass` or `keypath` |

---

## Related Links

- [Web Interface](04-webui.md) – Alternative to command line
- [REST API Reference](21-api-reference.md) – Programmatic access
- [Security](14-security.md) – Vault and encryption
