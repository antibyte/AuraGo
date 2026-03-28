## Tool: Manage Plan (`manage_plan`)

Create and maintain a structured work plan for the current chat session.

Use this when the task is multi-step, risky, or likely to span several tool calls and you want visible progress tracking.

Notes are **not** replaced by plans:
- use `manage_plan` for session-scoped execution structure
- use `manage_notes` for standalone reminders and durable note-taking

Only one unfinished plan (`draft`, `active`, `paused`, or `blocked`) exists per session.

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
- `acceptance_criteria`
- `owner` (`agent`, `user`, `external`)
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
- `status` (`all`, `draft`, `active`, `paused`, `blocked`, `completed`, `cancelled`)
- `limit`
- `include_archived` (`true` to include archived plans)

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
- `blocked`
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
- `blocked`
- `completed`
- `failed`
- `skipped`

Optional:
- `result`
- `error`

When an active task is marked `completed`, the next ready task becomes `in_progress` automatically. When no open tasks remain, the plan is completed automatically.

#### `advance`
Mark the current `in_progress` task as completed and move to the next ready task.

Required:
- `id`

Optional:
- `result`

Use this when you have finished the current active step and want the plan to continue safely.

#### `set_blocker`
Mark a task as blocked and move the whole plan into `blocked`.

Required:
- `id`
- `task_id`
- `reason` or `content`

#### `clear_blocker`
Clear a blocked task and reactivate the plan.

Required:
- `id`
- `task_id`

Optional:
- `content` or `reason` as a short unblock note

#### `append_note`
Append an event/note to the plan timeline.

Required:
- `id`
- `content`

#### `attach_artifact`
Attach an artifact to a specific task.

Required:
- `id`
- `task_id`

And one artifact value via:
- `content`
- `file_path`
- or `url`

Optional:
- `label`
- `artifact_type` such as `file`, `url`, `id`, or `report`

#### `split_task`
Replace one open task with several sequential subtasks.

Required:
- `id`
- `task_id`
- `items` with at least two subtask definitions

Each subtask item can use the same fields as `create` task items.

Use this when a task turns out to be too large and should be broken into smaller actionable steps.

#### `reorder_tasks`
Reorder all tasks in a plan.

Required:
- `id`
- `items` containing every task exactly once in the desired final order

For reorder, each item only needs:
- `task_id`

#### `archive_completed`
Archive finished plans so they stop cluttering the default session list.

Options:
- `id` to archive one completed or cancelled plan
- omit `id` to archive all completed/cancelled plans in the current session

Archived plans are excluded from the default `list` output unless `include_archived=true`.

#### `delete`
Delete a plan permanently.

Required:
- `id`

### Practical pattern
1. Create the plan.
2. Set it to `active`.
3. Work normally with the existing tools.
4. Use `update_task` after meaningful milestones.
5. Use `advance` for the normal happy-path handoff to the next step.
6. Use `set_blocker` / `clear_blocker` for real blockers.
7. Use `append_note` for important findings that are not task state changes.
8. Use `attach_artifact` when a file, URL, report, or generated output should stay attached to one task.
9. Use `split_task` when one task becomes too large.
10. Use `reorder_tasks` for reprioritization.
11. Archive completed plans once they are no longer actively useful.

### Output
Plan responses return structured JSON and include the current plan state where useful:
- `plan`
- `tasks`
- `events`
- `task_counts`
- `progress_pct`
- `current_task`
- `recommendation`
- `artifacts`

This tool is session-scoped and designed to complement the chat todo panel and prompt context.
