package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopThemeWindowDetailsBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "no-preference")
	s.store.set(map[string]string{"appearance.dock_pins": `["calculator","files"]`})
	s.pushSettings(t)
	s.openApps(t, "files", "calculator")
	s.eval(t, `() => auditTest.openApp('calculator', {forceNew:true}) && true`)
	page := s.page
	for _, combo := range themeSweepCombinations(t, parseDesktopThemeRegistry(t)) {
		t.Run(combo.label(), func(t *testing.T) {
			settings := map[string]string{"appearance.theme": combo.theme.ID, "appearance.icon_theme": combo.theme.IconTheme}
			if combo.theme.SchemeSetting != "" {
				settings[combo.theme.SchemeSetting] = combo.mode
			}
			s.store.set(settings)
			s.pushSettings(t)
			s.eval(t, `() => document.querySelector('.vd-window[data-app-id="files"] [data-action="maximize"]')
				.dispatchEvent(new MouseEvent('contextmenu', {bubbles:true, cancelable:true, button:2}))`)
			menu := page.Timeout(20 * time.Second).MustElement("#vd-window-layout-menu")
			dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
			if dir != "" {
				menu.MustScreenshot(filepath.Join(dir, "layouts-"+combo.label()+".png"))
			}
			if !s.eval(t, `() => {
				const diagrams = [...document.querySelectorAll('#vd-window-layout-menu [role=menuitemradio] .vd-window-layout-diagram')];
				const regions = diagrams.map(glyph => getComputedStyle(glyph, '::before').gridArea);
				return regions.join(';') === [
					'1 / 1 / span 2 / span 1', '1 / 2 / span 2 / span 1', '1 / 1 / span 1 / span 2', '2 / 1 / span 1 / span 2',
					'1 / 1 / span 1 / span 1', '1 / 2 / span 1 / span 1', '2 / 1 / span 1 / span 1', '2 / 2 / span 1 / span 1',
					'1 / 1 / span 2 / span 2'
				].join(';') && diagrams.every(glyph => getComputedStyle(glyph).display === 'grid' && glyph.ariaHidden === 'true');
			}`).Value.Bool() {
				t.Error("layout icons do not depict the nine snap regions in menu order")
			}
			page.Keyboard.MustType(input.Escape)
			group := page.Timeout(20 * time.Second).MustElement(`#vd-taskbar-apps [data-app-id="calculator"][data-window-count="2"]`)
			group.MustEval(`function() { this.focus(); }`)
			group.MustWaitVisible()
			if dir != "" {
				group.MustScreenshot(filepath.Join(dir, "group-"+combo.label()+".png"))
			}
			if !group.MustEval(`function() {
				const count = this.querySelector('.vd-task-count');
				return count?.textContent === '2' && count.ariaHidden === 'true' && count.offsetWidth > 0 &&
					!document.querySelector('#vd-taskbar-apps [data-app-id="files"] .vd-task-count');
			}`).Bool() {
				t.Error("the window count must appear on the grouped launcher only")
			}
			s.eval(t, `() => AuraDesktopWindowOverview.open({mode:'switcher', scope:'all'})`)
			s.waitFor(t, "window previews", 5*time.Second, `() =>
				document.querySelectorAll('.vd-window-overview-preview.is-ready').length === 3`)
			if dir != "" {
				page.MustElement(".vd-window-overview-panel").MustScreenshot(filepath.Join(dir, "overview-"+combo.label()+".png"))
			}
			if !s.eval(t, `() => {
				const cards = [...document.querySelectorAll('[data-overview-window]')];
				window.previousOverviewCard = cards.find(card => card.classList.contains('is-selected'));
				return new Set(cards.map(card => card.querySelector('.vd-window-overview-card-title').offsetTop)).size === 1;
			}`).Value.Bool() {
				t.Error("window titles do not align across projected and fallback previews")
			}
			page.Keyboard.MustType(input.ArrowRight)
			if !s.eval(t, `() => previousOverviewCard.isConnected && !previousOverviewCard.classList.contains('is-selected') &&
				document.querySelectorAll('.vd-window-overview-card.is-selected').length === 1 &&
				document.activeElement.matches('.vd-window-overview-card.is-selected [data-window-activate][aria-pressed=true]')`).Value.Bool() {
				t.Error("keyboard navigation must move focus and selection without replacing the preview cards")
			}
			page.Keyboard.MustType(input.Escape)
		})
	}

	page.MustSetViewport(390, 540, 1, false)
	s.store.set(map[string]string{"appearance.theme": "standard", "windows.animations": "true"})
	s.pushSettings(t)
	const openMotion = `() => {
		AuraDesktopWindowOverview.open({mode:'switcher'});
		return getComputedStyle(document.querySelector('.vd-window-overview')).animationName;
	}`
	if s.eval(t, openMotion).Value.Str() == "none" {
		t.Error("window overview has no entrance feedback with motion enabled")
	}
	page.Keyboard.MustType(input.Home, input.End)
	if !s.eval(t, `() => {
		const list = document.querySelector('.vd-window-overview-list');
		const selected = list.querySelector('.is-selected').getBoundingClientRect();
		const bounds = list.getBoundingClientRect();
		return list.scrollTop > 0 && selected.top >= bounds.top - 1 && selected.bottom <= bounds.bottom + 1;
	}`).Value.Bool() {
		t.Error("keyboard selection must reveal the last card on a narrow screen")
	}
	captureEverydayPanel(t, page, "overview-standard-narrow")
	s.store.set(map[string]string{"windows.animations": "false"})
	s.pushSettings(t)
	if s.eval(t, openMotion).Value.Str() != "none" {
		t.Error("window overview animates with the desktop motion switch off")
	}
	s.store.set(map[string]string{"windows.animations": "true"})
	s.pushSettings(t)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{
		{Name: "prefers-reduced-motion", Value: "reduce"},
	}}).Call(page); err != nil {
		t.Fatal(err)
	}
	if s.eval(t, openMotion).Value.Str() != "none" {
		t.Error("window overview animates with OS reduced motion enabled")
	}
}
