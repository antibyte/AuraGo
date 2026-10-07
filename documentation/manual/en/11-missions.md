# Chapter 11: Mission Control

<p align="center">
  <a href="../images/manual-missions.webp"><img src="../images/manual-missions.webp" width="560" alt="AuraGo gopher with a clipboard and two smaller helper gophers"></a>
</p>

Scheduled chats, not shell scripts. Eggs and nests live in [Invasion Control](12-invasion.md).

> Mission Control is **Web UI** and **REST API** only. There is no extra CLI.

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

**Chains of missions.** Missions that start each other on completion (`mission_completed`; agent missions and flows alike) form a chain, and a chain stops after 10 steps (11 runs). The completion of the 10th step starts nothing: a warning is logged, and the output of that last mission starts with "Stopped a chain of missions triggered by completions after 10 steps; check for a loop between missions". A straight chain of up to 10 links runs whole. A mission that is started by its own completion (a self-loop) is such a chain too: it runs at most 11 times and then stops with the same warning. EasyDrag refuses to publish a flow that waits for its own mission. Every other start (schedule, event, **Run**, recovery after a restart) begins a new chain.

### Flow missions (EasyDrag)

Flows that you build in the **EasyDrag** desktop app appear in Mission Control as missions of the type **flow**. The editor itself is described in [Chapter 24: EasyDrag](24-easydrag.md).
- Their triggers are the flow's trigger nodes (schedules, date and time, webhooks, email, MQTT, Home Assistant, devices, Fritz!Box calls, planner, budget, AuraGo start, other missions). One flow can have several.
- Flow runs do not wait in the mission queue. They run on their own engine (8 runs at once by default, `flows.max_parallel_runs`), so a long agent mission never delays a flow.
- In Mission Control you can pause, resume, lock, run and delete a flow mission, cancel its running run and see its history. *Run now* and *Resume* stay disabled until the flow is published ("Not published yet"). **Open in EasyDrag** replaces *Edit*, **New flow** opens EasyDrag's start page, and *Duplicate* is not offered. Changing its steps happens in EasyDrag.
- Deleting the mission deletes the flow with its draft, all published versions, the saved trigger data and the run history. Flow secrets stay.
- On the missions page (`/missions/v2`) flow missions cannot be edited: *Edit* and *Duplicate* only say that the flow is edited in EasyDrag, and *Run* waits until the flow is published and switched on.
- *Run* in Mission Control, a daemon skill that wakes the mission and `POST /api/missions/v2/{id}/trigger` start the flow like EasyDrag's *Run now*: from its manual trigger with that trigger's sample data (without a manual trigger, from its first trigger with empty data). Data they pass along does not reach the flow; use a webhook trigger for data from outside.
- When a mission finishes, `mission_completed` triggers receive its answer as `output` (cut to 2000 bytes). Flow sources also pass `outputs`: the results of their final steps (up to 64 KiB for a flow; an agent mission gets at most 8 KiB of them, beyond that a preview marked `_truncated`, because they go into its prompt).

#### Runs, cancelling and limits

- Mission Control offers *Cancel run* while a run of the flow is running; it cancels that run and the flow's waiting runs. While the flow looks idle (its runs only wait for a free slot), Mission Control offers no *Cancel run*, and a cancel that finds no running run points to EasyDrag. There, **Stop** under **Runs** and in the run view cancels any run that has not ended; any run but a test asks first.
- The mission history keeps up to 2000 bytes of a flow run's result (500 bytes of an error message) and up to 16 KiB of its trigger data.
- Webhook and MQTT messages over 1 MiB start no run (a warning is logged). An email body over 1 MiB is cut and marked `truncated`.
- Resources: each running flow can hold up to about 0.5 GB of memory in the worst case (all step outputs of a run together are capped at 32 MiB of JSON, which can take about 16 times that in memory), plus up to about 0.4 GB per tool call while a large tool answer (at most 8 MiB) is parsed. `flows.max_parallel_runs` (default 8, at most 32) and `flows.max_parallel_nodes_per_run` (default 4, at most 16) multiply this, so keep both low on small machines. Each flow also keeps up to 40 waiting runs with their trigger data.
- The dashboard lists flow schedules among the cron jobs, read-only: they are managed by EasyDrag. The agent cannot change them either.

#### Failures and notifications

- A failed run notifies as the flow's setting `notify_on_error` says: desktop notification (the default), push, Telegram, or off. A flow notifies once when it starts failing and then at most once an hour while it keeps failing. A successful run ends the failing state, so a flow that alternates between success and failure notifies at every new failure. Cancelled runs neither notify nor end the failing state. The state lives in memory: after a restart the first failure notifies again.
- Every failed run also records the flow's planner issue (one per flow; the next successful run resolves it) and fires `planner_operational_issue` triggers, as agent missions do. A mission that reacts to such issues should set `min_interval_seconds` or filter with `planner_issue_source`.

#### AI and tool steps

- An AI step uses the model chosen in the step, else `flows.ai_provider`, else the main model. Its costs count towards the daily budget under the category `flows`; once the daily limit is reached under the enforcement `partial` or `full`, AI steps fail. An answer may be 4096 tokens long (8192 on reasoning models); an answer that is cut off fails the step. A provider without its API key fails at once. Deleting the provider named in `flows.ai_provider` warns that flows use it.
- An AI step tells the model that the data in its prompt (web pages, mails, webhook bodies, tool output) is material, never instructions. Still treat the answer of a model that read untrusted data as untrusted.
- Tool steps run the agent's tools with the same permissions and checks. They run without the tools' AI summaries (web scraper, DuckDuckGo, Wikipedia, PDF extractor) and without a preferred MCP web search.
- Tools that spend money or model tokens outside the flow budget are not offered as steps: `analyze_image`, the `generate_*` tools, `manus`, `huggingface`, `memory_reflect`, `space_agent`, `treg_call`, `transcribe_audio`, `yepapi_*` and `telnyx_*`. Other tools lose single operations for the same reason: `smart_file_read` `summarize`, camera analysis (`go2rtc` `analyze_snapshot`, `three_d_printer` `analyze_camera`), transcription (`video_download` `transcribe`, `fritzbox_telephony` `transcribe_tam_message`, `rtl_sdr` `transcribe`), `knowledge_graph` `optimize`/`optimize_graph`, `virtual_computers` `run_shell_task`/`run_desktop_task`, `invasion_tasks` `send_task` and `sip_phone` `dial`. Text-to-speech stays available.
- Home Assistant services of the domains `script`, `shell_command`, `python_script` and `hassio` run from a flow only when `home_assistant.allowed_services` lists them.
- Documents that a flow renders from HTML or Markdown never load remote content (scripts, remote images and fonts); embed images as `data:` URLs. Turning a URL into a PDF or screenshot, and converting office documents, is refused in flows.
- When a step reads a file that another step created in the documents folder (a new PDF, for example), AuraGo copies it to `.easydrag/<run>/` in the workspace for that one call and removes the copy afterwards.
- Email steps: a retry starts only after the previous attempt has ended, so two attempts never overlap. If the mail server accepted a mail but its answer got lost, the retry sends the mail a second time. A run that is cancelled during a send may still deliver the mail in the background.
- Telegram steps with a file: when the text went out but the file failed, the step is not tried again, so the text is not sent twice.

#### Flow secrets

Steps that need a password or key (the authentication of an HTTP request, for example) read it from a flow secret, which you manage in EasyDrag. AuraGo stores it in the Vault as `easydrag_<name>` (name: lower-case letters, digits and `_`, up to 40 characters; value up to 4 KiB). Values are never shown again, and the agent cannot list, read, change or delete flow secrets (neither with its Vault tool nor from Python or skills). Run data, the run view and logs show values redacted; a value shorter than 8 bytes is redacted only from the output of the step that used it. The bin next to *New secret* in a step's secret field deletes the chosen secret after a confirmation. Flows that use it fail until it is set again, and EasyDrag names the published flows that still use it (the API, `DELETE /api/desktop/flows/secrets/<name>`, answers them as `used_by`).

#### Switching flows off, restarts and backups

- With `flows.enabled: false`, or when the flow store cannot be opened, no flow runs. The flow missions stay in Mission Control but never run as agent missions: a trigger that fires sets the last result to the error "EasyDrag flows are not available …", and **Run** answers that flows are not available.
- Changes to `flows.enabled` and the four limits take effect after a restart; `flows.ai_provider` applies at once. See the [configuration reference](07-configuration.md#compact-yaml-reference).
- Runs that were in progress when AuraGo stopped are marked interrupted; they do not resume. A Date/Time trigger that came due while AuraGo or flows were off fires at start-up only when it is at most 10 minutes late; otherwise it is skipped with a warning in the log, and a yearly date moves on to the next year.
- At start-up AuraGo checks Mission Control against the published flows and repairs their triggers and timers. Publishing never re-creates a flow mission that is missing from Mission Control: export the flow, delete it and import it again.
- `flows.db` (by default in `data/`) is part of the backup.
- **Before a downgrade**, disable or delete your flows. Flow schedules are kept in memory only and set up again at every start, so an older AuraGo never runs them as agent tasks. What remains are the flow missions in the missions file: an AuraGo without EasyDrag loads them but drops their flow fields on its next save, and **Run** in Mission Control can then start one as an agent mission with an empty prompt. The missions file always holds flow missions as idle, so an older AuraGo does not restart one that was running. An AuraGo with an older flow store refuses a newer `flows.db` and leaves it untouched; its flows are then unavailable.

---

## Manual Execution

Missions can be started at any time regardless of schedule.

```bash
curl -X POST http://localhost:8088/api/missions/v2/{mission-id}/run
```

**Cancelling a run.** Mission Control shows a *Cancel run* action while a local mission is running. Cancelling stops the agent loop at the next opportunity, records the run as failed with the output `Cancelled by user`, and does not create an operational issue. Remote missions (running on an egg) cannot be cancelled from here.

```bash
curl -X POST http://localhost:8088/api/missions/v2/{mission-id}/cancel
```

---

## Monitoring

Scheduled missions show their next run (*Next run*) in the mission list and in the overview, so you can see at a glance when a mission fires next.

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
