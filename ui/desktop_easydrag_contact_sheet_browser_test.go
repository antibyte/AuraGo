package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// easyDragContactRunDoneJS is a finished test run without the backend (every step succeeded, every wire delivered).
const easyDragContactRunDoneJS = `()=>{const ed=edFixture.editor(),counts=[7,42,1234];
	const steps=new Map(ed.model.doc.nodes.map(n=>[n.id,{node_id:n.id,status:'success',duration_ms:120,item_count:3}]));
	const edges=new Map(ed.model.doc.edges.map((e,i)=>[e.id,{state:'delivered',count:counts[i%3]}]));
	ed.run={id:'run_contact',status:'success',mode:'test',steps,changes:[...steps.keys()],edges,record:null,error:''};
	ed.bus.emit('run',ed.run);}`

// TestDesktopEasyDragContactSheetBrowser captures the S0 colour review for the owner: home, editor in the run-done state
// and palette for Standard dark, Fruity light and Fruity dark, and the editor in every other theme context. It runs only
// on request and only into an explicit folder outside the repository:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_EASYDRAG_CONTACT_SHEET=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0-contact/after go test ./ui/ -run 'TestDesktopEasyDragContactSheetBrowser$' -count=1 -timeout 30m
func TestDesktopEasyDragContactSheetBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	if os.Getenv("AURAGO_EASYDRAG_CONTACT_SHEET") != "1" {
		t.Skip("set AURAGO_EASYDRAG_CONTACT_SHEET=1 to capture the EasyDrag contact sheet")
	}
	if os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR") == "" {
		t.Fatal("set AURAGO_BROWSER_ARTIFACT_DIR to a scratch folder outside the repository")
	}
	s := startEasyDragSmoke(t)
	setReducedMotion(t, s.page, "reduce")
	page := s.page
	owner := []string{"standard", "fruity-light", "fruity-dark"}
	for _, name := range owner {
		page.MustEval(`theme=>fixtureTheme(theme)`, name)
		s.wait(`()=>!!document.querySelector('.ed-home') && document.querySelectorAll('.ed-flow-card').length>=1`)
		s.clearToasts()
		s.shot(name + "-home")
	}
	page.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, page.MustEval(`()=>edFixture.seededID`).Str())
	s.wait(`()=>!!edFixture.editor() && document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	page.MustEval(easyDragContactRunDoneJS)
	s.wait(`()=>document.querySelectorAll('.ed-edge.is-delivered').length===3 && document.querySelectorAll('.ed-pill').length===3`)
	s.fit()
	s.clearToasts()
	for _, name := range owner {
		page.MustEval(`theme=>fixtureTheme(theme)`, name)
		s.shot(name + "-editor-run-done")
		page.MustElement(".ed-palette").MustScreenshot(filepath.Join(s.dir, name+"-palette.png"))
	}
	// Representative survey: the editor in every other registry theme, and Minimal dark.
	for _, theme := range parseDesktopThemeRegistry(t) {
		if theme.ID == "standard" || theme.ID == "fruity" {
			continue
		}
		page.MustEval(`theme=>fixtureTheme(theme)`, theme.ID)
		s.wait(fmt.Sprintf(`()=>document.querySelector('.desktop-body').dataset.theme===%q`, theme.ID))
		s.shot("survey-" + theme.ID + "-editor")
	}
	page.MustEval(`()=>{Object.assign(aurora.state.bootstrap.settings,{'appearance.theme':'minimal','appearance.minimal_mode':'dark'});aurora.applyDesktopSettings();}`)
	s.wait(`()=>document.querySelector('.desktop-body').dataset.scheme==='dark'`)
	s.shot("survey-minimal-dark-editor")
	writeEasyDragContactIndex(t, s.dir)
}

// writeEasyDragContactIndex writes <parent>/index.html with the before and after shots side by side; the "after" run
// writes it.
func writeEasyDragContactIndex(t *testing.T, dir string) {
	t.Helper()
	if filepath.Base(dir) != "after" {
		return
	}
	shots, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(shots)
	var b strings.Builder
	b.WriteString("<!doctype html><meta charset=\"utf-8\"><title>EasyDrag S0 contact sheet</title><style>body{font:14px system-ui;background:#222;color:#eee}td{vertical-align:top;padding:6px}img{max-width:720px}</style><table><tr><th>before</th><th>after</th></tr>\n")
	for _, shot := range shots {
		name := filepath.Base(shot)
		fmt.Fprintf(&b, "<tr><td colspan=\"2\">%s</td></tr><tr><td><img src=\"before/%s\"></td><td><img src=\"after/%s\"></td></tr>\n", name, name, name)
	}
	b.WriteString("</table>\n")
	index := filepath.Join(filepath.Dir(dir), "index.html")
	if err := os.WriteFile(index, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", index)
}
