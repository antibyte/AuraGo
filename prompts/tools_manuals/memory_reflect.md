## Tool: Memory Reflection (`memory_reflect`)

Reflects on past interactions and generates insights about patterns, errors, progress, and relationships. Weekly reflections now also consider the recent activity timeline and daily rollups.

### When to use

- At the end of a productive week or major project
- After repeated errors (pattern analysis)
- To generate a progress summary for the user
- To analyse relationships and active projects in the Knowledge Graph
- Before long-term planning or goal-setting

### Schema

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `scope` | string | required | `session` / `day` / `week` / `month` / `project` / `all_time` |
| `focus` | string | `"all"` | `patterns` / `errors` / `progress` / `relationships` / `all` |
| `output_format` | string | `"summary"` | `summary` / `detailed` / `action_items` / `insights_only` |

### Scope

- **session** — current session only
- **day** — today
- **week** — last 7 days
- **month** — last 30 days
- **project** — active project (from KG context)
- **all_time** — full history

### Focus Areas

#### `patterns`
Analyses recurring behavioural patterns:
- Most frequent request topics
- Time-of-day activity preferences
- Tool usage patterns
- Session length and frequency

#### `errors`
Analyses errors and resolutions:
- Most frequent error types
- Successful workarounds
- Repeated errors (not yet learned)
- Error trends (↗ ↘)

#### `progress`
Shows achievements and completions:
- Finished projects and tasks
- Newly learned skills
- Configured integrations
- Milestones reached

#### `relationships`
Analyses the Knowledge Graph:
- New entities and connections added
- Active projects
- Important relationships

### Output Formats

- **summary** — highlights overview
- **detailed** — full analysis with examples
- **action_items** — concrete next-step suggestions
- **insights_only** — only the key "aha" findings

### Examples

#### Weekly reflection

```json
{"action": "memory_reflect", "scope": "week", "focus": "all", "output_format": "summary"}
```

**Result:**
```
📊 Your week (09.03 - 15.03.2026)

🎯 Highlights:
   ✅ 4 successful Docker setups
   ✅ 3 servers registered in inventory
   ✅ 2 cron jobs configured
   ✅ 1 Python tool created

🔄 Patterns:
   • Main focus: Docker/Infrastructure (65% of time)
   • Most active: 20:00–22:00
   • Favourite tools: docker, filesystem, execute_shell

⚠️ Learnings:
   • 3× Permission Denied → forgot sudo
     💡 Suggestion: always check sudo first?

   • 2× container name already in use
     💡 Suggestion: establish a naming convention?

📈 Knowledge Graph:
   +5 entities | +8 relations

🎯 Suggestions for next week:
   1. Set up Docker volumes backup?
   2. Create Proxmox templates?
   3. Automate SSH key management?
```

#### Error analysis

```json
{"action": "memory_reflect", "scope": "month", "focus": "errors", "output_format": "detailed"}
```

**Result:**
```
⚠️ Error Analysis (last month)

Top 3 error types:

1. Permission Denied (12×) ↗ +3 vs. previous month
   ├─ Resolved: 10/12 (83%)
   ├─ Recurring: 2× (not yet learned)
   └─ 💡 Recommendation: prefer execute_sudo

2. Container port already in use (5×) → unchanged
   ├─ Resolved: 5/5 (100%)
   └─ ✅ Learned: added port check before start

3. SSH key error (3×) ↘ -2 vs. previous month
   ├─ Resolved: 3/3 (100%)
   └─ ✅ Improvement detected!
```

#### Project-specific

```json
{"action": "memory_reflect", "scope": "project", "focus": "progress", "output_format": "action_items"}
```

### Best Practices

1. **Reflect regularly**
   - Weekly (e.g. Sunday evening)
   - After major projects
   - When you notice repeated errors

2. **Choose focus based on need**
   - Frustrated by errors → `focus: "errors"`
   - Need motivation → `focus: "progress"`
   - Lost the overview → `focus: "patterns"`

3. **Act on the insights**
   - Don't just read — update Core Memory with new constraints
   - Add lessons learned as journal entries
   - Establish new workflows based on pattern findings

4. **Share with user**
   - Weekly summaries are motivating and build trust
   - Show progress proactively — don't wait to be asked
