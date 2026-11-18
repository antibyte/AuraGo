# ansible — Ansible Automation

Local and sidecar inventories share the static-file restrictions below. Only the exact configured default may be outside the root. Inventory output excludes arbitrary variables and raw errors. Child processes never inherit the API token. Rebuild existing sidecars to advertise security policy 4 and use the matching `io.aurago.ansible.security-policy=4` label for managed images. Read-only blocks all playbook runs, including check and dry-run. Use the dedicated `ping` and `facts` operations. Module arguments accepted by read-only execution must contain literal values. Supply JSON for values that need backslash escapes.

Both modes require literal UTF-8 inventories: no Jinja templates/lookups, including escaped forms. Host/group patterns must resolve entirely to explicitly declared inventory hosts; implicit localhost and host-file limits (`@file`) are rejected. With no configured default, AuraGo supplies its own explicit private localhost inventory. Extra variables must be JSON objects containing application data, without templates or Ansible connection/engine overrides (`ansible_*`, `_ansible_*`, `hostvars`, `groups` and other reserved inventory variables). Malformed JSON is an error, not an empty variable set. These checks constrain supplied inputs; approved playbooks remain executable operator code and require write permission.

Run Ansible playbooks, ad-hoc commands, and inventory queries.
A2A peers can only inspect status, playbook lists and inventories.
Two execution modes are supported — choose with `ansible.mode` in `config.yaml`:

| Mode | How it works | When to use |
|---|---|---|
| `sidecar` (default) | Calls the Ansible HTTP sidecar API (`ansible.url`) | Docker / containerised deployments |
| `local` | Runs `ansible` / `ansible-playbook` binaries directly on the host | Bare-metal / VM installs where Ansible is already installed |

> **Sidecar mode** requires the `ansible` container in `docker-compose.yml` plus matching `ansible.url` and `ansible.token`.
> **Local mode** requires `ansible` to be on the system `PATH` and `ansible.playbooks_dir` / `ansible.default_inventory` to be set.

Local mode requires the effective shell permission for every process, including status, inventory, ping, facts and check runs. It uses the configured shell sandbox; an unavailable required sandbox blocks execution, and unsandboxed execution additionally requires `agent.allow_unsafe_host_execution`. Inputs are private snapshots of the configured playbook root (up to 512 files / 32 MiB). Absolute paths cannot escape that root. Inventories must be static INI or YAML (up to 1 MiB); executable inventories and dynamic inventory plugins are rejected. An administrator-configured default inventory may live outside the playbook root. SSH host-key checking is enabled.

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
Request Ansible check mode. Tasks may still mutate state; the same write grant as playbook execution is required.

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
- `ansible.timeout` covers inventory export, host selection and execution together.
- Auto-build refreshes missing or obsolete managed images before container replacement. Failed builds retain the previous container; rebuild external images explicitly before using policy 3.
