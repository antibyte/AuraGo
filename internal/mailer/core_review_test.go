package mailer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/dbutil"
)

func TestSyncPageBudgetIncludesSerializedBodies(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.PutAccount(ctx, Account{ID: "imap:a", Protocol: "imap"}); err != nil {
		t.Fatal(err)
	}

	base := Message{
		AccountID: "imap:a", FolderID: "INBOX", ThreadID: "imap:a:thread",
		Subject: "subject", Text: strings.Repeat("x", 128), Date: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC),
	}
	messages := []Message{base, base}
	messages[0].RemoteID = "remote-1"
	messages[1].RemoteID = "remote-2"
	for i := range messages {
		messages[i].ID = messageID(messages[i].AccountID, messages[i].FolderID, messages[i].RemoteID)
	}
	first, err := marshal(messages[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := marshal(messages[1])
	if err != nil {
		t.Fatal(err)
	}
	pageLimit := int64(len(first) + len(second) - 1)
	if pageLimit <= int64(len(first)) || pageLimit <= int64(len(second)) {
		t.Fatalf("test setup requires each body to fit while the page does not: %d, %d, limit %d", len(first), len(second), pageLimit)
	}
	if _, err := store.putMessagesWithLimits(ctx, messages, 1<<20, pageLimit); !errors.Is(err, ErrStorageFull) {
		t.Fatalf("page quota error = %v, want ErrStorageFull", err)
	}
	page, err := store.QueryMessages(ctx, MessageQuery{View: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 {
		t.Fatalf("partial sync page committed %d messages", page.Total)
	}
}

func TestPOP3RefreshPreservesLocalReadAndStarFlags(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.PutAccount(ctx, Account{ID: "email:pop", Protocol: "pop3"}); err != nil {
		t.Fatal(err)
	}
	initial := Message{AccountID: "email:pop", FolderID: "inbox", RemoteID: "uidl-1", Unread: true}
	if _, err := store.PutMessages(ctx, []Message{initial}); err != nil {
		t.Fatal(err)
	}
	indexed, err := store.GetMessage(ctx, messageID(initial.AccountID, initial.FolderID, initial.RemoteID))
	if err != nil {
		t.Fatal(err)
	}
	indexed.Unread = false
	indexed.Starred = true
	if err := store.UpdateMessage(ctx, indexed); err != nil {
		t.Fatal(err)
	}
	// POP3 has no remote flags; a later fetch reports its default unread value.
	if _, err := store.PutMessages(ctx, []Message{initial}); err != nil {
		t.Fatal(err)
	}
	refreshed, err := store.GetMessage(ctx, indexed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Unread || !refreshed.Starred {
		t.Fatalf("POP3 refresh replaced local flags: unread=%t starred=%t", refreshed.Unread, refreshed.Starred)
	}
}

type snapshotReviewAdapter struct {
	coreTestAdapter
	folder Folder
	page   SyncResult
	calls  int
	errOn  int
}

func (a *snapshotReviewAdapter) Folders(context.Context) ([]Folder, error) {
	return []Folder{a.folder}, nil
}

func (a *snapshotReviewAdapter) Sync(context.Context, string, string) (SyncResult, error) {
	a.calls++
	if a.calls == a.errOn {
		return SyncResult{}, errors.New("snapshot interrupted")
	}
	return a.page, nil
}

func TestUIDValidityResetKeepsCacheUntilSnapshotCompletesAndRejectsStaleMutation(t *testing.T) {
	ctx := context.Background()
	account := Account{
		ID: "email:uid-reset", Source: "email", Protocol: "imap",
		Capabilities: []string{"read", "read_state", "star", "move", "archive", "trash", "restore", "delete"},
	}
	adapter := &snapshotReviewAdapter{
		coreTestAdapter: coreTestAdapter{account: account},
		folder:          Folder{ID: "INBOX", ProviderID: "INBOX", Name: "Inbox", Kind: "inbox"},
		page: SyncResult{
			UIDValidity: 2, ResetSnapshot: true, HasMore: true, NextCursor: "next-page",
			Messages: []Message{{RemoteID: "2:9", ThreadID: "2:9", Subject: "replacement", Date: time.Now().UTC()}},
		},
		errOn: 2,
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
	if err := store.PutFolders(ctx, account.ID, []Folder{adapter.folder}); err != nil {
		t.Fatal(err)
	}
	old := Message{
		AccountID: account.ID, FolderID: adapter.folder.ID, RemoteID: "1:7", ThreadID: "1:7",
		Subject: "old cache", Raw: []byte("RFC822 old"), Unread: false, Starred: true,
	}
	if _, err := store.PutMessages(ctx, []Message{old}); err != nil {
		t.Fatal(err)
	}
	oldID := messageID(account.ID, adapter.folder.ID, old.RemoteID)
	if err := store.SetSnooze(ctx, oldID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCursor(ctx, account.ID, adapter.folder.ID, "old-cursor"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCursorValidity(ctx, account.ID, adapter.folder.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := service.SyncAccount(ctx, account.ID); !errors.Is(err, ErrSyncFailed) {
		t.Fatalf("interrupted snapshot error = %v, want ErrSyncFailed", err)
	}
	page, err := store.QueryMessages(ctx, MessageQuery{AccountID: account.ID, View: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 {
		t.Fatalf("interrupted UIDVALIDITY snapshot retained %d messages, want old and new cache entries", page.Total)
	}
	oldAfter, err := store.GetMessage(ctx, oldID)
	if err != nil {
		t.Fatal(err)
	}
	if string(oldAfter.Raw) != "RFC822 old" || !oldAfter.Starred || oldAfter.SnoozedUntil.IsZero() {
		t.Fatalf("old cache lost durable content or local state: %#v", oldAfter)
	}
	if err := service.Mutate(ctx, oldID, Mutation{Kind: MutationRead}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale UIDVALIDITY message mutation error = %v, want ErrConflict", err)
	}
	if adapter.mutates != 0 {
		t.Fatalf("stale message reached provider mutation %d times", adapter.mutates)
	}
}

func TestMigrationBackupCanRestoreOldSchemaAndMigrate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mailer.db")
	db, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := Draft{ID: "draft_before_upgrade", AccountID: "email:old", Revision: 3, To: []string{"person@example.test"}, Subject: "preserved"}
	body, err := marshal(draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE mailer_drafts (id TEXT PRIMARY KEY, account_id TEXT NOT NULL, revision INTEGER NOT NULL, body BLOB NOT NULL, updated_at INTEGER NOT NULL)`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mailer_drafts(id,account_id,revision,body,updated_at) VALUES(?,?,?,?,?)`, draft.ID, draft.AccountID, 3, body, timeNS(time.Now())); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version=5"); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetDraft(context.Background(), draft.ID); err != nil || got.Subject != draft.Subject || got.Revision != 3 {
		_ = store.Close()
		t.Fatalf("migrated old draft = %#v, %v", got, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(fmt.Sprintf("%s.pre-v%d-*.bak", path, schemaVersion))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("got %d migration backups, want one", len(backups))
	}

	restoredPath := filepath.Join(t.TempDir(), "restored.db")
	backup, err := os.Open(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	restored, err := os.Create(restoredPath)
	if err != nil {
		_ = backup.Close()
		t.Fatal(err)
	}
	if _, err := io.Copy(restored, backup); err != nil {
		_ = backup.Close()
		_ = restored.Close()
		t.Fatal(err)
	}
	if err := backup.Close(); err != nil {
		t.Fatal(err)
	}
	if err := restored.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(restoredPath, 0o600); err != nil {
		t.Fatal(err)
	}
	restoredStore, err := OpenStore(restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	defer restoredStore.Close()
	got, err := restoredStore.GetDraft(context.Background(), draft.ID)
	if err != nil || got.Subject != draft.Subject || got.Revision != 3 {
		t.Fatalf("restored and migrated draft = %#v, %v", got, err)
	}
}
