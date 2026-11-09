package tools

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepageDocumentIntegrity(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		valid         bool
	}{
		{"complete", "<html><head><title>Site</title></head><body><article>ok</article></body></html>", true},
		{"fragment", "<h1>Static fragment</h1>", true},
		{"comment after html", "<html></html><!-- built today -->", true},
		{"tag text in script", "<html><script>const x = '<article>';</script></html>", true},
		{"nested articles", "<html><article><article>comment</article></article></html>", true},
		{"missing html end", "<html><article>ok</article>", false},
		{"missing article end", "<html><article>cut</html>", false},
		{"self-closing article", "<html><article/></html>", false},
		{"self-closing html", "<html/>", false},
		{"unmatched article", "<html></article></html>", false},
		{"truncation marker", "<html>... [output truncated due to size limit]</html>", false},
		{"trailing content", "<html></html>unfinished sentence", false},
		{"empty", "  ", false},
		{"invalid utf8", "<html>\xff</html>", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := homepageDocumentIntegrity([]byte(tc.content))
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}

func TestHomepageVerifyDeploymentRejectsFalsePositives(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "true")
	for _, tc := range []struct {
		name, content, expected string
		redirect                bool
	}{
		{name: "truncated", content: "<html><article>cut off"},
		{name: "login", content: "<html><title>Log in to Vercel</title></html>"},
		{name: "login redirect", content: "<html><title>Log in to Vercel</title></html>", redirect: true},
		{name: "stale content", content: "<html>old</html>", expected: homepageDocumentHash([]byte("<html>new</html>"))},
		{name: "over limit", content: strings.Repeat("x", maxHomepageWriteFileSize+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.redirect && r.URL.Path != "/login" {
					http.Redirect(w, r, "/login", http.StatusFound)
					return
				}
				_, _ = io.WriteString(w, tc.content)
			}))
			defer server.Close()
			if got := homepageVerifyDeploymentExpected(server.URL, tc.expected); got.Status != "error" {
				t.Fatalf("invalid deployment verified: %+v", got)
			}
		})
	}
}

func TestHomepageVerifyDeploymentMatchesFullStaticArtifact(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "true")
	content := "<html><title>Site</title>" + strings.Repeat("x", 600*1024) + "</html>"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, content) }))
	defer server.Close()
	hash := homepageDocumentHash([]byte(content))
	if got := homepageVerifyDeploymentExpected(server.URL, hash); got.Status != "ok" || got.ContentSHA256 != hash {
		t.Fatalf("complete matching document rejected: %+v", got)
	}
}

func TestHomepageDeployCandidateRejectsIncompleteDocument(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html><article>incomplete"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := homepageDetectDeployCandidate(HomepageConfig{WorkspacePath: dir}, ".", ".", ""); err == nil {
		t.Fatal("incomplete source must fail before deployment")
	}
}
