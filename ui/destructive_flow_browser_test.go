package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

type destructiveFlowFixture struct {
	html         string
	sharedJS     string
	containersJS string
}

func newDestructiveFlowFixture(t *testing.T) *destructiveFlowFixture {
	t.Helper()

	read := func(path string) string {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(raw)
	}

	html := read(filepath.Join(".", "containers.html"))
	html = strings.ReplaceAll(html, "{{.Lang}}", "en")
	html = strings.ReplaceAll(html, "{{.I18N}}", "{}")
	html = strings.ReplaceAll(html, `<script src="/shared.js?v=12"></script>`, "")
	html = strings.ReplaceAll(html, `<script src="/js/containers/main.js"></script>`, "")

	return &destructiveFlowFixture{
		html:         html,
		sharedJS:     read(filepath.Join(".", "shared.js")),
		containersJS: read(filepath.Join(".", "js", "containers", "main.js")),
	}
}

func newSmokeBrowser(t *testing.T) *rod.Browser {
	t.Helper()

	bin, ok := browserExecutable()
	if !ok {
		t.Skip("headless browser smoke test requires Chrome or Edge")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	u, err := launcher.New().
		Context(ctx).
		Bin(bin).
		Headless(true).
		NoSandbox(true).
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		Launch()
	if err != nil {
		t.Skipf("headless browser smoke test skipped; browser launch failed: %v", err)
	}
	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		t.Fatalf("connect headless browser for UI smoke test: %v", err)
	}
	t.Cleanup(func() { _ = browser.Close() })
	return browser
}

func browserExecutable() (string, bool) {
	candidates := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func TestContainersDestructiveDeleteFlowBrowserSmoke(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the headless browser smoke test")
	}

	fixture := newDestructiveFlowFixture(t)
	browser := newSmokeBrowser(t)

	for _, viewport := range []struct {
		name   string
		width  int
		height int
		mobile bool
	}{
		{name: "desktop", width: 1280, height: 800},
		{name: "mobile", width: 390, height: 844, mobile: true},
	} {
		t.Run(viewport.name, func(t *testing.T) {
			page := browser.MustPage("about:blank")
			page.MustSetViewport(viewport.width, viewport.height, 1, viewport.mobile)
			defer page.MustClose()
			fixture.loadContainersPage(t, page)

			waitForJSBool(t, page, `() => document.querySelectorAll('.ct-card').length === 1`)
			page.MustElement(".ct-card .btn-danger").MustClick()

			if !page.MustEval(`() => document.getElementById('delete-modal').classList.contains('active')`).Bool() {
				t.Fatal("delete modal did not open")
			}
			if got := page.MustElement("#delete-container-name").MustText(); got != "aurago-test" {
				t.Fatalf("delete modal target name = %q, want aurago-test", got)
			}
			if page.MustEval(`() => document.getElementById('delete-force').checked`).Bool() {
				t.Fatal("force checkbox should reset to unchecked when opening delete modal")
			}
			if page.MustElement("#delete-confirm-btn") == nil {
				t.Fatal("delete confirmation button needs a stable smoke-test selector")
			}

			page.MustEval(`() => {
				document.getElementById('delete-confirm-btn').click();
				document.getElementById('delete-confirm-btn').click();
			}`)
			waitForJSBool(t, page, `() => !document.getElementById('delete-modal').classList.contains('active')`)
			if got := page.MustEval(`() => window.__deleteCalls`).Int(); got != 1 {
				t.Fatalf("double-clicking delete confirm issued %d DELETE requests, want 1", got)
			}
			waitForJSBool(t, page, `() => document.getElementById('ct-total').textContent === '0'`)
			if page.MustEval(`() => document.documentElement.scrollWidth <= window.innerWidth + 2`).Bool() != true {
				t.Fatalf("page overflows horizontally at %s viewport", viewport.name)
			}
		})
	}
}

func TestContainersDestructiveDeleteFlowContract(t *testing.T) {
	t.Parallel()

	html, err := os.ReadFile(filepath.Join(".", "containers.html"))
	if err != nil {
		t.Fatalf("read containers.html: %v", err)
	}
	js, err := os.ReadFile(filepath.Join(".", "js", "containers", "main.js"))
	if err != nil {
		t.Fatalf("read containers main.js: %v", err)
	}

	htmlText := string(html)
	jsText := string(js)
	for _, marker := range []string{
		`id="delete-confirm-btn"`,
		`onclick="confirmDelete()"`,
	} {
		if !strings.Contains(htmlText, marker) {
			t.Fatalf("containers delete modal is missing stable destructive-flow marker %q", marker)
		}
	}
	for _, marker := range []string{
		"let deleteInFlight = false;",
		"if (!deleteTarget || deleteInFlight) return;",
		"deleteInFlight = true;",
		"setDeleteConfirmBusy(true);",
		"setDeleteConfirmBusy(false);",
		"function setDeleteConfirmBusy(busy)",
		"confirmBtn.disabled = busy;",
	} {
		if !strings.Contains(jsText, marker) {
			t.Fatalf("containers delete flow is missing double-submit guard marker %q", marker)
		}
	}
}

func TestGalleryDestructiveDeleteFlowContract(t *testing.T) {
	t.Parallel()

	html, err := os.ReadFile(filepath.Join(".", "gallery.html"))
	if err != nil {
		t.Fatalf("read gallery.html: %v", err)
	}
	js, err := os.ReadFile(filepath.Join(".", "js", "gallery", "main.js"))
	if err != nil {
		t.Fatalf("read gallery main.js: %v", err)
	}

	htmlText := string(html)
	jsText := string(js)
	for _, marker := range []string{
		`id="gallery-delete-confirm-btn"`,
		`onclick="confirmDeleteGallery()"`,
		`id="lightbox-delete"`,
	} {
		if !strings.Contains(htmlText, marker) {
			t.Fatalf("gallery delete flow is missing stable destructive-flow marker %q", marker)
		}
	}
	for _, marker := range []string{
		"let galleryDeleteInFlight = false;",
		"if (galleryDeleteInFlight) return;",
		"galleryDeleteInFlight = true;",
		"setGalleryDeleteBusy(true);",
		"setGalleryDeleteBusy(false);",
		"function setGalleryDeleteBusy(busy)",
		"confirmBtn.disabled = busy;",
		"lightboxBtn.disabled = busy;",
	} {
		if !strings.Contains(jsText, marker) {
			t.Fatalf("gallery delete flow is missing double-submit guard marker %q", marker)
		}
	}
}

func TestSkillsDestructiveDeleteFlowContract(t *testing.T) {
	t.Parallel()

	html, err := os.ReadFile(filepath.Join(".", "skills.html"))
	if err != nil {
		t.Fatalf("read skills.html: %v", err)
	}
	js, err := os.ReadFile(filepath.Join(".", "js", "skills", "main.js"))
	if err != nil {
		t.Fatalf("read skills main.js: %v", err)
	}

	htmlText := string(html)
	jsText := string(js)
	for _, marker := range []string{
		`id="skill-delete-confirm-btn"`,
		`onclick="confirmDeleteSkill()"`,
		`id="delete-files-checkbox"`,
	} {
		if !strings.Contains(htmlText, marker) {
			t.Fatalf("skills delete flow is missing stable destructive-flow marker %q", marker)
		}
	}
	for _, marker := range []string{
		"let skillDeleteInFlight = false;",
		"if (!deleteTargetId || skillDeleteInFlight) return;",
		"skillDeleteInFlight = true;",
		"setSkillDeleteBusy(true);",
		"setSkillDeleteBusy(false);",
		"function setSkillDeleteBusy(busy)",
		"confirmBtn.disabled = busy;",
	} {
		if !strings.Contains(jsText, marker) {
			t.Fatalf("skills delete flow is missing double-submit guard marker %q", marker)
		}
	}
}

func TestKnowledgeDestructiveDeleteFlowContract(t *testing.T) {
	t.Parallel()

	html, err := os.ReadFile(filepath.Join(".", "knowledge.html"))
	if err != nil {
		t.Fatalf("read knowledge.html: %v", err)
	}
	js, err := os.ReadFile(filepath.Join(".", "js", "knowledge", "main.js"))
	if err != nil {
		t.Fatalf("read knowledge main.js: %v", err)
	}

	htmlText := string(html)
	jsText := string(js)
	for _, marker := range []string{
		`id="knowledge-delete-confirm-btn"`,
		`onclick="confirmDelete()"`,
		`id="delete-target-type"`,
	} {
		if !strings.Contains(htmlText, marker) {
			t.Fatalf("knowledge delete flow is missing stable destructive-flow marker %q", marker)
		}
	}
	for _, marker := range []string{
		"let knowledgeDeleteInFlight = false;",
		"if (knowledgeDeleteInFlight) return;",
		"knowledgeDeleteInFlight = true;",
		"setKnowledgeDeleteBusy(true);",
		"setKnowledgeDeleteBusy(false);",
		"function setKnowledgeDeleteBusy(busy)",
		"confirmBtn.disabled = busy;",
	} {
		if !strings.Contains(jsText, marker) {
			t.Fatalf("knowledge delete flow is missing double-submit guard marker %q", marker)
		}
	}
}

func TestConfigEmbeddingsResetRestartFlowContract(t *testing.T) {
	t.Parallel()

	html, err := os.ReadFile(filepath.Join(".", "config.html"))
	if err != nil {
		t.Fatalf("read config.html: %v", err)
	}
	js, err := os.ReadFile(filepath.Join(".", "js", "config", "main.js"))
	if err != nil {
		t.Fatalf("read config main.js: %v", err)
	}

	htmlText := string(html)
	jsText := string(js)
	for _, marker := range []string{
		`id="cfg-restart-btn"`,
		`onclick="restartAuraGo()"`,
		`id="btnSave"`,
		`onclick="saveConfig()"`,
	} {
		if !strings.Contains(htmlText, marker) {
			t.Fatalf("config restart/save flow is missing stable action marker %q", marker)
		}
	}
	for _, marker := range []string{
		`id="embeddings-reset-cancel"`,
		`id="embeddings-reset-continue"`,
		"let configSaveInFlight = false;",
		"if (configSaveInFlight) return;",
		"configSaveInFlight = true;",
		"setConfigSaveBusy(true);",
		"setConfigSaveBusy(false);",
		"function setConfigSaveBusy(busy)",
		"let restartInFlight = false;",
		"if (restartInFlight) return;",
		"restartInFlight = true;",
		"setRestartBusy(true);",
		"setRestartBusy(false);",
		"function setRestartBusy(busy)",
		"restartBtn.disabled = busy;",
	} {
		if !strings.Contains(jsText, marker) {
			t.Fatalf("config embeddings reset/restart flow is missing in-flight guard marker %q", marker)
		}
	}
}

func (f *destructiveFlowFixture) loadContainersPage(t *testing.T, page *rod.Page) {
	t.Helper()

	page.MustSetDocumentContent(f.html)
	if err := page.AddScriptTag("", `
		window.__deleteCalls = 0;
		window.__containers = [{
			id: 'abc123',
			names: ['/aurago-test'],
			image: 'aurago:test',
			state: 'running',
			status: 'Up 1 minute'
		}];
		window.EventSource = function () {
			this.close = function () {};
			setTimeout(() => { if (this.onopen) this.onopen({}); }, 0);
		};
		window.fetch = function (url, options) {
			const method = (options && options.method) || 'GET';
			if (url === '/api/auth/status') {
				return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ authenticated: true }) });
			}
			if (url === '/api/containers' && method === 'GET') {
				return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ status: 'ok', containers: window.__containers }) });
			}
			if (String(url).startsWith('/api/containers/') && method === 'DELETE') {
				window.__deleteCalls++;
				return new Promise(resolve => setTimeout(() => {
					window.__containers = [];
					resolve({ ok: true, status: 200, json: () => Promise.resolve({ status: 'ok' }) });
				}, 150));
			}
			return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ status: 'ok' }), text: () => Promise.resolve('ok') });
		};
	`); err != nil {
		t.Fatalf("install browser fixture script: %v", err)
	}
	if err := page.AddScriptTag("", f.sharedJS); err != nil {
		t.Fatalf("install shared.js: %v", err)
	}
	if err := page.AddScriptTag("", f.containersJS); err != nil {
		t.Fatalf("install containers main.js: %v", err)
	}
	page.MustEval(`() => document.dispatchEvent(new Event('DOMContentLoaded', { bubbles: true }))`)
}

func waitForJSBool(t *testing.T, page *rod.Page, expr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result := page.MustEval(expr)
		if result.Bool() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("condition did not become true: %s", expr)
}
