package storymaker

import "math"

// SafeBox is the drawing area of a code scene in 1920x1080 design pixels for an aspect.
type SafeBox struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

const (
	captionBandTop         = 0.794 // top of the two-line caption box, 16:9 and 1:1 (layout.go comment, measured 2026-11-09)
	captionBandTopPortrait = 0.654 // 0.78 box bottom - 0.126 two-line box height; caption sizes are fractions of the height
)

// SafeBoxFor is the part of the 1920x1080 design frame that stays visible in the aspect's cover fit, inside the safe zone and
// above the captions. An unknown aspect is 16:9. ui/js/storymaker/stage.js safeBox repeats it; change both together.
func SafeBoxFor(aspect string) SafeBox {
	w, h, top := 1920.0, 1080.0, captionBandTop
	switch aspect {
	case "9:16":
		w, h, top = 1080, 1920, captionBandTopPortrait
	case "1:1":
		w, h = 1080, 1080
	default:
		aspect = "16:9"
	}
	scale := math.Max(w/1920, h/1080)
	visW, visH := math.Min(1920, w/scale), math.Min(1080, h/scale)
	visX, visY := 960-visW/2, (1080-visH)/2
	z := SafeZone(aspect)
	left := visX + z.Left*visW
	right := visX + (1-z.Right)*visW
	top0 := visY + z.Top*visH
	bottom := math.Min(visY+(1-z.Bottom)*visH, visY+top*visH)
	x := int(math.Ceil(left - 1e-6))
	y := int(math.Ceil(top0 - 1e-6))
	return SafeBox{X: x, Y: y, W: int(math.Floor(right+1e-6)) - x, H: int(math.Floor(bottom+1e-6)) - y}
}
