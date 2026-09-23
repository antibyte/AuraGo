# Operational issues

## Purpose

Issue lifecycle, notification, and background retry policy.

## Ownership

`internal/planner` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Operational Issue Notification Contract

- Persist a background prompt execution ID before dispatch. Internal HTTP retries
  poll/replay that same execution and never start another tool chain. A pending
  result does not spend retry allowance; uncertain executions after a process
  restart require explicit retry. Cron prompts use their own background session.
- Background and maintenance contexts only record operational issues; they never send user notices themselves.
- Operational notices are limited to one batch at the first direct contact per local calendar day, across sessions/channels and restarts. Atomically claim the day even if no issues are pending; later contacts do not drain additional batches. New or changed issue revisions remain pending until the next day's first contact, with at most two issues ordered by severity, change, and recency. A one-off `tool_failure` warning stays internal until its second occurrence. High-severity open issues may repeat after 24 hours only after another occurrence or while explicitly awaiting a user decision.
- Supported brokers receive `operational_issue_notice`; other channels receive the same localized text as a deterministic final-answer prefix. Mark a revision notified only after broker delivery or durable final-message persistence.
- The model receives the notice only as already-delivered diagnostic context and must not be responsible for deciding whether the user sees it. Internal memory-reflection advice stays hidden unless blocked or awaiting a user decision.
- Archive stale active history losslessly: single warnings after seven days, recurring warnings and errors after 30 days, and never explicit `review_required` decisions. Archived issues stay out of notices, reminders, active counts, and mission triggers; a recurrence atomically reopens the same fingerprint and retains its history.
- The administrative operational-issues API and Dashboard expose only sanitized records with non-reversible public IDs. Retention deletes completed records only; archived records are not automatically deleted in v1.
- Explicit retry requests permit at most one supervisor-selected safe retry. Guardian blocks, credential searches, secret environment access, and `_guardian_justification` retries are never eligible.
- Tool-failure fingerprints include the normalized operation when one exists. A success resolves only the same operation; dedicated and legacy aliases for Virtual Desktop app installation share one semantic fingerprint.
- Repeated route-specific `context_budget_exceeded`, Telegram long-poll failures, and reproducible maintenance-phase failures use this lifecycle and resolve only after a success on the same route or phase. Telegram polling records its first issue after three consecutive failures and exposes only sanitized runtime codes.
- The nightly maintenance task emits one idempotent typed `morning_briefing` notification per run after persisting its phase ledger and current local integration checks. Disabled components are `skipped`; deferred retryable work makes the run `partial`, while only critical initialization or persistence failures make it `failed`. Background checks never send Telegram messages.
- Weekly reflection has its own maintenance phase; skipped runs cannot resolve its failures. Use route-aware reasoning/output limits, reject empty/truncated/invalid completions, and retry once with fewer source records. Persist only parsed results; successful persistence may resolve the matching issue. Never store raw failed output or emit separate reflection notifications.

## Verification

- Run `go test ./internal/planner` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
