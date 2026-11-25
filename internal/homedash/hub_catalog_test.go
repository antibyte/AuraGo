package homedash_test

import (
	"context"
	"testing"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func TestSourceCatalogReadsRawEntitiesWithoutGatesOrStart(t *testing.T) {
	fa := sourcetest.New("fa", sourcetest.Entity("fa:light", homedash.KindLight, homedash.CapabilitySpec{Name: homedash.CapToggle, Allowed: true}))
	h := newTestHub(t, func(c *homedash.RuntimeConfig) { c.ReadOnly = true }, fa)
	ctx := context.Background()

	list, _, active := h.SourceCatalog(ctx, "fa", false)
	if !active || len(list) != 0 || fa.CatalogCalls() != 0 {
		t.Fatalf("cold cached read = %v active=%v calls=%d", list, active, fa.CatalogCalls())
	}
	list, st, active := h.SourceCatalog(ctx, "fa", true)
	if !active || len(list) != 1 || fa.CatalogCalls() != 1 || st.Source != "fa" {
		t.Fatalf("fetch = %v %+v active=%v calls=%d", list, st, active, fa.CatalogCalls())
	}
	if !list[0].Capabilities[0].Allowed || list[0].Capabilities[0].BlockedBy != "" {
		t.Fatalf("raw read applied the hub gate: %+v", list[0].Capabilities)
	}
	if e, err := h.Entity(ctx, "fa:light"); err != nil || e.Capabilities[0].Allowed {
		t.Fatalf("the entity view must apply homedash.readonly: %+v %v", e, err)
	}
	list[0].Capabilities[0].Allowed = false
	again, _, _ := h.SourceCatalog(ctx, "fa", true)
	if !again[0].Capabilities[0].Allowed || fa.CatalogCalls() != 1 {
		t.Fatalf("copies and freshness: %+v calls=%d", again, fa.CatalogCalls())
	}
	if fa.Runs() != 0 {
		t.Fatal("SourceCatalog started the source")
	}
	if _, _, active := h.SourceCatalog(ctx, "nope", true); active {
		t.Fatal("unknown source reported active")
	}
}

func TestSourceCatalogCopyIsDeep(t *testing.T) {
	lo, hi := 1.0, 9.0
	e := sourcetest.Entity("fa:sel", homedash.KindLight,
		homedash.CapabilitySpec{Name: homedash.CapToggle, Allowed: true, Min: &lo, Max: &hi, Options: []string{"a", "b"}})
	e.Media = &homedash.MediaInfo{Snapshot: true}
	fa := sourcetest.New("fa", e)
	h := newTestHub(t, nil, fa)
	ctx := context.Background()

	list, _, _ := h.SourceCatalog(ctx, "fa", true)
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	list[0].Capabilities[0].Options[0] = "changed"
	*list[0].Capabilities[0].Min = 99
	list[0].Media.Snapshot = false
	again, _, _ := h.SourceCatalog(ctx, "fa", false)
	c := again[0].Capabilities[0]
	if c.Options[0] != "a" || *c.Min != 1 || !again[0].Media.Snapshot {
		t.Fatalf("hub state altered through a copy: %+v media=%+v", c, again[0].Media)
	}
}
