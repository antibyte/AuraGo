package mailer

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mailerSearchFixture struct {
	*coreTestAdapter
	calls      int
	lastCursor string
}

func (a *mailerSearchFixture) Search(_ context.Context, folder string, q MessageQuery) (MessagePage, error) {
	a.calls++
	a.lastCursor = q.Cursor
	page := MessagePage{Messages: []Message{{RemoteID: "4:7", FolderID: folder, Subject: "Remote result", From: "sender@example.test", Text: "indexed body", Date: time.Now(), Unread: true, MessageID: "<search@example.test>"}}, Total: 150}
	if q.Cursor == "" {
		page.NextCursor = "page-2"
	}
	return page, nil
}

func TestMailerRemoteSearchScopesResultsWithoutApplyingRules(t *testing.T) {
	ctx := context.Background()
	account := Account{ID: "email:search", Source: "email", Protocol: "imap", Capabilities: []string{"read", "search", "read_state"}}
	base := &coreTestAdapter{account: account}
	s, store := newCoreTestService(t, account, base)
	searcher := &mailerSearchFixture{coreTestAdapter: base}
	s.opts.ResolveAdapter = func(context.Context, string) (Adapter, Account, error) { return searcher, account, nil }
	if err := s.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.PutFolders(ctx, account.ID, []Folder{{ID: "INBOX", AccountID: account.ID, Kind: "inbox"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveRule(ctx, Rule{Name: "Unread must survive search", AccountID: account.ID, Enabled: true, Conditions: []RuleCondition{{Field: "subject", Operator: "contains", Value: "Remote"}}, Actions: []RuleAction{{Kind: MutationRead}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SearchRemote(ctx, MessageQuery{AccountID: account.ID, FolderID: "other-account", Query: "Remote"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-account folder accepted: %v", err)
	}
	if searcher.calls != 0 {
		t.Fatal("invalid query reached provider")
	}
	page, err := s.SearchRemote(ctx, MessageQuery{AccountID: account.ID, FolderID: "INBOX", Query: "Remote"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Threads) != 1 || !page.Threads[0].Unread || base.mutates != 0 || page.NextCursor != "page-2" {
		t.Fatal("search consumed unread or ran rules")
	}
	if _, err := s.SearchRemote(ctx, MessageQuery{AccountID: account.ID, FolderID: "INBOX", Query: "Remote", Cursor: page.NextCursor}); err != nil {
		t.Fatal(err)
	}
	if searcher.lastCursor != "page-2" {
		t.Fatalf("provider search cursor = %q", searcher.lastCursor)
	}
	message, err := s.GetMessage(ctx, page.Threads[0].Messages[0].ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	// The persisted metadata can be addressed independently of provider search.
	stored, err := store.GetMessage(ctx, page.Threads[0].Messages[0].ID)
	if err != nil || stored.AccountID != account.ID || !stored.Unread {
		t.Fatalf("result was not safely indexed: %v %#v %#v", err, stored, message)
	}
}
