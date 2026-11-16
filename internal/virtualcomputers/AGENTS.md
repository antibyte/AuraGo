# Virtual Computers

## Purpose

Workspace leases, managed Garage storage, desktop images and shared browser/native GUI control.

## Ownership

`internal/virtualcomputers` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

- Migration backups use `dbutil.BackupSQLite`: owner-only staging and file permissions/Windows ACL, WAL-inclusive copy, sync and atomic publication. Backup failure aborts migration. Keep existing copies; collisions insert a unique suffix before the extension. Restore guidance: `documentation/migration-backups.md`.

## Local Contracts

### Managed host idle behavior

- Managed installs set `BORING_DESKTOP_POOL=0`: create guests on demand rather than inheriting upstream's desktop warm pool and its five-second failed-boot retries. Desktop images remain available for requested machines.
- After stopping boringd, remove empty child cgroups with depth-first `rmdir`. Never use `find -empty -delete`: cgroup controller files are virtual and keep that test false. The kernel rejects removal of populated groups. Verify `TestSetupInstallKeepsMachinesOnDemandAndRemovesEmptyCgroups` and the existing generated Bash syntax check.

### Managed host networking

- Install/Repair must install and enable `boring-net.service` without suppressing errors. The managed `boringd.service.d/aurago-network.conf` drop-in requires and orders `boring-net.service` before boringd so the `boring0` bridge, DHCP/DNS, NAT and firewall exist before guests can boot.
- After stopping boringd, restart the network service before starting the daemon, including when the oneshot service already reports active. A failed network setup aborts repair before daemon startup. Verify `TestSetupInstallRequiresGuestNetworking` and `TestSetupInstallScriptParsesAsBash`.

### Manual machine networking
- `virtual_computers.allow_internet` is the parent gate for manual machine launches. The New computer dialog offers Internet and Offline only when the gate is enabled, defaults to Internet in that case, and sends the explicit per-machine `allow_internet` choice. The server rejects a direct internet-enabled launch while the gate is off; existing offline machines are not silently reconfigured.

### Guest browser lifecycle
- Guest structured file writes and parent creation use one `os.Root` handle.
  Final and swapped parent symlinks cannot redirect writes outside `/workspace`;
  a preflight `EvalSymlinks` alone is insufficient. Verify Linux
  `TestWorkspaceWritesStayRooted` and `TestWorkspaceWriteParentSwapCannotEscape`.
- `guest_workspace_agent/module.txt` and `sum.txt` are the independent module inputs installed on the KVM build host. Refresh both during dependency updates and build the standalone agent for Linux amd64 and arm64; root-module tests alone do not check these pins.
- The managed browser belongs to its session, not the opening RPC deadline. Session close is idempotent and asks Chrome to shut down gracefully with a five-second deadline, then forces process cleanup before removing the profile. Skip the non-idempotent Chrome cancellation when its context or connection has already ended (including a closed desktop window). Startup cancellation still aborts allocation immediately.
- A healthy boringd `/healthz` does not establish Agent Workspace compatibility. With auto-setup and agent control enabled, startup probes the verified install marker plus the running workspace protocol and asset fingerprint asynchronously. Only an incompatible result schedules the full install/repair; compatible restarts skip it. Existing retry and credential gates still apply.

### Desktop image and application launchers
- The pinned, hash-checked `infra/latitude/build-desktop-rootfs.sh` receives `0007-desktop-app-launchers.patch`. Install GTK helpers in its mounted chroot after debootstrap: Openbox settings (`obconf`), PCManFM, Mousepad, session D-Bus, DejaVu fonts, XDG helpers and screenshot support.
- The stock Openbox Applications menu uses `x-terminal-emulator -e`, never the absent `evte`. Super+E opens PCManFM at `/workspace`; Openbox runs inside `dbus-run-session`.
- Browser menu entries, Chromium desktop entries and HTTP(S) file associations use `aurago-browser`. Its `--open-browser` guest-agent invocation contacts only the mode-0600 Unix socket under `/run/aurago`; no TCP port or agent credentials are added. It focuses/navigates the existing managed Chromium or reopens it after closure, retaining the same browser controller/profile contract as structured actions.
- Desktop patch and guest runtime edits change `WorkspaceAssetFingerprint`, invalidating old images. Apply through Install/Repair and start a new VM; running machines retain their old image.

### Virtual Computers Storage / Managed Garage Contract
- Volume refresh retains locally known availability until the remote check gives a newer authoritative result. Lease reconciliation scans every ledger page and atomically claims only the current expired lease before closing a workspace. Activity touches update only activity fields, so delayed RPC replies cannot restore old controls. Expired human control leases are released by a scoped conditional UPDATE (`ReleaseExpiredControlLease`, julianday comparison) that touches only the control columns and `updated_at`; browser control is synced only when that update won.
- Workspace close treats boringd's JSON `404 {"error":"not found"}` as completed deletion, clears stale errors, closes jobs/browser sessions/grants and resolves only that workspace's `lease_close_failed` issue. Router/proxy 404s, authentication failures and server errors remain failures; closed workspaces must leave lease reconciliation.
- Default `virtual_computers.storage.mode` is `managed_garage`; `external_s3` remains supported. Legacy configs without `mode` normalize to `external_s3` when an endpoint is set, otherwise `managed_garage`.
- Managed Garage runs on the boringd control-plane host (`local_host` or `ssh_host`), never as a general AuraGo Compose service. Image is pinned `dxflrs/garage:v2.3.0@sha256:866bd13ed2038ba7e7190e840482bc27234c4afaf77be8cfa439ae088c1e4690`. Only S3 binds `127.0.0.1:3900`. Data lives under `${install_dir}/data/sidecars/garage`.
- Docker is required for managed volumes but is not installed by AuraGo. Missing Docker is a storage warning and must not set core preflight `Supported=false`.
- Managed Garage Vault keys (`virtual_computers_garage_*`) are separate from external S3 keys. Never export them to Python/skills; never show them in the UI. Stop without delete retains container data and source objects.
- Storage identity includes mode, endpoint/bucket/region/SSL and, for managed Garage, control-plane mode/host/install_dir. Source objects are never auto-deleted on switch. `previous_store` ledger volumes must not be removed by normal JSON-404 cleanup.
- Config save must return HTTP 409 (`storage_switch_required`) when identity changes while available volumes exist, unless a single-use `X-AuraGo-Storage-Switch-Token` from `/api/virtual-computers/storage/switch/authorize` matches the target identity hash. Switch-without-migration marks volumes `previous_store` and may stop managed Garage; automated object-copy migration is optional/not required for the gate.
- Agent Docker tools must hide and block lifecycle/inspect/exec/mount access to `aurago-boring-garage` and Garage data paths, same fail-closed pattern as Local LLM.

## Desktop invocation ownership

- Desktop task submission uses SubmitContext with a server-owned cancellable
  lifetime and drains its completion channel. Revocation closes the controlling
  WebSocket and suppresses late event publication. Authenticated cancellation
  remains available after readonly or agent-task permissions are disabled.

### Native application control

- `virtual_computer_ui` is optional (`agent_control.native_apps_enabled=false`) and main-agent only, bound to the trusted chat/mission and existing desktop Workspace. No manual VM, Python, co-agent or A2A access. Keep the typed `desktop.*` allowlist and `desktop.native.v1` capability aligned with schemas and training.
- The host admission lane and guest generation token serialize browser/native calls per workspace. Takeover, disable and close cancel and drain input before acknowledgement; failed cancellation leaves only the unconfirmed workspace blocked. Config revocation drains workspaces independently and retries blocked lanes on later config saves. Preserve existing browser operation timeouts; only native calls use the native deadline. Recheck live grants and TTL after queue admission.
- Guest CLI arguments are fixed arrays, text input is stdin, output/deadlines are bounded, and cancellation sends SIGTERM to the CLI process group, allowing the upstream two-second input-release budget before a bounded hard fallback. Before acknowledging control, use the pinned upstream status/permission probe to recover abandoned held input and require its private input journal to be empty; otherwise fail closed. Preserve upstream error disposition; never replay uncertain mutations. Private references/state and XDG runtime/journal files remain under `/run/aurago/native-desktop`.
- Desktop images use Debian trixie, AT-SPI >=2.52 and the size/hash-pinned agent-desktop 1.3.1 binaries plus its Apache-2.0 license. Openbox, applications and guest service share one D-Bus session on `:0`. Do not force `GTK_MODULES=gail:atk-bridge`: GTK3 has its own bridge and loading the legacy module crashes its file chooser. A disposable networkless candidate must confirm Mousepad accessibility, typing and a native Save dialog with an independent file read before atomic image replacement. Existing VMs retain their image; repair plus recreation is required. All embedded files affect the fingerprint.
- Screenshot preparation/commit binds a <=30-second image to one guest/window/process/geometry/resolution. Inputs/control changes invalidate it. Coordinate actions require current focus and in-window coordinates; private host image handles are owner/mission scoped and go only to existing Vision. Publish captures only under the current control token while the call is live. Remove tracked host captures on shutdown and only owned capture files from the dedicated directory on startup; never follow symlinks during cleanup. No base64 in tool text or public media registration.
- Status is observational; activity logs contain operation/outcome, no typed content or screenshot. Window content and Vision results remain externally isolated. Operator instructions: `documentation/virtual_computers.md`; agent instructions: `prompts/tools_manuals/virtual_computer_ui.md`.

## Verification

- Run `go test ./internal/virtualcomputers` and the named cross-component checks in the contracts above when those paths change.
- With Chrome or Chromium installed, run `go test ./internal/virtualcomputers/guest_workspace_agent -run TestManagedBrowser -count=20` to cover request-independent browser lifetime, repeated close and immediate profile removal.
- Guest launcher checks: `go test ./internal/virtualcomputers/guest_workspace_agent -run TestDesktopBrowser` on Linux. The opt-in `TestDesktopBrowserReusesAndReopensManagedChromium` requires `AURAGO_DESKTOP_BROWSER_SMOKE=1` in an isolated Debian guest/rootfs with Xvfb, Chromium, writable `/run/aurago` and `/workspace`; never opt in against a live guest's profile.

- Native checks: `go test ./internal/virtualcomputers ./internal/agent ./internal/server ./ui -run 'TestNativeDesktop|TestVirtualComputers|TestVirtualWorkspace|TestWorkspace|TestVirtualBrowser|Test.*Strict'`; Linux guest checks include `TestNativeDesktop` and the existing managed-browser suite. Image activation also runs `verify-native-image.sh` against a disposable guest. Verify both standalone Linux architectures and regenerate/validate the pinned training packs.

## Child DOX Index

None.
