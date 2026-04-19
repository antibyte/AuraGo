# Vercel (`vercel`)

Manage Vercel projects, deployments, environment variables, domains, and aliases via the Vercel REST API.

## Operations

| Operation | Description |
|-----------|-------------|
| `check_connection` | Test Vercel API connectivity |
| `list_projects` | List all projects in the account/team |
| `get_project` | Get detailed project information |
| `create_project` | Create a new Vercel project |
| `update_project` | Update project settings |
| `list_deployments` | List recent deployments for a project |
| `get_deployment` | Get details about a specific deployment |
| `list_env` | List project environment variables |
| `set_env` | Create or update an environment variable |
| `delete_env` | Delete an environment variable |
| `list_domains` | List project domains |
| `add_domain` | Add a domain to a project |
| `verify_domain` | Check the verification state of a domain |
| `list_aliases` | List aliases for the account, project, or deployment |
| `assign_alias` | Assign an alias or custom domain to a deployment |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `project_id` | string | for most project operations | Vercel project name or ID |
| `project_name` | string | for create/update | Human-readable project name |
| `deployment_id` | string | for get_deployment, list_aliases, assign_alias | Deployment ID |
| `env_key` | string | for set_env, delete_env | Environment variable key |
| `env_value` | string | for set_env | Environment variable value |
| `env_target` | string | for set_env | `production`, `preview`, `development`, or comma-separated values |
| `domain` | string | for add_domain, verify_domain | Domain to manage |
| `alias` | string | for assign_alias | Alias or custom domain to assign |
| `framework` | string | for create/update | Framework slug such as `nextjs`, `vite`, `astro`, `nuxtjs` |
| `root_directory` | string | optional | Root directory override |
| `output_directory` | string | optional | Output directory override |

## Examples

**Check API connection:**
```json
{"action": "vercel", "operation": "check_connection"}
```

**List projects:**
```json
{"action": "vercel", "operation": "list_projects"}
```

**Create a project:**
```json
{"action": "vercel", "operation": "create_project", "project_name": "my-homepage", "framework": "vite"}
```

**Set an environment variable:**
```json
{"action": "vercel", "operation": "set_env", "project_id": "my-homepage", "env_key": "API_URL", "env_value": "https://api.example.com", "env_target": "production,preview"}
```

**Assign a custom domain to a deployment:**
```json
{"action": "vercel", "operation": "assign_alias", "deployment_id": "dpl_123", "alias": "www.example.com"}
```

## Configuration

```yaml
vercel:
  enabled: true
  readonly: false
  allow_deploy: true
  allow_project_management: false
  allow_env_management: false
  allow_domain_management: false
  default_project_id: ""
  team_id: ""
  team_slug: ""
  # Personal Access Token stored in vault: vercel_token
```

## Notes

- **Authentication**: Store a Vercel Personal Access Token in the vault with key `vercel_token`.
- **Permission model**: `readonly` blocks all mutations. The `allow_*` flags further gate deployments, project changes, environment variables, and domains.
- **Scope handling**: Team-scoped API calls use `team_id` or `team_slug` when configured.
- **Homepage publishing**: Use `homepage` → `deploy_vercel` for homepage workspace deployments instead of hand-assembling CLI calls through `exec`.
