# AuraGo n8n Node - Architecture Plan

## Overview

A community n8n node that enables bidirectional communication between n8n workflows and AuraGo AI agent. The integration supports both triggering workflows from AuraGo and controlling the agent from n8n.

---

## Architecture

```
┌─────────────────┐         HTTP/REST          ┌─────────────────┐
│   n8n Workflow  │  ◄──────────────────────►  │    AuraGo       │
│                 │    Bearer Token Auth       │    Agent        │
│ ┌─────────────┐ │                            │  ┌───────────┐  │
│ │ AuraGo Node │ │                            │  │ n8n API   │  │
│ │  (Trigger)  │ │◄───────────────────────────┼──│  Handler  │  │
│ └─────────────┘ │   Webhook (Agent → n8n)    │  └───────────┘  │
│ ┌─────────────┐ │                            │  ┌───────────┐  │
│ │ AuraGo Node │ │───────────────────────────►│  │  Token    │  │
│ │   (Action)  │ │   /api/n8n/chat            │  │  Manager  │  │
│ └─────────────┘ │   /api/n8n/tools/{name}    │  └───────────┘  │
└─────────────────┘                            └─────────────────┘
```

---

## Part 1: AuraGo Backend Changes

### 1.1 New Configuration Section

Add to `config_types.go`:

```go
type N8nConfig struct {
    Enabled        bool     `yaml:"enabled"`
    ReadOnly       bool     `yaml:"readonly"`        // Only allow read operations
    WebhookBaseURL string   `yaml:"webhook_base_url"` // n8n webhook base URL
    AllowedEvents  []string `yaml:"allowed_events"`   // Filter which events trigger n8n
    RequireToken   bool     `yaml:"require_token"`    // Default: true
}
```

### 1.2 New API Endpoints (`internal/server/n8n_handlers.go`)

#### Authentication
All endpoints require Bearer token (same as MCP Server) with scope `n8n`.

```go
// POST /api/n8n/chat
// Send a message to the agent and get response
Request:
{
    "message": "string",           // User message
    "session_id": "string",        // Optional: maintain conversation context
    "system_prompt": "string",     // Optional: override system prompt
    "tools": ["tool1", "tool2"],   // Optional: restrict available tools
    "context_window": 10,          // Optional: number of previous messages
    "stream": false                // Optional: SSE streaming response
}

Response:
{
    "response": "string",          // Agent response
    "session_id": "string",        // Session ID for continuity
    "tool_calls": [...],           // Tools that were executed
    "tokens_used": 1234,
    "duration_ms": 2500
}
```

```go
// POST /api/n8n/tools/{tool_name}
// Direct tool execution without LLM
Request:
{
    "parameters": {...},           // Tool-specific parameters
    "async": false,                // Return immediately with task ID
    "timeout": 60                  // Max execution time
}

Response:
{
    "result": "...",               // Tool output
    "status": "success|error",
    "task_id": "..."               // If async=true
}
```

```go
// GET /api/n8n/tools
// List available tools
Response:
{
    "tools": [
        {
            "name": "filesystem",
            "description": "...",
            "parameters": {...}
        }
    ]
}
```

```go
// POST /api/n8n/memory/search
// Search agent memory
Request:
{
    "query": "string",
    "limit": 10,
    "type": "short_term|long_term|knowledge_graph"
}
```

```go
// POST /api/n8n/memory/store
// Store information in agent memory
Request:
{
    "content": "string",
    "type": "short_term|long_term|core",
    "metadata": {...}
}
```

```go
// POST /api/n8n/missions
// Create and run missions
Request:
{
    "name": "string",
    "description": "string",
    "steps": [...],
    "trigger": "manual|webhook|schedule"
}
```

```go
// GET /api/n8n/status
// Health and capability check
Response:
{
    "status": "ok",
    "version": "1.0.0",
    "capabilities": ["chat", "tools", "memory", "missions"],
    "config": {
        "readonly": false,
        "allowed_tools": [...]
    }
}
```

### 1.3 Webhook Integration (Agent → n8n)

AuraGo can trigger n8n workflows via webhooks:

```go
// Trigger events:
- "agent.response"      // After agent generates response
- "agent.tool_call"     // Before/after tool execution
- "agent.error"         // On errors
- "memory.stored"       // When new memory is stored
- "mission.completed"   // When a mission finishes

// POST {webhook_base_url}/webhook/aurago/{event}
Payload:
{
    "event": "agent.response",
    "timestamp": "2026-03-20T10:00:00Z",
    "session_id": "...",
    "data": {
        "message": "...",
        "tool_calls": [...],
        "tokens_used": 1234
    },
    "signature": "hmac-sha256"  // Webhook signature for verification
}
```

### 1.4 Token Scope Management

Extend existing TokenManager to support n8n scope:

```go
// Token scopes for n8n:
"n8n:read"      // Read-only operations
"n8n:chat"      // Chat with agent
"n8n:tools"     // Execute tools
"n8n:memory"    // Read/write memory
"n8n:missions"  // Create/run missions
"n8n:admin"     // Full access
```

---

## Part 2: n8n Community Node

### 2.1 Package Structure

```
n8n-nodes-aurago/
├── package.json
├── tsconfig.json
├── README.md
├── credentials/
│   └── AuraGoApi.credentials.ts
├── nodes/
│   ├── AuraGo/
│   │   ├── AuraGo.node.ts          # Main node definition
│   │   ├── AuraGoTrigger.node.ts   # Trigger node (webhooks)
│   │   ├── GenericFunctions.ts     # API helpers
│   │   ├── descriptions/           # Operation descriptions
│   │   │   ├── ChatDescription.ts
│   │   │   ├── ToolDescription.ts
│   │   │   ├── MemoryDescription.ts
│   │   │   └── MissionDescription.ts
│   │   └── types.d.ts
│   └── icons/
│       └── AuraGo.svg
└── workflows/
    └── examples/
```

### 2.2 Credentials (`AuraGoApi.credentials.ts`)

```typescript
export class AuraGoApi implements ICredentialType {
    name = 'auraGoApi';
    displayName = 'AuraGo API';
    documentationUrl = 'https://github.com/antibyte/aurago';
    
    properties: INodeProperties[] = [
        {
            displayName: 'Base URL',
            name: 'baseUrl',
            type: 'string',
            default: 'http://localhost:8088',
            required: true,
        },
        {
            displayName: 'API Token',
            name: 'apiToken',
            type: 'string',
            typeOptions: { password: true },
            default: '',
            required: true,
            description: 'Create token in AuraGo Config UI → API Tokens',
        },
        {
            displayName: 'Ignore SSL Issues',
            name: 'ignoreSslIssues',
            type: 'boolean',
            default: false,
        },
    ];
    
    authenticate: IAuthenticateGeneric = {
        type: 'generic',
        properties: {
            headers: {
                Authorization: '=Bearer {{$credentials.apiToken}}',
            },
        },
    };
    
    test: ICredentialTestRequest = {
        request: {
            baseURL: '={{ $credentials.baseUrl }}',
            url: '/api/n8n/status',
        },
    };
}
```

### 2.3 Main Node (`AuraGo.node.ts`)

```typescript
export class AuraGo implements INodeType {
    description: INodeTypeDescription = {
        displayName: 'AuraGo',
        name: 'auraGo',
        icon: 'file:AuraGo.svg',
        group: ['AI'],
        version: 1,
        subtitle: '={{ $parameter["operation"] }}',
        description: 'Interact with AuraGo AI Agent',
        defaults: {
            name: 'AuraGo',
        },
        inputs: [NodeConnectionType.Main],
        outputs: [NodeConnectionType.Main],
        credentials: [
            {
                name: 'auraGoApi',
                required: true,
            },
        ],
        properties: [
            {
                displayName: 'Operation',
                name: 'operation',
                type: 'options',
                noDataExpression: true,
                options: [
                    {
                        name: 'Chat',
                        value: 'chat',
                        description: 'Send message to agent',
                        action: 'Chat with agent',
                    },
                    {
                        name: 'Execute Tool',
                        value: 'executeTool',
                        description: 'Execute a specific tool',
                        action: 'Execute tool',
                    },
                    {
                        name: 'Memory',
                        value: 'memory',
                        description: 'Search or store memory',
                        action: 'Manage memory',
                    },
                    {
                        name: 'Mission',
                        value: 'mission',
                        description: 'Create or run mission',
                        action: 'Manage missions',
                    },
                    {
                        name: 'Get Status',
                        value: 'status',
                        description: 'Get agent status',
                        action: 'Get status',
                    },
                ],
                default: 'chat',
            },
            // Include operation-specific descriptions...
            ...chatOperations,
            ...toolOperations,
            ...memoryOperations,
            ...missionOperations,
        ],
    };

    async execute(this: IExecuteFunctions): Promise<INodeExecutionData[][]> {
        const items = this.getInputData();
        const returnData: INodeExecutionData[] = [];
        
        const credentials = await this.getCredentials('auraGoApi');
        const operation = this.getNodeParameter('operation', 0) as string;
        
        for (let i = 0; i < items.length; i++) {
            try {
                let response;
                
                switch (operation) {
                    case 'chat':
                        response = await chatWithAgent.call(this, i, credentials);
                        break;
                    case 'executeTool':
                        response = await executeTool.call(this, i, credentials);
                        break;
                    case 'memory':
                        response = await manageMemory.call(this, i, credentials);
                        break;
                    case 'mission':
                        response = await manageMission.call(this, i, credentials);
                        break;
                    case 'status':
                        response = await getStatus.call(this, credentials);
                        break;
                }
                
                returnData.push({
                    json: response,
                    pairedItem: { item: i },
                });
            } catch (error) {
                if (this.continueOnFail()) {
                    returnData.push({
                        json: { error: error.message },
                        pairedItem: { item: i },
                    });
                } else {
                    throw error;
                }
            }
        }
        
        return [returnData];
    }
}
```

### 2.4 Trigger Node (`AuraGoTrigger.node.ts`)

```typescript
export class AuraGoTrigger implements INodeType {
    description: INodeTypeDescription = {
        displayName: 'AuraGo Trigger',
        name: 'auraGoTrigger',
        icon: 'file:AuraGo.svg',
        group: ['trigger'],
        version: 1,
        description: 'Trigger workflows from AuraGo events',
        defaults: {
            name: 'AuraGo Trigger',
        },
        inputs: [],
        outputs: [NodeConnectionType.Main],
        credentials: [
            {
                name: 'auraGoApi',
                required: true,
            },
        ],
        webhooks: [
            {
                name: 'default',
                httpMethod: 'POST',
                responseMode: 'onReceived',
                path: 'webhook',
            },
        ],
        properties: [
            {
                displayName: 'Events',
                name: 'events',
                type: 'multiOptions',
                options: [
                    { name: 'Agent Response', value: 'agent.response' },
                    { name: 'Tool Call', value: 'agent.tool_call' },
                    { name: 'Error', value: 'agent.error' },
                    { name: 'Memory Stored', value: 'memory.stored' },
                    { name: 'Mission Completed', value: 'mission.completed' },
                ],
                default: ['agent.response'],
            },
            {
                displayName: 'Session Filter',
                name: 'sessionFilter',
                type: 'string',
                default: '',
                placeholder: 'filter by session ID',
            },
        ],
    };

    async webhook(this: IWebhookFunctions): Promise<IWebhookResponseData> {
        const body = this.getBodyData() as IDataObject;
        const events = this.getNodeParameter('events') as string[];
        
        // Filter by event type
        if (!events.includes(body.event as string)) {
            return {
                workflowData: [[]],
            };
        }
        
        // Verify webhook signature (HMAC)
        const credentials = await this.getCredentials('auraGoApi');
        if (!verifyWebhookSignature(body, credentials)) {
            throw new Error('Invalid webhook signature');
        }
        
        return {
            workflowData: [
                [
                    {
                        json: body,
                    },
                ],
            ],
        };
    }
}
```

---

## Part 3: Feature Suggestions

### Core Features (MVP)

| Feature | Priority | Description |
|---------|----------|-------------|
| Chat | P0 | Send/receive messages with context |
| Tool Execution | P0 | Execute any AuraGo tool directly |
| Tool Discovery | P0 | Auto-discover available tools |
| Memory Search | P1 | Query STM/LTM/Knowledge Graph |
| Status Check | P1 | Health and capability detection |
| Error Handling | P1 | Proper error propagation |

### Advanced Features

| Feature | Priority | Description |
|---------|----------|-------------|
| Streaming Response | P2 | SSE streaming for long responses |
| Mission Control | P2 | Create and monitor missions |
| File Upload | P2 | Send files to agent |
| Webhook Triggers | P2 | Agent → n8n event flow |
| Batch Operations | P3 | Process multiple items efficiently |
| Session Management | P3 | Persistent conversation contexts |

### Premium Features (Future)

| Feature | Description |
|---------|-------------|
| Co-Agent Mode | Spawn sub-agents from n8n |
| Visual Workflow Builder | Edit missions from n8n UI |
| Analytics Dashboard | Token usage, latency metrics |
| Multi-Agent Coordination | Connect multiple AuraGo instances |

---

## Part 4: Implementation Phases

### Phase 1: Foundation (Week 1)
- [ ] Add n8n config to `config_types.go`
- [ ] Create `/api/n8n/status` endpoint
- [ ] Implement token scope validation
- [ ] Create basic n8n node package structure
- [ ] Implement credentials and connection test

### Phase 2: Core Features (Week 2)
- [ ] Implement `/api/n8n/chat` endpoint
- [ ] Implement `/api/n8n/tools` endpoints
- [ ] Create n8n "Chat" operation
- [ ] Create n8n "Execute Tool" operation
- [ ] Add error handling and retry logic

### Phase 3: Advanced Features (Week 3)
- [ ] Implement `/api/n8n/memory/*` endpoints
- [ ] Implement `/api/n8n/missions` endpoint
- [ ] Create n8n "Memory" and "Mission" operations
- [ ] Add session management
- [ ] Implement webhook triggers (Agent → n8n)

### Phase 4: Polish & Release (Week 4)
- [ ] Streaming response support
- [ ] Comprehensive documentation
- [ ] Example workflows
- [ ] npm package publication
- [ ] AuraGo PR for backend changes

---

## Part 5: Configuration Examples

### AuraGo Config

```yaml
# config.yaml
n8n:
  enabled: true
  readonly: false                    # Set true for read-only access
  webhook_base_url: "https://n8n.example.com"  # Optional: for triggers
  allowed_events:                    # Filter events
    - "agent.response"
    - "agent.error"
    - "mission.completed"
  require_token: true                # Always true for security
```

### n8n Workflow Examples

#### Example 1: Simple Chat
```json
{
  "nodes": [
    {
      "parameters": {
        "operation": "chat",
        "message": "={{ $json.user_message }}",
        "session_id": "={{ $json.session_id }}",
        "context_window": 5
      },
      "name": "AuraGo Chat",
      "type": "n8n-nodes-aurago.auraGo",
      "typeVersion": 1,
      "position": [450, 300]
    }
  ]
}
```

#### Example 2: File Processing with Tool
```json
{
  "nodes": [
    {
      "parameters": {
        "operation": "executeTool",
        "tool_name": "pdf_extractor",
        "parameters": {
          "file_path": "={{ $json.file_path }}",
          "extract_text": true
        }
      },
      "name": "Extract PDF",
      "type": "n8n-nodes-aurago.auraGo",
      "typeVersion": 1
    }
  ]
}
```

#### Example 3: Memory-Enhanced Support Bot
```json
{
  "nodes": [
    {
      "parameters": {
        "operation": "memory",
        "memory_operation": "search",
        "query": "={{ $json.customer_question }}",
        "type": "long_term"
      },
      "name": "Search Knowledge",
      "type": "n8n-nodes-aurago.auraGo"
    },
    {
      "parameters": {
        "operation": "chat",
        "message": "Customer asked: {{ $json.customer_question }}\n\nRelevant knowledge: {{ $node['Search Knowledge'].json.results }}"
      },
      "name": "Generate Response",
      "type": "n8n-nodes-aurago.auraGo"
    }
  ]
}
```

---

## Part 6: Security Considerations

### Authentication
- Bearer token required for all endpoints
- Token scope validation (`n8n:*`)
- Token expiry support
- HMAC webhook signatures

### Authorization
- Read-only mode option
- Tool allowlists
- Rate limiting per token
- IP allowlist option

### Data Protection
- TLS required for production
- No sensitive data in webhook payloads
- Token prefix logging only (no full tokens)
- Configurable webhook payload filtering

---

## Part 7: npm Package Publication

### Package Metadata

```json
{
  "name": "n8n-nodes-aurago",
  "version": "1.0.0",
  "description": "n8n community node for AuraGo AI Agent",
  "keywords": ["n8n", "n8n-community-node", "aurago", "ai", "agent"],
  "license": "MIT",
  "homepage": "https://github.com/antibyte/aurago/tree/main/n8n-nodes-aurago",
  "author": {
    "name": "AuraGo Team",
    "email": "team@aurago.io"
  },
  "repository": {
    "type": "git",
    "url": "git+https://github.com/antibyte/aurago.git"
  },
  "engines": {
    "node": ">=18.10"
  },
  "peerDependencies": {
    "n8n-workflow": "*"
  }
}
```

### Installation

```bash
# In n8n community nodes
npm install n8n-nodes-aurago

# Or via n8n UI
# Settings → Community Nodes → Install
```

---

## Summary

This plan provides a complete architecture for integrating AuraGo with n8n through:

1. **Backend API** - New `/api/n8n/*` endpoints with token auth
2. **n8n Node Package** - Full-featured community node
3. **Bidirectional Flow** - n8n→Agent (API) and Agent→n8n (Webhooks)
4. **Security** - Token scopes, webhook signatures, TLS
5. **Extensibility** - Plugin architecture for future features

The implementation follows n8n best practices and AuraGo's existing patterns (MCP Server, A2A Protocol).
