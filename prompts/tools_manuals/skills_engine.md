## Tool: Skills Engine

Discover and execute admin-managed skill plugins from the `skills/` directory.

### Discover Skills (`list_skills`) — MANDATORY FIRST STEP

You MUST call this before writing custom Python code for: web search, web scraping, API interactions, file conversion (PDF/Office), or database access.

Using an existing skill is strictly preferred over writing custom tools. Only create a custom tool if `list_skills` returns no suitable capability.

Returns an array of plugins, each with `name`, `description`, `parameters` schema, and `returns` field.

```json
{"action": "list_skills"}
```

### Execute Skill (`execute_skill`)

Run a skill discovered via `list_skills`. Map arguments exactly to the skill's `parameters` schema inside `skill_args`.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `skill` | string | yes | Skill name from `list_skills` |
| `skill_args` | object | yes | Arguments matching the skill's parameter schema |

```json
{"action": "execute_skill", "skill": "pdf_reader", "skill_args": {"filepath": "doc.pdf"}}
```

### Create New Skills from Templates

If no existing skill fits your need, use `list_skill_templates` and `create_skill_from_template` to scaffold a new skill from a built-in template (api_client, file_processor, data_transformer, scraper). See the **Skill Templates** tool manual for details.