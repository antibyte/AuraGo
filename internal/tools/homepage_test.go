package tools

import (
	"strings"
	"testing"
)

// ─── isValidHomepageURL ──────────────────────────────────────────────────

func TestIsValidHomepageURL_Valid(t *testing.T) {
	valid := []string{
		"http://localhost:3000",
		"https://example.com",
		"http://192.168.1.1:8080/path",
		"https://my-site.example.com/page?q=1",
		"HTTP://EXAMPLE.COM",
	}
	for _, u := range valid {
		if !isValidHomepageURL(u) {
			t.Errorf("expected valid URL, got rejected: %q", u)
		}
	}
}

func TestIsValidHomepageURL_Rejects(t *testing.T) {
	invalid := []string{
		"",
		"ftp://example.com",
		"javascript:alert(1)",
		"http://example.com; rm -rf /",
		"http://example.com$(whoami)",
		"http://example.com`id`",
		"http://example.com|cat /etc/passwd",
		"http://example.com&bg",
		"http://example.com\nhttp://evil.com",
		"http://example.com\"injected",
		"http://example.com'injected",
		"http://example.com\\path",
		"http://example.com!",
		"/etc/passwd",
		"example.com",
	}
	for _, u := range invalid {
		if isValidHomepageURL(u) {
			t.Errorf("expected rejection, got valid for: %q", u)
		}
	}
}

// ─── sanitizeProjectDir ──────────────────────────────────────────────────

func TestSanitizeProjectDir_Valid(t *testing.T) {
	valid := []string{
		"my-site",
		"project123",
		"my_portfolio",
		"site-v2.0",
		"sub/dir",
	}
	for _, d := range valid {
		if err := sanitizeProjectDir(d); err != nil {
			t.Errorf("expected valid dir %q, got error: %v", d, err)
		}
	}
}

func TestSanitizeProjectDir_PathTraversal(t *testing.T) {
	traversal := []string{
		"..",
		"../etc",
		"foo/../bar",
		"foo/../../etc/passwd",
	}
	for _, d := range traversal {
		err := sanitizeProjectDir(d)
		if err == nil {
			t.Errorf("expected path traversal rejection for %q", d)
		}
		if err != nil && !strings.Contains(err.Error(), "path traversal") {
			t.Errorf("expected path traversal error for %q, got: %v", d, err)
		}
	}
}

func TestSanitizeProjectDir_AbsolutePaths(t *testing.T) {
	abs := []string{"/etc/passwd", "\\windows\\system32"}
	for _, d := range abs {
		err := sanitizeProjectDir(d)
		if err == nil {
			t.Errorf("expected absolute path rejection for %q", d)
		}
		if err != nil && !strings.Contains(err.Error(), "absolute paths") {
			t.Errorf("expected absolute paths error for %q, got: %v", d, err)
		}
	}
}

func TestSanitizeProjectDir_AbsolutePathGuidance(t *testing.T) {
	err := sanitizeProjectDir("/workspace/ki-news")
	if err == nil {
		t.Fatal("expected absolute path to be rejected")
	}
	if !strings.Contains(err.Error(), "project_dir must be relative") {
		t.Fatalf("expected relative project_dir guidance, got: %v", err)
	}
}

func TestValidateHomepageRelativePathArgGuidance(t *testing.T) {
	err := validateHomepageRelativePathArg("/workspace/ki-news/src/app/page.tsx", "path")
	if err == nil {
		t.Fatal("expected absolute homepage path to be rejected")
	}
	if !strings.Contains(err.Error(), "must be relative to the homepage workspace") {
		t.Fatalf("expected relative workspace guidance, got: %v", err)
	}
}

func TestHomepageWorkspacePathNotConfiguredJSONIncludesGuidance(t *testing.T) {
	result := homepageWorkspacePathNotConfiguredJSON()
	if !strings.Contains(result, "workspace_path not configured") {
		t.Fatalf("expected workspace_path guidance error, got: %s", result)
	}
	if !strings.Contains(result, "project_dir/path values") {
		t.Fatalf("expected detailed workspace guidance, got: %s", result)
	}
}

func TestSanitizeProjectDir_ShellMetachars(t *testing.T) {
	shellInjections := []string{
		"foo;bar",
		"foo|bar",
		"foo&bar",
		"foo`id`",
		"foo$(whoami)",
		"foo{a,b}",
		"foo>out",
		"foo<in",
		"foo'bar",
		"foo\"bar",
		"foo!bar",
		"foo bar",
		"foo\nbar",
	}
	for _, d := range shellInjections {
		err := sanitizeProjectDir(d)
		if err == nil {
			t.Errorf("expected shell metachar rejection for %q", d)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid character") {
			t.Errorf("expected invalid character error for %q, got: %v", d, err)
		}
	}
}

// ─── truncateStr ─────────────────────────────────────────────────────────

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello…"},
		{"", 5, ""},
		{"a", 1, "a"},
		{"ab", 1, "a…"},
	}
	for _, tc := range tests {
		got := truncateStr(tc.s, tc.maxLen)
		if got != tc.want {
			t.Errorf("truncateStr(%q, %d) = %q, want %q", tc.s, tc.maxLen, got, tc.want)
		}
	}
}

// ─── extractOutput ───────────────────────────────────────────────────────

func TestExtractOutput(t *testing.T) {
	// Valid JSON with output field
	got := extractOutput(`{"output":"hello","status":"ok"}`)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}

	// No output field — returns original
	got = extractOutput(`{"status":"ok"}`)
	if got != `{"status":"ok"}` {
		t.Errorf("expected original JSON, got %q", got)
	}

	// Not JSON — returns original
	got = extractOutput("plain text")
	if got != "plain text" {
		t.Errorf("expected 'plain text', got %q", got)
	}
}

func TestShellSingleQuoteEscapesSingleQuotes(t *testing.T) {
	got := shellSingleQuote("/var/www/it's-here")
	want := `'/var/www/it'"'"'s-here'`
	if got != want {
		t.Fatalf("shellSingleQuote = %q, want %q", got, want)
	}
}

func TestHomepageScreenshotFallsBackToWebCaptureWhenPlaywrightIsMissing(t *testing.T) {
	oldDockerExec := homepageDockerExecFunc
	oldWebCapture := homepageWebCaptureFunc
	defer func() {
		homepageDockerExecFunc = oldDockerExec
		homepageWebCaptureFunc = oldWebCapture
	}()

	homepageDockerExecFunc = func(cfg DockerConfig, containerName, command, user string) string {
		return `{"container_id":"aurago-homepage","exit_code":1,"output":"Error: Cannot find module 'playwright'"}`
	}
	homepageWebCaptureFunc = func(operation, rawURL, selector string, fullPage bool, outputDir string) string {
		if operation != "screenshot" {
			t.Fatalf("operation = %q, want screenshot", operation)
		}
		if rawURL != "https://example.com" {
			t.Fatalf("url = %q, want https://example.com", rawURL)
		}
		return `{"status":"success","operation":"screenshot","file":"agent_workspace/workdir/fallback.png"}`
	}

	got := HomepageScreenshot(HomepageConfig{}, "https://example.com", "", nil)
	if !strings.Contains(got, `"status":"success"`) {
		t.Fatalf("expected web_capture fallback result, got: %s", got)
	}
	if !strings.Contains(got, "fallback.png") {
		t.Fatalf("expected fallback screenshot path, got: %s", got)
	}
}

// ─── Build Auto-Fix Pattern Matching ─────────────────────────────────────

func TestBuildFixPatterns_MissingNpmModule(t *testing.T) {
	output := `Error: Cannot find module 'react-router-dom'`
	for _, p := range buildFixPatterns {
		if p.name != "missing-npm-module" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected missing-npm-module pattern to match")
		}
		fix := p.fix(m, "my-site")
		if !strings.Contains(fix, "npm install react-router-dom") {
			t.Errorf("expected fix to contain 'npm install react-router-dom', got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_MissingNpmModuleSubpath(t *testing.T) {
	output := `Error: Cannot find module 'lodash/debounce'`
	for _, p := range buildFixPatterns {
		if p.name != "missing-npm-module" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected pattern to match")
		}
		fix := p.fix(m, "site")
		// Should install just 'lodash', not 'lodash/debounce'
		if !strings.Contains(fix, "npm install lodash") {
			t.Errorf("expected fix to install 'lodash', got: %s", fix)
		}
		if strings.Contains(fix, "lodash/debounce") {
			t.Errorf("should strip subpath, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_ScopedPackage(t *testing.T) {
	output := `Error: Cannot find module '@emotion/react/jsx-runtime'`
	for _, p := range buildFixPatterns {
		if p.name != "missing-npm-module" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected pattern to match")
		}
		fix := p.fix(m, "site")
		if !strings.Contains(fix, "npm install @emotion/react") {
			t.Errorf("expected scoped package install, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_WebpackModuleNotFound(t *testing.T) {
	output := `Module not found: Error: Can't resolve 'axios' in '/workspace/site/src'`
	for _, p := range buildFixPatterns {
		if p.name != "module-not-found-webpack" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected module-not-found-webpack pattern to match")
		}
		fix := p.fix(m, "site")
		if !strings.Contains(fix, "npm install axios") {
			t.Errorf("expected fix to install 'axios', got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_WebpackRelativePath(t *testing.T) {
	output := `Module not found: Error: Can't resolve './components/Header' in '/workspace/site/src'`
	for _, p := range buildFixPatterns {
		if p.name != "module-not-found-webpack" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected pattern to match")
		}
		fix := p.fix(m, "site")
		// Relative paths should not produce a fix
		if fix != "" {
			t.Errorf("relative path should not produce fix, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_EslintFixable(t *testing.T) {
	output := `12 errors and 5 warnings potentially fixable with the --fix option.`
	for _, p := range buildFixPatterns {
		if p.name != "eslint-fixable" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected eslint-fixable pattern to match")
		}
		fix := p.fix(m, "my-site")
		if !strings.Contains(fix, "eslint . --fix") {
			t.Errorf("expected eslint fix, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_NpmInstallNeeded(t *testing.T) {
	output := `npm ERR! missing: webpack@^5.0.0, required by my-site@1.0.0`
	for _, p := range buildFixPatterns {
		if p.name != "npm-install-needed" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected npm-install-needed pattern to match")
		}
		fix := p.fix(m, "site")
		if !strings.Contains(fix, "npm install") {
			t.Errorf("expected npm install, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_NodeModulesMissing(t *testing.T) {
	output := `Error: Cannot find module '/workspace/site/node_modules/react/index.js'`
	for _, p := range buildFixPatterns {
		if p.name != "node-modules-missing" {
			continue
		}
		m := p.pattern.FindStringSubmatch(output)
		if m == nil {
			t.Fatal("expected node-modules-missing pattern to match")
		}
		fix := p.fix(m, "site")
		if !strings.Contains(fix, "npm install") {
			t.Errorf("expected npm install, got: %s", fix)
		}
	}
}

func TestBuildFixPatterns_NoMatch(t *testing.T) {
	output := `SyntaxError: Unexpected token in JSON at position 0`
	for _, p := range buildFixPatterns {
		m := p.pattern.FindStringSubmatch(output)
		if m != nil {
			t.Errorf("pattern %q should NOT match syntax error output", p.name)
		}
	}
}

// ─── Template System ─────────────────────────────────────────────────────

func TestGetTemplateFiles_KnownTemplates(t *testing.T) {
	templates := []string{"portfolio", "blog", "landing", "dashboard"}
	for _, name := range templates {
		files := getTemplateFiles(name, "test-project")
		if len(files) == 0 {
			t.Errorf("template %q returned no files", name)
		}
		// All templates should have a CSS file and a TEMPLATE_README.md
		hasCss := false
		hasReadme := false
		for _, f := range files {
			if strings.HasSuffix(f.path, ".css") {
				hasCss = true
			}
			if f.path == "TEMPLATE_README.md" {
				hasReadme = true
			}
		}
		if !hasCss {
			t.Errorf("template %q has no CSS file", name)
		}
		if !hasReadme {
			t.Errorf("template %q has no TEMPLATE_README.md", name)
		}
	}
}

func TestGetTemplateFiles_CaseInsensitive(t *testing.T) {
	files := getTemplateFiles("PORTFOLIO", "test")
	if len(files) == 0 {
		t.Error("expected PORTFOLIO (uppercase) to resolve to template files")
	}
	files = getTemplateFiles("Blog", "test")
	if len(files) == 0 {
		t.Error("expected Blog (mixed case) to resolve to template files")
	}
}

func TestGetTemplateFiles_Unknown(t *testing.T) {
	files := getTemplateFiles("nonexistent", "test")
	if len(files) != 0 {
		t.Errorf("expected no files for unknown template, got %d", len(files))
	}
}

func TestGetTemplateFiles_Empty(t *testing.T) {
	files := getTemplateFiles("", "test")
	if len(files) != 0 {
		t.Errorf("expected no files for empty template, got %d", len(files))
	}
}

// ─── getLocalLANIP ───────────────────────────────────────────────────────

func TestGetLocalLANIP_Format(t *testing.T) {
	ip := getLocalLANIP()
	// May return empty if CI has no real interfaces; that's acceptable.
	if ip == "" {
		t.Log("getLocalLANIP returned empty (no non-loopback interface) — OK in CI")
		return
	}
	// Must not be loopback
	if ip == "127.0.0.1" || ip == "::1" {
		t.Errorf("getLocalLANIP should not return loopback, got %q", ip)
	}
	// Should look like an IPv4 address
	if strings.Count(ip, ".") != 3 {
		t.Errorf("expected IPv4 format, got %q", ip)
	}
}
