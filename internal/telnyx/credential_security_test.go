package telnyx

import (
	"aurago/internal/security"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelnyxRegistersKeyAndScrubsProviderError(t *testing.T) {
	secret := "fixture-telnyx-private-key"
	client := NewClient(secret, nil)
	if security.Scrub(secret) == secret {
		t.Fatal("unregistered key before client use")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		fmt.Fprintf(w, `{"errors":[{"code":"bad","title":"echo","detail":%q}]}`, secret)
	}))
	defer server.Close()
	client.baseURL = server.URL
	_, _, err := client.do(context.Background(), http.MethodGet, "/balance", nil)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe error=%v", err)
	}
}
