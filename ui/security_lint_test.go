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
			if windowOpen.MatchString(line) && !strings.Contains(line, "noopener") && !allowsDetachedBlankPopup(lines, i) {
				t.Fatalf("%s:%d window.open must include noopener,noreferrer", filepath.ToSlash(file), i+1)
			}
		}
	}
}

func TestFrontendWindowOpenUsesValidationAndNoReferrer(t *testing.T) {
	t.Parallel()

	files := []string{
		filepath.Join("js", "desktop", "apps", "homepage-studio.js"),
		filepath.Join("js", "desktop", "apps", "quickconnect-launchpad-chat.js"),
		filepath.Join("js", "desktop", "apps", "software-store.js"),
		filepath.Join("js", "skills", "main.js"),
	}
	for _, file := range files {
		content := readUITestFile(t, file)
		if strings.Contains(content, "window.open(state.previewUrl, '_blank')") ||
			strings.Contains(content, "window.open(url, '_blank', 'noopener')") ||
			strings.Contains(content, "window.open(body.url, '_blank', 'noopener')") ||
			strings.Contains(content, "window.open('', '_blank')") {
			t.Fatalf("%s opens unvalidated or referrer-leaking windows", filepath.ToSlash(file))
		}
		if strings.Contains(content, "window.open(") && !strings.Contains(content, "noopener,noreferrer") {
			t.Fatalf("%s window.open calls must request noopener,noreferrer", filepath.ToSlash(file))
		}
	}

	for _, file := range files[:3] {
		content := readUITestFile(t, file)
		if !strings.Contains(content, "safeExternalURL") {
			t.Fatalf("%s must validate external URLs before window.open or iframe navigation", filepath.ToSlash(file))
		}
	}
}

func allowsDetachedBlankPopup(lines []string, idx int) bool {
	line := strings.ReplaceAll(lines[idx], " ", "")
	if !strings.Contains(line, "window.open('','_blank')") && !strings.Contains(line, `window.open("","_blank")`) {
		return false
	}
	for i := idx + 1; i < len(lines) && i <= idx+5; i++ {
		if strings.Contains(strings.ReplaceAll(lines[i], " ", ""), ".opener=null") {
			return true
		}
	}
	return false
}

func TestConfigAndSetupErrorsAreNotRawInnerHTML(t *testing.T) {
	t.Parallel()

	files := []string{
		filepath.Join("cfg", "providers.js"),
		filepath.Join("cfg", "remote_control.js"),
		filepath.Join("cfg", "updates.js"),
		filepath.Join("js", "config", "main.js"),
		filepath.Join("js", "setup", "main.js"),
	}
	rawErrorInnerHTML := regexp.MustCompile(`innerHTML\s*=.*(e\.message|err\.message|error\.message|data\.error|data\.message)`)
	for _, file := range files {
		content := readUITestFile(t, file)
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if rawErrorInnerHTML.MatchString(line) &&
				!strings.Contains(line, "escapeHtml") &&
				!strings.Contains(line, "escapeAttr") &&
				!strings.Contains(line, "esc(") {
				t.Fatalf("%s:%d raw error text must not be assigned to innerHTML", filepath.ToSlash(file), i+1)
			}
		}
	}
}

func TestConfigRestartMessageIsRenderedAsText(t *testing.T) {
	t.Parallel()

	content := readUITestFile(t, filepath.Join("js", "config", "main.js"))
	for _, forbidden := range []string{
		"${res.message}",
		"document.body.innerHTML = `",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("config restart response message must be rendered through textContent/DOM nodes, found %q", forbidden)
		}
	}
}

func TestProxyStatusUsesDOMTextNodes(t *testing.T) {
	t.Parallel()

	content := readUITestFile(t, filepath.Join("cfg", "security_proxy.js"))
	for _, forbidden := range []string{
		"el.innerHTML = `${icon}",
		"${p.state || 'unknown'}",
		"${p.image}",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("proxy status must render API text with DOM text nodes, found %q", forbidden)
		}
	}
	for _, marker := range []string{"document.createTextNode", "el.replaceChildren"} {
		if !strings.Contains(content, marker) {
			t.Fatalf("proxy status missing safe DOM marker %q", marker)
		}
	}
}

func TestMarkdownSinksUseDOMPurify(t *testing.T) {
	t.Parallel()

	files := []string{
		filepath.Join("js", "desktop", "apps", "cheater.js"),
		filepath.Join("js", "desktop", "apps", "viewer.js"),
		filepath.Join("js", "skills", "main.js"),
	}
	for _, file := range files {
		content := readUITestFile(t, file)
		for _, marker := range []string{"window.DOMPurify", "DOMPurify.sanitize"} {
			if !strings.Contains(content, marker) {
				t.Fatalf("%s rendered Markdown must be sanitized with %s", filepath.ToSlash(file), marker)
			}
		}
	}

	skills := readUITestFile(t, filepath.Join("js", "skills", "main.js"))
	if strings.Contains(skills, "} else if (window.marked)") ||
		strings.Contains(skills, "marked is configured for safe defaults") {
		t.Fatal("skills Markdown renderer must not fall back to unsafe marked output without DOMPurify")
	}

	loader := readUITestFile(t, filepath.Join("js", "desktop", "core", "module-loader.js"))
	if !strings.Contains(loader, "/js/vendor/purify.min.js") {
		t.Fatal("desktop Markdown apps must lazy-load DOMPurify")
	}

	skillsHTML := readUITestFile(t, "skills.html")
	if !strings.Contains(skillsHTML, "/js/vendor/purify.min.js") || !strings.Contains(skillsHTML, "/js/vendor/marked.min.js") {
		t.Fatal("skills documentation page must load marked and DOMPurify before rendering Markdown")
	}
}

func TestI18NHTMLUsesSafeTextNodeRendering(t *testing.T) {
	t.Parallel()

	files := []string{
		"shared.js",
		filepath.Join("js", "setup", "main.js"),
	}
	for _, file := range files {
		content := readUITestFile(t, file)
		for _, forbidden := range []string{"innerHTML = translated.replace", "innerHTML = val.replace"} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s data-i18n-html translations must be rendered as text nodes plus <br>, not assigned as raw HTML", filepath.ToSlash(file))
			}
		}
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
