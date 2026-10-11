# Remote Control

AuraGo Remote runs on Windows, Linux and macOS (amd64 and arm64). AgoDesk uses
the same device registry and enrollment tokens, with its own local permissions.
Enable Remote Control in Config, configure TLS, and download a personalized
client or generate a pairing token. Tokens expire after one hour and enroll
exactly one device. Approval of a pending device produces a fresh token; a used
token is never restored.

## Transport and upgrades

Both `/api/remote/ws` and `/api/agodesk/ws` require WSS. This also applies to
loopback and Tailscale. The classic client verifies the server certificate with
the operating system trust store; there is no insecure certificate option.
Install a private CA in that store when necessary.

TLS may terminate at an explicitly trusted reverse proxy. Configure AuraGo's
trusted proxy addresses and HTTPS proxy mode; forwarded headers from other
peers cannot grant a secure connection. Automatic downloads preserve the
public hostname and port. Manual URLs accept `wss://` or `https://` and retain
an explicit public port; the internal listener port is never appended.

Classic clients must announce security revision **1** and their capabilities
in their authenticated greeting. Missing or older revisions receive
`CLIENT_UPGRADE_REQUIRED` before enrollment or activation. The signed message
format remains version 2. Update in this order:

1. Provide and verify TLS.
2. Install updated clients and verify their service/configuration paths.
3. Activate the updated server with the revision requirement.

Existing device IDs and shared keys remain usable with an updated client.
To replace an installed WS endpoint, use `--install --supervisor wss://HOST/api/remote/ws`
without `--token`; this retains the existing device identity.
An explicit `--token` starts new enrollment in memory. Failure leaves the
previous identity file intact; success atomically saves the new identity.
Revocation and deletion close all transports, cancel pending work, and prevent
old sessions from affecting a replacement. Re-enrollment requires a fresh token.

## Installation and configuration

Run the downloaded executable with `--install`, or select installation in its
terminal menu. Windows and Linux system-service installation needs appropriate
administrator privileges. macOS installs a per-user LaunchAgent.

| Platform | Service | Protected default configuration |
| --- | --- | --- |
| Windows | SCM `AuraGoRemote` | `%ProgramData%\AuraGo\Remote\config.json` |
| Linux | systemd `aurago-remote` | `/var/lib/aurago-remote/config.json` |
| macOS | LaunchAgent `io.aurago.remote` | `~/Library/Application Support/AuraGo/Remote/config.json` |

Installers save configuration before starting the service. Service arguments
contain an absolute `--config` path, never tokens or shared keys. Existing
`~/.aurago-remote/config.json` configurations are read for migration and remain
available if migration fails. An explicit `--config PATH` selects another
protected configuration. Existing service identities are retained on reinstall.
Pending replacement enrollment keeps its target in `enrollment_url`; the
previous `supervisor_url`, device ID and key remain together until success.
Directories use mode 0700 and files 0600 on Unix; Windows grants access only to
the installing user, Administrators and SYSTEM. Do not place this configuration
in a shared or synchronized directory.

Use the platform service manager to stop/start/restart the installed client,
or `--uninstall` to stop and remove the service. Uninstallation retains identity
and device configuration. `--foreground --config PATH` runs without installing
a service; `--version` displays the build-injected version. A failed replacement
leaves the existing executable intact; after an installation error, check the
service status before retrying.

## Rights and device management

Config lists the last server-observed connection state, version, capabilities,
effective rights and settings acknowledgement. Use **Refresh** at any time;
the visible list refreshes every ten seconds without probing remote hosts.
**Manage device** exposes ReadOnly, allowed paths, revoke and delete.

Effective ReadOnly is the global setting **or** the device setting. Toggling the
global setting does not overwrite the individual setting. A classic device's
nonempty path list overrides the global list. An empty device path list inherits
the global list; if both lists are empty, file and shell access is denied.
AgoDesk retains its own local roots and approval policy and never inherits
classic-client paths.

Saving settings and confirming their application are separate states. Classic
clients ACK the exact configuration message on the current connection. Pending
settings block new work; a missing or failed ACK closes the connection after at
most ten seconds and appears as an error. Changed policies cancel active
classic commands by closing their transport before reconnecting with the new
policy. AgoDesk changes close its session; its state is labelled **server
enforced**, never falsely labelled as client-confirmed.

## Execution and file limits

- A classic client admits at most four concurrent commands and responds with
  `CLIENT_BUSY` before starting additional execution goroutines.
- Cancellation and connection loss cancel execution and terminate shell process
  trees. Socket writes allow at most five seconds or the remaining command
  deadline, whichever is shorter. Timed-out mutations are not automatically
  replayed.
- Active command IDs remain protected from duplicate execution. The last 1,024
  completed IDs retain at most 32 MiB of results. `RESULT_EVICTED` means the
  command completed but its payload expired; the same ID will not run again
  while it remains in that window.
- Text output is capped at 1 MiB with a visible truncation marker and
  `truncated:true`. Binary file reads are never shortened. Unsupported
  operations return `UNSUPPORTED_CAPABILITY`; persistent shell sessions and
  desktop/browser commands require a capable AgoDesk client.
- File reads, writes, text/JSON/YAML/XML edits and recursive search use
  authorized `os.Root` handles. File limits apply to inputs, actual reads and
  final saved content. WebSocket frame limits account for JSON/base64 overhead.
- Edits are serialized per file, staged, synced and atomically published through
  the shared file replacement helper. Cancellation, permission loss and write
  failures preserve the original. Existing Unix mode/ownership and Windows
  DACLs are retained. Shell working-directory validation is not an OS sandbox;
  grant shell access only to devices intended for arbitrary commands.
- CPU, memory, disk and uptime come from gopsutil. `metrics_available` identifies
  measured values; unavailable metrics are JSON `null`, rather than zero.

## Verification boundaries

Local unit, transport, browser and cross-compilation checks do not establish
native service installation or acceptance on another machine. Validate install,
start, stop, restart and uninstall in isolated Windows, Linux and macOS systems.
The external AgoDesk application needs a separate end-to-end acceptance run.
No deployment is required to run these local checks.
