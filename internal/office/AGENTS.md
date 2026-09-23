# Desktop Office

## Purpose

Workbook and document preservation, editing, and assist.

## Ownership

`internal/office` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Desktop Workbook Contract
- Tabellen uses exactly Univer OSS 0.25.1, Chart.js 4.5.1 and Excelize 2.11.0.
  Vendor assets/fonts stay local and permissive; never add Pro components.
- `/api/desktop/office/workbook?representation=editor-v2` exposes typed native
  snapshots. PATCH requires ETag or create-only preconditions, applies the
  editor's structural journal atomically to the original package, and retains
  opaque parts. Preserve images, macros (never execute them), chart XML and
  unsupported extensions; reject edits that cannot retain their references.
- Legacy Office/agent writes cannot silently flatten complex XLSX. Same-format
  exports pass through complete packages; explicit simplified copies are separate.
- Workbook assist is tool-free, uses only bounded explicit selection/context,
  and returns revision-bound proposals. The client applies only after approval.
- Both Office apps share revision-aware serial saves, asynchronous close guards
  and explicit IndexedDB recovery. Conflict copies carry original source bytes.

### Desktop Office Document Contract
- Autor uses the exact Apache-2.0 DOCX core 2.16.0, local fonts/WASM, and an MIT
  review extension; no paid Pro dependency. The UI remains Vanilla JavaScript.
- `/api/desktop/office/document?representation=docx` reads complete DOCX bytes;
  writes require `If-Match` or `If-None-Match: *` and use the desktop's atomic
  conditional-write path. Keep the legacy JSON API compatible.
- Legacy Office/agent writes must reject a DOCX they cannot preserve. Explicit
  simpler-format copies are allowed; DOCX-to-DOCX exports pass through full bytes.
- `/api/desktop/office/assist` is bounded, revision-bound and tool-free, without
  general chat history or file access. Applying a suggestion is a client decision.
- Desktop windows may install an asynchronous `beforeClose` guard. Await it before
  animation/disposal; refusal or errors leave the window open.

## Verification

- Run `go test ./internal/office` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
