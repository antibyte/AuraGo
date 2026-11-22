package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/desktop"
)

// chibiJobTimeout bounds one import or export job.
const chibiJobTimeout = 30 * time.Minute

// chibiWorkspace implements chibisensei.Workspace over the desktop service. Every path goes through
// the service, which keeps workspace containment, the no-symlink rule and the size budgets.
type chibiWorkspace struct{ svc *desktop.Service }

// ReadFile reads a workspace file. A file over maxBytes is ErrFileTooLarge; a missing file wraps
// os.ErrNotExist.
func (w chibiWorkspace) ReadFile(ctx context.Context, p string, maxBytes int64) ([]byte, error) {
	data, _, err := w.svc.ReadFileBytes(ctx, p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		if strings.Contains(err.Error(), "exceeds max size") {
			return nil, chibisensei.ErrFileTooLarge
		}
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, chibisensei.ErrFileTooLarge
	}
	return data, nil
}

// WriteFile streams r into a workspace file. Without overwrite it only creates: an existing target is
// ErrFileExists.
func (w chibiWorkspace) WriteFile(ctx context.Context, p string, r io.Reader, maxBytes int64, overwrite bool) error {
	var precondition desktop.FileWritePrecondition
	if overwrite {
		precondition = func(desktop.FileWriteState) error { return nil }
	}
	_, err := w.svc.WriteFileStreamConditional(ctx, p, r, maxBytes, desktop.SourceUser, precondition)
	var conflict *desktop.PathConflict
	if errors.As(err, &conflict) {
		return chibisensei.ErrFileExists
	}
	return err
}

// chibiPreloaded hands a file the handler already read to the job once, so a job does not read a big
// file twice.
type chibiPreloaded struct {
	chibisensei.Workspace
	path string
	data []byte
}

func (p *chibiPreloaded) ReadFile(ctx context.Context, path string, maxBytes int64) ([]byte, error) {
	if p.data != nil && path == p.path {
		data := p.data
		p.data = nil
		return data, nil
	}
	return p.Workspace.ReadFile(ctx, path, maxBytes)
}

// chibiWorkspaceError answers a failed workspace read: 404 for a missing file, 413 for a big one, and
// 422 for everything else (a path the desktop refuses). The cause is logged, never sent.
func chibiWorkspaceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		writeChibiError(w, http.StatusNotFound, "not_found", "The file was not found.", nil)
	case errors.Is(err, chibisensei.ErrFileTooLarge):
		chibiDomainError(w, err)
	default:
		slog.Warn("[ChibiSensei] Workspace read failed", "error", err)
		chibiValidation(w, "workspace_path", "import.unreadable_file", "The file cannot be read.")
	}
}

// chibiImportFatal are the issues that refuse a whole import, as opposed to the row problems of one line.
var chibiImportFatal = map[string]bool{
	"chibi_sensei.import.unsupported_file": true, "chibi_sensei.import.text_needs_free": true,
	"chibi_sensei.import.no_mapping": true, "chibi_sensei.import.unknown_column": true,
	"chibi_sensei.import.bad_mapping": true, "chibi_sensei.import.empty": true, "chibi_sensei.import.bad_csv": true,
}

// chibiFatalImportIssues keeps the whole-file issues (they carry no line number).
func chibiFatalImportIssues(issues []chibisensei.Issue) []chibisensei.Issue {
	var out []chibisensei.Issue
	for _, is := range issues {
		if is.Line == 0 && chibiImportFatal[is.MessageKey] {
			out = append(out, is)
		}
	}
	return out
}

type chibiImportRequest struct {
	WorkspacePath string               `json:"workspace_path"`
	Format        string               `json:"format"`
	Mapping       *chibisensei.Mapping `json:"mapping"`
}

// chibiReadImportFile resolves the format and reads the file; it answers the error itself.
func chibiReadImportFile(w http.ResponseWriter, r *http.Request, acc *chibiSenseiAccess, id string, req chibiImportRequest) (format string, recipe string, data []byte, ok bool) {
	req.WorkspacePath = strings.TrimSpace(req.WorkspacePath)
	if req.WorkspacePath == "" {
		chibiValidation(w, "workspace_path", "import.no_path", "Name a file in the workspace.")
		return "", "", nil, false
	}
	format, err := chibisensei.DetectFormat(req.WorkspacePath, req.Format)
	if err != nil {
		chibiDomainError(w, err)
		return "", "", nil, false
	}
	p, err := acc.rt.store.GetProject(r.Context(), id)
	if err != nil {
		chibiDomainError(w, err)
		return "", "", nil, false
	}
	data, err = chibiWorkspace{acc.svc}.ReadFile(r.Context(), req.WorkspacePath, int64(acc.cfg.ChibiSensei.Import.MaxFileMB)<<20)
	if err != nil {
		chibiWorkspaceError(w, err)
		return "", "", nil, false
	}
	return format, p.Recipe, data, true
}

// handleChibiImportPreview parses the first rows of a workspace file. A mapping problem is part of the
// answer (the UI shows the columns to choose from); only an unsupported file is refused.
func handleChibiImportPreview(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
	if !ok {
		return
	}
	var req chibiImportRequest
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	format, recipe, data, ok := chibiReadImportFile(w, r, acc, id, req)
	if !ok {
		return
	}
	pv, err := chibisensei.PreviewImport(data, format, req.Mapping, recipe)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, pv)
}

// handleChibiImport starts the import job. The format must be named and the mapping given; a file the
// import cannot read at all is refused here, before a job exists.
func handleChibiImport(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	var req chibiImportRequest
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	if req.Format == "" || req.Format == "auto" {
		chibiValidation(w, "format", "import.format_required", "Choose the file format.")
		return
	}
	if req.Mapping == nil {
		chibiValidation(w, "mapping", "import.no_mapping", "Choose how the columns map to rows.")
		return
	}
	if chibiStorageFull(w, acc) {
		return
	}
	format, recipe, data, ok := chibiReadImportFile(w, r, acc, id, req)
	if !ok {
		return
	}
	pv, err := chibisensei.PreviewImport(data, format, req.Mapping, recipe)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	if fatal := chibiFatalImportIssues(pv.Issues); len(fatal) > 0 {
		chibiDomainError(w, &chibisensei.ValidationError{Issues: fatal})
		return
	}
	params := chibisensei.ImportParams{WorkspacePath: strings.TrimSpace(req.WorkspacePath), Format: format, Mapping: req.Mapping}
	ws := &chibiPreloaded{Workspace: chibiWorkspace{acc.svc}, path: params.WorkspacePath, data: data}
	store := acc.rt.store
	maxFile := int64(acc.cfg.ChibiSensei.Import.MaxFileMB) << 20
	importMax, datasetMax := acc.cfg.ChibiSensei.Import.MaxRows, acc.cfg.ChibiSensei.Dataset.MaxRows
	ctx, release, err := s.beginDesktopBackgroundRun(chibiJobTimeout)
	if err != nil {
		writeChibiError(w, http.StatusForbidden, "desktop_readonly", "The desktop does not allow this action.", nil)
		return
	}
	job, err := acc.rt.jobs.Start(ctx, release, id, "import_file", params, 0,
		func(jctx context.Context, report func(chibisensei.Progress)) (chibisensei.JobResult, error) {
			return chibisensei.RunImport(jctx, store, ws, id, params, maxFile, importMax, datasetMax, report)
		})
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

// handleChibiExport starts the export job. The UI confirms an overwrite itself and then sends it.
func handleChibiExport(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	var params chibisensei.ExportParams
	if !chibiDecode(w, r, &params, chibiJSONLimit) {
		return
	}
	params.WorkspacePath = strings.TrimSpace(params.WorkspacePath)
	if params.Which == "" {
		params.Which = "all"
	}
	if !strings.HasSuffix(strings.ToLower(params.WorkspacePath), ".jsonl") {
		chibiValidation(w, "workspace_path", "export.bad_extension", "The export file must end with .jsonl.")
		return
	}
	switch params.Which {
	case "all", "train", "val", "eval":
	default:
		chibiValidation(w, "which", "export.bad_which", "Unknown export selection.")
		return
	}
	if _, err := acc.rt.store.GetProject(r.Context(), id); err != nil {
		chibiDomainError(w, err)
		return
	}
	if chibiStorageFull(w, acc) {
		return
	}
	ws := chibiWorkspace{acc.svc}
	store := acc.rt.store
	ctx, release, err := s.beginDesktopBackgroundRun(chibiJobTimeout)
	if err != nil {
		writeChibiError(w, http.StatusForbidden, "desktop_readonly", "The desktop does not allow this action.", nil)
		return
	}
	job, err := acc.rt.jobs.Start(ctx, release, id, "export", params, 0,
		func(jctx context.Context, report func(chibisensei.Progress)) (chibisensei.JobResult, error) {
			return chibisensei.RunExport(jctx, store, ws, id, params, report)
		})
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusAccepted, map[string]any{"job": job})
}
