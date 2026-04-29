package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestExecuteDocumentCreator_UnknownOperation(t *testing.T) {
	cfg := &config.DocumentCreatorConfig{
		Enabled:   true,
		Backend:   "maroto",
		OutputDir: t.TempDir(),
	}
	result := ExecuteDocumentCreator(context.Background(), cfg, "bogus", "", "", "", "", "", false, "", "")
	if !strings.Contains(result, "unknown operation") {
		t.Fatalf("expected unknown operation error, got: %s", result)
	}
}

func TestExecuteDocumentCreatorInWorkspaceRejectsSourceEscape(t *testing.T) {
	projectRoot := t.TempDir()
	workspaceDir := filepath.Join(projectRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll workspace: %v", err)
	}
	outside := filepath.Join(filepath.Dir(projectRoot), "outside.docx")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatalf("WriteFile outside: %v", err)
	}
	cfg := &config.DocumentCreatorConfig{
		Enabled:   true,
		Backend:   "gotenberg",
		OutputDir: t.TempDir(),
		Gotenberg: config.GotenbergConfig{URL: "http://127.0.0.1:1"},
	}

	result := ExecuteDocumentCreatorInWorkspace(context.Background(), cfg, workspaceDir, "convert_document", "", "", "", "", "", false, "", `["../../../outside.docx"]`)

	if !strings.Contains(result, "escapes the project root") {
		t.Fatalf("result = %s, want path escape rejection", result)
	}
}

func TestExecuteDocumentCreator_CreatePDFMaroto_Simple(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.DocumentCreatorConfig{
		Enabled:   true,
		Backend:   "maroto",
		OutputDir: dir,
	}
	result := ExecuteDocumentCreator(context.Background(), cfg, "create_pdf", "Test Report", "Hello world", "", "test_output.pdf", "A4", false, "", "")
	if !strings.Contains(result, `"status":"success"`) {
		t.Fatalf("expected success, got: %s", result)
	}
	if !strings.Contains(result, "test_output.pdf") {
		t.Fatalf("expected filename in result, got: %s", result)
	}
	// Verify file was created
	if _, err := os.Stat(filepath.Join(dir, "test_output.pdf")); err != nil {
		t.Fatalf("output file not created: %v", err)
	}
}

func TestExecuteDocumentCreator_CreatePDFMaroto_Sections(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.DocumentCreatorConfig{
		Enabled:   true,
		Backend:   "maroto",
		OutputDir: dir,
	}

	sections := []PDFSection{
		{Type: "text", Header: "Overview", Body: "This is an overview section."},
		{Type: "table", Header: "Metrics", Rows: [][]string{{"Name", "Value"}, {"CPU", "95%"}, {"RAM", "4GB"}}},
		{Type: "list", Header: "Tasks", Body: "Task 1\nTask 2\nTask 3"},
	}
	sectionsJSON, _ := json.Marshal(sections)

	result := ExecuteDocumentCreator(context.Background(), cfg, "create_pdf", "Sections Report", "", "", "", "A4", false, string(sectionsJSON), "")
	if !strings.Contains(result, `"status":"success"`) {
		t.Fatalf("expected success, got: %s", result)
	}
}

func TestExecuteDocumentCreator_CreatePDFMaroto_Landscape(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.DocumentCreatorConfig{
		Enabled:   true,
		Backend:   "maroto",
		OutputDir: dir,
	}
	result := ExecuteDocumentCreator(context.Background(), cfg, "create_pdf", "Landscape Doc", "Content here", "", "", "A4", true, "", "")
	if !strings.Contains(result, `"status":"success"`) {
		t.Fatalf("expected success, got: %s", result)
	}
}

func TestExecuteDocumentCreator_CreatePDFMaroto_PaperSizes(t *testing.T) {
	for _, size := range []string{"A3", "A4", "A5", "Letter", "Legal", "Tabloid"} {
		t.Run(size, func(t *testing.T) {
			dir := t.TempDir()
			cfg := &config.DocumentCreatorConfig{
				Enabled:   true,
				Backend:   "maroto",
				OutputDir: dir,
			}
			result := ExecuteDocumentCreator(context.Background(), cfg, "create_pdf", "Test "+size, "Body", "", "", size, false, "", "")
			if !strings.Contains(result, `"status":"success"`) {
				t.Fatalf("paper size %s: expected success, got: %s", size, result)
			}
		})
	}
}

func TestExecuteDocumentCreator_CreatePDFMaroto_InvalidSections(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.DocumentCreatorConfig{
		Enabled:   true,
		Backend:   "maroto",
		OutputDir: dir,
	}
	result := ExecuteDocumentCreator(context.Background(), cfg, "create_pdf", "Bad", "", "", "", "", false, "not json", "")
	if !strings.Contains(result, "invalid sections JSON") {
		t.Fatalf("expected invalid sections error, got: %s", result)
	}
}

func TestExecuteDocumentCreator_GotenbergOnly_MarotoBackend(t *testing.T) {
	cfg := &config.DocumentCreatorConfig{
		Enabled:   true,
		Backend:   "maroto",
		OutputDir: t.TempDir(),
	}
	operations := []string{"url_to_pdf", "html_to_pdf", "markdown_to_pdf", "convert_document", "merge_pdfs", "screenshot_url", "screenshot_html", "health"}
	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			result := ExecuteDocumentCreator(context.Background(), cfg, op, "", "content", "http://example.com", "", "", false, "", "")
			if !strings.Contains(result, "requires the Gotenberg backend") {
				t.Fatalf("op %s: expected Gotenberg required error, got: %s", op, result)
			}
		})
	}
}

func TestExecuteDocumentCreator_CreatePDFMaroto_AutoFilename(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.DocumentCreatorConfig{
		Enabled:   true,
		Backend:   "maroto",
		OutputDir: dir,
	}
	result := ExecuteDocumentCreator(context.Background(), cfg, "create_pdf", "Auto Name", "Body", "", "", "", false, "", "")
	if !strings.Contains(result, `"status":"success"`) {
		t.Fatalf("expected success, got: %s", result)
	}
	if !strings.Contains(result, "doc_") {
		t.Fatalf("expected auto-generated filename starting with doc_, got: %s", result)
	}
}

func TestBuildHTMLFromSections(t *testing.T) {
	sections := `[{"type":"text","header":"Intro","body":"Hello"},{"type":"table","header":"Data","rows":[["A","B"],["1","2"]]}]`
	html := buildHTMLFromSections("Title", "", sections)
	if !strings.Contains(html, "<h1>Title</h1>") {
		t.Error("expected title in HTML")
	}
	if !strings.Contains(html, "<h2>Intro</h2>") {
		t.Error("expected section header in HTML")
	}
	if !strings.Contains(html, "<table>") {
		t.Error("expected table in HTML")
	}
	if !strings.Contains(html, "<th>A</th>") {
		t.Error("expected table header cell")
	}
}

func TestBuildHTMLFromSections_List(t *testing.T) {
	sections := `[{"type":"list","header":"Items","body":"Apple\nBanana\nCherry"}]`
	html := buildHTMLFromSections("", "", sections)
	if !strings.Contains(html, "<ul>") {
		t.Error("expected unordered list")
	}
	if !strings.Contains(html, "<li>Apple</li>") {
		t.Error("expected list item")
	}
}

func TestBuildHTMLFromSections_PlainContent(t *testing.T) {
	html := buildHTMLFromSections("Doc", "<p>Hello</p>", "")
	if !strings.Contains(html, "<p>Hello</p>") {
		t.Error("expected plain content passthrough")
	}
}

func TestParseSourceFiles(t *testing.T) {
	// JSON array
	result := parseSourceFiles(`["file1.pdf","file2.pdf"]`)
	if len(result) != 2 || result[0] != "file1.pdf" {
		t.Fatalf("expected 2 files from JSON, got: %v", result)
	}

	// Non-JSON returns nil
	result = parseSourceFiles("a.pdf, b.pdf, c.pdf")
	if result != nil {
		t.Fatalf("expected nil from non-JSON, got: %v", result)
	}

	// Empty
	result = parseSourceFiles("")
	if len(result) != 0 {
		t.Fatalf("expected 0 files from empty string, got: %v", result)
	}
}

func TestEscapeHTML(t *testing.T) {
	input := `<script>alert("xss")</script>`
	result := escapeHTML(input)
	if strings.Contains(result, "<script>") {
		t.Error("expected HTML to be escaped")
	}
	if !strings.Contains(result, "&lt;script&gt;") {
		t.Errorf("expected escaped tags, got: %s", result)
	}
}
