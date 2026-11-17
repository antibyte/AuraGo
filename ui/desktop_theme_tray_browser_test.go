package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopThemeTrayFeedbackBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "no-preference")
	page := s.page
	for _, combo := range themeSweepCombinations(t, parseDesktopThemeRegistry(t)) {
		t.Run(combo.label(), func(t *testing.T) {
			settings := map[string]string{"appearance.theme": combo.theme.ID, "windows.animations": "true"}
			if combo.theme.SchemeSetting != "" {
				settings[combo.theme.SchemeSetting] = combo.mode
			}
			s.store.set(settings)
			s.pushSettings(t)
			page.Timeout(5 * time.Second).MustElement("#vd-quick-settings-button").MustClick()
			s.waitFor(t, "open sliders and themed switches", 3*time.Second, `() => {
				const button = document.getElementById('vd-quick-settings-button');
				const [upper, lower] = button.querySelectorAll('circle');
				const toggle = document.querySelector('#vd-quick-settings input[data-quick-setting="windows.animations"]');
				return button.getAttribute('aria-expanded') === 'true' &&
					upper.getBoundingClientRect().left > lower.getBoundingClientRect().left &&
					toggle?.checked && getComputedStyle(toggle).appearance === 'none' &&
					new DOMMatrix(getComputedStyle(toggle, '::before').transform).m41 > 0;
			}`)
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				page.MustElement("#vd-quick-settings").MustScreenshot(filepath.Join(dir, "tray-"+combo.label()+".png"))
			}
			page.Timeout(5 * time.Second).MustElement("[data-quick-action=close]").MustClick()
			s.waitFor(t, "closed sliders and restored focus", 3*time.Second, `() => {
				const button = document.getElementById('vd-quick-settings-button');
				const [upper, lower] = button.querySelectorAll('circle');
				return document.activeElement === button && button.getAttribute('aria-expanded') === 'false' &&
					upper.getBoundingClientRect().left < lower.getBoundingClientRect().left;
			}`)
		})
	}

	const movingParts = `() => {
		const knob = document.querySelector('#vd-quick-settings-button circle');
		const search = document.querySelector('.vd-start-search');
		const pager = document.querySelector('.vd-space-pager-btn');
		const toggle = document.querySelector('.vd-quick-settings-toggle input');
		return [getComputedStyle(knob), getComputedStyle(search, '::after'),
			getComputedStyle(pager, '::after'), getComputedStyle(toggle, '::before')]
			.filter(style => parseFloat(style.transitionDuration) > .001).length;
	}`
	s.store.set(map[string]string{"appearance.theme": "standard", "windows.animations": "true"})
	s.pushSettings(t)
	if got := s.eval(t, movingParts).Value.Int(); got != 4 {
		t.Fatalf("enabled motion reached %d of the four shell controls", got)
	}
	s.store.set(map[string]string{"windows.animations": "false"})
	s.pushSettings(t)
	if got := s.eval(t, movingParts).Value.Int(); got != 0 {
		t.Fatalf("%d shell controls still animate with window animations off", got)
	}
	next := page.MustElement(`.vd-space-pager-btn[aria-pressed="false"]`)
	id := *next.MustAttribute("data-space-id")
	next.MustClick()
	s.waitFor(t, "workspace marker follows the active workspace", 3*time.Second, `id => {
		const active = document.querySelector('.vd-space-pager-btn[aria-pressed="true"]');
		return active.dataset.spaceId === id && new DOMMatrix(getComputedStyle(active, '::after').transform).a === 1;
	}`, id)
	page.MustElement("#vd-start-button").MustClick()
	search := page.MustElement("#vd-start-search")
	search.MustInput("calc")
	const searchMark = `width => new DOMMatrix(getComputedStyle(document.querySelector('.vd-start-search'), '::after').transform).a === width`
	s.waitFor(t, "search indicator to open", 3*time.Second, searchMark, 1)
	search.MustSelectAllText().MustType(input.Backspace)
	s.waitFor(t, "search indicator to clear", 3*time.Second, searchMark, 0)
	page.Keyboard.MustType(input.Escape)
	media := proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{
		{Name: "prefers-reduced-motion", Value: "reduce"}, {Name: "forced-colors", Value: "active"},
	}}
	if err := media.Call(page); err != nil {
		t.Fatal(err)
	}
	s.store.set(map[string]string{"windows.animations": "true"})
	s.pushSettings(t)
	page.Timeout(5 * time.Second).MustElement("#vd-quick-settings-button").MustClick()
	if s.eval(t, movingParts).Value.Int() != 0 || !s.eval(t, `() => {
		const input = document.querySelector('#vd-quick-settings input[type="checkbox"]');
		return getComputedStyle(input).appearance === 'auto' && input.labels.length > 0;
	}`).Value.Bool() {
		t.Fatal("reduced motion and forced colors must preserve static feedback and native labeled checkboxes")
	}
}
