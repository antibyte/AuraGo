package sipphone

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreDeleteAllPurgesCallHistory(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "sip_calls.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	for _, id := range []string{"call-1", "call-2"} {
		if err := store.Upsert(ctx, CallRecord{
			ID: id, Direction: "outbound", RemoteParty: "redacted",
			StartedAt: time.Now().UTC(), State: StateEnded, Backend: "classic",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.DeleteAll(ctx); err != nil {
		t.Fatal(err)
	}
	calls, err := store.List(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("call history still contains %d records", len(calls))
	}
}
