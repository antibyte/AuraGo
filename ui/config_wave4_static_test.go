package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Config UX wave 4 guards. Ratchet files in testdata/ list the CURRENT offenders; a guard fails on a new
// offender and on a stale line (fixed but still listed), so module packages remove their lines when done.

// configRatchetCheck is configBaselineCheck with a hard failure for stale baseline lines.
func configRatchetCheck(t *testing.T, file, updateEnv string, current []string, header string) {
	t.Helper()
	set := map[string]bool{}
	for _, line := range current {
		set[line] = true
	}
	current = current[:0]
	for line := range set {
		current = append(current, line)
	}
	sort.Strings(current)
	if os.Getenv(updateEnv) == "1" {
		body := header + strings.Join(current, "\n")
		if len(current) > 0 {
			body += "\n"
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s rewritten with %d lines", file, len(current))
		return
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ratchet %s missing (run with %s=1): %v", file, updateEnv, err)
	}
	baseline := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			baseline[line] = true
		}
	}
	var added, stale []string
	for _, line := range current {
		if !baseline[line] {
			added = append(added, line)
		}
	}
	for line := range baseline {
		if !set[line] {
			stale = append(stale, line)
		}
	}
	sort.Strings(stale)
	if len(added) > 0 {
		t.Errorf("%d new offenders not in %s (fix them; do not add lines):\n%s", len(added), file, strings.Join(firstN(added, 60), "\n"))
	}
	if len(stale) > 0 {
		t.Errorf("%d fixed lines still listed in %s (remove them, or refresh with %s=1):\n%s", len(stale), file, updateEnv, strings.Join(firstN(stale, 60), "\n"))
	}
	t.Logf("%s: %d offenders", file, len(current))
}

func firstN(lines []string, n int) []string {
	if len(lines) > n {
		return lines[:n]
	}
	return lines
}

// configModuleFiles lists the section modules (ui/cfg/*.js without shared helpers) as name -> source.
func configModuleFiles(t *testing.T) map[string]string {
	t.Helper()
	shared := map[string]bool{"form-builder": true}
	files, err := filepath.Glob("cfg/*.js")
	if err != nil || len(files) == 0 {
		t.Fatalf("no config modules found: %v", err)
	}
	out := map[string]string{}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".js")
		if shared[name] {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = string(raw)
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// The class prefix of the old AdGuard module is retired (E3); shared names are cfg-*.
func TestConfigNoLegacyAdgPrefix(t *testing.T) {
	t.Parallel()
	needle := "adg" + "-"
	var hits []string
	for _, pattern := range []string{"cfg/*.js", "js/*.js", "js/config/*.js", "js/shared/*.js", "css/*.css", "*.html", "*_test.go"} {
		files, _ := filepath.Glob(pattern)
		for _, file := range files {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), needle) {
				hits = append(hits, file)
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("the retired %q class/id prefix is used again (use cfg-*): %s", needle, strings.Join(hits, ", "))
	}
}

// Emoji and pictographic symbols in module code (E6): icons come from cfgIcon. Content that is genuinely
// emoji (e.g. flags in language lists) is listed here with a reason instead of in the ratchet.
var configEmojiAllowed = map[string]string{}

var configEmojiPattern = regexp.MustCompile("[\\x{1F000}-\\x{1FFFF}\\x{2600}-\\x{27BF}\\x{2B00}-\\x{2BFF}\\x{25A0}-\\x{25FF}\\x{2300}-\\x{23FF}\\x{2190}-\\x{21FF}]")

func TestConfigModulesUseIconsNotEmojiRatchet(t *testing.T) {
	var offenders []string
	modules := configModuleFiles(t)
	for _, name := range sortedKeys(modules) {
		if _, ok := configEmojiAllowed[name]; ok {
			continue
		}
		if configEmojiPattern.MatchString(modules[name]) {
			offenders = append(offenders, name)
		}
	}
	configRatchetCheck(t, "testdata/config_emoji_modules.txt", "AURAGO_UPDATE_CONFIG_RATCHETS", offenders,
		"# <module>: ui/cfg module still contains emoji or pictographic symbols; use cfgIcon (E6). Ends empty.\n")
}

// Native checkboxes are only for "pick entries from a list" and carry data-cfg-choice (E4).
var configCheckboxPattern = regexp.MustCompile(`type\s*=\s*\\?["']checkbox\\?["']`)

func TestConfigNativeCheckboxNeedsChoiceMarkerRatchet(t *testing.T) {
	var offenders []string
	modules := configModuleFiles(t)
	for _, name := range sortedKeys(modules) {
		source := modules[name]
		for _, at := range configCheckboxPattern.FindAllStringIndex(source, -1) {
			start := strings.LastIndex(source[:at[0]], "<input")
			if start < 0 {
				start = at[0]
			}
			end := strings.Index(source[at[1]:], ">")
			tag := source[start:at[1]]
			if end >= 0 {
				tag += source[at[1] : at[1]+end]
			}
			if !strings.Contains(tag, "data-cfg-choice") {
				offenders = append(offenders, name)
				break
			}
		}
	}
	configRatchetCheck(t, "testdata/config_native_checkboxes.txt", "AURAGO_UPDATE_CONFIG_RATCHETS", offenders,
		"# <module>: native checkbox without data-cfg-choice; on/off settings are .toggle (E4).\n")
}

// Own delete, dialog and status-pill class families in modules (E7-E9). Line: <module> <class>.
var configForbiddenFamilies = []struct {
	file, header string
	pattern      *regexp.Regexp
}{
	{
		"testdata/config_delete_classes.txt",
		"# <module> <class>: own delete-button class; use cfgDeleteButton (btn-danger) (E7).\n",
		regexp.MustCompile(`\b(?:wh-btn-danger|cfgdev-icon-btn-danger|em-card-btn-delete|mcp-card-btn-delete|devices-action-btn-delete|sip-danger-button|prov-icon-btn is-delete|[a-z0-9]+-(?:btn|button|icon-btn)-(?:delete|danger)|[a-z0-9]+-delete-(?:btn|button))\b`),
	},
	{
		"testdata/config_modal_families.txt",
		"# <module> <class>: own dialog class family; use cfgModal + openModal/closeModal (E8).\n",
		regexp.MustCompile(`\b[a-z][a-z0-9]*-modal[a-z0-9_-]*`),
	},
	{
		"testdata/config_badge_classes.txt",
		"# <module> <class>: own status badge/pill class; use cfgBadge (E9).\n",
		regexp.MustCompile(`\b[a-z][a-z0-9]*-(?:badge|pill)[a-z0-9_-]*`),
	},
}

// Shared or non-visual names that look like a forbidden family but are not.
var configFamilyAllowed = map[string]bool{"aria-modal": true, "pw-badge": true, "cfg-delete-btn": true}

func TestConfigForbiddenClassFamiliesRatchet(t *testing.T) {
	modules := configModuleFiles(t)
	for _, family := range configForbiddenFamilies {
		family := family
		t.Run(strings.TrimSuffix(strings.TrimPrefix(family.file, "testdata/config_"), ".txt"), func(t *testing.T) {
			var offenders []string
			for _, name := range sortedKeys(modules) {
				for _, token := range family.pattern.FindAllString(modules[name], -1) {
					if !configFamilyAllowed[token] {
						offenders = append(offenders, name+" "+token)
					}
				}
			}
			configRatchetCheck(t, family.file, "AURAGO_UPDATE_CONFIG_RATCHETS", offenders, family.header)
		})
	}
}

// Modules that build their form by hand instead of AuraConfigForm.renderSpec (E12). May only shrink.
func TestConfigHandbuiltModulesRatchet(t *testing.T) {
	modules := configModuleFiles(t)
	var offenders []string
	for _, name := range sortedKeys(modules) {
		if !strings.Contains(modules[name], "renderSpec") {
			offenders = append(offenders, name)
		}
	}
	configRatchetCheck(t, "testdata/config_handbuilt_modules.txt", "AURAGO_UPDATE_CONFIG_RATCHETS", offenders,
		"# <module>: ui/cfg module without AuraConfigForm.renderSpec. Simple modules (fields + test + banner) move to the form builder (E12); big widget modules may stay.\n")
}

// New shared scripts are part of the page.
func TestConfigWave4SharedScriptsLoadedInOrder(t *testing.T) {
	t.Parallel()
	html := string(mustReadUIFile(t, "config.html"))
	last := -1
	for _, script := range []string{"/js/config/feedback.js", "/js/precision/icons.js", "/js/config/icons.js", "/js/config/components.js", "/cfg/form-builder.js"} {
		at := strings.Index(html, script)
		if at < 0 || at < last {
			t.Fatalf("config.html must load %s after the previous shared script", script)
		}
		last = at
	}
}
