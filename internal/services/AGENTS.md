# Services

## Purpose

Background services and workspace search.

## Ownership

`internal/services` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### File Indexing
- Image filenames remain searchable metadata; never append them to the vision
  prompt or allow them to select object-count mode. Decisions hints remain trusted.
  Verify `TestIndexImageFilenameCannotChangePromptOrCountingMode`.
- PDF extraction uses `pdftext.Extract` with the scan context. Limit and
  cancellation errors preserve the old index and never start multimodal fallback.
  Bounds and parser ownership: `../pdftext/AGENTS.md`; verify `TestPDFLimit*`.
- Indexer start and restart inherit the server shutdown context and request-scoped budget tracker; image analysis uses the shared completion accounting path.
- File replacement stages the complete generation before changing the active SQLite pointer. The native vector store owns generation receipts and recovery; the indexer keeps the old KG until publication succeeds. Unknown ownership never permits rollback deletion. `internal/memory/AGENTS.md` owns the shared replacement and visibility contract.
- A failed stat/read during a scan is not proof of deletion. Remove indexed and KG entries only after a confirmed missing-file result, or when the tracked path is now a symlink and therefore no longer an indexable file. Explicit directory cleanup remains scoped to its requested directory.

### Workspace Search System
- `internal/services.WorkspaceSearchService` maintains a Pure Go resident index for the full agent workspace derived from `directories.workspace_dir`; it must stay single-binary friendly with no CGO, mmap, FFI, or fsnotify dependency.
- The native `workspace_search` tool exposes `find`, `grep`, `glob`, `recent`, `rescan`, and `status`. Keep legacy `file_search` JSON shapes compatible when delegating to the resident index.
- Legacy file-search fallback resolves its glob under the configured workspace, including sibling `skills` and `tools`, never under the install root.
- Do not persist file content for workspace search. Only frecency/access metadata belongs in `data/workspace_search.db`.
- Background polls reuse immutable text for readable regular files with unchanged size and modification time. Recheck excludes, rooted paths, binary prefixes and both content budgets on every poll; never mutate an older published index while reusing text.
- Initial scans and explicit `rescan` operations reread contents, including edits that preserve size and modification time. Verify `TestWorkspaceSearchPoll*`, `TestWorkspaceSearchRescanDetectsPreservedMetadata` and `BenchmarkWorkspaceSearchUnchangedPoll`.

## Desktop invocation ownership

- Manual Desktop mission preparation inherits its server-owned invocation and
  uses fileutil.PublishContext for prepared content. A cancelled provider result
  cannot replace the previous valid preparation; failure-status cleanup remains
  allowed. Ordinary scheduled preparation keeps its service lifetime.

## Prompt optimizer

- Canonical manuals remain authoritative on dynamic, explicit and paginated
  reads. Runtime variants appear only as bounded, isolated optional hints, shed
  before canonical guides. Record the candidate exposure only if its complete
  hint survives final fitting; otherwise record the canonical baseline.

- Optimizer manual bodies and historical errors remain isolated user-message
  source data beneath a fixed system instruction. Never interpolate them into
  trusted instructions. Bound output and accept only completed responses;
  operator permissions and tool schemas remain authoritative.
- Optimizer provider requests inherit the service shutdown and budget context.
  Book every physical response once, including invalid helper output before a
  primary fallback. Verify `TestOptimizerTreatsManualAndErrorsAsIsolatedSource`.

## Verification

- Run `go test ./internal/services` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
