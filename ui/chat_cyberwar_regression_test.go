package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestChatFrontend_CyberwarPolishStaysWired pins the static contract of the
// Cyberwar chat theme polish pass: the fine details and effects in the
// css/chat-themes.css cyberwar section, the threat escalation wiring in
// js/chat/cyberwar-hud.js and the alert tint of the radar shader.
func TestChatFrontend_CyberwarPolishStaysWired(t *testing.T) {
	t.Parallel()

	css := readChatThemeSectionCSS(t, "chat-cyberwar.css")

	hudContent, err := os.ReadFile(filepath.Join("js", "chat", "cyberwar-hud.js"))
	if err != nil {
		t.Fatalf("read cyberwar-hud.js: %v", err)
	}
	hud := string(hudContent)

	shaderContent, err := os.ReadFile(filepath.Join("js", "chat", "cyberwar-shader.js"))
	if err != nil {
		t.Fatalf("read cyberwar-shader.js: %v", err)
	}
	shader := string(shaderContent)

	for _, marker := range []string{
		// selection, caret and focus ring
		`[data-theme="cyberwar"] ::selection,`,
		`caret-color: var(--accent);`,
		`[data-theme="cyberwar"] .chat-theme-option:focus-visible,`,
		`outline: 2px solid var(--accent);`,
		// transcript typography
		`[data-theme="cyberwar"] .bubble h1,`,
		`[data-theme="cyberwar"] .bubble a {`,
		`[data-theme="cyberwar"] .bubble blockquote {`,
		`[data-theme="cyberwar"] .bubble hr {`,
		`[data-theme="cyberwar"] .bubble table {`,
		// bubble labels must not inherit the speech-bubble tail geometry
		`content: '[SECURE INTELLIGENCE TRANSCRIPT]';`,
		`content: '[TRANSMITTING ENCRYPTED LINK]';`,
		`position: static;`,
		// HUD boot sequence and threat escalation
		`#cyberwar-hud.hud-active:not(.hud-reduced-motion) {`,
		`#cyberwar-hud-left.hud-active:not(.hud-reduced-motion) {`,
		`#cyberwar-hud .hud-panel:nth-child(1),`,
		`.hud-threat-level.hud-threat-flash {`,
		`.hud-panel.threat-level.hud-threat-critical::after {`,
		`.hud-threat-level.threat-red {`,
		`.hud-blist-item.hud-blist-error .hud-blist-dot {`,
		`.hud-blist-ring {`,
		// menus, composer, header scan and mascot
		`[data-theme="cyberwar"] .chat-theme-option.active::before,`,
		`[data-theme="cyberwar"] .personality-option.active::before {`,
		`[data-theme="cyberwar"] .btn-send:active::after {`,
		`[data-theme="cyberwar"] .app-header::before {`,
		`[data-theme="cyberwar"] #chat-robot-mascot {`,
		`[data-theme="cyberwar"] .avatar::before {`,
		// reduced motion guard
		`@media (prefers-reduced-motion: reduce) {`,
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("chat-cyberwar.css section missing polish marker %q", marker)
		}
	}

	keyframes := []string{
		"cyberwarHudBootRight", "cyberwarHudBootLeft", "cyberwarPanelBoot",
		"cyberwarThreatFlash", "cyberwarThreatBreathe", "cyberwarBlipRing",
		"cyberwarMarkerIn", "cyberwarUplinkPulse", "cyberwarHeaderScan", "cyberwarReticlePulse",
	}
	for _, name := range keyframes {
		if !strings.Contains(css, "@keyframes "+name+" {") {
			t.Fatalf("chat-cyberwar.css section missing keyframes %q", name)
		}
		if !regexp.MustCompile(`animation(?:-name)?:[^;]*\b` + name + `\b`).MatchString(css) {
			t.Fatalf("keyframes %q are defined but never used", name)
		}
	}

	for _, marker := range []string{
		"function flashThreatLevel() {",
		"threatLevelEl.classList.add('hud-threat-flash');",
		"threatPanelEl.classList.toggle('hud-threat-critical', threatLevel >= 4);",
		"ring.className = 'hud-blist-ring';",
		"if (event.target !== blip) return;",
		"function publishThreatLevel() {",
		"window.AuraGoCyberwarThreat = threatLevel;",
	} {
		if !strings.Contains(hud, marker) {
			t.Fatalf("cyberwar-hud.js missing threat escalation marker %q", marker)
		}
	}

	for _, marker := range []string{
		"uniform float u_threat;",
		"float threatMix = clamp(u_threat, 0.0, 1.0);",
		"gl.uniform1f(uniforms.threat, currentThreat());",
		"function setThreat(level) {",
		"window.AuraGoCyberwar = { start, stop, sync, setThreat };",
	} {
		if !strings.Contains(shader, marker) {
			t.Fatalf("cyberwar-shader.js missing alert tint marker %q", marker)
		}
	}
}
