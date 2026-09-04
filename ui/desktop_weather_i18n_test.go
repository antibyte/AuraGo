package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

var weatherI18nKeys = []string{
	"desktop.weather_change_location",
	"desktop.weather_geolocation_unavailable",
	"desktop.weather_load_error",
	"desktop.weather_loading",
	"desktop.weather_network_error",
	"desktop.weather_search_city",
	"desktop.weather_set",
	"desktop.weather_use_location",
	"desktop.weather_wind_kmh",
	"desktop.weather_wmo_0",
	"desktop.weather_wmo_1",
	"desktop.weather_wmo_2",
	"desktop.weather_wmo_3",
	"desktop.weather_wmo_45",
	"desktop.weather_wmo_48",
	"desktop.weather_wmo_51",
	"desktop.weather_wmo_53",
	"desktop.weather_wmo_55",
	"desktop.weather_wmo_56",
	"desktop.weather_wmo_57",
	"desktop.weather_wmo_61",
	"desktop.weather_wmo_63",
	"desktop.weather_wmo_65",
	"desktop.weather_wmo_66",
	"desktop.weather_wmo_67",
	"desktop.weather_wmo_71",
	"desktop.weather_wmo_73",
	"desktop.weather_wmo_75",
	"desktop.weather_wmo_77",
	"desktop.weather_wmo_80",
	"desktop.weather_wmo_81",
	"desktop.weather_wmo_82",
	"desktop.weather_wmo_85",
	"desktop.weather_wmo_86",
	"desktop.weather_wmo_95",
	"desktop.weather_wmo_96",
	"desktop.weather_wmo_99",
	"desktop.weather_wmo_unknown",
}

func TestDesktopWeatherWidgetI18n(t *testing.T) {
	t.Parallel()

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"key: 'desktop.weather_wmo_0'",
		"key: 'desktop.weather_wmo_99'",
		"label: t('desktop.weather_wmo_unknown')",
		"label: t(entry.key)",
		"t('desktop.weather_use_location')",
		"t('desktop.weather_change_location')",
		"t('desktop.weather_search_city')",
		"t('desktop.weather_set')",
		"t('desktop.weather_loading')",
		"t('desktop.weather_load_error'",
		"t('desktop.weather_network_error')",
		"t('desktop.weather_geolocation_unavailable')",
		"t('desktop.weather_wind_kmh'",
	} {
		if !strings.Contains(shell, want) {
			t.Fatalf("weather widget i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"label: 'Clear sky'",
		"label: 'Unknown'",
		"title=\"Use my location\"",
		"title=\"Change location\"",
		"placeholder=\"Search city",
		"Loading weather",
		"Could not load weather:",
		"Geolocation not available",
		" km/h",
	} {
		if strings.Contains(shell, forbidden) {
			t.Fatalf("weather widget still hardcodes English %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range weatherI18nKeys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
		}
		if !strings.Contains(values["desktop.weather_load_error"], "{{error}}") {
			t.Fatalf("%s weather load error must keep {{error}}", path)
		}
		if !strings.Contains(values["desktop.weather_wind_kmh"], "{{speed}}") {
			t.Fatalf("%s weather wind must keep {{speed}}", path)
		}
		if lang != "en" && values["desktop.weather_loading"] == "Loading weather…" {
			t.Fatalf("%s must not copy the English weather loading string", path)
		}
	}
}
