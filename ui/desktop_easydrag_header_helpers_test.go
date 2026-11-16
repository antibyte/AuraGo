package ui

// Helpers of the EasyDrag browser tests that act on the editor header's pill (W1c). The pill's popover holds what left the
// header: the Active switch, Run now, Runs and the actions that finish a publish.

// pillEntry opens the pill's popover when it is closed and clicks its entry: active, run-now, runs, publish-finish or repair. The
// popover stays open after Active (the switch acts in place) and closes after every other entry.
func (s *easyDragSmoke) pillEntry(name string) {
	s.t.Helper()
	if !s.page.MustEval(`()=>!!document.querySelector('[data-ed-pill-popover]')`).Bool() {
		s.page.MustElement(`[data-ed-pill]`).MustClick()
		s.wait(`()=>!!document.querySelector('[data-ed-pill-popover]')`)
	}
	s.page.MustElement(`[data-ed-pill-popover] [data-ed-pop="` + name + `"]`).MustClick()
}
