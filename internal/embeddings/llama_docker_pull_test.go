package embeddings

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/dockerutil"
)

func llamaPullTestEmbedder(t *testing.T, status int, writeStream func(io.Writer)) *dockerLlamaEmbedder {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := "/" + dockerutil.APIVersion
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, prefix+"/images/") && strings.HasSuffix(r.URL.Path, "/json"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == prefix+"/images/create":
			w.WriteHeader(status)
			writeStream(w)
		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return &dockerLlamaEmbedder{
		docker: newDockerAPIClient("tcp://" + strings.TrimPrefix(server.URL, "http://")),
		image:  "ghcr.io/ggml-org/llama.cpp:server@sha256:" + strings.Repeat("a", 64),
	}
}

const llamaPullError = `{"errorDetail":{"message":"no matching manifest for linux/amd64"},"error":"no matching manifest for linux/amd64"}` + "\n"

func TestDockerLlamaEnsureImageFailsOnStreamErrors(t *testing.T) {
	cases := map[string]struct {
		status int
		stream func(io.Writer)
		check  func(error) bool
	}{
		"error event": {http.StatusOK, func(w io.Writer) { _, _ = io.WriteString(w, llamaPullError) },
			func(err error) bool { return err != nil && strings.Contains(err.Error(), "no matching manifest") }},
		"cut stream": {http.StatusOK, func(w io.Writer) { _, _ = io.WriteString(w, `{"status":"Downloading"}`+"\n"+`{"status":"Downlo`) },
			func(err error) bool { return errors.Is(err, io.ErrUnexpectedEOF) }},
		"error after 16 MiB of progress": {http.StatusOK, func(w io.Writer) {
			line := `{"status":"Downloading","progressDetail":{"current":1,"total":2},"id":"0123456789ab"}` + "\n"
			chunk := strings.Repeat(line, (1<<20)/len(line)+1)
			for written := 0; written < 17<<20; written += len(chunk) {
				_, _ = io.WriteString(w, chunk)
			}
			_, _ = io.WriteString(w, llamaPullError)
		}, func(err error) bool { return err != nil && strings.Contains(err.Error(), "no matching manifest") }},
		"non-2xx": {http.StatusNotFound, func(w io.Writer) { _, _ = io.WriteString(w, `{"message":"manifest unknown"}`) },
			func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "HTTP 404") && strings.Contains(err.Error(), "manifest unknown")
			}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := llamaPullTestEmbedder(t, tc.status, tc.stream).ensureImageLocked(context.Background())
			if !tc.check(err) {
				t.Fatalf("ensureImageLocked() = %v", err)
			}
		})
	}
}

func TestDockerLlamaEnsureImageAcceptsCompletePull(t *testing.T) {
	embedder := llamaPullTestEmbedder(t, http.StatusOK, func(w io.Writer) {
		_, _ = io.WriteString(w, `{"status":"Status: Downloaded newer image"}`+"\n")
	})
	if err := embedder.ensureImageLocked(context.Background()); err != nil {
		t.Fatalf("ensureImageLocked() = %v, want nil", err)
	}
}
