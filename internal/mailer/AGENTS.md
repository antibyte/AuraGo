# Mailer core contract

## Ownership

This package owns the local mail index, cursors, linked account references,
revisioned drafts, templates/signatures, rules, snooze state, scheduled outbox,
and private attachment blobs. Adapters own provider protocols, authentication,
MIME parsing/composition, and provider-specific identifiers. Account records
persist source references and presentation/policy metadata only; credentials
remain in the Vault and are resolved only when an adapter is created.

## Safety and lifecycle

- Gmail requests bind redirects to the original API origin, including injected
  clients, without mutating shared clients or relaxing stricter redirect policies.

- Sync metadata in the background on the service lifecycle, independent of UI
  visibility. The loop still wakes every 10 seconds; only due accounts connect.
  Process start and manual sync run immediately. A quiet pass waits five
  minutes; new mail, a leftover page, deletions, or another error wait 60
  seconds. Disabled and read-only passes use the idle interval. The outbox loop
  stays at one second. The due map is in memory.
- One IMAP session serves each account pass, with sequential commands. A failed
  command closes that session so the next folder dials again. Cancellation still
  stops the pass. Completion
  stores the high-water UID and UIDVALIDITY, so the next pass searches only
  newer UIDs. A validity change clears the cursor and takes a full snapshot.
  After the first snapshot, later passes still do not remove older messages that
  disappeared on the server. Hydrate IMAP content on demand and fetch raw mail
  without setting `Seen`; POP sources remain permanent and sync never deletes
  them from the server.
- List metadata may store `snippet` and `attachment_count`. Those fields do not
  fill `Text`, `HTML`, or `Raw` and do not skip hydration. Previews use
  `BODY.PEEK`.
- One unreadable message stays in the thread. The thread path sets `ReadError`
  to the fixed code `message_unavailable` and does not write that code back.
  A failed HTML sanitize keeps an earlier HTML part that succeeded. `HTMLSkipped`
  is set only when no HTML part could be shown. Later text and attachments stay.
  A POP3 message that cannot be parsed keeps `Raw` and headers,
  leaves `Text` empty, and persists `ReadError` `message_unparsed`.
- Provider search pages one folder of one account and forwards the provider
  cursor. The UI toggle stays on while the query is edited. Account, view,
  folder, and clear-search reset it. A single-account inbox uses the folder
  whose kind is `inbox`. The all-accounts inbox stays local. Remote HTTP results
  keep `next_cursor`. `partial` is true only when that cursor is present, so the
  last page does not advertise another page. A later IMAP page searches only
  UIDs below the cursor.
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
