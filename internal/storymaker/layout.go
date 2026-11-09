package storymaker

import (
	"fmt"
	"math"
)

// LayoutBox is a rectangle in fractions of the output frame: x, y, width, height. The player measures it at the hold frame
// of a scene, under the scene's camera.
type LayoutBox [4]float64

// LayoutActor is the box of one actor on stage.
type LayoutActor struct {
	ID  string    `json:"id"`
	Box LayoutBox `json:"box"`
}

// LayoutProp is the box of one shown prop. Hero is the first cue-bound prop of the scene, Back a prop behind the actors,
// Scale the scale it carries (1 when none).
type LayoutProp struct {
	ID    string    `json:"id"`
	Item  string    `json:"item"`
	Hero  bool      `json:"hero"`
	Back  bool      `json:"back"`
	Scale float64   `json:"scale"`
	Box   LayoutBox `json:"box"`
}

// LayoutText is the box of one shown text card.
type LayoutText struct {
	ID  string    `json:"id"`
	Box LayoutBox `json:"box"`
}

// LayoutScene is the measured layout of one storyboard scene at its hold frame. Caption is the burned caption of that
// moment, nil when none is on screen. Missing counts boxes the player could not measure for lack of art. Static says that
// nothing on the stage changed between the first frame and the hold frame, apart from the idle cycle.
type LayoutScene struct {
	ID      string        `json:"id"`
	Index   int           `json:"index"`
	HoldS   float64       `json:"hold_s"`
	Actors  []LayoutActor `json:"actors"`
	Props   []LayoutProp  `json:"props"`
	Texts   []LayoutText  `json:"texts"`
	Caption *LayoutBox    `json:"caption"`
	Missing int           `json:"missing"`
	Static  bool          `json:"static"`
}

// The thresholds of the layout findings, in fractions of the frame. They come from the geometry measured on 2026-11-09 and
// are checked on a real run (task E6): the two-line caption box covers 79.4 to 92 percent of the height in 16:9 and 1:1.
const (
	layoutFootStrip     = 0.12 // the lower part of an actor or prop box that counts as its foot or base
	layoutCaptionSlack  = 0.004
	layoutHeroMinHeight = 0.12 // a hero prop below this share of the frame height is too small
	layoutHeroTarget    = 0.20 // the share the fix text asks for
	layoutOutsideShare  = 0.25 // actors and props: this share of the box outside the frame is too much
	layoutTextOutside   = 0.01 // texts must lie inside the frame
	layoutOverlapShare  = 0.35 // of the smaller box
	layoutScaleMax      = 2.0  // the validated range of a beat scale
)

// LayoutOptions say which rules apply. CaptionRule is true for a composition v2 film with captions on.
type LayoutOptions struct {
	CaptionRule bool
}

// IssueLayout says whether a code is a finding of the layout run. Like the craft codes they are model-fixable and never
// block an outcome.
func IssueLayout(code string) bool {
	switch code {
	case CodeCaptionOverlap, CodePropTooSmall, CodeOffscreen, CodeActorPropOverlap, CodeSceneStatic:
		return true
	}
	return false
}

// LayoutIssues turns the measured boxes into findings: an actor foot or a prop base inside the caption band, a hero prop that
// is too small, a box outside the frame, an actor that hides a prop or a text, and a scene in which nothing changes. They are
// warnings with a fix that names the field and the range.
func LayoutIssues(scenes []LayoutScene, opts LayoutOptions) []Issue {
	var out []Issue
	for i := range scenes {
		sc := &scenes[i]
		path := jp("scenes", sc.Index)
		add := func(code, message, fix string) {
			out = append(out, Issue{Severity: SeverityWarning, SceneID: sc.ID, Path: path, Code: code, Message: message, Fix: fix})
		}
		if opts.CaptionRule && sc.Caption != nil {
			for _, a := range sc.Actors {
				if footInCaption(a.Box, *sc.Caption) {
					add(CodeCaptionOverlap, fmt.Sprintf("The caption covers the feet of %s in scene %s.", a.ID, sc.ID),
						`Move the actor to a base spot ("left", "center" or "right") with patch_scene and the whole actors list, or shorten the narration so the caption has one line.`)
				}
			}
			for _, p := range sc.Props {
				if !p.Back && footInCaption(p.Box, *sc.Caption) {
					add(CodeCaptionOverlap, fmt.Sprintf("The caption covers the base of %s in scene %s.", p.ID, sc.ID),
						`Show the prop at a higher spot such as "board" or "center" with patch_scene and the whole beats list.`)
				}
			}
		}
		for _, p := range sc.Props {
			if p.Hero && p.Box[3] < layoutHeroMinHeight {
				scale := min(layoutScaleMax, round1(max(p.Scale, 0.5)*layoutHeroTarget/max(p.Box[3], 0.01)))
				add(CodePropTooSmall, fmt.Sprintf("The prop %s in scene %s is only %.0f percent of the frame height.", p.ID, sc.ID, p.Box[3]*100),
					fmt.Sprintf(`Make it bigger: send patch_scene with the whole beats list and "scale":%.1f on the show beat of %s (scale takes 0.5 to 2).`, scale, p.Item))
			}
		}
		for _, a := range sc.Actors {
			if outsideShare(a.Box) > layoutOutsideShare {
				add(CodeOffscreen, fmt.Sprintf("%s is partly outside the frame in scene %s.", a.ID, sc.ID), `Move the actor to a spot inside the frame such as "left", "center" or "right".`)
			}
		}
		for _, p := range sc.Props {
			if outsideShare(p.Box) > layoutOutsideShare {
				add(CodeOffscreen, fmt.Sprintf("The prop %s is partly outside the frame in scene %s.", p.ID, sc.ID), `Show it at a spot inside the frame such as "center" or "right".`)
			}
		}
		for _, tx := range sc.Texts {
			if outsideShare(tx.Box) > layoutTextOutside {
				add(CodeOffscreen, fmt.Sprintf("The text %s sticks out of the frame in scene %s.", tx.ID, sc.ID), `Shorten the text to five words or fewer, or show it at "center".`)
			}
		}
		for _, a := range sc.Actors {
			for _, p := range sc.Props {
				if !p.Back && overlapShare(a.Box, p.Box) > layoutOverlapShare {
					add(CodeActorPropOverlap, fmt.Sprintf("%s hides the prop %s in scene %s.", a.ID, p.ID, sc.ID), `Show the prop on a spot next to the actor, for example "center" when the actor stands at "left".`)
				}
			}
			for _, tx := range sc.Texts {
				if overlapShare(a.Box, tx.Box) > layoutOverlapShare {
					add(CodeActorPropOverlap, fmt.Sprintf("%s hides the text %s in scene %s.", a.ID, tx.ID, sc.ID), `Show the text at "center" or "board", away from the actor.`)
				}
			}
		}
		if sc.Static {
			add(CodeSceneStatic, fmt.Sprintf("Nothing shows, moves or changes in scene %s.", sc.ID),
				`Add a show beat with a prop and a gesture beat with "with":"prev" using patch_scene and the whole beats list.`)
		}
	}
	return out
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// footInCaption is true when the lower strip of a box meets the caption box. The top of the caption gets a small slack.
func footInCaption(box, caption LayoutBox) bool {
	strip := LayoutBox{box[0], box[1] + box[3]*(1-layoutFootStrip), box[2], box[3] * layoutFootStrip}
	top := LayoutBox{caption[0], caption[1] + layoutCaptionSlack, caption[2], caption[3] - layoutCaptionSlack}
	return intersection(strip, top) > 0
}

func area(b LayoutBox) float64 { return max(0, b[2]) * max(0, b[3]) }

// intersection is the area of two boxes' overlap.
func intersection(a, b LayoutBox) float64 {
	w := min(a[0]+a[2], b[0]+b[2]) - max(a[0], b[0])
	h := min(a[1]+a[3], b[1]+b[3]) - max(a[1], b[1])
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

// outsideShare is the share of a box's area outside the frame [0,1] x [0,1].
func outsideShare(b LayoutBox) float64 {
	if area(b) == 0 {
		return 0
	}
	return 1 - intersection(b, LayoutBox{0, 0, 1, 1})/area(b)
}

// overlapShare is the overlap as a share of the smaller box.
func overlapShare(a, b LayoutBox) float64 {
	smaller := min(area(a), area(b))
	if smaller == 0 {
		return 0
	}
	return intersection(a, b) / smaller
}
