package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/cydcreator"
)

const (
	cydSmallBodyLimit = 64 << 10
	cydLayoutBodySlop = 64 << 10 // the {"project": …} wrapper around a layout of MaxLayoutBytes
)

// cydCreatorCoreRoutes are the plan 2 routes (overview "HTTP API").
func cydCreatorCoreRoutes(s *Server) []cydRoute {
	return []cydRoute{
		{Method: http.MethodGet, Pattern: "capabilities", Handler: s.cydCapabilities},
		{Method: http.MethodPost, Pattern: "toolchain/pull", Write: true, Handler: s.cydPullToolchain},
		{Method: http.MethodGet, Pattern: "projects", Handler: s.cydListProjects},
		{Method: http.MethodPost, Pattern: "projects", Write: true, Handler: s.cydCreateProject},
		{Method: http.MethodGet, Pattern: "projects/{id}", Handler: s.cydGetProject},
		{Method: http.MethodPatch, Pattern: "projects/{id}", Write: true, Handler: s.cydRenameProject},
		{Method: http.MethodDelete, Pattern: "projects/{id}", Write: true, Handler: s.cydDeleteProject},
		{Method: http.MethodGet, Pattern: "projects/{id}/layout", Handler: s.cydGetLayout},
		{Method: http.MethodPut, Pattern: "projects/{id}/layout", Write: true, Handler: s.cydPutLayout},
		{Method: http.MethodGet, Pattern: "projects/{id}/files", Handler: s.cydListFiles},
		{Method: http.MethodGet, Pattern: "projects/{id}/files/{path...}", Handler: s.cydReadFile},
		{Method: http.MethodPut, Pattern: "projects/{id}/files/{path...}", Write: true, Handler: s.cydWriteFile},
		{Method: http.MethodDelete, Pattern: "projects/{id}/files/{path...}", Write: true, Handler: s.cydDeleteFile},
		{Method: http.MethodPost, Pattern: "projects/{id}/assets", Write: true, Handler: s.cydUploadAsset},
		{Method: http.MethodGet, Pattern: "projects/{id}/assets/{asset}", Handler: s.cydGetAsset},
		{Method: http.MethodDelete, Pattern: "projects/{id}/assets/{asset}", Write: true, Handler: s.cydDeleteAsset},
		{Method: http.MethodPost, Pattern: "projects/{id}/builds", Write: true, Handler: s.cydStartBuild},
		{Method: http.MethodGet, Pattern: "projects/{id}/builds", Handler: s.cydListBuilds},
		{Method: http.MethodGet, Pattern: "builds/{id}", Handler: s.cydGetBuild},
		{Method: http.MethodPost, Pattern: "builds/{id}/cancel", Stop: true, Handler: s.cydCancelBuild},
		{Method: http.MethodGet, Pattern: "builds/{id}/log", Handler: s.cydBuildLog},
		{Method: http.MethodGet, Pattern: "builds/{id}/artifacts/{name}", Handler: s.cydArtifact},
		{Method: http.MethodPost, Pattern: "builds/{id}/decode-backtrace", Handler: s.cydDecodeBacktrace},
		{Method: http.MethodGet, Pattern: "projects/{id}/events", Handler: s.cydEvents},
	}
}

// cydDecodeJSON decodes a bounded JSON body with unknown fields refused. An
// empty body decodes to the zero value.
func cydDecodeJSON(w http.ResponseWriter, r *http.Request, limit int64, dst any) bool {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			cydWriteError(w, http.StatusRequestEntityTooLarge, cydcreator.CodeBadRequest, fmt.Sprintf("the request body is larger than %d bytes", limit), nil)
			return false
		}
		cydWriteError(w, http.StatusBadRequest, cydcreator.CodeBadRequest, "the request body could not be read", nil)
		return false
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return true
	}
	if cydDecodeStrict(data, dst) != nil {
		cydWriteError(w, http.StatusBadRequest, cydcreator.CodeBadRequest,
			"the request body must be one JSON object with the fields of this request", nil)
		return false
	}
	return true
}

// cydDecodeStrict decodes exactly one JSON value into dst: unknown fields
// and anything after the value are refused. The error is for the caller's
// decision only; answers never echo decoder text.
func cydDecodeStrict(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing data after the JSON value")
	}
	return nil
}

// cydIfMatch returns the If-Match value without quotes or a weak prefix,
// lower-cased (SHA-256 values are compared as lower-case hex).
func cydIfMatch(r *http.Request) (string, bool) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	raw = strings.TrimPrefix(raw, "W/")
	raw = strings.ToLower(strings.TrimSpace(strings.Trim(raw, `"`)))
	return raw, raw != ""
}

// cydCapabilities overlays the service answer (always enabled, writable)
// with the configuration and the caller: readonly when the desktop is
// read-only or the bearer token cannot write (desktop:admin).
func (s *Server) cydCapabilities(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	caps := s.CYDCreator.Capabilities(r.Context())
	cfg := s.ConfigSnapshot()
	caps.Enabled = cfg != nil && cfg.CYDCreator.Enabled
	caps.ReadOnly = cfg == nil || cfg.VirtualDesktop.ReadOnly
	if raw, isBearer := bearerCredential(r.Header.Get("Authorization")); isBearer && !desktopTokenHasScope(s, raw, desktopScopeAdmin) {
		caps.ReadOnly = true
	}
	cydWriteJSON(w, http.StatusOK, caps)
}

func (s *Server) cydPullToolchain(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	var body struct{}
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &body) {
		return
	}
	if err := s.CYDCreator.PullToolchain(r.Context()); err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusAccepted, map[string]any{"toolchain": s.CYDCreator.Toolchain(r.Context())})
}

func (s *Server) cydListProjects(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	list, err := s.CYDCreator.ListProjects(r.Context())
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"projects": list})
}

func (s *Server) cydCreateProject(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	var in cydcreator.CreateProjectInput
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &in) {
		return
	}
	project, err := s.CYDCreator.CreateProject(r.Context(), in)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusCreated, map[string]any{"project": project})
}

func (s *Server) cydGetProject(w http.ResponseWriter, r *http.Request, params map[string]string) {
	project, err := s.CYDCreator.GetProject(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"project": project})
}

func (s *Server) cydRenameProject(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var body struct {
		Name *string `json:"name"`
	}
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &body) {
		return
	}
	if body.Name == nil {
		cydWriteError(w, http.StatusBadRequest, cydcreator.CodeBadRequest, "name is required", nil)
		return
	}
	project, err := s.CYDCreator.RenameProject(r.Context(), params["id"], *body.Name)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"project": project})
}

func (s *Server) cydDeleteProject(w http.ResponseWriter, r *http.Request, params map[string]string) {
	if err := s.CYDCreator.DeleteProject(r.Context(), params["id"]); err != nil {
		s.cydError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) cydGetLayout(w http.ResponseWriter, r *http.Request, params map[string]string) {
	project, version, issues, err := s.CYDCreator.GetLayout(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"version": version, "project": project, "issues": issues})
}

func (s *Server) cydPutLayout(w http.ResponseWriter, r *http.Request, params map[string]string) {
	raw, ok := cydIfMatch(r)
	ifVersion, err := strconv.ParseInt(raw, 10, 64)
	if !ok || err != nil || ifVersion < 1 {
		cydWriteError(w, http.StatusBadRequest, cydcreator.CodeBadRequest, "If-Match with the layout version is required", nil)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, cydcreator.MaxLayoutBytes+cydLayoutBodySlop))
	if err != nil {
		cydWriteError(w, http.StatusUnprocessableEntity, cydcreator.CodeInvalidLayout, "the layout is too large or could not be read",
			map[string]any{"limit": cydcreator.MaxLayoutBytes})
		return
	}
	var body struct {
		Project json.RawMessage `json:"project"`
	}
	if err := cydDecodeStrict(data, &body); err != nil || len(body.Project) == 0 {
		cydWriteError(w, http.StatusUnprocessableEntity, cydcreator.CodeInvalidLayout, `the body must be {"project": <layout>}`, nil)
		return
	}
	project, err := cydcreator.ParseProject(body.Project)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	version, issues, err := s.CYDCreator.PutLayout(r.Context(), params["id"], project, ifVersion, cydcreator.OriginUser)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"version": version, "issues": issues})
}

func (s *Server) cydListFiles(w http.ResponseWriter, r *http.Request, params map[string]string) {
	files, err := s.CYDCreator.ListFiles(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (s *Server) cydReadFile(w http.ResponseWriter, r *http.Request, params map[string]string) {
	data, sha, err := s.CYDCreator.ReadFile(r.Context(), params["id"], params["path"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-CYD-SHA256", sha)
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func (s *Server) cydWriteFile(w http.ResponseWriter, r *http.Request, params map[string]string) {
	ifSHA, ok := cydIfMatch(r)
	if !ok {
		cydWriteError(w, http.StatusBadRequest, cydcreator.CodeBadRequest, `If-Match with the file's SHA-256 (or "*" to create) is required`, nil)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, cydcreator.MaxAppFileBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			cydWriteError(w, http.StatusRequestEntityTooLarge, cydcreator.CodeFileLimit,
				fmt.Sprintf("a source file may have at most %d bytes", cydcreator.MaxAppFileBytes),
				map[string]any{"limit": "file_bytes", "max": cydcreator.MaxAppFileBytes, "path": params["path"]})
			return
		}
		cydWriteError(w, http.StatusBadRequest, cydcreator.CodeBadRequest, "the file content could not be read", nil)
		return
	}
	sha, err := s.CYDCreator.WriteFile(r.Context(), params["id"], params["path"], data, ifSHA, cydcreator.OriginUser)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"sha256": sha})
}

func (s *Server) cydDeleteFile(w http.ResponseWriter, r *http.Request, params map[string]string) {
	ifSHA, ok := cydIfMatch(r)
	if !ok {
		cydWriteError(w, http.StatusBadRequest, cydcreator.CodeBadRequest, "If-Match with the file's SHA-256 is required", nil)
		return
	}
	if err := s.CYDCreator.DeleteFile(r.Context(), params["id"], params["path"], ifSHA, cydcreator.OriginUser); err != nil {
		s.cydError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) cydUploadAsset(w http.ResponseWriter, r *http.Request, params map[string]string) {
	tooLarge := func() {
		cydWriteError(w, http.StatusRequestEntityTooLarge, cydcreator.CodeAssetTooLarge,
			fmt.Sprintf("an image file may have at most %d bytes", cydcreator.MaxAssetUploadBytes),
			map[string]any{"reason": "file_size", "max": cydcreator.MaxAssetUploadBytes})
	}
	badRequest := func() {
		cydWriteError(w, http.StatusBadRequest, cydcreator.CodeBadRequest, `send one PNG or JPEG image as multipart field "file"`, nil)
	}
	r.Body = http.MaxBytesReader(w, r.Body, cydcreator.MaxAssetUploadBytes+cydSmallBodyLimit)
	reader, err := r.MultipartReader()
	if err != nil {
		badRequest()
		return
	}
	for {
		part, err := reader.NextPart()
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			tooLarge()
			return
		case err != nil:
			badRequest()
			return
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, cydcreator.MaxAssetUploadBytes+1))
		_ = part.Close()
		switch {
		case errors.As(err, &maxErr), err == nil && len(data) > cydcreator.MaxAssetUploadBytes:
			tooLarge()
			return
		case err != nil:
			badRequest()
			return
		}
		asset, version, err := s.CYDCreator.PutAsset(r.Context(), params["id"], part.FileName(), data)
		if err != nil {
			s.cydError(w, r, err)
			return
		}
		cydWriteJSON(w, http.StatusCreated, map[string]any{"asset": asset, "version": version})
		return
	}
}

func (s *Server) cydGetAsset(w http.ResponseWriter, r *http.Request, params map[string]string) {
	file, asset, contentType, err := s.CYDCreator.OpenAsset(r.Context(), params["id"], params["asset"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	cydServeReader(w, r, asset.File, file)
}

// cydServeReader serves an opened asset or artefact (Range, HEAD; never a
// directory listing) and always closes it. Callers set Content-Type first,
// so ServeContent never sniffs.
func cydServeReader(w http.ResponseWriter, r *http.Request, name string, file io.ReadSeekCloser) {
	defer file.Close()
	http.ServeContent(w, r, name, time.Time{}, file)
}

func (s *Server) cydDeleteAsset(w http.ResponseWriter, r *http.Request, params map[string]string) {
	version, err := s.CYDCreator.DeleteAsset(r.Context(), params["id"], params["asset"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"version": version})
}
