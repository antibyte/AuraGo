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
	"desktop.ecalc_error_parse", "desktop.ecalc_settings", "desktop.ecalc_settings_digits",
	"desktop.ecalc_settings_symbols", "desktop.ecalc_symbols_iec", "desktop.ecalc_symbols_ansi",
	"desktop.ecalc_back", "desktop.ecalc_load_failed", "desktop.ecalc_retry", "desktop.ecalc_warnings",
	"desktop.ecalc_inputs", "desktop.ecalc_computed",
	"desktop.ecalc_dim_voltage", "desktop.ecalc_dim_current", "desktop.ecalc_dim_resistance",
	"desktop.ecalc_dim_power", "desktop.ecalc_dim_charge", "desktop.ecalc_dim_energy",
	"desktop.ecalc_dim_number", "desktop.ecalc_dim_gain",
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
	}
}
