package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatSessionDeleteReturnsTypedScopeConflict(t *testing.T) {
	server, _ := newChatPrivacyStatusFixture(t)
	session, err := server.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.ShortTermMem.PrepareChatPrivacyDeletion(t.Context(), session.ID, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/chat/sessions/"+session.ID+"?delete_memories=true", nil)
	handleDeleteChatSession(server).ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("scope mismatch status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "deletion_scope_conflict" {
		t.Fatalf("scope conflict code=%q body=%s", body.Code, response.Body.String())
	}
}
