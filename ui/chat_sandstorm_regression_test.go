package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestChatFrontend_SandstormSceneStaysPolished pins the static contract of the
// Sandstorm chat theme: the scene stack in css/chat-themes.css, the weather
// state hooks shared between the engine and the stylesheet, and the resource
// budgets and lifecycle gates of js/chat/sandstorm-particles.js.
func TestChatFrontend_SandstormSceneStaysPolished(t *testing.T) {
	t.Parallel()

	css := readChatThemeSectionCSS(t, "chat-sandstorm.css")
	engineContent, err := os.ReadFile(filepath.Join("js", "chat", "sandstorm-particles.js"))
	if err != nil {
		t.Fatalf("read sandstorm-particles.js: %v", err)
	}
	engine := string(engineContent)

	for _, marker := range []string{
		// scene stack: the static backdrop, the three canvas layers and the chat lane
		`[data-theme="sandstorm"] body::before {`,
		`--sand-dune-near:`,
		`--sand-dune-mid:`,
		`--sand-dune-far:`,
		`html[data-sandstorm][data-theme="sandstorm"] body::before {`,
		`[data-theme="sandstorm"] #sandstorm-fog,`,
		`[data-theme="sandstorm"] #sandstorm-scene {`,
		`[data-theme="sandstorm"] #sandstorm-overlay {`,
		`[data-theme="sandstorm"] #chat-box {`,
		`background: transparent;`,
		// weather reaction and dry lightning
		`--sandstorm-flash: 0;`,
		`html[data-sandstorm="storm"][data-theme="sandstorm"] #chat-box::before {`,
		`html[data-sandstorm="storm"][data-theme="sandstorm"] .app-header::after {`,
		`html[data-sandstorm="storm"][data-theme="sandstorm"] .app-footer {`,
		`var(--sandstorm-flash, 0)`,
		// polish layers
		`[data-theme="sandstorm"] .greeting-text {`,
		`font-family: var(--sand-font-journal);`,
		`[data-theme="sandstorm"] .btn-send::before {`,
		`[data-theme="sandstorm"] .status-pill {`,
		`[data-theme="sandstorm"] .typing-dots span:nth-child(3) {`,
		`[data-theme="sandstorm"] .message-stack::after {`,
		`[data-theme="sandstorm"] .avatar::after {`,
		`outline: 2px solid var(--sand-rim-strong);`,
		`@media (max-width: 767px) {`,
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("chat-sandstorm.css section missing scene marker %q", marker)
		}
	}

	keyframes := []string{
		"sandstormDuneHaze", "sandstormSunGlint", "sandstormSheen", "sandstormMaterialize",
		"sandstormDustWipe", "sandstormDustSweep", "sandstormDrawerDust", "sandstormHaloBreath",
		"sandstormMirage", "sandstormGrainBounce", "sandstormSunRays", "sandstormBeacon",
	}
	for _, name := range keyframes {
		if !strings.Contains(css, "@keyframes "+name+" {") {
			t.Fatalf("chat-sandstorm.css section missing keyframes %q", name)
		}
		if !regexp.MustCompile(`animation(?:-name)?:[^;]*\b` + name + `\b`).MatchString(css) {
			t.Fatalf("keyframes %q are defined but never used", name)
		}
	}

	reducedStart := strings.Index(css, "@media (prefers-reduced-motion: reduce) {")
	if reducedStart < 0 {
		t.Fatal("chat-sandstorm.css section must guard its animations for reduced motion")
	}
	reduced := css[reducedStart:]
	if end := strings.Index(reduced, "\n}\n"); end >= 0 {
		reduced = reduced[:end]
	}
	for _, selector := range []string{
		".msg-row", ".message-stack::after", ".app-header::after", ".logo .logo-wordmark-accent",
		".status-pill", ".typing-dots span", ".btn-send::before", ".avatar::after",
		".greeting-icon::after", ".session-drawer.open", ".chat-theme-dropdown:not([hidden])",
		".scroll-to-bottom-btn.has-new", "#chat-box::before",
	} {
		if !strings.Contains(reduced, `[data-theme="sandstorm"] `+selector) {
			t.Fatalf("reduced-motion guard must cover %q", selector)
		}
	}
	if !strings.Contains(reduced, "animation: none !important;") {
		t.Fatal("reduced-motion guard must disable the theme animations")
	}

	for _, marker := range []string{
		// fixed pools and the fog budget
		"const MAX_FLYING = 320;",
		"const MAX_GROUND = 2500;",
		"const MAX_TRAILS = 35;",
		"const MAX_CLOUDS = 5;",
		"Math.min(0.65, 960 / fw, 540 / fh)",
		"uniform vec4 u_eddies[3];",
		// scene layers
		"makeLayer('sandstorm-fog', 0)",
		"makeLayer('sandstorm-scene', 0)",
		"makeLayer('sandstorm-overlay', 2)",
		"function buildDunes(width, height, dpr)",
		"function drawDunes(width, height)",
		"function drawSky2D(width, height)",
		// sun, dust wall and lightning in the shader
		"uniform vec2 u_sun;",
		"uniform vec2 u_front;",
		"uniform float u_flash;",
		"uniform float u_glow;",
		"float wallRim =",
		// dry lightning, weather state and the CSS hooks
		"function scheduleLightning(stormStart)",
		"function makeBolt(width, height)",
		"function drawLightning(width, height)",
		"'--sandstorm-flash'",
		"setWeatherState('storm')",
		"setWeatherState('calm')",
		"setWeatherState(null)",
		"root.removeAttribute('data-sandstorm')",
		// pointer gust and storm erosion of bubble sand
		"chatBox.addEventListener('pointermove', onPointerMove, { passive: true })",
		"const POINTER_RADIUS = 110;",
		"function sweepBubbleSand(dt, height)",
		// lifecycle gates
		"!prefersReducedMotion() &&",
		"window.innerWidth >= 640",
		"document.addEventListener('visibilitychange'",
		"window.AuraGoSandstorm = { start, stop, sync };",
	} {
		if !strings.Contains(engine, marker) {
			t.Fatalf("js/chat/sandstorm-particles.js missing scene marker %q", marker)
		}
	}

	if strings.Count(engine, "requestAnimationFrame(render)") != 2 {
		t.Fatal("the engine must keep a single animation loop (start + render continuation)")
	}

	themeEffects := readDesktopAssetText(t, "js/chat/theme-effects.js")
	if !strings.Contains(themeEffects, `/js/chat/sandstorm-particles.js`) {
		t.Fatal("theme effect loader must lazy-load the sandstorm engine")
	}
}
