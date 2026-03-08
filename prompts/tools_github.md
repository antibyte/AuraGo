---
id: "tools_github"
tags: ["conditional"]
priority: 32
conditions: ["github_enabled"]
---
### GitHub Integration
| Tool | Purpose |
|---|---|
| `github` | Manage GitHub repositories, issues, pull requests, branches, files, commits, CI/CD runs, and track local projects |

**Repository operations:**
- `list_repos` — List all repositories (optionally filter by `owner`)
- `create_repo` — Create a new repository (`name` required, `description` optional; visibility follows config default)
- `delete_repo` — Delete a repository (`name` + `owner` required)
- `get_repo` — Get detailed info about a repo (`name` required)
- `search_repos` — Search GitHub repositories (`query` required, `limit` optional)

**Issue operations:**
- `list_issues` — List issues for a repo (`name` required, `value` = state filter: open/closed/all)
- `create_issue` — Create an issue (`name` = repo, `title` required, `body` optional, `label` = comma-separated labels)
- `close_issue` — Close an issue (`name` = repo, `id` = issue number)

**Pull Request & Branch operations:**
- `list_pull_requests` — List PRs (`name` = repo, `value` = state filter)
- `list_branches` — List branches (`name` = repo)

**File operations:**
- `get_file` — Read file from repo (`name` = repo, `path` = file path, `query` = branch)
- `create_or_update_file` — Create/update file (`name` = repo, `path`, `content` = base64, `body` = commit message, `value` = SHA for updates, `query` = branch)

**History & CI/CD operations:**
- `list_commits` — List recent commits (`name` = repo, `query` = branch, `limit` optional)
- `list_workflow_runs` — List GitHub Actions runs (`name` = repo, `limit` optional)

**Project tracking:**
- `list_projects` — Show all locally tracked GitHub projects with name, purpose, and repo URL
- `track_project` — Register a project for tracking (`name` = project/repo name, `content` or `description` = purpose)
- `untrack_project` — Remove a project from tracking (`name` = project name)

**Parameters:** `operation`, `name` (repo/project name), `owner` (defaults to configured owner), `title`, `body`, `description`, `path`, `content`, `query`, `value`, `id`, `label`, `limit`

**Important rules:**
- When creating a repository, ALWAYS also track it as a project with `track_project` — provide a clear purpose description.
- All GitHub project work (cloned repos, generated files) MUST be done in the `github/` subfolder of your workspace.
- When the user asks for a project overview, use `list_projects` to show the tracked project list.
