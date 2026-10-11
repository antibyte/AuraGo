# Correct current memory (`correct_memory`)

Use this for a clearly wrong or outdated current fact. Search with `query_memory`
first and copy its typed `references`. Existing result fields remain available.
Do not rewrite conversations, journals, notes or source documents. A statement
that was true in its historical context is not an erroneous current fact.

## Evidence and authority

`preview` takes `changes`, `reason`, and `evidence`. Each evidence item names
`reference` and an exact supporting `quote`. `user:current` denotes the actual
human message in this main-agent turn. Successful `system_metrics` and remote
`sysinfo` results can issue a runtime `tool:` reference. Other tool output,
invented references, remembered text, similarity scores, and `confirmed=true`
cannot authorize a correction. Personal preferences and identity require user
statements; never infer them from a system check or contradictory memories.
Unknown free-text facts conservatively require user evidence too. Tool-only
corrections target existing machine properties or LTM with explicit
`system_metrics`/`remote_control` provenance.
Ask the user about an ambiguous fact or the intended replacement.

## Target and operation

Each change has `reference`, `operation` (`replace` or `archive`), and optionally
`value`. Omit `value` for archive. Reference shapes:

| Store | Fields | Replacement |
| --- | --- | --- |
| `core` | `id` (numeric string) | Durable fact |
| `ltm` | `id` (memory document ID) | Complete replacement fact |
| `profile` | `category`, `key` | Value under existing profile privacy/size rules |
| `kg_node` | `id`, `field` | One property or `label` |
| `kg_edge` | `id` (source), `target`, `relation` | New existing target ID |

No whole-entity deletion is supported. Protected or externally owned entries
remain protected. Read-only and disabled stores are checked before applying.
Co-agents cannot use this tool. The backend freshly reads each target and includes
unambiguous exact fact copies; short scalar matches and other semantic neighbors
are `review_required` candidates, never implicitly authorized edits. Supply explicit
changes for candidates that evidence clearly identifies as the same fact. Arbitrary
paraphrases cannot be linked deterministically. `review_limited` means the bounded
candidate search could not establish complete coverage.

```json
{"operation":"preview","reason":"The user corrected their drink preference","changes":[{"reference":{"store":"core","id":"42"},"operation":"replace","value":"User prefers green tea"}],"evidence":[{"reference":"user:current","quote":"I prefer green tea"}]}
```

When evidence is clear, continue immediately with
`{"operation":"apply","id":"<returned-id>"}` without another approval round.
Apply and `status` accept only `operation` and the saved ID. Old content stays
in the protected database history; old LTM vectors remain archived. Pending
replacement vectors are invisible to normal memory retrieval.

## Result and recovery

Report which facts changed and which candidates still need review. `applied`
means all planned changes were reread and verified; `applied_with_review` also
has unresolved candidates. Never claim all copies were fixed in that case.
`incomplete` or `blocked` is not success, even if some items are `done`.
Use `status` with the ID to inspect durable per-step progress. Retries resume
the same ID only while saved target states, permissions and protections still
match. Changed facts require a new preview and fresh evidence. A later real
change in preference is a new correction, not a replay of an old operation.
