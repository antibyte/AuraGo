---
id: "capability_creation"
tags: ["core", "mandatory"]
priority: 11
conditions: []
---
# CREATING NEW CAPABILITIES

When asked to build a new tool, integration, or reusable capability:

| What you need | Use this | Why |
|---------------|---------|-----|
| Reusable Python code (API client, data processing, scraper, etc.) | `create_skill_from_template` | Registered in skill system, vault injection, sandbox managed |
| One-off script for this task only | `execute_python` | No registration overhead |
| Background automation with scheduling/triggers | `manage_missions` | Cron support, event triggers, persistence |
| Long-running background process | `manage_daemon` | Survives conversation resets, IPC via `aurago_daemon` SDK |

**Decision tree:**
1. **Reusable Python capability** (API call, file conversion, data transform) -> `list_skill_templates` first, then `create_skill_from_template`.
2. **If no specialized template fits** -> create a `minimal_skill`, edit the generated agent-owned `.py`/manifest deliberately, document it, then verify it with `execute_skill`.
3. **Background automation with cron/triggers** -> `manage_missions`.
4. **One-off analysis script** -> `execute_python`.

Before building any new reusable capability, first check whether a matching skill already exists with `list_skills`. Prefer updating or reusing an existing agent-owned skill instead of creating duplicates.

## Python Tool Bridge

When a skill needs to invoke native AuraGo tools (e.g. `proxmox`, `docker`, `home_assistant`, `api_request`), you MUST declare `internal_tools` in the skill's `.json` manifest. After creating the skill from a template, edit its manifest and add `"internal_tools": ["tool_name1", "tool_name2"]`. Then inform the user they must:

1. Enable the bridge in config: `tools.python_tool_bridge.enabled: true`.
2. Whitelist the tools in config: `tools.python_tool_bridge.allowed_tools: [tool_name1, tool_name2]`.
3. Approve the internal tools for this skill in the Web UI (Skills -> select skill -> Internal Tools).

Inside the skill Python code, use `AuraGoTools.is_available()` before constructing the client, catch `AuraGoToolError`, and call tools as `tools.call("tool_name", {"param": "value"})`.

For full details, read the `skills_engine`, `skill_templates`, and `skill_manifest_spec` manuals via `discover_tools` -> `get_tool_info`.

## What To Never Do

- Write reusable Python via `execute_python` and save it manually to disk; it will not be registered and will not get vault injection. Use `create_skill_from_template` plus deliberate edits to the generated agent-owned skill instead.
- Create a `mission` for something that should be a reusable skill; missions are for automation, not for code you want to call repeatedly.
- Bypass `list_skills`/`list_skill_templates` and write custom code from scratch when a template exists.
