# Skill Execution Tool (`execute_skill`)

Run a registered skill for external data retrieval, processing, or automation.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `skill` | Yes | Name of the skill to execute (from `list_skills`) |
| `skill_args` | No | Key-value arguments matching the skill's parameter schema |
| `vault_keys` | No | Extra vault secret keys to inject for this run as `AURAGO_SECRET_<KEY>` |
| `credential_ids` | No | Credential UUIDs to inject for this run as `AURAGO_CRED_<NAME>_<FIELD>` |

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
- **Vault secrets**: If a skill needs secrets (API keys, tokens), the user must store them in the vault and assign them to the skill via the Web UI. Secrets are injected as `AURAGO_SECRET_<KEY>` where the key is uppercased and non-alphanumeric characters become `_`. Always inform the user about required secrets.
- **Credentials**: Use `credential_ids` when a run needs stored credentials instead of simple vault keys. Credential records must allow Python access. Fields are injected as `AURAGO_CRED_<NAME>_<FIELD>`, for example `AURAGO_CRED_ROUTER_USERNAME`, `AURAGO_CRED_ROUTER_PASSWORD`, or `AURAGO_CRED_API_KEY_TOKEN`.
- **Tool bridge**: Skills can call native AuraGo tools (e.g., `proxmox`, `docker_management`) directly via the Python tool bridge if enabled in config (`python_tool_bridge.enabled: true`). Add required tool names to the skill manifest's `internal_tools` field and ensure they are listed in `python_tool_bridge.allowed_tools`. Use `from aurago_tools import AuraGoTools, AuraGoToolError`, check `AuraGoTools.is_available()`, and catch `AuraGoToolError`. Call tools as `tools.call("tool_name", {"param1": "val"})`.
- Use `list_skills` (not `list_tools`) to discover available skills
- Native AuraGo tools are **not** skills. If `discover_tools` reports `kind: native`, do not wrap it in `execute_skill`.
- If the native tool is `active`, call it directly with its own `action`. If it is `hidden`, use `invoke_tool` exactly as instructed by `discover_tools`; the real native schema will be re-injected for follow-up calls.
- Example: use `{"action":"upnp_scan"}` directly, or `{"action":"invoke_tool","tool_name":"upnp_scan","arguments":{...}}` only when `discover_tools` returned `call_method: "invoke_tool"`. Do not use `{"action":"execute_skill","skill":"upnp_scan"}`.
