# Invasion Control (`invasion_control`)

Manage deployment nests (target servers/VMs/containers) and eggs (sub-agent configurations). List, inspect, assign, deploy (hatch), stop, monitor eggs, send tasks and secrets to running eggs, and inspect artifacts or messages produced by eggs.

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
| `list_artifacts` | List files/images/audio/video/etc. uploaded by eggs to the host |
| `get_artifact` | Get metadata and host download path for one artifact |
| `read_artifact` | Read a small text artifact directly |
| `list_egg_messages` | List rate-limited messages that eggs sent to the host |
| `ack_egg_message` | Mark an egg message as acknowledged |
| `upload_artifact` | Egg-mode only: upload a local file from the Egg to the host |
| `send_host_message` | Egg-mode only: send a rate-limited message from the Egg to the host and optionally request wakeup |
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
| `mission_id` | string | no | Filter artifacts/messages by mission ID |
| `artifact_id` | string | for get_artifact/read_artifact | Artifact ID returned by an Egg, `task_status`, or `list_artifacts` |
| `id` | string | for ack_egg_message; optional alias for artifact_id | Generic record ID |
| `status` | string | no | Artifact status filter such as `completed` or `pending` |
| `limit` | integer | no | Maximum number of artifacts or messages to return |
| `file_path` / `path` | string | for upload_artifact | Local file path inside the Egg workspace to upload to the host |
| `filename` | string | no | Host-facing filename for upload_artifact |
| `mime_type` | string | no | MIME type for upload_artifact |
| `title` | string | no | Short title for send_host_message |
| `body` / `message` | string | for send_host_message | Message body to send to the host |
| `artifact_ids` | array | no | Artifact IDs attached to send_host_message |
| `severity` | string | no | Message severity such as `info`, `warning`, or `error` |
| `dedup_key` | string | no | Optional key to prevent duplicate host messages |
| `wakeup_requested` | boolean | no | Request host-agent wakeup; host-side rate limiting still applies |
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

Task results may include `artifact_ids`. These are host-local artifacts created by the Egg. Use `get_artifact` to find the host path/download URL. Use `read_artifact` only for small text files; for images, music, videos, archives, PDFs, and other binary files use `get_artifact`.

**Check a task result later:**
```json
{"action": "invasion_control", "operation": "task_status", "task_id": "task-123"}
```

**List artifacts produced by a task or mission:**
```json
{"action": "invasion_control", "operation": "list_artifacts", "task_id": "task-123", "limit": 20}
```

**Get one artifact's host path and download URL:**
```json
{"action": "invasion_control", "operation": "get_artifact", "artifact_id": "artifact-123"}
```

**Read a small text artifact:**
```json
{"action": "invasion_control", "operation": "read_artifact", "artifact_id": "artifact-123"}
```

**Inspect messages sent by eggs:**
```json
{"action": "invasion_control", "operation": "list_egg_messages", "nest_id": "nest-123", "limit": 20}
```

**From inside an Egg, upload a local file to the host:**
```json
{"action": "invasion_control", "operation": "upload_artifact", "file_path": "report.txt", "task_id": "task-123", "mime_type": "text/plain"}
```

**From inside an Egg, notify the host and request wakeup:**
```json
{"action": "invasion_control", "operation": "send_host_message", "title": "Report ready", "body": "The scrape report is uploaded.", "artifact_ids": ["artifact-123"], "wakeup_requested": true, "dedup_key": "scrape-report-task-123"}
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
- **Artifacts**: Files created by eggs and uploaded to the host, stored locally under the host data directory
- **Egg messages**: Rate-limited notifications from eggs; they can wake the host agent, but the agent must inspect them with `list_egg_messages`
- **Secrets**: Credentials or sensitive data passed to running eggs

## Notes

- Nests and eggs are the core concepts in the invasion control system
- Eggs must be assigned to a nest before they can be hatched
- Running eggs can receive tasks and secrets dynamically
- Use `egg_status` to monitor the state of deployed eggs. If the user names an Egg, pass `egg_name`; do not ask the user for a nest ID first.
- Use `task_status`/`get_result` with the `task_id` from `send_task` when a remote Egg result is still pending.
- If the user asks for files, screenshots, images, music, videos, logs, reports, or other outputs produced by an Egg, use `list_artifacts` and `get_artifact`.
- If an Egg says it created output but the chat does not show the data, inspect `artifact_ids`, then call `get_artifact` or `list_artifacts`.
- `upload_artifact` and `send_host_message` are for code running inside an Egg. On the host, use the list/get/read/message operations instead.
- `upload_artifact` only reads files inside the Egg workspace, then stores them on the host under the host data directory.
