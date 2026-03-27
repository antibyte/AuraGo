# Netlify Tool — Detailed Manual

## Overview
The `netlify` tool provides full management of Netlify sites, deployments, environment variables, forms, notification hooks, and SSL certificates through the Netlify REST API.

## Authentication
A Personal Access Token (PAT) is required. Store it in the vault with key `netlify_token`.
Generate a token at: https://app.netlify.com/user/applications#personal-access-tokens

## Operations

### Diagnostics

#### check_connection
Tests connectivity to the Netlify API in three steps: DNS resolution → TCP connection → authenticated API call.
Always run this first if any Netlify operation fails with a network error.
```json
{"operation": "check_connection"}
```
Returns `dns_ok`, `tcp_ok`, `api_ok`, resolved IPs, and account info on success.
If `tcp_ok: false`, the Netlify API is blocked by a firewall — this is a network/infrastructure problem, not a token problem.

### Sites

#### list_sites
Lists all sites in the account or team.
```json
{"operation": "list_sites"}
```

#### get_site
Gets detailed information about a specific site.
```json
{"operation": "get_site", "site_id": "abc123"}
```
If `default_site_id` is configured, `site_id` can be omitted.

#### create_site
Creates a new Netlify site.
```json
{"operation": "create_site", "site_name": "my-awesome-site", "custom_domain": "example.com"}
```
- `site_name` becomes `my-awesome-site.netlify.app`
- `custom_domain` is optional

#### update_site
Updates site settings (name, custom domain).
```json
{"operation": "update_site", "site_id": "abc123", "site_name": "new-name", "custom_domain": "new.example.com"}
```

#### delete_site
Permanently deletes a site. **This cannot be undone.**
```json
{"operation": "delete_site", "site_id": "abc123"}
```

### Deploys

#### list_deploys
Lists recent deploys (up to 20) for a site.
```json
{"operation": "list_deploys", "site_id": "abc123"}
```

#### get_deploy
Gets details about a specific deploy.
```json
{"operation": "get_deploy", "deploy_id": "def456"}
```

#### deploy_zip / deploy_draft
⚠️ **Not part of the supported agent flow.** These operations require passing a valid base64-encoded ZIP via `content`, but binary data cannot be reliably transported through LLM tool arguments (the ZIP will be truncated or corrupted, leading to 400 errors from the Netlify API).

**Always use `homepage` → `deploy_netlify` instead.** It performs the build, creates the ZIP, and uploads it entirely server-side without the agent needing to handle binary data:
```json
{"operation": "deploy_netlify", "project_dir": "my-site", "site_id": "abc123", "title": "v1.2.0", "draft": false}
```
If you truly need a manual ZIP deploy, do it outside the agent flow.

#### rollback
Restores a previous deploy.
```json
{"operation": "rollback", "site_id": "abc123", "deploy_id": "def456"}
```

#### cancel_deploy
Cancels a pending or in-progress deploy.
```json
{"operation": "cancel_deploy", "deploy_id": "def456"}
```

### Environment Variables

#### list_env
Lists all environment variables for a site (keys and scopes, no secret values).
```json
{"operation": "list_env", "site_id": "abc123"}
```

#### get_env
Gets details of a specific env var including its context-scoped values.
```json
{"operation": "get_env", "site_id": "abc123", "env_key": "API_URL"}
```

#### set_env
Creates or updates an environment variable.
```json
{"operation": "set_env", "site_id": "abc123", "env_key": "API_URL", "env_value": "https://api.example.com", "env_context": "production"}
```
- `env_context`: `all` (default), `production`, `deploy-preview`, `branch-deploy`, `dev`

#### delete_env
Deletes an environment variable.
```json
{"operation": "delete_env", "site_id": "abc123", "env_key": "OLD_VAR"}
```

### Files
#### list_files
Lists all files in the current production deploy.
```json
{"operation": "list_files", "site_id": "abc123"}
```

### Forms
#### list_forms
Lists all forms configured on the site.
```json
{"operation": "list_forms", "site_id": "abc123"}
```

#### get_submissions
Gets form submissions. **⚠️ Contains user-generated content wrapped in `<external_data>` for safety.**
```json
{"operation": "get_submissions", "form_id": "form123"}
```

### Hooks (Notifications)
#### list_hooks
Lists all notification hooks for a site.
```json
{"operation": "list_hooks", "site_id": "abc123"}
```

#### create_hook
Creates a notification hook.
```json
{"operation": "create_hook", "site_id": "abc123", "hook_type": "url", "hook_event": "deploy_created", "url": "https://hooks.slack.com/services/..."}
```
- `hook_type`: `url` (webhook), `email`, `slack`
- `hook_event`: `deploy_created`, `deploy_building`, `deploy_failed`, `deploy_locked`, `deploy_unlocked`
- `url`: Required for type `url`
- `value`: Email address for type `email`

#### delete_hook
Deletes a notification hook.
```json
{"operation": "delete_hook", "hook_id": "hook123"}
```

### SSL
#### provision_ssl
Provisions a Let's Encrypt SSL certificate for the site's custom domain.
```json
{"operation": "provision_ssl", "site_id": "abc123"}
```

## Permission Model
- **read_only**: Blocks all mutating operations (create, update, delete, deploy)
- **allow_deploy**: Controls rollback and cancel_deploy
- **allow_site_management**: Controls create_site, update_site, delete_site
- **allow_env_management**: Controls set_env, delete_env

## Rate Limits
- General API: 500 requests/minute
- Deploys: 3/minute, 100/day
- Large file uploads: 60-second timeout

## Homepage Integration
**Use `homepage` → `deploy_netlify` for a one-step build+deploy:**
```json
{"action": "homepage", "operation": "deploy_netlify", "project_dir": "landing-page", "site_id": "abc123", "title": "v1.0"}
```
This automatically builds (if a build script exists), packages as ZIP and uploads to Netlify.

**Manual ZIP deploys are intentionally excluded from the normal agent workflow.**
If you need custom packaging, prepare and upload it outside the agent path instead of sending ZIP/base64 through tool arguments.
