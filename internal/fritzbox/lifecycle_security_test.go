package fritzbox

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func fritzConfigForEndpoint(t *testing.T, endpoint string) config.Config {
	t.Helper()
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.Host = u.Hostname()
	cfg.FritzBox.Port = port
	cfg.FritzBox.WebPort = port
	cfg.FritzBox.Timeout = 10
	cfg.FritzBox.Telephony.Enabled = true
	cfg.FritzBox.Telephony.SubFeatures.CallLists = true
	return cfg
}

func TestDigestNonDigestResponseKeepsReadableBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="fixture"`)
		w.WriteHeader(401)
		fmt.Fprint(w, "access denied")
	}))
	defer srv.Close()
	client := &http.Client{Transport: NewDigestTransport("fixture", "fixture", nil)}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "access denied" {
		t.Fatalf("body=%q error=%v", body, err)
	}
}

func TestFritzSIDNetworkErrorsAreRedacted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer srv.Close()
	const sid = "abcdef9876543210"
	c := newAHAClient(srv.URL, "fixture", "fixture", time.Second, false)
	c.sid.sid = sid
	c.sid.expiresAt = time.Now().Add(time.Hour)
	if _, err := c.Command("", "getswitchlist", nil); err == nil || strings.Contains(err.Error(), sid) {
		t.Fatalf("AHA error exposes SID or missing error: %v", err)
	}
	if _, err := c.sid.GetWithSID(srv.URL + "/download.lua"); err == nil || strings.Contains(err.Error(), sid) {
		t.Fatalf("download error exposes SID or missing error: %v", err)
	}
}

func TestFritzPollerStopCancelsRequestAndWaitsForClient(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer srv.Close()
	p := NewPoller(fritzConfigForEndpoint(t, srv.URL), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	p.Start()
	defer p.Stop()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request not started")
	}
	done := make(chan struct{})
	go func() { p.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop failed to drain active request")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("request not cancelled")
	}
	if p.pooledClient != nil {
		t.Fatal("pooled client retained after Stop")
	}
	p.Start()
	if p.Context().Err() == nil {
		t.Fatal("stopped poller restarted")
	}
}

func TestFritzClientRejectsForeignDownloadAndRedirect(t *testing.T) {
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("foreign target received request") }))
	defer foreign.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("logout") == "1" {
			return
		}
		http.Redirect(w, r, foreign.URL, 302)
	}))
	defer origin.Close()
	c, err := NewClient(fritzConfigForEndpoint(t, origin.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.sid.sid = "aabbccddeeff0011"
	c.sid.expiresAt = time.Now().Add(time.Hour)
	for _, target := range []string{foreign.URL, origin.URL} {
		if _, err := c.sid.GetWithSID(target); err == nil {
			t.Fatal("foreign SID request accepted")
		}
	}
}

func TestFritzClientContextCancelsResponseBody(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, err := NewClientContext(ctx, fritzConfigForEndpoint(t, srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	req, _ := http.NewRequest("GET", srv.URL, nil)
	resp, err := c.tr.httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	<-started
	cancel()
	if _, err := io.ReadAll(resp.Body); err == nil {
		t.Fatal("response body survived cancellation")
	}
}
