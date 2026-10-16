package mailer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOutboxDirectCancelAndKeysetPagingBeyond500(t *testing.T) {
	ctx := context.Background()
	account := Account{ID: "email:outbox-page", Source: "email", Protocol: "imap", Capabilities: []string{"read", "send"}}
	adapter := &coreTestAdapter{account: account}
	service, store := newCoreTestService(t, account, adapter)
	if err := service.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	var target OutboxItem
	for i := 0; i < 501; i++ {
		draft, err := store.SaveDraft(ctx, Draft{AccountID: account.ID, To: []string{"recipient@example.test"}})
		if err != nil {
			t.Fatalf("create draft %d: %v", i, err)
		}
		sendAt := now.Add(48 * time.Hour)
		if i == 0 {
			sendAt = now.Add(time.Hour)
		}
		item, err := store.QueueSend(ctx, SendRequest{DraftID: draft.ID, Revision: draft.Revision, SendAt: sendAt}, draft)
		if err != nil {
			t.Fatalf("queue item %d: %v", i, err)
		}
		if i == 0 {
			target = item
		}
	}
	if err := service.CancelSend(ctx, target.ID); err != nil {
		t.Fatalf("cancel older stored item: %v", err)
	}

	seen := make(map[string]bool, 501)
	cursor := ""
	for {
		page, err := service.ListOutboxPage(ctx, cursor, 37)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatalf("keyset page repeated outbox item %q", item.ID)
			}
			seen[item.ID] = true
		}
		if !page.HasMore {
			if page.NextCursor != "" {
				t.Fatalf("final page cursor = %q, want empty", page.NextCursor)
			}
			break
		}
		if page.NextCursor == "" || page.NextCursor == cursor {
			t.Fatalf("page cursor did not advance: %q", page.NextCursor)
		}
		cursor = page.NextCursor
	}
	if len(seen) != 501 {
		t.Fatalf("paged %d outbox items, want 501", len(seen))
	}
}

func TestDraftAttachmentRemovalAndQueueOrderingsPreservePendingData(t *testing.T) {
	ctx := context.Background()
	account := Account{ID: "email:attachment-race", Source: "email", Protocol: "imap", Capabilities: []string{"read", "send"}}
	adapter := &coreTestAdapter{account: account}
	service, store := newCoreTestService(t, account, adapter)
	if err := service.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}

	t.Run("queue wins", func(t *testing.T) {
		draft, err := service.SaveDraft(ctx, Draft{AccountID: account.ID, To: []string{"recipient@example.test"}})
		if err != nil {
			t.Fatal(err)
		}
		draft, err = service.AddDraftAttachment(ctx, draft.ID, draft.Revision, "proof.txt", "text/plain", strings.NewReader("queued bytes"))
		if err != nil {
			t.Fatal(err)
		}
		attachment := draft.Attachments[0]
		if _, err := service.QueueSend(ctx, SendRequest{DraftID: draft.ID, Revision: draft.Revision, SendAt: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
		if err := service.blobs.Remove(ctx, attachment.ID); !errors.Is(err, ErrConflict) {
			t.Fatalf("remove pending blob directly = %v, want ErrConflict", err)
		}
		if _, err := service.RemoveDraftAttachment(ctx, draft.ID, draft.Revision, attachment.ID); !errors.Is(err, ErrConflict) {
			t.Fatalf("remove pending attachment = %v, want ErrConflict", err)
		}
		current, err := store.GetDraft(ctx, draft.ID)
		if err != nil || len(current.Attachments) != 1 || current.Attachments[0].ID != attachment.ID {
			t.Fatalf("queued draft lost its attachment: draft=%#v err=%v", current, err)
		}
		if _, err := os.Stat(filepath.Join(service.blobs.root, attachment.ID+".blob")); err != nil {
			t.Fatalf("queued attachment file missing: %v", err)
		}
	})

	t.Run("removal wins", func(t *testing.T) {
		draft, err := service.SaveDraft(ctx, Draft{AccountID: account.ID, To: []string{"recipient@example.test"}})
		if err != nil {
			t.Fatal(err)
		}
		draft, err = service.AddDraftAttachment(ctx, draft.ID, draft.Revision, "proof.txt", "text/plain", strings.NewReader("removed bytes"))
		if err != nil {
			t.Fatal(err)
		}
		attachment := draft.Attachments[0]
		removed, err := service.RemoveDraftAttachment(ctx, draft.ID, draft.Revision, attachment.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(removed.Attachments) != 0 {
			t.Fatalf("removed draft still has attachment: %#v", removed.Attachments)
		}
		if _, err := service.QueueSend(ctx, SendRequest{DraftID: draft.ID, Revision: draft.Revision, SendAt: time.Now().Add(time.Hour)}); !errors.Is(err, ErrConflict) {
			t.Fatalf("queue stale revision after removal = %v, want ErrConflict", err)
		}
		if _, err := os.Stat(filepath.Join(service.blobs.root, attachment.ID+".blob")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("removed attachment file still exists: %v", err)
		}
	})
}

type ruleReviewAdapter struct {
	coreTestAdapter
	mu        sync.Mutex
	actions   []MutationKind
	failOn    int
	failError error
}

func (a *ruleReviewAdapter) Mutate(_ context.Context, _, _ string, mutation Mutation) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.actions = append(a.actions, mutation.Kind)
	if a.failOn > 0 && len(a.actions) == a.failOn {
		return a.failError
	}
	return nil
}

func (a *ruleReviewAdapter) actionHistory() []MutationKind {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]MutationKind(nil), a.actions...)
}

func newRuleReviewService(t *testing.T) (*Service, *Store, *ruleReviewAdapter, Account) {
	t.Helper()
	account := Account{ID: "email:rules", Source: "email", Protocol: "imap", Capabilities: []string{"read", "read_state", "star"}}
	adapter := &ruleReviewAdapter{coreTestAdapter: coreTestAdapter{account: account}, failOn: 2, failError: errors.New("ambiguous provider response")}
	service, store := newCoreTestService(t, account, adapter)
	if err := service.UpdateSettings(context.Background(), Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	return service, store, adapter, account
}

func createRuleReviewMessage(t *testing.T, store *Store, account Account, suffix string) Message {
	t.Helper()
	m := Message{
		AccountID: account.ID, FolderID: "INBOX", RemoteID: suffix, Subject: "invoice for review",
		Unread: true, Date: time.Now().UTC(),
	}
	if _, err := store.PutSyncMessagesWithLimit(context.Background(), []Message{m}, 1<<20); err != nil {
		t.Fatal(err)
	}
	m.ID = messageID(m.AccountID, m.FolderID, m.RemoteID)
	return m
}

func saveRuleReviewRule(t *testing.T, service *Service, account Account) Rule {
	t.Helper()
	rule, err := service.SaveRule(context.Background(), Rule{
		AccountID: account.ID, Name: "invoice actions", Enabled: true,
		Conditions: []RuleCondition{{Field: "subject", Operator: "contains", Value: "invoice"}},
		Actions:    []RuleAction{{Kind: MutationRead}, {Kind: MutationStar}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return rule
}

func TestAutomaticRulesQueueOnlyForSyncInsertions(t *testing.T) {
	service, store, adapter, account := newRuleReviewService(t)
	_ = saveRuleReviewRule(t, service, account)

	historical := Message{AccountID: account.ID, FolderID: "INBOX", RemoteID: "historic", Subject: "invoice history", Date: time.Now().UTC()}
	if _, err := store.PutMessages(context.Background(), []Message{historical}); err != nil {
		t.Fatal(err)
	}
	if runs, err := store.ListRuleRuns(context.Background(), messageID(historical.AccountID, historical.FolderID, historical.RemoteID)); err != nil || len(runs) != 0 {
		t.Fatalf("generic historical insertion queued rule runs=%#v err=%v", runs, err)
	}

	newMessage := createRuleReviewMessage(t, store, account, "new")
	if err := service.processPendingRuleRuns(context.Background(), account.ID); err == nil {
		t.Fatal("uncertain provider response was not reported")
	}
	runs, err := store.ListRuleRuns(context.Background(), newMessage.ID)
	if err != nil || len(runs) != 1 || runs[0].Status != "unknown" || runs[0].ActionIndex != 1 {
		t.Fatalf("durable result after uncertain second action = %#v, %v", runs, err)
	}
	if got := adapter.actionHistory(); len(got) != 2 || got[0] != MutationRead || got[1] != MutationStar {
		t.Fatalf("initial action sequence = %v", got)
	}
	storedAccount, err := store.Account(context.Background(), account.ID)
	if err != nil || storedAccount.SyncStatus != "error" || storedAccount.SyncError != "rule_error" {
		t.Fatalf("rule failure was not surfaced in account sync state: %#v, %v", storedAccount, err)
	}
	if err := service.RetryRuleRun(context.Background(), newMessage.ID, runs[0].RuleID); err != nil {
		t.Fatalf("explicit retry of uncertain current action: %v", err)
	}
	runs, err = store.ListRuleRuns(context.Background(), newMessage.ID)
	if err != nil || len(runs) != 1 || runs[0].Status != "completed" || runs[0].ActionIndex != 2 {
		t.Fatalf("completed retry result = %#v, %v", runs, err)
	}
	if got := adapter.actionHistory(); len(got) != 3 || got[2] != MutationStar {
		t.Fatalf("retry replayed a previously completed action: %v", got)
	}
	storedAccount, err = store.Account(context.Background(), account.ID)
	if err != nil || storedAccount.SyncError != "" || storedAccount.SyncStatus != "ok" {
		t.Fatalf("successful retry did not clear rule error state: %#v, %v", storedAccount, err)
	}
}

func TestPendingRuleMarkerSurvivesRestartAfterSyncIndex(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "mailer.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	account := Account{ID: "email:pending-restart", Source: "email", Protocol: "imap", Capabilities: []string{"read", "read_state"}}
	if err := store.PutAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	rule, err := store.SaveRule(ctx, Rule{
		AccountID: account.ID, Name: "restart mark read", Enabled: true,
		Conditions: []RuleCondition{{Field: "subject", Operator: "contains", Value: "invoice"}},
		Actions:    []RuleAction{{Kind: MutationRead}},
	})
	if err != nil {
		t.Fatal(err)
	}
	m := Message{AccountID: account.ID, FolderID: "INBOX", RemoteID: "pending-restart", Subject: "invoice", Unread: true, Date: time.Now().UTC()}
	if _, err := store.PutSyncMessagesWithLimit(ctx, []Message{m}, 1<<20); err != nil {
		t.Fatal(err)
	}
	m.ID = messageID(m.AccountID, m.FolderID, m.RemoteID)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := store.PendingRuleRuns(ctx, account.ID)
	if err != nil || len(pending) != 1 || pending[0].Run.Status != "pending" || pending[0].Rule.ID != rule.ID {
		_ = store.Close()
		t.Fatalf("index-to-rule crash lost its durable marker: %#v, %v", pending, err)
	}
	adapter := &ruleReviewAdapter{coreTestAdapter: coreTestAdapter{account: account}}
	service, err := NewService(ServiceOptions{
		Store: store, BlobRoot: t.TempDir(),
		ListSources: func(context.Context) ([]Account, error) { return []Account{account}, nil },
		ResolveAdapter: func(context.Context, string) (Adapter, Account, error) { return adapter, account, nil },
	})
	if err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	defer service.Close()
	defer store.Close()
	if err := service.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := service.processPendingRuleRuns(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	runs, err := store.ListRuleRuns(ctx, m.ID)
	if err != nil || len(runs) != 1 || runs[0].Status != "completed" {
		t.Fatalf("recovered pending rule result = %#v, %v", runs, err)
	}
	if got := adapter.actionHistory(); len(got) != 1 || got[0] != MutationRead {
		t.Fatalf("recovered action sequence = %v", got)
	}
}

type readOnlyRuleSyncAdapter struct {
	coreTestAdapter
	folder Folder
	result SyncResult
}

func (a *readOnlyRuleSyncAdapter) Folders(context.Context) ([]Folder, error) {
	return []Folder{a.folder}, nil
}

func (a *readOnlyRuleSyncAdapter) Sync(context.Context, string, string) (SyncResult, error) {
	return a.result, nil
}

func TestReadOnlySyncDoesNotQueueAutomaticRules(t *testing.T) {
	ctx := context.Background()
	account := Account{ID: "email:readonly-sync", Source: "email", Protocol: "imap", ReadOnly: true, Capabilities: []string{"read", "read_state"}}
	folder := Folder{ID: "INBOX", ProviderID: "INBOX", Name: "Inbox", Kind: "inbox"}
	message := Message{RemoteID: "readonly-arrival", Subject: "invoice arrived", Date: time.Now().UTC()}
	adapter := &readOnlyRuleSyncAdapter{
		coreTestAdapter: coreTestAdapter{account: account}, folder: folder,
		result: SyncResult{Messages: []Message{message}, NextCursor: "cursor-1", RemoteSnapshotComplete: true},
	}
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(ServiceOptions{
		Store: store, BlobRoot: t.TempDir(),
		ListSources:    func(context.Context) ([]Account, error) { return []Account{account}, nil },
		ResolveAdapter: func(context.Context, string) (Adapter, Account, error) { return adapter, account, nil },
	})
	if err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close(); _ = store.Close() })
	if err := service.Connect(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveRule(ctx, Rule{
		AccountID: account.ID, Name: "readonly arrival", Enabled: true,
		Conditions: []RuleCondition{{Field: "subject", Operator: "contains", Value: "invoice"}},
		Actions:    []RuleAction{{Kind: MutationRead}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.SyncAccount(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	messageID := messageID(account.ID, folder.ID, message.RemoteID)
	runs, err := store.ListRuleRuns(ctx, messageID)
	if err != nil || len(runs) != 0 {
		t.Fatalf("read-only arrival queued automatic rules: %#v, %v", runs, err)
	}
}

type cancellableSyncAdapter struct {
	coreTestAdapter
	folder  Folder
	started chan struct{}
}

func (a *cancellableSyncAdapter) Folders(context.Context) ([]Folder, error) {
	return []Folder{a.folder}, nil
}

func (a *cancellableSyncAdapter) Sync(ctx context.Context, _, _ string) (SyncResult, error) {
	close(a.started)
	<-ctx.Done()
	return SyncResult{Messages: []Message{{RemoteID: "cancelled-arrival", Subject: "invoice", Date: time.Now().UTC()}}}, nil
}

func TestReadOnlyRevocationDuringSyncPreventsRuleQueue(t *testing.T) {
	ctx := context.Background()
	account := Account{ID: "email:sync-revocation", Source: "email", Protocol: "imap", Capabilities: []string{"read", "read_state"}}
	folder := Folder{ID: "INBOX", ProviderID: "INBOX", Name: "Inbox", Kind: "inbox"}
	adapter := &cancellableSyncAdapter{coreTestAdapter: coreTestAdapter{account: account}, folder: folder, started: make(chan struct{})}
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(ServiceOptions{
		Store: store, BlobRoot: t.TempDir(),
		ListSources:    func(context.Context) ([]Account, error) { return []Account{account}, nil },
		ResolveAdapter: func(context.Context, string) (Adapter, Account, error) { return adapter, account, nil },
	})
	if err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close(); _ = store.Close() })
	if err := service.Connect(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveRule(ctx, Rule{
		AccountID: account.ID, Name: "invoice read", Enabled: true,
		Conditions: []RuleCondition{{Field: "subject", Operator: "contains", Value: "invoice"}},
		Actions:    []RuleAction{{Kind: MutationRead}},
	}); err != nil {
		t.Fatal(err)
	}
	syncDone := make(chan error, 1)
	go func() { syncDone <- service.SyncAccount(ctx, account.ID) }()
	select {
	case <-adapter.started:
	case <-time.After(2 * time.Second):
		t.Fatal("sync did not reach provider fetch")
	}
	if err := service.SetAccountSettings(ctx, account.ID, AccountSettings{ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-syncDone:
	case <-time.After(2 * time.Second):
		t.Fatal("revoked sync did not stop")
	}
	id := messageID(account.ID, folder.ID, "cancelled-arrival")
	if _, err := store.GetMessage(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked sync indexed the arrival: %v", err)
	}
	runs, err := store.ListRuleRuns(ctx, id)
	if err != nil || len(runs) != 0 {
		t.Fatalf("revoked sync queued rules: %#v, %v", runs, err)
	}
}

func TestInterruptedRuleActionRecoversAsUnknownWithoutAutomaticReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mailer.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	account := Account{ID: "email:restart-rules", Source: "email", Protocol: "imap", Capabilities: []string{"read", "read_state"}}
	if err := store.PutAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	rule, err := store.SaveRule(ctx, Rule{AccountID: account.ID, Name: "mark read", Enabled: true, Conditions: []RuleCondition{{Field: "subject", Operator: "contains", Value: "invoice"}}, Actions: []RuleAction{{Kind: MutationRead}}})
	if err != nil {
		t.Fatal(err)
	}
	m := Message{AccountID: account.ID, FolderID: "INBOX", RemoteID: "restart", Subject: "invoice", Date: time.Now().UTC()}
	if _, err := store.PutSyncMessagesWithLimit(ctx, []Message{m}, 1<<20); err != nil {
		t.Fatal(err)
	}
	m.ID = messageID(m.AccountID, m.FolderID, m.RemoteID)
	if err := store.SetRuleRunExecuting(ctx, m.ID, rule.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runs, err := store.ListRuleRuns(ctx, m.ID)
	if err != nil || len(runs) != 1 || runs[0].Status != "unknown" {
		t.Fatalf("recovered action = %#v, %v", runs, err)
	}
	pending, err := store.PendingRuleRuns(ctx, account.ID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("restart automatically queued uncertain action: %#v, %v", pending, err)
	}
	if exists, err := store.HasRuleRunErrors(ctx, account.ID); err != nil || !exists {
		t.Fatalf("unknown action not visible as a durable error: exists=%v err=%v", exists, err)
	}
}

func TestChangedOrDisabledRuleSkipsPendingSnapshot(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*testing.T, *Service, *Store, Rule)
	}{
		{name: "edited", change: func(t *testing.T, service *Service, _ *Store, rule Rule) {
			rule.Name = "edited after arrival"
			if _, err := service.SaveRule(context.Background(), rule); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "disabled", change: func(t *testing.T, service *Service, _ *Store, rule Rule) {
			rule.Enabled = false
			if _, err := service.SaveRule(context.Background(), rule); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "deleted", change: func(t *testing.T, _ *Service, store *Store, rule Rule) {
			if err := store.DeleteRule(context.Background(), rule.ID, rule.Revision); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, store, adapter, account := newRuleReviewService(t)
			rule := saveRuleReviewRule(t, service, account)
			message := createRuleReviewMessage(t, store, account, fmt.Sprintf("%s-message", test.name))
			test.change(t, service, store, rule)
			if err := service.processPendingRuleRuns(context.Background(), account.ID); err != nil {
				t.Fatal(err)
			}
			runs, err := store.ListRuleRuns(context.Background(), message.ID)
			if err != nil || len(runs) != 1 || runs[0].Status != "skipped" {
				t.Fatalf("stale rule snapshot = %#v, %v", runs, err)
			}
			if got := adapter.actionHistory(); len(got) != 0 {
				t.Fatalf("stale rule performed actions: %v", got)
			}
		})
	}
}

func TestRuleRetrySkipsStaleUnknownAndClearsRuleError(t *testing.T) {
	service, store, adapter, account := newRuleReviewService(t)
	rule := saveRuleReviewRule(t, service, account)
	message := createRuleReviewMessage(t, store, account, "stale-retry")
	if err := store.SetRuleRunExecuting(context.Background(), message.ID, rule.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := store.RecoverRuleRuns(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAccountSyncStatus(context.Background(), account.ID, "error", "rule_error", time.Now()); err != nil {
		t.Fatal(err)
	}
	rule.Enabled = false
	if _, err := service.SaveRule(context.Background(), rule); err != nil {
		t.Fatal(err)
	}
	if err := service.RetryRuleRun(context.Background(), message.ID, rule.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("retry changed snapshot = %v, want ErrConflict", err)
	}
	runs, err := store.ListRuleRuns(context.Background(), message.ID)
	if err != nil || len(runs) != 1 || runs[0].Status != "skipped" {
		t.Fatalf("stale retry result = %#v, %v", runs, err)
	}
	stored, err := store.Account(context.Background(), account.ID)
	if err != nil || stored.SyncError != "" || stored.SyncStatus != "ok" {
		t.Fatalf("resolved rule error status = %#v, %v", stored, err)
	}
	if len(adapter.actionHistory()) != 0 {
		t.Fatal("stale explicit retry reached provider")
	}
}

func TestSetAccountSettingsValidatesBeforeChangingAgentAccess(t *testing.T) {
	account := Account{ID: "email:settings", Source: "email", Protocol: "imap", AgentAccess: true, Capabilities: []string{"read"}}
	adapter := &coreTestAdapter{account: account}
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	grantCalls := 0
	service, err := NewService(ServiceOptions{
		Store: store, BlobRoot: t.TempDir(),
		ListSources:    func(context.Context) ([]Account, error) { return []Account{account}, nil },
		ResolveAdapter: func(context.Context, string) (Adapter, Account, error) { return adapter, account, nil },
		SetAgentAccess: func(context.Context, string, bool) error { mu.Lock(); grantCalls++; mu.Unlock(); return nil },
	})
	if err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close(); _ = store.Close() })
	if err := service.Connect(context.Background(), account.ID); err != nil {
		t.Fatal(err)
	}
	for _, settings := range []AccountSettings{
		{AgentAccess: false, FolderMap: map[string]string{"inbox": "not-owned"}},
		{AgentAccess: false, SignatureID: "missing-signature"},
	} {
		if err := service.SetAccountSettings(context.Background(), account.ID, settings); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid settings error = %v, want ErrInvalid", err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if grantCalls != 0 {
		t.Fatalf("external AgentAccess changed %d times before settings validation", grantCalls)
	}
}

func TestSetAccountSettingsRollsBackAgentAccessWhenStoreWriteFails(t *testing.T) {
	ctx := context.Background()
	account := Account{ID: "email:settings-rollback", Source: "email", Protocol: "imap", AgentAccess: true, Capabilities: []string{"read"}}
	adapter := &coreTestAdapter{account: account}
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	setCalls := 0
	setAccess := func(_ context.Context, _ string, allowed bool) error {
		mu.Lock()
		defer mu.Unlock()
		setCalls++
		account.AgentAccess = allowed
		return nil
	}
	service, err := NewService(ServiceOptions{
		Store: store, BlobRoot: t.TempDir(),
		ListSources:    func(context.Context) ([]Account, error) { return []Account{account}, nil },
		ResolveAdapter: func(context.Context, string) (Adapter, Account, error) { return adapter, account, nil },
		SetAgentAccess: setAccess,
	})
	if err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close(); _ = store.Close() })
	if err := service.Connect(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`CREATE TRIGGER reject_mailer_settings BEFORE INSERT ON mailer_account_settings BEGIN SELECT RAISE(ABORT,'injected settings failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := service.SetAccountSettings(ctx, account.ID, AccountSettings{AgentAccess: false}); err == nil {
		t.Fatal("SetAccountSettings succeeded despite the injected store failure")
	}
	mu.Lock()
	defer mu.Unlock()
	if setCalls != 2 || !account.AgentAccess {
		t.Fatalf("agent access was not restored after persistence failure: calls=%d enabled=%t", setCalls, account.AgentAccess)
	}
}

func TestAccountReadOnlyRevocationCancelsAndSerializesActiveMutation(t *testing.T) {
	account := Account{ID: "email:revoke", Source: "email", Protocol: "imap", Capabilities: []string{"read", "read_state"}}
	started := make(chan struct{})
	var once sync.Once
	adapter := &permissionGateAdapter{coreTestAdapter: coreTestAdapter{account: account}, started: started, once: &once}
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(ServiceOptions{
		Store: store, BlobRoot: t.TempDir(),
		ListSources:    func(context.Context) ([]Account, error) { return []Account{account}, nil },
		ResolveAdapter: func(context.Context, string) (Adapter, Account, error) { return adapter, account, nil },
	})
	if err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close(); _ = store.Close() })
	if err := service.Connect(context.Background(), account.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateSettings(context.Background(), Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.PutFolders(context.Background(), account.ID, []Folder{{ID: "INBOX", Kind: "inbox"}}); err != nil {
		t.Fatal(err)
	}
	m := Message{AccountID: account.ID, FolderID: "INBOX", RemoteID: "1", Subject: "test", Date: time.Now().UTC()}
	if _, err := store.PutMessages(context.Background(), []Message{m}); err != nil {
		t.Fatal(err)
	}
	m.ID = messageID(m.AccountID, m.FolderID, m.RemoteID)

	mutationDone := make(chan error, 1)
	go func() { mutationDone <- service.Mutate(context.Background(), m.ID, Mutation{Kind: MutationRead}) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("provider mutation did not start")
	}
	if err := service.SetAccountSettings(context.Background(), account.ID, AccountSettings{ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	if err := <-mutationDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("active provider mutation error = %v, want cancellation", err)
	}
	if err := service.Mutate(context.Background(), m.ID, Mutation{Kind: MutationRead}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("mutation after revocation = %v, want ErrReadOnly", err)
	}
}

type permissionGateAdapter struct {
	coreTestAdapter
	started chan struct{}
	once    *sync.Once
}

func (a *permissionGateAdapter) Mutate(ctx context.Context, _, _ string, _ Mutation) error {
	a.once.Do(func() { close(a.started) })
	<-ctx.Done()
	return ctx.Err()
}
