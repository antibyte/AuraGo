# Local Wikipedia

## Purpose

Offline Wikipedia: one Kiwix ZIM edition, its catalog lookup, download, verification, publication and
daily update check, and the shared `Library` handle that the agent tool, the admin API and the Desktop
app read through `Manager.Acquire`.

## Ownership

`internal/localwiki` owns the manager and `Library`. These contracts also bind
`internal/server/local_wikipedia_*.go` (manager construction, settings sync, admin API, the read-only Desktop
API under `/api/desktop/local-wikipedia/` with its content route, and the `isStaticAsset` exclusion for that
prefix in `securityHeadersMiddleware`, `internal/server/server.go`),
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
  `ErrBusy` (`Install` answers `ErrDisabled` first when the integration is off). A changed storage
  directory that is being loaded also reports `loading: true` and `error_code: busy` (`Install` and
  `Delete` answer `ErrBusy`), while `state`, `edition` and `readable` still describe the previous
  directory's edition, which stays served until the new directory is published. Clients poll until
  `loading` is false. `Shutdown` cancels a running download (its `.part` file stays for "Resume"), stops
  the loops and closes the library once its readers released it. It honours its context even while a
  load or download is stuck in storage I/O that cannot be interrupted (an unreachable network share): it
  then returns `ctx.Err()`, the stuck goroutine finishes on its own and the library is closed after it.
  Nothing ever resumes a download by itself.
- A hung storage directory must never stall callers. `Manager.mu` is never held across file system or
  network I/O. Loads and cleanups (`loadLocked`, `removeStaleRestartFiles`, `discardPending`) mark the
  storage I/O instead (`ioToken`, set by `holdStorageIOLocked` through `loadIfStaleLocked` or
  `beginStorageIO`): while it is set no operation, `Delete`, load or other cleanup starts, and `Install` and
  `Delete` answer `ErrBusy`. The mark is always released by a `defer` (a load can run in a request
  goroutine, where net/http recovers a panic), and a release never ends a later holder's mark. Request
  paths never wait for
  `loadMu` (`tryLoadIfStale`, `Delete` uses `TryLock`). `Status` never touches the storage directory:
  `free_bytes` comes from a background measurement (`probeLoop`: at start, after a storage directory
  change, after an operation or `Delete`, every 10 s; a measurement running longer than 5 s reports -1),
  and the directory check is lexical (`Deps.IsSensitivePath` must not do I/O). The measuring goroutine is
  not tracked: `Shutdown` never waits for it.
- Settings reach the manager only through `Configure`. The server calls it from
  `replaceConfigSnapshot` -> `syncLocalWikipediaSettings` after every published config snapshot (config
  save, backup import, ...), under `localWikiSyncMu`, which covers reading the snapshot and
  configuring, so overlapping publications cannot leave older settings behind. Admin handlers never
  configure. Lock order: `Server.CfgMu` -> `localWikiSyncMu` -> `Manager.mu`; inside the package `loadMu`
  -> `stateMu` -> `mu`. `Configure` only stores the settings and signals the loops: it never waits for
  `loadMu` or storage I/O and never starts a download; a changed storage directory is loaded by the loop
  once nothing else uses the storage directory (`loadIfStale`).
- `Status` JSON: `state` (`not_installed|downloading|verifying|ready|interrupted|error`), `progress` (0..1
  fraction), `bytes_done`, `bytes_total`, `rate`, `eta_seconds`, `edition`, `selection`,
  `selection_matches_installed`, `update_available`, `fulltext`, `readable`, `loading`, `free_bytes` (the
  background measurement; -1 when unknown, not measured yet or the measurement hangs), `required_bytes`, `data_dir`, `data_dir_locked`, `operation_in_progress`, `error_code`,
  `recommendation`, `system_language`, `languages`.
- `readable` is true while an installed edition is open and served, in every state. Clients decide whether
  Wikipedia content is available from `readable`, never from `edition != nil` or `state`. An edition that
  is being served is never reported as `error`: a failed install, resume or update ends as `ready` (or
  `interrupted`) with the failed operation's `error_code`, and `readable` stays true.
- Startup problems and operation problems are tracked apart: `loadCode` (why the load failed:
  `zim_unreadable` for an installed edition that cannot be opened, `state_unreadable` for a `state.json`
  that cannot be read) and `errCode` (why the last operation stopped). An operation's code outranks the
  load code and a running operation hides it. `zim_unreadable` therefore means the installed edition
  (delete it) when `readable` is false and an `edition` exists, and the file a download just produced
  (already removed) otherwise. `state_unreadable` is a status code only: no `edition` is reported (nothing
  says which one is installed), the state is `error` (`interrupted` when a `download.json` exists), and
  `Install` or `Delete` replace or remove the file. Clients derive their wording from `error_code`,
  `readable` and `edition`. `recommendation` is English only and never shown by the config UI.
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
- `Delete` refuses while a download, a load or a cleanup runs (`busy`); it removes `download.json` first,
  then the `.part` and `.restart` files, an unpublished download, and retires the installed edition
  (readers finish first).

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
  and the code is `state_unreadable` (see "Manager lifecycle and status").

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
  `no_operation`, `unknown_language`, `localwiki_error`; the status alone also reports `state_unreadable`
  (no request fails with it).
- Config UI: the section derives every error text from `error_code` plus `readable` and `edition` with its
  own 16-locale strings (`config.local_wikipedia.error_*`, `help.local_wikipedia.*` in
  `ui/lang/config/local_wikipedia/`, German with "Du" and real umlauts) and never shows the server's
  `recommendation`; `localwiki_error`, `localwiki_unavailable` and `invalid_request` have their own texts
  that point to the AuraGo log. "The update failed" prefixes an operation error only for a real update: an
  edition is served (`readable`), `selection_matches_installed` (the failed install targeted the served
  language and variant), and not for a download paused by `insufficient_disk_space`. It shows a loading view
  while `loading` is true, polls every 2 s while an operation, the first load or an action is pending, keeps
  a failed poll apart from action messages (and announces it during the first load too), and offers
  Install/Update/Resume/Check/Delete only for saved settings (unsaved changes and a disabled integration
  block them; Delete needs no enabled integration and is also offered for `state_unreadable`). Questions to
  the administrator: install/update confirmation with size and free space, `free_space_unknown`,
  `can_delete_old`, delete. `#lw-announce` is the section's one live region (state and errors as sentences
  joined with ". ", never progress). Re-rendering the status area keeps the focus on the same button; when
  that one is gone or disabled the focus moves to the first enabled action button, else waits on the state
  banner (never on the catalog's Retry), and returns to the last action button used once that is enabled
  again. The section uses sprite slot 120 and no inline styles.

### Desktop content and API

- `Library.Content/Random/Main` (`content.go`) read only the archive's content namespace
  (`ContentNamespace()`); redirects resolve with the archive's depth limit and must stay in that
  namespace. `ContentItem.Path` is the resolved path (callers redirect when it differs), the ETag is the
  archive UUID plus a SHA-256 prefix of the path, text MIME types gain `charset=utf-8`. Random skips
  non-HTML title-list entries (8 attempts).
- `/api/desktop/local-wikipedia/{status,suggest,search,random,main,content/<path>}`
  (`internal/server/local_wikipedia_desktop_handlers.go`): `desktop:read`, GET/HEAD only (405 otherwise,
  so no Origin check). Virtual Desktop off 503 `desktop_unavailable`, integration off 503 `disabled`
  (+ `can_manage`), no manager 503 `unavailable`, no open edition 409 `not_ready`. `status` is the
  non-admin subset (state, progress, readable, loading, edition language/variant/date/article count,
  fulltext, update_available, error_code) plus `can_manage` (browser session or `admin` bearer); never
  paths, file names, hashes or free space. Queries <= 200 runes with a 6 s deadline; suggest <= 10 refs,
  search default 20 and at most 30 hits, never leads; `query_too_long`/`query_empty`/`bad_limit` 400,
  `busy` 503 (+ `Retry-After`), `timeout` 504, `search_failed` 500.
- Content paths are lookup keys, never filesystem paths; a redirect answers 302 to the resolved path
  (dot and empty segments refused). Every content answer (blobs, redirects, the framable HTML error pages
  with the `aurago-local-wikipedia-error` marker) carries `localWikipediaContentCSP` (sandbox without
  scripts, `connect-src 'none'`, `frame-ancestors 'self'`), `X-Frame-Options: SAMEORIGIN` set by the
  handler (`securityHeadersMiddleware` keeps `DENY` for every path), `nosniff`,
  `Referrer-Policy: no-referrer` and `Permissions-Policy: attribution-reporting=(), browsing-topics=()`.
  Blobs go through `http.ServeContent` (Range, `If-None-Match`, `If-Range`) with
  `Cache-Control: private, max-age=86400`; the middleware never treats the prefix as a static asset.
- Desktop capability `local_wikipedia` = `local_wikipedia.enabled` and a manager
  (`localWikipediaAvailable`); a config publication that flips `enabled` broadcasts `desktop_changed`
  `app_availability` on its own goroutine, outside the config lock. The app contract lives in
  `ui/js/desktop/apps/AGENTS.md` (Local Wikipedia).

## Work Guidance

- A change to the status fields, error codes or the `readable`/`loading` semantics updates the manager, the
  server tests, `ui/cfg/local_wikipedia.js`, its Node and browser tests and this file together.
- Keep new work inside the lock order above; never hold `Manager.mu` across file or network I/O (mark the
  section with `beginStorageIO` and `defer` its release instead), and never let a request path wait for
  `loadMu`. Operations start only through `startOperation`, which refuses while `op`, `deleting` or the
  storage-I/O mark is set and then sets `op`; `Delete` sets `deleting` itself. The stuck-I/O tests (`manager_io_test.go`) hold the
  `beforeStorageIO` seam and the free-space measurement and check that `Configure`, `Status`, `Install`,
  `Delete` and `Shutdown` answer at once.
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
- Desktop: `go test ./internal/localwiki -run '(?i)(TestArchive|TestContentMimeType|TestLibraryContent)'`,
  `go test ./internal/server -run '(?i)(LocalWiki|LocalWikipedia)'`,
  `go test ./internal/desktop -run TestBuiltinLocalWikipediaAppRequiresCapability` and the UI tests listed in
  `ui/js/desktop/apps/AGENTS.md` (Local Wikipedia)

## Child DOX Index

None.
