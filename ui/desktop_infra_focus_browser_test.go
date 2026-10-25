package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopInfraFocusReturnBrowser covers the focus fixes of Plan D Task 7:
//   - a dialog opened from a menu item gives the focus back to the trigger of that menu when it closes (the item itself is
//     gone by then, and the browser's own restore has nowhere to go);
//   - the narrow library sheet takes the focus in when it opens and gives it back to its toggle on Escape, and the toggle's
//     aria-expanded follows the sheet at every width.
func TestDesktopInfraFocusReturnBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	page := infraPolishOpen(t, fixture, 700, 800)
	defer page.Close()
	page.MustEval(`()=>{
		window.infraFocus=()=>{
			const a=document.activeElement,app=document.querySelector('.infra-app'),toggle=document.querySelector('[data-action="toggle-library"]');
			return JSON.stringify({
				sheet:app.dataset.librarySheet,expanded:toggle.getAttribute('aria-expanded'),
				inLibrary:!!a&&!!a.closest('[data-library]'),onToggle:a===toggle,
				onAddMenu:a===document.querySelector('[data-action="add-menu"]'),dialog:document.querySelector('[data-dialog]').open,
			});
		};
	}`)
	state := func() string { return page.MustEval(`()=>infraFocus()`).Str() }
	waitState := func(step, want string) {
		t.Helper()
		page.MustWait(`(want)=>infraFocus()===want`, want)
		if got := state(); got != want {
			t.Fatalf("%s: %s; want %s", step, got, want)
		}
	}

	// The narrow library sheet: opening moves the focus into it, Escape closes it and focuses the toggle.
	waitState("narrow window", `{"sheet":"closed","expanded":"false","inLibrary":false,"onToggle":false,"onAddMenu":false,"dialog":false}`)
	page.MustEval(`()=>document.querySelector('[data-action="toggle-library"]').focus()`)
	page.Keyboard.MustType(input.Enter)
	waitState("sheet opened by the keyboard", `{"sheet":"open","expanded":"true","inLibrary":true,"onToggle":false,"onAddMenu":false,"dialog":false}`)
	page.Keyboard.MustType(input.Escape)
	waitState("sheet closed by Escape", `{"sheet":"closed","expanded":"false","inLibrary":false,"onToggle":true,"onAddMenu":false,"dialog":false}`)
	// The toggle closes it again and keeps the focus.
	page.Keyboard.MustType(input.Enter)
	waitState("sheet opened again", `{"sheet":"open","expanded":"true","inLibrary":true,"onToggle":false,"onAddMenu":false,"dialog":false}`)
	page.MustEval(`()=>document.querySelector('[data-action="toggle-library"]').click()`)
	waitState("sheet closed by the toggle", `{"sheet":"closed","expanded":"false","inLibrary":false,"onToggle":true,"onAddMenu":false,"dialog":false}`)

	// A dialog opened from a menu item: its menu closes, and when the dialog closes the focus lands on the menu's trigger.
	infraPolishResize(page, 1100, 800)
	page.MustEval(`()=>document.querySelector('[data-action="add-menu"]').focus()`)
	page.Keyboard.MustType(input.Enter)
	page.MustWait(`()=>!document.querySelector('[data-menu="add"]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-menu="add"] [data-action="add-place"]').focus()`)
	page.Keyboard.MustType(input.Enter)
	page.MustWait(`()=>document.querySelector('[data-dialog]').open`)
	page.Keyboard.MustType(input.Escape)
	waitState("dialog closed", `{"sheet":"closed","expanded":"true","inLibrary":false,"onToggle":false,"onAddMenu":true,"dialog":false}`)
	requireInfraBrowserClean(t, page)
}
