package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestChatFrontend_GalaxyFrameStaysSymmetric pins the cockpit layout of the Galaxy
// chat theme: header and footer span the full width with equal gaps, navigation and
// status rails mirror each other, the clock is a header segment, and the decorative
// mottos and horizontal welcome offset stay retired.
func TestChatFrontend_GalaxyFrameStaysSymmetric(t *testing.T) {
	t.Parallel()

	css := readChatThemeSectionCSS(t, "chat-galaxy.css")
	scriptContent, err := os.ReadFile(filepath.Join("js", "chat", "galaxy-interface.js"))
	if err != nil {
		t.Fatalf("read galaxy-interface.js: %v", err)
	}
	script := string(scriptContent)

	for _, marker := range []string{
		`--galaxy-gap: 16px;`,
		`--galaxy-lane-inset: calc(var(--galaxy-gap) * 2 + var(--galaxy-rail-width));`,
		`margin: var(--galaxy-gap) var(--galaxy-gap) 0;`,
		`margin: 0 var(--galaxy-gap) var(--galaxy-gap);`,
		`[data-theme="galaxy"] .galaxy-nav,` + "\n" + `[data-theme="galaxy"] .galaxy-status {`,
		`[data-theme="galaxy"] .galaxy-nav { left: var(--galaxy-gap); }`,
		`[data-theme="galaxy"] .galaxy-status { right: var(--galaxy-gap); }`,
		`[data-theme="galaxy"] .app-header .galaxy-clock {`,
		`[data-theme="galaxy"] .galaxy-status .pill {`,
		`[data-theme="galaxy"] #chat-box { position: relative; flex: 1; min-height: 0; z-index: 3; background: transparent; margin: 0 var(--galaxy-lane-inset);`,
		`[data-theme="galaxy"] .galaxy-welcome { position: relative; display: flex; flex-direction: column; align-items: center;`,
		`[data-theme="galaxy"] .header-actions::before { display: none; }`,
		`[data-theme="galaxy"] .galaxy-status { display: none; }`,
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("chat-galaxy.css section missing frame marker %q", marker)
		}
	}
	for _, retired := range []string{"galaxy-motto", "translateX(-24px)", "--galaxy-bar-right", "35px 19px 24px 30px"} {
		if strings.Contains(css, retired) {
			t.Fatalf("chat-galaxy.css section still carries the retired layout marker %q", retired)
		}
	}
	if count := strings.Count(css, `position: fixed; top: calc(var(--galaxy-gap) * 2 + var(--galaxy-bar-height)); bottom: calc(var(--galaxy-gap) * 2 + var(--galaxy-bar-height));`); count != 1 {
		t.Fatalf("rails must share one fixed vertical extent between the bars, found %d", count)
	}

	for _, marker := range []string{
		`own(element('aside', 'galaxy-status'), document.body)`,
		`for (const id of ['connectionPill', 'tokenCounter', 'budgetPill', 'creditsPill', 'debug-pill']) move(byId(id), status);`,
		`own(element('div', 'galaxy-clock'), document.querySelector('.app-header'))`,
		`caption(byId('integrations-toggle-btn'), 'chat.integrations_title');`,
		`caption(byId('session-toggle-btn'), 'chat.sessions_title');`,
		`headerObserver.observe(byId('connectionPill'),`,
		`.galaxy-status [data-chat-icon]'`,
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("js/chat/galaxy-interface.js missing frame marker %q", marker)
		}
	}
	if strings.Contains(script, "galaxy-motto") {
		t.Fatal("js/chat/galaxy-interface.js must not create the retired mottos")
	}
}
