# MeshCore Companion integration

AuraGo connects to one Companion radio. USB is implemented for Linux, Windows
and macOS; Bluetooth uses BlueZ on native Linux. **Hardware acceptance is still
pending on all platforms.** Firmware flashing and repeater administration are
excluded. Administrative radio-setting changes require a separate opt-in below.

## Setup

**Additional agent instructions** optionally sets language, reply style or other
MeshCore-specific guidance for automatic replies and agent tool use. Enter up to
2000 characters and save; clear the field and save to remove the guidance.
Permissions, security checks, location privacy and radio limits still apply.
The separate inbound security scan and manual Desktop Messenger messages do not
use these instructions. The configuration key is `meshcore.additional_prompt`.

1. Install the Companion firmware appropriate for USB or BLE on your device.
2. Open **Settings → MeshCore**. Select the transport, choose a serial port or enter a
   Bluetooth address, enable the integration, and save. USB uses 115200 baud.
   The port list uses basic enumeration, without macOS CGO USB-detail discovery.
   **Refresh** reloads the dropdown; a previously selected, missing port stays
   selected and is marked as unavailable.
3. For BLE, explicitly pair the selected device first. You can save a BLE
   address while MeshCore is disabled, search and pair, then enable MeshCore.
   The existing Bluetooth settings must permit discovery/pairing; audio access
   is unnecessary. The optional PIN is transient and never saved. AuraGo does
   not pair automatically or connect to an unconfigured radio.
4. Compare the displayed full device public key with your device's identity,
   choose **Confirm this device identity**, and save. A test only reads saved
   connection settings and radio metadata; it sends no radio message.
5. Copy complete 64-character node public keys into the trusted list, one per
   line. Only unambiguous, synchronized chat contacts sending direct plain text
   may start the normal agent. Each full node key has its own chat session.
   Alternatively, search the node list by name or key, choose the permission list
   under **Add to**, and click a chat node. Its full key is added without
   duplicates; then select **Save**.
6. Enable **Reply to trusted direct messages** if desired. For channels, confirm
   the channel assignment and select receive-only, prefix (`!aura` followed by
   whitespace), or question detection. Save the configuration.
   Question detection includes open channel questions and radio checks such as
   "anyone receiving", even without punctuation or directly addressing AuraGo.
   Replies confirm arrival at this node only, without web search or claims about
   other receivers. They may report the supplied SNR and known hop count.
   Location disclosure is off by default. To allow it, enter a public location
   description and enable the adjacent option. Replies may disclose exactly that
   text; radio positions, coordinates, routes and inferred locations stay private.
7. Proactive sending is a separate opt-in. Enable it and allow individual node
   keys or channels. Automatic replies do not require proactive permission;
   their destination is fixed internally to the incoming node or channel.

Saved channel assignments survive restarts when the device and channel are
unchanged. After the fingerprint calculation update, older assignments may
require **Confirm channel assignment** and **Save** once. Existing histories
remain available; permissions are never automatically transferred to a different
channel binding.

In **Settings → MeshCore → Channels**, **Remove channel from device** asks for
confirmation, removes the channel from the connected Companion, and also removes
its saved agent permissions. Save other pending Config changes first. If a saved
rule no longer has a matching device channel, **Remove rule** changes only the
Config draft; use **Save** to persist that cleanup. An uncertain device edit
keeps the channel locked until its mapping is reconciled.

On Linux, the one-line installer, `install_service_linux.sh`, and `update.sh`
automatically grant the systemd service USB access through existing `dialout`/
`uucp` groups. Permissions take effect when the service starts, without a new
login. With `--no-restart`, they take effect on the next service restart.
Manual starts without systemd still require suitable device permissions.
On macOS use `/dev/cu.*`; on
Windows use a port such as `COM3`. For Docker, explicitly pass the device, e.g.:

```yaml
services:
  aurago:
    devices:
      - /dev/ttyACM0:/dev/ttyACM0
```

Bluetooth is unavailable in Docker. This integration requires no audio
passthrough, privileged container or general host D-Bus mount.

## Security and inbox

Every accepted text frame is validated, deduplicated and scanned for injection.
It then undergoes a separate LLM risk check. With LLM Guardian enabled, MeshCore
requires an actual successful content verdict even with global `fail_safe:
allow`. An unavailable enabled Guardian never silently falls back. When Guardian
is disabled, the main model scans with a fixed prompt, no tools, no history and
no private memory. Invalid/truncated output, tool calls and timeouts quarantine
the message. Trust never bypasses this check or AuraGo's existing tool gates.
Sender labels, @recipient tags, greetings and place names are ordinary radio
formatting, not threats on their own. The full text is still scanned. An old
quarantined message stays protected until an administrator reviews the cause
and requests a new check; it is never replayed automatically.

Channel replies run in a fresh minimal context. They use public knowledge and
optionally **native Brave web search**, capped at two individual calls, without
MCP redirection. They cannot access shell, Python, files, skills, general HTTP,
MCP, delegation, missions or messaging tools. Node/display names and unsigned
channel senders never authorize commands. Signed-plain and room-forwarded
messages are also excluded from trusted direct commands. Slash commands are
not passed to the global command handler.

Tool-call syntax is never sent as a radio answer. If a model writes an XML/JSON
call as text while search is available, it gets one correction to use the native
interface or provide a plain-text answer. Repeated invalid output is blocked;
tool-free security checks and final summaries reject it immediately. The two-call
search limit and destination permissions remain enforced.

Reasoning text is removed even when its opening tag is missing. An exact
`NO_REPLY` after cleanup sends nothing and creates no outgoing Messenger entry.

Other messages and blocked input remain in the protected inbox and do not
create system notifications in the general chat. At the next direct user
contact, the agent receives counts, validated source prefixes or channel
numbers, and inbox references; external message text is not injected into its
privileged context. Administrators can inspect text and request a new security
check. Already attempted commands and unknown outcomes cannot be retried
through this action.

The Settings inbox is an independently scrollable view of the latest 100
records, paginated in groups of 25. Older records remain subject to the existing
retention settings and are not deleted by this display limit.

Device identity changes or changed channel assignments block automatic work
until explicitly confirmed again. Bindings use a local keyed fingerprint of
the device and channel data. Raw channel secrets never reach normal API
responses, tool output or logs. An explicit administrator invitation export is
the sole browser exception described below; the device's configured BLE PIN
remains excluded. Changing permissions
cancels current work before new settings are published and suppresses pending
replies. Cancellation cannot undo a system operation or radio transmission
that already completed.

## Information provided on agent wakeup

Every admitted direct or channel message includes structured reception context:
message ID, message type, sender prefix/resolved key or unverified channel sender
label, channel slot/name/kind, sender timestamp, AuraGo queue-retrieval time,
Companion frame type/size, V3 SNR in dB, routing mode and known flood hop count.
Multi-byte repeater hashes are decoded correctly; the encoded path byte is also
retained. Forwarded messages retain their four-byte sender prefix but still
cannot authorize commands.

Authorized direct messages additionally include the reception-time contact and
receiver snapshot: contact name/type/flags, advertised coordinates, advertisement
and modification times, cached outgoing path hashes/hops, local public key/name,
firmware/build/manufacturer, protocol version/capacity/repeat settings, configured
position, transmit power, frequency, bandwidth, spreading factor, coding rate
and advertisement/telemetry settings. Public channel replies receive only their
own message/channel context, without local hardware/position or contact records.

Inbox metadata uses additive JSON fields; Messenger history uses the separate
schema upgrade described below. Old records
retain unknown metadata. Collection reuses existing local Companion reads and
does not send telemetry requests or discovery traffic over the mesh. PINs and
channel secrets remain excluded. Names and positions are external data, never
instructions or authorization.

SNR measures the final radio link. RSSI, incoming repeater identities and per-hop
measurements are not supplied by queued text frames and remain unknown. The
direct-route marker `0xFF` does **not** mean zero hops; a cached outgoing path
does not establish the incoming route. Sender time can drift, queue retrieval
may be delayed, and contact positions/routes may be stale. No propagation
latency is inferred from these timestamps.

## Reliability and operations

- `data/meshcore.db` (under `directories.data_dir`) stores versioned inbox,
  review, processing and send state. Defaults: seven days, 1,000 retained
  messages, a 128-entry queue, two automatic runs per node/channel per minute,
  twelve overall. Overflow remains in the inbox without automatic work.
- Direct commands must be at most 600 seconds old; 120 seconds of future clock
  skew is tolerated. Both values are configurable. Old commands remain readable.
- Runs are atomically reserved before execution. Startup marks interrupted
  processing/sends as `outcome_unknown`; pending work requires a new
  administrative check. Execution tombstones survive inbox eviction for 48
  hours, longer than the maximum configurable command age; a full 65,536-entry
  ledger refuses new automatic work.
- Automatic channel replies carry the fixed AI disclosure prefix `[AuraGo KI]`.
- Replies occupy at most three numbered, UTF-8-safe packets. Text is rejected
  if it cannot fit; it is never silently truncated. The channel sender-name
  bytes reduce available payload space. Transmit pacing is six packets/minute.
- `device_accepted` means the local device accepted a send; `delivered` requires
  a matching direct-message acknowledgement. Channels have no recipient
  acknowledgement. Partial/interrupted sends can be `outcome_unknown` and are
  never automatically retransmitted. Radio firmware may itself emit protocol
  acknowledgements; receive-only means no automatic application reply.
- Reconnects resynchronize contacts/channels and drain messages. Push events
  and bounded 15-second polling trigger further reads. BLE requires an adequate
  negotiated MTU; potentially truncated frames fail closed.

The administrative API is `/api/meshcore/{status,devices,contacts,channels,messages}`
(GET) and `/api/meshcore/{scan,pair,test,recheck}` (POST). All routes require
administrator access. The message endpoint exposes only the latest 100 records;
pagination uses `limit` (up to 100) and `offset` within that window.
Connection/security failures use the Operational Issues lifecycle.

The `meshcore` agent tool supports `status`, `contacts`, `channels`,
`send_direct` (`node_key`, `text`) and `send_channel` (`channel`, `text`). It has
no raw protocol, key management, flashing, pairing or radio settings operations.

## Desktop Messenger

Open **MeshCore** from the virtual Desktop. It reuses the server's Companion
connection; the browser does not connect to USB or Bluetooth. Connection,
identity and agent permissions remain under **Connection** (`/config#meshcore`).

- Direct conversations and channels offer search, favorites, unread counts,
  muting and history search. Narrow windows switch between the conversation
  list and chat with **Back**. Existing windows are reused across Spaces;
  session restoration and notifications reopen the selected conversation.
- **Enter** sends; **Shift+Enter** adds a line. Drafts stay in browser storage
  per device and conversation. The UTF-8 byte counter and packet preview show
  the maximum three numbered parts before sending. Manual sending does not
  invoke the agent and does not require proactive agent permission.
- Delivery states distinguish sending, device acceptance, confirmed direct
  delivery, not sent and uncertain outcomes. Channels never claim recipient
  acknowledgement. HTTP retries reuse a durable request ID. **Send again**
  requires confirmation because another copy may reach the recipient.
- Protected messages initially show placeholders. **Show protected text**
  reveals sanitized plain text for this open conversation only; it neither
  approves the message nor executes it. Message text is never rendered as HTML.
- Add contacts using their complete public key/name/type or a MeshCore contact
  link. Share public contact details as links/QR codes. **My node** explicitly
  announces in direct range or through the mesh; this can also broadcast any
  location already configured on the radio. Repeater, Room and sensor contacts
  are labelled but have no device-management controls.
- New channels default to the hashtag type: entering `#bot` as the name or on
  its own in the invitation field creates that hashtag channel. Selecting
  **Public** fixes the name to `Public`.
  Create/join public, hashtag or private channels in free slots only. Private
  keys are randomly generated unless explicitly supplied as 32 hexadecimal
  characters. Contact/channel changes are verified by another device read.
  New channels receive no agent permissions; removing contacts revokes trust.
  Uncertain edits remain locked until explicit mapping confirmation, which
  resets channel automation to receive-only.
- **Share → Show invitation** is an explicit administrative export. Private
  invitations expose the channel key only in the current dialog; responses
  use `Cache-Control: no-store`. They never enter notifications, logs, browser
  storage or agent tools. Copying requires a click. Close the dialog to remove
  the invitation. QR images can be imported where native `BarcodeDetector`
  exists; pasting an invitation works everywhere. Unsupported region options
  are rejected rather than silently ignored.

Messenger history defaults to **90 days and 10,000 messages total**, adjustable
in its Settings or `meshcore.history_days` / `meshcore.history_messages`. The
protected inbox retains its separate seven-day/1,000-message defaults. Clear
history removes visible chat text but keeps execution reservations and the
short-lived security inbox. Device/contact identities and channel fingerprints
keep old conversations separate after device or slot changes. Legacy ambiguous
prefixes remain unknown; missing historical delivery evidence stays unknown.
Schema upgrades create a private `meshcore-v1-*.backup.db` or
`meshcore-v2-*.backup.db` next to an existing database; administrators manage
these backups. Schema 3 retains reception snapshots separately from the inbox
and stores nullable per-packet routing and ACK duration. Backfill uses only
surviving, uncleared messages with exactly matching IDs and bindings; it never
recreates deleted history or guesses old data. Generated replies have no
incoming reception metadata. Protected sender labels remain hidden with their text.
Manual request tombstones persist independently of history, with a 65,536-entry
safety ceiling. A full ledger refuses new sends and requires maintenance.

Administrative API routes under `/api/meshcore/messenger/`:

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `bootstrap`, `conversations` | Status, conversations, unread counts, retention settings |
| GET | `messages?conversation=ID&before=SEQ&q=TEXT` | Up to 50 messages, stable exclusive sequence cursor |
| POST | `send` | `{id, conversation, text}`; returns reserved ID with HTTP 202 |
| POST | `conversation` | `{conversation, read?, favorite?, muted?, clear?}` |
| POST | `reveal` | Explicit protected-body read `{id}` |
| POST | `invitation` | Explicit export `{identity, conversation}`; `self` shares own contact |
| POST | `manage` | Identity-bound contact/channel actions and announcements |
| POST | `settings` | `{revision, history_days, history_messages, allow_device_settings?, allow_remote_diagnostics?}` via the existing config file; revision from bootstrap |
| GET | `device` | Current identity, settings revision, capabilities and timestamped local measurements |
| POST | `device-settings` | `{identity, revision, section, values}`; sections `identity`, `contacts`, `radio`, `clock`, `reconcile` |
| POST | `diagnostics` | `{identity, target, kind}` (`telemetry` or `path`); HTTP 202 with a job ID |
| GET | `diagnostics?id=ID` | Progress/result for an explicitly requested diagnostic |

All routes require administrative access. Writes enforce same-origin requests;
messages and invitations are uncached. Desktop events contain metadata only,
and reconnects reload state. Muting has no effect on agent inbox notifications.

## Device page and settings

**My node** opens firmware, capacity, position, radio and local measurements.
The page reads battery voltage, storage, uptime, queue/errors, packet counts,
airtime and the last device RSSI/SNR when supported. Measurements have their own
timestamps, refresh at most every 30 seconds while visible, and never imply
per-message RSSI or battery percentage. Message details retain sender/retrieval
times, SNR, known hops, frame data and reception snapshots. Device contact flags
and favorites are separate from Messenger favorites and AuraGo trust.

**Settings** contains independent Save/Discard sections. App history limits and
the two grants live in AuraGo configuration; device values live on the radio.
`meshcore.allow_device_settings` and `meshcore.allow_remote_diagnostics` default
to `false`. These grants add no agent capabilities. Connection, pairing, identity
confirmation and agent permissions stay at `/config#meshcore`.

Editable device values include name, coordinates and advertisement location
sharing; contact admission/filtering and basic/location/sensor telemetry modes;
radio frequency, bandwidth, SF/CR, power, extra ACKs and supported repeat/path-hash
options. Clock synchronization is a separate explicit action. Contact distance
0 means unlimited; the UI displays stored values 1–64 as 0–63 hops. Repeat mode
must use a frequency range reported by the device. Optional unsupported commands
are shown as unavailable without granting writes. Coordinates do not trigger an
advertisement and do not alter AuraGo's public location description.

Radio edits require review of old/new values. Each save rechecks identity,
connection session and settings revision, then reads values back. A conflict
preserves the draft; refresh and compare before discarding it. Commands are not
atomic: failures stop remaining writes and lock automation as `settings_uncertain`.
Review the actual values and explicitly **Accept actual device values** to
reconcile. This clears only the settings lock and preserves channel permissions.

**Fetch telemetry** and **Discover path** on contacts transmit only when clicked
and enabled. One diagnostic runs at a time, uses the shared six-packet/minute
limit, and waits at most 60 seconds without retry. Responses are session/target/tag
bound; ambiguous or late responses are discarded. Each path query renews the
connection before another query. The memory cache holds at most 128 jobs for ten
minutes. A timeout does not establish unreachability or denied telemetry access.
Positions and sensors appear only when actually returned with their units.

## Validation and sources

Run `go test ./internal/meshcore ./internal/security ./internal/agent ./internal/config ./internal/server`
and `go test ./ui -run 'ConfigMeshCore|DesktopMeshCore'`. Browser checks run with
`AURAGO_RUN_BROWSER_SMOKE=1` and Chrome/Edge. UI bundles are checked with
`node scripts/build-ui-bundles.js --check`. Practical USB tests on all three
operating systems and BLE tests on native Linux remain required before claiming
hardware support is verified.

Wire fixtures follow [firmware MyMesh.cpp at revision 0679dbe](https://github.com/meshcore-dev/MeshCore/blob/0679dbeffc504d562d2f09eb072fdc223f8ffc2a/examples/companion_radio/MyMesh.cpp),
with [Companion documentation](https://github.com/meshcore-dev/MeshCore/blob/0679dbeffc504d562d2f09eb072fdc223f8ffc2a/docs/companion_protocol.md)
and [payload security properties](https://github.com/meshcore-dev/MeshCore/blob/0679dbeffc504d562d2f09eb072fdc223f8ffc2a/docs/payloads.md).
