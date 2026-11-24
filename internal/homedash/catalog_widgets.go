package homedash

import "strconv"

func variant(id string, minW, minH, defW, defH int) Variant {
	return Variant{ID: id, MinSize: Size{minW, minH}, DefaultSize: Size{defW, defH}}
}

// aliased marks the families that draw the variant with another primitive.
func aliased(v Variant, family, to string) Variant {
	v.FamilyAlias = map[string]string{family: to}
	return v
}

func bind(name string, required bool, kinds ...Kind) BindingSpec {
	if kinds == nil {
		kinds = []Kind{}
	}
	return BindingSpec{Name: name, Required: required, Kinds: kinds}
}

func bindCap(c Capability, name string, required bool, kinds ...Kind) BindingSpec {
	b := bind(name, required, kinds...)
	b.AnyCapability = []Capability{c}
	return b
}

func bindNum(name string, required bool, kinds ...Kind) BindingSpec {
	return bindCap(CapNumeric, name, required, kinds...)
}

func bindHist(name string, required bool) BindingSpec {
	return bindCap(CapHistory, name, required)
}

func optBool(key string, def bool) OptionSpec {
	return OptionSpec{Key: key, Type: "bool", Default: def}
}

func optInt(key string, def, lo, hi int) OptionSpec {
	l, h := float64(lo), float64(hi)
	return OptionSpec{Key: key, Type: "int", Default: def, Min: &l, Max: &h}
}

// optNumber is a number option; a nil default means "unset".
func optNumber(key string, def any) OptionSpec {
	return OptionSpec{Key: key, Type: "number", Default: def}
}

func optEnum(key, def string, values ...string) OptionSpec {
	return OptionSpec{Key: key, Type: "enum", Default: def, Enum: values}
}

func optColor(key, def string) OptionSpec {
	return OptionSpec{Key: key, Type: "color", Default: def}
}

func optPage(key string, required bool) OptionSpec {
	return OptionSpec{Key: key, Type: "page", Default: "", Required: required}
}

func onOff() []Capability { return []Capability{CapToggle, CapTurnOn, CapTurnOff} }

func caps(c ...Capability) []Capability { return c }

func cmds(pairs ...any) map[string][]Capability {
	m := map[string][]Capability{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i].(string)] = pairs[i+1].([]Capability)
	}
	return m
}

func variants(v ...Variant) []Variant         { return v }
func bindings(b ...BindingSpec) []BindingSpec { return append([]BindingSpec{}, b...) }
func options(o ...OptionSpec) []OptionSpec    { return append([]OptionSpec{}, o...) }

func anyBindings(prefix string, from, to int, required int) []BindingSpec {
	var out []BindingSpec
	for i := from; i <= to; i++ {
		out = append(out, bind(prefix+strconv.Itoa(i), i <= required))
	}
	return out
}

// widgetTypes is the version 1 catalog of main design section 4.3, in table order.
func widgetTypes() []WidgetType {
	toggles := []Kind{KindSwitch, KindLight, KindFan, KindHumidifier}
	return []WidgetType{
		{Type: "switch", Category: "control", MaxSize: Size{4, 4},
			Variants: variants(variant("pill", 1, 1, 2, 1), variant("rocker", 1, 2, 2, 2), variant("push", 1, 1, 2, 1), variant("tile", 1, 1, 2, 2)),
			Bindings: bindings(bind("primary", true, KindSwitch, KindLight, KindFan, KindHumidifier)),
			Options:  options(),
			Commands: cmds("primary", onOff())},
		{Type: "light", Category: "control", MaxSize: Size{6, 4},
			Variants: variants(variant("slider", 2, 1, 4, 1), variant("fader", 1, 2, 1, 3), variant("dial", 2, 2, 2, 2), variant("tile", 1, 1, 2, 2)),
			Bindings: bindings(bind("primary", true, KindLight)),
			Options:  options(),
			Commands: cmds("primary", caps(CapToggle, CapTurnOn, CapTurnOff, CapBrightness, CapColorTemp, CapColor, CapEffect))},
		{Type: "climate", Category: "climate", MaxSize: Size{6, 6},
			Variants: variants(variant("dial", 2, 2, 3, 3), variant("stepper", 2, 1, 2, 1), variant("scale", 1, 2, 1, 3)),
			Bindings: bindings(bind("primary", true, KindClimate)),
			Options:  options(),
			Commands: cmds("primary", caps(CapSetTemperature, CapSetTemperatureRange, CapHVACMode, CapPresetMode, CapFanMode))},
		{Type: "temperature", Category: "climate", MaxSize: Size{4, 4},
			Variants: variants(variant("numeral", 1, 1, 2, 1), aliased(variant("thermometer", 1, 2, 1, 2), "minimal", "scale"), variant("comfort", 2, 2, 2, 2)),
			Bindings: bindings(bindNum("primary", true, KindSensor), bindNum("humidity", false, KindSensor)),
			Options:  options(optInt("decimals", 1, 0, 2), optBool("show_minmax", true)),
			Commands: cmds()},
		{Type: "sensor", Category: "sensor", MaxSize: Size{6, 4},
			Variants: variants(variant("numeral", 1, 1, 2, 1), variant("sparkline", 2, 1, 2, 2), variant("icon", 1, 1, 1, 1)),
			Bindings: bindings(bind("primary", true, KindSensor)),
			Options:  options(optInt("decimals", 1, 0, 3), optInt("hours", 24, 1, 168)),
			Commands: cmds()},
		{Type: "gauge", Category: "sensor", MaxSize: Size{6, 6},
			Variants: variants(aliased(variant("arc", 2, 2, 2, 2), "tactile", "needle"), variant("needle", 2, 2, 2, 2), variant("bar", 2, 1, 3, 1), variant("tank", 1, 2, 1, 2)),
			Bindings: bindings(bindNum("primary", true, KindSensor)),
			Options: options(optNumber("min", 0.0), optNumber("max", 100.0), optNumber("warn", nil), optNumber("alert", nil),
				optInt("decimals", 0, 0, 3)),
			Commands: cmds()},
		{Type: "binary", Category: "sensor", MaxSize: Size{4, 2},
			Variants: variants(variant("indicator", 1, 1, 2, 1), variant("lamp", 1, 1, 1, 1)),
			Bindings: bindings(bind("primary", true, KindBinarySensor, KindPresence)),
			Options:  options(),
			Commands: cmds()},
		{Type: "cover", Category: "control", MaxSize: Size{4, 6},
			Variants: variants(variant("shutter", 1, 2, 2, 2), variant("buttons", 1, 1, 1, 2)),
			Bindings: bindings(bind("primary", true, KindCover)),
			Options:  options(),
			Commands: cmds("primary", caps(CapOpen, CapClose, CapStop, CapSetPosition, CapSetTilt))},
		{Type: "action", Category: "control", MaxSize: Size{4, 2},
			Variants: variants(variant("button", 1, 1, 2, 1)),
			Bindings: bindings(bind("primary", true, KindScene, KindScript, KindButton)),
			Options:  options(),
			Commands: cmds("primary", caps(CapActivate, CapPress))},
		{Type: "select", Category: "control", MaxSize: Size{6, 2},
			Variants: variants(variant("segmented", 2, 1, 3, 1), variant("list", 1, 1, 2, 1)),
			Bindings: bindings(bind("primary", true, KindSelect)),
			Options:  options(),
			Commands: cmds("primary", caps(CapSelectOption))},
		{Type: "number", Category: "control", MaxSize: Size{4, 2},
			Variants: variants(variant("stepper", 2, 1, 2, 1), variant("slider", 2, 1, 2, 1)),
			Bindings: bindings(bind("primary", true, KindNumber)),
			Options:  options(),
			Commands: cmds("primary", caps(CapSetValue))},
		{Type: "fan", Category: "control", MaxSize: Size{4, 4},
			Variants: variants(variant("speed", 2, 1, 2, 2)),
			Bindings: bindings(bind("primary", true, KindFan)),
			Options:  options(),
			Commands: cmds("primary", caps(CapToggle, CapTurnOn, CapTurnOff, CapSetPercentage, CapPresetMode, CapOscillate))},
		{Type: "lock", Category: "control", MaxSize: Size{6, 2},
			Variants: variants(variant("slide", 2, 1, 3, 1)),
			Bindings: bindings(bind("primary", true, KindLock)),
			Options:  options(),
			Commands: cmds("primary", caps(CapLock, CapUnlock))},
		{Type: "media", Category: "media", MaxSize: Size{8, 4},
			Variants: variants(variant("compact", 2, 1, 4, 1), variant("cover", 2, 2, 3, 2)),
			Bindings: bindings(bind("primary", true, KindMediaPlayer)),
			Options:  options(),
			Commands: cmds("primary", caps(CapPlayPause, CapNext, CapPrevious, CapVolumeSet, CapVolumeMute, CapSourceSelect, CapToggle))},
		{Type: "chart", Category: "sensor", MaxSize: Size{12, 6},
			Variants: variants(variant("line", 3, 2, 4, 2), variant("timeline", 3, 1, 4, 1)),
			Bindings: bindings(bindHist("s1", true), bindHist("s2", false), bindHist("s3", false), bindHist("s4", false)),
			Options:  options(optInt("hours", 24, 1, 168)),
			Commands: cmds()},
		{Type: "camera", Category: "camera", MaxSize: Size{12, 8},
			Variants: variants(variant("frame", 2, 2, 4, 3)),
			Bindings: bindings(bind("primary", true, KindCamera)),
			Options:  options(optBool("live", true), optInt("refresh_seconds", 10, 2, 300)),
			Commands: cmds()},
		{Type: "status_list", Category: "status", MaxSize: Size{6, 8},
			Variants: variants(variant("list", 2, 2, 2, 3)),
			Bindings: bindings(anyBindings("e", 1, 8, 3)...),
			Options:  options(),
			Commands: cmds()},
		{Type: "battery", Category: "status", MaxSize: Size{2, 2},
			Variants: variants(variant("glyph", 1, 1, 1, 1)),
			Bindings: bindings(bindNum("primary", true, KindSensor)),
			Options:  options(),
			Commands: cmds()},
		{Type: "room", Category: "status", MaxSize: Size{6, 4},
			Variants: variants(variant("card", 2, 2, 3, 2)),
			Bindings: bindings(bindNum("temperature", false, KindSensor), bindNum("humidity", false, KindSensor),
				bind("toggle1", true, toggles...), bind("toggle2", true, toggles...), bind("toggle3", false, toggles...), bind("toggle4", false, toggles...)),
			Options:  options(),
			Commands: cmds("toggle1", onOff(), "toggle2", onOff(), "toggle3", onOff(), "toggle4", onOff())},
		{Type: "clock", Category: "layout", MaxSize: Size{6, 4},
			Variants: variants(variant("digital", 1, 1, 2, 1), aliased(variant("analog", 2, 2, 2, 2), "instrument", "digital")),
			Bindings: bindings(),
			Options:  options(optBool("show_seconds", false), optBool("show_date", true)),
			Commands: cmds()},
		{Type: "heading", Category: "layout", MaxSize: Size{12, 2},
			Variants: variants(variant("title", 1, 1, 4, 1)),
			Bindings: bindings(),
			Options:  options(optEnum("align", "start", "start", "center")),
			Commands: cmds()},
		{Type: "nav", Category: "layout", MaxSize: Size{4, 2},
			Variants: variants(variant("button", 1, 1, 1, 1)),
			Bindings: bindings(),
			Options:  options(optPage("page", true)),
			Commands: cmds()},
		{Type: "weather", Category: "climate", MaxSize: Size{8, 4},
			Variants: variants(variant("compact", 2, 1, 2, 1), variant("forecast", 3, 2, 4, 2)),
			Bindings: bindings(bind("primary", true, KindWeather)),
			Options:  options(),
			Commands: cmds()},
		{Type: "entity", Category: "status", MaxSize: Size{4, 2},
			Variants: variants(variant("tile", 1, 1, 2, 1)),
			Bindings: bindings(bind("primary", true)),
			Options:  options(),
			Commands: cmds("primary", caps(CapToggle))},
	}
}
