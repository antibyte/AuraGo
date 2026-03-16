# GitHub Tool (`github`)

Manage GitHub repositories, issues, pull requests, branches, files, commits, and workflow runs.

## Operations

| Operation | Description | Key Parameters |
|-----------|-------------|----------------|
| `list_repos` | List repositories | `limit` |
| `get_repo` | Get repository details | `name` |
| `create_repo` | Create a new repository | `name`, `description` |
| `delete_repo` | Delete a repository | `name` |
| `search_repos` | Search repositories | `query`, `limit` |
| `list_issues` | List issues | `name`, `value` (state filter) |
| `create_issue` | Create an issue | `name`, `title`, `description`, `label` |
| `close_issue` | Close an issue | `name`, `id` |
| `list_pull_requests` | List PRs | `name`, `value` (state filter) |
| `list_branches` | List branches | `name` |
| `list_commits` | List commits | `name`, `query` (branch) |
| `get_file` | Get file contents | `name`, `path`, `query` (branch) |
| `create_or_update_file` | Create/update a file | `name`, `path`, `content` (base64), `body` (commit msg), `value` (SHA) |
| `list_workflow_runs` | List CI/CD runs | `name`, `limit` |
| `list_projects` | List tracked projects | — |
| `track_project` | Track a local project | `name`, `content` (purpose) |
| `untrack_project` | Untrack a project | `name` |

## Examples

```json
{"action": "github", "operation": "list_repos", "limit": 10}
```

```json
{"action": "github", "operation": "create_issue", "name": "my-repo", "title": "Bug: Login fails", "description": "Steps to reproduce...", "label": "bug,high-priority"}
```

```json
{"action": "github", "operation": "get_file", "name": "my-repo", "path": "README.md"}
```

```json
{"action": "github", "operation": "list_workflow_runs", "name": "my-repo", "limit": 5}
```

## Notes
- `owner` defaults to the configured GitHub owner if omitted
- `value` is used as state filter for issues/PRs (`open`, `closed`, `all`)
- File content for `create_or_update_file` must be base64-encoded
