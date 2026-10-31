package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/kaggle"
	"aurago/internal/notebooks"
	"aurago/internal/security"
	"aurago/internal/tools"
)

type kaggleServices struct {
	*tools.KaggleBackend
	unregister func()
}

func (s *Server) kaggleSnapshot() kaggle.Snapshot {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return kaggle.Snapshot{}
	}
	c := cfg.Kaggle
	if s.Vault != nil {
		c.Token, _ = s.Vault.ReadSecret("kaggle_api_token")
		c.LegacyKey, _ = s.Vault.ReadSecret("kaggle_legacy_key")
	}
	p := tools.RuntimePermissionsFromConfig(cfg)
	return kaggle.Snapshot{Config: c, Access: kaggle.Access{Network: p.AllowNetworkRequests, Write: p.AllowFilesystemWrite && !cfg.VirtualDesktop.ReadOnly, Python: p.AllowPython, Docker: p.DockerEnabled && !p.DockerReadOnly, Admin: true}}
}
func (s *Server) ensureKaggleServices() (*kaggleServices, error) {
	s.kaggleMu.Lock()
	defer s.kaggleMu.Unlock()
	if s.kaggleServices != nil {
		return s.kaggleServices, nil
	}
	ctx := s.integrationCtx
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return nil, errors.New("configuration unavailable")
	}
	var secrets kaggle.SecretStore
	if s.Vault != nil {
		secrets = s.Vault
	}
	p, err := kaggle.NewService(ctx, cfg.Directories.DataDir, s.kaggleSnapshot, secrets)
	if err != nil {
		return nil, err
	}
	p.BeginJob = s.beginDesktopRun
	d := notebooks.NewDocuments(p)
	k := notebooks.NewKernels(ctx, p, d, notebooks.NewDocker(cfg.Docker.Host, p.Root))
	b := &tools.KaggleBackend{Platform: p, Documents: d, Kernels: k}
	s.kaggleServices = &kaggleServices{b, tools.RegisterKaggleBackend(cfg.Directories.DataDir, b)}
	return s.kaggleServices, nil
}
func (s *Server) closeKaggleServices() {
	s.kaggleMu.Lock()
	b := s.kaggleServices
	s.kaggleServices = nil
	s.kaggleMu.Unlock()
	if b != nil {
		b.unregister()
		b.Kernels.Close()
		b.Platform.Close()
	}
}
func (s *Server) registerKaggleRoutes(mux *http.ServeMux) {
	mux.Handle("/api/kaggle/", http.HandlerFunc(s.handleKaggle))
}
func kaggleJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func kaggleFailure(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, notebooks.ErrConflict) || errors.Is(err, kaggle.ErrFileConflict) {
		status = http.StatusConflict
	}
	kaggleJSON(w, status, map[string]string{"error": kaggle.PublicError(err)})
}
func decodeKaggle(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		kaggleJSON(w, 400, map[string]string{"error": "invalid JSON request"})
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		kaggleJSON(w, 400, map[string]string{"error": "trailing JSON data"})
		return false
	}
	return true
}

func (s *Server) handleKaggle(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/kaggle/")
	if path == "credentials" {
		if _, bearer := bearerCredential(r.Header.Get("Authorization")); !bearer && !checkCSRFOriginWithPolicy(r, true) {
			kaggleJSON(w, 403, map[string]string{"error": "same-origin request required"})
			return
		}
		requireAdmin(s, http.HandlerFunc(s.handleKaggleCredentials)).ServeHTTP(w, r)
		return
	}
	scope := desktopMethodScope(r.Method)
	if path == "operations" || path == "runtime" || path == "test" {
		scope = desktopScopeRead
	}
	// RPC bodies select semantics. A write-only token may enter the dispatcher,
	// but the selected operation still checks its exact scope below.
	if token, ok := bearerCredential(r.Header.Get("Authorization")); ok && (path == "operations" || path == "runtime") && desktopTokenHasScope(s, token, desktopScopeWrite) {
		scope = desktopScopeWrite
	}
	if !authenticateDesktopPermission(s, w, r, scope) {
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		if _, bearer := bearerCredential(r.Header.Get("Authorization")); !bearer && !checkCSRFOriginWithPolicy(r, true) {
			kaggleJSON(w, 403, map[string]string{"error": "same-origin request required"})
			return
		}
	}
	b, err := s.ensureKaggleServices()
	if err != nil {
		kaggleJSON(w, 503, map[string]string{"error": "Kaggle service unavailable"})
		return
	}
	if path == "events" && r.Method == "GET" {
		s.kaggleEvents(w, r, b)
		return
	}
	var result any
	switch {
	case path == "status" && r.Method == "GET":
		status := b.Platform.Status()
		status["can_resolve_unknown"] = s.kaggleCanResolveUnknown(r)
		result = status
	case path == "capabilities" && r.Method == "GET":
		result = map[string]any{"operations": kaggle.Catalog(), "definitions": kaggle.Definitions(), "remote_cell_execution": false, "benchmark_delete": false}
	case path == "test" && r.Method == "POST":
		result, err = b.Platform.Test(r.Context())
	case path == "quotas" && r.Method == "GET":
		results := map[string]any{}
		for _, entry := range [][2]string{{"notebooks", "get_accelerator_quota_statistics"}, {"benchmarks", "get_benchmark_task_quota"}} {
			v, e := b.Platform.Execute(r.Context(), kaggle.Request{Domain: entry[0], Operation: entry[1], Parameters: json.RawMessage(`{}`)}, s.kaggleSnapshot().Access)
			if e != nil {
				results[entry[0]] = map[string]string{"error": e.Error()}
			} else {
				results[entry[0]] = v.Data
			}
		}
		result = results
	case path == "jobs" && r.Method == "GET":
		result, err = b.Platform.Store.List(r.Context())
	case strings.HasPrefix(path, "jobs/") && strings.HasSuffix(path, "/resolve-unknown") && r.Method == "POST":
		if !checkDesktopOperation(s, w, r, desktopStop) {
			return
		}
		requireAdmin(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.resolveKaggleUnknownJob(w, r, b, strings.TrimSuffix(strings.TrimPrefix(path, "jobs/"), "/resolve-unknown"))
		})).ServeHTTP(w, r)
		return
	case strings.HasPrefix(path, "jobs/") && strings.HasSuffix(path, "/cancel") && r.Method == "POST":
		err = b.Platform.Cancel(r.Context(), strings.TrimSuffix(strings.TrimPrefix(path, "jobs/"), "/cancel"))
		result = map[string]bool{"requested": err == nil}
	case path == "projects" && r.Method == "GET":
		result, err = b.Platform.Store.Projects(r.Context())
	case path == "projects" && r.Method == "POST":
		var req struct {
			Title string `json:"title"`
		}
		if !decodeKaggle(w, r, &req) || !checkDesktopOperation(s, w, r, desktopWrite) {
			return
		}
		result, err = b.Platform.CreateProject(r.Context(), req.Title)
	case path == "operations" && r.Method == "POST":
		var req kaggle.Request
		if !decodeKaggle(w, r, &req) {
			return
		}
		op, e := kaggle.Lookup(req.Domain, req.Operation)
		if e != nil {
			kaggleFailure(w, e)
			return
		}
		operation := desktopWrite
		if op.Grant == "read" {
			operation = desktopRead
		}
		if op.Grant == "cancel" {
			operation = desktopStop
		}
		if !s.authorizeKaggleOperation(w, r, operation) {
			return
		}
		result, err = b.Platform.Execute(r.Context(), req, s.kaggleSnapshot().Access)
	case path == "runtime" && r.Method == "POST":
		var req tools.KaggleRuntimeRequest
		if !decodeKaggle(w, r, &req) {
			return
		}
		operation := desktopWrite
		switch req.Operation {
		case "status", "projects", "files", "read_file", "load_notebook", "jobs", "events", "sessions":
			operation = desktopRead
		case "stop", "interrupt", "cancel_job":
			operation = desktopStop
		}
		if !s.authorizeKaggleOperation(w, r, operation) {
			return
		}
		result, err = b.Runtime(r.Context(), req)
	case strings.HasPrefix(path, "projects/"):
		s.kaggleProject(w, r, b, strings.Split(path, "/"))
		return
	default:
		kaggleJSON(w, 404, map[string]string{"error": "unknown Kaggle endpoint"})
		return
	}
	if err != nil {
		kaggleFailure(w, err)
		return
	}
	kaggleJSON(w, 200, result)
}

func (s *Server) authorizeKaggleOperation(w http.ResponseWriter, r *http.Request, operation desktopOperation) bool {
	scope := desktopScopeWrite
	if operation == desktopRead {
		scope = desktopScopeRead
	}
	return requireDesktopOperation(s, w, r, scope, operation)
}

func (s *Server) handleKaggleCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		kaggleJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Token     *string `json:"token"`
		LegacyKey *string `json:"legacy_key"`
	}
	if !decodeKaggle(w, r, &req) {
		return
	}
	if s.Vault == nil {
		kaggleJSON(w, 503, map[string]string{"error": "Vault unavailable"})
		return
	}
	set := map[string]string{}
	remove := []string{}
	for key, value := range map[string]*string{"kaggle_api_token": req.Token, "kaggle_legacy_key": req.LegacyKey} {
		if value == nil {
			continue
		}
		if len(*value) > 16384 {
			kaggleJSON(w, 400, map[string]string{"error": "credential exceeds limit"})
			return
		}
		security.RegisterSensitive(*value)
		if *value == "" {
			remove = append(remove, key)
		} else {
			set[key] = *value
		}
	}
	if err := s.Vault.WriteSecrets(set, remove); err != nil {
		kaggleJSON(w, 500, map[string]string{"error": "could not save Kaggle credentials"})
		return
	}
	kaggleJSON(w, 200, map[string]bool{"saved": true})
}
func (s *Server) kaggleProject(w http.ResponseWriter, r *http.Request, b *kaggleServices, parts []string) {
	if len(parts) < 3 {
		kaggleJSON(w, 404, map[string]string{"error": "unknown project endpoint"})
		return
	}
	project := parts[1]
	if len(parts) == 3 && parts[2] == "files" && r.Method == "GET" {
		files, err := b.Platform.Files(project)
		if err != nil {
			kaggleFailure(w, err)
			return
		}
		kaggleJSON(w, 200, files)
		return
	}
	if len(parts) != 4 {
		kaggleJSON(w, 404, map[string]string{"error": "unknown project endpoint"})
		return
	}
	name := parts[3]
	if parts[2] == "notebooks" {
		if r.Method == "GET" {
			doc, err := b.Documents.Load(project, name)
			if err != nil {
				kaggleFailure(w, err)
				return
			}
			w.Header().Set("ETag", `"`+doc.Revision+`"`)
			kaggleJSON(w, 200, doc)
			return
		}
		if r.Method == "PUT" && checkDesktopOperation(s, w, r, desktopWrite) {
			var raw json.RawMessage
			if !decodeKaggle(w, r, &raw) {
				return
			}
			expected := strings.Trim(r.Header.Get("If-Match"), `"`)
			if r.Header.Get("If-None-Match") == "*" {
				expected = "*"
			}
			doc, err := b.Documents.Save(r.Context(), project, name, expected, raw)
			if err != nil {
				kaggleFailure(w, err)
				return
			}
			w.Header().Set("ETag", `"`+doc.Revision+`"`)
			kaggleJSON(w, 200, doc)
			return
		}
	}
	if parts[2] == "files" {
		if r.Method == "GET" {
			f, err := b.Platform.OpenFile(project, name)
			if err != nil {
				kaggleFailure(w, err)
				return
			}
			defer f.Close()
			i, err := f.Stat()
			if err != nil {
				kaggleFailure(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
			w.Header().Set("Cache-Control", "no-store")
			http.ServeContent(w, r, name, i.ModTime(), f)
			return
		}
		if r.Method == "PUT" && checkDesktopOperation(s, w, r, desktopWrite) {
			limit := int64(s.kaggleSnapshot().Config.MaxTransferMB) << 20
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			n, err := b.Platform.WriteFileVersion(r.Context(), project, name, "*", r.Body, nil)
			if err != nil {
				kaggleFailure(w, err)
				return
			}
			kaggleJSON(w, 201, map[string]any{"file": name, "bytes": n})
			return
		}
	}
	kaggleJSON(w, 405, map[string]string{"error": "method not allowed"})
}
func (s *Server) kaggleEvents(w http.ResponseWriter, r *http.Request, b *kaggleServices) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		kaggleJSON(w, 500, map[string]string{"error": "streaming unavailable"})
		return
	}
	after, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	if after == 0 {
		after, _ = strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		events, err := b.Platform.Store.Events(r.Context(), after)
		if err != nil {
			return
		}
		for _, e := range events {
			if after > 0 && e.ID > after+1 {
				if _, err = fmt.Fprintf(w, "id: %d\nevent: reset\ndata: {}\n\n", e.ID-1); err != nil {
					return
				}
			}
			if _, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.ID, e.Kind, e.Data); err != nil {
				return
			}
			after = e.ID
		}
		if len(events) > 0 {
			flusher.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if !s.kaggleStreamAuthorized(r) {
				return
			}
			if _, err = fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-tick.C:
		}
	}
}

// Revalidate long-lived streams without trying to write a new HTTP status.
func (s *Server) kaggleStreamAuthorized(r *http.Request) bool {
	if token, bearer := bearerCredential(r.Header.Get("Authorization")); bearer {
		return desktopTokenHasScope(s, token, desktopScopeRead)
	}
	cfg := s.ConfigSnapshot()
	return cfg != nil && (!cfg.Auth.Enabled || IsAuthenticated(r, cfg.Auth.SessionSecret))
}
