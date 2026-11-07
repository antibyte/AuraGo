package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestThemeOpticsRemainsWired pins the shared theme optics contract: one
// WebGL engine that serves the chat page and the virtual desktop agent chat,
// a recipe per theme, and the layering rules that keep the light behind the
// messages.
//
// The engine is shared on purpose. A theme needs a recipe, not a new shader:
// a per-theme shader would make every theme change a new download and every
// theme a second WebGL context.
func TestThemeOpticsRemainsWired(t *testing.T) {
	t.Parallel()

	core := readEmbeddedText(t, "js/shared/theme-optics-core.js")

	for _, marker := range []string{
		"window.AuraThemeOptics",
		"createRenderer",
		"const OPTICS = {",
		"aurora:",
		"caustic:",
		"lattice:",
		"plasma:",
		"motes:",
		"facet:",
		"rain:",
		"ember:",
		// The engine must survive a missing WebGL context.
		"if (!gl) {",
		"canvas.remove();",
		// Reduced motion: one still frame instead of a moving loop.
		"prefers-reduced-motion: reduce",
		"renderStatic();",
		// A hidden tab must not burn frames.
		"visibilitychange",
		"setPaused(document.hidden)",
		// Cleanup must release the context and the canvas.
		"function dispose()",
		"gl.deleteProgram(program)",
		"loseContext()",
		"canvas.parentNode.removeChild(canvas)",
		// Cleanup must also release the listeners it registered.
		"resizeObserver.disconnect()",
		"removeEventListener('visibilitychange', onVisibility)",
		// The palette arrives as three vec3 uniforms: ESSL 1.00 cannot mix a
		// float with a vec3, so a single vec3 palette would not compile.
		"uniform vec3 u_color_a;",
		"uniform vec3 u_color_b;",
		"uniform vec3 u_color_c;",
		// Uniform locations belong to their program, so the cache is dropped
		// when a new program is linked.
		"uniforms = {};",
	} {
		if !strings.Contains(core, marker) {
			t.Fatalf("js/shared/theme-optics-core.js missing marker %q", marker)
		}
	}

	// The engine must not hard-code a theme: it renders what the recipe says.
	for _, forbidden := range []string{
		"data-theme=\"",
		"data-theme='",
	} {
		if strings.Contains(core, forbidden) {
			t.Fatalf("theme optics core must stay theme agnostic, found %q", forbidden)
		}
	}
}

// TestChatThemeOpticsCoversEveryChatTheme checks that every chat theme in the
// picker has an optical recipe, and that the recipes name an optics the engine
// actually implements.
func TestChatThemeOpticsCoversEveryChatTheme(t *testing.T) {
	t.Parallel()

	recipes := readEmbeddedText(t, "js/chat/theme-optics-recipes.js")
	core := readEmbeddedText(t, "js/shared/theme-optics-core.js")

	themeIDs := chatThemeIDsFromDefinitions(t)
	for _, theme := range themeIDs {
		// The Standard theme deliberately stays plain, so it has no recipe.
		if theme == "dark" {
			continue
		}
		if !strings.Contains(recipes, "'"+theme+"': {") {
			t.Errorf("chat theme %q has no optics recipe", theme)
		}
	}

	// The Standard theme promises a plain look; it must not get a backdrop.
	if strings.Contains(recipes, "'dark': {") {
		t.Error("the Standard chat theme must stay free of an optical backdrop")
	}

	// Every recipe must name an optics the engine implements.
	optics := map[string]bool{}
	for _, name := range []string{"aurora", "caustic", "lattice", "plasma", "motes", "facet", "rain", "ember"} {
		if !strings.Contains(core, name+": `") {
			t.Errorf("theme optics core does not implement the %q optics", name)
		}
		optics[name] = true
	}

	for _, line := range strings.Split(recipes, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "optics: '") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(trimmed, "optics: '"), "',")
		if !optics[name] {
			t.Errorf("chat optics recipe uses the unknown optics %q", name)
		}
	}

	// Every recipe must carry a palette, a strength, a speed and a blend mode.
	for _, marker := range []string{
		"palette: { primary:",
		"intensity:",
		"speed:",
		"blend: 'screen'",
		"blend: 'multiply'",
	} {
		if !strings.Contains(recipes, marker) {
			t.Errorf("chat optics recipes missing marker %q", marker)
		}
	}
}

// TestThemeOpticsSurfacesStayBehindTheMessages pins the layering: the optics
// canvas is a backdrop, so it must never cover the text and never swallow a
// click.
func TestThemeOpticsSurfacesStayBehindTheMessages(t *testing.T) {
	t.Parallel()

	css := readEmbeddedText(t, "css/theme-optics.css")

	for _, marker := range []string{
		"#aurago-chat-optics {",
		".vd-chat-optics-canvas {",
		"pointer-events: none;",
		"z-index: 0;",
		// The layers fade in with the first painted frame.
		"#aurago-chat-optics.is-ready {",
		".vd-chat-optics-canvas.is-ready {",
		// The chat page and the desktop agent chat scroll, so the engine pins
		// the canvas to the scrollport.
		".vd-chat-log {",
		"position: relative;",
		// Reduced motion must not animate the fade either.
		"@media (prefers-reduced-motion: reduce)",
	} {
		if !strings.Contains(css, marker) {
			t.Errorf("css/theme-optics.css missing marker %q", marker)
		}
	}

	// The canvas must not sit above the conversation.
	for _, forbidden := range []string{
		"z-index: 9999",
		"z-index: 1200",
	} {
		if strings.Contains(css, forbidden) {
			t.Errorf("theme optics layer must stay behind the messages, found %q", forbidden)
		}
	}
}

// TestDesktopAgentChatOpticsRemainsWired checks the virtual desktop side: the
// agent chat window mounts the shared engine, follows the desktop theme and
// cleans the layer up with the window.
func TestDesktopAgentChatOpticsRemainsWired(t *testing.T) {
	t.Parallel()

	controller := readEmbeddedText(t, "js/desktop/apps/agent-chat-optics.js")
	recipes := readEmbeddedText(t, "js/desktop/apps/agent-chat-optics-recipes.js")
	app := readEmbeddedText(t, "js/desktop/apps/agent-chat.js")
	loader := readEmbeddedText(t, "js/desktop/core/module-loader.js")

	for _, marker := range []string{
		"window.AuraDesktopChatOptics",
		"function mount(host)",
		"createRenderer",
		"body.dataset.theme",
		"MutationObserver",
		"attributeFilter: ['data-theme']",
		"dispose() {",
		"renderer.dispose()",
	} {
		if !strings.Contains(controller, marker) {
			t.Errorf("js/desktop/apps/agent-chat-optics.js missing marker %q", marker)
		}
	}

	// Every desktop theme must have a recipe.
	for _, theme := range desktopThemeIDsFromRegistry(t) {
		if !strings.Contains(recipes, theme+": {") {
			t.Errorf("desktop theme %q has no agent chat optics recipe", theme)
		}
	}

	// The window must mount the layer and tear it down again.
	for _, marker := range []string{
		"host._desktopChatOptics = window.AuraDesktopChatOptics.mount(host)",
		"host._desktopChatOptics.dispose()",
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("js/desktop/apps/agent-chat.js missing marker %q", marker)
		}
	}

	// The engine and the recipes must ship with the agent chat app.
	for _, marker := range []string{
		"/js/shared/theme-optics-core.js",
		"/js/desktop/apps/agent-chat-optics-recipes.js",
		"/js/desktop/apps/agent-chat-optics.js",
		"/css/theme-optics.css",
	} {
		if !strings.Contains(loader, marker) {
			t.Errorf("desktop module loader missing optics marker %q", marker)
		}
	}
}

// TestChatThemeOpticsAreLoadedLazily checks that the optics load through the
// theme effect loader instead of blocking the first paint of the chat page.
func TestChatThemeOpticsAreLoadedLazily(t *testing.T) {
	t.Parallel()

	themeEffects := readEmbeddedText(t, "js/chat/theme-effects.js")
	html := readEmbeddedText(t, "index.html")

	for _, marker := range []string{
		"const OPTICS_SCRIPTS = [",
		"/js/shared/theme-optics-core.js",
		"/js/chat/theme-optics-recipes.js",
		"/js/chat/theme-optics.js",
		"function ensureOptics()",
		"window.AuraChatThemeEffects = { ensure, ensureOptics }",
	} {
		if !strings.Contains(themeEffects, marker) {
			t.Errorf("js/chat/theme-effects.js missing marker %q", marker)
		}
	}

	// The chat page must not pull the optics in with a blocking script tag.
	for _, forbidden := range []string{
		`src="/js/shared/theme-optics-core.js"`,
		`src="/js/chat/theme-optics.js"`,
	} {
		if strings.Contains(html, forbidden) {
			t.Errorf("chat page must lazy-load the theme optics instead of %q", forbidden)
		}
	}
}

// TestChatThemeOpticsStayOffInTheStandardTheme pins the plain contract of the
// Standard theme: no backdrop at all. A hidden canvas would still be a canvas,
// so the controller has to take the renderer down instead of dimming it.
func TestChatThemeOpticsStayOffInTheStandardTheme(t *testing.T) {
	t.Parallel()

	controller := readEmbeddedText(t, "js/chat/theme-optics.js")

	for _, marker := range []string{
		"if (!recipe) {",
		"renderer.dispose();",
		"renderer = null;",
	} {
		if !strings.Contains(controller, marker) {
			t.Errorf("js/chat/theme-optics.js missing marker %q", marker)
		}
	}

	// The controller must not paint a hidden layer for a theme without a recipe.
	if strings.Contains(controller, "instance.applyRecipe(null)") {
		t.Error("the Standard chat theme must not keep a hidden optics canvas around")
	}
}

// chatThemeIDsFromDefinitions reads the chat theme ids the picker offers.
func chatThemeIDsFromDefinitions(t *testing.T) []string {
	t.Helper()

	source := readEmbeddedText(t, "js/shared/shared-chat.js")
	var ids []string
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "{ theme: '") {
			continue
		}
		rest := strings.TrimPrefix(trimmed, "{ theme: '")
		idx := strings.Index(rest, "'")
		if idx <= 0 {
			continue
		}
		ids = append(ids, rest[:idx])
	}
	if len(ids) < 10 {
		t.Fatalf("expected the chat theme definitions, got %v", ids)
	}
	return ids
}

// desktopThemeIDsFromRegistry reads the desktop theme ids from the shell
// registry, so the recipe table and the registry cannot drift apart.
func desktopThemeIDsFromRegistry(t *testing.T) []string {
	t.Helper()

	source := readEmbeddedText(t, "js/desktop/core/theme-runtime.js")
	var ids []string
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "{ id: '") {
			continue
		}
		rest := trimmed[strings.Index(trimmed, "{ id: '")+len("{ id: '"):]
		idx := strings.Index(rest, "'")
		if idx <= 0 {
			continue
		}
		ids = append(ids, rest[:idx])
	}
	if len(ids) < 10 {
		t.Fatalf("expected the desktop theme registry, got %v", ids)
	}
	return ids
}

// TestThemeOpticsFilesAreShipped makes sure the new assets end up in the
// production asset set and in the bundles that need them.
func TestThemeOpticsFilesAreShipped(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"js/shared/theme-optics-core.js",
		"js/chat/theme-optics-recipes.js",
		"js/chat/theme-optics.js",
		"js/desktop/apps/agent-chat-optics-recipes.js",
		"js/desktop/apps/agent-chat-optics.js",
		"css/theme-optics.css",
	} {
		if _, err := Content.ReadFile(path); err != nil {
			t.Errorf("theme optics asset %s is not shipped: %v", path, err)
		}
	}

	// The chat bundle must carry the optics stylesheet and keep the Standard
	// theme last, because it has to win over the other chat styles.
	bundle := readEmbeddedText(t, "css/chat.bundle.css")
	opticsAt := strings.Index(bundle, "/* ui/css/theme-optics.css */")
	standardAt := strings.Index(bundle, "/* ui/css/chat-standard.css */")
	if opticsAt < 0 {
		t.Fatal("chat.bundle.css is missing the theme optics stylesheet")
	}
	if standardAt < opticsAt {
		t.Error("chat-standard.css must stay the last chat bundle part")
	}

	// The chat runtime bundle must ship the shared engine with the chat page.
	runtime := readEmbeddedText(t, "js/chat/bundles/chat-runtime.bundle.js")
	for _, marker := range []string{
		"/* ui/js/chat/theme-effects.js */",
	} {
		if !strings.Contains(runtime, marker) {
			t.Errorf("chat runtime bundle missing marker %q", marker)
		}
	}
}

// TestThemeOpticsEngineSurvivesWithoutWebGL is a static guard on the engine's
// defensive path: a missing context must never leave a broken canvas behind.
func TestThemeOpticsEngineSurvivesWithoutWebGL(t *testing.T) {
	t.Parallel()

	path := filepath.Join("js", "shared", "theme-optics-core.js")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	source := string(content)

	for _, marker := range []string{
		"try {",
		"} catch (_) {",
		"gl = null;",
		"if (!gl) {",
		"console.warn('[AuraGo] Theme optics program failed'",
	} {
		if !strings.Contains(source, marker) {
			t.Errorf("theme optics engine missing defensive marker %q", marker)
		}
	}
}
