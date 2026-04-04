## Tool: Manage Appointments (`manage_appointments`)

Create, read, update, and delete appointments. Appointments are stored in the planner database and automatically synced to the Knowledge Graph.

When an appointment has a notification time and `wake_agent` is enabled, the agent will be woken up at that time to execute the optional `agent_instruction`.

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | `list`, `get`, `add`, `update`, `delete`, `complete`, `cancel` |
| `id` | integer | for `get`, `update`, `delete`, `complete`, `cancel` | Appointment ID |
| `title` | string | for `add` | Appointment title |
| `description` | string | no | Optional description |
| `date_time` | string | for `add` | ISO 8601 datetime (e.g. `2025-03-15T14:00:00Z`) |
| `notification_at` | string | no | ISO 8601 datetime for notification |
| `wake_agent` | boolean | no | Wake agent at notification time (default: false) |
| `agent_instruction` | string | no | Instruction for the agent when woken up |
| `query` | string | no | Search query for `list` operation |
| `status` | string | no | Filter by status for `list`: `upcoming`, `completed`, `cancelled` |

### Status Values

| Status | Description |
|--------|-------------|
| `upcoming` | Future appointment (default for new) |
| `completed` | Marked as completed |
| `cancelled` | Cancelled appointment |

### Examples

#### Create an appointment with agent wake-up

```json
{"action": "manage_appointments", "operation": "add", "title": "Team Meeting", "description": "Weekly sync", "date_time": "2025-03-20T10:00:00Z", "notification_at": "2025-03-20T09:45:00Z", "wake_agent": true, "agent_instruction": "Send a reminder via Telegram"}
```

#### List upcoming appointments

```json
{"action": "manage_appointments", "operation": "list", "status": "upcoming"}
```

#### Search appointments

```json
{"action": "manage_appointments", "operation": "list", "query": "meeting"}
```

#### Complete an appointment

```json
{"action": "manage_appointments", "operation": "complete", "id": 3}
```

#### Delete an appointment

```json
{"action": "manage_appointments", "operation": "delete", "id": 5}
```
