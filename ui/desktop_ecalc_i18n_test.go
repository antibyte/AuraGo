package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ecalcLocaleKeys lists every desktop.* string of the ECalc app (ECalc phase 1 contract, section 10).
var ecalcLocaleKeys = []string{
	"desktop.app_ecalc",
	"desktop.ecalc_search_placeholder", "desktop.ecalc_favorites", "desktop.ecalc_recent",
	"desktop.ecalc_empty_favorites", "desktop.ecalc_empty_recent", "desktop.ecalc_no_results",
	"desktop.ecalc_favorite_add", "desktop.ecalc_favorite_remove", "desktop.ecalc_copy_results",
	"desktop.ecalc_copied", "desktop.ecalc_reset", "desktop.ecalc_explanation", "desktop.ecalc_source",
	"desktop.ecalc_derived", "desktop.ecalc_eseries_nearest", "desktop.ecalc_eseries_series",
	"desktop.ecalc_lock", "desktop.ecalc_unlock", "desktop.ecalc_unit_auto",
	"desktop.ecalc_error_undetermined", "desktop.ecalc_error_nonfinite", "desktop.ecalc_error_range",
	"desktop.ecalc_error_parse", "desktop.ecalc_error_locked", "desktop.ecalc_settings", "desktop.ecalc_settings_digits",
	"desktop.ecalc_settings_symbols", "desktop.ecalc_symbols_iec", "desktop.ecalc_symbols_ansi",
	"desktop.ecalc_back", "desktop.ecalc_load_failed", "desktop.ecalc_retry", "desktop.ecalc_warnings",
	"desktop.ecalc_inputs", "desktop.ecalc_computed",
	"desktop.ecalc_dim_voltage", "desktop.ecalc_dim_current", "desktop.ecalc_dim_resistance",
	"desktop.ecalc_dim_power", "desktop.ecalc_dim_charge", "desktop.ecalc_dim_energy",
	"desktop.ecalc_dim_number", "desktop.ecalc_dim_gain",
	// Phase 2: the dimensions of the calculator library, the dimmed field, the standard-value combiner and the chart.
	"desktop.ecalc_dim_capacitance", "desktop.ecalc_dim_inductance", "desktop.ecalc_dim_frequency", "desktop.ecalc_dim_time",
	"desktop.ecalc_dim_length", "desktop.ecalc_dim_area", "desktop.ecalc_dim_temperature", "desktop.ecalc_dim_tempdiff",
	"desktop.ecalc_dim_thermal_resistance", "desktop.ecalc_dim_resistivity", "desktop.ecalc_dim_tempco", "desktop.ecalc_dim_ratio",
	"desktop.ecalc_dim_angle", "desktop.ecalc_dim_baudrate", "desktop.ecalc_dim_integer",
	"desktop.ecalc_unused_field", "desktop.ecalc_combos", "desktop.ecalc_combo_single", "desktop.ecalc_combo_series",
	"desktop.ecalc_combo_parallel", "desktop.ecalc_combo_value", "desktop.ecalc_combo_deviation", "desktop.ecalc_chart",
}

func TestDesktopECalcLocaleKeysExistInAllLanguages(t *testing.T) {
	t.Parallel()

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range ecalcLocaleKeys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s is missing %s", path, key)
			}
		}
		if !strings.Contains(values["desktop.ecalc_eseries_nearest"], "{series}") {
			t.Fatalf("%s: desktop.ecalc_eseries_nearest must keep the {series} placeholder: %q", path, values["desktop.ecalc_eseries_nearest"])
		}
		if !strings.Contains(values["desktop.ecalc_combos"], "{series}") {
			t.Fatalf("%s: desktop.ecalc_combos must keep the {series} placeholder: %q", path, values["desktop.ecalc_combos"])
		}
	}
}
