# Targeted memory corrections

The main agent uses `correct_memory` to preview, apply and inspect corrections
across core facts, long-term memory, graph properties/claims and user profiles.
It uses current user statements or runtime-attested system observations. An
explicit preference or identity change always needs a user statement. Ambiguous
matches stay visible for review. No separate maintenance agent or UI is added.

The existing `tools.memory.enabled`, `tools.memory.read_only`, graph enablement
and read-only settings, and `personality.user_profiling` govern affected stores.
No correction unlocks a protected entry. Historical chat, journals, notes and
source documents are not rewritten. Source-owned graph facts must be changed
at their source.

SQLite stores a session-bound operation with evidence hashes, target references,
snapshots and execution receipts. Before the additive schema is installed on a
populated database, `VACUUM INTO` creates a consistent restricted-permission
`*.memory-corrections-v1-*.bak` backup. Main and graph databases commit separate
receipts; there is no distributed transaction. A process interruption or index
failure can leave an incomplete operation. `status` and reapplying its ID expose
and resume that state, provided the targets and gates are unchanged.

LTM replacements have deterministic, operation-owned IDs. Metadata marks them
archived before embedding; publication archives the old ID and activates the
new one in the same SQLite transaction. Old vectors are retained. Graph claim
history uses accepted/superseded/retracted states; property snapshots remain in
the operation history. Automatic extraction guards block withdrawn normalized
facts and graph assertions. Corrected profile fields retain a tombstone/lock so
delayed analysis and weak-observation cleanup cannot replace the explicit value.
Technical LTM replacements retain their attested source class under
`memory_correction:<source>` so later fresh observations can correct them again.

Copy detection includes exact normalized facts. Short scalar matches and fuzzy
neighbors are review candidates. Bounded searches report limited coverage.
Different wording may require an agent to identify explicit related targets
using the same evidence. System observations currently come from successful
`system_metrics` and remote-device `sysinfo` calls, not arbitrary shell, network,
search or model-generated output.
Tool-only authorization is limited to typed technical graph properties and LTM
with that explicit technical provenance. Unclassified free text requires user
evidence to avoid silently treating a personal fact as a machine observation.

Run focused `TestMemoryCorrection*` and `TestCorrectMemory*` tests in
`internal/memory` and `internal/agent`, then the memory, prompt, schema/exporter
and tool bridge checks. These tests use synthetic facts and local embeddings;
live-provider performance and deployment require separate validation.
