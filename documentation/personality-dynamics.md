# Personality dynamics

The existing `personality.engine` switch enables a shared, persistent personality
state across sessions and channels. Local observation processing uses Go only and
runs before the first response. V2 analysis, emotion synthesis and inner voice
remain controlled by their existing switches. There is no new activation switch,
model dependency, memory database, or response rewriting loop.

## State and evidence

`internal/memory/personality_dynamics.go` owns deterministic integration.
`personality_observations.go` commits affect, dynamics, bounded trait updates,
observation receipts, mood history and optional emotion/inner-voice history in
one SQLite transaction, together with profile contributions and their provenance.
`RunConfig.HumanInput` is captured at authenticated human ingress before media
enrichment. It contains the unchanged current text, session, opaque turn identity
and original event time. An empty caption stays empty. History, tool results,
recovery messages, snapshots and model JSON cannot create this proof. Only the
current input supports profile or relationship evidence; assistant context is
compact and explicitly labelled. Oversized human sources skip optional analysis
instead of contributing facts extracted from a truncated source.

Event IDs are hashed in the receipt ledger. New event
records contain codes and numerical values, not conversation or tool-output text.

Primary observations from concurrent channels integrate serially without losing
events; reset and persona changes still invalidate their old context. Each human
turn has one primary observation. Its asynchronous helper result can
enrich that observation once, guarded by revision, reset epoch, persona and turn
identity. A newer turn on another channel supersedes an older analysis. Technical
events can also make a captured revision stale; the safe fallback is the local
state, not a retry against a newer snapshot. Tool completion paths consume the
host's confirmed result status, never a success/error claim inside output text.
Suppressed relay runs produce no personality observations.

Familiarity uses the existing `affinity` trait, including its configured bounds and
long-term decay. Short-term friction is separate. Only agent-directed human
praise, criticism or confirmed repair with confidence at least 0.8 may affect the
relationship. Local recognition deliberately accepts only a small set of clear
complete German/English feedback phrases; V2 can supply structured evidence for
other wording and languages. Uncertain irony, third-party complaints, technical
failures and the agent's own apology cannot establish relationship effects.

Technical progress reduces load. Human praise or confirmed repair reduces
friction gradually. Repeated recovery evidence can contribute to existing daily
character reflection after three confirmed recoveries. A conflict episode earns
at most one recovery until new criticism; technical recovery requires prior load.
Transient mood/cause values are not sent as durable evidence to character
reflection. Existing two-notes-per-day limits and user-deletion tombstones apply.
These dynamics are never automatically copied into Core Memory.

## Initial calibration

| Quantity | Rule |
|---|---|
| Affect | Existing four-hour half-life |
| Load | Twelve-hour half-life |
| Friction | Twenty-four-hour half-life |
| Mood | Two primary confirmations; explicit feedback and working modes can act immediately |
| Habituation | Fifteen-minute repetition window by event family and origin; independent technical actions stay distinct |
| Adaptive sensitivity | Gain bounded to 0.5–1.25; effective repetition multiplier 0.1–1.25; quiet-time recovery toward 1 |
| Stimulus ledger | At most 32 families, deterministic eviction |
| Semantic affect contribution | Existing maximum absolute delta of 0.15 per axis |
| Familiarity observation | Bounded to -0.015 / +0.03 before sensitivity and saturation damping |

Persona metadata sets baseline sensitivity. Explicit zero volatility and empathy
bias remain meaningful. Creative and analytical working modes remain available
during strain. Familiarity and friction can coexist, for example as familiar
warmth with a measured tone after setbacks. Generated Go hints retain the selected
persona and fit through the existing trusted personality prompt budget. Model
narration remains isolated advisory material and cannot establish tool permissions,
evidence, user intent or task success.

Personality never changes a tool-call limit or confirmation policy. The main
agent receives one Go-generated style directive; co-agents retain their compact
temperament line. Technical words such as `error`, `broken`, `Fehler` and neutral
`again` do not imply displeasure with the agent, and short messages are not praise.

Time projection uses elapsed time across restarts, clamps clock reversals, and
does not write during dashboard reads. Reads cannot count as mood confirmations.
The synthesizer retains its shared cooldown and in-flight reservation. Reset or
persona changes invalidate previous narration even when it remains in history.

## API and interface

- `GET /api/personality/state` retains existing fields and adds `dynamics`:
  `version`, `revision`, `load`, `friction`, `familiarity`, `trend`, `reasons`,
  `recoveries`, `updated_at`. `trend` is `steady`, `strained` or `recovering`;
  `reasons` contains at most four host-owned cause codes.
- `POST /api/personality/feedback` retains all six feedback types. Optional
  `event_id` (maximum 128 characters) deduplicates request retries. A new ID requires
  `occurred_at` in RFC3339 format. Preserve both on retries. Events expire after
  90 days; at most five minutes of future clock skew is accepted. Expired events
  return 410, conflicting reuse of an ID returns 409, and a missing timestamp on
  a new ID returns 400. Already stored pre-upgrade IDs remain no-op retries without
  a timestamp during their remaining lifetime. Requests without an ID are fresh
  individual events. Startup and the independent privacy worker clean expired
  receipts in batches; there is no cap that evicts valid receipts.
- `GET /api/personality/legacy?after=0&limit=50` lists quarantined text for
  administrators. `POST /api/personality/legacy/review` accepts `id`, `fingerprint`
  and `action` (`discard` or `adopt`). Changed review state returns 409. Both routes
  require administrator access; writes retain normal CSRF protection. Only profile
  and character entries can be adopted. Emotion and old inner-voice text can be
  read or discarded, never reactivated as current emotion.
- `POST /api/personality/dynamics/reset` resets affect, load, friction, pending
  confirmations and adaptive short-term state. It preserves all traits including
  familiarity, selected persona, character notes and history. It uses the same
  authentication and CSRF protection as other personality writes and rejects
  requests while the engine is disabled.

Config → Personality and Dashboard → Personality display the three indicators,
current trend and a clearly labelled reset action. The existing Dashboard mood
and affect timelines provide event history. The shared widget is translated into
all 16 supported UI languages, rejects older response revisions and preserves the
displayed state if reset fails. It never writes config drafts.

## Migration and provenance

Dynamics schema version 1 adds singleton state and observation receipts. Existing
affect and affinity are preserved, load and friction start at zero, and old
conversations are not replayed. Before migrating populated on-disk personality
data, SQLite creates a consistent sibling backup named
`<database>.personality-dynamics-v1-<random>.bak`. A backup failure aborts migration.
The backup contains the original database's private data and should be handled
like the database itself. In-memory/fresh stores do not require a backup.

The evidence migration additionally takes a protected consistent
`personality-evidence-v1` backup and rehearses its transaction on a private copy
before modifying a disk store. Automatic profile, character, emotion and old
mood-trigger texts without reliable provenance move into `personality_legacy_review`.
Manual entries remain active. Quarantine has no normal model, history, tool or
reflection reader. Config → Personality provides pagination, explicit adoption
and deletion with a review fingerprint. Protected notes retain their protection.
Migration is idempotent; SQL failure rolls back the transaction.

Each automatic profile confirmation is one contribution from a unique turn.
Contradictory values keep separate provenance, and deletion recomputes the active
value and confidence from surviving evidence. Manual corrections always win.
Explicit profile deletion also fences analyses of older input. Conversation-bound
emotion and character text has session provenance. Deterministic character notes
are marked `code_generated`; model reflection uses only conversation-free state
and similarly sourced notes, marked `state_reflection`.

Chat privacy jobs capture candidates before purging the transcript, then recheck
sharing, fingerprints and protection transactionally. Deleted sessions reject late
personality writes. Without `delete_memories`, derived facts remain while direct
transcript text and identifying references are scrubbed. With it, the associated
contributions and exclusively owned text are removed. Shared, manual, protected
or changed candidates remain reviewable. Numeric character development survives.
The additive counters `personality_deleted` and `personality_retained` count
derivative candidates; a shared profile can remain while its deleted session's
contributions are removed. `personality_legacy_unknown` reports unassigned
quarantine entries and prevents a claim of complete cleanup.

## Configuration

Defaults are `engine: false`, `engine_v2: false`, `core_personality: neutral` and
`user_profiling_threshold: 3`. V2 enables the engine; emotion narration enables V2;
Inner Voice enables both. A configured helper is preferred. The existing bounded
main-model fallback remains available and is charged to the personality budget.
Invalid optional emotion JSON does not discard valid mood, trait or profile
analysis, request a second call, or replace an older inner voice. Narration on
mood change compares the Go-owned mood with the last narration; cooldown still
applies. User stop, shutdown and session deletion do not trip the circuit breaker.

`inner_voice.decay_max_age_secs` is the absolute age ceiling (default 300 seconds),
in addition to turn-based expiry. `error_streak_min` is retired: old files load
with a deprecation notice, and the UI/schema omit the unused field. Persona
`thresholds`, anchors and decay resistance remain supported; unknown metadata
and invalid values are rejected visibly. Invalid disk profiles produce diagnostics
and retain safe embedded fallbacks.

Both legacy V2 API-key spellings are rejected in config writes and hidden in
config reads. Startup migrates a key only to an unambiguous matching provider and
never overwrites a saved credential. Otherwise it uses a separate, agent-hidden
legacy Vault slot. YAML is cleaned only after successful Vault storage.

The implementation is independently authored Go. Conceptual references are
[Totemheart at 9aec8a2](https://github.com/AlejoMalia/Totemheart/tree/9aec8a2d5fe59fbfa9f3798ea2493af09af8b37d),
its GestaltAttractorEngine (inertia), AutoWeightingEngine (adaptation),
LoveHateEngine (coexisting relationship signals) and RepairProtocol (confirmed
repair). No upstream implementation, prompt or parameter table is used.
Parameters are AuraGo scenario defaults, not empirically validated claims about
human emotion; see the upstream
[calibration limitations](https://github.com/AlejoMalia/Totemheart/blob/9aec8a2d5fe59fbfa9f3798ea2493af09af8b37d/CALIBRATION.md).

## Verification

Run the focused `Personality|Emotion|Affect|Character|FinalizeTool|AnalyzeMood`
tests in `internal/memory`, `internal/agent`, `internal/prompts` and
`internal/server`. They cover numerical bounds, elapsed time, hysteresis, repeat
attenuation, semantic deduplication, migration/restart, transactional rollback,
concurrent channels, stale results, resets and relationship provenance.

Run `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestPersonality(Dynamics|Legacy)' -count=1`,
`node scripts/build-ui-bundles.js --check` and `npm run check:ui`. Browser tests
exercise real Config rendering at desktop/mobile widths, reset success/failure,
stale polling, draft preservation, quarantine review, feedback and the Dashboard
renderer. HTTP and browser runners must permit local loopback listeners.
Response tests use deterministic fixture models;
qualitative behavior of a deployed provider remains model-dependent.
