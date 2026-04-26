# Invasion Control (`invasion_control`)

Manage deployment nests (target servers/VMs/containers) and eggs (sub-agent configurations). List, inspect, assign, deploy (hatch), stop, monitor eggs, and send tasks and secrets to running eggs.

Use this tool whenever the user asks to talk to, ask, reach, test, or send work to an Egg, a Nest, a remote agent, or a sub-agent.

Egg names are not tool names. If an Egg is named `web scraper`, call `invasion_control` with `operation:"send_task"` and `egg_name:"web scraper"`. Do not call the `web_scraper` tool just because the Egg has that name.

## Operations

| Operation | Description |
|-----------|-------------|
| `list_nests` | List all configured nests |
| `list_eggs` | List all configured eggs |
| `nest_status` | Get status of a specific nest |
| `assign_egg` | Assign an egg to a nest |
| `hatch_egg` | Deploy/start an egg in a nest |
| `stop_egg` | Stop a running egg |
| `egg_status` | Get status of a running egg |
| `send_task` | Send a task to a running egg and wait briefly for the result |
| `task_status` | Check a previously sent task by `task_id` |
| `get_result` | Alias for `task_status` |
| `send_secret` | Send a secret to a running egg |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `nest_id` | string | for most nest operations | Nest ID |
| `nest_name` | string | no | Nest name — alternative to nest_id for lookup |
| `egg_id` | string | for assign_egg; optional target for egg_status/send_task | Egg ID |
| `egg_name` | string | optional target for egg_status/send_task | Egg name — alternative to egg_id when the user names the Egg |
| `task` | string | for send_task | Task description in natural language |
| `task_id` | string | for task_status/get_result | Task ID returned by send_task |
| `timeout` | integer | no | Optional task timeout in seconds; send_task waits up to this value, capped at 60 seconds, for a result |
| `key` | string | for send_secret | Secret key name |
| `value` | string | for send_secret | Secret value |

## Examples

**List all nests:**
```json
{"action": "invasion_control", "operation": "list_nests"}
```

**List eggs in a nest:**
```json
{"action": "invasion_control", "operation": "list_eggs", "nest_id": "nest-123"}
```

**Get nest status:**
```json
{"action": "invasion_control", "operation": "nest_status", "nest_name": "production"}
```

**Deploy an egg:**
```json
{"action": "invasion_control", "operation": "hatch_egg", "nest_id": "nest-123", "egg_id": "egg-456"}
```

**Send a task to a running egg:**
```json
{"action": "invasion_control", "operation": "send_task", "nest_id": "nest-123", "egg_id": "egg-456", "task": "Check the system metrics on all servers"}
```

**Send a task when only the Egg name is known:**
```json
{"action": "invasion_control", "operation": "send_task", "egg_name": "web scraper", "task": "Tell me a short joke in German"}
```

`send_task` returns the task result directly when the Egg answers within the wait window. If `result_available` is false, call `task_status` with the returned `task_id` instead of telling the user that no result can be retrieved.

**Check a task result later:**
```json
{"action": "invasion_control", "operation": "task_status", "task_id": "task-123"}
```

**Check status when only the Egg name is known:**
```json
{"action": "invasion_control", "operation": "egg_status", "egg_name": "web scraper"}
```

**Send a secret to a running egg:**
```json
{"action": "invasion_control", "operation": "send_secret", "nest_id": "nest-123", "egg_id": "egg-456", "key": "API_KEY", "value": "secret123"}
```

**Stop an egg:**
```json
{"action": "invasion_control", "operation": "stop_egg", "nest_id": "nest-123", "egg_id": "egg-456"}
```

## Architecture

- **Nests**: Target environments (servers, VMs, containers) where eggs are deployed
- **Eggs**: Sub-agent configurations that define what code runs in a nest
- **Hatching**: The process of deploying an egg to a nest
- **Tasks**: Natural language instructions sent to running eggs
- **Secrets**: Credentials or sensitive data passed to running eggs

## Notes

- Nests and eggs are the core concepts in the invasion control system
- Eggs must be assigned to a nest before they can be hatched
- Running eggs can receive tasks and secrets dynamically
- Use `egg_status` to monitor the state of deployed eggs. If the user names an Egg, pass `egg_name`; do not ask the user for a nest ID first.
- Use `task_status`/`get_result` with the `task_id` from `send_task` when a remote Egg result is still pending.
