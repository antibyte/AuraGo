# Quick Connect RDP: Files tab (SFTP / SMB)

Status 2026-11-25: plan drafted, not started. User decisions (do not re-ask): D7 strong-caller gate as planned; D4 existing RDP devices off, new RDP devices auto; D6 optional separate file-access credential.

# Part (a): How SFTP works today

## Routes and guard
- **Routes:** `internal/server/server_routes.go:629-646` registers `/api/desktop/sftp/{list,stat,mkdir,delete,rename,copy,move,upload,download}`. Each is built as `desktop.HandleSFTPX(s.InventoryDB, s.Vault, s.Logger)` and wrapped in `withDesktopRemoteGuard(s, "desktop_sftp_<op>", "", handler)`. The SSH route is at :622, the VNC route at :624, and the RDP ticket, pin-reset and gateway routes at :625-628.
- **Protocol check:** for SFTP the guard's `expectedProtocol` is `""`, so the guard accepts any device protocol. The protocol is enforced later, in `resolveSSHAccess` (`ssh_proxy.go:333-336`), which rejects anything that is not `ssh`.
- **Guard behaviour:** `internal/server/desktop_remote_security.go:21-93`.
  - Operation mapping is at :28-33. `desktop_sftp_list`, `desktop_sftp_stat` and `desktop_sftp_download` are read. SSH, VNC, RDP connect and the RDP ticket are execute. Everything else defaults to write.
  - Rate-limit lockout keys are per action, per IP and per IP plus device (:40, :190-200).
  - Multipart requests without `device_id` are deferred, and scoped tokens are refused for them (:46-57).
  - The guard checks the device exists, the protocol, and the device/tag scope (:63-88), and audits each attempt (:90, :218-230).
  - Lockout failures are recorded only for guard rejections, never for failures inside the handlers.
- **Strong caller:** `desktopRemoteStrongCaller` (:100-115) is true for a cookie session with auth enabled, or a bearer token with `desktop:admin`. It is false when auth is disabled.
- **Audit adapter:** `desktopRDPAudit` (:118-130) turns the desktop audit log into a `desktop.RDPAuditFunc`.

## Handlers (`internal/desktop/sftp_handlers.go`)
- **Connection:** `connectSFTP` (:31-64) calls `inventory.GetDeviceByID`, then `resolveSSHAccess`, then `buildSSHConfig(user, secret, logger, nil)`, then `ssh.Dial`. It has no context, so aborted browser requests do not cancel the dial. It then creates an `sftp.NewClient`. Every request opens a fresh connection.
- **Path handling:** `normalizeSFTPRemotePath` (:66-98) works on paths relative to the remote home: `/` becomes `.`. It rejects `..`, `~`, NUL, UNC paths and drive letters. Sensitive Linux roots are blocked by the list at :110.
- **Listing format:** entries use `sftpEntry` (:119-125: name, size, mode, mod_time, is_dir). The list response is `{"entries":[...]}` (:202).
- **Device ID check:** JSON mutations must carry the same `device_id` in the body as in the query; `validateSFTPDeviceID` (:134-148) enforces this.
- **Operations:**

| Operation | Lines | Notes |
| --- | --- | --- |
| list | :157 | |
| stat | :207 | |
| mkdir | :249 | |
| delete | :295 | stat, then RemoveDirectory or Remove |
| rename | :354 | |
| copy | :406 | read the file and write it back through the connection |
| move | :472 | a rename |
| upload | :524 | multipart, 50 MiB cap (`maxSFTPUploadBytes`, :23) |
| download | :591 | streamed with `io.Copy`; Content-Disposition filename is not escaped |

## How the SSH credential is resolved
`resolveSSHAccess` (`internal/desktop/ssh_proxy.go:332-389`):
- Host comes from `device.IPAddress`, or `device.Name` if empty. Port is the device port, default 22.
- If a credential is linked, `credentials.GetByID` is called. The credential's Host and Username override the device's (:354-359).
- For the secret, the certificate/key vault entry is preferred over the password (:360-367). The secret is read with `vault.ReadSecret`.
- `buildSSHConfig` (:394-426) tries `ssh.ParsePrivateKey(secret)` and falls back to password auth (:397-402).

## Host key handling
- **When `remote.InsecureHostKey` is set:** host keys are ignored (:406-408).
- **Otherwise:** keys are checked against `~/.ssh/known_hosts` (`defaultKnownHostsFile`, :428-434) via `knownHostsCallback` (:436-465):
  - Unknown key (`KeyError` with empty `Want`) and a prompter is set: prompt, then `promptAndStoreHostKey` and `appendKnownHostKey` (:467-506).
  - Changed key: always an error.
  - known_hosts file missing and no prompter: an error telling the user to run ssh-keyscan.
- **Terminal path:** the terminal (`HandleSSHProxy`, :157-159) passes `promptSSHHostKey` (:293-328) as the prompter. The prompt goes over the WebSocket with `{type:"host_key_prompt", code:"unknown_host_key", host, key_type, fingerprint}` and the browser answers with `host_key_decision`.
- **SFTP path:** SFTP passes `nil` as the prompter, so it fails on unknown keys.
- **Dialing:** `dialDesktopSSH(ctx, addr, cfg)` (`internal/desktop/ssh_dial.go:11-35`) can be cancelled through its context; only the terminal uses it.

## RDP
- `splitRDPUsername` (`rdp_proxy.go:178-184`) turns `DOMAIN\user` into domain and user.
- `HandleRDPTicket` (:201-257) releases the password only when `allowSecret(r)` is true.
- `resolveRDPTarget` (:443-470) returns host, port (default 3389), username and credential without reading the vault; the linked credential's Host overrides the device host. `resolveRDPAccess` (:474-496) also reads the password (credential password, or the device's vault secret).
- The gateway uses only `resolveRDPTarget` (:627). The destination sent by the client is ignored.

## Inventory and credentials
- **Devices:** `internal/inventory/inventory.go` has `DeviceRecord` (:16-29) with no JSON or extension field. The schema is at :47-59, column migrations through `pragma_table_info` at :66-92, and `inventorySchemaVersion = 4` at :99.
- **Why new columns are costly:**
  - `CreateDevice` takes 12 positional parameters (:192).
  - The column list is repeated in `deviceSelectColumns` (:436) and in hand-written SELECTs (:371, :386).
  - Rows are scanned in two places (:356, :424).
- **Side-table pattern:** `internal/inventory/rdp_cert_pins.go` keeps a lazily created table keyed by `device_id`. `DeleteDevice` (:297-309) removes the pin. Backups copy the whole `inventory.db` file (`internal/server/backup_security_changes.go:165`).
- **Credentials:** `internal/credentials/credentials.go:16-33` holds `PasswordVaultID` and `CertificateVaultID`. The types are `ssh` and `login`; the Quick Connect dialog creates `login` credentials for VNC and RDP.

## Device API
- **Handlers:** `internal/server/device_handlers.go`. List is at :14 (`?quick_connect=1`, admin only, :35-41), create at :80, update at :134. Both create and update decode `inventory.DeviceRecord` with plain `json.Unmarshal`, so unknown fields are ignored (`internal/server/json_request.go:50`).
- **Quick Connect list:** `quickConnectDevices` wraps each device in `quickConnectDevice{DeviceRecord; QuickConnect}` (`internal/server/device_quick_connect.go:16-52`).
- **UI path:** the UI calls `/api/desktop/integrations/devices` and `/api/desktop/integrations/credentials`, which proxy to `/api/devices` (`internal/server/desktop_integration_routes.go:12-57`).

## UI
`ui/js/desktop/apps/quickconnect-launchpad-chat.js`:
- **Tabs:** markup at :47-51; `switchTab` at :247-256 returns early unless `connectedProtocol === 'ssh'`.
- **Connect functions:** `connectToDevice` :696; `connectSSH` :793 (shows the tabs, :803-806); `connectVNC` :913 (hides them, :924-927); `connectRDP` :1000-1113 (hides them, :1010-1013, and mounts `.vd-qc-vnc-session.vd-qc-rdp-session` with a `.vd-qc-vnc-container.vd-qc-rdp-container` inside).
- **File panel:**
  - `closeSFTPPanel` and `openSFTPPanel` are at :1115-1199. Opening hides `.vd-qc-term-container` and `.vd-qc-vnc-container` with `display:none`, so the RDP toolbar would stay visible.
  - Listing goes through `createSFTPNavigator` (`quickconnect-sftp-navigator.js:17-55`), which reads only `resp.entries`.
  - The breadcrumb root is hard-coded as `/` (:1203).
  - The other operations: download through an anchor (:1296), upload with `fetch` (:1307-1324), and delete, rename, mkdir, copy and move through `api()` (:1326-1383).
- **Errors:** `api()` errors carry `err.status`, `err.body` and `err.retryAfter` (`ui/js/desktop/core/desktop-foundation.js:1108-1113`).
- **Device dialog:** `showServerModal` is at :491-684 and `updateModalForProtocol` at :584-592. Saving puts the credential first, then the device. The PUT body at :667 sends no `tags` or `mac_address`.
- **RDP client** (`ui/js/desktop/apps/quickconnect-rdp.js`): `connect` is at :115-232. Its ResizeObserver (:204-216) calls `desktopSize(container)`, which clamps a zero width or height up to the minimum. So hiding the container with `display:none` would resize the remote desktop to the minimum size. `ui.setVisibility(true)` is called at :199. The toolbar is at :234-264.
- **Bundle and tests:**
  - Bundle order is in `scripts/build-ui-bundles.js:78-88`.
  - `ui/desktop_quick_connect_test.go:311-400` checks markers in the built `js/desktop/main.js`, 16-language translations and the CSS.
  - `ui/desktop_qc_sftp_i18n_test.go` checks the SFTP strings.
  - `quickconnect-launchpad-chat.js` is already exempt from the 1100-line budget (`ui/desktop_js_line_budget_test.go:21`); a new file must stay under 1100 lines.

## Existing SMB client (`internal/smbclient/client.go`)
- `Dial` (:53-78) uses NTLM, requires message signing, and offers SMB 2.0.2 to 3.1.1 only (no SMB1). It requires a password.
- `ValidateTarget` (:89-100) rejects a host containing `:`, so IPv6 literals and ports are not allowed.
- `List` (:104-130) caps at 10000 entries and skips symlinks.
- `Open` (:147) hands ownership of the whole session to the returned reader, which does not suit several operations on one session.
- `cleanPath` (:202) rejects a leading `/`, `\` and `:`.
- go-smb2's `Share` provides Create, OpenFile, Mkdir, Remove, Rename, Stat, Lstat and ReadDir (module `share.go:124-941`), and `File.ReadFrom(ctx, r)` (`file.go:731`). The `Dialer` has no "require encryption" flag (`dialer.go:36-67`).
- There is no Go SMB server available for tests.

# Part (b): Design decisions

**D1. Keep the existing endpoints and put the backend behind an interface.**
- The frontend keeps calling `/api/desktop/sftp/*` with the RDP device id.
- Reasons: the guard actions, scopes, rate limits and audit names stay exactly as they are; the UI code and existing tests are reused; no new routes are needed except the trust endpoint.
- New file `internal/desktop/remote_files.go`:
```go
type remoteFS interface {
    Backend() string   // "sftp" | "smb"
    RootLabel() string // "~" for SFTP, share name for SMB
    ReadDir(ctx context.Context, p string) ([]sftpEntry, error)
    Stat(ctx context.Context, p string) (sftpEntry, error)
    Mkdir(ctx context.Context, p string) error
    Remove(ctx context.Context, p string, isDir bool) error
    Rename(ctx context.Context, from, to string) error
    Open(ctx context.Context, p string) (io.ReadCloser, int64, error)
    Create(ctx context.Context, p string) (io.WriteCloser, error)
    Close() error
}
var remoteFSOpener = openRemoteFS // test seam
```
- Handlers call `remoteFSOpener(r, deviceID, deps)` with `r.Context()`.
- The list response adds `backend` and `root_label`: `{"entries":[...],"backend":"smb","root_label":"C$"}`. Old clients ignore the extra fields.
- `connectSFTP(deviceID, db, vault, logger)` keeps its current signature for `sftp_mutations_integration_test.go:100`; it delegates to a new `dialSFTPClient(ctx, host, port, user, secret, logger, hk *hostKeyCapture)`, which uses `dialDesktopSSH`.

**D2. Resolve the file target from inventory only.**
- New `resolveFileTarget(device, db, vault, settings)` returns `{backend, host, port, username, domain, secret, share}`.
- `ssh` devices: unchanged, via `resolveSSHAccess`, always SFTP on the device port.
- `rdp` devices:
  - Host comes from `resolveRDPTarget` (device IP, or the linked credential's Host as today).
  - The SFTP port comes from the settings (default 22), never from the device port (3389).
  - SFTP secret: the credential's key if present (same preference as SSH), else its password, else the device's vault secret.
  - SMB: password only. The username is split with `splitRDPUsername`; `user@domain` stays as the username.
- `vnc` and `none` devices: error code `file_access_unsupported`.
- Nothing about the host, port or share comes from the request.

**D3. Store the per-device setting in a side table, not in new columns.**
- New file `internal/inventory/device_file_access.go`, modelled on `rdp_cert_pins.go`:
```sql
CREATE TABLE IF NOT EXISTS device_file_access (
  device_id TEXT PRIMARY KEY, mode TEXT NOT NULL DEFAULT 'off',
  sftp_port INTEGER NOT NULL DEFAULT 22, smb_share TEXT NOT NULL DEFAULT 'C$',
  credential_id TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL)
```
- Type: `FileAccess{Mode, SFTPPort, SMBShare, CredentialID}` with JSON fields `mode`, `sftp_port`, `smb_share` and `credential_id`.
- Functions: `Ensure…`, `Get…` (missing row means mode off), `Set…` (upsert), `Delete…`, `ListDeviceFileAccess(db) map[string]FileAccess`, and `NormalizeFileAccess`.
- `NormalizeFileAccess` rules:
  - mode must be one of off, auto, sftp, smb.
  - port must be 1 to 65535.
  - share is trimmed, 1 to 80 characters, has no `/ \ NUL CR LF`, and is not `IPC$` (case-insensitive).
  - credential_id is trimmed.
- `DeleteDevice` also deletes the row.
- Reasons: there is no JSON field on devices; a column would change the 12-parameter `CreateDevice` (agent tools call it) and four SELECT/scan sites; no schema-version bump is needed; backups already cover the table.

**D4. A missing row means off.**
- Existing RDP devices make no new connections to ports 22 or 445 until the user opts in.
- The dialog defaults to `auto` for new RDP devices.
- The Files tab is always shown for RDP. When file access is off, the panel says so and offers an "Edit device" button, so the feature is still easy to find.

**D5. "auto" means SFTP first, then SMB.**
- SFTP is chosen if the SFTP port returns an `SSH-` banner within 1.5 s; otherwise SMB if TCP port 445 connects.
- The result is cached in memory per device for 5 minutes (key: device id, host and settings) and dropped on a dial failure.
- If neither answers, the error code is `file_access_unavailable`. Ubuntu Desktop with GNOME has no sshd by default.
- Reasons: GNOME hosts usually have no Samba; Windows hosts with OpenSSH get SFTP, which is encrypted.

**D6. Optional separate credential for file access (`credential_id`).**
- GNOME Remote Desktop uses its own RDP username and password, which usually differ from the Linux account, so the RDP credential would fail on SFTP.
- An empty value means "same as RDP login".
- The override credential's Host is ignored; the target is always the RDP device's host.
- The handler checks that the credential exists when the device is saved.

**D7. Never send a password to the browser, and gate RDP file access to strong callers.**
- The handlers read the secret from the vault on the server.
- For RDP devices, file access requires `desktopRemoteStrongCaller`. Error code `file_access_admin_required`, HTTP 403.
- Reason: today a stored RDP password is only usable by strong callers, because the gateway never uses it. Without the gate, a device-scoped write token could use the stored password on the server to reach C$, which it cannot do today.
- SSH devices keep today's behaviour.
- The gate is passed in through an options struct as a variadic parameter, so existing calls and tests still compile. With no options given, RDP file access is refused (fail-closed):
```go
type RemoteFilesOptions struct { AllowStoredRDPSecret func(*http.Request) bool; Audit RDPAuditFunc }
func HandleSFTPList(db *sql.DB, vault *security.Vault, logger *slog.Logger, opts ...RemoteFilesOptions) http.HandlerFunc
```

**D8. SFTP host-key trust (shared by SSH and RDP devices).**
- **Normal operations:** pass a capture prompter that records the key and answers `(false, nil)`. The handshake is rejected before authentication, so the secret is never sent to an unverified host. Wrap the callback to flag `KeyError` with a non-empty `Want` as "changed".
- **Error mapping:**
  - Unknown key: 409 `{error, code:"unknown_host_key", host, key_type, fingerprint}` (same field names as the terminal prompt).
  - Changed key: 409 `{code:"host_key_changed"}`, with no override from the UI (same rule as the terminal).
- **Trust endpoint:** `POST /api/desktop/sftp/hostkey?device_id=…`, body `{device_id, fingerprint}`, guard action `desktop_sftp_hostkey_trust`, mapped to execute like `desktop_ssh_connect`.
  - The server resolves the SFTP target (for auto mode the resolved backend must be SFTP, else 409 `file_access_unavailable`).
  - It dials with no auth methods and a prompter that accepts only when `ssh.FingerprintSHA256(key) == fingerprint`. A match is stored through `promptAndStoreHostKey`; a mismatch returns 409 `host_key_mismatch`.
  - The expected authentication failure afterwards is ignored if the key was accepted.
  - No secret is used, so no strong-caller gate is needed here.
  - On success it audits `desktop_sftp_hostkey_trusted` with host and fingerprint.
  - When `remote.InsecureHostKey` is set, it returns ok immediately.

**D9. Back off after an authentication failure (RDP devices only).**
- After an SSH "unable to authenticate" error or SMB `STATUS_LOGON_FAILURE` (0xC000006D; check for `x/protocol.ResponseError` and fall back to matching the error text), refuse new attempts for 30 s.
- Response: 429 `file_access_auth_backoff` with a Retry-After header.
- A success clears the backoff.
- Reason: every click logs in again, and repeated failures could trip a Windows account lockout policy.

**D10. Extend the SMB client rather than writing a second one** (`internal/smbclient/files.go`):
- `Mkdir`, `Remove` (file or empty directory, no-follow), `Rename`.
- `WriteFrom(ctx, path, io.Reader) (int64, error)` using `File.ReadFrom`.
- `OpenFile(ctx, path) (io.ReadCloser, int64, error)`, which closes only the file.
- `Config.Port` (0 means 445) through a custom `TransportDialer`, so tests can use other ports.
- `Config.MaxListEntries` (0 means 10000); the desktop uses 50000.
- All methods reuse `cleanPath` and `rejectSymlinkPath`. Bookshelf keeps using only the read methods.
- Update the package comment (it currently says "read-only").

**D11. UI: show the tab strip for RDP, with tabs "Desktop" and "Files".**
- When Files is open, the whole `.vd-qc-rdp-session` is hidden and `session.setVisible(false)` is called; switching back reverses both.
- In `quickconnect-rdp.js`, add `setVisible(on)` (calls `ui.setVisibility`) and make the ResizeObserver skip when `container.clientWidth === 0 || container.clientHeight === 0`.
- New logic goes into a new bundled file `ui/js/desktop/apps/quickconnect-files.js` exposing `window.QuickConnectFiles`: the dialog section markup, wiring and reading; the host-key trust flow; and mapping error codes to messages. Register it before `quickconnect-launchpad-chat.js`.

# Part (c): Ordered tasks

Before each edit, run GitNexus `impact` upstream (CLAUDE.md). Treat UNKNOWN as unresolved and confirm with grep. Run `detect_changes` before each commit, and commit after each task.

Symbols to check: `connectSFTP`, `resolveSSHAccess`, `buildSSHConfig`, `knownHostsCallback`, `promptAndStoreHostKey`, `HandleSFTP*`, `resolveRDPTarget`, `DeleteDevice`, `handleCreateDevice`, `handleUpdateDevice`, `quickConnectDevices`, `withDesktopRemoteGuard`, `smbclient.Dial` and `List`, plus in JS `switchTab`, `openSFTPPanel`, `closeSFTPPanel`, `connectRDP`, `connectSSH`, `showServerModal`, `renderSFTPBreadcrumb`, `loadSFTPList` and `QuickConnectRDP.connect`.

**T1. Inventory side table.**
- Files: new `internal/inventory/device_file_access.go`; edit `inventory.go` `DeleteDevice` (:297-309) to add `_ = DeleteDeviceFileAccess(db, id)`.
- Tests (new `device_file_access_test.go`): missing row gives off with defaults; set/get round trip; upsert overwrites; `NormalizeFileAccess` rejects a bad mode, port 0 or 70000, an empty share, `a/b`, `IPC$` and CR/LF, and trims; `DeleteDevice` cascades; `ListDeviceFileAccess` returns a map.

**T2. Device API and Quick Connect list.**
- File: `internal/server/device_handlers.go`.
  - Decode into `struct{ inventory.DeviceRecord; FileAccess *inventory.FileAccess `json:"file_access"` }`.
  - Normalize and validate before writing the device (400, and no device created or updated on bad input). An override credential that does not exist gives 400.
  - After create or update: if protocol is rdp and `FileAccess` is not nil, call `Set`. On update, if protocol is not rdp, call `Delete`. A nil `FileAccess` leaves the row unchanged, so other clients do not wipe it.
- File: `device_quick_connect.go`. Add `FileAccess *inventory.FileAccess `json:"file_access,omitempty"`` to `quickConnectDevice`. Fill it for RDP devices in `handleListDevices` (quick_connect branch, :35-41) from `ListDeviceFileAccess`, defaulting to off.
- Tests (`internal/server/device_handlers_file_access_test.go`): create an RDP device with file_access and check it is stored; invalid input gives 400 and no device row; an update without file_access keeps it; changing protocol to ssh deletes it; the quick-connect list includes it only for RDP.

**T3. SMB client extensions.**
- Files: new `internal/smbclient/files.go`; `client.go` (Port, MaxListEntries, a `TransportDialer` that dials `JoinHostPort(host, port)`; check go-smb2's `TCPDialer` in the module's transport file first).
- Tests: path validation of each new method without a server (traversal, `:`, `\`, root not allowed for Mkdir/Remove/Rename); the Port default; the MaxListEntries default. Keep `client_test.go` green.

**T4. File-backend abstraction with SFTP refactor (no change for SSH devices).**
- Files: new `internal/desktop/remote_files.go` (interface, `RemoteFilesOptions`, `openRemoteFS`, `sftpFS`, error types and the `writeRemoteFSError` mapper); edit `sftp_handlers.go` so all nine handlers use `remoteFSOpener` and `r.Context()`; `connectSFTP` delegates to `dialSFTPClient` with `dialDesktopSSH`.
- Host-key capture as in D8. Keep `normalizeSFTPRemotePath` as the single path normalizer for both backends; the SMB backend additionally runs smbclient's checks. Keep the JSON error shape `{"error": …}` and add `code` where one applies.
- Tests:
  - The existing `TestSFTPMutationHandlersSucceedOverLocalSSHAndSFTP` must pass unchanged.
  - New: with the same harness but no known_hosts entry, list returns 409 `unknown_host_key` with the fingerprint, and the server never received an authentication attempt (count auth callbacks in the test server).
  - New: a known_hosts entry with a different key gives 409 `host_key_changed`.
  - New: the list response carries `backend:"sftp"`.

**T5. RDP resolver, SMB backend, strong-caller gate and backoff.**
- Files: new `internal/desktop/remote_files_rdp.go` (`resolveFileTarget`, the auto probe and cache, the backoff store) and `remote_files_smb.go` (`smbFS` over smbclient; RootLabel is the share).
- Tests:
  - Table test of `resolveFileTarget`: off gives disabled; vnc gives unsupported; sftp uses port 22 with the key preferred over the password; smb splits `LAB\bob` into LAB and bob; smb without a password errors; the override credential's Host is ignored while the RDP device host is used; the credential's Host override on the device credential still applies.
  - Strong-caller gate: with no options, an RDP device gets 403; with `AllowStoredRDPSecret` returning true it passes; an SSH device is unaffected.
  - Auto mode: a local listener that sends `SSH-2.0-x\r\n` gives sftp; no banner but an open SMB test port (via a port variable) gives smb; nothing open gives `file_access_unavailable`; the cache expires.
  - Backoff: after an injected authentication error the next call gets 429 with Retry-After; a success clears it.
  - Handler tests with a fake `remoteFS` through `remoteFSOpener`: list (`backend smb`, `root_label C$`), download headers and Content-Length, upload through `Create`, delete of a directory versus a file.
  - An SFTP-over-RDP integration test reusing the T4 harness (RDP device, file_access sftp, `sftp_port` set to the test server's port).

**T6. Trust endpoint.**
- Files: new `internal/desktop/remote_files_hostkey.go` (`HandleSFTPHostKeyTrust(db, vault, logger, opts...)`); `internal/server/desktop_remote_security.go:31` (add `desktop_sftp_hostkey_trust` to the execute group).
- Tests: POST with the matching fingerprint stores a known_hosts line and a following list works; a wrong fingerprint gives 409 `host_key_mismatch` and nothing is stored; GET gives 405; mismatched body and query device_id gives 400.

**T7. Route wiring.**
- File: `internal/server/server_routes.go:629-646`. Build `filesOpts := desktop.RemoteFilesOptions{AllowStoredRDPSecret: func(r *http.Request) bool { return desktopRemoteStrongCaller(s, r) }, Audit: desktopRDPAudit(s)}`, pass it to all nine handlers, and add `mux.HandleFunc("/api/desktop/sftp/hostkey", withDesktopRemoteGuard(s, "desktop_sftp_hostkey_trust", "", …))`.
- Tests: extend `desktop_rdp_gating_test.go`. An RDP device's `/api/desktop/sftp/list` through the real mux with a scoped bearer token gives 403 `file_access_admin_required`; with a cookie session it reaches the opener (stub it). The guard maps the trust action to execute.

**T8. UI.**
- New `ui/js/desktop/apps/quickconnect-files.js` (`window.QuickConnectFiles = (() => {…})()`, same style as `quickconnect-rdp.js`, no console output and no storage):
  - `sectionMarkup(t, device, credentials)`: a `vd-qc-form-section` with `data-qc-file-access` containing: a mode select (off/auto/sftp/smb); an SFTP port field (shown for auto and sftp); an SMB share field (shown for auto and smb; default `C$`); a credential select ("Same as RDP login" plus the cached credentials); and a hint.
  - `wire(overlay, protocolSelect)`: show the section only for rdp; toggle fields by mode.
  - `read(overlay)` returns the `file_access` object.
  - `handleListError(err, ctx)`: on `unknown_host_key`, call `showConfirmModal` with the existing `desktop.qc_host_key_prompt_*` keys, then POST `/api/desktop/sftp/hostkey?device_id=…`, then reload. Other codes map to the new strings below.
- `scripts/build-ui-bundles.js`: register it before line 88. Rebuild with `node scripts/build-ui-bundles.js` so `js/desktop/main.js` is updated (the UI tests read the bundle).
- `quickconnect-launchpad-chat.js`:
  - `showServerModal`: insert the section after the credential section, wire it in `updateModalForProtocol`, and add `file_access` to the POST (:675) and PUT (:667) bodies when protocol is rdp.
  - Add `setTabLabel(protocol)`: Terminal for SSH, Desktop (`desktop.qc_tab_desktop`) for RDP. Call it in `connectSSH`.
  - `connectRDP` (:1010-1013): set `qcTabs.hidden = false`, show the Files tab, and keep `activeRDPHandle = session` (clear it in `showEnd` and `closeRemoteSession`).
  - `switchTab` (:247-256): allow `ssh` or `rdp`. For rdp: on files, call `activeRDPHandle?.setVisible(false)` then open the panel; on the desktop tab, close the panel and call `setVisible(true)`.
  - `openSFTPPanel` and `closeSFTPPanel`: also hide and show `.vd-qc-rdp-session`. If the device's `file_access.mode === 'off'`, render the off hint with an "Edit device" button that calls `showServerModal(device)`.
  - `loadSFTPList` error branch (:1228-1231): delegate to `QuickConnectFiles.handleListError`.
  - `renderSFTPBreadcrumb` (:1203): use the root label from the response.
  - `createSFTPNavigator`: store `nav.backend` and `nav.rootLabel` from the response.
  - Add `vd-qc-sftp-smb` to the panel to hide the permissions column for SMB.
  - Optional fix of an existing bug: the PUT at :667 omits `tags` and `mac_address`, so editing a device in Quick Connect wipes them, and tag-scoped `desktop:remote:tag:` tokens depend on tags. Send `tags: existingDevice.tags || []` and `mac_address: existingDevice.mac_address || ''`.
- `quickconnect-rdp.js`: add `setVisible` to the handle (:146-157) and the zero-size guard in the observer (:205-214).
- `ui/css/desktop-app-quick-connect.css`: styles for the file-access section, the off/hint panel, and the SMB permissions-column hide rule; reuse the `.vd-qc-sftp-*` styles (:1156 onward).
- Tests (`ui/desktop_quick_connect_test.go`, new `TestDesktopQuickConnectRDPFilesContract`):
  - Bundle markers: `window.QuickConnectFiles`, `/api/desktop/sftp/hostkey?device_id=`, `file_access`, `desktop.qc_tab_desktop`, `setVisibility(`, `data-qc-file-access`.
  - The new file is registered in the builder.
  - The new file must not contain `console.` or `localStorage`.
  - Every new key is present in all 16 languages and differs from English.
  - Keep `TestDesktopQuickConnectRDPContract` green.

**T9. Translations.** Add these keys to all 16 files in `ui/lang/desktop/*.json` (cs da de el en es fr hi it ja nl no pl pt sv zh), with real translations:

| Key | English |
| --- | --- |
| `desktop.qc_tab_desktop` | Desktop |
| `desktop.qc_file_access` | File access |
| `desktop.qc_file_access_off` | Off |
| `desktop.qc_file_access_auto` | Automatic (SFTP, then SMB) |
| `desktop.qc_file_access_sftp` | SFTP (SSH server) |
| `desktop.qc_file_access_smb` | SMB share (Windows) |
| `desktop.qc_file_access_sftp_port` | SFTP port |
| `desktop.qc_file_access_smb_share` | SMB share |
| `desktop.qc_file_access_credential` | Credential for file access |
| `desktop.qc_file_access_same_credential` | Same as RDP login |
| `desktop.qc_file_access_hint` | Files are reached on the same host over SFTP or SMB. The password stays on the AuraGo server. |
| `desktop.qc_files_off` | File access is off for this device. |
| `desktop.qc_files_edit_device` | Edit device |
| `desktop.qc_files_admin_required` | File access to RDP devices needs a signed-in administrator session. |
| `desktop.qc_files_unavailable` | No SFTP or SMB service answered on this host. |
| `desktop.qc_files_host_key_changed` | The SSH host key of this host has changed. File access is blocked until known_hosts is fixed. |
| `desktop.qc_files_host_key_mismatch` | The host key changed while you were confirming it. Try again. |
| `desktop.qc_files_auth_backoff` | The host rejected the stored credential. Try again in {seconds} s. |

The existing `qc_host_key_*` keys are reused.

**T10. Documentation.**
- `documentation/quick-connect-rdp.md`: a new section "File access" covering:
  - The modes.
  - Linux: install `openssh-server`; GNOME's RDP credential differs from the Linux account, so use the file-access credential.
  - Windows: SMB or the OpenSSH Server feature; C$ needs admin rights and is subject to the UAC remote restriction for local accounts; or create a share.
  - SMB signing is mandatory and encryption is server-driven (`Set-SmbServerConfiguration -EncryptData $true`).
  - Trusting the SFTP host key.
  - The 50 MiB upload limit.
  - SFTP is limited to the home folder.
- Update "Limits" (:145, "No file transfer") and add troubleshooting rows for each error code.

**T11. Live acceptance (manual or env-gated).**
- A Samba container on local Docker Desktop (see the "Local Docker sidecar probe" memory) with an env-gated test, e.g. `AURAGO_SMB_TEST_ADDR`, `_SHARE`, `_USER`, `_PASS`, covering list, upload, rename, delete and download.
- A real Windows RDP host with SMB C$ and a custom share.
- A GNOME Ubuntu host with and without sshd.
- In a browser: Files tab with RDP connected, switch back without the remote desktop resizing, and typing in the rename prompt must not reach the remote session.

# Part (d): Risks and open questions

Please put questions 1, 2 and 7 to the user; they decide the defaults.

1. **Strong-caller gate (D7).** RDP file access will not work on installs with authentication disabled, or for device-scoped tokens. This matches the RDP password-release rule. Needs user confirmation.
2. **Default off for existing RDP devices (D4).** The alternative is auto. Needs user confirmation.
3. **SMB encryption.** Signing is enforced, but go-smb2's `Dialer` has no "require encryption" option. Unless the share or server sets EncryptData, file contents cross the LAN signed but unencrypted. Possible follow-up: refuse unencrypted sessions if go-smb2 exposes the session state (check `Session` and `Share`).
4. **NTLM only.** NTLMv2 works today. Windows Server 2025 and future Windows releases may block NTLM. go-smb2 has `auth/kerberos.go` for a later domain/Kerberos option. Windows Hello PIN-only and Microsoft accounts need the account's real password (Microsoft accounts use the e-mail address).
5. **C$ access.** It needs local admin rights. For local, non-domain admin accounts, UAC remote restrictions strip the admin token, so access to C$ is denied unless `LocalAccountTokenFilterPolicy=1` is set. Document a dedicated share as the recommended alternative.
6. **IPv6.** `smbclient.ValidateTarget` rejects `:`, so devices with IPv6 literal addresses cannot use SMB. SMB uses port 445 only (no setting).
7. **GNOME hosts.** There is no sshd by default, and the RDP credential is not the Linux account. The plan includes the file-access credential override (D6); confirm it is wanted.
8. **RDP idle timeout while on the Files tab.** The gateway closes the RDP session after `IdleTimeout` (default 5 min) without browser input (`rdp_proxy.go:747`). Long browsing in Files can drop the RDP session; the Reconnect button covers it. Document it, or later send keep-alives.
9. **Keyboard capture.** It is not verified that `iron-remote-desktop` stops taking key presses after `setVisibility(false)` and `display:none`. Check this in the browser in T11.
10. **One login per request.** Every SFTP or SMB call dials and authenticates again: SMB adds roughly 100 ms on a LAN, and the auth-backoff (D9) is there because every click logs in again. A short-lived session cache is a possible follow-up.
11. **Large files.** Uploads are capped at 50 MiB (multipart is buffered). Downloads stream; check that the main listener's write timeout does not cut long downloads. The Content-Disposition filename is not escaped (existing issue, :630); consider `mime.FormatMediaType`.
12. **SMB listing cap and links.** SMB `List` skips symlinks and junctions (for example `Documents and Settings`). The desktop cap is raised to 50000 entries; WinSxS-sized folders may still hit it and get an error message.
13. **Path rules on SMB.** The SFTP path normalizer also applies on SMB: drive letters, UNC paths and `..` are rejected, `/etc`-style roots are blocked (harmless on Windows), and `:` is rejected, which also blocks alternate data streams.
14. **Existing bug.** Editing a device in Quick Connect wipes its tags and MAC address (PUT at :667). This affects tag-scoped tokens. The fix is optional in T8.

# Part (e): Tests to run

Use the Go cache on D: per memory, with no GOTMPDIR for `internal/server`. Use `-overlay` if other sessions have untracked broken files. Group `-run` patterns as `(?i)(a|b)` and count the PASS lines.
- `go test ./internal/inventory/...`
- `go test ./internal/smbclient/...`
- `go test ./internal/desktop/ -run '(?i)(sftp|rdp|ssh|knownhost|remotefiles|filetarget|hostkey)'`
- `go test ./internal/server/ -run '(?i)(DesktopRemote|RDP|QuickConnect|Device|Bookshelf|IsAllowedDeviceProtocol)'` (Bookshelf because `smbclient` changes)
- `node scripts/build-ui-bundles.js`, then `go test ./ui/ -run '(?i)(QuickConnect|Sftp|LineBudget|i18n|SecurityLint|MainLoader|SemanticChunk)'`
- `go vet ./internal/desktop/ ./internal/inventory/ ./internal/smbclient/ ./internal/server/`
- The full `Desktop.*Browser` suite is optional; it needs `-timeout 30m`.
