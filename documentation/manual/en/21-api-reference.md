# Chapter 21: REST API Reference

AuraGo provides a comprehensive REST API for programmatic access to all features. The API follows REST principles and uses JSON for data transfer.

> 📅 **Updated:** June 7, 2026
> 🔌 **Base URL:** `http://localhost:8088` (default)

---

## Table of Contents

1. [Authentication](#authentication)
2. [Chat API](#chat-api)
3. [Memory API](#memory-api)
4. [Dashboard API](#dashboard-api)
5. [Config API](#config-api)
6. [Config Rules API](#config-rules-api)
7. [Vault API](#vault-api)
8. [Device API](#device-api)
9. [Credentials API](#credentials-api)
10. [Mission Control API](#mission-control-api)
11. [Container API](#container-api)
12. [Desktop API](#desktop-api)
13. [Pixel Image Editor API](#pixel-image-editor-api)
14. [Skills API](#skills-api)
15. [Knowledge API](#knowledge-api)
16. [Cheat Sheets API](#cheat-sheets-api)
17. [Contacts API](#contacts-api)
18. [SQL Connections API](#sql-connections-api)
19. [Cron API](#cron-api)
20. [Background Tasks API](#background-tasks-api)
21. [Indexing API](#indexing-api)
22. [Backup API](#backup-api)
23. [Update API](#update-api)
24. [Budget API](#budget-api)
25. [Upload API](#upload-api)
26. [Embeddings API](#embeddings-api)
27. [A2A Protocol API](#a2a-protocol-api)
28. [Invasion Control API](#invasion-control-api)
29. [Music Generation API](#music-generation-api)
30. [Video Generation API](#video-generation-api)
31. [Document Creator API](#document-creator-api)
32. [Knowledge Graph API](#knowledge-graph-api)
33. [Planner API](#planner-api)
34. [Helper LLM API](#helper-llm-api)
35. [Notification API](#notification-api)
36. [Push API](#push-api-pwa)
37. [Webhook API](#webhook-api)
38. [Token API](#token-api)
39. [Provider API](#provider-api)
40. [Ollama API](#ollama-api)
41. [Security Proxy API](#security-proxy-api)
42. [OpenRouter API](#openrouter-api)
43. [Personality API](#personality-api)
44. [Sandbox API](#sandbox-api)
45. [Security API](#security-api)
46. [System API](#system-api)
47. [i18n API](#i18n-api)
48. [Setup API](#setup-api)
49. [Health Check](#health-check)
50. [Agent Skills API](#agent-skills-api)
51. [Daemon Skills API](#daemon-skills-api)
52. [Agent Questions API](#agent-questions-api)
53. [People API](#people-api)
54. [Appointments API](#appointments-api)
55. [Todos API](#todos-api)
56. [Preferences API](#preferences-api)
57. [Space Agent API](#space-agent-api)
58. [Warnings API](#warnings-api)
59. [SSE Events](#sse-events)
60. [Error Handling](#error-handling)

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
  "password_set": true,
  "totp_enabled": false,
  "authenticated": true
}
```

### Set Password
```http
POST /api/auth/password
Content-Type: application/json

{
  "password": "new-secret",
  "current_password": "old-secret"
}
```

Sets or changes the login password. Allowed without authentication only when no password is set yet (first-time setup); otherwise requires an active session.

### TOTP Setup
```http
GET /api/auth/totp/setup
```

Generates a new TOTP secret and returns the `otpauth` URI. Requires authentication. Does not activate 2FA until confirmed.

### TOTP Confirm
```http
POST /api/auth/totp/confirm
Content-Type: application/json

{
  "code": "123456"
}
```

Verifies the first TOTP code and activates two-factor authentication.

### Disable TOTP
```http
DELETE /api/auth/totp
```

Disables TOTP authentication. Requires authentication.

### Login
```http
POST /auth/login
Content-Type: application/x-www-form-urlencoded

password=secret&totp_code=123456
```

**Response:**
```json
{
  "success": true,
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
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

**Response:**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you?"
      },
      "finish_reason": "stop"
    }
  ]
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

### List Chat Sessions
```http
GET /api/chat/sessions
```

Returns recent chat sessions from short-term memory.

### Create Chat Session
```http
POST /api/chat/sessions
```

Creates a new chat session and returns its metadata.

### Get Chat Session
```http
GET /api/chat/sessions/{id}
```

Returns session metadata and visible messages for the session.

### Delete Chat Session
```http
DELETE /api/chat/sessions/{id}
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

**Response:**
```json
{
  "success": true,
  "archived": 5
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

Returns runtime activity including cron jobs, processes, webhooks, background tasks, and active **co-agents** (`coagents` field).

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

**Response:**
```json
{
  "config": {
    "server": {
      "host": "127.0.0.1",
      "port": 8088
    },
    "llm": {
      "provider": "main"
    }
  }
}
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

## Config Rules API

Task rules are editable Markdown guardrails stored under `prompts/rules/<id>/rule.md`. All endpoints require admin access.

### List Rules
```http
GET /api/config/rules
```

Returns the enabled state, rule metadata, and candidate tools/workflows for the rule editor.

### Create Rule
```http
POST /api/config/rules
Content-Type: application/json

{
  "id": "homepage",
  "title": "Homepage rules",
  "enabled": true,
  "priority": 50,
  "tools": ["homepage"],
  "workflows": ["website"],
  "keywords": ["landing page"],
  "body": "Markdown rule body",
  "design": "Optional DESIGN.md content"
}
```

### Get / Update / Delete Rule
```http
GET /api/config/rules/{id}
PUT /api/config/rules/{id}
DELETE /api/config/rules/{id}
```

### Restore Built-In Rule
```http
POST /api/config/rules/{id}/restore
```

Removes the disk override so the embedded rule is used again.

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

**Response:**
```json
{
  "devices": []
}
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

**Response:**
```json
{
  "id": "dev-123",
  "success": true
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

**Response:**
```json
{
  "id": "mission-456",
  "status": "queued"
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

**Response:**
```json
{
  "containers": []
}
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

## Desktop API

Desktop endpoints use the same session/auth model as the Web UI. Mutating operations require desktop admin/write permissions and respect `virtual_desktop.readonly`.

### Desktop State
```http
GET /api/desktop/apps
GET /api/desktop/shortcuts
GET /api/desktop/widgets
GET /api/desktop/settings
POST /api/desktop/settings
GET /api/desktop/embed-token
```

### Desktop Chat and Streams
```http
POST /api/desktop/chat
GET /api/desktop/chat/stream
GET /api/desktop/ws
GET /api/agodesk/ws
GET /api/agodesk/media/{bucket}/{path}
```

`/api/agodesk/ws` uses the AgoDesk JSON envelope protocol. Production clients must pair with `session.start`; loopback development may use `?insecure_loopback=1`. After `session.accepted`, store `advertised_capabilities` and use the accepted `session_id` as the transport session.

AgoDesk chat can share AuraGo Web Chat sessions when `chat.sessions` is negotiated:

- `chat.sessions.list` returns recent `sess-*` conversations.
- `chat.session.create` starts New Chat and returns `chat.session`.
- `chat.session.load` loads a conversation and visible non-internal messages.
- `chat.message` should include both the AgoDesk `session_id` and active `conversation_id`.
- `chat.cancel` stops the active request and returns `chat.cancelled`.
- `chat.audio` is emitted only when `chat.audio_events` is negotiated. AuraGo-generated TTS paths use `/api/agodesk/tts/<filename>` so AgoDesk can fetch audio without a Web UI login cookie. `chat.voice_output` is offered only when AuraGo TTS is configured.
- `chat.media` is emitted when `chat.media_events` is negotiated for non-TTS images, audio/music, documents, videos, STL, links, and YouTube embeds. Protected `/files/...` assets are rewritten to short-lived signed `/api/agodesk/media/...` URLs; clients must keep the provided query parameters and refresh media metadata after `401`.
- `chat.voice_output.status` lets AgoDesk report the same `speaker_mode` preference used by Web Chat when speech output is enabled or disabled.
- `integrations.webhosts.list` returns the same integration webhost list used by the Web Chat integrations drawer when `integrations.webhosts` is negotiated.
- `system.warnings.list` and `system.warning.acknowledge` provide the Web Chat system warning snapshot and acknowledgement flow when `system.warnings` is negotiated.

See [AgoDesk backend protocol](../../agodesk_backend_protocol.md) for payload shapes and the client implementation checklist.

### Remote Desktop Proxies
```http
GET /api/desktop/ssh
GET /api/desktop/vnc
```

### Desktop Software Store
```http
GET /api/desktop/store/catalog
GET /api/desktop/store/apps
POST /api/desktop/store/install
GET /api/desktop/store/operations/{operation_id}
POST /api/desktop/store/apps/{app_id}/{start|stop|restart|update}
DELETE /api/desktop/store/apps/{app_id}?delete_data=false
GET /api/desktop/store/apps/{app_id}/open-url?port_id=web
GET /api/desktop/store/apps/{app_id}/credentials
POST /api/desktop/store/apps/beszel/companions/agent/config
```

Install request:
```json
{
  "app_id": "termix",
  "bind_mode": "local"
}
```

Store mutations return `202 Accepted` with an operation object. Poll `/api/desktop/store/operations/{operation_id}` until the operation reaches a terminal state.

### Code Studio
```http
GET /api/code-studio/status
GET /api/code-studio/files?path=/workspace
GET /api/code-studio/file?path=/workspace/main.go
PUT /api/code-studio/file
PATCH /api/code-studio/file
DELETE /api/code-studio/file?path=/workspace/main.go
POST /api/code-studio/directory
POST /api/code-studio/upload
GET /api/code-studio/download?path=/workspace/main.go
POST /api/code-studio/exec
GET /api/code-studio/search?q=needle&path=/workspace
GET /api/code-studio/terminal
```

Code Studio paths are sanitized inside the container workspace and mounted at `/workspace`.

---

## Pixel Image Editor API

Pixel is the desktop image editor. It can save canvas data to the virtual desktop workspace and optionally use the configured image-generation provider.

### Capabilities
```http
GET /api/pixel/config
```

Returns whether image generation is enabled, the resolved provider/model, default size/quality/style, and image-to-image support.

### Generate Image
```http
POST /api/pixel/generate
Content-Type: application/json

{
  "prompt": "A clean product photo of a brass lamp",
  "size": "1024x1024",
  "quality": "standard",
  "style": "natural"
}
```

### Enhance Image
```http
POST /api/pixel/enhance
Content-Type: application/json

{
  "source_path": "agent_workspace/virtual_desktop/image.png",
  "prompt": "Improve sharpness and lighting",
  "strength": 0.7
}
```

`source_data` may be provided instead of `source_path` as a base64 data URL.

### Save Canvas
```http
POST /api/pixel/save
Content-Type: application/json

{
  "path": "Images/edited.png",
  "data": "data:image/png;base64,...",
  "format": "png",
  "quality": 92
}
```

Relative paths are stored under the configured data workspace.

---

## Skills API

### List Skills
```http
GET /api/skills
```

**Response:**
```json
{
  "skills": []
}
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

**Response:**
```json
{
  "id": "skill-123",
  "success": true
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

### Get Skill Manual
```http
GET /api/skills/{id}/documentation
```

**Response:**
```json
{
  "status": "ok",
  "skill_id": "user_my_skill_1714123456789",
  "has_documentation": true,
  "content": "# my_skill\n\nDoes useful things.\n"
}
```

Returns `has_documentation: false` and `content: ""` when no manual exists.
Returns `404` if the skill ID is not found.

### Create / Replace Skill Manual
```http
PUT /api/skills/{id}/documentation
Content-Type: application/json

{
  "content": "# my_skill\n\nMarkdown content here.\n"
}
```

**Response:**
```json
{ "status": "saved" }
```

| Status | Meaning |
|--------|---------|
| `200`  | Manual saved |
| `400`  | Skill ID missing or content invalid |
| `403`  | Skill Manager is in read-only mode |
| `413`  | Content exceeds 64 KB limit |

Sending an empty `content` value deletes the existing manual.

### Delete Skill Manual
```http
DELETE /api/skills/{id}/documentation
```

**Response:**
```json
{ "status": "deleted" }
```

Returns `403` if the Skill Manager is in read-only mode. Deleting a non-existent manual is a no-op.

### Upload Skill Manual File
```http
POST /api/skills/{id}/documentation/upload
Content-Type: multipart/form-data

file=<binary>  (field name: "file")
```

Accepts `.md`, `.markdown`, and `.txt` files. Maximum size: 64 KB.

**Response:**
```json
{ "status": "uploaded" }
```

| Status | Meaning |
|--------|---------|
| `200`  | File stored |
| `400`  | Wrong file extension |
| `403`  | Read-only mode or uploads disabled (`allow_uploads: false`) |
| `413`  | File exceeds 64 KB limit |

### Daemon Skill Settings
```http
GET /api/skills/{id}/daemon
PUT /api/skills/{id}/daemon
```

Reads or updates daemon-specific manifest settings (`wake_agent`, `trigger_mission_id`, `cheatsheet_id`) for daemon skills.

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

**Response:**
```json
{
  "cheatsheets": []
}
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

**Response:**
```json
{
  "contacts": []
}
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

**Response:**
```json
{
  "jobs": []
}
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

**Response:**
```json
{
  "backup_path": "data/backups/aurago_backup_20260328_020000.ago",
  "size_bytes": 10485760
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

**Response:**
```json
{
  "today_usd": 0.5,
  "limit_usd": 5.0
}
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

**Response:**
```json
{
  "plans": []
}
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

**Response:**
```json
{
  "webhooks": []
}
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

**Response:**
```json
{
  "id": "webhook-456",
  "success": true
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

**Response:**
```json
{
  "id": "token-789",
  "token": "agt_...",
  "success": true
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

**Response:**
```json
{
  "success": true
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

### Setup Profiles
```http
GET /api/setup/profiles
```

Returns pre-configured provider profiles for the setup wizard plan selection step.

### Test Provider Connection
```http
POST /api/setup/test
Content-Type: application/json

{
  "provider_type": "openrouter",
  "base_url": "https://openrouter.ai/api/v1",
  "api_key": "sk-or-...",
  "model": "openai/gpt-4o"
}
```

Performs a lightweight LLM connectivity test before saving setup. Only available while setup is incomplete.

### Save Setup
```http
POST /api/setup
Content-Type: application/json

{
  "llm_provider": "openrouter",
  "api_key": "sk-or-..."
}
```

**Response:**
```json
{
  "success": true,
  "configured": true
}
```

Requires the CSRF token issued by `GET /api/setup/status`.

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

### Readiness
```http
GET /api/ready
```

Returns `200` with `{"status":"ready"}` when initialization is complete, or `503` with `{"status":"initializing"}` while the server is still starting. Used by Docker health checks and load balancers.

---

## Agent Skills API

Agent Skills are filesystem-based skills managed separately from the classic Skill Manager.

### List Agent Skills
```http
GET /api/agent-skills
GET /api/agent-skills?enabled=true&search=keyword
```

### Create Agent Skill
```http
POST /api/agent-skills
Content-Type: application/json

{
  "name": "my_skill",
  "description": "Does useful things",
  "body": "# SKILL.md content"
}
```

### Import Agent Skill
```http
POST /api/agent-skills/import
Content-Type: multipart/form-data
```

### Manage Agent Skill
```http
GET /api/agent-skills/{id}
GET /api/agent-skills/{id}?content=true
PUT /api/agent-skills/{id}
DELETE /api/agent-skills/{id}
```

### Verify / Approve / Test
```http
POST /api/agent-skills/{id}/verify
POST /api/agent-skills/{id}/approve-warning
POST /api/agent-skills/{id}/test
```

### Agent Skill Files
```http
GET /api/agent-skills/{id}/files?path=scripts/run.py
PUT /api/agent-skills/{id}/files
POST /api/agent-skills/{id}/files
```

---

## Daemon Skills API

Long-running daemon skills supervised by the daemon subsystem.

### List Daemons
```http
GET /api/daemons
```

### Refresh Daemon List
```http
POST /api/daemons/refresh
```

Rescans skills from disk and reconciles running daemons.

### Daemon Status / Actions
```http
GET /api/daemons/{id}
POST /api/daemons/{id}/start
POST /api/daemons/{id}/stop
POST /api/daemons/{id}/reenable
```

---

## Agent Questions API

Interactive question prompts shown in the Web UI while the agent waits for user input.

### Question Status
```http
GET /api/agent/question-status?session=default
```

Returns `{"status":"none"}` or `{"status":"pending","question":{...}}`.

### Submit Answer
```http
POST /api/agent/question-response
Content-Type: application/json

{
  "session_id": "default",
  "selected_value": "option_a",
  "free_text": ""
}
```

---

## People API

Knowledge-graph and contacts helpers for person-centric views.

### Person Lookup
```http
GET /api/people/lookup?q=name&mode=fts
```

Searches the knowledge graph for person nodes and related edges.

### KG Persons
```http
GET /api/people/kg-persons?limit=100
```

Returns person nodes from the knowledge graph.

### Upcoming Birthdays
```http
GET /api/people/upcoming?days=30
```

Returns upcoming birthdays from the contacts database.

---

## Appointments API

### List / Create Appointments
```http
GET /api/appointments?q=search&status=scheduled
POST /api/appointments
```

### Manage Appointment
```http
GET /api/appointments/{id}
PUT /api/appointments/{id}
DELETE /api/appointments/{id}
```

---

## Todos API

### List / Create Todos
```http
GET /api/todos?q=search&status=open
POST /api/todos
```

### Manage Todo
```http
GET /api/todos/{id}
PUT /api/todos/{id}
DELETE /api/todos/{id}
POST /api/todos/{id}/complete
```

### Todo Items
```http
POST /api/todos/{id}/items
PUT /api/todos/{id}/items/{item_id}
DELETE /api/todos/{id}/items/{item_id}
POST /api/todos/{id}/items/reorder
```

---

## Preferences API

Per-session UI preferences that influence agent behavior.

### Get Preferences
```http
GET /api/preferences
```

**Response:**
```json
{
  "speaker_mode": false
}
```

### Update Preferences
```http
POST /api/preferences
Content-Type: application/json

{
  "speaker_mode": true
}
```

---

## Space Agent API

Space Agent sidecar integration for external messaging bridges.

### Status
```http
GET /api/space-agent/status
```

### Recreate Sidecar
```http
POST /api/space-agent/recreate
```

### Send Message
```http
POST /api/space-agent/send
```

### Bridge Messages
```http
POST /api/space-agent/bridge/messages
```

Inbound bridge endpoint with its own Bearer token authentication.

---

## Warnings API

Runtime health warnings surfaced in the Web UI.

### List Warnings
```http
GET /api/warnings
```

Returns all warnings plus `total` and `unacknowledged` counts.

### Acknowledge Warnings
```http
POST /api/warnings/acknowledge
Content-Type: application/json

{"id": "warning_id"}
```

Or acknowledge all:

```json
{"all": true}
```

---

## 3D Printer API

### Test Connection
```http
GET /api/3d-printers/test
```

### Camera Snapshot / Stream
```http
GET /api/3d-printers/{printer_id}/camera/snapshot
GET /api/3d-printers/{printer_id}/camera/stream
```

**Test response example:**
```json
{
  "printers": [
    {
      "name": "elegoo",
      "type": "elegoo_centauri_carbon",
      "status": "printing",
      "progress": 42.5
    }
  ]
}
```

---

## AgentMail API

### Get Status
```http
GET /api/agentmail/status
```

**Response:**
```json
{
  "enabled": true,
  "inbox_id": "inbox_123",
  "address": "aurago@agentmail.io",
  "unread_count": 3
}
```

### Test Connection
```http
POST /api/agentmail/test
```

**Response:**
```json
{
  "success": true,
  "message": "Connection successful"
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
