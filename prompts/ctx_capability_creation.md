---
id: "capability_creation"
tags: ["core", "mandatory"]
priority: 11
conditions: []
---
# CREATING NEW CAPABILITIES

Choose the smallest durable capability:
- Reusable deterministic code, API clients, parsers, scrapers, file/data transforms, structured automation, Vault or Tool Bridge access -> check `list_skills` and `list_skill_templates`, then use `create_skill_from_template`.
- Reusable agent workflow, checklist, review/debug method, domain guidance, references, templates, or agentskills.io/Codex/Claude-style `SKILL.md` package -> use the Agent Skill Manager/API/UI path; discover with `list_agent_skills`, load with `activate_agent_skill`, run helper scripts with `run_agent_skill_script`.
- One-off script -> `execute_python`.
- Scheduled or triggered background work -> `manage_missions`.
- Long-running process -> `manage_daemon`.

Python skills: prefer an existing template; otherwise create `minimal_skill`, edit the generated agent-owned files deliberately, document the change, and verify with `execute_skill`. Do not save reusable Python manually through `execute_python`; it will not be registered or receive vault injection.

Agent Skills: package `skill-name/SKILL.md` plus optional `scripts/`, `references/`, `assets/`, and optional `agents/openai.yaml`. `SKILL.md` needs frontmatter `name` and `description`; keep the body short and put long details in `references/`. Create/import through the manager/API/UI, then verify, approve warnings if needed, enable, confirm with `list_agent_skills`, and activate before use. Do not write runtime Agent Skill folders by hand.

Tool Bridge: if a Python skill calls AuraGo native tools, declare `internal_tools` in the skill manifest and tell the user to enable `tools.python_tool_bridge.enabled`, whitelist `allowed_tools`, and approve the skill's internal tools in the Web UI. In code, use `AuraGoTools.is_available()`, catch `AuraGoToolError`, and call `tools.call("tool_name", {"param": "value"})`.

For details, use `discover_tools` -> `get_tool_info` for `skills_engine`, `skill_templates`, and `skill_manifest_spec`.
