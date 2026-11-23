# Video Studio — plan mode (agent-planned clips) — design

Date: 2026-11-23 · Status: approved in brainstorming (sections 1–6 confirmed by the user); written spec awaiting
user review. Builds on Video Studio Pro phases 0–2 (`2026-10-15-video-studio-pro-roadmap.md`,
`2026-10-16-video-studio-pro-effects-animation-design.md`) and reuses the agent-job pattern of the Storymaker agent
core (`docs/superpowers/plans/2026-11-06-storymaker-agent-core.md`).

## 1. Goal

The user describes an idea ("30 seconds: a fisherman heads out at dawn, the day wakes over the bay"). The agent plans
a complete clip as a storyboard of scenes: per scene a video prompt, durations, a start image (keyframe) and the
transition to the previous scene, plus narration and a music prompt. The user reviews and edits the storyboard and
approves it. The server then generates keyframes, narration, video clips and music, chains seamless transitions by
using the last frame of a scene as the start image of the next one, and builds the timeline with transitions. Every
plan respects what the active video generation API can actually do.

## 2. Non-goals (this design)

- Agnes AI (and any provider without a supported image mode) in plan mode — see §4.4.
- A temporary public image URL mechanism (rejected by the user for now).
- Real audio ducking (volume keyframes), subtitles from narration, lip sync.
- Agent review of generated clips by vision models.
- Several plans or plan variants per project.
- Automatic resubmission of a provider request whose outcome is uncertain.

## 3. Decisions from brainstorming

| Topic | Decision |
|---|---|
| Flow | Agent plans a storyboard; the user reviews, edits and approves; only then paid generation starts |
| Agnes | Left out of plan mode for now (images only as public URLs; no public URL mechanism) |
| Entry points | In-app plan panel in Video Studio **and** a chat tool; approval always in the app |
| Start images | Scenes that do not continue the previous scene get a keyframe from image generation when it is configured; library images can be assigned; without image generation text-only |
| Audio | Clip audio, plus music and narration |
| Architecture | Approach A: the agent plans, a deterministic server-side runner executes. Rejected: B (agent drives every step: long LLM loop, fragile, costly), C (browser orchestration: window must stay open, chat plans never run) |
| Slicing | One spec, four slices, each with its own plan after the previous one lands (§13) |
| Limits | Max 12 scenes, max 120 s per plan, max 4 scenes per seamless chain |
| Transitions | Seamless, bridge, cut, blend (§4.2) |
| Plan store | `plan.json` in the project folder, one plan per project |
| Panel | "Edit / Plan" switch; storyboard cards; a regenerated scene swaps only the asset of its existing timeline clip |
| Agent | Three phases, job-private plan tools without cost effects, one Go validator for agent, panel and approval |
| Execution | Two approvals (preview: keyframes + narration; then videos); own provider lane; dependency graph; timeline as a new revision with a version snapshot |
| Mix | Narration rendered in the preview step and measured; music after video approval in parallel; fixed levels |
| Chat tool | `create`, `status`, `open` only |

## 4. Capability profile and transitions

### 4.1 Capability profile

One Go function in a new file `internal/server/video_studio_generation_profile.go` (it reads config types) derives the
profile from provider type, resolved model and resolution. It is the single source for the validator, the agent prompt, the status JSON (`generation.profile`) and the
UI. The existing scattered rules `videoStudioDurationOptions` and `videoStudioImageModes`
(`internal/server/video_studio_handlers.go:261-299`) and the provider checks in the `generate` job admission
(`video_studio_handlers.go:1004-1060`) move onto it, so manual generation and plan mode cannot drift apart.

| | MiniMax Hailuo-2.3 / -2.3-Fast | MiniMax Hailuo-02 | Google Veo 3.x | Agnes / other |
|---|---|---|---|---|
| plan mode | yes | yes | yes | no (reason code) |
| durations | 6/10 s (1080P: 6) | 6/10 s (1080P: 6) | 4/6/8 s (1080P or references: 8) | – |
| first frame | yes | yes | yes | – |
| last frame | no | yes (768P/1080P) | no until the adapter sends `instances[0].lastFrame` and a live check passes | – |
| native audio | no | no | yes | – |
| aspect ratio | follows the first frame (no aspect parameter) | follows the first frame | 16:9 / 9:16 | – |

Profile fields: `plan_supported`, `reason`, `durations_seconds`, `resolutions`, `aspect_ratios`, `aspect_from_first_frame`,
`first_frame`, `last_frame`, `native_audio`, `max_prompt_chars`, `cost_per_clip_estimate` (today's values: MiniMax
0.20, Veo 0.50), `parallel` (default 2, capped by config).

Facts behind the table (checked 2026-11-23): MiniMax documents first+last frame (FL2V) only for MiniMax-Hailuo-02 at
768P/1080P (6 s at both, 10 s at 768P); Hailuo-2.3 is documented for first-frame image-to-video only. Veo 3.1 supports
last frame and reference images per Google, but AuraGo's adapter sends only `instances[0].image` today and forum
reports show field-placement mismatches; enabling Veo last frame is a later, separately verified step.

### 4.2 Transition kinds

The agent picks one kind per scene boundary; the validator accepts only what the profile allows.

- **Seamless** (`continue`): scene i+1 starts with the frame at scene i's out point. It is generated after scene i.
  On the timeline it is a hard cut and scene i+1 drops its first frame (the duplicate). Requires `first_frame`.
  A seamless chain holds at most 4 scenes (quality drifts with each image-to-video pass).
- **Bridge** (`bridge`, only with `last_frame`): a boundary keyframe K is the last frame of scene i and the first frame
  of scene i+1. Both scenes can be generated in parallel and still join seamlessly. Scene i is used to its end (no
  tail trim). Requires image generation (K is generated) or a library image.
- **Cut** (`cut`): scene i+1 is independent and starts from its own keyframe (generated, library image, or none =
  text only).
- **Blend** (`blend`): independent like a cut, joined by one of Video Studio's transitions (dissolve, fade to black,
  wipe, …). Types the server's FFmpeg lacks (`unavailable.transitions` in the status JSON) are refused. The overlap
  (0.5–1 s) is accounted for in the scene durations and must fit both clips.

### 4.3 Plan limits

Max 12 scenes and 120 s total. Before approval the server checks remaining daily slots (`video_generation.max_daily`
via `tools.VideoCounterGet`, image and music limits likewise) and the `video_generation` budget block. The estimated
total cost is shown on the approval button.

### 4.4 Unsupported providers

Agnes accepts start/end images only as publicly reachable URLs and AuraGo keeps project files private, so its profile
reports `plan_supported: false` with reason `provider_images_public_url_only`. Unknown providers report
`provider_unsupported`. The panel explains the reason and links to Settings > Video Generation.

## 5. Plan model

`plan.json` lives next to `project.json` in `Documents/Video Studio/<project>/` and uses the same ETag concurrency as
the project. Max size 256 KiB; every text field has a length cap (prompts ≤ profile `max_prompt_chars`, narration
≤ 600 chars, idea ≤ 4000 chars, titles ≤ 120 chars).

```json
{
  "version": 1,
  "id": "plan_…",
  "status": "planning | review | running | done | partial | failed | stopped",
  "idea": "…",
  "target_seconds": 30,
  "language": "de",
  "profile": { "provider": "minimax", "model": "MiniMax-Hailuo-2.3", "resolution": "768P" },
  "style": "shared look, camera language, lighting",
  "characters": [ { "name": "Fischer", "description": "…", "reference_asset_id": "" } ],
  "scenes": [ {
    "id": "s1",
    "title": "Hafen im Nebel",
    "video_prompt": "…",
    "negative_prompt": "",
    "generate_seconds": 6,
    "use_seconds": 5.5,
    "transition_in": { "kind": "cut | continue | bridge | blend", "type": "dissolve", "seconds": 0.8 },
    "keyframe": { "source": "generated | library | chain | bridge | none", "image_prompt": "…", "asset_id": "" },
    "narration": { "text": "…", "asset_id": "", "measured_seconds": 0 },
    "state": "pending | keyframe_ready | generating | ready | failed | uncertain | stale",
    "clip_asset_id": "", "still_asset_id": "", "timeline_clip_id": "",
    "cost": 0, "error": ""
  } ],
  "audio": { "voice": "", "music": { "prompt": "…", "instrumental": true, "asset_id": "" }, "clip_audio": "keep | duck | mute" },
  "cost": { "estimate": 0, "spent": 0 }
}
```

- `profile` is a snapshot taken when planning starts. If the active configuration no longer matches before approval,
  the panel shows a banner and approval requires a fresh validation (`profile_changed`).
- The server prepends `style` and the descriptions of the characters named in a scene to every video and image prompt;
  the agent refers to characters by name only.
- Scene `stale` marks seamless successors of a regenerated scene; the panel offers to regenerate them too.

## 6. Storyboard panel (UI)

- A segmented switch "Bearbeiten | Plan" (localized) in the Video Studio header. In plan mode a storyboard replaces the
  preview and the timeline; the library stays.
- Top: the idea and an agent status line (phase, provider/model/resolution).
- Scene cards: keyframe thumbnail (or "from scene n" for chained scenes), title, prompt, duration chip
  (use/generate), keyframe source, narration text with a play button once rendered. Inline editing of all text fields
  and choices; edits are saved through the same validator.
- Between cards: a transition chip (kind, type, seconds).
- Card actions: "Szene überarbeiten" (revise with a note, §7.4), "Keyframe neu", "Neu erzeugen" (after a run),
  "Mediathek-Bild zuweisen", "Nur Text".
- Bottom bar: total duration, number of clips/keyframes, audio parts, free daily slots, and the current primary action:
  "Plan erstellen" → "Vorschau erzeugen (≈ x $)" → "Freigeben und erzeugen (≈ y $)" → "Timeline bauen" (when partial).
- During a run each card shows its state; finished cards show the clip poster. "Stoppen" and, after a restart,
  "Fortsetzen" are available.
- Plan assets live in a media library folder "Plan: <title>" (existing folders feature).
- Phone width: cards stack, the bottom bar wraps; no horizontal overflow.

## 7. Agent planning job

### 7.1 Job

A server-side agent job modelled on the Storymaker agent (`internal/server/storymaker_agent.go`): own loop, own
tools via `agent.ExecutionHooks`, project and plan bound from the job only, never from tool parameters. The LLM is the
main agent's unless `video_studio.plan.agent_provider` / `agent_model` are set. Models without native tool calls use
the text mode known from Storymaker. One running planning job per project; at most
`video_studio.plan.max_daily_jobs` (default 20) per day. Desktop read-only refuses planning.

### 7.2 Phases

1. **Outline:** scene list with titles, use/generate durations and transition kinds.
2. **Details:** video and keyframe prompts, style block, characters, narration, music prompt.
3. **Check round:** run the validator and fix until no blocking issue remains (max 3 rounds), then `review`.

### 7.3 Tools (job-private, never in the general chat tool list)

- `vs_plan_read` — plan, profile and the latest check report.
- `vs_plan_write` — meta, style, characters, all scenes (validated on write; issues returned).
- `vs_plan_patch_scene` — change one scene by id.
- `vs_plan_check` — validator report.
- `vs_plan_finish` — submit for review; refused while blocking issues exist.

No agent tool spends money. Paid work starts only from the user's approval clicks.

### 7.4 Revise mode

"Szene überarbeiten" starts a short job in revise mode with the user's note; it may change only that scene and its
two transitions.

### 7.5 Prompt craft rules (short, also for weak models)

- One shot per clip with one main action; no location change inside a clip.
- Name camera movement, subject, action, setting and light explicitly.
- Seamless scenes describe the continuation of motion from the start frame, not the scene from scratch.
- Keyframe prompts describe exactly the first frame.
- Characters appear by name; their look is in the character block.
- Narration fits the word budget (§8).

## 8. Validator

Pure Go, shared by agent tools, panel edits (PUT) and approval:

- `generate_seconds` ∈ profile durations; `0 < use_seconds ≤ generate_seconds`.
- Transition kinds allowed by the profile; no `bridge` without `last_frame`; `continue` and `bridge` need `first_frame`.
- Seamless chains ≤ 4 scenes; first scene has no incoming transition.
- Blend overlap ≤ min(use_seconds of both scenes) − 0.5 s; blend type available on this server.
- Total length (use durations minus blend overlaps) within ±15 % of `target_seconds`, ≤ 120 s; ≤ 12 scenes.
- Prompt lengths ≤ profile limit; all text caps from §5.
- Narration word budget: German ≈ 2.2, English ≈ 2.5 words per second of `use_seconds` (other languages 2.3),
  minus 2 words. After the preview step the measured length replaces the estimate: a narration may run at most 1 s
  into the next scene, otherwise approval is blocked ("shorten or revise").
- `keyframe.source = generated` only when image generation is configured; `library` needs an image asset of this
  project; `chain` only for `continue`; `bridge` only for `bridge`.
- Narration fields only when TTS is ready (`tools.TTSReady`); music only when music generation is configured.

Issues carry a code, the scene id and a short text; blocking vs. warning is part of the code table.

## 9. Execution

### 9.1 Two approvals

1. **Preview** ("Vorschau erzeugen"): keyframe images (text-only request to the image provider) and narration TTS.
   Cheap; the user sees thumbnails and hears the narration before videos are paid. Single keyframes can be
   regenerated.
2. **Videos** ("Freigeben und erzeugen"): requires a keyframe for every scene whose source needs one (or the scene is
   switched to text-only) and measured narration that fits. The request carries the estimate the user saw; the server
   refuses with `estimate_changed` if its own estimate is higher.

### 9.2 Provider lane

The Video Studio manager has one FFmpeg job worker per server (`video_studio_service.go:32`); a generation blocks it
for minutes today. A new provider lane runs provider calls (video, image, TTS, music) outside that worker with bounded
concurrency (`video_studio.plan.generation_concurrency`, default 2, never above the profile's `parallel`). The
existing manual `generate` job moves to this lane too. The import code of `processGenerate`
(`video_studio_worker.go:512-728`: size checks, copy into the project, probe, poster, budget record, media record) is
extracted into one shared function used by both the manual job and the plan runner.

### 9.3 Plan runner

One goroutine per running plan; at most one per project and `video_studio.plan.max_running` (default 2) per server.
It executes a dependency graph:

- cut and blend scenes start as soon as their keyframe (if any) exists → parallel;
- a seamless scene i+1 waits for clip i; then the existing still extraction (the `still` job's code path, run inline
  in the lane) captures the last frame clip i shows on the timeline (`offset + round(use_seconds × fps) − 1`, where `offset` is 1 when
  clip i is itself a seamless successor, else 0) as an image asset, which becomes the
  first frame of i+1;
- a bridge's boundary keyframe K is the last frame of i and the first frame of i+1; both run in parallel;
- music starts right after video approval (the total length is known then).

State is written to `plan.json` after every step. Images are normalized with `videoStudioGenerationImagePayload`.

### 9.4 Format

Keyframes are generated in the project's aspect ratio, mapped to the closest image generation size, so providers
without an aspect parameter (MiniMax) produce portrait clips from portrait keyframes. Clips that still differ are
fitted by the existing cover fit. (Verify MiniMax's output aspect from a portrait first frame in the slice 2 spike.)

### 9.5 Timeline assembly

Runs when all scenes are `ready` ("Timeline bauen" after a partial run):

- creates a project version snapshot first (existing versions feature), then writes one new project revision under
  the project lock with the usual canonicalize/validate path;
- clips start at the current end of the timeline (0 in a new project); seamless joins become hard cuts and the
  successor's offset skips its first frame; blends become Video Studio transitions with the planned type/length;
- narration and music on their own audio tracks (§10);
- the plan stores the timeline clip ids; a later regeneration swaps the asset of an existing plan clip and keeps the
  user's trims and effects; a removed clip is not recreated automatically;
- an open editor picks up the revision through the existing revision/conflict flow.

### 9.6 Money and safety

- Before every paid call: budget block, daily limit and project storage are checked; a hit sets the plan `stopped`
  with the reason.
- Automatic retry only when the provider was provably not reached (pre-send transport failure).
- An uncertain outcome sets the scene `uncertain` and is never resent automatically (existing rule); the user chooses
  "erneut erzeugen" or "als fehlgeschlagen markieren".
- "Stoppen" submits nothing new; in-flight paid requests still import their results.
- After a server restart a running plan is `stopped` with "Fortsetzen".
- Costs are recorded per scene and summed in `cost.spent`.

## 10. Narration and music

- **Narration:** rendered in the preview step through the existing TTS configuration (route `video_studio`, else the
  default voice; voice choice in the panel where the TTS provider supports it). Each narration clip starts 0.3 s after
  its scene's start on its own audio track; at most 1 s spill into the next scene (§8). No TTS → no narration fields.
- **Music:** one prompt from the agent, instrumental by default when narration exists. Generated after video approval
  through the existing music generation (ACE-Step local, MiniMax, Lyria) with the target length where the provider
  supports it. Own audio track, 1 s fade-in, 2 s fade-out; longer music is trimmed, shorter music ends with its fade and
  a notice (no looping). Music generation off → no music part.
- **Levels** (editable afterwards like any clip):

| Track | with narration | without narration |
|---|---|---|
| Narration | 1.0 | – |
| Music | 0.25 | 0.7 |
| Clip audio, `duck` (default) | 0.35 | 0.6 |

`keep` sets clip audio to 1.0, `mute` to 0. Video Studio clips have a static `volume` and `fade_in`/`fade_out`
(`internal/videostudio/model.go:109-111`); real ducking needs volume keyframes and is a later step.

## 11. API and chat tool

Routes under `/api/desktop/video-studio/projects/{id}/plan` (existing VS auth; paid actions need the same admin scope
as `generate` today):

- `GET plan` (ETag; the panel polls every 2 s while `planning` or `running`, 304 when unchanged), `PUT plan` (user
  edits, `If-Match`, validated);
- `POST plan/agent` `{mode: create|revise, idea, target_seconds, scene_id, note}`, `POST plan/agent/stop`;
- `POST plan/preview` (all or `{scene_ids}`), `POST plan/run` `{estimate}`, `POST plan/stop`, `POST plan/resume`;
- `POST plan/scenes/{sid}/regenerate` `{what: keyframe|clip|narration}`, `POST plan/assemble`.

The status JSON gains `generation.profile` and `plan` availability (image generation, TTS, music ready).

**Chat tool `video_studio_plan`:** `create` (idea, target length, project name → new project, starts the planning job,
returns a link that opens Video Studio in plan mode), `status` (plan state, scenes, cost), `open` (opens the window).
No approve operation. Desktop read-only, the daily job limit and the action policy apply (unattended runs need a rule,
like Storymaker).

Config (`video_studio.plan.*`, config template, config merger, config page section): `enabled` (default true),
`agent_provider`, `agent_model` (empty = main agent), `max_daily_jobs` 20, `generation_concurrency` 2, `max_running` 2.

## 12. Errors, security, tests

**Error codes** (shown with a clear text in the panel): `plan_unsupported_provider`, `profile_changed`,
`image_generation_unavailable`, `tts_unavailable`, `music_unavailable`, `budget_blocked`, `daily_limit`,
`project_size_limit`, `provider_failed` (with HTTP status), `uncertain`, `chain_source_failed` (a seamless successor
waits on its failed predecessor), `estimate_changed`, `assembly_conflict`. A scene fails on its own; everything not
depending on it continues.

**Security:** plan content from chat is untrusted but cannot trigger paid work; images leave the server only after the
video approval (the keyframe step sends text only); approval routes need admin scope; `plan.json` size and field caps;
asset references must be assets of the same project; prompts are not added to INFO logs.

**Tests (lean):**
- Go unit: profile table, every validator rule, scheduler (chains, bridge, parallel cap, stop/resume, uncertain),
  assembly math (1-frame trim, blend overlap, narration placement, music trim, levels) against golden project JSON.
- Server: fake providers (`httptest`) for MiniMax, Veo, image, TTS, music; one full plan run to a project; restart →
  `stopped`; budget block mid-run.
- Agent: scripted model tests for the three phases and text mode.
- Browser: one panel tour with fake providers, including phone width.
- Real providers on aurago-test are the user's acceptance (they cost money).

## 13. Slices

Each slice gets its own implementation plan after the previous one has landed.

1. **Planning (no money):** capability profile (+ moving the existing rules onto it), validator, `plan.json` + API,
   agent planning job (create + revise), storyboard panel with editing, config keys.
2. **Execution (plan mode usable):** provider lane (+ manual `generate` moved), shared import function, preview step
   (keyframes), plan runner graph, still chaining, bridge, timeline assembly, stop/resume/regenerate.
3. **Audio:** narration in the preview step with measurement, music after approval, levels.
4. **Chat and docs:** `video_studio_plan` tool + tools manual, manuals de/en (`documentation/manuals/*/26-video-studio.md`),
   `documentation/video-studio.md`, AGENTS.md contracts.

## 14. Open verification items

- MiniMax: does Hailuo-2.3 accept `last_frame_image`? (Treated as no until verified.) Output aspect from a portrait
  first frame (slice 2 spike).
- Veo: `lastFrame` field placement and supported duration/audio combinations before enabling bridge for Veo.
- Image generation sizes per provider for 9:16 and 1:1 keyframes.
- Music providers' honoured target length (ACE-Step yes; MiniMax/Lyria to check).
