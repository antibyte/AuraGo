# n8n-nodes-aurago

[n8n](https://n8n.io/) community node for [AuraGo](https://github.com/antibyte/aurago) AI Agent. Integrate your self-hosted AI agent into n8n workflows for powerful automation.

## Features

### Operations

- **Chat** - Send messages to your AuraGo agent and receive responses
- **Tools** - Execute any AuraGo tool directly (filesystem, web scraper, API requests, etc.)
- **Memory** - Search and store memories in the agent's short-term, long-term, or knowledge graph
- **Missions** - Create and run automated mission chains

### Triggers

- **Agent Response** - Trigger workflows when the agent responds
- **Tool Call** - React to tool executions
- **Error Events** - Handle agent errors
- **Memory Events** - Process new memories
- **Mission Complete** - Chain mission executions

## Installation

### Via n8n Community Nodes (Recommended)

1. Open your n8n instance
2. Go to **Settings** → **Community Nodes**
3. Click **Install**
4. Enter `n8n-nodes-aurago`
5. Agree to the risks and install

### Via npm

```bash
cd ~/.n8n
npm install n8n-nodes-aurago
```

### Via Docker

```bash
docker run -it --rm \
  --name n8n \
  -p 5678:5678 \
  -v ~/.n8n:/home/node/.n8n \
  n8nio/n8n:latest \
  npm install n8n-nodes-aurago && n8n start
```

## Setup

### 1. Configure AuraGo

In your AuraGo instance:

1. Open the Config UI (`/config`)
2. Navigate to **n8n Integration**
3. Enable the integration
4. Generate an API token
5. Set your webhook URL (if using triggers)

```yaml
# Or in config.yaml:
n8n:
  enabled: true
  webhook_base_url: "https://your-n8n.com/webhook"
  allowed_events:
    - "agent.response"
    - "agent.error"
```

### 2. Create Credentials in n8n

1. In n8n, go to **Credentials** → **New**
2. Select **AuraGo API**
3. Enter:
   - **Base URL**: Your AuraGo URL (e.g., `http://localhost:8088`)
   - **API Token**: The token generated in step 1
4. Click **Save**

## Usage

### Example 1: Simple Chat

```
HTTP Request (Webhook) → AuraGo (Chat) → Email
```

1. Add an **AuraGo** node
2. Select **Resource: Chat**
3. Select **Operation: Send Message**
4. Enter your message (can use expressions: `{{ $json.message }}`)
5. Run the workflow

### Example 2: File Processing Pipeline

```
Google Drive (New File) → AuraGo (Tool: pdf_extractor) → Slack
```

1. Add a trigger for new files
2. Add **AuraGo** node
3. Select **Resource: Tool** → **Operation: Execute**
4. Select **Tool: pdf_extractor**
5. Parameters: `{"file_path": "{{ $json.url }}"}`

### Example 3: Knowledge Base Search

```
HTTP Request (Question) → AuraGo (Memory: Search) → AuraGo (Chat with context) → Response
```

1. Search memory for relevant context
2. Pass context + question to chat node
3. Return augmented response

### Example 4: Trigger on Mission Complete

```
AuraGo Trigger (Mission Completed) → HTTP Request (Notification)
```

1. Add **AuraGo Trigger** node
2. Select **Events: Mission Completed**
3. Connect notification action

## Node Reference

### Chat Resource

| Operation | Description |
|-----------|-------------|
| Send Message | Send a message to the agent |
| Continue Session | Continue an existing conversation |

**Parameters:**
- Message
- Session ID (for continuity)
- Context Window (number of previous messages)
- System Prompt (override)
- Tool Restrictions

### Tool Resource

| Operation | Description |
|-----------|-------------|
| Execute | Run a specific tool |
| List Available | Get all available tools |

**Parameters:**
- Tool Name
- Parameters (JSON)
- Timeout
- Async mode

### Memory Resource

| Operation | Description |
|-----------|-------------|
| Search | Query memories |
| Store | Save information |

**Types:**
- Short Term (chat history)
- Long Term (vector DB)
- Knowledge Graph
- Core Memory

### Mission Resource

| Operation | Description |
|-----------|-------------|
| Create | Create a new mission |
| Create and Run | Create and execute immediately |

**Parameters:**
- Name
- Description
- Trigger type (manual/webhook/schedule)
- Steps (JSON array)

## Webhook Security

Webhooks from AuraGo are signed with HMAC-SHA256 using your API token. The trigger node automatically verifies these signatures.

To verify manually:

```javascript
const crypto = require('crypto');
const signature = crypto
  .createHmac('sha256', API_TOKEN)
  .update(JSON.stringify(body.data))
  .digest('hex');
```

## Troubleshooting

### Connection Errors

1. Verify AuraGo is running
2. Check Base URL includes protocol (`http://` or `https://`)
3. Test with Ignore SSL Issues if using self-signed cert
4. Verify token has correct scopes

### Authentication Errors

1. Regenerate token in AuraGo Config UI
2. Ensure token is copied completely (starts with `n8n_`)
3. Check token hasn't expired

### Webhook Not Triggering

1. Verify webhook_base_url is set in AuraGo
2. Check event type is in allowed_events list
3. Ensure n8n webhook URL is publicly accessible
4. Check AuraGo logs for webhook delivery errors

## Development

```bash
# Clone repository
git clone https://github.com/antibyte/aurago.git
cd aurago/n8n-nodes-aurago

# Install dependencies
npm install

# Build
npm run build

# Development mode (watch)
npm run dev

# Lint
npm run lint
```

## License

MIT - See [LICENSE](LICENSE)

## Support

- [AuraGo Documentation](https://github.com/antibyte/aurago)
- [n8n Community](https://community.n8n.io/)
- [Issues](https://github.com/antibyte/aurago/issues)

## Contributing

Contributions welcome! Please follow the [n8n node development guidelines](https://docs.n8n.io/integrations/creating-nodes/).
