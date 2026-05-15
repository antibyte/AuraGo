package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchCloudBlocksVercelDeleteProjectWhenManagementDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Vercel.Enabled = true

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
	if !strings.Contains(out, "Vercel project management is not allowed") {
		t.Fatalf("expected delete_project to be blocked by config, got:\n%s", out)
	}
	if strings.Contains(out, "Vercel token not found") {
		t.Fatalf("delete_project should be blocked by config before token lookup, got:\n%s", out)
	}
}

func TestDispatchCloudBlocksNetlifyDeleteSiteWhenManagementDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Netlify.Enabled = true

	out, ok := dispatchCloud(context.Background(), ToolCall{
		Action:    "netlify",
		Operation: "delete_site",
		Params: map[string]interface{}{
			"site_id": "site-danger",
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchCloud to handle netlify")
	}
	if !strings.Contains(out, "Netlify site management is not allowed") {
		t.Fatalf("expected delete_site to be blocked by config, got:\n%s", out)
	}
	if strings.Contains(out, "Netlify token not found") {
		t.Fatalf("delete_site should be blocked by config before token lookup, got:\n%s", out)
	}
}
