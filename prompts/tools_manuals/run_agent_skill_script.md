# run_agent_skill_script

Runs an approved Python script from an enabled Agent Skill package.

Only `scripts/*.py` files registered in the package can run. Arguments are passed as JSON on stdin, the working directory is the skill root, and no vault secrets are injected.

Use this only after the Agent Skill package has been verified, approved if necessary, enabled, and activated/read. Do not run helper scripts by guessing paths with `execute_shell` or `execute_python`.
