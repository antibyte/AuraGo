package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"aurago/internal/flows"
)

// Block errors never return dependency payloads or reveal the kind of another ID.
func (s *Server) flowBlockError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, flows.ErrWrongFlowKind):
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "block not found")
	case errors.Is(err, flows.ErrBlockInUse):
		flowsError(w, http.StatusConflict, "FLOW_BLOCK_IN_USE", "the block is used by a published flow or retained run")
	case errors.Is(err, flows.ErrBlockInvalid):
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "invalid block request")
	default:
		s.flowsErrorFrom(w, r, err)
	}
}

func (s *Server) flowBlockSimulate(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		flowsMethodNotAllowed(w)
		return
	}
	var body struct {
		BaseRevision int            `json:"base_revision"`
		Inputs       map[string]any `json:"inputs"`
		At           time.Time      `json:"at"`
	}
	if !flowsDecode(w, r, &body, flowsTestDataBodyLimit, false) {
		return
	}
	if body.BaseRevision <= 0 {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "a positive draft revision is required")
		return
	}
	select {
	case flowSimulationSlots <- struct{}{}:
		defer func() { <-flowSimulationSlots }()
	default:
		flowsError(w, http.StatusTooManyRequests, "FLOW_RUN_LIMIT", "too many simulations are running")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := s.Flows.SimulateBlock(ctx, id, flows.BlockSimulationRequest{BaseRevision: body.BaseRevision, Inputs: body.Inputs, At: body.At})
	if ctx.Err() != nil {
		if r.Context().Err() != nil {
			s.flowsErrorFrom(w, r, r.Context().Err())
		} else {
			flowsError(w, http.StatusServiceUnavailable, "FLOW_SIMULATION_TIMEOUT", "the simulation took too long")
		}
		return
	}
	if err != nil {
		s.flowBlockError(w, r, err)
		return
	}
	s.flowsJSONScrubbed(w, http.StatusOK, result)
}

func (s *Server) flowBlocksCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		limit := 50
		if value := r.URL.Query().Get("limit"); value != "" {
			var err error
			limit, err = strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 100 {
				flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "limit must be between 1 and 100")
				return
			}
		}
		blocks, err := s.Flows.ListBlocks(r.Context(), r.URL.Query().Get("cursor"), limit)
		if err != nil {
			s.flowBlockError(w, r, err)
			return
		}
		answer := map[string]any{"blocks": blocks.Items}
		if blocks.NextCursor != "" {
			answer["next_cursor"] = blocks.NextCursor
		}
		flowsJSON(w, http.StatusOK, answer)
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
			return
		}
		block, err := s.Flows.CreateBlock(r.Context(), body.Name)
		if err != nil {
			s.flowBlockError(w, r, err)
			return
		}
		s.recordFlowAudit("flow_block_create", block.ID, "", "Block created")
		s.broadcastFlowsChanged(block.ID, "created")
		flowsJSON(w, http.StatusCreated, map[string]any{"block": block})
	default:
		flowsMethodNotAllowed(w)
	}
}

func (s *Server) flowBlocksValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		flowsMethodNotAllowed(w)
		return
	}
	var body struct {
		Doc  json.RawMessage `json:"doc"`
		Mode string          `json:"mode"`
	}
	if !flowsDecode(w, r, &body, flowsDocBodyLimit, false) {
		return
	}
	var mode flows.ValidationMode
	switch body.Mode {
	case "", "draft":
		mode = flows.ModeDraft
	case "publish":
		mode = flows.ModePublish
	default:
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", `mode must be "draft" or "publish"`)
		return
	}
	doc, err := flows.ParseFlow(body.Doc)
	if err != nil {
		flowsDocumentError(w, err)
		return
	}
	if doc.Kind != flows.KindBlock {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "a block document is required")
		return
	}
	issues := nonNilIssues(s.Flows.ValidateBlock(doc, mode))
	flowsJSON(w, http.StatusOK, map[string]any{"valid": !flows.HasErrors(issues), "issues": issues})
}

func (s *Server) flowBlocksNodeTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		flowsMethodNotAllowed(w)
		return
	}
	tr := flowsTranslator(s.flowsLang(r))
	infos := s.Flows.BlockCatalog(tr)
	flowsJSON(w, http.StatusOK, map[string]any{"node_types": infos, "categories": flowCategories(infos, tr)})
}

func (s *Server) flowBlockDocument(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		block, err := s.Flows.GetBlock(ctx, id)
		if err != nil {
			s.flowBlockError(w, r, err)
			return
		}
		issues := s.Flows.ValidateBlock(block.Draft, flows.ModeDraft)
		answer := map[string]any{"block": block, "issues": nonNilIssues(issues)}
		if contract, err := flows.BlockDraftContract(block.Draft); err == nil {
			answer["contract"] = contract
		}
		flowsJSON(w, http.StatusOK, answer)
	case http.MethodPut:
		var body struct {
			Doc          json.RawMessage `json:"doc"`
			BaseRevision *int            `json:"base_revision"`
		}
		if !flowsDecode(w, r, &body, flowsDocBodyLimit, false) {
			return
		}
		if body.BaseRevision == nil || *body.BaseRevision <= 0 {
			flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "a positive base revision is required")
			return
		}
		doc, err := flows.ParseFlow(body.Doc)
		if err != nil {
			flowsDocumentError(w, err)
			return
		}
		if doc.Kind != flows.KindBlock {
			flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "a block document is required")
			return
		}
		revision, issues, err := s.Flows.SaveBlockDraft(ctx, id, doc, *body.BaseRevision)
		if err != nil {
			s.flowBlockError(w, r, err)
			return
		}
		s.broadcastFlowsChanged(id, "saved")
		flowsJSON(w, http.StatusOK, map[string]any{"draft_revision": revision, "issues": nonNilIssues(issues)})
	case http.MethodDelete:
		var body struct {
			BaseRevision     *int `json:"base_revision"`
			BaseLiveRevision *int `json:"base_live_revision"`
		}
		if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
			return
		}
		if body.BaseRevision == nil || *body.BaseRevision <= 0 || body.BaseLiveRevision == nil || *body.BaseLiveRevision < 0 {
			flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "draft and live revisions are required")
			return
		}
		if err := s.Flows.DeleteBlock(ctx, id, *body.BaseRevision, *body.BaseLiveRevision); err != nil {
			s.flowBlockError(w, r, err)
			return
		}
		s.recordFlowAudit("flow_block_delete", id, "", "Block deleted")
		s.broadcastFlowsChanged(id, "deleted")
		flowsJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		flowsMethodNotAllowed(w)
	}
}

func (s *Server) flowBlockPreview(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		flowsMethodNotAllowed(w)
		return
	}
	preview, err := s.Flows.BlockPublishPreview(r.Context(), id)
	if err != nil {
		s.flowBlockError(w, r, err)
		return
	}
	s.flowsJSONScrubbed(w, http.StatusOK, preview)
}

func (s *Server) flowBlockPublish(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		flowsMethodNotAllowed(w)
		return
	}
	var body struct {
		BaseRevision *int `json:"base_revision"`
	}
	if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
		return
	}
	if body.BaseRevision == nil || *body.BaseRevision <= 0 {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "a positive base revision is required")
		return
	}
	block, issues, err := s.Flows.PublishBlock(r.Context(), id, *body.BaseRevision)
	if err != nil {
		s.flowBlockError(w, r, err)
		return
	}
	s.recordFlowAudit("flow_block_publish", id, "", "Block published")
	s.broadcastFlowsChanged(id, "published")
	flowsJSON(w, http.StatusOK, map[string]any{"block": block, "issues": nonNilIssues(issues)})
}

func (s *Server) flowBlockVersions(w http.ResponseWriter, r *http.Request, id string, rest []string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		flowsMethodNotAllowed(w)
		return
	}
	if len(rest) == 0 {
		before, limit := 0, 50
		if value := r.URL.Query().Get("before_revision"); value != "" {
			var err error
			before, err = strconv.Atoi(value)
			if err != nil || before < 0 {
				flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "before_revision must be zero or positive")
				return
			}
		}
		if value := r.URL.Query().Get("limit"); value != "" {
			var err error
			limit, err = strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 100 {
				flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "limit must be between 1 and 100")
				return
			}
		}
		versions, err := s.Flows.BlockVersions(r.Context(), id, before, limit)
		if err != nil {
			s.flowBlockError(w, r, err)
			return
		}
		answer := map[string]any{"versions": versions.Items}
		if versions.NextCursor > 0 {
			answer["next_cursor"] = versions.NextCursor
		}
		flowsJSON(w, http.StatusOK, answer)
		return
	}
	if len(rest) != 1 {
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown block route")
		return
	}
	rev, err := strconv.Atoi(rest[0])
	if err != nil || rev <= 0 {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "a positive revision is required")
		return
	}
	version, err := s.Flows.GetBlockVersion(r.Context(), id, rev)
	if err != nil {
		s.flowBlockError(w, r, err)
		return
	}
	flowsJSON(w, http.StatusOK, version)
}

func (s *Server) flowBlockDependents(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		flowsMethodNotAllowed(w)
		return
	}
	q := r.URL.Query()
	revision := 0
	if value := q.Get("revision"); value != "" {
		var err error
		revision, err = strconv.Atoi(value)
		if err != nil || revision < 0 {
			flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "revision must be zero or positive")
			return
		}
	}
	limit := 50
	if value := q.Get("limit"); value != "" {
		var err error
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "limit must be between 1 and 100")
			return
		}
	}
	page, err := s.Flows.BlockDependents(r.Context(), id, revision, q.Get("cursor"), limit)
	if err != nil {
		s.flowBlockError(w, r, err)
		return
	}
	flowsJSON(w, http.StatusOK, page)
}
