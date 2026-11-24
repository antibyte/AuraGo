package tools

import (
	"bytes"
	"context"
	"errors"
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
