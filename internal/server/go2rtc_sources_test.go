package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestGo2RTCOptionalSourcesVaultAndRollback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/go2rtc/proxy/api" {
			w.Write([]byte(`{"version":"1.9.14"}`))
			return
		}
		w.WriteHeader(200)
	}))
	defer upstream.Close()
	s, _, vault := newGo2RTCMutationTestServer(t, upstream.URL, true)
	changes := []go2RTCSourceChange{{ID: "door", Role: "main", Value: "rtsp://192.168.1.2/main"}, {ID: "door", Role: "sub", Value: "rtsp://user:fixture@192.168.1.2/sub"}, {ID: "door", Role: "snapshot", Value: "http://192.168.1.2/image.jpg"}}
	_, err := updateManagedGo2RTCConfig(context.Background(), s, func(c *config.Go2RTCConfig) error {
		c.Streams = []config.Go2RTCStreamConfig{{ID: "door", Enabled: true, PreviewSource: "sub"}}
		return nil
	}, changes, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"main", "sub", "snapshot"} {
		if _, err := vault.ReadSecret(config.Go2RTCSourceKey("door", role)); err != nil {
			t.Fatal(err)
		}
	}
	encoded, _ := json.Marshal(safeGo2RTCStream(s.Go2RTC.Config(), "door"))
	if strings.Contains(string(encoded), "192.168") || strings.Contains(string(encoded), "fixture") {
		t.Fatal("source leaked")
	}
	_, err = updateManagedGo2RTCConfig(context.Background(), s, func(c *config.Go2RTCConfig) error { c.Streams[0].PlaybackMode = "invalid"; return nil }, []go2RTCSourceChange{{ID: "door", Role: "sub", Delete: true}}, false)
	if err == nil {
		t.Fatal("invalid publication accepted")
	}
	if _, err := vault.ReadSecret(config.Go2RTCSourceKey("door", "sub")); err != nil {
		t.Fatal("rejected publication lost source")
	}
}

func TestGo2RTCOptionalSourceMasksAndClears(t *testing.T) {
	items := []interface{}{map[string]interface{}{"id": "door", "source": "••••••••", "sub_source": "", "clear_snapshot_source": true}}
	changes, err := go2RTCSourceChanges(items, true)
	if err != nil || len(changes) != 1 || changes[0].Role != "snapshot" || !changes[0].Delete {
		t.Fatalf("changes=%+v %v", changes, err)
	}
	if _, exists := items[0].(map[string]interface{})["source"]; exists {
		t.Fatal("source remains in YAML candidate")
	}
}
