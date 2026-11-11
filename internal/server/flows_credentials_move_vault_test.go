package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// bumpDraft saves the draft once more, so the move that follows loses its revision (a concurrent save).
func bumpDraft(t *testing.T, s *Server, flowID string) {
	t.Helper()
	rec, err := s.Flows.GetFlow(t.Context(), flowID)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := rec.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.Name = "Wetter, geaendert"
	if _, _, err := s.Flows.SaveDraft(t.Context(), flowID, doc, rec.DraftRevision); err != nil {
		t.Fatal(err)
	}
}

func readFlowSecret(s *Server, name string) (string, error) {
	return (flowSecrets{s: s}).ReadSecret(name)
}

func TestFlowCredentialMoveNeverLosesAVaultSecret(t *testing.T) {
	s, token := newFlowsTestServer(t)
	t.Cleanup(func() { flowCredentialMoveBeforeSave = func(string) {} })

	t.Run("a replace whose draft save conflicts puts the old value back", func(t *testing.T) {
		rec := createTestFlow(t, s, credentialMoveFlowJSON)
		if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/secrets/keep_me", token, `{"value":"old-value-123456"}`); w.Code != http.StatusOK {
			t.Fatalf("put secret = %d", w.Code)
		}
		flowCredentialMoveBeforeSave = func(id string) { bumpDraft(t, s, id) }
		w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "keep_me", true))
		flowCredentialMoveBeforeSave = func(string) {}
		if w.Code != http.StatusConflict {
			t.Fatalf("conflicting save = %d %s", w.Code, w.Body.String())
		}
		if v, err := readFlowSecret(s, "keep_me"); err != nil || v != "old-value-123456" {
			t.Fatalf("the vault holds %q (%v), want the old value back", v, err)
		}
	})

	t.Run("a new entry is removed again when the draft save conflicts", func(t *testing.T) {
		rec := createTestFlow(t, s, credentialMoveFlowJSON)
		flowCredentialMoveBeforeSave = func(id string) {
			if v, err := readFlowSecret(s, "fresh_one"); err != nil || v != credHeaderToken {
				t.Errorf("the new entry is missing before the save: %q %v", v, err)
			}
			bumpDraft(t, s, id)
		}
		w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "fresh_one", false))
		flowCredentialMoveBeforeSave = func(string) {}
		if w.Code != http.StatusConflict {
			t.Fatalf("conflicting save = %d %s", w.Code, w.Body.String())
		}
		if _, err := readFlowSecret(s, "fresh_one"); err == nil {
			t.Fatal("the vault entry of the failed move is still there")
		}
	})

	t.Run("two moves for one name run one after the other", func(t *testing.T) {
		a := createTestFlow(t, s, credentialMoveFlowJSON)
		b := createTestFlow(t, s, credentialMoveFlowJSON)
		inside, release := make(chan struct{}), make(chan struct{})
		flowCredentialMoveBeforeSave = func(string) { close(inside); <-release }
		first := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			first <- moveCall(t, s, token, a.ID, moveBody(a.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "shared_name", false))
		}()
		<-inside
		flowCredentialMoveBeforeSave = func(string) {}
		second := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			second <- moveCall(t, s, token, b.ID, moveBody(b.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "shared_name", false))
		}()
		select {
		case w := <-second:
			t.Fatalf("the second move ran while the first held the lock: %d %s", w.Code, w.Body.String())
		case <-time.After(300 * time.Millisecond):
		}
		close(release)
		if w := <-first; w.Code != http.StatusOK {
			t.Fatalf("first move = %d %s", w.Code, w.Body.String())
		}
		w := <-second
		if w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_SECRET_EXISTS" {
			t.Fatalf("second move = %d %s, want 409 FLOW_SECRET_EXISTS", w.Code, w.Body.String())
		}
		if v, err := readFlowSecret(s, "shared_name"); err != nil || v != credHeaderToken {
			t.Fatalf("the vault holds %q (%v)", v, err)
		}
	})
}
