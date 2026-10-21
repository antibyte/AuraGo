package server

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func TestLooperBusyStatusDoesNotGrantMaintenanceIdentity(t *testing.T) {
	previousPath, previousBusy := tools.GetBusyFilePath(), tools.IsBusy()
	tools.SetBusyFilePath("")
	tools.SetBusy(true)
	t.Cleanup(func() { tools.SetBusy(previousBusy); tools.SetBusyFilePath(previousPath) })
	cfg := &config.Config{}
	cfg.LLM.Model = "fixture-model"
	cfg.Directories.ToolsDir = t.TempDir()
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), LLMClient: llm.WrapOpenAIClient(openai.NewClient("fixture-unused-local-client"))}
	_, _, _, dc, _, err := buildLooperRuntime(s, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if dc.IsMaintenance || dc.MessageSource != "looper" {
		t.Fatalf("global busy state changed Looper identity: maintenance=%v source=%q", dc.IsMaintenance, dc.MessageSource)
	}
}
