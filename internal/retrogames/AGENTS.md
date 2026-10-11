# Retro Games package contract

- Purpose: local index of browser-playable archive.org games (8 fixed sources), FTS5 search, favorites /
  plays / broken markers, embedded Hall of Fame, and a `Service` for the desktop API and the
  `retro_games` agent tool. Spec: `docs/superpowers/specs/2026-10-11-retro-games-design.md` (local).
- Network: only `HTTPFetcher` talks to `https://archive.org/services/search/v1/scrape` through
  `security.NewStrictPublicHTTPClientForURL` (no redirects, no private addresses), 30 s timeout, 32 MiB
  page cap, 1 s pause between requests, 3 attempts per page for 429/5xx/network errors.
- No network or disk access while disabled; the store opens lazily on first use.
- Sync is single-flight. Automatic syncs start when a source never succeeded or is older than 7 days; after
  a failed automatic sync the next automatic one waits 1 hour. `StartSync` always runs.
- A source is replaced atomically in one transaction; failure, an empty answer or a shrink below 50 % keeps
  the old rows (`ErrShrinkGuard`).
- Ownership: an identifier in several sources belongs to the source that inserted it first
  (`ON CONFLICT DO NOTHING`); `item_count` is the real row count.
- Corruption: only SQLite corruption (`SQLITE_CORRUPT`, `SQLITE_NOTADB`, failed integrity check) moves the
  DB to `retrogames.db.corrupt-<UTC time>`; other open errors are returned and never move the file
  (library tables are user data).
- Rate limits: a server wait above 60 s or an exhausted 429 stops the whole run (`ErrRateLimited`) and
  marks the remaining sources failed.
- Shrink: automatic syncs keep the 50 % guard; a user-requested sync (`StartSync`) accepts a shrink only for
  a source whose previous attempt was rejected by the guard.
- Automatic syncs fetch only stale or never-succeeded sources; `ensureFresh` caches the next staleness
  deadline (`freshUntil`).
- Lifecycle: the store opens lazily outside `s.mu` (`openMu`); after `Shutdown` every call returns an
  `ErrDisabled`-wrapped error.
- Write lock: every SQLite write of a `Store` (`ReplaceSource` for its whole transaction,
  `RecordSyncFailure`, `SetFavorite`/`SetBroken`/`RecordPlay`) takes `writeSem`, a ctx-aware one-slot
  semaphore (`lockWrites`); the `*Locked` helpers expect the caller to hold it, never take it again. Only
  `OpenStore` writes without it (schema setup before anyone can reach the store). New write paths must use it.
- `PrepareLibraryChange` takes the write lock after resolving the game and returns `(commit, release, err)`:
  the desktop handler waits for a running sync there, outside the desktop publication gate, and the commit
  inside the gate never waits. Callers `defer release()` (idempotent); `commit` works once and frees the lock;
  a ctx that ends while waiting returns `ctx.Err()` (the handler maps `context.Canceled` to 403
  `desktop_readonly`). Teardown never needs the lock: a sync waiting for it ends with its context.
- Scrape pages are stream-decoded with a cap of 10,000 elements; `FlexText` stream-decodes with depth 3 / 64
  values.
- Identifiers must match `^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$` everywhere (API paths, embed URLs).
- User input never reaches FTS5 syntax: `BuildMatch` keeps letters and digits only and quotes each term.
- Hall of Fame ids were verified live on 2026-10-11; re-verify (metadata API: exists, mediatype software,
  emulator set) before adding or swapping entries. The opt-in live test is
  `AURAGO_RETROGAMES_LIVE=1 go test ./internal/retrogames/ -run TestLive -count=1`.
- Tests: `go test ./internal/retrogames/... -count=1`. Other packages use `retrogamestest` fakes.
