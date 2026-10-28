package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
)

func mediaTestManager(t *testing.T, profile bool) (*Go2RTCManager, *config.Config) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/go2rtc/proxy/api/frame.jpeg" {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(go2RTCTestJPEG(80, 40))
			return
		}
		w.WriteHeader(200)
	}))
	t.Cleanup(upstream.Close)
	u, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(u.Port())
	cfg := &config.Config{Go2RTC: config.Go2RTCConfig{Enabled: true, URL: upstream.URL, APIHostPort: port, APIPassword: "fixture"}}
	for _, id := range []string{"one", "two", "three"} {
		s := config.Go2RTCStreamConfig{ID: id, Enabled: true, Source: "rtsp://192.168.1.2/" + id, SubSource: "rtsp://192.168.1.2/sub"}
		if profile {
			s.CodecProfile = "h264_opus"
		}
		cfg.Go2RTC.Streams = append(cfg.Go2RTC.Streams, s)
	}
	m := NewGo2RTCManager(cfg, nil, nil, nil)
	t.Cleanup(m.Close)
	if _, err := m.ReconcileStreams(context.Background()); err != nil {
		t.Fatal(err)
	}
	return m, cfg
}

func TestGo2RTCMediaRevisionCancelsOnlyChangedCamera(t *testing.T) {
	m, cfg := mediaTestManager(t, false)
	one, err := m.AcquireMedia(context.Background(), "one", "main", false)
	if err != nil {
		t.Fatal(err)
	}
	defer one.Release()
	two, err := m.AcquireMedia(context.Background(), "two", "main", false)
	if err != nil {
		t.Fatal(err)
	}
	defer two.Release()
	if err := m.RememberHLS("one", "main", "ABCDEFGH", one.Revision); err != nil {
		t.Fatal(err)
	}
	cfg.Go2RTC.Streams[0].Source = "rtsp://192.168.1.3/replaced"
	m.Configure(cfg)
	if one.Context.Err() != nil {
		t.Fatal("source update interrupted the active revision before registration")
	}
	if _, err := m.ReconcileStreams(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-one.Context.Done():
	case <-time.After(time.Second):
		t.Fatal("old camera context still live")
	}
	if two.Context.Err() != nil {
		t.Fatal("unrelated camera stopped")
	}
	if _, err := m.HLSLease(context.Background(), "ABCDEFGH"); err == nil {
		t.Fatal("old HLS session still usable")
	}
	current, err := m.AcquireMedia(context.Background(), "one", "main", false)
	if err != nil {
		t.Fatal(err)
	}
	defer current.Release()
	if current.Alias == one.Alias || current.Revision == one.Revision {
		t.Fatal("source revision was reused")
	}
	if m.MediaRevisionCurrent("one", one.Revision) {
		t.Fatal("old snapshot revision accepted")
	}
}

func TestGo2RTCFailedSourceRegistrationPreservesActiveRevision(t *testing.T) {
	var fail atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() && r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Query().Get("name"), "~sub") {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(u.Port())
	cfg := &config.Config{Go2RTC: config.Go2RTCConfig{Enabled: true, URL: upstream.URL, APIHostPort: port, APIPassword: "fixture", Streams: []config.Go2RTCStreamConfig{{ID: "one", Enabled: true, Source: "rtsp://192.168.1.2/old", SubSource: "rtsp://192.168.1.2/sub"}}}}
	m := NewGo2RTCManager(cfg, nil, nil, nil)
	defer m.Close()
	if _, err := m.ReconcileStreams(context.Background()); err != nil {
		t.Fatal(err)
	}
	old, err := m.AcquireMedia(context.Background(), "one", "main", false)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Release()
	fail.Store(true)
	cfg.Go2RTC.Streams[0].Source = "rtsp://192.168.1.3/new"
	cfg.Go2RTC.Streams[0].CodecProfile = "h264_opus"
	m.Configure(cfg)
	if _, err := m.ReconcileStreams(context.Background()); err == nil {
		t.Fatal("partial source registration reported success")
	}
	status := m.ConfiguredStreamStatuses()[0]
	if old.Context.Err() != nil || status.Revision != old.Revision || !status.MediaReady || !status.SourcePending || status.Media.CodecProfile == "h264_opus" {
		t.Fatalf("failed replacement changed the active camera: %+v", status)
	}
	active, err := m.AcquireMedia(context.Background(), "one", "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if active.Alias != old.Alias {
		t.Fatal("partial transcoding profile became usable")
	}
	active.Release()
	fail.Store(false)
	if _, err := m.ReconcileStreams(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-old.Context.Done():
	case <-time.After(time.Second):
		t.Fatal("successful replacement did not revoke the old revision")
	}
	if status = m.ConfiguredStreamStatuses()[0]; !status.MediaReady || status.SourcePending || status.Revision == old.Revision || status.Media.CodecProfile != "h264_opus" {
		t.Fatalf("replacement was not published: %+v", status)
	}
}

func TestGo2RTCMediaSharesAndBoundsTranscoders(t *testing.T) {
	m, _ := mediaTestManager(t, true)
	a, err := m.AcquireMedia(context.Background(), "one", "main", true)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Release()
	b, err := m.AcquireMedia(context.Background(), "one", "main", true)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Release()
	c, err := m.AcquireMedia(context.Background(), "two", "main", true)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Release()
	if _, err := m.AcquireMedia(context.Background(), "three", "main", true); err == nil {
		t.Fatal("third distinct transcoder admitted")
	}
	a.Release()
	if _, err := m.AcquireMedia(context.Background(), "three", "main", true); err == nil {
		t.Fatal("shared transcoder released too early")
	}
	b.Release()
	d, err := m.AcquireMedia(context.Background(), "three", "main", true)
	if err != nil {
		t.Fatal(err)
	}
	d.Release()
}

func TestGo2RTCSourceRolesAndTransport(t *testing.T) {
	m, _ := mediaTestManager(t, false)
	l, err := m.AcquireMedia(context.Background(), "one", "preview", false)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	if l.Role != "sub" {
		t.Fatal("preview does not select substream")
	}
	u, _ := url.Parse(go2RTCTransportSource("rtsp://192.168.1.2/live#transport=tcp#media=video", "udp"))
	if u.Fragment != "media=video#transport=udp" {
		t.Fatal(u.Fragment)
	}
}

func TestGo2RTCHLSSessionCannotRebindAnotherCameraOrRole(t *testing.T) {
	m, _ := mediaTestManager(t, false)
	revision := m.StreamRevision("one")
	if err := m.RememberHLS("one", "main", "ABCDEFGH", revision); err != nil {
		t.Fatal(err)
	}
	for _, source := range []struct{ id, role string }{{"two", "main"}, {"one", "sub"}} {
		if err := m.RememberHLS(source.id, source.role, "ABCDEFGH", m.StreamRevision(source.id)); err == nil {
			t.Fatal("HLS session accepted another camera or role", source)
		}
	}
	lease, err := m.HLSLease(context.Background(), "ABCDEFGH")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	if lease.StreamID != "one" || lease.Role != "main" || lease.Revision != revision {
		t.Fatal("HLS collision changed the original binding")
	}
}
