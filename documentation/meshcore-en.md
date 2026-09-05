# MeshCore Companion integration

AuraGo connects to one Companion radio. USB is implemented for Linux, Windows
and macOS; Bluetooth uses BlueZ on native Linux. **Hardware acceptance is still
pending on all platforms.** No firmware flashing, repeater administration or
radio-parameter changes are included.

## Setup

1. Install the Companion firmware appropriate for USB or BLE on your device.
2. Open **Settings → MeshCore**. Select the transport, enter the serial port or
   Bluetooth address, enable the integration, and save. USB uses 115200 baud.
   The port list uses basic enumeration, without macOS CGO USB-detail discovery.
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
6. Enable **Reply to trusted direct messages** if desired. For channels, confirm
   the channel assignment and select receive-only, prefix (`!aura` followed by
   whitespace), or question detection. Save the configuration.
7. Proactive sending is a separate opt-in. Enable it and allow individual node
   keys or channels. Automatic replies do not require proactive permission;
   their destination is fixed internally to the incoming node or channel.

For Linux USB, grant the AuraGo service account access to the specific serial
device, commonly through its serial-device group. On macOS use `/dev/cu.*`; on
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
the device and channel data. Raw channel secrets and the device's configured
BLE PIN never reach API responses, tool output or logs. Changing permissions
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

## Validation and sources

Run `go test ./internal/meshcore ./internal/security ./internal/agent ./internal/config ./internal/server`
and `go test ./ui -run 'ConfigMeshCore'`. UI bundles are checked with
`node scripts/build-ui-bundles.js --check`. Practical USB tests on all three
operating systems and BLE tests on native Linux remain required before claiming
hardware support is verified.

Wire fixtures follow [firmware MyMesh.cpp at revision 0679dbe](https://github.com/meshcore-dev/MeshCore/blob/0679dbeffc504d562d2f09eb072fdc223f8ffc2a/examples/companion_radio/MyMesh.cpp),
with [Companion documentation](https://github.com/meshcore-dev/MeshCore/blob/0679dbeffc504d562d2f09eb072fdc223f8ffc2a/docs/companion_protocol.md)
and [payload security properties](https://github.com/meshcore-dev/MeshCore/blob/0679dbeffc504d562d2f09eb072fdc223f8ffc2a/docs/payloads.md).
