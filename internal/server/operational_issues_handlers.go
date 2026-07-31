package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/planner"
)

const operationalIssueArchiveConfirm = "ARCHIVE_STALE_ISSUES"

type operationalIssueMutationRequest struct {
	Confirm    string `json:"confirm,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

type operationalIssueAPIItem struct {
	ID string `json:"id"`
	planner.OperationalIssueListItem
}

func handleOperationalIssues(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.PlannerDB == nil {
			writeOperationalIssueError(w, http.StatusServiceUnavailable, "OPERATIONAL_ISSUES_UNAVAILABLE")
			return
		}
		path := strings.TrimSuffix(r.URL.Path, "/")
		switch {
		case path == "/api/operational-issues" && r.Method == http.MethodGet:
			handleOperationalIssueList(s, w, r)
		case path == "/api/operational-issues/stale-preview" && r.Method == http.MethodGet:
			handleOperationalIssueStalePreview(s, w, r)
		case path == "/api/operational-issues/archive-stale" && r.Method == http.MethodPost:
			if !requireOperationalIssueSameOrigin(w, r) {
				return
			}
			handleOperationalIssueArchiveStale(s, w, r)
		case strings.HasPrefix(path, "/api/operational-issues/") && r.Method == http.MethodPost:
			if !requireOperationalIssueSameOrigin(w, r) {
				return
			}
			handleOperationalIssueMutation(s, w, r)
		default:
			writeOperationalIssueError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		}
	}
}

func handleOperationalIssueList(s *Server, w http.ResponseWriter, r *http.Request) {
	page, err := planner.ListOperationalIssues(s.PlannerDB, planner.OperationalIssueListFilter{
		Status:   r.URL.Query().Get("status"),
		Kind:     r.URL.Query().Get("kind"),
		Source:   r.URL.Query().Get("source"),
		Severity: r.URL.Query().Get("severity"),
		Limit:    parseBoundedOperationalIssueInt(r.URL.Query().Get("limit"), 50, 200),
		Offset:   parseBoundedOperationalIssueInt(r.URL.Query().Get("offset"), 0, 100000),
	})
	if err != nil {
		logOperationalIssueHandlerError(s, "Failed to list operational issues", "OPERATIONAL_ISSUE_LIST_FAILED")
		writeOperationalIssueError(w, http.StatusInternalServerError, "OPERATIONAL_ISSUE_LIST_FAILED")
		return
	}
	items := make([]operationalIssueAPIItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, operationalIssueAPIItem{
			ID:                       planner.OperationalIssuePublicID(item.Fingerprint),
			OperationalIssueListItem: item,
		})
	}
	writeJSON(w, map[string]any{
		"items":         items,
		"total":         page.Total,
		"limit":         page.Limit,
		"offset":        page.Offset,
		"status_counts": page.StatusCounts,
	})
}

func handleOperationalIssueStalePreview(s *Server, w http.ResponseWriter, r *http.Request) {
	items, total, err := planner.PreviewStaleOperationalIssues(s.PlannerDB, time.Now(), 100)
	if err != nil {
		logOperationalIssueHandlerError(s, "Failed to preview stale operational issues", "OPERATIONAL_ISSUE_PREVIEW_FAILED")
		writeOperationalIssueError(w, http.StatusInternalServerError, "OPERATIONAL_ISSUE_PREVIEW_FAILED")
		return
	}
	apiItems := make([]operationalIssueAPIItem, 0, len(items))
	for _, item := range items {
		apiItems = append(apiItems, operationalIssueAPIItem{
			ID:                       planner.OperationalIssuePublicID(item.Fingerprint),
			OperationalIssueListItem: item,
		})
	}
	writeJSON(w, map[string]any{"count": total, "items": apiItems})
}

func handleOperationalIssueArchiveStale(s *Server, w http.ResponseWriter, r *http.Request) {
	var req operationalIssueMutationRequest
	if err := decodeStrictOperationalIssueJSON(w, r, &req); err != nil {
		return
	}
	if req.Confirm != operationalIssueArchiveConfirm {
		writeOperationalIssueError(w, http.StatusBadRequest, "CONFIRMATION_REQUIRED")
		return
	}
	archived, err := planner.ArchiveStaleOperationalIssues(s.PlannerDB, time.Now())
	if err != nil {
		logOperationalIssueHandlerError(s, "Failed to archive stale operational issues", "OPERATIONAL_ISSUE_ARCHIVE_FAILED")
		writeOperationalIssueError(w, http.StatusInternalServerError, "OPERATIONAL_ISSUE_ARCHIVE_FAILED")
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "archived": archived})
}

func handleOperationalIssueMutation(s *Server, w http.ResponseWriter, r *http.Request) {
	escapedPath := strings.TrimSuffix(r.URL.EscapedPath(), "/")
	escapedSuffix := strings.TrimPrefix(escapedPath, "/api/operational-issues/")
	separator := strings.LastIndexByte(escapedSuffix, '/')
	if separator <= 0 || separator == len(escapedSuffix)-1 {
		writeOperationalIssueError(w, http.StatusNotFound, "OPERATIONAL_ISSUE_NOT_FOUND")
		return
	}
	encodedID, err := url.PathUnescape(escapedSuffix[:separator])
	if err != nil {
		writeOperationalIssueError(w, http.StatusBadRequest, "OPERATIONAL_ISSUE_ID_INVALID")
		return
	}
	publicID, err := decodeOperationalIssueID(encodedID)
	if err != nil {
		writeOperationalIssueError(w, http.StatusBadRequest, "OPERATIONAL_ISSUE_ID_INVALID")
		return
	}
	fingerprint, found, err := planner.FindOperationalIssueFingerprintByPublicID(s.PlannerDB, publicID)
	if err != nil {
		logOperationalIssueHandlerError(s, "Failed to resolve operational issue identifier", "OPERATIONAL_ISSUE_LOOKUP_FAILED")
		writeOperationalIssueError(w, http.StatusInternalServerError, "OPERATIONAL_ISSUE_LOOKUP_FAILED")
		return
	}
	if !found {
		writeOperationalIssueError(w, http.StatusNotFound, "OPERATIONAL_ISSUE_NOT_FOUND")
		return
	}
	action := strings.ToLower(strings.TrimSpace(escapedSuffix[separator+1:]))
	var req operationalIssueMutationRequest
	if err := decodeStrictOperationalIssueJSON(w, r, &req); err != nil {
		return
	}

	var changed bool
	switch action {
	case "archive":
		changed, err = planner.ArchiveOperationalIssue(s.PlannerDB, fingerprint, req.Reason, time.Now())
	case "resolve":
		if strings.TrimSpace(req.Resolution) == "" {
			writeOperationalIssueError(w, http.StatusBadRequest, "RESOLUTION_REQUIRED")
			return
		}
		changed, err = planner.ResolveOperationalIssue(s.PlannerDB, fingerprint, req.Resolution, time.Now())
	default:
		writeOperationalIssueError(w, http.StatusNotFound, "OPERATIONAL_ISSUE_ACTION_INVALID")
		return
	}
	if err != nil {
		logOperationalIssueHandlerError(s, "Failed to mutate operational issue", "OPERATIONAL_ISSUE_UPDATE_FAILED", "action", action)
		writeOperationalIssueError(w, http.StatusInternalServerError, "OPERATIONAL_ISSUE_UPDATE_FAILED")
		return
	}
	if !changed {
		writeOperationalIssueError(w, http.StatusConflict, "OPERATIONAL_ISSUE_NOT_ACTIVE")
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "action": action})
}

func decodeStrictOperationalIssueJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type"))), "application/json") {
		writeOperationalIssueError(w, http.StatusUnsupportedMediaType, "JSON_REQUIRED")
		return errors.New("application/json required")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeOperationalIssueError(w, http.StatusBadRequest, "INVALID_JSON")
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeOperationalIssueError(w, http.StatusBadRequest, "INVALID_JSON")
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func requireOperationalIssueSameOrigin(w http.ResponseWriter, r *http.Request) bool {
	if checkCSRFOrigin(r) {
		return true
	}
	writeOperationalIssueError(w, http.StatusForbidden, "CSRF_CHECK_FAILED")
	return false
}

func decodeOperationalIssueID(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return "", errors.New("invalid operational issue id")
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return "", errors.New("invalid operational issue id")
		}
	}
	return value, nil
}

func parseBoundedOperationalIssueInt(value string, fallback, maximum int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 || parsed > maximum {
		return fallback
	}
	return parsed
}

func writeOperationalIssueError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error_code": code})
}

func logOperationalIssueHandlerError(s *Server, message, code string, attrs ...any) {
	if s == nil || s.Logger == nil {
		return
	}
	args := []any{"error_code", code}
	args = append(args, attrs...)
	s.Logger.Error(message, args...)
}
