# Skill Execution Tool (`execute_skill`)

Run a registered skill for external data retrieval, processing, or automation.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `skill` | Yes | Name of the skill to execute (from `list_skills`) |
| `skill_args` | No | Key-value arguments matching the skill's parameter schema |

## Discovery — MANDATORY First Step

Always call `list_skills` first to discover available skills and their parameters. Skills are added dynamically — the list changes over time.

```json
{"action": "list_skills"}
```

## Examples

```json
{"action": "execute_skill", "skill": "ddg_search", "skill_args": {"query": "golang best practices 2026", "max_results": 5}}
```

```json
{"action": "execute_skill", "skill": "web_scraper", "skill_args": {"url": "https://example.com"}}
```

```json
{"action": "execute_skill", "skill": "pdf_extractor", "skill_args": {"filepath": "docs/report.pdf"}}
```

Use `analyze_image` instead of `pdf_extractor` for PNG/JPG/WebP screenshots or photos.

## Notes
- Skills are Python scripts in `agent_workspace/skills/` with a `.json` manifest
- New skills can be created via `create_skill_from_template` and are immediately usable
- Skills run in a sandboxed Python environment with automatic venv activation and vault secret injection
- **Vault secrets**: If a skill needs secrets (API keys, tokens), the user must store them in the vault and assign them to the skill via the Web UI. Always inform the user about required secrets.
- Use `list_skills` (not `list_tools`) to discover available skills
- Native AuraGo tools are **not** skills. Call native tools directly with their own `action` instead of wrapping them in `execute_skill`.
- Example: use `{"action":"upnp_scan"}` directly, not `{"action":"execute_skill","skill":"upnp_scan"}`.
