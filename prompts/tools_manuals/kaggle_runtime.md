# kaggle_runtime

Manage the shared Kaggle workspace, durable jobs and **local** stateful Python kernels. This tool does not execute individual cells on Kaggle.

| Operation | Inputs / result |
| --- | --- |
| `status` | Current permissions, configuration and storage usage. No remote mutation. |
| `projects` | Managed project IDs and titles. |
| `create_project` | `title`; returns the new project ID. |
| `files` | `project_id`; lists regular project files. |
| `read_file` | `project_id`, `file`; bounded UTF-8 preview for logs and text results (256 KiB). Binary files use Desktop download. |
| `load_notebook` | `project_id`, `file`; returns nbformat-4 `content` and `revision`. |
| `save_notebook` | `project_id`, `file`, `revision`, `notebook_json`; complete notebook JSON, preserving metadata/attachments/outputs. Use revision `*` only to create a new file. Conflicts never overwrite newer content. |
| `jobs` | Durable local job receipts, progress, results and remote status. |
| `events` | Optional `after` cursor; returns up to 100 durable events and `next_cursor`. Poll for local cell outputs; match session/revision/cell/execution IDs. `gap` means retained output expired; never silently treat it as complete. |
| `cancel_job` | `job_id`; cancel a tracked transfer or request cancellation of a remote run. No auto-retry after an uncertain mutation. |
| `sessions` | Local kernel sessions and state. |
| `start` | `project_id`, `file`, explicitly selected `files`, optional `internet` (default false). Wait for session `idle`. |
| `execute` | `session_id`, saved `revision`, `cell_id`. Returns `execution_id`; observe outputs through the shared SSE channel. |
| `complete` | `session_id`, `code`, Unicode code-point `cursor`; completion only, never execution. |
| `interrupt` | `session_id`; interrupt the active cell. |
| `restart` | `session_id`; discard variables and restart without rerunning code. |
| `stop` | `session_id`; remove the managed container and discard variables. |

Local execution needs integration activation, non-read-only mode, local runtime and local execution grants, plus global Python, Docker and filesystem writes. Internet additionally needs the local internet grant and global network permission. Credentials never enter the kernel. The runtime uses one isolated, unprivileged CPU container per session, no host mounts or HTTP port, and only selected input files. Use `kaggle_notebooks.save_kernel` to execute a complete version on Kaggle.

Closing a Desktop window only detaches its observer. Idle expiry, revocation or backend shutdown stops the session. Reconnect never reruns code. Output is bounded and untrusted; correlate notebook revision, cell ID and execution ID. Preserve the original MIME bundles while keeping HTML sanitized and JavaScript/widgets inert.

Defaults: one session, 2 CPU cores, 4 GiB RAM, 15-minute idle timeout and 30-minute cell deadline. Projects have a 10 GiB total budget and 2 GiB transfer limit. Use the current `status` rather than assuming defaults.

Remote cancellation remains pending until confirmed by Kaggle. If a notebook run has no verifiable remote session ID, cancel it on Kaggle; do not guess an ID. `unknown` job outcomes require investigation and must not be automatically replayed.

Only an administrator in Desktop Jobs can release an `unknown` compute slot after confirming inactivity on Kaggle. The resulting `resolved_unknown` status records manual cleanup, not success or cancellation, and preserves the original receipt/idempotency binding. This tool cannot perform that action; never replay or infer a successful outcome from the released slot.
