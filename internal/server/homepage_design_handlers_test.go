package server

import (
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
