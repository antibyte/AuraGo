package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPublicHTTPClientRejectsPrivateDespiteDevelopmentOverride(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	hits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer target.Close()
	client := NewPublicHTTPClient(time.Second)
	if response, err := client.Get(target.URL); err == nil {
		response.Body.Close()
		t.Error("private download allowed")
	}
	if hits != 0 {
		t.Fatalf("private requests=%d", hits)
	}
}
