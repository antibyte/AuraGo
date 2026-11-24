package tools

import (
	"bytes"
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/testutil"
)

func TestHARequestJSONKeepsSameOriginRedirectPolicy(t *testing.T) {
	var foreignHits atomic.Int32
	foreign := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignHits.Add(1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer foreign.Close()
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/same":
			http.Redirect(w, r, "/api/target", http.StatusFound)
		case "/api/target":
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/cross":
			http.Redirect(w, r, foreign.URL+"/api/target", http.StatusFound)
		case "/api/denied":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`secret echo`))
		}
	}))
	defer ha.Close()
	cfg, ctx := HAConfig{URL: ha.URL, AccessToken: "tok"}, context.Background()
	var out struct {
		OK bool `json:"ok"`
	}
	if err := HARequestJSON(ctx, cfg, http.MethodGet, "/api/same", nil, &out); err != nil || !out.OK {
		t.Fatalf("same-origin redirect: %v %+v", err, out)
	}
	if err := HARequestJSON(ctx, cfg, http.MethodGet, "/api/cross", nil, &out); err == nil || foreignHits.Load() != 0 {
		t.Fatalf("cross-origin redirect followed: %v hits=%d", err, foreignHits.Load())
	}
	err := HARequestJSON(ctx, cfg, http.MethodGet, "/api/denied", nil, nil)
	var he *HAHTTPError
	if !errors.As(err, &he) || he.Status != http.StatusForbidden || strings.Contains(err.Error(), "secret") {
		t.Fatalf("non-2xx: %v", err)
	}
	if err := HARequestJSON(ctx, cfg, http.MethodGet, "/other", nil, nil); err == nil {
		t.Fatal("endpoint outside /api/ accepted")
	}
}

func TestHAGetBytesCapsSizeAndReturnsContentType(t *testing.T) {
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(bytes.Repeat([]byte{0xff}, 100))
	}))
	defer ha.Close()
	cfg := HAConfig{URL: ha.URL, AccessToken: "tok"}
	if _, _, err := HAGetBytes(context.Background(), cfg, "/api/camera_proxy/camera.x", 50); err == nil {
		t.Fatal("size cap not applied")
	}
	data, ct, err := HAGetBytes(context.Background(), cfg, "/api/camera_proxy/camera.x", 200)
	if err != nil || len(data) != 100 || ct != "image/jpeg" {
		t.Fatalf("%d bytes, %q, %v", len(data), ct, err)
	}
}

func TestHARequestJSONIsReadOnly(t *testing.T) {
	var hits atomic.Int32
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ha.Close()
	cfg, ctx := HAConfig{URL: ha.URL, AccessToken: "tok"}, context.Background()
	cases := []struct {
		name, method, endpoint string
		body                   any
	}{
		{"post", http.MethodPost, "/api/services/light/turn_on", nil},
		{"put", http.MethodPut, "/api/states/x", nil},
		{"delete", http.MethodDelete, "/api/states/x", nil},
		{"patch", http.MethodPatch, "/api/states/x", nil},
		{"lowercase get", "get", "/api/states/x", nil},
		{"get with body", http.MethodGet, "/api/states/x", map[string]string{"a": "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := HARequestJSON(ctx, cfg, tc.method, tc.endpoint, tc.body, nil); err == nil {
				t.Fatal("request accepted")
			}
		})
	}
	if hits.Load() != 0 {
		t.Fatalf("server saw %d requests", hits.Load())
	}
}

func TestHAValidEndpointRejectsTraversalAndForeignTargets(t *testing.T) {
	rejected := []string{
		"", "/other", "api/states", "//evil.example/api/x", "http://evil.example/api/x",
		"/api/../config", "/api/x/..", "/api/%2e%2e/config", "/api/%2E%2E/config", "/api/x/%2e%2e",
		"/api/./x", "/api//x", "/api/a%2fb", "/api/a%5cb", "/api/a" + string(rune(92)) + "b",
		"/api/a b", "/api/a" + string(rune(13)) + "b", "/api/a" + string(rune(10)) + "b", "/api/x#frag",
	}
	for _, e := range rejected {
		if err := haValidEndpoint(e); err == nil {
			t.Errorf("accepted %q", e)
		}
	}
	accepted := []string{
		"/api/states", "/api/camera_proxy/camera.front_door",
		"/api/history/period/2026-11-23T00:00:00Z?filter_entity_id=sensor.a&end_time=2026-11-24T00:00:00Z&minimal_response&no_attributes",
	}
	for _, e := range accepted {
		if err := haValidEndpoint(e); err != nil {
			t.Errorf("rejected %q: %v", e, err)
		}
	}
}

func TestHAGetBytesRejectsUnboundedMax(t *testing.T) {
	var hits atomic.Int32
	ha := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("x"))
	}))
	defer ha.Close()
	cfg, ctx := HAConfig{URL: ha.URL, AccessToken: "tok"}, context.Background()
	for _, m := range []int64{0, -1, 8<<20 + 1, math.MaxInt64} {
		if _, _, err := HAGetBytes(ctx, cfg, "/api/camera_proxy/camera.x", m); err == nil {
			t.Errorf("max %d accepted", m)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("server saw %d requests", hits.Load())
	}
	if _, _, err := HAGetBytes(ctx, cfg, "/api/camera_proxy/camera.x", 8<<20); err != nil {
		t.Fatalf("max 8 MiB rejected: %v", err)
	}
}
