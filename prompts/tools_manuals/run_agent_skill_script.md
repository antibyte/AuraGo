# run_agent_skill_script

Runs an approved Python script from an enabled Agent Skill package.

Only `scripts/*.py` files registered in the package can run. Arguments are passed as JSON on stdin, the working directory is the skill root, and no vault secrets are injected.
