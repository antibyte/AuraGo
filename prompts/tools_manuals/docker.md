# Docker Management Tool

Manage generic Docker containers, images, networks, and volumes directly through the Docker Engine API. AuraGo's managed homepage containers and the `aurago-homepage` image repository are reserved; use `homepage_project`, `homepage_file`, or `homepage_deploy` for homepage work.

## Prerequisites
- Docker must be running on the host
- `docker.enabled: true` in config.yaml
- Optional: `docker.host` — auto-detected if empty (Unix socket on Linux/Mac, TCP on Windows)

## Operations

### Container Operations

#### list_containers — List running (or all) containers
```json
{"action": "docker", "operation": "list_containers"}
{"action": "docker", "operation": "list_containers", "all": true}
```

#### inspect — Get detailed container info
```json
{"action": "docker", "operation": "inspect", "container_id": "my_container"}
```

#### start / stop / restart / pause / unpause
```json
{"action": "docker", "operation": "start", "container_id": "my_container"}
{"action": "docker", "operation": "stop", "container_id": "my_container"}
{"action": "docker", "operation": "restart", "container_id": "nginx_proxy"}
```

#### remove — Delete a container
```json
{"action": "docker", "operation": "remove", "container_id": "old_container"}
{"action": "docker", "operation": "remove", "container_id": "stuck_container", "force": true}
```

#### logs — Get container logs (last N lines)
```json
{"action": "docker", "operation": "logs", "container_id": "my_app", "tail": 50}
```

#### create — Create a new container (without starting)
`name` and `image` are required.

- **name** (required)
- **image** (required)
```json
{
  "action": "docker",
  "operation": "create",
  "name": "my_nginx",
  "image": "nginx:latest",
  "ports": {"80": "8080"},
  "volumes": ["/data/html:/usr/share/nginx/html:ro"],
  "env": ["NGINX_HOST=example.com"],
  "restart": "unless-stopped"
}
```

#### run — Create AND start a container in one step
`name` and `image` are required. Short-lived containers still need an explicit name. Set `auto_remove: true` only with restart policy `no`.

- **name** (required)
- **image** (required)
```json
{
  "action": "docker",
  "operation": "run",
  "name": "redis_cache",
  "image": "redis:7-alpine",
  "ports": {"6379": "6379"},
  "restart": "always"
}
```

For exact command arguments, use `command_args`. This array is forwarded without shell parsing:
```json
{
  "action": "docker",
  "operation": "run",
  "name": "one-shot-report",
  "image": "alpine:latest",
  "command_args": ["/bin/sh", "-lc", "printf '%s\\n' \"$REPORT\""],
  "auto_remove": true,
  "restart": "no"
}
```

For `create` and `run`, the legacy `command` field accepts only simple whitespace-separated arguments. Quotes, pipes, redirections, command chaining, substitutions, and globbing are rejected. Do not provide both `command` and `command_args`.

### Container Operations

#### exec — Run a command inside a running container
```json
{"action": "docker", "operation": "exec", "container_id": "my_db", "command": "mysql -u root -p my_db"}
{"action": "docker", "operation": "exec", "container_id": "my_web", "command": "cat /etc/nginx/nginx.conf", "user": "root"}
```

#### stats — Real-time resource usage of a container (CPU, Mem, Net I/O)
```json
{"action": "docker", "operation": "stats", "container_id": "my_container"}
```

#### top — List running processes inside a container
```json
{"action": "docker", "operation": "top", "container_id": "my_container"}
```

#### port — Show mapped ports for a container
```json
{"action": "docker", "operation": "port", "container_id": "my_container"}
```

#### cp — Copy files between host and container
Use `direction: "from_container"` or `"to_container"`. Path maps to the host's absolute path, Destination maps to the container's absolute path (or vice versa depending on direction).
```json
{"action": "docker", "operation": "cp", "container_id": "my_container", "source": "/etc/nginx/nginx.conf", "destination": "/tmp/host_nginx.conf", "direction": "from_container"}
{"action": "docker", "operation": "cp", "container_id": "my_container", "source": "/tmp/host_nginx.conf", "destination": "/etc/nginx/nginx.conf", "direction": "to_container"}
```

### Image Operations

#### list_images — List local images
```json
{"action": "docker", "operation": "list_images"}
```

#### pull — Pull an image from a registry
```json
{"action": "docker", "operation": "pull", "image": "postgres:16"}
```

#### remove_image — Delete a local image
```json
{"action": "docker", "operation": "remove_image", "image": "old_image:v1", "force": true}
```

### Infrastructure

#### list_networks — List Docker networks
```json
{"action": "docker", "operation": "list_networks"}
```

#### create_network / remove_network
```json
{"action": "docker", "operation": "create_network", "name": "my_net", "driver": "bridge"}
{"action": "docker", "operation": "remove_network", "name": "my_net"}
```

#### connect / disconnect — Connect a container to a network
```json
{"action": "docker", "operation": "connect", "container_id": "my_container", "network": "my_net"}
{"action": "docker", "operation": "disconnect", "container_id": "my_container", "network": "my_net"}
```

#### list_volumes — List Docker volumes
```json
{"action": "docker", "operation": "list_volumes"}
```

#### create_volume / remove_volume
```json
{"action": "docker", "operation": "create_volume", "name": "my_data_vol", "driver": "local"}
{"action": "docker", "operation": "remove_volume", "name": "my_data_vol", "force": true}
```

#### compose — Run docker compose commands
`file` is a Compose file inside the agent workspace (relative paths resolve against it).
```json
{"action": "docker", "operation": "compose", "command": "up -d", "file": "stacks/web/docker-compose.yml"}
{"action": "docker", "operation": "compose", "command": "down", "file": "stacks/web/docker-compose.yml"}
```
- AuraGo resolves the file with `docker compose config` before anything runs. A file that does not resolve returns `docker_compose_preflight_failed` with Compose's error.
- Every compose command is rejected when the stack touches AuraGo-managed containers, labels, images or volumes. When AuraGo itself runs in Docker, `up`, `create`, `start`, `stop`, `restart`, `down`, `rm`, `kill`, `pause`, `unpause`, `logs` and `top` are also rejected for a stack whose project name is AuraGo's own Compose project (`docker_managed_aurago_resource`): give the stack its own top-level `name:` or another folder.
- `up`, `create` and `build` (and `config`/`convert` for env files, secret and config files) also reject anything that points into AuraGo's own data directory, config, `.env` or master key, and `up`/`create`/`build`/`pull`/`config`/`convert` reject AuraGo's master key value anywhere in the resolved file (`docker_compose_protected_path_denied`), even with host access.
- Unless the administrator enabled **Docker host access** (`docker.allow_host_access`), `up`/`create`/`build` also reject binds outside the workspace (or that contain AuraGo's files), `/var/run/docker.sock`, `use_api_socket`, devices, `device_cgroup_rules`, `gpus` and reserved devices, `privileged`, host network/PID/IPC/UTS/user/cgroup namespaces, `cap_add`, unconfined `security_opt`, local volumes that bind a host directory, mount a `/dev` device or a non-network filesystem, `develop.watch` paths outside the workspace, build SSH (`build.ssh`, `build --ssh`), privileged builds, build entitlements, `build.network: host`, and env files, secrets, configs, Dockerfiles or build contexts outside the workspace (`docker_compose_host_access_denied`). A command without service names checks every service of the file's default profiles; a command that names services (`up -d web`) checks those, what they need (depends_on, links, volumes_from, `network_mode: service:`, `service:` build contexts) and, except for `build`/`pull`, what depends on them, as long as every name exists and every flag is a known one (an unknown flag such as `--scale` checks the whole file). Tell the user which setting is needed; do not retry unchanged.
- Without host access, `provider` services and privileged `post_start`/`pre_stop` hooks also block `down`, `start`, `stop`, `restart`, `rm` and `pull` (including profile services the command names and the services they depend on), because Compose runs them on the host.
- `kill`, `pause`, `unpause`, `ps`, `logs` and the other inspection commands are never blocked by the host-access check.
- Without host access, Compose runs with a minimal environment (`PATH`, `HOME`, Docker/Compose/BuildKit settings, proxy, temp and locale variables): `${VAR}` in a Compose file cannot read AuraGo's own variables, so put such values into the stack's `.env` file. Registry credentials must then come from the Docker config (`~/.docker/config.json`) or a credential helper that needs no secret environment variables.
- `config -o <file>` / `--output` writes only inside the workspace (relative paths resolve against it; AuraGo's own data, config and `.env` files, directories and special files are refused): Compose renders into a private staging file and AuraGo publishes it atomically at that path, creating missing folders, and reports `output_file`; `-q` or list flags such as `--services` write no file. `config`/`convert` stay read-only, so they also work when Docker is read-only (`docker.read_only`). `--environment` and `--env-file` are rejected.
- Without host access, AuraGo repeats its checks right before the run; `docker_compose_input_changed` means the Compose file or a file it reads changed in between. Retry once nothing rewrites the files.

#### info — Docker engine system info (version, resource counts)
```json
{"action": "docker", "operation": "info"}
```

## Important Notes
- Docker mutations are sent once. A lost response can leave a successful remote operation; inspect its existing container/exec ID before retrying. Do not automatically repeat the command.
- `exec` follows the tool run's cancellation and a ten-minute ceiling. Cancelling the HTTP stream does not prove that the command inside the container has stopped; verify its state before retrying.
- AuraGo negotiates the Engine API before operations. Socket proxies must allow `GET /version`; unsupported API ranges fail explicitly before mutation.
- `container_id` accepts both container IDs (short or full) and container names
- `name` is mandatory for every `create` and `run`, including short-lived jobs
- `run` = `create` + auto-`start` in a single call
- `auto_remove` defaults to `false`, is valid only for `run`, and conflicts with every restart policy other than `no`
- `aurago-homepage`, `aurago-homepage-web`, and the `aurago-homepage` image repository cannot be managed with this tool
- The names `aurago` (and compose replicas such as `stack-aurago-1`) and `aurago-boring-garage` are reserved the same way: they cannot be used as `name` for `create`/`run`, even next to a different `container_id`, and those containers cannot be inspected, controlled or read through this tool
- Logs are truncated to ~8000 chars to avoid flooding the context
- `force: true` on remove will kill a running container before removing it
- Port mapping format: `{"container_port": "host_port"}` — both as strings
- Volume bind format: `"/host/path:/container/path"` or `"/host/path:/container/path:ro"`
- When AuraGo runs in Docker, its data volume (`<project>_aurago_data` or the volume behind `/app/data`) cannot be mounted (`create`/`run`), created or removed, and no Compose stack may use it (`docker_managed_aurago_resource`). If AuraGo cannot identify its own container (Docker did not answer, or AuraGo shares another container's network), every volume named `aurago_data` or ending in `_aurago_data` counts as AuraGo's; give other stacks' volumes a different name. The workdir volume stays usable.
