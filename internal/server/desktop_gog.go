package server

import (
	"bytes"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/gog"
	"aurago/internal/tools"
)

// Desktop calls share the catalog, credentials and live permissions with agents.
func handleDesktopGog(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopOperation(s, w, r, desktopScopeAdmin, desktopRead) {
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		action := strings.TrimPrefix(r.URL.Path, "/api/desktop/integrations/gog/")
		cfg := s.ConfigSnapshot()
		if action == "accounts" && r.Method == http.MethodGet {
			accounts := []map[string]any{}
			if cfg.Gog.Enabled {
				for _, a := range cfg.Gog.Accounts {
					if a.Enabled && a.DesktopAccess {
						accounts = append(accounts, map[string]any{"id": a.ID, "name": a.Name, "services": a.Services})
					}
				}
			}
			writeJSON(w, map[string]any{"accounts": accounts, "readonly": cfg.Gog.ReadOnly || cfg.VirtualDesktop.ReadOnly, "desktop_readonly": cfg.VirtualDesktop.ReadOnly})
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", 405)
			return
		}
		var body struct {
			gog.Request
			Path       string `json:"path,omitempty"`
			ResourceID string `json:"resource_id,omitempty"`
			ParentID   string `json:"parent_id,omitempty"`
		}
		if e := decodeGogRequest(w, r, &body); e != nil {
			jsonError(w, e.Error(), 400)
			return
		}
		ctx, cancel := s.storageContext(r.Context())
		defer cancel()
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
		ctx = tools.WithRuntimePermissions(ctx, tools.RuntimePermissionsFromConfig(cfg))
		svc := s.gogService()
		if action == "call" {
			op, ok := gog.Lookup(body.Service, body.Operation)
			if !ok || op.Input || op.Output {
				jsonError(w, "Unsupported desktop operation", 400)
				return
			}
			if op.Grant != "read" && !requireDesktopOperation(s, w, r, desktopScopeAdmin, desktopWrite) {
				return
			}
			writeJSON(w, svc.Execute(ctx, "desktop", body.Request, nil))
			return
		}
		if action != "office/import" && action != "office/export" {
			http.NotFound(w, r)
			return
		}
		if !requireDesktopOperation(s, w, r, desktopScopeAdmin, desktopWrite) {
			return
		}
		formats := map[string]string{"docs": "docx", "sheets": "xlsx", "slides": "pdf"}
		format, ok := formats[body.Service]
		if !ok || body.Path == "" || strings.ToLower(filepath.Ext(body.Path)) != "."+format {
			jsonError(w, "Choose a new file with the matching Office extension", 400)
			return
		}
		local, _, e := s.getDesktopService(ctx)
		if e != nil {
			storageError(w, e)
			return
		}
		maxBytes := int64(local.Config().MaxFileSizeMB) << 20
		if maxBytes > int64(gog.MaxFile) {
			maxBytes = int64(gog.MaxFile)
		}
		if action == "office/import" {
			idField := map[string]string{"docs": "doc_id", "sheets": "spreadsheet_id", "slides": "presentation_id"}[body.Service]
			req := gog.Request{AccountID: body.AccountID, Service: body.Service, Operation: "export", Args: map[string]any{idField: body.ResourceID, "format": format}}
			var buf bytes.Buffer
			result := svc.Execute(ctx, "desktop", req, &gog.Files{Output: &buf})
			if result.Status != "ok" {
				writeJSON(w, result)
				return
			}
			entry, err := local.WriteFileStreamConditional(ctx, body.Path, &buf, maxBytes, "gog-import", func(state desktop.FileWriteState) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if state.Exists {
					return tools.ErrStorageConflict
				}
				_, err := gog.Authorize(s.ConfigSnapshot(), "desktop", req, "read")
				return err
			})
			if err != nil {
				storageError(w, err)
				return
			}
			writeJSON(w, map[string]any{"status": "ok", "file": entry})
			return
		}
		if body.Service == "slides" {
			jsonError(w, "Slides exports use the agent", 400)
			return
		}
		if _, e = gog.Authorize(s.ConfigSnapshot(), "desktop", body.Request, "create"); e != nil {
			jsonError(w, e.Error(), 403)
			return
		}
		file, _, _, e := local.OpenPreviewFile(ctx, body.Path)
		if e != nil {
			storageError(w, e)
			return
		}
		defer file.Close()
		parent := body.ParentID
		if parent == "" {
			parent = "root"
		}
		req := gog.Request{AccountID: body.AccountID, Service: "drive", Operation: "import_office", Args: map[string]any{"parent": parent, "name": strings.TrimSuffix(filepath.Base(body.Path), filepath.Ext(body.Path)), "convert_to": map[string]string{"docs": "doc", "sheets": "sheet"}[body.Service]}}
		writeJSON(w, svc.Execute(ctx, "desktop", req, &gog.Files{Input: storageContextReader{ctx, file}, Name: filepath.Base(body.Path)}))
	}
}
