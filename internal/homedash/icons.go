package homedash

import "slices"

// iconNames is the fixed list of HomeDash glyph names (M3 contract, decision D9). Sources send them
// as Entity.IconHint, templates set them as Widget.Icon; the frontend draws every name. Names are
// only ever added, together with the frontend glyph set (ui/js/homedash/hd-icons.js) and the contract.
var iconNames = []string{
	// system source
	"cpu", "memory", "disk", "download", "upload", "timer",
	// Home Assistant adapter
	"bulb", "power", "outlet", "scene", "script", "button", "list", "number", "thermometer", "droplet", "gauge", "lock",
	// control glyphs of the W1 widgets and the light sheet (never sent as hints in M3)
	"brightness", "color-temp", "color", "effect",
}

// IconNames returns a copy of the fixed icon-name list.
func IconNames() []string { return slices.Clone(iconNames) }

// KnownIcon reports whether name is on the fixed list.
func KnownIcon(name string) bool { return slices.Contains(iconNames, name) }
