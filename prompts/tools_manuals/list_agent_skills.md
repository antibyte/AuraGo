# list_agent_skills

Lists enabled Agent Skills packages that have an acceptable security status.

Use this when you need to discover package-first `SKILL.md` skills. The output contains only catalog metadata. Load full instructions with `activate_agent_skill`.

## Agent Skill Manager workflow

Agent Skills are managed packages, not raw files to drop into the runtime directory. To create or change one, use the Agent Skill Manager/API/UI path when it is available:

1. Create a simple skill with `POST /api/agent-skills` using `name`, `description`, and `body` or `skill_md`, or import a package with `POST /api/agent-skills/import`.
2. Add or update resources with `POST /api/agent-skills/{id}/files` or `PUT /api/agent-skills/{id}/files`.
3. Verify after every create, import, or edit with `POST /api/agent-skills/{id}/verify`.
4. If the scan reports a warning, wait for explicit admin approval via `POST /api/agent-skills/{id}/approve-warning`.
5. Enable only after the package is clean or warning-approved, then confirm discovery with `list_agent_skills` and load instructions with `activate_agent_skill`.

If you cannot access a safe Manager/API/UI path, do not use filesystem or shell writes into `agent_workspace/agent_skills`. Provide the complete `SKILL.md` and resource contents to the user and say it still needs to be imported and enabled.
