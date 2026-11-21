package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktopstore"
	"aurago/internal/tools"
)

func TestStorePDFSeparatesGrantsAndPublishesNewFiles(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/general/split-pages" {
			t.Error("unexpected PDF request")
			w.WriteHeader(400)
			return
		}
		calls++
		fmt.Fprint(w, "%PDF-1.7\nresult")
	}))
	defer upstream.Close()
	s, _ := storeIntegrationFixture(t, "stirling-pdf")
	cfg := s.ConfigSnapshot().Clone()
	root := t.TempDir()
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(root, "desktop")
	cfg.SQLite.VirtualDesktopPath = filepath.Join(root, "desktop.db")
	cfg.Directories.DataDir = root
	cfg.Agent.AllowFilesystemWrite = true
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(cfg)
	s.CfgMu.Unlock()
	previous, configured := tools.EffectiveRuntimePermissions(nil)
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(cfg))
	t.Cleanup(func() {
		if configured {
			tools.ConfigureRuntimePermissions(previous)
		} else {
			tools.ClearRuntimePermissionsForTest()
		}
	})
	svc, _, err := s.getDesktopService(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })
	const source = "Shared/input.pdf"
	const original = "%PDF-1.7\noriginal"
	if err = svc.WriteFile(t.Context(), source, original, "user"); err != nil {
		t.Fatal(err)
	}
	settings := desktopstore.IntegrationSettings{URL: upstream.URL, Credential: "pdf-test-key", Revision: "test-revision", Read: true}
	raw, _ := json.Marshal(settings)
	if err = s.Vault.WriteSecret(desktopstore.IntegrationKey("stirling-pdf"), string(raw)); err != nil {
		t.Fatal(err)
	}
	lifetime, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.storeIntegrations.bindings = map[string]storeIntegrationBinding{"stirling-pdf": {settings: settings, ctx: lifetime, cancel: cancel}}
	req := tools.StoreAppRequest{Operation: "split", Files: []string{source}, Pages: "all"}
	if _, err = s.executeStoreApplication(t.Context(), "stirling-pdf", req); err == nil || calls != 0 {
		t.Fatal("missing PDF grant ignored")
	}
	settings.Process = true
	s.storeIntegrations.bindings["stirling-pdf"] = storeIntegrationBinding{settings: settings, ctx: lifetime, cancel: cancel}
	result, err := s.executeStoreApplication(t.Context(), "stirling-pdf", req)
	if err != nil {
		t.Fatal(err)
	}
	dest := result.(map[string]any)["path"].(string)
	if !strings.HasPrefix(dest, "Shared/Store/stirling-pdf/") || dest == source {
		t.Fatalf("invalid output %q", dest)
	}
	data, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, filepath.FromSlash(source)))
	if err != nil || string(data) != original {
		t.Fatal("source changed")
	}
	if _, err = s.executeStoreApplication(t.Context(), "stirling-pdf", tools.StoreAppRequest{Operation: "cert_sign", Files: req.Files}); err == nil || calls != 1 {
		t.Fatal("processing granted signing")
	}
	blocked := tools.WithRuntimePermissions(t.Context(), tools.RuntimePermissions{AllowFilesystemWrite: false})
	if _, err = s.publishStoreFile(blocked, svc, "filebrowser", ".bin", []byte("data")); err == nil {
		t.Fatal("captured publication gate ignored")
	}
	cancel()
	if _, err = s.executeStoreApplication(t.Context(), "stirling-pdf", req); err == nil || calls != 1 {
		t.Fatal("revoked PDF call")
	}
}
