package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"syscall"
	"testing"

	"aurago/internal/config"
)

type ollamaPullTestTransport func(*http.Request) (*http.Response, error)

func (f ollamaPullTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOllamaPullRetriesOnlyProvenDialFailures(t *testing.T) {
	for _, test := range []struct {
		name     string
		failure  error
		state    string
		attempts int32
	}{
		{"dial", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}, "failed", 2},
		{"read", &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}, "unknown", 1},
		{"opaque", io.ErrUnexpectedEOF, "unknown", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := newOllamaRuntimeForTest(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"status":"success"}`) }))
			defer server.Close()
			previous := ollamaPullHTTPClient
			var attempts atomic.Int32
			ollamaPullHTTPClient = &http.Client{Transport: ollamaPullTestTransport(func(r *http.Request) (*http.Response, error) {
				if attempts.Add(1) == 1 {
					return nil, test.failure
				}
				return http.DefaultTransport.RoundTrip(r)
			})}
			defer func() { ollamaPullHTTPClient = previous }()
			var cfg config.Config
			cfg.Ollama.Enabled = true
			cfg.Ollama.URL = server.URL
			ConfigureOllamaRuntime(context.Background(), &cfg)
			var first, second struct{ Operation OllamaOperation }
			if err := json.Unmarshal([]byte(StartOllamaToolPull(OllamaConfig{URL: server.URL}, "fixture")), &first); err != nil {
				t.Fatal(err)
			}
			w.wg.Wait()
			if operations := OllamaOperations(""); len(operations) != 1 || operations[0].State != test.state {
				t.Fatal(operations)
			}
			if err := json.Unmarshal([]byte(StartOllamaToolPull(OllamaConfig{URL: server.URL}, "fixture")), &second); err != nil {
				t.Fatal(err)
			}
			w.wg.Wait()
			if attempts.Load() != test.attempts {
				t.Fatalf("attempts=%d, want %d", attempts.Load(), test.attempts)
			}
			if (first.Operation.ID == second.Operation.ID) != (test.state == "unknown") {
				t.Fatal(first, second)
			}
			if test.state == "failed" {
				for _, op := range OllamaOperations("") {
					if op.ID == second.Operation.ID && op.State != "ready" {
						t.Fatal(op)
					}
				}
			}
		})
	}
}
