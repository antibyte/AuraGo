package mailer

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMailerDraftWithoutRecipientsAndSenderChange(t *testing.T) {
	ctx := context.Background()
	accounts := []Account{{ID: "email:a", Source: "email", Protocol: "imap", Capabilities: []string{"read", "send"}}, {ID: "email:b", Source: "email", Protocol: "imap", Capabilities: []string{"read", "send"}}}
	service, err := NewService(ServiceOptions{DataDir: t.TempDir(), ListSources: func(context.Context) ([]Account, error) { return accounts, nil }, ResolveAdapter: func(_ context.Context, id string) (Adapter, Account, error) {
		for _, account := range accounts {
			if account.ID == id {
				return &coreTestAdapter{account: account}, account, nil
			}
		}
		return nil, Account{}, ErrNotFound
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if err := service.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	for _, account := range accounts {
		if err := service.Connect(ctx, account.ID); err != nil {
			t.Fatal(err)
		}
	}
	draft, err := service.SaveDraft(ctx, Draft{AccountID: accounts[0].ID, Text: "Unfinished email", ReplyRemoteID: "fixture-original-message"})
	if err != nil {
		t.Fatalf("autosave without recipients: %v", err)
	}
	if _, err := service.QueueSend(ctx, SendRequest{DraftID: draft.ID, Revision: draft.Revision}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("recipient-free send: %v", err)
	}
	draft.AccountID = accounts[1].ID
	draft.To = []string{"recipient@example.test"}
	draft, err = service.SaveDraft(ctx, draft)
	if err != nil {
		t.Fatalf("change sender: %v", err)
	}
	if draft.ReplyRemoteID != "" {
		t.Fatal("sender change retained another account's provider reply identifier")
	}
	item, err := service.QueueSend(ctx, SendRequest{DraftID: draft.ID, Revision: draft.Revision})
	if err != nil || item.AccountID != accounts[1].ID {
		t.Fatalf("wrong send account: %#v / %v", item, err)
	}
	draft.AccountID = accounts[0].ID
	if _, err := service.Store().SaveDraft(ctx, draft); !errors.Is(err, ErrConflict) {
		t.Fatalf("queued sender changed outside revision guard: %v", err)
	}
}

func TestMailerDraftRejectsUnsafeProviderReplyIdentifier(t *testing.T) {
	for _, id := range []string{"a\r\nb", "a\x00b", strings.Repeat("a", 999)} {
		if !errors.Is(validateDraft(Draft{ReplyRemoteID: id}), ErrInvalid) {
			t.Fatal("accepted invalid provider reply identifier")
		}
	}
}

func TestMailerOutgoingAggregateAttachmentLimit(t *testing.T) {
	for _, sizes := range [][]int64{{-1}, {maxOutgoingAttachmentBytes + 1}, {maxOutgoingAttachmentBytes, 1}} {
		draft := Draft{}
		for _, size := range sizes {
			draft.Attachments = append(draft.Attachments, Attachment{Size: size})
		}
		if !errors.Is(validateOutgoingSize(draft), ErrInvalid) {
			t.Fatalf("accepted unsafe attachment sizes %v", sizes)
		}
	}
}
