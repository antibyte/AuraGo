package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/tools"
)

type desktopStorageRequest struct {
	Location    tools.StorageLocation `json:"location"`
	Destination tools.StorageLocation `json:"destination"`
	Version     string                `json:"version,omitempty"`
	Mode        string                `json:"mode,omitempty"`
}

type desktopStorageSession struct {
	s        *Server
	svc      *desktop.Service
	cloud    tools.CloudStorage
	maxBytes int64
	ctx      context.Context
}

func (s *Server) revokeStorageRequests() {
	r := &s.storageRuns
	r.mu.Lock()
	runs := r.runs
	r.runs = make(map[uint64]context.CancelFunc)
	r.mu.Unlock()
	for _, cancel := range runs {
		cancel()
	}
}

func (s *Server) storageContext(parent context.Context) (context.Context, context.CancelFunc) {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	r := &s.storageRuns
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runs == nil {
		r.runs = make(map[uint64]context.CancelFunc)
	}
	r.next++
	id := r.next
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	if s.integrationCtx != nil {
		stop := context.AfterFunc(s.integrationCtx, cancel)
		context.AfterFunc(ctx, func() { stop() })
	}
	r.runs[id] = cancel
	context.AfterFunc(ctx, func() { r.mu.Lock(); delete(r.runs, id); r.mu.Unlock() })
	return ctx, cancel
}

func desktopStorageCloud(cfg *config.Config) tools.CloudStorage {
	return tools.CloudStorage{
		WebDAV: tools.WebDAVConfig{URL: cfg.WebDAV.URL, AuthType: cfg.WebDAV.AuthType, Username: cfg.WebDAV.Username, Password: cfg.WebDAV.Password, Token: cfg.WebDAV.Token, ReadOnly: cfg.WebDAV.ReadOnly},
		Koofr:  tools.KoofrConfig{BaseURL: cfg.Koofr.BaseURL, Username: cfg.Koofr.Username, AppPassword: cfg.Koofr.AppPassword, ReadOnly: cfg.Koofr.ReadOnly},
	}
}

func (d *desktopStorageSession) authorize(loc tools.StorageLocation, write bool) error {
	if err := d.ctx.Err(); err != nil {
		return err
	}
	if err := tools.ValidateStorageLocation(loc); err != nil {
		return err
	}
	cfg := d.s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		return tools.ErrStorageReadOnly
	}
	if write && cfg.VirtualDesktop.ReadOnly {
		return tools.ErrStorageReadOnly
	}
	switch loc.Provider {
	case "gog":
		a, ok := cfg.Gog.Account(loc.AccountID)
		g := a.Services["drive"]
		if !cfg.Gog.Enabled || !ok || !a.Enabled || !a.DesktopAccess || !g.Read || write && (cfg.Gog.ReadOnly || !g.Create && !g.Update && !g.Delete) {
			return tools.ErrStorageReadOnly
		}
		return nil
	case "local":
		return nil
	case "webdav":
		if !cfg.WebDAV.Enabled || !cfg.WebDAV.ShowInFileManager || (write && cfg.WebDAV.ReadOnly) {
			return tools.ErrStorageReadOnly
		}
	case "koofr":
		if !cfg.Koofr.Enabled || !cfg.Koofr.ShowInFileManager || (write && cfg.Koofr.ReadOnly) {
			return tools.ErrStorageReadOnly
		}
		mounts, err := d.cloud.Mounts(d.ctx)
		if err != nil {
			return err
		}
		for _, m := range mounts {
			if m.ID == loc.MountID && m.Read && (!write || m.Write) && (m.Online == nil || *m.Online) {
				return nil
			}
		}
		return tools.ErrStorageReadOnly
	}
	return nil
}

func handleDesktopStorage(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeAdmin, desktopMethodOperation(r.Method)) {
			return
		}
		ctx, cancel := s.storageContext(r.Context())
		defer cancel()
		r = r.WithContext(ctx)
		go func() {
			tick := time.NewTicker(time.Second)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					if !desktopWSAuthorizationValid(s, r, desktopScopeAdmin) {
						cancel()
						return
					}
				}
			}
		}()
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.VirtualDesktop.Enabled {
			storageError(w, tools.ErrStorageReadOnly)
			return
		}
		action := strings.TrimPrefix(r.URL.Path, "/api/desktop/storage/")
		if action == "sources" && r.Method == http.MethodGet {
			writeJSON(w, map[string]interface{}{"gog": cfg.Gog.Enabled, "gog_accounts": tools.GogStorageAccounts(cfg), "gog_readonly": cfg.Gog.ReadOnly, "webdav": cfg.WebDAV.Enabled && cfg.WebDAV.ShowInFileManager, "koofr": cfg.Koofr.Enabled && cfg.Koofr.ShowInFileManager, "readonly": cfg.VirtualDesktop.ReadOnly, "webdav_readonly": cfg.WebDAV.ReadOnly, "koofr_readonly": cfg.Koofr.ReadOnly})
			return
		}
		cloud := desktopStorageCloud(cfg)
		cloud.Gog = &tools.GogStorage{Service: s.gogService()}
		if action == "mounts" && r.Method == http.MethodGet {
			if !cfg.Koofr.Enabled || !cfg.Koofr.ShowInFileManager {
				storageError(w, tools.ErrStorageReadOnly)
				return
			}
			mounts, err := cloud.Mounts(ctx)
			if err != nil {
				storageError(w, err)
				return
			}
			writeJSON(w, map[string]interface{}{"mounts": mounts})
			return
		}
		svc, _, err := s.getDesktopService(ctx)
		if err != nil {
			storageError(w, err)
			return
		}
		d := &desktopStorageSession{s: s, svc: svc, cloud: cloud, maxBytes: int64(svc.Config().MaxFileSizeMB) << 20, ctx: ctx}
		var req desktopStorageRequest
		q := r.URL.Query()
		if r.Method == http.MethodGet || action == "upload" {
			req.Location = tools.StorageLocation{Provider: q.Get("provider"), MountID: q.Get("mount_id"), Path: q.Get("path"), AccountID: q.Get("account_id"), ItemID: q.Get("item_id"), ParentID: q.Get("parent_id"), Name: q.Get("name")}
			req.Version = r.Header.Get("If-Match")
		} else if err = decodeDesktopJSON(w, r, &req, desktopSmallJSONBodyLimit); err != nil {
			storageError(w, err)
			return
		}
		write := r.Method != http.MethodGet
		if action == "transfer" {
			write = req.Mode == "move"
		}
		if err = d.authorize(req.Location, write); err != nil {
			storageError(w, err)
			return
		}
		switch {
		case action == "files" && r.Method == http.MethodGet:
			entries, err := d.list(req.Location)
			if err != nil {
				storageError(w, err)
				return
			}
			writeJSON(w, map[string]interface{}{"files": entries})
		case action == "info" && r.Method == http.MethodGet:
			entry, err := d.stat(req.Location)
			if err != nil {
				storageError(w, err)
				return
			}
			writeJSON(w, entry)
		case action == "content" && r.Method == http.MethodGet:
			d.serveContent(w, r, req.Location)
		case action == "upload" && r.Method == http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, d.maxBytes)
			readStopped := make(chan struct{})
			stopRead := context.AfterFunc(ctx, func() {
				// Context checks alone cannot interrupt a paused client's Body.Read.
				_ = http.NewResponseController(w).SetReadDeadline(time.Now())
				close(readStopped)
			})
			defer func() {
				if !stopRead() {
					<-readStopped
				}
			}()
			entry, err := d.put(req.Location, r.Body, req.Version)
			if err != nil {
				d.writeConflict(w, err, req.Location)
				return
			}
			writeJSON(w, entry)
		case action == "directory" && r.Method == http.MethodPost:
			if err = d.mkdir(req.Location); err != nil {
				d.writeConflict(w, err, req.Location)
				return
			}
			writeJSON(w, map[string]string{"status": "ok"})
		case action == "entry" && r.Method == http.MethodDelete:
			if req.Location.Path == "" {
				storageError(w, fmt.Errorf("cannot delete storage root"))
				return
			}
			if req.Location.Provider == "local" {
				err = svc.DeletePath(ctx, req.Location.Path, "desktop")
			} else if req.Location.Provider == "gog" {
				err = cloud.Gog.Trash(ctx, req.Location)
			} else {
				err = cloud.Remove(ctx, req.Location, req.Version, false)
			}
			if err != nil {
				storageError(w, err)
				return
			}
			writeJSON(w, map[string]string{"status": "ok"})
		case action == "transfer" && r.Method == http.MethodPost:
			d.serveTransfer(w, r, req)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func storageError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, tools.ErrStorageReadOnly) {
		status = http.StatusForbidden
	}
	if errors.Is(err, tools.ErrStorageNotFound) || errors.Is(err, os.ErrNotExist) {
		status = http.StatusNotFound
	}
	if errors.Is(err, tools.ErrStorageConflict) {
		status = http.StatusConflict
	}
	jsonError(w, err.Error(), status)
}

func storageFileEntry(f desktop.FileEntry) tools.StorageEntry {
	return tools.StorageEntry{Name: f.Name, Path: f.Path, Type: f.Type, Size: f.Size, Modified: f.Modified.UTC().Format(time.RFC3339), ContentType: f.MIMEType}
}

func (d *desktopStorageSession) list(loc tools.StorageLocation) ([]tools.StorageEntry, error) {
	if err := d.authorize(loc, false); err != nil {
		return nil, err
	}
	if loc.Provider != "local" {
		return d.cloud.List(d.ctx, loc)
	}
	files, err := d.svc.ListFiles(d.ctx, loc.Path)
	if err != nil {
		return nil, err
	}
	if len(files) > 10000 {
		return nil, fmt.Errorf("storage listing exceeds 10000 entries")
	}
	entries := make([]tools.StorageEntry, 0, len(files))
	for _, f := range files {
		entries = append(entries, storageFileEntry(f))
	}
	return entries, nil
}

func (d *desktopStorageSession) stat(loc tools.StorageLocation) (tools.StorageEntry, error) {
	if err := d.authorize(loc, false); err != nil {
		return tools.StorageEntry{}, err
	}
	if loc.Provider != "local" {
		return d.cloud.Stat(d.ctx, loc)
	}
	f, version, err := d.svc.StatStoragePath(d.ctx, loc.Path, d.maxBytes)
	entry := storageFileEntry(f)
	entry.Version = version
	return entry, err
}

func (d *desktopStorageSession) open(loc tools.StorageLocation, version string) (io.ReadCloser, error) {
	if err := d.authorize(loc, false); err != nil {
		return nil, err
	}
	if loc.Provider != "local" {
		return d.cloud.Open(d.ctx, loc, version)
	}
	in, _, _, err := d.svc.OpenPreviewFile(d.ctx, loc.Path)
	return in, err
}

func (d *desktopStorageSession) mkdir(loc tools.StorageLocation) error {
	if err := d.authorize(loc, true); err != nil {
		return err
	}
	if loc.Path == "" {
		return fmt.Errorf("cannot create root")
	}
	if _, err := d.stat(loc); !errors.Is(err, tools.ErrStorageNotFound) && !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return tools.ErrStorageConflict
		}
		return err
	}
	if loc.Provider != "local" {
		return d.cloud.Mkdir(d.ctx, loc)
	}
	err := d.svc.CreateStorageDirectory(d.ctx, loc.Path)
	if errors.Is(err, os.ErrExist) {
		return tools.ErrStorageConflict
	}
	return err
}

func storagePrecondition(version string) desktop.FileWritePrecondition {
	return func(state desktop.FileWriteState) error {
		if !state.Exists && version == "" {
			return nil
		}
		current := state.Version
		if current == "" && state.Exists {
			current = desktop.NoteVersion(state.Data)
		}
		if state.Exists && state.Entry.Type != "directory" && version != "" && current == version {
			return nil
		}
		return tools.ErrStorageConflict
	}
}

func (d *desktopStorageSession) put(loc tools.StorageLocation, reader io.Reader, version string) (tools.StorageEntry, error) {
	if err := d.authorize(loc, true); err != nil {
		return tools.StorageEntry{}, err
	}
	if version != "" {
		entry, err := d.stat(loc)
		if err != nil {
			return tools.StorageEntry{}, err
		}
		if entry.Version == "" || entry.Version != version || entry.Type != "file" {
			return tools.StorageEntry{}, tools.ErrStorageConflict
		}
	}
	if loc.Provider == "local" {
		f, err := d.svc.WriteFileStreamConditional(d.ctx, loc.Path, reader, d.maxBytes, "desktop", storagePrecondition(version))
		return storageFileEntry(f), err
	}
	// Validate size before any remote mutation, including unknown-length uploads.
	tmp, err := os.CreateTemp("", "aurago-storage-*")
	if err != nil {
		return tools.StorageEntry{}, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	n, err := io.Copy(tmp, io.LimitReader(storageContextReader{d.ctx, reader}, d.maxBytes+1))
	if err != nil {
		return tools.StorageEntry{}, err
	}
	if n > d.maxBytes {
		return tools.StorageEntry{}, fmt.Errorf("storage file exceeds size limit")
	}
	if _, err = tmp.Seek(0, io.SeekStart); err != nil {
		return tools.StorageEntry{}, err
	}
	if err = d.authorize(loc, true); err != nil {
		return tools.StorageEntry{}, err
	}
	return d.cloud.Put(d.ctx, loc, tmp, version)
}

type storageContextReader struct {
	ctx context.Context
	io.Reader
}

func (r storageContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

func (d *desktopStorageSession) writeConflict(w http.ResponseWriter, err error, loc tools.StorageLocation) {
	var conflict *desktop.PathConflict
	if !errors.Is(err, tools.ErrStorageConflict) && !errors.As(err, &conflict) {
		storageError(w, err)
		return
	}
	entry, _ := d.stat(loc)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "file_conflict", "code": "file_conflict", "conflict": map[string]interface{}{"path": loc.Path, "version": entry.Version, "directory": entry.Type == "directory"}})
}

func (d *desktopStorageSession) serveContent(w http.ResponseWriter, r *http.Request, loc tools.StorageLocation) {
	entry, err := d.stat(loc)
	if err != nil {
		storageError(w, err)
		return
	}
	if entry.Type != "file" || entry.Size < 0 || entry.Size > d.maxBytes {
		storageError(w, fmt.Errorf("storage content exceeds file limit"))
		return
	}
	in, err := d.open(loc, entry.Version)
	if err != nil {
		storageError(w, err)
		return
	}
	defer in.Close()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	kind := entry.ContentType
	if kind == "" {
		kind = mime.TypeByExtension(path.Ext(entry.Name))
	}
	limit := entry.Size
	if r.URL.Query().Get("preview") == "text" {
		kind = "text/plain; charset=utf-8"
		if limit > 64<<10 {
			limit = 64 << 10
		}
	}
	mediaType, _, _ := mime.ParseMediaType(kind)
	disposition := "attachment"
	if r.URL.Query().Get("download") != "1" && (mediaType == "text/plain" || strings.HasPrefix(mediaType, "image/") && mediaType != "image/svg+xml" || strings.HasPrefix(mediaType, "audio/") || strings.HasPrefix(mediaType, "video/")) {
		disposition = "inline"
	}
	if kind == "" {
		kind = "application/octet-stream"
	}
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": entry.Name}))
	w.Header().Set("Content-Length", fmt.Sprint(limit))
	if _, err := io.CopyN(w, storageContextReader{d.ctx, in}, limit); err != nil {
		panic(http.ErrAbortHandler) // A partial body must not become a successful download.
	}
}
