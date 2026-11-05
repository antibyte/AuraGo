# EasyDrag library pause contract

Status: implementation contract; acceptance evidence is recorded separately.
Roadmap: `2026-11-04-easydrag-flagship-plan.md`, F.4 and J.5.
Owners: flows, tools, server and Desktop EasyDrag guides. Existing single-flow controls remain available.

## User journey

Select up to 50 explicit published flows in the current library view, review their current pause eligibility
and the consequences, then pause the eligible flows and inspect every outcome. Selection is
temporary UI state: it never changes drafts, metadata, favorites or saved filters. Changing the
library query/filter/folder clears selection rather than hiding selected targets. Blocks are excluded.
Do not add bulk activation, deletion, replay, automatic mutation retries or a background bulk job.

## Ownership and authority

- `GET /api/desktop/flows/{id}/enabled` returns a small, no-store, read-authorized status object
  with `mission_state`: `enabled`, `disabled`, `missing`, `mismatch` or `unavailable`. The response
  carries no flow document, credential or other mission payload. Missing flows retain normal 404.
- Read the current stored flow-to-mission binding and verify mission execution type and exact
  `FlowID` under the mission manager lock. An absent manager, missing mission and wrong owner are
  distinct from a disabled mission. An unprovable owner cannot be reported enabled or disabled.
- Existing POST enabled writes retain admin, origin, integration and read-only gates. Pass the
  expected flow identity through the existing service/bridge/manager call and reject an ownership
  mismatch under the same manager lock used for mutation. Update every caller and test double.
  A prior GET is useful evidence but is not the write authorization or a TOCTOU defense.
- Reuse existing pause lifecycle: stop future schedule/timer admission, invalidate in-progress
  block admissions and suspend durable recovery. Ordinary already accepted runs may continue;
  durable checkpoints remain retained. Pause is not Stop and does not undo completed effects.
  This work does not activate durable execution or alter its pure-local allowlist.

## Review and execution

- A bounded review reads the selected IDs; show enabled, already paused, missing/changed and
  unavailable targets separately. Names are text, never HTML. Require explicit confirmation after
  this review; no mutation occurs while selecting or opening the review.
- Use the existing house modal and controls. Keep keyboard focus, accessible names, live progress,
  Escape/Close and touch targets usable at 320/390 px in Standard dark and Fruity light/dark.
- Execute serially. Recheck each target immediately before sending its one pause POST. Skip
  already disabled, missing or mismatched targets. Never substitute a newly listed selection.
- After every attempted POST, including a lost/error response, reconcile with one read-only GET.
  Only exact-owner `disabled` confirms the current paused state. Enabled is not paused;
  missing/mismatch or an unreadable reply is unresolved. Distinguish an already paused target
  from a pause confirmed after an acknowledged write. If the write failed or its reply was lost,
  keep that uncertainty even when the read says disabled: currently paused, saving unconfirmed.
  The manager can expose a changed in-memory switch after a failed save. A read is not proof of
  persistence across restart or proof that this client caused the transition.
- No automatic POST retry. An explicit later user action may start a new reviewed operation.
  Any failed/lost mutation reply stops remaining writes after that target's single readback;
  an unreadable post-write status also stops them. This includes an authority change racing past
  the HTTP pre-gate, which may surface as a generic internal error. Preflight missing/mismatched
  or already disabled targets can be skipped without stopping the other reviewed candidates.
  Auth/read-only loss stops remaining writes; remaining targets are not attempted. Closing or
  cancelling stops unsent work and never silently sends the remaining requests. A sent request
  may finish after cancellation; its result must not be invented or repaint a different view.
- Every selected target has an outcome. Partial success never produces an all-success message.
  Use per-request deadlines and an operation identity fence; abort obsolete reads and ignore late
  replies after teardown. Retain only bounded IDs/names/status, not whole flow documents.

## Verification and documentation

- Manager/service/HTTP tests cover foreign owner, missing manager/mission, valid enabled/disabled,
  read versus write scope, origin/read-only gates and unchanged flow documents/revisions.
- Existing schedule, timer, admission and durable pause tests remain authoritative and pass with
  the tightened identity contract. Run affected Linux race checks.
- UI assertions cover the 50-target limit, view changes, preview without writes, sequential writes,
  partial results, lost response/readback, no retry, permission loss, close and late responses.
- Native browser journeys use the actual library/preview/results at desktop and 320/390 px,
  keyboard focus and both themes; existing library journeys remain intact.
- Translate every new string genuinely into all 16 locales. Update owning DOX and EN/DE manual 24.
  No new schema, dependency, production configuration, push or deployment is required.
