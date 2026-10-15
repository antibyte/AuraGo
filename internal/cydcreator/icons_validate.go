package cydcreator

func (v *validator) iconProps(w *Widget, spec *WidgetSpec, bad func(string, string, ...any)) {
	key := "asset"
	if w.Type == "button" {
		key = "icon_asset"
	}
	id, _ := v.propValue(w, spec, key).(string)
	motion, _ := v.propValue(w, spec, "icon_motion").(string)
	frames := v.listProp(w, "icon_frames")
	animated := v.boolProp(w, spec, "icon_animate")
	if len(frames) == 0 && !animated {
		return
	}
	var base *Asset
	for i := range v.p.Assets {
		if v.p.Assets[i].ID == id {
			base = &v.p.Assets[i]
			break
		}
	}
	if base == nil {
		bad(key, "animation needs an image")
		return
	}
	if base.W > 64 || base.H > 64 {
		bad(key, "animated icons may be at most 64 pixels per side")
	}
	if len(frames) != 0 && len(frames) != 6 || animated && motion == "frames" && len(frames) != 6 {
		bad("icon_frames", "a frame animation needs exactly six images")
		return
	}
	for _, frame := range frames {
		found := false
		for _, a := range v.p.Assets {
			if a.ID != frame {
				continue
			}
			found = true
			if a.W != base.W || a.H != base.H || a.Encoding != base.Encoding {
				bad("icon_frames", "frames must match the base image size and encoding")
			}
			break
		}
		if !found {
			bad("icon_frames", "an animation frame is missing")
		}
	}
}
