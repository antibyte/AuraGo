package tools

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDockerBodyMessageExtractsEngineError(t *testing.T) {
	got := dockerBodyMessage(500, []byte(`{"message":"driver failed programming external connectivity"}`))
	if got != "driver failed programming external connectivity" {
		t.Fatalf("dockerBodyMessage = %q", got)
	}
}

func TestBuildImageWaitUsesDockerAPIBuildEndpoint(t *testing.T) {
	var sawBuild bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+dockerAPIVersion+"/build" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Query().Get("t"); got != "aurago/code-studio:latest" {
			t.Fatalf("tag query = %q", got)
		}
		if got := r.URL.Query().Get("dockerfile"); got != "Dockerfile" {
			t.Fatalf("dockerfile query = %q", got)
		}
		var buildArgs map[string]string
		if err := json.Unmarshal([]byte(r.URL.Query().Get("buildargs")), &buildArgs); err != nil {
			t.Fatalf("parse buildargs: %v", err)
		}
		if buildArgs["TARGETARCH"] != "amd64" {
			t.Fatalf("TARGETARCH = %q", buildArgs["TARGETARCH"])
		}
		tr := tar.NewReader(r.Body)
		header, err := tr.Next()
		if err != nil {
			t.Fatalf("read tar header: %v", err)
		}
		if header.Name != "Dockerfile" {
			t.Fatalf("tar entry = %q, want Dockerfile", header.Name)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read dockerfile body: %v", err)
		}
		if !strings.Contains(string(body), "FROM alpine") {
			t.Fatalf("dockerfile body = %q", string(body))
		}
		sawBuild = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"stream":"ok"}` + "\n"))
	}))
	defer server.Close()

	host := "tcp://" + strings.TrimPrefix(server.URL, "http://")
	if err := BuildImageWait(context.Background(), DockerConfig{Host: host}, "aurago/code-studio:latest", "Dockerfile", []byte("FROM alpine\n"), map[string]string{"TARGETARCH": "amd64"}, nil); err != nil {
		t.Fatalf("BuildImageWait returned error: %v", err)
	}
	if !sawBuild {
		t.Fatal("fake Docker build endpoint was not called")
	}
}

func TestBuildImageWaitReturnsDockerBuildStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errorDetail":{"message":"apt failed"},"error":"apt failed"}` + "\n"))
	}))
	defer server.Close()

	host := "tcp://" + strings.TrimPrefix(server.URL, "http://")
	err := BuildImageWait(context.Background(), DockerConfig{Host: host}, "aurago/code-studio:latest", "Dockerfile", []byte("FROM alpine\n"), nil, nil)
	if err == nil {
		t.Fatal("expected Docker build stream error")
	}
	if !strings.Contains(err.Error(), "apt failed") {
		t.Fatalf("error = %v, want Docker stream message", err)
	}
}
