package videostudio

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Crop, flip and effect-stack validation (plan 2c, plan index §4.1 and §4.2). JS VideoStudioEffects.validPicture
// mirrors every rule here (TestVideoStudioPictureValidationMatchesGo, ui).

// maxCropFraction bounds each crop side and the sums l+r and t+b (spec §5.3). The sums get 1e-9 of float slack, so
// 0.9 + 0.05 passes. JS uses the same tolerance.
const maxCropFraction = 0.95

// FieldError is a validation error with the path of the field it concerns. Paths look like:
//   - clips/<id>/crop, clips/<id>/flip_h and clips/<id>/flip_v;
//   - clips/<id>/blend, clips/<id>/mask and clips/<id>/mask/<field> (2d);
//   - clips/<id>/anim_in, clips/<id>/anim_in/preset and clips/<id>/anim_in/duration, and anim_out alike (2e);
//   - clips/<id>/effects (stack limits), clips/<id>/effects/<i>, clips/<id>/effects/<i>/params and
//     clips/<id>/effects/<i>/params/<param>;
//   - clips/<id>/effects/<i>/keyframes/<param>;
//   - assets/<id>, assets/<id>/lut_size and assets (LUT count).
//
// The project PUT reports Path as "field" next to invalid_project (writeVideoStudioErrorFields).
type FieldError struct {
	Path string
	Err  error
}

func (e *FieldError) Error() string { return e.Path + ": " + e.Err.Error() }

func (e *FieldError) Unwrap() error { return e.Err }

// fieldError returns a *FieldError at path with a formatted message.
func fieldError(path, format string, args ...any) error {
	return &FieldError{Path: path, Err: fmt.Errorf(format, args...)}
}

// validateCrop checks a clip's crop and flips; both belong to visual clips only. A crop holds fractions of the
// uncropped source with these rules:
//   - each side lies between 0 and 0.95;
//   - l+r and t+b are at most 0.95;
//   - the sides are not all zero (no crop is nil, so the canonical form stays unique).
//
// Crop keyframes are registry base paths, so validateKeyframes checks them.
func validateCrop(track Track, clip Clip) error {
	path := "clips/" + clip.ID
	if track.Kind == TrackAudio {
		switch {
		case clip.Crop != nil:
			return fieldError(path+"/crop", "clip %q on an audio track cannot be cropped", clip.ID)
		case clip.FlipH:
			return fieldError(path+"/flip_h", "clip %q on an audio track cannot be flipped", clip.ID)
		case clip.FlipV:
			return fieldError(path+"/flip_v", "clip %q on an audio track cannot be flipped", clip.ID)
		}
		return nil
	}
	crop := clip.Crop
	if crop == nil {
		return nil
	}
	for _, side := range []float64{crop.L, crop.T, crop.R, crop.B} {
		if !finite(side) || side < 0 || side > maxCropFraction {
			return fieldError(path+"/crop", "clip %q crop sides must be between 0 and %v", clip.ID, maxCropFraction)
		}
	}
	if crop.L+crop.R > maxCropFraction+1e-9 || crop.T+crop.B > maxCropFraction+1e-9 {
		return fieldError(path+"/crop", "clip %q crop must keep at least 5 %% of the picture on each axis", clip.ID)
	}
	if *crop == (Crop{}) {
		return fieldError(path+"/crop", "clip %q has an empty crop; leave crop out instead", clip.ID)
	}
	return nil
}

// maskPaths are the keyframable mask paths (the base rows mask.x|y|w|h|rotation|feather, 2d) in registry order. JS:
// VideoStudioEffects.MASK_PATHS.
var maskPaths = []string{"mask.x", "mask.y", "mask.w", "mask.h", "mask.rotation", "mask.feather"}

// validateBlend checks a clip's blend mode (2d, plan index §4.1): visual clips only, a registry mode, and never the name
// "normal". An absent blend is normal, so refusing the explicit name keeps the canonical form unique; the editor
// deletes the field instead.
func validateBlend(track Track, clip Clip) error {
	if clip.Blend == "" {
		return nil
	}
	path := "clips/" + clip.ID + "/blend"
	switch {
	case track.Kind == TrackAudio:
		return fieldError(path, "clip %q on an audio track cannot have a blend mode", clip.ID)
	case clip.Blend == BlendNormal:
		return fieldError(path, "clip %q: normal is the default blend mode; leave blend out instead", clip.ID)
	}
	if _, ok := BlendModeByID(clip.Blend); !ok {
		return fieldError(path, "clip %q has an unknown blend mode %q", clip.ID, clip.Blend)
	}
	return nil
}

// validateMask checks a clip's mask (2d, plan index §4.1 and §4.2):
//   - visual clips only;
//   - mask.* keyframes only on a clip with a mask (validateKeyframes has already checked their values against the base
//     rows);
//   - shape rect or ellipse;
//   - x, y, w, h, rotation and feather finite and inside their base rows.
func validateMask(track Track, clip Clip) error {
	path := "clips/" + clip.ID + "/mask"
	mask := clip.Mask
	if mask == nil {
		for _, key := range maskPaths {
			if len(clip.Keyframes[key]) > 0 {
				return fieldError(path, "clip %q has mask keyframes but no mask", clip.ID)
			}
		}
		return nil
	}
	if track.Kind == TrackAudio {
		return fieldError(path, "clip %q on an audio track cannot have a mask", clip.ID)
	}
	if mask.Shape != MaskRect && mask.Shape != MaskEllipse {
		return fieldError(path+"/shape", "clip %q mask shape must be rect or ellipse", clip.ID)
	}
	for i, value := range []float64{mask.X, mask.Y, mask.W, mask.H, mask.Rotation, mask.Feather} {
		spec, _ := BaseParam(maskPaths[i])
		if !finite(value) || value < spec.Min || value > spec.Max {
			name := strings.TrimPrefix(maskPaths[i], "mask.")
			return fieldError(path+"/"+name, "clip %q mask %s must be between %v and %v", clip.ID, name, spec.Min, spec.Max)
		}
	}
	return nil
}

// validateAnim checks a clip's in and out animations (2e, plan index §4.1, §4.24):
//   - visual clips only (clips/<id>/anim_in, clips/<id>/anim_out);
//   - a registry preset, never empty (…/preset). An unavailable preset validates; only the export refuses it;
//   - a duration inside AnimFrameRange(rate) (…/duration).
//
// The duration is never checked against the clip length: the render clamps each span to half the clip, so trims never
// make a project invalid. JS: VideoStudioAnim.valid (Task 5).
func validateAnim(track Track, clip Clip, rate Rate) error {
	minFrames, maxFrames, _ := AnimFrameRange(rate)
	for _, side := range []struct {
		name string
		anim *ClipAnim
	}{{"anim_in", clip.AnimIn}, {"anim_out", clip.AnimOut}} {
		if side.anim == nil {
			continue
		}
		path := "clips/" + clip.ID + "/" + side.name
		if track.Kind == TrackAudio {
			return fieldError(path, "clip %q on an audio track cannot have an animation", clip.ID)
		}
		if side.anim.Preset == "" {
			return fieldError(path+"/preset", "clip %q %s needs a preset", clip.ID, side.name)
		}
		if _, ok := AnimPresetByID(side.anim.Preset); !ok {
			return fieldError(path+"/preset", "clip %q %s has an unknown preset %q", clip.ID, side.name, side.anim.Preset)
		}
		if side.anim.Duration < minFrames || side.anim.Duration > maxFrames {
			return fieldError(path+"/duration", "clip %q %s duration must be between %d and %d frames", clip.ID, side.name, minFrames, maxFrames)
		}
	}
	return nil
}

// validateEffects checks a clip's effect stack (spec §5.4, §5.8):
//   - Effects belong to visual clips only, at most MaxEffectsPerClip entries.
//   - Each entry names a registry effect and has a params object (never null).
//   - The params keys are parameters of that effect, and their values have the registry type and range. LUT refs name
//     a built-in look or a lut asset of the project.
//   - Entry keyframes sit on keyframable number parameters only (the blur radius is not one). They follow the base
//     rules: 1..64 keys, strictly increasing frames within ±MaxClipSpeed·maxFrames, values in range, known easings.
//   - All keys of the clip, base paths and entries together, stay within MaxKeyframesPerClip.
//
// Entries that are off are validated too, so turning one on never makes a project invalid.
func validateEffects(track Track, clip Clip, assets map[string]Asset, maxFrames int) error {
	if len(clip.Effects) == 0 {
		return nil
	}
	path := "clips/" + clip.ID + "/effects"
	if track.Kind == TrackAudio {
		return fieldError(path, "clip %q on an audio track cannot have effects", clip.ID)
	}
	if len(clip.Effects) > MaxEffectsPerClip {
		return fieldError(path, "clip %q has more than %d effects", clip.ID, MaxEffectsPerClip)
	}
	if countKeyframes(clip) > MaxKeyframesPerClip {
		return fieldError(path, "clip %q has more than %d keyframes", clip.ID, MaxKeyframesPerClip)
	}
	bound := MaxClipSpeed * maxFrames
	for i, entry := range clip.Effects {
		at := path + "/" + strconv.Itoa(i)
		if _, ok := EffectByID(entry.ID); !ok {
			return fieldError(at, "clip %q has an unknown effect %q", clip.ID, entry.ID)
		}
		if entry.Params == nil {
			return fieldError(at+"/params", "clip %q effect %d needs a params object", clip.ID, i)
		}
		for _, name := range slices.Sorted(maps.Keys(entry.Params)) {
			spec, ok := effectParamSpec(entry.ID, name)
			if !ok {
				return fieldError(at+"/params/"+name, "clip %q effect %q has no parameter %q", clip.ID, entry.ID, name)
			}
			if err := validEffectParam(spec, entry.Params[name], assets); err != nil {
				return fieldError(at+"/params/"+name, "clip %q effect %q parameter %q %v", clip.ID, entry.ID, name, err)
			}
		}
		for _, name := range slices.Sorted(maps.Keys(entry.Keyframes)) {
			keyPath := at + "/keyframes/" + name
			spec, ok := effectParamSpec(entry.ID, name)
			if !ok || !spec.Keyframable || spec.Type != ParamNumber {
				return fieldError(keyPath, "clip %q effect %q parameter %q cannot have keyframes", clip.ID, entry.ID, name)
			}
			if err := validEffectKeys(entry.Keyframes[name], spec, bound); err != nil {
				return fieldError(keyPath, "clip %q effect %q keyframes on %q %v", clip.ID, entry.ID, name, err)
			}
		}
	}
	return nil
}

// validEffectParam checks one params value against its registry spec:
//   - number: a finite float64 in range;
//   - lut: a known LUT reference;
//   - bool: a bool;
//   - choice: one of the listed choices;
//   - color: a #rrggbb colour (2d).
func validEffectParam(spec ParamSpec, value any, assets map[string]Asset) error {
	switch spec.Type {
	case ParamNumber:
		number, ok := value.(float64)
		if !ok || !finite(number) || number < spec.Min || number > spec.Max {
			return fmt.Errorf("must be a number between %v and %v", spec.Min, spec.Max)
		}
	case ParamLUT:
		ref, ok := value.(string)
		if !ok || !validLUTRef(ref, assets) {
			return fmt.Errorf("must be empty, a built-in look or a LUT of the project")
		}
	case ParamBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("must be true or false")
		}
	case ParamChoice:
		choice, ok := value.(string)
		if !ok || !slices.Contains(spec.Choices, choice) {
			return fmt.Errorf("must be one of %v", spec.Choices)
		}
	case ParamColor:
		color, ok := value.(string)
		if !ok || !validHexColor(color) {
			return fmt.Errorf("must be a #rrggbb colour")
		}
	default:
		return fmt.Errorf("has an unsupported type %q", spec.Type)
	}
	return nil
}

// validEffectKeys checks one keyframe list of an effect parameter with the base rules of validateKeyframes.
func validEffectKeys(list []Keyframe, spec ParamSpec, bound int) error {
	if len(list) == 0 {
		return fmt.Errorf("are empty")
	}
	if len(list) > MaxKeyframesPerParam {
		return fmt.Errorf("exceed %d", MaxKeyframesPerParam)
	}
	for i, key := range list {
		switch {
		case i > 0 && key.F <= list[i-1].F:
			return fmt.Errorf("must have strictly increasing frames")
		case key.F < -bound || key.F > bound:
			return fmt.Errorf("have frame %d out of range", key.F)
		case !finite(key.V) || key.V < spec.Min || key.V > spec.Max:
			return fmt.Errorf("have an invalid value %v", key.V)
		case !validEasing(key.E):
			return fmt.Errorf("have an unsupported easing %q", key.E)
		}
	}
	return nil
}

// validLUTRef accepts three kinds of reference: "" (no LUT, pass-through), "builtin:<look id>" of the registry, or the
// id of a lut asset of the project.
func validLUTRef(ref string, assets map[string]Asset) bool {
	if ref == "" {
		return true
	}
	if look, ok := strings.CutPrefix(ref, "builtin:"); ok {
		_, known := LookByID(look)
		return known
	}
	asset, ok := assets[ref]
	return ok && asset.Kind == AssetLUT
}

// validHexColor reports whether value is a #rrggbb colour.
func validHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, r := range value[1:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}
