# REST API Reference

AuraGo provides a comprehensive REST API for programmatic access to all features. The API follows REST principles and uses JSON for data transfer.

> 📅 **Updated:** April 2026
> 🔌 **Base URL:** `http://localhost:8088` (default)

---

## Table of Contents

1. [Authentication](#authentication)
2. [Chat API](#chat-api)
3. [Memory API](#memory-api)
4. [Dashboard API](#dashboard-api)
5. [Config API](#config-api)
6. [Vault API](#vault-api)
7. [Device API](#device-api)
8. [Mission Control API](#mission-control-api)
9. [Container API](#container-api)
10. [Skills API](#skills-api)
11. [Webhook API](#webhook-api)
12. [SSE Events](#sse-events)

---

## Authentication

### Check Auth Status
```http
GET /api/auth/status
```

**Response:**
```json
{
  "enabled": true,
  "configured": true,
  "totp_enabled": false
}
```

### Login
```http
POST /auth/login
Content-Type: application/x-www-form-urlencoded

password=secret&totp_code=123456
```

### Logout
```http
POST /api/auth/logout
```

---

## Chat API

### Chat Completion (OpenAI-compatible)
```http
POST /v1/chat/completions
Content-Type: application/json

{
  "messages": [
    {"role": "user", "content": "Hello AuraGo!"}
  ],
  "stream": true
}
```

### Get Chat History
```http
GET /history
```

**Response:**
```json
[
  {
    "id": "msg-123",
    "role": "user",
    "content": "Hello!",
    "timestamp": "2026-03-28T10:00:00Z"
  }
]
```

### Clear Chat History
```http
DELETE /clear
```

### Interrupt Current Action
```http
POST /api/admin/stop
```

---

## Memory API

### Archive Memory
```http
POST /api/memory/archive
Content-Type: application/json

{
  "session_id": "default",
  "before_message_id": 123
}
```

### Memory Activity Overview
```http
GET /api/memory/activity-overview
```

---

## Dashboard API

### System Metrics
```http
GET /api/dashboard/system
```

**Response:**
```json
{
  "cpu": 15.2,
  "memory": 45.8,
  "disk": 72.1,
  "network": {
    "rx_mbps": 1.2,
    "tx_mbps": 0.8
  },
  "uptime_seconds": 86400
}
```

### Mood History
```http
GET /api/dashboard/mood-history
```

### Emotion History
```http
GET /api/dashboard/emotion-history
```

### Memory Overview
```http
GET /api/dashboard/memory
```

### Core Memory
```http
GET /api/dashboard/core-memory
```

### Mutate Core Memory
```http
POST /api/dashboard/core-memory/mutate
Content-Type: application/json

{
  "action": "add",
  "content": "Important information"
}
```

### Profile Data
```http
GET /api/dashboard/profile
PUT /api/dashboard/profile/entry
```

### Activity Statistics
```http
GET /api/dashboard/activity
```

### GitHub Repositories
```http
GET /api/dashboard/github-repos
GET /api/github/repos
```

### Logs
```http
GET /api/dashboard/logs
```

### Dashboard Overview
```http
GET /api/dashboard/overview
```

### Notes
```http
GET /api/dashboard/notes
```

### Journal
```http
GET /api/dashboard/journal
GET /api/dashboard/journal/summaries
GET /api/dashboard/journal/stats
```

### Guardian Status
```http
GET /api/dashboard/guardian
```

### Error Overview
```http
GET /api/dashboard/errors
```

### Prompt Statistics
```http
GET /api/dashboard/prompt-stats
```

### Tool Statistics
```http
GET /api/dashboard/tool-stats
```

---

## Config API

### Get Configuration
```http
GET /api/config
```

### Update Configuration
```http
PUT /api/config
Content-Type: application/json

{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  }
}
```

### Configuration Schema
```http
GET /api/config/schema
```

### Change UI Language
```http
POST /api/ui-language
Content-Type: application/json

{
  "language": "de"
}
```

---

## Vault API

### Vault Status
```http
GET /api/vault/status
```

**Response:**
```json
{
  "initialized": true,
  "secret_count": 12
}
```

### List Secrets
```http
GET /api/vault/secrets
```

### Delete Secret
```http
DELETE /api/vault?key=secret_name
```

---

## Device API (Device Registry)

### List Devices
```http
GET /api/devices
```

### Create Device
```http
POST /api/devices
Content-Type: application/json

{
  "hostname": "server1",
  "device_type": "server",
  "ip_address": "192.168.1.10",
  "ssh_port": 22,
  "ssh_user": "root",
  "tags": ["production", "web"]
}
```

### Get Device
```http
GET /api/devices/{id}
```

### Update Device
```http
PUT /api/devices/{id}
Content-Type: application/json

{
  "tags": ["new", "updated"]
}
```

### Delete Device
```http
DELETE /api/devices/{id}
```

### MAC Lookup
```http
GET /api/tools/mac_lookup?mac=aa:bb:cc:dd:ee:ff
```

---

## Credentials API

### List Credentials
```http
GET /api/credentials
```

### Python-Accessible Credentials
```http
GET /api/credentials/python-accessible
```

### Create Credential
```http
POST /api/credentials
Content-Type: application/json

{
  "name": "api_key_production",
  "type": "token",
  "value": "sk-...",
  "python_accessible": false
}
```

### Update/Delete Credential
```http
PUT /api/credentials/{id}
DELETE /api/credentials/{id}
```

---

## Mission Control API

### List Missions
```http
GET /api/missions
GET /api/missions/v2
```

### Create Mission
```http
POST /api/missions/v2
Content-Type: application/json

{
  "name": "Create Backup",
  "description": "Daily database backup",
  "trigger": {
    "type": "cron",
    "schedule": "0 2 * * *"
  },
  "actions": [
    {
      "type": "shell",
      "command": "pg_dump mydb > backup.sql"
    }
  ]
}
```

### Manage Mission
```http
GET /api/missions/v2/{id}
PUT /api/missions/v2/{id}
DELETE /api/missions/v2/{id}
```

### Queue
```http
GET /api/missions/v2/queue
```

### Executions
```http
GET /api/missions/v2/execution
```

### Dependencies
```http
GET /api/missions/v2/dependencies
```

---

## Container API

### List Containers
```http
GET /api/containers
```

### Container Actions
```http
POST /api/containers/{id}/start
POST /api/containers/{id}/stop
POST /api/containers/{id}/restart
POST /api/containers/{id}/pause
POST /api/containers/{id}/unpause
DELETE /api/containers/{id}
```

### Runtime Information
```http
GET /api/runtime
```

---

## Skills API

### List Skills
```http
GET /api/skills
```

### Create Skill
```http
POST /api/skills
Content-Type: application/json

{
  "name": "My Skill",
  "description": "Description",
  "code": "..."
}
```

### Manage Skill
```http
GET /api/skills/{id}
PUT /api/skills/{id}
DELETE /api/skills/{id}
```

### Test Skill
```http
POST /api/skills/{id}/test
```

### Verify Skill
```http
POST /api/skills/{id}/verify
```

### Export Skill
```http
GET /api/skills/{id}/export
```

### Skill Versions
```http
GET /api/skills/{id}/versions
```

### Skill Audit
```http
GET /api/skills/{id}/audit
```

### Templates
```http
GET /api/skills/templates
POST /api/skills/templates
```

### Import Skill
```http
POST /api/skills/import
Content-Type: multipart/form-data
```

### Generate Skill Draft
```http
POST /api/skills/generate
Content-Type: application/json

{
  "description": "A skill that analyzes files"
}
```

### Skill Statistics
```http
GET /api/skills/stats
```

---

## Knowledge API

### Knowledge Files
```http
GET /api/knowledge
POST /api/knowledge/upload
Content-Type: multipart/form-data
```

### Manage Knowledge File
```http
GET /api/knowledge/{id}
DELETE /api/knowledge/{id}
```

---

## Cheat Sheets API

### Cheat Sheets
```http
GET /api/cheatsheets
POST /api/cheatsheets
```

### Manage Cheat Sheet
```http
GET /api/cheatsheets/{id}
PUT /api/cheatsheets/{id}
DELETE /api/cheatsheets/{id}
```

---

## Contacts API

### Contacts
```http
GET /api/contacts
POST /api/contacts
```

### Manage Contact
```http
GET /api/contacts/{id}
PUT /api/contacts/{id}
DELETE /api/contacts/{id}
```

---

## SQL Connections API

### List Connections
```http
GET /api/sql-connections
```

### Create Connection
```http
POST /api/sql-connections
Content-Type: application/json

{
  "name": "Production DB",
  "type": "postgres",
  "host": "localhost",
  "port": 5432,
  "database": "mydb"
}
```

### Test Connection
```http
POST /api/sql-connections/{id}/test
```

### Manage Connection
```http
GET /api/sql-connections/{id}
PUT /api/sql-connections/{id}
DELETE /api/sql-connections/{id}
```

---

## Cron API

### Manage Cron Jobs
```http
GET /api/cron
POST /api/cron
PUT /api/cron/{id}
DELETE /api/cron/{id}
```

---

## Background Tasks API

### List Tasks
```http
GET /api/background-tasks
```

### Get Task
```http
GET /api/background-tasks/{id}
```

---

## Indexing API

### Indexing Status
```http
GET /api/indexing/status
```

### Rescan
```http
POST /api/indexing/rescan
```

### Manage Directories
```http
GET /api/indexing/directories
POST /api/indexing/directories
```

---

## Backup API

AuraGo backup archives use the `.ago` format. Archives are ZIP-based and can be encrypted with AES-256-GCM using Argon2id-derived keys. They can include configuration, SQLite databases including WAL/SHM files, vector DB data, skills, tools, selected workspace files, and separately encrypted vault secrets for cross-instance migration.

### Create Backup
```http
POST /api/backup/create
Content-Type: application/json

{
  "include_vectordb": true,
  "include_workdir": false,
  "encrypt": true,
  "passphrase": "strong-passphrase"
}
```

### Import Backup
```http
POST /api/backup/import
Content-Type: multipart/form-data
```

Imports are staged first, checked for path traversal, schema warnings, and archive compatibility, then restored atomically where possible.

---

## Update API

### Check for Updates
```http
GET /api/updates/check
```

### Install Update
```http
POST /api/updates/install
```

### Restart
```http
POST /api/restart
```

---

## Budget API

### Budget Status
```http
GET /api/budget
```

### OpenRouter Credits
```http
GET /api/credits
```

---

## Upload API

### Upload File
```http
POST /api/upload
Content-Type: multipart/form-data
```

### Upload Voice Message
```http
POST /api/upload-voice
Content-Type: multipart/form-data
```

---

## Embeddings API

### Reset Embeddings
```http
POST /api/embeddings/reset
```

---

## A2A Protocol API

### A2A Status
```http
GET /api/a2a/status
```

### Remote Agents
```http
GET /api/a2a/remote-agents
POST /api/a2a/remote-agents
PUT /api/a2a/remote-agents/{id}
DELETE /api/a2a/remote-agents/{id}
```

### Agent Card
```http
GET /api/a2a/card
```

### A2A Test
```http
POST /api/a2a/test
```

---

## Invasion Control API

### Nests
```http
GET /api/invasion/nests
```

### Eggs
```http
GET /api/invasion/eggs
POST /api/invasion/eggs
PUT /api/invasion/eggs/{id}
DELETE /api/invasion/eggs/{id}
```

### WebSocket
```http
WS /api/invasion/ws
```

---

## Music Generation API

### Test Connection
```http
POST /api/music-generation/test
```

---

## Video Generation API

### Test Connection
```http
POST /api/video-generation/test
```

Validates provider configuration and credential availability for the configured video-generation backend.

---

## Document Creator API

### Test Gotenberg Connection
```http
POST /api/document-creator/test
```

---

## Knowledge Graph API

### Nodes
```http
GET /api/knowledge-graph/nodes
POST /api/knowledge-graph/nodes
```

### Edges
```http
GET /api/knowledge-graph/edges
POST /api/knowledge-graph/edges
```

### Node Detail
```http
GET /api/knowledge-graph/node/{id}
POST /api/knowledge-graph/node/protect
```

### Edge Mutate
```http
POST /api/knowledge-graph/edge
```

### Search / Stats / Quality
```http
GET /api/knowledge-graph/search
GET /api/knowledge-graph/stats
GET /api/knowledge-graph/quality
GET /api/knowledge-graph/important
```

`/api/knowledge-graph/quality` reports isolated nodes, untyped nodes, and likely duplicate candidates. `POST /api/knowledge-graph/node/protect` marks important nodes as protected so automated cleanup does not remove them accidentally.

### File Sync Debugging
```http
GET /api/debug/kg-file-sync-stats
GET /api/debug/kg-orphans
GET /api/debug/file-sync-status
GET /api/debug/file-sync-last-run
GET /api/debug/kg-file-entities
GET /api/debug/kg-node-sources
POST /api/debug/kg-file-sync-cleanup
```

These endpoints inspect and maintain the background File KG Sync service that extracts entities and relationships from indexed files into the Knowledge Graph.

---

## Planner API

### List Plans
```http
GET /api/plans
```

### Get Active Plan
```http
GET /api/plans/active
```

### Manage Plan
```http
GET /api/plans/{id}
PUT /api/plans/{id}
DELETE /api/plans/{id}
```

---

## Helper LLM API

### Stats
```http
GET /api/debug/helper-llm/stats
GET /api/dashboard/helper-llm
```

---

## Notification API

### Notifications
```http
GET /notifications
POST /notifications/read
```

---

## Push API (PWA)

### VAPID Public Key
```http
GET /api/push/vapid-pubkey
```

### Subscribe Push
```http
POST /api/push/subscribe
Content-Type: application/json

{
  "endpoint": "https://push.service/...",
  "keys": {
    "p256dh": "...",
    "auth": "..."
  }
}
```

### Unsubscribe Push
```http
POST /api/push/unsubscribe
```

### Push Status
```http
GET /api/push/status
```

---

## Webhook API

### List Webhooks
```http
GET /api/webhooks
```

### Create Webhook
```http
POST /api/webhooks
Content-Type: application/json

{
  "name": "GitHub Webhook",
  "url": "https://example.com/webhook",
  "events": ["push", "pull_request"]
}
```

### Manage Webhook
```http
GET /api/webhooks/{id}
PUT /api/webhooks/{id}
DELETE /api/webhooks/{id}
```

### Webhook Log
```http
GET /api/webhooks/{id}/log
GET /api/webhooks/log
```

### Test Webhook
```http
POST /api/webhooks/{id}/test
```

### Webhook Presets
```http
GET /api/webhooks/presets
```

### Webhook Receiver (public)
```http
POST /webhook/{slug}
```

---

## Token API

### List Tokens
```http
GET /api/tokens
```

### Create Token
```http
POST /api/tokens
Content-Type: application/json

{
  "name": "API Access",
  "scopes": ["read", "write"]
}
```

### Manage Token
```http
PUT /api/tokens/{id}
DELETE /api/tokens/{id}
```

---

## Provider API

### List Providers
```http
GET /api/providers
```

### Provider Pricing
```http
GET /api/providers/pricing
```

---

## Ollama API

### List Models
```http
GET /api/ollama/models
```

### Managed Ollama Status
```http
GET /api/ollama/managed/status
```

Returns the managed Docker container state, detected runtime, model volume status, GPU availability where detectable, and configured default models.

### Recreate Managed Ollama
```http
POST /api/ollama/managed/recreate
```

Recreates the managed `aurago_ollama_managed` container after configuration changes.

---

## Security Proxy API

### Proxy Lifecycle
```http
GET /api/proxy/status
POST /api/proxy/start
POST /api/proxy/stop
POST /api/proxy/destroy
POST /api/proxy/reload
GET /api/proxy/logs
```

The Security Proxy API controls the managed Caddy protection layer used for rate limiting, TLS termination, IP filtering, geo-blocking, and public-facing hardening.

---

## OpenRouter API

### OpenRouter Models
```http
GET /api/openrouter/models
```

---

## Personality API

### List Personalities
```http
GET /api/personalities
```

### Update Personality
```http
POST /api/personality
Content-Type: application/json

{
  "core_personality": "tech"
}
```

### Personality State
```http
GET /api/personality/state
```

### Personality Feedback
```http
POST /api/personality/feedback
Content-Type: application/json

{
  "mood": "happy",
  "trigger": "positive_interaction"
}
```

### Personality Files
```http
GET /api/config/personality-files
POST /api/config/personality-files
DELETE /api/config/personality-files
```

---

## Sandbox API

### Sandbox Status
```http
GET /api/sandbox/status
```

### Shell Sandbox Status
```http
GET /api/sandbox/shell-status
```

---

## Security API

### Security Status
```http
GET /api/security/status
```

### Security Hints
```http
GET /api/security/hints
```

### Auto-Hardening
```http
POST /api/security/harden
```

---

## System API

### Operating System
```http
GET /api/system/os
```

### Runtime Environment
```http
GET /api/runtime
```

---

## i18n API

### Translations
```http
GET /api/i18n?lang=en
```

---

## Setup API

### Setup Status
```http
GET /api/setup/status
```

### Save Setup
```http
POST /api/setup
Content-Type: application/json

{
  "llm_provider": "openrouter",
  "api_key": "sk-or-..."
}
```

---

## Health Check

### Health
```http
GET /api/health
```

**Response:**
```json
{
  "status": "ok"
}
```

---

## SSE Events

AuraGo supports Server-Sent Events (SSE) for real-time updates.

### Connect to Event Stream
```http
GET /events
Accept: text/event-stream
```

### Event Types

| Event | Description |
|-------|-------------|
| `system_metrics` | System metrics (CPU, RAM, etc.) |
| `container_update` | Container status changes |
| `personality_update` | Personality status |
| `tsnet_status` | Tailscale tsnet status |

**Example Stream:**
```
event: system_metrics
data: {"cpu": 15.2, "memory": 45.8, "disk": 72.1}

event: container_update
data: [{"id": "abc", "state": "running"}]
```

---

## Error Handling

The API uses standard HTTP status codes:

| Code | Meaning |
|------|---------|
| 200 | OK |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 500 | Internal Server Error |

**Error Response:**
```json
{
  "error": "Description of the error"
}
```

---

## Related Links

- [Chat Commands](20-chat-commands.md) – Alternative API via chat
- [Mission Control](11-missions.md) – Automation
- [Security](14-security.md) – Authentication & Vault
