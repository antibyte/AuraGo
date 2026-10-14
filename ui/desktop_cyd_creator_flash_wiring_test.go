package ui

import (
	"regexp"
	"strings"
	"testing"
)

// TestCYDCreatorFlashModulesAreWired checks that the plan 5 modules load below the "plan 5" marker in
// overview order before cyd-creator-app.js, that the shell lists them and that app attaches the monitor.
func TestCYDCreatorFlashModulesAreWired(t *testing.T) {
	block := cydCreatorLoaderBlock(t)
	last := strings.Index(block, cydCreator5Marker)
	for _, name := range cydCreatorFlashModules {
		idx := strings.Index(block, "'/js/desktop/apps/"+name+"'")
		if idx < last {
			t.Fatalf("%s must load below the plan 5 marker in overview order", name)
		}
		last = idx
	}
	if app := strings.Index(block, "'/js/desktop/apps/cyd-creator-app.js'"); app < last {
		t.Fatal("cyd-creator-app.js must load after the plan 5 modules")
	}
	listed := map[string]bool{}
	for _, name := range cydCreatorShellModules {
		listed[name] = true
	}
	for _, name := range cydCreatorFlashModules {
		if !listed[name] {
			t.Errorf("cydCreatorShellModules misses %s", name)
		}
	}
	app := rawDesktopAssetText(t, "js/desktop/apps/cyd-creator-app.js")
	for _, marker := range []string{"const monitor = P.monitor.attach(inst);", "monitor.dispose();", "inst.monitor = monitor;"} {
		if !strings.Contains(app, marker) {
			t.Errorf("cyd-creator-app.js misses %q", marker)
		}
	}
}

func TestCYDCreatorFlashStylesheetSection(t *testing.T) {
	css := rawDesktopAssetText(t, "css/desktop-app-cyd-creator.css")
	begin := strings.Index(css, "/* ==== CYD Creator flash, monitor and devices (plan 5) — begin")
	end := strings.Index(css, "/* ==== CYD Creator flash, monitor and devices (plan 5) — end")
	if begin < 0 || end < begin || !strings.Contains(css[:begin], "/* ==== CYD Creator designer (plan 4b) — end") {
		t.Fatal("the plan 5 section must follow the designer section")
	}
	section := css[begin:end]
	if hex := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindString(section); hex != "" {
		t.Fatalf("literal colour %s in the plan 5 section; use the --cc-* tokens", hex)
	}
	for _, class := range []string{".cydc-flasher", ".cydc-monitor-lines", ".cydc-monitor-line.is-backtrace", ".cydc-device-facts", "@container cydc-app"} {
		if !strings.Contains(section, class) {
			t.Errorf("the plan 5 section misses %s", class)
		}
	}
	// The shell's theme-token test stops at the designer section and the designer's guards cover only
	// their own section: this section keeps the same rules (no colour functions, only defined --cc-*
	// tokens, every rule inside the CYD Creator window).
	if fn := regexp.MustCompile(`\b(?:rgba?|hsla?)\(`).FindString(section); fn != "" {
		t.Errorf("literal colour function %s…) in the plan 5 section; use the --cc-* tokens", fn)
	}
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`(--cc-[a-z0-9-]+)\s*:`).FindAllStringSubmatch(css, -1) {
		defined[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`var\((--[a-z0-9-]+)`).FindAllStringSubmatch(section, -1) {
		if !defined[m[1]] {
			t.Errorf("the plan 5 section reads %s, which is no --cc-* token of desktop-app-cyd-creator.css", m[1])
		}
	}
	scoped := regexp.MustCompile(`^\.cydc-app(?:$|[\s.:\[>+~])`)
	for _, rule := range cydCSSRules(section) {
		for _, sel := range cydSelectors(rule.selector) {
			if !scoped.MatchString(sel) {
				t.Errorf("plan 5 selector %q is not scoped to .cydc-app", sel)
			}
		}
	}
}
