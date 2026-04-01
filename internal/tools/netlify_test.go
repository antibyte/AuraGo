package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNetlifyDeleteSiteAcceptsHTTP200And204(t *testing.T) {
	for _, statusCode := range []int{http.StatusOK, http.StatusNoContent} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Fatalf("method = %s, want DELETE", r.Method)
				}
				if got := r.URL.Path; got != "/api/v1/sites/site-123" {
					t.Fatalf("path = %s, want /api/v1/sites/site-123", got)
				}
				w.WriteHeader(statusCode)
			}))
			defer server.Close()

			prevBaseURL := netlifyBaseURL
			prevClient := netlifyHTTPClient
			netlifyBaseURL = server.URL + "/api/v1"
			netlifyHTTPClient = server.Client()
			defer func() {
				netlifyBaseURL = prevBaseURL
				netlifyHTTPClient = prevClient
			}()

			result := NetlifyDeleteSite(NetlifyConfig{Token: "test-token"}, "site-123")
			if !strings.Contains(result, `"status":"ok"`) {
				t.Fatalf("expected success result, got %s", result)
			}
		})
	}
}

func TestNetlifyRequestRejectsOversizedResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("n", int(maxHTTPResponseSize)+1)))
	}))
	defer server.Close()

	prevBaseURL := netlifyBaseURL
	prevClient := netlifyHTTPClient
	netlifyBaseURL = server.URL
	netlifyHTTPClient = server.Client()
	defer func() {
		netlifyBaseURL = prevBaseURL
		netlifyHTTPClient = prevClient
	}()

	_, _, err := netlifyRequest(NetlifyConfig{Token: "token"}, http.MethodGet, "/sites", nil)
	if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}
