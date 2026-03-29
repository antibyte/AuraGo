---
id: "tools_docker"
tags: ["conditional"]
priority: 31
conditions: ["docker_enabled"]
---
### Docker Management

The `docker` tool provides comprehensive Docker container, image, network, and volume management capabilities.

#### Container Operations
| Function | Purpose |
|---|---|---|
| `docker_list_containers` | List all containers (running and stopped) |
| `docker_inspect_container` | Get detailed container information |
| `docker_container_action` | Start, stop, restart, pause, unpause, or remove containers |
| `docker_container_logs` | Retrieve container logs (last N lines) |
| `docker_create_container` | Create a new container with custom configuration |
| `docker_exec` | Execute commands inside a running container |
| `docker_stats` | Get real-time resource usage statistics |
| `docker_top` | List running processes inside a container |
| `docker_port` | Show mapped ports for a container |
| `docker_rename_container` | Rename an existing container |

#### Image Operations
| Function | Purpose |
|---|---|---|
| `docker_list_images` | List local Docker images |
| `docker_pull_image` | Pull an image from a registry |
| `docker_remove_image` | Remove a local image |

#### Network Operations
| Function | Purpose |
|---|---|---|
| `docker_list_networks` | List all Docker networks |
| `docker_create_network` | Create a new network |
| `docker_remove_network` | Remove a network |
| `docker_connect_network` | Connect a container to a network |

#### Volume Operations
| Function | Purpose |
|---|---|---|
| `docker_list_volumes` | List all Docker volumes |

#### System Operations
| Function | Purpose |
|---|---|---|
| `docker_system_info` | Get Docker engine information (version, containers, images count) |
| `docker_system_prune` | Remove unused data (containers, networks, images, volumes) - **destructive** |

#### Usage Notes
- All container/image names are validated for safety (no path traversal, special characters)
- Docker API requests include automatic retry logic for transient failures
- Container logs are truncated to 8000 characters max to prevent memory issues
- Exec output is limited to 64KB to prevent memory exhaustion
- Health status is included in container listings when available
