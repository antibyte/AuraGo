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
- The boringd patch series in `patches/` (0001–0009) applies in sorted filename order. `0009-volume-retention.patch` adds `PATCH /v1/volumes/{id}`, persistent/locked volumes, 423 for locked deletes, and 413 for oversized saves. `workspace_assets.go` pins the patched `boringd/volumes.go` (`e6c427d759f361b217d49b4f7e21ff5ac37b83b2e279a35cc3a0114a0b1a8309`) and `boringd/storage.go` (`3d7d010a642e08359527ae9fb132e605e3ecf83a37112a2175ebd16293103c1f`) with `sha256sum -c`. Adding 0009 changes the fingerprint; verify `TestWorkspaceAssetsArePinnedAndComplete` and `TestWorkspaceVolumeRetentionPatchIsPinned`.

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
- Ledger v5 (`migrateToV5`) adds `volumes.persistent/locked/used_bytes/file_count`, `machines.label/volume_id`, and `workspaces.origin`. A v4 ledger is copied to `.v4.bak` before migration. `UpsertMachine` must not touch `label` or `volume_id`; use `SetMachineLabel`/`SetMachineVolume`. Verify `TestLedgerMigratesToV5WithBackup`.
- Refresh and import use `mergeUpstreamVolume`/`TrackUpstreamVolume`: local `Name`, `Format`, `Availability` and `StorageEpochID` win; expiry, quota, persistent, locked, used bytes, file count and raw data come from boringd. `UpsertVolume` keeps a non-empty stored name. Verify `TestListTrackedVolumesPreservesNameAndTakesUpstreamRetention`, `TestVirtualComputersVolumeImportPreservesLocalFields`.
- Creation uses `CreateVolumeWithOptions`; `persistent` and `locked` are sent only when true, so an unpatched boringd receives the pre-0009 body. `UpdateVolume` sends only non-nil pointer fields. Verify `TestClientVolumeRetentionContracts`.
- `PATCH /volumes/{id}`: retention fields require `availability=available`, the retention capability, and `ttl_seconds <= VolumeMaxTTL()`; a name-only change needs none of these. Verify `TestVirtualComputersVolumePatchRenamesAndUpdatesRetention`.
- `DELETE /volumes/{id}` is refused with `volume_locked` (ledger `locked`, or upstream 423 via `ClassifyError`) and with `volume_in_use` (`attached_to`) for active workspaces and for boringd-listed machines recorded with the volume. The ledger row is removed only after upstream success. An unavailable ledger returns `storage_unavailable` (503) without contacting boringd. `VolumeInUse`/`VolumeAttachments` fail closed: if boringd's machine list cannot be read, every recorded machine reference counts as attached. The legacy `delete_volume` applies the same in-use rule and refuses ledger-locked volumes before any upstream call; legacy `save_machine` and agent checkpoints refuse locked targets with `volume_locked` (HTTP checkpoint/save pass `AllowLocked`). The agent has no lock, unlock or rename operation. Verify `TestVirtualComputersVolumeDeleteRefusesLockedAndInUse`, `TestVirtualComputersVolumeDeleteFailsClosedWithoutLedger`, `TestExecuteToolRefusesDeletingLockedVolume`, `TestLegacyDeleteVolumeRefusesInUse`, `TestLegacyToolLockedSaveAndCreateVolumeClamp`, `TestCheckpointLockedVolumeAndTTLClamp`.
- Upstream volume views that omit `persistent`, `locked`, or usage fields keep the locally known values (`RetentionReported`/`UsageReported`); an explicit upstream `false` is applied. Verify `TestUpstreamViewWithoutRetentionKeepsLocalState`.
- Retention probe: `ProbeVolumeRetention` sends `PATCH /v1/volumes/vol-0000000000` with `{"locked":false}`. A JSON 404 or 400 means supported; 405 or a non-JSON 404 means unsupported; other errors return `(false, err)` and are not cached. `CachedVolumeRetention` caches successes per base URL for 5 minutes; `ResetVolumeRetentionCache` runs after Install/Repair and in auto-setup. Verify `TestClientVolumeRetentionContracts`, `TestVirtualComputersSetupStatusReportsVolumeLimitsAndRetention`.
- `storage.volume_quota_mb` (64–65536, default 2048) and `storage.volume_max_ttl_days` (1–365, default 30) are normalized in `internal/config`, mapped by `FromAuraConfig`, and written by Install/Repair as `BORING_VOLUME_QUOTA_MB` and `BORING_VOLUME_TTL_MAX` (days × 86400). Changing them needs Install/Repair. Verify `TestVirtualComputersVolumeRetentionDefaultsAndClamp`, `TestSetupInstallWritesVolumeRetentionLimits`, `TestVirtualComputersSetupOptionsCarriesVolumeLimits`.
- `ClassifyError` maps upstream 423 to `volume_locked` (409) and 413 to `volume_quota_exceeded` (413). `writeVirtualWorkspaceError` passes `RESTError` through `ClassifyError`. Checkpoint and legacy import TTL default to `VolumeMaxTTL()`; `CheckpointWithOptions` clamps any TTL above it (the HTTP checkpoint handler rejects it with `volume_ttl_too_long` first), and agent checkpoints stay non-persistent. `CheckpointOptions.AllowLocked` is set only by the HTTP path. The agent's `create_volume` clamps with `clampVolumeTTL`. Verify `TestClassifyErrorMapsVolumeLockAndQuota`, `TestCheckpointUsesConfiguredVolumeTTLAndOptions`, `TestVirtualWorkspaceErrorMapping`.

### Setup single-flight

- Manual Install/Repair holds `virtualComputersManualSetup`; a manual request answers 409 `setup_in_progress` while auto-setup runs (`virtualComputersAutoSetupRunning`), and the auto trigger backs off while the manual mutex is held. Verify `TestVirtualComputersSetupSingleFlightBothWays`.

### Adopted workspaces

- Only `AdoptMachine` creates origin `adopted` rows (`WorkspaceOriginAdopted`, owner `virtual-desktop`, empty mission). The HTTP `POST /machines/{id}/assign-agent` route requires `desktop:admin` scope (`authenticateDesktopPermission`); a write-scope token gets 403. `AdoptMachine` itself requires an admin identity, write access and agent control, and runs the guest handshake before any ledger write. Verify `TestVirtualComputersAssignAgentRequiresAdminScope`. A handshake failure returns `workspace_agent_upgrade_required` and never touches the machine. An active `ready` row for the machine is returned with `reused=true`; a closed, failed or lost row is reopened under its old ID. Verify `TestAdoptMachineCreatesAdoptedWorkspace`, `TestAdoptMachineReusesActiveAndReopensClosedRow`, `TestAdoptMachineHandshakeFailureMapsToUpgradeRequired`.
- Release (`CloseWorkspace` → `releaseAdoptedWorkspace`) never calls guest `workspace.close`, which resets the runtime, closes the managed Chromium, deletes its profile and kills all jobs. It cancels jobs with `job.cancel`, revokes grants with `credential.revoke`, closes browser-session ledger records, and records `workspace_released`. If the GUI input gate cannot confirm that input has stopped, the release returns `desktop_control_unconfirmed` (409), keeps the workspace active with `LastError` set, and never destroys the machine; retry. Verify `TestReleaseAdoptedWorkspaceKeepsMachine`, `TestReleaseAdoptedWorkspaceRefusesWhenGUIRevokeFails`.
- Never call `DestroyMachine` or `ExtendMachine` for an adopted row. `touch` only updates ledger activity. Extending through the admin API moves `MaxExpiresAt` to the machine's expiry instead. Verify `TestTouchDoesNotExtendAdoptedMachine`, `TestVirtualComputersExtendRejectsMalformedJSONAndFollowsAdoption`.
- `DELETE /machines/{id}` requires a successful ledger ownership lookup. Active workspaces return `machine_owned_by_workspace` (409). Unavailable or unreadable ownership data returns `storage_unavailable` (503) before any upstream deletion. Verify `TestVirtualComputersMachineDeleteRefusesWorkspaceOwned` and `TestVirtualComputersMachineDeleteFailsClosedWithoutLedger`.
- An omitted or zero `max_forks` uses the default limit of 3 per operation. Config help explains the zero value in all locales.
- Revoke the GUI gate with `revokeGUI(ctx, cfg, w, false)`. `true` destroys the machine when the guest is silent, so it is not allowed on release.
- Reopening calls `forgetGUI(id)` so a stale `closed` or `blocked` flag cannot reject GUI calls with `desktop_control_changed`.
- Lease reaper: an expired adopted lease goes through `CloseWorkspace` and therefore releases, not destroys. `reconcileLeases` calls `GetMachine` for adopted rows; a machine that is really gone (`workspaceMachineAlreadyGone`) marks the row lost through `markWorkspaceLost`; a transient `GetMachine` error keeps the row `ready` (`TestReconcileKeepsAdoptedWorkspaceOnTransientMachineError`); otherwise `MaxExpiresAt` follows the machine expiry (persistent: +100 years). Verify `TestLeaseReaperReleasesAdoptedWorkspace`, `TestReconcileLeasesMarksAdoptedLostWhenMachineGone`.
- `Open` clamps the lease to the machine expiry and to `MaxWorkspaceSeconds`. Verify `TestOpenClampsLeaseToMaxWorkspaceAndMachineExpiry`.
- Adopted rows count toward `MaxActiveWorkspaces`.
- Native control (`virtual_computer_ui`) is allowed on adopted desktop rows: `DesktopStatus` and `DesktopAction` gate on the desktop template and the `desktop.native.v1` capability, not on origin.

### Legacy virtual_computers tool guard

- `workspaceOwnedMachineGuard` refuses `machineOwnedOps` (`destroy`, `delete`, `extend`, `fork`, `exec`, `screenshot`, `upload`, `download`, `publish`, `save_machine`, `run_shell_task`, `run_desktop_task`) with `machine_owned_by_workspace` while an active workspace owns the machine. The message names the `workspace_id` and points to `virtual_workspace`/`virtual_browser`. `get` and `get_machine` stay allowed. The guard is skipped only when no ledger path is configured. A ledger that cannot be opened or read fails closed with `storage_unavailable`, so the operation does not run. Verify `TestToolWorkspaceGuardFailsClosedOnLedgerError`.
- `fork` uses `defaultMaxForks` (3) when `MaxForks <= 0`. Verify `TestExecuteToolRefusesWorkspaceOwnedMachine`, `TestExecuteToolForkDefaultsMaxWhenUnset`.

## Desktop invocation ownership

- Desktop task submission uses SubmitContext with a server-owned cancellable
  lifetime and drains its completion channel. Revocation closes the controlling
  WebSocket and suppresses late event publication. Authenticated cancellation
  remains available after readonly or agent-task permissions are disabled.

### Native application control

- `virtual_computer_ui` is optional (`agent_control.native_apps_enabled=false`) and main-agent only, bound to the trusted chat/mission and existing desktop Workspace. Launched and adopted desktop Workspaces qualify; Python, co-agent and A2A access stay excluded. Keep the typed `desktop.*` allowlist and `desktop.native.v1` capability aligned with schemas and training.
- The host admission lane and guest generation token serialize browser/native calls per workspace. Takeover, disable and close cancel and drain input before acknowledgement; failed cancellation leaves only the unconfirmed workspace blocked. Config revocation drains workspaces independently and retries blocked lanes on later config saves. Preserve existing browser operation timeouts; only native calls use the native deadline. Recheck live grants and TTL after queue admission.
- Guest CLI arguments are fixed arrays, text input is stdin, output/deadlines are bounded, and cancellation sends SIGTERM to the CLI process group, allowing the upstream two-second input-release budget before a bounded hard fallback. Before acknowledging control, use the pinned upstream status/permission probe to recover abandoned held input and require its private input journal to be empty; otherwise fail closed. Preserve upstream error disposition; never replay uncertain mutations. Private references/state and XDG runtime/journal files remain under `/run/aurago/native-desktop`.
- Desktop images use Debian trixie, AT-SPI >=2.52 and the size/hash-pinned agent-desktop 1.3.1 binaries plus its Apache-2.0 license. Openbox, applications and guest service share one D-Bus session on `:0`. Do not force `GTK_MODULES=gail:atk-bridge`: GTK3 has its own bridge and loading the legacy module crashes its file chooser. A disposable networkless candidate must confirm Mousepad accessibility, typing and a native Save dialog with an independent file read before atomic image replacement. Existing VMs retain their image; repair plus recreation is required. All embedded files affect the fingerprint.
- Screenshot preparation/commit binds a <=30-second image to one guest/window/process/geometry/resolution. Inputs/control changes invalidate it. Coordinate actions require current focus and in-window coordinates; private host image handles are owner/mission scoped and go only to existing Vision. Publish captures only under the current control token while the call is live. Remove tracked host captures on shutdown and only owned capture files from the dedicated directory on startup; never follow symlinks during cleanup. No base64 in tool text or public media registration.
- Status is observational; activity logs contain operation/outcome, no typed content or screenshot. Window content and Vision results remain externally isolated. Operator instructions: `documentation/virtual_computers.md`; agent instructions: `prompts/tools_manuals/virtual_computer_ui.md`.

## Verification

- Run `go test ./internal/virtualcomputers` and the named cross-component checks in the contracts above when those paths change.
- Volume retention and adoption: `go test ./internal/virtualcomputers -run 'TestLedgerMigratesToV5WithBackup|TestClientVolumeRetentionContracts|TestAdopt|TestReleaseAdoptedWorkspace|TestLeaseReaper|TestReconcileLeases|TestTouchDoesNotExtend|TestOpenClamps|TestExecuteTool|TestWorkspaceAssets|TestWorkspaceVolumeRetention|TestToolWorkspaceGuard|TestCheckpointLocked|TestLegacyToolLocked|TestLegacyDelete|TestUpstreamView|TestReconcileKeeps|TestReleaseAdoptedWorkspaceRefuses' -count=1`; server: `go test ./internal/server -run 'TestVirtualComputers|TestVirtualWorkspaceErrorMapping' -count=1`.
- With Chrome or Chromium installed, run `go test ./internal/virtualcomputers/guest_workspace_agent -run TestManagedBrowser -count=20` to cover request-independent browser lifetime, repeated close and immediate profile removal.
- Guest launcher checks: `go test ./internal/virtualcomputers/guest_workspace_agent -run TestDesktopBrowser` on Linux. The opt-in `TestDesktopBrowserReusesAndReopensManagedChromium` requires `AURAGO_DESKTOP_BROWSER_SMOKE=1` in an isolated Debian guest/rootfs with Xvfb, Chromium, writable `/run/aurago` and `/workspace`; never opt in against a live guest's profile.

- Native checks: `go test ./internal/virtualcomputers ./internal/agent ./internal/server ./ui -run 'TestNativeDesktop|TestVirtualComputers|TestVirtualWorkspace|TestWorkspace|TestVirtualBrowser|Test.*Strict'`; Linux guest checks include `TestNativeDesktop` and the existing managed-browser suite. Image activation also runs `verify-native-image.sh` against a disposable guest. Verify both standalone Linux architectures and regenerate/validate the pinned training packs.

## Child DOX Index

None.
