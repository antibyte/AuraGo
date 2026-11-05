package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

func (s *Server) flowPackExport(w http.ResponseWriter, r *http.Request, flowID string) {
	if r.Method != http.MethodGet {
		flowsMethodNotAllowed(w)
		return
	}
	pack, err := s.Flows.ExportLocalPack(r.Context(), flowID)
	if err != nil {
		s.flowPackError(w, r, err)
		return
	}
	if pack.Format != "aurago.easydrag.local-pack" || pack.Version != 1 || pack.Parent.Flow == nil {
		flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", flowsInternalMessage)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+flowPackExportName(pack.Parent.Flow.Name)+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err := flowsWriteJSON(w, http.StatusOK, pack); err != nil {
		s.Logger.Warn("Local flow pack response could not be encoded", "flow_id", flowID, "error", flowsErrorText(err))
	}
}

func flowPackExportName(name string) string {
	return strings.TrimSuffix(flowExportName(name), ".easydrag.json") + ".easydrag-pack.json"
}

func (s *Server) flowsPacks(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) != 1 {
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown flow pack route")
		return
	}
	if r.Method != http.MethodPost {
		flowsMethodNotAllowed(w)
		return
	}
	switch rest[0] {
	case "preview":
		s.flowPackPreview(w, r)
	case "import":
		s.flowPackImport(w, r)
	case "reconcile":
		s.flowPackReconcile(w, r)
	default:
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown flow pack route")
	}
}

func (s *Server) flowPackPreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pack flows.LocalPack `json:"pack"`
	}
	if !s.decodeFlowPackJSON(w, r, &body) {
		return
	}
	preview, err := s.Flows.PreviewLocalPack(r.Context(), body.Pack)
	if err != nil {
		s.flowPackError(w, r, err)
		return
	}
	if !validLocalPackProposal(preview.Proposal) {
		flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", flowsInternalMessage)
		return
	}
	flowsJSON(w, http.StatusOK, preview)
}

func (s *Server) flowPackImport(w http.ResponseWriter, r *http.Request) {
	var body flows.LocalPackImportRequest
	if !s.decodeFlowPackJSON(w, r, &body) {
		return
	}
	result, err := s.Flows.ImportLocalPack(r.Context(), body)
	if err != nil {
		s.flowPackError(w, r, err)
		return
	}
	if !validLocalPackImportResult(result, body.Proposal) {
		flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", flowsInternalMessage)
		return
	}
	s.recordFlowAudit("flow_import", result.Flow.ID, result.Flow.Name, "Flow "+result.Flow.Name+" imported from a local pack")
	s.broadcastFlowsChanged(result.Flow.ID, "created")
	for _, block := range result.Blocks {
		s.recordFlowAudit("flow_block_create", block.ID, block.Name, "Flow block "+block.Name+" imported from a local pack")
		s.broadcastFlowsChanged(block.ID, "created")
	}
	flowsJSON(w, http.StatusOK, result)
}

func (s *Server) flowPackReconcile(w http.ResponseWriter, r *http.Request) {
	var body flows.LocalPackReconcileRequest
	if !s.decodeFlowPackJSON(w, r, &body) {
		return
	}
	result, err := s.Flows.ReconcileLocalPack(r.Context(), body)
	if err != nil {
		s.flowPackError(w, r, err)
		return
	}
	switch result.Status {
	case "applied_state_confirmed":
		if !validLocalPackRecords(result.Flow, result.Blocks, body.Proposal) {
			flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", flowsInternalMessage)
			return
		}
	case "unknown":
		if result.Flow != nil || len(result.Blocks) != 0 {
			flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", flowsInternalMessage)
			return
		}
	default:
		flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", flowsInternalMessage)
		return
	}
	flowsJSON(w, http.StatusOK, result)
}

func (s *Server) decodeFlowPackJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, flowPackBodyLimit))
	if err != nil {
		if jsonDecodeStatus(err) == http.StatusRequestEntityTooLarge {
			flowsError(w, http.StatusRequestEntityTooLarge, "FLOW_PACK_TOO_LARGE", "the flow pack request is too large")
		} else {
			flowsError(w, http.StatusBadRequest, "FLOW_PACK_INVALID", "the flow pack request is not valid JSON")
		}
		return false
	}
	if len(bytes.TrimSpace(raw)) == 0 || !validLibraryJSONText(raw) {
		flowsError(w, http.StatusBadRequest, "FLOW_PACK_INVALID", "the flow pack request contains invalid text")
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		flowsError(w, http.StatusBadRequest, "FLOW_PACK_INVALID", "the flow pack request is invalid")
		return false
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		flowsError(w, http.StatusBadRequest, "FLOW_PACK_INVALID", "the flow pack request contains additional JSON")
		return false
	}
	return true
}

func (s *Server) flowPackError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, flows.ErrLocalPackTooLarge):
		flowsError(w, http.StatusRequestEntityTooLarge, "FLOW_PACK_TOO_LARGE", "the flow pack is too large")
	case errors.Is(err, flows.ErrLocalPackInvalid):
		flowsError(w, http.StatusBadRequest, "FLOW_PACK_INVALID", "the flow pack is invalid")
	case errors.Is(err, flows.ErrLocalPackUnsupported):
		flowsError(w, http.StatusUnprocessableEntity, "FLOW_PACK_UNSUPPORTED", "the flow pack contains unsupported content")
	case errors.Is(err, flows.ErrLocalPackConflict):
		flowsError(w, http.StatusConflict, "FLOW_PACK_CONFLICT", "the flow pack proposal conflicts with current data")
	case errors.Is(err, tools.ErrFlowMissionOwnerConflict):
		flowsError(w, http.StatusConflict, "FLOW_PACK_CONFLICT", "the proposed flow ID is already owned in Mission Control")
	default:
		s.flowsErrorFrom(w, r, err)
	}
}

func validLocalPackProposal(proposal flows.LocalPackProposal) bool {
	if !extractionHashPattern.MatchString(proposal.PackDigest) || !extractionHashPattern.MatchString(proposal.Digest) ||
		!extractionBlockPattern.MatchString(proposal.ParentID) || !extractionHashPattern.MatchString(proposal.ParentHash) {
		return false
	}
	seen := make(map[string]struct{}, len(proposal.Blocks))
	seenLocal := make(map[string]struct{}, len(proposal.Blocks))
	for _, block := range proposal.Blocks {
		key := block.SourceID + "/" + strconv.Itoa(block.SourceRevision)
		if !flowIDPattern.MatchString(block.SourceID) || block.SourceRevision < 1 ||
			!extractionBlockPattern.MatchString(block.LocalID) || block.LocalID == block.SourceID || !extractionHashPattern.MatchString(block.Hash) {
			return false
		}
		if _, ok := seen[key]; ok {
			return false
		}
		if _, ok := seenLocal[block.LocalID]; ok || block.LocalID == proposal.ParentID {
			return false
		}
		seen[key] = struct{}{}
		seenLocal[block.LocalID] = struct{}{}
	}
	return true
}

func validLocalPackImportResult(result flows.LocalPackImportResult, proposal flows.LocalPackProposal) bool {
	return result.Status == "applied" && validLocalPackRecords(result.Flow, result.Blocks, proposal)
}

func validLocalPackRecords(parent *flows.LocalPackRecord, blocks []flows.LocalPackRecord, proposal flows.LocalPackProposal) bool {
	if parent == nil || parent.ID != proposal.ParentID || parent.Kind != flows.KindFlow || parent.DraftRevision != 1 || parent.LiveRevision != 0 || parent.MissionID == "" || parent.Hash != proposal.ParentHash || len(parent.Pins) != 0 || len(blocks) != len(proposal.Blocks) {
		return false
	}
	byID := make(map[string]flows.LocalPackRecord, len(blocks))
	want := make(map[string]string, len(proposal.Blocks))
	for _, target := range proposal.Blocks {
		want[target.LocalID] = target.Hash
	}
	for _, block := range blocks {
		if block.Kind != flows.KindBlock || block.DraftRevision != 1 || block.LiveRevision != 1 || block.MissionID != "" || want[block.ID] == "" || block.Hash != want[block.ID] {
			return false
		}
		if _, exists := byID[block.ID]; exists {
			return false
		}
		for _, pin := range block.Pins {
			if pin.Revision != 1 || want[pin.BlockID] == "" || pin.Hash != want[pin.BlockID] {
				return false
			}
		}
		byID[block.ID] = block
	}
	for _, target := range proposal.Blocks {
		if _, ok := byID[target.LocalID]; !ok {
			return false
		}
	}
	return true
}
