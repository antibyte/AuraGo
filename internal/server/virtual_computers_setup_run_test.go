package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/virtualcomputers"
)

func isolateVirtualComputersSetupRun(t *testing.T) {
	t.Helper()
	oldLoad, oldSave, oldInstall := virtualComputersLoadSetupState, virtualComputersSaveSetupState, virtualComputersSetupInstall
	oldPath := virtualComputersSetupStatePath
	virtualComputersSetupStatePath = filepath.Join(t.TempDir(), "vc_setup_state.json")
	virtualComputersLoadSetupState = func() (virtualComputersSetupState, error) { return virtualComputersSetupState{}, nil }
	virtualComputersSaveSetupState = func(virtualComputersSetupState) error { return nil }
	virtualComputersSetupRun.Lock()
	virtualComputersSetupRun.running, virtualComputersSetupRun.lastState, virtualComputersSetupRun.lastMessage = false, "", ""
	virtualComputersSetupRun.startedAt, virtualComputersSetupRun.finishedAt = time.Time{}, time.Time{}
	virtualComputersSetupRun.Unlock()
	resetVirtualComputersRepairProbe()
	t.Cleanup(func() {
		virtualComputersLoadSetupState, virtualComputersSaveSetupState, virtualComputersSetupInstall = oldLoad, oldSave, oldInstall
		virtualComputersSetupStatePath = oldPath
		waitVirtualComputersSetupIdle(t)
		virtualComputersSetupRun.Lock()
		virtualComputersSetupRun.running, virtualComputersSetupRun.lastState, virtualComputersSetupRun.lastMessage = false, "", ""
		virtualComputersSetupRun.Unlock()
		resetVirtualComputersRepairProbe()
	})
}

func waitVirtualComputersSetupIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !virtualComputersSetupRunning() && virtualComputersManualSetup.TryLock() {
			virtualComputersManualSetup.Unlock()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("setup run did not finish")
}

func setupStatusPayload(t *testing.T, s *Server) map[string]interface{} {
	t.Helper()
	rec := httptest.NewRecorder()
	handleVirtualComputersSetupStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/virtual-computers/setup/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	return decodeVolumeTestBody(t, rec)
}

// The install is bound to the process, not to the request: a dropped client neither cancels it nor loses the outcome.
func TestVirtualComputersSetupInstallSurvivesClientDisconnect(t *testing.T) {
	isolateVirtualComputersSetupRun(t)
	_, upstream := newFakeMachinesBoringd(t)
	s, _, mux := newMachineTestServer(t, upstream.URL)
	s.Cfg.VirtualComputers.AutoSetup = true
	s.Cfg.VirtualComputers.AllowVolumes = false
	s.Cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost

	started := make(chan struct{})
	release := make(chan struct{})
	var installCtxErr atomic.Value
	virtualComputersSetupInstall = func(ctx context.Context, _ virtualcomputers.SetupManager) (virtualcomputers.SetupStatus, error) {
		close(started)
		<-release
		installCtxErr.Store(ctx.Err() == nil)
		return virtualcomputers.SetupStatus{
			Healthy:   true,
			Preflight: virtualcomputers.PreflightResult{Checks: map[string]string{"HOST_OS": "linux", "ARCH": "amd64"}},
		}, nil
	}

	reqCtx, cancelReq := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/api/virtual-computers/setup/repair", strings.NewReader(`{}`)).WithContext(reqCtx)
	rec := httptest.NewRecorder()
	handlerDone := make(chan struct{})
	go func() {
		mux.ServeHTTP(rec, req)
		close(handlerDone)
	}()
	select {
	case <-started:
	case <-handlerDone:
		t.Fatalf("handler returned before install started: %d %s", rec.Code, rec.Body.String())
	case <-time.After(5 * time.Second):
		t.Fatal("install never started")
	}
	cancelReq() // the browser tab closes
	select {
	case <-handlerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("handler kept waiting after the client left")
	}

	running := setupStatusPayload(t, s)["setup"].(map[string]interface{})
	if running["running"] != true || running["action"] != "repair" || running["started_at"] == "" || running["finished_at"] != "" {
		t.Fatalf("setup during run = %v", running)
	}
	if payload := setupStatusPayload(t, s); payload["repair_required"] != false {
		t.Fatalf("repair_required during run = %v", payload["repair_required"])
	}
	// A second manual run is refused while the first owns the lock.
	again := httptest.NewRecorder()
	mux.ServeHTTP(again, httptest.NewRequest(http.MethodPost, "/api/virtual-computers/setup/install", strings.NewReader(`{}`)))
	if again.Code != http.StatusConflict {
		t.Fatalf("second run status = %d body=%s", again.Code, again.Body.String())
	}

	close(release)
	waitVirtualComputersSetupIdle(t)
	if ok, _ := installCtxErr.Load().(bool); !ok {
		t.Fatal("install context was cancelled by the client disconnect")
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("handler wrote after the client left: %q", rec.Body.String())
	}
	done := setupStatusPayload(t, s)["setup"].(map[string]interface{})
	if done["running"] != false || done["last_state"] != "installed" || done["finished_at"] == "" || done["last_message"] == "" {
		t.Fatalf("setup after run = %v", done)
	}
}

func TestVirtualComputersSetupStatusReportsRepairRequired(t *testing.T) {
	isolateVirtualComputersSetupRun(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/workspace/capabilities" {
			_ = json.NewEncoder(w).Encode(virtualcomputers.WorkspaceControlPlaneStatus{
				ProtocolVersion: virtualcomputers.WorkspaceProtocolVersion, AssetFingerprint: "stale-fingerprint",
			})
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(upstream.Close)
	s, _, _ := newMachineTestServer(t, upstream.URL)
	s.Cfg.VirtualComputers.AgentControl.Enabled = true

	if payload := setupStatusPayload(t, s); payload["repair_required"] != true {
		t.Fatalf("repair_required = %v, want true for a stale control plane", payload["repair_required"])
	}
	virtualComputersBeginSetupRun("install")
	if payload := setupStatusPayload(t, s); payload["repair_required"] != false {
		t.Fatalf("repair_required while running = %v, want false", payload["repair_required"])
	}
	virtualComputersEndSetupRun("failed", "boom")
	payload := setupStatusPayload(t, s)
	setup := payload["setup"].(map[string]interface{})
	if payload["repair_required"] != true || setup["last_state"] != "failed" || setup["last_message"] != "boom" {
		t.Fatalf("after run: repair_required=%v setup=%v", payload["repair_required"], setup)
	}
}

func TestVirtualComputersAssignAgentRefusedWhileSetupRuns(t *testing.T) {
	isolateVirtualComputersSetupRun(t)
	_, upstream := newFakeMachinesBoringd(t)
	_, _, mux := newMachineTestServer(t, upstream.URL)
	virtualComputersBeginSetupRun("auto")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/virtual-computers/machines/vm-1/assign-agent", nil))
	if payload := decodeVolumeTestBody(t, rec); rec.Code != http.StatusConflict || payload["code"] != "setup_in_progress" {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	virtualComputersEndSetupRun("installed", "ok")
}
