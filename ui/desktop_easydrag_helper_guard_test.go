package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// easyDragDirectLaunchExceptions lists the EasyDrag browser test files that may launch a browser
// themselves, each with its reason. It is empty: every test opens its page through
// startEasyDragSmokeWith (or newEasyDragPage). Adding a file here needs a reason that the helper
// options (PrivateBrowser, Motion) cannot give.
var easyDragDirectLaunchExceptions = map[string]string{}

var easyDragDirectLaunch = regexp.MustCompile(`\blauncher\.New\b|\brod\.New\b|\bnewSmokeBrowser(?:With)?\b`)

// easyDragHelperSection is the part of desktop_easydrag_browser_test.go between the helper markers.
var easyDragHelperSection = regexp.MustCompile(`(?s)// easydrag-helpers:begin.*?// easydrag-helpers:end`)

// easyDragDirectLaunches returns the lines of src (comments cut) that launch a browser directly.
func easyDragDirectLaunches(src string) []string {
	var found []string
	for i, line := range strings.Split(src, "\n") {
		if cut := strings.Index(line, "//"); cut >= 0 {
			line = line[:cut]
		}
		if easyDragDirectLaunch.MatchString(line) {
			found = append(found, strings.TrimSpace(line)+" (line "+strconv.Itoa(i+1)+")")
		}
	}
	return found
}

// TestEasyDragBrowserTestsUseTheSmokeHelper keeps every EasyDrag browser test on the shared
// Chrome, with reduced motion set by the helper: no such file launches a browser itself, apart
// from the helper section of desktop_easydrag_browser_test.go and the listed exceptions. It reads
// source only, so it runs without a browser (T0).
func TestEasyDragBrowserTestsUseTheSmokeHelper(t *testing.T) {
	// The scanner itself.
	if got := easyDragDirectLaunches("p := newSmokeBrowser(t).MustPage()\nb := rod.New()\nl := launcher.New() // fine\n// rod.New() in a comment\nx := 1"); len(got) != 3 {
		t.Fatalf("scanner found %d launches in the sample, want 3: %v", len(got), got)
	}

	files, err := filepath.Glob("*easydrag*_browser_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no EasyDrag browser test files found (%v)", err)
	}
	sort.Strings(files)
	for file, reason := range easyDragDirectLaunchExceptions {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("exception %s has no reason", file)
		}
		if _, err := os.Stat(file); err != nil {
			t.Errorf("exception %s names a file that does not exist", file)
		}
	}
	const helperFile = "desktop_easydrag_browser_test.go"
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		if file == helperFile {
			if len(easyDragHelperSection.FindAllString(src, -1)) != 1 {
				t.Fatalf("%s must hold exactly one easydrag-helpers:begin ... easydrag-helpers:end section", file)
			}
			src = easyDragHelperSection.ReplaceAllString(src, "")
		}
		if _, listed := easyDragDirectLaunchExceptions[file]; listed {
			continue
		}
		for _, hit := range easyDragDirectLaunches(src) {
			t.Errorf("%s launches a browser itself: %s; use startEasyDragSmokeWith (PrivateBrowser: true for an own Chrome)", file, hit)
		}
	}
}
