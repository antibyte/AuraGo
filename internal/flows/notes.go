package flows

import "math"

// Sticky note bounds (UI-11). Wave 4 renders the notes; S0 only bounds and protects them.
const (
	maxStickyNotes = 100
	maxNoteIDBytes = 64
	maxNoteRunes   = 2048
	maxNoteBytes   = 8192
	minNoteSide    = 80
	maxNoteSide    = 1200
)

// noteChecks bounds the sticky notes of the flow. Like groupChecks, it reports one
// NOTE_INVALID for the whole list and never echoes note text. A note is valid when the flow
// holds at most 100 notes; its id is unique, 1-64 bytes and one line; its text is at most
// 2,048 runes and 8,192 bytes (empty is allowed) and passes validText; width and height lie in
// 80-1,200; x and y are finite; and its colour is empty or a group colour.
func (v *validator) noteChecks() {
	if len(v.f.Notes) == 0 {
		return
	}
	bad := func() { v.add(IssueNoteInvalid, SeverityError, "", "", "notes", "sticky notes are invalid") }
	if len(v.f.Notes) > maxStickyNotes {
		bad()
		return
	}
	ids := make(map[string]bool, len(v.f.Notes))
	for _, note := range v.f.Notes {
		validID := note.ID != "" && validText(note.ID, maxNoteIDBytes, maxNoteIDBytes, false)
		validColor := note.Color == "" || groupColors[note.Color]
		if !validID || ids[note.ID] || !validText(note.Text, maxNoteRunes, maxNoteBytes, true) || !validColor ||
			!noteSide(note.Width) || !noteSide(note.Height) || !finiteCoordinate(note.Position.X) || !finiteCoordinate(note.Position.Y) {
			bad()
			return
		}
		ids[note.ID] = true
	}
}

// noteSide reports whether a width or height lies in 80-1,200 (NaN does not).
func noteSide(v float64) bool { return v >= minNoteSide && v <= maxNoteSide }

func finiteCoordinate(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// annotationIssue reports the codes of canvas annotations (groups, sticky notes). They never
// change execution, so a live flow and a block step ignore them.
func annotationIssue(code string) bool { return code == IssueGroupInvalid || code == IssueNoteInvalid }

// withoutAnnotationIssues returns issues without GROUP_INVALID and NOTE_INVALID.
func withoutAnnotationIssues(issues []Issue) []Issue {
	var out []Issue
	for _, issue := range issues {
		if !annotationIssue(issue.Code) {
			out = append(out, issue)
		}
	}
	return out
}
