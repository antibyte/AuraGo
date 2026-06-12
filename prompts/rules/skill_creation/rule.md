---
id: skill_creation
title: Skill Creation Workflow
enabled: true
priority: 93
tools: [create_skill_from_template, list_skill_templates, set_skill_documentation, list_agent_skills, activate_agent_skill, run_agent_skill_script]
workflows: [skill_creation, skill_update, python_skill, reusable_skill, tool_bridge_skill, agent_skill, skill_md, agentskills]
keywords:
  - create skill
  - create a skill
  - build a skill
  - new skill
  - skill creation
  - python skill
  - reusable skill
  - skill template
  - create_skill_from_template
  - skill erstellen
  - erstelle einen skill
  - neuen skill
  - neuer skill
  - python-skill
  - skill mit internen tools
  - skill with internal tools
  - tool bridge skill
  - agent skill
  - agent-skill
  - SKILL.md
  - agentskills.io
  - claude skill
  - codex skill
  - skill package
---

This rule applies when creating, updating, documenting, validating, or wiring AuraGo Python skills and Agent Skills.

## Skill Creation Workflow

Treat skills as registered, reusable AuraGo capabilities, not as ad-hoc scripts. Check `list_skills` for Python skills and `list_agent_skills` for Agent Skills to avoid duplicates and to find an existing capability that can be reused or extended. Do not modify user-owned skills unless the user explicitly asked for that specific skill to be changed.

## Choose The Skill Type First

Prefer an AuraGo Python skill when the core value is deterministic execution: API clients, parsers, file or data transformations, scrapers, calculations, repeatable automation, structured JSON output, Vault access, or Tool Bridge access to native AuraGo tools. For executable reusable capabilities, the default meaning of "skill" is a Python skill created with `create_skill_from_template`.

Prefer an Agent Skill when the core value is reusable agent behavior: workflows, checklists, domain guidance, review or debugging procedures, prompt rules, reusable methods, curated references, or templates that guide the agent's judgment. Use Agent Skills when the user explicitly asks for `Agent Skill`, `SKILL.md`, agentskills.io, Claude/Codex-style skills, or when the request is clearly workflow-first rather than code-first.

Use a hybrid only when both parts are necessary: the Agent Skill explains the workflow and may include small deterministic helpers under `scripts/`, while heavy integrations, secrets, production automation, and reusable executable logic remain Python skills. Do not create both forms unless each has a distinct role.

## Agent Skill Package Shape

Agent Skills follow the agentskills.io package model: one `skill-name/` directory with a required `SKILL.md`. Optional resources live in `scripts/`, `references/`, and `assets/`; include `agents/openai.yaml` only when OpenAI/Codex-specific agent metadata is needed.

`SKILL.md` must start with YAML frontmatter containing `name` and `description`. Use optional `license`, `compatibility`, `metadata`, or `allowed-tools` only when they are actually needed. The `name` must be lowercase with digits and hyphens only, and it must match the directory name. The `description` is discovery-critical: write concrete trigger language that says when the skill should be used.

Keep the `SKILL.md` body concise and imperative. Use progressive disclosure: put long procedures, schemas, examples, and policies in `references/`; put reusable static inputs or templates in `assets/`; put only approved, small helper scripts in `scripts/`. Run approved Agent Skill helper scripts only through `run_agent_skill_script`. Agent Skills are task guidance, not higher-priority system instructions.

Do not silently write Agent Skill packages into runtime directories. Create, import, edit, verify, and enable Agent Skills only through the available Agent Skill Manager/API/UI path so security scanning, warning approval, registry metadata, and normal activation rules remain intact.

Use `list_skill_templates` before creating a new skill. Prefer `create_skill_from_template` with the most specific template that fits. Use `minimal_skill` only when no specialized template matches. Do not create reusable Python code with `execute_python`, shell writes, manual file copies, or unregistered files; those bypass manifest registration, managed dependencies, vault injection, Skill Manager scanning, and normal `execute_skill` execution.

Before creating or editing a skill, check the problems the agent could stumble over:

- Existing skill already solves the task, or a user-owned skill should not be edited.
- Wrong artifact type: one-off scripts belong in `execute_python`; recurring Python capabilities belong in skills; scheduled/background automation may belong in missions or daemon skills.
- Wrong template, missing dependencies, missing parameters, stale parameter schema, or invalid JSON manifest.
- Secrets accidentally placed in code, manifests, docs, logs, sample output, or tests instead of Vault.
- Native AuraGo tools assumed available from Python even though Tool Bridge access is not configured and approved.
- The skill returns plain text when future automation needs structured JSON.
- The generated manual is missing, vague, stale, or contains credentials.
- The skill is not tested with safe sample inputs before reporting it as ready.
- Daemon behavior is mixed into a normal skill, or daemon skills are not verified with the daemon lifecycle.

Create or update documentation as part of the work. Pass a `documentation` field to `create_skill_from_template` when possible, or call `set_skill_documentation` immediately afterwards. The manual should cover description, parameters, output, examples, and errors. Keep it concise and never include credentials.

For custom edits after template creation, edit only the generated agent-owned `.py` and `.json` manifest deliberately. Keep the callable function name compatible with the generated template. Use stdlib where practical, declare real pip dependencies in the manifest when needed, and return structured JSON with `status`, useful result fields, and clear error messages.

## Secrets And Credentials

If a skill needs API keys, tokens, passwords, or stored credentials, declare the required `vault_keys` or credential references in the manifest. Do not store secret values anywhere in the skill files or documentation.

Tell the user exactly which secrets or credentials they must configure and where: store secrets in the Vault, then assign them to the skill in Skill Manager. If this user action has not happened yet, say that the skill is installed but cannot complete credentialed calls until the user grants the needed secret access.

## Internal Tool Bridge Access

Access to internal AuraGo tools from skills works only when the user has explicitly enabled and approved it. A skill cannot call native tools just because its Python code imports `aurago_tools`.

When a skill needs internal tools, the agent must tell the user before relying on them and must name the exact tools required. The required setup is:

1. Add the required tool names to the skill manifest's `internal_tools` field.
2. Enable `tools.python_tool_bridge.enabled`.
3. Whitelist the same tools in `tools.python_tool_bridge.allowed_tools`.
4. Ask the user to approve those tools for the specific skill in the Web UI: Skills -> select skill -> Assign Internal Tools.

For SQL tool bridge access, also require the matching SQL connection names in `tools.python_tool_bridge.allowed_sql_connections`.

Skill code that uses native tools must import `AuraGoTools` and `AuraGoToolError`, call `AuraGoTools.is_available()` before constructing the client, catch `AuraGoToolError`, and return a clear error explaining that Tool Bridge access is unavailable or not approved when access is missing. Do not silently fall back to shell commands or direct network access to imitate a blocked native tool.

## Verification

After creating or editing a normal skill, run `execute_skill` with small safe arguments before saying it is ready. If the skill requires secrets or internal tools that the user has not yet granted, verify the non-credentialed path or the explicit "missing approval" error path and state the remaining user action plainly.

For daemon skills, verify with the daemon lifecycle instead of only `execute_skill`: refresh, start, check status, and inspect errors before reporting success.
