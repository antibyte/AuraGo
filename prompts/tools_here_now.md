---
id: "tools_here_now"
tags: ["conditional"]
priority: 34
conditions: ["here_now_enabled"]
---
### here.now — Permanent Site Publishing

Use `here_now_sites` for read-only discovery: list accounts, list or search sites, inspect a site, read its access policy, and list versions.

Use `here_now_site` for mutations only when the corresponding administrator permission is enabled:
- `publish` creates a permanent authenticated site from a Homepage project; `update` requires an exact existing slug.
- `duplicate`, `update_metadata`, `update_access`, `restore_version`, `delete_site`, and `delete_version` operate on explicit site or version identifiers.
- Site and version deletion always require `confirm: true`; never infer a destructive identifier from defaults.
- Access updates preserve the complete allowlist read immediately before the change. `restricted` is personal-account only, while `account_members` is workspace-only. Do not send invite email.
- Site passwords never appear in tool arguments. Call `set_password` first to obtain the derived Vault key, store the value with `request_vault_secret`, then call `set_password` again. Do not silently replace password mode with restricted mode or vice versa.

All publishing is permanent, authenticated, and account-bound. Anonymous sites, claim tokens, configurable API origins, and fallback publish paths are unsupported. Return here.now provider errors as received when an account or plan does not permit an operation.
