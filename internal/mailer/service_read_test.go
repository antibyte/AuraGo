package mailer

import (
	"context"
	"testing"
	"time"
)

type hydrateBodyAdapter struct {
	coreTestAdapter
	body string
}

func (a *hydrateBodyAdapter) Read(context.Context, string, string) (Message, error) {
	return Message{Text: a.body}, nil
}

func TestGetMessageHydratesWhenOnlySnippetIsStored(t *testing.T) {
	ctx := context.Background()
	account := Account{ID: "email:snippet", Source: "email", Protocol: "imap", Capabilities: []string{"read"}}
	adapter := &hydrateBodyAdapter{coreTestAdapter: coreTestAdapter{account: account}, body: "full body"}
	service, store := newCoreTestService(t, account, adapter)
	if err := service.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.PutMessages(ctx, []Message{{
		AccountID: account.ID, FolderID: "INBOX", RemoteID: "9", ThreadID: "9",
		Subject: "preview", From: "a@example.test", Snippet: "short preview", AttachmentCount: 2,
		Date: time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC),
	}})
	if err != nil || len(stored) != 1 {
		t.Fatal(err)
	}
	got, err := service.GetMessage(ctx, stored[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "full body" || got.HTML != "" || len(got.Raw) != 0 {
		t.Fatalf("snippet was treated as a hydrated body: %+v", got)
	}
}

func TestGetThreadKeepsIndexedMessageWhenOneReadFails(t *testing.T) {
	ctx := context.Background()
	account := Account{ID: "email:reader", Source: "email", Protocol: "imap", Capabilities: []string{"read"}}
	service, store := newCoreTestService(t, account, &coreTestAdapter{account: account})
	if err := service.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	if _, err := store.PutMessages(ctx, []Message{
		{AccountID: account.ID, FolderID: "INBOX", RemoteID: "1", ThreadID: "thread", MessageID: "<root@example.test>", Subject: "hello", From: "a@example.test", Text: "visible", Date: base},
		{AccountID: account.ID, FolderID: "INBOX", RemoteID: "2", ThreadID: "thread", MessageID: "<reply@example.test>", InReplyTo: "<root@example.test>", References: []string{"<root@example.test>"}, Subject: "Re: hello", From: "b@example.test", Date: base.Add(time.Minute)},
	}); err != nil {
		t.Fatal(err)
	}
	page, err := store.QueryThreadPage(ctx, MessageQuery{AccountID: account.ID, Limit: 10, View: "all"})
	if err != nil || len(page.Threads) != 1 {
		t.Fatalf("threads = %#v err=%v", page.Threads, err)
	}
	thread, err := service.GetThread(ctx, page.Threads[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.Messages) != 2 {
		t.Fatalf("messages = %#v", thread.Messages)
	}
	var visible, unavailable Message
	for _, message := range thread.Messages {
		if message.Text == "visible" {
			visible = message
		}
		if message.RemoteID == "2" {
			unavailable = message
		}
	}
	if visible.ReadError != "" || visible.Text != "visible" {
		t.Fatalf("readable message = %+v", visible)
	}
	if unavailable.ReadError != "message_unavailable" || unavailable.Subject != "Re: hello" || unavailable.Text != "" {
		t.Fatalf("unreadable message = %+v", unavailable)
	}
	stored, err := store.GetMessage(ctx, unavailable.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ReadError != "" {
		t.Fatalf("read error was persisted: %+v", stored)
	}
}
