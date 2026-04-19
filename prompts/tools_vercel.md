---
id: "tools_vercel"
tags: ["conditional"]
priority: 32
conditions: ["vercel_enabled"]
---
### Vercel Integration
| Tool | Purpose |
|---|---|
| `vercel` | Manage Vercel projects, deployments, environment variables, domains, and aliases |

**Project operations:**
- `list_projects` — List all accessible Vercel projects
- `get_project` — Get details for a Vercel project (`project_id` optional if default configured)
- `create_project` — Create a new Vercel project (`project_name`, `framework`, `root_directory`, `output_directory`)
- `update_project` — Update project settings (`project_id`, `project_name`, `framework`, `root_directory`, `output_directory`)

**Deployment operations:**
- `list_deployments` — List recent deployments for a project
- `get_deployment` — Get details for a deployment (`deployment_id`)

**Environment variable operations:**
- `list_env` — List project environment variables
- `set_env` — Create or update an environment variable (`env_key`, `env_value`, `env_target`)
- `delete_env` — Delete an environment variable (`env_key`)

**Domain & alias operations:**
- `list_domains` — List project domains
- `add_domain` — Add a domain to a project (`domain`)
- `verify_domain` — Check verification status for a project domain (`domain`)
- `list_aliases` — List aliases globally, per project, or per deployment
- `assign_alias` — Assign an alias or custom domain to a deployment (`deployment_id`, `alias`)

**Diagnostics:**
- `check_connection` — Test connectivity to the Vercel API and validate the stored token

**Homepage → Vercel deployment:**
- Use `homepage` → `deploy_vercel` for homepage workspace publishing
  - Params: `project_dir`, `project_id` (optional), `build_dir` (optional), `target`, `alias`, `domain`

📖 See **tools_manuals/vercel.md** for detailed usage.
