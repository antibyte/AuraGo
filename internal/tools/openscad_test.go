package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteOpenSCADRenderHonorsDesktopReadOnly(t *testing.T) {
	cfg := testVirtualDesktopConfig(t)
	cfg.VirtualDesktop.OpenSCAD.Enabled = true
	cfg.VirtualDesktop.ReadOnly = true
	result := ExecuteOpenSCADRender(context.Background(), cfg, map[string]interface{}{
		"source_scad": "cube(1);",
		"exports":     []string{"png"},
	})
	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(result.Output), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, result.Output)
	}
	if payload.Status != "error" || !strings.Contains(payload.Message, "virtual desktop is read-only") {
		t.Fatalf("status=%q message=%q, want read-only refusal", payload.Status, payload.Message)
	}
	jobsRoot := filepath.Join(cfg.Directories.DataDir, "openscad", "jobs")
	if _, err := os.Stat(jobsRoot); !os.IsNotExist(err) {
		t.Fatalf("jobs directory %s exists (err=%v), want no job directory", jobsRoot, err)
	}
}
