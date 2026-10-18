package ui

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

var (
	dashboardAttrOpen = regexp.MustCompile(`[\w:-]+=(\\?)(["'])`)
	dashboardEscCall  = regexp.MustCompile(`(^|[^\w$.])esc\(`)
)

// escCallsInOpenAttributes returns the offsets of the esc( calls on one source line that sit inside an HTML attribute
// value: the last name= plus quote before the call (or name=\" in a quoted JS string) has no closing quote yet. esc()
// leaves quotes alone, so such a value can end its attribute; attribute values take escAttr().
func escCallsInOpenAttributes(line string) []int {
	var calls []int
	for _, m := range dashboardEscCall.FindAllStringSubmatchIndex(line, -1) {
		at := m[3]
		before := line[:at]
		opens := dashboardAttrOpen.FindAllStringSubmatchIndex(before, -1)
		if len(opens) == 0 {
			continue
		}
		last := opens[len(opens)-1]
		closing := before[last[2]:last[5]]
		if !strings.Contains(before[last[1]:], closing) {
			calls = append(calls, at)
		}
	}
	return calls
}

// TestDashboardAttributeValuesUseEscAttr: no dashboard script puts esc() into an HTML attribute value, and none
// brings back escapeJsString, whose output in an inline handler ended the attribute at a double quote (ids and other
// handler arguments go into data attributes). The detector is proven on samples first. It reads one line at a time,
// so a value assembled in a variable first is not seen; TestDashboardAttributeEscapingBrowser renders the real
// renderers with hostile values.
func TestDashboardAttributeValuesUseEscAttr(t *testing.T) {
	for _, line := range []string{
		"`<span title=\"${esc(x)}\">`",
		"'<span title=\"' + esc(x) + '\">'",
		"\"<span title=\\\"\" + esc(x) + \"\\\">\"",
		"`<b data-id='${esc(x)}'>`",
		"`<b title=\"${escAttr(a)}: ${esc(b)}\">`",
		"`<i class=\"badge ${esc(kind)}\">`",
		"`<a href=\"/x?q=${esc(q)}\">`",
	} {
		if len(escCallsInOpenAttributes(line)) != 1 {
			t.Errorf("detector misses the attribute esc() in %s", line)
		}
	}
	for _, line := range []string{
		"`<b>${esc(x)}</b>`",
		"`<b class=\"a\">${esc(x)}</b>`",
		"`<b title=\"${escAttr(x)}\">${esc(x)}</b>`",
		"`<b data-x=\"${y}\">${esc(x)} ${esc(z)}</b>`",
		"const s = '' + esc(x);",
		"el.title = esc(x);",
		"if (a === 'x') html += esc(b);",
		"const v = escAttr(x) + myesc(y) + obj.esc(z);",
	} {
		if calls := escCallsInOpenAttributes(line); len(calls) != 0 {
			t.Errorf("detector reports a text-position esc() in %s", line)
		}
	}

	var files []string
	if err := fs.WalkDir(Content, "js/dashboard", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".js") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk js/dashboard: %v", err)
	}
	if len(files) < 20 {
		t.Fatalf("found only %d dashboard scripts", len(files))
	}
	for _, file := range files {
		source := normalizeAssetText(mustReadUIFile(t, file))
		for i, line := range strings.Split(source, "\n") {
			if len(escCallsInOpenAttributes(line)) > 0 {
				t.Errorf("%s:%d puts esc() into an attribute value, use escAttr(): %s", file, i+1, strings.TrimSpace(line))
			}
			if strings.Contains(line, "escapeJsString") {
				t.Errorf("%s:%d uses escapeJsString; pass ids to inline handlers through data attributes: %s", file, i+1, strings.TrimSpace(line))
			}
		}
	}
}
