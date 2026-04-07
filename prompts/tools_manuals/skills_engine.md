# Skills Engine (`skills_engine`)

Discover and execute admin-managed skill plugins from the `skills/` directory. Skills are pre-built Python utilities managed by the supervisor with automatic path resolution, venv activation, secret injection, and output scrubbing.

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
- **Skill templates**: Available templates include `api_client`, `file_processor`, `data_transformer`, `scraper`, `example_use_vault_login`, `example_use_vault_token`.
- **Supervisor features**: The skill supervisor handles venv activation, secret injection from vault, and output scrubbing automatically.
