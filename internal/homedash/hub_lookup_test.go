package homedash_test

import (
	"context"
	"errors"
	"testing"
	"time"

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

// catalogFailHub returns a hub with one source whose first Catalog call fails after a delay.
func catalogFailHub(t *testing.T) *homedash.Hub {
	t.Helper()
	fa := sourcetest.New("fa", sourcetest.Entity("fa:x", homedash.KindSensor))
	fa.CatalogScript = []sourcetest.CatalogStep{{Delay: 150 * time.Millisecond, Err: errors.New("boom")}}
	h := homedash.NewHub(context.Background(), homedash.Options{IdleStop: time.Second, CatalogTTL: time.Minute, CatalogTimeout: 2 * time.Second})
	t.Cleanup(h.Close)
	h.Register(fa.ID(), func(homedash.RuntimeConfig) (homedash.Source, bool) { return fa, true })
	h.Reconfigure(homedash.RuntimeConfig{Enabled: true, Sources: map[homedash.SourceID]homedash.SourceSettings{"fa": {Enabled: true}}})
	sourcetest.WaitFor(t, time.Second, func() bool { return h.Sources()[0].State != homedash.StatusDisabled }, "configured")
	return h
}

func TestPrefetchCatalogsCallerCancelDoesNotMarkSourceFailed(t *testing.T) {
	h := catalogFailHub(t)
	before := h.Sources()[0]
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(30*time.Millisecond, cancel)
	h.PrefetchCatalogs(ctx, []homedash.EntityID{"fa:x"})
	got := h.Sources()[0]
	if got.State != before.State || got.Reason != before.Reason {
		t.Fatalf("caller cancel changed the status: before %+v, after %+v", before, got)
	}
}

func TestPrefetchCatalogsRealFailureStillMarksSourceFailed(t *testing.T) {
	h := catalogFailHub(t)
	h.PrefetchCatalogs(context.Background(), []homedash.EntityID{"fa:x"})
	got := h.Sources()[0]
	if got.State != homedash.StatusUnavailable || got.Reason != homedash.ReasonCatalogFailed {
		t.Fatalf("status after a real failure = %+v", got)
	}
}
