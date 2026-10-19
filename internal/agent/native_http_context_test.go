package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestNativeHTTPDispatchStopIsSessionScoped(t *testing.T) {
	for _, route := range []string{"result", "raw"} {
		t.Run(route, func(t *testing.T) {
			entered := make(chan struct{}, 2)
			releaseB := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				entered <- struct{}{}
				if strings.Contains(r.URL.Path, "/2/") {
					select {
					case <-releaseB:
						_, _ = io.WriteString(w, `{"id":2,"title":"session-b-success","content":"fixture"}`)
					case <-r.Context().Done():
					}
					return
				}
				<-r.Context().Done()
			}))
			defer server.Close()
			cfg := &config.Config{}
			cfg.PaperlessNGX.Enabled, cfg.PaperlessNGX.ReadOnly = true, true
			cfg.PaperlessNGX.URL, cfg.PaperlessNGX.APIToken = server.URL, "fixture-token"
			cfg.Agent.ActionPolicy.Enabled = true
			cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: "paperless_ngx", Operation: "*", Target: "*"}}
			dcA, dcB := newBuiltinHandlerTestDispatchContext(cfg), newBuiltinHandlerTestDispatchContext(cfg)
			dcA.SessionID, dcB.SessionID = t.Name()+"/a", t.Name()+"/b"
			// The call must not replace a context stored in shared dependencies.
			original, originalCancel := context.WithCancel(context.Background())
			originalCancel()
			dcA.Ctx, dcB.Ctx = original, original
			ctxA, cancelA := context.WithCancel(context.Background())
			defer cancelA()
			ctxB, cancelB := context.WithCancel(context.Background())
			defer cancelB()
			run := func(ctx context.Context, id string, dc *DispatchContext, done chan<- string) {
				call := ToolCall{Action: "paperless_ngx", Params: map[string]interface{}{"operation": "get", "document_id": id}}
				if route == "raw" {
					done <- dispatchInner(ctx, call, dc)
					return
				}
				done <- DispatchToolCallResult(ctx, &call, dc, "fixture read").Output
			}
			a, b := make(chan string, 1), make(chan string, 1)
			go run(ctxA, "1", dcA, a)
			go run(ctxB, "2", dcB, b)
			for i := 0; i < 2; i++ {
				select {
				case <-entered:
				case <-time.After(3 * time.Second):
					t.Fatal("dispatch did not reach the local HTTP fixture")
				}
			}
			cancelA()
			select {
			case out := <-a:
				if !strings.Contains(out, "context canceled") {
					t.Fatalf("Stop result = %s", out)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Stop did not reach native HTTP")
			}
			select {
			case out := <-b:
				t.Fatalf("session A canceled B: %s", out)
			case <-time.After(50 * time.Millisecond):
			}
			close(releaseB)
			select {
			case out := <-b:
				if !strings.Contains(out, "session-b-success") {
					t.Fatalf("session B result = %s", out)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("session B did not complete")
			}
			if dcA.Ctx != original || dcB.Ctx != original {
				t.Fatal("dispatch mutated the shared context")
			}
		})
	}
}

func TestNativeHTTPDispatchNormalizesNilContextLocally(t *testing.T) {
	cfg := &config.Config{}
	cfg.PaperlessNGX.Enabled = true
	dc := newBuiltinHandlerTestDispatchContext(cfg)
	dc.ExecutionHooks = &ExecutionHooks{HandleTool: func(ctx context.Context, _ ToolCall) (string, bool) {
		if ctx == nil {
			t.Fatal("nil dispatch context")
		}
		local, _ := ctx.Value(projectRunKey{}).(*DispatchContext)
		if local == nil || local == dc || local.Ctx == nil {
			t.Fatal("missing per-call dispatch copy")
		}
		return `{"status":"success"}`, true
	}}
	call := ToolCall{Action: "paperless_ngx", Params: map[string]interface{}{"operation": "search"}}
	if out := dispatchInner(nil, call, dc); !strings.Contains(out, "success") {
		t.Fatal(out)
	}
	if dc.Ctx != nil {
		t.Fatal("nil owner was written to the shared dispatch context")
	}
}
