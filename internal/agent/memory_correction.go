package agent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"aurago/internal/memory"
	"aurago/internal/security"
)

type correctionEvidenceRequest struct {
	Reference string `json:"reference"`
	Quote     string `json:"quote"`
}

type memoryCorrectionArgs struct {
	Operation string                          `json:"operation"`
	ID        string                          `json:"id"`
	Reason    string                          `json:"reason"`
	Changes   []memory.MemoryCorrectionChange `json:"changes"`
	Evidence  []correctionEvidenceRequest     `json:"evidence"`
}

type correctionEvidenceRecord struct{ kind, text string }

// Only the running main loop owns this ledger. Tool arguments cannot populate it.
type memoryCorrectionRun struct {
	mu             sync.Mutex
	id             string
	evidence       map[string]correctionEvidenceRecord
	dirty          bool
	learningChecks map[string]bool
}

func (s *agentLoopState) correctionRun() *memoryCorrectionRun {
	if s.memoryCorrections == nil {
		r := &memoryCorrectionRun{id: rand.Text(), evidence: make(map[string]correctionEvidenceRecord)}
		if !s.runCfg.IsCoAgent && !isCoAgentSession(s.runCfg.SessionID) && !s.runCfg.IsMission && !s.runCfg.IsMaintenance && !s.runCfg.SuppressTurnSideEffects && !isAutonomousAgentRun(s.runCfg, s.runCfg.SessionID) && strings.TrimSpace(s.initialUserMsg) != "" {
			r.evidence["user:current"] = correctionEvidenceRecord{"user", s.initialUserMsg}
		}
		s.memoryCorrections = r
	}
	return s.memoryCorrections
}

func (r *memoryCorrectionRun) resolveEvidence(requests []correctionEvidenceRequest) ([]memory.CorrectionEvidence, error) {
	if r == nil {
		return nil, fmt.Errorf("correction evidence requires an active main-agent turn")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(requests) == 0 || len(requests) > 8 {
		return nil, fmt.Errorf("provide 1 to 8 current-run evidence references and exact quotes")
	}
	var result []memory.CorrectionEvidence
	for _, req := range requests {
		record, ok := r.evidence[req.Reference]
		if !ok || strings.TrimSpace(req.Quote) == "" || len(req.Quote) > 4000 || !strings.Contains(record.text, req.Quote) {
			return nil, fmt.Errorf("evidence reference or quote does not match the current run")
		}
		hash := sha256.Sum256([]byte(record.text))
		result = append(result, memory.CorrectionEvidence{Reference: req.Reference, Kind: record.kind, Quote: req.Quote, ContentHash: hex.EncodeToString(hash[:])})
	}
	return result, nil
}

func decodeMemoryCorrectionArgs(tc ToolCall) (memoryCorrectionArgs, error) {
	args := memoryCorrectionArgs{Operation: tc.Operation, ID: tc.ID, Reason: tc.Reason, Changes: tc.CorrectionChanges, Evidence: tc.CorrectionEvidence}
	if tc.Params != nil {
		data, err := json.Marshal(tc.Params)
		if err != nil {
			return args, err
		}
		if err := json.Unmarshal(data, &args); err != nil {
			return args, fmt.Errorf("invalid correction arguments: %w", err)
		}
		for key, value := range tc.Params {
			if value == nil {
				continue
			}
			switch key {
			case "action", "operation", "id", "reason", "changes", "evidence", "_todo":
			default:
				return args, fmt.Errorf("unsupported correction field %q", key)
			}
		}
	}
	return args, nil
}

func dispatchMemoryCorrection(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	respond := func(status string, plan *memory.MemoryCorrection, err error) string {
		body := struct {
			Status     string                   `json:"status"`
			Message    string                   `json:"message,omitempty"`
			Correction *memory.MemoryCorrection `json:"correction,omitempty"`
		}{Status: status}
		if plan != nil {
			copy := *plan
			copy.Items = append([]memory.MemoryCorrectionItem(nil), plan.Items...)
			copy.Evidence = nil
			for i := range copy.Items {
				copy.Items[i].Snapshot = nil
				copy.Items[i].After = nil
			}
			body.Correction = &copy
		}
		if err != nil {
			body.Message = err.Error()
		}
		data, _ := json.Marshal(body)
		return "Tool Output: " + string(data)
	}
	if dc == nil || dc.Cfg == nil || dc.ShortTermMem == nil || !dc.Cfg.Tools.Memory.Enabled || dc.IsCoAgent || isCoAgentSession(dc.SessionID) {
		return respond("policy_denied", nil, fmt.Errorf("memory corrections require the main agent and enabled memory"))
	}
	args, err := decodeMemoryCorrectionArgs(tc)
	if err != nil {
		return respond("error", nil, err)
	}
	service := memory.MemoryCorrectionService{Memory: dc.ShortTermMem, Vector: dc.LongTermMem, Graph: dc.KG, LearningsEnabled: learningPolicy(dc.Cfg, false) && learningScopeAuthorized(dc.LearningScope, dc.CodeProject, dc.LearningProjectValid), LearningScope: dc.LearningScope, Access: memory.CorrectionAccess{
		MemoryWrite:    !dc.Cfg.Tools.Memory.ReadOnly,
		GraphWrite:     dc.Cfg.Tools.KnowledgeGraph.Enabled && !dc.Cfg.Tools.KnowledgeGraph.ReadOnly,
		ProfileEnabled: dc.Cfg.Personality.UserProfiling,
	}}
	var plan *memory.MemoryCorrection
	switch args.Operation {
	case "preview":
		if args.ID != "" {
			return respond("error", nil, fmt.Errorf("preview does not accept an existing ID"))
		}
		evidence, evidenceErr := dc.MemoryCorrections.resolveEvidence(args.Evidence)
		if evidenceErr != nil {
			return respond("policy_denied", nil, evidenceErr)
		}
		plan, err = service.Preview(ctx, dc.SessionID, dc.MemoryCorrections.id, args.Reason, args.Changes, evidence)
	case "apply", "status":
		if args.ID == "" || args.Reason != "" || len(args.Changes) != 0 || len(args.Evidence) != 0 {
			return respond("error", nil, fmt.Errorf("apply/status accept only operation and the saved correction ID"))
		}
		if args.Operation == "status" {
			plan, err = dc.ShortTermMem.MemoryCorrection(args.ID, dc.SessionID)
			if err == nil {
				for _, item := range plan.Items {
					if item.Reference.Store == "learning" {
						if !service.LearningsEnabled {
							return respond("policy_denied", nil, fmt.Errorf("learnings disabled"))
						}
						if _, e := dc.ShortTermMem.Learning(ctx, item.Reference.ID, dc.LearningScope); e != nil {
							return respond("policy_denied", nil, fmt.Errorf("learning scope unavailable"))
						}
					}
				}
			}
		} else {
			// Invalidate even after cancellation or an uncertain commit; a conservative refresh is safe.
			if dc.MemoryCorrections != nil {
				dc.MemoryCorrections.mu.Lock()
				dc.MemoryCorrections.dirty = true
				dc.MemoryCorrections.mu.Unlock()
			}
			plan, err = service.Apply(ctx, args.ID, dc.SessionID)
		}
	default:
		err = fmt.Errorf("operation must be preview, apply or status")
	}
	if err != nil {
		return respond("error", plan, err)
	}
	return respond("success", plan, nil)
}

func correctionDiagnostic(tc ToolCall, content string) string {
	tc = toolCallForExecutionTracking(tc)
	if tc.Action != "correct_memory" {
		return content
	}
	switch tc.Operation {
	case "preview", "apply", "status":
	default:
		tc.Operation = "invalid"
	}
	if !strings.HasPrefix(tc.ID, "mc_") || len(tc.ID) > 64 || strings.IndexFunc(tc.ID, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	}) >= 0 {
		tc.ID = ""
	}
	return fmt.Sprintf("correct_memory operation=%s id=%s status=%s", tc.Operation, tc.ID, tc.DispatchStatus)
}

func (s *agentLoopState) recordCorrectionEvidence(tc ToolCall, content string, successful bool) string {
	r := s.correctionRun()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dirty {
		s.coreMemDirty = true
		s.cachedSysPromptKey = ""
		if s.turnSnapshot != nil {
			s.turnSnapshot.MemoryDirty = true
		}
		r.dirty = false
	}
	tc = toolCallForExecutionTracking(tc)
	// Only deterministic read handlers attest observations; search, memory, shell and external prose cannot self-authorize corrections.
	verifiedRead := tc.Action == "system_metrics" || ((tc.Action == "remote_control" || tc.Action == "remote_control_devices") && tc.Operation == "sysinfo")
	if !successful || !verifiedRead || len(r.evidence) >= 32 || len(content) > 64000 {
		return content
	}
	ref := "tool:" + rand.Text()
	r.evidence[ref] = correctionEvidenceRecord{"tool", security.Scrub(content)}
	return content + "\nCorrection evidence reference: " + ref
}
