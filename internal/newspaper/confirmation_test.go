package newspaper

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestEmailChallengeCancellationOnlyRemovesMatchingCode(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "newspaper.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	address, account := "reader@example.org", "agentmail"
	first, err := store.NewEmailChallenge(ctx, address, account)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.NewEmailChallenge(ctx, address, account); !errors.Is(err, ErrChallengePending) {
		t.Fatalf("recent code did not block a duplicate: %v", err)
	}
	if removed, err := store.CancelEmailChallenge(ctx, address, account, "wrong-code"); err != nil || removed {
		t.Fatalf("wrong code removed challenge: %v, %v", removed, err)
	}
	if removed, err := store.CancelEmailChallenge(ctx, address, account, first); err != nil || !removed {
		t.Fatalf("matching code was not removed: %v, %v", removed, err)
	}
	second, err := store.NewEmailChallenge(ctx, address, account)
	if err != nil || second == first {
		t.Fatalf("retry after known rejection: %q, %v", second, err)
	}
	if removed, err := store.CancelEmailChallenge(ctx, address, account, first); err != nil || removed {
		t.Fatalf("old code removed a replacement: %v, %v", removed, err)
	}
	if _, err := store.NewEmailChallenge(ctx, address, account); !errors.Is(err, ErrChallengePending) {
		t.Fatalf("replacement code was not retained: %v", err)
	}
}
