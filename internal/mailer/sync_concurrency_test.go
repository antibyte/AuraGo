package mailer

import (
	"context"
	"testing"
	"time"
)

type accountSyncTestAdapter struct {
	coreTestAdapter
	blocked bool
	started chan struct{}
	called  chan struct{}
}

func (a *accountSyncTestAdapter) Folders(context.Context) ([]Folder, error) {
	return []Folder{{ID: "INBOX", Name: "Inbox", Kind: "inbox"}}, nil
}

func (a *accountSyncTestAdapter) Sync(ctx context.Context, _, _ string) (SyncResult, error) {
	if a.blocked {
		select {
		case a.started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return SyncResult{}, ctx.Err()
	}
	select {
	case a.called <- struct{}{}:
	default:
	}
	return SyncResult{RemoteSnapshotComplete: true}, nil
}

func TestSyncAllHealthyAccountRunsAlongsideBlockedAccount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	blockedAccount := Account{ID: "email:blocking", Source: "email", Protocol: "imap", Capabilities: []string{"read"}}
	healthyAccount := Account{ID: "email:healthy", Source: "email", Protocol: "imap", Capabilities: []string{"read"}}
	blockedAdapter := &accountSyncTestAdapter{coreTestAdapter: coreTestAdapter{account: blockedAccount}, blocked: true, started: make(chan struct{}, 1)}
	healthyAdapter := &accountSyncTestAdapter{coreTestAdapter: coreTestAdapter{account: healthyAccount}, called: make(chan struct{}, 1)}
	accounts := map[string]Account{blockedAccount.ID: blockedAccount, healthyAccount.ID: healthyAccount}
	adapters := map[string]Adapter{blockedAccount.ID: blockedAdapter, healthyAccount.ID: healthyAdapter}
	service, err := NewService(ServiceOptions{
		Store: store, BlobRoot: t.TempDir(),
		ListSources: func(context.Context) ([]Account, error) {
			return []Account{blockedAccount, healthyAccount}, nil
		},
		ResolveAdapter: func(_ context.Context, id string) (Adapter, Account, error) {
			return adapters[id], accounts[id], nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	for _, account := range []Account{blockedAccount, healthyAccount} {
		if err := service.Connect(ctx, account.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.UpdateSettings(ctx, Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		service.syncAll(ctx)
		close(done)
	}()
	select {
	case <-blockedAdapter.started:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked account sync did not start")
	}
	select {
	case <-healthyAdapter.called:
	case <-time.After(2 * time.Second):
		t.Fatal("healthy account sync waited for the blocked account")
	}
	deadline := time.NewTimer(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		account, err := store.Account(ctx, healthyAccount.ID)
		if err != nil {
			t.Fatal(err)
		}
		if account.SyncStatus == "ok" {
			break
		}
		select {
		case <-deadline.C:
			t.Fatalf("healthy sync status remained %q while the other account was blocked", account.SyncStatus)
		case <-ticker.C:
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sync cycle did not stop after cancellation")
	}
}
