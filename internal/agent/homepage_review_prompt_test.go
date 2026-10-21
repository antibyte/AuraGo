package agent

import (
	"aurago/internal/config"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepageReviewInventoryFitsWithLongReasons(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "sites.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	reason := strings.Repeat("layout ", 280) // A valid purpose below the 2000-byte mutation limit.
	for i := 0; i < 8; i++ {
		project, err := tools.EnsureHomepageProjectForDir(db, tools.HomepageConfig{WorkspacePath: cfg.Homepage.WorkspacePath}, fmt.Sprintf("site-%d", i), "", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := tools.LogEdit(db, project.ID, reason); err != nil {
			t.Fatal(err)
		}
		if _, err := tools.RecordHomepageEvent(db, project.ID, "operation_finished", "test", "Update", map[string]interface{}{"reason": reason}); err != nil {
			t.Fatal(err)
		}
	}
	inventory := buildManagedSitesPromptContext(RunConfig{Config: cfg, HomepageRegistryDB: db})
	result, err := prompts.FitSystemPromptToBudget(context.Background(), prompts.PromptFitRequest{Text: "# IDENTITY\nAgent.", Tokens: -1, Model: "gpt-4o-mini", TokenBudget: 4000, Addenda: []prompts.PromptAddendum{{ID: "managed_websites", Text: security.IsolateExternalData(inventory)}}}, testLogger)
	if err != nil || result.BudgetExceeded != nil {
		t.Fatalf("valid inventory blocked every model request: %v %v; chars=%d", err, result.BudgetExceeded, len(inventory))
	}
	if !strings.Contains(result.Text, "total=8") || !strings.Contains(result.Text, "get_site") {
		t.Fatal("bounded inventory lost full lookup access")
	}
}

func TestHomepageReviewInventoryBoundsEscapedMetadata(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "sites.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := &config.Config{}
	cfg.Homepage.WorkspacePath = t.TempDir()
	name := strings.Repeat("<&", 5000)
	for i := 0; i < 8; i++ {
		project, err := tools.EnsureHomepageProjectForDir(db, tools.HomepageConfig{WorkspacePath: cfg.Homepage.WorkspacePath}, fmt.Sprintf("site-%d", i), "", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := tools.UpdateProject(db, project.ID, map[string]interface{}{"name": fmt.Sprintf("%s%d", name, i)}); err != nil {
			t.Fatal(err)
		}
	}
	inventory := buildManagedSitesPromptContext(RunConfig{Config: cfg, HomepageRegistryDB: db})
	if len(security.IsolateExternalData(inventory)) > 4000 || !strings.Contains(inventory, "total=8") || !strings.Contains(inventory, "get_site") {
		t.Fatal("escaped metadata exceeded the bounded inventory index")
	}
	first, err := tools.GetProjectByDir(db, "site-0")
	if err != nil || first.Name != name+"0" {
		t.Fatal("prompt compaction modified the authoritative registry")
	}
}
