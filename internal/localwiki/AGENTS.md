# Local Wikipedia

## Purpose

Offline Wikipedia: one Kiwix ZIM edition, its catalog lookup, download, verification, publication and
daily update check, and the shared `Library` handle that the agent tool, the admin API and the Desktop
app read through `Manager.Acquire`.

## Ownership

`internal/localwiki` owns the manager and `Library`. These contracts also bind
`internal/server/local_wikipedia_*.go` (manager construction, settings sync, admin API),
`ui/cfg/local_wikipedia.js`, `ui/lang/config/local_wikipedia/`, `scripts/test-local-wikipedia-config.mjs`,
the `local_wikipedia` config section (`internal/config/local_wikipedia.go`, `config_template.yaml`), the
`tools.IsSensitiveHostDirectory` helper (`internal/tools/sensitive_host_directory.go`) and their tests.
ZIM and Xapian formats belong to `internal/zim` and `internal/zim/xapian`. `internal/localwiki` must not
import `internal/tools` (the agent tool imports `localwiki`); the server injects the tools-side checks
through `Deps`.

## Local Contracts

### Languages, editions and configuration

- Languages are the 16 UI languages; `languageTable` must equal `config.LocalWikipediaLanguageCodes()`
  (a test pins it). Kiwix names are `wikipedia_<code>_all`, except Norwegian `no` -> `wikipedia_nb_all`
  (ISO 639-3 `nob`, label "Norsk"). Only `nopic` ("without media") and `maxi` ("with media"). The largest
  edition is about 127 GB, hence the UI banner "up to 130 GB". Full-text support per language comes from
  `xapian.NewAnalyzer(<ISO 639-3>).FulltextSupported()`; it is true for all 16 languages today, so the
  title-search-only marker and `fulltext_unsupported` only appear for an edition whose index cannot be
  opened.
- Config `local_wikipedia`: `enabled` (false), `agent_access` (true), `language` ("" = follow
  `agent.system_language` through `i18n.NormalizeLang`, fallback `en`), `variant` (`nopic`), `data_dir`
  ("" = `<directories.data_dir>/wikipedia`) and `update_check` (true). The loader sets the true defaults
  before unmarshalling, `NormalizeLocalWikipediaConfig` repairs an unknown language or variant at load, and
  `ValidateLocalWikipediaConfig` rejects them on save (HTTP 400). `GET /api/config` shows the loader defaults
  for keys an older `config.yaml` lacks (`injectLocalWikipediaDefaults`). No field is secret.
- Docker forces the storage directory to `<directories.data_dir>/wikipedia` (`data_dir_locked`); the UI field
  is read-only there and the server skips the `data_dir` save validation.
- Native installs validate a saved `data_dir`: absolute, not a system location, not AuraGo's own data
  directory root (`validateLocalWikipediaSettings`, same check the manager gets as `Deps.IsSensitivePath`).

### Manager lifecycle and status

- `NewManager` is passive and `Start` does no I/O: the first load of the storage directory runs in the
  background loop. Until it finishes, `Status` reports `loading: true`, `state: not_installed`,
  `readable: false`, `error_code: busy`; `Acquire` returns `ok == false`; `Install` and `Delete` answer
  `ErrBusy` (`Install` answers `ErrDisabled` first when the integration is off). Clients poll until
  `loading` is false. `Shutdown` cancels a running download (its `.part` file stays for "Resume"), stops
  the loop and closes the library once its readers released it. Nothing ever resumes a download by itself.
- Settings reach the manager only through `Configure`. The server calls it from
  `replaceConfigSnapshot` -> `syncLocalWikipediaSettings` after every published config snapshot (config
  save, backup import, ...), under `localWikiSyncMu`, which covers reading the snapshot and
  configuring, so overlapping publications cannot leave older settings behind. Admin handlers never
  configure. Lock order: `Server.CfgMu` -> `localWikiSyncMu` -> `Manager.mu`; inside the package `loadMu`
  -> `stateMu` -> `mu`. `Configure` never starts a download; a changed storage directory is loaded by the
  loop once no operation runs (`loadIfStale`).
- `Status` JSON: `state` (`not_installed|downloading|verifying|ready|interrupted|error`), `progress` (0..1
  fraction), `bytes_done`, `bytes_total`, `rate`, `eta_seconds`, `edition`, `selection`,
  `selection_matches_installed`, `update_available`, `fulltext`, `readable`, `loading`, `free_bytes` (-1
  when unknown), `required_bytes`, `data_dir`, `data_dir_locked`, `operation_in_progress`, `error_code`,
  `recommendation`, `system_language`, `languages`.
- `readable` is true while an installed edition is open and served, in every state. Clients decide whether
  Wikipedia content is available from `readable`, never from `edition != nil` or `state`. An edition that
  is being served is never reported as `error`: a failed install, resume or update ends as `ready` (or
  `interrupted`) with the failed operation's `error_code`, and `readable` stays true.
- Startup problems and operation problems are tracked apart: `loadCode` (why the installed edition could
  not be loaded: `zim_unreadable`) and `errCode` (why the last operation stopped). An operation's code
  outranks the load code and a running operation hides it. `zim_unreadable` therefore means the installed
  edition (delete it) when `readable` is false and an `edition` exists, and the file a download just
  produced (already removed) otherwise; clients derive their wording from `error_code`, `readable` and
  `edition`. `recommendation` is English only and never shown by the config UI.
- `error_code` for an idle manager also covers `busy` (first load), `fulltext_unsupported` (informational:
  ready without a full-text index) and `data_dir_invalid` (the configured directory fails the shape
  check).
- `Acquire` hands out the library only while `enabled` and a readable edition is loaded; callers release
  exactly once (extra releases are ignored). `libraryRef` closes a retired library after its last reader
  released it, then runs the after-close hook (deleting the retired file, which Windows refuses while open).

### Network

- HTTPS only. Catalog `<base>/catalog/v2/entries?name=wikipedia_<kiwix>_all&count=-1` (cached 6 h, 30 s
  deadline, 1 MiB cap, must be an Atom feed); the `.meta4` only from a Kiwix host (`kiwix.org` and
  subdomains) or the catalog host (8 MiB cap, 30 s deadline). Downloads have no overall timeout: every
  attempt has a 2-minute stall watchdog covering the response headers and each body read.
- The `.meta4` supplies the exact size (at most 1 TiB, hostile sizes saturate the disk math), the SHA-256
  and the mirrors: HTTPS without credentials, path ending in `/<file>`, public host, ordered by priority,
  at most 16 (`download.json` refuses more). The load-balancer URL (the `.meta4` URL without `.meta4`) is
  always the last mirror.
- Redirects (at most 10) to non-HTTPS URLs, URLs with credentials or local hosts are refused
  (`httpsOnlyRedirect`). The default client also refuses to connect to loopback, private, link-local and
  other local addresses on the resolved address of every attempt (`dialGuard`, DNS rebinding). Exempt are the
  catalog host (a LAN Kiwix mirror, the tests' fake Kiwix) and proxies chosen from the environment; a client
  injected through `Deps.HTTPClient` keeps its own transport and gets only the redirect policy.
- Every `&http.Client{` is listed in `internal/audit.NetworkClientInventory` (`internal/localwiki/`) and the
  admin routes in `RouteContractManifest` (`/api/local-wikipedia/`, session-admin).

### Install, download and publication

- Downloads start only from `Install` (admin POST): never on a selection change and never automatically
  after a restart (`interrupted` + "Resume"). `Install` checks synchronously: enabled, idle, storage
  directory (below), catalog and `.meta4`, free space. A matching `download.json` is resumed (its mirrors
  are reused); a differing one is stale and its files are removed. The newest catalog edition already
  installed answers `ErrAlreadyInstalled`. The background work is bound to the manager lifetime, not to the
  request.
- Space: free >= missing bytes + max(1 GiB, 1 % of the edition) (`requiredBytes`). Unknown free space needs
  `confirm_unknown_space`. `replace_mode: delete_old_first` counts the installed edition as free and takes
  it out of service before downloading (`detachInstalled`); an `InsufficientSpaceError` carries
  `can_delete_old` so the UI can offer it. During the download the free space is re-checked, with an fsync,
  after every 1 GiB written across all attempts; too little pauses the job as `interrupted` +
  `insufficient_disk_space` with the needed bytes in `required_bytes`.
- Storage directory (`prepareDataDir`): absolute, not sensitive, created if missing, writable (probe file).
  The sensitive check (`Deps.IsSensitivePath`; the server passes `tools.IsSensitiveHostDirectory` plus
  AuraGo's data directory root) runs on the lexical path and on the resolved path (`EvalSymlinks`; on Windows
  `GetFinalPathNameByHandle`) - first on the nearest existing ancestor plus the missing components, before
  `MkdirAll`, so nothing is created inside a protected tree, and again on the directory itself. Links,
  junctions and 8.3 short names cannot point the edition into a system location; the macOS aliases `/var`,
  `/tmp` and `/etc` (resolving below `/private`) are tolerated.
- Download job: `<edition>.zim.part`, SHA-256 while downloading, an existing part is re-hashed first (state
  `verifying`). Range resume: `206` must start at the offset and name the edition size; a `200` is accepted
  only with `Content-Length` equal to the size, no content encoding. A mirror answering `416`, or `200` to a
  Range request, cannot continue and is skipped; the part file is untouched. Failing mirrors move on to the
  next; rounds that make progress repeat; a round without progress ends the job as `download_failed`
  (`interrupted`, part kept). Only after two consecutive rounds without progress in which just such mirrors
  answered (30 s apart) does the job start over, into `<edition>.zim.part.restart`; the restart file
  replaces the part file only once it holds more bytes, so the part file never shrinks, and the free space
  for it is accounted exactly (`restartRequiredBytes`). A restart that ends earlier is discarded.
- A checksum mismatch deletes the part and download.json (`checksum_mismatch`; state `ready` with an installed
  edition, else `error`). `Cancel` stops the job (it waits up to 10 s),
  keeps the part and `download.json` and leaves the state `interrupted`.
- Publish: open the verified part with `OpenLibrary` (header, main page; a missing or unsupported
  `X/fulltext/xapian` only means title search; the analyzer follows the index's own `language` metadata,
  falling back to `M/Language`), `fileutil.RenameContext`, open the final file, write `state.json`, swap the
  refcounted library, remove `download.json`. A file that cannot be read after the download is removed
  (`zim_unreadable`); a `state.json` write failure renames the file back to `.part` for a retry. The previous
  edition is deleted after its last reader released it; `pending_delete` in `state.json` survives restarts
  and is retried after loads, after a swap and hourly (Windows keeps open files locked). Never delete files in
  the storage directory that are not listed there or named by `download.json`; only names matching
  `^wikipedia_[a-z]{2,3}_all_(maxi|nopic)_\d{4}-\d{2}[a-z]?\.zim$` are ever deleted.
- `Delete` refuses while a download runs (`busy`); it removes `download.json` first, then the `.part` and
  `.restart` files, an unpublished download, and retires the installed edition (readers finish first).

### Persistence and crash recovery

- Files: `<name>.zim`, `<name>.zim.part`, `<name>.zim.part.restart`, `state.json`
  (`{"version":1,"edition":{...},"last_update_check":...,"update":{"name","date","size"},"pending_delete":[...]}`)
  and `download.json` (`{"version":1,"target":{...},"urls":[...],"last_url":...,"started_at":...}`). Files with
  a newer `version` are ignored; persisted editions, mirrors and pending deletes are validated on read.
- Load (`loadLocked`) never touches the network. It removes stale restart files (never resumed), reconciles
  `download.json` (`reconcileDownload`), then opens the edition. A crash after `state.json` was written leaves
  a `download.json` of the installed edition: it and the partial files are dropped. A crash between the rename
  and the state write leaves a finished `<edition>.zim` that `state.json` does not name: it becomes the part
  file again, so "Resume" only re-hashes it. Without a readable `state.json` the directory is left as it is
  and the code is `zim_unreadable`.

### Updates

- The loop runs the daily check only when `enabled`, `update_check` is on, an edition is installed and no
  download runs (first check two minutes after `Start`, then hourly ticks, due after 24 h). `CheckUpdate`
  (also the admin button) bypasses the catalog cache and only sets `update_available`, which is persisted in
  `state.json` and dropped when it stops applying (other language/variant, not newer). A re-run suffix
  (`2026-07b`) counts as newer than `2026-07`. An update is a normal `Install` and only starts on a click.
- `selection_matches_installed` compares the installed language and variant with the saved selection; a
  mismatch offers "Install" for the new selection and never deletes the installed edition by itself.

### Admin API and config UI

- `/api/local-wikipedia/{catalog,status,install,cancel,delete,check-update}`: `requireAdmin`, the prefix in
  `isAdminProtectedPath`, GET (catalog, status) and POST (rest), POSTs must be same-origin unless they carry
  a Bearer token, `Cache-Control: no-store`. The install body is one JSON object (at most 16 KiB, no unknown
  fields) with `replace_mode` (`keep_old` default, `delete_old_first`) and `confirm_unknown_space`; success is
  `202 {"status":"accepted"}`. `catalog?lang=` defaults to the system language; an unknown language is 400.
  `check-update` answers the status.
- Errors are `{"error","error_code","recommendation"}` and never carry paths or hosts (details go to the
  log): `insufficient_disk_space` 422 (+ `required_bytes`, `free_bytes`, `can_delete_old`) and
  `data_dir_invalid` 422; `busy`, `disabled`, `free_space_unknown`, `already_installed`, `no_operation` 409;
  `catalog_unreachable` 502; `unknown_language` and `invalid_request` 400; `localwiki_unavailable` 503;
  anything else `localwiki_error` 500. Error codes of `ErrorCode`: `insufficient_disk_space`,
  `free_space_unknown`, `checksum_mismatch`, `download_failed`, `catalog_unreachable`, `zim_unreadable`,
  `fulltext_unsupported` (warning), `busy`, `disabled`, `data_dir_invalid`, `already_installed`,
  `no_operation`, `unknown_language`, `localwiki_error`.
- Config UI: the section derives every error text from `error_code` plus `readable` and `edition` with its
  own 16-locale strings (`config.local_wikipedia.error_*`, `help.local_wikipedia.*` in
  `ui/lang/config/local_wikipedia/`, German with "Du" and real umlauts) and never shows the server's
  `recommendation`. It shows a loading view while `loading` is true, polls every 2 s while an operation,
  the first load or an action is pending, keeps a failed poll apart from action messages, and offers
  Install/Update/Resume/Check/Delete only for saved settings (unsaved changes and a disabled integration
  block them; Delete needs no enabled integration). Questions to the administrator: install/update
  confirmation with size and free space, `free_space_unknown`, `can_delete_old`, delete. `#lw-announce` is the
  section's one live region (state and errors, never progress); re-rendering the status area keeps the focus
  on the same button. The section uses sprite slot 120 and no inline styles.

## Work Guidance

- A change to the status fields, error codes or the `readable`/`loading` semantics updates the manager, the
  server tests, `ui/cfg/local_wikipedia.js`, its Node and browser tests and this file together.
- Keep new work inside the lock order above; never hold `Manager.mu` across file or network I/O. Operations
  start only through `startOperation`, which also sets `deleting`/`op` exclusivity.
- Tests use the fake Kiwix TLS server (`fake_kiwix_test.go`) and the ZIM fixtures of `internal/zim/testdata`; Windows-only behaviour
  (locked files, `GetFinalPathNameByHandle`) is covered by `*_windows_test.go`.

## Verification

- `go test ./internal/localwiki ./internal/config ./cmd/config-merger`
- `go test ./internal/server -run '(?i)(LocalWikipedia)'`, `go test ./internal/audit -run '(?i)(LocalWikipedia|RouteContract|NetworkClient)'`
  and `go test ./internal/tools -run TestIsSensitiveHostDirectory`
- `go test ./ui -run '(?i)(LocalWikipedia|ConfigSidebarIconSprite)'` and `npm run test:local-wikipedia-config`;
  browser: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '(?i)(TestConfigLocalWikipedia)'` and
  `TestConfigRefreshRealSectionsBrowser/local_wikipedia`, `TestConfigRefreshRealSectionsBrowser/matrix/local_wikipedia` and
  `TestConfigRefreshPopulatedBrowser/local_wikipedia` (same variable) cover the topic layout and the width/theme/density matrix
- `npm run build:ui && npm run check:ui` (the config modules are lazy-loaded raw files; no bundle changes)

## Child DOX Index

None.
