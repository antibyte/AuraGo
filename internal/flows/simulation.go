package flows

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var ErrInvalidSimulationFixture = errors.New("invalid simulation fixture")

// SimulationFixture replaces one non-pure node with an explicit result.
type SimulationFixture struct {
	Output    map[string]any `json:"output"`
	Ports     []string       `json:"ports,omitempty"`
	ErrorCode string         `json:"error_code,omitempty"`
}

// SimulationRequest describes an isolated, non-persistent draft run.
type SimulationRequest struct {
	TriggerNode       string
	TriggerData       map[string]any
	OnlyNode          string
	BaseRevision      int
	Fixtures          map[string]SimulationFixture
	At                time.Time
	blockFingerprints map[string]string
}

func scenarioBlockPlanMatches(saved map[string]string, plan *blockPlan) bool {
	if saved == nil {
		return true
	}
	expected := map[string]string{}
	for key, hash := range saved {
		if strings.HasPrefix(key, "__block__:") {
			expected[key] = hash
		}
	}
	actual := map[string]string{}
	if plan != nil {
		for _, block := range plan.blocks {
			actual[fmt.Sprintf("__block__:%s:%d", block.Ref.FlowID, block.Ref.Revision)] = hex.EncodeToString(block.Hash[:])
		}
	}
	return equalScenarioFingerprints(expected, actual)
}

// SimulationResult contains bounded step records, never live action results.
type SimulationResult struct {
	Mode           RunMode              `json:"mode"`
	Revision       int                  `json:"revision"`
	StartedAt      time.Time            `json:"started_at"`
	Status         RunStatus            `json:"status"`
	ErrorCode      string               `json:"error_code,omitempty"`
	ErrorMessage   string               `json:"error_message,omitempty"`
	ErrorNodeID    string               `json:"error_node_id,omitempty"`
	Steps          []StepRecord         `json:"steps"`
	ChildTraces    []ChildTrace         `json:"child_traces,omitempty"`
	ChildDocuments []ChildTraceDocument `json:"child_documents,omitempty"`
}

type simulationClock struct{ at time.Time }

func simulationPureType(typ string) bool {
	switch typ {
	case TypeIf, TypeSwitch, TypeMerge, TypeSet, TypeStop:
		return true
	}
	return false
}

func (c simulationClock) Now() time.Time { return c.at }
func (c simulationClock) After(time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- c.Now()
	return ch
}

// Simulate executes only trusted built-in pure logic; all other built-in nodes
// require a fixture. It never uses the production registry's executable hooks.
func (s *Service) Simulate(ctx context.Context, id string, req SimulationRequest) (*SimulationResult, error) {
	rec, err := s.requireFlow(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.BaseRevision > 0 && req.BaseRevision != rec.DraftRevision {
		return nil, ErrRevisionConflict
	}
	for _, node := range rec.Draft.Nodes {
		if node.Type == durableWaitType || node.Type == durableApprovalType {
			return nil, ErrDurableInvalidGraph
		}
	}
	var plan *blockPlan
	for _, node := range rec.Draft.Nodes {
		if node.Type != blockCallType && node.Type != blockEachType {
			continue
		}
		plan, err = prepareDraftBlockPlan(ctx, s.store, rec.Draft, rec.DraftRevision, s.services)
		if err != nil {
			if req.blockFingerprints != nil && !errors.Is(err, ErrRevisionConflict) && ctx.Err() == nil {
				return nil, ErrScenarioStale
			}
			return nil, err
		}
		break
	}
	if !scenarioBlockPlanMatches(req.blockFingerprints, plan) {
		return nil, ErrScenarioStale
	}
	type inputData struct {
		Data     map[string]any               `json:"data"`
		Fixtures map[string]SimulationFixture `json:"fixtures"`
	}
	encoded, err := json.Marshal(inputData{req.TriggerData, req.Fixtures})
	if err != nil {
		return nil, fmt.Errorf("simulation data is not JSON: %w", err)
	}
	if len(encoded) > MaxStoredOutputBytes {
		return nil, ErrTestDataTooLarge
	}
	var snapshot inputData
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return nil, ErrInvalidSimulationFixture
	}
	req.TriggerData, req.Fixtures = snapshot.Data, snapshot.Fixtures
	reg := NewRegistry()
	if err := RegisterCatalog(reg, nil); err != nil {
		return nil, err
	}
	if plan != nil {
		if _, ok := reg.Lookup(blockCallType); !ok {
			if err := reg.Register(blockCallDef()); err != nil {
				return nil, err
			}
		}
		for id := range req.Fixtures {
			if _, ok := plan.byNode[id]; ok {
				return nil, ErrInvalidSimulationFixture
			}
		}
	}
	if err := validateSimulationFixtures(rec.Draft, reg, req.Fixtures); err != nil {
		return nil, err
	}
	for _, def := range reg.All() {
		if def.Trigger {
			continue
		}
		pure := simulationPureType(def.Type) || def.Type == blockCallType || def.Type == blockEachType
		copyDef := *def
		original := def.Execute
		copyDef.Execute = func(nodeCtx context.Context, in ExecInput) (ExecResult, error) {
			fixture, ok := req.Fixtures[in.Node.ID]
			if !ok {
				if pure {
					return original(nodeCtx, in)
				}
				return ExecResult{}, NewNodeError("FLOW_SIMULATION_FIXTURE_REQUIRED", "this step needs simulation data")
			}
			if fixture.ErrorCode != "" {
				return ExecResult{}, NewNodeError(fixture.ErrorCode, "simulated failure")
			}
			return ExecResult{Output: fixture.Output, Ports: fixture.Ports}, nil
		}
		reg.Replace(&copyDef)
	}
	vc := s.vc(ModeDraft)
	if issues := Validate(rec.Draft, reg, vc); HasErrors(issues) {
		return nil, &ValidationError{Issues: issues}
	}
	trigger := ""
	for i := range rec.Draft.Nodes {
		n := &rec.Draft.Nodes[i]
		def, ok := reg.Lookup(n.Type)
		if !ok || !def.Trigger || n.Settings.Disabled {
			continue
		}
		if req.TriggerNode != "" && n.ID == req.TriggerNode {
			trigger = n.ID
			break
		}
		if req.TriggerNode == "" && (trigger == "" || n.Type == TypeTriggerManual) {
			trigger = n.ID
			if n.Type == TypeTriggerManual {
				break
			}
		}
	}
	if trigger == "" {
		return nil, ErrNoTrigger
	}
	if req.OnlyNode != "" && rec.Draft.NodeByID(req.OnlyNode) == nil {
		return nil, &ValidationError{Issues: []Issue{{Code: IssueNodeNotFound, Severity: SeverityError, Message: "the node to test " + quoteForError(req.OnlyNode) + " does not exist"}}}
	}
	data := req.TriggerData
	if data == nil {
		data, err = s.triggerSample(ctx, rec, trigger)
		if err != nil {
			return nil, err
		}
	}
	encoded, err = json.Marshal(inputData{data, req.Fixtures})
	if err != nil {
		return nil, fmt.Errorf("simulation data is not JSON: %w", err)
	}
	if len(encoded) > MaxStoredOutputBytes {
		return nil, ErrTestDataTooLarge
	}
	at := req.At
	if at.IsZero() {
		at = s.now()
	}
	engine := NewEngine(reg, &Services{Clock: simulationClock{at: at}, Location: s.services.Loc()}, s.logger, 1)
	engine.simulation = true
	var scope *blockRunScope
	if plan != nil {
		scope = &blockRunScope{historyScrubber: s.store.childHistoryScrubber()}
	}
	result := engine.Execute(ctx, RunRequest{Flow: rec.Draft, Revision: rec.DraftRevision, Mode: ModeSimulation,
		TriggerNode: trigger, TriggerType: "simulation", TriggerData: data, OnlyNode: req.OnlyNode, blocks: plan, scope: scope}, nil)
	if result.Steps == nil {
		result.Steps = []StepRecord{}
	}
	return &SimulationResult{Mode: ModeSimulation, Revision: rec.DraftRevision, StartedAt: result.StartedAt, Status: result.Status,
		ErrorCode: result.ErrorCode, ErrorMessage: result.ErrorMessage, ErrorNodeID: result.ErrorNodeID, Steps: result.Steps,
		ChildTraces: result.childTraces, ChildDocuments: childSimulationDocuments(result.childTraces, plan)}, nil
}

func validateSimulationFixtures(doc *Flow, reg *Registry, fixtures map[string]SimulationFixture) error {
	if doc == nil || reg == nil {
		return ErrInvalidSimulationFixture
	}
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		if _, ok := reg.Lookup(n.Type); !ok {
			return &ValidationError{Issues: []Issue{{Code: IssueNodeTypeUnknown, Severity: SeverityError,
				NodeID: truncateRunes(n.ID, maxIssueIDRunes), Message: "node type " + quoteForError(n.Type) + " cannot be simulated"}}}
		}
	}
	for id, fixture := range fixtures {
		switch fixture.ErrorCode {
		case "", "FLOW_NODE_UNAVAILABLE", "FLOW_NODE_TIMEOUT", "FLOW_NODE_FAILED", "FLOW_TOOL_ERROR":
		default:
			return ErrInvalidSimulationFixture
		}
		n := doc.NodeByID(id)
		if n == nil {
			return ErrInvalidSimulationFixture
		}
		def, ok := reg.Lookup(n.Type)
		if !ok || def.Trigger {
			return ErrInvalidSimulationFixture
		}
		var fields []FieldSpec
		var ports []string
		if catchPanic(func() { fields, ports = def.FieldsOf(n), def.OutputPorts(n) }) != nil {
			return ErrInvalidSimulationFixture
		}
		for _, port := range fixture.Ports {
			if port == PortError || !containsString(ports, port) {
				return ErrInvalidSimulationFixture
			}
		}
		for _, field := range fields {
			if value, present := fixture.Output[field.Name]; present && value != nil && !simulationValueMatches(field.Type, value) {
				return ErrInvalidSimulationFixture
			}
		}
	}
	return nil
}

func simulationValueMatches(typ string, value any) bool {
	switch typ {
	case "text":
		_, ok := value.(string)
		return ok
	case "file":
		if _, ok := value.(string); ok {
			return true
		}
		ref, ok := value.(map[string]any)
		if !ok {
			return false
		}
		if _, ok := ref["path"].(string); !ok {
			return false
		}
		for _, key := range []string{"$type", "name", "mime", "web_path"} {
			if field, present := ref[key]; present {
				text, ok := field.(string)
				if !ok || (key == "$type" && text != "file") {
					return false
				}
			}
		}
		if size, present := ref["size"]; present {
			n, ok := size.(float64)
			if !ok || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || math.Trunc(n) != n {
				return false
			}
		}
		return true
	case "number":
		_, ok := value.(float64)
		return ok
	case "bool":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "list", "array":
		_, ok := value.([]any)
		return ok
	default:
		return true
	}
}
