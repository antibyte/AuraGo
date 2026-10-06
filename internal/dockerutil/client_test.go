package dockerutil

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPClientWithTimeoutKeepsTransportAndOverridesTimeout(t *testing.T) {
	client := NewClient("", 30*time.Second)
	streaming := client.HTTPClientWithTimeout(30 * time.Minute)
	if streaming == nil {
		t.Fatal("streaming client is nil")
	}
	if streaming.Timeout != 30*time.Minute {
		t.Fatalf("streaming timeout = %s, want 30m", streaming.Timeout)
	}
	if streaming.Transport != client.HTTPClient().Transport {
		t.Fatal("streaming client must reuse the Docker transport")
	}
}

// DoJSON reports status 0 when no HTTP response arrived and the real status
// with an error for a non-2xx answer; callers classify Engine answers by code.
func TestDoJSONStatusCodeContract(t *testing.T) {
	versionJSON := `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`
	t.Run("unreachable engine returns status 0", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		host := "tcp://" + strings.TrimPrefix(server.URL, "http://")
		server.Close()
		client := NewClient(host, time.Second)
		defer client.CloseIdleConnections()
		code, err := client.DoJSON(context.Background(), http.MethodDelete, "containers/x?force=true", nil, nil)
		if code != 0 || err == nil {
			t.Fatalf("DoJSON() = (%d, %v), want (0, error)", code, err)
		}
	})
	t.Run("refused negotiation returns status 0", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/version" {
				t.Errorf("request %s reached the Engine although negotiation failed", r.URL.Path)
			}
			http.NotFound(w, r)
		}))
		defer server.Close()
		client := NewClient("tcp://"+strings.TrimPrefix(server.URL, "http://"), time.Second)
		defer client.CloseIdleConnections()
		code, err := client.DoJSON(context.Background(), http.MethodDelete, "containers/x?force=true", nil, nil)
		if code != 0 || err == nil {
			t.Fatalf("DoJSON() = (%d, %v), want (0, error)", code, err)
		}
	})
	t.Run("non-2xx answer returns the real status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/version" {
				fmt.Fprint(w, versionJSON)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"No such container: x"}`)
		}))
		defer server.Close()
		client := NewClient("tcp://"+strings.TrimPrefix(server.URL, "http://"), time.Second)
		defer client.CloseIdleConnections()
		code, err := client.DoJSON(context.Background(), http.MethodDelete, "containers/x?force=true", nil, nil)
		if code != http.StatusNotFound || err == nil || !strings.Contains(err.Error(), "No such container") {
			t.Fatalf("DoJSON() = (%d, %v), want (404, error naming the Engine message)", code, err)
		}
	})
	t.Run("2xx answer returns its status without error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/version" {
				fmt.Fprint(w, versionJSON)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		client := NewClient("tcp://"+strings.TrimPrefix(server.URL, "http://"), time.Second)
		defer client.CloseIdleConnections()
		code, err := client.DoJSON(context.Background(), http.MethodDelete, "containers/x?force=true", nil, nil)
		if code != http.StatusNoContent || err != nil {
			t.Fatalf("DoJSON() = (%d, %v), want (204, nil)", code, err)
		}
	})
}

func TestDoJSONErrorTextIsOneBoundedLine(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		// Pins: printable Engine bodies keep today's exact text.
		{"engine JSON body", `{"message":"No such container: x"}` + "\n", `Docker API returned 404: {"message":"No such container: x"}`},
		{"long ASCII body cut at 512 bytes", strings.Repeat("x", 600), "Docker API returned 404: " + strings.Repeat("x", 512)},
		{"empty body", "", "Docker API returned 404: "},
		{"non-ASCII printable body", "Container läuft nicht\n", "Docker API returned 404: Container läuft nicht"},
		// New: one printable line, never a split rune.
		{"line breaks become spaces", "<html>\n403 Forbidden\n</html>", "Docker API returned 404: <html> 403 Forbidden </html>"},
		{"rune at the cut is not split", strings.Repeat("x", 511) + "é", "Docker API returned 404: " + strings.Repeat("x", 511)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/version" {
					fmt.Fprint(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
					return
				}
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client := NewClient("tcp://"+strings.TrimPrefix(server.URL, "http://"), time.Second)
			defer client.CloseIdleConnections()
			code, err := client.DoJSON(context.Background(), http.MethodGet, "containers/x/json", nil, nil)
			if code != http.StatusNotFound || err == nil || err.Error() != tc.want {
				t.Fatalf("DoJSON() = (%d, %v), want (404, %q)", code, err, tc.want)
			}
		})
	}
}
