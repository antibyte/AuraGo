package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestConfigHTMLDoesNotUseDocumentWrite(t *testing.T) {
	t.Parallel()

	content := readUITestFile(t, "config.html")
	if strings.Contains(content, "document.write") {
		t.Fatal("config.html must use DOM/i18n placeholders instead of document.write")
	}
}

func TestConfigWindowOpenUsesNoopener(t *testing.T) {
	t.Parallel()

	files := []string{
		filepath.Join("cfg", "google_workspace.js"),
		filepath.Join("cfg", "mcp_server.js"),
		filepath.Join("cfg", "providers.js"),
	}
	windowOpen := regexp.MustCompile(`window\.open\s*\(`)
	for _, file := range files {
		content := readUITestFile(t, file)
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if windowOpen.MatchString(line) && !strings.Contains(line, "noopener") {
				t.Fatalf("%s:%d window.open must include noopener,noreferrer", filepath.ToSlash(file), i+1)
			}
		}
	}
}

func TestConfigAndSetupErrorsAreNotRawInnerHTML(t *testing.T) {
	t.Parallel()

	files := []string{
		filepath.Join("js", "config", "main.js"),
		filepath.Join("js", "setup", "main.js"),
	}
	rawErrorInnerHTML := regexp.MustCompile(`innerHTML\s*=.*(e\.message|err\.message|error\.message|data\.error|data\.message)`)
	for _, file := range files {
		content := readUITestFile(t, file)
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if rawErrorInnerHTML.MatchString(line) && !strings.Contains(line, "escapeHtml") {
				t.Fatalf("%s:%d raw error text must not be assigned to innerHTML", filepath.ToSlash(file), i+1)
			}
		}
	}
}

func TestI18NHTMLUsesSafeTextNodeRendering(t *testing.T) {
	t.Parallel()

	content := readUITestFile(t, "shared.js")
	if strings.Contains(content, "innerHTML = translated.replace") {
		t.Fatal("data-i18n-html translations must be rendered as text nodes plus <br>, not assigned as raw HTML")
	}
}

func readUITestFile(t *testing.T, rel string) string {
	t.Helper()
	content, err := os.ReadFile(rel)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(content)
}
