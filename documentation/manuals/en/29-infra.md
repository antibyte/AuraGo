# Chapter 29: Infra – the homelab atlas

Infra is a shared map of your homelab. It shows hardware, virtual machines, containers, services and network segments
as an atlas, a topology diagram or an inventory list, and links each entry to its source: the device registry, Docker,
Proxmox or FRITZ!Box. It is for administrators who want one picture of their setup. It does not scan your network or
monitor devices, and it changes a device only when you ask it to.

Technical details for administrators (configuration reference, API routes, error codes, development notes) are in the
[operator reference](../../infra.md).

## Table of Contents

1. [What You Need and Switching It On](#what-you-need-and-switching-it-on)
2. [First Steps](#first-steps)
3. [Build Your Map](#build-your-map)
4. [Views, Places and Relations](#views-places-and-relations)
5. [Edit Assets and Save](#edit-assets-and-save)
6. [Source Edits](#source-edits)
7. [Power Actions](#power-actions)
8. [Good to Know](#good-to-know)
9. [Privacy and Safety](#privacy-and-safety)
10. [When Something Goes Wrong](#when-something-goes-wrong)

---

## What You Need and Switching It On

Infra is for administrators only: every Infra request needs a Desktop administrator session. Two switches must be on:

- **Config → Virtual Desktop → "Enable Virtual Desktop"** (`virtual_desktop.enabled`; the shipped `config_template.yaml` turns it on, a config without
  the key leaves it off).
- **Config → Virtual Desktop → "Enable Infra"** (`virtual_desktop.infra.enabled`, on by default). It sits in the
  **Infra** group of that page. Switching it off closes the atlas for everyone; the stored map stays on disk.

Three more switches in the same group are separate on purpose:

| Switch | Config key | Default | What it allows |
|---|---|---|---|
| "Read-only" | `virtual_desktop.infra.read_only` | off | Browsing only. Blocks local edits, source edits and device actions. |
| "Allow source edits" | `virtual_desktop.infra.allow_source_edit` | off | Changing supported fields in the device registry, Docker, Proxmox and FRITZ!Box. |
| "Allow device control" | `virtual_desktop.infra.allow_control` | off | Power actions: start, stop, restart, shut down, reboot and Wake-on-LAN. |

Source edits and device control each need more than their own switch:

- The Desktop's **Read-only mode** (`virtual_desktop.readonly`) blocks all writes, including source edits and actions.
- Source edits need a writable source: enabled and not read-only (`docker.readonly`, `proxmox.readonly`,
  `fritzbox.network.readonly`).
- Device control needs the source's permission for each action; see [Power Actions](#power-actions).

No restart is needed for these switches. Enabling Infra does not enable Docker, Proxmox or FRITZ!Box.

---

## First Steps

1. Turn on the Desktop and Infra as described above, then open the Desktop in your browser.
2. Open **Infra** from the start menu, in the **System** category. The window opens maximized.
3. A map with nothing placed yet offers **Add your first node** and **Import existing assets**. Start with the import
   for devices that already exist.

---

## Build Your Map

Infra never adds an entry by itself. You create entries in one of two ways.

**Import from a source.** Click **Import** in the toolbar, or choose it from the window menu **Infra**. The dialog lists
new candidates from these sources:

| Source | Available when |
|---|---|
| Device registry | Always, from the inventory database |
| Docker | `docker.enabled` is on |
| Proxmox | `proxmox.enabled` is on |
| FRITZ!Box Hosts | FRITZ!Box is enabled with a host, and its network Hosts feature is on |

Other integrations (Home Assistant, TrueNAS, Tailscale) do not feed Infra; see [Chapter 8: Integrations](./08-integrations.md).

Tick the candidates, keep **Import as new node** or choose **Link to existing**, and click **Import selected**. Matching
names or addresses only suggest a link. Registry and FRITZ!Box entries whose name is empty, a default (`unnamed device`,
`Fritz!Box host`) or only a hex ID, UUID or MAC address stay hidden until the source gives them a readable name.

**Create a manual entry.** Click **Add node** (or **Add infrastructure node** in the window menu), then enter a name, a
type (Device, Virtual machine, Container, Service, Network, Site, Room, Rack or Group) and an icon. A manual **Device**
also creates a registry row with protocol `none`; add SSH, VNC or credentials there, on purpose. The other types create
no registry devices.

---

## Views, Places and Relations

**Atlas**, **Topology** and **Inventory** show the same entries. The atlas is a grid map. The topology arranges entries in
layers: network and places first, then devices and groups, then virtual machines and containers, then services.

Power is not a fourth view. It is a reading on each entry: *Running*, *Stopped*, *Paused*, *Unknown* or *No power
reading*. The sidebar narrows the list to **All assets**, **Unplaced** or **Needs attention** (stale, unknown or removed
sources). The toolbar filters by type, source and power.

**Places** are sites, rooms, racks and groups, listed under the sidebar heading **Places**. A place with placed children
draws a frame around them.

**Placing.** Unplaced entries wait in the tray below the map. Drag one onto the map, or choose **Place at center**,
**Place upper left** or **Move with keyboard** in its detail editor. With the keyboard, move the entry with the arrow keys
and press Enter to save.

**Arrange** lays the map out again (grid on the atlas, layers on the topology); pinned entries stay put. **Undo** reverts
the last layout change.

**Relations.** Click **Connect** and choose two entries, or use **Add relation** in the detail editor. Types are
*Ethernet*, *Fiber*, *Wi-Fi*, *Member of*, *Runs on*, *Depends on* and *Powered by*. A shared subnet is not a cable.
On the atlas, opening a host expands the services explicitly linked to it.

---

## Edit Assets and Save

The detail editor has these parts: **Identity**, **Hardware & network** (role, vendor, model, serial number, operating
system, firmware, CPU, memory, storage, rack position, addresses, tags, notes, interfaces, links, custom fields),
**Local app links**, **Placement**, **Observed sources** and **Relations**.

**Save** stores your changes in Infra and stays disabled until something changes. It never changes a registry device or
its Quick Connect target; use source edits for those. **Discard** drops the draft. Closing Infra with unsaved changes asks
whether to discard them or keep editing.

If someone saved the same entry meanwhile, Infra refuses your save and keeps your draft. Click **Refresh**, compare under
**Review current asset values**, and choose **Keep and reapply my draft** if your version should win.

**Remove from Infra** deletes only the Infra entry. The source device, container or VM stays, and so does a registry
device created by a manual entry; delete that in the device registry if you no longer need it. An entry with children
cannot be removed until they are moved or removed.

---

## Source Edits

Source edits change metadata that the source stores itself. You need **Allow source edits** and a writable source. Each
source row in the detail editor has an **Edit source** button, and the form shows only the fields the source supports:

| Source | Editable fields |
|---|---|
| Device registry | Name, type, protocol (*None*, *SSH*, *VNC*), IP address, port, username, credential (picked from the list), description, tags, MAC address |
| Docker | Container name. Docker renames the container; it is not recreated. |
| Proxmox | Name for a VM, hostname for a container (LXC), description, tags |
| FRITZ!Box Hosts | Friendly name |

**Save source metadata** writes the change and reads it back to confirm it. A Proxmox change may stay *pending* until
Proxmox confirms it. Proxmox changes carry a config digest, so an edit made elsewhere is refused, not overwritten.
AuraGo-managed containers cannot be renamed here.

---

## Power Actions

Power actions send one explicit request to a source. Open the source row in the detail editor and choose
**Infrastructure actions**.

| Source | Action | Confirmation | Additional requirement |
|---|---|---|---|
| Docker container | Start | No | `docker.enabled` on, `docker.readonly` off |
| Docker container | Stop, Restart | Yes | As for Start |
| Proxmox VM or container | Start | No | `proxmox.enabled` on, `proxmox.readonly` off |
| Proxmox VM or container | Shut down, Reboot | Yes | Also `proxmox.allow_destructive: true` |
| FRITZ!Box host | Wake | No | `fritzbox.network.sub_features.wake_on_lan` on, network not read-only |

Stop, restart, shut down and reboot ask you to confirm first ("Confirm Stop for web?"), and the server enforces that too.
AuraGo-managed containers (for example go2rtc, RuView, Security Proxy, Decisions, local LLM, ACE-Step, Garage, Homepage,
CYD Creator, Ollama, Piper, Browser Automation and AuraGo itself) are shown, but Infra cannot start, stop, restart or rename them.

Each action ends with one of these outcomes:

- **Running**: the request is still in progress.
- **Accepted**: Proxmox accepted the task, or the Wake-on-LAN packet was sent. The device may still be starting, and
  Infra does not confirm that.
- **Completed**: Docker read the container back, and the new state is confirmed.
- **Failed**: Infra did not run the action, or the source rejected it.
- **Unknown outcome**: the request may or may not have reached the source.

After an unknown outcome, Infra blocks that action on that entry. Check the device in its own tool, then click **I checked
the device state · allow one new attempt**. Infra never retries an uncertain action by itself. A window reload clears
the block, so check the device again after a reload. If a permission is revoked, unsent actions are cancelled; a sent
action cannot be recalled.

**Audit.** Infra writes no log entry for actions and keeps no separate audit trail. The action ledger stores the request
ID, the action, the outcome and, for Proxmox, the task ID, but not the user. Use the source's own logs to see who did what.

---

## Good to Know

- **One map for all administrators.** Map, positions and relations are shared by every administrator on this instance.
  Zoom, the selected view and open windows belong to each window.
- **Power is the latest observation.** A stale source keeps no power reading. For FRITZ!Box, *Running* means the host was
  active on the LAN, not that Wake-on-LAN has finished.
- **Missing is not proven gone.** Unreachable or partial sources make entries *Stale*; only a complete answer marks one
  *Removed*.

**Limits.** Infra enforces these maximums:

| Item | Limit |
|---|---|
| Entry name | 200 characters |
| Hardware text fields (vendor, model, OS and so on) | 256 characters each; notes 8 KiB |
| Tags | 64 per entry, 64 characters each |
| Interfaces, links, app links | 32, 32 and 64 per entry |
| Custom fields | 64 per entry, values up to 2048 bytes |
| Entries one source may report per refresh | 4096 (more keep that source stale) |

---

## Privacy and Safety

**Stored where.** The map lives in the inventory database `data/inventory.db` (config key `sqlite.inventory_path`), in
`infra_*` tables next to the device registry. Before the first schema migration, AuraGo copies the database to
`inventory.db.infra-v<version>-<random>.bak` and keeps the three newest copies.

**What is polled.** Opening Infra shows the stored map at once. In the background it reads only the configured sources:
the device registry (local) and, when enabled, Docker, Proxmox and FRITZ!Box Hosts. A visible window refreshes about every
30 seconds; **Refresh** waits for fresh results. Hidden windows stop asking. Infra runs no network scans.

**What leaves the machine.** Infra has no external service of its own. The only traffic is the calls the Docker, Proxmox
and FRITZ!Box integrations already make to the endpoints you configured. Infra keeps a reduced copy of what they report:
names, types, state, addresses, hardware figures, power state, tags, container image names, Proxmox VM IDs and nodes.
Credentials, raw provider responses and Docker environment variables are not stored. Links must be HTTP or HTTPS
addresses without a user name or password.

**Credentials and access.** A registry device stores only a credential reference. The secret stays in the Vault and in the
integration's own settings. Only administrators can use Infra. The agent has no Infra tool and cannot read or change the
atlas; its Docker and Proxmox tools keep their own settings ([Chapter 8](./08-integrations.md)).

---

## When Something Goes Wrong

| Message | What helps |
|---|---|
| "Infra is disabled or unavailable. An administrator can enable it in Desktop settings." | Only administrators can open Infra. An administrator turns on **Enable Virtual Desktop** and **Enable Infra** under **Config → Virtual Desktop**. The message says "Desktop settings", but the switches are there. |
| "Infra is read-only." | An administrator clears **Read-only** under **Config → Virtual Desktop** and checks that **Read-only mode** is off. |
| "Infra storage is unavailable." | The inventory database cannot be used. An administrator checks `sqlite.inventory_path` and the AuraGo log. |
| "This asset changed elsewhere. Your draft is still here; refresh to review the latest version." | Click **Refresh**, compare under **Review current asset values**, then choose **Keep and reapply my draft**. |
| "This asset was removed elsewhere. Your local draft is still preserved." | Discard the draft, or create the entry again with the same details. |
| "The source did not confirm the change. Your draft is preserved; review the source before trying again." | Check the value in Docker, Proxmox or FRITZ!Box, then click **Review current source** before saving again. |
| "The source change is still pending. Your draft remains here." | Wait until Proxmox confirms the change, then click **Check pending source**. |
| "The action result is unknown. Check the device state before trying again." | Check the device in its own tool, then click **I checked the device state · allow one new attempt**. |
| "Source editing is disabled." | An administrator turns on **Allow source edits**. |
| "Control actions are disabled." | An administrator turns on **Allow device control**. |
| "This source is disabled." | The integration behind the source is switched off in its own configuration. |
| "The source policy blocks this action." | For a Proxmox shutdown or reboot, an administrator sets `proxmox.allow_destructive: true`. |
| "This container is managed by AuraGo and cannot be renamed here." | AuraGo manages this container, so leave its name alone. The same text appears when a power action on it is blocked. |
| "No import candidates found" | The configured sources have nothing new. Check that the source is enabled and its entries have readable names. |
| "Move or remove child assets first." | The entry still has places or entries inside it. Move or remove them first. |
| "Infra could not complete the request." | Generic message for error codes without their own text, such as a refused action. Check the switches above and try again. The [operator reference](../../infra.md) lists the codes. |

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 8: Integrations](./08-integrations.md) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 28: Mailer](./28-mailer.md)
**Next Chapter:** [Chapter 30: RuView](./30-ruview.md)
