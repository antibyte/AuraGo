package server

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestDashboardTemplatePartialsAreListedAndPacked: the ui asset namespace has no glob, so the dashboard
// template is parsed from an explicit partial list. The list must equal the files on disk in sidebar order,
// the set must parse and execute, and the release asset pack must ship the "dashboard" directory
// (cmd/assetpack silently drops unlisted top-level ui directories).
func TestDashboardTemplatePartialsAreListedAndPacked(t *testing.T) {
	t.Parallel()

	ui := os.DirFS(filepath.Join("..", "..", "ui"))
	order := []string{"overview", "personality", "memory", "user", "knowledge", "filesync", "problems",
		"automations", "system", "audit", "logs", "llm", "prompts", "tools"}
	want := make([]string, 0, len(order))
	for _, id := range order {
		want = append(want, "dashboard/sections/"+id+".html")
	}
	if !slices.Equal(dashboardSectionPartials, want) {
		t.Fatalf("dashboardSectionPartials = %v, want %v", dashboardSectionPartials, want)
	}
	onDisk, err := fs.Glob(ui, "dashboard/sections/*.html")
	if err != nil {
		t.Fatal(err)
	}
	sorted := slices.Clone(want)
	slices.Sort(sorted)
	if !slices.Equal(onDisk, sorted) {
		t.Fatalf("ui/dashboard/sections = %v, want exactly %v", onDisk, sorted)
	}

	tmpl, err := parseDashboardTemplate(ui)
	if err != nil {
		t.Fatalf("parse dashboard template: %v", err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, uiTemplateData("en", "dashboard")); err != nil {
		t.Fatalf("execute dashboard template: %v", err)
	}
	if !strings.Contains(out.String(), `data-workspace-page="dashboard"`) {
		t.Fatal("dashboard template rendered without the page body")
	}

	raw, err := os.ReadFile(filepath.Join("..", "..", "assets", "web-assets.json"))
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Roots []struct {
			Source      string   `json:"source"`
			Directories []string `json:"directories"`
		} `json:"roots"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	for _, root := range spec.Roots {
		if root.Source != "ui" {
			continue
		}
		if !slices.Contains(root.Directories, "dashboard") {
			t.Fatalf("assets/web-assets.json ui directories = %v; without \"dashboard\" the release asset pack drops the section partials", root.Directories)
		}
		return
	}
	t.Fatal("assets/web-assets.json has no ui root")
}
