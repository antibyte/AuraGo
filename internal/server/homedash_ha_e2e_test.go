package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass"
	"aurago/internal/homedash/sources/hass/hasstest"
	"aurago/internal/homedash/sourcetest"
)

func newHomeDashHAEnv(t *testing.T, ha *hasstest.Server) *homeDashTestEnv {
	t.Helper()
	spec := homeDashHASpec(hass.FactoryOptions{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		WS: hass.WSOptions{HandshakeTimeout: time.Second, PingInterval: time.Hour, PongTimeout: time.Minute, AuthRetry: time.Hour,
			RegistryDebounce: 20 * time.Millisecond, Backoff: []time.Duration{20 * time.Millisecond}, Jitter: -1}})
	// Registered before the env's own cleanup, so it runs after the HomeDash service closed: no
	// websocket may stay open at the fake.
	t.Cleanup(func() {
		sourcetest.WaitFor(t, 5*time.Second, func() bool { return ha.OpenWebsockets() == 0 }, "fake HA has no open websocket")
	})
	return newHomeDashTestEnv(t, func(c *config.Config) {
		c.HomeAssistant.Enabled, c.HomeAssistant.URL, c.HomeAssistant.AccessToken = true, ha.URL, ha.Token
		c.HomeDash.Sources.HomeAssistant = config.HomeDashSourceConfig{Enabled: true, Control: true, PollSeconds: 5}
	}, spec)
}

func waitHAState(t *testing.T, events <-chan hdEvent, id string, match func(homedash.State) bool) homedash.State {
	t.Helper()
	for i := 0; i < 50; i++ {
		var st struct {
			Changes map[string]homedash.State `json:"changes"`
		}
		if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "state").data), &st); err != nil {
			t.Fatal(err)
		}
		if s, ok := st.Changes[id]; ok && match(s) {
			return s
		}
	}
	t.Fatalf("no matching state frame for %s", id)
	return homedash.State{}
}

// M3's result end to end: template -> stream -> snapshot -> command -> the fake Home Assistant
// receives the service call -> a state frame confirms.
func TestHomeDashHomeAssistantEndToEnd(t *testing.T) {
	shortenHomeDashStreamTimers(t)
	ha := hasstest.New(t, hasstest.DemoOptions())
	env := newHomeDashHAEnv(t, ha)

	sourcetest.WaitFor(t, 5*time.Second, func() bool {
		var boot struct {
			Templates []string `json:"templates"`
		}
		hdDecode(t, env.do(t, env.admin, http.MethodGet, "bootstrap", nil), &boot)
		return slices.Contains(boot.Templates, "home_assistant")
	}, "bootstrap lists home_assistant")

	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "home_assistant", "name": "Zuhause", "unassigned_title": "Ohne Raum"})
	var created struct {
		Dashboard homedash.Dashboard `json:"dashboard"`
		Warnings  []homedash.Issue   `json:"warnings"`
		Omitted   int                `json:"omitted"`
	}
	hdDecode(t, w, &created)
	var names []string
	for _, p := range created.Dashboard.Pages {
		names = append(names, p.Name)
	}
	if w.Code != http.StatusCreated || created.Omitted != 0 || len(created.Warnings) != 0 || !slices.Equal(names, []string{"Kitchen", "Living room", "Ohne Raum"}) {
		t.Fatalf("template = %d %s", w.Code, w.Body.String())
	}

	id, events, stop := openHomeDashStream(t, env, env.admin)
	defer stop()
	bound := homeDashBoundIDs(created.Dashboard)
	ids := make([]string, len(bound))
	for i, b := range bound {
		ids[i] = string(b)
	}
	if w := env.do(t, env.admin, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": id, "entities": ids}); w.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
	}
	seen := map[homedash.EntityID]homedash.Entity{}
	for len(seen) < len(ids) {
		var snap struct {
			Entities []homedash.Entity `json:"entities"`
		}
		if err := json.Unmarshal([]byte(waitHomeDashEvent(t, events, "snapshot").data), &snap); err != nil {
			t.Fatal(err)
		}
		for _, e := range snap.Entities {
			seen[e.ID] = e
		}
	}
	kitchen := seen["ha:light.kitchen"]
	if kitchen.Kind != homedash.KindLight || kitchen.State.Attributes["brightness_pct"] != 40.0 || kitchen.AreaID != "kitchen" {
		t.Fatalf("snapshot light = %+v", kitchen)
	}

	w = env.do(t, env.admin, http.MethodPost, "commands", map[string]any{"entity": "ha:light.kitchen", "command": "brightness", "args": map[string]any{"value": 60}, "request_id": "e2e-1"})
	var acc struct {
		Accepted   bool            `json:"accepted"`
		Superseded bool            `json:"superseded"`
		Expected   *homedash.State `json:"expected"`
	}
	hdDecode(t, w, &acc)
	if w.Code != http.StatusAccepted || !acc.Accepted || acc.Expected == nil || acc.Expected.Attributes["brightness_pct"] != 60.0 {
		t.Fatalf("command = %d %s", w.Code, w.Body.String())
	}
	calls := ha.Calls()
	if last := calls[len(calls)-1]; last.Domain != "light" || last.Service != "turn_on" || last.Data["entity_id"] != "light.kitchen" || last.Data["brightness_pct"] != 60.0 {
		t.Fatalf("service call = %+v", last)
	}
	waitHAState(t, events, "ha:light.kitchen", func(s homedash.State) bool { return s.Attributes["brightness_pct"] == 60.0 })

	w = env.do(t, env.admin, http.MethodPost, "commands", map[string]any{"entity": "ha:switch.coffee", "command": "toggle", "request_id": "e2e-2"})
	hdDecode(t, w, &acc)
	if w.Code != http.StatusAccepted || acc.Expected == nil || acc.Expected.Value != true {
		t.Fatalf("toggle = %d %s", w.Code, w.Body.String())
	}
	waitHAState(t, events, "ha:switch.coffee", func(s homedash.State) bool { return s.Value == true })
	if !env.audits.has("ha:light.kitchen|brightness|ok") || !env.audits.has("ha:switch.coffee|toggle|ok") {
		t.Fatalf("audit = %+v", env.audits.entries)
	}
}

func TestHomeDashHomeAssistantUnreachableCommandCarriesSource(t *testing.T) {
	ha := hasstest.New(t, hasstest.DemoOptions())
	env := newHomeDashHAEnv(t, ha)
	if w := env.do(t, env.admin, http.MethodGet, "entity?id=ha:switch.coffee", nil); w.Code != http.StatusOK {
		t.Fatalf("entity = %d %s", w.Code, w.Body.String())
	}
	ha.Close()
	w := env.do(t, env.admin, http.MethodPost, "commands", map[string]any{"entity": "ha:switch.coffee", "command": "toggle", "request_id": "gone-1"})
	var body struct {
		Code, Source string
	}
	hdDecode(t, w, &body)
	if w.Code != http.StatusServiceUnavailable || body.Code != "source_unavailable" || body.Source != "ha" || strings.Contains(w.Body.String(), hasstest.UpstreamSecret) {
		t.Fatalf("answer = %d %s", w.Code, w.Body.String())
	}
	if !env.audits.has("ha:switch.coffee|toggle|failed") {
		t.Fatalf("audit = %+v", env.audits.entries)
	}
}
