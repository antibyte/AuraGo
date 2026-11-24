# Video Studio plan mode — slice 1 — contracts (drafting brief + reconcile decisions)

Repo: `C:\Users\Andi\Documents\repo\AuraGo` (Go 1.2x module `aurago`, vanilla JS desktop apps). Design spec (approved,
normative): `docs/superpowers/specs/2026-11-23-video-studio-plan-mode-design.md` — read it fully first. Slice 1 = spec
§13 item 1: capability profile (+ move existing rules onto it), validator, `plan.json` + API, agent planning job
(create + revise), storyboard panel with editing, config keys. NOT in slice 1: any provider call that costs money
(keyframes, TTS, video, music), provider lane, runner, assembly, chat tool, manuals.

The plan is written by four parallel drafters (parts A–D). The contracts below are BINDING so the parts fit together.
If you find that a contract cannot work against the real code, keep the name/shape where at all possible and say so in
a "Contract notes" section at the end of your part (the controller reconciles). Never invent a second name for a
thing named here.

## 0. Plan format (every part)

- Output ONE Markdown document as your final answer, starting with the exact first line given in your assignment.
- writing-plans format: tasks `### Task <Part><n>: <name>` (e.g. `### Task A3: ...`), each with **Files:** (Create /
  Modify with function names or line hints / Test), then checkbox steps: write failing test (full code) → run it
  (exact command, expected FAIL text) → implement (full code) → run (expected PASS) → commit (exact message,
  conventional commits, scope `video-studio`). No placeholders, no "similar to Task X", no "add validation".
- For edits of existing files use exact **Old:** / **New:** fenced blocks copied from the current file (enough lines to
  be unique), or **Insert after:** an exact anchor line. For new files give the whole file.
- Go tests: standard `testing`, table tests where natural. Keep tests lean (the user wants essential tests only: the
  behaviour each task adds, no mutation/property/stress suites).
- Commands run from the worktree root. Go test commands must work on Windows Git Bash; use `go test ./<pkg> -run
  '<regex>' -count=1`. Group several names as `-run '(?i)(NameA|NameB)'`.
- Each commit message ends with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Line endings: Go and locale JSON files are LF (`.gitattributes` eol=lf); never convert files.
- Write in English.

## 1. Packages and files (binding names)

Go, new package `internal/videostudio/storyplan` (package `storyplan`, pure Go, no config/server imports):
- `profile.go` — `Profile` type (data only).
- `plan.go` — plan model types, constants, caps, `NewPlan`, `Decode`, `Marshal`, `Normalize`.
- `words.go` — word counting and narration budget.
- `validate.go` — `Env`, `Issue`, issue code constants, `Validate`, `HasBlocking`, `PhaseComplete`.
- tests next to them.

Go, package `server` (`internal/server`):
- `video_studio_generation_profile.go` — `videoStudioGenerationProfile(cfg *config.Config, resolution string) storyplan.Profile`
  plus the thin wrappers `videoStudioDurationOptions` and `videoStudioImageModes` re-implemented on top of it (same
  signatures as today; callers unchanged); the `generate` admission checks in `handleVideoStudioJobCreate` use the
  profile for the provider support / image-mode decisions (behaviour unchanged).
- `video_studio_plan_store.go` — plan.json storage.
- `video_studio_plan_handlers.go` — HTTP routes.
- `video_studio_plan_agent.go` — agent job manager + job run.
- `video_studio_plan_agent_profile.go` — prompts (system sections, workflows, text-mode tool guide, nudges, user message).
- `video_studio_plan_studio.go` — tool-call handling for the five tools (the "studio").
- tests: `video_studio_generation_profile_test.go`, `video_studio_plan_store_test.go`,
  `video_studio_plan_handlers_test.go`, `video_studio_plan_agent_test.go`, `video_studio_plan_studio_test.go`.

Go, package `agent` (`internal/agent`): `native_tools_video_studio_plan.go` — tool schemas; test
`native_tools_video_studio_plan_test.go`.

Go, package `config`: `internal/config/video_studio.go` gains the plan sub-config.

Go, package `tools`: if needed, one exported helper `tools.MiniMaxVideoAPIModel(model string) string` wrapping the
unexported `miniMaxVideoModelForAPI` (returns the API model id).

JS (classic scripts, IIFE, `window.*` globals like the other Video Studio parts):
- `ui/js/desktop/apps/video-studio-plan.js` → `window.VideoStudioPlan` (API calls, plan state, polling, mode switch).
- `ui/js/desktop/apps/video-studio-plan-board.js` → `window.VideoStudioPlanBoard` (DOM of the storyboard: cards,
  transition chips, inline editors, issues, agent status line, start/revise dialogs).
- Both inserted into the Video Studio script list in `ui/js/desktop/core/module-loader.js` immediately before
  `'/js/desktop/apps/video-studio.js'` (board first, then plan).
- CSS: append a section `/* === Plan mode (storyboard) === */` to `ui/css/desktop-app-video-studio.css`.
- Strings: keys `videoStudio.plan.*` in all 16 `ui/lang/desktop/<lang>.json` (cs da de el en es fr hi it ja nl no pl pt
  sv zh), looked up through the existing `tr(s, key, fallback)` (key without the `videoStudio.` prefix, e.g.
  `tr(s, 'plan.modeSwitch', 'Plan')`).
- Browser test: `ui/desktop_video_studio_plan_browser_test.go` (find how existing `ui/desktop_video_studio*browser_test.go`
  tests serve the app and fake the API, and follow that).

## 2. Config (binding)

```go
// in internal/config/video_studio.go
type VideoStudioPlanConfig struct {
	Enabled           bool   `yaml:"enabled" json:"enabled"`
	AgentProvider     string `yaml:"agent_provider" json:"agent_provider"`
	AgentModel        string `yaml:"agent_model" json:"agent_model"`
	MaxDailyJobs      int    `yaml:"max_daily_jobs" json:"max_daily_jobs"`
	JobTimeoutSeconds int    `yaml:"job_timeout_seconds" json:"job_timeout_seconds"`
}
// VideoStudioConfig gains:  Plan VideoStudioPlanConfig `yaml:"plan" json:"plan"`
```
Normalize: MaxDailyJobs 0 → 20, clamp 1..1000; JobTimeoutSeconds 0 → 900, clamp 120..3600; trim provider/model.
Template (`config_template.yaml`, under `video_studio:`): `plan:` with `enabled: true`, `agent_provider: ""`,
`agent_model: ""`, `max_daily_jobs: 20`, `job_timeout_seconds: 900`, each with a short trailing comment like the
storymaker block. Part A finds and updates whatever else the repo requires for a new config key (config merger
tests, config UI section if Video Studio has one, setup/defaults tests).

## 3. storyplan types (binding names and JSON)

```go
package storyplan

type Profile struct {
	PlanSupported        bool     `json:"plan_supported"`
	Reason               string   `json:"reason,omitempty"`   // provider_images_public_url_only | provider_unsupported | generation_unavailable
	Provider             string   `json:"provider"`           // "minimax" | "google" | "agnes" | ...
	Model                string   `json:"model"`
	Resolution           string   `json:"resolution"`
	DurationsSeconds     []int    `json:"durations_seconds"`
	AspectRatios         []string `json:"aspect_ratios"`
	AspectFromFirstFrame bool     `json:"aspect_from_first_frame"`
	FirstFrame           bool     `json:"first_frame"`
	LastFrame            bool     `json:"last_frame"`
	NativeAudio          bool     `json:"native_audio"`
	MaxPromptChars       int      `json:"max_prompt_chars"`
	CostPerClipEstimate  float64  `json:"cost_per_clip_estimate"`
	Parallel             int      `json:"parallel"`
}

const (
	StatusPlanning = "planning"; StatusReview = "review"; StatusRunning = "running"; StatusDone = "done"
	StatusPartial = "partial"; StatusFailed = "failed"; StatusStopped = "stopped"
)
const (
	TransitionCut = "cut"; TransitionContinue = "continue"; TransitionBridge = "bridge"; TransitionBlend = "blend"
)
const (
	KeyframeGenerated = "generated"; KeyframeLibrary = "library"; KeyframeChain = "chain"; KeyframeBridge = "bridge"; KeyframeNone = "none"
)
const (
	SceneRuntimePending = "pending"; SceneKeyframeReady = "keyframe_ready"; SceneGenerating = "generating"; SceneReady = "ready"
	SceneFailed = "failed"; SceneUncertain = "uncertain"; SceneStale = "stale"
)
const (
	ClipAudioKeep = "keep"; ClipAudioDuck = "duck"; ClipAudioMute = "mute"
)
const (
	MaxPlanBytes = 256 << 10; MaxScenes = 12; MaxTotalSeconds = 120; MaxChainScenes = 4
	MaxIdeaChars = 4000; MaxTitleChars = 120; MaxNarrationChars = 600; MaxStyleChars = 2000
	MaxCharacters = 6; MaxCharacterDescriptionChars = 600; MaxNoteChars = 1000
	MinTargetSeconds = 5
)

type ProfileRef struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Resolution string `json:"resolution"`
}
type Character struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	ReferenceAssetID string `json:"reference_asset_id,omitempty"`
}
type Transition struct {
	Kind    string  `json:"kind"`              // cut | continue | bridge | blend ; the first scene uses "cut"
	Type    string  `json:"type,omitempty"`    // blend only: a Video Studio transition name
	Seconds float64 `json:"seconds,omitempty"` // blend only
}
type Keyframe struct {
	Source      string `json:"source"`                 // generated | library | chain | bridge | none
	ImagePrompt string `json:"image_prompt,omitempty"`
	AssetID     string `json:"asset_id,omitempty"`
}
type Narration struct {
	Text            string  `json:"text,omitempty"`
	AssetID         string  `json:"asset_id,omitempty"`
	MeasuredSeconds float64 `json:"measured_seconds,omitempty"`
}
type Scene struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	VideoPrompt     string     `json:"video_prompt"`
	NegativePrompt  string     `json:"negative_prompt,omitempty"`
	GenerateSeconds int        `json:"generate_seconds"`
	UseSeconds      float64    `json:"use_seconds"`
	TransitionIn    Transition `json:"transition_in"`
	Keyframe        Keyframe   `json:"keyframe"`
	Narration       Narration  `json:"narration"`
	State           string     `json:"state"`
	ClipAssetID     string     `json:"clip_asset_id,omitempty"`
	StillAssetID    string     `json:"still_asset_id,omitempty"`
	TimelineClipID  string     `json:"timeline_clip_id,omitempty"`
	Cost            float64    `json:"cost,omitempty"`
	Error           string     `json:"error,omitempty"`
}
type Music struct {
	Prompt       string `json:"prompt,omitempty"`
	Instrumental bool   `json:"instrumental"`
	AssetID      string `json:"asset_id,omitempty"`
}
type Audio struct {
	Voice     string `json:"voice,omitempty"`
	Music     Music  `json:"music"`
	ClipAudio string `json:"clip_audio"` // keep | duck | mute ; default duck
}
type Cost struct {
	Estimate float64 `json:"estimate"`
	Spent    float64 `json:"spent"`
}
type Plan struct {
	Version       int         `json:"version"` // 1
	ID            string      `json:"id"`      // "plan_" + uid
	Status        string      `json:"status"`
	Idea          string      `json:"idea"`
	TargetSeconds int         `json:"target_seconds"`
	Language      string      `json:"language"`
	Profile       ProfileRef  `json:"profile"`
	Style         string      `json:"style"`
	Characters    []Character `json:"characters"`
	Scenes        []Scene     `json:"scenes"`
	Audio         Audio       `json:"audio"`
	Cost          Cost        `json:"cost"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

func NewPlan(id, idea string, targetSeconds int, language string, ref ProfileRef) Plan // status planning, empty scenes, audio.clip_audio duck, music.instrumental true
func Decode(data []byte) (Plan, error)   // rejects > MaxPlanBytes, unknown fields, version != 1; never panics
func Marshal(p Plan) ([]byte, error)     // json; errors when the result exceeds MaxPlanBytes
func Normalize(p *Plan)                  // trims strings; empty/duplicate scene ids → "s1".."sN" by position (unique);
                                         // empty scene State → "pending"; first scene TransitionIn.Kind forced "cut";
                                         // non-blend transitions drop Type/Seconds; empty keyframe source → "none";
                                         // empty ClipAudio → "duck"; nil slices → empty slices
```
Validation never mutates; Normalize never validates.

## 4. Validator (binding API; rules = spec §8)

```go
type Env struct {
	Profile                Profile
	ImageGeneration        bool            // image generation configured
	TTS                    bool            // tools.TTSReady(cfg, "video_studio")
	Music                  bool            // music generation configured
	UnavailableTransitions []string        // registry names this server's FFmpeg lacks
	KnownTransitions       []string        // registry transition names (videostudio.EffectRegistry() transitions)
	ImageAssetIDs          map[string]bool // image assets of the project (for keyframe.source=library, character refs)
}
type Issue struct {
	Code     string `json:"code"`
	SceneID  string `json:"scene_id,omitempty"`
	Field    string `json:"field,omitempty"` // e.g. "scenes[2].generate_seconds", "target_seconds"
	Message  string `json:"message"`         // short English text for the agent and logs; the UI translates by code
	Blocking bool   `json:"blocking"`
}
func Validate(p Plan, env Env) []Issue   // deterministic order: plan-level first, then scenes in order
func HasBlocking(issues []Issue) bool
func PhaseComplete(p Plan, phase string, env Env) bool // phases: "outline" (>=1 scene, every scene has title,
     // generate_seconds, use_seconds, transition kind), "details" (outline + every scene has video_prompt, keyframe
     // image_prompt when source generated, narration text when env.TTS and the plan has any narration intent is NOT
     // required — narration stays optional), "check" (no blocking issue)
```
Issue codes (constants `Code...` in validate.go), all blocking unless marked (w):
`no_scenes`, `too_many_scenes`, `target_seconds_invalid`, `total_length_mismatch` (±15 % of target),
`total_length_too_long` (>120 s), `idea_too_long`, `style_too_long`, `too_many_characters`, `character_invalid`,
`character_reference_invalid`, `title_missing`, `title_too_long`, `prompt_missing` (video_prompt empty),
`prompt_too_long` (> profile.MaxPromptChars), `negative_prompt_too_long`, `duration_unsupported` (generate_seconds ∉
profile.DurationsSeconds), `use_seconds_invalid` (<=0 or > generate_seconds), `transition_unknown`,
`transition_unsupported` (continue/bridge without first_frame, bridge without last_frame), `first_scene_transition`
(first scene not cut), `chain_too_long` (> 4 scenes joined by continue), `blend_type_unknown`,
`blend_type_unavailable`, `blend_overlap_invalid` (seconds < 0.5 or > min(use of both) − 0.5),
`keyframe_source_invalid` (unknown, chain without continue, bridge without bridge, generated without image
generation), `keyframe_prompt_missing` (generated without image_prompt), `keyframe_asset_invalid` (library without a
project image asset), `narration_unavailable` (text while !env.TTS), `narration_too_long` (chars > 600 or words over
budget), `music_unavailable` (prompt while !env.Music), `clip_audio_invalid`, `plan_unsupported_provider`
(!profile.PlanSupported), `profile_changed` (w) (plan.Profile ≠ env.Profile provider/model/resolution).
Word budget (words.go): `func WordBudget(language string, useSeconds float64) int` = floor(rate × useSeconds) − 2,
min 0; rate de 2.2, en 2.5, else 2.3 (language = first two letters, lowercased). `func CountWords(text string) int`
splits on Unicode whitespace; for zh/ja count runes of letters ÷ 2 rounded up as words (document it).
Total length = Σ use_seconds − Σ blend overlaps.

## 5. Capability profile derivation (server, binding behaviour = spec §4.1 table)

`videoStudioGenerationProfile(cfg, resolution)`: provider = lowercased `cfg.VideoGeneration.ProviderType`
("google_veo" → "google"); model = resolved model (MiniMax via the API-model mapping; Google as configured, default
`veo-3.1-generate-preview`); resolution = argument or `cfg.VideoGeneration.DefaultResolution` (MiniMax default
"768P"). Generation not enabled/configured → PlanSupported false, Reason `generation_unavailable`. Agnes → false,
`provider_images_public_url_only` (durations still filled 4..12 so manual generation keeps working). Unknown →
false, `provider_unsupported`, empty durations. MiniMax: durations as today (`[6,10]`, `[6]` at 1080P or non-Hailuo
model), FirstFrame true, LastFrame true only for API model `MiniMax-Hailuo-02` at 768P/1080P, NativeAudio false,
AspectFromFirstFrame true, AspectRatios [] , cost 0.20. Google: durations `[4,6,8]` (1080P → `[8]`), FirstFrame true,
LastFrame false, NativeAudio = model contains "veo-3", AspectRatios ["16:9","9:16"], cost 0.50. MaxPromptChars 2000
for all supported providers. Parallel 2. `videoStudioDurationOptions(cfg, resolution, hasReferences)` keeps its
exact current results (Google with references → [8]). `videoStudioImageModes(provider)` keeps `["first_frame"]` for
minimax/google, `[]` otherwise. Status JSON `generation` gains `"profile": <Profile>`.

## 6. Plan storage (server, binding)

```go
func videoStudioPlanPath(projectID string) string // videoStudioProjectPath(projectID) + "/plan.json"
var errVideoStudioPlanNotFound = errors.New("plan_not_found")
// readVideoStudioPlan returns the decoded+normalized plan, its raw bytes and version (desktop.NoteVersion(data)).
func readVideoStudioPlan(ctx context.Context, svc *desktop.Service, projectID string) (storyplan.Plan, []byte, string, error)
// writeVideoStudioPlan normalizes, sets UpdatedAt (videoStudioNow()), marshals and writes conditionally;
// expectVersion "" = must not exist, "*" = unconditional, else must match. Returns the new version.
// Caller holds manager.projectLock(projectID). Counts against the project storage quota like project.json.
func writeVideoStudioPlan(ctx context.Context, svc *desktop.Service, manager *videoStudioManager, cfg *config.Config, projectID string, plan storyplan.Plan, expectVersion string) (string, error)
// videoStudioPlanEnv builds the validator environment for a project (profile for plan.Profile.Resolution or the
// default resolution, availability, registry transitions, project image assets).
func videoStudioPlanEnv(ctx context.Context, s *Server, cfg *config.Config, manager *videoStudioManager, projectID string) storyplan.Env
// videoStudioPlanAvailability: what the panel and the agent may use.
type videoStudioPlanAvailability struct {
	Enabled         bool   `json:"enabled"`          // cfg.VideoStudio.Plan.Enabled
	Supported       bool   `json:"supported"`        // profile.PlanSupported
	Reason          string `json:"reason,omitempty"` // profile.Reason, or "plan_disabled"
	ImageGeneration bool   `json:"image_generation"`
	TTS             bool   `json:"tts"`
	Music           bool   `json:"music"`
}
func videoStudioPlanAvailabilityFor(cfg *config.Config, profile storyplan.Profile) videoStudioPlanAvailability
```
The status JSON gains top-level `"plan": videoStudioPlanAvailability`.

## 7. HTTP API (server, binding)

Routing: add to the `projects/{id}/...` switch in `handleVideoStudio`:
`len(parts)==3 && parts[2]=="plan"` → `handleVideoStudioPlan(s,w,r,id)` (GET, PUT);
`len(parts)==4 && parts[2]=="plan" && parts[3]=="agent"` → `handleVideoStudioPlanAgent` (POST start);
`len(parts)==5 && parts[2]=="plan" && parts[3]=="agent" && parts[4]=="stop"` → `handleVideoStudioPlanAgentStop` (POST).

- `GET .../plan` (read access): 404 `plan_not_found` when absent. 200 body:
  `{"plan": Plan, "etag": "...", "issues": [Issue], "profile": Profile, "availability": videoStudioPlanAvailability,
  "agent": videoStudioPlanAgentStatus | null}`; header `ETag`; honours `If-None-Match` → 304. If `plan.status ==
  "planning"` and no live job, the response reports `status: "review"` (interrupted job; nothing written).
- `PUT .../plan` (mutation access, desktopScopeWrite, `If-Match` required → 428 `precondition_required`): body = Plan
  JSON (≤ MaxPlanBytes, Decode rules). 409 `plan_agent_running` while a job runs for the project. Server keeps
  server-owned fields from the stored plan: `id`, `version`, `profile`, `status`, `cost`, every scene's `state`,
  `clip_asset_id`, `still_asset_id`, `timeline_clip_id`, `cost`, `error`, `narration.asset_id`,
  `narration.measured_seconds`, `keyframe.asset_id` only when source != library, music `asset_id` (matched by scene id;
  new scenes get empty server fields). Saves even with issues (they are returned). 412 on ETag mismatch via the
  existing desktop precondition errors. Response like GET.
- `POST .../plan/agent` (mutation access, desktopScopeWrite; read-only desktop/VS refused by the access helper):
  body `{"mode":"create"|"revise","idea":"...","target_seconds":30,"language":"de","scene_id":"s2","note":"..."}`.
  create: idea 1..4000 chars, target 5..120; creates (or replaces after confirmation flag `"replace":true` when a plan
  exists — else 409 `plan_exists`) plan.json via NewPlan with the current profile ref, then starts the job.
  revise: plan must exist, scene_id must exist, note 1..1000 chars. Errors: 409 `plan_unsupported_provider` (with
  `reason`), 409 `plan_disabled`, 409 `plan_agent_running`, 429 `plan_daily_limit`, 503 `plan_agent_unavailable` (no
  LLM). 202 `{"agent": videoStudioPlanAgentStatus}`.
- `POST .../plan/agent/stop`: 200 `{"stopped": true|false}`.
- All errors through `writeVideoStudioError(w, status, code, message)`.

## 8. Agent manager and job (server, binding seams between part B and part C)

```go
type videoStudioPlanAgentRequest struct {
	ProjectID     string
	Mode          string // "create" | "revise"
	Idea          string
	TargetSeconds int
	Language      string
	SceneID       string
	Note          string
}
type videoStudioPlanAgentStatus struct {
	State      string    `json:"state"`           // running | done | failed | stopped
	Mode       string    `json:"mode"`
	Phase      string    `json:"phase,omitempty"` // outline | details | check | revise
	Error      string    `json:"error,omitempty"` // code: model_error | timeout | tool_limit | stopped | plan_conflict
	Tokens     int       `json:"tokens"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}
var (
	errVideoStudioPlanAgentBusy        = errors.New("plan_agent_running")
	errVideoStudioPlanAgentDailyLimit  = errors.New("plan_daily_limit")
	errVideoStudioPlanAgentUnavailable = errors.New("plan_agent_unavailable")
)
type videoStudioPlanAgents struct { /* mu, jobs map[projectID]*videoStudioPlanAgentJob, last status per project, daily counter */ }
func (s *Server) videoStudioPlanAgents() *videoStudioPlanAgents               // lazily created, one per Server
func (m *videoStudioPlanAgents) Start(req videoStudioPlanAgentRequest) (videoStudioPlanAgentStatus, error)
func (m *videoStudioPlanAgents) Stop(projectID string) bool
func (m *videoStudioPlanAgents) Status(projectID string) (videoStudioPlanAgentStatus, bool) // live or last finished (in memory)
func (m *videoStudioPlanAgents) Running(projectID string) bool
```
- Part B owns the routes and calls only these four methods; Part C owns the type and the job.
- Daily counter: local day, `cfg.VideoStudio.Plan.MaxDailyJobs`, create and revise both count, a refused start does
  not count. One running job per project.
- Job context: background context with `cfg.VideoStudio.Plan.JobTimeoutSeconds` deadline; Stop cancels it.
- Route: like `storymakerAgentResolveRoute` (`internal/server/storymaker_agent.go:44`), with
  `cfg.VideoStudio.Plan.AgentProvider/AgentModel`; compact/text decided once per job.
- Loops per phase like `storymakerAgentJob.runLoop` (`agent.ExecuteAgentLoop` with a prepared profile, isolated
  RunConfig from `buildDesktopRunConfigForSession`, ExecutionHooks.HandleTool for actions with prefix `vs_plan_`,
  a broker that withholds prose (copy the storymakerBroker idea; only token usage + model status are kept, on the
  job status)). create: outline → details → check (max 3 check loops while blocking issues remain and the loop
  made changes). revise: one `revise` loop. Tool limits: outline 6, details 16, check 20, revise 10; temperatures
  outline 0.6, details 0.4, check 0.2, revise 0.3; RunComplete = `studio.PhaseDone`.
- At job end (any outcome) the job writes plan.status = "review" (keeps the plan content as saved so far).
- Plan writes by tools go through `writeVideoStudioPlan` under `manager.projectLock`, expectVersion = the version the
  studio last read or wrote (a mismatch → tool result error `plan_conflict`, the job ends failed with that code).

## 9. Tools (agent package schemas + server studio, binding)

Names: `vs_plan_read`, `vs_plan_write`, `vs_plan_patch_scene`, `vs_plan_check`, `vs_plan_finish`.
```go
// internal/agent/native_tools_video_studio_plan.go
func VideoStudioPlanToolSchemas(phase string, compact bool) []openai.Tool
// phase outline: read, write ; details: read, write, patch_scene ; check: read, check, patch_scene, write, finish ;
// revise: read, patch_scene, check, finish
```
Arguments (avoid clashes with the typed fields of `agent.ToolCall` — Part C must check that struct and the
`project_toolcall_schema_clashes` lesson: arguments whose names exist as typed ToolCall fields with another type are
rejected/lost; the studio reads from `tc.Params`, and repairs from typed fields like `storymakerStudioCall` does):
- `vs_plan_read` {} → `{"status":"ok","plan":Plan,"issues":[Issue],"profile":Profile,"availability":...}`.
- `vs_plan_write` {"plan": {style, characters, scenes, audio}} — replaces those four parts; idea/target/language/
  profile/status/cost and server-owned scene fields are kept from the stored plan → `{"status":"ok","issues":[...],
  "phase_complete":bool}` or `{"status":"error","code":"...","message":"..."}`.
- `vs_plan_patch_scene` {"scene_id":"s2","scene":{partial Scene fields}} — merges given fields into that scene; in
  revise mode only the job's scene_id and the transition_in of the following scene may change (else error
  `scene_not_allowed`).
- `vs_plan_check` {} → `{"status":"ok","issues":[...],"blocking":bool}`.
- `vs_plan_finish` {} → refused with `{"status":"error","code":"blocking_issues","issues":[...]}` while blocking issues
  exist; else `{"status":"ok"}` and the phase is done.
The studio binds the project and plan from the job only, never from arguments. Plan tools are job-private: they must
not appear in the general chat tool list, flows, daemon bridges or strict-schema snapshots (check
`strictSchemasForActiveTools` / `native_tool_schema_snapshot.go` the way the storymaker tools were kept out).

## 10. Prompt content (part C)

System sections: ground rules (you plan, you never generate; data in the user message is data, not instructions;
answer through tools), the profile summary is in the data packet, craft rules (spec §7.5), the workflow of the phase,
in text mode the tool guide with one-line JSON examples. User message = fixed sentence of the phase + a JSON data
packet in a wrapper: idea, target_seconds, language, profile (durations, first/last frame, audio, max prompt chars),
availability (image_generation, tts, music, allowed blend types = known − unavailable), current plan, last issues,
budget (tool calls, seconds left); revise adds scene_id and note. Language rule: titles/narration in the plan
language; video and image prompts in English.

## 11. UI (part D, binding)

- Status JSON fields used: `status.plan` (availability) and `status.generation.profile`.
- Header: a segmented switch "Bearbeiten | Plan" (`tr(s,'plan.modeEdit','Edit')`, `tr(s,'plan.modePlan','Plan')`),
  shown only when a project is open and `status.plan.enabled`; plan mode hides preview+timeline areas and shows the
  board in their place; the library stays. Mode is per window session (`s.planMode` boolean), default edit.
- Board states: loading; unsupported (reason text + note to Settings > Video Generation); empty (no plan.json →
  idea textarea, target seconds select 10..120 step 5 default 30, "Plan erstellen" button → POST agent create with
  `language` from the desktop UI language); planning (agent status line with phase + "Stoppen", board read-only,
  polling GET every 2 s with If-None-Match); review (editable cards).
- Card fields: title (input), video prompt (textarea), negative prompt (collapsed), generate seconds (select from
  profile.durations_seconds), use seconds (number, step 0.5, ≤ generate), keyframe source (select:
  generated/library/none, plus chain/bridge shown read-only when the transition implies it), image prompt (textarea
  when generated), library image (select of project image assets), narration (textarea, shown only when
  availability.tts), transition chip before every card except the first (kind select limited by profile; blend → type
  select from allowed blend types + seconds). Plan-level: style (textarea), characters (list name+description, add/
  remove, max 6), music prompt (only when availability.music), clip audio select.
- Saving: debounce 800 ms after an edit → PUT with If-Match; 412 → reload and notice; issues shown inline next to the
  field (by `field`/`scene_id`) and as a count in the bottom bar; issue text translated by code
  (`tr(s,'plan.issue.'+code, message)`).
- Card actions in slice 1: "Szene überarbeiten" (dialog with note → POST agent revise), "Nur Text" (keyframe.source =
  none), remove scene, move up/down, add scene after (new empty scene). "Keyframe neu", "Neu erzeugen", preview and
  approval buttons are NOT in slice 1.
- Bottom bar: total length (Σ use − blend overlaps) vs target, scene count, blocking issue count, primary action:
  "Plan erstellen" (empty) / "Neu planen" (review: replaces after a confirm → POST create with replace:true).
- Read-only Video Studio: board visible, no editing, no agent buttons.
- Phone width (≤ 600 px): cards stack, no horizontal overflow.

## 12. Test commands (use exactly; add only `-run` filters)

```
go test ./internal/videostudio/storyplan/... -count=1
go test ./internal/server -run '(?i)(VideoStudioGenerationProfile|VideoStudioPlan)' -count=1
go test ./internal/agent -run '(?i)VideoStudioPlan' -count=1
go test ./internal/config -run '(?i)VideoStudio' -count=1
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '(?i)DesktopVideoStudioPlan' -count=1 -timeout 15m
npm run check:ui
```
Existing suites that must stay green and are run at batch ends (not per task):
`go test ./internal/server -run '(?i)VideoStudio' -count=1`, `go test ./internal/videostudio -count=1`,
`AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '(?i)DesktopVideoStudio' -count=1 -timeout 30m`.


## 13. Reconcile decisions (after drafting; these win over §1–§12)

1. **Plan settings never cancel renders.** `videoStudioConfigRootsChanged` compares `VideoStudio` with `Plan` zeroed
   on both sides (A7, test `TestVideoStudioPlanSettingsKeepVideoStudioJobs`).
2. **The transition decides chain and bridge.** `storyplan.Normalize` sets `keyframe.source` to `chain` for a
   continue scene and `bridge` for a bridge scene, and turns chain/bridge on any other scene into `none`. The agent
   and the UI never set chain/bridge themselves; the UI shows them read-only. A bridge's boundary keyframe K needs a
   project image (`keyframe.asset_id`) or an `image_prompt` with image generation (`keyframe_prompt_missing` /
   `keyframe_asset_invalid`). Bridge is offered (UI and agent) only with first frame, last frame and image generation.
3. **Ownership.** C4 declares the agent manager types, errors and `s.videoStudioPlanAgents()`; B3/B4 run after C4 and
   only call `Start/Stop/Status/Running`. Mode, phase and tool-name constants live in package `agent`
   (`agent.VideoStudioPlanMode*`, `agent.VideoStudioPlanPhase*`, `agent.VideoStudioPlanTool*`). Test helpers with
   prefix `vsp*` belong to C; B4's live-job test uses them.
4. **Env.** Callers that hold a plan use `videoStudioPlanEnvFor(ctx, cfg, manager, projectID, resolution)` (B1); the
   studio passes `plan.Profile.Resolution`.
5. **Start is synchronous up to the route.** `Start` checks busy → daily limit → model route → desktop run before it
   returns; a refused create makes B undo its plan write. Lock order: project lock → agents mutex.
6. **GET.** Reports `status: "planning"` while a job runs (also during revise); a stored `planning` without a live job
   is reported as `review`. 304 only when no agent status (live or last finished) is known and the stored status is
   not `planning`. 409 `plan_invalid` means a damaged plan.json; the UI shows the empty state with `plan.damaged` and
   its create sends `replace: true`.
7. **Stop** uses `requireDesktopOperation(..., desktopScopeWrite, desktopStop)` so a running job can be stopped on a
   read-only desktop.
8. **Extra files outside §1** (needed to keep the plan tools job-private and correct): `internal/agent/action_policy.go`,
   `internal/agent/native_tool_schema_snapshot.go`, `internal/security/tool_output_trust.go`, `internal/server/server.go`
   (C); `internal/server/video_studio_service.go` (A7); config page section with 12 `config.video_studio.plan_*` keys in
   16 languages, `Config.ProviderReferences()`, decisions-only provider guard (A7/A8); `ui/desktop_video_studio_plan_strings_test.go`
   and the script tags in `internal/server/video_studio_browser_test.go` (D).
