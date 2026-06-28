## Tool: Memory Reflection (`memory_reflect`)

Reflects on past interactions and generates structured insights about patterns, recurring errors, progress, relationships, memory quality, and safe follow-ups. Weekly reflections also consider recent activity, daily rollups, Error Learning, Learned Rules, Core Memory, Knowledge Graph context, previous reflections, and the memory curator dry-run.

### When to use

- At the end of a productive week or major project
- After repeated errors (pattern analysis)
- To generate a progress summary for the user
- To analyse relationships and active projects in the Knowledge Graph
- Before long-term planning or goal-setting

### Schema

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `scope` | string | `"recent"` | `session` / `day` / `week` or `recent` / `month` or `monthly` / `project` / `all_time` or `full` |
| `focus` | string | `"all"` | `patterns` / `errors` / `progress` / `relationships` / `all` |
| `output_format` | string | `"summary"` | `summary` / `detailed` / `action_items` / `insights_only` |

### Scope

- **session** — v1 best-effort recent context
- **day** — today
- **week** — last 7 days
- **month** — last 30 days
- **project** — v1 best-effort recent/project context from available memory signals
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
- Error trends and missing Learned Rules

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

### Output

The tool always returns structured JSON. `output_format` changes emphasis, not the wire shape.

Expected fields include:

- `patterns`
- `contradictions`
- `gaps`
- `suggestions`
- `error_patterns`
- `learned_rule_review`
- `trends`
- `action_items`
- `metrics`
- `summary`
- `quality_flags`
- `curator_dry_run`
- `actionable_findings`

If the LLM response is too generic or invalid, AuraGo retries once. If the retry is still weak, the result is stored with `quality_flags` and an Operational Issue is recorded for later review.

### Examples

#### Weekly reflection

```json
{"action": "memory_reflect", "scope": "week", "focus": "all", "output_format": "summary"}
```

**Result shape:**
```json
{
  "patterns": ["Docker and infrastructure tasks dominated the week."],
  "contradictions": [],
  "gaps": ["Confirm the current NAS hostname before storing another host fact."],
  "suggestions": ["Review stale memory candidates from the curator dry-run."],
  "error_patterns": ["docker inspect failures recur without a strong learned rule."],
  "learned_rule_review": ["Create a rule to list containers before inspecting IDs."],
  "action_items": ["Verify the NAS host fact.", "Add the Docker lookup rule."],
  "summary": "The week shows useful progress, but repeated Docker lookup errors and one missing infrastructure detail should be cleaned up first."
}
```

#### Error analysis

```json
{"action": "memory_reflect", "scope": "month", "focus": "errors", "output_format": "detailed"}
```

Use this when recurring tool failures should be converted into concrete action items or Learned Rule review work.

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

3. **Act safely on the insights**
   - Do not automatically delete or mutate Core Memory based only on reflection
   - Verify contradictions before changing durable memory
   - Create Learned Rules only when the recommended rule is specific and repeatable

4. **Share with user**
   - Weekly summaries are motivating and build trust
   - Show progress proactively — don't wait to be asked
