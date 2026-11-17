package embeddings

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type embeddingErrorTransport func(*http.Request) (*http.Response, error)

func (f embeddingErrorTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLlamaHTTPInputErrorsDoNotPoisonManager(t *testing.T) {
	for _, status := range []int{400, 413, 422, 401, 404, 429, 500, 503} {
		for _, docker := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/docker=%v", status, docker), func(t *testing.T) {
				client := &http.Client{Transport: embeddingErrorTransport(func(r *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"fixture rejection"}}`)), Header: make(http.Header)}, nil
				})}
				var err error
				if docker {
					backend := &dockerLlamaEmbedder{client: client, baseURL: "http://fixture.invalid"}
					_, err = backend.embedLocked(context.Background(), []string{"text"})
				} else {
					backend := &llamaEmbedder{client: client, baseURL: "http://fixture.invalid"}
					_, err = backend.embedLocked(context.Background(), []string{"text"})
				}
				inputError := status == 400 || status == 413 || status == 422
				if err == nil || errors.Is(err, ErrInvalidInput) != inputError {
					t.Fatalf("wrong HTTP classification: %v", err)
				}
				if !inputError {
					return
				}
				ready := make(chan struct{})
				close(ready)
				active := &countingEmbedder{status: Status{Fingerprint: "llama"}, err: err}
				fallback := &countingEmbedder{status: Status{Fingerprint: "onnx"}, vectors: [][]float32{unitTestVector()}}
				manager := &Manager{active: active, fallback: fallback, ready: ready, status: Status{State: "ready"}, logger: discardEmbeddingLogger()}
				if _, err := manager.Embed(context.Background(), []string{"bad"}); !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("manager rejected wrong error: %v", err)
				}
				active.err, active.vectors = nil, [][]float32{unitTestVector()}
				if _, err := manager.Embed(context.Background(), []string{"good"}); err != nil {
					t.Fatalf("next valid request failed: %v", err)
				}
				if manager.failures != 0 || manager.status.RestartRequired || manager.status.State != "ready" || fallback.calls.Load() != 0 {
					t.Fatalf("input error changed backend health: %+v, fallback calls=%d", manager.status, fallback.calls.Load())
				}
			})
		}
	}
}
