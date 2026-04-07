# Manage Plan (`manage_plan`)

Create and maintain a structured work plan for the current chat session.

Use this when the task is multi-step, risky, or likely to span several tool calls and you want visible progress tracking.

Notes are **not** replaced by plans:
- use `manage_plan` for session-scoped execution structure
- use `manage_notes` for standalone reminders and durable note-taking

Only one unfinished plan (`draft`, `active`, `paused`, or `blocked`) exists per session.

## When to use

- complex implementations with several dependent steps
- debugging or investigation with checkpoints
- work that should be easy to resume later in the same session

## When not to use

- tiny one-step tasks
- quick lookups or trivial edits
- long-term automation flows better handled by missions/follow-ups

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | `create`, `list`, `get`, `set_status`, `update_task`, `advance`, `set_blocker`, `clear_blocker`, `append_note`, `attach_artifact`, `split_task`, `reorder_tasks`, `archive_completed`, `delete` |
| `id` | string | for most ops | Plan ID |
| `task_id` | string | for task ops | Task ID within the plan |
| `status` | string | for set_status | New status (draft, active, paused, blocked, completed, cancelled) |
| `title` | string | for create | Plan title |
| `items` | array | for create, split_task | Task items array |
| `content` | string | optional | Original user request or note |
| `priority` | int | optional | 1=low, 2=medium, 3=high |

## Practical pattern

1. Create the plan with `create`.
2. Set it to `active` with `set_status`.
3. Work normally with the existing tools.
4. Use `update_task` after meaningful milestones.
5. Use `advance` for the normal happy-path handoff to the next step.
6. Use `set_blocker` / `clear_blocker` for real blockers.
7. Use `append_note` for important findings that are not task state changes.
8. Use `attach_artifact` when a file, URL, report, or generated output should stay attached to one task.
9. Use `split_task` when one task becomes too large.
10. Use `reorder_tasks` for reprioritization.
11. Archive completed plans once they are no longer actively useful.

## Notes

- **Task kinds**: `task` (generic), `tool` (specify tool_name), `reasoning` (thinking step), `verification` (test/check), `note` (info only)
- **Dependency format**: `depends_on` accepts task IDs or 1-based task indices (e.g. `[2]` means depends on task #2)
- **Auto-completion**: When an active task is marked `completed`, the next ready task becomes `in_progress` automatically. When no open tasks remain, the plan is completed automatically.
- **Session-scoped**: Plans exist only in the current session. Use `archive_completed` to clean up.
- **Output includes**: plan state, tasks, events, task_counts, progress_pct, current_task, recommendation, artifacts
