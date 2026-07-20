package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestGo2RTCConfigModuleUsesSavedConfigurationAndNoBrowserAlerts(t *testing.T) {
	module := string(mustReadUIFile(t, "cfg/go2rtc.js"))
	mainJS := string(mustReadUIFile(t, "js/config/main.js"))
	for _, required := range []string{
		`/api/go2rtc/status`,
		`/api/go2rtc/test`,
		`/api/go2rtc/snapshot`,
		`/api/go2rtc/viewer/`,
		`isDirty`,
	} {
		if !strings.Contains(module, required) {
			t.Fatalf("go2rtc config module missing %q", required)
		}
	}
	if !strings.Contains(mainJS, `go2rtc: { m: 'go2rtc', fn: 'renderGo2RTCSection' }`) {
		t.Fatal("config main module registry is missing go2rtc lazy loading")
	}
	for _, forbidden := range []string{"alert(", "confirm(", "prompt("} {
		if strings.Contains(module, forbidden) {
			t.Fatalf("go2rtc config module uses forbidden browser dialog %q", forbidden)
		}
	}
}

func TestGo2RTCConfigLocalesHaveCompleteKeyCoverage(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	var expected []string
	for _, locale := range locales {
		path := filepath.Join("lang", "config", "go2rtc", locale+".json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var decoded map[string]interface{}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		keys := flattenLocaleKeys("", decoded)
		sort.Strings(keys)
		if expected == nil {
			expected = keys
			continue
		}
		if !reflect.DeepEqual(keys, expected) {
			t.Fatalf("%s keys differ from en\n got: %v\nwant: %v", locale, keys, expected)
		}
	}
}

func flattenLocaleKeys(prefix string, value map[string]interface{}) []string {
	var keys []string
	for key, child := range value {
		current := key
		if prefix != "" {
			current = prefix + "." + key
		}
		if nested, ok := child.(map[string]interface{}); ok {
			keys = append(keys, flattenLocaleKeys(current, nested)...)
			continue
		}
		keys = append(keys, current)
	}
	return keys
}
