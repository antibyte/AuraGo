# Chapter 25: CYD Creator – apps for the Cheap Yellow Display

CYD Creator builds apps for the "Cheap Yellow Display" and its relatives: ESP32 and ESP32-S3 boards with a
built-in touch display. You describe the app to the agent, or you design the screens yourself in the designer and
let the agent write the logic behind them. AuraGo compiles in a Docker container, and you flash the result straight
from the browser to the board. Later versions can reach the device over Wi-Fi.

Technical details for administrators (configuration, Docker, OTA protocol, error codes) are in the
[operator reference](../../cyd-creator.md).

## Table of Contents

1. [What You Need](#what-you-need)
2. [Your First App](#your-first-app)
3. [Flashing: Keep Settings or Full Install](#flashing-keep-settings-or-full-install)
4. [The Serial Monitor](#the-serial-monitor)
5. [Wi-Fi on the Device](#wi-fi-on-the-device)
6. [Updates over Wi-Fi (OTA)](#updates-over-wi-fi-ota)
7. [Icon Library](#icon-library)
8. [Export and Import](#export-and-import)
9. [Good to Know](#good-to-know)
10. [Designer Shortcuts](#designer-shortcuts)
11. [When Something Goes Wrong](#when-something-goes-wrong)

---

## What You Need

- **A supported board.** When you create a project, the board list shows what has been tested:

  | Board | Status |
  |---|---|
  | CYD 2.8" (ESP32-2432S028R, micro-USB), the classic CYD | *Verified on hardware* |
  | CYD2USB 2.8" (USB-C, ST7789), CYD 2.4" resistive and capacitive, CYD 3.5" resistive and capacitive | *Community, untested* |
  | Guition 4.3" (JC4827W543C, ESP32-S3), Sunton 4.3" and 7" (ESP32-8048S043C/S070C, ESP32-S3, 800×480) | *Community, untested* |

  Community boards compile, but nobody has tested them on real hardware yet. The Guition board cannot rotate its
  display, so it offers landscape only.
- **One-time setup by an administrator:**
  - Docker is configured in AuraGo and not read-only.
  - **Config → CYD Creator → Enable CYD Creator** is on. Switching it on takes effect after an AuraGo restart;
    then CYD Creator appears in the start menu.
  - On first use the build tools are missing. **Install build tools** downloads the toolchain image once (the app
    says "about 3 GB"; on disk it takes about 2.3 GB). As long as the image is not published yet, the administrator
    has to build it, see the [operator reference](../../cyd-creator.md#builder-image).
- **For flashing and the serial monitor:**
  - Chrome or Edge on the computer the board is plugged into. Firefox and Safari have no Web Serial.
  - AuraGo over HTTPS or `localhost`, with CYD Creator open in the desktop itself (not inside an embedded page).
  - The administrator switch **Config → Virtual Desktop → Serial ports on this computer**
    (`virtual_desktop.serial_browser_enabled`) is on. It is off by default and applies to administrators only.
    Flashing and the monitor do not work on a read-only desktop.
  - If anything of this is missing, the Flash dialog says what and offers **Download image**: write `merged.bin` at
    address 0x0, for example with esptool or ESP Web Tools.
- On Windows the classic CYD may need the CH340 driver.

## Your First App

1. Open **CYD Creator** on the desktop (start menu → Development).
2. Choose **Describe with the agent** or **Start in the designer**. The dialog asks for the name, board, theme,
   orientation and device language (the language of the texts the device shows by itself, such as the Wi-Fi
   setup).
3. **With the agent:** describe the app, for example "Weather display for Berlin with temperature, humidity and a
   forecast page", and click **Create and plan**. The agent shows a plan first (screens, widgets, data sources,
   libraries, Wi-Fi). Nothing is built until you click **Build it**; **Change plan** tells the agent what to do
   differently. The agent then creates screens and widgets, writes the code and compiles. It repairs compile errors
   on its own (up to three rounds by default).
4. **In the designer:** drag widgets from the **Widgets** list onto the screen, move them and resize them with the
   handles. **Properties** on the right sets text, values, colours and events. For an event you pick a built-in
   action (**Open screen**, **Go back**, **Open Wi-Fi setup**) or **Custom logic (agent)** and write in plain
   language what should happen. **Generate code from layout** in the agent panel lets the agent write that logic.
   With nothing selected, **Properties** shows the project settings: theme, orientation, Wi-Fi, AuraGo link,
   libraries, backlight, status bar clock, device language and time zone.
5. **Build** compiles the app. Progress shows in the **Build log** at the bottom, errors and warnings under
   **Problems**. Clicking a problem shows the widget in the designer or opens the file at that line.
6. Plug in the board and click **Flash**, then **Connect and flash**. Pick the port in the browser dialog. If the
   board does not respond, hold the BOOT button while it connects. The serial monitor opens afterwards (you can
   switch that off in the dialog).

## Flashing: Keep Settings or Full Install

The Flash dialog asks for the **Install mode**:

- **Keep settings** writes only the app. The touch calibration and Wi-Fi settings stay on the board. This needs a
  board that was flashed with CYD Creator before.
- **Full install (wipes calibration and Wi-Fi)** writes the complete image from address 0x0. Set up calibration and
  Wi-Fi again afterwards.
- **Automatic** (the default) keeps the settings only when this browser flashed this board for this project before
  (same MAC address, chip and board profile, unchanged partition table). Otherwise it does a full install. Another
  browser or another project counts as a new board.

The dialog flashes at 921600 baud and falls back to 460800 and 115200 on trouble. It checks the image before
writing (SHA-256) and afterwards (MD5 read back from the board).

## The Serial Monitor

The **Monitor** tab at the bottom shows the board's serial output (115200 baud by default). You can **Pause**,
**Clear**, **Filter lines** and show **Timestamps**. **Restart board** resets it through the USB control lines.
After a crash a **Decode** button appears next to the `Backtrace:` line: AuraGo turns the addresses into files and
lines with the build this browser flashed last. **Attach log** hands up to 16 KiB of the selected or visible lines
to the agent. The monitor pauses while a flash runs.

## Wi-Fi on the Device

Apps that use Wi-Fi show the setup on first start:

- **With your phone:** scan the QR code on the display. The phone joins the network `CYD-Setup-…` (WPA2, the
  password is in the QR code) and the setup page opens. Pick your network and enter the password.
- **On the device:** tap **Set up on this screen**, pick the network from the list and type the password on the
  on-screen keyboard. Enter hidden networks by hand under **Hidden network**.
- Pressing **BOOT** right after power-on and holding it for five seconds makes the device forget its Wi-Fi
  settings. (Do not hold it while powering on, or the chip starts in download mode.)
- Boards with resistive touch (like the classic CYD) ask for a touch calibration on first start. Hold the display
  for three seconds during the boot screen to repeat it.

## Updates over Wi-Fi (OTA)

Switch on **AuraGo link (heartbeat and OTA updates)** in the project settings (it needs Wi-Fi) and build again.
When flashing, CYD Creator then registers the board with AuraGo and writes a device key into a separate partition
of the board. Nobody sees that key: AuraGo stores only its hash, and an export never contains it. The device then
checks in with AuraGo regularly (every minute by default).

**Devices** in the toolbar lists the project's boards with their state (*Online*, *Offline*, *Waiting for first
contact*), running build, last contact, IP address and Wi-Fi signal:

- **Update to** picks a successful build. The device fetches it at its next check-in, shows a progress bar and
  restarts.
- If a build installed over Wi-Fi crashes three times in a row before it has checked in as stable, the device
  returns to the previous build on its own. Choosing the same build again then has no effect until the board
  restarts: make a new build or switch the board off and on to retry.
- **Rename** changes the device name. **Remove** revokes the key at once; the board can report again only after
  you flash it again.
- A project has at most 64 devices. A board that does not check in within an hour after flashing drops off the
  list by itself. Flashing a board again from the same browser replaces its old entry.

For this to work the board must reach AuraGo. AuraGo derives the address from the server settings; inside Docker,
behind a reverse proxy or NAT an administrator enters the right one under **Config → CYD Creator → AuraGo address for
devices** (`cyd_creator.ota.public_url`). The host name may contain only letters, digits, dots and hyphens, no
underscores; IPv6 addresses do not work. When in doubt, use the IPv4 address, for example
`http://192.168.1.20:8088`.

With HTTPS the board remembers the fingerprint of AuraGo's certificate when it is flashed. When the certificate
changes (a Let's Encrypt certificate is renewed about every 60 days, a self-signed one may be regenerated), the board
shows "AuraGo certificate changed – please reflash via USB". Only a new flash over USB helps then. `http://` has no
fingerprint, but the key travels unencrypted; the device still checks the update images.

## Icon Library

Select an image or button and open **Icon library** in its inspector. Choose
from 60 motifs in four styles, each in colour or monochrome and still or
animated form: 960 variants. Search by name, filter by category and select
24, 32, 48 or 64 pixels. Light/dark previews show device pixels and the memory
display helps you choose. Monochrome icons inherit the device theme text
colour; an explicit icon colour can be set in the inspector.

Only selected variants enter the project. Reused images and frames count once
against image memory. If an import exceeds the budget, choose a smaller size
or a still image. PSRAM does not increase the flash image budget. Animation can
be switched off; the system reduced-motion preference uses posters in the
designer. Insertion and style changes support undo. Project ZIPs include
masks, animation frames and settings. Using the library needs no image
generation service.

## Export and Import

**Export** saves the project as a ZIP: a complete PlatformIO project with `cyd.project.json`, your code under
`src/app`, the images, the generated code and a README. You can build it further, for example in VS Code with
PlatformIO. The fonts in the generated code come from the newest successful build, so build once in CYD Creator
before you export. Wi-Fi settings and device keys are never in an export. Your code, however, is exported as it is:
**check `src/app` for keys or passwords before you share the archive.** An export may unpack to at most 128 MiB; at
most two exports run at the same time.

**Import ZIP** on the start screen opens a CYD Creator export as a new project; an existing project is never
overwritten. The archive may be at most 20 MiB with at most 500 entries, and the files it keeps may unpack to at
most 20 MiB. Generated files, fonts and the README in the archive are ignored and generated again. CYD Creator
refuses an archive when

- it is no ZIP, or `cyd.project.json` is missing or damaged,
- it is too large or has too many entries,
- a path is unsafe or appears twice,
- it contains a file that does not belong to a project,
- the board is unknown,
- a file under `src/app` is too large, not UTF-8 text or differs from another one only in letter case,
- an image is damaged, too large or has a name that is not allowed, or the images take more than 16 MiB together,
- it needs a newer template version than this AuraGo knows.

## Good to Know

- **Saved automatically.** The designer and the code editor save one second after your last change. When the
  project was changed elsewhere, CYD Creator asks whether to keep editing, load the saved version or overwrite it.
- **While the agent works,** layout and code are locked. You can watch and stop it with **Cancel job**. The project
  then stays as it was before the job; what the agent did so far is kept as a draft that you can **Resume** or
  **Discard draft**. Only one agent job runs at a time, across all projects.
- **Versions.** A version is recorded before and after every agent job, on import and on every restore (not for
  your own edits in the designer and the code). **Versions** shows the differences and restores earlier states;
  CYD Creator saves the current state as a version first. Each project keeps its newest 50 versions.
- **Code.** The **Code** view edits everything under `src/app/` (at most 64 files, 256 KiB each, 2 MiB together).
  Everything produced from the layout (`src/generated/` and the template) is read-only; change it in the designer.
- **Builds.** One build runs at a time; the first build of a project for a board takes longer. An error marked
  *generated* is in the generated code: a problem of CYD Creator, not of your code. By default CYD Creator keeps the
  last five builds per project, plus always the newest successful one and every build a device runs or is due to
  get.
- **Themes.** Clear, Night, Terminal, Sandstorm, Dark Sun, Synthwave and High contrast. Each widget can also be
  recoloured individually in its properties.
- **Languages.** The CYD Creator interface comes in AuraGo's 16 languages. The device itself speaks Czech, Danish,
  Dutch, English, French, German, Italian, Norwegian, Polish, Portuguese, Spanish and Swedish.
- **Images.** PNG and JPEG (SVG is converted to PNG in the browser), at most 2 MiB per file and 16 MiB per project.
  On the device the decoded size counts: on boards with 4 MB flash the images may use at most 256 KiB together, on
  boards with 16 MB flash 4 MiB.
- **Security in app code.** HTTPS requests from your app are encrypted but do not check the certificate. Do not send
  passwords or keys through them.

## Designer Shortcuts

| Key | Action |
|---|---|
| Arrow keys / Shift+arrow | Move the selection by 1 / 8 pixels |
| Del or Backspace | Delete the selection |
| Ctrl+C / Ctrl+X / Ctrl+V / Ctrl+D | Copy / cut / paste / duplicate |
| Ctrl+Z / Ctrl+Y or Ctrl+Shift+Z | Undo / redo |
| Ctrl+A | Select all widgets on the screen |
| Esc | Clear the selection (also cancels a running drag) |
| + / - / 0 | Zoom in / zoom out / fit |
| Enter or Space (in the Widgets list) | Add the widget in the centre |
| F2 / Alt+arrow up or down (in the screen list) | Rename / move the screen |

## When Something Goes Wrong

| Message | What helps |
|---|---|
| CYD Creator is missing from the start menu | An administrator enables CYD Creator, configures Docker (not read-only) and restarts AuraGo. |
| "The build tools … are not installed yet." | Click **Install build tools**. |
| "Installing the build tools failed." | The registry is unreachable, the disk is full or the image is not published yet; see the [operator reference](../../cyd-creator.md#builder-image). |
| "Docker is not reachable, so nothing can be built." | An administrator checks the Docker host or the socket proxy. |
| "The build took too long and was stopped." | Build again (the cache is warmer then) or raise the timeout or the CPU cores in the configuration. |
| "The app does not fit into the flash memory of the board." | Remove images or font sizes, or pick a board with more flash. |
| Error marked *generated* | A bug in CYD Creator, not in your code; export the project and report it. |
| "Serial ports are switched off for the desktop …" | An administrator turns on **Config → Virtual Desktop → Serial ports on this computer**. |
| "This browser has no Web Serial …" / "… only works over HTTPS …" | Use Chrome or Edge and open AuraGo over HTTPS or `localhost`; otherwise **Download image**. |
| "The serial port is in use by another program or tab." | Close the other program, for example a Quick Connect terminal, a second tab or another serial monitor. |
| "The connected board has a different chip …" | The board does not match the project's board profile. |
| "The board was disconnected." | Flash again and hold BOOT if needed; this cannot damage the board. |
| "AuraGo has no address the board can use to reach it." | An administrator sets **AuraGo address for devices** (rules above). |
| "AuraGo could not read the HTTPS certificate …" | An administrator checks that the configured https address is reachable, or sets another one. |
| A device never shows up under **Devices** | Check that the board reaches the AuraGo address from its Wi-Fi; the serial monitor shows the attempts. |
| "AuraGo certificate changed – please reflash via USB" | Flash the board once more over USB. |
| "This project needs a review after an interrupted publication …" | An administrator resolves it as the [operator reference](../../cyd-creator.md#troubleshooting) describes. |

---

**Previous Chapter:** [Chapter 24: EasyDrag](./24-easydrag.md)
