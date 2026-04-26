# Skill Manifest Specification

AuraGo skills consist of a Python script plus a `.json` manifest in the skills directory. Use `create_skill_from_template` whenever possible. Edit the manifest only when a template-created skill needs extra metadata, vault access, native tool bridge access, or daemon settings.

## Skill Manifest Schema

```json
{
  "name": "skill_name",
  "description": "What the skill does",
  "executable": "skill_name.py",
  "category": "automation",
  "tags": ["tag1", "tag2"],
  "parameters": {
    "input": "Legacy flat parameter description"
  },
  "returns": "JSON object with status/result fields",
  "dependencies": ["requests"],
  "vault_keys": ["API_KEY"],
  "internal_tools": ["proxmox"],
  "daemon": {
    "enabled": true,
    "wake_agent": true,
    "wake_rate_limit_seconds": 60,
    "max_runtime_hours": 0,
    "restart_on_crash": true,
    "max_restart_attempts": 3,
    "restart_cooldown_seconds": 300,
    "health_check_interval_seconds": 60,
    "env": {"EXAMPLE_MODE": "safe"},
    "trigger_mission_id": "mission-uuid",
    "trigger_mission_name": "Mission display name",
    "cheatsheet_id": "cheatsheet-uuid",
    "cheatsheet_name": "Cheatsheet display name"
  }
}
```

## Fields

| Field | Required | Description |
|---|---|---|
| `name` | yes | Unique skill name. Use ASCII letters, numbers, underscores, or hyphens. |
| `description` | yes | User-facing explanation of what the skill does. |
| `executable` | yes | Plain filename inside the skills directory, usually `name.py`. |
| `category` | no | Short lowercase grouping such as `automation`, `network`, or `media`. |
| `tags` | no | Search/filter tags. |
| `parameters` | no | Either a legacy flat map (`"field": "description"`) or JSON Schema with `type`, `properties`, and `required`. |
| `returns` | no | Description of the expected output. |
| `dependencies` | no | Pip packages installed before execution. Use stdlib when possible. |
| `vault_keys` | no | Vault secret keys to inject at runtime as `AURAGO_SECRET_<KEY>`. |
| `internal_tools` | no | Native AuraGo tools this skill may call through the Python Tool Bridge. |
| `daemon` | no | Object that turns the skill into a long-running daemon. Omit or set `null` for normal skills. |

## Parameter Schema

Legacy flat maps are accepted and easiest for simple skills:

```json
{
  "parameters": {
    "query": "Search query",
    "limit": "Maximum number of results"
  }
}
```

JSON Schema is also accepted when you need required fields, types, or enums:

```json
{
  "parameters": {
    "type": "object",
    "properties": {
      "operation": {
        "type": "string",
        "enum": ["status", "run"]
      }
    },
    "required": ["operation"]
  }
}
```

## Secrets and Credentials

Vault keys from `vault_keys` are injected as environment variables:

- `API_KEY` becomes `AURAGO_SECRET_API_KEY`
- `base-url` becomes `AURAGO_SECRET_BASE_URL`
- Non-alphanumeric characters are converted to `_` and keys are uppercased.

Credential IDs are a separate mechanism. The `execute_skill` tool call can pass `credential_ids`, which inject fields as `AURAGO_CRED_<NAME>_<FIELD>` such as `AURAGO_CRED_ROUTER_PASSWORD` or `AURAGO_CRED_API_KEY_TOKEN`.

For manifest-level credential requests, use the existing compatibility pattern inside `vault_keys`:

```json
{
  "vault_keys": ["cred:<credential-id>"]
}
```

Do not store secret values in manifests or code. Store the secret in the Vault, then assign it to the skill in the Skill Manager.

## Daemon Manifest Settings

| Field | Default | Description |
|---|---|---|
| `enabled` | false | Whether the daemon should start automatically. |
| `wake_agent` | false | Whether daemon wake events should prompt the main agent. |
| `wake_rate_limit_seconds` | 60 | Minimum seconds between accepted wake-ups for this daemon. |
| `max_runtime_hours` | 0 | Hard runtime limit. `0` means unlimited. |
| `restart_on_crash` | true | Restart the daemon after unexpected exits. |
| `max_restart_attempts` | 3 | Max restart attempts within the cooldown window. |
| `restart_cooldown_seconds` | 300 | Restart counting window. |
| `health_check_interval_seconds` | 60 | Process liveness check interval. |
| `env` | `{}` | Extra environment variables for the daemon process. |
| `trigger_mission_id` | empty | Mission to trigger when the daemon emits a wake event. |
| `trigger_mission_name` | empty | Display name for the selected mission. |
| `cheatsheet_id` | empty | Cheatsheet injected as working instructions for triggered missions. |
| `cheatsheet_name` | empty | Display name for the selected cheatsheet. |

Daemon template parameters are not passed through `execute_skill`. Daemon lifecycle is controlled with `manage_daemon`, while runtime settings live in the manifest `daemon` object or in the daemon-specific `env` map.

## Tool Bridge

To call native AuraGo tools from Python skills:

1. Add required tool names to `internal_tools`.
2. Ensure config enables `tools.python_tool_bridge.enabled`.
3. Ensure config whitelists the tools in `tools.python_tool_bridge.allowed_tools`.
4. Ask the user to approve the skill's internal tool access in the Web UI.

Use the SDK defensively:

```python
from aurago_tools import AuraGoTools, AuraGoToolError

if not AuraGoTools.is_available():
    return {"status": "error", "message": "Tool bridge is not available or not approved"}

try:
    tools = AuraGoTools()
    result = tools.call("proxmox", {"operation": "overview"})
except AuraGoToolError as exc:
    return {"status": "error", "message": str(exc)}
```

## Verification

After creating or editing a normal skill, run `execute_skill` with small safe test arguments before telling the user it is ready. For daemon skills, run `manage_daemon` with `refresh`, then `start`, then `status` to verify the daemon is running and healthy.
