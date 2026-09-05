# MeshCore Companion integration

AuraGo connects to one Companion radio. USB is implemented for Linux, Windows
and macOS; Bluetooth uses BlueZ on native Linux. **Hardware acceptance is still
pending on all platforms.** No firmware flashing, repeater administration or
radio-parameter changes are included.

## Setup

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
7. Proactive sending is a separate opt-in. Enable it and allow individual node
   keys or channels. Automatic replies do not require proactive permission;
   their destination is fixed internally to the incoming node or channel.

Saved channel assignments survive restarts when the device and channel are
unchanged. After the fingerprint calculation update, older assignments may
require **Confirm channel assignment** and **Save** once. Existing histories
remain available; permissions are never automatically transferred to a different
channel binding.

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

Channel replies run in a fresh minimal context. They use public knowledge and
optionally **native Brave web search**, capped at two individual calls, without
MCP redirection. They cannot access shell, Python, files, skills, general HTTP,
MCP, delegation, missions or messaging tools. Node/display names and unsigned
channel senders never authorize commands. Signed-plain and room-forwarded
messages are also excluded from trusted direct commands. Slash commands are
not passed to the global command handler.

Other messages and blocked input remain in the protected inbox. Typed
`meshcore_message` notifications contain fixed metadata only. At the next
direct user contact, the agent receives counts, validated source prefixes or
channel numbers, and inbox references; external message text is not injected
into its privileged context. Administrators can inspect text and request a new
security check. Already attempted commands and unknown outcomes cannot be
retried through this action.

Device identity changes or changed channel assignments block automatic work
until explicitly confirmed again. Bindings use a local keyed fingerprint of
the device and channel data. Raw channel secrets never reach normal API
responses, tool output or logs. An explicit administrator invitation export is
the sole browser exception described below; the device's configured BLE PIN
remains excluded. Changing permissions
cancels current work before new settings are published and suppresses pending
replies. Cancellation cannot undo a system operation or radio transmission
that already completed.

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
administrator access. Message pagination uses `limit` (up to 100) and `offset`.
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
- Create/join public, hashtag or private channels in free slots only. Private
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
The first migration of an existing database creates a private
`meshcore-v1-*.backup.db` next to it; administrators manage these backups.
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
| POST | `settings` | `{history_days, history_messages}` via the existing config file |

All routes require administrative access. Writes enforce same-origin requests;
messages and invitations are uncached. Desktop events contain metadata only,
and reconnects reload state. Muting has no effect on agent inbox notifications.

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
