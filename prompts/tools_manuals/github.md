# GitHub Tool (`github`)

Manage GitHub repositories, issues, pull requests, branches, files, commits, and workflow runs.

## Operations

| Operation | Description | Key Parameters |
|-----------|-------------|----------------|
| `list_repos` | List repositories allowed by policy. An omitted `limit` returns 100 after filtering. The config UI and dashboard collect up to 10 pages. | `limit` |
| `get_repo` | Get repository details | `name` |
| `create_repo` | Create a repository under the authenticated user when `github.owner` is that user or empty, otherwise under `/orgs/{owner}/repos`, then auto-track it as agent-created | `name`, `description` |
| `delete_repo` | Delete a repository; requires `github.allow_delete`. A refusal restores a pre-existing trust grant. A request that never receives an answer leaves `trust_revoked` in the error. | `name` |
| `search_repos` | Search GitHub. Private repositories appear only when policy allows them. | `query`, `limit` |
| `list_issues` | List issues | `name`, `value` (state filter) |
| `create_issue` | Create an issue. `description` is accepted when `body` is empty. | `name`, `title`, `body`, `label` |
| `close_issue` | Close an issue | `name`, `id` |
| `list_pull_requests` | List PRs | `name`, `value` (state filter) |
| `list_branches` | List branches | `name` |
| `list_commits` | List commits | `name`, `query` (branch) |
| `get_file` | Get file contents. Text files return UTF-8, binary files return base64, and a directory returns `type: dir` with `entries`. | `name`, `path`, `query` (branch) |
| `create_or_update_file` | Create/update a file. Plain text is accepted. A value that is already standard base64 and round-trips is sent unchanged. | `name`, `path`, `content`, `body` (commit msg), `value` (SHA) |
| `list_workflow_runs` | List CI/CD runs | `name`, `limit` |
| `list_projects` | List tracked projects | — |
| `track_project` | Track a local project | `name`, `content` (purpose) |
| `untrack_project` | Untrack a project | `name` |

## Examples

```json
{"action": "github", "operation": "list_repos", "limit": 10}
```

```json
{"action": "github", "operation": "create_issue", "name": "my-repo", "title": "Bug: Login fails", "body": "Steps to reproduce...", "label": "bug,high-priority"}
```

```json
{"action": "github", "operation": "get_file", "name": "my-repo", "path": "README.md"}
```

```json
{"action": "github", "operation": "list_workflow_runs", "name": "my-repo", "limit": 5}
```

## Notes
- `owner` defaults to the configured GitHub owner if omitted
- `github.allowed_repos` is strict. Prefer `owner/repo`; legacy bare repo names only match the configured owner.
- If `github.allowed_repos` is empty, the agent may only access repos recorded by AuraGo in its protected server trust ledger after creation. Workspace `agent_created` flags do not grant access. Old entries require an administrator to approve the repository in Settings.
- `delete_repo` additionally requires `github.allow_delete`, which defaults off. Read-only mode still blocks deletion.
- A successful creation with `trust_migration_required` means the repository exists but needs administrator approval. Do not repeat the creation.
- `track_project` only records local project metadata and does not grant remote GitHub access.
- `value` is used as state filter for issues/PRs (`open`, `closed`, `all`)
- `create_issue` uses `body`. When `body` is empty, `description` is the issue text.
- `create_or_update_file` accepts plain text. Standard base64 that round-trips is sent unchanged.
