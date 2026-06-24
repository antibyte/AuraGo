# Chapter 13: Dashboard

The AuraGo Dashboard is the operational control panel for your agent. It shows system health, memory, personality, missions, knowledge graph quality, audit trails, and more — organized into tabs that load on demand.

Access it at `http://localhost:8088/dashboard` or via the radial menu (☰ → 📊 Dashboard). Authentication is required when Web UI auth is enabled.

## Layout

The dashboard has two persistent elements above the tab content:

1. **Agent Status Banner** — always visible: active model, personality profile, context fill level, connected integrations, and last activity.
2. **Tab navigation** — eight tabs; only the active tab's cards are loaded initially.

```
┌─────────────────────────────────────────────────────────────────────┐
│ 📊 AURAGO DASHBOARD                                    🌙         ≡ │
├─────────────────────────────────────────────────────────────────────┤
│ 🤖 Agent Status: model · personality · context · integrations      │
├─────────────────────────────────────────────────────────────────────┤
│ Overview │ Agent │ User │ Knowledge Graph │ File-Sync │ Audit │ …  │
├─────────────────────────────────────────────────────────────────────┤
│  [Cards for the active tab — collapsible, grid layout]             │
└─────────────────────────────────────────────────────────────────────┘
```

**Persistence:**
- The last selected tab is stored in the URL hash (`#overview`, `#agent`, …) and in `localStorage`.
- Individual cards can be collapsed; that state is saved per card in `localStorage`.

There is **no** dashboard-wide widget layout editor, export menu, or YAML-based dashboard configuration in the current UI.

---

## Tab: Overview

The **Overview** tab is the at-a-glance health view.

| Card | What it shows | API |
|------|---------------|-----|
| **System Health** | CPU, RAM, and disk gauges; network sent/received; connected SSE clients; uptime | `GET /api/dashboard/system` |
| **Quick Status** | Missions, integrations, tunnel, planner, skills, and related snapshot badges | `GET /api/dashboard/overview` |
| **Budget & Tokens** | Daily spend vs. limit, per-model usage, OpenRouter credits, average tokens per request | `GET /api/budget`, `GET /api/credits` |
| **Optimization** | Active prompt overrides, running shadow tests, rejected mutations, trace events, success rate | `GET /api/dashboard/optimization` |
| **Output Compression** | Compression count, characters saved, top tools and filters (when enabled) | `GET /api/dashboard/compression` |
| **Mission History** | Recent mission runs with status, trigger, start time, and duration | `GET /api/dashboard/mission-history` |

Budget tracking appears only when budget tracking is enabled in config. When disabled, the card shows an empty-state message.

Mission History supports pagination via **Load more** (`limit` + `offset` query parameters).

---

## Tab: Agent

The **Agent** tab focuses on personality and memory.

### Personality

- **Trait radar chart** for the seven personality traits (curiosity, thoroughness, creativity, empathy, confidence, and related values from the personality engine).
- **Current mood badge** and trigger text.
- **Emotional state** (when the emotion synthesizer is active): description, cause, response style, and an emotion timeline.
- **Mood timeline chart** with selectable ranges: 1 h, 6 h, 24 h, 7 d, 30 d.

| API | Purpose |
|-----|---------|
| `GET /api/personality/state` | Current traits, mood, enabled flags |
| `GET /api/dashboard/mood-history?hours=N` | Mood history for the chart |
| `GET /api/dashboard/emotion-history?hours=N` | Emotion history entries |

When the personality engine is disabled, the card shows an empty state.

### Memory

- Counts for core memory, chat messages, vector DB entries, and knowledge graph nodes/edges.
- **Memory health** summary: retrieval stats, confidence, stale candidates, conflicts, strategy mode.
- **Memory Health / Curation** panel with dry-run preview and admin-only apply actions.
- **Recent episodes** and milestone list.
- **Error patterns** from repeated failures.

Clicking **Core Memory** opens a modal to inspect and delete individual core facts (admin delete-all requires confirmation `DELETE_ALL_CORE_MEMORY`).

| API | Purpose |
|-----|---------|
| `GET /api/dashboard/memory` | Memory stats and health |
| `GET /api/dashboard/core-memory` | Core facts list |
| `DELETE /api/dashboard/core-memory/mutate` | Delete core facts |
| `GET /api/dashboard/memory/curation` | Curation status |
| `POST /api/dashboard/memory/curation/dry-run` | Preview safe cleanup actions |
| `POST /api/dashboard/memory/curation/apply` | Apply curation (admin) |
| `GET /api/dashboard/errors` | Error pattern overview |

---

## Tab: User

The **User** tab covers profiling and journaling.

| Card | What it shows | API |
|------|---------------|-----|
| **User Profile** | Searchable profile entries grouped by category | `GET /api/dashboard/profile` |
| **Last 7 Days** | Activity overview with highlights and pending items | `GET /api/memory/activity-overview?days=7` |
| **Journal** | Timeline entries, sentiment trend, and key topics | `GET /api/dashboard/journal`, `GET /api/dashboard/journal/summaries` |

Profile entries can be updated via `PUT /api/dashboard/profile/entry`.

---

## Tab: Knowledge Graph

The **Knowledge Graph** tab is a full KG operations view.

| Section | Function |
|---------|----------|
| **Summary** | Node/edge counts, type breakdown, search bar | `GET /api/knowledge-graph/stats`, `GET /api/knowledge-graph/search` |
| **Graph Quality** | Protected, isolated, untyped, and duplicate-candidate nodes | `GET /api/knowledge-graph/quality` |
| **KG Explorer** | Search results, recent nodes, recent edges | `GET /api/knowledge-graph/nodes`, `GET /api/knowledge-graph/edges`, `GET /api/knowledge-graph/important` |
| **Graph View** | Interactive overview/focus graph (force-graph) | Built from loaded nodes and edges |
| **Node Inspector** | Properties, neighbors, protect/edit actions | `GET /api/knowledge-graph/node`, `POST /api/knowledge-graph/node/protect` |

Click a node in the list or graph to open the Node Inspector modal.

---

## Tab: File-Sync

The **File-Sync** tab shows indexer and knowledge-graph synchronization status.

Four columns display:

- **File Indexer** — running state and indexer stats
- **Knowledge Graph** — KG sync stats
- **Collections** — indexed collection overview
- **Last Synchronization** — timestamp of the last sync run

Use **Refresh** to reload status or **Rescan** to trigger a manual indexer rescan (`POST /api/indexing/rescan`).

Status data is loaded from:

```bash
GET /api/debug/file-sync-status
GET /api/debug/kg-file-sync-stats
GET /api/debug/file-sync-last-run
```

---

## Tab: Audit

The **Audit** tab is the central activity trail for actions AuraGo performs. It records agent tool calls, mission runs, heartbeat scheduler wake-ups, and remote device events (connects, disconnects, heartbeats, command results).

Each audit entry contains time, source, event type, target, status, summary, duration, and scrubbed detail data. Sensitive values are cleaned before storage; long details are shortened.

### Filtering and Search

Use the search field to find entries by summary, target, actor, event type, or cleaned details. Source, status, type, and date filters can be combined; pagination keeps large logs responsive.

### Deleting Audit Entries

Administrators can delete a single audit row from the row action menu or delete all rows matching the current filters. Bulk deletion requires server-side confirmation `DELETE_AUDIT_EVENTS`.

### API Access

```bash
GET /api/dashboard/audit?limit=25&offset=0&q=mission&source=mission&status=success
DELETE /api/dashboard/audit/{id}
DELETE /api/dashboard/audit
```

DELETE endpoints require administrator access.

---

## Tab: Cronjobs

The **Cronjobs** tab lists internal scheduled jobs from the built-in CronManager — including scheduler-tool jobs and mission-backed schedules stored in the same scheduler.

The table shows job ID, source, cron expression, next run, status (`enabled`, `disabled`, or `error`), prompt, and row actions. Error rows expose the scheduler `last_error` in the status tooltip so invalid persisted jobs and disabled scheduler runtime are visible.

Administrators can edit cron expression, prompt, and enabled state from the row action. The job source is shown in the edit dialog but preserved automatically.

```bash
GET /api/dashboard/cronjobs?q=backup&source=agent&status=enabled
GET /api/dashboard/cronjobs?status=error
PUT /api/dashboard/cronjobs
DELETE /api/dashboard/cronjobs/{id}
```

PUT and DELETE require administrator access.

---

## Tab: System

The **System** tab covers operations, diagnostics, and logs.

| Card | What it shows | API |
|------|---------------|-----|
| **Operations & Services** | Missions, invasion nests, file indexer, MQTT, notes, vault, devices, context summary, cheatsheets, tunnel | `GET /api/dashboard/overview` |
| **LLM Guardian** | Security guardian status and metrics (visible only when enabled) | `GET /api/dashboard/guardian` |
| **Daemon Skills** | Running daemon processes (visible when daemons exist) | `GET /api/daemons` |
| **Helper LLM** | Helper model status, metrics, and recent operations | `GET /api/dashboard/helper-llm` |
| **Activity** | Tool usage, automations, cron summary | `GET /api/dashboard/activity` |
| **Prompt Builder Analytics** | Build counts, token averages, tier distribution, budget-shed sections, savings breakdown | `GET /api/dashboard/prompt-stats` |
| **Adaptive Tool Filtering** | Tool filter KPIs (when telemetry exists) | `GET /api/dashboard/tool-stats` |
| **Tooling Diagnostics** | Parse sources, recovery events, policy signals, failure-prone tools | `GET /api/dashboard/tool-stats` |
| **GitHub Repositories** | Linked repos (when GitHub integration is configured) | `GET /api/dashboard/github-repos` |
| **Live Log** | Tail of server logs with regex filter | `GET /api/dashboard/logs?lines=100` |

Prompt statistics reset on server restart and appear after the first conversation.

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
| `system_metrics` | Updates CPU/RAM/disk gauges and system stats (pushed about every 10 seconds) |
| `memory_update` | Refreshes memory bar chart and stat counts |
| `personality_update` | Updates mood badge; refreshes emotion timeline on Agent tab |
| `daemon_update` | Reloads daemon card on System tab |
| `audit_update` | Schedules audit table refresh when Audit tab is active |
| `budget_update` / `budget_warning` / `budget_blocked` | Updates spend display; shows toast warnings |

If the SSE connection drops for more than a few seconds, a reconnect banner appears at the top of the page.

### Manual Refresh

There is no global **Refresh** button for the entire dashboard. Refresh per area instead:

- **Audit** tab → **Refresh** button
- **Cronjobs** tab → **Refresh** button
- **File-Sync** tab → **Refresh** / **Rescan** buttons
- **Live Log** card → **Refresh** button

Reloading the browser page also reloads the active tab's data. There is **no** `/dashboard refresh` chat command.

### What Is Not in the UI

The dashboard does **not** currently offer:

- CSV, PDF, or PNG export of metrics
- A generic `/api/metrics` endpoint
- Browser push notification configuration
- YAML-driven widget layout or retention settings

For programmatic access, use the dashboard REST endpoints listed below and [Chapter 21: REST API Reference](21-api-reference.md).

---

## Dashboard API Summary

| Endpoint | Description |
|----------|-------------|
| `GET /api/dashboard/overview` | Agent banner and quick-status snapshot |
| `GET /api/dashboard/system` | CPU, memory, disk, network, uptime |
| `GET /api/budget` | Budget spend and limits |
| `GET /api/credits` | OpenRouter credit balance |
| `GET /api/dashboard/optimization` | Prompt optimization stats |
| `GET /api/dashboard/compression` | Output compression stats |
| `GET /api/dashboard/mission-history` | Mission run history |
| `GET /api/personality/state` | Personality and mood state |
| `GET /api/dashboard/mood-history` | Mood timeline data |
| `GET /api/dashboard/emotion-history` | Emotion synthesizer history |
| `GET /api/dashboard/memory` | Memory statistics and health |
| `GET /api/dashboard/memory/curation*` | Memory curation preview and apply |
| `GET /api/dashboard/core-memory` | Core facts |
| `GET /api/dashboard/profile` | User profile entries |
| `GET /api/memory/activity-overview` | Rolling activity summary |
| `GET /api/dashboard/journal*` | Journal entries and summaries |
| `GET /api/dashboard/notes` | Open/done notes counts |
| `GET /api/knowledge-graph/*` | Knowledge graph browse and quality |
| `GET /api/dashboard/audit` | Audit log (admin delete) |
| `GET /api/dashboard/cronjobs` | Cronjob list (admin edit/delete) |
| `GET /api/dashboard/guardian` | LLM Guardian status |
| `GET /api/dashboard/helper-llm` | Helper LLM metrics |
| `GET /api/dashboard/errors` | Error patterns |
| `GET /api/dashboard/activity` | Activity and automation stats |
| `GET /api/dashboard/prompt-stats` | Prompt builder analytics |
| `GET /api/dashboard/tool-stats` | Tooling telemetry |
| `GET /api/dashboard/github-repos` | GitHub repository list |
| `GET /api/dashboard/logs` | Server log tail |

---

## Tips

- **Deep-link a tab:** `http://localhost:8088/dashboard#agent` opens directly to the Agent tab.
- **Collapse noisy cards:** Use the ▼ toggle on card headers to hide sections you rarely need.
- **Theme:** Use the 🌙 button in the header; the choice applies across Web UI pages.
- **Budget alerts:** When daily limits are approached, SSE pushes in-dashboard toasts — configure limits in [Chapter 7: Configuration](07-configuration.md), not in dashboard YAML.

---

## Troubleshooting

| Problem | Likely cause | What to try |
|---------|--------------|-------------|
| Cards show "—" or empty | Feature disabled in config, or no data yet | Check config; run a chat session for prompt stats |
| Gauges stop updating | SSE disconnected | Wait for reconnect banner to clear, or reload the page |
| Budget card hidden | Budget tracking off | Enable budget in config |
| Guardian card missing | LLM Guardian disabled | Expected — card is hidden when `enabled: false` |
| Audit/cron edit fails | Non-admin session | Log in as administrator |
| File-Sync shows no data | Indexer not running or never synced | Trigger rescan; check indexing config |

---

## Next Steps

- **[Chapter 10: Personality](10-personality.md)** — Configure mood engine and traits
- **[Chapter 9: Memory](09-memory.md)** — Understand memory layers shown on the Agent tab
- **[Chapter 11: Mission Control](11-missions.md)** — Mission history and cron-backed schedules
- **[Chapter 12: Invasion Control](12-invasion.md)** — Remote nodes reflected in operations stats
- **[Chapter 14: Security](14-security.md)** — Audit log and LLM Guardian
- **[Chapter 21: REST API Reference](21-api-reference.md)** — Full endpoint details
