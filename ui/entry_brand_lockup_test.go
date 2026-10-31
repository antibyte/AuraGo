package ui

import (
	"strings"
	"testing"
)

// TestEntryPagesUseSharedBrandLockup pins that login, setup and 404 show the
// same robot icon + two-tone wordmark as the workspace header, without growing
// the workspace chrome (header, density toggle, radial anchor).
func TestEntryPagesUseSharedBrandLockup(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"login.html", "setup.html", "404.html"} {
		file := file
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			page := renderUIPage(t, file)
			for _, want := range []string{
				`<span class="logo-icon"><img class="aurago-brand-icon" src="/img/chat-ui-icons/robot.png?v=test"`,
				`<span class="logo-wordmark-accent">AURA</span>`,
				`<span class="logo-wordmark-base">GO</span>`,
			} {
				if !strings.Contains(page, want) {
					t.Errorf("%s missing brand lockup part %q", file, want)
				}
			}
			for _, banned := range []string{
				`<div class="logo-icon">`,
				`login-logo-icon`,
				`>AURAGO<`,
				`data-app-header`,
				`class="app-header"`,
				`id="radialMenuAnchor"`,
				`pw-density-toggle`,
				`data-pw-density-toggle`,
			} {
				if strings.Contains(page, banned) {
					t.Errorf("%s must not contain %q", file, banned)
				}
			}
		})
	}
}
