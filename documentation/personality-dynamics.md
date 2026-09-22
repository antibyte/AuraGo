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
one SQLite transaction. Event IDs are hashed in the receipt ledger. New event
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
  `event_id` (maximum 128 characters) deduplicates request retries. Older clients
  may omit it; those requests each count as a new observation.
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

Run `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestPersonalityDynamics -count=1`
and `npm run check:ui`. The browser test exercises real Config rendering at
desktop/mobile widths, reset success/failure, stale polling, draft preservation
and the Dashboard renderer. Response tests use deterministic fixture models;
qualitative behavior of a deployed provider remains model-dependent.
