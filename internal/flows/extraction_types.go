package flows

import "errors"

var (
	ErrExtractionInvalid     = errors.New("invalid extraction request")
	ErrExtractionUnsupported = errors.New("selection cannot be extracted safely")
	ErrExtractionConflict    = errors.New("extraction proposal conflicts with stored data")
)

// ExtractionError identifies a fixed, payload-free reason a selection is unsafe.
type ExtractionError struct{ Reason string }

func (e *ExtractionError) Error() string { return "selection cannot be extracted safely: " + e.Reason }
func (e *ExtractionError) Unwrap() error { return ErrExtractionUnsupported }

type ExtractionIDs struct {
	BlockID     string `json:"block_id"`
	CallID      string `json:"call_id"`
	InputID     string `json:"input_id"`
	OutputID    string `json:"output_id"`
	EntryEdgeID string `json:"entry_edge_id"`
	ExitEdgeID  string `json:"exit_edge_id"`
}

type ExtractionRequest struct {
	BaseRevision int      `json:"base_revision"`
	NodeIDs      []string `json:"node_ids"`
	Name         string   `json:"name"`
}

type ExtractionApplyRequest struct {
	ExtractionRequest
	IDs    ExtractionIDs `json:"ids"`
	Digest string        `json:"digest"`
}

type ExtractionReview struct {
	Inputs       []BlockField `json:"inputs"`
	Outputs      []BlockField `json:"outputs"`
	GroupsMoved  int          `json:"groups_moved"`
	GroupsShrunk int          `json:"groups_shrunk"`
	ExtraNodes   int          `json:"extra_nodes"`
	ExtraEdges   int          `json:"extra_edges"`
	BudgetNote   bool         `json:"budget_note"`
}

type ExtractionPreview struct {
	BaseRevision  int              `json:"base_revision"`
	IDs           ExtractionIDs    `json:"ids"`
	Digest        string           `json:"digest"`
	ParentHash    string           `json:"parent_hash"`
	BlockHash     string           `json:"block_hash"`
	BlockRevision int              `json:"block_revision"`
	Review        ExtractionReview `json:"review"`
}

type ExtractionApplyResult struct {
	Flow  *FlowRecord `json:"flow"`
	Block *FlowRecord `json:"block"`
}

type ExtractionReconcileRequest struct {
	BaseRevision int    `json:"base_revision"`
	BlockID      string `json:"block_id"`
	ParentHash   string `json:"parent_hash"`
	BlockHash    string `json:"block_hash"`
}

type ExtractionReconcileResult struct {
	Status string      `json:"status"`
	Flow   *FlowRecord `json:"flow,omitempty"`
	Block  *FlowRecord `json:"block,omitempty"`
}
