package tools

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNetlifyEnvironmentMetadataNeverIncludesValues(t *testing.T) {
	oldURL, oldClient := netlifyBaseURL, netlifyHTTPClient
	t.Cleanup(func() { netlifyBaseURL = oldURL; netlifyHTTPClient = oldClient })
	code := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		fmt.Fprint(w, `{"key":"APP_KEY","scopes":["runtime"],"value":"top-fixture-secret","unknown":{"value":"nested-fixture-secret"},"values":[{"value":"fixture-env-secret","context":"production"}]}`)
	}))
	defer server.Close()
	netlifyBaseURL = server.URL
	netlifyHTTPClient = server.Client()
	for _, status := range []int{200, 500} {
		code = status
		result := NetlifyGetEnvVar(NetlifyConfig{Token: "fixture", TeamSlug: "team", ReadOnly: true}, "site", "APP_KEY")
		if strings.Contains(result, "fixture-secret") || strings.Contains(result, "fixture-env-secret") || strings.Contains(result, `"value"`) {
			t.Fatal("environment value leaked")
		}
		if status == 200 && (!strings.Contains(result, `"redacted":true`) || !strings.Contains(result, `"values_count":1`)) {
			t.Fatal(result)
		}
	}
}

func TestHomepageVercelGatesBeforeAnyBuildOrIO(t *testing.T) {
	for _, cfg := range []VercelConfig{{ReadOnly: true, AllowDeploy: true}, {AllowDeploy: false}} {
		result := HomepageDeployVercel(HomepageConfig{}, cfg, "site", "", "", "production", "", "", true, true, slogDiscard())
		if !strings.Contains(result, "read-only") && !strings.Contains(result, "allow_deploy") {
			t.Fatal(result)
		}
	}
}
func TestHomepageVercelProductionRequiresExplicitTarget(t *testing.T) {
	for _, target := range []string{"", "preview", "staging"} {
		cmd := buildVercelDeployCommand("site", "", normalizeVercelDeployTarget(target), VercelConfig{})
		if strings.Contains(cmd, "production") || !strings.Contains(cmd, "preview") {
			t.Fatal(cmd)
		}
	}
	if normalizeVercelDeployTarget("production") != "production" {
		t.Fatal("explicit production lost")
	}
}
