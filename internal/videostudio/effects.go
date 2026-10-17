package videostudio

import "math"

// Effect registry (spec §4.2): base properties, effects, transitions, blend modes, in/out animation presets and
// looks, each table in menu order. The status endpoint serves EffectRegistry() to the editor; the DOM-free module
// video-studio-effects.js mirrors it as BUILTIN, and TestVideoStudioEffectsRegistryMatchesGo (ui) keeps the two
// equal. Later slices append rows: effects and looks (2c), blend modes (2d), transitions and animation presets
// (2e). Label keys omit the "videoStudio." prefix; the editor's tr() adds it.

// ParamType is the value type of a base property or effect parameter.
type ParamType string

const (
	ParamNumber ParamType = "number"
	ParamColor  ParamType = "color"
	ParamBool   ParamType = "bool"
	ParamChoice ParamType = "choice"
	ParamLUT    ParamType = "lut"
)

// ParamSpec is one base property or effect parameter.
type ParamSpec struct {
	ID          string    `json:"id"`
	Type        ParamType `json:"type"`
	Min         float64   `json:"min"`
	Max         float64   `json:"max"`
	Default     any       `json:"default"` // float64, string (#rrggbb, choice, lut ref) or bool
	Step        float64   `json:"step,omitempty"`
	Keyframable bool      `json:"keyframable,omitempty"`
	Unit        string    `json:"unit,omitempty"`
	Choices     []string  `json:"choices,omitempty"`
	LabelKey    string    `json:"label_key"` // without the "videoStudio." prefix (tr() adds it)
}

// EffectSpec is one effect of the clip effect stack.
type EffectSpec struct {
	ID       string      `json:"id"`
	Category string      `json:"category"`
	LabelKey string      `json:"label_key"`
	Params   []ParamSpec `json:"params"`
	Filters  []string    `json:"filters"` // FFmpeg filters the export needs (capability check)
	// build is the effect's FFmpeg builder (effects_build.go, plan 2c Tasks 8 and 9). It is unexported, so it is never
	// serialized, and cloneEffect copies it with the struct.
	build effectBuilder
}

// effectBuilder is a placeholder until plan 2c Task 8. That task deletes this declaration and declares the final
// builder type in effects_build.go. Every registry row leaves build nil until Tasks 8 and 9 attach the builders.
type effectBuilder func()

// TransitionSpec is one transition type and the xfade transition the export uses for it. The name avoids a clash
// with the clip's Transition type.
type TransitionSpec struct {
	ID       TransitionType `json:"id"`
	XFade    string         `json:"xfade"`
	Category string         `json:"category"`
	LabelKey string         `json:"label_key"`
}

// BlendModeSpec is one clip blend mode and its FFmpeg blend filter mode (2d).
type BlendModeSpec struct {
	ID       string `json:"id"`
	Blend    string `json:"blend"` // FFmpeg blend mode name (2d)
	LabelKey string `json:"label_key"`
}

// AnimPresetSpec is one in/out animation preset (2e).
type AnimPresetSpec struct {
	ID       string `json:"id"`
	LabelKey string `json:"label_key"`
}

// LookSpec is one built-in look (2c).
type LookSpec struct {
	ID       string `json:"id"`
	LabelKey string `json:"label_key"`
}

// RegistryVersion changes when the registry JSON changes shape, not when rows are added.
const RegistryVersion = 1

// Registry is the JSON the status endpoint serves under "registry".
type Registry struct {
	Version     int              `json:"version"`
	Base        []ParamSpec      `json:"base"`
	Effects     []EffectSpec     `json:"effects"`
	Transitions []TransitionSpec `json:"transitions"`
	BlendModes  []BlendModeSpec  `json:"blend_modes"`
	AnimPresets []AnimPresetSpec `json:"anim_presets"`
	Looks       []LookSpec       `json:"looks"`
}

// UnavailableSet lists the registry ids this FFmpeg cannot export (Registry.Unavailable). All four lists are always
// present in JSON.
type UnavailableSet struct {
	Effects     []string `json:"effects"`
	Transitions []string `json:"transitions"`
	BlendModes  []string `json:"blend_modes"`
	AnimPresets []string `json:"anim_presets"`
}

// registryBase holds the fixed picture properties of a clip. The label keys of x and y are the inspector's existing
// keys box_x and box_y (the fine-tune X and Y fields); crop and flip (2c) use crop_l|t|r|b and flip_h|v. Number
// defaults are float64, as JSON decodes them. 2d appends blend and mask.*.
var registryBase = []ParamSpec{
	{ID: "x", Type: ParamNumber, Min: 0, Max: 1, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "fraction", LabelKey: "box_x"},
	{ID: "y", Type: ParamNumber, Min: 0, Max: 1, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "fraction", LabelKey: "box_y"},
	{ID: "width", Type: ParamNumber, Min: 0, Max: 1, Default: 1.0, Step: 0.001, Keyframable: true, Unit: "fraction", LabelKey: "width"},
	{ID: "height", Type: ParamNumber, Min: 0, Max: 1, Default: 1.0, Step: 0.001, Keyframable: true, Unit: "fraction", LabelKey: "height"},
	{ID: "rotation", Type: ParamNumber, Min: -360, Max: 360, Default: 0.0, Step: 0.1, Keyframable: true, Unit: "deg", LabelKey: "rotation"},
	{ID: "opacity", Type: ParamNumber, Min: 0, Max: 1, Default: 1.0, Step: 0.01, Keyframable: true, LabelKey: "opacity"},
	{ID: "fit", Type: ParamChoice, Default: string(FitContain), Choices: []string{string(FitContain), string(FitCover)}, LabelKey: "fit"},
	{ID: "crop.l", Type: ParamNumber, Min: 0, Max: maxCropFraction, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "fraction", LabelKey: "crop_l"},
	{ID: "crop.t", Type: ParamNumber, Min: 0, Max: maxCropFraction, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "fraction", LabelKey: "crop_t"},
	{ID: "crop.r", Type: ParamNumber, Min: 0, Max: maxCropFraction, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "fraction", LabelKey: "crop_r"},
	{ID: "crop.b", Type: ParamNumber, Min: 0, Max: maxCropFraction, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "fraction", LabelKey: "crop_b"},
	{ID: "flip_h", Type: ParamBool, Default: false, LabelKey: "flip_h"},
	{ID: "flip_v", Type: ParamBool, Default: false, LabelKey: "flip_v"},
}

// registryTransitions keeps the phase 1 ids, export mapping and label keys. A wipe id names the edge the incoming
// clip enters from: "wipeleft" enters from the left, which is xfade's wiperight (the edge moves right), and
// "wiperight" is xfade's wipeleft. 2e appends 20 rows whose ids equal their xfade names (label keys transition_<id>).
var registryTransitions = []TransitionSpec{
	{ID: TransitionDissolve, XFade: "fade", Category: "basic", LabelKey: "dissolve"},
	{ID: TransitionBlack, XFade: "fadeblack", Category: "basic", LabelKey: "black"},
	{ID: TransitionWipeLeft, XFade: "wiperight", Category: "wipe", LabelKey: "wipeleft"},
	{ID: TransitionWipeRight, XFade: "wipeleft", Category: "wipe", LabelKey: "wiperight"},
}

var (
	registryBlendModes  = []BlendModeSpec{}  // 2d
	registryAnimPresets = []AnimPresetSpec{} // 2e
)

// registryEffects is the effect stack menu (2c, plan index §4.2).
//   - Label keys are effect.<id>.name and effect.<id>.<param>; the category label is the locale key
//     effectCategory_<category>.
//   - Colour parameters follow the index table order.
//   - The blur radius is not keyframable: gblur takes no reproducible sigma commands (index §7, risk 16).
//   - The blur filters include premultiply and unpremultiply, which wrap gblur on image clips (plan 2c decision A2).
//
// 2d appends chroma_key (category key).
var registryEffects = []EffectSpec{
	{ID: "color", Category: "color", LabelKey: "effect.color.name", Filters: []string{"lutrgb", "colorchannelmixer"}, Params: []ParamSpec{
		{ID: "exposure", Type: ParamNumber, Min: -3, Max: 3, Default: 0.0, Step: 0.01, Keyframable: true, Unit: "ev", LabelKey: "effect.color.exposure"},
		{ID: "contrast", Type: ParamNumber, Min: 0, Max: 3, Default: 1.0, Step: 0.01, Keyframable: true, LabelKey: "effect.color.contrast"},
		{ID: "saturation", Type: ParamNumber, Min: 0, Max: 3, Default: 1.0, Step: 0.01, Keyframable: true, LabelKey: "effect.color.saturation"},
		{ID: "temperature", Type: ParamNumber, Min: 2000, Max: 12000, Default: 6500.0, Step: 10, Keyframable: true, Unit: "K", LabelKey: "effect.color.temperature"},
		{ID: "tint", Type: ParamNumber, Min: -1, Max: 1, Default: 0.0, Step: 0.01, Keyframable: true, LabelKey: "effect.color.tint"},
		{ID: "lift_x", Type: ParamNumber, Min: -1, Max: 1, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "wheel", LabelKey: "effect.color.lift_x"},
		{ID: "lift_y", Type: ParamNumber, Min: -1, Max: 1, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "wheel", LabelKey: "effect.color.lift_y"},
		{ID: "lift", Type: ParamNumber, Min: -0.5, Max: 0.5, Default: 0.0, Step: 0.001, Keyframable: true, LabelKey: "effect.color.lift"},
		{ID: "gamma_x", Type: ParamNumber, Min: -1, Max: 1, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "wheel", LabelKey: "effect.color.gamma_x"},
		{ID: "gamma_y", Type: ParamNumber, Min: -1, Max: 1, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "wheel", LabelKey: "effect.color.gamma_y"},
		{ID: "gamma", Type: ParamNumber, Min: 0.2, Max: 5, Default: 1.0, Step: 0.01, Keyframable: true, LabelKey: "effect.color.gamma"},
		{ID: "gain_x", Type: ParamNumber, Min: -1, Max: 1, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "wheel", LabelKey: "effect.color.gain_x"},
		{ID: "gain_y", Type: ParamNumber, Min: -1, Max: 1, Default: 0.0, Step: 0.001, Keyframable: true, Unit: "wheel", LabelKey: "effect.color.gain_y"},
		{ID: "gain", Type: ParamNumber, Min: 0, Max: 4, Default: 1.0, Step: 0.01, Keyframable: true, LabelKey: "effect.color.gain"},
	}},
	{ID: "lut", Category: "color", LabelKey: "effect.lut.name", Filters: []string{"lut3d", "blend"}, Params: []ParamSpec{
		{ID: "lut", Type: ParamLUT, Default: "", LabelKey: "effect.lut.lut"},
		{ID: "intensity", Type: ParamNumber, Min: 0, Max: 1, Default: 1.0, Step: 0.01, Keyframable: true, LabelKey: "effect.lut.intensity"},
	}},
	{ID: "blur", Category: "detail", LabelKey: "effect.blur.name", Filters: []string{"gblur", "premultiply", "unpremultiply"}, Params: []ParamSpec{
		{ID: "radius", Type: ParamNumber, Min: 0, Max: 0.05, Default: 0.01, Step: 0.0005, Unit: "height", LabelKey: "effect.blur.radius"},
	}},
	{ID: "sharpen", Category: "detail", LabelKey: "effect.sharpen.name", Filters: []string{"cas"}, Params: []ParamSpec{
		{ID: "strength", Type: ParamNumber, Min: 0, Max: 1, Default: 0.5, Step: 0.01, Keyframable: true, LabelKey: "effect.sharpen.strength"},
	}},
	{ID: "vignette", Category: "stylize", LabelKey: "effect.vignette.name", Filters: []string{"vignette", "alphaextract", "alphamerge"}, Params: []ParamSpec{
		{ID: "amount", Type: ParamNumber, Min: 0, Max: 1, Default: 0.5, Step: 0.01, Keyframable: true, LabelKey: "effect.vignette.amount"},
		{ID: "size", Type: ParamNumber, Min: 0.25, Max: 2, Default: 1.0, Step: 0.01, Keyframable: true, Unit: "height", LabelKey: "effect.vignette.size"},
	}},
}

// registryLooks are the built-in looks (2c). lut.go generates their tables from the colour formula (plan index §4.20).
// Label keys are look_<id>. A lut effect refers to a look as "builtin:<id>".
var registryLooks = []LookSpec{
	{ID: "warm", LabelKey: "look_warm"},
	{ID: "cool", LabelKey: "look_cool"},
	{ID: "teal_orange", LabelKey: "look_teal_orange"},
	{ID: "mono", LabelKey: "look_mono"},
	{ID: "faded_film", LabelKey: "look_faded_film"},
	{ID: "high_contrast", LabelKey: "look_high_contrast"},
	{ID: "vintage", LabelKey: "look_vintage"},
	{ID: "bleach_bypass", LabelKey: "look_bleach_bypass"},
}

// Colour constants (plan index §4.20). JS VideoStudioPicture mirrors them: TestVideoStudioPictureConstantsMatchGo (ui)
// checks the exported ones, and TestColorConstants pins the unexported ones.
const (
	// ColorWheelK scales a colour wheel's x/y offset into the per-channel lift and gain, and into the exponent of the
	// gamma factor.
	ColorWheelK = 0.2
	// ColorNeutralKelvin is the temperature whose white-balance gains are all 1.
	ColorNeutralKelvin = 6500
	// colorTemperatureGain is the exponent of the red (+) and blue (−) gains per ln(T/6500). colorTintGain is the
	// exponent of the green gain per tint unit (+tint is magenta).
	colorTemperatureGain = 0.5
	colorTintGain        = 0.3
)

// lumaBT709 weighs R, G and B into luma. The saturation step and the white-balance normalization use it.
var lumaBT709 = [3]float64{0.2126, 0.7152, 0.0722}

// EffectRegistry returns a deep copy of the registry in menu order. No list is nil.
func EffectRegistry() Registry {
	registry := Registry{
		Version:     RegistryVersion,
		Base:        make([]ParamSpec, len(registryBase)),
		Effects:     make([]EffectSpec, len(registryEffects)),
		Transitions: append([]TransitionSpec{}, registryTransitions...),
		BlendModes:  append([]BlendModeSpec{}, registryBlendModes...),
		AnimPresets: append([]AnimPresetSpec{}, registryAnimPresets...),
		Looks:       append([]LookSpec{}, registryLooks...),
	}
	for i, param := range registryBase {
		registry.Base[i] = cloneParam(param)
	}
	for i, effect := range registryEffects {
		registry.Effects[i] = cloneEffect(effect)
	}
	return registry
}

// TransitionByID returns the registry entry of a transition type.
func TransitionByID(id TransitionType) (TransitionSpec, bool) {
	for _, spec := range registryTransitions {
		if spec.ID == id {
			return spec, true
		}
	}
	return TransitionSpec{}, false
}

// EffectByID returns a copy of an effect's registry entry.
func EffectByID(id string) (EffectSpec, bool) {
	for _, effect := range registryEffects {
		if effect.ID == id {
			return cloneEffect(effect), true
		}
	}
	return EffectSpec{}, false
}

// BaseParam returns a copy of a base property's registry entry.
func BaseParam(id string) (ParamSpec, bool) {
	for _, param := range registryBase {
		if param.ID == id {
			return cloneParam(param), true
		}
	}
	return ParamSpec{}, false
}

// LookByID returns the registry entry of a built-in look (2c).
func LookByID(id string) (LookSpec, bool) {
	for _, look := range registryLooks {
		if look.ID == id {
			return look, true
		}
	}
	return LookSpec{}, false
}

// effectParamSpec returns one parameter of a registry effect without copying the table. It is for read-only callers:
// validation and value lookup.
func effectParamSpec(effectID, param string) (ParamSpec, bool) {
	for _, effect := range registryEffects {
		if effect.ID != effectID {
			continue
		}
		for _, spec := range effect.Params {
			if spec.ID == param {
				return spec, true
			}
		}
		return ParamSpec{}, false
	}
	return ParamSpec{}, false
}

func cloneParam(param ParamSpec) ParamSpec {
	if param.Choices != nil {
		param.Choices = append([]string(nil), param.Choices...)
	}
	return param
}

func cloneEffect(effect EffectSpec) EffectSpec {
	params := make([]ParamSpec, len(effect.Params))
	for i, param := range effect.Params {
		params[i] = cloneParam(param)
	}
	effect.Params = params
	effect.Filters = append(make([]string, 0, len(effect.Filters)), effect.Filters...)
	return effect
}

// Unavailable maps the entries ProbeCapabilities reports ("filter:<name>", "xfade:<name>") to the registry ids the
// editor greys out:
//   - an effect when one of its Filters is missing;
//   - a transition when its xfade transition is missing;
//   - a blend mode with an FFmpeg blend name when the blend filter is missing.
//
// Animation presets (2e) add their rule here. Unknown entries are ignored, and no list is nil.
func (r Registry) Unavailable(missing []string) UnavailableSet {
	lacking := make(map[string]bool, len(missing))
	for _, entry := range missing {
		lacking[entry] = true
	}
	set := UnavailableSet{Effects: []string{}, Transitions: []string{}, BlendModes: []string{}, AnimPresets: []string{}}
	for _, effect := range r.Effects {
		for _, filter := range effect.Filters {
			if lacking["filter:"+filter] {
				set.Effects = append(set.Effects, effect.ID)
				break
			}
		}
	}
	for _, spec := range r.Transitions {
		if lacking["xfade:"+spec.XFade] {
			set.Transitions = append(set.Transitions, string(spec.ID))
		}
	}
	for _, mode := range r.BlendModes {
		if mode.Blend != "" && lacking["filter:blend"] {
			set.BlendModes = append(set.BlendModes, mode.ID)
		}
	}
	return set
}

// fadeBlackWeights splits a fade through black at transition progress p = i/D (transition-local frame i of D, as xfade
// counts it) into the weights of the outgoing track layer, the incoming track layer and opaque black. The export runs
// xfade fadeblack on the premultiplied layers, so both painters compose out·outgoing + inc·incoming +
// black·(0, 0, 0, 1) with these numbers; the GL shader evaluates the same formula. The weights sum to 1.
//
// The curve is not linear. It was fitted to FFmpeg's output (TestFadeBlackMatchesFFmpeg renders every transition
// frame of D=20 and D=8 through the export graph; a 200-frame run read at p steps of 0.005 fixed the edges) and never
// taken from FFmpeg's source. With S(p; a, b) the smoothstep t²(3−2t) of t = clamp((p−a)/(b−a), 0, 1):
//
//	out   = (1 − p) · (1 − S(p; 0, 0.2))
//	inc   = p · S(p; 0, 0.8)
//	black = 1 − out − inc
//
// In words: the outgoing layer is weighted by (1−p) and by an eased fade that is gone at p = 0.2; the incoming layer
// is weighted by p and by an eased rise that is complete at p = 0.8. Between the two, black carries the rest, so
// the picture is at its darkest around p = 0.2 (black ≈ 0.97) and black falls back to exactly 1−p once p ≥ 0.8.
// FFmpeg 5.1.9 (the production floor) lies within 1/255 of the formula; FFmpeg 8.1.1's 8-bit arithmetic truncates and
// sits up to 1.9/255 below it. The test tolerates 2/255. The fit holds for any colour: the planes are mixed linearly
// in code values (checked at levels 255 and 128).
func fadeBlackWeights(p float64) (out, inc, black float64) {
	if !(p > 0) {
		p = 0
	} else if p > 1 {
		p = 1
	}
	out = (1 - p) * (1 - fadeEase(p, 0, 0.2))
	inc = p * fadeEase(p, 0, 0.8)
	return out, inc, 1 - out - inc
}

// fadeRamp rises linearly from 0 at edge0 to 1 at edge1 and is clamped outside.
func fadeRamp(x, edge0, edge1 float64) float64 {
	return math.Max(0, math.Min(1, (x-edge0)/(edge1-edge0)))
}

// fadeEase is fadeRamp eased with smoothstep, t²(3−2t).
func fadeEase(x, edge0, edge1 float64) float64 {
	t := fadeRamp(x, edge0, edge1)
	return t * t * (3 - 2*t)
}

// wipeEdge is the export's wipe geometry: at progress p the incoming layer shows in [left, left+reveal) of a frame
// width pixels wide. The user's "wipeleft" reveals from the left edge (xfade wiperight), "wiperight" from the right
// edge (xfade wipeleft). ok is false for every other transition.
func wipeEdge(kind TransitionType, p, width float64) (left, reveal float64, ok bool) {
	if kind != TransitionWipeLeft && kind != TransitionWipeRight {
		return 0, 0, false
	}
	if !(p > 0) {
		p = 0
	} else if p > 1 {
		p = 1
	}
	width = math.Max(0, width)
	reveal = width * p
	if kind == TransitionWipeLeft {
		return 0, reveal, true
	}
	return width - reveal, reveal, true
}
