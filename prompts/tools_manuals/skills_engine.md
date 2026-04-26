# Skills Engine (`skills_engine`)

Discover, create, and execute Python skills from the `skills/` directory. Skills are Python utilities with automatic path resolution, venv activation, vault secret injection, and output scrubbing. Skills can be pre-built, created by the agent from templates, or uploaded by the user.

> **IMPORTANT:** Never run skills via `execute_shell` or `execute_python` directly. Always use `execute_skill` — guessing filesystem paths will fail.

## Operations

| Operation | Description |
|-----------|-------------|
| `list_skills` | Discover available skills (MANDATORY first step) |
| `execute_skill` | Execute a discovered skill |
| `list_skill_templates` | List templates for creating new skills |
| `create_skill_from_template` | Create a new skill from a template |
| `get_skill_documentation` | Read the Markdown manual attached to a skill |
| `set_skill_documentation` | Write or replace a skill's Markdown manual |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `skill` | string | for execute_skill | Skill name from `list_skills` |
| `skill_args` | object | for execute_skill | Arguments matching the skill's parameter schema |
| `vault_keys` | array | optional for execute_skill | Additional vault keys to inject as `AURAGO_SECRET_<KEY>` for this run |
| `credential_ids` | array | optional for execute_skill | Credential UUIDs to inject as `AURAGO_CRED_<NAME>_<FIELD>` for this run |
| `template` | string | for create_skill_from_template | Template name (e.g., `minimal_skill`, `api_client`) |
| `name` | string | for create_skill_from_template | Name for the new skill |

## Examples

**Discover available skills (MANDATORY before writing custom code):**
```json
{"action": "list_skills"}
```

**Execute a skill:**
```json
{"action": "execute_skill", "skill": "pdf_reader", "skill_args": {"filepath": "doc.pdf"}}
```

**List skill templates:**
```json
{"action": "list_skill_templates"}
```

**Create a new skill from template:**
```json
{"action": "create_skill_from_template", "template": "api_client", "name": "my_api_client"}
```

**Create a skill WITH a manual (recommended):**
```json
{"action": "create_skill_from_template", "template": "api_client", "name": "weather_api",
 "documentation": "## Description\nFetches the current weather for a city.\n\n## Parameters\n- `city` (string, required)\n\n## Output\n`{ \"status\": \"ok\", \"result\": { \"temp_c\": 21.4 } }`\n\n## Example\n`{ \"city\": \"Berlin\" }`\n\n## Errors\n- `status: \"error\"` with `message` on network failure"}
```

**Read a skill's manual before reusing it:**
```json
{"action": "get_skill_documentation", "name": "weather_api"}
```

**Write/replace a skill's manual:**
```json
{"action": "set_skill_documentation", "name": "weather_api", "documentation": "## Description\n…"}
```

## Notes

- **MANDATORY first step**: Call `list_skills` before writing custom Python code for web search, web scraping, API interactions, file conversion (PDF/Office), or database access.
- **Use existing skills**: Using an existing skill is strictly preferred over writing custom tools. Only create a custom tool if `list_skills` returns no suitable capability.
- **Skill templates**: Use `list_skill_templates` to see all available templates. Common templates: `minimal_skill`, `api_client`, `data_transformer`, `notification_sender`, `monitor_check`, `log_analyzer`, `docker_manager`, `backup_runner`, `database_query`, `ssh_executor`, `mqtt_publisher`. Daemon templates: `daemon_monitor`, `daemon_watcher`, `daemon_listener`, `daemon_mission`.
- **Supervisor features**: The skill supervisor handles venv activation, secret injection from vault, and output scrubbing automatically.
- **Skills are immediately available**: After `create_skill_from_template`, the skill is ready to use via `execute_skill` without any restart.
- **Always document skills you create**: When you call `create_skill_from_template`, pass a `documentation` field — or call `set_skill_documentation` immediately afterwards. The Markdown manual is what lets future invocations (also after a context reset) reuse the skill correctly. Recommended sections: `## Description`, `## Parameters`, `## Output`, `## Example`, `## Errors`. Hard limit: 64 KB. Never paste secrets, API keys, or credentials into the manual.
- **Read manuals before calling unfamiliar skills**: Whenever `list_skills` shows a skill you didn't just create, call `get_skill_documentation` first if `has_documentation` is true. It is the canonical contract for parameters and output.
- **Skill manifest schema**: For manual manifest edits, read `skill_manifest_spec.md`. It documents `parameters`, `vault_keys`, `internal_tools`, daemon fields, and secret environment variable names.
- **Vault secrets require user approval**: If a skill needs vault secrets (API keys, tokens, passwords), the user must: (1) store the secret in the vault via Web UI → Settings → Secrets, and (2) assign the secret to the skill in the Skill Manager → select skill → Assign Secrets. Vault keys are injected as `AURAGO_SECRET_<KEY>` where the key is uppercased and non-alphanumeric characters become `_`. **Always tell the user which secrets they need to configure and where.** Without this step, the skill cannot access the required credentials.
- **Credentials**: `execute_skill` can pass `credential_ids` for credentials that have Python access enabled. Credential fields are injected as `AURAGO_CRED_<NAME>_<FIELD>`, for example `AURAGO_CRED_ROUTER_PASSWORD` or `AURAGO_CRED_API_KEY_TOKEN`. For manifest-level credential requests, use `vault_keys` entries in the form `cred:<credential-id>`.
- **Tool bridge**: Skills can call native AuraGo tools directly via the Python tool bridge. Use `from aurago_tools import AuraGoTools, AuraGoToolError` in the skill code and add required tool names to `internal_tools` in the manifest JSON. Check `AuraGoTools.is_available()` before constructing the client and catch `AuraGoToolError`. The bridge must be enabled in config (`python_tool_bridge.enabled: true`) with the tool names whitelisted in `python_tool_bridge.allowed_tools`. This allows efficient automation (e.g., a monitoring skill calling `proxmox` or `docker_management`) without LLM token cost.
- **Testing**: After creating or editing a normal skill, run `execute_skill` once with small safe `skill_args`. For daemon skills, run `manage_daemon` with `refresh`, then `start`, then `status`.
- **WRONG paths**: Never use `execute_python` + manual file save to create reusable capabilities — they bypass vault injection and aren't registered. Never create a `manage_missions` mission for reusable code. Always use `create_skill_from_template` for new reusable Python skills.
