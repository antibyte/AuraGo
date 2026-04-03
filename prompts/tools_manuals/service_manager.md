# service_manager — System Service Management

Manage system services (systemd on Linux, Services on Windows) directly.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `status` | Get the status of a service | `service_name` |
| `start` | Start a service | `service_name` |
| `stop` | Stop a service | `service_name` |
| `restart` | Restart a service | `service_name` |
| `enable` | Enable a service on boot | `service_name` |
| `disable` | Disable a service on boot | `service_name` |
| `list` | List all relevant services | (none) |

## Key Behaviors

- Abstracted across OS: Uses `systemctl` on Linux and `Get-Service`/`Start-Service` on Windows seamlessly.
- Will fail gracefully if the agent does not have sufficient privileges to control the requested service.
- Works across local and active SSH connections.

## Examples

```
# Check if nginx is running
service_manager(operation="status", service_name="nginx")

# Restart docker service
service_manager(operation="restart", service_name="docker")

# Enable a custom service on boot
service_manager(operation="enable", service_name="my-app")
```

## Tips
- Always check `status` before and after modifying a service state.
- Some services might require `sudo` or elevated privileges.