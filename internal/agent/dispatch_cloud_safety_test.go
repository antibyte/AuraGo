package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchCloudBlocksAgentVercelDeleteProject(t *testing.T) {
	cfg := &config.Config{}
	cfg.Vercel.Enabled = true
	cfg.Vercel.AllowProjectManagement = true

	out, ok := dispatchCloud(context.Background(), ToolCall{
		Action:    "vercel",
		Operation: "delete_project",
		Params: map[string]interface{}{
			"project_id": "prj_danger",
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchCloud to handle vercel")
	}
	if !strings.Contains(out, "not available to the autonomous agent") {
		t.Fatalf("expected delete_project to be hard-blocked, got:\n%s", out)
	}
	if strings.Contains(out, "Vercel token not found") {
		t.Fatalf("delete_project should be blocked before token lookup, got:\n%s", out)
	}
}
