---
id: "tools_ansible"
tags: ["conditional"]
priority: 33
conditions: ["ansible_enabled"]
---
### Ansible Automation
| Tool | Purpose |
|---|---|
| `ansible` | Run Ansible automation: execute playbooks, ad-hoc modules, ping hosts, gather facts — via sidecar container **or** local CLI |

**Operations:**
- `status` — Check Ansible version and connectivity
- `list_playbooks` — List available playbook files
- `inventory` — Parse and return the host inventory
- `ping` — Test connectivity to hosts (`ansible -m ping`)
- `adhoc` — Run any ad-hoc module (requires `module`, optionally `command` for args)
- `playbook` — Run a playbook (requires `name`; optionally `host_limit`, `tags`, `skip_tags`, `body` for extra_vars)
- `check` — Dry-run a playbook without making changes (`--check --diff`)
- `facts` — Gather system facts from host(s) via the setup module

**Key parameters:** `operation`, `name` (playbook), `hostname` (host pattern), `module`, `command` (module args), `host_limit`, `tags`, `skip_tags`, `inventory` (override), `body` (extra_vars JSON), `preview` (--check mode)

**Modes** (configured via `ansible.mode`):
- `sidecar` (default) — calls the Ansible Docker/sidecar API at `ansible.url`
- `local` — calls `ansible` / `ansible-playbook` binaries directly on the host
