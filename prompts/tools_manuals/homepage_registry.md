# Homepage Registry Tool

Track homepage projects with their framework, URLs, deployment history, edits, and known problems.
Projects are automatically registered and updated when you use `homepage` operations (init_project, build, deploy, lighthouse).

## Prerequisites
- `homepage.enabled: true` in config.yaml
- DB is auto-created at `sqlite.homepage_registry_path`

## Operations

### register — Register a new project
```json
{
  "action": "homepage_registry",
  "operation": "register",
  "name": "My Portfolio",
  "description": "Personal portfolio site with project showcase",
  "framework": "astro",
  "url": "https://mysite.example.com",
  "project_dir": "/workspace/portfolio",
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
  "description": "Updated portfolio with new project section",
  "url": "https://newsite.example.com",
  "tags": ["portfolio", "redesigned"]
}
```

### delete — Soft-delete a project
```json
{"action": "homepage_registry", "operation": "delete", "id": 1}
```

### log_edit — Record an edit to a project
```json
{"action": "homepage_registry", "operation": "log_edit", "id": 1, "reason": "Added contact form and updated styling"}
```

### log_deploy — Record a deployment
```json
{"action": "homepage_registry", "operation": "log_deploy", "id": 1, "url": "https://mysite.example.com"}
```

### log_problem — Record a known problem
```json
{"action": "homepage_registry", "operation": "log_problem", "id": 1, "problem": "Mobile navigation menu doesn't close on tap"}
```

### resolve_problem — Mark a problem as resolved
```json
{"action": "homepage_registry", "operation": "resolve_problem", "id": 1}
```

## Notes
- Projects are auto-registered when you use `homepage` → `init_project`
- Builds auto-log edits, deploys auto-log deployments
- Lighthouse scores are auto-stored when you run `homepage` → `lighthouse`
- Use `log_problem` to track issues, `resolve_problem` to clear them
- Status values: `active`, `archived`, `maintenance`
