# Chapter 13: Dashboard

<p align="center">
  <a href="../../../assets/readme/system-wiring.svg"><img src="../../../assets/readme/system-wiring.svg" width="640" alt="AuraGo system wiring overview"></a>
</p>

Numbers, issues, affect, 3D graph. Not a second chat.

Access it at `http://localhost:8088/dashboard` or via the radial menu (☰ → 📊 Dashboard). Authentication is required when Web UI auth is enabled.

## Table of Contents

1. [Layout](#layout)
2. [Overview](#overview)
3. [Agent](#agent)
4. [Knowledge](#knowledge)
5. [Operations](#operations)
6. [Analytics](#analytics)
7. [Operational Issues](#operational-issues)
8. [Mood States](#mood-states)
9. [Real-Time Updates (SSE)](#real-time-updates-sse)
10. [Dashboard API Summary](#dashboard-api-summary)
11. [Tips](#tips)
12. [Troubleshooting](#troubleshooting)
13. [Next Steps](#next-steps)

---

## Layout

The header holds the logo, an icon-only menu button (its accessible name is "Sections menu"; shown only in windows up to 900 px wide), one density button that shows the current mode **Comfortable** or **Compact** and toggles it on click, and the theme button. Below it, the sidebar is on the left and the active section on the right.

```text
┌───────────────────────────────────────────────────────────────────────────────────┐
│ AuraGo                           [☰]       [Comfortable]        [Theme]           │
├──────────────────────────────┬────────────────────────────────────────────────────┤
│ ● All good                   │ Overview                                           │
│ ● Ready · ● Live             │ [one-line description]                             │
│                              │ Updated 09:41:12                                ⟳  │
│ Overview                 [2] │                                                    │
│ AGENT                        │ [cards of the active section, 12-column grid]      │
│   Personality                │                                                    │
│   Memory                     │                                                    │
│   User                       │                                                    │
│ KNOWLEDGE                    │                                                    │
│   Knowledge graph            │                                                    │
│   File sync                  │                                                    │
│ OPERATIONS                   │                                                    │
│   Problems               [1] │                                                    │
│   Automations                │                                                    │
│   System and services        │                                                    │
│   Audit                      │                                                    │
│   Logs                       │                                                    │
│ ANALYTICS                    │                                                    │
│   LLM and costs              │                                                    │
│   Prompts                    │                                                    │
│   Tools                      │                                                    │
└──────────────────────────────┴────────────────────────────────────────────────────┘
```

| Group | Section | Link | Cards |
|---|---|---|---|
| — | Overview | `#overview` | Status, Needs attention, System, Budget, Automations, Knowledge, Recent activity, Agent state |
| Agent | Personality | `#personality` | Personality |
| Agent | Memory | `#memory` | Memory, Latest reflection |
| Agent | User | `#user` | User Profile, Last 7 Days, Journal, Newspaper |
| Knowledge | Knowledge graph | `#knowledge` | Knowledge Graph, Graph View, KG Explorer, Graph Health, Graph Quality |
| Knowledge | File sync | `#filesync` | File Sync Status |
| Operations | Problems | `#problems` | Operational Issues, Error patterns |
| Operations | Automations | `#automations` | Cronjobs, Mission History, Daemon Skills |
| Operations | System and services | `#system` | System Status, Operations & Services, LLM Guardian, Realtime Speech, GitHub Repositories |
| Operations | Audit | `#audit` | Audit |
| Operations | Logs | `#logs` | Live Log |
| Analytics | LLM and costs | `#llm` | Budget & Tokens, LLM router, Helper LLM |
| Analytics | Prompts | `#prompts` | Prompt Analytics, Prompt Optimization, Output Compression |
| Analytics | Tools | `#tools` | Activity, Adaptive Tool Filtering, Tooling Diagnostics |

**Sidebar:**

- **Status block.**
  - The overall pill reads "All good" / "{{n}} critical" / "{{n}} warnings".
  - It reads "Checking…" while loading, and "Status unknown" when the first load of the page failed with an HTTP error; after a successful load, a failed refresh keeps the previous text.
  - The agent dot reads "Ready" / "Busy".
  - The live dot reads "Live" / "Offline" / "Connecting…".
  - A click opens the overview.
- **Badges, hidden at 0.**
  - Overview: the critical and warning rows of **Needs attention**. A trailing `+` (also on the pill, for example "3+ warnings") means critical or warning rows were cut off.
  - Problems: the active operational issues.
  - Automations: cron jobs in error, failed mission runs (24 h), and failed daemon skills.
- **Drawer.** In windows up to 900 px wide the sidebar becomes a drawer. The menu button opens it. Choosing a section, clicking the backdrop or pressing Escape closes it; after choosing a section, focus moves to the section title, and after Escape it returns to the menu button.

**Section header:**

- It holds the title, a one-line description, and the stamp `Updated <time>`, set whenever the section's loader finishes, also when the data came from the 15-second cache. The overview and Memory skip the stamp when their load failed, and the overview re-stamps on every summary update.
- The refresh button "Refresh section" reloads the section and bypasses the 15-second cache.
- The overview adds **Customize** and, while customizing, **Show all tiles**.

**Cards:**

- Every section card has an icon, a title and a ▼ collapse toggle; the overview tiles have no collapse toggle.
- Cards sit in a 12-column grid. When a section is narrower than 1100 px, quarter- and third-width cards take half the row and two-thirds cards the full row; below 700 px every card takes the full row.
- The KPI rows and detail blocks of the big cards are explained under Memory.

**Persistence:** everything below is stored per browser:

- The URL hash `#<section-id>`. Old tab hashes are mapped and replaced in the URL: `#agent` → `#personality`, `#cronjobs` → `#automations`; `#overview`, `#user`, `#knowledge`, `#filesync`, `#audit` and `#system` keep their names.
- localStorage `aurago-dash-section` holds the last section. An old `aurago-dash-tab` value is migrated once.
- `dash-collapse-<card-id>`: the collapsed state of each card.
- `aurago-dash-hidden-tiles`: the tiles hidden with **Customize**.
- `dash-disclosures`: the detail blocks that are open.
- Without localStorage, the last section, the hidden tiles and the open blocks are kept in memory for the page's lifetime; the collapse toggles need localStorage.

Tiles can only be hidden or shown (see [Customize](#customize)); there is no export menu and no YAML-based dashboard configuration.

---

## Overview

- `#overview` answers "Is everything OK?".
- All tiles come from one `GET /api/dashboard/summary` answer, which is administrator-only. The **Agent state** tile also reads `GET /api/personality/state`. If the summary request fails, every tile shows an error state with a retry button.

| Tile | What it shows | Can be hidden |
|---|---|---|
| **Status** | pill; Agent ("Ready" / "Busy" / "Debug Mode"), Model, Persona, Mood, Context (fill bar), Last activity, and Nightly maintenance while maintenance is on | no |
| **Needs attention** | up to 8 rows (severity icon, text, time), each linking to its section; "+{{n}} more" links to Problems; "Everything is in order" when empty; "Problems could not be checked" is added as an extra warning row when the issues source failed (the pill and the badges ignore that row) | no |
| **System** | CPU, RAM and Disk bars and Uptime, updated in place by `system_metrics` (about every 10 s); "Waiting for the first sample (every 10 s)"; links to System and services | yes |
| **Budget** | spend today, "{{spent}} of {{limit}} today", a bar, and a badge "Within limit" / "Warning" / "Blocked"; "Budget tracking disabled." while off; links to LLM and costs | yes |
| **Automations** | Running, Queued, Cron errors, Failed missions (24 h); links to Automations | yes |
| **Knowledge** | Graph nodes, Core Facts; links to Knowledge graph | yes |
| **Recent activity** | the latest 8 audit events (time, type, target, source, status); missions link to Automations, the rest to Audit; "No recent activity" | yes |
| **Agent state** | mood, Valence and Arousal bars ("At rest" without affect), Trend Steady / Recovering / Strained; "Personality engine disabled."; links to Personality; loaded only while shown | yes |

A tile whose source failed shows "Not available" when:

- **Automations**: the mission history;
- **Knowledge**: the knowledge graph or the memory store;
- **Recent activity**: the audit log;
- **Agent state**: its own request.

### Needs attention

| Code | Row text | Severity | Section |
|---|---|---|---|
| `operational_issue` | "{{title}} ({{occurrences}}×)" | From the issue: critical/error/high → Critical, warning/warn/medium → Warning, otherwise Note | Problems |
| `cron_errors` | "Cron jobs with errors: {{count}} (first: {{first_id}})" | Warning | Automations |
| `missions_failed` | "Failed mission runs in the last 24 h: {{count}}" | Warning | Automations |
| `daemon_failed` | "Failed daemon skills: {{count}} (first: {{first_name}})" | Warning | Automations |
| `budget_warning` | "Daily budget at {{percent}}%" | Warning | LLM and costs |
| `budget_blocked` | "Daily budget used up ({{percent}}%), LLM calls are blocked" | Critical | LLM and costs |
| `disk_high` | "Disk {{percent}}% full" | Warning at ≥ 90 %, Critical at ≥ 97 % | System and services |
| `memory_high` | "RAM usage at {{percent}}%" | Warning at ≥ 95 % | System and services |
| `indexer_error` | "File indexer errors: {{count}}" | Warning | File sync |
| `memory_conflicts` | "Open memory conflicts: {{count}}" | Note | Memory |
| `localwiki_update` | "Local Wikipedia update available ({{date}})" | Note | System and services |

Rows are ordered by severity first, then newest first (rows without a time come last), then by code. The rules consider at most 20 active operational issues, so "+{{n}} more" counts only those.

The pill is critical as soon as one row is critical and warning as soon as one is a warning; Note rows keep "All good". Only the operational-issue title and inserted values such as job ids, daemon names and dates are not translated.

### Customize

- **Customize** shows a switch on every hideable tile ("Show {{tile}} on the overview").
- Hidden tiles are stored per browser in `aurago-dash-hidden-tiles`, and the remaining tiles widen.
- **Show all tiles** resets the list.
- Status and Needs attention cannot be hidden.
- A hidden Agent state tile costs no request.

---

## Agent

This group holds three sections: Personality, Memory and User.

### Personality

`#personality`. The old `#agent` link lands here. It holds the **Personality** card.

- **Trait radar chart** for the seven personality traits (curiosity, thoroughness, creativity, empathy, confidence, and related values from the personality engine).
- **Current mood badge** and trigger text.
- **Emotional state** (when the emotion synthesizer is active): description, cause, response style, and an emotion timeline.
- **Mood timeline chart** with selectable ranges: 1h, 6h, 24h, 7d, 30d.
- **World affect and lived notes:** the card shows valence and arousal meters, the most recent sanitized affect events (up to 8), and lived notes (user-reversible; reflection proposes at most two notes per day).
- **Current dynamics:** load, familiarity, and friction with a trend line (Steady, Recovering, Strained). The **Reset current dynamics** button calls `POST /api/personality/dynamics/reset`, sets load and friction to zero, keeps familiarity, and returns affect to rest values without changing traits, lived notes, or the active persona.

| API | Purpose |
|-----|---------|
| `GET /api/personality/state` | Current traits, mood, enabled flags, dynamics |
| `GET /api/dashboard/mood-history?hours=N` | Mood history for the chart |
| `GET /api/dashboard/emotion-history?hours=N` | Emotion history entries |
| `POST /api/personality/dynamics/reset` | Zero the dynamics and return affect to rest |

When the personality engine is disabled, the card shows an empty state.

### Memory

`#memory`. The section holds the **Memory** and **Latest reflection** cards.

**KPI rows and detail blocks.** This is explained here once; the other sections refer to it:

- Memory, Graph Health, Graph Quality, Prompt Analytics and Tooling Diagnostics show a row of 3–4 numbers first; "—" appears when a value is not provided (for example Consistency issues while semantic search is off).
- Their details sit in blocks that are closed by default and open with a click.
- Open blocks are remembered per browser in `dash-disclosures`.
- Some block titles show a count.

| Card | What it shows | API |
|---|---|---|
| **Memory** | KPI row: **Core Facts** (a button that opens the core facts modal), Messages, Embeddings ("Embeddings ✗" when the vector store is off), Graph Nodes. Blocks: All counters, Strategy and usage, Memory Health, Insights, Notes, Deletion jobs, Recent Episodes, Milestones. | `GET /api/dashboard/memory` |
| **Latest reflection** | The latest weekly reflection, and **Run now**, which is administrator-only. | `POST /api/dashboard/memory/reflection/run` |

The **Core Facts** KPI opens a modal to inspect and delete single core facts; deleting all of them needs the confirmation `DELETE_ALL_CORE_MEMORY`.

| API | Purpose |
|-----|---------|
| `GET /api/dashboard/memory` | Memory statistics and health |
| `GET /api/dashboard/core-memory` | Core facts list |
| `DELETE /api/dashboard/core-memory/mutate` | Delete core facts |
| `GET /api/dashboard/memory/curation` | Curation status |
| `POST /api/dashboard/memory/curation/dry-run` | Preview safe cleanup actions |
| `POST /api/dashboard/memory/curation/apply` (administrator-only) | Apply curation |
| `POST /api/dashboard/memory/hygiene/dry-run` | Preview memory hygiene |
| `POST /api/dashboard/memory/hygiene/apply` (administrator-only) | Apply memory hygiene |
| `GET /api/privacy-deletions` (administrator-only, Deletion jobs) | Deletion jobs |
| `PATCH /api/dashboard/notes` (administrator-only, note protection) | Note protection |
| `POST /api/memory/conflicts/resolve` (administrator-only) | Resolve memory conflicts |

Error patterns are now in **Problems**.

### User

`#user`. The section holds the cards below.

| Card | What it shows | API |
|---|---|---|
| **User Profile** | Searchable profile entries grouped by category | `GET /api/dashboard/profile` |
| **Last 7 Days** | Activity overview with highlights and pending items | `GET /api/memory/activity-overview?days=7` |
| **Journal** | Timeline entries, sentiment trend, and topics | `GET /api/dashboard/journal`, `GET /api/dashboard/journal/summaries` |
| **Newspaper** | The headline of the latest edition ("Latest edition") and, when the paper is daily, its "Next target time" | `GET /api/desktop/newspaper/capabilities`, `GET /api/desktop/newspaper/editions` |

Profile entries can be updated via `PUT /api/dashboard/profile/entry` and removed via `DELETE /api/dashboard/profile/entry`.

---

## Knowledge

### Knowledge graph

`#knowledge`. The section holds these cards:

| Card | Function | Endpoints |
|---|---|---|
| **Knowledge Graph** | Node and edge counts, type breakdown, search bar | `GET /api/knowledge-graph/stats`, `GET /api/knowledge-graph/search` |
| **Graph View** | Interactive overview rendered as a 3D constellation (WebGL) with an enhanced 2D canvas fallback; 2D↔3D toggle and automatic fallback when WebGL is unavailable | Built from loaded nodes and edges |
| **KG Explorer** | Search results and the Nodes and Edges lists | `GET /api/knowledge-graph/nodes`, `GET /api/knowledge-graph/edges`, `GET /api/knowledge-graph/important` |
| **Graph Health** | KPI row: Consistency issues ("—" with the hint "Checked only while semantic search is on"), Pending Nodes, Open Conflicts. Its status pills stay visible. Blocks: All metrics, Index Consistency. | `GET /api/knowledge-graph/health` |
| **Graph Quality** | KPI row: Protected Nodes, Isolated Nodes, Untyped Nodes, Duplicate Suggestions. Blocks: All metrics, Isolated Nodes, Untyped Nodes, Generic Nodes, Same-label Review Candidates, ID Duplicate Variants. | `GET /api/knowledge-graph/quality`, `POST /api/knowledge-graph/merge` (Merge buttons) |
| **Node Inspector** (modal) | Properties, neighbors, protect/edit actions | `GET`/`PUT`/`DELETE /api/knowledge-graph/node`, `PUT`/`DELETE /api/knowledge-graph/edge`, `POST /api/knowledge-graph/node/protect` |

Click a node in the KG Explorer lists to open the Node Inspector modal. Node and edge changes, conflict resolution and merges are administrator-only.

### File sync

`#filesync`. The **File Sync Status** card has four columns: File Indexer, Knowledge Graph (file_sync), Collections and Last Synchronization.

The card buttons are **Refresh** and **Rescan**; Rescan calls `POST /api/indexing/rescan`.

Status data is loaded from:

```bash
GET /api/debug/file-sync-status
GET /api/debug/kg-file-sync-stats
GET /api/debug/file-sync-last-run
```

Indexer errors raise an `indexer_error` row in Needs attention.

---

## Operations

### Problems

`#problems`. The section holds these cards:

| Card | What it shows | API |
|---|---|---|
| **Operational Issues** | See [Operational Issues](#operational-issues). | `GET /api/operational-issues`, administrator-only |
| **Error patterns** | Repeated failures | `GET /api/dashboard/errors` |

The sidebar badge of Problems counts the active issues.

### Automations

`#automations`. The old `#cronjobs` link lands here.

| Card | What it shows | API |
|---|---|---|
| **Cronjobs** | Scheduled jobs, described in [Cron jobs](#cron-jobs) below | See below |
| **Mission History** | Recent mission runs with status, trigger, start time and duration; **Load more** uses `limit` and `offset`. | `GET /api/dashboard/mission-history` |
| **Daemon Skills** | All daemons with Total, Running, Stopped and Error counts and an alert for auto-disabled daemons; visible when daemons exist | `GET /api/daemons` |

#### Cron jobs

The **Cronjobs** card lists internal scheduled jobs from the built-in CronManager — including scheduler-tool jobs and mission-backed schedules stored in the same scheduler.

The table shows ID, Source, Schedule, Next run, Status (Enabled, Disabled, or Error), Prompt, and Action. Error rows expose the scheduler `last_error` in the status tooltip; a stored job that is not registered with the scheduler also shows Error.

Administrators can edit cron expression, prompt, and enabled state from the row action. The job source is shown in the edit dialog but preserved automatically.

Schedules of an EasyDrag flow show "Managed by EasyDrag" instead of edit and delete.

```bash
GET /api/dashboard/cronjobs?q=backup&source=agent&status=enabled
GET /api/dashboard/cronjobs?status=error
PUT /api/dashboard/cronjobs
DELETE /api/dashboard/cronjobs/{id}
```

All cronjob endpoints, including `GET`, are administrator-only.

### System and services

`#system`. The section holds these cards:

| Card | What it shows | API |
|---|---|---|
| **System Status** | CPU, RAM and Disk gauges, Net ↑ / Net ↓, SSE Clients, Uptime; live through `system_metrics`. | `GET /api/dashboard/system` |
| **Operations & Services** | Missions, invasion nests, file indexer, MQTT, notes, vault, devices, context summary, cheatsheets, tunnel, integration badges (Local Wikipedia shows **New edition** when an update is available and links to its config section) | `GET /api/dashboard/overview` |
| **LLM Guardian** | Security guardian status and metrics (visible only when enabled) | `GET /api/dashboard/guardian` |
| **Realtime Speech** | Read-only status without audio, transcripts, or credentials. Enabled, Profiles, Active profile, Session. | `GET /api/realtime-speech/status` |
| **GitHub Repositories** | Linked repos (when GitHub integration is configured) | `GET /api/dashboard/github-repos` |

### Audit

`#audit`. The **Audit** section is the central activity trail for actions AuraGo performs. It records agent tool calls and actions, mission runs, heartbeat scheduler wake-ups, EasyDrag flow events, and remote device events (connects, disconnects, heartbeats, command results).

Each audit entry contains time, source, event type, target, status, summary, duration, and scrubbed detail data. Sensitive values are cleaned before storage; long details are shortened.

#### Filtering and Search

Use the search field to find entries by summary, target, event type, or cleaned details. Source, status, type, and date filters can be combined, and the list is paginated.

#### Deleting Audit Entries

Administrators can delete a single audit row with its **Delete** button or delete all rows matching the current filters; with no filter set, that is every row. Bulk deletion requires server-side confirmation `DELETE_AUDIT_EVENTS`.

#### API Access

```bash
GET /api/dashboard/audit?limit=25&offset=0&q=mission&source=mission&status=success
DELETE /api/dashboard/audit/{id}
DELETE /api/dashboard/audit
```

DELETE endpoints require administrator access.

### Logs

`#logs`. The **Live Log** card shows the last 100 lines (`GET /api/dashboard/logs?lines=100`).

It has the filter field "Filter logs (regex supported)..." and the buttons **↓ Latest** and **↻ Refresh**.

---

## Analytics

### LLM and costs

`#llm`. The section holds these cards:

| Card | What it shows | API |
|---|---|---|
| **Budget & Tokens** | Daily spend vs. limit, per-model usage, OpenRouter credits, average tokens per request. "Budget tracking disabled." while off; the card stays visible. | `GET /api/budget`, `GET /api/credits` |
| **LLM router** | In-memory counters since the restart: helper and decision calls, tokens, failures, fallbacks, and decisions per area and source. | `GET /api/llm-router/status`, administrator-only |
| **Helper LLM** | Helper model status, metrics, and per-operation counters (requests, cache hits, LLM calls, fallbacks) | `GET /api/dashboard/helper-llm` |

The `budget_warning` and `budget_blocked` events show toasts.

### Prompts

`#prompts`. The section holds these cards:

| Card | What it shows | API |
|---|---|---|
| **Prompt Analytics** | KPI row: Total Builds, Avg Tokens, Checks with reductions, Avg Saving %. Blocks: Module selection, Base prompt, Request budget, Requests and reuse, Avg Prompt Distribution. | `GET /api/dashboard/prompt-stats` |
| **Prompt Optimization** | Active Overrides, Running Shadows, Rejected Mutations, Delivered guides, Verified samples, Legacy samples, Global Success Rate | `GET /api/dashboard/optimization` |
| **Output Compression** | Compressed and Skipped counters, characters saved, and top tools and top filters when data exists | `GET /api/dashboard/compression` |

Prompt statistics reset on server restart and appear after the first conversation.

### Tools

`#tools`. The section holds these cards:

| Card | What it shows | API |
|---|---|---|
| **Activity** | Cron jobs, processes, webhooks, co-agents with state and runtime, background tasks | `GET /api/dashboard/activity` |
| **Adaptive Tool Filtering** | Filter KPIs, only while adaptive tool filtering is on | `GET /api/dashboard/tool-stats` |
| **Tooling Diagnostics** | Only once telemetry exists. KPI row: Output format errors, Recoveries, Policy Signals, Tools with failures. Blocks: All counters, Parse Sources, Recovery Events, Policy Signals, Retrieval Metrics, Failure-heavy tools, Tool Families, Model Segments, Model Comparison. | `GET /api/dashboard/tool-stats` |

One tool-stats request feeds both tool cards.

---

## Operational Issues

The card lives in **Problems** (`#problems`); the sidebar badge of Problems counts the active issues, and active issues also appear in Needs attention as `operational_issue` rows.

Operational Issues are durable records for problems found during background work, maintenance, health checks, or repeated tool execution. AuraGo deduplicates them by fingerprint and keeps revisions, occurrences, status, and last-seen timestamps instead of creating an unbounded stream of identical warnings.

### Notices and Delivery

- Background and maintenance code records the issue; it never sends a user notice directly.
- Only at the first direct chat contact of each local calendar day, the supervisor selects at most two changed issues, ordered by severity, revision change, and recency. At most one such notice is sent per day across sessions, chat channels and restarts. If no issues are due at first contact, issues added later wait until first contact the following day. Remaining open issues stay visible in the Dashboard. The model receives already-prepared diagnostic context and does not decide whether a notice is shown.
- Supported notification brokers can receive the `operational_issue_notice` event. Other channels get the same localized text as a deterministic final-answer prefix. A revision is marked notified only after broker delivery or durable final-message persistence.
- A one-off `tool_failure` warning remains internal until its second occurrence. High-severity open issues can repeat after 24 hours only after another occurrence or while waiting for a user decision.
- Findings of the nightly memory reflection are a model's suggestions, not supervisor findings. They appear under "Model-generated reflection (advisory, not from the supervisor)", only through notification brokers (never as a prefix on an answer), after any supervisor findings, and the model treats them as untrusted hints.

The Dashboard card supports filters for status, kind, severity, and source. Administrators can resolve or archive an issue and can preview or archive stale issues. The API returns sanitized records with non-reversible public IDs; it never exposes raw credentials, complete tool payloads, or internal database identifiers.

### Retention and Mission Triggers

Active history is archived losslessly: single warnings after seven days, recurring warnings and errors after 30 days. Explicit `review_required` decisions are never auto-archived. Archived issues are excluded from notices, active counts, reminders, and mission triggers; a recurrence reopens the same fingerprint and retains its history.

```yaml
maintenance:
  retention:
    operational_issues_days: 30
```

Mission Control can react to a planner issue without matching raw log text:

```yaml
trigger:
  type: planner_operational_issue
  planner_issue_source: maintenance
  planner_issue_severity: high
  planner_title_contains: backup
```

The source, severity, and title filters are optional. Use this trigger for an explicit follow-up workflow, not as a replacement for the supervisor's direct notice policy.

### Operational Issues API

```http
GET /api/operational-issues?status=active&severity=high&limit=25&offset=0
GET /api/operational-issues/stale-preview
POST /api/operational-issues/archive-stale
POST /api/operational-issues/{id}/resolve
POST /api/operational-issues/{id}/archive
```

The list, preview, archive, and resolve endpoints require administrator access. Archiving does not delete the record or its history.

---

## Mood States

AuraGo's personality engine uses **named mood states**, not a simple happy/sad emoji scale. Valid moods:

| Mood | Typical character |
|------|-------------------|
| **curious** | Default; exploratory, open to new approaches |
| **focused** | Task-oriented, precise |
| **creative** | Imaginative, novel suggestions |
| **analytical** | Structured, detail-driven |
| **cautious** | Careful, risk-aware |
| **playful** | Light, informal tone |
| **frustrated** | Direct; often after repeated errors |
| **concerned** | Attentive when issues or risks appear |
| **relaxed** | Calm, low-pressure interactions |

Mood changes are logged with trigger text and shown in the mood timeline. With the emotion synthesizer enabled, a richer emotional description and timeline supplement the heuristic mood.

> 🔍 **Deep Dive:** See [Chapter 10: Personality](10-personality.md) for engine configuration and trait behavior.

---

## Real-Time Updates (SSE)

The dashboard uses **Server-Sent Events (SSE)** through the shared `AuraSSE` connection — not a separate dashboard WebSocket.

On connect, the dashboard registers handlers for:

| SSE event | Dashboard effect |
|-----------|------------------|
| `system_metrics` | Updates the gauges and stats of System Status and the overview's System tile in place (about every 10 s); no summary refresh. |
| `memory_update` | Updates the memory chart and counters; schedules a summary refresh. |
| `personality_update` | Updates the mood badge, and on Personality the emotion timeline; schedules a summary refresh. |
| `daemon_update` | Reloads Daemon Skills while Automations is open; schedules a summary refresh. |
| `audit_update` | Schedules an audit table refresh and a summary refresh. |
| `budget_update` / `budget_warning` / `budget_blocked` | Updates the spend display; warning and blocked show toasts; schedules a summary refresh. |
| `_open` / `_error` (connection) | Turns the sidebar live dot to Live / Offline. |

- A summary refresh waits 2 seconds after the first trigger, so a burst costs one request.
- Besides that, the summary is fetched on load, by the overview's refresh, and 60 seconds after the last fetch while the page is visible. A hidden page does not poll and catches up when it becomes visible again.
- Most sections reuse their data for 15 seconds when you switch back; Automations, Audit and File sync always fetch again. **Refresh section** always fetches fresh data.

When the connection drops, the live dot turns "Offline" at once, after 3 seconds a "Reconnecting..." banner appears, and after 8 seconds the page checks the login state and returns to the login page when the session has expired.

### Manual Refresh

- Every section header has "Refresh section".
- Cards with their own buttons: Cronjobs **Refresh**, Audit **Refresh**, Operational Issues **Refresh** / **Archive stale**, File Sync Status **Refresh** / **Rescan**, Live Log **↻ Refresh**.
- There is no global refresh button and no `/dashboard refresh` chat command; reloading the page reloads the active section.

### What Is Not in the UI

The dashboard does **not** currently offer:

- CSV, PDF, or PNG export of metrics
- A generic `/api/metrics` endpoint
- Browser push notification configuration
- YAML-driven widget layout or retention settings

For programmatic access, use the dashboard REST endpoints listed below and [Chapter 21: REST API Reference](21-api-reference.md).

---

## Dashboard API Summary

- Every route the dashboard uses. Examples and response shapes are in [Chapter 21: REST API Reference](21-api-reference.md#dashboard-api).
- *Administrator-only* routes are registered behind `requireAdmin`: with Web UI authentication off every request passes; with it on, any signed-in session passes (there are no separate roles). A bearer token needs the `admin` scope, and the auth middleware demands that scope on every dashboard route, not only the marked ones.

| Endpoint | Used by | Administrator-only |
|---|---|---|
| `GET /api/dashboard/summary` | Overview, sidebar status block and badges | yes |
| `GET /api/dashboard/overview` | Operations & Services | — |
| `GET /api/dashboard/system` | System Status | — |
| `GET /api/budget`, `GET /api/credits` | Budget & Tokens | — |
| `GET /api/llm-router/status` | LLM router | yes |
| `GET /api/dashboard/helper-llm` | Helper LLM | — |
| `GET /api/dashboard/prompt-stats` | Prompt Analytics | — |
| `GET /api/dashboard/optimization` | Prompt Optimization | — |
| `GET /api/dashboard/compression` | Output Compression | — |
| `GET /api/dashboard/activity` | Activity | — |
| `GET /api/dashboard/tool-stats` | Adaptive Tool Filtering, Tooling Diagnostics | — |
| `GET /api/personality/state`, `GET /api/dashboard/mood-history`, `GET /api/dashboard/emotion-history`, `POST /api/personality/dynamics/reset`, `POST /api/personality/character-notes` | Personality, Agent state tile | — |
| `GET /api/dashboard/memory` | Memory | — |
| `GET /api/dashboard/core-memory`, `DELETE /api/dashboard/core-memory/mutate` | core facts modal | — |
| `GET`/`POST /api/dashboard/memory/curation*` | Memory Health (curation) | `apply` and per-document actions |
| `GET`/`POST /api/dashboard/memory/hygiene*` | Memory Health (hygiene) | `apply` |
| `POST /api/dashboard/memory/reflection/run` | Latest reflection → Run now | yes |
| `GET`/`PATCH /api/dashboard/notes` | Notes | `PATCH` |
| `POST /api/memory/conflicts/resolve`, `/api/privacy-deletions*`, `GET`/`POST /api/learnings` | Memory Health (conflicts), Deletion jobs, Insights | yes |
| `GET /api/dashboard/profile`, `PUT`/`DELETE /api/dashboard/profile/entry` | User Profile | — |
| `GET /api/memory/activity-overview` | Last 7 Days | — |
| `GET /api/dashboard/journal*` | Journal | — |
| `GET /api/desktop/newspaper/capabilities`, `GET /api/desktop/newspaper/editions` | Newspaper | — |
| `/api/knowledge-graph/*` | Knowledge graph cards | node and edge changes, conflict resolution, merges |
| `POST /api/indexing/rescan`, `GET /api/debug/file-sync-status`, `GET /api/debug/kg-file-sync-stats`, `GET /api/debug/file-sync-last-run` | File Sync Status | — |
| `/api/operational-issues*` | Operational Issues | yes |
| `GET /api/dashboard/errors` | Error patterns | — |
| `GET`/`PUT /api/dashboard/cronjobs`, `DELETE /api/dashboard/cronjobs/{id}` | Cronjobs | yes, all methods |
| `GET /api/dashboard/mission-history` | Mission History | — |
| `GET /api/daemons` | Daemon Skills | — |
| `GET /api/dashboard/guardian` | LLM Guardian | — |
| `GET /api/realtime-speech/status` | Realtime Speech | — |
| `GET /api/dashboard/github-repos` | GitHub Repositories | — |
| `GET /api/dashboard/audit`, `DELETE /api/dashboard/audit`, `DELETE /api/dashboard/audit/{id}` | Audit | `DELETE` |
| `GET /api/dashboard/logs` | Live Log | — |

Cards, blocks and modals in this column are named as in the interface.

---

## Tips

- **Deep-link a section.** `http://localhost:8088/dashboard#automations` opens Automations; the ids are in the table under Layout. Old links such as `#agent` still work and are replaced by `#personality`.
- **Collapse noisy cards** with ▼; the choice is remembered per card.
- **Trim the overview** with **Customize**.
- **Theme and density.** The theme button in the header cycles through the Web UI themes and applies to all pages; the density button toggles between **Comfortable** and **Compact**, shared with the other workspace pages that have it.
- **Budget alerts:** When daily limits are approached, SSE pushes in-dashboard toasts — configure limits in [Chapter 7: Configuration](07-configuration.md), not in dashboard YAML.

---

## Troubleshooting

| Problem | Likely cause | What to try |
|---------|--------------|-------------|
| A card shows "—" or stays empty | the feature is disabled, or there is no data yet | check the config; prompt statistics appear after the first conversation |
| A tile shows "Not available" | its source failed on the server and is listed in the summary's `unavailable`; for Agent state, its own personality request failed | look for `[Dashboard] Summary source unavailable` in the server log; use **Refresh section** |
| The sidebar pill shows "Status unknown" | the first `GET /api/dashboard/summary` of the page failed with an HTTP error (signed out, server error) | reload, sign in again, check the server log |
| Gauges and the System tile stop updating | SSE is disconnected (live dot "Offline") | wait for the "Reconnecting..." banner to clear, or reload |
| The budget card shows "Budget tracking disabled." | budget tracking is off | enable budget tracking in the config |
| The LLM Guardian, GitHub Repositories, Daemon Skills, Adaptive Tool Filtering or Tooling Diagnostics card is missing | Guardian, GitHub (with an owner) and adaptive tool filtering must be enabled in the config; Daemon Skills and Tooling Diagnostics appear once data exists | expected |
| A dashboard route answers `403` to an API token | the token lacks the `admin` scope, which every dashboard route needs | use the Web UI session or a token with the `admin` scope |
| File sync shows no data | Indexer not running or never synced | Trigger rescan; check indexing config |
| An old bookmark opens a different section | old hashes are mapped (`#agent` → `#personality`, `#cronjobs` → `#automations`) | update the bookmark |

---

## Next Steps

- **[Chapter 10: Personality](10-personality.md)** — Configure mood engine and traits
- **[Chapter 9: Memory](09-memory.md)** — Understand the memory layers shown in the **Memory** section
- **[Chapter 11: Mission Control](11-missions.md)** — Mission history and cron-backed schedules
- **[Chapter 12: Invasion Control](12-invasion.md)** — Remote nodes reflected in operations stats
- **[Chapter 14: Security](14-security.md)** — Audit log and LLM Guardian
- **[Chapter 21: REST API Reference](21-api-reference.md)** — Full endpoint details

*End of Part 3 — Autopilot. On to Part 4: Care and Depth.*

---

**Previous Chapter:** [Chapter 12: Invasion Control](./12-invasion.md)
**Next Chapter:** [Chapter 14: Security](./14-security.md)
