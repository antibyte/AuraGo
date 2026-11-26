package mailer

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// auditAdapter extends coreTestAdapter with a configurable Read and a hook that
// runs inside the provider round-trip of Mutate.
type auditAdapter struct {
	coreTestAdapter
	mu       sync.Mutex
	read     Message
	onMutate func(Mutation)
	onRead   func()
}

func (a *auditAdapter) Read(context.Context, string, string) (Message, error) {
	if a.onRead != nil {
		a.onRead()
	}
	return a.read, nil
}

func (a *auditAdapter) Mutate(ctx context.Context, folder, remote string, mutation Mutation) error {
	a.mu.Lock()
	hook := a.onMutate
	a.mu.Unlock()
	if hook != nil {
		hook(mutation)
	}
	return nil
}

func newAuditService(t *testing.T, caps ...string) (*Service, *Store, *auditAdapter, Account) {
	t.Helper()
	account := Account{ID: "email:audit", Source: "email", Protocol: "imap", Capabilities: caps}
	adapter := &auditAdapter{coreTestAdapter: coreTestAdapter{account: account}}
	service, store := newCoreTestService(t, account, adapter)
	if err := service.UpdateSettings(context.Background(), Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	return service, store, adapter, account
}

func putAuditMessage(t *testing.T, store *Store, m Message) Message {
	t.Helper()
	if m.Date.IsZero() {
		m.Date = time.Now().UTC()
	}
	if _, err := store.PutSyncMessagesWithLimit(context.Background(), []Message{m}, 1<<20); err != nil {
		t.Fatal(err)
	}
	m.ID = messageID(m.AccountID, m.FolderID, m.RemoteID)
	return m
}

func TestAuditAttachmentRuleUsesIndexedCountAndHydrates(t *testing.T) {
	if ok, err := matchesRule(Rule{Conditions: []RuleCondition{{Field: "has_attachments", Operator: "equals", Value: "true"}}}, Message{AttachmentCount: 2}); err != nil || !ok {
		t.Fatalf("matchesRule with AttachmentCount = %v, %v", ok, err)
	}
	if !needsBody(Rule{Conditions: []RuleCondition{{Field: "has_attachments"}}}) {
		t.Fatal("has_attachments rule must hydrate the message")
	}

	service, store, adapter, account := newAuditService(t, "read", "read_state")
	// Gmail-style metadata: neither Attachments nor AttachmentCount is indexed.
	adapter.read = Message{Text: "see attached", Attachments: []Attachment{{ID: "a1", Filename: "x.pdf"}}}
	if _, err := service.SaveRule(context.Background(), Rule{
		AccountID: account.ID, Name: "attachments", Enabled: true,
		Conditions: []RuleCondition{{Field: "has_attachments", Operator: "equals", Value: "true"}},
		Actions:    []RuleAction{{Kind: MutationRead}},
	}); err != nil {
		t.Fatal(err)
	}
	m := putAuditMessage(t, store, Message{AccountID: account.ID, FolderID: "INBOX", RemoteID: "att", Subject: "files", Unread: true})
	if err := service.processPendingRuleRuns(context.Background(), account.ID); err != nil {
		t.Fatal(err)
	}
	runs, err := store.ListRuleRuns(context.Background(), m.ID)
	if err != nil || len(runs) != 1 || runs[0].Status != "completed" {
		t.Fatalf("rule run = %#v, %v; want completed (matched)", runs, err)
	}
}

func TestAuditBlobContentTypeSizeAndMessageAggregate(t *testing.T) {
	ctx := context.Background()
	service, store, _, account := newAuditService(t, "read", "send")

	a, err := service.blobs.Put(ctx, "draft", "d1", "a.txt", "text/plain\r\nBcc: evil@example.test", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	if a.ContentType != "application/octet-stream" {
		t.Fatalf("content type with CR/LF = %q", a.ContentType)
	}

	small, err := OpenBlobStore(t.TempDir(), store, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := small.Put(ctx, "draft", "d1", "b.bin", "", strings.NewReader("12345")); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize file error = %v, want ErrTooLarge", err)
	}

	draft, err := store.SaveDraft(ctx, Draft{AccountID: account.ID, To: []string{"to@example.test"}})
	if err != nil {
		t.Fatal(err)
	}
	chunk := bytes.Repeat([]byte("a"), 15<<20)
	draft, err = service.AddDraftAttachment(ctx, draft.ID, draft.Revision, "one.bin", "application/octet-stream", bytes.NewReader(chunk))
	if err != nil {
		t.Fatalf("first attachment: %v", err)
	}
	if _, err := service.AddDraftAttachment(ctx, draft.ID, draft.Revision, "two.bin", "application/octet-stream", bytes.NewReader(chunk)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("second attachment over the per-message aggregate = %v, want ErrTooLarge", err)
	}
}

func TestAuditMergeHydratedKeepsSnippetAndAttachmentCount(t *testing.T) {
	h := Message{Text: "body", Attachments: []Attachment{{ID: "1"}, {ID: "2"}}}
	mergeHydratedMessage(&h, Message{Snippet: "preview", AttachmentCount: 1})
	if h.Snippet != "preview" || h.AttachmentCount != 2 {
		t.Fatalf("snippet=%q count=%d", h.Snippet, h.AttachmentCount)
	}
	h = Message{Text: "body"}
	mergeHydratedMessage(&h, Message{AttachmentCount: 3})
	if h.AttachmentCount != 3 {
		t.Fatalf("count = %d, want 3", h.AttachmentCount)
	}
}

func TestAuditHydrationRespectsCacheBudget(t *testing.T) {
	ctx := context.Background()
	service, store, adapter, account := newAuditService(t, "read")
	m := putAuditMessage(t, store, Message{AccountID: account.ID, FolderID: "INBOX", RemoteID: "big", Subject: "big"})
	var used int64
	if err := store.db.QueryRow("SELECT COALESCE(SUM(length(raw)+length(body)),0) FROM mailer_messages").Scan(&used); err != nil {
		t.Fatal(err)
	}
	service.opts.MaxMessageCacheBytes = used + 100
	adapter.read = Message{Text: strings.Repeat("t", 5000)}
	got, err := service.GetMessage(ctx, m.ID)
	if err != nil {
		t.Fatalf("over-budget hydration must not fail the read: %v", err)
	}
	if len(got.Text) != 5000 {
		t.Fatalf("caller got %d text bytes, want 5000", len(got.Text))
	}
	stored, err := store.GetMessage(ctx, m.ID)
	if err != nil || stored.Text != "" {
		t.Fatalf("over-budget body was persisted: %d bytes, %v", len(stored.Text), err)
	}
	service.opts.MaxMessageCacheBytes = used + 1<<20
	if _, err := service.GetMessage(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if stored, _ = store.GetMessage(ctx, m.ID); len(stored.Text) != 5000 {
		t.Fatalf("in-budget body was not persisted: %d", len(stored.Text))
	}
}

func TestAuditConcurrentMutationsKeepBothChanges(t *testing.T) {
	ctx := context.Background()
	service, store, adapter, account := newAuditService(t, "read", "read_state", "star")
	m := putAuditMessage(t, store, Message{AccountID: account.ID, FolderID: "INBOX", RemoteID: "race", Subject: "race", Text: "kept body", Unread: true})
	var once sync.Once
	adapter.onMutate = func(mutation Mutation) {
		if mutation.Kind != MutationRead {
			return
		}
		// While the first mutation is in its provider round-trip (stale snapshot),
		// a second mutation of another field completes.
		once.Do(func() {
			if err := service.Mutate(ctx, m.ID, Mutation{Kind: MutationStar}); err != nil {
				t.Errorf("inner star: %v", err)
			}
		})
	}
	if err := service.Mutate(ctx, m.ID, Mutation{Kind: MutationRead}); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetMessage(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Unread || !got.Starred {
		t.Fatalf("lost update: unread=%v starred=%v, want read and starred", got.Unread, got.Starred)
	}
	if got.Text != "kept body" {
		t.Fatalf("body rewritten: %q", got.Text)
	}
}

func TestAuditHydrationKeepsConcurrentMutation(t *testing.T) {
	ctx := context.Background()
	service, store, adapter, account := newAuditService(t, "read", "read_state")
	m := putAuditMessage(t, store, Message{AccountID: account.ID, FolderID: "INBOX", RemoteID: "hyd", Subject: "hyd", Unread: true})
	adapter.read = Message{Text: "fetched body"}
	entered := make(chan struct{})
	release := make(chan struct{})
	adapter.onRead = func() {
		close(entered)
		<-release
	}
	done := make(chan error, 1)
	go func() {
		_, err := service.GetMessage(ctx, m.ID)
		done <- err
	}()
	<-entered
	if err := service.Mutate(ctx, m.ID, Mutation{Kind: MutationRead}); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	got, err := store.GetMessage(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Unread {
		t.Fatal("hydration reverted the concurrent mark-read")
	}
	if got.Text != "fetched body" {
		t.Fatalf("body not persisted: %q", got.Text)
	}
}
