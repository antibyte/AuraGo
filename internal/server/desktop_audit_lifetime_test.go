package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/rtlsdr"
)

func TestDesktopReadonlyCleanupRoutes(t *testing.T) {
	s := rtlSDRServer(t)
	if err := s.RTLSDR.Tune(context.Background(), "browser", rtlsdr.DefaultTuning()); err != nil {
		t.Fatal(err)
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/api/desktop/rtl-sdr/stop", `{"client":"browser"}`, 200},
		{"DELETE", "/api/desktop/rtl-sdr/scan", "", 202},
		{"POST", "/api/desktop/rtl-sdr/tune", `{"client":"browser"}`, 403},
	} {
		w := httptest.NewRecorder()
		s.handleRTLSDR(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/api/game-maker/jobs/job/cancel", "/api/desktop/personal-radio/stations/id/stop", "/api/desktop/personal-radio/stations/id/pause", "/api/desktop/looper/stop", "/api/desktop/looper/pause"} {
		if op := desktopRequestOperation(httptest.NewRequest("POST", path, nil)); op != desktopStop {
			t.Errorf("%s classified %d", path, op)
		}
	}
	for _, path := range []string{"/api/game-maker/jobs/job/cancel/extra", "/api/desktop/personal-radio/stations/id/resume"} {
		if op := desktopRequestOperation(httptest.NewRequest("POST", path, nil)); op == desktopStop {
			t.Errorf("unsafe cleanup alias %s", path)
		}
	}
}

func TestDesktopRevokedChatBrokerDropsLateProviderEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	b := &desktopStreamCombinedBroker{ctx: ctx, stream: &desktopStreamBroker{w: w}}
	// A nil global broker would panic if any revoked event reached it.
	b.Send("message", "late")
	b.SendJSON(`{"late":true}`)
	b.SendLLMStreamDelta("late", "", "", 0, "")
	b.SendLLMStreamDone("stop")
	b.SendTokenUpdate(1, 1, 2, 2, 2, false, true, "")
	b.SendThinkingBlock("", "late", "")
	if delivered, _ := b.SendTypedWithTransport("late", map[string]bool{"late": true}); delivered {
		t.Fatal("late event delivered")
	}
	if w.Body.Len() != 0 {
		t.Fatal("late direct stream output")
	}
}
