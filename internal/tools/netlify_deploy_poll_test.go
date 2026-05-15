package tools

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNetlifyWaitForDeployReturnsReadyDeploy(t *testing.T) {
	oldBase := netlifyBaseURL
	defer func() { netlifyBaseURL = oldBase }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/deploys/dpl_ready" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"id":"dpl_ready","state":"ready","url":"https://example.netlify.app","deploy_url":"https://deploy-preview.netlify.app"}`)
	}))
	defer server.Close()
	netlifyBaseURL = server.URL

	result := NetlifyWaitForDeploy(NetlifyConfig{Token: "token"}, "dpl_ready", 2, time.Millisecond)
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected ready deploy success, got %s", result)
	}
	if !strings.Contains(result, `"state":"ready"`) {
		t.Fatalf("expected ready state, got %s", result)
	}
}

func TestNetlifyWaitForDeployReturnsProviderError(t *testing.T) {
	oldBase := netlifyBaseURL
	defer func() { netlifyBaseURL = oldBase }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":"dpl_error","state":"error","error_message":"Build asset processing failed"}`)
	}))
	defer server.Close()
	netlifyBaseURL = server.URL

	result := NetlifyWaitForDeploy(NetlifyConfig{Token: "token"}, "dpl_error", 2, time.Millisecond)
	if !strings.Contains(result, `"status":"error"`) {
		t.Fatalf("expected deploy error, got %s", result)
	}
	if !strings.Contains(result, "Build asset processing failed") {
		t.Fatalf("expected provider error message, got %s", result)
	}
}
