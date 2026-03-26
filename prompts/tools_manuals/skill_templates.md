## Tool: Skill Templates

Create new skills from built-in templates instead of writing Python code from scratch.

### List Templates (`list_skill_templates`)

Returns all available templates with their names, descriptions, and expected parameters.

```json
{"action": "list_skill_templates"}
```

### Create Skill from Template (`create_skill_from_template`)

Generate a complete skill (manifest + Python code) from a template. The skill is immediately usable via `execute_skill`.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `template` | string | yes | Template name: `api_client`, `file_processor`, `data_transformer`, `scraper` |
| `name` | string | yes | Unique name for the new skill (e.g. `weather_api`) |
| `description` | string | no | What this skill does |
| `url` | string | no | Base URL for API (api_client template only) |
| `dependencies` | array | no | Additional pip packages beyond template defaults |
| `vault_keys` | array | no | Vault secret keys the skill needs at runtime |

#### Templates

**api_client** — REST API client with auth header and vault key injection.
- Default deps: `requests`
- Vault: `API_KEY` (injected as `AURAGO_SECRET_API_KEY`), `BASE_URL` (optional override)
- Params: `endpoint`, `method`, `body`

**file_processor** — Read, transform, and write files with regex operations.
- No default deps (stdlib only)
- Params: `input_path`, `output_path`, `operation` (extract_lines/search/replace/head/tail/count), `pattern`, `replacement`

**data_transformer** — Convert between JSON, CSV, and YAML formats.
- Default deps: `pyyaml`
- Params: `input_path`, `output_path`, `input_format`, `output_format`, `fields`

**scraper** — Web scraper with CSS selectors. Output wrapped in `<external_data>` tags.
- Default deps: `requests`, `beautifulsoup4`
- Params: `url`, `selector`, `attr`, `limit`

#### Example

```json
{
  "action": "create_skill_from_template",
  "template": "api_client",
  "name": "weather_api",
  "description": "Fetch weather data from OpenWeatherMap",
  "url": "https://api.openweathermap.org/data/2.5",
  "vault_keys": ["API_KEY"]
}
```

After creation, use:
```json
{"action": "execute_skill", "skill": "weather_api", "skill_args": {"endpoint": "weather?q=Berlin", "method": "GET"}}
```
