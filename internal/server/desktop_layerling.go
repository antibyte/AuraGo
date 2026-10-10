package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/fileutil"
	"aurago/internal/layerling"
	"aurago/internal/webassets"
	"github.com/gorilla/websocket"
)

const layerlingBase = "/api/desktop/layerling/"

type layerlingPreview struct {
	owner   string
	data    []byte
	mime    string
	expires time.Time
}
type layerlingHTTP struct {
	s        *Server
	broker   *layerling.Broker
	mu       sync.Mutex
	previews map[string]layerlingPreview
	assets   fs.FS
}

func registerLayerlingRoutes(mux *http.ServeMux, s *Server) *layerlingHTTP {
	h := &layerlingHTTP{s: s, broker: layerling.NewBroker(), previews: map[string]layerlingPreview{}, assets: webassets.Namespace("ui/js/vendor/layerling")}
	mux.HandleFunc(layerlingBase, h.serve)
	return h
}
func layerlingOwner(r *http.Request) string {
	if v := desktopRequestSessionHash(r); v != "" {
		return v
	}
	return "local-desktop"
}
func (h *layerlingHTTP) enabled() bool {
	h.s.CfgMu.RLock()
	defer h.s.CfgMu.RUnlock()
	return h.s.Cfg != nil && h.s.Cfg.VirtualDesktop.Enabled && h.s.Cfg.VirtualDesktop.Layerling.Enabled
}
func (h *layerlingHTTP) allowed(r *http.Request, write bool) bool {
	scope := desktopScopeRead
	if write {
		scope = desktopScopeWrite
	}
	if !desktopWSAuthorizationValid(h.s, r, scope) {
		return false
	}
	h.s.CfgMu.RLock()
	defer h.s.CfgMu.RUnlock()
	if h.s.Cfg == nil {
		return false
	}
	d := h.s.Cfg.VirtualDesktop
	return d.Enabled && d.Layerling.Enabled && d.AllowAgentControl && h.s.Cfg.Tools.VirtualDesktop.Enabled && (d.Layerling.AgentAccess == "read" || d.Layerling.AgentAccess == "write") && (!write || (!d.ReadOnly && d.Layerling.AgentAccess == "write"))
}
func (h *layerlingHTTP) chat(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := layerling.WithOwner(r.Context(), h.broker, layerlingOwner(r), func() bool { return h.allowed(r, false) })
		next(w, r.WithContext(ctx))
	}
}
func (h *layerlingHTTP) serve(w http.ResponseWriter, r *http.Request) {
	if !requireDesktopPermission(h.s, w, r, desktopScopeRead) {
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, layerlingBase)
	// Corresponding source and licensing remain available even when CAD is disabled.
	source := suffix == "ui/source.zip" || suffix == "ui/LICENSE.txt"
	if !source && !h.enabled() {
		jsonError(w, "layerling_disabled", http.StatusForbidden)
		return
	}
	switch {
	case suffix == "state" && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		h.s.CfgMu.RLock()
		d := h.s.Cfg.VirtualDesktop
		h.s.CfgMu.RUnlock()
		json.NewEncoder(w).Encode(map[string]interface{}{"enabled": d.Layerling.Enabled, "readonly": d.ReadOnly, "agent_access": d.Layerling.AgentAccess})
	case suffix == "connect" && r.Method == http.MethodGet:
		h.connect(w, r)
	case suffix == "file":
		h.file(w, r)
	case suffix == "files" && r.Method == http.MethodGet:
		svc, _, err := h.s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, "Desktop unavailable", 503)
			return
		}
		p := r.URL.Query().Get("path")
		files, err := svc.ListFiles(r.Context(), p)
		// The suggested first-save folder exists only after the first atomic save.
		if p == "Documents/Layerling" && errors.Is(err, fs.ErrNotExist) {
			files = []desktop.FileEntry{}
			err = nil
		}
		if err != nil {
			writeDesktopFileError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"files": files})
	case (strings.HasPrefix(suffix, "preview/") || strings.HasPrefix(suffix, "artifact/")) && r.Method == http.MethodGet:
		h.mu.Lock()
		p, ok := h.previews[suffix[strings.IndexByte(suffix, '/')+1:]]
		h.mu.Unlock()
		if !ok || p.owner != layerlingOwner(r) || time.Now().After(p.expires) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", p.mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(p.data)
	case strings.HasPrefix(suffix, "ui/") && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		name := strings.TrimPrefix(suffix, "ui/")
		if name == "" {
			name = "index.html"
		} else if strings.HasSuffix(name, "/") {
			name += "index.html"
		}
		if !fs.ValidPath(name) {
			http.NotFound(w, r)
			return
		}
		content, err := fs.ReadFile(h.assets, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if name == "index.html" {
			content = injectDesktopSDKChannelHTML(content)
		}
		w.Header().Set("Cache-Control", "no-store")
		// The bundled CAD editor needs local workers/WASM. Network access stays same-origin.
		// OCCT's pinned Emscripten binding uses Function constructors; scope eval to this verified app.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self' blob:; worker-src 'self' blob:; frame-ancestors 'self'; object-src 'none'; base-uri 'self'")
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(content))
	default:
		http.NotFound(w, r)
	}
}
func (h *layerlingHTTP) file(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	agent := r.Header.Get("X-Layerling-Editor") != "" || r.Header.Get("X-Layerling-Command") != ""
	allowed := func() bool {
		return !agent || h.broker.AuthorizeFile(layerlingOwner(r), r.Header.Get("X-Layerling-Editor"), r.Header.Get("X-Layerling-Command"), p, r.Method == http.MethodPut)
	}
	if !allowed() {
		jsonError(w, "Layerling command revoked", 403)
		return
	}
	switch strings.ToLower(path.Ext(p)) {
	case ".lyl", ".stl", ".obj", ".3mf", ".step", ".stp", ".svg", ".png":
	default:
		jsonError(w, "unsupported Layerling file type", 400)
		return
	}
	if len(p) > 512 || desktop.NotesPath(p, true) {
		jsonError(w, "invalid Layerling file path", 403)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		jsonError(w, "Method not allowed", 405)
		return
	}
	if r.Method == http.MethodPut && !requireDesktopOperation(h.s, w, r, desktopScopeWrite, desktopWrite) {
		return
	}
	svc, _, err := h.s.getDesktopService(r.Context())
	if err != nil {
		jsonError(w, "Desktop unavailable", 503)
		return
	}
	if r.Method == http.MethodGet {
		data, _, err := svc.ReadFileBytes(r.Context(), p)
		if err != nil {
			writeDesktopFileError(w, err)
			return
		}
		if err := layerling.ValidateArchive(r.Context(), p, data); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("ETag", desktop.NoteVersion(data))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(data)
		return
	}
	condition, err := desktopFilePrecondition(r)
	if err != nil {
		writeDesktopFileError(w, err)
		return
	}
	max := int64(svc.Config().MaxFileSizeMB) << 20
	if max <= 0 {
		max = 50 << 20
	}
	parent := r.Context()
	publication := fileutil.WithPublicationGate(parent, func(commit func() error) error {
		publish := func() error {
			h.s.CfgMu.RLock()
			defer h.s.CfgMu.RUnlock()
			d := h.s.Cfg.VirtualDesktop
			if !d.Enabled || !d.Layerling.Enabled || d.ReadOnly || (agent && (!d.AllowAgentControl || !h.s.Cfg.Tools.VirtualDesktop.Enabled || d.Layerling.AgentAccess != "write")) {
				return fmt.Errorf("Layerling permission revoked")
			}
			return publishDesktopResult(parent, commit)
		}
		if agent {
			return h.broker.PublishFile(layerlingOwner(r), r.Header.Get("X-Layerling-Editor"), r.Header.Get("X-Layerling-Command"), p, publish)
		}
		return publish()
	})
	source := desktop.SourceUser
	if agent {
		source = desktop.SourceAgent
	}
	data, err := svc.WriteFileStreamConditional(publication, p, http.MaxBytesReader(w, r.Body, max), max, source, func(state desktop.FileWriteState) error {
		if !h.enabled() || !desktopWSAuthorizationValid(h.s, r, desktopScopeWrite) || !allowed() {
			return fmt.Errorf("Layerling permission revoked")
		}
		return condition(state)
	})
	if err != nil {
		writeDesktopFileError(w, err)
		return
	}
	// The streamed body is hashed by the browser for its next conditional save.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
func (h *layerlingHTTP) connect(w http.ResponseWriter, r *http.Request) {
	if !desktop.SameHostWebSocketOrigin(r) {
		jsonError(w, "Same-origin connection required", 403)
		return
	}
	windowID := r.URL.Query().Get("window_id")
	if len(windowID) < 1 || len(windowID) > 128 {
		jsonError(w, "Invalid window id", 400)
		return
	}
	up := websocket.Upgrader{CheckOrigin: desktop.SameHostWebSocketOrigin}
	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(8 << 20)
	editor := h.broker.Register(layerlingOwner(r), windowID, func(write bool) bool { return h.allowed(r, write) })
	if editor == nil {
		return
	}
	defer h.broker.Remove(editor)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	if err = conn.WriteJSON(map[string]string{"type": "connected", "editor_id": editor.ID}); err != nil {
		return
	}
	go func() {
		defer cancel()
		for {
			var result layerling.Result
			if conn.ReadJSON(&result) != nil {
				return
			}
			if len(result.Error) > 1024 || len(result.ID) != 48 {
				return
			}
			if _, err := hex.DecodeString(result.ID); err != nil {
				return
			}
			if len(result.Data) > layerling.MaxResult || bytes.Contains(result.Data, []byte(`"dataUrl"`)) {
				var preview struct {
					DataURL string `json:"dataUrl"`
				}
				_ = json.Unmarshal(result.Data, &preview)
				data, mime, kind := []byte(result.Data), "application/json", "artifact/"
				if preview.DataURL != "" {
					if !strings.HasPrefix(preview.DataURL, "data:image/png;base64,") {
						return
					}
					var e error
					data, e = base64.StdEncoding.DecodeString(strings.TrimPrefix(preview.DataURL, "data:image/png;base64,"))
					if e != nil || len(data) > 5<<20 || !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
						return
					}
					mime, kind = "image/png", "preview/"
				}
				h.mu.Lock()
				for k, v := range h.previews {
					if time.Now().After(v.expires) {
						delete(h.previews, k)
					}
				}
				if len(h.previews) >= 16 {
					h.mu.Unlock()
					return
				}
				h.previews[result.ID] = layerlingPreview{owner: layerlingOwner(r), data: data, mime: mime, expires: time.Now().Add(5 * time.Minute)}
				h.mu.Unlock()
				result.Data, _ = json.Marshal(map[string]interface{}{"artifact": layerlingBase + kind + result.ID, "bytes": len(data)})
			}
			if !editor.Reply(result) {
				return
			}
		}
	}()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var shutdown <-chan struct{}
	if h.s.integrationCtx != nil {
		shutdown = h.s.integrationCtx.Done()
	}
	for {
		select {
		case command := <-editor.Commands():
			if !h.allowed(r, !layerling.ReadOnly(command.Operation)) {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if conn.WriteJSON(command) != nil {
				return
			}
		case <-tick.C:
			if !h.enabled() || !desktopWSAuthorizationValid(h.s, r, desktopScopeRead) {
				return
			}
		case <-editor.Done():
			return
		case <-ctx.Done():
			return
		case <-shutdown:
			return
		}
	}
}
