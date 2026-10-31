package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/kaggle"
	"aurago/internal/notebooks"
	"aurago/internal/tools"
)

func TestKaggleNativeSchemasBoundedAndFeatureGated(t *testing.T) {
	for _, flags := range []ToolFeatureFlags{{}, {KaggleEnabled: true}} {
		found := 0
		for _, tool := range BuildNativeToolSchemaSnapshot(t.TempDir(), nil, flags, nil).StrictSchemas() {
			if !strings.HasPrefix(tool.Function.Name, "kaggle_") {
				continue
			}
			found++
			b, err := json.Marshal(tool.Function.Parameters)
			if err != nil || len(b) > 200000 {
				t.Fatalf("unbounded schema %s", tool.Function.Name)
			}
		}
		want := 0
		if flags.KaggleEnabled {
			want = 7
		}
		if found != want {
			t.Fatalf("got %d tools, want %d", found, want)
		}
	}
	if (ToolFeatureFlags{}).Key() == (ToolFeatureFlags{KaggleEnabled: true}).Key() {
		t.Fatal("schema cache key ignores Kaggle")
	}
}

func TestKaggleDispatchSharesGrantsAndIsolation(t *testing.T) {
	cfg := &config.Config{}
	cfg.Kaggle = config.DefaultKaggleConfig()
	cfg.Kaggle.Enabled = true
	cfg.Kaggle.Token = "fixture-denied"
	cfg.Directories.DataDir = t.TempDir()
	cfg.Agent.AllowNetworkRequests = true
	cfg.Agent.AllowFilesystemWrite = true
	useRuntimePermissionsForTest(t, cfg)
	s, err := kaggle.NewService(context.Background(), cfg.Directories.DataDir, func() kaggle.Snapshot {
		return kaggle.Snapshot{Config: cfg.Kaggle, Access: kaggle.Access{Network: true, Write: true}}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	unregister := tools.RegisterKaggleBackend(cfg.Directories.DataDir, &tools.KaggleBackend{Platform: s, Documents: notebooks.NewDocuments(s)})
	defer unregister()
	dc := &DispatchContext{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for _, fixture := range []struct {
		tool, operation string
		mutating        bool
	}{{"kaggle_datasets", "list_datasets", false}, {"kaggle_notebooks", "save_kernel", true}, {"kaggle_competitions", "create_submission", true}, {"kaggle_runtime", "status", false}} {
		tc := ToolCall{Action: fixture.tool, Operation: fixture.operation, Parameters: map[string]interface{}{}}
		if got := DescribeAction(tc, dc); got.Mutating != fixture.mutating {
			t.Fatalf("classification %s: %+v", fixture.tool, got)
		}
		if !fixture.mutating {
			continue
		}
		output, handled := dispatchCloud(context.Background(), tc, dc)
		if !handled || !strings.Contains(output, "read-only") || !strings.Contains(output, "external_data") {
			t.Fatalf("dispatch policy or isolation: %s", output)
		}
	}
	cfg.Kaggle.ReadOnly = false
	cfg.Kaggle.AllowCreate = true
	cfg.Agent.AllowFilesystemWrite = false
	out := tools.RunKaggle(context.Background(), cfg, "runtime", json.RawMessage(`{"operation":"create_project","title":"denied"}`))
	if !strings.Contains(out, "filesystem writes are disabled") {
		t.Fatal("run snapshot bypass", out)
	}
}
