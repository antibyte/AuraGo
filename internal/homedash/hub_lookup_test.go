package homedash_test

import (
	"context"
	"testing"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func TestPrefetchCatalogsFetchesOncePerSourceWithoutStarting(t *testing.T) {
	a := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor))
	other := sourcetest.New("other", sourcetest.Entity("other:x", homedash.KindSwitch))
	h := newTestHub(t, nil, a, other)
	ctx := context.Background()

	h.PrefetchCatalogs(ctx, []homedash.EntityID{"fake:a", "fake:b", "bad id", "nope:x"})
	if a.CatalogCalls() != 1 || other.CatalogCalls() != 0 {
		t.Fatalf("catalog calls fake=%d other=%d, want 1 and 0", a.CatalogCalls(), other.CatalogCalls())
	}
	if a.Runs() != 0 {
		t.Fatal("PrefetchCatalogs started the source")
	}
	h.PrefetchCatalogs(ctx, []homedash.EntityID{"fake:a"})
	if a.CatalogCalls() != 1 {
		t.Fatal("a fresh catalog was fetched again")
	}

	sub, err := h.Attach(homedash.Principal{Kind: homedash.PrincipalSession, Key: "k"})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if _, _, err := sub.Set([]homedash.EntityID{"fake:a"}); err != nil {
		t.Fatal(err)
	}
	b := sub.Drain()
	if len(b.Snapshot) != 1 || b.Snapshot[0].Kind != homedash.KindSensor || !b.Snapshot[0].State.Available {
		t.Fatalf("snapshot after prefetch = %+v, want the cataloged sensor", b.Snapshot)
	}
}

func TestLookupEntitiesReturnsCatalogedEntitiesWithGates(t *testing.T) {
	a := sourcetest.New("fake",
		sourcetest.Entity("fake:a", homedash.KindSwitch, homedash.CapabilitySpec{Name: homedash.CapToggle, Allowed: true}))
	h := newTestHub(t, func(c *homedash.RuntimeConfig) { c.ReadOnly = true }, a)

	got := h.LookupEntities(context.Background(), []homedash.EntityID{"fake:a", "fake:b", "bad id", "nope:x"})
	if len(got) != 1 {
		t.Fatalf("lookup = %+v, want only fake:a", got)
	}
	e := got["fake:a"]
	if e.Kind != homedash.KindSwitch || !e.ReadOnly || e.ReadOnlyReason != "homedash.readonly" {
		t.Fatalf("fake:a = %+v, want the switch with the homedash.readonly gate applied", e)
	}
	if a.Runs() != 0 {
		t.Fatal("LookupEntities started the source")
	}
}
