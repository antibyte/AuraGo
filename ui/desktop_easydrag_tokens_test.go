package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	easyDragTokenRead = regexp.MustCompile(`var\(\s*(--ed-[a-z0-9_-]+)\s*([,)])`)
	easyDragTokenDecl = regexp.MustCompile(`(--ed-[a-z0-9_-]+)\s*:`)
	easyDragTokenSet  = regexp.MustCompile(`setProperty\(\s*['"](--ed-[a-z0-9_-]+)['"]`)
)

// easyDragUndeclaredTokens returns the --ed-* tokens a stylesheet reads without a fallback and nobody declares: not in
// the css (a "--ed-x:" declaration) and not in the scripts (setProperty('--ed-x' or "--ed-x:" in a style string).
func easyDragUndeclaredTokens(css string, scripts []string) []string {
	declared := map[string]bool{}
	for _, m := range easyDragTokenDecl.FindAllStringSubmatch(css, -1) {
		declared[m[1]] = true
	}
	for _, js := range scripts {
		for _, m := range easyDragTokenDecl.FindAllStringSubmatch(js, -1) {
			declared[m[1]] = true
		}
		for _, m := range easyDragTokenSet.FindAllStringSubmatch(js, -1) {
			declared[m[1]] = true
		}
	}
	missing := map[string]bool{}
	for _, m := range easyDragTokenRead.FindAllStringSubmatch(css, -1) {
		if m[2] == ")" && !declared[m[1]] {
			missing[m[1]] = true
		}
	}
	var out []string
	for name := range missing {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// TestEasyDragCSSTokensAreDeclared: a var(--ed-x) without a fallback that nothing declares is an invalid declaration at
// computed-value time (a radius or a colour silently drops). Every such read needs a declaration in the css or in an
// EasyDrag script.
func TestEasyDragCSSTokensAreDeclared(t *testing.T) {
	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-easydrag.css"), "\r\n", "\n")
	files, err := filepath.Glob(filepath.Join("js", "desktop", "apps", "easydrag-*.js"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no EasyDrag scripts found (%v)", err)
	}
	var scripts []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		scripts = append(scripts, string(b))
	}
	if got := easyDragUndeclaredTokens(css, scripts); len(got) > 0 {
		t.Errorf("tokens read without a fallback and declared nowhere: %s", strings.Join(got, ", "))
	}
	// the scan must see the real stylesheet
	if len(easyDragTokenRead.FindAllString(css, -1)) < 100 {
		t.Fatal("the token scan found almost no var(--ed-*) read: the pattern is broken")
	}
	// samples: a read without fallback and no declaration is found; a fallback, a css declaration and a script one are not
	got := easyDragUndeclaredTokens(`.a { x: var(--ed-gone); y: var(--ed-fb, 1px); z: var(--ed-css); } :root { --ed-css: 1px; }`, []string{`el.style.setProperty('--ed-js', '1')`, `'--ed-str:1px'`})
	if len(got) != 1 || got[0] != "--ed-gone" {
		t.Errorf("sample scan gave %v, want only --ed-gone", got)
	}
}
