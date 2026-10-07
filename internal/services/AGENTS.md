# Services

## Purpose

Background services and workspace search.

## Ownership

`internal/services` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### File Indexing
- File replacement stages the complete generation before changing the active SQLite pointer. The native vector store owns generation receipts and recovery; the indexer keeps the old KG until publication succeeds. Unknown ownership never permits rollback deletion. `internal/memory/AGENTS.md` owns the shared replacement and visibility contract.
- A failed stat/read during a scan is not proof of deletion. Remove indexed and KG entries only after a confirmed missing-file result. Explicit directory cleanup remains scoped to its requested directory.

### Workspace Search System
- `internal/services.WorkspaceSearchService` maintains a Pure Go resident index for the full agent workspace derived from `directories.workspace_dir`; it must stay single-binary friendly with no CGO, mmap, FFI, or fsnotify dependency.
- The native `workspace_search` tool exposes `find`, `grep`, `glob`, `recent`, `rescan`, and `status`. Keep legacy `file_search` JSON shapes compatible when delegating to the resident index.
- Legacy file-search fallback resolves its glob under the configured workspace, including sibling `skills` and `tools`, never under the install root.
- Do not persist file content for workspace search. Only frecency/access metadata belongs in `data/workspace_search.db`.

## Desktop invocation ownership

- Manual Desktop mission preparation inherits its server-owned invocation and
  uses fileutil.PublishContext for prepared content. A cancelled provider result
  cannot replace the previous valid preparation; failure-status cleanup remains
  allowed. Ordinary scheduled preparation keeps its service lifetime.

## Verification

- Run `go test ./internal/services` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
