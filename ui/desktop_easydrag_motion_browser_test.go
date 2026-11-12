package ui

import "testing"

// TestEasyDragSmokeSetsReducedMotion: a page that startEasyDragSmoke opens reports
// prefers-reduced-motion: reduce, so no test depends on the machine's animation setting.
func TestEasyDragSmokeSetsReducedMotion(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	if !s.page.MustEval(`()=>matchMedia('(prefers-reduced-motion: reduce)').matches`).Bool() {
		t.Fatal("the page does not report prefers-reduced-motion: reduce")
	}
}
