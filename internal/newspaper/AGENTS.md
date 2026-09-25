# Newspaper

## Purpose

Own the installation's personal daily publication, durable editions and delivery ledger.

## Ownership

- `service.go` owns one cancellable research run and the local-date scheduler.
- `store.go` owns the versioned SQLite state under the configured data directory.
- `types.go` validates profiles, source-backed drafts and immutable edition hashes.
- `render.go` renders the saved edition for email and PDF.
- `skills/aurago-newspaper/SKILL.md` is the bundled editorial policy, registered through the Agent Skill Manager.

## Local Contracts

- The profile is installation-owned. Saving preferences does not send an edition; changing the email destination or account clears verification and daily email.
- Up to ten profile-owned RSS feeds can supplement or replace Brave Search. Each feed belongs to a selected section; only guarded public HTTP(S) retrieval is permitted. Feed entries are leads until their original article pages are fetched.
- Published revisions are immutable. Each story paragraph cites a fetched source passage; unsupported or unsafe drafts do not publish.
- A correction note requires an existing edition and an explicit new revision. Keep the earlier revision unchanged and render the note in the new app, email and PDF output.
- Schedule by the profile's IANA time zone and local date. A DST gap advances to the next real local minute, and a repeated minute uses its first occurrence. Startup reconciles interrupted runs and sends without replaying them.
- Delivery receipts bind the edition hash, channel, destination hash and idempotency key. Only a known pre-send failure is retry-safe; ambiguous sends remain uncertain.
- Keep the feature disabled by default. Read-only blocks profile changes, research and delivery while preserving archive reads. Keep source evidence, run events and editions bounded.

## Verification

- Run `go test ./internal/newspaper` for validation, persistence, restart and DST behavior.
- Run `go test ./internal/server -run TestNewspaper` for HTTP policy and delivery adapters.
- Run `go test ./ui -run 'TestNewspaperTranslations|TestDesktopNewspaperBrowser'` with `AURAGO_RUN_BROWSER_SMOKE=1` for the reader.

## Child DOX Index

None.
