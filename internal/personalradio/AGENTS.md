# Personal Radio

## Ownership

This package owns station profiles, the validated radio library, rotation,
production reservations, listener leases and scheduling. Provider adapters and
authenticated endpoints live in `internal/server/personal_radio_*.go`. The
desktop player and controls live in `ui/js/desktop/apps/personal-radio*.js`.
User documentation: `documentation/personal-radio.md`.

## Contracts

- Store only app-owned data in `<data_dir>/personal-radio`. SQLite profiles,
  plays, quotas, news provenance and produced-file recovery survive restart.
  Playback restarts stopped with a new epoch; stale queues and transient speech
  are never replayed automatically. Original generated music remains in the
  shared media library. Imports copy MP3/PCM WAV; never modify their sources.
- Full bounded decode and durable registration precede airtime. Music reserve
  excludes in-flight jobs, duplicate files, speech and invalid files. Strict
  rotation must satisfy the start reserve using currently schedulable tracks.
- Default startup needs two ready music tracks and no fixed minute reserve.
  Generator latency must not increase the requirement. Migrate only the former
  factory 30-minute/eight-track setting once; retain other explicit reserves.
  Discover suitable registry music through the Library adapter before buying
  more. Decode independently of the shared speech/music accelerator and allow
  playback before the scan finishes. Preserve blocked/removed associations and
  station-specific genres; repair missing cached copies from valid originals.
  Generated/mixed modes still grow the library and, once full, request one fresh
  track per start and after four more music starts, within the existing quotas.
- Attempt one opening moderation per start epoch unless moderation is off.
  Pass registry preparation status, actual eligible track count and music duration/requirements to the
  tool-free planner. Give opening planning/TTS up to 45 seconds before scheduling
  long music work on the shared accelerator; a failure releases music production.
  Only a fully prepared opening may play before `MusicReady`. It never counts
  toward the music reserve, never promises an ETA and is not repeated while waiting.
  Start normal editorial/news only after music readiness; research news during music.
- Use one active owner and epoch. Window disposal does not stop radio. Explicit
  stop and lease expiry cancel production; late completions cannot revive it.
  Duplicated start, stop, skip and playback events cannot consume two tracks or
  count scheduled music as heard. No timer manufactures playback history.
- Persist production quota reservations before requests, including failed or
  interrupted attempts. Persist successful provider file receipts before
  registration/import, including prompt, style, lyrics, language, provider/model,
  duration, generation time, cost and tags. Enrich an already registered media ID.
  Retry these stages without buying another generation even when the pool is full.
  Daily allowances use UTC. No automatic durable music eviction.
- `Issue` only records/resolves sanitized operational issues through the
  existing supervisor lifecycle. Never emit chat, SSE chat or Telegram notices.
- Editorial calls are tool-free, route-budgeted and isolated from private chat
  and memory. Fetched sources remain untrusted data. Validate source IDs and
  speech size. News requires a known publication date and expires after one
  interval. Deduplicate already aired, unchanged sources. A missing bulletin
  does not stop music or assert that no news exists.
- The browser fetches bounded sample-aligned WAV windows and schedules two
  music segments plus intervening speech on one Web Audio clock. Initial
  music playback requires a prepared next segment, including after an opening.
  The one opening may play alone. Music crossfades; other speech uses a
  title boundary. Server queue updates
  cannot revive cancelled browser work. Explicitly retain the desktop runtime
  when disposing a window and restore other media handlers after stopping.

## Verification

Run package tests plus server `TestPersonalRadio*`. Browser tests opt in with
`AURAGO_RUN_BROWSER_SMOKE=1`; they use the real desktop shell, authenticated API
contracts, active Speech Lab mocks and real Web Audio. Continuity coverage
renders 120 music and 120 mixed transitions with OfflineAudioContext. Simulated scheduler time does
not establish an actual 90-minute hardware/provider acceptance run.

Rebuild and check UI bundles after shell edits. Full runtime acceptance needs a
binary bound to its packaged assets and real configured music/TTS providers.

## Child DOX Index

None.
