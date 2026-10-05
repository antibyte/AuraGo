package llm

import (
	"net/http"
	"testing"
	"time"
)

func TestProbeHTTPClientUsesSameOriginPolicy(t *testing.T) {
	client := newProbeHTTPClient(5 * time.Second)
	first, _ := http.NewRequest(http.MethodGet, "http://ollama.lan:11434/api/tags", nil)
	cross, _ := http.NewRequest(http.MethodGet, "http://other.lan:11434/api/tags", nil)
	if err := client.CheckRedirect(cross, []*http.Request{first}); err == nil {
		t.Fatal("probe clients must not follow cross-origin redirects")
	}
	if client.Timeout != 5*time.Second {
		t.Fatalf("timeout = %v, want 5s", client.Timeout)
	}
}
