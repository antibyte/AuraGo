package rocketchat

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

type maintenanceFileReadClient struct {
	sessionTestClient
	toolOutput string
	filePath   string
}

func (c *maintenanceFileReadClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.requests = append(c.requests, req.Messages)
	message := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Read complete"}
	finish := openai.FinishReasonStop
	if len(c.requests) == 1 {
		arguments, err := json.Marshal(map[string]string{"operation": "read_file", "file_path": c.filePath})
		if err != nil {
			return openai.ChatCompletionResponse{}, err
		}
		message.Content = ""
		message.ToolCalls = []openai.ToolCall{{ID: "read-fixture", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{
			Name: "filesystem", Arguments: string(arguments),
		}}}
		finish = openai.FinishReasonToolCalls
	} else {
		for _, entry := range req.Messages {
			if entry.Role == openai.ChatMessageRoleTool {
				c.toolOutput = entry.Content
			}
		}
	}
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: finish}}}, nil
}

func TestRocketChatBusyStatusDoesNotGrantMaintenanceIdentity(t *testing.T) {
	previousPath, previousBusy := tools.GetBusyFilePath(), tools.IsBusy()
	tools.SetBusyFilePath("")
	tools.SetBusy(true)
	t.Cleanup(func() { tools.SetBusy(previousBusy); tools.SetBusyFilePath(previousPath) })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	chat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer chat.Close()
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.Agent.ContextWindow = 32768
	cfg.Agent.SystemLanguage = "English"
	cfg.Agent.ActionPolicy.Enabled = true
	cfg.Directories.PromptsDir, err = filepath.Abs("../../prompts")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.DataDir = t.TempDir()
	cfg.RocketChat.Enabled = true
	cfg.RocketChat.URL = chat.URL
	cfg.RocketChat.AllowedUsers = []string{"id:tester"}
	if err := os.WriteFile(filepath.Join(cfg.Directories.WorkspaceDir, "fixture.txt"), []byte("interactive-file-evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	client := &maintenanceFileReadClient{filePath: filepath.Join(cfg.Directories.WorkspaceDir, "fixture.txt")}
	msg := message{ID: "busy-read", Msg: "Read fixture.txt"}
	msg.User.ID, msg.User.Username = "tester", "tester"
	processMessage(context.Background(), cfg, logger, client, store, nil, nil, tools.NewProcessRegistry(logger), nil, nil, nil, nil, "room", msg, nil, nil, nil, nil, nil)
	if len(client.requests) != 2 || !strings.Contains(client.toolOutput, "interactive-file-evidence") || strings.Contains(client.toolOutput, "maintenance_scope_denied") {
		t.Fatalf("interactive read was reclassified as maintenance: requests=%d output=%q", len(client.requests), client.toolOutput)
	}
}
