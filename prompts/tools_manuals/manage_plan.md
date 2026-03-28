## Tool: Manage Plan (`manage_plan`)

Create and maintain a structured work plan for the current chat session.

Use this when the task is multi-step, risky, or likely to span several tool calls and you want visible progress tracking.

Notes are **not** replaced by plans:
- use `manage_plan` for session-scoped execution structure
- use `manage_notes` for standalone reminders and durable note-taking

Only one unfinished plan (`draft`, `active`, or `paused`) exists per session.

### When to use
- complex implementations with several dependent steps
- debugging or investigation with checkpoints
- work that should be easy to resume later in the same session

### When not to use
- tiny one-step tasks
- quick lookups or trivial edits
- long-term automation flows better handled by missions/follow-ups

### Operations

#### `create`
Create a new session plan with ordered tasks.

Required:
- `title`
- `items` with at least one task

Optional:
- `description`
- `content` for the original user request
- `priority` (`1` low, `2` medium, `3` high)

Task item fields:
- `title`
- `description`
- `kind` (`task`, `tool`, `reasoning`, `verification`, `note`)
- `tool_name`
- `tool_args`
- `depends_on` as task IDs or 1-based task indices

Example:
```json
{
  "action": "manage_plan",
  "operation": "create",
  "title": "Fix MCP bridge persistence",
  "description": "Track the config save bug from analysis to verification",
  "content": "Investigate why vscode_debug_bridge is not persisted",
  "priority": 3,
  "items": [
    {
      "title": "Inspect config save path",
      "kind": "reasoning"
    },
    {
      "title": "Patch UI persistence logic",
      "kind": "tool",
      "tool_name": "filesystem"
    },
    {
      "title": "Run regression tests",
      "kind": "verification",
      "depends_on": [2]
    }
  ]
}
```

#### `list`
List plans for the current session.

Optional:
- `status` (`all`, `draft`, `active`, `paused`, `completed`, `cancelled`)
- `limit`

#### `get`
Fetch one plan in detail.

Required:
- `id`

#### `set_status`
Change a plan status.

Required:
- `id`
- `status`

Valid statuses:
- `draft`
- `active`
- `paused`
- `completed`
- `cancelled`

Optional:
- `content` for a short note explaining the change

When a draft plan is set to `active`, the first ready task is promoted to `in_progress`.

#### `update_task`
Update one task inside a plan.

Required:
- `id`
- `task_id`
- `status`

Valid task statuses:
- `pending`
- `in_progress`
- `completed`
- `failed`
- `skipped`

Optional:
- `result`
- `error`

When an active task is marked `completed`, the next ready task becomes `in_progress` automatically. When no open tasks remain, the plan is completed automatically.

#### `append_note`
Append an event/note to the plan timeline.

Required:
- `id`
- `content`

#### `delete`
Delete a plan permanently.

Required:
- `id`

### Practical pattern
1. Create the plan.
2. Set it to `active`.
3. Work normally with the existing tools.
4. Use `update_task` after meaningful milestones.
5. Use `append_note` for important findings or blockers.

### Output
Plan responses return structured JSON and include the current plan state where useful:
- `plan`
- `tasks`
- `events`
- `task_counts`
- `progress_pct`
- `current_task`

This tool is session-scoped and designed to complement the chat todo panel and prompt context.
