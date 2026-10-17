# Insights and the native code graph

**Insights** (German: **Erkenntnisse**) extend Memory with evidenced decisions,
rationale and revision history. They are independent of the Decisions model
routing service. Both new capabilities are opt-in in **Configuration → Memory**:

```yaml
learnings:
  enabled: false
code_graph:
  enabled: false
```

Existing facts, preferences and learned tool rules retain their behavior. Enabling
insights does not import old conversations or convert existing memory. The existing
post-turn memory analysis can propose at most two insights per eligible turn;
there is no second analysis request. Its existing enablement, privacy suppression,
shutdown cancellation and memory write gates also apply.

## Evidence and review

An insight records an assertion, rationale, actually evidenced alternatives,
sources, global/project/resource scope, an optional exact validity condition,
revision and state: proposed, active, needs revalidation, superseded or archived.
Explicit human decisions with sufficiently clear evidence can activate immediately.
An inferred preference needs confidence of at least 0.95 from two independent
sessions and no unresolved structured contradiction. Duplicate observations do
not create independent evidence. Model confidence alone is not proof.
Additional evidence must match the stored assertion, rationale, kind, alternatives
and resource fingerprint; a differing draft needs explicit review or correction.
Corrections retain superseded alternatives only in the old entry's history.

The runtime resolves `user:current` and supported structured technical evidence.
Where ingress captures the original human text, both direct tools and background
analysis use that text for evidence; an empty human input cannot fall back to
attachment or handoff content. Resource-bound runs cannot broaden their scope
through a condition's target.
Quoted/fenced/imported source text and assistant responses cannot authorize a
personal preference. Technical checks never grant permanent permissions.
Model-invented conditions and paraphrased decisions or technical assertions remain
proposals. A technical source must support the asserted statement, not merely
exist in the runtime evidence ledger. No source identifier or project path supplied
by the model establishes access.

The central Memory panel and Code Studio/Game Maker project views share the same
records and review actions. Inspect sources and history, then confirm, protect,
correct or archive. A correction creates a linked successor and retires the old
entry. Protected entries must first be unprotected; retired entries cannot be
silently resurrected. Concurrent edits use revision checks. A failed write is
never automatically retried; reload and inspect the outcome.

The additive SQLite migration takes a consistent database backup before adding
the insight tables to an existing database. It is transactional and restart-safe.
Existing archive/correction data remain untouched. Backups, records and revisions
are local private runtime data; do not publish them or export them to Git.

## Context and write checks

Retrieval filters scope before ranking exact resources, textual task relevance
and recency. Automatic context contains at most three insights and 600 tokens,
including scope, source and selection reason. Prepared prompt profiles retain
their immutable system prompt; observations enter as isolated conversation data.
Co-agents receive only their inherited project scope and cannot persist insights.
The paired local-agent memory adapter has global scope only: client-supplied
paths cannot authorize project memory. Normal channel loops share the server's
project resolver; project jobs use their own registered working copies.
Active runs recheck project registration for recall, correction and publication.
Prepared observations refresh between model calls, including after revocation.

Content fingerprints mark relevant resource insights for revalidation. A changed
file is not proof that its decision is wrong. Active graph indexes poll every
five seconds; learning-only runs recheck the selected resource at access/write
boundaries using fresh fingerprints. Read-only runs omit stale or unreadable
resource insights without changing stored revisions. Files that cannot be read
are not treated as confirmed deletions. Revalidation covers every matching active
or pending resource insight; further changes invalidate an older review revision.
Readable content changes still trigger revalidation when the new source has
syntax errors; failed parser facts remain excluded from the index.

Supported structured operations include filesystem writes/moves/copies/deletes
(including batches), file editors, JSON/YAML/TOML property edits, Desktop file
operations, Homepage files and bound Game Maker/CYD job file writes. A known
exact target/property/value conflict pauses only the undispatched call. The
agent can adjust it or resubmit the same action with `learning_reason`; the
exception binds to that run, arguments, targets and insight revisions and is
audited. Completed siblings are never replayed. Natural-language observations
are hints, not semantic conflict proofs. Shell/Python programs and unknown
effects are explicitly outside this preflight coverage. Security, approval and
write gates remain authoritative after reconsideration.
Read-only memory and co-agents do not persist conflict metrics or deviations;
their deliberate deviations are recorded in the server log instead.

An in-process advisory list records known write targets for active runs. Related
subtasks share a work-chain identity. Completion, cancellation and shutdown
release entries. Existing project locks still decide concurrency. This is not
monitoring of other processes, remote agents or unfetched Git branches.

## Code graph

`code_graph` supports `status`, `search`, `outline`, `callers`, `callees`, `impact`
and `rescan`. Code Studio exposes the same graph with source jumps and coverage.
Go, JavaScript/JSX, TypeScript/TSX and Python use **gotreesitter v0.56.0**, with
only the corresponding grammar packages linked. No Node/Python process or CGO
binding is required. C/C++ and other languages are outside coverage.

Definitions, types/classes, imports, static calls and inheritance have source
positions and short snippets. Lexical names and unique project-local imports
can resolve; dynamic dispatch, ambiguous bindings, reflection and external
dependencies and JS/TS re-exports remain unresolved. Missing edges never establish
that a change has no effects. This feature does not rename symbols or modify source.

First use indexes a registered workspace; source and index stay in RAM. Restart
rebuilds them lazily. Container `/workspace` maps only to the configured Desktop
workspace. Ordinary agent, Desktop, Homepage and project-job roots are distinct.
No arbitrary host directory may be indexed through the API or tool.

Bounds: one parser worker; 1 MiB per file; 64 MiB parser allocation budget;
250 ms parse timeout per file; 256 MiB estimated retained index; 16 active
projects; 50,000 visited entries per scan. Replacement accounting reserves the
previous snapshot too. The response retains at most 100 individual skip reasons
and reports the full skipped count. Syntax failures, limits and unreadable
files produce visible partial coverage. Generated/dependency directories and
symlinks are excluded. Each response includes freshness, duration, coverage and
unresolved relationships. Responses cap results at 100 and impact depth at four.

## API and validation

Admin routes `/api/learnings` and `/api/code-graph` back central management.
`/api/desktop/learnings` and `/api/desktop/code-graph` additionally enforce
Desktop scopes and write policy. Browser writes require a valid same-origin
request. Bearer access to Desktop knowledge requires both admin and the
appropriate Desktop scope. Responses are not cacheable.

Agent manuals document `remember_learning`, `query_memory`, `correct_memory`
and `code_graph`. Native discovery and Strict schemas use the same definitions;
the separate agotrain repository retains the curated, synthetic DE/EN fixtures.

Local tests cover migration/backup, provenance, repeated observations, scope
isolation, correction history, conflict bindings, source changes, parser fixtures
and revocation/shutdown. Browser fixtures exercise actual Code Studio controls
and the shared view in Standard and Fruity. These do not establish live-LLM
quality or production acceptance. Retrievals, discarded eligible candidates,
revalidations, conflicts, index duration/coverage and tokenizer-counted inserted
context are measured directly; no token or monetary saving is presumed.

`TestLearningBilingualTaskEvaluation` fixes ten DE/EN tasks covering resource
decisions, matching and conflicting port edits, TypeScript work across two files,
foreign projects and unrelated requests. It compares retrieval and inserted
tokens with a baseline without insights. Graph fixtures independently verify
multi-file imports and callers. Neither test substitutes for live-LLM acceptance.
