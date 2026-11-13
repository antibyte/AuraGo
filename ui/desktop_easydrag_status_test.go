package ui

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// W1a A (spec §3.5): a status is drawn only through ED.status. Outside easydrag-status.js and its CSS block (.ed-status-disc,
// .ed-status-label, .ed-status-word) no EasyDrag file may carry a status class or a status word key.
var (
	easyDragStatusClass = regexp.MustCompile(`(?:^|[^-])ed-status|ed-run-dot|\bstatus-(?:success|error|running|queued|skipped|waiting|pending|cancelled)\b|'status-'\s*\+`)
	easyDragStatusKey   = regexp.MustCompile(`easydrag\.ui\.status_|easydrag\.status\.`)
	easyDragStatusOwn   = regexp.MustCompile(`ed-status-(?:disc|label|word)\b`)
	easyDragStatusOld   = regexp.MustCompile(`ed-status--|ed-status-pill|ed-run-dot`)
)

// easyDragStatusAllowed lists what may stay, as "file|needle" with the reason. A needle that matches nothing fails the test,
// so the list cannot go stale.
var easyDragStatusAllowed = map[string]string{
	"js/desktop/apps/easydrag-canvas.js|'status-' +":            "the card ring: the card's state border keyed to the step record, not a rendered status (browser tests select .ed-node.status-*)",
	"css/desktop-app-easydrag.css|.ed-node.status-":             "the card ring rules",
	"css/desktop-app-easydrag.css|.ed-node.is-selected.status-": "the selected card ring",
}

func TestEasyDragStatusesGoThroughTheStatusMap(t *testing.T) {
	used := map[string]bool{}
	allowed := func(file, line string) bool {
		for needle := range easyDragStatusAllowed {
			parts := strings.SplitN(needle, "|", 2)
			if parts[0] == file && strings.Contains(line, parts[1]) {
				used[needle] = true
				return true
			}
		}
		return false
	}
	files := []string{"css/desktop-app-easydrag.css"}
	for _, name := range easyDragModules {
		if name != "easydrag-status.js" {
			files = append(files, "js/desktop/apps/"+name)
		}
	}
	for _, file := range files {
		isCSS := strings.HasSuffix(file, ".css")
		for i, line := range strings.Split(readDesktopAssetText(t, file), "\n") {
			hit := easyDragStatusClass.MatchString(line) || (!isCSS && easyDragStatusKey.MatchString(line))
			// the disc family is the CSS block of the status module
			if !hit || (isCSS && easyDragStatusOwn.MatchString(line) && !easyDragStatusOld.MatchString(line)) || allowed(file, line) {
				continue
			}
			t.Errorf("%s:%d draws a status outside ED.status: %s", file, i+1, strings.TrimSpace(line))
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
