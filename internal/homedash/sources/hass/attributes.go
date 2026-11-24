package hass

import (
	"math"
	"strings"

	"aurago/internal/homedash"
)

var attributeAllowlist = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`unit_of_measurement device_class state_class color_mode supported_color_modes
		color_temp_kelvin min_color_temp_kelvin max_color_temp_kelvin hs_color effect effect_list current_temperature
		temperature target_temp_low target_temp_high target_temp_step min_temp max_temp hvac_modes hvac_action
		preset_mode preset_modes fan_mode fan_modes current_position current_tilt_position percentage percentage_step
		oscillating direction media_title media_artist source source_list volume_level is_volume_muted options min max
		step mode battery_level installed_version latest_version next_rising next_setting elevation humidity pressure
		wind_speed wind_bearing`) {
		attributeAllowlist[k] = true
	}
}

func deniedAttribute(key string) bool {
	switch key {
	case "entity_picture", "access_token", "latitude", "longitude", "gps_accuracy":
		return true
	}
	return strings.Contains(key, "url") || strings.Contains(key, "token") || strings.Contains(key, "password")
}

// FilterAttributes keeps the allowlisted attributes (brightness becomes brightness_pct) bounded by
// homedash.BoundAttributes, and returns the cleaned friendly name separately.
func FilterAttributes(attrs map[string]any) (string, map[string]any) {
	name, kept := "", map[string]any{}
	for k, v := range attrs {
		switch {
		case deniedAttribute(k):
		case k == "friendly_name":
			if s, ok := v.(string); ok {
				name = homedash.CleanText(s, homedash.MaxNameRunes)
			}
		case k == "brightness":
			if f, ok := v.(float64); ok {
				kept["brightness_pct"] = math.Round(f / 255 * 100)
			}
		case attributeAllowlist[k]:
			kept[k] = v
		}
	}
	return name, homedash.BoundAttributes(kept)
}
