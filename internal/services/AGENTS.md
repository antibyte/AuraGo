# Services

## Purpose

Background services and workspace search.

## Ownership

`internal/services` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Workspace Search System
- `internal/services.WorkspaceSearchService` maintains a Pure Go resident index for the full agent workspace derived from `directories.workspace_dir`; it must stay single-binary friendly with no CGO, mmap, FFI, or fsnotify dependency.
- The native `workspace_search` tool exposes `find`, `grep`, `glob`, `recent`, `rescan`, and `status`. Keep legacy `file_search` JSON shapes compatible when delegating to the resident index.
- Do not persist file content for workspace search. Only frecency/access metadata belongs in `data/workspace_search.db`.

## Verification

- Run `go test ./internal/services` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
