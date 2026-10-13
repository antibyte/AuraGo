package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/memory"
	"aurago/internal/meshcore"
	"aurago/internal/security"
)

type ingressDecisions struct {
	fakeDecisions
	cancel context.CancelFunc
}

func (f *ingressDecisions) Evaluate(ctx context.Context, request decisions.Request) (decisions.Response, error) {
	response, err := f.fakeDecisions.Evaluate(ctx, request)
	f.cancel() // Stop before the ordinary chat model, after observing classifier input.
	return response, err
}

func routerIngressFixture(t *testing.T, cancel context.CancelFunc) (*Server, *ingressDecisions) {
	t.Helper()
	s := newTestDesktopChatServer(t)
	s.Cfg.LLM.Provider = "main"
	s.Cfg.LLM.Model = "gpt-4o"
	s.Cfg.Providers = []config.ProviderEntry{
		{ID: "main", Type: "openai", Model: "gpt-4o"},
		{ID: "code", Type: "openai", Model: "gpt-4o-mini", APIKey: "test-fixture"},
	}
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
	s.Cfg.LLMRouter.HelperFallback = false
	s.Cfg.LLMRouter.Areas["coding"] = config.LLMRouterTarget{Provider: "code"}
	s.Cfg.Decisions = config.DecisionsConfig{Enabled: true, Backend: "provider", Provider: "cloud", Model: "test-decision-model"}
	fake := &ingressDecisions{fakeDecisions: fakeDecisions{status: decisions.Status{Enabled: true, Ready: true, Configured: true}}, cancel: cancel}
	s.Decisions = fake
	return s, fake
}

func TestLLMRouterIngressPrivacy(t *testing.T) {
	// Admission quota is installation-global. Isolate these two physical calls
	// from other tests without exposing a production limiter-reset API.
	const childEnv = "AURAGO_TEST_ROUTER_INGRESS_CHILD"
	if os.Getenv(childEnv) != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLLMRouterIngressPrivacy$", "-test.v")
		cmd.Env = append(os.Environ(), childEnv+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("isolated ingress regressions: %v\n%s", err, output)
		}
		return
	}
	for _, channel := range []string{"agodesk", "meshcore"} {
		t.Run(channel, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s, fake := routerIngressFixture(t, cancel)
			const original = "Mach das passend"
			const private = "PRIVATE_INGRESS_SENTINEL"
			var err error
			if channel == "agodesk" {
				path := filepath.Join(s.Cfg.Directories.WorkspaceDir, "attachments", "note.txt")
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(private), 0600); err != nil {
					t.Fatal(err)
				}
				input := buildAgodeskMessageWithAttachments(s, original+"\nLocal handoff transcript: "+private, []memory.AgoDeskAttachmentRecord{{AttachmentID: "test", RelativePath: "attachments/note.txt", Filename: "note.txt", MimeType: "text/plain"}})
				if strings.Count(input, private) != 2 {
					t.Fatal("fixture must include both transcript and file text")
				}
				r := httptest.NewRequest("POST", "/api/agodesk/ws", nil).WithContext(contextWithAgodeskPersistedMessage(ctx, original))
				_, err = runAgodeskAgentChat(s, r, nil, nil, "request", "transport", "routing-agodesk", "device", input, false)
			} else {
				msg := meshcore.Message{ID: "routing-test", Kind: "direct", Text: original, Sender: "peer", Receiver: &meshcore.ReceiverInfo{Name: private}}
				_, err = s.runMeshCoreMessage(ctx, msg, "trusted")
			}
			if !errors.Is(err, context.Canceled) || fake.calls != 1 {
				t.Fatalf("classifier boundary: calls=%d err=%v", fake.calls, err)
			}
			state, _ := fake.request.State.(string)
			if !strings.Contains(state, security.IsolateExternalData(original)) || strings.Contains(state, private) || strings.Contains(state, "agodesk_attachments") || strings.Contains(state, "MeshCore reception metadata") {
				t.Fatal("classifier received enriched input instead of isolated human text")
			}
			if len(fake.request.Images) != 0 || len(fake.request.Questions) != 1 {
				t.Fatal("classifier input contract changed")
			}
		})
	}
}

func TestLLMRouterDesktopEmptyOriginalStaysEmpty(t *testing.T) {
	s, fake := routerIngressFixture(t, func() {})
	original := ""
	enriched := "Implement a Go function from the uploaded attachment"
	turn, err := prepareDesktopAgentTurnWithOptions(context.Background(), s, enriched, desktopChatContext{}, false, desktopAgentTurnOptions{SessionID: "routing-empty", MessageSource: agodeskMessageSource, TaskRoutingText: &original})
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls != 0 || turn.runCfg.TaskRoutingText == nil || *turn.runCfg.TaskRoutingText != "" || turn.runCfg.TaskRouting.Reason != "uncertain" || turn.req.Model != s.Cfg.LLM.Model {
		t.Fatal("attachment-only input selected a route or reached a classifier")
	}
	if len(turn.req.Messages) == 0 || !strings.Contains(turn.req.Messages[len(turn.req.Messages)-1].Content, enriched) {
		t.Fatal("ordinary chat lost its attachment context")
	}
}
