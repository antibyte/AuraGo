package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/memory"
)

func TestChatSessionHandlersStripAgodeskAttachmentBlocks(t *testing.T) {
	s := newTestDesktopChatServer(t)
	sess, err := s.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}
	leaked := "konvertiere nach jpg und sende es\n\n<agodesk_attachments>\nmaja.png | image/png | 218432 bytes | agent_workspace/workdir/attachments/agodesk/sess-1/att-1/maja.png\n</agodesk_attachments>"
	if _, err := s.ShortTermMem.InsertMessage(sess.ID, "user", leaked, false, false); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	if err := s.ShortTermMem.UpdateChatSessionPreview(sess.ID); err != nil {
		t.Fatalf("UpdateChatSessionPreview: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sess.ID, nil)
	handleGetChatSession(s)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get session status = %d body=%s", rec.Code, rec.Body.String())
	}
	var getPayload struct {
		Status   string                  `json:"status"`
		Session  memory.ChatSession      `json:"session"`
		Messages []memory.HistoryMessage `json:"messages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&getPayload); err != nil {
		t.Fatalf("decode get session: %v", err)
	}
	if len(getPayload.Messages) != 1 {
		t.Fatalf("messages = %+v, want one", getPayload.Messages)
	}
	got := getPayload.Messages[0].Content
	if got != "konvertiere nach jpg und sende es" {
		t.Fatalf("sanitized message = %q", got)
	}
	if strings.Contains(got, "agodesk_attachments") || strings.Contains(got, "agent_workspace/workdir/attachments") {
		t.Fatalf("get session leaked agodesk attachment context: %q", got)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/chat/sessions", nil)
	handleListChatSessions(s)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list sessions status = %d body=%s", rec.Code, rec.Body.String())
	}
	var listPayload struct {
		Status   string               `json:"status"`
		Sessions []memory.ChatSession `json:"sessions"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list sessions: %v", err)
	}
	if len(listPayload.Sessions) == 0 {
		t.Fatal("list sessions returned no sessions")
	}
	for _, listed := range listPayload.Sessions {
		if listed.ID != sess.ID {
			continue
		}
		if strings.Contains(listed.Preview, "agodesk_attachments") || strings.Contains(listed.Preview, "agent_workspace/workdir/attachments") {
			t.Fatalf("session preview leaked agodesk attachment context: %q", listed.Preview)
		}
		return
	}
	t.Fatalf("created session %q not found in list: %+v", sess.ID, listPayload.Sessions)
}
