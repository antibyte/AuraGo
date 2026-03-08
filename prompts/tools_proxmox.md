---
id: "tools_proxmox"
tags: ["conditional"]
priority: 31
conditions: ["proxmox_enabled"]
---
### Proxmox VE Management
| Tool | Purpose |
|---|---|
| `proxmox` | Manage Proxmox VE: list nodes, VMs, containers, start/stop/reboot VMs/CTs, snapshots, storage info, cluster resources |

**Operations:**
- `list_nodes` — List all cluster nodes
- `list_vms` — List QEMU VMs on a node
- `list_containers` — List LXC containers on a node
- `status` — Get VM/CT status (requires vmid)
- `start`/`stop`/`shutdown`/`reboot`/`suspend`/`resume` — VM/CT power actions (requires vmid)
- `node_status` — Get node resource usage
- `cluster_resources` — Unified resource list (optionally filter by type: vm, node, storage)
- `storage` — List storage on a node
- `create_snapshot` — Create a snapshot (requires vmid)
- `list_snapshots` — List snapshots (requires vmid)
- `task_log` — Get task log (requires upid)

**Parameters:** `operation`, `node` (optional, uses default), `vmid`, `vm_type` ("qemu" or "lxc"), `name` (snapshot name), `description`, `upid`, `resource_type`
