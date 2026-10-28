# Operational issues

## Purpose

Issue lifecycle, notification, and background retry policy.

## Ownership

`internal/planner` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

- Overdue todo notifications claim the todo ID and exact due value durably in `planner_meta` before dispatch. Recheck due/status atomically. A restart or ambiguous/panicking delivery cannot replay that revision; changing the due value creates a new claimable revision. A due value of `YYYY-MM-DDT00:00:00Z` is that civil day in the machine's local timezone. It becomes overdue on the next local day. Any other timestamp is overdue once that instant has passed. Reopening through `UpdateTodo` with status `open` or `in_progress` keeps checklist ticks. Verify `TestOverdueClaimSurvivesNotifierRestartAndPanic`, `TestCivilDueDateIsNotOverdueOnItsLocalDay`, and `TestUpdateTodoReopenKeepsFinishedItems`.

### Appointment admission

- Normalize appointment instants to UTC and compare indexed numeric epoch milliseconds;
  timestamp spelling and offsets must never decide ordering or due status. The v11
  migration keeps a consistent owner-only backup before converting populated stores.
- Appointment notification revisions have a durable outbox per mission/agent destination.
  Acknowledge only persisted queue/task admission. Retry proven non-admission, retain
  unknown or interrupted dispatches for review without automatic replay. Mission event
  tombstones survive queue completion; planner flow targets are also persisted before
  dispatch. A changed due instant creates a new revision. Todo claims retain their
  separate at-most-once contract above.
- Verify `TestAppointmentV11MigrationBacksUpAndRehearsesOnCopy`, `TestAppointmentOutbox*`,
  `TestAppointmentInstantsNormalizeAndCompareOffsets`, and tools `TestPlannerMissionAdmission*`.

### Operational Issue Notification Contract

- Persist a background prompt execution ID before dispatch. Internal HTTP retries
  poll/replay that same execution and never start another tool chain. A pending
  result does not spend retry allowance; uncertain executions after a process
  restart require explicit retry. Cron prompts use their own background session.
- Background and maintenance contexts only record operational issues; they never send user notices themselves.
- Code Studio assistant turns suppress operational notices before claiming the daily contact. Leave issues pending for ordinary chat; preserve Desktop tool policy, issue recording, and normal chat delivery.
- Operational notices are limited to one batch at the first direct contact per local calendar day, across sessions/channels and restarts. Atomically claim the day even if no issues are pending; later contacts do not drain additional batches. New or changed issue revisions remain pending until the next day's first contact, with at most two issues ordered by severity, change, and recency. A one-off `tool_failure` warning stays internal until its second occurrence. High-severity open issues may repeat after 24 hours only after another occurrence or while explicitly awaiting a user decision.
- Supported brokers receive `operational_issue_notice`; other channels receive the same localized text as a deterministic final-answer prefix. Mark a revision notified only after broker delivery or durable final-message persistence.
- The model receives the notice only as already-delivered diagnostic context and must not be responsible for deciding whether the user sees it. Internal memory-reflection advice stays hidden unless blocked or awaiting a user decision.
- Memory-reflection issues (`Source` `memory_reflect`, `planner.OperationalIssueSourceMemoryReflect`) are a model's text over journal and knowledge-graph content, so their notices carry `OperationalIssueNotice.Advisory`. The agent shows them under `backend.operational_issue_reflection_title` ("Model-generated reflection (advisory, not from the supervisor)") instead of the notice title, never prepends them to a final answer or marks them persisted with it (they stay pending for a typed delivery), and gives the model a prompt context that starts with `prompts.OperationalIssueAdvisoryPrefix`, so the builder writes the `MODEL-GENERATED REFLECTION (ADVISORY)` section instead of REQUIRED USER NOTICE and drops the supervisor wording. A notice is either supervisor findings or reflection items; reflection items that compete with supervisor findings wait for a contact without any, and `ListPendingOperationalIssueNotices` lists supervisor findings ahead of them so that advisories, which a channel without typed notices never delivers, cannot crowd a finding out of the two-item batch. The source decides, not the stored kind: the kind is shown to administrators, and the source also covers records written earlier. The Go-computed curator review (`Memory review needed`) shares the source and is shown the same way. Keep both locale keys in all 16 `ui/lang/backend` files. Verify `TestOperationalIssueNoticeMarksMemoryReflectionAdvisory`, `TestOperationalIssueNoticeListsSupervisorFindingsBeforeReflectionAdvisories` and, in `internal/agent`, `TestMemoryReflectionIssueIsAdvisoryNotSupervisorNotice` and `TestOperationalIssueNoticeKeepsSupervisorFindingsApartFromReflection`; the heading is covered by `TestBuildSystemPromptFramesReflectionAdvisoryWithoutSupervisorWording` in `internal/prompts`.
- Archive stale active history losslessly: single warnings after seven days, recurring warnings and errors after 30 days, and never explicit `review_required` decisions. Archived issues stay out of notices, reminders, active counts, and mission triggers; a recurrence atomically reopens the same fingerprint and retains its history.
- The administrative operational-issues API and Dashboard expose only sanitized records with non-reversible public IDs. Retention deletes completed records only; archived records are not automatically deleted in v1.
- Explicit retry requests permit at most one supervisor-selected safe retry. Guardian blocks, credential searches, secret environment access, and `_guardian_justification` retries are never eligible.
- Tool-failure fingerprints include the normalized operation when one exists. A success resolves only the same operation; dedicated and legacy aliases for Virtual Desktop app installation share one semantic fingerprint.
- A blocked unattended action retains its tool issue without a duplicate loopback runtime failure. Cancellation is not a runtime failure. A successful loopback resolves only the same source/context's `loopback` fingerprint. Verify `TestLoopbackActionRuleBlockRecordsToolIssueWithoutRuntimeFailure` and `TestLoopbackSuccessResolvesOnlyMatchingRuntimeIssue`.
- Repeated route-specific `context_budget_exceeded`, Telegram long-poll failures, and reproducible maintenance-phase failures use this lifecycle and resolve only after a success on the same route or phase. Telegram polling records its first issue after three consecutive failures and exposes only sanitized runtime codes.
- RuView edge hints use fingerprints `ruview:sensing_stale`, `ruview:storage_limit`, and `ruview:stop_unconfirmed` with kind `runtime_failure`. The integration signals a transition only; this package records, resolves, and notifies.
- The nightly maintenance task emits one idempotent typed `morning_briefing` notification per run after the optional catch-up, its final phase ledger and current local integration checks. Combined totals count remaining archive work once. Recovered helper attempts are sanitized warnings and do not fail a completed fallback. Disabled components are `skipped`; deferred retryable work makes the run `partial`, while only critical initialization or persistence failures make it `failed`. Background checks never send Telegram messages.
- The briefing distinguishes processed archive messages from all processed work, lists unfinished phases with deferred counts and sanitized error codes, and identifies the active issue count as the total across runs. Passing integration checks never hide partial maintenance phases. Verify `TestMorningBriefingExplainsUnfinishedPhasesAndConsolidationProgress`.
- Weekly reflection has its own maintenance phase; skipped runs cannot resolve its failures. Use route-aware reasoning/output limits, reject empty/truncated/invalid completions, and retry once with fewer source records. Persist only parsed results; successful persistence may resolve the matching issue. Never store raw failed output or emit separate reflection notifications.

## Verification

- Run `go test ./internal/planner` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
