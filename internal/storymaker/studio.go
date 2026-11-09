package storymaker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Tools of an agent job.
const (
	ToolStory   = "storymaker_story"
	ToolCatalog = "storymaker_catalog"
	ToolCheck   = "storymaker_check"
)

// Phases of an agent job.
const (
	PhaseOutline = "outline"
	PhaseCast    = "cast"
	PhaseFill    = "fill"
	PhaseCheck   = "check"
	PhaseRevise  = "revise"
)

// Outcomes of an agent job.
const (
	OutcomeReady           = "ready"
	OutcomeReadyUnverified = "ready_unverified"
	OutcomeIncomplete      = "incomplete"
	OutcomeFailed          = "failed"
)

// Kinds of an agent job.
const (
	AgentKindCreate = "create"
	AgentKindRevise = "revise"
)

// Budgets of an agent job.
const (
	StudioSceneChargedLimit = 3
	StudioSceneWriteLimit   = 8
	StudioDryRunsPerScene   = 2
	StudioDryRunsPerJob     = 20
	StudioCheckRounds       = 6
)

// studioDefaultTargetSeconds is the film length when a job names none.
const studioDefaultTargetSeconds = 60

// studioPhaseTools lists the tools of each phase.
var studioPhaseTools = map[string][]string{
	PhaseOutline: {ToolStory},
	PhaseCast:    {ToolCatalog, ToolStory},
	PhaseFill:    {ToolCatalog, ToolStory},
	PhaseCheck:   {ToolCheck, ToolStory, ToolCatalog},
	PhaseRevise:  {ToolCatalog, ToolStory, ToolCheck},
}

// studioPhaseOperations lists the operations of each tool in each phase.
var studioPhaseOperations = map[string]map[string][]string{
	PhaseOutline: {
		ToolStory: {"outline", "get"},
	},
	PhaseCast: {
		ToolCatalog: {"search", "describe", "reference"},
		ToolStory:   {"set_meta", "get"},
	},
	PhaseFill: {
		ToolCatalog: {"search", "describe", "reference"},
		ToolStory:   {"put_scene", "patch_scene", "dry_run", "get", "get_code", "put_code"},
	},
	PhaseCheck: {
		ToolCheck:   {"validate"},
		ToolStory:   {"patch_scene", "get", "get_code", "put_code"},
		ToolCatalog: {"reference"},
	},
	PhaseRevise: {
		ToolCatalog: {"search", "describe", "reference"},
		ToolStory:   {"get", "put_scene", "patch_scene", "remove_scene", "move_scene", "set_meta", "dry_run", "get_code", "put_code"},
		ToolCheck:   {"validate"},
	},
}

// studioWriteOps change the story, so a read-only desktop refuses them.
var studioWriteOps = nameSet("outline", "set_meta", "put_scene", "patch_scene", "remove_scene", "move_scene", "put_code")

// PhaseTools returns the tool names of a phase, or nil for an unknown phase.
func PhaseTools(phase string) []string {
	return append([]string(nil), studioPhaseTools[phase]...)
}

// PhaseOperations returns the operations of a tool in a phase, or nil when the phase does not offer the tool.
func PhaseOperations(phase, tool string) []string {
	return append([]string(nil), studioPhaseOperations[phase][tool]...)
}

// OperationOwners maps every operation to its tool. The operation sets of the tools are disjoint.
func OperationOwners() map[string]string {
	owners := map[string]string{}
	for _, tools := range studioPhaseOperations {
		for tool, operations := range tools {
			for _, operation := range operations {
				owners[operation] = tool
			}
		}
	}
	return owners
}

// StudioDeps are the services of a job's studio. Catalog, Voices, Compile and ReadOnly may be nil.
// MaxDurationSeconds is the longest film the export accepts; Check reports a longer film as
// over_export_limit. Zero means no limit.
type StudioDeps struct {
	Projects           *ProjectStore
	Catalog            *Catalog
	Limits             Limits
	Voices             func(ctx context.Context) []VoiceProvider
	Compile            func(ctx context.Context, projectID string) (CompileSummary, error)
	ReadOnly           func() bool
	MaxDurationSeconds int
}

// StudioJob is what the server binds to a job. The model never names any of it.
type StudioJob struct {
	JobID, ProjectID, Kind string // Kind is AgentKindCreate or AgentKindRevise
	Language, Audience     string
	TargetSeconds          int
	VoiceAvailable         bool
}

// StudioCall is one tool call of the model.
type StudioCall struct {
	Tool, Operation string
	Args            map[string]any
}

// StudioResult is the answer to a call. JSON goes to the model, the other fields go to the event log.
type StudioResult struct {
	JSON                       []byte
	Status, Operation, SceneID string // for events: closed values only
}

// CompileSummary is the result of a compile check in the browser.
type CompileSummary struct {
	Available  bool
	Reason     string
	DurationMS int64
	Scenes     []CompileScene
	Issues     []Issue
	// Layout is the measured layout of the storyboard scenes (empty when the player did not measure). StoryboardScenes and
	// CodeScenes count the scenes of each kind; LayoutError is the player's reason when the measurement failed.
	Layout           []LayoutScene
	StoryboardScenes int
	CodeScenes       int
	LayoutError      string
	// Stills are the hashes of the captured pictures, one per measured scene. Sheet is their contact sheet as a PNG, until the
	// job saves it; the Studio does not keep it.
	Stills []StillInfo
	Sheet  []byte
}

// CompileScene is the measured timing of one scene.
type CompileScene struct {
	ID      string
	StartMS int64
	EndMS   int64
	CuesMS  []CompileCue
}

// CompileCue is the measured time of one narration marker.
type CompileCue struct {
	Marker int
	AtMS   int64
}

// Studio is the session of one agent job. Identity is bound at construction, so a tool call can
// only reach this job's project. The story file is the checkpoint: session state is lost with the
// job, and a later revise job finishes what is left.
type Studio struct {
	deps StudioDeps
	job  StudioJob

	// mu serialises Call and PhaseDone. Operation handlers run with mu held, so they must not call
	// a locking method of Studio.
	mu      sync.Mutex
	phase   string
	sceneID string

	outlineSaved    bool
	outlineRefusals int
	template        string
	castSaved       bool
	pendingRepairs  []UmlautFix // umlaut repairs of the call in progress; takeRepairs hands them out
	repairs         int         // umlaut repairs of the job so far, for the job report

	// Scene budget, keyed by scene id.
	drafts         map[string]map[string]any
	charged        map[string]int
	writes         map[string]int
	lastIssueCount map[string]int
	dryRunsScene   map[string]int
	dryRunsJob     int

	// Check state.
	checkRounds    int
	checkClean     bool
	compileETag    string
	compileSummary CompileSummary
	codeChanges    map[string]studioCodeChange
}

// NewStudio returns the session of one job.
func NewStudio(deps StudioDeps, job StudioJob) (*Studio, error) {
	if deps.Projects == nil {
		return nil, fmt.Errorf("%w: a studio needs a project store", ErrInvalidArgument)
	}
	if !ValidProjectID(job.ProjectID) {
		return nil, fmt.Errorf("%w: project %q", ErrInvalidID, clip(job.ProjectID, 40))
	}
	if job.Kind != AgentKindCreate && job.Kind != AgentKindRevise {
		return nil, fmt.Errorf("%w: job kind %q", ErrInvalidArgument, clip(job.Kind, 40))
	}
	if deps.Limits == (Limits{}) {
		deps.Limits = DefaultLimits()
	}
	if job.TargetSeconds <= 0 {
		job.TargetSeconds = studioDefaultTargetSeconds
	}
	return &Studio{
		deps:           deps,
		job:            job,
		drafts:         map[string]map[string]any{},
		charged:        map[string]int{},
		writes:         map[string]int{},
		lastIssueCount: map[string]int{},
		dryRunsScene:   map[string]int{},
	}, nil
}

// SetPhase names the phase of the next loop and, for fill, its one scene.
func (s *Studio) SetPhase(phase, sceneID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = phase
	s.sceneID = sceneID
}

// studioOp handles one operation. It runs with s.mu held, after the phase and read-only gates.
// Args of storymaker_story are already repaired.
type studioOp func(ctx context.Context, s *Studio, args map[string]any) StudioResult

// studioOps is the routing table of Call, keyed by "<tool>/<operation>". Files fill it from init.
var studioOps = map[string]studioOp{}

// registerStudioOp adds a handler to the routing table. It is only called from init functions.
func registerStudioOp(tool, operation string, op studioOp) {
	studioOps[tool+"/"+operation] = op
}

// Call runs one tool call. Every failure comes back as an answer the model can act on.
func (s *Studio) Call(ctx context.Context, call StudioCall) StudioResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	tool := strings.TrimSpace(call.Tool)
	operation := strings.ToLower(strings.TrimSpace(call.Operation))
	if operation == "" {
		operation = strings.ToLower(studioArgString(call.Args, "operation"))
	}
	if operation == "" && tool == ToolCheck {
		operation = "validate"
	}
	res := s.route(ctx, tool, operation, call.Args)
	if _, known := OperationOwners()[operation]; known {
		res.Operation = operation
	}
	return res
}

func (s *Studio) route(ctx context.Context, tool, operation string, args map[string]any) StudioResult {
	allowed := PhaseOperations(s.phase, tool)
	if len(allowed) == 0 {
		return studioRefused("operation_not_in_phase",
			fmt.Sprintf("The tool %q is not available in the %s phase.", clip(tool, 40), s.phase),
			"Use one of these tools: "+listOf(PhaseTools(s.phase))+".")
	}
	if !containsName(allowed, operation) {
		return studioRefused("operation_not_in_phase",
			fmt.Sprintf("The operation %q of %s is not available in the %s phase.", clip(operation, 40), tool, s.phase),
			"Use one of these operations: "+listOf(allowed)+".")
	}
	if studioWriteOps[operation] && s.deps.ReadOnly != nil && s.deps.ReadOnly() {
		return studioRefused("desktop_readonly", "The desktop is read-only, so the story cannot change.",
			"Stop and answer with one short sentence.")
	}
	op := studioOps[tool+"/"+operation]
	if op == nil {
		return studioRefused("operation_not_in_phase", fmt.Sprintf("The operation %q is not available.", operation),
			"Use one of these operations: "+listOf(allowed)+".")
	}
	if tool == ToolStory {
		args = repairStoryArgs(args)
	}
	return op(ctx, s, args)
}

// PhaseDone is the server-owned end of the current loop. A revise loop ends on a text reply instead.
func (s *Studio) PhaseDone() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.phase {
	case PhaseOutline:
		return s.outlineSaved
	case PhaseCast:
		return s.castSaved
	case PhaseFill:
		st, _, err := s.loadStory(context.Background())
		if err != nil {
			return false
		}
		sc := studioFindScene(st, s.sceneID)
		return sc != nil && studioSceneFilled(sc)
	case PhaseCheck:
		return s.checkClean || s.checkRounds >= StudioCheckRounds
	}
	return false
}

// loadStory reads the story of this job with the defaults applied.
func (s *Studio) loadStory(ctx context.Context) (*Story, string, error) {
	data, etag, err := s.deps.Projects.LoadStory(ctx, s.job.ProjectID)
	if err != nil {
		return nil, "", err
	}
	st, _, err := DecodeStory(data, s.deps.Limits)
	if err != nil {
		return nil, "", err
	}
	ApplyDefaults(&st)
	return &st, etag, nil
}

// saveStory writes the story against the etag that was read and returns the new etag.
func (s *Studio) saveStory(ctx context.Context, st *Story, etag string) (string, error) {
	ApplyDefaults(st)
	data, err := CanonicalJSON(*st)
	if err != nil {
		return "", err
	}
	return s.deps.Projects.SaveStory(ctx, s.job.ProjectID, data, etag)
}

// studioFindScene returns the scene with an id, or nil.
func studioFindScene(st *Story, id string) *Scene {
	for i := range st.Scenes {
		if st.Scenes[i].ID == id {
			return &st.Scenes[i]
		}
	}
	return nil
}

// studioSceneFilled says whether a scene has more than its outline. ApplyDefaults writes an
// environment into every scene, so the environment alone does not count.
func studioSceneFilled(sc *Scene) bool {
	return sc.Kind == "code" || len(sc.Actors) > 0 || len(sc.Props) > 0 || len(sc.Beats) > 0
}

// studioRev is the revision the model sees: the first 12 hex digits of the etag.
func studioRev(etag string) string {
	rev := strings.Trim(etag, `"`)
	if len(rev) > 12 {
		rev = rev[:12]
	}
	return rev
}

// studioAnswer encodes an answer. encoding/json escapes angle brackets, so story text cannot close
// the fence around tool output.
func studioAnswer(status string, fields map[string]any) StudioResult {
	out := map[string]any{"status": status}
	for key, value := range fields {
		out[key] = value
	}
	data, err := json.Marshal(out)
	if err != nil {
		return StudioResult{JSON: []byte(`{"status":"error","message":"The answer could not be encoded."}`), Status: "error"}
	}
	return StudioResult{JSON: data, Status: status}
}

// studioOK is a successful answer.
func studioOK(fields map[string]any) StudioResult {
	return studioAnswer("ok", fields)
}

// studioRefused is a refusal with its code and the next thing to do.
func studioRefused(code, message, nextAction string) StudioResult {
	return studioRefusedWith(code, message, nextAction, nil)
}

// studioRefusedWith is a refusal with more fields, such as issues or word numbers.
func studioRefusedWith(code, message, nextAction string, extra map[string]any) StudioResult {
	fields := map[string]any{"code": code, "message": message, "next_action": nextAction}
	for key, value := range extra {
		fields[key] = value
	}
	return studioAnswer("refused", fields)
}

// studioFailed is the answer to a storage failure. It names no path.
func studioFailed() StudioResult {
	return studioAnswer("error", map[string]any{
		"message":     "The story could not be read or saved.",
		"next_action": "Stop and answer with one short sentence.",
	})
}
