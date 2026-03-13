---
id: "tools_netlify"
tags: ["conditional"]
priority: 32
conditions: ["netlify_enabled"]
---
### Netlify Integration
| Tool | Purpose |
|---|---|
| `netlify` | Manage Netlify sites, deployments, environment variables, forms, hooks, and SSL certificates |

**Site operations:**
- `list_sites` — List all Netlify sites for your account/team
- `get_site` — Get detailed info about a site (`site_id` optional if default configured)
- `create_site` — Create a new site (`site_name` = subdomain, `custom_domain` optional)
- `update_site` — Update site settings (`site_id`, `site_name`, `custom_domain`)
- `delete_site` — Permanently delete a site (`site_id` required)

**Deploy operations:**
- `list_deploys` — List recent deploys for a site
- `get_deploy` — Get deploy details (`deploy_id` required)
- `deploy_zip` — Deploy a ZIP archive (`content` = base64 ZIP, `site_id`, `title`, `draft`)
- `deploy_draft` — Deploy as draft (same as deploy_zip with draft=true)
- `rollback` — Rollback to a previous deploy (`site_id`, `deploy_id`)
- `cancel_deploy` — Cancel a pending deploy (`deploy_id`)

**Environment variable operations:**
- `list_env` — List all env vars for a site
- `get_env` — Get details of a specific env var (`env_key`)
- `set_env` — Create or update env var (`env_key`, `env_value`, `env_context`)
- `delete_env` — Delete an env var (`env_key`)

**File & Form operations:**
- `list_files` — List files in the current deploy
- `list_forms` — List all forms for a site
- `get_submissions` — Get form submissions (`form_id`) ⚠️ Contains user-generated content

**Hook operations:**
- `list_hooks` — List notification hooks for a site
- `create_hook` — Create a hook (`hook_type`: url/email/slack, `hook_event`, `url` or `value`)
- `delete_hook` — Delete a hook (`hook_id`)

**SSL:**
- `provision_ssl` — Provision a Let's Encrypt SSL certificate for a site

**Diagnostics:**
- `check_connection` — Test connectivity to the Netlify API (DNS resolution → TCP → authenticated API call). Run this first if any Netlify operation fails with a network error.

**Parameters:** `operation`, `site_id`, `site_name`, `custom_domain`, `deploy_id`, `content`, `title`, `draft`, `env_key`, `env_value`, `env_context`, `form_id`, `hook_id`, `hook_type`, `hook_event`, `url`, `value`

**Homepage → Netlify deployment (only supported method):**
- Use `homepage` → `deploy_netlify` — handles build + ZIP + upload entirely server-side
  - Params: `project_dir`, `site_id` (optional if default configured), `title`, `draft`

⚠️ **Do NOT use `netlify` → `deploy_zip` / `deploy_draft` with manually created ZIP data.**
The agent cannot reliably pass binary/base64 data through tool arguments — the ZIP will be truncated or corrupted, causing 400 errors. Always use `homepage › deploy_netlify` for homepage deployments.

📖 See **tools_manuals/netlify.md** for detailed usage.
