package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/bookshelf"
	"aurago/internal/bookshelfconvert"
	"aurago/internal/fileutil"
	"aurago/internal/security"
	"aurago/internal/smbclient"
	"aurago/internal/tools"
)

var errBookshelfDisabled = errors.New("bookshelf disabled")
var errBookshelfConverterUnavailable = errors.New("bookshelf converter unavailable")

var bookshelfConvertibleFormats = []string{"cbr", "rtf", "docx", "odt", "lit", "pdb", "lrf"}

func bookshelfConverterCapability(executable string) map[string]any {
	available, reason := bookshelfconvert.Capability(executable)
	formats := []string{}
	if available {
		formats = append(formats, bookshelfConvertibleFormats...)
	}
	return map[string]any{"available": available, "enabled": strings.TrimSpace(executable) != "", "reason": reason, "formats": formats}
}

func handleDesktopBookshelf(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		action := strings.TrimPrefix(r.URL.Path, "/api/desktop/bookshelf")
		if action == "/reader" {
			if !bookshelfAuthorize(s, w, r, false) {
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !s.bookshelfEnabled() {
				writeBookshelfError(w, errBookshelfDisabled)
				return
			}
			s.serveBookshelfReader(w, r)
			return
		}
		if !bookshelfAuthorize(s, w, r, bookshelfRouteWrites(r, action)) {
			return
		}
		if action == "/capabilities" {
			s.serveBookshelfCapabilities(w, r)
			return
		}
		if !s.bookshelfEnabled() {
			writeBookshelfError(w, errBookshelfDisabled)
			return
		}
		ctx, cancel := s.storageContext(r.Context())
		defer cancel()
		r = r.WithContext(ctx)
		runtime, err := s.desktopBookshelf(ctx)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		switch {
		case action == "/sources/browse" && r.Method == http.MethodGet:
			s.serveBookshelfBrowse(w, r, runtime)
		case action == "/sources" && r.Method == http.MethodGet:
			sources, err := runtime.service.ListSources(ctx)
			if err != nil {
				writeBookshelfError(w, err)
				return
			}
			writeJSON(w, map[string]any{"sources": sources})
		case action == "/sources" && r.Method == http.MethodPost:
			s.bookshelfAddSource(w, r, runtime)
		case strings.HasPrefix(action, "/sources/"):
			s.bookshelfSourceRoute(w, r, runtime, strings.TrimPrefix(action, "/sources/"))
		case action == "/jobs" && r.Method == http.MethodGet:
			writeJSON(w, map[string]any{"jobs": s.bookshelfJobs(runtime)})
		case action == "/cache" && r.Method == http.MethodGet:
			info, err := runtime.service.CacheInfo(ctx)
			if err != nil {
				writeBookshelfError(w, err)
				return
			}
			writeJSON(w, info)
		case strings.HasPrefix(action, "/jobs/") && r.Method == http.MethodGet:
			s.bookshelfGetJob(w, runtime, strings.TrimPrefix(action, "/jobs/"))
		case action == "/books" && r.Method == http.MethodGet:
			s.bookshelfListBooks(w, r, runtime)
		case action == "/books/resolve" && r.Method == http.MethodGet:
			s.bookshelfResolveGet(w, r, runtime)
		case action == "/books/resolve" && r.Method == http.MethodPost:
			s.bookshelfResolvePost(w, r, runtime)
		case strings.HasPrefix(action, "/books/"):
			s.bookshelfBookRoute(w, r, runtime, strings.TrimPrefix(action, "/books/"))
		case action == "/collections" && r.Method == http.MethodGet:
			s.bookshelfCollections(w, r, runtime)
		case action == "/collections" && r.Method == http.MethodPost:
			s.bookshelfCreateCollection(w, r, runtime)
		case strings.HasPrefix(action, "/collections/"):
			s.bookshelfCollectionRoute(w, r, runtime, strings.TrimPrefix(action, "/collections/"))
		case strings.HasPrefix(action, "/annotations/"):
			s.bookshelfAnnotationRoute(w, r, runtime, strings.TrimPrefix(action, "/annotations/"))
		case action == "/imports" && r.Method == http.MethodPost:
			s.bookshelfImport(w, r, runtime)
		default:
			writeBookshelfErrorStatus(w, "not_found", http.StatusNotFound)
		}
	}
}

func bookshelfAuthorize(s *Server, w http.ResponseWriter, r *http.Request, write bool) bool {
	if !authenticateDesktopPermission(s, w, r, desktopScopeAdmin) {
		return false
	}
	operation := desktopRead
	if write {
		operation = desktopWrite
	}
	if !checkDesktopOperation(s, w, r, operation) {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && !checkCSRFOriginWithPolicy(r, true) {
		writeBookshelfErrorStatus(w, "csrf_check_failed", http.StatusForbidden)
		return false
	}
	return true
}

func bookshelfRouteWrites(r *http.Request, action string) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return false
	}
	// Preparing and releasing content leases only read the source or change the
	// private bounded cache. They remain available when the catalog is read-only.
	if r.Method == http.MethodPost && (strings.HasSuffix(action, "/prepare") || strings.HasSuffix(action, "/release")) {
		return false
	}
	return true
}

func (s *Server) serveBookshelfCapabilities(w http.ResponseWriter, r *http.Request) {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		writeBookshelfError(w, errBookshelfDisabled)
		return
	}
	capabilities := map[string]any{
		"enabled":     cfg.VirtualDesktop.Enabled && cfg.VirtualDesktop.BookshelfEnabled,
		"can_read":    cfg.VirtualDesktop.Enabled && cfg.VirtualDesktop.BookshelfEnabled,
		"can_write":   cfg.VirtualDesktop.Enabled && cfg.VirtualDesktop.BookshelfEnabled && !cfg.VirtualDesktop.ReadOnly,
		"readonly":    cfg.VirtualDesktop.ReadOnly,
		"limits":      map[string]int64{"max_files_per_scan": 10_000, "max_book_bytes": bookshelfEffectiveMaxBytes(cfg)},
		"local_roots": []map[string]string{{"id": "workspace", "label": "Desktop workspace", "path": "."}},
		"sources": []map[string]any{
			{"provider": "local", "available": cfg.VirtualDesktop.Enabled},
			{"provider": "webdav", "available": cfg.VirtualDesktop.Enabled && cfg.WebDAV.Enabled && cfg.WebDAV.ShowInFileManager},
			{"provider": "koofr", "available": cfg.VirtualDesktop.Enabled && cfg.Koofr.Enabled && cfg.Koofr.ShowInFileManager},
			{"provider": "smb", "available": cfg.VirtualDesktop.Enabled && cfg.VirtualDesktop.BookshelfEnabled},
		},
		"converter": bookshelfConverterCapability(cfg.VirtualDesktop.BookshelfConverterPath),
	}
	writeJSON(w, capabilities)
}

func (s *Server) serveBookshelfBrowse(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime) {
	provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	mountID := strings.TrimSpace(r.URL.Query().Get("mount_id"))
	location := r.URL.Query().Get("path")
	if provider == "" || location == "" && r.URL.Query().Has("path") {
		// Empty path is the valid root; this branch only documents the distinction.
	}
	switch provider {
	case bookshelf.ProviderLocal:
		if mountID != "workspace" {
			writeBookshelfError(w, bookshelf.ErrUnsupported)
			return
		}
		if _, err := joinBookshelfPath("", location); err != nil {
			writeBookshelfError(w, err)
			return
		}
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		entries, err := svc.ListFiles(r.Context(), location)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		items := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			if entry.Mount != "" || strings.ContainsAny(entry.Name, "/\\") {
				continue
			}
			items = append(items, bookshelfBrowseItem(entry.Name, path.Join(location, entry.Name), entry.Type == "directory", entry.Size, entry.Modified, formatBookshelfPath(entry.Name)))
		}
		writeJSON(w, map[string]any{"items": items})
	case bookshelf.ProviderWebDAV, bookshelf.ProviderKoofr:
		if provider == bookshelf.ProviderWebDAV && mountID != "" {
			writeBookshelfError(w, bookshelf.ErrInvalidPath)
			return
		}
		cfg := s.ConfigSnapshot()
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		cloud := desktopStorageCloud(cfg)
		if provider == bookshelf.ProviderKoofr && mountID == "" {
			if !cfg.Koofr.Enabled || !cfg.Koofr.ShowInFileManager {
				writeBookshelfError(w, tools.ErrStorageAccessDenied)
				return
			}
			ctx, cancel := s.storageContext(r.Context())
			defer cancel()
			mounts, err := cloud.Mounts(ctx)
			if err != nil {
				writeBookshelfError(w, err)
				return
			}
			writeJSON(w, map[string]any{"mounts": mounts})
			return
		}
		d := &desktopStorageSession{s: s, svc: svc, cloud: cloud, maxBytes: int64(svc.Config().MaxFileSizeMB) << 20, ctx: r.Context()}
		loc := tools.StorageLocation{Provider: provider, MountID: mountID, Path: location}
		if err := d.authorize(loc, false); err != nil {
			writeBookshelfError(w, err)
			return
		}
		entries, err := cloud.List(r.Context(), loc)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		items := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			modified, _ := time.Parse(time.RFC3339, entry.Modified)
			items = append(items, bookshelfBrowseItem(entry.Name, entry.Path, entry.Type == "directory", entry.Size, modified, formatBookshelfPath(entry.Name)))
		}
		writeJSON(w, map[string]any{"items": items})
	default:
		writeBookshelfError(w, bookshelf.ErrUnsupported)
	}
}

func bookshelfBrowseItem(name, p string, isDir bool, size int64, modified time.Time, format string) map[string]any {
	return map[string]any{"name": name, "path": p, "is_dir": isDir, "size": size, "modified": modified, "format": format}
}

func (s *Server) bookshelfAddSource(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime) {
	var req struct {
		Source     bookshelf.Source `json:"source"`
		Credential struct {
			Password string `json:"password"`
		} `json:"credential"`
	}
	if err := decodeDesktopJSON(w, r, &req, desktopSmallJSONBodyLimit); err != nil {
		writeBookshelfError(w, err)
		return
	}
	if err := validateBookshelfSource(req.Source, req.Credential.Password, false); err != nil {
		writeBookshelfError(w, err)
		return
	}
	source, err := runtime.service.AddSource(r.Context(), req.Source)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	if source.Provider == bookshelf.ProviderSMB {
		if s.Vault == nil {
			_ = runtime.service.RemoveSource(r.Context(), source.ID)
			writeBookshelfError(w, errors.New("SMB credentials are unavailable"))
			return
		}
		security.RegisterSensitive(req.Credential.Password)
		if err := s.Vault.WriteSecret(bookshelfSMBSecretKey(source.ID), req.Credential.Password); err != nil {
			_ = runtime.service.RemoveSource(r.Context(), source.ID)
			writeBookshelfError(w, err)
			return
		}
	}
	writeJSON(w, map[string]any{"source": source})
}

func validateBookshelfSource(source bookshelf.Source, password string, updating bool) error {
	source.Provider = strings.ToLower(strings.TrimSpace(source.Provider))
	source.Name = strings.TrimSpace(source.Name)
	if source.Name == "" || len(source.Name) > 200 {
		return errors.New("invalid source name")
	}
	switch source.Provider {
	case bookshelf.ProviderLocal:
		if source.MountID != "workspace" || source.Host != "" || source.Share != "" || source.Username != "" || source.Domain != "" {
			return errors.New("local Bookshelf sources must use the Desktop workspace")
		}
	case bookshelf.ProviderWebDAV:
		if source.MountID != "" || source.Host != "" || source.Share != "" || source.Username != "" || source.Domain != "" {
			return errors.New("invalid WebDAV source")
		}
	case bookshelf.ProviderKoofr:
		if source.MountID == "" || source.Host != "" || source.Share != "" || source.Username != "" || source.Domain != "" {
			return errors.New("invalid Koofr source")
		}
	case bookshelf.ProviderSMB:
		if source.Host == "" || source.Share == "" || source.Username == "" || !updating && password == "" {
			return errors.New("SMB host, share, username, and password are required")
		}
		if err := smbclient.ValidateTarget(smbclient.Config{Host: source.Host, Share: source.Share, Username: source.Username, Domain: source.Domain}); err != nil {
			return err
		}
	default:
		return bookshelf.ErrUnsupported
	}
	return nil
}

func (s *Server) bookshelfSourceRoute(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, rest string) {
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeBookshelfErrorStatus(w, "not_found", http.StatusNotFound)
		return
	}
	sourceID, err := url.PathUnescape(parts[0])
	if err != nil {
		writeBookshelfError(w, bookshelf.ErrNotFound)
		return
	}
	if len(parts) == 2 && parts[1] == "browse" && r.Method == http.MethodGet {
		source, err := runtime.service.GetSource(r.Context(), sourceID)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		items, err := (&desktopBookshelfAdapter{s: s, importsDir: runtime.importsDir}).List(r.Context(), source, r.URL.Query().Get("path"))
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			out = append(out, bookshelfBrowseItem(item.Name, item.Path, item.IsDir, item.Size, item.Modified, item.Format))
		}
		writeJSON(w, map[string]any{"items": out})
		return
	}
	if len(parts) == 2 && parts[1] == "scan" && r.Method == http.MethodPost {
		jobID, err := s.startBookshelfScanJob(runtime, sourceID, r)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		writeJSON(w, map[string]any{"job_id": jobID})
		return
	}
	if len(parts) != 1 {
		writeBookshelfErrorStatus(w, "not_found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		s.bookshelfUpdateSource(w, r, runtime, sourceID)
	case http.MethodDelete:
		s.bookshelfRemoveSource(w, r, runtime, sourceID)
	default:
		w.Header().Set("Allow", "GET, PATCH, DELETE")
		writeBookshelfErrorStatus(w, "method_not_allowed", http.StatusMethodNotAllowed)
	}
}

func pruneBookshelfJobs(runtime *desktopBookshelfRuntime) {
	if len(runtime.jobs) < 64 {
		return
	}
	for id, job := range runtime.jobs {
		if job.Status != "running" {
			delete(runtime.jobs, id)
			if len(runtime.jobs) < 48 {
				return
			}
		}
	}
}

func (s *Server) startBookshelfScanJob(runtime *desktopBookshelfRuntime, sourceID string, request *http.Request) (string, error) {
	source, err := runtime.service.GetSource(context.Background(), sourceID)
	if err != nil {
		return "", err
	}
	if source.ID == bookshelfWorkspaceSourceID || source.ID == bookshelfImportsSourceID {
		return "", bookshelf.ErrUnsupported
	}
	jobCtx, cancel := s.storageContext(context.Background())
	s.bookshelfMu.Lock()
	if s.bookshelf != runtime || runtime.closed {
		s.bookshelfMu.Unlock()
		cancel()
		return "", errBookshelfDisabled
	}
	for _, existing := range runtime.jobs {
		if existing.Kind != "conversion" && existing.SourceID == sourceID && existing.Status == "running" {
			id := existing.ID
			s.bookshelfMu.Unlock()
			cancel()
			return id, nil
		}
	}
	active := 0
	for _, existing := range runtime.jobs {
		if existing.Kind != "conversion" && existing.Status == "running" {
			active++
		}
	}
	if active >= 4 {
		s.bookshelfMu.Unlock()
		cancel()
		return "", bookshelf.ErrLimitExceeded
	}
	pruneBookshelfJobs(runtime)
	jobID := randomBookshelfID()
	job := &bookshelfJob{ID: jobID, Kind: "scan", SourceID: sourceID, Status: "running", StartedAt: time.Now().UTC(), cancel: cancel}
	runtime.jobs[jobID] = job
	s.bookshelfMu.Unlock()
	if request != nil {
		request = request.Clone(context.Background())
	}
	go s.runBookshelfScan(runtime, job, request, jobCtx)
	return jobID, nil
}

func (s *Server) runBookshelfScan(runtime *desktopBookshelfRuntime, job *bookshelfJob, request *http.Request, ctx context.Context) {
	if request != nil {
		cancelJob := job.cancel
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !desktopWSAuthorizationValid(s, request, desktopScopeAdmin) {
						cancelJob()
						return
					}
				}
			}
		}()
	}
	result, err := runtime.service.Scan(ctx, job.SourceID)
	s.bookshelfMu.Lock()
	defer s.bookshelfMu.Unlock()
	job.EndedAt = time.Now().UTC()
	job.cancel()
	job.cancel = nil
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			job.Status = "canceled"
		} else {
			job.Status = "failed"
			job.Error = bookshelfErrorCode(err)
		}
		return
	}
	job.Scanned = result.Seen
	job.Imported = result.Added + result.Updated
	job.Skipped = len(result.Issues)
	job.Status = "complete"
	if !result.Complete {
		job.Status = "partial"
	}
}

func (s *Server) bookshelfJobs(runtime *desktopBookshelfRuntime) []*bookshelfJob {
	s.bookshelfMu.Lock()
	defer s.bookshelfMu.Unlock()
	jobs := make([]*bookshelfJob, 0, len(runtime.jobs))
	for _, job := range runtime.jobs {
		copy := *job
		copy.cancel = nil
		jobs = append(jobs, &copy)
	}
	return jobs
}

func (s *Server) bookshelfGetJob(w http.ResponseWriter, runtime *desktopBookshelfRuntime, id string) {
	s.bookshelfMu.Lock()
	job := runtime.jobs[id]
	if job != nil {
		copy := *job
		copy.cancel = nil
		job = &copy
	}
	s.bookshelfMu.Unlock()
	if job == nil {
		writeBookshelfError(w, bookshelf.ErrNotFound)
		return
	}
	writeJSON(w, job)
}

func (s *Server) bookshelfStartConversion(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, bookID string) {
	book, err := runtime.service.GetBook(r.Context(), bookID)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		writeBookshelfError(w, errBookshelfDisabled)
		return
	}
	capability := bookshelfConverterCapability(cfg.VirtualDesktop.BookshelfConverterPath)
	if !capability["available"].(bool) {
		writeBookshelfError(w, errBookshelfConverterUnavailable)
		return
	}
	format := strings.ToLower(book.Format)
	if !bookshelfContainsString(bookshelfConvertibleFormats, format) {
		writeBookshelfError(w, bookshelf.ErrUnsupported)
		return
	}
	jobID := ""
	s.bookshelfMu.Lock()
	if s.bookshelf != runtime || runtime.closed {
		s.bookshelfMu.Unlock()
		writeBookshelfError(w, errBookshelfDisabled)
		return
	}
	for _, existing := range runtime.jobs {
		if existing.SourceID == book.SourceID && existing.Kind == "conversion" && existing.BookID == bookID && existing.Status == "running" {
			jobID = existing.ID
			break
		}
	}
	if jobID == "" {
		pruneBookshelfJobs(runtime)
		jobID = randomBookshelfID()
		ctx, cancel := s.storageContext(context.Background())
		job := &bookshelfJob{ID: jobID, BookID: bookID, SourceID: book.SourceID, Kind: "conversion", Status: "running", StartedAt: time.Now().UTC(), cancel: cancel}
		runtime.jobs[jobID] = job
		request := r.Clone(context.Background())
		go s.runBookshelfConversion(runtime, job, request, ctx)
	}
	s.bookshelfMu.Unlock()
	writeJSON(w, map[string]string{"job_id": jobID})
}

func bookshelfContainsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func (s *Server) runBookshelfConversion(runtime *desktopBookshelfRuntime, job *bookshelfJob, request *http.Request, ctx context.Context) {
	cancelJob := job.cancel
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !desktopWSAuthorizationValid(s, request, desktopScopeAdmin) {
					cancelJob()
					return
				}
			}
		}
	}()

	rendition, err := s.convertBookshelfBook(ctx, runtime, job.BookID)
	s.bookshelfMu.Lock()
	defer s.bookshelfMu.Unlock()
	job.EndedAt = time.Now().UTC()
	if job.cancel != nil {
		job.cancel()
		job.cancel = nil
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			job.Status = "canceled"
		} else {
			job.Status = "failed"
			job.Error = bookshelfErrorCode(err)
		}
		return
	}
	job.Status = "complete"
	job.Rendition = &rendition
}

func (s *Server) convertBookshelfBook(ctx context.Context, runtime *desktopBookshelfRuntime, bookID string) (bookshelf.Rendition, error) {
	book, err := runtime.service.GetBook(ctx, bookID)
	if err != nil {
		return bookshelf.Rendition{}, err
	}
	extension := strings.ToLower(filepath.Ext(book.Path))
	if !bookshelfContainsString(bookshelfConvertibleFormats, strings.TrimPrefix(extension, ".")) {
		return bookshelf.Rendition{}, bookshelf.ErrUnsupported
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !s.bookshelfEnabled() {
		return bookshelf.Rendition{}, errBookshelfDisabled
	}
	converterVersion, err := bookshelfconvert.Version(ctx, cfg.VirtualDesktop.BookshelfConverterPath)
	if err != nil {
		return bookshelf.Rendition{}, err
	}
	lease, err := runtime.service.Prepare(ctx, bookID)
	if err != nil {
		return bookshelf.Rendition{}, err
	}
	defer lease.Close()
	if lease.Size <= 0 || lease.Size > runtime.maxBookBytes.Load() {
		return bookshelf.Rendition{}, bookshelf.ErrLimitExceeded
	}
	book, err = runtime.service.GetBook(ctx, bookID)
	if err != nil {
		return bookshelf.Rendition{}, err
	}
	for _, rendition := range book.Renditions {
		if rendition.Format == "epub" && rendition.Available && rendition.BaseRevision == lease.Revision && rendition.ConverterVersion == converterVersion {
			return rendition, nil
		}
	}
	input, err := os.CreateTemp(runtime.root, ".bookshelf-convert-input-*"+extension)
	if err != nil {
		return bookshelf.Rendition{}, fmt.Errorf("stage Bookshelf converter input: %w", err)
	}
	inputPath := input.Name()
	defer os.Remove(inputPath)
	if _, err := lease.File.Seek(0, io.SeekStart); err != nil {
		_ = input.Close()
		return bookshelf.Rendition{}, err
	}
	maxBytes := runtime.maxBookBytes.Load()
	written, err := io.Copy(input, io.LimitReader(lease.File, maxBytes+1))
	if err != nil || written != lease.Size {
		_ = input.Close()
		if err != nil {
			return bookshelf.Rendition{}, fmt.Errorf("copy Bookshelf converter input: %w", err)
		}
		return bookshelf.Rendition{}, bookshelf.ErrStaleRendition
	}
	if err := input.Sync(); err != nil {
		_ = input.Close()
		return bookshelf.Rendition{}, err
	}
	if err := input.Close(); err != nil {
		return bookshelf.Rendition{}, err
	}
	output, err := os.CreateTemp(runtime.root, ".bookshelf-convert-output-*.epub")
	if err != nil {
		return bookshelf.Rendition{}, fmt.Errorf("reserve Bookshelf converter output: %w", err)
	}
	outputPath := output.Name()
	if err := output.Close(); err != nil {
		_ = os.Remove(outputPath)
		return bookshelf.Rendition{}, err
	}
	if err := os.Remove(outputPath); err != nil {
		return bookshelf.Rendition{}, err
	}
	defer os.Remove(outputPath)
	if err := bookshelfconvert.Convert(ctx, cfg.VirtualDesktop.BookshelfConverterPath, inputPath, outputPath); err != nil {
		return bookshelf.Rendition{}, err
	}
	outputFile, err := os.Open(outputPath)
	if err != nil {
		return bookshelf.Rendition{}, fmt.Errorf("open Bookshelf converter output: %w", err)
	}
	defer outputFile.Close()
	rendition, err := runtime.service.AttachRenditionVersioned(ctx, bookID, book.Revision, lease.Revision, "epub", converterVersion, outputFile)
	if err != nil {
		return bookshelf.Rendition{}, err
	}
	return rendition, nil
}

func (s *Server) bookshelfUpdateSource(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, id string) {
	if id == bookshelfWorkspaceSourceID || id == bookshelfImportsSourceID {
		writeBookshelfError(w, bookshelf.ErrUnsupported)
		return
	}
	current, err := runtime.service.GetSource(r.Context(), id)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	oldProvider := current.Provider
	var patch map[string]json.RawMessage
	if err := decodeDesktopJSON(w, r, &patch, desktopSmallJSONBodyLimit); err != nil {
		writeBookshelfError(w, err)
		return
	}
	fields := patch
	if raw, ok := patch["source"]; ok {
		if err := json.Unmarshal(raw, &fields); err != nil {
			writeBookshelfError(w, err)
			return
		}
	}
	var credential struct {
		Password *string `json:"password"`
	}
	if raw, ok := patch["credential"]; ok {
		if err := json.Unmarshal(raw, &credential); err != nil {
			writeBookshelfError(w, err)
			return
		}
	}
	if err := mergeBookshelfSourcePatch(&current, fields); err != nil {
		writeBookshelfError(w, err)
		return
	}
	newPassword := ""
	if credential.Password != nil {
		newPassword = *credential.Password
	}
	if err := validateBookshelfSource(current, newPassword, true); err != nil {
		writeBookshelfError(w, err)
		return
	}
	if current.Provider == bookshelf.ProviderSMB && s.Vault == nil {
		writeBookshelfError(w, errors.New("SMB credentials are unavailable"))
		return
	}
	if oldProvider == bookshelf.ProviderSMB && s.Vault == nil {
		writeBookshelfError(w, errors.New("SMB credentials are unavailable"))
		return
	}
	var oldPassword string
	if oldProvider == bookshelf.ProviderSMB && s.Vault != nil {
		oldPassword, _ = s.Vault.ReadSecret(bookshelfSMBSecretKey(id))
	}
	if current.Provider == bookshelf.ProviderSMB {
		if newPassword == "" && oldPassword == "" {
			writeBookshelfError(w, errors.New("SMB credentials are unavailable"))
			return
		}
	}
	if current.Provider == bookshelf.ProviderSMB && newPassword != "" {
		security.RegisterSensitive(newPassword)
		if err := s.Vault.WriteSecret(bookshelfSMBSecretKey(id), newPassword); err != nil {
			writeBookshelfError(w, err)
			return
		}
	}
	if raw := strings.TrimSpace(r.Header.Get("If-Match")); raw != "" {
		generation, err := requiredBookshelfRevision(raw)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		current.Generation = generation
	}
	updated, err := runtime.service.UpdateSource(r.Context(), current)
	if err != nil {
		if current.Provider == bookshelf.ProviderSMB && newPassword != "" {
			if oldPassword != "" {
				_ = s.Vault.WriteSecret(bookshelfSMBSecretKey(id), oldPassword)
			} else {
				_ = s.Vault.DeleteSecret(bookshelfSMBSecretKey(id))
			}
		}
		writeBookshelfError(w, err)
		return
	}
	if oldProvider == bookshelf.ProviderSMB && current.Provider != bookshelf.ProviderSMB {
		if err := s.Vault.DeleteSecret(bookshelfSMBSecretKey(id)); err != nil {
			writeBookshelfError(w, err)
			return
		}
	}
	s.cancelBookshelfSourceJobs(runtime, id)
	s.revokeDesktopBookshelfSourceTickets(runtime, id)
	writeJSON(w, map[string]any{"source": updated})
}

func mergeBookshelfSourcePatch(source *bookshelf.Source, patch map[string]json.RawMessage) error {
	for key, value := range patch {
		switch key {
		case "name":
			if err := json.Unmarshal(value, &source.Name); err != nil {
				return err
			}
		case "provider":
			if err := json.Unmarshal(value, &source.Provider); err != nil {
				return err
			}
		case "mount_id":
			if err := json.Unmarshal(value, &source.MountID); err != nil {
				return err
			}
		case "path":
			if err := json.Unmarshal(value, &source.Path); err != nil {
				return err
			}
		case "recursive":
			if err := json.Unmarshal(value, &source.Recursive); err != nil {
				return err
			}
		case "host":
			if err := json.Unmarshal(value, &source.Host); err != nil {
				return err
			}
		case "share":
			if err := json.Unmarshal(value, &source.Share); err != nil {
				return err
			}
		case "username":
			if err := json.Unmarshal(value, &source.Username); err != nil {
				return err
			}
		case "domain":
			if err := json.Unmarshal(value, &source.Domain); err != nil {
				return err
			}
		case "id":
			return errors.New("source identity cannot be changed")
		case "generation":
			// The source generation in a PATCH body is an optimistic concurrency token.
			if err := json.Unmarshal(value, &source.Generation); err != nil {
				return err
			}
		case "refresh_minutes":
			if err := json.Unmarshal(value, &source.RefreshMinutes); err != nil {
				return err
			}
		default:
			return errors.New("unknown source field")
		}
	}
	return nil
}

func sourceGenerationHeader(header string, fallback int64) int64 {
	value, err := parseBookshelfRevision(header)
	if err != nil {
		return fallback
	}
	return value
}

func (s *Server) bookshelfRemoveSource(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, id string) {
	if id == bookshelfWorkspaceSourceID || id == bookshelfImportsSourceID {
		writeBookshelfError(w, bookshelf.ErrUnsupported)
		return
	}
	source, err := runtime.service.GetSource(r.Context(), id)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	if source.Provider == bookshelf.ProviderSMB && s.Vault == nil {
		writeBookshelfError(w, errors.New("SMB credentials are unavailable"))
		return
	}
	var secret string
	if source.Provider == bookshelf.ProviderSMB && s.Vault != nil {
		secret, _ = s.Vault.ReadSecret(bookshelfSMBSecretKey(id))
		if err := s.Vault.DeleteSecret(bookshelfSMBSecretKey(id)); err != nil {
			writeBookshelfError(w, err)
			return
		}
	}
	s.cancelBookshelfSourceJobs(runtime, id)
	if err := runtime.service.RemoveSource(r.Context(), id); err != nil {
		if secret != "" && s.Vault != nil {
			security.RegisterSensitive(secret)
			_ = s.Vault.WriteSecret(bookshelfSMBSecretKey(id), secret)
		}
		writeBookshelfError(w, err)
		return
	}
	s.revokeDesktopBookshelfSourceTickets(runtime, id)
	writeJSON(w, map[string]bool{"removed": true})
}

func (s *Server) cancelBookshelfSourceJobs(runtime *desktopBookshelfRuntime, sourceID string) {
	s.bookshelfMu.Lock()
	defer s.bookshelfMu.Unlock()
	for _, job := range runtime.jobs {
		if job.SourceID == sourceID && job.cancel != nil {
			job.cancel()
		}
	}
}

func (s *Server) bookshelfCollections(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime) {
	collections, err := runtime.service.ListCollections(r.Context())
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(collections))
	for _, collection := range collections {
		bookIDs, err := runtime.service.CollectionBookIDs(r.Context(), collection.ID)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		out = append(out, map[string]any{"id": collection.ID, "name": collection.Name, "position": collection.Position, "revision": collection.Revision, "book_count": collection.BookCount, "book_ids": bookIDs, "created_at": collection.CreatedAt})
	}
	writeJSON(w, map[string]any{"collections": out})
}

func (s *Server) bookshelfCreateCollection(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime) {
	var body struct {
		Name     string `json:"name"`
		Position int    `json:"position"`
	}
	if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
		writeBookshelfError(w, err)
		return
	}
	collection, err := runtime.service.CreateCollection(r.Context(), body.Name, body.Position)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	writeJSON(w, map[string]any{"collection": collection})
}

func (s *Server) bookshelfCollectionRoute(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime, rest string) {
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		writeBookshelfError(w, bookshelf.ErrNotFound)
		return
	}
	id, err := url.PathUnescape(parts[0])
	if err != nil {
		writeBookshelfError(w, bookshelf.ErrNotFound)
		return
	}
	if len(parts) == 2 && parts[1] == "books" && r.Method == http.MethodPatch {
		if _, err := runtime.service.GetCollection(r.Context(), id); err != nil {
			writeBookshelfError(w, err)
			return
		}
		expectedRevision, err := requiredBookshelfRevision(r.Header.Get("If-Match"))
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		var body struct {
			BookIDs []string `json:"book_ids"`
		}
		if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
			writeBookshelfError(w, err)
			return
		}
		updated, err := runtime.service.SetCollectionBooks(r.Context(), id, expectedRevision, body.BookIDs)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		writeJSON(w, map[string]any{"collection": updated})
		return
	}
	if len(parts) == 3 && parts[1] == "books" {
		bookID, err := url.PathUnescape(parts[2])
		if err != nil {
			writeBookshelfError(w, bookshelf.ErrNotFound)
			return
		}
		collection, err := runtime.service.GetCollection(r.Context(), id)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		bookIDs, err := runtime.service.CollectionBookIDs(r.Context(), id)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		found := false
		for _, current := range bookIDs {
			found = found || current == bookID
		}
		if r.Method == http.MethodPut && !found {
			bookIDs = append(bookIDs, bookID)
		}
		if r.Method == http.MethodDelete && found {
			filtered := bookIDs[:0]
			for _, current := range bookIDs {
				if current != bookID {
					filtered = append(filtered, current)
				}
			}
			bookIDs = filtered
		}
		if r.Method != http.MethodPut && r.Method != http.MethodDelete {
			writeBookshelfErrorStatus(w, "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		if !found && r.Method == http.MethodDelete || found && r.Method == http.MethodPut {
			writeJSON(w, map[string]any{"collection": collection})
			return
		}
		updated, err := runtime.service.SetCollectionBooks(r.Context(), id, collection.Revision, bookIDs)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		writeJSON(w, map[string]any{"collection": updated})
		return
	}
	if len(parts) != 1 {
		writeBookshelfError(w, bookshelf.ErrNotFound)
		return
	}
	collection, err := runtime.service.GetCollection(r.Context(), id)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Name     *string `json:"name"`
			Position *int    `json:"position"`
		}
		if err := decodeDesktopJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
			writeBookshelfError(w, err)
			return
		}
		name := collection.Name
		if body.Name != nil {
			name = *body.Name
		}
		position := collection.Position
		if body.Position != nil {
			position = *body.Position
		}
		updated, err := runtime.service.UpdateCollection(r.Context(), id, collection.Revision, name, position)
		if err != nil {
			writeBookshelfError(w, err)
			return
		}
		writeJSON(w, map[string]any{"collection": updated})
	case http.MethodDelete:
		if err := runtime.service.DeleteCollection(r.Context(), id, collection.Revision); err != nil {
			writeBookshelfError(w, err)
			return
		}
		writeJSON(w, map[string]bool{"removed": true})
	default:
		writeBookshelfErrorStatus(w, "method_not_allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) bookshelfImport(w http.ResponseWriter, r *http.Request, runtime *desktopBookshelfRuntime) {
	maxBytes := runtime.maxBookBytes.Load()
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeBookshelfError(w, bookshelf.ErrLimitExceeded)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	defer file.Close()
	if header.Size < 0 || header.Size > maxBytes {
		writeBookshelfError(w, bookshelf.ErrLimitExceeded)
		return
	}
	free, err := fileutil.FreeDiskBytes(runtime.importsDir)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	if free < header.Size+bookshelfMinFreeBytes {
		writeBookshelfErrorStatus(w, "insufficient_disk_space", http.StatusInsufficientStorage)
		return
	}
	name, size, revision, err := stageBookshelfUpload(r.Context(), runtime.importsDir, header.Filename, file, maxBytes)
	if err != nil {
		writeBookshelfError(w, err)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		original := path.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
		title = strings.TrimSuffix(original, filepath.Ext(original))
		if strings.HasSuffix(strings.ToLower(original), ".fb2.zip") {
			title = strings.TrimSuffix(original, filepath.Ext(original))
			title = strings.TrimSuffix(title, filepath.Ext(title))
		}
	}
	metadata := bookshelf.Metadata{Title: title, Author: strings.TrimSpace(r.FormValue("author")), Format: formatBookshelfPath(name)}
	entry := bookshelf.Entry{Path: name, Name: title, Format: metadata.Format, Size: size, Revision: revision}
	book, err := runtime.service.AddBook(r.Context(), bookshelfImportsSourceID, entry, metadata)
	if err != nil {
		_ = os.Remove(filepath.Join(runtime.importsDir, name))
		writeBookshelfError(w, err)
		return
	}
	writeJSON(w, map[string]any{"book": book})
}

func (s *Server) serveBookshelfReader(w http.ResponseWriter, r *http.Request) {
	shell, err := fs.ReadFile(uiFiles, "bookshelf-reader.html")
	if err != nil {
		writeBookshelfErrorStatus(w, "reader_unavailable", http.StatusServiceUnavailable)
		return
	}
	_, err = fs.ReadFile(uiFiles, "js/desktop/apps/bookshelf/reader-host.js")
	if err != nil {
		writeBookshelfErrorStatus(w, "reader_unavailable", http.StatusServiceUnavailable)
		return
	}
	_, err = fs.ReadFile(uiFiles, "js/vendor/foliate/reader.bundle.js")
	if err != nil {
		writeBookshelfErrorStatus(w, "reader_unavailable", http.StatusServiceUnavailable)
		return
	}
	version := html.EscapeString(uiBuildVersion)
	hostPath := "/js/desktop/apps/bookshelf/reader-host.js?v=" + url.QueryEscape(uiBuildVersion)
	shell = []byte(strings.Replace(string(shell), `src="/js/desktop/apps/bookshelf/reader-host.js"`, `src="`+hostPath+`"`, 1))
	shell = []byte(strings.Replace(string(shell), "</head>", `<meta name="bookshelf-version" content="`+version+`"></head>`, 1))
	origin := bookshelfRequestOrigin(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=(), serial=(), bluetooth=()")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src "+origin+"/js/desktop/apps/bookshelf/reader-host.js "+origin+"/js/vendor/foliate/reader.bundle.js; style-src 'unsafe-inline' blob:; img-src blob: data:; media-src blob: data:; font-src blob: data:; connect-src blob:; frame-src data: blob:; worker-src blob:; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(shell)
	}
}

func bookshelfRequestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || trustedForwardedRequest(r) && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: r.Host}).String()
}

func bookshelfContentType(format string) string {
	switch strings.ToLower(format) {
	case "epub":
		return "application/epub+zip"
	case "pdf":
		return "application/pdf"
	case "fb2", "fb2.zip", "cbz", "cbr":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

func randomBookshelfID() string { return strings.ToLower(rand.Text()) }

func parseBookshelfRevision(raw string) (int64, error) {
	raw = strings.TrimSpace(strings.Trim(raw, `"`))
	if raw == "" {
		return 0, errors.New("missing revision")
	}
	return strconv.ParseInt(raw, 10, 64)
}

func requiredBookshelfRevision(raw string) (int64, error) {
	revision, err := parseBookshelfRevision(raw)
	if err != nil {
		return 0, bookshelf.ErrConflict
	}
	return revision, nil
}

func writeBookshelfError(w http.ResponseWriter, err error) {
	status, code := http.StatusBadRequest, bookshelfErrorCode(err)
	switch {
	case errors.Is(err, bookshelf.ErrNotFound), errors.Is(err, tools.ErrStorageNotFound), errors.Is(err, os.ErrNotExist):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, bookshelf.ErrConflict):
		status, code = http.StatusConflict, "revision_conflict"
	case errors.Is(err, bookshelf.ErrStaleRendition):
		status, code = http.StatusConflict, "source_changed"
	case errors.Is(err, bookshelf.ErrLimitExceeded), errors.Is(err, bookshelf.ErrInsufficientDisk):
		status, code = http.StatusRequestEntityTooLarge, "limit_exceeded"
	case errors.Is(err, bookshelf.ErrUnsupported):
		status, code = http.StatusUnprocessableEntity, "unsupported"
	case errors.Is(err, errBookshelfDisabled):
		status, code = http.StatusServiceUnavailable, "bookshelf_disabled"
	case errors.Is(err, tools.ErrStorageReadOnly):
		status, code = http.StatusForbidden, "source_readonly"
	case errors.Is(err, tools.ErrStorageAccessDenied):
		status, code = http.StatusForbidden, "source_access_denied"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status, code = http.StatusServiceUnavailable, "request_canceled"
	case errors.Is(err, bookshelf.ErrInvalidPath):
		status, code = http.StatusBadRequest, "invalid_path"
	}
	writeBookshelfErrorStatus(w, code, status)
}

func bookshelfErrorCode(err error) string {
	switch {
	case errors.Is(err, bookshelf.ErrNotFound), errors.Is(err, tools.ErrStorageNotFound), errors.Is(err, os.ErrNotExist):
		return "not_found"
	case errors.Is(err, bookshelf.ErrConflict):
		return "revision_conflict"
	case errors.Is(err, bookshelf.ErrStaleRendition):
		return "source_changed"
	case errors.Is(err, bookshelf.ErrLimitExceeded), errors.Is(err, bookshelf.ErrInsufficientDisk):
		return "limit_exceeded"
	case errors.Is(err, bookshelf.ErrUnsupported):
		return "unsupported"
	case errors.Is(err, errBookshelfDisabled):
		return "bookshelf_disabled"
	case errors.Is(err, tools.ErrStorageReadOnly):
		return "source_readonly"
	case errors.Is(err, tools.ErrStorageAccessDenied):
		return "source_access_denied"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "request_canceled"
	default:
		return "bookshelf_unavailable"
	}
}

func writeBookshelfErrorStatus(w http.ResponseWriter, code string, status int) {
	jsonError(w, code, status)
}
