package server

import (
	"net/http"
	"strings"

	"aurago/internal/flows"
)

const flowBlocksPath = "/api/desktop/flow-blocks"

// A distinct namespace cannot shadow a legacy flow ID in /flows/{id}.
func isFlowBlocksAPIPath(path string) bool {
	return path == flowBlocksPath || strings.HasPrefix(path, flowBlocksPath+"/")
}

func flowBlockPathParts(path string) []string {
	if !isFlowBlocksAPIPath(path) {
		return nil
	}
	rest := strings.Trim(strings.TrimPrefix(path, flowBlocksPath), "/")
	if rest == "" {
		return nil
	}
	return strings.Split(rest, "/")
}

// Repeated separators are not aliases for a collection or document route.
func validFlowBlockPath(path string) bool {
	return isFlowBlocksAPIPath(path) && !strings.Contains(path, "//")
}

func flowBlocksRequiredScope(method string) string {
	if method == http.MethodGet || method == http.MethodHead {
		return desktopScopeRead
	}
	return desktopScopeAdmin
}

// admitFlowBlockRequest mirrors the existing Flow gates. Validating a document
// is read-only, but still requires the Block API's admin bearer scope and the
// usual same-origin protection for session POSTs.
func (s *Server) admitFlowBlockRequest(w http.ResponseWriter, r *http.Request, parts []string) bool {
	if !authenticateDesktopPermission(s, w, r, flowBlocksRequiredScope(r.Method)) {
		return false
	}
	if !flowsOriginOK(r) {
		flowsError(w, http.StatusForbidden, "FLOW_PERMISSION_DENIED", "same-origin request required")
		return false
	}
	if !s.flowsAvailable() {
		flowsError(w, http.StatusServiceUnavailable, "FLOWS_DISABLED", "EasyDrag flows are disabled")
		return false
	}
	validation := r.Method == http.MethodPost && len(parts) == 1 && parts[0] == "validate"
	operation := desktopMethodOperation(r.Method)
	if validation {
		operation = desktopRead
	}
	cfg := s.ConfigSnapshot()
	if operation != desktopRead && (cfg.Tools.Missions.ReadOnly || cfg.VirtualDesktop.ReadOnly) {
		flowsError(w, http.StatusForbidden, "FLOW_PERMISSION_DENIED", "flows are read-only")
		return false
	}
	if operation != desktopRead && r.Context().Err() != nil {
		s.flowsErrorFrom(w, r, r.Context().Err())
		return false
	}
	return checkDesktopOperation(s, w, r, operation)
}

// handleFlowBlocks serves the separate reusable-block namespace.
func (s *Server) handleFlowBlocks(w http.ResponseWriter, r *http.Request) {
	parts := flowBlockPathParts(r.URL.Path)
	if !s.admitFlowBlockRequest(w, r, parts) {
		return
	}
	if !validFlowBlockPath(r.URL.Path) {
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown block route")
		return
	}
	if len(parts) == 0 {
		s.flowBlocksCollection(w, r)
		return
	}
	if parts[0] == "validate" || parts[0] == "node-types" {
		if len(parts) != 1 {
			flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown block route")
			return
		}
		if parts[0] == "validate" {
			s.flowBlocksValidate(w, r)
		} else {
			s.flowBlocksNodeTypes(w, r)
		}
		return
	}
	id := parts[0]
	if !flowIDPattern.MatchString(id) {
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "block not found")
		return
	}
	if len(parts) == 1 {
		s.flowBlockDocument(w, r, id)
		return
	}
	switch parts[1] {
	case "publish-preview":
		if len(parts) == 2 {
			s.flowBlockPreview(w, r, id)
			return
		}
	case "publish":
		if len(parts) == 2 {
			s.flowBlockPublish(w, r, id)
			return
		}
	case "versions":
		if len(parts) == 4 && parts[3] == "note" {
			s.flowVersionNote(w, r, id, parts[2], flows.KindBlock)
			return
		}
		s.flowBlockVersions(w, r, id, parts[2:])
		return
	case "dependents":
		if len(parts) == 2 {
			s.flowBlockDependents(w, r, id)
			return
		}
	case "simulate":
		if len(parts) == 2 {
			s.flowBlockSimulate(w, r, id)
			return
		}
	}
	flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown block route")
}
