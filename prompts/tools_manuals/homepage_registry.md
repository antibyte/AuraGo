# Homepage Registry Tool

Track homepage projects with their framework, URLs, deployment history, edits, known problems, and chronological project history (decisions, notes, feedback, observations).
Projects are automatically registered and updated when you use focused homepage operations (`homepage_project` init_project, `homepage_deploy` build/deploy, `homepage_quality` lighthouse).

You MUST actively maintain the project history. Read `list_history` before making changes and add an `add_history` entry after every meaningful change.

## Prerequisites
- `homepage.enabled: true` in config.yaml
- DB is auto-created at `sqlite.homepage_registry_path`

## Identity Contract

`project_dir` is required when registering a project. It must be relative to the homepage workspace, for example `portfolio` or `sites/portfolio`, never `/workspace/portfolio`. AuraGo rejects new entries without `project_dir` so later cleanup and deployment tasks can always map a registry row back to the correct local folder.

## Operations

### register — Register a new project
Required fields for `register`: `name`, `project_dir`.

```json
{
  "action": "homepage_registry",
  "operation": "register",
  "name": "My Portfolio",
  "description": "Personal portfolio site with project showcase",
  "framework": "astro",
  "url": "https://mysite.example.com",
  "project_dir": "portfolio",
  "tags": ["portfolio", "personal"]
}
```

### search — Search projects by name, description, URL, or framework
```json
{"action": "homepage_registry", "operation": "search", "query": "portfolio", "limit": 10}
```

### get — Get a single project by ID
```json
{"action": "homepage_registry", "operation": "get", "id": 1}
```

### list — List all projects with optional filters
```json
{"action": "homepage_registry", "operation": "list", "limit": 20}
{"action": "homepage_registry", "operation": "list", "status": "active"}
```

### update — Update project metadata
```json
{
  "action": "homepage_registry",
  "operation": "update",
  "id": 1,
  "project_dir": "portfolio",
  "description": "Updated portfolio with new project section",
  "url": "https://newsite.example.com",
  "tags": ["portfolio", "redesigned"]
}
```

### delete — Soft-delete a project
```json
{"action": "homepage_registry", "operation": "delete", "id": 1, "project_dir": "portfolio"}
```

### log_edit — Record an edit to a project
```json
{"action": "homepage_registry", "operation": "log_edit", "id": 1, "project_dir": "portfolio", "reason": "Added contact form and updated styling"}
```

### log_deploy — Record a deployment
```json
{"action": "homepage_registry", "operation": "log_deploy", "id": 1, "project_dir": "portfolio", "url": "https://mysite.example.com"}
```

### log_problem — Record a known problem
```json
{"action": "homepage_registry", "operation": "log_problem", "id": 1, "project_dir": "portfolio", "problem": "Mobile navigation menu doesn't close on tap"}
```

### resolve_problem — Mark a problem as resolved
```json
{"action": "homepage_registry", "operation": "resolve_problem", "id": 1, "project_dir": "portfolio"}
```

## Project History (MUST USE)

Every project has a chronological history. This is not optional. Use it to remember decisions, user feedback, open questions, and milestones across sessions.

### Before making changes
1. Call `homepage_registry` → `list_history` for the project (`id` = project ID).
2. Read recent entries to understand prior decisions and user intent.
3. If something is unclear, ask the user or call `get_history` for details.

### After making changes
1. After any `homepage_file` write/edit, `homepage` build/deploy, or `homepage_project` init, call `homepage_registry` → `add_history`.
2. Pick the right `entry_type`:
   - `decision` — design or architecture choices
   - `note` — general observation
   - `feedback` — captured user feedback
   - `question` — open question or assumption
   - `milestone` — completed goal (e.g. "Hero section done")
   - `observation` — finding from quality checks
3. Write a concise `content`: what changed, why, and next steps.
4. Set `source` to the originating tool (e.g. `homepage_file`, `homepage_deploy`).
5. Include the same workspace-relative `project_dir` used for the project.

### add_history — Add a history entry
```json
{
  "action": "homepage_registry",
  "operation": "add_history",
  "id": 1,
  "project_dir": "portfolio",
  "entry_type": "decision",
  "content": "User wants a dark hero section with single CTA. Carousel rejected in favor of static gallery.",
  "source": "homepage_file",
  "tags": ["design", "hero"]
}
```

### list_history — List history entries for a project
```json
{"action": "homepage_registry", "operation": "list_history", "id": 1, "limit": 20}
```

### get_history — Get a single history entry
```json
{"action": "homepage_registry", "operation": "get_history", "history_id": 42}
```

### search_history — Search history content
```json
{"action": "homepage_registry", "operation": "search_history", "id": 1, "history_query": "hero", "limit": 10}
```

### update_history — Update an existing entry
```json
{"action": "homepage_registry", "operation": "update_history", "history_id": 42, "project_dir": "portfolio", "content": "Updated decision text"}
```

### delete_history — Delete an entry
```json
{"action": "homepage_registry", "operation": "delete_history", "history_id": 42, "project_dir": "portfolio"}
```

## Notes
- Projects are auto-registered when you use `homepage_project` `init_project`
- Builds auto-log edits, deploys auto-log deployments
- Lighthouse scores are auto-stored when you run `homepage` → `lighthouse`
- Use `log_problem` to track issues, `resolve_problem` to clear them
- Status values: `active`, `archived`, `maintenance`
