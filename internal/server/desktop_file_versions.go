package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"aurago/internal/desktop"
)

type desktopFileConflict struct {
	Source  string `json:"source,omitempty"`
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Status  int    `json:"-"`
}

func (e *desktopFileConflict) Error() string { return e.Code }

// A strong, observed version is required to replace bytes. If-Match: * would
// silently discard an intervening change and is deliberately not supported.
func desktopFilePrecondition(r *http.Request) (desktop.FileWritePrecondition, error) {
	match := strings.TrimSpace(r.Header.Get("If-Match"))
	create := strings.TrimSpace(r.Header.Get("If-None-Match"))
	if match == "" && create == "" {
		return nil, &desktopFileConflict{Code: "file_precondition_required", Status: http.StatusPreconditionRequired}
	}
	if (match != "" && create != "") || (create != "" && create != "*") || (match != "" && (len(match) != 66 || match[0] != '"' || match[65] != '"' || strings.ContainsAny(match[1:65], "\" ,"))) {
		return nil, &desktopFileConflict{Code: "file_precondition_invalid", Status: http.StatusBadRequest}
	}
	return func(state desktop.FileWriteState) error {
		if state.Exists && state.Entry.Type == "directory" {
			return &desktopFileConflict{Code: "directory_conflict", Path: state.Entry.Path, Status: http.StatusConflict}
		}
		version := ""
		if state.Exists {
			version = desktop.NoteVersion(state.Data)
		}
		if create == "*" && !state.Exists {
			return nil
		}
		if match != "" && state.Exists && match == version {
			return nil
		}
		return &desktopFileConflict{Code: "file_conflict", Path: state.Entry.Path, Version: version, Status: http.StatusPreconditionFailed}
	}, nil
}

func writeDesktopFileError(w http.ResponseWriter, err error) {
	var conflict *desktopFileConflict
	if !errors.As(err, &conflict) {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(conflict.Status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": conflict.Code, "code": conflict.Code, "conflict": conflict})
}
