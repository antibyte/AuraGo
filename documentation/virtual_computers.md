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

- `boringd`, the private control plane on `127.0.0.1:18080`
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

In `local_host` mode, AuraGo installs and probes both services on the same supported Linux/KVM host.

In `ssh_host` mode, AuraGo installs both services on the selected remote Linux/KVM host. It maintains separate loopback SSH tunnels for boringd and the management application, reuses healthy tunnels, replaces them when the SSH target changes, and closes partially established tunnels after failed health checks.

Do not publish ports `18080` or `18081`. Remote browser access should expose the authenticated AuraGo server, for example through the existing Tailscale integration.

## Status and troubleshooting

`GET /api/virtual-computers/setup/status` retains the existing `control_plane` configuration object and adds:

- `control_plane_status`: configured and healthy state for boringd
- `management`: configured and healthy state for the Boring Computers web application

If `/boring-computers/` returns `503`:

1. Open Virtual Computers settings and run the status check.
2. Confirm that the selected local or SSH host supports Linux, systemd, KVM, and the configured credentials.
3. Run Repair. This safely re-runs the pinned, idempotent deployment for both components.
4. Check AuraGo's structured server log for the detailed setup or tunnel error. Browser responses intentionally contain only a safe summary.

Never solve a `503` by opening either loopback port, copying the Vault token into browser configuration, or setting `PUBLIC_BORING_URL` to the private control plane.
