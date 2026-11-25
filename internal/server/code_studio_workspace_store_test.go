package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCodeStudioWorkspaceStoreUpsertListDelete(t *testing.T) {
	s := testCodeStudioServerWithFakeCodeContainer(t, 1)
	store, err := codeStudioHandlers{server: s}.workspaceStore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tick := time.Unix(1_700_000_000, 0).UTC()
	store.now = func() time.Time { tick = tick.Add(time.Second); return tick }
	ctx := context.Background()
	a, err := store.Upsert(ctx, "dev1", "/home/u/proj", "Proj", &codeStudioCapabilities{Git: true})
	if err != nil || !strings.HasPrefix(a.ID, "ssh_") || !codeStudioWorkspaceIDPattern.MatchString(a.ID) {
		t.Fatal(a, err)
	}
	b, _ := store.Upsert(ctx, "dev1", "/home/u/proj", "", nil) // same device+root -> same id, label kept
	if b.ID != a.ID || b.Label != "Proj" || b.Capabilities == nil || !b.Capabilities.Git {
		t.Fatalf("%+v", b)
	}
	c, _ := store.Upsert(ctx, "dev1", "/srv/other", "", nil)
	list, _ := store.List(ctx)
	if len(list) != 2 || list[0].ID != c.ID {
		t.Fatalf("order %+v", list)
	}
	if n, _ := store.CountDevice(ctx, "dev1"); n != 2 {
		t.Fatal(n)
	}
	if _, err := store.Get(ctx, "ssh_nope"); !errors.Is(err, errCodeStudioWorkspaceRecordNotFound) {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, a.ID); !errors.Is(err, errCodeStudioWorkspaceRecordNotFound) {
		t.Fatal(err)
	}
}
