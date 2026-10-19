package tools

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readNetlifyZipEntries(t *testing.T, data []byte) map[string]string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	entries := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			rc.Close()
			t.Fatalf("read %s: %v", file.Name, err)
		}
		rc.Close()
		entries[file.Name] = buf.String()
	}
	return entries
}

func writeNetlifyDeployFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

func TestBuildNetlifyZipSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	deploy := filepath.Join(root, "dist")
	if err := os.MkdirAll(deploy, 0o755); err != nil {
		t.Fatalf("mkdir deploy: %v", err)
	}
	writeNetlifyDeployFiles(t, deploy, map[string]string{"index.html": "<h1>ok</h1>"})

	secret := filepath.Join(root, "vault.bin")
	if err := os.WriteFile(secret, []byte("SECRET"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.Symlink(secret, filepath.Join(deploy, "asset.txt")); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}

	data, stats, err := buildNetlifyZip(HomepageConfig{}, deploy, slogDiscard())
	if err != nil {
		t.Fatalf("build zip: %v", err)
	}
	entries := readNetlifyZipEntries(t, data)
	if _, published := entries["asset.txt"]; published {
		t.Fatal("a symlink must never be read into a public deploy")
	}
	for name, content := range entries {
		if strings.Contains(content, "SECRET") {
			t.Fatalf("%s leaked content from outside the deploy tree", name)
		}
	}
	if stats.Skipped < 1 {
		t.Fatalf("skipped = %d, want at least the symlink", stats.Skipped)
	}
	if entries["index.html"] != "<h1>ok</h1>" {
		t.Fatalf("index.html = %q", entries["index.html"])
	}
}

func TestBuildNetlifyZipSkipsCredentialMaterial(t *testing.T) {
	deploy := t.TempDir()
	writeNetlifyDeployFiles(t, deploy, map[string]string{
		"index.html":    "<h1>ok</h1>",
		"assets/app.js": "console.log(1)",
		"prod.env":      "API_TOKEN=secret",
		"deploy.p12":    "binary",
		"id_ecdsa":      "private key",
		".netrc":        "machine example.test",
		"session.crt":   "certificate",
	})

	data, _, err := buildNetlifyZip(HomepageConfig{}, deploy, slogDiscard())
	if err != nil {
		t.Fatalf("build zip: %v", err)
	}
	entries := readNetlifyZipEntries(t, data)
	for _, name := range []string{"prod.env", "deploy.p12", "id_ecdsa", ".netrc", "session.crt"} {
		if _, published := entries[name]; published {
			t.Fatalf("%s must not be published", name)
		}
	}
	if _, published := entries["assets/app.js"]; !published {
		t.Fatal("regular assets must stay in the deploy")
	}
}

func TestBuildNetlifyZipRejectsTooManyEntries(t *testing.T) {
	previous := netlifyZipMaxEntries
	netlifyZipMaxEntries = 2
	t.Cleanup(func() { netlifyZipMaxEntries = previous })

	deploy := t.TempDir()
	writeNetlifyDeployFiles(t, deploy, map[string]string{
		"index.html": "<h1>ok</h1>",
		"a.js":       "1",
		"b.js":       "2",
	})

	if _, _, err := buildNetlifyZip(HomepageConfig{}, deploy, slogDiscard()); err == nil {
		t.Fatal("expected the entry limit to abort the deploy")
	}
}

func TestBuildNetlifyZipRejectsOversizedOutput(t *testing.T) {
	previous := netlifyZipMaxBytes
	netlifyZipMaxBytes = 32
	t.Cleanup(func() { netlifyZipMaxBytes = previous })

	deploy := t.TempDir()
	writeNetlifyDeployFiles(t, deploy, map[string]string{
		"index.html": strings.Repeat("a", 64),
	})

	if _, _, err := buildNetlifyZip(HomepageConfig{}, deploy, slogDiscard()); err == nil {
		t.Fatal("expected the byte budget to abort the deploy")
	}
}

func TestBuildNetlifyZipAddsSPAFallbackOnlyForSinglePageSites(t *testing.T) {
	single := t.TempDir()
	writeNetlifyDeployFiles(t, single, map[string]string{
		"index.html": "<div id=app></div>",
		"app.js":     "console.log(1)",
	})
	data, stats, err := buildNetlifyZip(HomepageConfig{}, single, slogDiscard())
	if err != nil {
		t.Fatalf("build single-page zip: %v", err)
	}
	if !stats.SPAFallback {
		t.Fatal("a single-page site needs the index.html rewrite")
	}
	if entries := readNetlifyZipEntries(t, data); !strings.Contains(entries["_redirects"], "index.html") {
		t.Fatalf("_redirects = %q", entries["_redirects"])
	}

	multi := t.TempDir()
	writeNetlifyDeployFiles(t, multi, map[string]string{
		"index.html": "<a href=/about.html>about</a>",
		"about.html": "<h1>about</h1>",
	})
	data, stats, err = buildNetlifyZip(HomepageConfig{}, multi, slogDiscard())
	if err != nil {
		t.Fatalf("build multi-page zip: %v", err)
	}
	if stats.SPAFallback {
		t.Fatal("a multi-page site must keep Netlify's real 404")
	}
	if _, published := readNetlifyZipEntries(t, data)["_redirects"]; published {
		t.Fatal("a multi-page site must not receive a catch-all rewrite")
	}
}

func TestBuildNetlifyZipKeepsOneMimeSource(t *testing.T) {
	deploy := t.TempDir()
	writeNetlifyDeployFiles(t, deploy, map[string]string{"index.html": "<h1>ok</h1>"})

	data, _, err := buildNetlifyZip(HomepageConfig{}, deploy, slogDiscard())
	if err != nil {
		t.Fatalf("build zip: %v", err)
	}
	entries := readNetlifyZipEntries(t, data)
	if _, ok := entries["_headers"]; !ok {
		t.Fatal("_headers is the MIME source and must be present")
	}
	if _, ok := entries["netlify.toml"]; ok {
		t.Fatal("a generated netlify.toml would duplicate the MIME rules")
	}
}

// netlifyDeployTestSite serves one deploy cycle and records which URL was verified.
func netlifyDeployTestSite(t *testing.T, deployURLPath, siteURLPath string, deployBody func(siteURL string) string) (*httptest.Server, *[]string) {
	t.Helper()
	var verified []string
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sites/site-123/deploys":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"deploy-123","site_id":"site-123","state":"uploaded"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/deploys/deploy-123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(deployBody(server.URL + siteURLPath)))
		case r.Method == http.MethodGet && r.URL.Path == deployURLPath:
			verified = append(verified, r.URL.Path)
			_, _ = w.Write([]byte(`<html><body>permalink</body></html>`))
		case r.Method == http.MethodGet && r.URL.Path == siteURLPath:
			verified = append(verified, r.URL.Path)
			_, _ = w.Write([]byte(`<html><body>site</body></html>`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return server, &verified
}

func prepareNetlifyDeployTest(t *testing.T) {
	t.Helper()
	previousAttempts := netlifyDeployPollAttempts
	previousInterval := netlifyDeployPollInterval
	netlifyDeployPollAttempts = 1
	netlifyDeployPollInterval = 0
	t.Cleanup(func() {
		netlifyDeployPollAttempts = previousAttempts
		netlifyDeployPollInterval = previousInterval
	})
}

func netlifyDeployTestProject(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	writeNetlifyDeployFiles(t, filepath.Join(workspace, "site-a"), map[string]string{
		"index.html": "<h1>Site</h1>",
	})
	return workspace
}

func TestHomepageDeployNetlifyKeepsDeployPermalink(t *testing.T) {
	prepareNetlifyDeployTest(t)
	server, verified := netlifyDeployTestSite(t, "/permalink", "/live", func(siteURL string) string {
		return `{"id":"deploy-123","state":"ready","url":"` + siteURL + `","deploy_url":"` + strings.Replace(siteURL, "/live", "/permalink", 1) + `"}`
	})
	useNetlifyTestBase(t, server.URL)
	workspace := netlifyDeployTestProject(t)

	result := HomepageDeployNetlify(
		HomepageConfig{WorkspacePath: workspace},
		NetlifyConfig{Token: "token", DefaultSiteID: "site-123", AllowDeploy: true},
		"site-a", ".", "", "", false, slogDiscard(),
	)
	parsed := decodeNetlifyJSON(t, result)
	if parsed["status"] == "error" {
		t.Fatalf("expected a successful deploy, got %s", result)
	}
	if parsed["deploy_url"] != server.URL+"/permalink" {
		t.Fatalf("deploy_url = %v, want the deploy permalink (the site URL must not overwrite it)", parsed["deploy_url"])
	}
	if parsed["site_url"] != server.URL+"/live" {
		t.Fatalf("site_url = %v, want the published site URL", parsed["site_url"])
	}
	if parsed["verified"] != true || parsed["published"] != true {
		t.Fatalf("expected a verified published deploy, got %s", result)
	}
	if len(*verified) != 1 || (*verified)[0] != "/permalink" {
		t.Fatalf("verified %v, want the deploy permalink only", *verified)
	}
}

func TestHomepageDeployNetlifyDraftNeverClaimsPublication(t *testing.T) {
	prepareNetlifyDeployTest(t)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sites/site-123/deploys":
			if r.URL.Query().Get("draft") != "true" {
				t.Fatalf("expected a draft deploy, got query %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"deploy-123","site_id":"site-123","state":"uploaded"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/deploys/deploy-123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"deploy-123","state":"ready","url":"` + server.URL + `/live","deploy_url":"` + server.URL + `/draft-permalink"}`))
		default:
			// The draft permalink is not publicly readable, like a login-gated preview.
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)
	workspace := netlifyDeployTestProject(t)

	result := HomepageDeployNetlify(
		HomepageConfig{WorkspacePath: workspace},
		NetlifyConfig{Token: "token", DefaultSiteID: "site-123", AllowDeploy: true},
		"site-a", ".", "", "", true, slogDiscard(),
	)
	parsed := decodeNetlifyJSON(t, result)
	if parsed["status"] == "error" {
		t.Fatalf("a draft deploy must not fail on an unreachable permalink: %s", result)
	}
	if parsed["published"] != false {
		t.Fatalf("published = %v, want false for a draft", parsed["published"])
	}
	if parsed["verified"] == true {
		t.Fatalf("a draft deploy must not claim a live verification: %s", result)
	}
	if _, ok := parsed["verification_note"].(string); !ok {
		t.Fatalf("expected a verification note: %s", result)
	}
}
