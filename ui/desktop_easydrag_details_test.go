package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var easyDragNativeDisclosure = regexp.MustCompile(`(?i)<\s*/?\s*(details|summary)\b`)

// TestEasyDragHasNoNativeDetails: every disclosure in EasyDrag is ED.kit.disclosure (a button head with
// aria-expanded), never a native details/summary element, in any easydrag*.js (comments included).
func TestEasyDragHasNoNativeDetails(t *testing.T) {
	files, err := filepath.Glob("js/desktop/apps/easydrag*.js")
	if err != nil || len(files) < 40 {
		t.Fatalf("found %d easydrag scripts (%v): the glob is wrong", len(files), err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if easyDragNativeDisclosure.MatchString(line) {
				t.Errorf("%s:%d uses a native details/summary element: use ED.kit.disclosure or ED.kit.disclosureMarkup", file, i+1)
			}
		}
	}
}
