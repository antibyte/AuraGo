# Chapter 11: Mission Control

> ⚠️ **Important:** Mission Control is available via **Web UI** and **REST API** only. CLI commands for mission management are not implemented.

Mission Control is AuraGo's automation center. Define recurring tasks that the agent executes on its own — from simple backups to complex monitoring routines.

> **Note:** Nests and Eggs belong to [Invasion Control](12-invasion.md), not Mission Control. Missions are **prompt-based agent tasks**, not shell/script templates.

---

## What are Missions?

**Missions** are automated agent tasks that run on a schedule, on demand, or when a trigger fires. Each mission contains:

- **Prompt** — What the agent should do
- **Schedule or trigger** — When it runs
- **Execution type** — Manual, scheduled, or event-triggered
- **Optional dependencies** — Wait for other missions to finish first

```
┌─────────────────────────────────────────────────────────────┐
│  Mission: "Daily Backup"                                    │
│  ├─ Prompt: Create a database backup and report status      │
│  ├─ Schedule: Daily at 02:00 (cron)                         │
│  ├─ Execution: scheduled                                    │
│  └─ Result: success / error                                 │
└─────────────────────────────────────────────────────────────┘
```

> 💡 Missions run in the background and do not block normal chat.

---

## Prerequisites

Mission Control requires the scheduler tool:

### Web UI Setup
1. Open **Config → Tools** (Tool Permissions).
2. Enable **Scheduler** and **Missions** (set **Read-only** to `false` if the agent should create or edit missions).
3. Save changes.

### YAML Reference
```yaml
# config.yaml
tools:
  scheduler:
    enabled: true
    readonly: false   # false = allow create/edit
  missions:
    enabled: true
    readonly: false
```

The Web UI is available at `/missions/v2` (`/missions` redirects there).

---

## Mission Control Concepts

### Missions (V2)

A **mission** is a scheduled or triggered **agent prompt**. The agent receives the prompt and uses its tools to complete the task — it does not run raw shell commands directly.

| Execution type | Description | Use case |
|----------------|-------------|----------|
| `manual` | Run on demand only | Ad-hoc tasks |
| `scheduled` | Cron-based | Backups, reports |
| `triggered` | Event-driven | Webhooks, email, MQTT, HA state changes |

### Mission Preparation (Optional)

With **Mission Preparation**, the agent can analyze required tools, risks, and steps before execution.

### Web UI Setup
1. Open **Config → Agent Tools → Mission Preparation**.
2. Enable the feature and configure provider, timeout, and confidence thresholds.
3. Save changes.

### YAML Reference
```yaml
# config.yaml
mission_preparation:
  enabled: false
  provider: ""                    # Provider ID; empty = main LLM
  timeout_seconds: 120
  max_essential_tools: 5
  cache_expiry_hours: 24
  min_confidence: 0.5
  auto_prepare_scheduled: true
```

> 💡 Mission Preparation is advisory only — it never blocks execution.

### Dependencies and Queue

- **Dependencies:** Mission B starts only after Mission A completes
- **Queue:** Missions run sequentially when resources are limited
- **Remote execution:** Missions can run on invasion nests via `runner_type: remote`

---

## Creating Missions

### Via Web UI (Recommended)

1. Open **Mission Control** from the radial menu (🚀) at `/missions/v2`
2. Click **New Mission**
3. Configure name, **prompt**, schedule or trigger
4. Save the mission

### Via REST API

Use session cookies (`credentials: 'same-origin'`) when calling from the browser. Admin API tokens work for automation.

```bash
# Create mission (v2 API)
curl -X POST http://localhost:8088/api/missions/v2 \
  -H "Content-Type: application/json" \
  -b "session=YOUR_SESSION" \
  -d '{
    "name": "daily-backup",
    "prompt": "Create a database backup and report the result.",
    "execution_type": "scheduled",
    "schedule": "0 2 * * *",
    "enabled": true
  }'

# List all missions
curl http://localhost:8088/api/missions/v2

# Run mission manually
curl -X POST http://localhost:8088/api/missions/v2/{mission-id}/run

# View queue
curl http://localhost:8088/api/missions/v2/queue

# Execution history
curl http://localhost:8088/api/missions/v2/history?limit=10

# Dependencies
curl http://localhost:8088/api/missions/v2/dependencies

# Remote targets
curl http://localhost:8088/api/missions/v2/remote-targets
```

---

## Scheduling with Cron

AuraGo accepts standard 5-field **cron expressions** and optional 6-field expressions with seconds first for scheduled missions:

```
┌───────────── second (0 - 59, optional)
│ ┌───────────── minute (0 - 59)
│ │ ┌───────────── hour (0 - 23)
│ │ │ ┌───────────── day of month (1 - 31)
│ │ │ │ ┌───────────── month (1 - 12)
│ │ │ │ │ ┌───────────── weekday (0 - 6, Sunday = 0)
│ │ │ │ │ │
* * * * * *
```

### Common Cron Patterns

| Expression | Meaning |
|------------|---------|
| `0 2 * * *` | Daily at 02:00 |
| `0 */6 * * *` | Every 6 hours |
| `0 0 * * 0` | Every Sunday at midnight |
| `0 9-17 * * 1-5` | Hourly 9–17, Mon–Fri |
| `*/15 * * * *` | Every 15 minutes |
| `0 */15 * * * *` | Every 15 minutes, with explicit seconds |
| `0 0 1 * *` | First day of each month |

> 💡 Use [crontab.guru](https://crontab.guru) to test cron expressions.

---

## Event Triggers

Set `execution_type: triggered` and choose a `trigger_type`:

| Trigger | Description |
|---------|-------------|
| `mission_completed` | Another mission finished |
| `email_received` | Incoming email |
| `webhook` | Incoming webhook |
| `mqtt_message` | MQTT message |
| `system_startup` | AuraGo startup |
| `home_assistant_state` | HA entity state change |
| `fritzbox_call` | Fritz!Box call/voicemail |
| `budget_warning` / `budget_exceeded` | Budget thresholds |
| `device_connected` / `device_disconnected` | Remote device events |
| `planner_appointment_due` / `planner_todo_overdue` | Planner reminders |
| `egg_hatched` / `nest_cleared` | Invasion events |

Configure filters in `trigger_config` (e.g. email subject, MQTT topic, HA entity).

---

## Manual Execution

Missions can be started at any time regardless of schedule.

```bash
curl -X POST http://localhost:8088/api/missions/v2/{mission-id}/run
```

---

## Monitoring

### Status Values

| Status | Meaning |
|--------|---------|
| `idle` | Not running, waiting for next trigger |
| `queued` | Waiting in execution queue |
| `running` | Currently executing |
| `waiting` | Waiting for a dependency mission |

Results are `success` or `error` in `last_result`.

### API

```bash
curl http://localhost:8088/api/missions/v2/{mission-id}
curl http://localhost:8088/api/missions/v2/history?mission_id={mission-id}
```

Dashboard history: `GET /api/dashboard/mission-history`

---

## Examples

### Daily System Check

- **Name:** `daily-system-check`
- **Prompt:** `Check disk space, CPU usage, and running Docker containers. Create a short report.`
- **Execution type:** `scheduled`
- **Schedule:** `0 8 * * *`
- **Enabled:** `true`

### Weekly Report

- **Name:** `weekly-report`
- **Prompt:** `Summarize important events from the last week using logs and memory.`
- **Schedule:** `0 9 * * 1`

### API Health Check

- **Name:** `api-health-check`
- **Prompt:** `Check if these APIs are reachable: https://api.example.com/health. Report failures.`
- **Schedule:** `*/15 * * * *`

---

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Mission stuck in `running` | Hung agent loop | Check logs, stop via API if needed |
| Cron not firing | Wrong expression | Validate with crontab.guru |
| "Scheduler tool disabled" | Tool off | **Config → Tools** → enable **Scheduler** (YAML: `tools.scheduler.enabled: true`) |
| "Missions tool disabled" | Tool off | **Config → Tools** → enable **Missions** (YAML: `tools.missions.enabled: true`) |

> 🖥️ **Debug logging:** **Config → Agent** → **Debug Mode** (YAML: `agent.debug_mode: true`).

---

## Summary

| Feature | Availability |
|---------|--------------|
| **Web UI** | ✅ Full (`/missions/v2`) |
| **REST API** | ✅ Full (`/api/missions/v2/*`) |
| **CLI commands** | ❌ Not implemented |
| **Cron scheduling** | ✅ Supported |
| **Event triggers** | ✅ Supported |
| **Manual execution** | ✅ Web UI / API |
| **Remote execution** | ✅ Via invasion nests |

> 💡 For complex automation use the Web UI. For external integrations use the REST API with session auth.

---

**Previous:** [Chapter 10: Personality](10-personality.md)  
**Next:** [Chapter 12: Invasion Control](12-invasion.md)
