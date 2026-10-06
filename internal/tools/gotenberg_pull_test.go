package tools

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPullDockerImageKeepsTagSplit(t *testing.T) {
	var rawQuery atomic.Value
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		rawQuery.Store(r.URL.RawQuery)
		_, _ = io.WriteString(w, pullStreamSuccess)
	})
	cfg := DockerConfig{Host: host}
	for image, want := range map[string]string{
		"ollama/ollama": "fromImage=ollama%2Follama&tag=latest",
		gotenbergImage:  "fromImage=gotenberg%2Fgotenberg&tag=8",
	} {
		if err := pullDockerImage(cfg, image); err != nil {
			t.Fatalf("pullDockerImage(%q) = %v", image, err)
		}
		if got, _ := rawQuery.Load().(string); got != want {
			t.Fatalf("pullDockerImage(%q) query = %q, want %q", image, got, want)
		}
	}
}

func TestPullDockerImageReadsTheWholeStream(t *testing.T) {
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		line := `{"status":"Downloading","progressDetail":{"current":1,"total":2},"id":"0123456789ab"}` + "\n"
		chunk := strings.Repeat(line, (1<<20)/len(line)+1)
		for written := 0; written < 11<<20; written += len(chunk) {
			_, _ = io.WriteString(w, chunk)
		}
		_, _ = io.WriteString(w, pullStreamFailure)
	})
	err := pullDockerImage(DockerConfig{Host: host}, gotenbergImage)
	if err == nil || !strings.Contains(err.Error(), "no space left on device") {
		t.Fatalf("pullDockerImage() = %v, want the error event after 11 MiB of progress", err)
	}
}

func TestPullDockerImageKeepsStatusText(t *testing.T) {
	var pulls atomic.Int32
	host := pullOnlyDockerHost(t, http.StatusNotFound, `{"message":"manifest unknown"}`, &pulls)
	if err := pullDockerImage(DockerConfig{Host: host}, gotenbergImage); err == nil || err.Error() != "Docker returned HTTP 404 during image pull: manifest unknown" {
		t.Fatalf("pullDockerImage() = %v", err)
	}
	empty := pullOnlyDockerHost(t, http.StatusNotFound, "", &pulls)
	if err := pullDockerImage(DockerConfig{Host: empty}, gotenbergImage); err == nil || err.Error() != "Docker returned HTTP 404 during image pull" {
		t.Fatalf("pullDockerImage() = %v", err)
	}
}
