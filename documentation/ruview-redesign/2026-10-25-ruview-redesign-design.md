# RuView redesign: calm status first, native essentials, upstream in a Lab

Status: approved by the user on 2026-10-25 (decisions D1–D8 taken, all as recommended). P0 landed on main (279e4f636..649ec0e72); P1 is next.

Source audit: `reports/ruview-audit-2026-10-25.md` (German). Intended location: `documentation/ruview-redesign/2026-10-25-ruview-redesign-design.md`. Behavioural contract: `documentation/ruview.md`.

## 1. Problem

The audit describes RuView (`ui/js/desktop/apps/ruview.js`, `ui/css/ruview.css`, `ui/js/ruview/*`, `internal/ruview`, `internal/server/ruview_handlers.go`) as three programs stacked on top of each other. Its safety and honesty plumbing is good. The interface is the weak part.

- **The shell shows plumbing instead of answers.** The overview has six cards (artifact, storage, recording marker…), a disclaimer paragraph, a five-item checklist, and the actions below the window edge at 1100×760 (screenshots 01, 03). Nothing answers "is it working, is someone there?" A Docker failure puts the concrete cause last (04). One card is titled "Unavailable" because a key is missing (M1). Wi-Fi and demo users see a false "needs attention" every time (H3). A brief outage shows "Disabled" (H9). Keyboard focus is lost every 3 s (H10).
- **The embedded upstream UI is bright and English inside a dark, German window** (09, 20, 21-*). It has ten tabs of its own that duplicate AuraGo's, a tour, a bell and marketing copy (M15–M19). In two places it shows invented data without a label (H1 random CSI, H2 pose demo). Hidden windows keep rendering (M20).
- **The board wizard shows all three steps at once** (a 1500 px dialog, 12b) and asks for raw profile IDs. A brand-new board reliably fails on the first attempt (H4). Closing during a write leaves a half-written board (H5). A second board inherits the first board's ID and passes on its frames (H6). Recovery texts are wrong (H8). Further traps are M9–M14.
- **The backend reports technical states that the UI cannot explain.** All errors come back as raw English 502s (M2). There is no start progress (M3). Stopped or exited containers are not distinguished (M23, M24). Config edits cancel unrelated operations (M27). The config test reports success for `fresh=false` (H7). `udp_ready` ignores whether the service runs (M30).

## 2. Goals and non-goals

Goals:

1. **Calm first view.** One sentence answers "is it working?", followed by one next step. Details are available but collapsed.
2. **Native AuraGo views for everyday use:** Status, Live, Boards, Recordings & training. They are themed, translated and keyboard-accessible, and they use one navigation level.
3. **Upstream pages move into a clearly labelled Lab.** They open on demand and render nothing while unseen.
4. **A guided setup assistant** where a new ESP32-S3/C6 board succeeds on the first try, and a second board is one click away.
5. **Precise states:** every runtime, source and data state has its own headline, colour and action. Errors are codes with translated causes, not raw Engine text.
6. **Looks native** in all 14 desktop themes (light and dark) and is complete in all 16 locales.

Non-goals:

- Vital signs, pose, zone occupancy or fall detection in native views.
- Re-theming or translating upstream pages (see D1).
- New hardware profiles.
- Native calibration UI, LoRA training and model loading. Model loading is blocked upstream; LoRA stays in the Lab.
- Automatic restarts.
- Changes to the agent tool set.
- Server-side recording-state inference.

**Invariants that must survive** (from `documentation/ruview.md`; each phase's review checks them):

- No invented data. Demo data appears only for an explicitly selected `simulated` source and is always labelled. There is no client-side simulation fallback.
- Data older than 10 s loses its fresh/live status. A fresh real board needs `esp32`, not simulated, `csi_status=active`, a sequence and an age ≤ 10 s (`presenceFresh`, `internal/ruview/read.go:98`).
- Opening or reopening the app never starts, installs or probes a managed runtime. External services are probed only by explicit checks and by the Live and Boards views while visible (D6).
- Read-only blocks starts, changes and deletions. It still allows stopping the managed service, stopping a recording and stopping training (`ruview.IsStop`, `internal/ruview/policy.go:14`). Every mutation is admin-only and origin-checked.
- The recording marker records only start/stop responses that this AuraGo process forwarded. Unknown after a restart, "not confirmed" after a failed or incomplete forward, no auto-retry, no timed recordings.
- Wi-Fi credentials live only in browser memory: never in APIs, never in storage, cleared when the assistant closes or access is lost.
- Firmware hash, size, chip and flash are checked before any write, with a readback afterwards. An uncertain write is never retried automatically.
- The upstream frame stays sandboxed without network access. Calls go only through the reviewed relay and allowlist.

## 3. Information architecture

### 3.1 Vocabulary

| Code concept | UI (en) | UI (de) | Notes |
|---|---|---|---|
| runtime | service | Dienst | "artifact/revision" appears only in Details as "version" |
| node | board | Board | "node ID" appears only in expert details |
| `node_id` | board number | Board-Nummer | shown as `#7` after the name (D5) |
| source `esp32` | ESP32 boards (CSI) | ESP32-Boards (CSI) | |
| source `wifi` | this computer's Wi-Fi (RSSI) | WLAN dieses Rechners (RSSI) | |
| source `simulated` | demo (simulated) | Demo (simuliert) | never success-coloured |
| fresh | live | live | |
| stale | no new data | keine neuen Daten | always with the age |
| `observed_at` | last checked | zuletzt geprüft | never shown without a label (M8) |
| `receiver_ip` | receiver address | Empfängeradresse | |
| recording marker | last recording action | letzte Aufnahme-Aktion | never "recording now" |
| readonly | read-only | schreibgeschützt | |

### 3.2 Views

| id | Tab (en / de) | Answers | Shown when |
|---|---|---|---|
| `status` | Status / Status | Is it working? What should I do next? | always (default) |
| `live` | Live / Live | Is someone there? Is the signal arriving? | enabled |
| `boards` | Boards / Boards | Which boards exist, which are live? Add, fix or rename a board | source `esp32`, or known boards exist for this receiver |
| `records` | Recordings / Aufnahmen (heading "Recordings & training" / "Aufnahmen & Training") | Record, train, see models | enabled |
| `lab` | Lab / Labor | Upstream Observatory, Pose fusion, original dashboard | enabled |

Unavailable views are hidden rather than shown disabled. The Status details explain why ("Boards are only used with the ESP32 source"). This fixes M7: Wi-Fi and demo users never see "Write firmware".

### 3.3 Config vs. app

- **Config (`ui/cfg/ruview.js`) keeps everything stored:**
  - enable, read-only, agent access
  - mode and source
  - external URL, token and `allow_private`
  - managed HTTP port and storage budget
  - UDP reception, bind and allowlist
  - default receiver address
  
  Its **Test connection** applies the H7 fix (§14 P0). Its **Set up board** opens the same assistant module as the app.
- **The app keeps everything operational:**
  - install, start, stop, check now
  - board setup, rename and forget
  - recordings, training and models (read)
  - the Lab
  
  It writes no configuration, with one possible exception: the read-only switch (D4).
- **Navigation between the two:** the app's `⋯` menu and empty states link to Config with the existing `/config#ruview` link (or `ctx.openApp`, if Config accepts a section; verify). Config keeps **Open RuView**.

### 3.4 Lab framing

The Lab has a landing page with three cards: **Observatory**, **Pose fusion** and **Original RuView dashboard** (the latter depends on D2). Each card has:

- the upstream name plus a translated one-line description
- chips **Experimental** and **English only**
- the requirement ("needs live CSI"; "demo only with the demo source")
- an **Open** button

Behaviour:

- **Mounting:** the iframe is created only after **Open**. A slim Lab bar above the frame shows `← Lab`, the page name, the chip and **Close** (which unmounts the frame).
- **Suspension:** when the window is minimised, hidden, scrolled out of view or switched to another view, the iframe is removed and its relay detached, which closes the relay's sockets. On return a placeholder shows "Paused while hidden · Resume". The page never remounts silently.
- **Frame title:** a translated title such as "Observatory (experimental, English)" replaces "RuView · index.html" (M5).
- **Footer:** "Pages from the RuView project v3122 (MIT licence), shown with AuraGo's safety patches." It links the locally served `ui/js/vendor/ruview/LICENSE.txt` and `NOTICE.txt`.

## 4. Layout

```
┌ command bar 44px ──────────────────────────────────────────────────────────────┐
│ ◉ RuView  [Status|Live|Boards|Recordings|Lab]        ● Live · 0.8 s  [action] ⋯ │
├────────────────────────────────────────────────────────────────────────────────┤
│                         view body (scrolls inside)                              │
└────────────────────────────────────────────────────────────────────────────────┘
```

**Command bar** (`.ruview-commandbar`, 44 px, never wraps):

- **Brand:** the RuView icon and name, plus a visually hidden `<h1>`.
- **View switch:** `role=tablist`, roving tabindex, arrow keys. Each view body is a `role=tabpanel`.
- **State pill** (`[data-ruview-pill]`): glyph plus short label plus age. It is the same everywhere; clicking it opens Status.
- **Primary action slot:** shows the current view's primary action, for example **Add board** on Boards and **Start/Stop recording** on Recordings. On Status it stays empty because the next-step button sits under the headline; on Live and Lab it stays empty.
- **`⋯` overflow:** Check now, Stop service (managed and running only), Install (native only), Settings, Copy diagnostics, About Wi-Fi sensing (the quality note).

**Responsive** (container queries on `.ruview-app`):

- below 720 px the view switch collapses to a menu button showing the current view
- below 560 px the pill shrinks to glyph plus word, and the primary action becomes icon-only with an `aria-label`
- nothing wraps to a second line (fixes 07/13)

## 5. Views

### 5.1 Status

```
│  ● Working: live data from 2 boards, last 0.8 s ago.                      │
│    Last checked 14:03:12 · ESP32 boards (CSI) · Managed Docker            │
│    [ Open Live ]                                                          │
│                                                                           │
│  ┌ Setup (only while incomplete) ───────────────────────────────────────┐ │
│  │ ✓ Turned on  ✓ Service running  ✓ Receiver set  ● First board  ○ Data │ │
│  └──────────────────────────────────────────────────────────────────────┘ │
│  ▸ Details                                                                │
│     Service    running · Managed Docker · verified v3122                  │
│     Source     ESP32 boards (CSI) · receiver 192.168.1.20:5005 · UDP on   │
│     Storage    312 MB of 1 GB                                             │
│     Recording  last confirmed action: started 14:01 (this AuraGo session) │
│     Hints      none open · Copy diagnostics                               │
│  ⓘ Wi-Fi sensing is experimental and does not prove presence.  More…     │
```

- **Headline:** one sentence, with its colour, glyph, primary action and secondary line from §7.
  - The headline region is the only status live region. It announces transitions, never polls.
  - The age ticks in an `aria-live=off` span.
- **Setup stepper:** visible until "data arriving" has been reached once on this receiver (`last_fresh_at` set) or the source is demo. Steps depend on the source:
  - **ESP32:** on → (changes allowed) → service → receiver and UDP → board → data
  - **Wi-Fi:** on → service → data
  - **External:** on → connection checked → board → data

  Each open step links to its fix. The UDP step counts as done only when the managed runtime is running (M30).
- **Details:** collapsed by default (`<details>`, open state remembered in `localStorage` key `ruview.ui.v1`). They contain service, source, version, storage numbers (M8), the recording marker, open planner hints with a link (M8) and **Copy diagnostics**. Diagnostics are a plain-text status snapshot: no tokens, error detail already scrubbed.
- **Errors:** the cause is the headline. Raw Engine text appears only under **Technical details** inside the error block (M2, 04).

### 5.2 Live

```
│ ┌ Presence ───────────────────────┐ ┌ Signal · last 2 min while open ───┐ │
│ │  ◐ Someone is here, not moving   │ │  ╱╲_╱╲__ ╱\_   (gap = no data)     │ │
│ │    estimate 72 % (uncalibrated)  │ │  −52 dBm now                       │ │
│ │    from 2 boards · 0.8 s ago     │ └────────────────────────────────────┘ │
│ └──────────────────────────────────┘                                        │
│ Boards  ● Living room 0.7 s   ● Kitchen 1.2 s   ○ Office · no data          │
│ ⓘ An estimate from Wi-Fi signals, not a safety or medical device.          │
```

- **Presence tile:**
  - Comes from the new `GET …/live` summary (§8.3), not from the iframe.
  - Levels: `absent` → "Nobody detected" / "Niemand erkannt"; `present_still` → "Someone is here, not moving" / "Jemand ist da, ruhig"; `active` → "Movement detected" / "Bewegung erkannt".
  - Missing or unknown → "No estimate" / "Keine Schätzung".
  - The estimate is shown only while the data is fresh. Once stale it is greyed out as "Last estimate 42 s ago: Movement", never as current.
  - With the demo source the tile carries a **Demo · simulated** ribbon, and the pill reads Demo.
  - The motion level depends on the verified `sensing/latest` schema (§8.3). Until then the tile shows freshness and board count only.
- **Sparkline:**
  - A client ring buffer of `rssi_dbm` from the summary, filled only while Live is visible and cleared on hide.
  - Failed or stale polls break the line, so gaps are never interpolated.
  - Drawn on a canvas, using the token colour read from computed style.
- **Board strip:** per board, a freshness dot and age (or "no data"), using the same rule as Boards.
- **Empty states:**
  - not enabled / not running: the Status headline plus "Go to Status"
  - waiting for frames: "Waiting for data from the boards"
  - no boards: "No boards yet" plus **Add board**

### 5.3 Boards

```
│ Boards (3)                                                   [+ Add board] │
│ ┌──────────────────────────────┐ ┌──────────────────────────────┐          │
│ │ ● Living room         #7     │ │ ▲ Kitchen             #8     │          │
│ │ Live · last frame 0.7 s      │ │ No new data for 42 s         │          │
│ │ Signal −51 dBm               │ │ Last value −60 dBm           │          │
│ │ [Reconfigure]  ⋯             │ │ [Reconfigure]  ⋯             │          │
│ └──────────────────────────────┘ └──────────────────────────────┘          │
│ ○ Office #9 · known board, no data from it right now                        │
│ 2 boards saved for a different receiver are hidden.  Why? (M14)             │
```

- **Card content:**
  - name and `#id`
  - freshness dot with text: live, no new data (with age), or "known board, no data right now" (offline)
  - RSSI only for live boards; the retained last value is labelled as such and only for the same receiver (keeps the existing `applyNodes` rule, `ruview.js:138`)
- **Actions:**
  - **Reconfigure** opens the assistant at "Wi-Fi & receiver" with the ID fixed.
  - The `⋯` menu holds **Rename** (D5), **Check settings** (readback only, M9) and **Forget**. Forget is a write; it fixes B13 and needs confirmation.
- **Rendering:** keyed by `node_id`, and the age updates via `textContent` only (H10).
- **Add board:** disabled with a visible reason when the preflight fails (read-only, no secure context, no Web Serial).

### 5.4 Recordings & training

```
│ Recording   Last confirmed action: started 14:01 (this AuraGo session).     │
│             RuView cannot report whether it is still recording.             │
│             [ Stop recording ]  Start recording          312 MB of 1 GB     │
│ Recordings  rec_7f3a… · 12 480 frames · 41 MB · 3 min        [Delete]      │
│ Training    Idle   [ Train a model ▸ ]   (form: recordings ☑, epochs …)     │
│ Models      Active: none · 0 models · details read-only                     │
```

- **Recording:**
  - Start sends `POST proxy/api/v1/recording/start` with `{id:"rec"}`; the proxy appends the random suffix.
  - Stop sends `POST proxy/api/v1/recording/stop`.
  - Both are accepted only through `controlResult(data,'success')` from `ui/js/vendor/ruview/control-contracts.js`, which the native code imports rather than copying.
  - Both buttons are always present, because the state is unknown. The marker only decides which one is emphasised.
  - Copy for the marker values: `unknown` → "No recording action since AuraGo started"; `recording_ack=unconfirmed` → "The last request was not confirmed".
  - Stop stays enabled while read-only; Start shows "Read-only" as its reason.
  - There is no duration field and no auto-retry.
- **Recordings list:**
  - Reads `recording/list`. The fields shown (`id`, `name`, `frame_count`, `file_size_bytes`, `started_at`, `ended_at`) are those read by `TrainingPanel.js`; verify against pinned upstream.
  - A failed read keeps the old list marked stale, with Refresh. An empty success shows its own empty state.
  - Delete needs confirmation and is blocked while read-only.
- **Training:**
  - Status comes from `train/status`, polled every 3 s while visible (`active` must be boolean). Progress fields are those `TrainingPanel` reads (`epoch`, `total_epochs`, `best_pck`, `phase`); verify.
  - Start covers standard training and pretraining, with the payload from `trainingPayload()` and acknowledgement `'started'`.
  - Stop requires `'stopping'` and stays allowed while read-only.
  - Server-disabled training stays disabled with its reason. LoRA links to the Lab.
- **Models:** `model/info` and `/api/v1/models`, read-only.

### 5.5 Lab

See §3.4. The upstream tour and bell stay as upstream ships them inside the frame. The Lab bar's chip sets expectations, and nothing upstream leaks into the native views.

## 6. Setup assistant

This replaces the three-section wall in `ui/js/ruview/wizard.mjs` with a stepper. It is a new module, `ui/js/ruview/assistant.mjs`, shared by Desktop and Config. `wizard.mjs` remains as a re-export until Config switches.

```
┌ Set up a board ─────────────────────────────────────────────── × ┐
│ ① Check  ② Connect  ③ Firmware  ④ Wi-Fi  ⑤ Confirm  ⑥ Data       │
│──────────────────────────────────────────────────────────────────│
│ Connect the board                                                 │
│ Plug the board into this computer with a USB data cable.          │
│ [ Choose USB device ]                                             │
│ ✓ ESP32-S3 · 16 MB flash · native USB  → firmware available       │
│ ✓ New board (empty flash)                                         │
│ ▸ Choose profile manually (experts)                               │
│──────────────────────────────────────────────────────────────────│
│                                          [Back]  [ Install ▸ ]    │
└──────────────────────────────────────────────────────────────────┘
```

The dialog holds at most 640 px of content and shows one step at a time. Its header is an `<ol>` with `aria-current="step"`. Each step has its own heading, which receives focus when the step changes.

1. **Check (preflight).** Runs before anything else, and the same check gates **Add board**. It covers:
   - secure context and `serialAvailable()` (`flash.mjs:4`)
   - enabled, writable and source `esp32`
   - firmware catalog loaded
   - managed UDP reception on, with a warning and Settings link if not
   - fetch timeouts of 10 s

   The step shows only if something blocks, and lists each blocker with its fix (Open over HTTPS, Use Chrome/Edge, Allow changes (D4), Open settings).
2. **Connect.**
   - `requestPort` runs in the click stack. The console comes from VID/PID (`serialVariant`, `flash.mjs:6`).
   - Chip and flash size come from the loader (`loader.chip.CHIP_NAME`, `readFlashId`), and the profile `<chip>-<MB>mb-<console>` is selected automatically.
   - The partition table at `0x8000` is classified: **blank** (all `0xFF`), **compatible** (hash match) or **different layout**.
   - Unknown combinations are rejected with the detected values; `validateBoard` stays authoritative.
   - Expert override: a collapsed select listing only catalog profiles.
   - "This board already runs RuView 0.8.12 → skip to Wi-Fi" appears as a secondary link.
   - If a native-USB device re-enumerates, the assistant asks to choose it again and explains that the COM number may change.
3. **Firmware.**
   - **Blank:** writes with no erase option (H4).
   - **Compatible:** update that keeps board settings; erasing is optional and collapsed.
   - **Different layout:** erasing is required, with an explicit consent checkbox and the existing copy explaining what is lost.
   - Phases list: checking files, connecting, erasing, writing %, verifying, restarting.
   - **Close protection (H5):**
     - `×`, Escape (`cancel` is prevented), backdrop and the window close button go through `ctx.setWindowBeforeClose`. During a write each asks: "Stop writing? The board would be left half-written."
     - A `beforeunload` guard applies while writing.
     - A policy loss or 401 during a write clears credentials immediately and closes after verification (D7).
     - An interrupted write leaves a banner the next time the assistant opens on this page.
4. **Wi-Fi & receiver.**
   - SSID; password with a **Show/Hide** toggle (`aria-pressed`).
   - **Keep the board's Wi-Fi** is offered only when the board's settings partition was kept (compatible update or skipped firmware), fixing M11.
   - Board number is suggested freshly for every board (H6); the name is optional (D5).
   - Receiver address is prefilled from `board-setup`, with candidates as choices, and the port is prefilled too.
   - **Send to board** validates first (`validateConfig`) and marks fields with `aria-invalid`/`aria-describedby` before the port chooser opens (M10, M13).
   - Labels are distinct: the entry point is **Add board**, this step's action is **Send to board**, and the dialog title is "Set up a board" (12b).
5. **Confirm.** Reconnect, then read back via HELLO:
   - The result shows "Board #7 · sends to 192.168.1.20:5005 · Wi-Fi configured ✓ (password not shown)".
   - It compares node, receiver, port, `configured=1` and the device digest captured in step 4.
   - Success registers the ID with `POST nodes`.
   - The non-secret expectation (node, receiver, port, chip, device digest, time) is kept so that a reload still allows confirmation without re-sending Wi-Fi settings (M9, D8).
6. **Data.**
   - Polls `POST test` and `proxy/api/v1/nodes` every 3 s for 45 s, with a visible countdown (announced at start and end only).
   - Success needs the existing fresh-CSI rule plus a frame from this node that is newer than the configure acknowledgement: frame time = poll time − `csi_last_seen_ms`, and the sequence must differ from the one recorded at configuration (H6).
   - On timeout, a specific checklist appears: UDP off, receiver not reachable, wrong Wi-Fi password (Back to Wi-Fi), ID already used. Plus **Keep waiting**.
7. **Success.**
   - Message: "Board #7 'Living room' is sending data."
   - Buttons: **Set up another board** (resets port, identity, expectation and ID suggestion; keeps the Wi-Fi draft in memory as today) and **Done → Live**.
   - If read-only was switched off for setup, also: ☐ "Turn read-only back on" (D4).

Errors map by stage first (`ruviewStage`, `flash.mjs:21`), then by error name. One table in the module gives each (stage, name/code) its own copy and recovery action:

- `NetworkError` while connecting → port busy or unplugged
- timeout while connecting → bootloader help with the BOOT/RESET steps
- readback mismatch → "the board reports different settings", with values
- `config_rejected` → the board rejected the settings

This fixes H8, including "Error · Error". `openAssistant()` returns `{close, done}` synchronously, so disposing the app during initialisation closes the dialog (M12).

| Finding | Fixed in step |
|---|---|
| H4 blank chip | Connect (classification), Firmware (no erase) |
| H5 close during write | Firmware (guards, D7) |
| H6 second board | Success (fresh ID), Data (frame after config) |
| H8 recovery texts | all steps (stage error table) |
| M9 readback after reload | Confirm (kept expectation, D8) and Boards → Check settings |
| M10 validation order | Wi-Fi & receiver |
| M11 erase vs keep Wi-Fi | Wi-Fi & receiver |
| M12 init timeout and close | Check (timeouts), synchronous handle |
| M13 accessibility | all steps |
| M14 other-scope boards | Wi-Fi (ID hint), Boards (hidden count) |
| M7 flash for Wi-Fi/demo | Check, Boards hidden |

## 7. State model

`RuViewModel.deriveState(status, nodes, live, now)` is a pure function evaluated in table order, first match wins. Pill tone tokens: success `--ds-color-success-fg`, warning `--ds-color-warning-fg`, danger `--ds-color-danger-fg`, info `--ds-color-info-fg`, neutral `--vd-theme-muted`. Every pill pairs colour with a glyph and a word.

| # | State (condition) | Headline en | Headline de | Pill | Primary action | Secondary |
|---|---|---|---|---|---|---|
| 1 | loading (no status yet) | Checking RuView… | RuView wird geprüft … | neutral ○ "Checking" | none | none |
| 2 | status request failed, last known kept (H9) | Can't load the RuView status. Showing the last known state. | RuView-Status nicht abrufbar. Letzter bekannter Stand wird angezeigt. | warning ▲ | Try again | time of last success |
| 3 | `!enabled` | RuView is turned off. | RuView ist ausgeschaltet. | neutral ○ "Off" | Open settings | what RuView does (one line) |
| 4 | managed, read-only, `runtime ∈ {not_started, stopped}` | RuView is read-only, so the service can't start. | RuView ist schreibgeschützt, der Dienst kann nicht starten. | info ◇ "Read-only" | Allow changes… (D4) / Open settings | what read-only blocks |
| 5 | native, `error_code=not_installed` | RuView isn't installed on this computer yet. | RuView ist auf diesem Rechner noch nicht installiert. | neutral ○ | Install | download size and platform |
| 6 | managed, `runtime ∈ {not_started, stopped}` | The RuView service isn't running. | Der RuView-Dienst läuft nicht. | neutral ○ "Stopped" | Start service | `stop_reason`: "Stopped by you at 14:02" / "Stopped after a settings change" / "Not started since AuraGo started" |
| 7 | `runtime=starting`, `phase=pulling_image` | Starting RuView: downloading the service image… | RuView startet: Dienst-Image wird geladen … | info ◐ "Starting" | none ("Starting…", disabled with reason) | "Step 2 of 4 · can take minutes the first time" |
| 8 | `runtime=starting`, other phase | Starting RuView: {phase}… | RuView startet: {Phase} … | info ◐ | none | phase since 0:42 |
| 9 | `runtime=stopping` | Stopping RuView… | RuView wird gestoppt … | info ◐ | none | none |
| 10 | `error_code=stop_failed` | Stopping wasn't confirmed. RuView may still be running. | Stoppen nicht bestätigt. RuView läuft eventuell noch. | danger ✕ | Try stopping again | planner hint link |
| 11 | `error_code` set (start, connection or contract) | cause sentence per code, for example "RuView can't start: the registry denied the service image." / "RuView is sending simulated data although real sensing is set." / "RuView uses a different signal source than set in AuraGo." / "A container named aurago-ruview belongs to another AuraGo installation." / "RuView stopped unexpectedly." / "RuView can't be reached." | e.g. "RuView startet nicht: Die Registry verweigert das Dienst-Image." / "RuView sendet simulierte Daten, obwohl echte Messung eingestellt ist." / "RuView nutzt eine andere Signalquelle als in AuraGo eingestellt." / "Ein Container namens aurago-ruview gehört zu einer anderen AuraGo-Installation." / "RuView wurde unerwartet beendet." / "RuView ist nicht erreichbar." | danger ✕ | per code: Retry start · Start again · Open settings · Check again | Technical details (scrubbed `error_detail`), last good at… |
| 12 | external, never checked (`observed_at` null) | External RuView service, not checked yet. | Externer RuView-Dienst, noch nicht geprüft. | neutral ○ "Not checked" | Check connection | the service's address host |
| 13 | ready, `simulated`, source `simulated` | Demo mode: simulated data, not measurements. | Demo-Modus: simulierte Daten, keine Messungen. | info ◇ "Demo" | Open Live | "Choose ESP32 or Wi-Fi in settings for real sensing" |
| 14 | ready, `waiting_for_frames` or ESP32 with no frame yet | RuView is running and waiting for data from the boards. | RuView läuft und wartet auf Daten der Boards. | info ◐ "Waiting" | Add board (no known boards) / Show boards | last checked |
| 15 | ready, source state `disconnected`, no error code | RuView is running, but its signal source is disconnected. | RuView läuft, aber die Signalquelle ist getrennt. | warning ▲ | Show boards / Open settings (Wi-Fi) | last live at… |
| 16 | fresh real data | Working: live data from {n} boards, last {age} ago. (Wi-Fi: "Working: this computer's Wi-Fi signal, last {age} ago.") | Läuft: Live-Daten von {n} Boards, zuletzt vor {age}. (WLAN: "Läuft: WLAN-Signal dieses Rechners, zuletzt vor {age}.") | success ● "Live · {age}" | Open Live | last checked; source; mode |
| 17 | was fresh, age > 10 s (`stale`) | No new data for {age}. | Seit {age} keine neuen Daten. | warning ▲ "No new data" | Show boards | last live at… (`last_fresh_at`) |

Overlays add a chip next to the pill but never replace the headline:

- **Read-only while running:** "Read-only".
- **`storage_limit`:** "Storage full · recording stopped" (warning).
- **External with a last check older than 60 s while on Status:** "Last checked 5 min ago". This replaces the open-ended "ready" behind M25.

## 8. Data and API changes

All additions are new keys. Existing clients (Config, agent, Lab relay) keep working.

### 8.1 `Status` (`internal/ruview/manager.go:20`)

| Field | Meaning | Source |
|---|---|---|
| `waiting_for_frames` | upstream `waiting_for_frames` (H7) | `test()` already parses it at `manager.go:212` but drops it |
| `runtime` | `external\|not_started\|starting\|running\|stopping\|stopped\|exited\|stop_unconfirmed` | from `managed`, `process`, the stop path and exit detection (M24) |
| `phase`, `phase_since` | `preparing\|pulling_image\|creating\|waiting_ready\|downloading\|verifying` | set in `start` / `startDocker` / `Install` (`runtime.go:43,235`) |
| `error_code`, `error_detail` | stable code plus scrubbed detail of at most 240 chars, without URLs or file paths (S9; reuse `readStatusError`) | replaces raw `Error` as the UI's input; `error` stays for compatibility |
| `last_fresh_at` | time of the last probe with fresh real data | kept across probes; reset only by `Configure` |
| `stop_reason` | `user\|config_changed\|process_exit\|aurago_shutdown` | `Stop`, `watch` (M27), `reapNativeLocked` |
| `storage_used_bytes`, `storage_budget_bytes` | managed only, null for external (M8) | watcher, walk throttled to 30 s, errors reported as `error_code=storage_unreadable`, not "full" (M28) |
| `recording_at` | time of the last confirmed recording action | `noteRecording` |
| `hints` | open operational hints (`sensing_stale`, `storage_limit`, `stop_unconfirmed`) | `m.hintOpen` (`hints.go`) |

Semantic fixes that come with these fields:

- `udp_ready` requires a running managed runtime (M30, `applyPassiveStatus` at `manager.go:142` gains a `running` input).
- `test()` merges `stop_failed` / `process_exited` instead of overwriting them (M26, `manager.go:250`).
- `Configure` cancels the generation only when `sameRuntime` (`manager.go:408`) reports a runtime-relevant change. `agent_access`, `readonly` and `storage_mb` no longer abort a pull (M27).
- `opMu` is released during the image pull or download so that phase updates, the watchdog and Stop are not blocked (M29). During `starting`, Stop is shown disabled with the reason "Wait for the start to finish" until this fix lands.
- `stopRuntime` removes the owned container after a confirmed stop. An owner-label mismatch yields `container_conflict` with instructions (M23). `watch` detects an exited container (M24).

### 8.2 Board inventory (`internal/ruview/nodes.go`)

- **`NodeList.live_state`:** `ok\|not_applicable` (source not ESP32) `\|not_running` (managed and not running; no dial) `\|unreachable\|invalid`. `available` stays as `live_state=="ok"`. The UI uses only `live_state`, which fixes H3.
- **`NodeList.other_scopes`:** the number of saved board files for other receivers (M14). Computed by globbing `data/ruview/nodes-*.json`; no contents are read.
- **Names (D5):**
  - The scope file becomes `{"version":2,"boards":[{"id":7,"name":"Living room"}]}`. `knownNodes` (`nodes.go:117`) still reads the legacy `[]int`.
  - The size limit rises from 4 KiB to 32 KiB.
  - Names are 1–40 characters with no control characters.
  - Nodes gain `name`; the agent `presence` read may include it (D5).
- **New writes** (admin, origin and write gate, in `ruview_handlers.go`): `PUT /api/ruview/nodes/{id}` `{name}` and `DELETE /api/ruview/nodes/{id}` (forget, B13). A corrupt file returns `error_code=inventory_invalid` with **Reset list** instead of a permanent 502.

### 8.3 Desktop live summary

`GET /api/ruview/live`, also reachable through `/api/desktop/integrations/ruview/live`:

- **Access:** admin-only through the existing `requireAdmin` and `desktopScopeAdmin` wrappers; read-only; never gated on `agent_access`. The new method is exported `Manager.Live(ctx)`, not `Read()`, because `Read()` requires agent access (`manager.go:266`).
- **Reuse:** `readPresence` (nodes plus `attachStatus`) and `presenceFresh`.
- **Shape:** `{observed_at, source, simulated, fresh, data_state, waiting_for_frames, last_frame_age_ms, status_error, live_state, nodes:[{node_id, name, state, fresh, age_ms, rssi_dbm?}], rssi_dbm?, motion?}`.
- **Managed but not running:** returns the status only, with no upstream dial.
- **Coalescing:** concurrent callers share one in-flight request through a singleflight with a 1 s cache, so two open windows do not double the upstream load.
- **`motion`** = `{level, confidence}`. It is filled only after the first P3 task confirms the shape against pinned upstream `a71634ae`:
  - The repo pins no `sensing/latest` schema; `read_test.go` uses a `{frames,note}` fixture.
  - `SensingTab.js` reads `classification.motion_level ∈ {absent, present_still, active}`, `classification.confidence`, `features.mean_rssi` and `node_features[]` from the `/ws/sensing` frames.
  - **Verify against pinned upstream** that `sensing/latest` returns the same frame. If it does not, `motion` is omitted and the tile shows freshness only.
  - Unknown levels map to "No estimate".

### 8.4 Errors and HTTP status

A typed `ruview.Error{Code, Detail, HTTP}` replaces the bare `fmt.Errorf` values at the decision points. A `ruviewError(w, err)` helper in `ruview_handlers.go` replaces the blanket 502 at `:182` and writes `{error_code, error, detail}`.

| Code | HTTP |
|---|---|
| `disabled`, `readonly`, `external_runtime` | 403 |
| `already_running`, `not_installed`, `busy`, `cancelled`, `container_conflict` | 409 |
| `invalid_node_id`, `invalid_name` | 400 |
| `unreachable`, `upstream_auth`, `docker_unavailable`, `image_denied`, `image_pull_failed`, `contract_unsupported` | 502 |
| `timeout` | 504 |
| `storage_full` | 507 (the proxy already uses it) |

The UI translates `ruview.err_<code>`. Unknown codes fall back to `err_unknown` plus Technical details, never to raw text in the headline.

### 8.5 Recording marker alignment

`noteRecording` (`proxy.go:147`) counts any 2xx as confirmed, while `controlResult` also requires `success:true`. P4 makes the proxy (which already buffers the JSON body at `proxy.go:136`) record "confirmed" only for 2xx with `success==true`, and treat other bodies as `unconfirmed`. Verify that pinned upstream always returns `success` for recording start and stop.

### 8.6 Push (later phase)

`GET /api/ruview/events` is an admin-scoped SSE stream, the same pattern as `handleDesktopLogStream` (`desktop_log_handlers.go:90`).

- It is not the global `/events` broadcaster, which is auth-only while RuView status can contain LAN addresses.
- It emits a full `status` snapshot when `runtime`, `phase`, `data_state`, `fresh`, `ready`, `error_code`, the recording marker, `storage_limit` or `hints` change. Changes are debounced by 250 ms, with a heartbeat comment every 25 s.
- It is fed by one `setStatus` helper in the manager and capped in subscribers.
- Clients keep 3 s polling as a fallback, and for ages.

### 8.7 Shell context

`menus-and-routing.js:1654` passes only `{esc,t,api}`. Pass the Infra set (`:1644`): `openApp`, `confirmDialog`, `setWindowBeforeClose`, `setWindowMenus`, `clearWindowMenus`, `readonly`.

### 8.8 Security items carried into the redesign

- M31: on every Lab mount, the relay resets on `load` and uses a per-mount handshake token in the fragment, and clicks on links inside the frame are intercepted.
- M32: per-endpoint body allowlist with re-serialisation for the writes the Lab uses.
- New endpoints get route tests for admin, origin and read-only refusals.

## 9. Frontend architecture

New classic scripts in `ui/js/desktop/apps/` are registered in `module-loader.js:28` in this order and bundled by `npm run build:ui`. Each file stays under 900 lines.

| File | Global | Responsibility |
|---|---|---|
| `ruview-model.js` | `RuViewModel` | pure: `deriveState`, setup steps, board freshness (mirrors `presenceFresh`), age/number formatting, sparkline buffer |
| `ruview-store.js` | `RuViewStore` | fetch with timeouts, per-view polling while visible (Infra's `visibility()` pattern, `infra-store.js:785`), epochs, error codes, actions, later SSE |
| `ruview-views.js` | `RuViewViews` | Status, Live, Boards; keyed rendering |
| `ruview-records.js` | `RuViewRecords` | Recordings & training; dynamic `import()` of `control-contracts.js` |
| `ruview-lab.js` | `RuViewLab` | landing, mount/unmount, relay attach/detach |
| `ruview.js` | `RuViewApp` | composition root, command bar, `render/dispose` |

`ui/js/ruview/assistant.mjs` is the stepper. `flash.mjs` splits into `detectBoard(port)` → `{console, chip, flashBytes, partition: 'blank'|'compatible'|'other', loader}` and `installFirmware(...)`. `ui/css/ruview.css` is rewritten with tokens only.

## 10. Accessibility

- **Structure:**
  - visually hidden `<h1>` "RuView"
  - real tabs (`tablist`/`tab`/`tabpanel`) with arrow keys
  - every card and section has a heading (M5)
- **Live regions:** one stable visually hidden `role=status` region at the app root. It is never re-created, never `display:none`, and announces only transitions and action results.
- **Focus and polling:**
  - Keyed DOM patches mean focus is never lost (H10).
  - Ticking ages live in `aria-live=off` spans.
- **Status colour:** never colour alone; glyph plus word.
- **Disabled actions:** use `aria-disabled` plus a visible reason linked by `aria-describedby`, so they stay focusable and explain themselves.
- **Assistant:**
  - the dialog is labelled by the current step heading, which gets focus on every step change
  - errors focus the first invalid field (M13)
  - the clicked button keeps focus while busy (`aria-busy`)
  - `<progress>` carries `aria-valuetext`
- **Target sizes:** 36 px controls, 44 px on coarse pointers.
- **Lab frame:** translated iframe title (§3.4).

## 11. Theming

- **Tokens only**, using the same set as Infra (`desktop-app-infra.css:2`):
  - surfaces: `--vd-theme-app-bg`, `--vd-theme-panel-bg(-strong)`, `--vd-theme-control-bg/hover`, `--vd-theme-overlay-bg`
  - lines: `--vd-theme-border(-strong)`
  - text: `--vd-text`, `--vd-theme-muted`
  - accent: `--vd-accent`, `--vd-accent-contrast`, `--vd-theme-accent-soft/border`
  - status: `--ds-color-{success,warning,danger,info}-fg`, mixed with `light-dark()` for readable text, as Infra does
  - spacing, radius and motion: `--ds-space-*`, `--ds-radius-*`, `--ds-motion-*`
  - focus: `--ds-focus-ring-width/offset` + `--ds-color-border-focus`
- **No hex colours** outside `color-mix()` fallbacks. This removes `#35a8bf`, `#71809655` and `#57adbe` (M6).
- **The assistant** keeps `--pw-*` as the outer fallback when mounted in Config.
- **Lab experiment (P4, cheap):** set `color-scheme` on the iframe element to the desktop scheme so upstream's `prefers-color-scheme` may follow. Verify in Chrome; if it does nothing, the Lab chip already sets expectations.

## 12. i18n

- **Locales:** all native strings in `ui/lang/ruview/*.json` for all 16 locales.
- **`t()` fallback:** returns the key and warns once in the console. It no longer turns every gap into "Unavailable" (`ruview.js:8`, M1).
- **Literal keys:** dynamic copy comes from literal maps (`STATE_COPY`, `ERROR_COPY`, `PHASE_COPY`), so the existing helper lint sees every key. All new files are registered in `desktopLocalI18nHelpers` (`ui/i18n_lint_test.go:1058`).
- **Error-code test:** a new Go test, `TestRuViewErrorCodesTranslated`, enumerates the `internal/ruview` error-code constants plus the `data_state` and `phase` values and checks `ruview.err_<code>` / `ruview.state_<x>` in all locales.
- **Formatting:** numbers, dBm and sizes use `Intl.NumberFormat`; ages use `Intl.RelativeTimeFormat` / unit formatting.
- **German copy:** state words capitalised ("Erledigt"), noun forms, informal "du", matching existing RuView strings.
- **Lab:** upstream pages stay English and are labelled as such.

## 13. Motion and performance

- **Polling:** 3 s only while the view is visible. Status, Boards and Live each poll their own endpoint, and Recordings polls only `train/status`. Polling pauses on `document.hidden`, a minimised or hidden window, or the window being out of view (Infra's observer pattern).
- **Ticking:** the 1 s age ticker runs only when an age is on screen. The sparkline redraws only on new data. There are no rAF loops in native views.
- **Lab:** unmounted when hidden or minimised, or when the user switches views (M20). Vendor code is never loaded unless a Lab page is open.
- **Reduced motion:** spinners become static glyphs under `prefers-reduced-motion` and the desktop `data-animations` gate.

## 14. Phases

Order: P0 → P1 → {P2, P3, P4} → P5. P2, P3 and P4 can run in parallel once the P1 shell is merged, and the `assistant.mjs` and `flash.mjs` split can start during P1. Each phase ends with:

- tests green
- one commit series
- one combined review (Critical/Important only) that includes the invariant list in §2

### P0: Honesty and correctness fixes (no redesign; ships alone)

**Status: done (2026-10-25, main 279e4f636..649ec0e72).** Deferred to P1: Docker self-exit detection (M24), the frame adapter keeping a stale `simulation` flag on a hung status fetch, lower-case check states in non-German locales, unused keys `live_unverified`, `synthetic`, `stopped`.

- **Goal:** today's UI stops inventing data and stops raising false alarms.
- **Files:**
  - `scripts/ruview/vendor-ui.mjs`, `patch-controls.mjs` and regenerated `ui/js/vendor/ruview/*`
  - `internal/ruview/{nodes,manager}.go`
  - `ui/js/desktop/apps/ruview.js`, `ui/cfg/ruview.js`
  - `ui/js/ruview/flash.mjs`
  - `ui/lang/ruview/*.json`
  - `documentation/ruview.md`
- **Tasks:**
  1. H1: patch out the `HardwareTab` random timer. Show "Not measured in this view."
  2. H2: gate the pose Demo button behind `auragoRuViewSimulationAllowed`, and make `stop()` cancel `demoAnimationFrame`.
  3. M19: label the hero, performance and applications sections "Upstream research claims, not verified by AuraGo".
  4. M22: replace all unchecked `.replace` / `replaceAll` calls with the checked `patch()`. Add an `upstream.test.mjs` check that the gate strings exist in the output.
  5. H3: add `live_state` and make the checklist treat `not_applicable`/`not_running` as neutral.
  6. H7: add `waiting_for_frames`. The Config test shows success only for `fresh && !error`; waiting, stale and `source_mismatch` become warnings.
  7. H9: add a `statusLoaded` flag; a failure keeps the last `enabled`.
  8. M1: add the missing keys (`recording`, `starting`, `stopping`, `stopped_state`, `waiting_for_frames`), make the `t()` fallback return the key, and add key enumeration to the lint.
  9. M26: `test()` keeps stop/exit errors.
  10. M30: `udp_ready` requires a running runtime.
  11. H4 minimal: a blank flash (all `0xFF` at `0x8000`) is written without erase, and the message distinguishes a blank chip from a changed layout.
  12. Fix the documentation drift listed under Low.
- **Acceptance:**
  - Without a sensor, the Hardware tab shows no changing values.
  - The pose demo is absent unless the source is demo.
  - Wi-Fi and demo sources show no "attention".
  - The Config test while waiting reads "Waiting for first data".
  - A blank-board fixture flashes without the erase checkbox.
  - No card title reads "Unavailable" in any locale.
- **Tests:**
  - Go: `TestRuViewNodesLiveState`, `TestRuViewTestKeepsStopFailed`, `TestRuViewUDPReadyRequiresRunningRuntime`
  - Node: `upstream.test.mjs` gate strings, `protocol.test.mjs` blank-flash fixture (today it uses zeros, `:65`)
  - one run of `scripts/ruview/browser.test.mjs` and `TestRuViewConfigBrowser`
- **Leaves out:** layout, error codes, new views.

### P1: Shell and Status view

- **Goal:** command bar, view switch, Status view, and the backend fields that drive §7.
- **Files:**
  - new `ruview-model.js`, `ruview-store.js`, `ruview-views.js` (Status only); rewritten `ruview.js` and `ruview.css`
  - `menus-and-routing.js:1654`, `module-loader.js:28`
  - `internal/ruview/{manager,runtime,hints}.go`, a new `internal/ruview/errors.go`
  - `internal/server/ruview_handlers.go`
- **Tasks:**
  - §8.1 fields and fixes M23, M24, M27, M28, M29
  - §8.4 error codes and HTTP mapping
  - shell context
  - command bar and responsive rules
  - Status headline, stepper and details
  - the old Devices list moves into Boards unchanged but restyled
  - the old iframes move under Lab with a minimal landing and label (full Lab behaviour comes in P4)
- **Acceptance:**
  - In every state row (fixture), the headline and next action are visible without scrolling at 1100×760.
  - The bar never wraps at 560 px.
  - Focus is retained across two polls.
  - The live region stays silent on routine polls.
  - A Docker `denied` error appears as the headline cause.
  - No hex colours remain in `ruview.css` (grep check in the Node test).
- **Tests:**
  - Go: status field table, error-code → HTTP mapping table, M24/M26/M28/M29 unit tests
  - Node: `deriveState` table test, one case per §7 row, in a new `scripts/ruview/model.test.mjs`
  - browser smoke: extend `browser.test.mjs` with a states walk-through at 1100×760 that checks focus retention
- **Leaves out:** Live, native recordings, the new assistant, SSE.

### P2: Boards and setup assistant

- **Files:**
  - `ruview-views.js` (Boards)
  - `ui/js/ruview/assistant.mjs`, `flash.mjs` (split), `wizard.mjs` (re-export)
  - `ui/cfg/ruview.js`
  - `internal/ruview/nodes.go` (v2 file, names, forget, `other_scopes`), `ruview_handlers.go`
- **Tasks:**
  - Boards cards and actions
  - the assistant steps from §6 and its finding map (H5, H6, H8, M9–M14, M7)
  - Config switches to `openAssistant`
- **Acceptance (fixtures):**
  - A blank board succeeds end to end.
  - Closing during a write asks for confirmation.
  - A second board gets a new ID and does not pass on board 1's frames.
  - A receiver typo is flagged before the port chooser.
  - After a reload, Confirm works without the password.
  - Keep Wi-Fi is not offered after an erase.
- **Tests:**
  - Go: `TestRuViewNodeNames*`, `TestRuViewForgetNode`, legacy file migration
  - Node: detection classification
  - browser smoke: update `scripts/ruview/wizard-browser.test.mjs` to the stepper
  - **Hardware acceptance by the user on aurago-test:** S3 blank, S3 update, C6, reconfigure, restart, UDP reception
- **Leaves out:** Live, native recordings.

### P3: Live view and live endpoint

- **Files:** new `internal/ruview/live.go`, `read.go` (shared freshness), `ruview_handlers.go`, `ruview-views.js` (Live), `ruview-model.js` (buffer).
- **Tasks:**
  1. **First task:** verify the `sensing/latest` shape against pinned upstream and record the result in `documentation/ruview.md`.
  2. The endpoint with coalescing.
  3. Presence tile, sparkline, board strip and their states.
  4. The D6 probing rule.
- **Acceptance:**
  - Fixture frames show Live.
  - Once frames stop, "outdated" appears after more than 10 s.
  - The demo source shows the ribbon.
  - A managed service that is not running causes no upstream dial (fixture counter).
- **Tests:**
  - Go: `TestRuViewLiveSummary*`, covering fixture contract, not-running, agent-access-independent, coalescing
  - Node: tile derivation
  - one browser smoke scenario
- **Leaves out:** vital signs, pose, zones.

### P4: Recordings and training (native) and Lab framing

- **Files:**
  - new `ruview-records.js`, `ruview-lab.js`
  - `ui/js/ruview/relay.mjs` (M31)
  - `internal/ruview/proxy.go` (M32, §8.5), `manager.go` (`recording_at`)
- **Tasks:**
  - §5.4 and the full §3.4 behaviour: mount on demand, unmount when hidden, translated titles, licence footer
  - M20, M31, M32
  - the `color-scheme` experiment
- **Acceptance:**
  - Start or stop recording shows the confirmed marker with its time.
  - A forwarded failure shows "not confirmed".
  - Read-only leaves Stop recording and Stop training enabled and Start disabled.
  - A hidden or minimised Lab window has no relay socket open (fixture socket count).
  - The iframe is absent from the DOM while Lab is not the active view.
- **Tests:**
  - Go: proxy marker with `success:false` and per-endpoint body allowlist
  - Node: relay handshake
  - browser smoke: recordings flow plus Lab mount/unmount
- **Leaves out:** LoRA, calibration and model delete in native views (they stay in the Lab, depending on D2).

### P5: Optional push and polish

- **Tasks:**
  - §8.6 SSE if D3 says so, with the client fallback
  - Copy diagnostics refinements
  - screenshots at 1500×970, 1000×700 and 430×700 in Standard, Fruity light, Minimal light/dark, Orbit and Windows XP
  - manuals `documentation/manuals/{de,en}/` for RuView and `documentation/ruview.md`
- **Tests:** Go SSE admin scope (401/403 for non-admin, no global `/events` leakage) and one smoke.

Guards in every phase: `TestShippedWebAssetsLoadNothingExternal`, `TestUIAvoidsCookieConsentClassPrefix` (no `cc-` classes; all new classes use the `ruview-` prefix), `TestFrontend_JS_NoLiteralTranslationFallbacks`, `TestDesktopUsedI18nKeysExistInAllLanguages`, the JS line budget, `node scripts/build-ui-bundles.js --check`, and `node scripts/ruview/vendor-ui.mjs --check`.

## 15. Decisions (taken 2026-10-25)

- **D1: Variant.**
  - **Decided:** variant (b) — native Status, Live, Boards and Recordings views plus a labelled Lab.
  - (b) native essentials plus Lab, or (a) theme and translate the iframes.
  - **Recommendation: (b).** Variant (a) means patching ten upstream tabs for theme, 16 locales and controls that do nothing, on every upstream bump, and it still leaves three navigation levels.
- **D2: The original upstream dashboard.** Keep it reachable in the Lab, or drop it.
  - **Decided:** keep the original upstream dashboard in the Lab; drop it at the next upstream bump if re-patching costs more than a day.
  - **Recommendation:** keep it in the Lab as "Original RuView dashboard" once the P0 honesty patches are in. It is the only UI for LoRA, the 3D sensing view and the model library.
  - At the next upstream bump, drop it if re-patching it costs more than a day.
- **D3: Polling vs SSE.**
  - **Decided:** 3 s polling while visible through P4; admin-scoped SSE only in P5 if start progress feels laggy in acceptance.
  - **Recommendation:** 3 s polling while visible through P4; it is proven and cheap. Build P5 SSE only if start progress feels laggy in acceptance.
- **D4: Read-only on first setup.**
  - **Decided:** read-only stays default-on; Status and the setup assistant offer "Allow changes…" with a confirm sheet that saves only `ruview.readonly=false` through the admin config save path (P1 must verify a single-field save is supported; otherwise fall back to a Config link); the success screen offers "Turn read-only back on".
  - **Recommendation:** keep read-only on by default.
  - Status and the assistant preflight offer **Allow changes…**: a confirm sheet that explains what read-only blocks and writes only `ruview.readonly=false` through the existing admin config save path (verify that a single-field save is supported).
  - The success screen offers "Turn read-only back on".
  - Alternative: only link to Config.
- **D5: Board names.**
  - **Decided:** optional board names (≤ 40 characters, per-receiver file v2, legacy `[]int` still read), editable in Boards and the assistant, and included in the agent presence read.
  - **Recommendation:** yes. An optional name (at most 40 characters) stored next to the ID in the per-receiver file (v2 format), editable in Boards and the assistant.
  - Sub-question: include names in the agent `presence` read? Recommended yes; names are user-chosen room labels and the read stays read-only.
- **D6: Probing an external service while the app is open** (M25, documentation drift).
  - **Decided:** Status never probes an external service automatically ("Check now" + "last checked …"); Live and Boards count as explicit data reads every 3 s while visible; `documentation/ruview.md` is updated accordingly in P1/P3.
  - **Recommendation:** Status never probes an external service automatically.
  - Live and Boards count as explicit data reads while visible (3 s).
  - Update the documentation to say so.
- **D7: Access loss during a firmware write.**
  - **Decided:** on access loss during a firmware write, clear Wi-Fi credentials immediately, finish the running write and verification, then close; `documentation/ruview.md`'s invariant is amended in P2.
  - Today: close immediately, which aborts the write.
  - **Recommendation:** clear credentials immediately, finish the running write and verification (about 2 min at most), then close. This amends one documented invariant.
- **D8: Readback after a reload** (M9).
  - **Decided:** keep only the non-secret readback expectation (node ID, receiver, port, chip, device digest, time) in `sessionStorage` until success, logout or 30 minutes; Wi-Fi credentials stay memory-only.
  - **Recommendation:** keep the non-secret expectation (node ID, receiver, port, chip, device digest, time) in `sessionStorage` until success, logout or 30 min.
  - Wi-Fi credentials stay memory-only.
  - Alternative: memory only, with readback lost on reload.

## 16. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Upstream updates break native views | Native views use few endpoints (`status`, `nodes`, `sensing/latest`, `recording/*`, `train/*`, `models`, `model/info`). Go fixture contract tests per endpoint; fields not confirmed against the pin are marked "verify" and their UI degrades to freshness-only. |
| Logic duplicated against the vendored panels | Import `control-contracts.js` (payloads, acknowledgements) instead of copying. Freshness has one Go source (`presenceFresh`) with a shared fixture table tested in Go and Node. |
| Lab patch burden | M22 checked patches plus gate-string tests; D2 sets a drop rule. |
| Licence | Upstream is MIT (`ui/js/vendor/ruview/LICENSE.txt`) with `NOTICE.txt`. Both remain shipped and are linked from the Lab footer. Native code is written against the API; any ported snippet keeps an MIT attribution header. |
| `sensing/latest` shape unknown | The first P3 task verifies it; the motion block is optional. |
| Hardware acceptance only on the user's test server | Serial fixtures (`wizard-browser.test.mjs`) cover the flow; the user does S3/C6 acceptance on aurago-test in their own browser (Web Serial needs HTTPS). |
| aurago-test rules | `startDocker` removes and recreates an owned `aurago-ruview` container (`runtime.go:249`). The test-server rule forbids recreating existing containers, so ask before running Start/Stop acceptance there. Live, Boards and the assistant do not touch containers. |
| Upstream load with several windows | The `/live` endpoint coalesces; polling runs only while visible. |
| New endpoints widen the attack surface | They sit under the existing admin wrappers; writes need origin and write gates; `error_detail` is scrubbed; SSE is admin-scoped; route refusal tests in every phase; a security review of the P1/P2/P4 endpoints. |
| Translation volume (about 150 new keys × 16 locales) | Keys are added per phase with the lint; German is reviewed by the user during acceptance. |
| Scope creep | Native views cover only §5. Everything else stays in the Lab until a concrete need appears. |
