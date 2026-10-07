---
id: "tools_docker"
tags: ["conditional"]
priority: 31
conditions: ["docker_enabled"]
---
### Docker Management

The `docker` tool manages Docker containers, images, networks and volumes. Every call names its action in the `operation` argument, for example `{"operation": "list_containers", "all": true}`. There are no separate functions per action.

#### Containers
| Operation | Purpose |
|---|---|
| `list_containers` | List running containers (`all: true` adds stopped ones) |
| `inspect` | Detailed container information (`container_id`) |
| `start`, `stop`, `restart`, `pause`, `unpause` | Container lifecycle (`container_id`) |
| `remove` | Remove a container (`force: true` stops it first) |
| `logs` | Last log lines of a container (`tail`, default 100) |
| `create` | Create a container (`name` and `image` are required) |
| `run` | Create and start a container in one call |
| `exec` | Run a command in a running container (`command`, optional `user`) |
| `stats` | Resource usage of a container |
| `top` | Processes inside a container |
| `port` | Port mappings of a container |
| `cp` | Copy a file between the workspace and a container (`source`, `destination`, `direction`) |

#### Images
| Operation | Purpose |
|---|---|
| `list_images` | List local images |
| `pull` | Pull an image (`image`) |
| `remove_image` | Remove a local image |

#### Networks and volumes
| Operation | Purpose |
|---|---|
| `list_networks` | List networks |
| `create_network`, `remove_network` | Create or remove a network (`name`, optional `driver`) |
| `connect`, `disconnect` | Attach a container to a network or detach it (`container_id`, `network`) |
| `list_volumes` | List volumes |
| `create_volume`, `remove_volume` | Create or remove a volume (`name`, optional `driver`) |

#### Compose and engine
| Operation | Purpose |
|---|---|
| `compose` | Run `docker compose` for a file inside the workspace (`file`, `command`) |
| `info` | Docker engine information (version, container and image counts) |

#### Usage Notes
- There is no rename operation.
- There is no system prune operation. Remove unused images and containers one at a time with `remove_image`/`remove` after checking `list_images`/`list_containers` (see the Docker rule).
- All container/image names are validated for safety (no path traversal, special characters)
- Docker API requests include automatic retry logic for transient failures
- Container logs are truncated to 8000 characters max to prevent memory issues
- Exec output is limited to 64KB to prevent memory exhaustion
- Health status is included in container listings when available
