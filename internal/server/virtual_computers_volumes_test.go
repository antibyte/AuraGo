package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/virtualcomputers"
)

// fakeRetentionBoringd is a minimal boringd for the volume routes. patched
// decides whether PATCH /v1/volumes/{id} exists (patch 0009).
type fakeRetentionBoringd struct {
	mu       sync.Mutex
	patched  bool
	requests []string
	volumes  map[string]map[string]interface{}
}

func newFakeRetentionBoringd(t *testing.T, patched bool) (*fakeRetentionBoringd, *httptest.Server) {
	t.Helper()
	fake := &fakeRetentionBoringd{patched: patched, volumes: map[string]map[string]interface{}{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		fake.mu.Lock()
		defer fake.mu.Unlock()
		fake.requests = append(fake.requests, r.Method+" "+r.URL.Path+" "+string(data))
		w.Header().Set("Content-Type", "application/json")
		id := strings.TrimPrefix(r.URL.Path, "/v1/volumes/")
		switch {
		case r.URL.Path == "/healthz":
			_, _ = io.WriteString(w, `{"ok":true}`)
		case r.URL.Path == "/v1/machines" && r.Method == http.MethodGet:
			_, _ = io.WriteString(w, `[]`)
		case r.Method == http.MethodPatch && id == "vol-0000000000":
			if !fake.patched {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"no such volume"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/volumes":
			var req map[string]interface{}
			_ = json.Unmarshal(data, &req)
			volume := map[string]interface{}{"id": "vol-created", "quota_mb": 2048, "used_bytes": 0, "files": 0,
				"persistent": req["persistent"] == true, "locked": req["locked"] == true,
				"expires_at": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339), "created_at": time.Now().UTC().Format(time.RFC3339)}
			if req["persistent"] == true {
				volume["expires_at"] = ""
			}
			fake.volumes["vol-created"] = volume
			_ = json.NewEncoder(w).Encode(volume)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/volumes/"):
			var req map[string]interface{}
			_ = json.Unmarshal(data, &req)
			volume := fake.volumes[id]
			if volume == nil {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"error":"no such volume"}`)
				return
			}
			for _, key := range []string{"persistent", "locked"} {
				if value, ok := req[key]; ok {
					volume[key] = value
				}
			}
			if volume["persistent"] == true {
				volume["expires_at"] = ""
			}
			_ = json.NewEncoder(w).Encode(volume)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/volumes/"):
			if volume := fake.volumes[id]; volume != nil {
				_ = json.NewEncoder(w).Encode(volume)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"no such volume"}`)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/volumes/"):
			delete(fake.volumes, id)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return fake, server
}

func (f *fakeRetentionBoringd) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, request := range f.requests {
		if strings.HasPrefix(request, prefix) {
			n++
		}
	}
	return n
}

func newVolumeTestServer(t *testing.T, upstreamURL string) (*Server, *virtualcomputers.Ledger, *http.ServeMux) {
	t.Helper()
	virtualcomputers.ResetVolumeRetentionCache()
	t.Cleanup(virtualcomputers.ResetVolumeRetentionCache)
	ledger, err := virtualcomputers.OpenLedger(filepath.Join(t.TempDir(), "virtual_computers.db"))
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
	cfg := virtualComputersTestConfig(upstreamURL)
	cfg.VirtualComputers.AllowVolumes = true
	cfg.VirtualComputers.Storage.VolumeMaxTTLDays = 7
	cfg.VirtualComputers.Storage.VolumeQuotaMB = 4096
	cfg.SQLite.VirtualComputersPath = filepath.Join(t.TempDir(), "unused.db")
	s := &Server{Cfg: cfg, VirtualComputersDB: ledger}
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)
	return s, ledger, mux
}

func volumeTestRequest(mux *http.ServeMux, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, reader))
	return rec
}

func decodeVolumeTestBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return payload
}

func TestVirtualComputersSetupStatusReportsVolumeLimitsAndRetention(t *testing.T) {
	for _, patched := range []bool{true, false} {
		_, upstream := newFakeRetentionBoringd(t, patched)
		s, _, _ := newVolumeTestServer(t, upstream.URL)
		s.Cfg.VirtualComputers.MaxTTLSeconds = 0
		rec := httptest.NewRecorder()
		handleVirtualComputersSetupStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/virtual-computers/setup/status", nil))
		var body struct {
			Capabilities map[string]bool `json:"capabilities"`
			Limits       map[string]int  `json:"limits"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("status=%d err=%v body=%s", rec.Code, err, rec.Body.String())
		}
		if body.Capabilities["volume_retention"] != patched {
			t.Fatalf("patched=%v volume_retention=%v", patched, body.Capabilities["volume_retention"])
		}
		if body.Limits["max_ttl_seconds"] != 900 || body.Limits["volume_max_ttl_seconds"] != 7*86400 || body.Limits["volume_quota_mb"] != 4096 ||
			body.Limits["default_ttl_seconds"] != 600 || body.Limits["max_forks"] != 3 {
			t.Fatalf("limits = %v", body.Limits)
		}
	}
}

func TestVirtualComputersVolumeCreateValidatesRetention(t *testing.T) {
	fake, upstream := newFakeRetentionBoringd(t, false)
	_, ledger, mux := newVolumeTestServer(t, upstream.URL)

	for _, tc := range []struct {
		name string
		body map[string]interface{}
		code string
		http int
	}{
		{"control character", map[string]interface{}{"name": "bad\nname", "ttl_seconds": 60}, "invalid_name", http.StatusBadRequest},
		{"too long name", map[string]interface{}{"name": strings.Repeat("x", 65), "ttl_seconds": 60}, "invalid_name", http.StatusBadRequest},
		{"ttl above maximum", map[string]interface{}{"ttl_seconds": 8 * 86400}, "volume_ttl_too_long", http.StatusBadRequest},
		{"permanent without patch", map[string]interface{}{"persistent": true}, "volume_retention_requires_repair", http.StatusConflict},
		{"locked without patch", map[string]interface{}{"ttl_seconds": 60, "locked": true}, "volume_retention_requires_repair", http.StatusConflict},
	} {
		rec := volumeTestRequest(mux, http.MethodPost, "/api/virtual-computers/volumes", tc.body)
		payload := decodeVolumeTestBody(t, rec)
		if rec.Code != tc.http || payload["code"] != tc.code || payload["status"] != "error" {
			t.Fatalf("%s: status=%d body=%s", tc.name, rec.Code, rec.Body.String())
		}
	}
	if fake.count("POST /v1/volumes") != 0 {
		t.Fatal("rejected requests must not reach boringd")
	}

	rec := volumeTestRequest(mux, http.MethodPost, "/api/virtual-computers/volumes", map[string]interface{}{"name": "  Backup  "})
	if rec.Code != http.StatusOK {
		t.Fatalf("plain create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := fake.count(`POST /v1/volumes {"ttl_seconds":86400}`); got != 1 {
		t.Fatalf("default ttl not forwarded as 86400: %v", fake.requests)
	}
	if tracked, ok, _ := ledger.GetVolume(context.Background(), "vol-created"); !ok || tracked.Name != "Backup" {
		t.Fatalf("tracked = %+v ok=%v", tracked, ok)
	}

	fake.mu.Lock()
	fake.patched = true
	fake.mu.Unlock()
	virtualcomputers.ResetVolumeRetentionCache()
	rec = volumeTestRequest(mux, http.MethodPost, "/api/virtual-computers/volumes", map[string]interface{}{"name": "Keep", "persistent": true, "locked": true})
	payload := decodeVolumeTestBody(t, rec)
	volume, _ := payload["volume"].(map[string]interface{})
	if rec.Code != http.StatusOK || volume["persistent"] != true || volume["locked"] != true {
		t.Fatalf("permanent create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.count(`POST /v1/volumes {"locked":true,"persistent":true}`) != 1 {
		t.Fatalf("permanent body: %v", fake.requests)
	}
}

func TestVirtualComputersVolumePatchRenamesAndUpdatesRetention(t *testing.T) {
	fake, upstream := newFakeRetentionBoringd(t, true)
	_, ledger, mux := newVolumeTestServer(t, upstream.URL)
	ctx := context.Background()
	fake.volumes["vol-1"] = map[string]interface{}{"id": "vol-1", "persistent": false, "locked": false, "quota_mb": 2048}
	if err := ledger.UpsertVolume(ctx, virtualcomputers.Volume{ID: "vol-1", Name: "Old", Format: virtualcomputers.WorkspaceVolumeFormat, Availability: "available"}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.UpsertVolume(ctx, virtualcomputers.Volume{ID: "vol-gone", Availability: "previous_store"}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, path string
		body       interface{}
		code       string
		http       int
	}{
		{"empty body", "vol-1", map[string]interface{}{}, "invalid_argument", http.StatusBadRequest},
		{"unknown volume", "vol-zzz", map[string]interface{}{"name": "x"}, "volume_not_found", http.StatusNotFound},
		{"previous store", "vol-gone", map[string]interface{}{"locked": true}, "volume_unavailable", http.StatusConflict},
		{"ttl too long", "vol-1", map[string]interface{}{"ttl_seconds": 8 * 86400}, "volume_ttl_too_long", http.StatusBadRequest},
		{"bad name", "vol-1", map[string]interface{}{"name": "a\x00b"}, "invalid_name", http.StatusBadRequest},
	} {
		rec := volumeTestRequest(mux, http.MethodPatch, "/api/virtual-computers/volumes/"+tc.path, tc.body)
		payload := decodeVolumeTestBody(t, rec)
		if rec.Code != tc.http || payload["code"] != tc.code {
			t.Fatalf("%s: status=%d body=%s", tc.name, rec.Code, rec.Body.String())
		}
	}

	rec := volumeTestRequest(mux, http.MethodPatch, "/api/virtual-computers/volumes/vol-1", map[string]interface{}{"name": "Renamed"})
	if rec.Code != http.StatusOK {
		t.Fatalf("rename status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.count("PATCH /v1/volumes/vol-1") != 0 {
		t.Fatal("a pure rename must stay local")
	}
	rec = volumeTestRequest(mux, http.MethodPatch, "/api/virtual-computers/volumes/vol-1", map[string]interface{}{"persistent": true, "locked": true, "name": "Final"})
	payload := decodeVolumeTestBody(t, rec)
	volume, _ := payload["volume"].(map[string]interface{})
	if rec.Code != http.StatusOK || volume["name"] != "Final" || volume["persistent"] != true || volume["locked"] != true {
		t.Fatalf("retention status=%d body=%s", rec.Code, rec.Body.String())
	}
	tracked, _, _ := ledger.GetVolume(ctx, "vol-1")
	if tracked.Name != "Final" || tracked.Format != virtualcomputers.WorkspaceVolumeFormat || !tracked.Persistent || !tracked.Locked || tracked.ExpiresAt != nil {
		t.Fatalf("tracked = %+v", tracked)
	}
	// Ending permanence sends a concrete retention instead of leaving the volume permanent.
	rec = volumeTestRequest(mux, http.MethodPatch, "/api/virtual-computers/volumes/vol-1", map[string]interface{}{"persistent": false})
	if rec.Code != http.StatusOK || fake.count(`PATCH /v1/volumes/vol-1 {"ttl_seconds":86400,"persistent":false}`) != 1 {
		t.Fatalf("unpin status=%d requests=%v", rec.Code, fake.requests)
	}

	// An unpatched control plane asks for Install / Repair.
	fake.mu.Lock()
	fake.patched = false
	fake.mu.Unlock()
	virtualcomputers.ResetVolumeRetentionCache()
	rec = volumeTestRequest(mux, http.MethodPatch, "/api/virtual-computers/volumes/vol-1", map[string]interface{}{"locked": true})
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusConflict || payload["code"] != "volume_retention_requires_repair" {
		t.Fatalf("unpatched status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVirtualComputersVolumeDeleteRefusesLockedAndInUse(t *testing.T) {
	fake, upstream := newFakeRetentionBoringd(t, true)
	_, ledger, mux := newVolumeTestServer(t, upstream.URL)
	ctx := context.Background()
	for _, volume := range []virtualcomputers.Volume{
		{ID: "vol-locked", Locked: true}, {ID: "vol-busy"}, {ID: "vol-free"},
	} {
		fake.volumes[volume.ID] = map[string]interface{}{"id": volume.ID}
		if err := ledger.UpsertVolume(ctx, volume); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	if err := ledger.UpsertWorkspace(ctx, virtualcomputers.Workspace{ID: "ws-1", OwnerSessionID: "s", MachineID: "vm-1", State: virtualcomputers.WorkspaceStateReady,
		VolumeID: "vol-busy", LeaseExpiresAt: now.Add(time.Hour), MaxExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	rec := volumeTestRequest(mux, http.MethodDelete, "/api/virtual-computers/volumes/vol-locked", nil)
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusConflict || payload["code"] != "volume_locked" {
		t.Fatalf("locked status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = volumeTestRequest(mux, http.MethodDelete, "/api/virtual-computers/volumes/vol-busy", nil)
	payload := decodeVolumeTestBody(t, rec)
	attached, _ := payload["attached_to"].([]interface{})
	if rec.Code != http.StatusConflict || payload["code"] != "volume_in_use" || len(attached) != 1 {
		t.Fatalf("busy status=%d body=%s", rec.Code, rec.Body.String())
	}
	if first, _ := attached[0].(map[string]interface{}); first["kind"] != "workspace" || first["id"] != "ws-1" {
		t.Fatalf("attached_to = %v", attached)
	}
	if fake.count("DELETE /v1/volumes/") != 0 {
		t.Fatal("refused deletes must not reach boringd")
	}
	rec = volumeTestRequest(mux, http.MethodDelete, "/api/virtual-computers/volumes/vol-free", nil)
	if rec.Code != http.StatusOK || fake.count("DELETE /v1/volumes/vol-free") != 1 {
		t.Fatalf("free status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok, _ := ledger.GetVolume(ctx, "vol-free"); ok {
		t.Fatal("ledger row must be removed after the upstream delete")
	}
	// Upstream 423 maps to volume_locked even when the ledger does not know the lock.
	rec = httptest.NewRecorder()
	writeVirtualComputersError(rec, virtualcomputers.RESTError{StatusCode: http.StatusLocked, Path: "/v1/volumes/x", Body: `{"error":"volume is locked"}`})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "volume_locked") {
		t.Fatalf("423 mapping status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVirtualComputersVolumeImportPreservesLocalFields(t *testing.T) {
	fake, upstream := newFakeRetentionBoringd(t, true)
	_, ledger, mux := newVolumeTestServer(t, upstream.URL)
	ctx := context.Background()
	fake.volumes["vol-1"] = map[string]interface{}{"id": "vol-1", "persistent": true, "locked": true, "used_bytes": 77, "files": 3, "expires_at": ""}
	if err := ledger.UpsertVolume(ctx, virtualcomputers.Volume{ID: "vol-1", Name: "Mine", Format: virtualcomputers.WorkspaceVolumeFormat, Availability: "available", StorageEpochID: 4}); err != nil {
		t.Fatal(err)
	}
	rec := volumeTestRequest(mux, http.MethodGet, "/api/virtual-computers/volumes/vol-1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	tracked, _, _ := ledger.GetVolume(ctx, "vol-1")
	if tracked.Name != "Mine" || tracked.Format != virtualcomputers.WorkspaceVolumeFormat || tracked.StorageEpochID != 4 || !tracked.Persistent || !tracked.Locked || tracked.UsedBytes != 77 || tracked.FileCount != 3 {
		t.Fatalf("tracked after GET = %+v", tracked)
	}
	// A volume boringd knows but the ledger never saw becomes legacy_root / available.
	fake.volumes["vol-2"] = map[string]interface{}{"id": "vol-2"}
	if rec := volumeTestRequest(mux, http.MethodGet, "/api/virtual-computers/volumes/vol-2", nil); rec.Code != http.StatusOK {
		t.Fatalf("import status=%d", rec.Code)
	}
	if imported, ok, _ := ledger.GetVolume(ctx, "vol-2"); !ok || imported.Format != "legacy_root" || imported.Availability != "available" {
		t.Fatalf("imported = %+v ok=%v", imported, ok)
	}
}

func TestVirtualComputersSetupOptionsCarriesVolumeLimits(t *testing.T) {
	opts := virtualComputersSetupOptions(virtualcomputers.ToolConfig{VolumeQuotaMB: 512, VolumeMaxTTLSeconds: 3 * 86400}, "token", virtualComputersSetupRequest{})
	if opts.VolumeQuotaMB != 512 || opts.VolumeMaxTTLSeconds != 3*86400 {
		t.Fatalf("limits = %d / %d", opts.VolumeQuotaMB, opts.VolumeMaxTTLSeconds)
	}
	opts = virtualComputersSetupOptions(virtualcomputers.ToolConfig{}, "token", virtualComputersSetupRequest{})
	if opts.VolumeQuotaMB != 2048 || opts.VolumeMaxTTLSeconds != 30*86400 {
		t.Fatalf("default limits = %d / %d", opts.VolumeQuotaMB, opts.VolumeMaxTTLSeconds)
	}
}

func TestVirtualWorkspaceErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"plain error", errors.New("database exploded at /secret/path"), http.StatusInternalServerError, "workspace_error"},
		{"rest 413", virtualcomputers.RESTError{StatusCode: http.StatusRequestEntityTooLarge, Path: "/v1/machines/vm-1/save", Body: `{"error":"too big"}`}, http.StatusRequestEntityTooLarge, "volume_quota_exceeded"},
		{"timeout", context.DeadlineExceeded, http.StatusGatewayTimeout, "workspace_timeout"},
		{"not found", virtualcomputers.WorkspaceRPCError{Code: "not_found", Message: "x"}, http.StatusNotFound, "not_found"},
		{"forbidden", virtualcomputers.WorkspaceRPCError{Code: "forbidden", Message: "x"}, http.StatusForbidden, "forbidden"},
		{"upgrade", virtualcomputers.WorkspaceRPCError{Code: "workspace_agent_upgrade_required", Message: "x"}, http.StatusServiceUnavailable, "workspace_agent_upgrade_required"},
		{"control plane repair", virtualcomputers.WorkspaceRPCError{Code: "control_plane_repair_required", Message: "x"}, http.StatusServiceUnavailable, "control_plane_repair_required"},
		{"limit", virtualcomputers.WorkspaceRPCError{Code: "workspace_limit_reached", Message: "x"}, http.StatusConflict, "workspace_limit_reached"},
		{"owned machine", virtualcomputers.WorkspaceRPCError{Code: "machine_owned_by_workspace", Message: "x"}, http.StatusConflict, "machine_owned_by_workspace"},
		{"retention repair", virtualcomputers.WorkspaceRPCError{Code: "volume_retention_requires_repair", Message: "x"}, http.StatusConflict, "volume_retention_requires_repair"},
		{"readonly", virtualcomputers.WorkspaceRPCError{Code: "readonly", Message: "x"}, http.StatusForbidden, "readonly"},
		{"ttl", virtualcomputers.WorkspaceRPCError{Code: "volume_ttl_too_long", Message: "x"}, http.StatusBadRequest, "volume_ttl_too_long"},
		{"other rpc", virtualcomputers.WorkspaceRPCError{Code: "invalid_control_owner", Message: "x"}, http.StatusBadRequest, "invalid_control_owner"},
	} {
		rec := httptest.NewRecorder()
		writeVirtualWorkspaceError(rec, tc.err)
		payload := decodeVolumeTestBody(t, rec)
		if rec.Code != tc.status || payload["error"] != tc.code || payload["status"] != "error" {
			t.Fatalf("%s: status=%d body=%s", tc.name, rec.Code, rec.Body.String())
		}
		if tc.name == "plain error" && strings.Contains(rec.Body.String(), "/secret/path") {
			t.Fatalf("plain errors must not leak internals: %s", rec.Body.String())
		}
	}
}
