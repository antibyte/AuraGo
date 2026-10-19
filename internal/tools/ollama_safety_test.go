package tools

import (
	"net/http"
	"strings"
	"testing"
)

func TestOllamaPullRequiresVerifiedCompletion(t *testing.T) {
	for _, body := range []string{``, `{`, `{"error":"download interrupted"}`, `{"status":"downloading"}`, `{"status":"success"} trailing`} {
		if validateOllamaPullResult([]byte(body)) == nil {
			t.Fatalf("accepted %q", body)
		}
	}
	if err := validateOllamaPullResult([]byte(`{"status":"success"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestOllamaImagePullRefusesReadOnlyBeforeDocker(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	if err := ensureOllamaImage(DockerConfig{Host: "tcp://127.0.0.1:1"}, "ollama/ollama:test"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal(err)
	}
}

func TestOllamaRemoveStopsOnFailedStop(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.Contains(r.URL.Path, "/stop") {
			t.Errorf("unexpected mutation %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(500)
	})
	if err := removeOllamaContainerBestEffort(DockerConfig{Host: host}, "test"); err == nil {
		t.Fatal("failed stop was ignored")
	}
}
