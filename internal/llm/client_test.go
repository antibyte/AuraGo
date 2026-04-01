package llm

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestAIGatewayAuthTransportAddsHeader(t *testing.T) {
	transport := &aiGatewayAuthTransport{
		token: "test-token",
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("cf-aig-authorization"); got != "Bearer test-token" {
				t.Fatalf("cf-aig-authorization = %q, want %q", got, "Bearer test-token")
			}
			if got := req.Header.Get("Authorization"); got != "Bearer provider-key" {
				t.Fatalf("Authorization = %q, want %q", got, "Bearer provider-key")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	req, err := http.NewRequest(http.MethodPost, "https://example.invalid/v1/chat/completions", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer provider-key")

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
}
