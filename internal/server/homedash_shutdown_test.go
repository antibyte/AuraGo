package server

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

// homeDashStreamVia opens GET stream on the given base URL and returns its events after hello.
func homeDashStreamVia(t *testing.T, base, token string) <-chan hdEvent {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+homeDashAPIPrefix+"stream", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("stream = %v %v", resp, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	events := make(chan hdEvent, 64)
	go func() {
		defer close(events)
		sc := bufio.NewScanner(resp.Body)
		var ev hdEvent
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if ev.name != "" {
					events <- ev
				}
				ev = hdEvent{}
			case strings.HasPrefix(line, "event: "):
				ev.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	waitHomeDashEvent(t, events, "hello")
	return events
}

func TestHomeDashStreamRevokedOnShutdown(t *testing.T) {
	fake := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor))
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))
	mux := http.NewServeMux()
	mux.HandleFunc(homeDashAPIPrefix, handleHomeDash(env.s))
	srv := httptest.NewServer(env.s.trackHTTP(mux))
	defer srv.Close()
	events := homeDashStreamVia(t, srv.URL, env.admin)
	sourcetest.WaitFor(t, time.Second, func() bool { return env.svc.streams.Load() == 1 }, "stream counted")

	env.s.endHomeDashStreams(2 * time.Second) // what serveWithShutdown does first
	env.s.beginHTTPDrain()                    // then the drain cancels request contexts
	if ev := waitHomeDashEvent(t, events, "revoked"); ev.data != `{"reason":"shutdown"}` {
		t.Fatalf("revoked = %s", ev.data)
	}
	if n := env.svc.streams.Load(); n != 0 {
		t.Fatalf("%d streams still open", n)
	}
}

func TestServeWithShutdownEndsHomeDashStreamsBeforeTheDrain(t *testing.T) {
	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	start := strings.Index(body, "func (s *Server) serveWithShutdown(")
	if start < 0 {
		t.Fatal("serveWithShutdown not found")
	}
	fn := body[start:]
	if end := strings.Index(fn[1:], "\nfunc "); end > 0 {
		fn = fn[:end+1]
	}
	i, j := strings.Index(fn, "s.endHomeDashStreams("), strings.Index(fn, "s.beginHTTPDrain()")
	if i < 0 || j < 0 || i > j {
		t.Fatal("serveWithShutdown must call endHomeDashStreams before beginHTTPDrain (decision D7 gap a)")
	}
}

func TestEndHomeDashStreamsIsSafeWithoutServiceAndTwice(t *testing.T) {
	var nilServer *Server
	nilServer.endHomeDashStreams(time.Millisecond)
	(&Server{}).endHomeDashStreams(time.Millisecond) // HomeDash never initialised

	fake := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor))
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))
	env.s.endHomeDashStreams(time.Second)
	env.s.endHomeDashStreams(time.Second)
}
