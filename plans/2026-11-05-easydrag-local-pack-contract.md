# EasyDrag local flow packs

Status: implementation in progress; acceptance evidence is recorded separately.
Roadmap: `2026-11-04-easydrag-flagship-plan.md`, J.7 and the reuse reference journey.
Owners: flows, server and Desktop EasyDrag guides; Mission Control ownership remains in tools.

## Complete first journey

Export a normal local flow together with its exact nested pure block versions. On another
installation, review the contents, create a new inactive draft and independent local blocks,
then use the existing simulation flow with explicitly entered sample input. Existing plain
flow v1/v2 import and export remain available and unchanged.

This first pack format permits a manual trigger, Set, If, Switch, Merge and pure Call block.
Block documents use their existing pure allowlist and may call pinned nested blocks. Reject
batches, waits, AI, tools, integrations, secret references and unknown node types in every
node, including disabled and unreachable nodes. Do not infer purity from an empty effect list.

Broader integration packs still need explicit catalog contracts for local resource IDs, paths,
provider selection, Vault references and credential-bearing parameters. Current OptionsSource
metadata covers only some of these. This slice must not advertise general resource rebinding.

## File and privacy boundary

- Use a distinct versioned JSON pack envelope and file extension. Bound the raw upload and
  aggregate document bytes before decoding; retain existing graph, depth, reference and pin caps.
- Bind export to one exact draft snapshot and verify the complete referenced version closure,
  including stored hashes and transitive pins. Include every referenced source version once.
  Multiple revisions of the same source block remain distinct snapshots.
- Include no runs, recorded inputs/outputs, trigger history, private scenarios, fixture records,
  credentials or Mission Control state. Clear the manual trigger's sample data by default.
  Reject secret expressions; apply registered-secret redaction to user strings before export.
  User-entered literal text remains content: explain that the user must review it before sharing,
  without claiming unknown secrets can be detected reliably.
- Verify source hashes before sanitization. Exported hashes cover the actual sanitized bytes;
  local hashes are recomputed after reference remapping. These are integrity checks, not signatures.
  Fixed structural identity must not be corrupted by generic text scrubbing. Revalidate the final
  serialized pack and fail safely if redaction makes required structure invalid.

## Review and import

- Read-only preview validates the complete pack and shows the parent, exact block versions,
  included literal content, removed sample-data policy and local-only execution boundary.
  No mission, row, version, pin or run is created by preview.
- Allocate fresh proposed parent/block identities. Map each distinct source block/version to its
  own new local block at revision 1; never overwrite or silently adopt an existing ID. Remap only
  typed Call block reference fields. Node, edge and group member IDs remain frame-local and unchanged.
- An explicit Import submits a bounded proposal tied to the exact pack digest and proposed IDs.
  Treat the proposal as untrusted and fully validate it again. Reuse the existing extraction
  proposal/reconciliation approach where applicable; do not introduce an asynchronous job system.
  The digest and proposed identity/hash manifest are sufficient correlation; do not add a request
  ID that implies a durable receipt ledger.
- Insert all new block drafts, immutable versions, their transitive version pins and the parent
  draft in one SQLite transaction. The parent has no published revision and cannot run automatically.
  Validate the combined local plan before committing. Existing rows remain untouched on refusal.
  Acquire SQLite write ownership by inserting the fresh parent first; insert dependency blocks
  child before parent. Existing-row extraction locks cannot protect a not-yet-created parent.
- Mission Control is a separate JSON store. Create the parent's mission disabled through its
  existing owner boundary, outside the SQLite transaction. Reject a duplicate flow owner under
  the manager mutation lock, including a retained orphan from an uncertain earlier import.
  Do not claim cross-store atomicity.
  On a definite SQLite rollback, clean up only the exact mission created for this parent.
  On an uncertain commit, check the proposed IDs and hashes before cleanup; retain the disabled
  mission if the outcome cannot be proved. Existing orphan reconciliation reports rather than
  silently deletes such state. Any new cleanup helper must verify exact owner under its write lock.
- A lost Import reply triggers only read-only reconciliation, never an automatic Import retry.
  Confirm an import only from matching parent, block versions and pins; missing, changed or
  unreadable evidence remains uncertain. Show the created draft only after acknowledgement or
  matching reconciliation. Do not promise durable at-most-once imports without a receipt ledger.
  Keep an unresolved proposal in the current app window; ending that local check requires an
  explicit acknowledgement and changes no server data. Page reload does not retain the proposal.
- Imported direct dependencies follow the existing draft lifetime: an unpublished parent does
  not create a fake publication or run pin. Explicitly deleting an unpinned block can invalidate
  that draft. Nested immutable version pins retain their established deletion protection.

## Wire contract

- Format marker: `aurago.easydrag.local-pack`, version `1`; download suffix
  `.easydrag-pack.json`. A recognized pack never falls back to plain flow creation.
- `GET /api/desktop/flows/{id}/pack` downloads the bare pack JSON. The parent carries
  `{hash, source_revision, flow}`; blocks carry `{source_id, source_revision, hash, flow}`.
  Editor export verifies this source revision against its acknowledged save before downloading.
- Read-authorized `POST /api/desktop/flows/packs/preview` accepts `{pack}` and returns
  `{proposal, review}`. Review contains bounded display metadata, separate from authority.
- The proposal contains `pack_digest`, `proposal_digest`, `parent_id`, `parent_hash` and
  `blocks[{source_id, source_revision, local_id, hash}]`.
- Admin-authorized `POST /api/desktop/flows/packs/import` accepts `{pack, proposal}` and
  acknowledges `{status: "applied", flow, blocks}` only after the complete import commits.
  Records are compact summaries, not full documents: identity, kind, name, optional mission ID,
  draft/live/published revisions, remapped hash and exact pin summaries. Source hashes differ
  from hashes recomputed after new IDs and typed references are assigned.
- Read-authorized `POST /api/desktop/flows/packs/reconcile` accepts `{proposal}` and returns
  `{status: "applied_state_confirmed", flow, blocks}` only for complete matching evidence;
  a confirmed empty closure uses `blocks: []`. Otherwise return `{status: "unknown", blocks: null}`
  without records. These read-only POST routes must
  retain the existing authentication, origin and availability contracts.
- Bound raw requests to 32 MiB, each canonical document to 2 MiB and the canonical parent plus
  child documents to 18 MiB. The existing tighter graph and closure limits also apply.
- Errors use `FLOW_PACK_TOO_LARGE` (413), `FLOW_PACK_INVALID` (400),
  `FLOW_PACK_UNSUPPORTED` (422) and `FLOW_PACK_CONFLICT` (409), plus existing authority errors.

## Acceptance before completion

- Real two-store export/import/simulation with a nested block and two different revisions of
  one source block. Assert fresh identities, exact outputs, preserved expressions/groups and
  zero production invoker calls, runs or parent activation.
- Tampering, missing/extra/cyclic dependencies, malformed references, unsupported nodes,
  credential/secret expressions, control characters and all size bounds fail closed.
- Faults during each SQL insertion/pin/parent boundary roll back the complete SQLite import.
  Test mission creation failure, definite rollback cleanup, cleanup failure, commit uncertainty,
  lost replies and later edits without overwriting existing data or blindly repeating creation.
- Preserve old v1/v2 import/export and all affected block, extraction, publication, simulation,
  cancellation and authority tests. Run affected Linux race checks and inspect actual native
  preview/import/result flows at desktop and 320/390 px in supported Standard/Fruity themes.
- Genuine translations in all 16 locales, EN/DE manual updates and owning DOX changes accompany
  implementation. No production activation, push or deployment is part of this slice.
