package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
)

func TestSetupProviderPublicTransportBlocksPrivateTargets(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "")
	var calls atomic.Int32
	backend := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/tags" {
			fmt.Fprint(w, `{"models":[{"name":"test-model"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"test-model"}]}`)
	}))
	defer backend.Close()
	for _, typ := range []string{"custom", "openai", "ollama"} {
		t.Run(typ, func(t *testing.T) {
			calls.Store(0)
			p := config.ProviderEntry{ID: "setup-preview", Type: typ, BaseURL: backend.URL + "/v1", Model: "test-model"}
			result := checkSetupProvider(context.Background(), nil, &config.Config{}, p)
			if typ == "ollama" {
				if !result.OK || calls.Load() != 1 {
					t.Fatalf("local Ollama = %+v, calls=%d", result, calls.Load())
				}
			} else if result.OK || calls.Load() != 0 {
				t.Fatalf("public setup contacted private target: result=%+v calls=%d", result, calls.Load())
			}
		})
	}
}
