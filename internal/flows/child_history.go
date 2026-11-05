package flows

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// Child-history limits apply to encoded JSON, not the execution's output budget.
const (
	MaxChildTraceBytes              = 256 << 10
	MaxChildSimulationDocumentBytes = 2 << 20
	maxChildHistoryBytes            = 4 << 20
	maxChildHistoryMetadataBytes    = 512 << 10
	maxChildHistoryFrames           = 1000
	maxChildHistorySteps            = 2000
	maxChildHistoryValueBytes       = 16 << 10
)

var (
	ErrChildHistoryNotFound    = errors.New("child history not found")
	ErrChildHistoryUnavailable = errors.New("child history unavailable")
)

type ChildHistoryState string

const (
	ChildHistoryPending       ChildHistoryState = "pending"
	ChildHistoryComplete      ChildHistoryState = "complete"
	ChildHistoryTruncated     ChildHistoryState = "truncated"
	ChildHistoryIncomplete    ChildHistoryState = "incomplete"
	ChildHistoryLegacyUnknown ChildHistoryState = "legacy_unknown"
	ChildDocumentUnavailable                    = "unavailable"
	ChildDocumentRedacted                       = "redacted"
	ChildDocumentBudget                         = "budget"
)

// ChildHistorySummary never claims totals that cannot be proved.
type ChildHistorySummary struct {
	ParentSeq           int               `json:"parent_seq"`
	State               ChildHistoryState `json:"state"`
	Truncated           bool              `json:"truncated,omitempty"`
	RetainedInvocations int               `json:"retained_invocations"`
	TotalInvocations    *int              `json:"total_invocations"`
	RetainedSteps       int               `json:"retained_steps"`
	TotalSteps          *int              `json:"total_steps"`
}

// childSimulationDocuments uses only the already verified immutable plan.
func childSimulationDocuments(traces []ChildTrace, plan *blockPlan) []ChildTraceDocument {
	if plan == nil || len(traces) == 0 {
		return nil
	}
	byKey := make(map[string]resolvedBlock, len(plan.blocks))
	for _, block := range plan.blocks {
		hash := hex.EncodeToString(block.Hash[:])
		byKey[block.Ref.FlowID+":"+strconv.Itoa(block.Ref.Revision)+":"+hash] = block
	}
	seen := map[string]bool{}
	var result []ChildTraceDocument
	bytes := 2
	for _, trace := range traces {
		for _, frame := range trace.Frames {
			key := frame.BlockID + ":" + strconv.Itoa(frame.Revision) + ":" + frame.Hash
			if seen[key] {
				continue
			}
			seen[key] = true
			doc := ChildTraceDocument{BlockID: frame.BlockID, Revision: frame.Revision, Hash: frame.Hash}
			block, ok := byKey[key]
			if !ok {
				doc.DocMissing = ChildDocumentUnavailable
			} else if safe, err := RedactCredentials(block.Document); err != nil {
				doc.DocMissing = ChildDocumentUnavailable
			} else if data, err := json.Marshal(safe); err != nil || len(data) > MaxChildSimulationDocumentBytes-bytes {
				doc.DocMissing = ChildDocumentBudget
			} else {
				bytes += len(data)
				doc.Doc = safe
			}
			result = append(result, doc)
		}
	}
	return result
}

type ChildStep struct {
	Ordinal int        `json:"ordinal"`
	Step    StepRecord `json:"step"`
}

// Earlier attempts have no separate observed timing; omit zero timestamps
// rather than portraying the whole node lifetime as an attempt duration.
func (entry ChildStep) MarshalJSON() ([]byte, error) {
	type plain ChildStep
	data, err := json.Marshal(plain(entry))
	if err != nil || (!entry.Step.StartedAt.IsZero() && !entry.Step.FinishedAt.IsZero()) {
		return data, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	var step map[string]json.RawMessage
	if err := json.Unmarshal(object["step"], &step); err != nil {
		return nil, err
	}
	if entry.Step.StartedAt.IsZero() {
		delete(step, "started_at")
	}
	if entry.Step.FinishedAt.IsZero() {
		delete(step, "finished_at")
	}
	if entry.Step.StartedAt.IsZero() || entry.Step.FinishedAt.IsZero() {
		delete(step, "duration_ms")
	}
	object["step"], err = json.Marshal(step)
	if err != nil {
		return nil, err
	}
	return json.Marshal(object)
}

type ChildFrame struct {
	InvocationID       string      `json:"invocation_id"`
	ParentInvocationID string      `json:"parent_invocation_id,omitempty"`
	CallNodeID         string      `json:"call_node_id"`
	CallAttempt        int         `json:"call_attempt"`
	BlockID            string      `json:"block_id"`
	Revision           int         `json:"revision"`
	Hash               string      `json:"hash"`
	ItemIndex          *int        `json:"item_index,omitempty"`
	Entered            bool        `json:"entered"`
	Status             RunStatus   `json:"status,omitempty"`
	ErrorCode          string      `json:"error_code,omitempty"`
	Steps              []ChildStep `json:"steps"`
}

type ChildTrace struct {
	Schema    int                 `json:"schema"`
	RunID     string              `json:"run_id,omitempty"`
	ParentSeq int                 `json:"parent_seq"`
	History   ChildHistorySummary `json:"history"`
	Frames    []ChildFrame        `json:"frames"`
}

type ChildTraceDocument struct {
	BlockID    string `json:"block_id,omitempty"`
	Revision   int    `json:"revision,omitempty"`
	Hash       string `json:"hash,omitempty"`
	Doc        *Flow  `json:"doc,omitempty"`
	DocMissing string `json:"doc_missing,omitempty"`
}

func ValidChildParentSeq(seq int) bool { return seq > 0 }

func ValidChildInvocationID(id string) bool {
	if !strings.HasPrefix(id, "i_") || len(id) < 3 || len(id) > 6 {
		return false
	}
	n, err := strconv.Atoi(id[2:])
	return err == nil && n >= 1 && n <= maxChildHistoryFrames && id == "i_"+strconv.Itoa(n)
}

// childOmitted drops the entire incomplete value. A partial preview cannot be
// reliably scrubbed when a secret is registered after the preview was cut.
func childOmitted() map[string]any {
	return map[string]any{"_preview": "", "_omitted": "child_history_omitted"}
}

// childBoundValue copies complete values only. The supplied scrubber must walk
// decoded JSON values; it is also applied again on every response.
func childBoundValue(value map[string]any, partial bool, scrub Scrubber) (map[string]any, bool) {
	if value == nil {
		return nil, partial
	}
	if partial {
		return childOmitted(), true
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) > maxChildHistoryValueBytes {
		return childOmitted(), true
	}
	if scrub.JSON != nil {
		data, err = scrub.JSON(data)
		if err != nil || len(data) > maxChildHistoryValueBytes {
			return childOmitted(), true
		}
	}
	var result map[string]any
	if json.Unmarshal(data, &result) != nil || result == nil {
		return childOmitted(), true
	}
	return result, false
}
