package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/chibisensei/chatrt"
)

func chibiAdminToken(t *testing.T, s *Server) string {
	t.Helper()
	tok, _, err := s.TokenManager.Create("chibi admin", []string{"admin", desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func chibiRuntimeState(t *testing.T, s *Server, token string) map[string]any {
	t.Helper()
	w := chibiDo(t, s, token, http.MethodGet, "/runtime", nil)
	chibiWant(t, w, http.StatusOK, "")
	return chibiBody(t, w)["runtime"].(map[string]any)
}

func chibiWaitRuntimeState(t *testing.T, s *Server, token, want string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		rt := chibiRuntimeState(t, s, token)
		if rt["state"] == want {
			return rt
		}
		if time.Now().After(deadline) {
			t.Fatalf("runtime state = %v, want %s", rt["state"], want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestChibiSenseiRuntimeRoutes(t *testing.T) {
	s, token, _, _, _ := chibiTestFixture(t, false)
	admin := chibiAdminToken(t, s)

	rt := chibiRuntimeState(t, s, token)
	dl := rt["download"].(map[string]any)
	lim := rt["limits"].(map[string]any)
	if rt["state"] != "not_installed" || rt["build"] != "b9994" || dl["bytes"] != float64(15857307) || dl["host"] != "github.com" ||
		dl["done_bytes"] != float64(0) || lim["max_instances"] != float64(2) || lim["idle_minutes"] != float64(10) ||
		lim["context_size"] != float64(4096) || rt["backend"].(map[string]any)["requested"] != "auto" || rt["notice_code"] != "" {
		t.Fatalf("runtime = %v", rt)
	}
	if insts, ok := rt["instances"].([]any); !ok || len(insts) != 0 {
		t.Fatalf("instances = %v", rt["instances"])
	}

	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runtime/install", nil), http.StatusForbidden, "admin_required")
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runtime/install", nil), http.StatusMethodNotAllowed, "method_not_allowed")
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runtime/nope", nil), http.StatusNotFound, "not_found")

	// A header-less caller needs a matching Origin (login off), as on the other admin writes.
	saved := s.ConfigSnapshot()
	open := *saved
	open.Auth.Enabled = false
	s.replaceConfigSnapshot(&open)
	chibiWant(t, chibiDo(t, s, "", http.MethodPost, "/runtime/install", nil, "Origin", "https://evil.example"), http.StatusForbidden, "csrf_check_failed")
	s.replaceConfigSnapshot(saved)

	w := chibiDo(t, s, admin, http.MethodPost, "/runtime/install", nil)
	chibiWant(t, w, http.StatusAccepted, "")
	if st := chibiBody(t, w)["runtime"].(map[string]any)["state"]; st != "installing" && st != "ready" {
		t.Fatalf("state after install = %v", st) // the fake installs instantly, so the answer may already say ready
	}
	rt = chibiWaitRuntimeState(t, s, token, "ready")
	if rt["download"].(map[string]any)["error_code"] != "" {
		t.Fatalf("runtime = %v", rt)
	}
	chibiWant(t, chibiDo(t, s, admin, http.MethodPost, "/runtime/install", nil), http.StatusAccepted, "") // idempotent when installed

	// A read-only desktop still stops the models but refuses a download.
	ro := *s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&ro)
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/runtime/stop", nil), http.StatusOK, "")
	chibiWant(t, chibiDo(t, s, admin, http.MethodPost, "/runtime/install", nil), http.StatusForbidden, "desktop_readonly")
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/runtime", nil), http.StatusOK, "")
}

func TestChibiSenseiRuntimeDisabled(t *testing.T) {
	s, token, _, _, _ := chibiTestFixture(t, false)
	admin := chibiAdminToken(t, s)
	cfg := *s.ConfigSnapshot()
	cfg.ChibiSensei.Runtime.Enabled = false
	s.replaceConfigSnapshot(&cfg)
	if rt := chibiRuntimeState(t, s, token); rt["state"] != "disabled" {
		t.Fatalf("runtime = %v", rt)
	}
	chibiWant(t, chibiDo(t, s, admin, http.MethodPost, "/runtime/install", nil), http.StatusServiceUnavailable, "runtime_unavailable")
}

func TestChibiRuntimeErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{chatrt.ErrDisabled, http.StatusServiceUnavailable, "runtime_unavailable"},
		{chatrt.ErrUnsupported, http.StatusServiceUnavailable, "runtime_unavailable"},
		{chatrt.ErrClosed, http.StatusServiceUnavailable, "runtime_unavailable"},
		{chatrt.ErrNotInstalled, http.StatusConflict, "runtime_not_installed"},
		{chatrt.ErrInstalling, http.StatusConflict, "runtime_busy"},
		{chatrt.ErrBusy, http.StatusConflict, "runtime_busy"},
		{chatrt.ErrModelMissing, http.StatusNotFound, "model_missing"},
		{errors.New("upstream said: secret-token-123"), http.StatusServiceUnavailable, "runtime_unavailable"},
	} {
		w := httptest.NewRecorder()
		chibiRuntimeError(w, tc.err)
		chibiWant(t, w, tc.status, tc.code)
		if got := w.Body.String(); strings.Contains(got, "secret-token-123") {
			t.Errorf("upstream error text reaches the answer: %s", got)
		}
	}
}

func TestChibiSenseiSmokerAdapter(t *testing.T) {
	s, token, fake, rt, pid := chibiTestFixture(t, false)
	admin := chibiAdminToken(t, s)
	run := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	other1 := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	other2 := chibiSucceededTestRun(t, rt, pid, "qwen35-0p8b", false, 0)
	r, err := rt.store.GetRun(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	tuned, _, err := chibisensei.RunModels(context.Background(), rt.store, rt.dataDir, r)
	if err != nil || !tuned.Available {
		t.Fatalf("tuned = %+v %v", tuned, err)
	}
	prof, _ := rt.catalog.Get("qwen35-0p8b")
	smoke := chibiSmoker{chat: rt.chat}
	in := chibisensei.SmokeInput{RunID: run, ModelPath: tuned.Path, ModelKey: tuned.Key, ModelBytes: tuned.Bytes, Profile: prof}

	if res := smoke.Smoke(context.Background(), in); res.State != "skipped" || res.Code != "runtime_not_installed" {
		t.Fatalf("not installed: %+v", res)
	}
	chibiWant(t, chibiDo(t, s, admin, http.MethodPost, "/runtime/install", nil), http.StatusAccepted, "")
	chibiWaitRuntimeState(t, s, token, "ready")
	fake.Replies["tuned:"] = "Hello from the tuned model."
	res := smoke.Smoke(context.Background(), in)
	if res.State != "ok" || res.Text != "Hello from the tuned model." || res.MS < 1 {
		t.Fatalf("installed: %+v", res)
	}
	reqs := fake.Requests("tuned:")
	if len(reqs) != 1 || reqs[0].Sampling.MaxTokens != 16 || reqs[0].Sampling.TopK != 20 || len(reqs[0].Messages) != 1 {
		t.Fatalf("smoke request = %+v", reqs)
	}

	fake.ChatErr["tuned:"] = errors.New("boom with secret-token-123")
	if res := smoke.Smoke(context.Background(), in); res.State != "failed" || res.Code != "generation_failed" || res.Text != "" {
		t.Fatalf("chat error: %+v", res)
	}
	delete(fake.ChatErr, "tuned:")

	// Both slots held by other models: skipped, not failed.
	stop := chibiHoldTwo(t, s, token, fake, rt, other1, other2)
	defer stop()
	if res := smoke.Smoke(context.Background(), in); res.State != "skipped" || res.Code != "runtime_busy" {
		t.Fatalf("busy: %+v", res)
	}
	stop()

	rt.chat.Close()
	if res := smoke.Smoke(context.Background(), in); res.State != "skipped" || res.Code != "runtime_disabled" {
		t.Fatalf("closed: %+v", res)
	}
}
