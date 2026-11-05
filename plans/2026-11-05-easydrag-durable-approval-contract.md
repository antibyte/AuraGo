# EasyDrag durable human approval

Status: implemented and locally verified for the pure-local boundary below; production and human acceptance remain open.
Roadmap: `2026-11-04-easydrag-flagship-plan.md`, H.2 and H.6.
Base: the pure-local Runner contract in `2026-11-04-easydrag-durable-execution-contract.md`.
Owners: flows, server and Desktop EasyDrag guides. No production activation or deployment.

## User journey and boundary

An explicitly enabled, manually started pure-local durable flow reaches a human review.
It releases its physical worker, retains its original run/revision/owner/deadline, and appears
in the EasyDrag library's approval inbox. Opening the item shows the exact run and complete
review. An explicit authenticated approval permits that same run to resume through the fair
queue. Rejection, expiry or cancellation stops it. Opening or refreshing a review never decides.

- Reuse `flows.durable_enabled`, which stays false by default; introduce no new setting.
- Add only `logic.durable_approval` to the existing durable pure allowlist. Tools, models,
  external effects, calls, batches, ordinary test execution and selected-step execution cannot
  enter this path. Registration allows stored documents to remain readable while unavailable.
- The node has required templatable `summary` text (at most 2 KiB UTF-8) and optional
  templatable `review_data` JSON (default `{}`, at most 16 KiB encoded and depth 32).
  Reuse current parameter forms/resolution. Resolve once at the quiescent suspension boundary.
  Reject blank summaries, unsupported values, bounds violations and registered-secret content
  before inserting a gate; never truncate the review or silently redact an executable value.
  Reject integer values outside the browser's safe range (±9,007,199,254,740,991), including
  exponent notation; identifiers requiring more digits must be strings.
- Expiry is the original absolute run deadline. Add no timeout override or extended deadline.
  Approval completes the node with `{approved:true}`; rejection/expiry have fixed error codes
  and stop, without a configurable alternate branch. Pure nodes already committed never repeat.
- Simulation remains isolated: it may use an explicit bounded fixture for this known node,
  but cannot create a gate, wait, authenticate, write a decision or call the real coordinator.

## Identity, privacy and storage

- Migrate store v7 to v8 with the existing verified backup/transaction/version-last workflow.
  Preserve documents, pins, v6 durable owner markers, v7 child history and existing encrypted
  checkpoint bytes. The actual v7 binary must refuse v8 before changing database bytes.
- Add `flow_durable_gates`: opaque gate ID primary key, run ID foreign key with cascade delete,
  pinned flow/revision/node identity, admission owner, immutable `gate_generation`, content
  digest, expiry, state, creation/decision timestamps and bounded server-derived actor.
  A partial unique index permits at most one pending gate per run. The gate generation is the
  checkpoint generation at suspension and does not change while the person reviews it.
- Keep summary and JSON review only in an authenticated, versioned `pendingApproval` payload
  inside the encrypted checkpoint. SQL gate metadata, logs, events, exports and notifications
  contain no review payload or session credentials. Bind the SHA-256 digest to the exact
  displayed summary/data and run, flow, immutable revision, owner, node, gate and gate generation.
- A pending gate shares the existing 64 active durable-run, 40 MiB checkpoint/run and 512 MiB
  aggregate checkpoint limits. Graphs are acyclic and have at most 500 nodes, so each retained
  run has at most 500 gate records; do not retry a gate as another identity. Terminal gate
  metadata follows existing run retention and cascades when that run is pruned. Active gates
  and their checkpoint/version evidence remain protected from ordinary pruning.
- After continuation/termination removes the review payload, retain only gate metadata and its
  digest. Historical UI explicitly says that the complete review is no longer retained; ordinary
  step previews are not authoritative substitutes. No additional personal-data archive is added.

## State and transaction contract

The gate has `pending -> approved | rejected | expired | cancelled`; terminal decisions never
change. The run remains an active durable wait. Public detail reports `approval_pending` as an
observed wait reason; do not invent a second run or treat it as a completed history event.

1. At a quiescent boundary, atomically persist the encrypted checkpoint, waiting run and pending
   gate at generation g. A failed transaction leaves no gate. Release the physical worker only
   after this committed evidence exists; preserve the existing per-flow reservation.
2. Pending gates are excluded from ordinary timed checkpoint claims. Restart cannot execute
   past a pending review, and polling cannot change its generation. Expiry is still processed
   by a bounded coordinator path, including when ordinary durable execution is disabled and
   the flow service still exists. Globally disabling flows boots without that service; it
   cannot process expiry until the service is available again.
3. A decision authenticates the checkpoint and verifies exact gate identity, generation, digest,
   owner/revision and current run. One SQLite transaction CASes pending state and records actor,
   time and decision. Approval advances the encrypted checkpoint to g+1 with a wake at now;
   rejection/expiry/cancellation terminalize the run and remove its checkpoint in the same
   transaction. Existing payload-free history projection remains separate and idempotent.
4. Approval does not directly execute a node or hold a global slot. The ordinary fair queue
   admits the same run, rechecks live authority and verifies the approved gate before completing
   its node once. Keep the original deadline, budgets, outputs and published document.
5. Repeating the same decision for the same immutable identity returns its recorded outcome
   without another generation change. A conflicting decision, wrong digest/generation or stale
   evidence returns conflict and changes nothing. Lost replies require GET-only reconciliation,
   not automatic POST retries. A recorded approval does not claim that continuation completed.
6. Generation-fenced Stop also closes a pending gate atomically. Cancellation/rejection/expiry
   races and late workers cannot overwrite a terminal outcome or resurrect a run. Authority
   and external callbacks remain outside flow/Runner/SQLite locks.

Corrupt, unsupported or undecryptable state stays retained and blocked. `stale` is an observed
identity/integrity mismatch; `unknown` means an authoritative read failed. Disabled processing,
authentication or policy is a separate blocked reason, never evidence of rejection or completion.

## Authentication and HTTP

- Publication, admission and resume of an approval-capable graph require configured login.
  With authentication disabled, return the fixed `approval_auth_unavailable` reason before
  parking a new run. Existing gates remain inspectable under ordinary read policy and explicitly
  cancellable; they cannot be approved without login. Preserve current live flow/mission owner,
  enablement, read-only and app authority checks.
- Decisions require `Auth.Enabled`, a valid administrator browser session and normal same-origin
  Origin/CSRF checks. Reject any bearer Authorization, including alongside a cookie. Generic
  admin/write scope and its bearer CSRF exception do not satisfy this human-decision boundary.
- Derive actor `local_admin` on the server. Reject client actor fields; persist no cookie, token,
  nonce or IP. AuraGo currently has one owner login: people sharing it cannot be distinguished.
  Multi-user authentication is outside this slice.
- Read routes use Desktop read authority and `Cache-Control: no-store`:
  `GET /api/desktop/flows/approvals` (bounded cursor page) and
  `GET /api/desktop/flows/approvals/{gate_id}` (exact review or terminal metadata).
- Session-only `POST /api/desktop/flows/approvals/{gate_id}/decision` accepts strictly
  `{gate_generation, content_digest, decision: "approve" | "reject"}`. Return the stored gate
  outcome and decision metadata; a returned resume generation describes admission state only.
  No checkpoint payload is exposed. Bound request bodies and reject extra fields.
- Deep links carry only flow/run/gate IDs and open an authenticated read. No GET, notification,
  query token or link can authorize a decision. Notifications are not added implicitly.

## Interface and documentation

Use the EasyDrag library for the bounded inbox and its exact RunView for review/decision.
Reuse existing app/run navigation; add no Desktop app or competing Mission Control inbox.
Show flow, immutable revision, complete escaped summary/data, deadline and current gate state.
Allow scrolling without truncation; do not enable decisions if complete review cannot be shown.
Display policy/read-only restrictions and separate unknown reads from actual terminal decisions.

Approve and Reject each need an explicit confirmation and one guarded request. Disable competing
controls while it is pending. A lost reply offers read-only status checks; closing fences late UI
updates but cannot cancel an already accepted decision. Refreshing/opening never starts a flow.
Historical metadata must disclose when the full review is gone. Keep keyboard/focus, mobile
320/390 px, Standard dark/Fruity light/dark and all 16 genuine translations in the same slice.
Update English/German manuals and nearest owning guides with the implemented boundary.

## Required verification

The focused local gate passes: populated schema migrations and rollback, actual v7-binary
refusal of v8, three combined runtime cases including process exits and a fractional-microsecond
clock, two HTTP authorization/reconciliation cases, and Linux race checks of those affected paths.
One combined native journey covers the inbox, exact review, uncertain response, stale digest,
blocked session and terminal history in Standard dark and Fruity light/dark at desktop/phone sizes.
Root screenshot review confirms the 320/390 px dialog keeps its title and footer visible while
complete review data scrolls. Locale, module, syntax and bundle checks pass. Full suites,
performance benchmarks, live integrations and assistive-technology certification were not run.

- Migration: populated v7/WAL, backup verification and injected migration failures; unchanged v6
  owner markers/v7 history/ciphertext; actual v7 binary refusal of v8 without mutation.
- Store/Runner: gate-create and decision rollback, real process exits before/after decision commit
  and before queue claim, pending restart, same-run recovery, committed-step preservation,
  exact review identity, missing/replaced keys, corruption, quotas and retention.
- Concurrency: duplicate approvals, approve/reject, approve/expiry, approve/Stop, two restorers,
  late workers and lost response. Prove one terminal gate and no duplicate continuation.
- Policy: live auth/flow/mission/read-only revocation, default-off and unchanged ordinary durable
  waits; rejected/expired gates stop. No authority callbacks under coordinator locks.
- HTTP: real owner session + Origin succeeds; disabled auth, anonymous, bearer, bearer+cookie,
  bad Origin/CSRF, actor spoofing and token-only deep links fail without mutation. GETs are inert.
- UI: inbox -> exact run -> complete review -> explicit decision; terminal/blocked/unknown states,
  stale replies, GET-only uncertainty recovery, close/dispose, hostile text, keyboard and phone
  theme journeys. Keep module budgets, locale/loader coverage and legacy flows passing.
- Minimize verification to the changed behavior and meaningful risk boundaries above. Combine
  cases in existing/table-driven harnesses; avoid duplicate assertions and implementation-mirroring
  tests. Use focused host checks, then exact clean-checkout race checks for affected durable/approval
  paths and one combined native UI journey with cheap locale/module/bundle checks. Repeat or broaden
  only for a new change, failure or unresolved concern. Do not rerun unrelated package/browser suites
  or add performance benchmarks to this slice. Human usability, assistive technology, reference
  hardware and production acceptance remain separate release gates.
