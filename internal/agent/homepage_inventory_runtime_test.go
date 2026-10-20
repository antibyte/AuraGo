package agent

import (
	"aurago/internal/config"
	"aurago/internal/prompts"
	"aurago/internal/tools"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepageInventorySurvivesReusePolicyAndRequiredPromptFitting(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "homepage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	os.MkdirAll(filepath.Join(cfg.Homepage.WorkspacePath, "site-a"), 0755)
	tools.EnsureHomepageProjectForDir(db, tools.HomepageConfig{WorkspacePath: cfg.Homepage.WorkspacePath}, "site-a", "Site A", "html")
	flags := buildPromptContextFlags(RunConfig{Config: cfg, HomepageRegistryDB: db}, buildToolingPolicy(cfg, "homepage"), promptContextOptions{})
	applyRuntimePromptContextPolicy(&flags, runtimePromptContextOptions{UserText: "hello"})
	if !strings.Contains(flags.ManagedSitesContext, "site-a") {
		t.Fatal("homepage inventory lost through reuse policy")
	}
	result, err := prompts.FitSystemPromptToBudget(context.Background(), prompts.PromptFitRequest{Text: "# IDENTITY\nAgent.", Tokens: -1, Model: "gpt-4o-mini", TokenBudget: 4000, Addenda: []prompts.PromptAddendum{{ID: "managed_websites", Text: flags.ManagedSitesContext}}}, testLogger)
	if err != nil || !strings.Contains(result.Text, "site-a") {
		t.Fatalf("inventory missing in rendered prompt: %v %s", err, result.Text)
	}
}
func TestHomepageFileMutationRequiresReasonBeforeWriting(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "hp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	out, _ := dispatchServices(context.Background(), ToolCall{Action: "homepage_file", Operation: "write_file", Path: "site/index.html", Content: "hello"}, &DispatchContext{Cfg: cfg, HomepageRegistryDB: db, Logger: testLogger})
	if !strings.Contains(out, "reason is required") {
		t.Fatal(out)
	}
	if _, err := os.Stat(filepath.Join(cfg.Homepage.WorkspacePath, "site", "index.html")); !os.IsNotExist(err) {
		t.Fatal("missing reason still wrote file")
	}
}

func TestHomepageInventoryReachesPhysicalModelRequest(t *testing.T) {
	runCfg, client, cleanup := newPromptPipelineTestRunConfig(t, "homepage-inventory", "web")
	defer cleanup()
	runCfg.Config.Homepage.Enabled = true
	runCfg.Config.Homepage.WorkspacePath = t.TempDir()
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "sites.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runCfg.HomepageRegistryDB = db
	tools.EnsureHomepageProjectForDir(db, tools.HomepageConfig{WorkspacePath: runCfg.Config.Homepage.WorkspacePath}, "existing-site", "Existing site", "html")
	LoopbackContext(context.Background(), runCfg, "Which websites have you created?", NoopBroker{})
	if client.calls != 1 {
		t.Fatalf("model calls: %d", client.calls)
	}
	if len(client.lastReq.Messages) == 0 || !strings.Contains(client.lastReq.Messages[0].Content, "existing-site") {
		t.Fatal("inventory was lost before the model request")
	}
}

func TestHomepageInventoryHonorsEmptyToolScope(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "sites.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	if _, err := tools.EnsureHomepageProjectForDir(db, tools.HomepageConfig{WorkspacePath: cfg.Homepage.WorkspacePath}, "private-site", "", ""); err != nil {
		t.Fatal(err)
	}
	run := RunConfig{Config: cfg, HomepageRegistryDB: db, AllowedTools: []string{}}
	if got := buildManagedSitesPromptContext(run); got != "" {
		t.Fatal("empty tool scope exposed website inventory")
	}
	run.AllowedTools = []string{"homepage_registry"}
	if got := buildManagedSitesPromptContext(run); !strings.Contains(got, "private-site") {
		t.Fatal("explicit homepage scope lost its inventory")
	}
}
