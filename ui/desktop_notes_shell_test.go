package ui

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"path/filepath"
	"testing"
)

func verifyNotesShell(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	page.MustEval(`async()=>{await fixtureOpen('notes');window.notesWindow=[...aurora.state.windows.values()].at(-1);aurora.toggleMaximizeWindow(notesWindow.id);window.actualNotes=NotesApp.instances.get(notesWindow.id);await actualNotes.ready;}`)
	if !page.MustEval(`()=>!!actualNotes.editor`).Bool() {
		t.Fatal(page.MustEval(`()=>({notice:document.querySelector("[data-notice-text]")?.textContent,errors:fixtureErrors,requests:fixtureRequests.filter(p=>p.includes("notes"))})`).JSON("", ""))
	}
	page.MustEval(`async()=>{await actualNotes.act('info');document.querySelector('[data-scroll]').scrollTop=0;}`)
	point := page.MustEval(`()=>{const b=actualNotes.editor.view.dom.querySelector('p').getBoundingClientRect();return [b.x+25,b.y+8];}`)
	page.Mouse.MustMoveTo(point.Arr()[0].Num(), point.Arr()[1].Num()).MustClick(proto.InputMouseButtonLeft)
	page.MustEval(`async()=>{await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));if(document.querySelector('[data-scroll]').scrollTop!==0)throw Error('Initial editor focus jumped');await actualNotes.act('closePanel');}`)
	for _, size := range [][2]int{{1920, 1080}, {1366, 768}, {430, 932}} {
		page.MustSetViewport(size[0], size[1], 1, size[0] < 821)
		if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: size[0] < 821}).Call(page); err != nil {
			t.Fatal(err)
		}
		for _, theme := range []string{"standard", "fruity-dark", "fruity-light"} {
			for _, density := range []string{"comfortable", "compact"} {
				page.MustEval(`async([theme,density])=>{
    aurora.state.bootstrap.settings['appearance.density']=density;fixtureTheme(theme);
    const app=document.querySelector('.vd-notes-app');app.classList.toggle('notes-hide-library',innerWidth<900);
    await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
    if(app.scrollWidth>app.clientWidth+2||app.getBoundingClientRect().right>innerWidth+2)throw Error('Notes chrome overflow '+JSON.stringify({theme,density,app:app.getBoundingClientRect().toJSON(),window:notesWindow.element.getBoundingClientRect().toJSON(),innerWidth,scroll:app.scrollWidth,client:app.clientWidth}));
    const notice=document.querySelector('[data-notice]');if(!notice.hidden&&notice.dataset.error==='true')throw Error(notice.textContent);
    const names=[...app.querySelectorAll('button[title]')].filter(button=>button.title.includes('desktop.'));if(names.length)throw Error('Untranslated button labels');
   }`, []string{theme, density})
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("notes-%s-%s-%dx%d.png", theme, density, size[0], size[1])))
			}
		}
	}
	page.MustSetViewport(1366, 768, 1, false)
	(proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page)
	for _, theme := range []string{"standard", "fruity-dark", "fruity-light"} {
		page.MustEval(`async theme=>{fixtureTheme(theme);document.querySelector('.vd-notes-app').classList.remove('notes-hide-library');await actualNotes.act('info');}`, theme)
		page.MustScreenshot(filepath.Join(dir, "notes-"+theme+"-info.png"))
		page.MustEval(`()=>actualNotes.act('closePanel')`)
	}
	page.MustEval(`async()=>{await aurora.closeWindow(notesWindow.id);}`)
	page.MustWait(`()=>!NotesApp.instances.has(notesWindow.id)`)
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
