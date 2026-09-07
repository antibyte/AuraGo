package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const gamePlanPath = ".aurago/game-plan.json"

// GamePlan is a bounded design artifact, never executable or trusted instructions.
type GamePlan struct {
	SchemaVersion int               `json:"schema_version"`
	Template      string            `json:"template"`
	Objective     string            `json:"objective"`
	CoreLoop      string            `json:"core_loop"`
	Scope         []string          `json:"scope"`
	Perspective   string            `json:"perspective"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	Camera        string            `json:"camera"`
	Controls      map[string]string `json:"controls"`
	States        []string          `json:"states"`
	Rules         map[string]string `json:"rules"`
	Assets        []PlanAsset       `json:"assets"`
	Scenarios     []GameScenario    `json:"scenarios"`
	Assumptions   []string          `json:"assumptions"`
	Fallback      string            `json:"fallback"`
	Preserve      []string          `json:"preserve,omitempty"`
}

type PlanAsset struct {
	Role          string   `json:"role"`
	PackID        string   `json:"pack_id,omitempty"`
	Version       string   `json:"version,omitempty"`
	AssetID       string   `json:"asset_id,omitempty"`
	AssemblyID    string   `json:"assembly_id,omitempty"`
	Animations    []string `json:"animations,omitempty"`
	Direction     string   `json:"direction"`
	DisplayHeight float64  `json:"display_height"`
	Origin        Point    `json:"origin"`
	Collider      string   `json:"collider"`
	Fallback      string   `json:"fallback,omitempty"`
}

// Scenarios use the same finite commands and comparisons as the built-in tests.
type GameScenario struct {
	ID      string         `json:"id"`
	Steps   []GameTestStep `json:"steps"`
	Metric  string         `json:"metric"`
	Compare string         `json:"compare"`
	Value   float64        `json:"value"`
}

type GameTestStep struct {
	Action string  `json:"action"`
	Key    string  `json:"key,omitempty"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	MS     int     `json:"ms,omitempty"`
}

func templateNames() []string {
	return []string{"shooter", "platformer", "topdown", "blocks", "board", "minimal", "three"}
}

// PlanningComplete ends the internal agent round; the orchestrator alone decides
// whether the accepted plan advances to building or exhausted corrections fail.
func (s *Service) PlanningComplete(jobID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeJobID == jobID && (s.acceptedPlans[jobID] || s.planAttempts[jobID] >= 3)
}

func (s *Service) GetPlan(ctx context.Context, jobID string) (*GamePlan, error) {
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return nil, err
	}
	path, _, err := secureJoin(stage, gamePlanPath, true)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read game plan: %w", err)
	}
	if len(data) > 32768 {
		return nil, fmt.Errorf("game plan exceeds 32768 bytes")
	}
	var plan GamePlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("decode game plan: %w", err)
	}
	return &plan, nil
}

func (s *Service) SetPlan(ctx context.Context, jobID string, plan GamePlan) (err error) {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	if !s.policy.Enabled {
		return ErrDisabled
	}
	if s.policy.ReadOnly || !s.policy.AllowEdit {
		return ErrReadOnly
	}
	project, job, err := s.ProjectForJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Phase != "planning" {
		return fmt.Errorf("plan is locked during %s; follow the accepted plan", job.Phase)
	}
	s.mu.Lock()
	if s.planAttempts[jobID] >= 3 {
		s.mu.Unlock()
		return fmt.Errorf("plan correction limit reached (initial plan plus two corrections)")
	}
	s.planAttempts[jobID]++
	s.mu.Unlock()
	// Preserve the actual field error across fresh planning rounds. A subsequent
	// correction-limit rejection must not replace it with a generic message.
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err == nil {
			delete(s.planErrors, jobID)
		} else {
			s.planErrors[jobID] = err
		}
	}()
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("encode game plan: %w", err)
	}
	if len(data) > 32768 || int64(len(data)) > s.opts.MaxFileBytes {
		return fmt.Errorf("game plan exceeds allowed size")
	}
	if err := s.checkPlan(project, plan); err != nil {
		return err
	}
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return err
	}
	path, _, err := secureJoin(stage, gamePlanPath, true)
	if err != nil {
		return err
	}
	oldBytes := int64(0)
	extraFiles := 1
	if info, err := os.Stat(path); err == nil {
		oldBytes = info.Size()
		extraFiles = 0
	}
	if err := validateTreeLimits(stage, s.opts.MaxFilesPerProject-extraFiles, s.opts.MaxProjectBytes-int64(len(data)+1)+oldBytes); err != nil {
		return fmt.Errorf("plan exceeds project limits: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create plan directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".plan-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	_, writeErr := tmp.Write(append(data, '\n'))
	if writeErr == nil {
		writeErr = tmp.Sync()
	}
	closeErr := tmp.Close()
	if writeErr != nil {
		return fmt.Errorf("write plan: %w", writeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("publish plan: %w", err)
	}
	s.mu.Lock()
	s.acceptedPlans[jobID] = true
	s.mu.Unlock()
	return nil
}

func (s *Service) checkPlan(project Project, p GamePlan) error {
	bad := func(field, message string) error { return fmt.Errorf("plan.%s: %s", field, message) }
	if p.SchemaVersion != 1 {
		return bad("schema_version", "must be 1")
	}
	if !slices.Contains(templateNames(), p.Template) {
		return bad("template", "choose shooter, platformer, topdown, blocks, board, minimal or three")
	}
	if (project.Dimension == "3d") != (p.Template == "three") {
		return bad("template", "must match the project dimension")
	}
	for key, value := range map[string]string{"objective": p.Objective, "core_loop": p.CoreLoop, "camera": p.Camera, "fallback": p.Fallback} {
		if strings.TrimSpace(value) == "" || len(value) > 2000 {
			return bad(key, "provide 1–2000 characters")
		}
	}
	if !slices.Contains([]string{"side", "top", "board", "3d"}, p.Perspective) {
		return bad("perspective", "choose side, top, board or 3d")
	}
	if project.Dimension == "3d" && p.Perspective != "3d" || project.Dimension == "2d" && p.Perspective == "3d" {
		return bad("perspective", "must match the project dimension")
	}
	if p.Width < 320 || p.Width > 1920 || p.Height < 240 || p.Height > 1080 {
		return bad("resolution", "width 320–1920 and height 240–1080 required; default 960×540")
	}
	if len(p.Scope) == 0 || len(p.Scope) > 12 {
		return bad("scope", "list 1–12 concrete features")
	}
	for _, list := range [][]string{p.Scope, p.States, p.Assumptions, p.Preserve} {
		for _, value := range list {
			if strings.TrimSpace(value) == "" {
				return bad("content", "list entries must not be empty")
			}
		}
	}
	if len(p.States) < 2 || len(p.States) > 8 || !slices.Contains(p.States, "playing") {
		return bad("states", "include playing and a terminal or continued-play state")
	}
	if len(p.Controls) > 12 {
		return bad("controls", "at most 12 input actions")
	}
	for _, key := range []string{"primary", "restart"} {
		if strings.TrimSpace(p.Controls[key]) == "" {
			return bad("controls."+key, "describe the input")
		}
	}
	for _, key := range []string{"progress", "failure", "completion"} {
		if strings.TrimSpace(p.Rules[key]) == "" {
			return bad("rules."+key, "describe the rule or explain why it does not apply")
		}
	}
	if project.CurrentRevision > 0 && len(p.Preserve) == 0 {
		return bad("preserve", "list working behavior retained by this edit")
	}
	if len(p.Assets) < 1 || len(p.Assets) > 24 || len(p.Assumptions) > 12 {
		return bad("assets", "map 1–24 visual roles, including procedural graphics; limit assumptions to 12")
	}
	roles := map[string]bool{}
	for i, a := range p.Assets {
		field := fmt.Sprintf("assets[%d]", i)
		if strings.TrimSpace(a.Role) == "" || roles[a.Role] {
			return bad(field+".role", "use a unique role")
		}
		roles[a.Role] = true
		if !finite(a.DisplayHeight) || a.DisplayHeight <= 0 || a.DisplayHeight > 2048 || !validOrigin(a.Origin) {
			return bad(field, "valid display_height and normalized origin required")
		}
		if !slices.Contains([]string{"none", "rectangle", "circle", "feet"}, a.Collider) {
			return bad(field+".collider", "choose none, rectangle, circle or feet")
		}
		if !slices.Contains([]string{"up", "right", "down", "left", "none"}, a.Direction) {
			return bad(field+".direction", "use up, right, down, left or none")
		}
		if a.PackID == "" {
			if strings.TrimSpace(a.Fallback) == "" {
				return bad(field+".fallback", "describe procedural/custom graphics")
			}
			if a.AssetID != "" || a.AssemblyID != "" || len(a.Animations) > 0 || a.Version != "" {
				return bad(field+".pack_id", "library references require a known pack; omit library IDs for procedural graphics")
			}
			continue
		}
		detail, err := s.describeAsset(a.PackID, a.AssetID, a.AssemblyID)
		if err != nil {
			return bad(field, err.Error())
		}
		if a.Version != detail.Version {
			return bad(field+".version", "use version "+detail.Version)
		}
		if detail.Asset != nil && detail.Asset.AssemblyPart {
			return bad(field, "select the complete assembly, not an isolated fragment")
		}
		view := detail.View
		if project.Dimension == "2d" && (view == "side" && p.Perspective != "side" || view == "top" && p.Perspective == "side") {
			return bad(field+".view", "asset perspective does not match the scene")
		}
		if !detail.allowsDirection(a.Direction) {
			return bad(field+".direction", "direction is not supported by this asset; choose a directional asset or an allowed transform")
		}
		for _, id := range a.Animations {
			if !slices.ContainsFunc(detail.Animations, func(anim PackAnimation) bool { return anim.ID == id }) {
				return bad(field+".animations", "unknown or unrelated animation "+id)
			}
		}
	}
	if len(p.Scenarios) == 0 || len(p.Scenarios) > 8 {
		return bad("scenarios", "provide 1–8 observable checks in addition to template checks")
	}
	seen := map[string]bool{}
	duration := 0
	for i, scenario := range p.Scenarios {
		if seen[scenario.ID] {
			return bad("scenarios", "duplicate check ID")
		}
		seen[scenario.ID] = true
		if err := validateScenario(scenario); err != nil {
			return bad(fmt.Sprintf("scenarios[%d]", i), err.Error())
		}
		for _, step := range scenario.Steps {
			duration += step.MS
		}
	}
	if duration > 25000 {
		return bad("scenarios", "additional scenarios must total at most 25 seconds")
	}
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func validOrigin(p Point) bool {
	return finite(p.X) && finite(p.Y) && p.X >= 0 && p.X <= 1 && p.Y >= 0 && p.Y <= 1
}

// CheckJobMutation is shared by code writes, imports and media dispatch. Internal
// preselected imports use importAssetPack directly before the planner starts.
func (s *Service) CheckJobMutation(ctx context.Context, jobID string) error {
	if _, err := s.JobDirectory(jobID); err != nil {
		return err
	}
	_, job, err := s.ProjectForJob(ctx, jobID)
	if err != nil {
		return err
	}
	s.policyMu.RLock()
	policy := s.policy
	s.policyMu.RUnlock()
	if !policy.Enabled {
		return ErrDisabled
	}
	if policy.ReadOnly || !policy.AllowEdit {
		return ErrReadOnly
	}
	s.mu.RLock()
	accepted := s.acceptedPlans[jobID]
	exhausted := s.validationFailures[jobID] >= 4
	s.mu.RUnlock()
	if exhausted {
		return ErrRepairLimit
	}
	if job.Phase == "planning" || !accepted {
		return fmt.Errorf("planning_required: submit a valid plan with game_maker_project set_plan before writing code or generating/importing assets")
	}
	return nil
}

func (s *Service) HoldAgentSummary(jobID, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len([]rune(text)) > 6000 {
		text = string([]rune(text)[:6000])
	}
	s.jobSummaries[jobID] = text
}

func JobNextAction(job Job) string {
	if job.Phase == "planning" {
		return "Read the current project and get_plan, search/describe relevant assets, then set_plan with a complete design. Code writes, imports and media generation are locked."
	}
	return "Read the accepted plan and installed template. Implement its core loop, call game_maker_validate with scope full, then finish remaining features and validate again. Never claim an unobserved check."
}

// ExampleGamePlan is a schema example, not a replacement for the user's design.
func ExampleGamePlan(project Project) GamePlan {
	p := GamePlan{SchemaVersion: 1, Template: "minimal", Objective: "Replace with the requested objective", CoreLoop: "Replace with the input, consequence, feedback and progression loop", Scope: []string{"Replace with concrete requested features"}, Perspective: "top", Width: 960, Height: 540, Camera: "Fixed logical viewport with FIT scaling", Controls: map[string]string{"move": "Arrow keys", "primary": "Space", "restart": "R"}, States: []string{"playing", "ended"}, Rules: map[string]string{"progress": "Describe score or progression", "failure": "Describe defeat or explain why absent", "completion": "Describe victory or continued play"}, Assets: []PlanAsset{}, Scenarios: []GameScenario{{ID: "primary_action", Steps: []GameTestStep{{Action: "key", Key: "SPACE", MS: 200}}, Metric: "actions", Compare: "increased"}}, Assumptions: []string{"Single-player offline game"}, Fallback: "Use named procedural shapes when matching art is unavailable"}
	if project.Dimension == "3d" {
		p.Template = "three"
		p.Perspective = "3d"
	} else {
		p.Scenarios = append(p.Scenarios, GameScenario{ID: "player_movement", Steps: []GameTestStep{{Action: "key", Key: "RIGHT", MS: 350}}, Metric: "player_x", Compare: "changed"})
	}
	p.Assets = []PlanAsset{{Role: "player", Direction: "none", DisplayHeight: 32, Origin: Point{.5, .5}, Collider: "rectangle", Fallback: "Replace with a specific procedural shape or choose exact library IDs"}}
	if project.CurrentRevision > 0 {
		p.Preserve = []string{"Replace with working behavior retained by this edit"}
	}
	return p
}
