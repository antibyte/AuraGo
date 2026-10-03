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
- Up to ten profile-owned RSS/Atom feeds supplement permitted Brave News/Web or DuckDuckGo search. Each feed belongs to a selected section; only guarded public HTTP(S) retrieval is permitted. Up to 40 current feed entries are leads until their originals are fetched.
- Research uses the bundled guide for open, international source discovery and validated topic/language plans with deterministic fallback. The server owns capability snapshots, retries and shared budgets: up to 200 queued candidates, 32 searches by default (1–64 including retries), two planning calls, four original-page reads (one per publisher domain) and two editors. Stop discovery at 80% of the deadline; one follow-up may widen 24 hours to seven days. Keep issue ceilings 6/12/16 and permit smaller, explicitly partial editions.
- `Run.Research` is optional JSON in the existing run record, not a new table or migration. Persist bounded counters, rejection codes and topic gaps independently of accepted edition sources. Preserve legacy runs and source evidence on interruption. Recheck write policy before publication; disabling the feature or enabling read-only cannot publish a new revision.
- Published revisions are immutable. Each story paragraph cites a fetched source passage; unsupported or unsafe drafts do not publish.
- A failed or interrupted run without a published edition may be retried manually for the same local date as the next numbered revision. The daily scheduler records its own durable one-attempt-per-date claim; manual runs do not consume it. A version-1 store is backed up before migrating that claim to version 2. An existing published edition requires an explicit new revision.
- A correction note requires an existing edition and an explicit new revision. Keep the earlier revision unchanged and render the note in the new app, email and PDF output.
- Schedule by the profile's IANA time zone and local date. A DST gap advances to the next real local minute, and a repeated minute uses its first occurrence. Startup reconciles interrupted runs and sends without replaying them.
- Delivery receipts bind the edition hash, channel, destination hash and idempotency key. Only a known pre-send failure is retry-safe; ambiguous sends remain uncertain.
- Email confirmation codes have a one-minute request cooldown and expire after ten minutes. Cancel only the matching code after a known send rejection so the user can retry; retain codes after ambiguous sends so delayed mail can still be confirmed.
- Keep the feature disabled by default. Read-only blocks profile changes, research and delivery while preserving archive reads. Keep source evidence, run events and editions bounded.
- Prune old scheduler claims and their runs in one transaction, retaining current-date claims even after many manual revisions. Report prune failures. A separate run context retains explicit cancellation through publication/delivery, including after the research deadline permits a valid partial edition. Persist final state with a separate bounded update context.

## Verification

- Run `go test ./internal/newspaper` for validation, persistence, restart and DST behavior.
- Run `go test ./internal/server -run TestNewspaper` for HTTP policy, guided research, yield/concurrency/budgets and delivery adapters; `go test ./internal/tools -run 'Brave|ExecuteDDG|Newspaper'` for search compatibility and bundled skill registration.
- Run `go test ./ui -run 'TestNewspaperTranslations|TestDesktopNewspaperBrowser'` with `AURAGO_RUN_BROWSER_SMOKE=1` for the reader.

## Child DOX Index

None.
