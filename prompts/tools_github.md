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
- `list_repos` — List repositories allowed by policy (optionally filter by `owner`). An omitted `limit` returns 100 after filtering.
- `create_repo` — Create a repository (`name` required, `description` optional; visibility follows config default). Posts to the authenticated user when `github.owner` is that login or empty, otherwise to `/orgs/{owner}/repos`. A failed user lookup does not create a repository. AuraGo auto-tracks a successful create as agent-created.
- `delete_repo` — Delete a repository (`name` + `owner` required). A refusal restores a pre-existing trust grant. A request that never receives an answer reports `trust_revoked`.
- `get_repo` — Get detailed info about a repo (`name` required)
- `search_repos` — Search GitHub repositories (`query` required, `limit` optional). Private repositories appear only when policy allows them.

**Issue operations:**
- `list_issues` — List issues for a repo (`name` required, `value` = state filter: open/closed/all)
- `create_issue` — Create an issue (`name` = repo, `title` required, `body` optional, `description` used only when `body` is empty, `label` = comma-separated labels)
- `close_issue` — Close an issue (`name` = repo, `id` = issue number)

**Pull Request & Branch operations:**
- `list_pull_requests` — List PRs (`name` = repo, `value` = state filter)
- `list_branches` — List branches (`name` = repo)

**File operations:**
- `get_file` — Read a path from a repo (`name` = repo, `path`, `query` = branch). Text comes back as UTF-8, binary as base64, and a directory as `type: dir` with `entries`.
- `create_or_update_file` — Create/update file (`name` = repo, `path`, `content` = plain text or round-trip base64, `body` = commit message, `value` = SHA for updates, `query` = branch)

**History & CI/CD operations:**
- `list_commits` — List recent commits (`name` = repo, `query` = branch, `limit` optional)
- `list_workflow_runs` — List GitHub Actions runs (`name` = repo, `limit` optional)

**Project tracking:**
- `list_projects` — Show all locally tracked GitHub projects with name, purpose, and repo URL
- `track_project` — Register a project for tracking (`name` = project/repo name, `content` or `description` = purpose)
- `untrack_project` — Remove a project from tracking (`name` = project name)

**Parameters:** `operation`, `name` (repo/project name), `owner` (defaults to configured owner), `title`, `body`, `description`, `path`, `content`, `query`, `value`, `id`, `label`, `limit`

**Important rules:**
- Repository access is `github.allowed_repos` plus the protected `github_trust.json` ledger. Workspace `agent_created` does not grant access.
- Prefer `owner/repo` allowlist entries. Legacy bare repo names only match the configured GitHub owner.
- `track_project` is local inventory only. It never grants remote repository access.
- All GitHub project work (cloned repos, generated files) MUST be done in the `github/` subfolder of your workspace.
- When the user asks for a project overview, use `list_projects` to show the tracked project list.
