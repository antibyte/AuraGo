# Chapter 12: Invasion Control

> ⚠️ **Important:** Invasion Control is available via **Web UI** and **REST API** only. Dedicated CLI commands for nest/egg management are not implemented. The agent can also use the `invasion_control` tool when enabled.

Invasion Control deploys **AuraGo sub-agents** (Eggs) to remote or local targets (Nests). The master pushes a worker binary plus generated `config.yaml`, the Egg starts in **egg mode**, and connects back to the master over WebSocket.

> **Note:** Eggs are **LLM sub-agent configuration templates**, not shell scripts, cron jobs, or Docker image definitions. Nests and Eggs are stored in the invasion SQLite database, not in `config.yaml`.

---

## Concepts: Nests & Eggs

### Nests (deployment targets)

A **Nest** describes *where* an Egg is deployed:

| Field | Values | Description |
|-------|--------|-------------|
| `access_type` | `ssh`, `docker`, `local` | How the master reaches the target |
| `deploy_method` | `ssh`, `docker_remote`, `docker_local` | How the Egg binary is deployed |
| `route` | `direct`, `ssh_tunnel`, `tailscale`, `wireguard`, `custom` | How the Egg reaches the master WebSocket |
| `target_arch` | `linux/amd64`, `linux/arm64` | Binary architecture to deploy |
| `egg_id` | UUID | Assigned Egg template (required for hatch) |
| `hatch_status` | see below | Current deployment state |

Supported access types are **SSH**, **Docker API**, and **Local** only. Kubernetes is not implemented.

### Eggs (sub-agent templates)

An **Egg** describes *how* the deployed worker behaves:

| Field | Description |
|-------|-------------|
| `name`, `description` | Human-readable labels |
| `model`, `provider`, `base_url` | LLM settings (used when `inherit_llm` is false) |
| `api_key_ref` | Vault reference for the Egg's API key |
| `inherit_llm` | Use the master's LLM config instead of Egg-specific fields (default: true) |
| `allowed_tools` | JSON array of tool IDs, e.g. `["shell","python"]` (empty = shell + python) |
| `egg_port` | HTTP port on the target (default: `8099`) |
| `permanent` | Install as systemd service (`true`) or run once (`false`) |
| `include_vault` | Ship an encrypted vault export to the target (use only on trusted hosts) |
| `active` | Whether the Egg can be assigned |

```
┌─────────────────────────────────────────────────────────────┐
│  AuraGo Master (HQ)                                         │
│                                                             │
│  Eggs (templates)          Nests (targets)                  │
│  ├─ analytics-agent        ├─ prod-server (SSH)             │
│  ├─ edge-worker            ├─ docker-host (Docker API)      │
│  └─ inherit-llm-default   └─ local-docker (local)          │
│           │                         │                       │
│           └──────── Hatch ──────────┘                       │
│                     │                                       │
│                     ▼                                       │
│            Deployed Egg (egg_mode worker)                   │
│            connects via WS → /api/invasion/ws               │
└─────────────────────────────────────────────────────────────┘
```

---

## Prerequisites

```yaml
# config.yaml
web_config:
  enabled: true          # required for /api/invasion/* REST endpoints

invasion_control:
  enabled: false         # exposes the invasion_control agent tool (default: false)
  readonly: false        # true = block hatch/stop/send_task/send_secret and other mutations

sqlite:
  invasion_path: ./data/invasion.db   # nests, eggs, tasks, deployment history
```

The Web UI page is always registered at `/invasion`. REST API routes are available when `web_config.enabled` is true and the invasion database initialized successfully.

When `invasion_control.readonly` is `true`, mutating API calls (hatch, stop, send-task, send-secret, safe-reconfigure, rollback, rotate-key, etc.) return HTTP 403.

---

## Web UI

Open **Invasion Control** at `/invasion` (also reachable from the radial menu).

The UI has **two tabs only**:

| Tab | Purpose |
|-----|---------|
| **Nests** | Manage deployment targets, assign Eggs, hatch, stop, reconfigure |
| **Eggs** | Manage sub-agent LLM configuration templates |

There is **no Deployments tab**. Deployment history is available via the REST API (`/api/invasion/nests/{id}/deployments`).

### Nest card actions

- **Edit** — connection settings, assigned Egg, deploy method, route
- **Hatch** — deploy the assigned Egg (when status is `idle`, `failed`, or `stopped`)
- **Stop** — stop the running Egg
- **Safe Reconfigure** — apply a whitelisted config patch without full redeploy
- **Config History** — view and roll back safe config revisions
- **Activate / Deactivate** — toggle `active`
- **Delete** — requires typing the exact nest name

### Egg card actions

- **Edit** — LLM settings, tools, port, permanent/vault/inherit flags
- **Activate / Deactivate**
- **Delete** — requires typing the exact egg name

---

## Creating a Nest

### Via Web UI

1. Open the **Nests** tab → **Create New**
2. Fill in the form:

| Field | Notes |
|-------|-------|
| Name | Required |
| Notes | Optional |
| Access Type | `SSH`, `Docker API`, or `Local` |
| Host / Port / Username | Required for SSH and Docker; hidden for Local |
| Secret | SSH key or password; stored in vault (not returned by API) |
| Assign Egg | Select an Egg or leave empty |
| Deploy Method | `SSH`, `Docker (Remote)`, or `Docker (Local)` |
| Target Architecture | `linux/amd64` or `linux/arm64` |
| Route | How the Egg reaches the master WebSocket |
| Route Config | JSON, e.g. `{"tunnel_port":8443}` or a full WebSocket URL for `custom` |

3. Save, then use **Test Connection** (edit mode only) to validate reachability

### Via REST API

```bash
curl -X POST http://localhost:8088/api/invasion/nests \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production-server-01",
    "access_type": "ssh",
    "host": "192.168.1.10",
    "port": 22,
    "username": "deploy",
    "secret": "-----BEGIN OPENSSH PRIVATE KEY-----\n...",
    "deploy_method": "ssh",
    "target_arch": "linux/amd64",
    "route": "direct",
    "active": true
  }'
```

Test the connection:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/validate
```

Response:

```json
{
  "success": true,
  "message": "Connection successful (45ms)",
  "time_ms": 45
}
```

> 💡 **Tip:** Store SSH keys and passwords in the vault via the UI/API at creation time. Secrets are never included in list/get responses (`has_secret: true` indicates a stored credential).

---

## Creating an Egg

### Via Web UI

1. Open the **Eggs** tab → **Create New**
2. Configure:

| Field | Notes |
|-------|-------|
| Name | Required |
| Description | What this sub-agent does |
| Provider / Model / Base URL | Used when **Inherit LLM** is off |
| API Key | Stored in vault (`has_api_key` in API responses) |
| Egg Port | Default `8099` |
| Allowed Tools | JSON array, e.g. `["shell","python"]` |
| Permanent | Systemd service vs. one-shot run |
| Include Vault | Export master vault to target (security-sensitive) |
| Inherit LLM | Use master's LLM settings (default: on) |

### Via REST API

```bash
curl -X POST http://localhost:8088/api/invasion/eggs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "edge-analytics",
    "description": "Lightweight analytics sub-agent",
    "inherit_llm": true,
    "egg_port": 8099,
    "allowed_tools": "[\"shell\",\"python\"]",
    "permanent": true,
    "active": true
  }'
```

Assign the Egg to a Nest (via UI dropdown or API):

```bash
curl -X PUT http://localhost:8088/api/invasion/nests/{nest-id} \
  -H "Content-Type: application/json" \
  -d '{"egg_id": "{egg-id}", "name": "production-server-01", ...}'
```

---

## Hatching (Deploying an Egg)

**Hatch** deploys the assigned Egg to the Nest:

1. Master generates a shared HMAC key and Egg `config.yaml` (with `egg_mode` enabled)
2. Binary (`linux/amd64` or `linux/arm64`), `resources.dat`, and config are transferred
3. Egg process starts on the target (systemd if `permanent`, otherwise one-shot)
4. Egg connects to `ws[s]://<master>/api/invasion/ws` and authenticates
5. Master marks the nest `running` when the WebSocket connects

### Via Web UI

1. Ensure the Nest has an Egg assigned and is **active**
2. Click **Hatch** on the Nest card
3. Status updates automatically (`hatching` → `running` or `failed`)

### Via REST API

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/hatch
```

Response:

```json
{
  "status": "hatching",
  "nest_id": "...",
  "egg_id": "..."
}
```

Poll status:

```bash
curl http://localhost:8088/api/invasion/nests/{nest-id}/status
```

```json
{
  "nest_id": "...",
  "hatch_status": "running",
  "last_hatch_at": "2026-06-07T10:23:45Z",
  "hatch_error": "",
  "ws_connected": true,
  "telemetry": { "cpu_percent": 12, "mem_percent": 34, "uptime_seconds": 86400 }
}
```

Stop a running Egg:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/stop
```

---

## Hatch Status & Lifecycle

### Nest `hatch_status` values

| Status | Meaning |
|--------|---------|
| `idle` | No active deployment (initial state) |
| `hatching` | Deployment in progress |
| `running` | Egg deployed; WebSocket connected (or recently connected) |
| `failed` | Deployment or heartbeat failure (`hatch_error` has details) |
| `stopped` | Egg was stopped manually or lost connection |

Status transitions:

```
idle ──Hatch──► hatching ──success──► running
                  │                      │
                  │ failure              ├── disconnect / stop ──► stopped
                  ▼                      │
               failed ◄── heartbeat timeout
                  │
                  └── Hatch again (from idle/failed/stopped)
```

The UI also shows:

- **WebSocket connected / disconnected** badge
- **Config drift / synced** badge (`desired_config_rev` vs `applied_config_rev`)
- **Telemetry** (CPU, memory, uptime) when connected

Heartbeat monitor: checks every 30 seconds; marks `failed` with `heartbeat timeout` after 90 seconds without a heartbeat.

---

## Routing Options

The `route` field controls how the deployed Egg reaches the master WebSocket (`/api/invasion/ws`):

| Route | Behavior |
|-------|----------|
| `direct` | Egg connects to nest `host` (or master host as fallback) |
| `ssh_tunnel` | Egg uses localhost; tunnel configured via `route_config` |
| `tailscale` | Egg connects via Tailscale IP/hostname |
| `wireguard` | Egg connects via WireGuard endpoint |
| `custom` | Full WebSocket URL in `route_config` |

For `docker_local` deployments, the master uses `host.docker.internal` so the container can reach the host.

Example `route_config` for SSH tunnel:

```json
{"tunnel_port": 8443}
```

Example `route_config` for custom route:

```
wss://aurago.example.com/api/invasion/ws
```

---

## Tasks, Artifacts & Messages

Once an Egg is connected (`ws_connected: true`), the master can interact with it:

### Send a task

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/send-task \
  -H "Content-Type: application/json" \
  -d '{"description": "Check disk usage and summarize", "timeout": 120}'
```

Task statuses: `pending` → `sent` → `acked` → `completed` / `failed` / `timeout`

```bash
curl http://localhost:8088/api/invasion/nests/{nest-id}/tasks
curl http://localhost:8088/api/invasion/tasks/{task-id}
```

### Send a runtime secret

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/send-secret \
  -H "Content-Type: application/json" \
  -d '{"key": "openrouter_api_key", "value": "sk-..."}'
```

Secrets are encrypted with the nest's shared key before transmission.

### Artifacts

Eggs can offer files back to the master:

- `POST /api/invasion/artifacts/offer` — Egg initiates upload (HMAC-signed)
- `POST /api/invasion/artifacts/upload/{token}` — Upload payload
- `GET /api/invasion/artifacts/{id}` — Download artifact

### Egg messages

- `POST /api/invasion/messages` — Egg sends alerts/notifications to the master

Pending tasks are automatically re-sent after an Egg reconnects.

---

## Safe Reconfigure & Config History

**Safe Reconfigure** applies whitelisted changes to a running Egg without a full redeploy. Available in the Web UI (🔧 button) or via API:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/safe-reconfigure \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "allowed_tools": ["shell", "python"],
    "allow_filesystem_write": true,
    "allow_network_requests": true
  }'
```

Allowed patch fields:

| Field | Description |
|-------|-------------|
| `provider`, `base_url`, `model` | LLM settings (not combinable with `inherit_llm: true`) |
| `allowed_tools` | `shell`, `execute_shell_command`, `python`, `python_execute` |
| `allow_filesystem_write` | Agent filesystem write permission |
| `allow_network_requests` | Agent network access |
| `allow_remote_shell` | Remote shell permission |
| `allow_self_update` | Self-update permission |

> ⚠️ The Egg is restarted after applying changes.

View revision history:

```bash
curl "http://localhost:8088/api/invasion/nests/{nest-id}/config-history?limit=20"
```

Roll back an applied revision:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/config-rollback \
  -H "Content-Type: application/json" \
  -d '{"revision_id": "{revision-id}"}'
```

Revision statuses: `pending`, `applying`, `applied`, `failed`, `rolled_back`

---

## Deployment Rollback & History

Deployment history is tracked per nest (API only, no UI tab):

```bash
curl http://localhost:8088/api/invasion/nests/{nest-id}/deployments
```

Deployment record statuses: `started`, `deployed`, `verified`, `failed`, `rolled_back`

Manual rollback to the previous deployment backup:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/rollback
```

Rotate the master↔egg shared key on a connected nest:

```bash
curl -X POST http://localhost:8088/api/invasion/nests/{nest-id}/rotate-key
```

If a health check fails after deploy, the system attempts **automatic rollback**.

---

## Egg Mode (Worker Configuration)

Deployed Eggs run with `egg_mode` enabled in their generated `config.yaml`:

```yaml
egg_mode:
  enabled: true
  master_url: "wss://aurago.example.com/api/invasion/ws"
  shared_key: ""         # hex-encoded AES-256 key (set at deploy time)
  egg_id: ""
  nest_id: ""
  tls_skip_verify: false # set true for self-signed master TLS
```

The master generates this configuration during hatch. You do not edit `egg_mode` manually for managed Eggs.

---

## Agent Tool: `invasion_control`

When `invasion_control.enabled` is true, the agent can manage nests and eggs programmatically:

| Operation | Description |
|-----------|-------------|
| `list_nests`, `list_eggs` | List all records (no secrets) |
| `nest_status`, `egg_status` | Status lookup |
| `assign_egg` | Assign an Egg to a Nest |
| `hatch_egg`, `stop_egg` | Deploy or stop |
| `send_task`, `task_status`, `get_result` | Task management |
| `send_secret` | Send runtime secret to connected Egg |
| `list_artifacts`, `get_artifact`, `read_artifact` | Artifact access |
| `list_egg_messages`, `ack_egg_message` | Egg notifications |
| `upload_artifact`, `send_host_message` | Host → Egg communication |

See [Chapter 22: Internal Tools](22-internal-tools.md) for full parameter details.

---

## REST API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/invasion/nests` | GET, POST | List / create nests |
| `/api/invasion/nests/{id}` | GET, PUT, DELETE | Get / update / delete nest |
| `/api/invasion/nests/{id}/toggle` | POST | Enable/disable nest |
| `/api/invasion/nests/{id}/validate` | POST | Test connection |
| `/api/invasion/nests/{id}/hatch` | POST | Deploy assigned Egg |
| `/api/invasion/nests/{id}/stop` | POST | Stop running Egg |
| `/api/invasion/nests/{id}/status` | GET | Hatch status + telemetry |
| `/api/invasion/nests/{id}/send-task` | POST | Send task to connected Egg |
| `/api/invasion/nests/{id}/send-secret` | POST | Send encrypted secret |
| `/api/invasion/nests/{id}/tasks` | GET | Task history |
| `/api/invasion/nests/{id}/rotate-key` | POST | Rotate shared key |
| `/api/invasion/nests/{id}/rollback` | POST | Roll back deployment |
| `/api/invasion/nests/{id}/deployments` | GET | Deployment history |
| `/api/invasion/nests/{id}/safe-reconfigure` | POST | Apply safe config patch |
| `/api/invasion/nests/{id}/config-history` | GET | Config revision history |
| `/api/invasion/nests/{id}/config-rollback` | POST | Roll back config revision |
| `/api/invasion/eggs` | GET, POST | List / create eggs |
| `/api/invasion/eggs/{id}` | GET, PUT, DELETE | Get / update / delete egg |
| `/api/invasion/eggs/{id}/toggle` | POST | Enable/disable egg |
| `/api/invasion/tasks/{id}` | GET | Get task by ID |
| `/api/invasion/artifacts/offer` | POST | Egg artifact offer |
| `/api/invasion/artifacts/upload/{token}` | POST | Upload artifact |
| `/api/invasion/artifacts/{id}` | GET | Download artifact |
| `/api/invasion/messages` | POST | Egg message ingestion |
| `/api/invasion/ws` | WS | Egg ↔ master bridge |

---

## Troubleshooting

### Connection refused / timeout

1. Verify the target is reachable (`ping`, `ssh`)
2. Check firewall rules and correct port (22 for SSH, 2375 for Docker API)
3. Run **Test Connection** or `POST .../validate`
4. For SSH nests, ensure a secret is configured

### Authentication failed

1. Verify username and SSH key/password
2. Check key permissions locally (`chmod 600`)
3. Confirm `authorized_keys` on the target

### Hatch failed

1. Check `hatch_error` on the nest (UI or `GET /api/invasion/nests/{id}`)
2. Ensure the correct `target_arch` binary exists on the master
3. For Docker deployments, verify daemon access and `deploy_method`
4. Review server logs for deployment details

### Egg not connecting (stuck at `running` but `ws_connected: false`)

1. Verify `route` and `route_config` — the Egg must reach the master WebSocket
2. For `docker_local`, ensure the container can reach `host.docker.internal`
3. For HTTPS masters, check TLS/`tls_skip_verify` settings
4. Check firewall rules on the master port

### Heartbeat timeout → `failed`

The Egg lost its WebSocket connection or stopped responding. Re-hatch or investigate the remote process.

| Error | Likely cause | Fix |
|-------|--------------|-----|
| `No egg assigned` | Missing `egg_id` | Assign an Egg before hatching |
| `Hatch already in progress` | Concurrent hatch | Wait for current hatch to finish |
| `No active WebSocket connection` | Egg offline | Re-hatch or check remote process |
| `Shared key not found` | Missing deploy state | Re-hatch the nest |

---

## Security Notes

> ⚠️ **Important:**
> - Store SSH keys, passwords, and API keys in the vault — never in chat logs or plain config
> - `include_vault` ships encrypted vault data to the target; use only on trusted hosts
> - `inherit_llm` copies the master's API key into the Egg config — the Egg host must be trusted
> - Use `invasion_control.readonly: true` for monitoring-only setups
> - Rotate shared keys with `/rotate-key` if compromise is suspected

---

## Next Steps

- **[Chapter 11: Mission Control](11-missions.md)** — Schedule agent tasks; remote missions can target connected Eggs
- **[Chapter 13: Dashboard](13-dashboard.md)** — Overview of system health
- **[Chapter 21: API Reference](21-api-reference.md)** — Full API listing
- **[Chapter 22: Internal Tools](22-internal-tools.md)** — `invasion_control` tool details