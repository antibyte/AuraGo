# Chapter 13: Dashboard

The AuraGo Dashboard provides real-time insights into your agent's performance, system health, and operational metrics. Monitor everything at a glance and make data-driven decisions.

## Dashboard Overview

Access the dashboard at `http://localhost:8088/dashboard` or through the radial menu (☰ → 📊 Dashboard).

```
┌─────────────────────────────────────────────────────────────────────┐
│ ⚡ AURAGO              🌙         ≡              👤 user            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ 📊 Dashboard                                     [Refresh]  │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────┐   │
│  │ 💻 System    │ │ 🧠 Memory    │ │ 💰 Budget    │ │ 📈 Mood  │   │
│  │ 12% CPU      │ │ 3.2 GB Free  │ │ $0.45 Today  │ 😊 Happy   │   │
│  └──────────────┘ └──────────────┘ └──────────────┘ └──────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  [System Metrics Chart]          [Mood History]             │   │
│  │                                                             │   │
│  │  CPU  ████████████████████░░░░░░░░░░░░  45%                 │   │
│  │  RAM  ██████████████░░░░░░░░░░░░░░░░░░  32%                 │   │
│  │  Disk ████████░░░░░░░░░░░░░░░░░░░░░░░░  18%                 │   │
│  │                                                             │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## System Metrics (CPU, RAM, Disk)

Real-time monitoring of your AuraGo host's resource usage.

### CPU Usage

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| **Current Usage** | Real-time CPU percentage | > 80% |
| **Load Average** | 1, 5, and 15-minute averages | > cores × 2 |
| **Core Count** | Number of CPU cores | - |
| **Frequency** | Current CPU clock speed | - |

**Chart Types:**
- **Real-time**: Last 60 seconds (updates every second)
- **Hourly**: Last 24 hours (5-minute averages)
- **Daily**: Last 30 days (hourly averages)

```
CPU Usage (Last Hour)
100%│                                    ╭─╮
 80%│                              ╭────╯ ╰──╮
 60%│        ╭──╮            ╭────╯           ╰──
 40%│   ╭────╯  ╰────╮  ╭────╯
 20%│───╯            ╰──╯
  0%└──────────────────────────────────────────────
     10:00  10:15  10:30  10:45  11:00  11:15  11:30
```

### RAM Usage

| Metric | Description | Formula |
|--------|-------------|---------|
| **Total** | Physical RAM installed | - |
| **Used** | Currently allocated | Total - Free |
| **Free** | Unallocated RAM | - |
| **Cached** | Disk cache | Included in Used |
| **Available** | RAM for new applications | Free + reclaimable |
| **Swap** | Swap space usage | Should be < 50% |

```
Memory Breakdown (8 GB Total)
┌─────────────────────────────────────────────────────────┐
│ ████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │
│ Used: 3.2 GB │ Free: 2.8 GB │ Cached: 2.0 GB           │
└─────────────────────────────────────────────────────────┘
```

### Disk Usage

Monitors AuraGo's working directories and system disks:

| Path | Purpose | Default Warning |
|------|---------|-----------------|
| `/` (root) | System partition | 90% |
| `agent_workspace/` | Working files | 85% |
| `data/` | Database and storage | 85% |
| `log/` | Log files | 95% |

```
Disk Usage Overview
┌─────────────────────────────────────────────────────────┐
│ / (System)                                              │
│ ████████████████████████████████████░░░░░░░░░░░░░░░░░░░ │
│ 72% used (144 GB / 200 GB)                              │
├─────────────────────────────────────────────────────────┤
│ agent_workspace/                                        │
│ ██████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │
│ 28% used (14 GB / 50 GB)                                │
├─────────────────────────────────────────────────────────┤
│ data/                                                   │
│ ██████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │
│ 12% used (6 GB / 50 GB)                                 │
└─────────────────────────────────────────────────────────┘
```

> 💡 **Tip:** Enable automatic cleanup in config to prevent disk space issues:
> ```yaml
> dashboard:
>   auto_cleanup:
>     enabled: true
>     max_log_age: 7d
>     max_workspace_age: 30d
> ```

## Mood History Visualization

AuraGo's personality engine tracks mood states over time, providing insights into interaction patterns.

### Mood States

| Mood | Description | Color |
|------|-------------|-------|
| 😊 **Happy** | Positive, enthusiastic | Green |
| 😐 **Neutral** | Balanced, factual | Blue |
| 😔 **Sad** | Subdued, careful | Purple |
| 😠 **Angry** | Direct, forceful | Red |
| 🤔 **Curious** | Inquisitive, exploratory | Yellow |
| 😴 **Tired** | Slow, minimal responses | Gray |

### Mood Timeline

```
Mood History (Last 7 Days)
     😊 Happy    ████████████████████████████████  45%
     😐 Neutral  ██████████████████               25%
     🤔 Curious  ████████████                     15%
     😔 Sad      ██████                            8%
     😠 Angry    ███                               5%
     😴 Tired    ██                                2%
```

### Mood Influencing Factors

The dashboard shows what affects AuraGo's mood:

```
Mood Factors (Last 24 Hours)
┌─────────────────────────────────────────────────────────┐
│ Positive Influences         │ Negative Influences      │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━│━━━━━━━━━━━━━━━━━━━━━━━━━ │
│ • Successful tool calls     │ • Errors/exceptions      │
│ • User appreciation         │ • Rate limiting          │
│ • Task completion           │ • Timeout events         │
│ • Learning new patterns     │ • Repetitive tasks       │
└─────────────────────────────────────────────────────────┘
```

> 🔍 **Deep Dive:** Mood affects not just responses but also tool selection and creativity. A "curious" AuraGo might try novel approaches, while "neutral" tends toward proven solutions.

## Prompt Builder Analytics

Track how AuraGo builds and optimizes prompts for LLM interactions.

### Token Usage Statistics

| Metric | Description | Optimization |
|--------|-------------|--------------|
| **Input Tokens** | Tokens sent to LLM | Lower is cheaper |
| **Output Tokens** | Tokens received | Varies by task |
| **Context Window** | % of max used | Alert at > 80% |
| **Compression Ratio** | Before/after optimization | Higher is better |

```
Token Usage (Today)
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  Requests: 142                                          │
│  Total Input: 45,230 tokens                             │
│  Total Output: 12,890 tokens                            │
│  Avg per request: 318 in / 91 out                       │
│                                                         │
│  Cost Estimate: $0.45                                   │
│  Model: openai/gpt-4o-mini                              │
│                                                         │
│  [████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░]  │
│  Daily budget used: 45%                                 │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Prompt Optimization Metrics

```
Prompt Builder Efficiency
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  Optimizations Applied:                                 │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │
│  • Context pruning      │  23% token reduction        │
│  • Tool description trim│  15% token reduction        │
│  • History summarization│  31% token reduction        │
│  • Duplicate removal    │   8% token reduction        │
│                                                         │
│  Total Savings: 12,450 tokens (~$0.12 today)           │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Response Time Analysis

```
Response Times (Last 100 Requests)
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  Min:    0.8s  │  ███                                   │
│  Max:   12.4s  │  ████████████████████████████████████  │
│  Mean:   2.3s  │  ████████                              │
│  P95:    5.1s  │  ██████████████████                    │
│  P99:    8.7s  │  ██████████████████████████████        │
│                                                         │
│  [Response Time Distribution Chart]                     │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Budget Tracking Display

Monitor API costs and stay within your budget.

### Daily Budget Overview

```
Budget Status (Today)
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  Daily Limit: $5.00                                     │
│  Spent: $1.23                                           │
│  Remaining: $3.77 (75%)                                 │
│                                                         │
│  [████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░]  │
│                                                         │
│  Projected (24h): $2.15                                 │
│  Status: ✅ On track                                    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Cost Breakdown by Model

| Model | Requests | Tokens | Cost | % of Total |
|-------|----------|--------|------|------------|
| gpt-4o-mini | 89 | 42,100 | $0.85 | 69% |
| gpt-4o | 12 | 8,200 | $0.31 | 25% |
| claude-3-haiku | 41 | 15,400 | $0.07 | 6% |
| **Total** | **142** | **65,700** | **$1.23** | **100%** |

### Historical Cost Trends

```
Cost History (Last 30 Days)
$6.00│                              ╭──╮
$5.00│                    ╭────╮   │  │
$4.00│           ╭──╮    │    │   │  │
$3.00│    ╭────╮│  │╭───╯    ╰───╯  ╰──
$2.00│╭───╯    ╰╯  ╰╯
$1.00│╯
 $0.0└────────────────────────────────────────────
      Day 1  5   10   15   20   25   30

Average: $2.34/day  │  Projected monthly: $70.20
```

> ⚠️ **Warning:** Set up budget alerts to avoid surprises:
> ```yaml
> budget:
>   daily_limit: 5.00
>   alerts:
>     - at: 80%  # $4.00
>       action: notify.webui
>     - at: 100% # $5.00
>       action: notify.telegram
>       message: "Daily budget exceeded!"
> ```

## Memory Statistics

Insights into AuraGo's memory system usage.

### Short-Term Memory (STM)

```
Short-Term Memory
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  Conversations: 142                                     │
│  Total Messages: 2,847                                  │
│  Avg Conversation Length: 20 messages                   │
│                                                         │
│  Database Size: 12.4 MB                                 │
│  Last Cleanup: 2 hours ago                              │
│  Retention: 30 days                                     │
│                                                         │
│  [Storage Usage Over Time]                              │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Long-Term Memory (RAG)

| Metric | Value | Description |
|--------|-------|-------------|
| **Vectors Stored** | 45,230 | Semantic memory entries |
| **Index Size** | 89 MB | Vector database size |
| **Dimensions** | 1536 | Embedding dimensions |
| **Avg Query Time** | 45ms | Semantic search latency |
| **Last Index Update** | 5 min ago | Freshness |

```
Long-Term Memory Categories
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  Facts & Knowledge      ████████████████████  45%      │
│  User Preferences       ██████████            25%      │
│  Tool Results           ████████              20%      │
│  External Data          ████                  10%      │
│                                                         │
│  Total Vectors: 45,230                                  │
│  Coverage: 127 days of memory                           │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Knowledge Graph

```
Knowledge Graph Statistics
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  Entities: 1,247                                        │
│  Relationships: 3,892                                   │
│  Connections per Entity: 3.1 avg                        │
│                                                         │
│  Top Entity Types:                                      │
│  • Person: 234    • Location: 189                       │
│  • Topic: 412     • Event: 156                          │
│  • Tool: 89       • Other: 167                          │
│                                                         │
│  Graph Density: Medium                                  │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Understanding the Charts

### Time Range Selection

All charts support multiple time ranges:

| Range | Data Points | Use Case |
|-------|-------------|----------|
| **Live** | Real-time | Monitoring current activity |
| **1H** | 60 points (1/min) | Recent performance |
| **24H** | 288 points (5/min) | Daily patterns |
| **7D** | 168 points (1/hr) | Weekly trends |
| **30D** | 720 points (1/hr) | Monthly analysis |

### Chart Types

**Line Charts:**
- Best for: Trends over time
- Examples: CPU usage, mood history, token usage

**Bar Charts:**
- Best for: Comparisons, distributions
- Examples: Disk usage by directory, cost by model

**Pie/Donut Charts:**
- Best for: Proportions, percentages
- Examples: Memory categories, mood distribution

**Heatmaps:**
- Best for: Activity patterns
- Examples: Hourly activity by day, error frequency

### Reading the Charts

```
Interpreting System Metrics
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Normal Pattern:
  ╭──╮      ╭──╮      ╭──╮
──╯  ╰──────╯  ╰──────╯  ╰────  Regular activity spikes

Concerning Pattern:
  ╭────────╮
──╯        ╰───────────────────  Sustained high usage

Error Pattern:
  ╭╮    ╭╮    ╭╮    ╭╮
──╯╰────╯╰────╯╰────╯╰────────  Frequent spikes (check logs)
```

## Dashboard Customization

### Widget Configuration

Arrange and customize dashboard widgets:

```yaml
dashboard:
  layout: grid  # or 'list', 'compact'
  
  widgets:
    - type: system_metrics
      position: top-left
      size: large
      refresh: 5s
      
    - type: budget_tracker
      position: top-right
      show_projected: true
      
    - type: mood_chart
      position: middle
      timeframe: 7d
      
    - type: memory_stats
      position: bottom-left
      show_stm: true
      show_ltm: true
      
    - type: quick_actions
      position: bottom-right
```

### Custom Views

Create specialized dashboards for different needs:

**Developer View:**
- Token usage
- Response times
- Error rates
- Tool call frequency

**Administrator View:**
- System resources
- Disk usage
- Active connections
- Security events

**Budget View:**
- Cost tracking
- Model usage comparison
- Daily/weekly/monthly trends
- Budget projections

### Display Options

| Setting | Options | Default |
|---------|---------|---------|
| **Theme** | Dark / Light / Auto | Auto |
| **Refresh Rate** | 1s / 5s / 30s / Manual | 5s |
| **Time Format** | 12h / 24h | 24h |
| **Date Format** | Local / ISO / US | Local |
| **Units** | Metric / Imperial / Binary | Binary |

## Exporting Data

Export dashboard data for analysis or reporting.

### Export Formats

| Format | Use Case | Data Included |
|--------|----------|---------------|
| **CSV** | Spreadsheet analysis | Raw metrics |
| **JSON** | API integration | Full data |
| **PDF** | Reports | Visual charts |
| **PNG** | Presentations | Screenshot |

### Export Methods

**Web UI:**
```
Dashboard → ⋮ (Menu) → Export → Select Format
```

**Via Chat:**
```
You: Export dashboard data to CSV
Agent: 📊 Generating CSV export...
     
     ✓ Export ready
     File: dashboard_export_2024-01-15.csv
     Size: 45 KB
     
     Download: [Download Link]
```

**Automated Export:**
```yaml
exports:
  weekly_report:
    schedule: "0 9 * * 1"  # Mondays at 9 AM
    format: pdf
    recipients:
      - admin@company.com
    include:
      - budget_summary
      - usage_stats
      - performance_metrics
```

### Data Retention

Configure how long different metrics are kept:

```yaml
retention:
  realtime_metrics: 7d    # High-frequency data
  hourly_metrics: 90d     # Aggregated data
  daily_metrics: 1y       # Long-term trends
  events: 30d             # Event logs
  audit_logs: 2y          # Security/compliance
```

## Real-time Updates

The dashboard updates automatically to reflect current state.

### WebSocket Connection

```
┌──────────┐     WebSocket      ┌──────────┐
│ Browser  │◀══════════════════▶│ AuraGo   │
│ Dashboard│   Real-time Push   │ Server   │
└──────────┘                    └──────────┘
```

**Connection Status:**
- 🟢 Connected – Live updates active
- 🟡 Reconnecting – Temporary disruption
- 🔴 Disconnected – Manual refresh needed

### Manual Refresh

Force a data refresh when needed:

```
Dashboard → 🔄 Refresh Button
          or
Chat: /dashboard refresh
```

### Push Notifications

Enable browser notifications for important events:

```yaml
notifications:
  browser:
    enabled: true
    events:
      - budget_threshold_exceeded
      - system_resource_critical
      - mission_failed
      - agent_offline
```

### Mobile Dashboard

Access a simplified mobile-optimized dashboard:

```
📱 Mobile Dashboard View
┌─────────────────────────┐
│ ⚡ AuraGo Dashboard     │
├─────────────────────────┤
│                         │
│ 💻 CPU: 12%  🟢         │
│ 🧠 RAM: 3.2GB free      │
│ 💰 $0.45 / $5.00        │
│                         │
│ 📈 Mood: 😊 Happy       │
│                         │
│ [View Details]          │
│                         │
└─────────────────────────┘
```

---

> 💡 **Tip:** Pin frequently used metrics to your browser's bookmark bar for quick access: `http://localhost:8088/dashboard?widget=system`

> 🔍 **Deep Dive:** The dashboard is built on top of AuraGo's metrics API. Power users can query metrics directly via `/api/metrics` for custom integrations.

---

## Next Steps

- **[Chapter 11: Mission Control](11-missions.md)** – Monitor automated task metrics
- **[Chapter 12: Invasion Control](12-invasion.md)** – Track remote deployment health
- **[Chapter 14: Security](14-security.md)** – Review security events and audit logs
