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

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `skill` | string | for execute_skill | Skill name from `list_skills` |
| `skill_args` | object | for execute_skill | Arguments matching the skill's parameter schema |
| `template` | string | for create_skill_from_template | Template name (e.g., `api_client`, `file_processor`) |
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

## Notes

- **MANDATORY first step**: Call `list_skills` before writing custom Python code for web search, web scraping, API interactions, file conversion (PDF/Office), or database access.
- **Use existing skills**: Using an existing skill is strictly preferred over writing custom tools. Only create a custom tool if `list_skills` returns no suitable capability.
- **Skill templates**: Use `list_skill_templates` to see all available templates. Common templates: `api_client`, `data_transformer`, `notification_sender`, `monitor_check`, `log_analyzer`, `docker_manager`, `backup_runner`, `database_query`, `ssh_executor`, `mqtt_publisher`. Daemon templates: `daemon_monitor`, `daemon_watcher`, `daemon_listener`, `daemon_mission`.
- **Supervisor features**: The skill supervisor handles venv activation, secret injection from vault, and output scrubbing automatically.
- **Skills are immediately available**: After `create_skill_from_template`, the skill is ready to use via `execute_skill` without any restart.
- **Vault secrets require user approval**: If a skill needs vault secrets (API keys, tokens, passwords), the user must: (1) store the secret in the vault via Web UI → Settings → Secrets, and (2) assign the secret to the skill in the Skill Manager → select skill → Assign Secrets. **Always tell the user which secrets they need to configure and where.** Without this step, the skill cannot access the required credentials.
- **Tool bridge**: Skills can call native AuraGo tools directly via the Python tool bridge. Use `from aurago_tools import AuraGoTools` in the skill code and add required tool names to `internal_tools` in the manifest JSON. The bridge must be enabled in config (`python_tool_bridge.enabled: true`) with the tool names whitelisted in `python_tool_bridge.allowed_tools`. This allows efficient automation (e.g., a monitoring skill calling `proxmox` or `docker_management`) without LLM token cost.
- **WRONG paths**: Never use `execute_python` + manual file save to create reusable capabilities — they bypass vault injection and aren't registered. Never create a `manage_missions` mission for reusable code. Always use `create_skill_from_template` for new reusable Python skills.
