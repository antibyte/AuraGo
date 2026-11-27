package storymaker

import (
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf16"
)

// Block limits repeat ui/js/storymaker/blocks.js (LAYOUTS, CHARTS, ICON_STYLES, LIST_MAX, POINTS_MAX); change both together.
var (
	diagramLayouts = map[string][2]int{"flow": {2, 5}, "cycle": {3, 6}, "hub": {3, 7}, "stack": {2, 5}, "compare": {2, 2}}
	chartRanges    = map[string][2]int{"bar": {1, 8}, "line": {2, 24}, "pie": {2, 6}, "counter": {1, 1}}
	iconStyles     = []string{"line", "badge", "card"}
)

const (
	blockListMax      = 4
	blockPointsMax    = 3
	blockHeadlineMax  = 80
	blockListLineMax  = 40
	blockChartLabel   = 24
	blockChartUnit    = 8
	blockChartTitle   = 40
	blockNodeLabelMax = 40
)

// jsLen counts UTF-16 units, as String.length does in blocks.js, so Go and the player refuse the same texts.
func jsLen(s string) int { return len(utf16.Encode([]rune(s))) }

// blockKind says which block a show beat makes: icon, diagram, chart, headline or list, or "" for an item or a plain text.
func blockKind(b *Beat) string {
	switch {
	case b.Item != "":
		return ""
	case b.Diagram != nil:
		return "diagram"
	case b.Icon != "":
		return "icon"
	case b.Chart != "":
		return "chart"
	case b.Text != "" && (b.Style == "headline" || b.Style == "list"):
		return b.Style
	}
	return ""
}

// showKinds counts what a show beat shows. A text next to an icon is the card label and does not count.
func showKinds(b *Beat) int {
	n := 0
	for _, set := range []bool{b.Item != "", b.Text != "" && b.Icon == "", b.Icon != "", b.Chart != "", b.Diagram != nil} {
		if set {
			n++
		}
	}
	return n
}

// block validates the fields of a block show beat.
func (v *validator) block(bb string, b *Beat, kind string, refs *sceneRefs) {
	switch kind {
	case "icon":
		v.iconBlock(bb, b)
	case "diagram":
		v.diagramBlock(bb, b, refs)
	case "chart":
		v.chartBlock(bb, b)
	case "headline":
		if strings.TrimSpace(strings.ReplaceAll(b.Text, "*", "")) == "" {
			v.errorf(CodeInvalidValue, bb+"/text", "The headline has no word to show.", "Write a short headline.")
		}
		if n := jsLen(b.Text); n > blockHeadlineMax {
			v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("The headline has %d characters; the limit is %d.", n, blockHeadlineMax), "Shorten the headline.")
		}
		if n := strings.Count(b.Text, "*"); n != 0 && n != 2 {
			v.warnf(CodeInvalidValue, bb+"/text", "A headline marks one word with a pair of * signs, for example Small models are *fast*.", "Use exactly two * around one word, or none.")
		}
	case "list":
		lines := 0
		for _, line := range strings.Split(b.Text, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lines++
			if n := jsLen(strings.TrimRightFunc(line, unicode.IsSpace)); n > blockListLineMax {
				v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("A list line has %d characters; the limit is %d.", n, blockListLineMax), "Shorten the line.")
			}
		}
		if lines == 0 {
			v.errorf(CodeInvalidValue, bb+"/text", "The list has no line to show.", "Write one to four lines, one per line break.")
		}
		if lines > blockListMax {
			v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("The list has %d lines; the limit is %d.", lines, blockListMax), "Keep at most 4 lines, one per line break.")
		}
	}
}

func (v *validator) iconBlock(bb string, b *Beat) {
	v.checkIcon(bb+"/icon", b.Icon)
	if b.Style != "" && !containsName(iconStyles, b.Style) {
		v.errorf(CodeInvalidValue, bb+"/style", fmt.Sprintf("An icon style is line, badge or card, not %q.", clip(b.Style, 20)), `Use "style": "card".`)
	}
	// A text next to an icon is its card label, whatever the style.
	if n := jsLen(b.Text); n > blockNodeLabelMax {
		v.errorf(CodeLimitExceeded, bb+"/text", fmt.Sprintf("The icon label has %d characters; the limit is %d.", n, blockNodeLabelMax), "Use one or two words.")
	}
}

// checkIcon reports an icon id the catalog does not know.
func (v *validator) checkIcon(path, id string) {
	if v.cat == nil {
		return
	}
	if _, ok := v.cat.Icon(id); ok {
		return
	}
	fix := "Search the catalog with kind icon."
	if alts := closestNames(id, v.cat.IconIDs(), 5); len(alts) > 0 {
		fix = "Use one of: " + listOf(alts) + ". Example: " + exampleJSON("icon", alts[0])
	}
	v.errorf(CodeUnknownAsset, path, fmt.Sprintf("The icon %q is not in the catalog.", clip(id, 60)), fix)
}

// chartBlock validates the chart type and data of a show beat or of the legacy chart beat.
func (v *validator) chartBlock(bb string, b *Beat) {
	rng, known := chartRanges[b.Chart]
	switch {
	case b.Chart == "":
		v.missing(bb, b, "chart", " (bar, line, pie or counter)")
	case !known:
		v.errorf(CodeInvalidValue, bb+"/chart", fmt.Sprintf("A chart is bar, line, pie or counter, not %q.", clip(b.Chart, 20)), `Use "chart": "bar".`)
	}
	d := b.Data
	if d == nil {
		v.missing(bb, b, "data", " ({labels, values})")
		return
	}
	n := len(d.Labels)
	switch {
	case n == 0 || n != len(d.Values):
		v.errorf(CodeInvalidValue, bb+"/data", "Chart data needs the same number (at least one) of labels and values.", `Use "data": {"labels":["Mo","Di"],"values":[2,5]}.`)
		return
	case known && (n < rng[0] || n > rng[1]):
		v.errorf(CodeLimitExceeded, bb+"/data", fmt.Sprintf("A %s chart takes %d to %d values, not %d.", b.Chart, rng[0], rng[1], n), "Show "+fmt.Sprintf("%d to %d", rng[0], rng[1])+" values.")
	case n > 24:
		v.errorf(CodeLimitExceeded, bb+"/data", fmt.Sprintf("The chart has %d entries; the limit is 24.", n), "Show fewer values.")
	}
	if b.Chart == "pie" {
		sum, negative := 0.0, false
		for _, x := range d.Values {
			sum += x
			negative = negative || x < 0 || math.IsNaN(x)
		}
		if negative || sum <= 0 {
			v.errorf(CodeInvalidValue, bb+"/data/values", "A pie chart needs values that are not negative and add up to more than 0.", "Use positive values.")
		}
	}
	for k, l := range d.Labels {
		if n := jsLen(l); n > blockChartLabel {
			v.errorf(CodeLimitExceeded, fmt.Sprintf("%s/data/labels/%d", bb, k), fmt.Sprintf("A chart label has %d characters; the limit is %d.", n, blockChartLabel), "Shorten the label.")
		}
	}
	if jsLen(d.Unit) > blockChartUnit {
		v.errorf(CodeLimitExceeded, bb+"/data/unit", fmt.Sprintf("A chart unit longer than %d characters does not fit.", blockChartUnit), `Use a short unit such as "%".`)
	}
	if jsLen(d.Title) > blockChartTitle {
		v.errorf(CodeLimitExceeded, bb+"/data/title", fmt.Sprintf("A chart title longer than %d characters does not fit.", blockChartTitle), "Shorten the title.")
	}
}

func (v *validator) diagramBlock(bb string, b *Beat, refs *sceneRefs) {
	d := b.Diagram
	rng, known := diagramLayouts[d.Layout]
	if !known {
		v.errorf(CodeInvalidValue, bb+"/diagram/layout", fmt.Sprintf("A diagram layout is flow, cycle, hub, stack or compare, not %q.", clip(d.Layout, 20)), `Use "layout": "flow".`)
	} else if n := len(d.Nodes); n < rng[0] || n > rng[1] {
		v.errorf(CodeLimitExceeded, bb+"/diagram/nodes", fmt.Sprintf("A %s diagram takes %d to %d nodes, not %d.", d.Layout, rng[0], rng[1], n), fmt.Sprintf("Use %d to %d nodes.", rng[0], rng[1]))
	}
	seen := map[string]bool{}
	for k := range d.Nodes {
		n := &d.Nodes[k]
		np := fmt.Sprintf("%s/diagram/nodes/%d", bb, k)
		switch {
		case !validSlug(n.ID):
			v.errorf(CodeInvalidValue, np+"/id", fmt.Sprintf("The node id %q is not a slug.", clip(n.ID, 40)), "Use lowercase letters, digits and dashes, for example srv.")
		case seen[n.ID]:
			v.errorf(CodeDuplicateID, np+"/id", fmt.Sprintf("The node id %q is used twice in this diagram.", n.ID), "Give each node its own id.")
		default:
			seen[n.ID] = true
		}
		switch l := jsLen(n.Label); {
		case strings.TrimSpace(n.Label) == "":
			v.errorf(CodeMissingField, np+"/label", "A diagram node needs a label.", `Add "label": "Server".`)
		case l > blockNodeLabelMax:
			v.errorf(CodeLimitExceeded, np+"/label", fmt.Sprintf("The node label has %d characters; the limit is %d.", l, blockNodeLabelMax), "Use one or two words.")
		}
		if n.Icon != "" {
			v.checkIcon(np+"/icon", n.Icon)
		}
		for k, pt := range n.Points {
			if l := jsLen(pt); l < 1 || l > blockNodeLabelMax {
				v.errorf(CodeLimitExceeded, fmt.Sprintf("%s/points/%d", np, k), fmt.Sprintf("A point has %d characters; it needs 1 to %d.", l, blockNodeLabelMax), "Use a short, non-empty point.")
			}
		}
		switch {
		case len(n.Points) > blockPointsMax:
			v.errorf(CodeLimitExceeded, np+"/points", fmt.Sprintf("A node has %d points; the limit is %d.", len(n.Points), blockPointsMax), "Keep at most 3 points.")
		case len(n.Points) > 0 && d.Layout != "compare":
			v.warnf(CodeInvalidValue, np+"/points", "Only the compare layout shows points.", `Remove "points" or use "layout": "compare".`)
		}
		if n.Cue != 0 {
			switch {
			case n.Cue < 0:
				v.errorf(CodeInvalidValue, np+"/cue", "A cue is a narration marker number, 1 or more.", `Use "cue": 1 and put {1} into the narration.`)
			case !refs.markers[n.Cue]:
				v.errorf(CodeUnknownCue, np+"/cue", fmt.Sprintf("The narration has no marker {%d}.", n.Cue), v.cueFix(refs, n.Cue))
			}
		}
	}
}

// blockAt validates the position of a block: a point or a name of BlockAreas. Blocks skip the environment spot check.
func (v *validator) blockAt(path string, p *Position, env string) {
	if p != nil && !p.Point && p.Spot != "" && len(p.Invalid) == 0 {
		if !containsName(BlockAreas, p.Spot) {
			v.errorf(CodeInvalidValue, path, fmt.Sprintf("A block is shown at %s, not %q.", listOf(BlockAreas), clip(p.Spot, 40)), `Use "at": "full".`)
		}
		return
	}
	v.position(path, p, env)
}
