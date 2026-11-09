package storymaker

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
