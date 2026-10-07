package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// A run cancel stops work. Main's Desktop contract lets stop and cancel routes through a
// read-only desktop (desktopStop, as Mission Control's cancel); read-only missions still
// refuse it, mirroring CancelCheck, and the FF1 admin bearer scope still applies.

// fuStartWaitingRun publishes and switches on the waiting flow, starts a live run and waits
// until it runs. It returns the run id.
func fuStartWaitingRun(t *testing.T, s *Server, token string) string {
	t.Helper()
	rec := createTestFlow(t, s, waitFlowJSON)
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/publish", token, `{"base_revision":1}`); w.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/enabled", token, `{"enabled":true}`); w.Code != http.StatusOK {
		t.Fatalf("enable = %d %s", w.Code, w.Body.String())
	}
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/run", token, "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("run = %d %s", w.Code, w.Body.String())
	}
	runID := flowsBody(t, w)["run_id"].(string)
	waitFlowRunStatus(t, s, token, runID, "running")
	return runID
}

func TestFUReadOnlyDesktopStillCancelsFlowRuns(t *testing.T) {
	s, token := newFlowsTestServer(t)
	runID := fuStartWaitingRun(t, s, token)
	s.Cfg.VirtualDesktop.ReadOnly = true

	// Other writes stay refused with the flows API's code.
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"name":"x"}`); w.Code != http.StatusForbidden || flowsBody(t, w)["code"] != "FLOW_PERMISSION_DENIED" {
		t.Fatalf("create on a read-only desktop = %d %s", w.Code, w.Body.String())
	}
	// The cancel still needs the admin scope (FF1).
	writeToken, _, err := s.TokenManager.Create("fu write", []string{desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/runs/"+runID+"/cancel", writeToken, ""); w.Code != http.StatusForbidden {
		t.Fatalf("cancel with a write token = %d %s, want 403", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/runs/"+runID+"/cancel", token, ""); w.Code != http.StatusAccepted || flowsBody(t, w)["cancelled"] != true {
		t.Fatalf("cancel on a read-only desktop = %d %s, want 202", w.Code, w.Body.String())
	}
	waitFlowRunStatus(t, s, token, runID, "cancelled")
}

func TestFUReadOnlyMissionsRefuseFlowRunCancel(t *testing.T) {
	s, token := newFlowsTestServer(t)
	runID := fuStartWaitingRun(t, s, token)
	s.Cfg.Tools.Missions.ReadOnly = true
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/runs/"+runID+"/cancel", token, "")
	if w.Code != http.StatusForbidden || flowsBody(t, w)["code"] != "FLOW_PERMISSION_DENIED" {
		t.Fatalf("cancel with read-only missions = %d %s, want 403 FLOW_PERMISSION_DENIED", w.Code, w.Body.String())
	}
	s.Cfg.Tools.Missions.ReadOnly = false
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/runs/"+runID+"/cancel", token, ""); w.Code != http.StatusAccepted {
		t.Fatalf("cancel after missions are writable again = %d %s", w.Code, w.Body.String())
	}
	waitFlowRunStatus(t, s, token, runID, "cancelled")
}

func TestFUFlowsDesktopOperation(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         desktopOperation
	}{
		{http.MethodGet, "/api/desktop/flows/f1", desktopRead},
		{http.MethodHead, "/api/desktop/flows/node-types", desktopRead},
		{http.MethodPost, "/api/desktop/flows/validate", desktopRead},
		{http.MethodPost, "/api/desktop/flows/runs/run_a/cancel", desktopStop},
		{http.MethodGet, "/api/desktop/flows/runs/run_a/cancel", desktopRead},
		{http.MethodPost, "/api/desktop/flows/runs/run_a/cancel/x", desktopWrite},
		{http.MethodPost, "/api/desktop/flows/f1/cancel", desktopWrite},
		{http.MethodPost, "/api/desktop/flows/f1/run", desktopWrite},
		{http.MethodDelete, "/api/desktop/flows/f1", desktopWrite},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		if got := flowsDesktopOperation(r, flowsPathParts(tc.path)); got != tc.want {
			t.Errorf("flowsDesktopOperation(%s %s) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}

// fuWebhookFlowJSON is a flow with a webhook trigger: switching it on registers the webhook in
// Mission Control while the flow's lock is held.
const fuWebhookFlowJSON = `{"schema":1,"name":"Hook","nodes":[
 {"id":"n_aaaaaaaa","key":"hook","type":"trigger.webhook","type_version":1,"label":"Hook","position":{"x":0,"y":0},"params":{"webhook":"wh_fu"}},
 {"id":"n_bbbbbbbb","key":"set","type":"logic.set","type_version":1,"label":"Set","position":{"x":300,"y":0},
  "params":{"fields":[{"name":"x","value":"y"}]}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`

// fuBlockingWebhooks holds the first webhook registration until release closes.
type fuBlockingWebhooks struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (f *fuBlockingWebhooks) RegisterMissionTriggerForKey(string, string, func([]byte)) {
	first := false
	f.once.Do(func() { first = true })
	if first {
		close(f.entered)
		<-f.release
	}
}

func (f *fuBlockingWebhooks) UnregisterMissionTrigger(string) {}

// A flows write runs under a Desktop grant: switching the desktop to read-only
// (revokeDesktopRuns) cancels a Publish that waits for the flow's lock, which answers 503.
func TestFUFlowsWriteCarriesARevocableGrant(t *testing.T) {
	s, token := newFlowsTestServer(t)
	webhooks := &fuBlockingWebhooks{entered: make(chan struct{}), release: make(chan struct{})}
	s.MissionManagerV2.SetWebhookManager(webhooks)
	rec := createTestFlow(t, s, fuWebhookFlowJSON)
	pub, _, err := s.Flows.Publish(context.Background(), rec.ID, rec.DraftRevision)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	// A first API call fills the catalog cache, so the request below only waits for the lock.
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+rec.ID, token, ""); w.Code != http.StatusOK {
		t.Fatalf("get = %d %s", w.Code, w.Body.String())
	}
	enabled := make(chan error, 1)
	go func() { enabled <- s.Flows.SetEnabled(context.Background(), rec.ID, true) }()
	released := false
	release := func() {
		if !released {
			released = true
			close(webhooks.release)
		}
	}
	defer release()
	select {
	case <-webhooks.entered: // SetEnabled holds the flow's lock now
	case <-time.After(5 * time.Second):
		t.Fatal("the webhook registration did not start")
	}
	answer := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		answer <- flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/publish", token, `{"base_revision":`+strconv.Itoa(pub.DraftRevision)+`}`)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.desktopRuns.mu.Lock()
		admitted := len(s.desktopRuns.runs)
		s.desktopRuns.mu.Unlock()
		if admitted == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the publish request holds %d Desktop grants, want 1", admitted)
		}
		time.Sleep(5 * time.Millisecond)
	}
	s.revokeDesktopRuns()
	select {
	case w := <-answer:
		if body := flowsBody(t, w); w.Code != http.StatusServiceUnavailable || body["code"] != "FLOWS_DISABLED" ||
			!strings.Contains(w.Body.String(), "the request was cancelled") {
			t.Fatalf("revoked publish = %d %s, want 503 the request was cancelled", w.Code, w.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the revocation did not end the publish that waits for the lock")
	}
	release()
	if err := <-enabled; err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
}
