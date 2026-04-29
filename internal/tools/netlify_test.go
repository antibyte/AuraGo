package tools

import (
	"aurago/internal/testutil"
	"net/http"
	"strings"
	"testing"
)

func TestNetlifyDeleteSiteAcceptsHTTP200And204(t *testing.T) {
	for _, statusCode := range []int{http.StatusOK, http.StatusNoContent} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			result := NetlifyDeleteSite(NetlifyConfig{Token: "test-token", AllowSiteManagement: true}, "site-123")
			if !strings.Contains(result, `"status":"ok"`) {
				t.Fatalf("expected success result, got %s", result)
			}
		})
	}
}

func TestNetlifyRequestRejectsOversizedResponseBody(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestNetlifyDirectMutationsRespectReadOnly(t *testing.T) {
	cfg := NetlifyConfig{
		Token:               "token",
		DefaultSiteID:       "site-123",
		ReadOnly:            true,
		AllowDeploy:         true,
		AllowSiteManagement: true,
		AllowEnvManagement:  true,
	}

	tests := map[string]string{
		"create site":   NetlifyCreateSite(cfg, "site", ""),
		"update site":   NetlifyUpdateSite(cfg, "site-123", "site", ""),
		"delete site":   NetlifyDeleteSite(cfg, "site-123"),
		"deploy zip":    NetlifyDeployZip(cfg, "site-123", "title", false, []byte("zip")),
		"rollback":      NetlifyRollback(cfg, "site-123", "deploy-123"),
		"cancel deploy": NetlifyCancelDeploy(cfg, "deploy-123"),
		"set env":       NetlifySetEnvVar(cfg, "site-123", "KEY", "value", ""),
		"delete env":    NetlifyDeleteEnvVar(cfg, "site-123", "KEY"),
		"create hook":   NetlifyCreateHook(cfg, "site-123", "slack", "deploy_created", nil),
		"delete hook":   NetlifyDeleteHook(cfg, "hook-123"),
		"provision ssl": NetlifyProvisionSSL(cfg, "site-123"),
	}

	for name, got := range tests {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(got, `"status":"error"`) || !strings.Contains(strings.ToLower(got), "read-only") {
				t.Fatalf("expected read-only error, got %s", got)
			}
		})
	}
}

func TestNetlifyDirectMutationsRequireGranularAllows(t *testing.T) {
	cfg := NetlifyConfig{Token: "token", DefaultSiteID: "site-123"}

	tests := map[string]string{
		"site management": NetlifyCreateSite(cfg, "site", ""),
		"deploy":          NetlifyRollback(cfg, "site-123", "deploy-123"),
		"env management":  NetlifySetEnvVar(cfg, "site-123", "KEY", "value", ""),
	}

	for name, got := range tests {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(got, `"status":"error"`) || !strings.Contains(strings.ToLower(got), "not allowed") {
				t.Fatalf("expected permission error, got %s", got)
			}
		})
	}
}
