# Skill Execution Tool (`execute_skill`)

Run a pre-built registered skill for external data retrieval and processing.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `skill` | Yes | Name of the skill to execute |
| `skill_args` | No | Key-value arguments for the skill |

## Available Skills

| Skill | Description | Key Arguments |
|-------|-------------|---------------|
| `ddg_search` | DuckDuckGo web search | `query`, `max_results` |
| `web_scraper` | Scrape webpage content | `url` |
| `pdf_extractor` | Extract text from PDF | `filepath` |
| `wikipedia_search` | Search Wikipedia | `query`, `lang` |
| `virustotal_scan` | Scan URL, domain, IP, file hash, or local file | `resource` or `file_path`, optional `mode` |

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
- Skills are Python scripts in `agent_workspace/skills/`
- New skills can be registered dynamically
- Skills run in the same sandboxed Python environment as `execute_python`
- `list_tools` does not list these pre-built skills; use `list_skills` for discovery
- Native AuraGo tools are **not** skills. Call native tools directly with their own `action` instead of wrapping them in `execute_skill`.
- Example: use `{"action":"upnp_scan"}` directly, not `{"action":"execute_skill","skill":"upnp_scan"}`.
