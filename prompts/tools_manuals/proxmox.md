# Proxmox VE Tool (`proxmox`)

Manage Proxmox VE virtual machines and containers.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
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
