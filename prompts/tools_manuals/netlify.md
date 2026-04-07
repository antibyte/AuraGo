# Netlify (`netlify`)

Full management of Netlify sites, deployments, environment variables, forms, notification hooks, and SSL certificates through the Netlify REST API.

## Operations

| Operation | Description |
|-----------|-------------|
| `check_connection` | Test Netlify API connectivity |
| `list_sites` | List all sites in the account |
| `get_site` | Get detailed site information |
| `create_site` | Create a new Netlify site |
| `update_site` | Update site settings |
| `delete_site` | Permanently delete a site |
| `list_deploys` | List recent deploys for a site |
| `get_deploy` | Get details about a specific deploy |
| `rollback` | Restore a previous deploy |
| `cancel_deploy` | Cancel a pending/in-progress deploy |
| `list_env` | List environment variables for a site |
| `get_env` | Get a specific env var details |
| `set_env` | Create or update an environment variable |
| `delete_env` | Delete an environment variable |
| `list_files` | List files in the production deploy |
| `list_forms` | List forms configured on the site |
| `get_submissions` | Get form submissions |
| `list_hooks` | List notification hooks |
| `create_hook` | Create a notification hook |
| `delete_hook` | Delete a notification hook |
| `provision_ssl` | Provision Let's Encrypt SSL certificate |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `site_id` | string | for most operations | Netlify site ID |
| `deploy_id` | string | for get_deploy, rollback, cancel_deploy | Deploy ID |
| `site_name` | string | for create_site, update_site | Site name |
| `custom_domain` | string | for create_site, update_site | Custom domain |
| `env_key` | string | for get_env, set_env, delete_env | Environment variable key |
| `env_value` | string | for set_env | Environment variable value |
| `env_context` | string | for set_env | Context: `all`, `production`, `deploy-preview`, `branch-deploy`, `dev` |
| `hook_type` | string | for create_hook | Type: `url`, `email`, `slack` |
| `hook_event` | string | for create_hook | Event: `deploy_created`, `deploy_building`, `deploy_failed` |
| `hook_id` | string | for delete_hook | Hook ID |
| `form_id` | string | for get_submissions | Form ID |

## Examples

**Check API connection:**
```json
{"action": "netlify", "operation": "check_connection"}
```

**List sites:**
```json
{"action": "netlify", "operation": "list_sites"}
```

**Create a new site:**
```json
{"action": "netlify", "operation": "create_site", "site_name": "my-awesome-site", "custom_domain": "example.com"}
```

**Set an environment variable:**
```json
{"action": "netlify", "operation": "set_env", "site_id": "abc123", "env_key": "API_URL", "env_value": "https://api.example.com", "env_context": "production"}
```

**Provision SSL:**
```json
{"action": "netlify", "operation": "provision_ssl", "site_id": "abc123"}
```

## Configuration

```yaml
netlify:
  enabled: true
  # Personal Access Token stored in vault: netlify_token
  # Generate at: https://app.netlify.com/user/applications#personal-access-tokens
  readonly: false           # Set true to block mutations
  allow_deploy: true        # Allow rollback/cancel_deploy
  allow_site_management: true  # Allow create_site, update_site, delete_site
  allow_env_management: true  # Allow set_env, delete_env
```

## Notes

- **Authentication**: A Personal Access Token (PAT) is required. Store it in the vault with key `netlify_token`.
- **ZIP deploys**: Binary ZIP data cannot be reliably transported through LLM tool arguments. Use `homepage` → `deploy_netlify` instead for builds.
- **Rate limits**: General API: 500 requests/minute. Deploys: 3/minute, 100/day.
- **Permission model**: `readonly` blocks all mutations. Other flags control specific operations.
- **Form submissions**: Contain user-generated content and are wrapped in `<external_data>` for safety.
