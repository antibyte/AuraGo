# Docker Management Tool

Manage Docker containers, images, networks, and volumes directly through the Docker Engine API.

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
Requires `file` pointing to the `docker-compose.yml` path.
```json
{"action": "docker", "operation": "compose", "command": "up -d", "file": "/path/to/project/docker-compose.yml"}
{"action": "docker", "operation": "compose", "command": "down", "file": "/path/to/project/docker-compose.yml"}
```

#### info — Docker engine system info (version, resource counts)
```json
{"action": "docker", "operation": "info"}
```

## Important Notes
- `container_id` accepts both container IDs (short or full) and container names
- `run` = `create` + auto-`start` in a single call
- Logs are truncated to ~8000 chars to avoid flooding the context
- `force: true` on remove will kill a running container before removing it
- Port mapping format: `{"container_port": "host_port"}` — both as strings
- Volume bind format: `"/host/path:/container/path"` or `"/host/path:/container/path:ro"`
