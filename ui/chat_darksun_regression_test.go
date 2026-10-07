package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestChatFrontend_DarkSunSceneStaysPolished pins the static contract of the
// Dark Sun chat theme: the eclipse scene stack in css/chat-themes.css, the
// eruption state hooks shared between the engine and the stylesheet, and the
// resource budgets and lifecycle gates of js/chat/dark-sun-shader.js.
func TestChatFrontend_DarkSunSceneStaysPolished(t *testing.T) {
	t.Parallel()

	css := readChatThemeSectionCSS(t, "chat-dark-sun.css")
	engineContent, err := os.ReadFile(filepath.Join("js", "chat", "dark-sun-shader.js"))
	if err != nil {
		t.Fatalf("read dark-sun-shader.js: %v", err)
	}
	engine := string(engineContent)

	for _, marker := range []string{
		// scene stack: static backdrop, the three canvas layers and the chat lane
		`[data-theme="dark-sun"] body::before {`,
		`--ds-horizon:`,
		`html[data-darksun][data-theme="dark-sun"] body::before {`,
		`[data-theme="dark-sun"] #dark-sun-sky,`,
		`[data-theme="dark-sun"] #dark-sun-scene {`,
		`[data-theme="dark-sun"] #dark-sun-overlay {`,
		`mix-blend-mode: screen !important;`,
		`[data-theme="dark-sun"] #chat-box {`,
		`background: transparent;`,
		// eruption reaction and the flash variable
		`--darksun-flash: 0;`,
		`html[data-darksun="flare"][data-theme="dark-sun"] #chat-box::before {`,
		`html[data-darksun="flare"][data-theme="dark-sun"] .app-header::after {`,
		`html[data-darksun="flare"][data-theme="dark-sun"] .app-footer {`,
		`var(--darksun-flash, 0)`,
		// polish layers
		`[data-theme="dark-sun"] .greeting-text {`,
		`font-family: var(--darksun-font-ui);`,
		`[data-theme="dark-sun"] .btn-send::before {`,
		`[data-theme="dark-sun"] .status-pill::before {`,
		`[data-theme="dark-sun"] .typing-dots span:nth-child(3) {`,
		`[data-theme="dark-sun"] .message-stack::after {`,
		`[data-theme="dark-sun"] .avatar::after,`,
		`outline: 2px solid var(--ds-rim-strong);`,
		`@media (max-width: 767px) {`,
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("chat-dark-sun.css section missing scene marker %q", marker)
		}
	}
	for _, legacy := range []string{"#dark-sun-ember-layer", ".dark-sun-ash", ".dark-sun-ember", "darkSunAshFall", "darkSunEmberRise"} {
		if strings.Contains(css, legacy) {
			t.Fatalf("chat-dark-sun.css section still carries the retired DOM ember layer marker %q", legacy)
		}
	}

	keyframes := []string{
		"darksunHazeDrift", "darksunRimIgnite", "darksunMoltenSheen", "darksunIgnite", "darksunEmberWipe",
		"darksunDustIgnite", "darksunDrawerIgnite", "darksunCoronaBreath", "darksunCoronaPulse",
		"darksunEmberGlow", "darksunCoronaSpin", "darksunBeacon",
	}
	for _, name := range keyframes {
		if !strings.Contains(css, "@keyframes "+name+" {") {
			t.Fatalf("chat-dark-sun.css section missing keyframes %q", name)
		}
		if !regexp.MustCompile(`animation(?:-name)?:[^;]*\b` + name + `\b`).MatchString(css) {
			t.Fatalf("keyframes %q are defined but never used", name)
		}
	}

	reducedStart := strings.Index(css, "@media (prefers-reduced-motion: reduce) {")
	if reducedStart < 0 {
		t.Fatal("chat-dark-sun.css section must guard its animations for reduced motion")
	}
	reduced := css[reducedStart:]
	if end := strings.Index(reduced, "\n}\n"); end >= 0 {
		reduced = reduced[:end]
	}
	for _, selector := range []string{
		".msg-row", ".message-stack::after", ".app-header::after", ".logo .logo-wordmark-accent",
		".status-pill::before", ".typing-dots span", ".btn-send::before", ".avatar::after",
		".greeting-icon::after", ".session-drawer.open", ".chat-theme-dropdown:not([hidden])",
		".scroll-to-bottom-btn.has-new", "#chat-box::before",
	} {
		if !strings.Contains(reduced, `[data-theme="dark-sun"] `+selector) {
			t.Fatalf("reduced-motion guard must cover %q", selector)
		}
	}
	if !strings.Contains(reduced, "animation: none !important;") {
		t.Fatal("reduced-motion guard must disable the theme animations")
	}

	for _, marker := range []string{
		// fixed pools and the sky budget
		"const MAX_EMBERS = 160;",
		"const MAX_SPARKS = 240;",
		"const SKY_MAX_W = 1280;",
		"const SKY_MAX_H = 720;",
		"Math.min(0.75, SKY_MAX_W / sw, SKY_MAX_H / sh)",
		// scene layers
		"makeLayer('dark-sun-sky', 0, 'normal')",
		"makeLayer('dark-sun-scene', 0, 'normal')",
		"makeLayer('dark-sun-overlay', 2, 'screen')",
		"function buildPlain(width, height, dpr)",
		"function drawPlain(width, height)",
		"function drawEclipse2D(width, height)",
		// corona, eruption and the light wave in the shader
		"uniform vec2 u_sun;",
		"uniform float u_flare;",
		"uniform float u_flareAngle;",
		"uniform float u_wave;",
		"uniform float u_glint;",
		"uniform float u_flash;",
		"float prominence =",
		"float glintCore =",
		// eruption cycle, scene state and the CSS hooks
		"function updateFlare(dt, time)",
		"const FLARE_DURATION = 5000;",
		"'--darksun-flash'",
		"setSceneState('flare')",
		"setSceneState('calm')",
		"setSceneState(null)",
		"root.removeAttribute('data-darksun')",
		// embers, sparks on bubble contact and pointer heat
		"function spawnSparks(x, y, count)",
		"function updateBubbleBounds(time)",
		"chatBox.addEventListener('pointermove', onPointerMove, { passive: true })",
		"const HEAT_RADIUS = 90;",
		"globalCompositeOperation = 'lighter'",
		// lifecycle gates
		"!document.hidden &&",
		"!prefersReducedMotion() &&",
		"window.innerWidth >= 768",
		"document.addEventListener('visibilitychange'",
		"window.AuraGoDarkSun = { start, stop, sync };",
	} {
		if !strings.Contains(engine, marker) {
			t.Fatalf("js/chat/dark-sun-shader.js missing scene marker %q", marker)
		}
	}

	if strings.Count(engine, "requestAnimationFrame(render)") != 2 {
		t.Fatal("the engine must keep a single animation loop (start + render continuation)")
	}

	themeEffects := readDesktopAssetText(t, "js/chat/theme-effects.js")
	if !strings.Contains(themeEffects, `scripts: ['/js/chat/dark-sun-shader.js']`) {
		t.Fatal("theme effect loader must lazy-load only the dark-sun engine")
	}
	if strings.Contains(themeEffects, "dark-sun-embers") {
		t.Fatal("theme effect loader must not reference the retired ember layer")
	}
	if _, err := Content.ReadFile("js/chat/dark-sun-embers.js"); err == nil {
		t.Fatal("the retired DOM ember layer js/chat/dark-sun-embers.js should be removed")
	}
}
