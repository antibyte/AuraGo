# Proxmox VE Tool (`proxmox`)

Manage Proxmox VE virtual machines and containers.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `overview` | Monitoring overview of nodes, VMs, and containers | `node` |
| `list_nodes` | List cluster nodes | — |
| `list_vms` | List virtual machines | `node` |
| `list_containers` | List LXC containers | `node` |
| `status` | Get VM/CT status | `node`, `vmid`, `vm_type` |
| `start` | Start a VM/CT | `node`, `vmid`, `vm_type` |
| `stop` | Force stop | `node`, `vmid`, `vm_type` |
| `shutdown` | Graceful shutdown | `node`, `vmid`, `vm_type` |
| `reboot` | Reboot | `node`, `vmid`, `vm_type` |
| `suspend` / `resume` | Suspend/resume | `node`, `vmid`, `vm_type` |
| `node_status` | Node resource usage | `node` |
| `cluster_resources` | Cluster overview | `resource_type` |
| `storage` | Storage info | `node` |
| `create_snapshot` | Create snapshot | `node`, `vmid`, `name`, `description` |
| `list_snapshots` | List snapshots | `node`, `vmid` |
| `task_log` | View task log | `node`, `upid` |

## Examples

```json
{"action": "proxmox", "operation": "list_vms"}
```

```json
{"action": "proxmox", "operation": "overview"}
```

```json
{"action": "proxmox", "operation": "start", "vmid": "100", "vm_type": "qemu"}
```

```json
{"action": "proxmox", "operation": "create_snapshot", "vmid": "101", "name": "before-update", "description": "Pre-update snapshot"}
```

```json
{"action": "proxmox", "operation": "cluster_resources", "resource_type": "vm"}
```

## Notes
- `node` defaults to the configured default node if omitted
- `vm_type` defaults to `qemu` (VMs); use `lxc` for containers
- `overview` is the best default operation for monitoring. It returns normalized node, VM, and container lists plus compact summaries in one read-only call.
- Agent guidance: for monitoring, health checks, or general environment awareness, prefer `overview` before narrower follow-up calls.
- In `read_only` mode, read operations such as `list_nodes`, `list_vms`, `list_containers`, `status`, `node_status`, `cluster_resources`, `storage`, `list_snapshots`, and `task_log` remain allowed. Only mutating operations are blocked.
- `list_vms` and `list_containers` now include a normalized `monitoring` list plus a compact `summary` block (`total`, `running`, `stopped`, `paused`, `unknown`) to make monitoring easier for the agent. The original raw `vms` or `containers` array is still included for compatibility.
- If node-specific VM/container or node status endpoints return `403 Forbidden`, AuraGo may automatically fall back to `cluster_resources` and return `source: "cluster_resources"` in the result. This is still read-only data and is useful when the token can read cluster overview but not the more specific endpoint.
- If a supposedly unrestricted root API token still returns `403`, the most likely Proxmox-side causes are token privilege separation, token scope/ACL assignment, or a mismatch in the configured token identity. That is a Proxmox permission issue, not AuraGo `read_only`.
