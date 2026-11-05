package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"aurago/internal/flows"
)

const flowFoldersPath = "/api/desktop/flow-folders"

func isFlowFoldersAPIPath(path string) bool {
	return path == flowFoldersPath || strings.HasPrefix(path, flowFoldersPath+"/")
}

func flowFoldersRequiredScope(method string) string {
	if method == http.MethodGet || method == http.MethodHead {
		return desktopScopeRead
	}
	return desktopScopeAdmin
}

// admitFlowFolderRequest applies the same Desktop and flow gates as block mutations.
func (s *Server) admitFlowFolderRequest(w http.ResponseWriter, r *http.Request) bool {
	if !authenticateDesktopPermission(s, w, r, flowFoldersRequiredScope(r.Method)) {
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
	operation := desktopMethodOperation(r.Method)
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

type flowLibraryPatch struct {
	BaseRevision int
	SetFolder    bool
	FolderID     *string
	SetTags      bool
	Tags         []string
}

// validLibraryJSONText checks raw UTF-8 and escaped surrogate pairs before
// Go's JSON decoder can replace invalid metadata text with U+FFFD.
func validLibraryJSONText(data []byte) bool {
	if !utf8.Valid(data) || !json.Valid(data) {
		return false
	}
	inside := false
	for i := 0; i < len(data); i++ {
		if !inside {
			if data[i] == '"' {
				inside = true
			}
			continue
		}
		if data[i] == '"' {
			inside = false
			continue
		}
		if data[i] != '\\' {
			continue
		}
		if data[i+1] != 'u' {
			i++
			continue
		}
		code := libraryHex16(data[i+2 : i+6])
		if code >= 0xd800 && code <= 0xdbff {
			if i+12 > len(data) || data[i+6] != '\\' || data[i+7] != 'u' {
				return false
			}
			low := libraryHex16(data[i+8 : i+12])
			if low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 11
		} else {
			if code >= 0xdc00 && code <= 0xdfff {
				return false
			}
			i += 5
		}
	}
	return true
}

// json.Valid has already checked that these four bytes are hexadecimal.
func libraryHex16(data []byte) uint16 {
	var result uint16
	for _, b := range data {
		result <<= 4
		switch {
		case b >= '0' && b <= '9':
			result |= uint16(b - '0')
		case b >= 'a' && b <= 'f':
			result |= uint16(b-'a') + 10
		default:
			result |= uint16(b-'A') + 10
		}
	}
	return result
}

// decodeFlowLibraryPatch preserves the distinction between omitted fields and
// explicit clears. A masked field never needs to travel back to the server.
func decodeFlowLibraryPatch(w http.ResponseWriter, r *http.Request) (flowLibraryPatch, bool) {
	var body struct {
		BaseRevision *int            `json:"base_revision"`
		FolderID     json.RawMessage `json:"folder_id"`
		Tags         json.RawMessage `json:"tags"`
	}
	if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
		return flowLibraryPatch{}, false
	}
	if body.BaseRevision == nil || *body.BaseRevision < 0 || len(body.FolderID) == 0 && len(body.Tags) == 0 {
		flowsError(w, http.StatusBadRequest, "FLOW_LIBRARY_INVALID", "a base revision and at least one field are required")
		return flowLibraryPatch{}, false
	}
	patch := flowLibraryPatch{BaseRevision: *body.BaseRevision}
	if len(body.FolderID) != 0 {
		patch.SetFolder = true
		if !validLibraryJSONText(body.FolderID) {
			flowsError(w, http.StatusBadRequest, "FLOW_LIBRARY_INVALID", "folder_id must contain valid UTF-8 JSON text")
			return flowLibraryPatch{}, false
		}
		if !bytes.Equal(bytes.TrimSpace(body.FolderID), []byte("null")) {
			var id string
			if err := json.Unmarshal(body.FolderID, &id); err != nil || id == "" {
				flowsError(w, http.StatusBadRequest, "FLOW_LIBRARY_INVALID", "folder_id must be a folder ID or null")
				return flowLibraryPatch{}, false
			}
			patch.FolderID = &id
		}
	}
	if len(body.Tags) != 0 {
		patch.SetTags = true
		if !validLibraryJSONText(body.Tags) || bytes.Equal(bytes.TrimSpace(body.Tags), []byte("null")) || json.Unmarshal(body.Tags, &patch.Tags) != nil || patch.Tags == nil {
			flowsError(w, http.StatusBadRequest, "FLOW_LIBRARY_INVALID", "tags must be an array of strings")
			return flowLibraryPatch{}, false
		}
	}
	return patch, true
}

// optionalFlowPublishNote keeps omission distinct from an explicit empty note;
// null and non-string values are rejected before any publish side effect.
func optionalFlowPublishNote(w http.ResponseWriter, raw json.RawMessage) (*string, bool) {
	if len(raw) == 0 {
		return nil, true
	}
	if !validLibraryJSONText(raw) || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		flowsError(w, http.StatusBadRequest, "FLOW_LIBRARY_INVALID", "note must be a string")
		return nil, false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		flowsError(w, http.StatusBadRequest, "FLOW_LIBRARY_INVALID", "note must be a string")
		return nil, false
	}
	return &value, true
}

func decodeFlowFolderName(w http.ResponseWriter, raw json.RawMessage) (string, bool) {
	var name string
	if len(raw) == 0 || !validLibraryJSONText(raw) || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &name) != nil {
		flowsError(w, http.StatusBadRequest, "FLOW_LIBRARY_INVALID", "name must be valid UTF-8 text")
		return "", false
	}
	return name, true
}

func (s *Server) handleFlowFolders(w http.ResponseWriter, r *http.Request) {
	if !s.admitFlowFolderRequest(w, r) {
		return
	}
	if !isFlowFoldersAPIPath(r.URL.Path) || strings.Contains(r.URL.Path, "//") {
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown folder route")
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, flowFoldersPath), "/")
	if path == "" {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			folders, err := s.Flows.ListLibraryFolders(r.Context())
			if err != nil {
				s.flowsErrorFrom(w, r, err)
				return
			}
			for i := range folders {
				folders[i].RefreshPrivacy()
			}
			flowsJSON(w, http.StatusOK, map[string]any{"folders": folders})
		case http.MethodPost:
			var body struct {
				Name json.RawMessage `json:"name"`
			}
			if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
				return
			}
			name, ok := decodeFlowFolderName(w, body.Name)
			if !ok {
				return
			}
			folder, err := s.Flows.CreateLibraryFolder(r.Context(), name)
			if err != nil {
				s.flowsErrorFrom(w, r, err)
				return
			}
			s.recordFlowAudit("flow_library_change", folder.ID, "", "Flow library changed")
			s.broadcastFlowsChanged("", "folders")
			folder.RefreshPrivacy()
			flowsJSON(w, http.StatusCreated, map[string]any{"folder": folder})
		default:
			flowsMethodNotAllowed(w)
		}
		return
	}
	if strings.Contains(path, "/") || !flowIDPattern.MatchString(path) {
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "folder not found")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var body struct {
			Name         json.RawMessage `json:"name"`
			BaseRevision *int            `json:"base_revision"`
		}
		if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
			return
		}
		if body.BaseRevision == nil || *body.BaseRevision <= 0 {
			flowsError(w, http.StatusBadRequest, "FLOW_LIBRARY_INVALID", "a name and base revision are required")
			return
		}
		name, ok := decodeFlowFolderName(w, body.Name)
		if !ok {
			return
		}
		folder, err := s.Flows.UpdateLibraryFolder(r.Context(), path, name, *body.BaseRevision)
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		s.recordFlowAudit("flow_library_change", path, "", "Flow library changed")
		s.broadcastFlowsChanged("", "folders")
		folder.RefreshPrivacy()
		flowsJSON(w, http.StatusOK, map[string]any{"folder": folder})
	case http.MethodDelete:
		var body struct {
			BaseRevision *int `json:"base_revision"`
		}
		if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
			return
		}
		if body.BaseRevision == nil || *body.BaseRevision <= 0 {
			flowsError(w, http.StatusBadRequest, "FLOW_LIBRARY_INVALID", "a base revision is required")
			return
		}
		if err := s.Flows.DeleteLibraryFolder(r.Context(), path, *body.BaseRevision); err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		s.recordFlowAudit("flow_library_change", path, "", "Flow library changed")
		s.broadcastFlowsChanged("", "folders")
		flowsJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		flowsMethodNotAllowed(w)
	}
}

func (s *Server) flowLibrary(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		metadata, err := s.Flows.GetFlowLibrary(r.Context(), id)
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		metadata.RefreshPrivacy()
		flowsJSON(w, http.StatusOK, map[string]any{"library": metadata})
	case http.MethodPut:
		patch, ok := decodeFlowLibraryPatch(w, r)
		if !ok {
			return
		}
		metadata, err := s.Flows.UpdateFlowLibrary(r.Context(), id, flows.LibraryUpdate{BaseRevision: patch.BaseRevision, SetFolder: patch.SetFolder, FolderID: patch.FolderID, SetTags: patch.SetTags, Tags: patch.Tags})
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		s.recordFlowAudit("flow_library_change", id, "", "Flow library changed")
		s.broadcastFlowsChanged(id, "library")
		metadata.RefreshPrivacy()
		flowsJSON(w, http.StatusOK, map[string]any{"library": metadata})
	default:
		flowsMethodNotAllowed(w)
	}
}

func (s *Server) flowVersionNote(w http.ResponseWriter, r *http.Request, id, rawRevision string, kind flows.Kind) {
	revision, err := strconv.Atoi(rawRevision)
	if err != nil || revision <= 0 {
		flowsError(w, http.StatusBadRequest, "FLOW_LIBRARY_INVALID", "a positive version is required")
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		note, err := s.Flows.GetVersionNote(r.Context(), id, kind, revision)
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		note.RefreshPrivacy()
		flowsJSON(w, http.StatusOK, note)
	case http.MethodPut:
		var body struct {
			Note         json.RawMessage `json:"note"`
			BaseRevision *int            `json:"base_revision"`
		}
		if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
			return
		}
		if body.BaseRevision == nil || *body.BaseRevision < 0 || len(body.Note) == 0 {
			flowsError(w, http.StatusBadRequest, "FLOW_LIBRARY_INVALID", "a note and base revision are required")
			return
		}
		value, ok := optionalFlowPublishNote(w, body.Note)
		if !ok {
			return
		}
		note, err := s.Flows.SetVersionNote(r.Context(), id, kind, revision, *body.BaseRevision, *value)
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		s.recordFlowAudit("flow_version_note", id, "", "Flow version note changed")
		s.broadcastFlowsChanged(id, "version_note")
		note.RefreshPrivacy()
		flowsJSON(w, http.StatusOK, note)
	default:
		flowsMethodNotAllowed(w)
	}
}
