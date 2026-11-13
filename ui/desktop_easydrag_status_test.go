package ui

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// W1a A (spec §3.5): a status is drawn only through ED.status. Outside easydrag-status.js and its CSS block (.ed-status-disc,
// .ed-status-label, .ed-status-word, [data-status=]) no EasyDrag file may carry a status class or a status word key.
// The CSS block is bracketed by marker comments; the exemption depends on the position of a line, not on its content.
const (
	easyDragStatusBlockStart = "/* status map (W1a A"
	easyDragStatusBlockEnd   = "/* end status map */"
)

var (
	easyDragStatusClass = regexp.MustCompile(`(?:^|[^-])ed-status|ed-run-dot|\bstatus-(?:success|error|running|queued|skipped|waiting|pending|cancelled|ok|stopped|failed|uncertain|blocked|paused|simulated|not_run)\b|\bstatus-['"` + "`" + `]\s*\+|\bstatus-\$\{`)
	easyDragStatusKey   = regexp.MustCompile(`easydrag\.ui\.status_|easydrag\.status\.`)
	easyDragStatusOwn   = regexp.MustCompile(`ed-status-(?:disc|label|word)\b|\[data-status=`)
	easyDragStatusOld   = regexp.MustCompile(`ed-status--|ed-status-pill|ed-run-dot`)
)

// easyDragStatusAllowed lists what may stay, as "file|needle" with the reason. A needle that matches nothing fails the test,
// so the list cannot go stale.
var easyDragStatusAllowed = map[string]string{
	"js/desktop/apps/easydrag-canvas.js|'status-' +":            "the card ring: the card's state border keyed to the step record, not a rendered status (browser tests select .ed-node.status-*)",
	"css/desktop-app-easydrag.css|.ed-node.status-":             "the card ring rules",
	"css/desktop-app-easydrag.css|.ed-node.is-selected.status-": "the selected card ring",
	"css/desktop-app-easydrag.css|rect[data-status=":            "the minimap tint of a node: a few pixels of fill keyed to the raw step status, with no disc or word of its own",
}

// easyDragStatusViolation reports whether a line draws a status outside ED.status. inBlock is true for lines between the CSS
// block markers. used collects the allow-list entries that took effect.
func easyDragStatusViolation(file, line string, inBlock bool, used map[string]bool) bool {
	isCSS := strings.HasSuffix(file, ".css")
	hit := easyDragStatusClass.MatchString(line) || easyDragStatusOwn.MatchString(line) && isCSS || (!isCSS && easyDragStatusKey.MatchString(line))
	if !hit {
		return false
	}
	if isCSS && inBlock && !easyDragStatusOld.MatchString(line) {
		return false
	}
	for needle := range easyDragStatusAllowed {
		parts := strings.SplitN(needle, "|", 2)
		if parts[0] == file && strings.Contains(line, parts[1]) {
			used[needle] = true
			return false
		}
	}
	return true
}

func TestEasyDragStatusesGoThroughTheStatusMap(t *testing.T) {
	used := map[string]bool{}
	files := []string{"css/desktop-app-easydrag.css"}
	for _, name := range easyDragModules {
		if name != "easydrag-status.js" {
			files = append(files, "js/desktop/apps/"+name)
		}
	}
	for _, file := range files {
		inBlock := false
		for i, line := range strings.Split(readDesktopAssetText(t, file), "\n") {
			if strings.HasSuffix(file, ".css") {
				if strings.Contains(line, easyDragStatusBlockStart) {
					inBlock = true
				}
				if strings.Contains(line, easyDragStatusBlockEnd) {
					inBlock = false
					continue
				}
			}
			if easyDragStatusViolation(file, line, inBlock, used) {
				t.Errorf("%s:%d draws a status outside ED.status: %s", file, i+1, strings.TrimSpace(line))
			}
		}
	}
	var stale []string
	for needle := range easyDragStatusAllowed {
		if !used[needle] {
			stale = append(stale, needle)
		}
	}
	sort.Strings(stale)
	for _, needle := range stale {
		t.Errorf("allow-list entry %q matches nothing; remove it", needle)
	}
}

// The lint must catch what it claims to: probe lines that draw a status are flagged, the allowed card ring is not.
func TestEasyDragStatusLintCatchesProbes(t *testing.T) {
	const js, css = "js/desktop/apps/easydrag-detail.js", "css/desktop-app-easydrag.css"
	flagged := []struct {
		file, line string
		inBlock    bool
	}{
		{js, `'<span class="x status-' + s`, false},
		{js, `'<span class="x status-" + s`, false},
		{js, "`x status-${s}`", false},
		{js, `el.className = 'status-ok'`, false},
		{js, `t('easydrag.ui.status_failed')`, false},
		{css, `.ed-run-row.status-ok { color: green; }`, false},
		{css, `.ed-callout .ed-status-disc { width: 9px; }`, false},
		{css, `.ed-callout [data-status="ok"] { color: green; }`, false},
		{css, `.ed-status-word { color: red; }`, false},
		{css, `.ed-run-dot { background: red; }`, true},
	}
	for _, probe := range flagged {
		if !easyDragStatusViolation(probe.file, probe.line, probe.inBlock, map[string]bool{}) {
			t.Errorf("lint lets %q through", probe.line)
		}
	}
	allowed := []struct{ file, line string }{
		{css, `.ed-node.status-success { border-color: red; }`},
		{css, `.ed-node.is-selected.status-error { border-color: red; }`},
		{"js/desktop/apps/easydrag-canvas.js", `cls.push('status-' + status)`},
	}
	for _, probe := range allowed {
		if easyDragStatusViolation(probe.file, probe.line, false, map[string]bool{}) {
			t.Errorf("lint flags the allowed card ring %q", probe.line)
		}
	}
	if easyDragStatusViolation(css, `.ed-status-disc[data-status="ok"] { background: green; }`, true, map[string]bool{}) {
		t.Error("lint flags the disc family inside the status block")
	}
}

// Every locale has the twelve status words and aria sentences and uses one word for "unclear": the port label and the status
// word are the same text. The old status keys are gone.
func TestEasyDragLocalesUseOneStatusWordForUncertain(t *testing.T) {
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		words := readEasyDragLang(t, lang)
		for _, value := range []string{"ok", "stopped", "failed", "running", "waiting", "uncertain", "blocked", "skipped", "paused", "simulated", "not_run", "cancelled"} {
			for _, key := range []string{"easydrag.status." + value, "easydrag.status." + value + "_aria"} {
				if strings.TrimSpace(words[key]) == "" {
					t.Errorf("%s.json misses %s", lang, key)
				}
			}
		}
		if words["easydrag.ui.port_uncertain"] != words["easydrag.status.uncertain"] {
			t.Errorf("%s.json: port_uncertain %q and status.uncertain %q must be the same word", lang, words["easydrag.ui.port_uncertain"], words["easydrag.status.uncertain"])
		}
		for key := range words {
			if strings.HasPrefix(key, "easydrag.ui.status_") {
				t.Errorf("%s.json still has %s; statuses use easydrag.status.<value>", lang, key)
			}
		}
	}
}
