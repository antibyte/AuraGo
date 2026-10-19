package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
)

func TestGo2RTCPreloadRestartAndShutdownCleanup(t *testing.T) {
	const old = "aurago~previous-process~one~main"
	var mu sync.Mutex
	aliases := map[string]interface{}{old: map[string]interface{}{}}
	preloads := map[string]interface{}{old: map[string]interface{}{}}
	patches := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/streams") {
			switch r.Method {
			case "GET":
				json.NewEncoder(w).Encode(aliases)
			case "PATCH":
				aliases[r.URL.Query().Get("name")] = map[string]interface{}{}
				patches++
			case "DELETE":
				delete(aliases, r.URL.Query().Get("src"))
			}
		} else if strings.HasSuffix(r.URL.Path, "/preload") {
			switch r.Method {
			case "GET":
				json.NewEncoder(w).Encode(preloads)
			case "PUT":
				preloads[r.URL.Query().Get("src")] = map[string]interface{}{}
			case "DELETE":
				delete(preloads, r.URL.Query().Get("src"))
			}
		}
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(u.Port())
	cfg := &config.Config{Go2RTC: config.Go2RTCConfig{Enabled: true, URL: upstream.URL, APIHostPort: port, APIPassword: "fixture", Streams: []config.Go2RTCStreamConfig{{ID: "one", Enabled: true, Source: "rtsp://192.168.1.2/main", Preload: true}}}}
	m := NewGo2RTCManager(cfg, nil, nil, nil)
	defer m.Close()
	for range 2 {
		if _, err := m.ReconcileStreams(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	_, staleAlias := aliases[old]
	_, stalePreload := preloads[old]
	warm := len(preloads)
	count := patches
	mu.Unlock()
	if staleAlias || stalePreload || warm != 1 || count != 1 {
		t.Fatalf("restart reconciliation: stale=%v/%v warm=%d patches=%d", staleAlias, stalePreload, warm, count)
	}
	m.Close()
	mu.Lock()
	warm = len(preloads)
	mu.Unlock()
	if warm != 0 {
		t.Fatal("manager shutdown retained warm connections")
	}
}
