package render

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// probeServer is a loopback server on its own port that records every request it receives.
type probeServer struct {
	*httptest.Server
	mu   sync.Mutex
	hits []string
}

func newProbeServer(t *testing.T) *probeServer {
	t.Helper()
	p := &probeServer{}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.hits = append(p.hits, r.Method+" "+r.URL.Path)
		p.mu.Unlock()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write([]byte("/* probe */"))
	}))
	t.Cleanup(p.Close)
	return p
}

func (p *probeServer) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.hits)
}

// pageServer serves one page below a 32-character prefix: the markup of the render page (an external
// script, as the CSP demands) and that script at <base>/js/storymaker/host.bundle.js. It hands over
// what the page POSTs to <base>/result; every other path below the prefix answers with a harmless
// script.
type pageServer struct {
	*httptest.Server
	prefix  string
	results chan string
}

// newPageServer starts the server. csp, when not nil, computes the Content-Security-Policy of the
// page from the render base.
func newPageServer(t *testing.T, script string, csp func(base string) string) *pageServer {
	t.Helper()
	ps := &pageServer{prefix: strings.Repeat("a1", 16), results: make(chan string, 16)}
	ps.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root := "/" + ps.prefix + "/"
		if !strings.HasPrefix(r.URL.Path, root) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-SM-Frame")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		switch strings.TrimPrefix(r.URL.Path, root) {
		case "":
			if csp != nil {
				w.Header().Set("Content-Security-Policy", csp("http://"+r.Host+"/"+ps.prefix))
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(RenderPage("http://"+r.Host+"/"+ps.prefix, 320, 180)))
		case "js/storymaker/host.bundle.js":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write([]byte(script))
		case "result":
			body := make([]byte, 8192)
			n, _ := r.Body.Read(body)
			select {
			case ps.results <- string(body[:n]):
			default:
			}
		case "boot.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write([]byte("/* ok */"))
		}
	}))
	t.Cleanup(ps.Close)
	return ps
}

func (ps *pageServer) base() string { return ps.URL + "/" + ps.prefix }

func (ps *pageServer) wait(t *testing.T, d time.Duration) string {
	t.Helper()
	select {
	case r := <-ps.results:
		return r
	case <-time.After(d):
		t.Fatal("the page reported nothing")
		return ""
	}
}
