---
id: ansible
title: Ansible Workflow
enabled: true
priority: 91
tools: [ansible]
workflows: [ansible, playbook, infrastructure, configuration_management, automation]
keywords:
  - ansible
  - playbook
  - playbooks
  - adhoc
  - ad-hoc
  - inventory
  - configuration management
  - infrastructure as code
  - iac
  - automation
  - deploy
  - provisioning
---

This rule applies whenever executing, writing, debugging, or managing Ansible playbooks, ad-hoc commands, inventories, or roles.

## Ansible Workflow

Treat Ansible as an infrastructure-as-code tool, not a remote shell replacement. Every playbook run and ad-hoc command must be idempotent, observable, and safe.

### Pre-Execution Checklist

Before running any mutating playbook or ad-hoc command:

1. **Verify connectivity.** Run `ping` against the target host pattern first.
2. **Inspect inventory.** Call `inventory` to confirm the target hosts are expected.
3. **Gather facts.** For unfamiliar hosts, run `facts` to understand the environment before making changes.
4. **Dry-run first.** Use `check` (or `preview: true` for playbooks) for the initial run against new or critical hosts. Review the diff before applying.

### Playbook Quality

- **Idempotence is mandatory.** Every playbook must be safe to run multiple times without unintended side effects. Use `changed_when` and `failed_when` conditions where appropriate.
- **Tag granularity.** Structure playbooks with meaningful tags so selective execution (`tags` / `skip_tags`) is possible.
- **Variable separation.** Keep secrets in Ansible Vault or the AuraGo secrets vault. Never hardcode passwords, tokens, or API keys in playbook YAML.
- **Role structure.** For reusable automation, prefer Ansible roles with a clear directory structure (`tasks/`, `handlers/`, `templates/`, `vars/`, `defaults/`).

### Ad-Hoc Command Discipline

- Use ad-hoc commands (`adhoc`) only for one-off queries or emergency fixes, not for ongoing configuration.
- Prefer modules over raw shell commands. Use `shell` or `command` modules only when no native Ansible module exists.
- Always quote module arguments correctly (e.g., `"cmd='uptime'"` or `"name=nginx state=started"`).

### Execution Safety

- **Limit blast radius.** Use `host_limit` to restrict playbook execution to specific hosts or groups when the full inventory is not the intended target.
- **Sidecar awareness.** When using the Ansible sidecar (default), ensure the sidecar is running (`status`) before executing playbooks. The sidecar uses a dedicated container with mounted SSH keys and playbook directories.
- **Local mode.** When `ansible.mode` is set to `local`, commands run directly on the host. Ensure the `ansible` and `ansible-playbook` binaries are available and the configured `playbooks_dir` exists.

### Error Handling and Diagnostics

- If a playbook fails, inspect the stderr output and task failure details before retrying.
- Do not retry the exact same playbook with the same variables after a failure without understanding the root cause.
- Use `--diff` (`diff: true`) for configuration-file tasks to see exactly what changed.

### Documentation and Reuse

- After creating or refining a reusable playbook, consider saving it as a cheatsheet or registering it in the skill manifest if it represents a recurring workflow.
- Document host-specific variables and inventory group layouts in the knowledge graph for long-term infrastructure tracking.
