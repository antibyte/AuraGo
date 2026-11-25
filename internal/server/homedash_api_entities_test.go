package server

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func TestHomeDashEntitiesEntityHistorySnapshot(t *testing.T) {
	fake := sourcetest.New("fake",
		sourcetest.Entity("fake:temp", homedash.KindSensor, homedash.CapabilitySpec{Name: homedash.CapNumeric}),
		sourcetest.Entity("fake:lamp", homedash.KindLight))
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))

	var list struct {
		Entities []homedash.Entity       `json:"entities"`
		Total    int                     `json:"total"`
		Sources  []homedash.SourceStatus `json:"sources"`
	}
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "entities", nil), &list)
	if list.Total != 2 || list.Entities[0].ID != "fake:lamp" || list.Entities[1].ID != "fake:temp" || len(list.Sources) != 1 {
		t.Fatalf("entities = %+v", list)
	}
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "entities?kind=light", nil), &list)
	if list.Total != 1 {
		t.Fatalf("kind filter = %+v", list)
	}
	for _, q := range []string{"entities?limit=0", "entities?limit=501", "entities?kind=nope", "entities?source=Bad"} {
		if w := env.do(t, env.admin, http.MethodGet, q, nil); w.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d", q, w.Code)
		}
	}
	var one struct{ Entity homedash.Entity }
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "entity?id=fake:temp", nil), &one)
	if one.Entity.Kind != homedash.KindSensor {
		t.Fatalf("entity = %+v", one)
	}
	if w := env.do(t, env.admin, http.MethodGet, "entity?id=fake:none", nil); w.Code != http.StatusNotFound || hdCode(t, w) != "entity_not_found" {
		t.Fatalf("unknown entity = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodGet, "entity?id=", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("empty id = %d", w.Code)
	}

	// History: the ring records only subscribed entities.
	sub, err := env.svc.hub.Attach(homedash.Principal{Kind: homedash.PrincipalSession, Key: "test"})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if _, _, err := sub.Set([]homedash.EntityID{"fake:temp"}); err != nil {
		t.Fatal(err)
	}
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return fake.Running() == 1 }, "source running")
	base := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Minute)
	for i, v := range []float64{20, 21, 22} {
		fake.Push(homedash.StateUpdate{Native: "temp", State: homedash.State{Value: v, Available: true, UpdatedAt: base.Add(time.Duration(2*i) * time.Minute)}})
	}
	from, to := base.Add(-time.Hour).Format(time.RFC3339), base.Add(time.Hour).Format(time.RFC3339)
	var hist struct {
		Origin string            `json:"origin"`
		Since  *time.Time        `json:"since"`
		Points []homedash.Sample `json:"points"`
	}
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "history?id=fake:temp&from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil), &hist)
	if hist.Origin != "memory" || hist.Since == nil || len(hist.Points) != 3 || hist.Points[2].V != 22 {
		t.Fatalf("history = %+v", hist)
	}
	if w := env.do(t, env.admin, http.MethodGet, "history?id=fake:temp&points=501", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("points 501 = %d", w.Code)
	}
	if w := env.do(t, env.admin, http.MethodGet, "snapshot?id=fake:temp", nil); w.Code != http.StatusBadRequest || hdCode(t, w) != "unsupported_command" {
		t.Fatalf("snapshot = %d %s", w.Code, w.Body.String())
	}
}
