package tools

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/testutil"
)

func TestHACallServiceStatesReturnsChangedStates(t *testing.T) {
	var mu sync.Mutex
	var method, path, auth string
	var body map[string]any
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Unlock()
		if r.URL.Path == "/api/services/light/toggle" {
			_, _ = w.Write([]byte(`{"changed_states":[{"entity_id":"light.kitchen","state":"off","attributes":{},"last_changed":"2026-11-25T10:00:00.5+00:00","last_updated":"2026-11-25T10:00:00.5+00:00"}],"service_response":null}`))
			return
		}
		_, _ = w.Write([]byte(`[{"entity_id":"light.kitchen","state":"on","attributes":{"brightness":102},"last_changed":"2026-11-25T10:00:00.5+00:00","last_updated":"2026-11-25T10:00:00.5+00:00"},{"entity_id":"bad id","state":"x"}]`))
	}))
	defer ha.Close()
	cfg := HAConfig{URL: ha.URL, AccessToken: "tok"}
	states, err := HACallServiceStates(context.Background(), cfg, "light", "turn_on", "light.kitchen", map[string]any{"brightness_pct": 40.0})
	if err != nil || len(states) != 1 || states[0].State != "on" || states[0].Attributes["brightness"] != 102.0 || states[0].LastUpdated.IsZero() {
		t.Fatalf("states = %+v %v", states, err)
	}
	mu.Lock()
	if method != http.MethodPost || path != "/api/services/light/turn_on" || auth != "Bearer tok" || body["entity_id"] != "light.kitchen" || body["brightness_pct"] != 40.0 {
		t.Fatalf("request = %s %s %q %v", method, path, auth, body)
	}
	mu.Unlock()
	states, err = HACallServiceStates(context.Background(), cfg, "light", "toggle", "light.kitchen", nil)
	if err != nil || len(states) != 1 || states[0].State != "off" {
		t.Fatalf("changed_states shape = %+v %v", states, err)
	}
}

func TestHACallServiceStatesRefusesBeforeTheNetwork(t *testing.T) {
	var hits atomic.Int32
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); _, _ = w.Write([]byte(`[]`)) }))
	defer ha.Close()
	base := HAConfig{URL: ha.URL, AccessToken: "tok"}
	ro := base
	ro.ReadOnly = true
	blocked := base
	blocked.BlockedServices = []string{"switch.turn_on"}
	for _, tc := range []struct {
		name                    string
		cfg                     HAConfig
		domain, service, entity string
		data                    map[string]any
		kind                    HAServiceErrorKind
	}{
		{"read-only", ro, "switch", "turn_on", "switch.a", nil, HAServicePolicy},
		{"blocked", blocked, "switch", "turn_on", "switch.a", nil, HAServicePolicy},
		{"cover needs allowlist", base, "cover", "open_cover", "cover.g", nil, HAServicePolicy},
		{"script needs allowlist", base, "script", "turn_on", "script.s", nil, HAServicePolicy},
		{"bad domain", base, "Light", "turn_on", "light.a", nil, HAServiceInvalid},
		{"bad service", base, "light", "turn on", "light.a", nil, HAServiceInvalid},
		{"group target", base, "light", "turn_on", "group.all", nil, HAServiceInvalid},
		{"no entity", base, "light", "turn_on", "", nil, HAServiceInvalid},
		{"area in data", base, "light", "turn_on", "light.a", map[string]any{"area_id": "kitchen"}, HAServiceInvalid},
		{"second entity in data", base, "light", "turn_on", "light.a", map[string]any{"entity_id": "light.b"}, HAServiceInvalid},
		{"NaN data", base, "light", "turn_on", "light.a", map[string]any{"brightness_pct": math.NaN()}, HAServiceInvalid},
		{"+Inf data", base, "light", "turn_on", "light.a", map[string]any{"brightness_pct": math.Inf(1)}, HAServiceInvalid},
		{"-Inf data", base, "light", "turn_on", "light.a", map[string]any{"brightness_pct": math.Inf(-1)}, HAServiceInvalid},
	} {
		_, err := HACallServiceStates(context.Background(), tc.cfg, tc.domain, tc.service, tc.entity, tc.data)
		var se *HAServiceError
		if !errors.As(err, &se) || se.Kind != tc.kind {
			t.Errorf("%s: %v, want kind %s", tc.name, err, tc.kind)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("server saw %d requests", hits.Load())
	}
}

func TestHACallServiceStatesClassifiesFailuresWithoutUpstreamText(t *testing.T) {
	const secret = "upstream secret detail"
	var status atomic.Int32
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := int(status.Load())
		w.WriteHeader(code)
		if code >= 300 {
			_, _ = w.Write([]byte(`{"message":"` + secret + ` tok-live-123"}`))
			return
		}
		_, _ = w.Write([]byte(`<html>not json</html>`))
	}))
	cfg := HAConfig{URL: ha.URL, AccessToken: "tok-live-123"}
	for _, tc := range []struct {
		status int
		kind   HAServiceErrorKind
	}{{400, HAServiceRejected}, {404, HAServiceRejected}, {401, HAServiceAuth}, {403, HAServiceAuth}, {500, HAServiceUpstream}, {503, HAServiceUpstream}, {200, HAServiceBadAnswer}} {
		status.Store(int32(tc.status))
		_, err := HACallServiceStates(context.Background(), cfg, "switch", "toggle", "switch.a", nil)
		var se *HAServiceError
		if !errors.As(err, &se) || se.Kind != tc.kind || se.Status != tc.status {
			t.Fatalf("HTTP %d: %v", tc.status, err)
		}
		if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "not json") {
			t.Fatalf("HTTP %d: upstream text in Error(): %v", tc.status, err)
		}
		if tc.status >= 300 && (!strings.Contains(se.LogDetail, secret) || strings.Contains(se.LogDetail, "tok-live-123")) {
			t.Fatalf("HTTP %d: log detail %q must keep the text and scrub the token", tc.status, se.LogDetail)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := HACallServiceStates(ctx, cfg, "switch", "toggle", "switch.a", nil)
	var se *HAServiceError
	if !errors.As(err, &se) || se.Kind != HAServiceUnreachable || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled = %v", err)
	}
	ha.Close()
	if _, err := HACallServiceStates(context.Background(), cfg, "switch", "toggle", "switch.a", nil); !errors.As(err, &se) || se.Kind != HAServiceUnreachable {
		t.Fatalf("closed server = %v", err)
	}
}
