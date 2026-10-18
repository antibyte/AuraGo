# Infra: the homelab atlas

Infra is the Desktop's shared inventory map for hardware, virtual machines,
containers, applications and network segments. Atlas, topology and inventory
show the same entries. Positions belong to the AuraGo instance; view, selection
and navigation belong to the current window.

## Configuration

Open Configuration → Virtual Desktop → Infra. Infra is enabled by default when
the Desktop is enabled; an explicitly saved disabled setting remains respected.
Local editing, source editing and device control are separate capabilities:

```yaml
virtual_desktop:
    infra:
        enabled: true
        read_only: false
        allow_source_edit: false
        allow_control: false
```

All Infra API access requires a Desktop administrator. Desktop read-only mode
and Infra read-only mode block edits and actions. Source edits and control also
require their own Infra grant and the source's existing permission. Enabling
Infra does not enable Docker, Proxmox, FRITZ!Box or any other integration.

## Build your map

Use **Import** to preview entries from configured, allowed sources, or create a
local entry. Linking an imported item to an existing entry is an explicit
choice. Matching names and addresses are suggestions, never identity. Manual
device entries create a registry device with protocol `none`; assign SSH/VNC
and credentials deliberately through the existing registry and credential UI.
VMs, containers, services, groups and network segments do not create registry
devices. A VM or container can be explicitly linked to an existing registry
device to select its Quick Connect target.

Discovered Registry and FRITZ!Box devices with blank/default names or only a
hexadecimal identifier, UUID or MAC address are hidden from the candidate list.
Give them a recognizable name in their source to make them available. This
filter also hides untouched, unplaced imports with these names. Registry data
and saved nodes remain stored; edited, placed, grouped and connected entries
remain visible. When the source resolves a friendly name for a hidden import,
Infra shows that name without rewriting the saved node.

Create sites, rooms and racks to organize the map. The side list treats sites,
rooms, racks and groups as containers. On the atlas and topology, a container
with at least one placed child draws a frame around those saved positions.
Place unplaced entries, pin positions, and connect entries with a selected
relationship type. Arrange on the atlas keeps the grid. Arrange on the topology
lays unpinned entries in layers: network and places, then devices, then guests,
then services. Both can be undone. Lines stop short of the node art and show
port and speed text when those fields are set. Refreshing source data preserves
positions. A visible window keeps the inspector in place while its draft is
dirty or focused.
Open a host to expand its explicitly linked services in the atlas. Topology
and inventory keep the complete view of those entries.
Ethernet, fiber, Wi-Fi, network membership, hosting, dependencies and power
relationships remain distinct; a shared subnet does not imply a cable.

The detail editor separates **Save in Infra** from **Save in source**. Local
profiles hold hardware details, network interfaces, CIDR/VLAN, location,
management links, notes and custom fields. Blank information remains unknown.
Saving local details never changes an existing registry device or its Quick
Connect destination; use the source editor for those changes.
Failed saves and version conflicts retain the draft rather than silently
overwriting another editor's changes. Closing a window with unsaved local,
source or layout changes asks whether to discard them or keep editing.

App links select an existing target: Quick Connect selects a registry device
without connecting or replacing the current session; Network Cameras selects a
configured camera; Software Store selects an installed app; MeshCore selects a
confirmed conversation. Opening these targets does not dispatch power actions.

## Source edits and actions

| Source | Editable source metadata | Control |
| --- | --- | --- |
| Registry | Existing device and connection fields, credential reference | Open Quick Connect |
| Docker | Container name | Start, stop, restart |
| Proxmox | QEMU name / LXC hostname, description, tags | Start, graceful shutdown, reboot |
| FRITZ!Box | Supported host / friendly name | Wake-on-LAN |

Two gates apply on top of the table. Proxmox graceful shutdown and reboot also
require `proxmox.allow_destructive`; without it they are reported as blocked by
source policy. Docker stop/restart and Proxmox shutdown/reboot require the
explicit confirmation of the action dialog, and the API refuses them without
`confirmed: true` (400 `confirmation_required`). A managed container
(go2rtc, RuView, Security Proxy, Decisions, local LLM, ACE-Step, Garage,
Homepage, CYD Creator and AuraGo itself) is shown as protected and answers an
action with `status: "failed"` and `reason: "managed_container"`.

Source forms expose only supported fields. Docker labels are not changed by
recreating a container. Protected and AuraGo-managed containers retain their
existing restrictions, including the rename prohibition. LXC hostname changes
have an explicit warning. Proxmox config digests protect against concurrent
source edits. Pending Proxmox metadata is explicitly marked as pending;
Infra retains the draft until the source confirms the effective change.

Docker, Proxmox guests and FRITZ!Box hosts show a separate power reading:
running, stopped, paused or unknown. It comes from the latest current
observation. A stale source does not keep a power claim. The reading is not
stored in the local role field, and an older imported role is left as written.
FRITZ!Box power means the host was active on the LAN, not that Wake-on-LAN
finished. A Proxmox task acknowledgement and a sent Wake-on-LAN packet mean
accepted, not confirmed device startup. An uncertain operation is never automatically retried. Request
IDs and outcomes survive server restarts, so replay does not repeat an uncertain
mutation. Inspect the source before issuing a new operation after uncertainty.
Revoking Infra or integration permissions cancels affected requests. Operations
that have not been sent are blocked; cancellation after dispatch does not prove
that the device action was stopped and can leave its outcome unknown.

## Persistence and privacy

The `infra_*` tables share the existing Inventory SQLite database. Infra backs
up that database before its additive migration, keeps its own schema version,
and does not change the registry's SQLite version. Deleting an Infra entry does
not delete the source device, container or guest. Linked last-known source data
survives outages and removal; stale and removed are visible states. A failed or
partial enumeration never proves an object was deleted.

Identities use registry IDs, full Docker IDs, Proxmox instance plus guest type
and VMID, or a server-owned opaque FRITZ!Box reference. Proxmox node migration
changes routing, not identity. Names and addresses do not merge identities.

Only bounded, typed projections reach the graph. Credentials, raw provider
responses and Docker environment variables are excluded. Management links must
be HTTP(S) addresses without embedded credentials. Opening Infra immediately
loads the saved map while one bounded background refresh updates configured
sources. Visible windows pick up these observations about every 30 seconds;
**Refresh** waits for fresh source results. Hidden or closed windows stop
requesting refreshes. Infra does not run network scans or continuous monitoring.

The store is initialized when the server starts, so its additive migration and
the one-time database backup never run inside a request. A provider that cannot
be read is reported as a sanitized code (`provider_unreachable`,
`auth_failed`, `observation_limit`) in the snapshot's `source_errors` map and
the affected sources stay stale; provider error text is never forwarded.

## API overview

All routes require a Desktop administrator session or an admin bearer token and
answer `no-store` JSON. Writes additionally require the Desktop write
permission and a configuration that is enabled and not read-only.

| Route | Purpose |
| --- | --- |
| `GET /api/desktop/infra/snapshot` | Stored graph plus `capabilities` and optional `source_errors`; `?refresh=1` waits for providers |
| `GET /api/desktop/infra/candidates` | Import preview for configured sources |
| `GET /api/desktop/infra/targets` | Selectable app targets (camera, Store app, MeshCore conversation) |
| `POST /api/desktop/infra/nodes` | Create a node (manual devices also create a `protocol: none` registry row) |
| `PUT`/`DELETE /api/desktop/infra/nodes/{id}` | Update (version CAS) or delete a node; `?version=` required for delete |
| `GET`/`PUT /api/desktop/infra/nodes/{id}/source-edit` | Read or write the source's own metadata |
| `POST /api/desktop/infra/nodes/{id}/actions` | Reserved power action with `request_id` and optional `confirmed` |
| `POST /api/desktop/infra/import` | Import a candidate or link it to `node_id` |
| `POST /api/desktop/infra/edges`, `PUT`/`DELETE /edges/{id}` | Manual relationships (provider edges are read-only) |
| `PUT /api/desktop/infra/layout` | Atomic layout batch; answers the changed nodes only |

Stable error codes include `infra_disabled`, `infra_readonly`,
`infra_policy_changed`, `invalid_request`, `request_too_large` (413),
`not_found`, `conflict`, `has_children`, `source_unavailable`,
`source_readonly`, `source_edit_disabled`, `confirmation_required`,
`request_id_conflict`, `action_disabled` and `provider_unavailable`.

Infra is an operator surface: the agent has no tool that reads or writes the
atlas. The registry it links to stays reachable through the existing inventory
tools.

## Local assets and development

The 64 selectable Imagegen miniatures, Infra app icon and empty-atlas artwork
are local WebP assets under `ui/img/infra/`. Their manifest and prompt provenance
are versioned with the UI. Using Infra never requires an image generator.
The map uses focusable DOM nodes and SVG connections, without a 3D runtime.

The package contract is [internal/infra/AGENTS.md](../internal/infra/AGENTS.md).
Run the focused Infra package, server adapter, configuration and UI tests, the
UI bundle check and the target-context browser fixture before changing this
flow. Local fixtures do not establish live-provider or hardware compatibility;
that acceptance requires an explicitly authorized deployment and device test.
