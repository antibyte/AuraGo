package server

import (
	"reflect"
	"testing"
)

func TestNormalizeConfigYAMLValueStringifiesLegacyMapKeys(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"providers": []interface{}{
			map[interface{}]interface{}{
				"id":    "main",
				1:       "numeric-key",
				true:    "bool-key",
				"nested": map[interface{}]interface{}{
					3: "three",
				},
			},
		},
	}

	got := normalizeConfigYAMLMap(input)
	want := map[string]interface{}{
		"providers": []interface{}{
			map[string]interface{}{
				"id":   "main",
				"1":    "numeric-key",
				"true": "bool-key",
				"nested": map[string]interface{}{
					"3": "three",
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized config mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}
