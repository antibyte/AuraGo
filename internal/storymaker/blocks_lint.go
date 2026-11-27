package storymaker

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const (
	blocksMinScenes   = 5 // a film of this many scenes should use at least one block
	blocksRepeatRun   = 3 // the same block key in this many scenes in a row is a repeat
	blocksNodeWords   = 3
	blocksHeadlineMax = 8
)

// The patterns use character classes instead of escapes: [.] is a dot, [(] a parenthesis, [ \t\r\n] any white space.
var (
	codeBlockCall   = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])api[ \t\r\n]*[.][ \t\r\n]*(icon|diagram|chart|headline|list)[ \t\r\n]*[(]`)
	codeBlockLayout = regexp.MustCompile(`layout[ \t\r\n]*:[ \t\r\n]*['"](flow|cycle|hub|stack|compare)['"]`)
)

// LintBlocks gives the advice about the explainer blocks of a film: no_blocks (5 scenes or more and none shows a block),
// block_repeat (the third scene of every run of 3 scenes whose first block is the same kind) and label_long (a node label
// of more than 3 words, a headline of more than 8). sources maps a code scene id to its source; a scene without a source
// has no block. Every finding is a warning.
func LintBlocks(st *Story, sources map[string]string) []Issue {
	if st == nil {
		return nil
	}
	var out []Issue
	keys := make([]string, len(st.Scenes))
	hasBlock := false
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		if sc.Kind == "code" {
			keys[i] = codeBlockKey(sources[sc.ID])
		} else {
			keys[i] = storyboardBlockKey(sc)
			out = append(out, lintBlockLabels(sc, i)...)
		}
		hasBlock = hasBlock || keys[i] != ""
	}
	if len(st.Scenes) >= blocksMinScenes && !hasBlock {
		out = append(out, Issue{
			Severity: SeverityWarning, Path: "/scenes", Code: CodeNoBlocks,
			Message: fmt.Sprintf("None of the %d scenes shows an explainer block (diagram, chart, icon, headline or list).", len(st.Scenes)),
			Fix:     `Show a block in some scenes: a flow diagram for steps, a chart for numbers, an icon card for a thing, a headline for the key sentence. Example beat: {"do":"show","diagram":{"layout":"flow","nodes":[{"id":"a","label":"Start"},{"id":"b","label":"Ziel"}]},"at":"board","entry":"draw"}.`,
		})
	}
	run := 0
	for i, key := range keys {
		if key == "" || i == 0 || key != keys[i-1] {
			run = 0
		}
		if key == "" {
			continue
		}
		run++
		if run%blocksRepeatRun == 0 {
			out = append(out, Issue{
				Severity: SeverityWarning, SceneID: st.Scenes[i].ID, Path: jp("scenes", i), Code: CodeBlockRepeat,
				Message: fmt.Sprintf("Scene %s is the third scene in a row whose block is %q.", st.Scenes[i].ID, key),
				Fix:     "Show another kind of block in this scene (chart, cycle, compare, icon card or headline), or a prop with the actor.",
			})
		}
	}
	return out
}

// storyboardBlockKey is the kind of the first block beat of a storyboard scene: the layout of a diagram, else chart, icon,
// headline or list. "" means none.
func storyboardBlockKey(sc *Scene) string {
	for j := range sc.Beats {
		b := &sc.Beats[j]
		if b.Do != "show" {
			continue
		}
		switch kind := blockKind(b); kind {
		case "":
		case "diagram":
			if b.Diagram.Layout != "" {
				return b.Diagram.Layout
			}
			return kind
		default:
			return kind
		}
	}
	return ""
}

// codeBlockKey is the kind of the first api block call of a code scene source; a diagram counts by the first layout after it.
func codeBlockKey(src string) string {
	loc := codeBlockCall.FindStringSubmatchIndex(src)
	if loc == nil {
		return ""
	}
	kind := src[loc[2]:loc[3]]
	if kind != "diagram" {
		return kind
	}
	if m := codeBlockLayout.FindStringSubmatch(src[loc[1]:]); m != nil {
		return m[1]
	}
	return kind
}

// lintBlockLabels reports the node labels and headlines of a storyboard scene that are too long to read on a board.
func lintBlockLabels(sc *Scene, i int) []Issue {
	var out []Issue
	for j := range sc.Beats {
		b := &sc.Beats[j]
		if b.Do != "show" {
			continue
		}
		switch blockKind(b) {
		case "diagram":
			for k, n := range b.Diagram.Nodes {
				if words := len(strings.Fields(n.Label)); words > blocksNodeWords {
					out = append(out, Issue{
						Severity: SeverityWarning, SceneID: sc.ID, Path: jp("scenes", i, "beats", j, "diagram", "nodes", k, "label"), Code: CodeLabelLong,
						Message: fmt.Sprintf("The node label %q has %d words; a label reads best with at most %d.", clip(n.Label, 40), words, blocksNodeWords),
						Fix:     "Shorten the label to one to three words.",
					})
				}
			}
		case "headline":
			if words := len(strings.Fields(strings.ReplaceAll(b.Text, "*", ""))); words > blocksHeadlineMax {
				out = append(out, Issue{
					Severity: SeverityWarning, SceneID: sc.ID, Path: jp("scenes", i, "beats", j, "text"), Code: CodeLabelLong,
					Message: fmt.Sprintf("The headline has %d words; a headline reads best with at most %d.", words, blocksHeadlineMax),
					Fix:     "Shorten the headline to the key sentence.",
				})
			}
		}
	}
	return out
}

// blockIssues runs LintBlocks for the job's story. Like craftIssues it is for a create job only. The sources of the code
// scenes are read from the project; a scene whose source cannot be read is left out.
func (s *Studio) blockIssues(ctx context.Context, story *Story) []Issue {
	if s.job.Kind != AgentKindCreate {
		return nil
	}
	sources := map[string]string{}
	for i := range story.Scenes {
		if story.Scenes[i].Kind != "code" {
			continue
		}
		if data, _, err := s.deps.Projects.LoadCodeSource(ctx, s.job.ProjectID, story.Scenes[i].ID); err == nil {
			sources[story.Scenes[i].ID] = string(data)
		}
	}
	return LintBlocks(story, sources)
}
