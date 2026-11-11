package flows

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Format characters that validText keeps: joined emoji and Persian text need them
// (security.hardenExternalBoundary keeps the same two).
const (
	zeroWidthNonJoiner = 0x200C
	zeroWidthJoiner    = 0x200D
)

// groupColors are the colour names of visual groups. Sticky notes use the same palette.
var groupColors = map[string]bool{"neutral": true, "blue": true, "green": true, "amber": true, "rose": true, "violet": true}

// validText reports whether text is valid UTF-8 within maxRunes and maxBytes, with no control
// characters (tab, line feed and carriage return are allowed when multiline) and no format
// characters (Cf) except ZWNJ and ZWJ. A bidi override then cannot reorder what the owner
// reads. Groups and sticky notes share it.
func validText(text string, maxRunes, maxBytes int, multiline bool) bool {
	if !utf8.ValidString(text) || len(text) > maxBytes || utf8.RuneCountInString(text) > maxRunes {
		return false
	}
	for _, r := range text {
		if unicode.IsControl(r) && !(multiline && (r == '\t' || r == '\n' || r == '\r')) {
			return false
		}
		if unicode.Is(unicode.Cf, r) && r != zeroWidthNonJoiner && r != zeroWidthJoiner {
			return false
		}
	}
	return true
}

// groupChecks never echoes author-supplied text in validation issues.
func (v *validator) groupChecks() {
	if v.f.Groups == nil {
		return
	}
	groups := *v.f.Groups
	bad := func() { v.add(IssueGroupInvalid, SeverityError, "", "", "groups", "visual groups are invalid") }
	if len(groups) > 100 {
		bad()
		return
	}
	nodes := make(map[string]bool, len(v.f.Nodes))
	for _, node := range v.f.Nodes {
		nodes[node.ID] = true
	}
	ids := make(map[string]bool, len(groups))
	members := make(map[string]bool, len(nodes))
	for _, group := range groups {
		validID := len(group.ID) == 10 && strings.HasPrefix(group.ID, "g_") && ValidNodeID("n_"+group.ID[2:])
		validTitle := strings.TrimSpace(group.Title) != "" && validText(group.Title, 80, 320, false)
		validNote := validText(group.Note, 2048, 8192, true)
		validColor := groupColors[group.Color]
		if !validID || ids[group.ID] || !validTitle || !validNote || !validColor || len(group.Members) == 0 || len(group.Members) > MaxNodes {
			bad()
			return
		}
		ids[group.ID] = true
		for _, id := range group.Members {
			if !ValidNodeID(id) || !nodes[id] || members[id] {
				bad()
				return
			}
			members[id] = true
		}
	}
}
