package mailer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type deliveryTestAdapter struct {
	coreTestAdapter
	result      SendResult
	sendErr     error
	appendErr   error
	sendCalls   int
	appendCalls int
}

func (a *deliveryTestAdapter) Send(context.Context, Outgoing) (SendResult, error) {
	a.sendCalls++
	return a.result, a.sendErr
}

func (a *deliveryTestAdapter) AppendSent(context.Context, Outgoing, SendResult) error {
	a.appendCalls++
	return a.appendErr
}

type outboxFixture struct {
	account *Account
	adapter *deliveryTestAdapter
	service *Service
	store   *Store
}

func newOutboxFixture(t *testing.T, databasePath string) *outboxFixture {
	t.Helper()
	store, err := OpenStore(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	account := &Account{
		ID: "email:delivery", Source: "email", Protocol: "imap",
		Capabilities: []string{"read", "send"},
	}
	adapter := &deliveryTestAdapter{}
	service, err := NewService(ServiceOptions{
		Store: store, BlobRoot: t.TempDir(),
		ListSources:    func(context.Context) ([]Account, error) { return []Account{*account}, nil },
		ResolveAdapter: func(context.Context, string) (Adapter, Account, error) { return adapter, *account, nil },
	})
	if err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if err := service.Connect(context.Background(), account.ID); err != nil {
		_ = service.Close()
		_ = store.Close()
		t.Fatal(err)
	}
	if err := service.UpdateSettings(context.Background(), Settings{Enabled: true}); err != nil {
		_ = service.Close()
		_ = store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = service.Close()
		_ = store.Close()
	})
	return &outboxFixture{account: account, adapter: adapter, service: service, store: store}
}

func (f *outboxFixture) queueDue(t *testing.T) OutboxItem {
	t.Helper()
	draft, err := f.service.SaveDraft(context.Background(), Draft{
		AccountID: f.account.ID,
		To:        []string{"recipient@example.test"},
		Subject:   "delivery test",
		Text:      "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	item, err := f.service.QueueSend(context.Background(), SendRequest{DraftID: draft.ID, Revision: draft.Revision, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.db.ExecContext(context.Background(), "UPDATE mailer_outbox SET send_at=? WHERE id=?", timeNS(time.Now().Add(-time.Second)), item.ID); err != nil {
		t.Fatal(err)
	}
	return item
}

func outboxItem(t *testing.T, store *Store, id string) OutboxItem {
	t.Helper()
	items, err := store.ListOutbox(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("outbox item %q not found", id)
	return OutboxItem{}
}

func TestDispatchConfirmedSendAppendFailureNeverRedelivers(t *testing.T) {
	f := newOutboxFixture(t, ":memory:")
	f.adapter.result = SendResult{Confirmed: true, RemoteID: "provider-1"}
	f.adapter.appendErr = errors.New("sent folder unavailable")
	item := f.queueDue(t)

	f.service.dispatchDue(context.Background())
	got := outboxItem(t, f.store, item.ID)
	if got.Status != "sent" || got.RemoteID != "provider-1" || got.AppendError != "sent_copy_failed" {
		t.Fatalf("confirmed delivery state = %#v, want sent with safe append error", got)
	}
	if f.adapter.sendCalls != 1 || f.adapter.appendCalls != 1 {
		t.Fatalf("send/append calls = %d/%d, want 1/1", f.adapter.sendCalls, f.adapter.appendCalls)
	}
	f.service.dispatchDue(context.Background())
	if f.adapter.sendCalls != 1 || f.adapter.appendCalls != 1 {
		t.Fatalf("second dispatch retried confirmed delivery: send/append calls = %d/%d", f.adapter.sendCalls, f.adapter.appendCalls)
	}
}

func TestDispatchUncertainSendErrorNeverAutomaticallyReplays(t *testing.T) {
	f := newOutboxFixture(t, ":memory:")
	f.adapter.sendErr = errors.New("transport disconnected after request")
	item := f.queueDue(t)

	f.service.dispatchDue(context.Background())
	got := outboxItem(t, f.store, item.ID)
	if got.Status != "unknown" || got.Error != "transport_unknown" {
		t.Fatalf("uncertain delivery state = %#v, want unknown with safe error code", got)
	}
	f.service.dispatchDue(context.Background())
	if f.adapter.sendCalls != 1 {
		t.Fatalf("uncertain delivery replayed automatically %d times", f.adapter.sendCalls)
	}
}

func TestDispatchDisabledAccountDoesNotSend(t *testing.T) {
	f := newOutboxFixture(t, ":memory:")
	item := f.queueDue(t)
	f.account.Disabled = true

	f.service.dispatchDue(context.Background())
	got := outboxItem(t, f.store, item.ID)
	if got.Status != "failed" || got.Error != "account_unavailable" {
		t.Fatalf("disabled-account delivery state = %#v, want failed/account_unavailable", got)
	}
	if f.adapter.sendCalls != 0 {
		t.Fatalf("disabled account reached provider Send %d times", f.adapter.sendCalls)
	}
}

func TestDispatchMissingAttachmentDoesNotSend(t *testing.T) {
	f := newOutboxFixture(t, ":memory:")
	draft, err := f.service.SaveDraft(context.Background(), Draft{
		AccountID: f.account.ID,
		To:        []string{"recipient@example.test"},
		Subject:   "attachment test",
		Text:      "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	draft, err = f.service.AddDraftAttachment(context.Background(), draft.ID, draft.Revision, "note.txt", "text/plain", strings.NewReader("attachment"))
	if err != nil {
		t.Fatal(err)
	}
	item, err := f.service.QueueSend(context.Background(), SendRequest{DraftID: draft.ID, Revision: draft.Revision, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.db.ExecContext(context.Background(), "UPDATE mailer_outbox SET send_at=? WHERE id=?", timeNS(time.Now().Add(-time.Second)), item.ID); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.service.blobs.root, draft.Attachments[0].ID+".blob")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	f.service.dispatchDue(context.Background())
	got := outboxItem(t, f.store, item.ID)
	if got.Status != "failed" || got.Error != "attachment_unavailable" {
		t.Fatalf("missing-attachment delivery state = %#v, want failed/attachment_unavailable", got)
	}
	if f.adapter.sendCalls != 0 {
		t.Fatalf("missing attachment reached provider Send %d times", f.adapter.sendCalls)
	}
}

func TestDispatchPausedMailerLeavesDueJobQueued(t *testing.T) {
	f := newOutboxFixture(t, ":memory:")
	f.adapter.result = SendResult{Confirmed: true, RemoteID: "provider-2"}
	item := f.queueDue(t)
	if err := f.service.UpdateSettings(context.Background(), Settings{Enabled: false}); err != nil {
		t.Fatal(err)
	}

	f.service.dispatchDue(context.Background())
	got := outboxItem(t, f.store, item.ID)
	if got.Status != "scheduled" || f.adapter.sendCalls != 0 {
		t.Fatalf("paused job state=%#v sends=%d, want scheduled and no send", got, f.adapter.sendCalls)
	}
	if err := f.service.UpdateSettings(context.Background(), Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	f.service.dispatchDue(context.Background())
	got = outboxItem(t, f.store, item.ID)
	if got.Status != "sent" || f.adapter.sendCalls != 1 {
		t.Fatalf("resumed job state=%#v sends=%d, want sent and one send", got, f.adapter.sendCalls)
	}
}

func TestRestartRecoversSendingAsUnknownWithoutReplay(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "mailer.db")
	f := newOutboxFixture(t, databasePath)
	item := f.queueDue(t)
	if _, _, err := f.store.ClaimDue(context.Background(), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := f.service.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}

	restarted := newOutboxFixture(t, databasePath)
	got := outboxItem(t, restarted.store, item.ID)
	if got.Status != "unknown" {
		t.Fatalf("recovered in-flight delivery status = %q, want unknown", got.Status)
	}
	restarted.service.dispatchDue(context.Background())
	if restarted.adapter.sendCalls != 0 {
		t.Fatalf("restart replayed uncertain delivery %d times", restarted.adapter.sendCalls)
	}
}
