# Mailer core contract

## Ownership

This package owns the local mail index, cursors, linked account references,
revisioned drafts, templates/signatures, rules, snooze state, scheduled outbox,
and private attachment blobs. Adapters own provider protocols, authentication,
MIME parsing/composition, and provider-specific identifiers. Account records
persist source references and presentation/policy metadata only; credentials
remain in the Vault and are resolved only when an adapter is created.

## Safety and lifecycle

- Sync metadata in the background on the service lifecycle, independent of UI
  visibility. Hydrate IMAP content on demand and fetch raw mail without setting
  `Seen`; POP sources remain permanent.
- Automatically evaluate rules only for newly indexed messages. Historical
  application is explicit and previews make no changes. Rules cannot forward or
  permanently delete messages.
- Snooze is local state. Scheduled sends have a ten-second undo window; queued
  sends survive restart and run when due. A send left in `sending` at restart is
  marked `unknown` and must never be replayed. Store confirmed sends separately
  from the Sent-folder append attempt.
- Permission changes revoke active writes and prevent new ones. Check the gate
  immediately before every external mutation and before publishing local state.
- Attachments use generated IDs under a private configured root and enforce
  per-file and aggregate bounds. Never resolve a caller-provided path.
- Migrations take a consistent SQLite backup before changing an existing
  database. Reject schemas newer than this package understands.

## Validation

Run focused `go test ./internal/mailer` tests for store migration, message
indexing/cursors, draft revision conflicts, rule preview/new-only behavior,
outbox restart reconciliation and no-replay semantics, revocation, and attachment
root/size bounds.
