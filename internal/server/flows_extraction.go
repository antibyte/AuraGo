package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"aurago/internal/flows"
	"aurago/internal/security"
)

var (
	extractionHashPattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	extractionBlockPattern = regexp.MustCompile(`^flow_[a-z2-7]{26}$`)
	extractionEdgePattern  = regexp.MustCompile(`^e_[a-z2-7]{8}$`)
)

// decodeFlowExtraction reuses the common bounded decoder and metadata Unicode check.
func (s *Server) decodeFlowExtraction(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, flowsSmallBodyLimit)
	var raw json.RawMessage
	if err := decodeJSONRequest(w, r, &raw, s); err != nil {
		if jsonDecodeStatus(err) == http.StatusRequestEntityTooLarge {
			flowsError(w, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE", "the extraction request is too large")
		} else {
			flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "the extraction request is not valid JSON")
		}
		return false
	}
	if !validLibraryJSONText(raw) {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "the extraction request is not valid JSON text")
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "the extraction request is invalid")
		return false
	}
	return true
}

func (s *Server) flowExtractionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, flows.ErrExtractionInvalid):
		flowsError(w, http.StatusBadRequest, "FLOW_EXTRACTION_INVALID", "the extraction request is invalid")
	case errors.Is(err, flows.ErrExtractionConflict):
		flowsError(w, http.StatusConflict, "FLOW_EXTRACTION_CONFLICT", "the extraction proposal conflicts with current data")
	case errors.Is(err, flows.ErrExtractionUnsupported):
		body := map[string]any{"error": "the selected steps cannot be extracted safely", "code": "FLOW_EXTRACTION_UNSUPPORTED"}
		var reason *flows.ExtractionError
		if errors.As(err, &reason) && reason != nil {
			switch reason.Reason {
			case "selection", "boundary", "node", "branch", "fields", "reference", "taint", "groups", "budget":
				body["reason"] = reason.Reason
			}
		}
		flowsJSON(w, http.StatusUnprocessableEntity, body)
	default:
		s.flowsErrorFrom(w, r, err)
	}
}

func validExtractionPreview(p *flows.ExtractionPreview) bool {
	return p != nil && p.BaseRevision > 0 && p.BlockRevision == 1 && extractionBlockPattern.MatchString(p.IDs.BlockID) &&
		flows.ValidNodeID(p.IDs.CallID) && flows.ValidNodeID(p.IDs.InputID) && flows.ValidNodeID(p.IDs.OutputID) &&
		p.IDs.CallID != p.IDs.InputID && p.IDs.CallID != p.IDs.OutputID && p.IDs.InputID != p.IDs.OutputID &&
		extractionEdgePattern.MatchString(p.IDs.EntryEdgeID) && extractionEdgePattern.MatchString(p.IDs.ExitEdgeID) && p.IDs.EntryEdgeID != p.IDs.ExitEdgeID &&
		extractionHashPattern.MatchString(p.Digest) && extractionHashPattern.MatchString(p.ParentHash) && extractionHashPattern.MatchString(p.BlockHash)
}

// flowExtraction operates under handleFlows' admin, origin, availability and readonly gates.
func (s *Server) flowExtraction(w http.ResponseWriter, r *http.Request, id, action string) {
	if action != "preview" && action != "apply" && action != "reconcile" {
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown flow route")
		return
	}
	if r.Method != http.MethodPost {
		flowsMethodNotAllowed(w)
		return
	}
	ctx := r.Context()
	switch action {
	case "preview":
		var body flows.ExtractionRequest
		if !s.decodeFlowExtraction(w, r, &body) {
			return
		}
		proposal, err := s.Flows.PreviewExtraction(ctx, id, body)
		if err != nil {
			s.flowExtractionError(w, r, err)
			return
		}
		if !validExtractionPreview(proposal) || proposal.BaseRevision != body.BaseRevision {
			flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", flowsInternalMessage)
			return
		}
		for i := range proposal.Review.Inputs {
			proposal.Review.Inputs[i].Name = security.Scrub(proposal.Review.Inputs[i].Name)
		}
		for i := range proposal.Review.Outputs {
			proposal.Review.Outputs[i].Name = security.Scrub(proposal.Review.Outputs[i].Name)
		}
		flowsJSON(w, http.StatusOK, proposal)
	case "apply":
		var body flows.ExtractionApplyRequest
		if !s.decodeFlowExtraction(w, r, &body) {
			return
		}
		result, err := s.Flows.ApplyExtraction(ctx, id, body)
		if err != nil {
			s.flowExtractionError(w, r, err)
			return
		}
		if result == nil || result.Flow == nil || result.Block == nil || result.Flow.ID != id || result.Block.ID != body.IDs.BlockID {
			flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", flowsInternalMessage)
			return
		}
		s.recordFlowAudit("flow_block_create", result.Block.ID, "", "Block created")
		s.recordFlowAudit("flow_block_publish", result.Block.ID, "", "Block published")
		s.broadcastFlowsChanged(result.Block.ID, "created")
		s.broadcastFlowsChanged(result.Block.ID, "published")
		s.broadcastFlowsChanged(id, "saved")
		flowsJSON(w, http.StatusOK, result)
	case "reconcile":
		var body flows.ExtractionReconcileRequest
		if !s.decodeFlowExtraction(w, r, &body) {
			return
		}
		result, err := s.Flows.ReconcileExtraction(ctx, id, body)
		if err != nil {
			s.flowExtractionError(w, r, err)
			return
		}
		if result == nil || (result.Status != "applied_state_confirmed" && result.Status != "unknown_or_conflict") ||
			(result.Status == "applied_state_confirmed" && (result.Flow == nil || result.Block == nil || result.Flow.ID != id || result.Block.ID != body.BlockID)) ||
			(result.Status == "unknown_or_conflict" && (result.Flow != nil || result.Block != nil)) {
			flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", flowsInternalMessage)
			return
		}
		flowsJSON(w, http.StatusOK, result)
	}
}
