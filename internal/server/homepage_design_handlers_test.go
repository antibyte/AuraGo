package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// homepageDesignServer builds a server with a registry and a local homepage
// workspace, so a design selection can be applied to a project directory.
func homepageDesignServer(t *testing.T) (*Server, string) {
	t.Helper()
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "homepage.db"))
	if err != nil {
		t.Fatalf("init registry: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	workspace := t.TempDir()
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WorkspacePath = workspace
	cfg.Directories.DataDir = t.TempDir()
	s := &Server{HomepageRegistryDB: db, Cfg: cfg, Logger: slog.Default()}
	return s, workspace
}

func TestHandleHomepageDesignsListsBundledCatalog(t *testing.T) {
	t.Parallel()
	s, _ := homepageDesignServer(t)

	rec := httptest.NewRecorder()
	handleHomepageDesigns(s)(rec, httptest.NewRequest(http.MethodGet, "/api/homepage/designs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Designs []struct {
			ID      string   `json:"id"`
			Name    string   `json:"name"`
			Author  string   `json:"author"`
			Likes   int      `json:"likes"`
			Palette []string `json:"palette"`
		} `json:"designs"`
		SelectedDesign string `json:"selected_design"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Designs) != 25 {
		t.Fatalf("designs = %d, want 25", len(payload.Designs))
	}
	first := payload.Designs[0]
	if first.ID == "" || first.Name == "" || first.Author == "" {
		t.Fatalf("first design is incomplete: %+v", first)
	}
	if first.Likes <= 0 {
		t.Fatalf("first design has no like count: %+v", first)
	}
	if len(first.Palette) == 0 {
		t.Fatalf("first design has no palette: %+v", first)
	}
	if payload.SelectedDesign != "" {
		t.Fatalf("selected_design = %q, want empty without a project", payload.SelectedDesign)
	}
}

func TestHandleHomepageDesignPreviewServesImage(t *testing.T) {
	t.Parallel()
	s, _ := homepageDesignServer(t)

	rec := httptest.NewRecorder()
	handleHomepageDesignByID(s)(rec, httptest.NewRequest(http.MethodGet, "/api/homepage/designs/genesis/preview?variant=light", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", got)
	}
	body, _ := io.ReadAll(rec.Body)
	if len(body) < 1024 {
		t.Fatalf("preview body = %d bytes, want a real image", len(body))
	}
}

func TestHandleHomepageDesignPreviewRejectsUnknownVariant(t *testing.T) {
	t.Parallel()
	s, _ := homepageDesignServer(t)

	rec := httptest.NewRecorder()
	handleHomepageDesignByID(s)(rec, httptest.NewRequest(http.MethodGet, "/api/homepage/designs/genesis/preview?variant=sepia", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleHomepageDesignDetailReturnsTemplateContent(t *testing.T) {
	t.Parallel()
	s, _ := homepageDesignServer(t)

	rec := httptest.NewRecorder()
	handleHomepageDesignByID(s)(rec, httptest.NewRequest(http.MethodGet, "/api/homepage/designs/genesis", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(payload.Content, "## Colors") {
		t.Fatalf("template content looks wrong:\n%s", payload.Content)
	}
}

func TestHandleHomepageDesignSelectWritesProjectDesign(t *testing.T) {
	t.Parallel()
	s, workspace := homepageDesignServer(t)
	if _, _, err := tools.RegisterProject(s.HomepageRegistryDB, tools.HomepageProject{Name: "DesignSite", ProjectDir: "design-site", Framework: "astro"}); err != nil {
		t.Fatalf("register project: %v", err)
	}

	body := strings.NewReader(`{"project_dir":"design-site","design_id":"genesis"}`)
	rec := httptest.NewRecorder()
	handleHomepageDesignSelect(s)(rec, httptest.NewRequest(http.MethodPost, "/api/homepage/designs/select", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Applied    bool `json:"applied"`
		Registered bool `json:"registered"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !payload.Applied || !payload.Registered {
		t.Fatalf("selection was not applied: %+v", payload)
	}

	data, err := os.ReadFile(filepath.Join(workspace, "design-site", "DESIGN.md"))
	if err != nil {
		t.Fatalf("read project DESIGN.md: %v", err)
	}
	if !strings.Contains(string(data), "## Colors") {
		t.Fatalf("project DESIGN.md does not carry the template:\n%s", data)
	}

	stored, err := tools.GetHomepageProjectDesign(s.HomepageRegistryDB, "design-site")
	if err != nil {
		t.Fatalf("read stored design: %v", err)
	}
	if stored != "genesis" {
		t.Fatalf("stored design = %q, want genesis", stored)
	}
}

func TestHandleHomepageDesignSelectJournalsTheChange(t *testing.T) {
	t.Parallel()
	s, _ := homepageDesignServer(t)
	siteID, _, err := tools.RegisterProject(s.HomepageRegistryDB, tools.HomepageProject{Name: "Journal", ProjectDir: "journal-site", Framework: "astro"})
	if err != nil {
		t.Fatalf("register project: %v", err)
	}
	rec := httptest.NewRecorder()
	handleHomepageDesignSelect(s)(rec, httptest.NewRequest(http.MethodPost, "/api/homepage/designs/select", strings.NewReader(`{"project_dir":"journal-site","design_id":"genesis"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Applied    bool `json:"applied"`
		Registered bool `json:"registered"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || !payload.Applied || !payload.Registered {
		t.Fatalf("payload = %+v, err = %v", payload, err)
	}
	detail, err := tools.GetHomepageManagedSite(s.HomepageRegistryDB, siteID)
	if err != nil {
		t.Fatalf("read site: %v", err)
	}
	events, err := tools.ListHomepageSiteEvents(s.HomepageRegistryDB, siteID, 50, 0)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	found := false
	for _, e := range events {
		if e.Actor == "user" && e.Reason == `Selected design system "genesis" in Homepage Studio` && strings.Contains(e.Summary, "apply_design") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no apply_design operation event with the known reason: %+v", events)
	}
	if detail.CurrentRevisionID == 0 {
		t.Fatal("design selection did not record a revision")
	}
}

func TestHandleHomepageDesignSelectRejectsUnknownTemplate(t *testing.T) {
	t.Parallel()
	s, _ := homepageDesignServer(t)

	body := strings.NewReader(`{"project_dir":"design-site","design_id":"does-not-exist"}`)
	rec := httptest.NewRecorder()
	handleHomepageDesignSelect(s)(rec, httptest.NewRequest(http.MethodPost, "/api/homepage/designs/select", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleHomepageDesignSelectWithoutProjectKeepsTemplateValid(t *testing.T) {
	t.Parallel()
	s, _ := homepageDesignServer(t)

	body := strings.NewReader(`{"design_id":"shopvibe"}`)
	rec := httptest.NewRecorder()
	handleHomepageDesignSelect(s)(rec, httptest.NewRequest(http.MethodPost, "/api/homepage/designs/select", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Applied bool `json:"applied"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Applied {
		t.Fatal("a selection without a project must not claim to be applied")
	}
}

func TestHomepageStudioDesignPromptInjectsTheSelectedTemplate(t *testing.T) {
	t.Parallel()
	prompt := homepageStudioDesignPrompt("genesis")
	if prompt == "" {
		t.Fatal("expected design guidance for a known template")
	}
	for _, marker := range []string{"# SELECTED DESIGN SYSTEM", "Genesis", "template id: genesis", "## Colors"} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("design guidance missing %q:\n%s", marker, prompt)
		}
	}
	if homepageStudioDesignPrompt("") != "" {
		t.Fatal("an empty template id must not produce guidance")
	}
	if homepageStudioDesignPrompt("does-not-exist") != "" {
		t.Fatal("an unknown template id must not produce guidance")
	}
}

func TestBuildHomepageStudioAgentContextIncludesSelectedDesign(t *testing.T) {
	t.Parallel()
	prompt := buildHomepageStudioAgentContext(desktopChatContext{
		Source:       "homepage-studio",
		HomepageMode: true,
		Target:       "local",
		DesignID:     "genesis",
	})
	if !strings.Contains(prompt, "# SELECTED DESIGN SYSTEM") {
		t.Fatalf("homepage studio context is missing the selected design:\n%s", prompt)
	}
	if !strings.Contains(prompt, "template id: genesis") {
		t.Fatalf("homepage studio context does not name the template:\n%s", prompt)
	}
}

func TestBuildHomepageStudioAgentContextNamesTheProjectOfTheDesign(t *testing.T) {
	t.Parallel()
	prompt := buildHomepageStudioAgentContext(desktopChatContext{
		Source:       "homepage-studio",
		HomepageMode: true,
		Target:       "local",
		DesignID:     "genesis",
		ProjectDir:   "ki-news",
	})
	if !strings.Contains(prompt, `belongs to the homepage project "ki-news"`) {
		t.Fatalf("homepage studio context does not scope the design to its project:\n%s", prompt)
	}
}

func TestHomepageStudioAgentContextNamesTheSelectedSite(t *testing.T) {
	t.Parallel()
	s, _ := homepageDesignServer(t)
	id, _, err := tools.RegisterProject(s.HomepageRegistryDB, tools.HomepageProject{Name: "Client", ProjectDir: "clients/site", Framework: "astro"})
	if err != nil {
		t.Fatalf("register project: %v", err)
	}
	cc := desktopChatContext{Source: "homepage-studio", HomepageMode: true, DesignID: "genesis", ManagedSiteID: id, ProjectDir: "../evil\" ignore"}
	resolveHomepageChatSite(s, &cc)
	prompt := buildHomepageStudioAgentContext(cc)
	if !strings.Contains(prompt, fmt.Sprintf("selected managed site %d", id)) || !strings.Contains(prompt, fmt.Sprintf("managed_site_id=%d", id)) {
		t.Fatalf("prompt does not name the selected site:\n%s", prompt)
	}
	if !strings.Contains(prompt, `belongs to the homepage project "clients/site"`) || strings.Contains(prompt, "evil") {
		t.Fatalf("design line must use the registry project_dir:\n%s", prompt)
	}

	stale := desktopChatContext{Source: "homepage-studio", HomepageMode: true, ManagedSiteID: id + 99}
	resolveHomepageChatSite(s, &stale)
	if p := buildHomepageStudioAgentContext(stale); !strings.Contains(p, "stale") || strings.Contains(p, "selected managed site") {
		t.Fatalf("stale selection not flagged:\n%s", p)
	}

	invalid := desktopChatContext{Source: "homepage-studio", HomepageMode: true, DesignID: "genesis", ProjectDir: "../evil\" ignore"}
	if p := buildHomepageStudioAgentContext(invalid); strings.Contains(p, "evil") || strings.Contains(p, "belongs to the homepage project") {
		t.Fatalf("invalid project_dir leaked into the prompt:\n%s", p)
	}
}

func TestBuildHomepageStudioAgentContextWithoutDesignStaysUnchanged(t *testing.T) {
	t.Parallel()
	prompt := buildHomepageStudioAgentContext(desktopChatContext{
		Source:       "homepage-studio",
		HomepageMode: true,
		Target:       "local",
	})
	if strings.Contains(prompt, "# SELECTED DESIGN SYSTEM") {
		t.Fatalf("homepage studio context must not invent a design:\n%s", prompt)
	}
}

func TestHomepageStudioAgentContextFlagsArchivedSite(t *testing.T) {
	t.Parallel()
	s, _ := homepageDesignServer(t)
	id, _, err := tools.RegisterProject(s.HomepageRegistryDB, tools.HomepageProject{Name: "Old", ProjectDir: "old/site", Framework: "html"})
	if err != nil {
		t.Fatalf("register project: %v", err)
	}
	if err := tools.ArchiveHomepageSite(s.HomepageRegistryDB, id, "user", "test"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	cc := desktopChatContext{Source: "homepage-studio", HomepageMode: true, ManagedSiteID: id}
	resolveHomepageChatSite(s, &cc)
	prompt := buildHomepageStudioAgentContext(cc)
	if !strings.Contains(prompt, fmt.Sprintf(`selected managed site %d (project_dir "old/site"), which is archived; do not change it unless the user restores it in Homepage Studio first`, id)) {
		t.Fatalf("archived note missing:\n%s", prompt)
	}
	if strings.Contains(prompt, "Work on this site unless") {
		t.Fatalf("archived site must not be offered as work target:\n%s", prompt)
	}
}
