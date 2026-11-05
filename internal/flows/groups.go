package flows

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func validGroupText(text string, maxRunes, maxBytes int, multiline bool) bool {
	if !utf8.ValidString(text) || len(text) > maxBytes || utf8.RuneCountInString(text) > maxRunes {
		return false
	}
	for _, r := range text {
		if unicode.IsControl(r) && !(multiline && (r == '\t' || r == '\n' || r == '\r')) {
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
		validTitle := strings.TrimSpace(group.Title) != "" && validGroupText(group.Title, 80, 320, false)
		validNote := validGroupText(group.Note, 2048, 8192, true)
		validColor := group.Color == "neutral" || group.Color == "blue" || group.Color == "green" || group.Color == "amber" || group.Color == "rose" || group.Color == "violet"
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
