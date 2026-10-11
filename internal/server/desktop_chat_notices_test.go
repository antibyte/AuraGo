package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/llm"
	"aurago/internal/planner"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func TestDesktopCodeStudioDefersOperationalNoticesToNormalChat(t *testing.T) {
	const issueTitle = "Background maintenance probe failed"
	const answer = "This code prints a greeting."
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			requests := make(chan openai.ChatCompletionRequest, 2)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("decode LLM request: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				requests <- req
				if req.Stream {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":%q},\"finish_reason\":null}]}\n\n", answer)
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
					Choices: []openai.ChatCompletionChoice{{
						Message:      openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: answer},
						FinishReason: openai.FinishReasonStop,
					}},
				})
			}))
			t.Cleanup(upstream.Close)

			s := newTestDesktopChatServer(t)
			s.Cfg.Directories.PromptsDir = testPromptsDir(t)
			s.Cfg.Directories.SkillsDir = t.TempDir()
			s.Cfg.Agent.SystemLanguage = "English"
			s.Cfg.Agent.ContextWindow = 32768
			s.Registry = tools.NewProcessRegistry(s.Logger)
			openaiCfg := openai.DefaultConfig("test-key")
			openaiCfg.BaseURL = upstream.URL + "/v1"
			s.LLMClient = llm.WrapOpenAIClient(openai.NewClientWithConfig(openaiCfg))
			db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			s.PlannerDB = db
			if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{
				Source: "maintenance", Title: issueTitle, Severity: "error",
			}); err != nil {
				t.Fatal(err)
			}

			for _, source := range []string{"code-studio", "desktop"} {
				body, err := json.Marshal(map[string]any{
					"message": "Explain the current code.",
					"context": desktopChatContext{Source: source, CurrentFile: "/workspace/hello.py", CurrentContent: `print("Hello from Code Studio")`},
				})
				if err != nil {
					t.Fatal(err)
				}
				path := "/api/desktop/chat"
				handler := handleDesktopChat(s)
				if stream {
					path += "/stream"
					handler = handleDesktopChatStream(s)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)))
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), answer) {
					t.Fatalf("%s response = %d %s", source, rec.Code, rec.Body.String())
				}
				wantNotice := source == "desktop"
				if got := strings.Contains(rec.Body.String(), issueTitle); got != wantNotice {
					t.Errorf("%s response includes operational notice = %t, want %t", source, got, wantNotice)
				}
				select {
				case req := <-requests:
					prompt, err := json.Marshal(req.Messages)
					if err != nil {
						t.Fatal(err)
					}
					if got := strings.Contains(string(prompt), issueTitle); got != wantNotice {
						t.Errorf("%s LLM context includes operational notice = %t, want %t", source, got, wantNotice)
					}
				default:
					t.Fatal("missing LLM request")
				}
				pending, err := planner.ListPendingOperationalIssueNotices(db, time.Now(), 2)
				if err != nil {
					t.Fatal(err)
				}
				wantPending := 1
				if wantNotice {
					wantPending = 0
				}
				if len(pending) != wantPending {
					t.Errorf("%s left %d pending notices, want %d", source, len(pending), wantPending)
				}
			}
		})
	}
}
