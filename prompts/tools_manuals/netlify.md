# Netlify (`netlify`)

Management of Netlify sites, deployments, environment variables, forms, notification hooks, and SSL certificates through the Netlify REST API. Mutating site-level operations are available only when the matching permission toggle is enabled and readonly mode is off.

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
| `get_env` | Get variable metadata only (key, scopes, timestamp, value count); secret values are never returned |
| `set_env` | Create or update an environment variable |
| `delete_env` | Delete an environment variable |
| `list_files` | List files in the production deploy |
| `list_forms` | List forms configured on the site |
| `get_submissions` | Get form submissions |
| `list_hooks` | List notification hooks |
| `create_hook` | Create a notification hook; requires site management |
| `delete_hook` | Delete a notification hook; requires site management |
| `provision_ssl` | Provision Let's Encrypt SSL certificate; requires site management |

`deploy_zip` and `deploy_draft` are not available in the agent flow: a ZIP cannot
be transported through tool arguments. Use the `homepage` tool with
`operation: deploy_netlify` instead.

Environment variable operations require the Netlify account (team). Without
`netlify.team_slug` a single visible account is used automatically; several
accounts produce an error naming the setting.

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
  allow_deploy: false       # Explicitly allow rollback/cancel_deploy
  allow_site_management: false  # Explicitly allow create/update/delete site, hooks, and SSL
  allow_env_management: false  # Explicitly allow set_env/delete_env
  team_slug: ""             # Optional: required when the token sees several accounts
  default_site_id: ""       # Optional: site UUID, name or custom domain
```

## Notes

- **Authentication**: A Personal Access Token (PAT) is required. Store it in the vault with key `netlify_token`.
- **Homepage/project deploys**: Do not use this tool or `homepage exec` with the Netlify CLI to upload website files. Direct CLI commands do not receive the vault token. For AuraGo homepage workspace projects, use `homepage_deploy` `deploy_netlify` with `project_dir`; it installs dependencies, builds, validates static output, uploads a hardened ZIP, polls Netlify until ready, and verifies the public URL.
- **Deploy URLs**: `deploy_url` in a deploy result is the immutable deploy permalink; `site_url` is the published site. Verification always uses the permalink.
- **Draft deploys**: `draft: true` publishes nothing. The result reports `published: false`; the Homepage ledger is not updated. A permalink that requires a Netlify login is reported as `verified: false` with a `verification_note`, not as an error.
- **ZIP contents**: only regular files are packed, and credential material (`.env*`, keys, certificates, `.netrc`, databases, logs) is filtered by name. A catch-all `_redirects` rewrite is added only for a single-page output; multi-page sites keep Netlify's 404.
- **ZIP deploys**: Binary ZIP data cannot be reliably transported through LLM tool arguments. Use `homepage_deploy` `deploy_netlify` instead for builds.
- **Destructive cleanup**: `delete_site` permanently deletes a site. Keep `netlify.allow_site_management=false` or `netlify.readonly=true` if agents should not be able to delete sites.
- **Rate limits**: General API: 500 requests/minute. Deploys: 3/minute, 100/day. Read requests retry a rate limit or a transient gateway error at most twice, honouring `Retry-After` for up to 30 seconds; mutations are sent exactly once, so a failed write is never silently repeated.
- **Site references**: `site_id` accepts the site UUID, its name or its custom domain. AuraGo resolves names across all result pages.
- **Permission model**: `readonly` blocks all mutations. `allow_deploy` controls rollback/cancel. `allow_site_management` controls create/update/delete site plus hook mutations and SSL provisioning. `allow_env_management` controls env var writes/deletes.
- **Form submissions**: Contain user-generated content and are wrapped in `<external_data>` for safety.
