# Chapter 29: Infra – the homelab map

Infra is a shared map of your homelab. It shows hardware, virtual machines, containers, services and network segments
as a map, a topology diagram or a list, and links each asset to its source: the device registry, Docker, Proxmox or
FRITZ!Box. It is for administrators who want one picture of their setup. It does not scan your network or monitor
devices, and it changes a device only when you ask it to.

Technical details for administrators (configuration reference, API routes, error codes, development notes) are in the
[operator reference](../../infra.md).

## Table of Contents

1. [What You Need and Switching It On](#what-you-need-and-switching-it-on)
2. [First Steps](#first-steps)
3. [The Infra Window](#the-infra-window)
4. [Import Discoveries](#import-discoveries)
5. [Build the Map](#build-the-map)
6. [Topology and List](#topology-and-list)
7. [Cleanup and Ignore](#cleanup-and-ignore)
8. [Edit Assets and Save](#edit-assets-and-save)
9. [Source Edits](#source-edits)
10. [Power Actions](#power-actions)
11. [Keyboard Shortcuts](#keyboard-shortcuts)
12. [Good to Know](#good-to-know)
13. [Privacy and Safety](#privacy-and-safety)
14. [When Something Goes Wrong](#when-something-goes-wrong)

---

## What You Need and Switching It On

Infra is for administrators only: every Infra request needs a Desktop administrator session. Two switches must be on:

- **Config → Virtual Desktop → "Enable Virtual Desktop"** (`virtual_desktop.enabled`; the shipped `config_template.yaml` turns it on, a config without
  the key leaves it off).
- **Config → Virtual Desktop → "Enable Infra"** (`virtual_desktop.infra.enabled`, on by default). It sits in the
  **Infra** group of that page. Switching it off closes Infra for everyone; the stored map stays on disk.

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
3. An empty map shows **Start your homelab map** and three steps: connect your sources, import the discoveries you
   want to manage, and drag assets from the library onto the map. Its two buttons are **Import discoveries…** and
   **New asset**. Start with the import for devices that already exist.

---

## The Infra Window

The window has a command bar across the top, the **Library** on the left, the map in the middle and the details panel
on the right.

- **Command bar:**
  - View switch: **Map**, **Topology** and **List**.
  - Search: **Search or jump…** (Ctrl+K). Below 560 px the search collapses to an icon button that opens the field over
    the bar.
  - **Filters** by type, source and power. Active filters show as removable chips. When there are too many for the bar,
    a **+N** chip opens the rest.
  - The status dot: **Live**, **Syncing…**, **Not up to date** or **Paused**.
  - **Undo** and **Redo**.
  - **New asset**, with a menu next to it that offers **New place** and **Import discoveries…**.
  - **⋯** (More actions): **Refresh**, **Arrange** (**Arrange automatically** in Topology), **Keyboard shortcuts** and
    **Legend**. The commands show icons, shortcut hints on the right and separators between groups. On the map, **Arrange**
    reads **Arrange selection** when several assets of one place are selected, and **Arrange whole map** otherwise. In the
    List, **Arrange** and **Legend** are disabled, with the reason "Only in Map and Topology".
- **Library sections:** **To place** (hidden in Topology), **On the map** (**In topology** in Topology),
  **Needs attention**, **Cleanup** and **Ignored**. The **+** next to **On the map** creates a place.
- **Map tools:** **Select (V)**, **Pan the map (H)** and **Connect assets (C)**. **Zoom in**, **Zoom out** and
  **Reset zoom** (the percentage) control the zoom; at 25% and 200% the zoom buttons are disabled, and a zoom key reports
  the limit once. **Fit to window (F)** frames everything. The **Overview map**
  shows where you are on a large map. It hides itself while the whole map fits on screen; its switch then only shows a
  message: "The whole map is visible. The overview map appears when you zoom in." While a tool or the keyboard move mode is
  active, a small banner at the top centre of the map names it, for example "Drag from asset to asset · Esc ends".
- **Details panel:** opens when you select an asset and closes with Esc or its close button. If the panel would cover the
  selected asset, the map moves just far enough to keep it in view.
- **Narrow windows:** below 760 px the library opens over the map from its button in the command bar. Below 1100 px the
  details panel slides over the map.
- **Layout preferences:** library and details panel width, the collapsed library, the Overview map, zoom and the chosen
  tab are remembered in this browser.

---

## Import Discoveries

Infra never adds an asset by itself. The **Import discoveries…** sheet opens beside the map and is titled **Import**. It lists new discoveries from these
sources:

| Source | Available when |
|---|---|
| Device registry | Always, from the inventory database |
| Docker | `docker.enabled` is on |
| Proxmox | `proxmox.enabled` is on |
| Fritz!Box Hosts | FRITZ!Box is enabled with a host, and its network Hosts feature is on |

Other integrations (Home Assistant, TrueNAS, Tailscale) do not feed Infra; see [Chapter 8: Integrations](./08-integrations.md).
Several Docker engines are told apart by the name of the host device, for example **Docker · aurago-test**, once that name
is known. Otherwise they count as **Docker 1** and **Docker 2**. Proxmox and FRITZ!Box hosts keep their numbers.

- **Groups:** discoveries are grouped into **Recommended** (selected when the sheet opens), **Possible duplicates** and
  **Probably not relevant**. Each row shows its reason, for example **Same address as pve-01** or **Not active right now**.
  A group without rows is not shown.
- **Rows:** each row starts folded, with its checkbox, icon, name and one line of details. Its expand button shows the
  link choice and the **Import** and **Ignore** buttons. Possible duplicates and rows with an error start expanded.
- **Row actions:** **Import** imports one row. **Link to existing** links it to an asset you already have instead; the
  default is **Import as new asset**. **Ignore** hides a row. With rows ticked, **Import selected** and **Ignore
  selected** act on all of them.
- **Load errors:** if the discoveries cannot be loaded, the sheet shows "Discoveries could not be loaded" with a **Retry**
  button. The "no new discoveries" message appears only after a successful load that found nothing.
- **Partial import:** if some items cannot be imported, a message gives the count, for example "1 discovery could not be
  imported".
- **Drag to import:** drag a discovery onto the map to import and place it in one step. A discovery dropped into a place
  joins that place. A VM or container is imported into its host.
- **Hidden names:** Registry and FRITZ!Box discoveries whose name is empty, a default (`unnamed device`, `Fritz!Box host`)
  or only a hex ID, UUID or MAC address stay hidden until the source gives them a readable name.
- **Manual asset:** **New asset** opens a dialog titled **New asset** (**New place** for a place). Enter a name, a type and
  an icon, then choose **Create**. Without a name the dialog says "Enter a name" under the field and moves the cursor there.
  A manual **Device** also creates a registry row with protocol `none`; add SSH, VNC or credentials there, on purpose. The
  other types create no registry devices. A click beside the dialog closes it only while nothing has been typed; Esc always
  closes it. When Infra is read-only, **Create** is disabled and the dialog shows "Read-only access: saving is not possible
  right now".

---

## Build the Map

- **Placing:** drag rows from **To place** onto the map, or select a row and press **P** to place it at the centre of
  the map. For an unplaced asset without a host, the **Overview** tab of the details panel offers **Place on map**.
  While you drag over a place, the region shows **Drop into Server room**. Drag an asset out of a place to clear its place,
  or use **Move to place…** in the context menu.
- **Dragging:** while you drag, a dashed outline shows where the asset will land; it snaps to the 24 px grid. Dropping a row
  on the sidebar cancels the drag, and Infra announces "Drag cancelled". Dragging an asset or a selection to the edge of the
  map moves the map in that direction.
- **Places:** sites, rooms, racks and groups. Move a place by its header and resize it from its edges. **New place**
  creates one, as does the **+** next to **On the map**. Messages call a place a "place" too, for example "Place created".
- **Guests:** a VM or container sits inside its host. The badge on a host card (for example **2 guests · 1 running**)
  opens its guest shelf. A host with no running guests reads **2 guests · stopped**, and hovering the badge lists the
  first guest names.
- **Connections:**
  - Hover a card and drag one of its handles onto another card, or use the **Connect assets (C)** tool.
  - The **New connection** popover suggests **Ethernet**, or **Wi-Fi** for an access point or mesh node, and **Depends on**
    when one end is a service.
  - The connection types are **Ethernet**, **Fiber**, **Wi-Fi**, **Member of**, **Runs on**, **Depends on** and
    **Powered by**. A shared subnet is not a cable.
  - Selecting a card highlights its connections and fades the others. In Topology, **Depends on** and **Powered by**
    connections appear only while one of their ends is selected or hovered. The details panel's **Connections** tab
    lists all of them.
  - If the asset is the target of a directed connection, its row names the reverse direction: **Hosts**, **Required by**,
    **Powers** or **Contains**.
  - A connection reported by a source cannot be edited or removed. Its row in the **Connections** tab says
    "Reported by Docker" instead.
- **Selection:** Shift-click or Ctrl-click adds an asset to the selection or removes it again, and Shift-drag across empty
  map selects an area. Drag one selected card to move the whole selection, or press **M** to move it with the arrow keys.
- **Undo:** undo and redo hold 50 steps per view. Placing from the library and Arrange can be undone. The buttons name the
  next step, for example "Undo: Move (Ctrl+Z)". If a save conflict makes earlier steps impossible to undo, Infra says so:
  "Earlier steps can no longer be undone".
- **Context menu:** right-click, or Shift+F10, opens it: **Show details**, **Move to place…**, **Connect to…**,
  **Open in Quick Connect**, the asset's power actions as their own group, **Remove from map** (in Topology: **Place
  automatically**), **Remove from Infra** (its shortcut hint is Delete) and, for assets with sources, **Remove and ignore**.
- **Positions not saved:** Infra retries a conflicting save once on its own. If it still fails, the cards stay marked
  behind **Positions not saved** with a **Retry** button. Click **Retry**; if that keeps failing, click **Refresh**.

---

## Topology and List

**Topology** draws four full-width lanes: **Internet**, **Network**, **Servers and storage** and **Clients and IoT**. The lane
labels stay at the left edge of the view while you pan. Assets land in lanes like this:

- **Internet:** modems, ONTs, internet gateways and cloud services.
- **Network:** routers, firewalls, VPN endpoints, switches, access points, mesh nodes and LAN, VLAN and WLAN segments.
- **Servers and storage:** servers, NAS and storage shelves, mini PCs, hypervisors, Kubernetes, databases, network video recorders,
  UPS and PDUs. An asset that runs guests also lands here when no other rule matches.
- **Clients and IoT:** everything else, such as cameras, printers, TVs, phones, tablets, laptops and sensors.
  Single-board computers follow their role or their guests; without either they land here.

The lanes are assigned automatically. Drag a card into another lane to keep it there.

- **Auto layout:** assets without a topology position are laid out automatically, with connected assets placed close
  together. Every administrator sees the same layout, and opening Topology saves nothing. Only your own moves and
  arrangements are saved.
- **Arrange automatically** hands the selection, or every card, back to the automatic layout. It is one step, and you
  can undo it.
- **Place automatically:** the context menu's entry for one card hands it back to the automatic layout. Guests stay
  inside their host.
- **Lines:** connections between lanes are curves. A connection inside a lane that skips over a card arcs above the row.
- **Zoomed out:** cards turn compact and show only the icon and the name. A long name stays on one line and ends in an
  ellipsis; the full name is in the tooltip. A name is hidden only where it would cover another card's icon, and it shows
  again on hover, focus or selection.

**List** shows the same assets as a sortable table. On narrow windows it hides columns instead of scrolling sideways: first
**Power**, then **Hardware**, **Source**, **Place**, **Type** and **Address**, and last **Status**. The name column always stays.

- Click a column header to sort by it (**Sort by Name**). Every column sorts, including **Hardware**. The columns are
  **Name**, **Type**, **Place**, **Address**, **Hardware**, **Source**, **Status** and **Power**. Truncated text carries the
  full text as a tooltip.
- The **Status** column shows reachability: **Live**, **Offline**, **Unknown**, or a dash when there is no signal. A stale or
  removed source also gets a small "!" warning.
- A click anywhere in a row opens the asset in the details panel, except on the checkbox, the expander and links. A double
  click on an unplaced asset does the same.
- The list has one Tab stop. Up and Down, Home and End move between rows, Space toggles the checkbox, and Enter opens the
  asset. Right expands a host and Left collapses it (Left on a guest goes to its host).
- Hosts expand to show their guests (**Show guests of pve-01**).
- Ticking rows opens the bulk bar, **Actions for the selection**, with **Move to place…**, **Remove and ignore**,
  **Remove from Infra** and **Clear selection**.

---

## Cleanup and Ignore

- **Needs attention:** this section lists sources that are stale, unknown or no longer reported, and unsaved work, such
  as a position that was not saved or a draft that conflicts with a newer version.
- **Cleanup suggestions:**
  - possible duplicates: **Remove duplicate and ignore**;
  - Docker guests without a host: **Assign engine host**;
  - other guests without a host: **Choose host…**;
  - assets with unreadable names: **Rename**.
- **Dismissing:** the **Dismiss** button hides a suggestion. **Show 3 dismissed suggestions** brings them back.
- **Remove and ignore:** deletes the asset and ignores its sources. They are not discovered again until you restore them
  under **Ignored**, where **Restore** puts a source back into discovery.
- **Linked sources:** a source that is still linked to an asset cannot be ignored. Unlink or remove the asset first.

---

## Edit Assets and Save

Select an asset to open its details panel. The header shows the icon (**Change icon**), the name (click it to rename
it), the place breadcrumb, the state and power, and the primary action: **Open in Quick Connect** when the asset has a linked
app, otherwise **Edit**.

The details panel has four tabs: **Overview**, **Details**, **Sources** and **Connections**.

- **Details groups:** **Identity**, **Network**, **Hardware**, **Links and apps**, **Notes** and **Custom fields**.
  Empty groups start closed.
- **Save bar:** when something changes, the **Unsaved changes** bar appears with **Discard** and **Save in Infra**
  (or Ctrl+S). Changed fields are marked.
- **Guests in the Overview tab:** a host lists its guests with their power state. After eight guests, **+3 more**
  reveals the rest in the library.
- **Validation:** invalid CIDR, link, IP, MAC or VLAN values and missing required fields are explained under the field once
  you leave it or try to save. **Save in Infra** stays enabled while something has changed. A save attempt with an invalid field
  moves the cursor to the first one and saves nothing.
- Leaving an asset or closing the details panel with unsaved changes keeps the draft. Infra says so once: "Draft for TheBrain
  is kept".
- **App links:** **Link an app…** opens a search. **MeshCore** conversations appear only for radio and mesh assets, or
  after you type two characters.
- **Conflicts:** if someone saved the same asset meanwhile, Infra refuses your save and keeps your draft. Click
  **Refresh**, compare under **Review current asset values**, and choose **Reapply my draft** if your version
  should win.
- **Remove from Infra** deletes only the Infra asset. The source device, container or VM stays, and so does a registry
  device created by a manual asset; delete that in the device registry if you no longer need it. An asset with children
  cannot be removed until they are moved or removed.
- **Closing Infra** with unsaved changes asks whether to discard them (**Discard**) or keep editing.

---

## Source Edits

Source edits change metadata that the source stores itself. You need **Allow source edits** and a writable source. On
the **Sources** tab of the details panel, each source row has an **Edit source** button where the source allows it. The
form shows only the fields the source supports:

| Source | Editable fields |
|---|---|
| Device registry | Name, type, protocol (*None*, *SSH*, *VNC*), IP address, port, username, credential (picked from the list, or **No credential**), description, tags, MAC address |
| Docker | Container name. Docker renames the container; it is not recreated. |
| Proxmox | Name for a VM, hostname for a container (LXC), description, tags |
| Fritz!Box Hosts | Friendly name |

**Save source metadata** writes the change and reads it back to confirm it. A Proxmox change may stay *pending* until
Proxmox confirms it. Until then the draft stays in place and Infra says so. Click **Check pending source** once Proxmox
has confirmed it. Proxmox changes carry a config digest, so an edit made elsewhere is refused, not overwritten.
AuraGo-managed containers cannot be renamed here.

---

## Power Actions

Power actions send one explicit request to a source. On the **Sources** tab, open the source row and choose
**Infrastructure actions**. The dialog is titled **Source actions**.

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

The **Completed** and **Failed** outcomes also appear as a short message that names the device and the action, for example
"web: Restart completed" or "web: Restart failed".

Each action ends with one of these outcomes:

- **Running**: the request is still in progress.
- **Accepted**: Proxmox accepted the task, or the Wake-on-LAN packet was sent. The device may still be starting, and
  Infra does not confirm that.
- **Completed**: Docker read the container back, and the new state is confirmed.
- **Failed**: Infra did not run the action, or the source rejected it.
- **Unknown outcome**: the request may or may not have reached the source.

After an unknown outcome, Infra blocks that action on that asset. Check the device in its own tool, then click **I checked
the device state · allow one new attempt**. Infra never retries an uncertain action by itself. A window reload clears
the block, so check the device again after a reload. If a permission is revoked, unsent actions are cancelled; a sent
action cannot be recalled.

**Audit.** Infra writes no log entry for actions and keeps no separate audit trail. The action ledger stores the request
ID, the action, the outcome and, for Proxmox, the task ID, but not the user. Use the source's own logs to see who did what.

---

## Keyboard Shortcuts

Ctrl is ⌘ on a Mac. The shortcut list is also available from **⋯** → **Keyboard shortcuts**.

| Keys | Action |
|---|---|
| Ctrl+K | Search or jump to an asset |
| Ctrl+Z / Ctrl+Shift+Z (or Ctrl+Y) | Undo or redo |
| Ctrl+S | Save the open details |
| Esc | Cancel or close. With no gesture in progress, Esc first returns to the **Select** tool, then clears the selection |
| ? | Show this list |
| Arrow keys | Move between assets on the map |
| Enter | Select the focused asset and open its details |
| Space | Add the focused asset to the selection, or remove it |
| M | Move the selection with the arrow keys: 24 px, or 8 px with Shift. Enter saves, Esc cancels. For a place, R switches between moving and resizing |
| P | Place a library row at the map centre |
| Ctrl+A | Select all visible assets |
| Delete | Remove the selection from Infra. With a connection selected and no asset, Delete removes the connection |
| Shift+F10 or Menu key | Open the context menu |
| F | Fit the map to the window |
| + / - / 0 | Zoom in, zoom out or reset to 100 % |
| V / H / C | Select, pan or connect tool |
| Shift+drag | Select an area |
| Space+drag, middle mouse button | Pan the map |
| Ctrl+wheel, pinch | Zoom at the pointer |
| Up and Down, Home, End (List) | Move between rows of the List |
| Right and Left (List) | Expand or collapse a host |
| Space and Enter (List) | Toggle the row's checkbox, or open the asset |

---

## Good to Know

- **One map for all administrators.** The map, its positions and its connections are shared by every administrator on
  this instance. Zoom, the selected view and open windows belong to each window.
- **Positions are shared and saved per view.** The Topology layout is calculated the same way for everyone, so everyone
  sees the same picture. Opening Topology saves nothing.
- **Power is the latest observation.** A stale source keeps no power reading. For FRITZ!Box, *Running* means the host was
  active on the LAN, not that Wake-on-LAN has finished.
- **Missing is not proven gone.** Unreachable or partial sources make assets *Stale*; only a complete answer marks one
  *Removed*.
- **Layout preferences stay in this browser.** Panel widths, zoom, the chosen tab, the List sort and dismissed
  suggestions are not shared.

**Limits.** Infra enforces these maximums:

| Item | Limit |
|---|---|
| Asset name | 200 characters |
| Hardware text fields (vendor, model, OS and so on) | 256 characters each; notes 8 KiB |
| Tags | 64 per asset, 64 characters each |
| Interfaces, links, app links | 32, 32 and 64 per asset |
| Custom fields | 64 per asset, values up to 2048 bytes |
| Assets one source may report per refresh | 4096 (more keep that source stale) |

---

## Privacy and Safety

**Stored where.** The map lives in the inventory database `data/inventory.db` (config key `sqlite.inventory_path`), in
`infra_*` tables next to the device registry. Before the first schema migration, AuraGo copies the database to
`inventory.db.infra-v<version>-<random>.bak` and keeps the three newest copies. Ignored sources stay stored, so they are
not discovered again.

**Stored in the browser only.** View preferences (panel widths, zoom, chosen tab, List sort, dismissed suggestions)
are stored only in the browser, never on the server.

**What is polled.** Opening Infra shows the stored map at once. In the background it reads only the configured sources:
the device registry (local) and, when enabled, Docker, Proxmox and FRITZ!Box Hosts. A visible window refreshes about every
30 seconds; **Refresh** waits for fresh results. Hidden windows stop asking. Infra runs no network scans.

**What leaves the machine.** Infra has no external service of its own. The only traffic is the calls the Docker, Proxmox
and FRITZ!Box integrations already make to the endpoints you configured. Infra keeps a reduced copy of what they report:
names, types, state, addresses, hardware figures, power state, tags, container image names, Proxmox VM IDs and hosts.
Credentials, raw provider responses and Docker environment variables are not stored. Links must be HTTP or HTTPS
addresses without a user name or password.

**Credentials and access.** A registry device stores only a credential reference. The secret stays in the Vault and in the
integration's own settings. Only administrators can use Infra. The agent has no Infra tool and cannot read or change the
map; its Docker and Proxmox tools keep their own settings ([Chapter 8](./08-integrations.md)).

---

## When Something Goes Wrong

| Message | What helps |
|---|---|
| "Infra is disabled or unavailable. An administrator can enable it under Configuration → Virtual Desktop → Infra." | Infra is switched off. An administrator turns on **Enable Virtual Desktop** and **Enable Infra** under **Config → Virtual Desktop**. |
| "Infra is available to administrators only." | Your browser session has expired, or the API token in use lacks the `desktop:admin` scope. Sign in again, or use a token with that scope. |
| "Infra is read-only." | An administrator clears **Read-only** under **Config → Virtual Desktop** and checks that **Read-only mode** is off. |
| "Infra storage is unavailable." | The inventory database cannot be used. An administrator checks `sqlite.inventory_path` and the AuraGo log. |
| "This asset changed elsewhere. Your draft is still here; refresh to review the latest version." | Click **Refresh**, compare under **Review current asset values**, then choose **Reapply my draft**. |
| "This asset was removed elsewhere. Your local draft is still preserved." | Discard the draft, or create the asset again with the same details. |
| "The source did not confirm the change. Your draft is preserved; review the source before trying again." | Check the value in Docker, Proxmox or FRITZ!Box, then click **Review current source** before saving again. |
| "The source change is still pending. Your draft remains here." | Wait until Proxmox confirms the change, then click **Check pending source**. |
| "The action result is unknown. Check the device state before trying again." | Check the device in its own tool, then click **I checked the device state · allow one new attempt**. |
| "Source editing is disabled." | An administrator turns on **Allow source edits**. |
| "Control actions are disabled." | An administrator turns on **Allow device control**. |
| "This source is disabled." | The integration behind the source is switched off in its own configuration. |
| "The source policy blocks this action." | For a Proxmox shutdown or reboot, an administrator sets `proxmox.allow_destructive: true`. |
| "This container is managed by AuraGo and cannot be renamed here." | AuraGo manages this container, so leave its name alone. The same text appears when a power action on it is blocked. |
| "No new discoveries" | The configured sources have nothing new right now. Check that the source is enabled and its assets have readable names. |
| "Move or remove child assets first." | The asset still has places or assets inside it. Move or remove them first. |
| "Positions not saved" | Click **Retry**. If it keeps failing, click **Refresh**. |
| "Infra could not refresh. You are seeing the last known state." | Infra keeps showing the last known state and retries on its own. Click **Refresh** to ask for fresh results now. |
| "This connection already exists." | Choose a different second asset or a different connection type. |
| "Infra could not confirm the change. The latest state was reloaded; check it before you try again." | Check the asset's current state, then try again. |
| "This source is already linked to an Infra asset." | Unlink or remove the asset instead. |
| "Earlier steps can no longer be undone" | A save conflict reloaded the map. New steps can be undone again. |
| "Could not copy" | The browser refused the copy. Select the value and copy it by hand. |
| "Discoveries could not be loaded" | Click **Retry** in the import sheet. |
| "Read-only access: saving is not possible right now" | The dialog is read-only. An administrator clears **Read-only** under **Config → Virtual Desktop**. |
| "Infra could not complete the request." | Generic message for error codes without their own text, such as a refused action. Check the switches above and try again. The [operator reference](../../infra.md) lists the codes. |

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 8: Integrations](./08-integrations.md) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 28: Mailer](./28-mailer.md)
**Next Chapter:** [Chapter 30: RuView](./30-ruview.md)
