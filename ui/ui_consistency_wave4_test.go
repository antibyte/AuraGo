package ui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Wave 4: UI consistency cleanup. Static guards plus count ratchets.
//
// The ratchets (testdata/wave4_{colours,fontsizes,breakpoints,emoji}.txt) hold
// one "<file> <count>" line per file that still has violations. A file may
// never exceed its baseline; lower counts are reported so the baseline can be
// tightened. Refresh them after fixing violations with
//
//	AURAGO_UPDATE_WAVE4_BASELINES=1 go test ./ui/ -run TestUIWave4
//
// Counts only go down: never refresh a baseline to make a new violation pass.

const wave4UpdateEnv = "AURAGO_UPDATE_WAVE4_BASELINES"

// Operational page stylesheets plus the entry pages (setup is baseline-only).
var wave4PageCSS = []string{
	"css/setup.css", "css/login.css", "css/not-found.css",
	"css/dashboard.css", "css/dashboard-shell.css", "css/gallery.css", "css/truenas.css",
	"css/containers.css", "css/plans.css", "css/missions.css", "css/media.css",
	"css/skills.css", "css/knowledge.css", "css/invasion.css", "css/cheatsheets.css",
}

var wave4OperationalPages = map[string]string{
	"cheatsheets.html": "js/cheatsheets/main.js", "containers.html": "js/containers/main.js",
	"dashboard.html": "", "invasion_control.html": "js/invasion/main.js", "knowledge.html": "js/knowledge/main.js",
	"media.html": "js/media/main.js", "missions_v2.html": "js/missions/main.js", "plans.html": "js/plans/main.js",
	"skills.html": "js/skills/main.js", "truenas.html": "js/truenas.js",
}

var wave4Breakpoints = map[float64]bool{360: true, 480: true, 640: true, 768: true, 900: true, 1024: true, 1200: true}

var (
	wave4CommentRe   = regexp.MustCompile(`(?s)/\*.*?\*/`)
	wave4ColourRe    = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(`)
	wave4FontSizeRe  = regexp.MustCompile(`font-size\s*:\s*([^;}\n]+)`)
	wave4FontLitRe   = regexp.MustCompile(`\d+(?:\.\d+)?(?:rem|px|em|%|vw|vh)\b`)
	wave4MediaRe     = regexp.MustCompile(`@media[^{]*`)
	wave4WidthRe     = regexp.MustCompile(`\b(max|min)-width\s*:\s*([\d.]+)(px|em|rem)`)
	wave4RangeRe     = regexp.MustCompile(`\bwidth\s*(<=|>=|<|>)\s*([\d.]+)(px|em|rem)`)
	wave4PwIconRe    = regexp.MustCompile(`(?:pwIcon|AuraIcons\.(?:svg|el|has))\(\s*['"]([a-z0-9-]+)['"]`)
	wave4IconEntryRe = regexp.MustCompile(`(?m)^\s+'([a-z0-9-]+)':\s*'<`)
)

func wave4StripComments(css string) string { return wave4CommentRe.ReplaceAllString(css, "") }

func wave4ReadText(t *testing.T, path string) string {
	t.Helper()
	return strings.ReplaceAll(string(mustReadUIFile(t, path)), "\r\n", "\n")
}

// wave4Ratchet compares current per-file counts with the baseline file.
func wave4Ratchet(t *testing.T, baselineFile, header string, current map[string]int) {
	t.Helper()
	if os.Getenv(wave4UpdateEnv) == "1" {
		var keys []string
		for k, v := range current {
			if v > 0 {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteString(header)
		for _, k := range keys {
			fmt.Fprintf(&b, "%s %d\n", k, current[k])
		}
		if err := os.WriteFile(baselineFile, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s rewritten with %d files", baselineFile, len(keys))
		return
	}
	raw, err := os.ReadFile(baselineFile)
	if err != nil {
		t.Fatalf("baseline %s missing (run with %s=1): %v", baselineFile, wave4UpdateEnv, err)
	}
	baseline := map[string]int{}
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		at := strings.LastIndex(line, " ")
		if at < 0 {
			t.Fatalf("%s: malformed line %q", baselineFile, line)
		}
		n, err := strconv.Atoi(line[at+1:])
		if err != nil {
			t.Fatalf("%s: malformed count in %q", baselineFile, line)
		}
		baseline[line[:at]] = n
	}
	var keys []string
	for k := range current {
		keys = append(keys, k)
	}
	for k := range baseline {
		if _, ok := current[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k] {
			continue
		}
		seen[k] = true
		cur, base := current[k], baseline[k]
		switch {
		case cur > base:
			t.Errorf("%s: %s has %d violations, baseline allows %d (fix them; never raise the baseline)", baselineFile, k, cur, base)
		case cur < base:
			t.Logf("improved: %s %d -> %d (refresh %s with %s=1)", k, base, cur, baselineFile, wave4UpdateEnv)
		}
	}
}

func TestUIWave4ColourLiteralsRatchet(t *testing.T) {
	current := map[string]int{}
	for _, file := range wave4PageCSS {
		css := wave4StripComments(wave4ReadText(t, file))
		count := 0
		for _, line := range strings.Split(css, "\n") {
			if wave4ColourRe.MatchString(line) {
				count++
			}
		}
		current[file] = count
	}
	wave4Ratchet(t, "testdata/wave4_colours.txt",
		"# <css file> <lines with a hex/rgb()/hsl() literal, comments stripped>: wave 4 colour ratchet. Only ever goes down.\n"+
			"# Refresh after fixing with AURAGO_UPDATE_WAVE4_BASELINES=1 go test ./ui/ -run TestUIWave4. shared-components.css is protected and excluded.\n",
		current)
}

func TestUIWave4FontSizeLiteralsRatchet(t *testing.T) {
	current := map[string]int{}
	for _, file := range append([]string{"css/precision-pages.css"}, wave4PageCSS...) {
		css := wave4StripComments(wave4ReadText(t, file))
		count := 0
		for _, m := range wave4FontSizeRe.FindAllStringSubmatch(css, -1) {
			value := strings.TrimSpace(m[1])
			if strings.HasPrefix(value, "var(") || !wave4FontLitRe.MatchString(value) {
				continue
			}
			count++
		}
		current[file] = count
	}
	wave4Ratchet(t, "testdata/wave4_fontsizes.txt",
		"# <css file> <font-size declarations with a literal length instead of a --pw-font-* / --text-* token>: wave 4 type-scale ratchet. Only ever goes down.\n"+
			"# Refresh after fixing with AURAGO_UPDATE_WAVE4_BASELINES=1 go test ./ui/ -run TestUIWave4. Icon/illustration sizes count too; prefer em or a token.\n",
		current)
}

func wave4PxValue(raw, unit string) float64 {
	v, _ := strconv.ParseFloat(raw, 64)
	if unit != "px" {
		v *= 16
	}
	return v
}

// wave4BreakpointViolations counts width terms outside the allowed set. An
// upper bound one pixel (or 0.02px) below an allowed value is accepted when the
// same query also has a min-width (a range query paired with its neighbour).
func wave4BreakpointViolations(css string) int {
	count := 0
	for _, media := range wave4MediaRe.FindAllString(css, -1) {
		paired := strings.Contains(media, "min-width")
		for _, m := range wave4WidthRe.FindAllStringSubmatch(media, -1) {
			v := wave4PxValue(m[2], m[3])
			if wave4Breakpoints[v] {
				continue
			}
			if m[1] == "max" && paired && (wave4Breakpoints[v+1] || wave4Breakpoints[v+0.02]) {
				continue
			}
			count++
		}
		for _, m := range wave4RangeRe.FindAllStringSubmatch(media, -1) {
			if !wave4Breakpoints[wave4PxValue(m[2], m[3])] {
				count++
			}
		}
	}
	return count
}

func TestUIWave4BreakpointsRatchet(t *testing.T) {
	current := map[string]int{}
	for _, file := range append([]string{"css/precision-pages.css", "css/precision-workspace.css"}, wave4PageCSS...) {
		current[file] = wave4BreakpointViolations(wave4StripComments(wave4ReadText(t, file)))
	}
	wave4Ratchet(t, "testdata/wave4_breakpoints.txt",
		"# <css file> <@media width terms outside 360/480/640/768/1024/1200 (+900 reserved for the dashboard drawer)>: wave 4 breakpoint ratchet. Only ever goes down.\n"+
			"# max-width N-1 is allowed when paired with a min-width. Refresh after fixing with AURAGO_UPDATE_WAVE4_BASELINES=1 go test ./ui/ -run TestUIWave4.\n",
		current)
}

func wave4CountEmoji(s string) int {
	n := 0
	for _, r := range s {
		switch {
		case r >= 0x1F000 && r <= 0x1FAFF,
			r >= 0x2600 && r <= 0x27BF,
			r >= 0x2B00 && r <= 0x2BFF,
			r >= 0x231A && r <= 0x23FF,
			r == 0x2934 || r == 0x2935 || r == 0x3030 || r == 0x303D || r == 0x3297 || r == 0x3299:
			n++
		}
	}
	return n
}

func wave4EmojiFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	add := func(root string, match func(string) bool) {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && match(path) {
				files = append(files, filepath.ToSlash(path))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	isJS := func(p string) bool { return strings.HasSuffix(p, ".js") }
	isJSON := func(p string) bool { return strings.HasSuffix(p, ".json") }
	for _, dir := range []string{"cheatsheets", "containers", "dashboard", "gallery", "invasion", "knowledge", "media", "missions", "plans", "skills"} {
		add(filepath.Join("js", dir), isJS)
	}
	files = append(files, "js/truenas.js")
	for page := range wave4OperationalPages {
		files = append(files, page)
	}
	for _, dir := range []string{"cheatsheets", "containers", "dashboard", "gallery", "invasion", "knowledge", "media", "missions", "plans", "skills", "truenas"} {
		add(filepath.Join("lang", dir), isJSON)
	}
	sort.Strings(files)
	return files
}

func TestUIWave4EmojiRatchet(t *testing.T) {
	current := map[string]int{}
	for _, file := range wave4EmojiFiles(t) {
		if _, err := os.Stat(file); err != nil {
			continue
		}
		current[file] = wave4CountEmoji(string(mustReadUIFile(t, file)))
	}
	wave4Ratchet(t, "testdata/wave4_emoji.txt",
		"# <file> <pictograph runes>: wave 4 emoji ratchet over operational page JS, operational HTML and the page lang files (16 locales). Only ever goes down.\n"+
			"# Replace emoji with pwIcon() / .pw-badge[data-status]. Refresh after fixing with AURAGO_UPDATE_WAVE4_BASELINES=1 go test ./ui/ -run TestUIWave4.\n",
		current)
}

func TestUIWave4TailwindCompatHasNoRootPalette(t *testing.T) {
	t.Parallel()
	css := wave4StripComments(wave4ReadText(t, "css/tailwind-compat.css"))
	if regexp.MustCompile(`(?m)^\s*:root\b`).MatchString(css) {
		t.Error("tailwind-compat.css must not define a :root palette; shared-variables.css and tokens.css own the primitives")
	}
}

func TestUIWave4EveryPwColourTokenHasLightOverride(t *testing.T) {
	t.Parallel()
	css := wave4ReadText(t, "css/precision-workspace.css") + "\n" + wave4ReadText(t, "css/precision-pages.css")
	lightBlocks := regexp.MustCompile(`(?s)\[data-theme="light"\] \.pw-page,\s*\.pw-page\[data-theme="light"\] \{(.*?)\}`).FindAllStringSubmatch(css, -1)
	if len(lightBlocks) == 0 {
		t.Fatal("light theme blocks for .pw-page not found")
	}
	var light strings.Builder
	for _, block := range lightBlocks {
		light.WriteString(block[1])
	}
	for _, token := range []string{
		"canvas", "surface", "surface-elevated", "surface-soft", "text", "muted", "subtle", "accent", "accent-strong",
		"line", "line-strong", "danger", "warning", "success", "on-accent", "scrim", "terminal-bg", "terminal-text",
		"status-ok", "status-warn", "status-error", "status-idle", "status-running",
	} {
		if !strings.Contains(css, "--pw-"+token+":") {
			t.Errorf("--pw-%s is not defined", token)
			continue
		}
		if !strings.Contains(light.String(), "--pw-"+token+":") {
			t.Errorf("--pw-%s has no light-theme override", token)
		}
	}
}

func TestUIWave4LegacyBridgeAndScaleExist(t *testing.T) {
	t.Parallel()
	css := wave4ReadText(t, "css/precision-workspace.css")
	start := strings.Index(css, ".pw-page.pw-operational-page,\n.pw-page.pw-entry-page {")
	if start < 0 {
		t.Fatal("legacy bridge block for operational and entry pages missing")
	}
	end := strings.Index(css[start:], "}")
	bridge := css[start : start+end]
	for _, want := range []string{
		"--border: var(--pw-line)", "--border-focus: var(--pw-accent)", "--bg-elevated: var(--pw-surface-elevated)",
		"--text-muted: var(--pw-subtle)", "--text-inverse: var(--pw-on-accent)", "--info:", "--accent-glow:",
		"--accent-hover:", "--accent-active:", "--text-xs: var(--pw-font-xs)", "--text-sm: var(--pw-font-md)",
		"--text-base: var(--pw-font-base)", "--text-lg: var(--pw-font-xl)", "--text-xl: var(--pw-font-2xl)",
		"--text-2xl:", "--transition-fast:", "--transition-base:", "--transition-slow:",
	} {
		if !strings.Contains(bridge, want) {
			t.Errorf("legacy bridge missing %q", want)
		}
	}
	for _, token := range []string{"2xs", "xs", "sm", "md", "base", "lg", "xl", "2xl"} {
		if !strings.Contains(css, "--pw-font-"+token+":") {
			t.Errorf("--pw-font-%s missing", token)
		}
	}
	pages := wave4ReadText(t, "css/precision-pages.css")
	if n := len(regexp.MustCompile(`(?m)^\.pw-page \.modal \{`).FindAllString(pages, -1)); n != 1 {
		t.Errorf("precision-pages.css must define .pw-page .modal exactly once, found %d", n)
	}
	if !strings.Contains(pages, ".pw-page.pw-operational-page .is-hidden {\n    display: none !important;") {
		t.Error("precision-pages.css must carry the single operational .is-hidden rule")
	}
}

func wave4IconNames(t *testing.T) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for _, m := range wave4IconEntryRe.FindAllStringSubmatch(wave4ReadText(t, "js/precision/icons.js"), -1) {
		names[m[1]] = true
	}
	return names
}

func TestUIWave4IconSetIsCompleteAndSelfContained(t *testing.T) {
	t.Parallel()
	names := wave4IconNames(t)
	for _, want := range []string{
		"play", "stop", "edit", "trash", "check", "x", "search", "lock", "egg", "link", "bot",
		"pause", "unlock", "download", "upload", "copy", "undo", "plus", "minus", "chevron-left", "chevron-right",
		"chevrons-left", "chevrons-right", "layout-grid", "list", "external", "paperclip", "bell", "users", "pointer",
		"info", "alert", "check-circle", "x-circle", "cpu", "nest", "image", "music", "phone", "camera", "radio",
		"check-square", "file-code", "table",
		"activity", "folder-open", "file", "network", "mail", "mic", "terminal", "save", "zap", "flask", "bulb", "moon", "sun",
		"gamepad", "arrow-left", "arrow-right", "arrow-up", "loader", "circle", "sparkles", "building", "tag", "heart", "chevron-down",
	} {
		if !names[want] {
			t.Errorf("icons.js is missing icon %q", want)
		}
	}
	js := wave4ReadText(t, "js/precision/icons.js")
	for _, want := range []string{"window.AuraIcons", "window.pwIcon", "function injectSprite", "pw-ic pw-ic-", "aria-hidden", "role=\"img\" aria-label="} {
		if !strings.Contains(js, want) {
			t.Errorf("icons.js missing %q", want)
		}
	}
	// The SVG namespace is an identifier, not a request.
	if regexp.MustCompile(`https?://|@import|url\(`).MatchString(strings.ReplaceAll(js, "http://www.w3.org/2000/svg", "")) {
		t.Error("icons.js must not reference external URLs")
	}

	shim := wave4ReadText(t, "js/dashboard/dash-icons.js")
	for _, want := range []string{"window.dashIcon = dashIcon", "window.dashIconEl = dashIconEl", "window.DASH_ICONS", "dash-ic dash-ic-", `href="#ic-`, "injectSprite"} {
		if !strings.Contains(shim, want) {
			t.Errorf("dash-icons.js shim missing %q", want)
		}
	}
	if strings.Contains(shim, "ICON_PATHS = {") {
		t.Error("dash-icons.js must not keep its own path table")
	}
}

func TestUIWave4PwIconUsageReferencesExistingIcons(t *testing.T) {
	t.Parallel()
	names := wave4IconNames(t)
	for _, file := range wave4EmojiFiles(t) {
		if strings.HasPrefix(file, "lang/") {
			continue
		}
		if _, err := os.Stat(file); err != nil {
			continue
		}
		for _, m := range wave4PwIconRe.FindAllStringSubmatch(string(mustReadUIFile(t, file)), -1) {
			if !names[m[1]] {
				t.Errorf("%s uses unknown icon %q", file, m[1])
			}
		}
	}
}

func TestUIWave4OperationalPagesLoadIconsBeforePageScripts(t *testing.T) {
	t.Parallel()
	const icons = `/js/precision/icons.js?v={{.BuildVersion}}`
	for page, main := range wave4OperationalPages {
		html := wave4ReadText(t, page)
		at := strings.Index(html, icons)
		if at < 0 {
			t.Errorf("%s does not load icons.js", page)
			continue
		}
		if strings.Count(html, icons) != 1 {
			t.Errorf("%s loads icons.js more than once", page)
		}
		if !strings.Contains(html[:strings.Index(html, "</head>")], icons) {
			t.Errorf("%s must load icons.js in the head", page)
		}
		if main != "" {
			if mainAt := strings.Index(html, "/"+main); mainAt >= 0 && mainAt < at {
				t.Errorf("%s loads %s before icons.js", page, main)
			}
		}
		if page == "dashboard.html" {
			dashAt := strings.Index(html, "/js/dashboard/dash-icons.js")
			if dashAt < at {
				t.Error("dashboard.html must load icons.js before dash-icons.js")
			}
			if !regexp.MustCompile(`icons\.js\?v=\{\{\.BuildVersion\}\}" defer>`).MatchString(html) {
				t.Error("dashboard.html must defer icons.js like dash-icons.js")
			}
		}
	}
	for _, entry := range []string{"login.html", "setup.html", "404.html"} {
		if !strings.Contains(wave4ReadText(t, entry), "pw-page pw-entry-page") {
			t.Errorf("%s body must carry pw-page pw-entry-page", entry)
		}
	}
}

func TestUIWave4WorkspaceDelegatesFocusTrapToSharedController(t *testing.T) {
	t.Parallel()
	ws := wave4ReadText(t, "js/precision/workspace.js")
	for _, bad := range []string{"'Tab'", "\"Tab\"", "keydown", "focusModal", "modalFocusableElements", "handleModalKeydown"} {
		if strings.Contains(ws, bad) {
			t.Errorf("workspace.js must not carry its own focus trap code (%s); use window.AuraModalController", bad)
		}
	}
	for _, want := range []string{"window.AuraModalController", "background: 'keep'", "aria: false"} {
		if !strings.Contains(ws, want) {
			t.Errorf("workspace.js missing %q", want)
		}
	}
	core := wave4ReadText(t, "js/shared/shared-core.js")
	for _, want := range []string{"window.AuraModalController = _modalCtl;", "options.background === 'keep' ? [] : setBackgroundInert(modal)", "options.aria !== false"} {
		if !strings.Contains(core, want) {
			t.Errorf("shared-core.js missing additive controller hook %q", want)
		}
	}
}

// The shared `.pw-page.pw-operational-page .is-hidden` rule is !important, so an
// element that ships with is-hidden can only be revealed by removing the class;
// an inline style.display = 'block' silently does nothing.
//
// Best effort, static: it finds ids of elements with is-hidden in an operational
// page's HTML and flags page JS that assigns a value other than 'none' to
// getElementById('<id>').style.display or to a variable declared as
// `const/let/var x = document.getElementById('<id>')`. It does not follow
// querySelector, helper functions that receive the element, or re-bound
// variables.
func TestUIWave4HiddenElementsAreNotRevealedThroughInlineDisplay(t *testing.T) {
	t.Parallel()
	tagRe := regexp.MustCompile(`<[a-zA-Z][^>]*>`)
	idRe := regexp.MustCompile(`\bid="([^"]+)"`)
	classRe := regexp.MustCompile(`\bclass="([^"]*)"`)
	for page, script := range wave4OperationalPages {
		var sources []string
		switch {
		case script == "":
			sources = wave4DashboardJS(t)
		case strings.HasSuffix(script, "/main.js") && page == "knowledge.html":
			sources = wave4JSInDir(t, "js/knowledge")
		default:
			sources = []string{script}
		}
		var js strings.Builder
		for _, src := range sources {
			js.WriteString(wave4ReadText(t, src))
			js.WriteString("\n")
		}
		code := js.String()
		for _, tag := range tagRe.FindAllString(wave4ReadText(t, page), -1) {
			idm, clm := idRe.FindStringSubmatch(tag), classRe.FindStringSubmatch(tag)
			if idm == nil || clm == nil || !regexp.MustCompile(`(^|\s)is-hidden(\s|$)`).MatchString(clm[1]) {
				continue
			}
			id := regexp.QuoteMeta(idm[1])
			direct := regexp.MustCompile(`getElementById\(\s*['"]` + id + `['"]\s*\)\??\.style\.display\s*=\s*([^;]+);`)
			for _, m := range direct.FindAllStringSubmatch(code, -1) {
				if v := strings.TrimSpace(m[1]); v != "'none'" && v != `"none"` {
					t.Errorf("%s: #%s carries is-hidden but %s sets style.display = %s; toggle the class instead", page, idm[1], script, v)
				}
			}
			bound := regexp.MustCompile(`(?:const|let|var)\s+(\w+)\s*=\s*document\.getElementById\(\s*['"]` + id + `['"]\s*\)`)
			for _, b := range bound.FindAllStringSubmatch(code, -1) {
				assign := regexp.MustCompile(`\b` + regexp.QuoteMeta(b[1]) + `\??\.style\.display\s*=\s*([^;]+);`)
				for _, m := range assign.FindAllStringSubmatch(code, -1) {
					if v := strings.TrimSpace(m[1]); v != "'none'" && v != `"none"` {
						t.Errorf("%s: #%s carries is-hidden but %s sets %s.style.display = %s; toggle the class instead", page, idm[1], script, b[1], v)
					}
				}
			}
		}
	}
}

func wave4JSInDir(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".js") {
			out = append(out, filepath.ToSlash(path))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func wave4DashboardJS(t *testing.T) []string { return wave4JSInDir(t, "js/dashboard") }
