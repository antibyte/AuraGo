# ansible — Ansible Automation

Run Ansible playbooks, ad-hoc commands, and inventory queries.
Two execution modes are supported — choose with `ansible.mode` in `config.yaml`:

| Mode | How it works | When to use |
|---|---|---|
| `sidecar` (default) | Calls the Ansible HTTP sidecar API (`ansible.url`) | Docker / containerised deployments |
| `local` | Runs `ansible` / `ansible-playbook` binaries directly on the host | Bare-metal / VM installs where Ansible is already installed |

> **Sidecar mode** requires the `ansible` container in `docker-compose.yml` plus matching `ansible.url` and `ansible.token`.
> **Local mode** requires `ansible` to be on the system `PATH` and `ansible.playbooks_dir` / `ansible.default_inventory` to be set.

---

## Operations

### status
Check that the sidecar is reachable and return the installed Ansible version.

```json
{"action": "ansible", "operation": "status"}
```

---

### list_playbooks
List all `.yml`/`.yaml` playbook files available in the sidecar's `/playbooks` directory.

```json
{"action": "ansible", "operation": "list_playbooks"}
```

---

### inventory
Parse the inventory and return all defined hosts and groups.

```json
{"action": "ansible", "operation": "inventory"}
{"action": "ansible", "operation": "inventory", "inventory": "/inventory/production"}
```

---

### ping
Test SSH connectivity to hosts using the `ansible -m ping` module.

```json
{"action": "ansible", "operation": "ping"}
{"action": "ansible", "operation": "ping", "hostname": "webservers"}
{"action": "ansible", "operation": "ping", "hostname": "192.168.1.10"}
```

---

### adhoc
Run any Ansible ad-hoc module against target hosts.

```json
{"action": "ansible", "operation": "adhoc", "hostname": "all", "module": "ping"}
{"action": "ansible", "operation": "adhoc", "hostname": "webservers", "module": "shell", "command": "uptime"}
{"action": "ansible", "operation": "adhoc", "hostname": "db01", "module": "service", "command": "name=postgresql state=restarted"}
{"action": "ansible", "operation": "adhoc", "hostname": "all", "module": "apt", "command": "name=nginx state=latest update_cache=yes"}
```

**Common modules:**
| Module | Use case |
|---|---|
| `ping` | Test connectivity |
| `shell` / `command` | Run a shell command (`command` args: `cmd=...`) |
| `copy` | Copy a file |
| `service` | Manage services (`name=nginx state=started`) |
| `apt` / `yum` | Package management |
| `user` | Manage users |
| `file` | Manage files/directories |
| `uri` | Make HTTP requests |

---

### playbook
Run an Ansible playbook.

```json
{"action": "ansible", "operation": "playbook", "name": "site.yml"}
{"action": "ansible", "operation": "playbook", "name": "deploy.yml", "host_limit": "webservers"}
{"action": "ansible", "operation": "playbook", "name": "deploy.yml", "tags": "nginx,ssl"}
{"action": "ansible", "operation": "playbook", "name": "deploy.yml", "body": "{\"env\": \"prod\", \"version\": \"1.2.3\"}"}
{"action": "ansible", "operation": "playbook", "name": "update.yml", "host_limit": "db01", "skip_tags": "reboot"}
```

---

### check
Dry-run a playbook (`--check --diff`) — shows what **would** change without applying anything.

```json
{"action": "ansible", "operation": "check", "name": "site.yml"}
{"action": "ansible", "operation": "check", "name": "deploy.yml", "host_limit": "staging"}
```

Alternatively, use `playbook` with `"preview": true`:
```json
{"action": "ansible", "operation": "playbook", "name": "site.yml", "preview": true}
```

---

### facts
Gather system facts from hosts using the `setup` module. Output is trimmed to 8 KB.

```json
{"action": "ansible", "operation": "facts", "hostname": "db01"}
{"action": "ansible", "operation": "facts", "hostname": "all"}
```

---

## Parameter Reference

| Parameter | Operations | Description |
|---|---|---|
| `operation` | All | Operation name (required) |
| `name` | playbook, check | Playbook filename relative to `/playbooks` on the sidecar (e.g. `site.yml`) |
| `hostname` | ping, adhoc, facts | Target host pattern: `all`, group name, hostname, or IP |
| `module` | adhoc | Ansible module name (e.g. `ping`, `shell`, `apt`, `service`) |
| `command` | adhoc | Module arguments string (e.g. `"name=nginx state=started"`) |
| `host_limit` | playbook, check | `--limit` host subset (e.g. `webservers`, `192.168.1.10`) |
| `tags` | playbook, check | `--tags` comma-separated (e.g. `"deploy,restart"`) |
| `skip_tags` | playbook, check | `--skip-tags` comma-separated |
| `inventory` | All | Override the default inventory path on the sidecar |
| `body` | playbook, check, adhoc | Extra variables as JSON string (e.g. `"{\"env\":\"prod\"}"`) |
| `preview` | playbook | When `true`, appends `--check` flag (dry-run mode) |

---

## Configuration

### Sidecar Mode (Docker)

The Ansible sidecar is defined in `docker-compose.yml` as a commented-out block.
Uncomment the `ansible:` service block, then mount your playbooks and inventory:

- `./ansible/playbooks` → `/playbooks`
- `./ansible/inventory` → `/inventory`
- `~/.ssh` → `/home/ansibleuser/.ssh:ro` (container runs as `ansibleuser`, not `root`)

```yaml
ansible:
  enabled: true
  mode: sidecar          # default
  url: "http://ansible:5001"
  token: "your_secret_token_here"
  timeout: 300
```

### Local Mode (no Docker required)

Requires `ansible` and `ansible-playbook` to be installed and on `PATH`.

```yaml
ansible:
  enabled: true
  mode: local
  playbooks_dir: "/opt/ansible/playbooks"   # or ANSIBLE_PLAYBOOKS_DIR env var
  default_inventory: "/etc/ansible/hosts"   # or ANSIBLE_INVENTORY env var
  timeout: 300
```

---

## Notes

- In **local mode**, `inventory` parameter overrides `default_inventory` per request.
- In **sidecar mode**, `inventory` parameter overrides the sidecar's `DEFAULT_INVENTORY` per request.
- Playbook run output (stdout/stderr) is returned as-is from the ansible CLI.
- Facts output is trimmed to 8 KB to avoid overwhelming the context window.
- Playbook paths are validated to prevent directory traversal (sidecar mode).
- Playbooks are listed recursively in both local and sidecar modes.
- `ansible.timeout` applies to both modes as the maximum seconds per command.
