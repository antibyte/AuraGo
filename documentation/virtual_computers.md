# Virtual Computers and Boring Computers

AuraGo manages the complete [Boring Computers](https://github.com/michaelshimeles/boring-computers) deployment for the Virtual Computers integration. Users do not need to clone the upstream repository, install Node.js, create systemd services, expose ports, or maintain SSH tunnels themselves.

## Automatic provisioning

Set both options in the Virtual Computers configuration:

```yaml
virtual_computers:
  enabled: true
  auto_setup: true
```

AuraGo then provisions the reviewed upstream revision automatically at server startup. Enabling or changing the integration through the configuration UI triggers the same idempotent setup through hot reload. A generation-aware reconciler prevents overlapping installations, cancels a superseded attempt, and continues with the newest configuration. Failed background attempts wait five minutes before retrying. Disabling the integration cancels pending setup, closes its management tunnel, and removes the drawer's cached state.

Install and Repair manage two components as one deployment:

- `boringd`, the private control plane on `127.0.0.1:18082`
- the Boring Computers management application on `127.0.0.1:18081`

The installer verifies the pinned upstream source, applies AuraGo's reviewed base-path overlay, performs a locked npm build, and writes a revision marker used during startup reconciliation. Every run creates a unique immutable release and switches the `current` link atomically only after a successful build. The `boring-web.service` systemd unit runs with filesystem and privilege hardening. If activation fails, the distinct previous web release is restored. AuraGo installs Node.js privately below the configured Boring Computers install directory and does not replace the host's global `node`, `npm`, or `npx` commands.

## Chat drawer and access

When Virtual Computers is enabled, the right-hand integrations drawer in Chat contains **Boring Computers**. It opens:

```text
/boring-computers/
```

The link is shown only while the integration is enabled. Its status changes from `starting` to `running` after a bounded, passive management health probe succeeds; opening the drawer or status page never initiates an SSH connection. AuraGo requires the normal authenticated session or a method-appropriate Desktop bearer token before proxying the management application. Read-scoped tokens can browse, while mutating requests require write scope. `virtual_computers.readonly=true` blocks mutations at the AuraGo proxy boundary as well as in native tools.

Both HTTP and WebSocket traffic stay on the AuraGo origin. The browser never receives `BORING_TOKEN`, the private boringd URL, or an authorization header for boringd. The management application injects the token only in its server-side proxy.

## Local and SSH-host modes

In `local_host` mode, AuraGo installs and probes both services on the same supported Linux/KVM host. If installation needs authenticated sudo, store the password on the Virtual Computers configuration page. AuraGo reuses the central `sudo_password` Vault secret, including a value previously stored through `/sudopwd` or the Secrets page. Saving it automatically retries an enabled local auto-setup, even after a failed-attempt cooldown, and supersedes an in-flight attempt so the new credential is used. The password is never written to `config.yaml`, command arguments, setup logs, or API responses. Root and passwordless-sudo hosts do not need a stored password.

The `sudo_password` secret is shared with `execute_sudo`, package management, and other privileged host features. Virtual Computers therefore does not offer a delete action for it. Manage removal centrally on the Secrets page, where the system-wide effect is explicit.

In `ssh_host` mode, AuraGo installs both services on the selected remote Linux/KVM host. It maintains separate loopback SSH tunnels for boringd and the management application, reuses healthy tunnels, replaces them when the SSH target changes, and closes partially established tunnels after failed health checks.

Do not publish ports `18081` or `18082`. Remote browser access should expose the authenticated AuraGo server, for example through the existing Tailscale integration.

## Status and troubleshooting

`GET /api/virtual-computers/setup/status` retains the existing `control_plane` configuration object and adds:

- `control_plane_status`: configured and healthy state for boringd
- `management`: configured and healthy state for the Boring Computers web application
- `sudo_password_stored`: safe boolean indicating whether the central Vault secret is available; the value itself is never returned

The passive status request never submits the stored password to sudo. When passwordless sudo is unavailable but a Vault credential exists, `has_sudo_or_root` remains `null` until the explicit Preflight action validates the credential.

If `/boring-computers/` returns `503`:

1. Open Virtual Computers settings and run the status check.
2. Confirm that the selected local or SSH host supports Linux, systemd, KVM, and the configured credentials.
3. Run Repair. This safely re-runs the pinned, idempotent deployment for both components.
4. Check AuraGo's structured server log for the detailed setup or tunnel error. Browser responses intentionally contain only a safe summary.

Never solve a `503` by opening either loopback port, copying the Vault token into browser configuration, or setting `PUBLIC_BORING_URL` to the private control plane.

## Machines, screenshots, and files

AuraGo follows the pinned boringd API contract. Desktop screenshots are read as bounded `image/png` data and exposed as `{mime_type,data_base64}`. The UI offers screenshots only for machines reporting `display=true`; a direct request for a headless machine returns `capability_unavailable`. Uploads accept a filename, send only its safe basename, and report the actual `/root/<filename>` destination. File operations on a disconnected machine return `machine_not_connected`.

Publishing requires a template name. Forking uses `count`; command execution accepts one complete command string rather than a separate argument array. Persistent machines may return an empty expiry timestamp, which AuraGo treats as no expiry.

## Live VNC in Virtual Computers

Display-capable machines expose **Live VNC** in the Virtual Computers app on the virtual desktop. The session opens in the right-hand detail area and provides controls for fitting the remote desktop, 1:1 display, view-only mode, Ctrl+Alt+Del, reconnecting, disconnecting, maximizing the app window, and browser fullscreen. Browser fullscreen includes both the remote display and its toolbar; pressing Esc exits fullscreen without disconnecting the VNC session.

An app window keeps at most one visible VNC session. Selecting a screenshot or another machine, destroying the active machine, or closing the app disconnects it. Normal data refreshes preserve a visible session while its machine still exists, reports `display=true`, and remains writable. Tasks, volumes, and screenshots remain available in the overview and screenshot views, outside the live session. Separate Virtual Computers windows may each maintain their own session.

Live VNC is an interactive desktop channel and therefore requires Desktop write permission. `virtual_computers.readonly=true` disables the Live VNC action and the server rejects direct VNC WebSocket requests with HTTP 403 before opening an upstream connection. Screenshots remain available with read permission. Headless machines never offer VNC.

The browser connects only to AuraGo's existing same-origin `/api/virtual-computers/machines/{id}/vnc` WebSocket endpoint. boringd tokens, private upstream URLs, and authorization headers stay on the server and are never included in browser URLs or UI output. If a VNC server requests browser-side credentials, AuraGo shows a localized authentication error and safely disconnects instead of prompting for a password.

## Persistent agent tasks

Shell and desktop agent tasks use boringd's authenticated WebSocket channels with a URL-encoded `goal`. Starting a task returns its ID immediately. AuraGo stores task state and ordered `say`, `action`, `preview`, `done`, and `error` events in `virtual_computers.db`. At restart, unfinished tasks become `interrupted` and are not retried. Canceling closes the task context without rolling back already executed actions. Native-tool output wraps event text as untrusted external data.

The REST API provides `GET|POST /api/virtual-computers/tasks` and `GET|DELETE /api/virtual-computers/tasks/{id}`. Read-only mode keeps task history readable but blocks starting and canceling tasks.

## Volume storage

boringd intentionally has no global volume discovery endpoint. AuraGo keeps a local ledger of known unguessable volume IDs, verifies them with `GET /v1/volumes/{id}`, removes confirmed missing entries, and marks temporarily unreachable entries stale. `GET /api/virtual-computers/volumes/{id}` imports a known ID. Creation uses a TTL, save attaches a machine snapshot to a selected volume, and launch accepts at most one `volume_id`.

Managed volume storage uses:

```yaml
virtual_computers:
  allow_volumes: true
  storage:
    endpoint: minio.local:9000
    bucket: boring-volumes
    region: ""
    use_ssl: true
```

Store the S3 access and secret keys through the Virtual Computers Vault fields. AuraGo writes the corresponding `BORING_S3_*` environment values during managed setup and never serializes credentials into `config.yaml`. The Storage Test performs an authenticated, read-only bucket HEAD request and does not create buckets or objects.
