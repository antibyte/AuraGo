# Chapter 30: RuView – Wi-Fi Sensing (Experimental)

RuView is AuraGo's experimental Wi-Fi sensing app. It wraps the upstream RuView v3122 project by ruvnet, with its
Dashboard, Observatory, Pose fusion, calibration, recording and training views. It reads RSSI, the signal strength of
a connected host Wi-Fi adapter, or CSI (channel state information) from ESP32 boards. Use it to experiment. It is not
a validated presence or pose sensor: a connected service proves nothing about accuracy, and a simulation is not a sensor.

Technical details for administrators (runtimes, network rules, storage layout, artifact pins) are in the
[operator reference](../../ruview.md). The short administrator summary is in
[Chapter 8: Integrations](08-integrations.md#ruview-wi-fi-sensing).

## Table of Contents

1. [What You Need and Switching It On](#what-you-need-and-switching-it-on)
2. [First Steps](#first-steps)
3. [Choosing a Runtime](#choosing-a-runtime)
4. [Reading Status and Boards](#reading-status-and-boards)
5. [Setting Up an ESP32 Board](#setting-up-an-esp32-board)
6. [The Lab](#the-lab)
7. [Recordings and Storage](#recordings-and-storage)
8. [Good to Know](#good-to-know)
9. [Privacy and Safety](#privacy-and-safety)
10. [When Something Goes Wrong](#when-something-goes-wrong)

---

## What You Need and Switching It On

RuView is in **Config → RuView**, in the group **Network & Remote**. The `ruview:` block in `config_template.yaml`
holds these settings:

| Setting (UI label) | Config key | Default | What it does |
|---|---|---|---|
| **Enable integration** | `ruview.enabled` | off | Turns RuView on. Save after changing it. |
| **Runtime** (*Managed Docker*, *External instance*, *Native host runtime*) | `ruview.mode` | `docker` | Where the RuView service runs. See [Choosing a Runtime](#choosing-a-runtime). |
| **Source** (*ESP32 CSI*, *Host Wi-Fi RSSI*, *Explicit demo*) | `ruview.source` | `esp32` | Where the data comes from. Host Wi-Fi is not offered with Managed Docker. |
| **Read only** | `ruview.readonly` | on | Blocks starts, changes and deletions. Administrators can still stop services, recordings and training. |
| **Allow agent read access** (under **Permissions**) | `ruview.agent_access` | off | Lets the agent read status and measurements. See [Privacy and Safety](#privacy-and-safety). |

Changes take effect after **Save**. Enabling RuView does not need an AuraGo restart. At startup, AuraGo starts an
enabled and writable managed runtime in the background. During a running session, press **Start service** in the app instead.
AuraGo never starts an external instance.

**Read only** is on by default, so nothing can start or change until you switch it off and save. While it is on, the
form shows: "Read only blocks starts, changes and deletions. Administrators can stop the service, recording and
training." **Test connection** works in read-only mode. It reports success only when fresh sensor data arrives. A connected service that has not sent its first frame shows *Waiting for first data* as a warning.

Under **Connection**, a collapsed section holds **HTTP port** (`ruview.http_port`, default 3022, bound to loopback) and
**Storage budget (MiB)** (`ruview.storage_mb`, default 1024).

## First Steps

1. Open **Config → RuView**, switch on **Enable integration**, pick a **Runtime** and a **Source**, and click **Save**.
2. For a managed runtime, switch off **Read only** and save again.
3. Click **Open RuView**. The app opens in a new tab. It is also listed in the start menu under **Communication**.
   Opening the app does not start the service. While the **Boards** tab is open, AuraGo asks the service for its board list every three seconds. The **Status** view asks only for a managed ESP32 service, and a managed service that is not running is not contacted.
4. In the app's **Status** view, click **Start service** for a managed runtime. On a native host, click **Install** first.
   The headline reads *RuView is running and waiting for data from the boards.* once the runtime answers, and starts with *Working:* once data arrives.

## Choosing a Runtime

| Runtime | Use it for | Notes |
|---|---|---|
| **Managed Docker** | ESP32 CSI on a Linux sensing server, or a demo | AuraGo pulls the pinned image, runs its own labelled container and binds it to loopback. Docker must not be read-only. |
| **Native host runtime** | Host Wi-Fi RSSI on Windows or macOS | Click **Install** once, or **Install service** in the ⋯ menu. This release has a verified package for Windows (amd64) only, so the install stops on macOS. |
| **External instance** | A RuView service you run yourself | Enter the **Service URL** as a bare origin, for example `http://192.168.1.20:3000`, with no path, query or login. Private or loopback addresses need **Allow private network access**. AuraGo never installs, starts, stops or upgrades it. |

A container cannot see the host's Wi-Fi adapter. If AuraGo runs in a container, the configuration check rejects the
native runtime. Run RuView as an external instance on the computer with the adapter instead. Host Wi-Fi RSSI needs
Windows or macOS. On Linux, use ESP32 boards. **Explicit demo** is for trying the app. Nothing falls back to it
automatically.

## Reading Status and Boards

The command bar has three views: **Status**, **Boards** and **Lab**. **Boards** appears only for the ESP32 CSI source.
**Lab** appears while RuView is turned on. In a narrow window, the views move into a menu.

**Status** answers one question first: is it working? Read the headline first. It is one sentence. Below it, you find:

- **The next step**, as one button, when there is one. If the button is greyed out, the line below it says why.
- **Setup**, for a new installation. It lists what is still missing: *Turned on*, *Changes allowed*, *Service running*,
  *Receiver and UDP set*, *First board* and *Data arriving*. Each open step has its own button. The list disappears
  once real sensor data has arrived.
- **Details**, collapsed by default. It shows the service and its revision, the source, the storage used against the
  budget, the last recording action that this AuraGo session confirmed, and any open hints. **Copy diagnostics** copies
  the same facts as plain `key=value` lines, without tokens, URLs or file paths.

The pill in the command bar repeats the state in words, with a symbol and a colour: *Live*, *Waiting*, *Problem*,
*Read-only* and so on.

Install is offered only for the native host runtime. A managed service that is not running shows *The RuView service
isn't running.* and offers **Start service**. The line below says why it stopped, for example *Stopped by you at
14:02.* Opening the app never starts a service.

Status never contacts an external service in the background. For an external instance, click **Check now**, in the
**⋯** menu or as the next step. The check result appears in the headline. Ten seconds later, unless the check failed,
the headline says when the service was last checked and offers **Check now** again.

A connected service that has not sent its first frame shows *RuView is running and waiting for data from the boards.*
That is a waiting state, not a success. A stop that could not be confirmed, or a service that ended on its own, stays
shown until you click **Start service** or **Stop service** again.

**Allow changes…** appears when read-only blocks a start. After you confirm, AuraGo saves only the read-only switch,
through the normal administrator settings save. Nothing else is sent. On a read-only desktop, the button opens the
settings instead.

**Boards** lists every board AuraGo knows and reads that list every three seconds while the tab is visible. **Last
measurement** shows the age of the last value. Values older than ten seconds are marked stale. Quiet boards stay in the
list. **Set up this board** opens the board wizard for that node ID, and **Add board** opens it for a new ID. An empty
list means that boards appear again when they next send data, so flashing them again is not needed. If the list cannot
be read, the tab says why. For a managed runtime that is not running, it reads "The RuView service is not running. Saved
boards are shown without live data." The Boards tab is hidden for Wi-Fi and demo sources.

## Setting Up an ESP32 Board

Before you start:

- Open AuraGo over HTTPS or `localhost` in Chrome or Edge. Firefox and Safari have no Web Serial.
- Plug the board into the computer that runs the browser, using a cable that carries data. Close other programs that
  use the same serial port.
- Set **Source** to **ESP32 CSI** and switch off **Read only**. The board section is hidden for other sources.

Open the wizard with **Configure board** in the configuration, with **Add board** on the **Boards** tab, or with
**Set up this board** on a known board.

**Step 1: Write firmware.** Choose the **Board profile** that matches the chip, the flash size and the USB connection.
Use `cdc` for "USB JTAG/serial debug unit" and `uart` for a USB-UART bridge. An ESP32-S3-WROOM-1 N16R8 has 16 MB flash
and 8 MB PSRAM, so pick a 16 MB profile. The firmware does not use PSRAM. Tick **Erase all settings and recordings on
this board** only when the dialog asks for it. That happens only for a different partition layout. A new, blank chip is written without erasing. The label mentions recordings,
but recordings made by AuraGo are stored on the host, not on the board. Click **Write firmware**. The dialog shows each
phase: file checks, connection, erase, writing with a percentage, verification and restart. When it reports
**Step 1 complete**, click **Continue to step 2**.

**Step 2: Set up Wi-Fi and receiver.** Enter the **Wi-Fi network (SSID)** and the **Wi-Fi password**, or tick **Keep
current Wi-Fi settings** to keep the board's existing ones. Choose a **Node ID** from 1 to 255. The dialog suggests the
lowest free ID among the boards AuraGo knows. The suggestion reserves nothing, and boards AuraGo has never seen cannot
be counted. Enter the **RuView receiver IPv4** and the **UDP port** (default 5005), then click **Configure board**.
After the board restarts, reconnect it and click **Confirm after restart**.

The receiver is the address of the RuView machine, reachable by the board over Wi-Fi. With managed Docker, use the LAN
address of the Docker host, not a container address. **Use LAN address** (with the detected IP) fills empty fields
only. It does not enable UDP or save anything.

**Step 3: Check sensor data.** Click **Check sensor data**. The check passes only with fresh CSI data from the configured
node. A successful flash or a confirmed configuration does not prove that UDP reaches AuraGo.

For step 3 to work, an administrator must publish UDP. Switch on **Publish UDP**, set **LAN bind IPv4** to a concrete
private address of the RuView machine (never `0.0.0.0`), and list the boards under **Allowed source IPs/CIDRs, comma
separated**, for example `192.168.1.50` or `192.168.1.0/24`. Docker NAT and the host firewall must let the UDP packets
through.

Wi-Fi credentials stay in browser memory, go straight to the board and are cleared when you close the dialog. AuraGo
never stores them. Your entries stay in place when a step fails. A write that ends in doubt is never retried
automatically. If the bootloader cannot be reached, hold BOOT while reconnecting, release it and try again. After an
AuraGo update, reload the page before you set up the next board.

## The Lab

**Lab** holds RuView's own web pages: **Observatory**, **Pose fusion** and the original RuView dashboard. Each page has
a card that names its requirement. Choose **Open** to load a page. It opens in a sandboxed frame. **← Lab** or
**Close** removes the frame, and so does leaving the Lab view. The pages are in English and remain experimental.
**Settings** in the **⋯** menu opens the configuration.

- **Language.** The upstream views keep their own language (English, with a Polish option in their interface). AuraGo
  translates its management pages and the board setup, not the upstream views.
- **Calibration, recording and training** are write actions. Starts and changes need **Read only** to be off. Stopping
  stays allowed. Recording and training are in the Dashboard's **Live Demo** and **Training** tabs.
- **Pose fusion** starts in **CSI Only (WiFi)**. Camera and video modes are not available in AuraGo. The tab says:
  "Pose-Fusion uses untrained example models. Its visualization and confidence scores are not validated measurements."
- **Blocked features.** Model loading and unloading, SONA and LoRA activation, pose statistics and demo zones are not
  available in this revision, and AuraGo blocks them. Timed recordings are refused.
- **Honesty.** The Hardware tab of the upstream Dashboard no longer shows simulated CSI: amplitude and phase show "—", and the antenna and Wi-Fi values are example configuration, not read from your hardware. The Live Demo shows "—" for signal strength and latency. The **Demo** button appears only while the RuView source is **Explicit demo**. It stops when the source changes, when the stream stops, or when you leave the tab. Marketing and benchmark sections carry the note "Upstream RuView project claims — not measured or verified by AuraGo."
- **Tour.** Skipping or finishing the Dashboard tour is remembered in this browser. To see it again, open the Dashboard's
  **Settings** button, choose **Reset onboarding** and click **Reset**.

## Recordings and Storage

The Dashboard's **Live Demo** and **Training** tabs have **Start Recording** and **Stop Recording** buttons. A recording runs until you stop it, even
after you close the view. After a reload, the views cannot tell whether a recording is still running, and the file list
does not show it either. Details therefore show only the last start or stop that AuraGo sent and got a
confirmation for in this session. After an AuraGo restart it reads "Recording status unknown" again.

Managed recordings and models are stored in `data/ruview/data` below AuraGo's data directory. The **Storage budget
(MiB)** limits them. AuraGo measures the folder at most every 30 seconds. Once the budget is reached, AuraGo refuses new recording
starts (HTTP 507) and asks the service to stop the current recording. Data written between two measurements can exceed
the budget. **Details** shows the storage used against the budget. The Status view shows the chip **Storage full ·
recording stopped** when the budget is reached, or **Storage can't be checked** when the folder cannot be measured. In
that case recording starts are refused with HTTP 503, and the budget is not treated as full.

AuraGo never deletes recordings. Remove the ones you no longer need yourself. Deletion is blocked in read-only mode.
External instances keep their own storage, and the budget does not apply to them.

## Good to Know

- **Experimental.** A connection, reception and a fresh status show that data is flowing. They do not show that a
  measurement, a presence or a pose is correct. Confidence values are not calibrated.
- **Signals.** RSSI needs Windows or macOS with a native or external instance. CSI needs ESP32 boards.
- **Demo data** is labelled. The headline reads *Demo mode: simulated data, not measurements.* If a sensor disconnects, AuraGo does not switch to demo data.
- **Stale data.** Frames older than ten seconds lose their fresh status. Values of that age are marked stale.
- **Supported boards.** Only the eight firmware profiles in the catalog: ESP32-S3 with 4, 8 or 16 MB flash, and ESP32-C6
  with 4 MB flash, each as a UART-bridge (`uart`) or native USB (`cdc`) variant. Other chips, including the classic
  ESP32, are rejected before anything is written. The firmware is version 0.8.12.
- **Not in this revision:** model loading and unloading, SONA and LoRA activation, pose statistics, demo zones, camera
  and video modes, and timed recordings.
- **Platforms.** This release has a native package for Windows (amd64) only. macOS packages are pending. The managed
  image is published for amd64 and arm64, but the arm64 build has not run on an arm64 machine yet. The managed container
  gets two CPUs, 2 GiB of memory and a read-only root filesystem.
- **Hardware acceptance** is not complete for this release. Real first installs on S3 and C6, updates that keep
  settings, reconfiguration and RSSI on real adapters are still open.

## Privacy and Safety

- **What leaves the machine.** Managed Docker pulls the pinned image `ghcr.io/antibyte/aurago-ruview` from the GitHub
  Container Registry. The Windows runtime and the firmware come from AuraGo's own resource package and are checked by
  hash before use. With an external instance, AuraGo talks only to the URL you saved. Requests with credentials never
  follow redirects.
- **Wi-Fi credentials** go from the browser straight to the board over USB. AuraGo's APIs never receive them, and they
  are not kept in browser storage.
- **Sandbox.** The upstream views have no network access of their own and no serial, camera or microphone permission.
  AuraGo relays only the reviewed calls.
- **Where data lives:**

  | Data | Location |
  |---|---|
  | Managed recordings and models | `data/ruview/data` |
  | Native runtime files | `data/ruview/runtime/<revision>/<platform>` |
  | Known board IDs (no measurements, no Wi-Fi credentials) | `data/ruview/`, one file per receiver |

- **Vault keys.** `ruview_api_token` holds the **API token (Vault)** of an external instance. `ruview_managed_token` is
  created on the first managed start and secures the link between AuraGo and the managed service. Neither key is
  exported to Python scripts or the agent.
- **Audit.** This release has no separate RuView audit log. Changes go through AuraGo's administrator login, its
  origin checks and the read-only switch.
- **The agent.** With **Allow agent read access** on and the integration enabled, the agent can call the `ruview` tool
  with `status`, `nodes`, `sensing`, `model_info`, `recordings` and `presence`. It cannot install, start, flash,
  configure, calibrate, record or train. Every answer carries source, time and freshness. `fresh: false` does not prove
  presence, and a simulation is not a sensor.
- **People in the room.** Sensing can show that someone is moving in a space. Tell the people affected, decide whether
  that is acceptable, and delete recordings you no longer need.

## When Something Goes Wrong

| Message | What helps |
|---|---|
| "Enable RuView and save your settings before continuing." | Switch on **Enable integration** and click **Save**. |
| "Save changes before testing or configuring boards." | Click **Save**. Tests and board setup use the saved settings. |
| "Turn off Read only above and save to set up a board." | Switch off **Read only**, click **Save**, then try again. |
| "Host Wi-Fi requires a native or external instance. Choose that runtime or another data source." | Choose **Native host runtime** or **External instance** as **Runtime**, or pick another **Source**. |
| "Install is available only for the native runtime." | **Install** (or **Install service** in the ⋯ menu) is offered only for the native host runtime. Managed Docker starts with **Start service**. |
| "Start and stop an external instance on its own computer." | AuraGo does not start or stop external instances. Work on that computer. |
| "RuView stopped unexpectedly." | Click **Start again**. AuraGo does not restart the service on its own. |
| "Stopping wasn't confirmed. RuView may still be running." | Click **Try stopping again**. If it stays, check the service on its computer. |
| "RuView can't start: the registry denied the service image." | The pinned image must be published and readable by AuraGo. Changing the receiver IP does not fix it. |
| "RuView can't start: the service image could not be downloaded." | Check the network and the Docker host, then click **Retry start**. |
| "RuView can't start: Docker can't be reached." | Check that Docker runs and that AuraGo can reach it, then click **Retry start**. |
| "A container named aurago-ruview belongs to another AuraGo installation. Remove or rename it in Docker, then start again." | AuraGo never removes a container it did not create. Rename or remove it yourself in Docker, then start again. |
| "RuView is busy with another start or installation. Wait for it to finish." | Wait for the other action to finish, then check the status again. |
| "RuView didn't answer in time." | Click **Check again** or **Retry start**. |
| "RuView rejected AuraGo's access token." | Check the **API token (Vault)** of the external instance in the settings. |
| "RuView uses a different signal source than set in AuraGo." | Make **Source** in AuraGo match the source RuView uses, then check again. |
| "RuView is sending simulated data although real sensing is set." | Check RuView's source setting. Real sensing must not carry simulated data. |
| The **Receiver and UDP set** step is open (Status setup list). | Enter the **RuView receiver IPv4**, switch on **Publish UDP** with a LAN bind address and an allowed source, save, then check again. |
| "Cannot reach the bootloader. Hold BOOT while reconnecting, release BOOT afterwards and close other programs using the serial port." | Hold BOOT while reconnecting, release it, and close other serial programs. |
| "Use HTTPS or localhost and a browser with Web Serial enabled. Close other serial tools; use a data-capable USB cable. Hold BOOT while reconnecting if detection fails." | Open AuraGo over HTTPS or `localhost` in Chrome or Edge, and use a data cable. |
| "Detected: {detected} MB flash. Selected profile: {expected} MB. Choose the matching profile; nothing was written." | Choose the profile that matches the detected flash size. |
| "The USB connection does not match the board profile. For "USB JTAG/serial debug unit", choose a "cdc" profile; for a USB-UART bridge, choose "uart"." | Pick `cdc` for the native USB port or `uart` for a bridge. |
| "This board has a different partition layout. Tick “Erase all settings and recordings on this board” to replace it; this deletes settings and recordings on the board." | Tick the erase checkbox and click **Write firmware** again. This erases the board's own settings. |
| "Verification failed. Nothing is retried automatically. Check the firmware files, USB connection and board before another attempt." | Nothing was retried. Check the cable and the board, then start again yourself. |
| "Reconnect the board after restart, then confirm" | Reconnect the board after it restarts, then click **Confirm after restart**. |
| "Node ID: 1–255; RuView receiver IPv4: 10/8, 172.16/12, 192.168/16; UDP: 1–65535." | Correct the value: a node ID from 1 to 255, a private IPv4 receiver, and a UDP port in range. |
| "All IDs (1–255) are reported as occupied. Enter an existing ID only to reconfigure that board." | Enter an existing ID only to reconfigure that board. |
| **Storage full · recording stopped** (Status chip) | No new recordings start. Remove recordings you no longer need, or raise **Storage budget (MiB)**. |
| **Storage can't be checked** (Status chip) | The data folder cannot be measured: a permission error, a link, a special file, or more than 200,000 entries. Recording starts are refused until it can be measured. Check the folder's permissions, and remove files you no longer need. |
| "RuView is running, but its signal source is disconnected." | Check the board or the Wi-Fi adapter. The **Boards** view shows the age of each board's last measurement. |
| "No new data for {age}." | The frames are older than ten seconds. Check the board's Wi-Fi and the RuView service. The headline clears when new data arrives. |
| "External RuView service, not checked yet." or "last checked {age} ago." | Click **Check connection** or **Check now**. Only a check asks an external service. |
| **Waiting for first data** (Test connection), or "RuView is running and waiting for data from the boards." (Status) | The service is connected, but no sensor frame has arrived yet. It is shown as waiting, not as success. Check the board's Wi-Fi, the **RuView receiver IPv4** and **Publish UDP**, then test again. |
| "Can't load the RuView status." (**Status** view) | The last known state stays on screen. The view retries every three seconds while it is open. If it stays, check that AuraGo is running and reachable. |
| "The setting could not be saved." (**Allow changes…**) | Check the RuView settings. If another configuration section is invalid, the save fails. Fix that section in **Settings**, then try again. |

**Technical details** appear under the headline. They show the scrubbed detail from RuView or Docker, with no tokens, URLs or file paths. **Copy diagnostics** (in the **⋯** menu or under **Details**) copies `key=value` lines with the state, the error code and that detail. Paste them into a report as plain text. If the clipboard is blocked, the text appears in a box that you can copy by hand.

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 8: Integrations](./08-integrations.md#ruview-wi-fi-sensing) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 29: Infra](./29-infra.md)
