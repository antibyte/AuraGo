package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/office"
)

// Exercise the real lazy apps and the bundled text editor with a missing
// remembered file. The shared Code Studio fixture also supplies its backend.
func TestDesktopFileLoadRecoveryBrowser(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	doc, err := office.EncodeDOCX(office.Document{Text: "Existing document remains intact."})
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("testdata/desktop-file-load-recovery.js")
	if err != nil {
		t.Fatal(err)
	}
	page.MustEval(string(script), base64.StdEncoding.EncodeToString(doc))
	for _, app := range []string{"writer", "editor", "notes", "codeStudio", "pixel", "zipper", "viewer", "viewer3d"} {
		t.Run(app, func(t *testing.T) {
			result, err := page.Eval(`async app => { await recoveryCases[app](); return fixtureErrors; }`, app)
			if err != nil {
				t.Fatal(err)
			}
			if errors := result.Value.Arr(); len(errors) != 0 {
				t.Fatal(errors)
			}
		})
	}
	if t.Failed() {
		return
	}
	out := filepath.Join("..", "reports", "file-load-recovery")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`async()=>await showRecoveryScreenshot()`)
	for _, theme := range []string{"standard", "fruity"} {
		for _, width := range []int{1366, 390} {
			page.MustSetViewport(width, 768, 1, false)
			page.MustEval(`theme=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode='light';}`, theme)
			page.MustEval(`()=>{for(const button of document.querySelectorAll('#recovery-screenshot [data-load-actions] button')){const r=button.getBoundingClientRect();if(r.height<44||r.left<0||r.right>innerWidth)throw Error('Recovery button is clipped');}}`)
			page.MustScreenshot(filepath.Join(out, fmt.Sprintf("writer-%s-%d.png", theme, width)))
		}
	}
}
