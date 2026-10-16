package mailer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type coreTestAdapter struct {
	account Account
	mutates int
}

func (a *coreTestAdapter) Folders(context.Context) ([]Folder, error) { return nil, nil }
func (a *coreTestAdapter) Sync(context.Context, string, string) (SyncResult, error) {
	return SyncResult{}, nil
}
func (a *coreTestAdapter) Read(context.Context, string, string) (Message, error) {
	return Message{}, ErrNotFound
}
func (a *coreTestAdapter) Raw(context.Context, string, string) ([]byte, error) {
	return nil, ErrNotFound
}
func (a *coreTestAdapter) Attachment(context.Context, string, string, string) (io.ReadCloser, error) {
	return nil, ErrNotFound
}
func (a *coreTestAdapter) Mutate(context.Context, string, string, Mutation) error {
	a.mutates++
	return nil
}
func (a *coreTestAdapter) Send(context.Context, Outgoing) (SendResult, error) {
	return SendResult{}, nil
}
func (a *coreTestAdapter) Test(context.Context) error { return nil }

func newCoreTestService(t *testing.T, account Account, adapter *coreTestAdapter) (*Service, *Store) {
	t.Helper()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service, err := NewService(ServiceOptions{
		Store:    store,
		BlobRoot: t.TempDir(),
		ListSources: func(context.Context) ([]Account, error) {
			return []Account{account}, nil
		},
		ResolveAdapter: func(context.Context, string) (Adapter, Account, error) {
			return adapter, account, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	if err := service.Connect(context.Background(), account.ID); err != nil {
		t.Fatal(err)
	}
	return service, store
}

func TestPutMessagesThreadsByRFCHeadersWithinAccount(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, id := range []string{"imap:a", "imap:b"} {
		if err := store.PutAccount(ctx, Account{ID: id, Source: "email", Protocol: "imap"}); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	// The reply arrives before its parent; the later parent must join the same thread.
	if _, err := store.PutMessages(ctx, []Message{{
		AccountID: "imap:a", FolderID: "INBOX", RemoteID: "20", ThreadID: "20",
		MessageID: "<reply@example.test>", InReplyTo: "<root@example.test>", References: []string{"<root@example.test>"},
		Subject: "Re: hello", Date: base.Add(time.Minute),
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutMessages(ctx, []Message{{
		AccountID: "imap:a", FolderID: "INBOX", RemoteID: "1", ThreadID: "1",
		MessageID: "<root@example.test>", Subject: "hello", Date: base,
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutMessages(ctx, []Message{{
		AccountID: "imap:a", FolderID: "INBOX", RemoteID: "21", ThreadID: "21",
		MessageID: "<reply2@example.test>", References: []string{"<root@example.test>"}, Subject: "Re: hello", Date: base.Add(2 * time.Minute),
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutMessages(ctx, []Message{{
		AccountID: "imap:b", FolderID: "INBOX", RemoteID: "1", ThreadID: "1",
		MessageID: "<root@example.test>", Subject: "hello", Date: base,
	}}); err != nil {
		t.Fatal(err)
	}
	page, err := store.QueryThreadPage(ctx, MessageQuery{Limit: 10, View: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Threads) != 2 {
		t.Fatalf("got %d account-scoped threads, want 2: %#v", page.Total, page.Threads)
	}
	var accountA Thread
	for _, thread := range page.Threads {
		if thread.AccountID == "imap:a" {
			accountA = thread
		}
	}
	messages, err := store.GetThread(ctx, accountA.ID)
	if err != nil || accountA.Count != 3 || len(messages) != 3 {
		t.Fatalf("account A thread has count=%d messages=%d err=%v, want 3", accountA.Count, len(messages), err)
	}
	if messages[0].ThreadID != accountA.ID || messages[1].ThreadID != accountA.ID {
		t.Fatalf("thread resolver left split IDs: %#v", messages)
	}
}

func TestThreadAndMessageCursorsRemainStableAcrossSyncInsert(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.PutAccount(ctx, Account{ID: "imap:a", Protocol: "imap"}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		_, err := store.PutMessages(ctx, []Message{{AccountID: "imap:a", FolderID: "INBOX", RemoteID: string(rune('1' + i)), ThreadID: string(rune('1' + i)), Date: base.Add(time.Duration(i) * time.Minute), Subject: "message"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.QueryThreadPage(ctx, MessageQuery{Limit: 1, View: "all"})
	if err != nil || len(first.Threads) != 1 || first.NextCursor == "" {
		t.Fatalf("first cursor page = %#v, %v", first, err)
	}
	_, err = store.PutMessages(ctx, []Message{{AccountID: "imap:a", FolderID: "INBOX", RemoteID: "new", ThreadID: "new", Date: base.Add(10 * time.Minute), Subject: "new arrival"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.QueryThreadPage(ctx, MessageQuery{Limit: 1, Cursor: first.NextCursor, View: "all"})
	if err != nil || len(second.Threads) != 1 {
		t.Fatalf("second cursor page = %#v, %v", second, err)
	}
	if second.Threads[0].ID == first.Threads[0].ID || second.Threads[0].Subject == "new arrival" {
		t.Fatalf("new insertion shifted cursor page: first=%#v second=%#v", first.Threads, second.Threads)
	}
}

func TestRawCacheQuotaRollsBackWholeSyncPage(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.PutAccount(ctx, Account{ID: "imap:a", Protocol: "imap"}); err != nil {
		t.Fatal(err)
	}
	_, err = store.PutMessagesWithLimit(ctx, []Message{
		{AccountID: "imap:a", FolderID: "INBOX", RemoteID: "1", Raw: []byte("123456")},
		{AccountID: "imap:a", FolderID: "INBOX", RemoteID: "2", Raw: []byte("123456")},
	}, 10)
	if !errors.Is(err, ErrStorageFull) {
		t.Fatalf("quota error = %v, want ErrStorageFull", err)
	}
	page, err := store.QueryMessages(ctx, MessageQuery{View: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 {
		t.Fatalf("partial page committed %d messages after quota failure", page.Total)
	}
}

func TestRemoteSnapshotDeletesOnlyAfterCompleteSnapshot(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.PutAccount(ctx, Account{ID: "imap:a", Protocol: "imap"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutMessages(ctx, []Message{
		{AccountID: "imap:a", FolderID: "INBOX", RemoteID: "keep"},
		{AccountID: "imap:a", FolderID: "INBOX", RemoteID: "remove"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginRemoteSnapshot(ctx, "imap:a", "INBOX"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkRemoteSnapshotSeen(ctx, "imap:a", "INBOX", []string{"keep"}); err != nil {
		t.Fatal(err)
	}
	page, err := store.QueryMessages(ctx, MessageQuery{AccountID: "imap:a", View: "all"})
	if err != nil || page.Total != 2 {
		t.Fatalf("incomplete snapshot purged data: total=%d err=%v", page.Total, err)
	}
	if err := store.FinishRemoteSnapshot(ctx, "imap:a", "INBOX"); err != nil {
		t.Fatal(err)
	}
	page, err = store.QueryMessages(ctx, MessageQuery{AccountID: "imap:a", View: "all"})
	if err != nil || page.Total != 1 || page.Messages[0].RemoteID != "keep" {
		t.Fatalf("complete snapshot did not reconcile: %#v err=%v", page, err)
	}
}

func TestPOP3MutationsAreLocalAndRetainRawMessage(t *testing.T) {
	account := Account{ID: "email:pop", Source: "email", Protocol: "pop3", Capabilities: []string{"read", "folders_read", "read_state", "star", "move", "archive", "trash", "restore", "delete", "junk", "labels_write"}}
	adapter := &coreTestAdapter{account: account}
	service, store := newCoreTestService(t, account, adapter)
	ctx := context.Background()
	if err := service.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	inbox := Folder{ID: "pop-inbox", AccountID: account.ID, ProviderID: "INBOX", Name: "Inbox", Kind: "inbox"}
	if err := store.PutFolders(ctx, account.ID, []Folder{inbox}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutMessages(ctx, []Message{{AccountID: account.ID, FolderID: inbox.ID, RemoteID: "uidl-1", ThreadID: "uidl-1", Subject: "hello", Raw: []byte("From: a@example.test\r\nSubject: hello\r\n\r\nbody")}}); err != nil {
		t.Fatal(err)
	}
	page, err := service.QueryMessages(ctx, MessageQuery{AccountID: account.ID, Limit: 10, View: "all"})
	if err != nil || len(page.Messages) != 1 {
		t.Fatalf("message query = %#v, %v", page, err)
	}
	messageID := page.Messages[0].ID
	if err := service.Mutate(ctx, messageID, Mutation{Kind: MutationArchive}); err != nil {
		t.Fatal(err)
	}
	archived, err := store.GetMessage(ctx, messageID)
	if err != nil {
		t.Fatal(err)
	}
	if archived.FolderID == inbox.ID || !strings.Contains(archived.FolderID, "archive") {
		t.Fatalf("archive did not use local POP3 folder: %q", archived.FolderID)
	}
	// An empty raw update from a later POP3 index pass must preserve the downloaded source.
	if _, err := store.PutMessages(ctx, []Message{{AccountID: account.ID, FolderID: inbox.ID, RemoteID: "uidl-1", ThreadID: "uidl-1", Subject: "hello"}}); err != nil {
		t.Fatal(err)
	}
	archived, err = store.GetMessage(ctx, messageID)
	if err != nil || !strings.Contains(string(archived.Raw), "body") {
		t.Fatalf("POP3 RFC822 body was not retained: raw=%q err=%v", archived.Raw, err)
	}
	if err := service.Mutate(ctx, messageID, Mutation{Kind: MutationDelete}); err != nil {
		t.Fatal(err)
	}
	deleted, err := store.GetMessage(ctx, messageID)
	if err != nil || !strings.Contains(deleted.FolderID, "deleted") {
		t.Fatalf("POP3 delete must move to local Deleted folder: %#v, %v", deleted, err)
	}
	if adapter.mutates != 0 {
		t.Fatalf("POP3 made %d remote mutation calls, want none", adapter.mutates)
	}
}

func TestCurrentCapabilitiesGateCachedMessages(t *testing.T) {
	account := Account{ID: "imap:a", Source: "email", Protocol: "imap", Capabilities: []string{"read", "folders_read"}}
	adapter := &coreTestAdapter{account: account}
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service, err := NewService(ServiceOptions{
		Store: store, BlobRoot: t.TempDir(),
		ListSources:    func(context.Context) ([]Account, error) { return []Account{account}, nil },
		ResolveAdapter: func(context.Context, string) (Adapter, Account, error) { return adapter, account, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	if err := service.Connect(context.Background(), account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutMessages(context.Background(), []Message{{AccountID: account.ID, FolderID: "INBOX", RemoteID: "1", ThreadID: "1", Subject: "private"}}); err != nil {
		t.Fatal(err)
	}
	page, err := service.QueryMessages(context.Background(), MessageQuery{AccountID: account.ID, View: "all"})
	if err != nil || page.Total != 1 {
		t.Fatalf("initial read = %#v, %v", page, err)
	}
	messageID := page.Messages[0].ID
	account.Capabilities = []string{"send", "folders_read"}
	adapter.account = account
	page, err = service.QueryMessages(context.Background(), MessageQuery{AccountID: account.ID, View: "all"})
	if err != nil || page.Total != 0 {
		t.Fatalf("cache remained visible after read capability revocation: %#v, %v", page, err)
	}
	if _, err := service.GetMessage(context.Background(), messageID); !errors.Is(err, ErrCapability) {
		t.Fatalf("cached message read error = %v, want ErrCapability", err)
	}
}

func TestDraftRevisionCAS(t *testing.T) {
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	draft, err := store.SaveDraft(context.Background(), Draft{AccountID: "imap:a", To: []string{"person@example.test"}, Subject: "first"})
	if err != nil {
		t.Fatal(err)
	}
	stale := draft
	draft.Subject = "second"
	updated, err := store.SaveDraft(context.Background(), draft)
	if err != nil {
		t.Fatal(err)
	}
	stale.Subject = "stale write"
	if _, err := store.SaveDraft(context.Background(), stale); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale draft update error = %v, want ErrConflict", err)
	}
	if updated.Revision != 2 {
		t.Fatalf("updated revision = %d, want 2", updated.Revision)
	}
}

func TestAccountFolderMappingValidationAndRevocableOperations(t *testing.T) {
	account := Account{ID: "imap:a", Source: "email", Protocol: "imap", Capabilities: []string{"read", "send", "folders_read"}}
	adapter := &coreTestAdapter{account: account}
	service, store := newCoreTestService(t, account, adapter)
	ctx := context.Background()
	if err := store.PutFolders(ctx, account.ID, []Folder{{ID: "INBOX", AccountID: account.ID, Kind: "inbox"}, {ID: "Sent", AccountID: account.ID, Kind: "sent"}}); err != nil {
		t.Fatal(err)
	}
	if err := service.SetAccountSettings(ctx, account.ID, AccountSettings{FolderMap: map[string]string{"sent": "foreign-account-folder"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("foreign folder mapping error = %v, want ErrInvalid", err)
	}
	if err := service.SetAccountSettings(ctx, account.ID, AccountSettings{FolderMap: map[string]string{"sent": "Sent"}}); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	op, cancel, err := service.begin(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if err := service.UpdateSettings(ctx, Settings{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(op.Err(), context.Canceled) {
		t.Fatalf("active operation context = %v, want cancelled when service is disabled", op.Err())
	}
}

func TestRuleAttachmentConditionAndSafeActions(t *testing.T) {
	rule := Rule{
		Name:       "attachment label",
		Conditions: []RuleCondition{{Field: "has_attachments", Operator: "equals", Value: "true"}},
		Actions:    []RuleAction{{Kind: MutationLabelAdd, Label: "receipts"}},
	}
	if err := validateRule(rule); err != nil {
		t.Fatalf("valid attachment rule rejected: %v", err)
	}
	matched, err := matchesRule(rule, Message{Attachments: []Attachment{{Filename: "receipt.pdf"}}})
	if err != nil || !matched {
		t.Fatalf("attachment condition = %t, %v", matched, err)
	}
	rule.Actions = []RuleAction{{Kind: MutationDelete}}
	if err := validateRule(rule); err == nil {
		t.Fatal("rule accepted permanent delete action")
	}
}

func TestMigrationCreatesBackupBeforeSchemaUpgrade(t *testing.T) {
	path := t.TempDir() + "/mailer.db"
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(context.Background(), "PRAGMA user_version=5"); err != nil {
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
	backups, err := filepath.Glob(path + fmt.Sprintf(".pre-v%d-*.bak", schemaVersion))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("migration created %d backups, want exactly one", len(backups))
	}
}

func TestOutboxSendingRecoveryIsUnknownAndNotReplayed(t *testing.T) {
	path := t.TempDir() + "/mailer.db"
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := store.SaveDraft(context.Background(), Draft{AccountID: "imap:a", To: []string{"person@example.test"}, Subject: "send"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.QueueSend(context.Background(), SendRequest{DraftID: draft.ID, Revision: draft.Revision}, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(context.Background(), "UPDATE mailer_outbox SET send_at=? WHERE id=?", timeNS(time.Now().Add(-time.Minute)), item.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ClaimDue(context.Background(), time.Now()); err != nil {
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
	items, err := store.ListOutbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != "unknown" {
		t.Fatalf("recovered outbox = %#v, want unknown", items)
	}
	if _, _, err := store.ClaimDue(context.Background(), time.Now().Add(time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown send was claimable for replay: %v", err)
	}
}
