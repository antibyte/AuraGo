package mailer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMailerOutboxDoubleClickAndRevisionReplacement(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	draft, err := store.SaveDraft(ctx, Draft{AccountID: "mail", To: []string{"recipient@example.test"}, Text: "first"})
	if err != nil {
		t.Fatal(err)
	}
	items := make([]OutboxItem, 2)
	errs := make([]error, 2)
	var group sync.WaitGroup
	for i := range items {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			items[i], errs[i] = store.QueueSend(ctx, SendRequest{DraftID: draft.ID, Revision: draft.Revision, Timezone: "UTC"}, draft)
		}(i)
	}
	group.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if items[0].ID != items[1].ID {
		t.Fatal("concurrent send clicks created duplicate delivery jobs")
	}
	draft.Text = "second"
	draft, err = store.SaveDraft(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ClaimDue(ctx, time.Now().Add(time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("edited draft still eligible for old send: %v", err)
	}
	replacement, err := store.QueueSend(ctx, SendRequest{DraftID: draft.ID, Revision: draft.Revision}, draft)
	if err != nil {
		t.Fatal(err)
	}
	claimed, _, err := store.ClaimDue(ctx, time.Now().Add(time.Minute))
	if err != nil || claimed.ID != replacement.ID {
		t.Fatalf("replacement not claimed: %v", err)
	}
	if err := store.RecoverSending(ctx); err != nil {
		t.Fatal(err)
	}
	draft.Text = "must not resend"
	draft, err = store.SaveDraft(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.QueueSend(ctx, SendRequest{DraftID: draft.ID, Revision: draft.Revision}, draft); !errors.Is(err, ErrConflict) {
		t.Fatalf("unknown delivery replayed after editing: %v", err)
	}
}

func TestMailerConfirmedDraftLeavesDraftList(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	draft, err := store.SaveDraft(ctx, Draft{AccountID: "mail", To: []string{"recipient@example.test"}, Text: "body"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.QueueSend(ctx, SendRequest{DraftID: draft.ID, Revision: draft.Revision}, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ClaimDue(ctx, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishSend(ctx, item.ID, "sent", "", "provider-id"); err != nil {
		t.Fatal(err)
	}
	drafts, err := store.ListDrafts(ctx)
	if err != nil || len(drafts) != 0 {
		t.Fatalf("confirmed delivery still listed as draft: %v / %d", err, len(drafts))
	}
	if _, err := store.GetDraft(ctx, draft.ID); err != nil {
		t.Fatalf("delivery record content lost: %v", err)
	}
}
