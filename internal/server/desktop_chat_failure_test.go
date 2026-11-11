package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/tools"
)

func TestDesktopStopBypassesSessionTurnAndQuestion(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "stream"}[stream], func(t *testing.T) {
			s := newTestDesktopChatServer(t)
			s.Cfg.VirtualDesktop.ReadOnly = true // Cleanup remains available after revocation.
			active, release, err := s.ShortTermMem.BeginSessionTurn(context.Background(), desktopChatSessionID)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			tools.RegisterQuestion(desktopChatSessionID, &tools.PendingQuestion{Question: "Continue?", AllowFreeText: true})
			defer tools.CancelQuestion(desktopChatSessionID)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/desktop/chat", bytes.NewBufferString(`{"message":"/stop"}`))
			done := make(chan struct{})
			go func() {
				defer close(done)
				if stream {
					handleDesktopChatStream(s)(rec, req)
				} else {
					handleDesktopChat(s)(rec, req)
				}
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("stop waited behind active session")
			}
			if rec.Code != http.StatusOK || active.Err() != context.Canceled || tools.HasPendingQuestion(desktopChatSessionID) {
				t.Fatalf("stop failed: status=%d cancelled=%v pending=%v", rec.Code, active.Err(), tools.HasPendingQuestion(desktopChatSessionID))
			}
		})
	}
}

func TestDesktopPreparationFailureClosesStreamWithError(t *testing.T) {
	s := newTestDesktopChatServer(t) // No LLM client: preparation must fail without a model call.
	rec := httptest.NewRecorder()
	handleDesktopChatStream(s)(rec, httptest.NewRequest(http.MethodPost, "/api/desktop/chat/stream", bytes.NewBufferString(`{"message":"hello"}`)))
	body := rec.Body.String()
	for _, marker := range []string{`"event":"llm_stream_done"`, `"finish_reason":"error"`, `"error":`, "[DONE]"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("missing %s: %s", marker, body)
		}
	}
	if strings.Contains(body, `"event":"final_response"`) {
		t.Fatal("failure replayed an earlier assistant response")
	}
}
