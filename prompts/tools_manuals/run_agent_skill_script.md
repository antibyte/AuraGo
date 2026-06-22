# run_agent_skill_script

Runs an approved helper script from an enabled Agent Skill package.

Only registered files under `scripts/` can run. Supported extensions are `.py`, `.sh`, and `.js`; Bash and JavaScript require `tools.skill_manager.allowed_script_languages` to include `bash` or `javascript`, and Bash also requires `agent.allow_shell: true`.

Use this only after the Agent Skill package has been verified, approved if necessary, enabled, and activated/read. Do not run helper scripts by guessing paths with `execute_shell` or `execute_python`.
