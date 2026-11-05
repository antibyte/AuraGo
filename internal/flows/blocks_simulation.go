package flows

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// BlockSimulationRequest supplies typed data for an isolated draft preview.
type BlockSimulationRequest struct {
	BaseRevision int            `json:"base_revision"`
	Inputs       map[string]any `json:"inputs"`
	At           time.Time      `json:"at,omitempty"`
}

// BlockSimulationResult is a nonpersistent simulation and its declared outputs.
type BlockSimulationResult struct {
	SimulationResult
	Outputs map[string]any `json:"outputs"`
}

// SimulateBlock executes one verified pure block draft without tools or persistence.
func (s *Service) SimulateBlock(ctx context.Context, id string, req BlockSimulationRequest) (*BlockSimulationResult, error) {
	rec, err := s.GetBlock(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.BaseRevision <= 0 || req.BaseRevision != rec.DraftRevision {
		return nil, ErrRevisionConflict
	}
	issues := s.ValidateBlock(rec.Draft, ModePublish)
	if HasErrors(issues) {
		return nil, &ValidationError{Issues: issues}
	}
	plan, err := prepareDraftBlockPlan(ctx, s.store, rec.Draft, rec.DraftRevision, s.services)
	if err != nil {
		return nil, err
	}
	contract, err := readBlockContract(rec.Draft)
	if err != nil {
		return nil, ErrBlockInvalid
	}
	data, err := json.Marshal(req.Inputs)
	if err != nil {
		return nil, ErrBlockInvalid
	}
	if len(data) > MaxStoredOutputBytes {
		return nil, ErrTestDataTooLarge
	}
	inputs := map[string]any{}
	if string(data) != "null" {
		if err := json.Unmarshal(data, &inputs); err != nil {
			return nil, ErrBlockInvalid
		}
	}
	if err := checkBlockValues(contract.Inputs, inputs); err != nil {
		return nil, ErrBlockInvalid
	}
	at := req.At
	if at.IsZero() {
		at = s.now()
	}
	services := &Services{Clock: simulationClock{at: at}, Location: s.services.Loc()}
	result := executePureBlockPlanned(ctx, rec.Draft, inputs, services, s.logger, ModeSimulation, rec.DraftRevision, "", &blockRunScope{}, plan)
	response := &BlockSimulationResult{SimulationResult: SimulationResult{Mode: ModeSimulation, Revision: rec.DraftRevision,
		StartedAt: result.StartedAt, Status: result.Status, ErrorCode: result.ErrorCode, ErrorMessage: result.ErrorMessage,
		ErrorNodeID: result.ErrorNodeID, Steps: result.Steps}}
	if response.Steps == nil {
		response.Steps = []StepRecord{}
	}
	if result.Status == RunSuccess {
		for _, node := range rec.Draft.Nodes {
			if node.Type == blockOutputType {
				output, ok := result.Outputs[node.Key]
				if !ok || checkBlockValues(contract.Outputs, output) != nil {
					response.Status, response.ErrorCode, response.ErrorMessage = RunError, "FLOW_PARAM_INVALID", "block outputs do not match the declared contract"
				} else {
					response.Outputs = output
				}
				break
			}
		}
	}
	return response, nil
}

// executePureBlock uses only fresh built-ins and never a production invoker.
func executePureBlock(ctx context.Context, doc *Flow, inputs map[string]any, services *Services, logger *slog.Logger, mode RunMode, revision int, runID string, scope *blockRunScope) RunResult {
	return executePureBlockPlanned(ctx, doc, inputs, services, logger, mode, revision, runID, scope, nil)
}

func executePureBlockPlanned(ctx context.Context, doc *Flow, inputs map[string]any, services *Services, logger *slog.Logger, mode RunMode, revision int, runID string, scope *blockRunScope, plan *blockPlan) RunResult {
	if !blockDocumentAllowed(doc) {
		return RunResult{Status: RunError, ErrorCode: "FLOW_PARAM_INVALID", ErrorMessage: "the block contains an unsupported node"}
	}
	contract, err := readBlockContract(doc)
	if err != nil || checkBlockValues(contract.Inputs, inputs) != nil {
		return RunResult{Status: RunError, ErrorCode: "FLOW_PARAM_INVALID", ErrorMessage: "block inputs do not match the declared contract"}
	}
	trigger := ""
	for _, node := range doc.Nodes {
		if node.Type == blockInputType {
			trigger = node.ID
			break
		}
	}
	engine := NewEngine(blockRegistry(), services, logger, 1)
	engine.simulation = mode == ModeSimulation
	result := engine.Execute(ctx, RunRequest{RunID: runID, Flow: doc, Revision: revision, Mode: mode, TriggerNode: trigger,
		TriggerType: blockInputType, TriggerData: inputs, scope: scope, blocks: plan}, nil)
	if result.Status == RunSuccess {
		for _, node := range doc.Nodes {
			if node.Type != blockOutputType {
				continue
			}
			output, ok := result.Outputs[node.Key]
			if !ok || checkBlockValues(contract.Outputs, output) != nil {
				result.Status, result.ErrorCode, result.ErrorMessage, result.ErrorNodeID = RunError, "FLOW_PARAM_INVALID", "block outputs do not match the declared contract", node.ID
			}
			break
		}
	}
	return result
}
