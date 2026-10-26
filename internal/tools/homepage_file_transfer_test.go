package tools

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHomepageWriteDockerFileReportsUploadFailureWithoutRetry(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	uploads := 0
	cfg := newArchiveTestEngine(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || !strings.HasSuffix(r.URL.Path, "/archive") {
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL)
		}
		uploads++
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"message":"disk full"}`)
	})
	oldExec := homepageDockerExecFunc
	t.Cleanup(func() { homepageDockerExecFunc = oldExec })
	homepageDockerExecFunc = func(DockerConfig, string, string, string) string {
		return `{"status":"ok","exit_code":0}`
	}
	err := homepageWriteDockerFile(cfg, "news/index.html", "<html></html>")
	if err == nil || !strings.Contains(err.Error(), "HTTP 500: disk full") || uploads != 1 {
		t.Fatalf("upload failure = %v, uploads=%d; want one failed attempt", err, uploads)
	}
}
