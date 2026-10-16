package ui

import (
	"strings"
	"testing"
)

func TestCodeStudioUsesPerWindowStateAndClosesTerminal(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/code-studio.js")
	for _, marker := range []string{
		"const instances = new Map()",
		"function createInstance",
		"instances.set(windowId, instance)",
		"CodeStudioApp.dispose",
		"const sockets = new Set()",
		"sockets.add(instance.ws)",
		"sockets.add(session.ws)",
		"endTerminalSocket(socket)",
		"function runWithInstance(instance, fn)",
		"finally {",
		"async function runAsyncStep",
		"instances.get(state.windowId) === state",
		"if (!isLiveInstance(instance)) return;",
		"const target = state;",
		"if (!isLiveInstance(target)) return;",
		"if (!isLiveInstance(instance)) return undefined;",
		"renderStatus(tr('codeStudio.running'",
		"function destroyTabView",
		"destroyTabView(tab);",
		"function registerDisposer",
		"state.disposers = state.disposers.filter(item => item !== disposeFn)",
		"registerDisposer(() => cleanup(''))",
		"registerDisposer(() => cleanup(false))",
		"term.dispose()",
		"closeTerminalSessionSockets(instance);",
		"function closeTerminalSessionSockets(instance)",
		"session.ws = null",
		"terminalDisposed",
		"state.disposers.push(() => {",
		"document.removeEventListener('mousedown'",
		"cleanup('')",
		"cleanup(false)",
		"function normalizeCodeStudioPath(rawPath)",
		"path = normalizeCodeStudioPath(path)",
		"if (path === WORKSPACE_ROOT)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("Code Studio per-window lifecycle missing marker %q", marker)
		}
	}
	if strings.Contains(source, "result && typeof result.then === 'function'") {
		t.Fatalf("Code Studio runWithInstance must not hold state until promises settle")
	}
	if strings.Contains(source, "return await fn(instance)") {
		t.Fatal("Code Studio must release global state before awaiting instance work")
	}
	if strings.Contains(source, "return runWithInstance(instance, async () => {") {
		t.Fatalf("Code Studio render must not hold global state across awaited operations")
	}
	if strings.Contains(source, "if (!instances.has(windowId)) return;") {
		t.Fatalf("Code Studio render must ignore stale instances, not just reused window IDs")
	}
	if strings.Contains(source, "if (instances.has(windowId)) runWithInstance(instance") {
		t.Fatalf("Code Studio render catch path must ignore stale instances")
	}
	if !strings.Contains(source, "window.CodeStudio = window.CodeStudioApp") {
		t.Fatalf("Code Studio compatibility export missing")
	}
}

func TestDesktopCodeStudioRunUsesTerminalSession(t *testing.T) {
	t.Parallel()

	core := readDesktopAssetText(t, "js/desktop/apps/code-studio/core.js")
	terminal := readDesktopAssetText(t, "js/desktop/apps/code-studio/terminal.js")
	bundle := readDesktopAssetText(t, "js/desktop/apps/code-studio.js")
	for _, want := range []string{
		"writeTerminalBytes(command + '\\n', terminal)",
		"tr('codeStudio.noRunner'",
		"state.runSession = terminal",
		"if (ext === 'js' || ext === 'mjs') return 'node ' + quoted;",
		"gcc -std=c17 -Wall -Wextra -x c",
		`rustc ' + quoted + ' -o "$cs_run_dir/program"`,
		"mktemp -d /tmp/cs-run.XXXXXX",
		"return null;",
	} {
		if !strings.Contains(core, want) {
			t.Fatalf("Code Studio run path missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"return 'cat '",
		"if (lang === 'javascript')",
		"-o /tmp/cs-run",
	} {
		if strings.Contains(core, forbidden) {
			t.Fatalf("Code Studio run path still contains %q", forbidden)
		}
	}
	start := strings.Index(core, "async function runCurrentFile()")
	end := strings.Index(core, "function toggleSidebar()")
	if start < 0 || end < start {
		t.Fatal("runCurrentFile body missing")
	}
	if strings.Contains(core[start:end], "/api/code-studio/exec") {
		t.Fatal("runCurrentFile must write the terminal session instead of posting /api/code-studio/exec")
	}
	for _, want := range []string{
		"function writeTerminalBytes(",
		"function stopTerminalRun()",
		"data-terminal-stop",
		"new Uint8Array([0x03])",
	} {
		if !strings.Contains(terminal, want) {
			t.Fatalf("Code Studio terminal run control missing %q", want)
		}
	}
	for _, want := range []string{
		"writeTerminalBytes(command + '\\n', terminal)",
		"function writeTerminalBytes(",
		"new Uint8Array([0x03])",
		"data-terminal-stop",
		`rustc ' + quoted + ' -o "$cs_run_dir/program"`,
	} {
		if !strings.Contains(bundle, want) {
			t.Fatalf("Code Studio bundle missing run marker %q", want)
		}
	}
	bundleStart := strings.Index(bundle, "async function runCurrentFile()")
	bundleEnd := strings.Index(bundle, "function toggleSidebar()")
	if bundleStart < 0 || bundleEnd < bundleStart || strings.Contains(bundle[bundleStart:bundleEnd], "/api/code-studio/exec") {
		t.Fatal("Code Studio bundle run still posts /api/code-studio/exec")
	}
}

func TestCodeStudioMarkdownLinksRejectUnsafeProtocols(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/code-studio.js")
	for _, marker := range []string{
		"function sanitizeMarkdownHref(rawHref)",
		"protocol === 'http:' || protocol === 'https:' || protocol === 'mailto:'",
		"href=\"${sanitizeMarkdownHref(href)}\"",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("Code Studio markdown link sanitizer missing marker %q", marker)
		}
	}
	if strings.Contains(source, `'<a href="$2" target="_blank" rel="noopener">$1</a>'`) {
		t.Fatalf("Code Studio markdown renderer must not emit raw hrefs from agent text")
	}
}

func TestCodeStudioTerminalSwitchPreservesMountedSession(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/code-studio.js")
	for _, marker := range []string{
		"function mountActiveTerminalSession(session)",
		"mountActiveTerminalSession(session);",
		"renderTerminal();",
		"connectTerminalSession(index, screen, label);",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("Code Studio terminal lifecycle missing marker %q", marker)
		}
	}
	if strings.Contains(source, "state.ws = session?.ws || null;\n        renderTerminal();") {
		t.Fatalf("Code Studio terminal switch must not rerender after mounting an existing xterm session")
	}
}

func TestDesktopCodeStudioOpenAppReusesWindowAndSanitizesLaunchPath(t *testing.T) {
	t.Parallel()

	runtime := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, marker := range []string{
		"appId === 'code-studio' && context && context.path != null",
		"window.CodeStudio.openPath(context.path, true, existing.id)",
	} {
		if !strings.Contains(runtime, marker) {
			t.Fatalf("desktop runtime missing Code Studio reuse marker %q", marker)
		}
	}

	source := readDesktopAssetText(t, "js/desktop/apps/code-studio.js")
	for _, marker := range []string{
		"if (/^[a-zA-Z]:\\//.test(value) || value.startsWith('~')) return WORKSPACE_ROOT;",
		"lower.startsWith('/home/')",
		"if (value === 'workspace' || lower.startsWith('workspace/'))",
		"return '/' + parts.join('/');",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("Code Studio path sanitizer missing marker %q", marker)
		}
	}
}

func TestCodeStudioScriptsUseBuildVersionCacheBusting(t *testing.T) {
	t.Parallel()

	desktopHTML := rawDesktopAssetText(t, "desktop.html")
	if !strings.Contains(desktopHTML, `{{.TemplateDataJSON}}`) ||
		!strings.Contains(desktopHTML, `/js/shared/template-data.js?v={{.BuildVersion}}`) {
		t.Fatalf("desktop BUILD_VERSION must be exported through template-data for deferred module loaders")
	}
	if strings.Contains(desktopHTML, `<script defer src="/js/desktop/apps/code-studio.js`) {
		t.Fatalf("Code Studio app script must be lazy-loaded instead of upfront in desktop.html")
	}

	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, marker := range []string{
		"/js/desktop/bundles/code-studio.bundle.js",
		"window.BUILD_VERSION || 'dev'",
		"function versionedURL(url)",
	} {
		if !strings.Contains(loader, marker) {
			t.Fatalf("Code Studio lazy loader missing cache-busting marker %q", marker)
		}
	}

	source := rawDesktopAssetText(t, "js/desktop/apps/code-studio.js")
	for _, marker := range []string{
		"loadBundle('code-studio', '/js/desktop/bundles/code-studio.bundle.js')",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("Code Studio loader missing bundle marker %q", marker)
		}
	}
	if strings.Contains(source, "?v=1") {
		t.Fatalf("Code Studio loader must not pin module parts to a stale cache version")
	}

	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, marker := range []string{
		"function versionedIconAssetPath(path)",
		"var v = window.BUILD_VERSION || 'dev';",
		"encodeURIComponent(v)",
		"function iconUrlStyle(path) { return 'url(' + versionedIconAssetPath(path) + ')'; }",
	} {
		if !strings.Contains(foundation, marker) {
			t.Fatalf("desktop theme icon cache-busting missing marker %q", marker)
		}
	}
}

func TestCodeStudioSmallControlIconsStaySymbolic(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"img/papirus/icons/terminal.svg",
		"img/papirus/icons/chat.svg",
		"img/whitesur/icons/terminal.svg",
		"img/whitesur/icons/chat.svg",
	} {
		svg := rawDesktopAssetText(t, path)
		for _, marker := range []string{`width="24"`, `height="24"`, "currentColor"} {
			if !strings.Contains(svg, marker) {
				t.Fatalf("%s must stay a compact symbolic icon, missing %q", path, marker)
			}
		}
		for _, forbidden := range []string{"<image", "base64", `width="64"`, `height="64"`} {
			if strings.Contains(svg, forbidden) {
				t.Fatalf("%s must not use raster or full-size app icon artwork, found %q", path, forbidden)
			}
		}
	}
}
