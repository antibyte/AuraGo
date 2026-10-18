package contacts

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestContactLockCancellationAndConcurrentUpdates(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "contacts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, err := Create(db, Contact{Name: "Synthetic contact"})
	if err != nil {
		t.Fatal(err)
	}
	unlock := LockID(id)
	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan error, 1)
	go func() {
		release, err := LockIDContext(ctx, id)
		if release != nil {
			release()
		}
		finished <- err
	}()
	cancel()
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued operation did not cancel: %v", err)
	}
	unlock()
	unlock() // Defensive duplicate release cannot unlock a subsequent operation.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Update(db, Contact{ID: id, Name: "Updated synthetic contact"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if c, err := GetByID(db, id); err != nil || c.Name != "Updated synthetic contact" {
		t.Fatalf("contact=%+v err=%v", c, err)
	}
	contactLocks.Lock()
	_, remaining := contactLocks.entries[id]
	contactLocks.Unlock()
	if remaining {
		t.Fatal("contact lock entry leaked")
	}
}
