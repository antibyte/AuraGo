package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestGo2RTCRevokedSnapshotCannotCacheOrPublish(t *testing.T) {
	started, reply := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/frame.jpeg") {
			close(started)
			<-reply
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(go2RTCTestJPEG(20, 10))
			return
		}
		w.WriteHeader(200)
	}))
	defer upstream.Close()
	defer close(reply)
	u, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(u.Port())
	cfg := &config.Config{Go2RTC: config.Go2RTCConfig{Enabled: true, URL: upstream.URL, APIHostPort: port, APIPassword: "fixture", Streams: []config.Go2RTCStreamConfig{{ID: "one", Enabled: true, Source: "rtsp://192.168.1.2/old"}, {ID: "two", Enabled: true, Source: "rtsp://192.168.1.2/two"}}}}
	cfg.Directories.DataDir = t.TempDir()
	m := NewGo2RTCManager(cfg, nil, nil, nil)
	defer m.Close()
	if _, err := m.ReconcileStreams(context.Background()); err != nil {
		t.Fatal(err)
	}
	other, err := m.AcquireMedia(context.Background(), "two", "main", false)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Release()
	finished := make(chan error, 1)
	go func() {
		_, _, err := m.Snapshot(context.Background(), "one", Go2RTCSnapshotOptions{CacheSeconds: 30, Store: true})
		finished <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("snapshot did not start")
	}
	cfg.Go2RTC.Streams[0].Source = "rtsp://192.168.1.2/new"
	m.Configure(cfg)
	if _, err := m.ReconcileStreams(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("revoked snapshot returned success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("source revision did not abort HTTP retrieval")
	}
	m.mu.RLock()
	cached := len(m.cache)
	m.mu.RUnlock()
	if cached != 0 {
		t.Fatal("revoked JPEG entered cache")
	}
	if _, err := os.Stat(filepath.Join(cfg.Directories.DataDir, "go2rtc", "snapshots")); !os.IsNotExist(err) {
		t.Fatal("revoked JPEG published a snapshot directory", err)
	}
	if other.Context.Err() != nil {
		t.Fatal("snapshot revocation stopped another camera")
	}
}
