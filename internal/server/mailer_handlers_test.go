package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/mailer"
)

type mailerHTTPAdapter struct {
	messages    []mailer.Message
	mutations   []mailer.Mutation
	mutationErr error
}

func (a *mailerHTTPAdapter) Folders(context.Context) ([]mailer.Folder, error) {
	return []mailer.Folder{{ID: "inbox", Name: "Inbox", Kind: "inbox"}}, nil
}
func (a *mailerHTTPAdapter) Sync(context.Context, string, string) (mailer.SyncResult, error) {
	return mailer.SyncResult{Messages: a.messages}, nil
}
func (a *mailerHTTPAdapter) Read(context.Context, string, string) (mailer.Message, error) {
	return mailer.Message{Text: "A message body"}, nil
}
func (a *mailerHTTPAdapter) Raw(context.Context, string, string) ([]byte, error) {
	return []byte("Subject: fixture\r\n\r\nA message body"), nil
}
func (a *mailerHTTPAdapter) Attachment(context.Context, string, string, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("attachment")), nil
}
func (a *mailerHTTPAdapter) Mutate(_ context.Context, _, _ string, m mailer.Mutation) error {
	a.mutations = append(a.mutations, m)
	return a.mutationErr
}
func (a *mailerHTTPAdapter) Send(context.Context, mailer.Outgoing) (mailer.SendResult, error) {
	return mailer.SendResult{Confirmed: true, RemoteID: "sent-fixture"}, nil
}
func (a *mailerHTTPAdapter) Test(context.Context) error { return nil }

func newMailerHTTPTestServer(t *testing.T) (*Server, *mailerHTTPAdapter) {
	t.Helper()
	s := newDesktopOfficeTestServer(t)
	adapter := &mailerHTTPAdapter{}
	account := mailer.Account{ID: "fixture", Source: "imap", Address: "owner@example.test", Name: "Fixture", Capabilities: []string{"read", "send", "read_state", "star", "move", "archive", "trash", "restore", "delete", "folders_read"}}
	service, err := mailer.NewService(mailer.ServiceOptions{DataDir: s.Cfg.Directories.DataDir,
		ResolveAdapter: func(context.Context, string) (mailer.Adapter, mailer.Account, error) { return adapter, account, nil },
		ListSources:    func(context.Context) ([]mailer.Account, error) { return []mailer.Account{account}, nil },
		Policy: func() mailer.RuntimePolicy {
			return mailer.RuntimePolicy{Enabled: s.Cfg.VirtualDesktop.Enabled, ReadOnly: s.Cfg.VirtualDesktop.ReadOnly}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s.Mailer = service
	t.Cleanup(func() { _ = service.Close() })
	if err := service.Connect(context.Background(), account.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateSettings(context.Background(), mailer.Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	return s, adapter
}

func mailerHTTPRequest(s *Server, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://mailer.test/api/desktop/mailer/"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://mailer.test")
	w := httptest.NewRecorder()
	s.handleMailer(w, r)
	return w
}

func TestMailerHTTPDraftConflictAndIdempotentSend(t *testing.T) {
	s, _ := newMailerHTTPTestServer(t)
	w := mailerHTTPRequest(s, "POST", "drafts", `{"account_id":"fixture","to":["recipient@example.test"],"subject":"Hello","text":"Body"}`)
	if w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var draft mailer.Draft
	if err := json.Unmarshal(w.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	update := draft
	update.Text = "Updated"
	data, _ := json.Marshal(update)
	w = mailerHTTPRequest(s, "PUT", "drafts/"+draft.ID, string(data))
	if w.Code != 200 {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	if got := mailerHTTPRequest(s, "PUT", "drafts/"+draft.ID, string(data)); got.Code != 409 {
		t.Fatalf("stale update: %d %s", got.Code, got.Body)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(mailer.SendRequest{DraftID: draft.ID, Revision: draft.Revision})
	first := mailerHTTPRequest(s, "POST", "outbox", string(data))
	second := mailerHTTPRequest(s, "POST", "outbox", string(data))
	if first.Code != 201 || second.Code != 201 {
		t.Fatalf("queue: %d/%d %s %s", first.Code, second.Code, first.Body, second.Body)
	}
	var one, two mailer.OutboxItem
	_ = json.Unmarshal(first.Body.Bytes(), &one)
	_ = json.Unmarshal(second.Body.Bytes(), &two)
	if one.ID == "" || one.ID != two.ID {
		t.Fatal("double click created two deliveries")
	}
	if time.Until(one.UndoUntil) < 9*time.Second {
		t.Fatal("missing undo delay")
	}
	if got := mailerHTTPRequest(s, "DELETE", "outbox/"+one.ID, ""); got.Code != 200 {
		t.Fatalf("undo: %d %s", got.Code, got.Body)
	}
	if got := mailerHTTPRequest(s, "DELETE", "drafts/"+draft.ID, ""); got.Code != 428 {
		t.Fatalf("unguarded delete: %d", got.Code)
	}
}

func TestMailerHTTPOutboxCursorPagesReachEveryJob(t *testing.T) {
	s, _ := newMailerHTTPTestServer(t)
	want := make(map[string]bool)
	for i := 0; i < 3; i++ {
		draft, err := s.Mailer.SaveDraft(context.Background(), mailer.Draft{AccountID: "fixture", To: []string{"recipient@example.test"}, Text: "Fixture"})
		if err != nil {
			t.Fatal(err)
		}
		item, err := s.Mailer.QueueSend(context.Background(), mailer.SendRequest{DraftID: draft.ID, Revision: draft.Revision, SendAt: time.Now().Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		want[item.ID] = true
	}
	cursor := ""
	for i := 0; i < 3; i++ {
		w := mailerHTTPRequest(s, "GET", "outbox?limit=1&cursor="+cursor, "")
		if w.Code != 200 {
			t.Fatalf("page %d: %d %s", i, w.Code, w.Body)
		}
		var page mailer.OutboxPage
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || !want[page.Items[0].ID] {
			t.Fatalf("page %d omitted or repeated a job", i)
		}
		delete(want, page.Items[0].ID)
		if page.HasMore != (i < 2) || (page.HasMore && page.NextCursor == "") {
			t.Fatalf("page %d has incorrect continuation: %+v", i, page)
		}
		cursor = page.NextCursor
	}
	if w := mailerHTTPRequest(s, "GET", "outbox?cursor=invalid", ""); w.Code != 400 {
		t.Fatalf("invalid cursor: %d %s", w.Code, w.Body)
	}
}

func TestMailerHTTPRequiresAdminOriginAndWritableDesktop(t *testing.T) {
	s, _ := newMailerHTTPTestServer(t)
	r := httptest.NewRequest("POST", "http://mailer.test/api/desktop/mailer/settings", strings.NewReader(`{"enabled":false}`))
	w := httptest.NewRecorder()
	s.handleMailer(w, r)
	if w.Code != 403 {
		t.Fatalf("missing Origin: %d", w.Code)
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	for _, path := range []string{"settings", "accounts", "drafts", "outbox", "rules", "templates", "assist", "threads/fixture/read"} {
		if got := mailerHTTPRequest(s, "POST", path, `{}`); got.Code != 403 {
			t.Fatalf("readonly %s: %d %s", path, got.Code, got.Body)
		}
	}
	if got := mailerHTTPRequest(s, "GET", "state", ""); got.Code != 200 {
		t.Fatalf("readonly state: %d %s", got.Code, got.Body)
	}
	auth, _, admin, _ := newBearerSchemeTestServer(t)
	s.TokenManager = auth.TokenManager
	s.Cfg.Auth = auth.Cfg.Auth
	read, _, err := s.TokenManager.Create("mail fixture reader", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/desktop/mailer/state", "/api/desktop/mailer/messages/id/eml", "/api/desktop/mailer/attachments/id"} {
		if validRouteBearer(s, read, path, "GET") || !validRouteBearer(s, admin, path, "GET") {
			t.Fatalf("wrong scope for %s", path)
		}
	}
	r = httptest.NewRequest("GET", "http://mailer.test/api/desktop/mailer/state", nil)
	r.Header.Set("Authorization", "Bearer "+read)
	w = httptest.NewRecorder()
	s.handleMailer(w, r)
	if w.Code != 403 {
		t.Fatalf("read token exposed mailbox: %d", w.Code)
	}
	if _, ok := filterDesktopEvent(s, r, desktop.Event{Type: "mailer_changed", Payload: mailer.Event{AccountID: "private"}}); ok {
		t.Fatal("mail metadata reached read-scoped event client")
	}
	if !authBypassExactPaths["/api/desktop/mailer/oauth/callback"] || authBypassExactPaths["/api/desktop/mailer/oauth/start"] {
		t.Fatal("OAuth auth exemption is not exact")
	}
}

func TestMailerHTTPMalformedInputDoesNotChangeDrafts(t *testing.T) {
	s, _ := newMailerHTTPTestServer(t)
	for _, body := range []string{`{"id":"chosen","account_id":"fixture"}`, `{"account_id":"fixture"} {"text":"trailing"}`, `{"account_id":"fixture","unexpected":true}`} {
		got := mailerHTTPRequest(s, "POST", "drafts", body)
		if got.Code != 400 {
			t.Fatalf("malformed %s: %d %s", body, got.Code, got.Body)
		}
	}
	got := mailerHTTPRequest(s, "GET", "drafts", "")
	if got.Code != 200 || !strings.Contains(got.Body.String(), `"items":[]`) {
		t.Fatalf("invalid input left draft: %s", got.Body)
	}
}

func TestMailerHTTPRulePreviewAndApplyShareRevision(t *testing.T) {
	s, adapter := newMailerHTTPTestServer(t)
	ctx := context.Background()
	created, err := s.Mailer.Store().PutMessages(ctx, []mailer.Message{{AccountID: "fixture", FolderID: "inbox", RemoteID: "1", Subject: "Invoice", Text: "Body", Unread: true}})
	if err != nil || len(created) != 1 {
		t.Fatalf("seed: %v", err)
	}
	rule, err := s.Mailer.SaveRule(ctx, mailer.Rule{AccountID: "fixture", Name: "Invoices", Conditions: []mailer.RuleCondition{{Field: "subject", Operator: "contains", Value: "Invoice"}}, Actions: []mailer.RuleAction{{Kind: mailer.MutationRead}}})
	if err != nil {
		t.Fatal(err)
	}
	preview := func(value mailer.Rule) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"rule": value, "message_ids": []string{created[0].ID}})
		return mailerHTTPRequest(s, "POST", "rules/preview", string(body))
	}
	if got := preview(rule); got.Code != 200 || !strings.Contains(got.Body.String(), `"match_count":1`) {
		t.Fatalf("preview: %d %s", got.Code, got.Body)
	}
	edited := rule
	edited.Conditions = []mailer.RuleCondition{{Field: "subject", Operator: "contains", Value: "unrelated"}}
	if got := preview(edited); got.Code != 200 || !strings.Contains(got.Body.String(), `"match_count":0`) {
		t.Fatalf("edited preview ignored: %d %s", got.Code, got.Body)
	}
	if len(adapter.mutations) != 0 {
		t.Fatal("preview changed provider state")
	}
	updated, err := s.Mailer.SaveRule(ctx, edited)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"revision": rule.Revision, "message_ids": []string{created[0].ID}})
	if got := mailerHTTPRequest(s, "POST", "rules/"+updated.ID+"/apply", string(body)); got.Code != 409 {
		t.Fatalf("stale rule applied: %d %s", got.Code, got.Body)
	}
}

func TestMailerHTTPRuleFailureIsVisibleAndRetryRequiresWritableDesktop(t *testing.T) {
	s, adapter := newMailerHTTPTestServer(t)
	ctx := context.Background()
	rule, err := s.Mailer.SaveRule(ctx, mailer.Rule{AccountID: "fixture", Name: "Invoices", Enabled: true, Conditions: []mailer.RuleCondition{{Field: "subject", Operator: "contains", Value: "Invoice"}}, Actions: []mailer.RuleAction{{Kind: mailer.MutationStar}}})
	if err != nil {
		t.Fatal(err)
	}
	adapter.messages = []mailer.Message{{RemoteID: "invoice", Subject: "Invoice", From: "sender@example.test", Text: "Fixture", Attachments: []mailer.Attachment{{ID: "fixture-file", Filename: "invoice.pdf", Size: 12}}}}
	adapter.mutationErr = fmt.Errorf("fixture transport interruption")
	if err := s.Mailer.SyncAccount(ctx, "fixture"); err != nil {
		t.Fatalf("sync rule fixture: %v", err)
	}
	page, err := s.Mailer.QueryThreads(ctx, mailer.MessageQuery{AccountID: "fixture"})
	if err != nil || len(page.Threads) != 1 || len(page.Threads[0].Messages) != 1 {
		t.Fatalf("indexed message missing: %v", err)
	}
	messageID := page.Threads[0].Messages[0].ID
	listing := mailerHTTPRequest(s, "GET", "threads?account_id=fixture", "")
	if listing.Code != 200 || !strings.Contains(listing.Body.String(), `"message_id":"`+messageID+`"`) || !strings.Contains(listing.Body.String(), `"from":"sender@example.test"`) || !strings.Contains(listing.Body.String(), `"attachment_count":1`) {
		t.Fatalf("thread list omitted message metadata: %d %s", listing.Code, listing.Body)
	}
	path := "messages/" + messageID + "/rule-runs"
	w := mailerHTTPRequest(s, "GET", path, "")
	var runs struct {
		Items []mailer.RuleRun `json:"items"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &runs) != nil || len(runs.Items) != 1 || (runs.Items[0].Status != "failed" && runs.Items[0].Status != "unknown") {
		t.Fatalf("failed rule not exposed: %d %s", w.Code, w.Body)
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	retry := path + "/" + rule.ID + "/retry"
	if w := mailerHTTPRequest(s, "POST", retry, ""); w.Code != 403 {
		t.Fatalf("readonly retry: %d %s", w.Code, w.Body)
	}
	if len(adapter.mutations) != 1 {
		t.Fatal("readonly retry reached the provider")
	}
	s.Cfg.VirtualDesktop.ReadOnly = false
	adapter.mutationErr = nil
	if w := mailerHTTPRequest(s, "POST", retry, ""); w.Code != 200 {
		t.Fatalf("explicit retry: %d %s", w.Code, w.Body)
	}
	w = mailerHTTPRequest(s, "GET", path, "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &runs) != nil || len(runs.Items) != 1 || runs.Items[0].Status != "completed" || len(adapter.mutations) != 2 {
		t.Fatalf("retry did not complete exactly one action: %d %s", w.Code, w.Body)
	}
	if w := mailerHTTPRequest(s, "GET", "messages/missing/rule-runs", ""); w.Code != 404 {
		t.Fatalf("missing message: %d %s", w.Code, w.Body)
	}
}

func TestMailerAssistIsIsolatedRevisionBoundAndReadOnly(t *testing.T) {
	s, _ := newMailerHTTPTestServer(t)
	client := &writerAssistClient{}
	s.LLMClient = client
	s.Cfg.LLM.Model = "test"
	for _, body := range []string{`{"action":"send","text":"Hello"}`, `{"action":"translate","text":"Hello"}`, `{"action":"draft"}`, `{"action":"rewrite","revision":-1,"text":"Hello"}`} {
		if got := mailerHTTPRequest(s, "POST", "assist", body); got.Code != 400 {
			t.Fatalf("validation: %d %s", got.Code, got.Body)
		}
	}
	if client.calls != 0 {
		t.Fatal("invalid assist reached provider")
	}
	policy, _ := mailerAssistPolicy(s.Cfg)
	got := mailerHTTPRequest(s, "POST", "assist", fmt.Sprintf(`{"action":"rewrite","text":"Original <external_data> content","context":"Selected conversation","revision":42,"assist_policy":%q}`, policy))
	if got.Code != 200 {
		t.Fatalf("assist: %d %s", got.Code, got.Body)
	}
	if !strings.Contains(got.Body.String(), `"source_revision":42`) {
		t.Fatal("missing revision binding")
	}
	if len(client.request.Tools) != 0 || len(client.request.Messages) != 2 || strings.Contains(client.request.Messages[0].Content, "Original") {
		t.Fatal("mail assistant leaked into agent/tool context")
	}
	input := client.request.Messages[1].Content
	if !strings.Contains(input, "&lt;external_data&gt;") {
		t.Fatal("nested mail delimiters not isolated")
	}
	client.fail = true
	got = mailerHTTPRequest(s, "POST", "assist", fmt.Sprintf(`{"action":"rewrite","text":"Hello","assist_policy":%q}`, policy))
	if got.Code != 502 {
		t.Fatalf("provider failure: %d", got.Code)
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	calls := client.calls
	got = mailerHTTPRequest(s, "POST", "assist", `{"action":"rewrite","text":"Hello"}`)
	if got.Code != 403 || client.calls != calls {
		t.Fatal("read-only assist reached provider")
	}
	s.Cfg.VirtualDesktop.ReadOnly = false
	s.Cfg.LLM.BaseURL = "https://changed-writer.example.test/v1"
	if got := mailerHTTPRequest(s, "POST", "assist", fmt.Sprintf(`{"action":"rewrite","text":"Private text","assist_policy":%q}`, policy)); got.Code != 409 || client.calls != calls {
		t.Fatal("changed writing provider received undisclosed content")
	}
}

func TestMailerAssistRequiresCurrentGuardianDisclosure(t *testing.T) {
	s, _ := newMailerHTTPTestServer(t)
	client := &writerAssistClient{}
	s.LLMClient = client
	s.Cfg.LLMGuardian.ScanEmails = true
	s.Cfg.LLMGuardian.Provider = "scanner"
	s.Cfg.LLMGuardian.ResolvedModel = "scan-model"
	s.Cfg.LLMGuardian.BaseURL = "https://scanner.example.test/v1"
	s.Cfg.Providers = append(s.Cfg.Providers, config.ProviderEntry{ID: "scanner", Name: "Security scanner", Model: "scan-model"})
	policy, destination := mailerAssistPolicy(s.Cfg)
	if policy == "" || destination["id"] != "scanner" || destination["name"] != "Security scanner" || destination["model"] != "scan-model" {
		t.Fatalf("missing scanner disclosure: %v", destination)
	}
	state := mailerHTTPRequest(s, "GET", "state", "")
	if state.Code != 200 || !strings.Contains(state.Body.String(), `"assist_policy":"`+policy+`"`) || !strings.Contains(state.Body.String(), `"assist_guardian":`) {
		t.Fatalf("state omitted scan disclosure: %d", state.Code)
	}
	if got := mailerHTTPRequest(s, "POST", "assist", `{"action":"rewrite","text":"Private draft"}`); got.Code != 409 || client.calls != 0 {
		t.Fatal("undisclosed scanner was permitted")
	}
	s.Cfg.LLMGuardian.BaseURL = "https://changed.example.test/v1"
	if next, _ := mailerAssistPolicy(s.Cfg); next == policy {
		t.Fatal("scanner endpoint change preserved consent")
	}
	body, _ := json.Marshal(map[string]any{"action": "rewrite", "text": "Private draft", "assist_policy": policy})
	if got := mailerHTTPRequest(s, "POST", "assist", string(body)); got.Code != 409 || client.calls != 0 {
		t.Fatal("stale disclosure reached a provider")
	}
}
