package ui

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// The lab (TestEasyDragLabBrowser) renders scenes in the page and measures there; every decision is made in this file,
// on the colour helpers of desktop_easydrag_colour_test.go and desktop_theme_css_contract_test.go.

const (
	labTextMin = 4.5 // status words and other text
	labMarkMin = 3.0 // discs, tiles' icons, group frames
)

// labKnown holds findings that were reported to the owner instead of fixed (spec §11: a material that blocks a surface
// token fix). The key is theme|scene|element|check, the value is the reason. A finding in this map is logged as
// LAB-KNOWN and does not fail the lab. The map starts empty and an entry without a reason fails TestEasyDragLabFindings.
var labKnown = map[string]string{}

// labProbe names a group of elements the page measures. Role decides the check:
//   - text: the element's text colour against what is behind it, 4.5:1;
//   - mark: the element's own colour (background, else its first svg shape) against what is behind it, 3:1;
//   - tile: the icon colour (the element's text colour) against the tile's own background, 3:1;
//   - edge: the left border colour against what is behind it, 3:1 (group frames);
//   - ref: only measured, used by the distance check.
//
// Min is the number of visible matches the scene must have, so a renamed class cannot make a scene pass empty.
type labProbe struct {
	Name     string `json:"name"`
	Selector string `json:"selector"`
	Role     string `json:"role"`
	Min      int    `json:"min"`
}

// labSpec is what the page measures in one scene.
type labSpec struct {
	Probes []labProbe `json:"probes"`
	Scope  string     `json:"scope"` // root selector for probes, names and clipping; "" means .ed-app
	Names  bool       `json:"names"`
	Clip   bool       `json:"clip"`
}

// labElement is one measured element. Colours are sRGB 0-255, already composited over what is behind them.
type labElement struct {
	Probe   string      `json:"probe"`
	Role    string      `json:"role"`
	Name    string      `json:"name"` // tag.class[status=..][data-ed..]@surface
	Text    string      `json:"text"`
	Status  string      `json:"status"`
	Surface string      `json:"surface"`
	Fg      *[3]float64 `json:"fg"`     // text colour over Eff (set by labPaint, or given directly in a test)
	Eff     *[3]float64 `json:"eff"`    // the element's own background over Behind, faded (idem)
	Op      float64     `json:"op"`     // the element's opacity times its ancestors'
	OwnRaw  *[4]float64 `json:"ownRaw"` // the computed background-color, r g b 0-255 and alpha 0-1, nil when transparent
	FgRaw   *[4]float64 `json:"fgRaw"`  // the computed text colour, r g b 0-255 and alpha 0-1
	Behind  *[3]float64 `json:"behind"` // the ancestors' backgrounds
	Own     *[3]float64 `json:"own"`    // the element's own background over Behind, nil when it has none
	Ring    *[3]float64 `json:"ring"`   // the outermost solid box-shadow ring over Behind (Part A's --ed-status-edge), nil when none
	Svg     *[3]float64 `json:"svg"`    // the first svg shape's fill (or stroke) over Behind
	Edge    *[3]float64 `json:"edge"`   // the left border over Behind
	Approx  bool        `json:"approx"` // a gradient or pattern lies under it; only background-color was read
}

type labBox struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

type labClip struct {
	Name        string `json:"name"`
	Text        string `json:"text"`
	ScrollWidth int    `json:"sw"`
	ClientWidth int    `json:"cw"`
	Ellipsis    bool   `json:"ellipsis"`
	Title       bool   `json:"title"`
	Looked      string `json:"looked"` // where the title was looked for
}

// labScene is the page's answer for one scene.
type labScene struct {
	Elements []labElement   `json:"elements"`
	Counts   map[string]int `json:"counts"`
	Unnamed  []labBox       `json:"unnamed"`
	Scanned  int            `json:"scanned"`
	Clipped  []labClip      `json:"clipped"`
}

type labFinding struct{ Theme, Scene, Element, Check, Value, Want string }

func (f labFinding) String() string {
	return fmt.Sprintf("LAB-FINDING theme=%s scene=%s element=%s check=%s value=%s want=%s", f.Theme, f.Scene, f.Element, f.Check, f.Value, f.Want)
}

func (f labFinding) Key() string { return f.Theme + "|" + f.Scene + "|" + f.Element + "|" + f.Check }

// labMarkColour is the colour a disc, dot or tile shows: its own background, else its svg fill.
func labMarkColour(e labElement) *[3]float64 {
	if e.Own != nil {
		return e.Own
	}
	return e.Svg
}

// labPaint composites the raw page colours the way the browser paints them. Opacity fades the element as a group:
// the background and the text are composited inside the group first (the text over the element's own background, which sits
// over what is behind), and the group is faded once over what is behind. Fading each layer on its own would apply the
// opacity twice. It leaves Fg and Eff alone when the element carries no raw colour (a test that gives them directly).
func labPaint(e *labElement) {
	if e.FgRaw == nil || e.Behind == nil {
		return
	}
	op := e.Op
	if op == 0 {
		op = 1
	}
	over := func(top [4]float64, under [3]float64) [3]float64 {
		var out [3]float64
		for i := 0; i < 3; i++ {
			out[i] = top[i]*top[3] + under[i]*(1-top[3])
		}
		return out
	}
	fade := func(c [3]float64) *[3]float64 {
		var out [3]float64
		for i := 0; i < 3; i++ {
			out[i] = c[i]*op + e.Behind[i]*(1-op)
		}
		return &out
	}
	ownO := *e.Behind
	if e.OwnRaw != nil && e.OwnRaw[3] > 0 {
		ownO = over(*e.OwnRaw, *e.Behind)
	}
	e.Eff = fade(ownO)
	e.Fg = fade(over(*e.FgRaw, ownO))
}

// labMarkFinding passes a mark when its fill or its outer ring reaches labMarkMin against what is behind it: Part A's
// 1px --ed-status-edge ring exists to give the discs 3:1 on the light surfaces (amendment 4). When both fail, the finding
// prints both values.
func labMarkFinding(out *[]labFinding, theme, scene string, e labElement, fill [3]float64) {
	fr := cssContrastRatio(fill, *e.Behind)
	if fr >= labMarkMin {
		return
	}
	value := fmt.Sprintf("%.3f:1", fr)
	if e.Ring != nil {
		rr := cssContrastRatio(*e.Ring, *e.Behind)
		if rr >= labMarkMin {
			return
		}
		value = fmt.Sprintf("fill %.3f:1, ring %.3f:1", fr, rr)
	}
	*out = append(*out, labFinding{theme, scene, e.Name, "mark-contrast", value + labApproxNote(e), fmt.Sprintf("at least %.1f:1 (fill or ring)", labMarkMin)})
}

func labApproxNote(e labElement) string {
	if e.Approx {
		return " [a gradient or pattern lies under it; only its background-color was read]"
	}
	return ""
}

// labContrastFindings checks every measured element by its role and that each probe found its minimum.
func labContrastFindings(theme, scene string, probes []labProbe, els []labElement, counts map[string]int) []labFinding {
	var out []labFinding
	for _, p := range probes {
		if counts[p.Name] < p.Min {
			out = append(out, labFinding{theme, scene, "probe " + p.Name + " (" + p.Selector + ")", "present", fmt.Sprintf("%d visible", counts[p.Name]), fmt.Sprintf("at least %d", p.Min)})
		}
	}
	for _, e := range els {
		unmeasured := func(what string) {
			out = append(out, labFinding{theme, scene, e.Name, "measured", "no " + what, "a colour"})
		}
		ratio := func(check string, a, b [3]float64, need float64) {
			if r := cssContrastRatio(a, b); r < need {
				out = append(out, labFinding{theme, scene, e.Name, check, fmt.Sprintf("%.3f:1%s", r, labApproxNote(e)), fmt.Sprintf("at least %.1f:1", need)})
			}
		}
		labPaint(&e)
		switch e.Role {
		case "text":
			if e.Fg == nil || e.Eff == nil {
				unmeasured("text colour")
				continue
			}
			ratio("text-contrast", *e.Fg, *e.Eff, labTextMin)
		case "tile":
			if e.Fg == nil || e.Eff == nil {
				unmeasured("icon colour")
				continue
			}
			ratio("icon-on-tile-contrast", *e.Fg, *e.Eff, labMarkMin)
		case "mark":
			colour := labMarkColour(e)
			if colour == nil || e.Behind == nil {
				unmeasured("disc colour")
				continue
			}
			labMarkFinding(&out, theme, scene, e, *colour)
		case "edge":
			if e.Edge == nil || e.Behind == nil {
				unmeasured("frame colour")
				continue
			}
			ratio("frame-contrast", *e.Edge, *e.Behind, labMarkMin)
		}
	}
	return out
}

// labStatusRefs are the four coloured statuses a category colour must stay away from (spec §7). The grey statuses are
// left out on purpose: the logic and tool categories are grey by design (easyDragS0MarkSoft).
var labStatusRefs = map[string]bool{"ok": true, "failed": true, "running": true, "uncertain": true}

// labDistanceFindings keeps rendered category colours (probe "tile", their own colour; probe "group", their frame colour)
// 0.20 OKLab from every rendered status colour (probe "ref-status") and, for tiles with chroma, 30 degrees of hue.
func labDistanceFindings(theme, scene string, els []labElement) []labFinding {
	var statuses []labElement
	for _, e := range els {
		if e.Probe == "ref-status" && labStatusRefs[e.Status] && labMarkColour(e) != nil {
			statuses = append(statuses, e)
		}
	}
	var out []labFinding
	if len(statuses) < 4 {
		return append(out, labFinding{theme, scene, "probe ref-status", "present", fmt.Sprintf("%d coloured status discs", len(statuses)), "at least 4"})
	}
	for _, e := range els {
		var colour *[3]float64
		switch e.Probe {
		case "tile":
			colour = e.Own
		case "group":
			colour = e.Edge
		}
		if colour == nil {
			continue
		}
		_, chroma, hue := easyDragOKLCH(*colour)
		for _, s := range statuses {
			sc := *labMarkColour(s)
			if d := easyDragOKLabDistance(*colour, sc); d < easyDragS0StatusMin {
				out = append(out, labFinding{theme, scene, e.Name, "oklab-distance-to-status-" + s.Status, fmt.Sprintf("%.3f", d), "at least 0.20"})
			}
			if e.Probe == "tile" && chroma >= 0.05 {
				if _, _, sh := easyDragOKLCH(sc); easyDragHueGap(hue, sh) < easyDragS0MarkHueGapMin {
					out = append(out, labFinding{theme, scene, e.Name, "hue-gap-to-status-" + s.Status, fmt.Sprintf("%.0f degrees (hue %.0f against %.0f)", easyDragHueGap(hue, sh), hue, sh), "at least 30 degrees"})
				}
			}
		}
	}
	return out
}

func labNameFindings(theme, scene string, res labScene) []labFinding {
	var out []labFinding
	if res.Scanned == 0 {
		return []labFinding{{theme, scene, "name scan", "present", "0 controls scanned", "at least 1"}}
	}
	for _, b := range res.Unnamed {
		out = append(out, labFinding{theme, scene, b.Name, "accessible-name", "none (text " + fmt.Sprintf("%q", b.Text) + ")", "aria-label, label text, text content or role=img with aria-label"})
	}
	return out
}

func labClipFindings(theme, scene string, res labScene) []labFinding {
	var out []labFinding
	for _, c := range res.Clipped {
		out = append(out, labFinding{theme, scene, c.Name, "text-clipped", fmt.Sprintf("scrollWidth %d > clientWidth %d, ellipsis=%v, title=%v (looked at %s), text %q", c.ScrollWidth, c.ClientWidth, c.Ellipsis, c.Title, c.Looked, c.Text), "no overflow, or text-overflow: ellipsis plus a title"})
	}
	return out
}

// labSplit separates findings the owner accepted (labKnown) from the ones that fail the lab.
func labSplit(findings []labFinding) (failing, known []labFinding) {
	for _, f := range findings {
		if _, ok := labKnown[f.Key()]; ok {
			known = append(known, f)
		} else {
			failing = append(failing, f)
		}
	}
	return failing, known
}

// labRound keeps a measured number readable in logs.
func labRound(v float64) float64 { return math.Round(v*100) / 100 }

func TestEasyDragLabFindings(t *testing.T) {
	white, black, grey := &[3]float64{255, 255, 255}, &[3]float64{0, 0, 0}, &[3]float64{150, 150, 150}
	probes := []labProbe{{Name: "word", Selector: ".ed-status-word", Role: "text", Min: 1}}

	// a grey word on white is 2.96:1: one finding that names theme, scene, element and value
	bad := []labElement{{Probe: "word", Role: "text", Name: "span.ed-status-word[status=ok]@panel", Fg: grey, Eff: white, Behind: white}}
	got := labContrastFindings("amiga", "1-status", probes, bad, map[string]int{"word": 1})
	if len(got) != 1 {
		t.Fatalf("got %d findings %v, want 1", len(got), got)
	}
	text := got[0].String()
	for _, want := range []string{"theme=amiga", "scene=1-status", "span.ed-status-word[status=ok]@panel", "check=text-contrast", "2.9", "at least 4.5:1"} {
		if !strings.Contains(text, want) {
			t.Errorf("finding %q lacks %q", text, want)
		}
	}

	// black on white passes, and a probe that finds nothing is a finding
	good := []labElement{{Probe: "word", Role: "text", Name: "x", Fg: black, Eff: white, Behind: white}}
	if got := labContrastFindings("amiga", "1-status", probes, good, map[string]int{"word": 1}); len(got) != 0 {
		t.Errorf("black on white gave findings %v", got)
	}
	if got := labContrastFindings("amiga", "1-status", probes, nil, map[string]int{"word": 0}); len(got) != 1 || !strings.Contains(got[0].String(), "0 visible") {
		t.Errorf("an empty probe gave %v, want one finding naming 0 visible", got)
	}

	// a mark is read from its own colour, else its svg fill; 3:1 is the line (grey 150 on white is 2.96, 140 is 3.4)
	svgMark := []labElement{{Probe: "disc", Role: "mark", Name: "disc", Svg: &[3]float64{140, 140, 140}, Behind: white}}
	if got := labContrastFindings("t", "s", nil, svgMark, nil); len(got) != 0 {
		t.Errorf("a 3.4:1 svg disc gave findings %v", got)
	}
	// the fill fails but the ring reaches 3:1: no finding; both fail: one finding that prints both values
	ringOK := []labElement{{Probe: "disc", Role: "mark", Name: "disc", Own: &[3]float64{200, 220, 200}, Ring: &[3]float64{140, 140, 140}, Behind: white}}
	if got := labContrastFindings("t", "s", nil, ringOK, nil); len(got) != 0 {
		t.Errorf("a failing fill with a 3.4:1 ring gave findings %v", got)
	}
	ringWeak := []labElement{{Probe: "disc", Role: "mark", Name: "disc", Own: &[3]float64{200, 220, 200}, Ring: grey, Behind: white}}
	if got := labContrastFindings("t", "s", nil, ringWeak, nil); len(got) != 1 || !strings.Contains(got[0].Value, "fill ") || !strings.Contains(got[0].Value, "ring 2.9") {
		t.Errorf("fill and ring both failing gave %v, want one finding naming both", got)
	}
	// opacity fades the element as a group, once: a tile at 55% opacity is 2.98:1, not the 3.30:1 of fading each layer
	// (panel 245, tile 180/220/240, ink 20/40/60)
	faded := []labElement{{Probe: "tile", Role: "tile", Name: "tile", Op: 0.55, OwnRaw: &[4]float64{180, 220, 240, 1}, FgRaw: &[4]float64{20, 40, 60, 1}, Behind: &[3]float64{245, 245, 245}}}
	if got := labContrastFindings("t", "s", nil, faded, nil); len(got) != 1 || got[0].Check != "icon-on-tile-contrast" || !strings.Contains(got[0].Value, "2.9") {
		t.Errorf("a 55%% opacity tile gave %v, want one icon-on-tile-contrast finding near 2.98:1", got)
	}
	solid := faded
	solid[0].Op = 1
	if got := labContrastFindings("t", "s", nil, solid, nil); len(got) != 0 {
		t.Errorf("the same tile at full opacity gave %v", got)
	}
	weakMark := []labElement{{Probe: "disc", Role: "mark", Name: "disc", Own: grey, Behind: white}}
	if got := labContrastFindings("t", "s", nil, weakMark, nil); len(got) != 1 || got[0].Check != "mark-contrast" {
		t.Errorf("a 2.96:1 disc gave %v, want one mark-contrast finding", got)
	}

	// two identical colours break the OKLab distance; the grey statuses are not references
	blue := &[3]float64{40, 90, 220}
	els := []labElement{{Probe: "tile", Role: "tile", Name: "tile[cat=web]", Own: blue}}
	for _, v := range []string{"ok", "failed", "running", "uncertain"} {
		els = append(els, labElement{Probe: "ref-status", Role: "ref", Name: "disc " + v, Status: v, Own: blue})
	}
	if got := labDistanceFindings("t", "2", els); len(got) < 4 {
		t.Errorf("a tile equal to four statuses gave %d findings, want at least 4", len(got))
	}
	if got := labDistanceFindings("t", "2", els[:2]); len(got) != 1 || got[0].Check != "present" {
		t.Errorf("one status disc gave %v, want the 'present' finding", got)
	}

	// names and clipping
	if got := labNameFindings("t", "s", labScene{Scanned: 3, Unnamed: []labBox{{Name: "button.ed-icon-btn", Text: ""}}}); len(got) != 1 || got[0].Check != "accessible-name" {
		t.Errorf("an unnamed button gave %v", got)
	}
	if got := labNameFindings("t", "s", labScene{}); len(got) != 1 {
		t.Errorf("an empty scan gave %v, want one finding", got)
	}
	clip := labScene{Clipped: []labClip{{Name: "span.ed-rail-caption", Text: "[Vörlägén]", ScrollWidth: 80, ClientWidth: 60, Ellipsis: true}}}
	if got := labClipFindings("t", "s", clip); len(got) != 1 || !strings.Contains(got[0].Value, "scrollWidth 80 > clientWidth 60") {
		t.Errorf("a clipped caption gave %v", got)
	}

	// known findings are split off; an entry needs a reason
	f := labFinding{"amiga", "1-status", "x", "text-contrast", "1", "2"}
	labKnown[f.Key()] = "test"
	failing, known := labSplit([]labFinding{f})
	delete(labKnown, f.Key())
	if len(failing) != 0 || len(known) != 1 {
		t.Errorf("labSplit gave failing %v known %v", failing, known)
	}
	for key, reason := range labKnown {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("labKnown[%q] has no reason", key)
		}
	}
	if labRound(2.456) != 2.46 {
		t.Errorf("labRound(2.456) = %v", labRound(2.456))
	}
}
