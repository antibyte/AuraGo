package hass

import (
	"context"
	"strings"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
	"aurago/internal/tools"
)

func TestHAWSFetchFailedStatesIsAnErrorWithoutUpstreamText(t *testing.T) {
	o := hasstest.DemoOptions()
	o.FailStates = true
	ha := hasstest.New(t, o)
	c, err := NewWSClient(ha.URL, ha.Token, nopHandler{}, fastWS)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	states, _, err := c.Fetch(ctx)
	if err == nil || states != nil {
		t.Fatalf("Fetch with failing get_states = %v, %v", states, err)
	}
	if strings.Contains(err.Error(), "Scripted failure") || strings.Contains(err.Error(), "unknown_error") {
		t.Fatalf("error carries Home Assistant text: %v", err)
	}
}

func TestHAWSFetchFailedRegistriesLeavesThemEmpty(t *testing.T) {
	o := hasstest.DemoOptions()
	o.FailRegistries = true
	ha := hasstest.New(t, o)
	c, _ := NewWSClient(ha.URL, ha.Token, nopHandler{}, fastWS)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	states, reg, err := c.Fetch(ctx)
	if err != nil || len(states) != 15 {
		t.Fatalf("Fetch = %d states, %v", len(states), err)
	}
	if len(reg.Areas) != 0 || len(reg.Devices) != 0 || len(reg.Entities) != 0 {
		t.Fatalf("registries not empty: %+v", reg)
	}
	a := newTestAdapter(t, ha, tools.HAConfig{})
	list, err := a.Catalog(context.Background())
	if err != nil || len(list) == 0 {
		t.Fatalf("Catalog = %d, %v", len(list), err)
	}
	for _, e := range list {
		if e.Area != "" {
			t.Fatalf("entity %s has area %q without a registry", e.ID, e.Area)
		}
	}
}

func TestHAAdapterRunWithFailingStatesDegradesAsProtocol(t *testing.T) {
	o := hasstest.DemoOptions()
	o.FailStates = true
	ha := hasstest.New(t, o)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"switch.coffee"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 3*time.Second, func() bool { return sink.sawStatus(homedash.StatusDegraded, ReasonProtocol) }, "degraded/protocol")
	if sink.sawStatus(homedash.StatusLive, "") {
		t.Fatal("adapter went live although get_states failed")
	}
}

func TestHACommandFeedsWrappedChangedStates(t *testing.T) {
	o := hasstest.DemoOptions()
	o.NoEvents = true // only the service answer can carry the new state
	o.ChangedStatesWrapped = true
	o.States[0].Attributes["access_token"] = "leak-token-2"
	ha := hasstest.New(t, o)
	a := newTestAdapter(t, ha, tools.HAConfig{})
	a.SetInterest([]string{"light.kitchen"})
	sink := &lockedSink{}
	runAdapter(t, a, sink)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return sink.sawStatus(homedash.StatusLive, "") }, "live")
	res, err := a.Command(context.Background(), homedash.Command{Entity: "ha:light.kitchen", Native: "light.kitchen", Name: homedash.CapBrightness, Args: map[string]any{"value": 60.0}})
	if err != nil || res.Expected == nil || res.Expected.Attributes["brightness_pct"] != 60.0 {
		t.Fatalf("command = %+v %v", res, err)
	}
	st, ok := sink.last("light.kitchen")
	if !ok || st.Attributes["brightness_pct"] != 60.0 || st.Value != true {
		t.Fatalf("fed state = %+v %v", st, ok)
	}
	for _, k := range []string{"entity_picture", "access_token", "icon"} {
		if _, bad := st.Attributes[k]; bad {
			t.Errorf("attribute %q reached the sink", k)
		}
	}
}
