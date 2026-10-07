# Quick Connect serial access

Quick Connect has two independent serial sources. **This computer** uses the
browser's Web Serial API and opens a device attached to the computer running the
browser. **AuraGo server** opens a device attached to the machine running AuraGo.
When the browser and server are on different machines, choose the source that
matches where the adapter is plugged in.

Web Serial requires a [supported browser and a secure context](https://developer.chrome.com/docs/capabilities/serial): use HTTPS or
`localhost`. The browser opens its device chooser when you connect. A saved
profile keeps connection settings; it does not identify or authorize a USB
device, so the browser chooser remains the authority for device selection.

The AuraGo server source uses Go serial support on Linux, Windows, and macOS.
On Windows, ports appear as COM names; on macOS, select the `/dev/cu.*` device.
Host-side RTS/CTS hardware flow control is unavailable. Browser Web Serial can
offer hardware flow control when the browser and device support it. The UI
reports the selected source's capabilities rather than implying parity.

New connections start at 115200 baud, 8 data bits, no parity, one stop bit,
with DTR and RTS off. Enter sends a carriage return by default. The console
shows ANSI text or hexadecimal receive data, can send text or hex bytes, and
supports local echo, DTR/RTS controls, Break (250 ms), and capture export.
Opening a device can briefly pulse modem-control lines, depending on the OS and
adapter, and may reset attached microcontrollers even with DTR and RTS off.

Each serial port has one owner at a time, including MeshCore. Close its current
owner before connecting from another Quick Connect session. There is no
automatic reconnect. Closing the session, logging out, entering read-only mode,
or revoking its serial capability closes the connection. Serial bytes stay
in the live Quick Connect session and are not forwarded to agent conversations
or application logs. The in-memory capture is capped at 10 MiB and 10,000
records; when older data is dropped, the capture and hex export show a
truncation marker. Export preserves the retained bytes exactly.

Serial access is disabled by default. Enable only the source you need with
`virtual_desktop.serial_browser_enabled` and/or
`virtual_desktop.serial_host_enabled`; read-only mode overrides both. The
console does not reconnect automatically, send data to AuraGo's agent, run
macros or tools, update firmware, or access filesystems.

Server connections require Desktop admin access. Sessions follow the existing
remote maximum duration and idle timeout (60 and 5 minutes by default). Received
data does not count as user activity or extend the browser login session. A lost
Desktop connection closes browser serial sessions; reconnecting the Desktop or
plugging a device back in never reopens the port automatically.

On Linux, the AuraGo service account needs access to the selected device, often
through its `dialout` or `uucp` group; the required group depends on the system.
Windows and macOS may need the adapter manufacturer's driver. A busy port must
be released by its current application. Quick Connect and MeshCore share an
in-process reservation; neither takes over the other's connection.

## Docker device passthrough

For a Docker installation, edit the Compose service that runs AuraGo. Add the
device and the host device group's numeric GID, then recreate the container:

```yaml
services:
  aurago:
    devices:
      - /dev/ttyUSB0:/dev/ttyUSB0
    group_add:
      - "${SERIAL_GID}"
```

Set the variable to the device's host group before running Compose:

```bash
export SERIAL_GID="$(stat -c '%g' /dev/ttyUSB0)"
docker compose up -d
```

Use the actual device path and GID for your adapter. The Compose example grants
access only to that device; do not switch the container to privileged mode.
Changing passthrough requires recreating the container. Without the mapping and
matching permissions, the AuraGo server cannot open that device. Browser-local
Web Serial does not use Docker passthrough. Hardware acceptance still requires
testing with the actual adapter and device.

## Hardware acceptance

Automated tests use simulated ports and browser streams. Before using an adapter
with equipment, verify RX/TX with a suitable loopback at the intended baud and
framing settings, DTR/RTS and the 250 ms Break, unplug during receive and send,
then reconnect explicitly. Repeat for browser and server access as applicable.
Check that closing a window, logout, readonly and grant revocation release the
port. These hardware checks are separate from CGO-free builds and mock tests.
