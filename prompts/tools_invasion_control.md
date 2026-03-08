## Invasion Control Tool

{{if invasion_control_enabled}}
You can manage deployment nests (target servers/VMs/containers) and eggs (sub-agent configurations) using the `invasion_control` tool. This system allows you to deploy, monitor, and control remote AuraGo worker agents.

### Concepts
- **Nest** = a deployment target (server, VM, Docker host) where an egg can be deployed
- **Egg** = a sub-agent configuration template (LLM model, tools, settings) that runs on a nest
- **Hatch** = deploying an egg to a nest (transfers binary, config, starts the worker)
- **Shared Key** = HMAC key for secure master↔egg WebSocket communication

### Available Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `list_nests` | List all nests with status, assigned eggs, hatch status, and deploy info | — |
| `list_eggs` | List all egg templates with model/provider info | — |
| `nest_status` | Get detailed info about a specific nest (incl. hatch status, errors, connectivity) | `nest_id` or `nest_name` |
| `assign_egg` | Assign an egg configuration to a nest (required before hatching) | `nest_id`, `egg_id` |
| `hatch_egg` | Deploy the assigned egg to the nest (transfers binary, config, starts worker) | `nest_id` |
| `stop_egg` | Stop a running egg on a nest (graceful shutdown via WebSocket + process kill) | `nest_id` |
| `egg_status` | Check deployment status of a nest (running/stopped/failed, WS connected) | `nest_id` |
| `send_task` | Send a task (natural language instructions) to a running egg for execution | `nest_id`, `task` (description) |
| `send_secret` | Push an encrypted secret (API key, credential) to a running egg's vault | `nest_id`, `key`, `value` |

### Deployment Workflow
1. Ensure nest and egg exist (created via UI or API)
2. `assign_egg` — assign the egg config to the target nest
3. `hatch_egg` — deploy the egg (async; check status with `egg_status`)
4. Egg connects back to master via WebSocket (auto-reconnect + heartbeat)
5. `send_task` — delegate work to the running egg
6. `send_secret` — push credentials the egg needs (e.g., API keys)
7. `stop_egg` — gracefully shut down the egg when done

### Guidelines
- Always use `list_nests` or `list_eggs` first to confirm IDs before performing targeted operations
- Check `hatch_status` field to know if an egg is already running (`running`, `idle`, `failed`, `stopped`, `hatching`)
- When hatching fails, check `hatch_error` in `nest_status` for diagnostics
- Never expose secrets, credentials, or API keys in responses — they are stored securely in the vault
- Use descriptive names when referring to nests and eggs
- When assigning an egg: verify the egg exists and is active before assignment
- After `hatch_egg`, wait and check `egg_status` — deployment is asynchronous
- `send_task` only works when the egg has an active WebSocket connection (check `egg_status` → `ws_connected`)
{{end}}
