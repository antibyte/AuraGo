package ui

// Helpers of the EasyDrag browser tests that act on the editor header's pill (W1c). The pill's popover holds what left the
// header: the Active switch, Run now, Runs and the actions that finish a publish.

// pillEntry opens the pill's popover when it is closed and clicks its entry: active, run-now, runs, publish-finish or repair. The
// popover stays open after Active (the switch acts in place) and closes after every other entry.
func (s *easyDragSmoke) pillEntry(name string) {
	s.t.Helper()
	if !s.page.MustEval(`()=>!!document.querySelector('[data-ed-pill-popover]')`).Bool() {
		s.clickSettled(`[data-ed-pill]`)
		s.wait(`()=>!!document.querySelector('[data-ed-pill-popover]')`)
	}
	s.clickSettled(`[data-ed-pill-popover] [data-ed-pop="` + name + `"]`)
}

// markTested says that the open flow's draft passed a test, as the server would after one: the fixture answers tested=true from now
// on (flows), the editor takes the flag at once (flows and blocks), and the header repaints. The primary button then offers Publish.
func (s *easyDragSmoke) markTested() {
	s.t.Helper()
	// An edit that is still on its way to the server makes the draft untested, and its save answer (a block fixture says nothing of
	// "tested") would drop the flag again: the mark is set once the saver is idle.
	s.wait(`()=>{const ed=edFixture.editor();return !ed||ed.saver.state==='saved'}`)
	s.page.MustEval(`()=>{edFixture.state.tested=true;const ed=edFixture.editor();if(!ed)return;ed.flow.tested=true;ed.flowFactsRevision=ed.saver.revision;ed.bus.emit('flow-facts',ed.flow)}`)
}
