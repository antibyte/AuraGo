package consumers

import (
	"strings"

	"aurago/internal/security"
)

// State collects prompt state. Untrusted text is scrubbed, truncated and
// fenced like helperExternalDataBlock in internal/agent.
type State struct{ blocks []string }

func NewState() State { return State{} }

func (s State) Untrusted(kind, text string, maxRunes int) State {
	if maxRunes <= 0 {
		return s
	}
	text = strings.TrimSpace(security.Scrub(text))
	if runes := []rune(text); len(runes) > maxRunes {
		text = string(runes[:maxRunes])
	}
	if text == "" {
		return s
	}
	s.blocks = append(append([]string(nil), s.blocks...), kind+" (untrusted):\n"+security.IsolateExternalData(text))
	return s
}

// Fact adds short system-authored facts (camera name, counts). Still scrubbed.
func (s State) Fact(kind, text string) State {
	text = strings.TrimSpace(security.Scrub(text))
	if text == "" {
		return s
	}
	s.blocks = append(append([]string(nil), s.blocks...), kind+": "+text)
	return s
}

func (s State) String() string { return strings.Join(s.blocks, "\n\n") }
