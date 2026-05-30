---
id: proxmox
title: Proxmox VE Workflow
enabled: true
priority: 90
tools: [proxmox]
workflows: [proxmox, virtualization, vm, lxc, cluster, infrastructure]
keywords:
  - proxmox
  - proxmox ve
  - pve
  - vm
  - vms
  - virtual machine
  - lxc
  - container
  - qemu
  - node
  - cluster
  - snapshot
  - hypervisor
---

This rule applies whenever managing, inspecting, or operating Proxmox VE virtual machines, LXC containers, nodes, storage, or cluster resources.

## Proxmox VE Workflow

Treat Proxmox as a production virtualization platform. Every action affecting VMs, containers, or nodes must be deliberate, observable, and reversible where possible.

### Read-Only First

Always start with read-only discovery operations before making changes:

1. **Cluster overview.** Call `overview` to understand the cluster topology, node health, and running workloads.
2. **List resources.** Use `list_vms`, `list_containers`, or `cluster_resources` to see the current state.
3. **Node status.** Check `node_status` for resource utilization (CPU, memory, disk) before scheduling new workloads.
4. **VM/CT status.** Call `status` for the specific target before starting, stopping, or rebooting it.

### Snapshot Before Mutate

For any VM or container that hosts important services:

- **Create a snapshot before destructive or significant changes.** Use `create_snapshot` with a descriptive name and optional description.
- Verify the snapshot was created successfully via `list_snapshots` before proceeding with the operation.
- This applies to: package upgrades, configuration changes, migrations, or experimental modifications.

### Power Action Discipline

Power actions (`start`, `stop`, `shutdown`, `reboot`, `suspend`, `resume`, `reset`) are gated by permissions:

- Respect the `read_only` flag: if Proxmox is configured read-only, refuse mutating operations and explain the constraint.
- Destructive actions (`stop`, `shutdown`, `reboot`, `suspend`, `reset`) require `allow_destructive=true` in config. If blocked, inform the user which config key is needed.
- Prefer `shutdown` over `stop` for graceful termination. Use `stop` only when the guest is unresponsive or the user explicitly requests a hard stop.
- Use `reset` only as a last resort for hung VMs.

### VM vs. Container Selection

- Use `qemu` (VM) when the workload requires a full kernel, custom operating system, GPU passthrough, or legacy compatibility.
- Use `lxc` (container) when the workload is Linux-native, lightweight, and does not require a custom kernel. LXC containers start faster and have lower overhead.
- When the user says "container" in a Proxmox context, default to `lxc`, not Docker.

### Node and Cluster Awareness

- Always specify the `node` parameter when operating on node-scoped resources. If omitted, the default node from config is used.
- For cluster-wide queries, rely on `overview` and `cluster_resources` rather than iterating individual nodes.
- If a node-specific API call returns `403 Forbidden`, fall back to cluster resource views which may have broader read permissions.

### Storage and Tasks

- Check `storage` before creating new VMs or containers to ensure sufficient space.
- Long-running operations (clone, migrate, backup) return a task UPID. Use `task_log` to monitor progress and completion.
- Do not assume a task completed successfully just because it was initiated. Poll the task log for status when the user needs confirmation.

### Safety and Permissions

- Never guess VMIDs. Always list existing VMs/containers to find available IDs or use the next free ID from the cluster resource view.
- Do not expose Proxmox API tokens or secrets in conversation. The tool handles authentication securely via the vault.
- Tag resources consistently where the Proxmox tagging feature is available, to enable filtering and organization.
