package ui

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func verifyWriterShell(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	page.MustEval(`async()=>{await fixtureOpen('writer');window.writerWindow=[...aurora.state.windows.values()].at(-1);aurora.toggleMaximizeWindow(writerWindow.id);}`)
	page.Timeout(60 * time.Second).MustWait(`()=>WriterApp.instances.get(writerWindow.id)?.editor && document.querySelector('[data-loading]')?.hidden`)
	page.MustEval(`async()=>{
        const app=WriterApp.instances.get(writerWindow.id),editor=app.editor;window.actualWriter=app;
        editor.exec({type:'paste',text:'',html:'<h1>Raum für gute Gedanken.</h1><p>Ein Dokument ist mehr als eine Sammlung von Wörtern. Es ist der Ort, an dem aus einer ersten Idee etwas Greifbares wird.</p><h2>Ein klarer Anfang</h2><p>Autor verbindet konzentriertes Schreiben mit den Werkzeugen, die du im richtigen Moment brauchst. Echte Seiten geben deinen Gedanken Struktur.</p><h2>Gemeinsam weiterdenken</h2><p>Kommentare halten Rückfragen direkt am Text. Vorschläge bleiben nachvollziehbar, bis du sie bewusst übernimmst.</p>'});
        const first=editor.findMatches('ersten Idee')[0];editor.selectMatch(first);const comment=editor.addComment('Hier können wir den Einstieg noch konkreter machen.','Alex');if(!comment.ok)throw Error(JSON.stringify(comment));
        const last=editor.surface.session.paragraphIds().at(-1),offset=editor.query({type:'paragraphs'}).at(-1).text.length;
        editor.exec({type:'setSelection',range:{anchor:{paragraphId:last,offset},head:{paragraphId:last,offset}}});
        editor.exec({type:'insertBreak',kind:'page'});editor.exec({type:'paste',text:'Die nächste Seite. Neue Perspektiven brauchen Raum.'});
        const heading=editor.surface.session.paragraphIds()[0];editor.exec({type:'setSelection',range:{anchor:{paragraphId:heading,offset:0},head:{paragraphId:heading,offset:0}}});
        editor.scrollToPage(1);await app.act('format');await app.session.save();
    }`)
	page.MustEval(`()=>{const win=writerWindow;aurora.toggleMaximizeWindow(win.id);Object.assign(win.element.style,{top:'100px',left:'100px',width:'1000px',height:'600px'});actualWriter.editor.setZoom(.8);}`)
	verifyWriterPointerFocus(t, page, page.MustEval(`()=>writerWindow.id`).Str(), "Ein Dokument")
	page.MustEval(`()=>aurora.toggleMaximizeWindow(writerWindow.id)`)
	for _, size := range [][2]int{{1920, 1080}, {1366, 768}, {390, 844}} {
		page.MustSetViewport(size[0], size[1], 1, size[0] < 821)
		if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: size[0] < 821}).Call(page); err != nil {
			t.Fatal(err)
		}
		for _, theme := range []string{"standard", "fruity-dark", "fruity-light"} {
			for _, density := range []string{"comfortable", "compact"} {
				page.MustEval(`async([theme,density])=>{
                    aurora.state.bootstrap.settings['appearance.density']=density;fixtureTheme(theme);
                    actualWriter.editor.setZoomMode({type:'fit',fit:'pageWidth',minZoom:.25,maxZoom:1});
                    actualWriter.editor.scrollToPage(1);
                    await new Promise(r=>setTimeout(r,150));
                    const menuBar=[...document.querySelectorAll('.vd-window-menubar')].find(x=>x.dataset.ownerWindow===writerWindow.id);
                    const menuIcons=[...menuBar.querySelectorAll('.vd-window-menu-icon:not(.empty)')];
                    if(!menuIcons.length || menuIcons.some(x=>x.querySelector('.vd-symbol-fallback') || !x.querySelector('.vd-mini-icon,.vd-mini-symbol,.vd-theme-icon')))throw Error('Writer menu contains missing icons or text placeholders: '+JSON.stringify(menuIcons.filter(x=>x.querySelector('.vd-symbol-fallback') || !x.querySelector('.vd-mini-icon,.vd-mini-symbol,.vd-theme-icon')).map(x=>x.outerHTML)));
                    if(document.querySelector('.writer-app').scrollWidth>document.querySelector('.writer-app').clientWidth+2)throw Error('Writer chrome overflows');
                    if(document.querySelector('[data-notice]')?.dataset.error==='true' && !document.querySelector('[data-notice]').hidden)throw Error(document.querySelector('[data-notice-text]').textContent);
                }`, []string{theme, density})
				name := fmt.Sprintf("writer-%s-%s-%dx%d", theme, density, size[0], size[1])
				page.MustScreenshot(filepath.Join(dir, name+".png"))
			}
		}
	}
	page.MustSetViewport(1366, 768, 1, false)
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"standard", "fruity-dark", "fruity-light"} {
		page.MustEval(`async theme=>{fixtureTheme(theme);if(!document.querySelector('[data-review]'))await actualWriter.act('review');actualWriter.editor.scrollToPage(1);await new Promise(r=>setTimeout(r,100));}`, theme)
		page.MustScreenshot(filepath.Join(dir, "writer-"+theme+"-review.png"))
		page.MustEval(`()=>document.querySelector('.vd-window-menubar[data-owner-window="'+writerWindow.id+'"] [data-window-menu="file"]').click()`)
		page.MustScreenshot(filepath.Join(dir, "writer-"+theme+"-file-menu.png"))
		page.MustEval(`()=>document.querySelector('.vd-window-menubar[data-owner-window="'+writerWindow.id+'"] [data-window-menu="file"]').click()`)
	}
	page.MustEval(`async()=>{
        const win=writerWindow,id=win.id;
        let release;win.beforeClose=()=>new Promise(r=>release=r);
        const close=aurora.closeWindow(id);
        if(!aurora.state.windows.has(id) || win.closing)throw Error('Window closed before async guard');
        release(false);await close;
        if(!aurora.state.windows.has(id) || win.closing)throw Error('Cancelled close discarded window');
        win.beforeClose=async()=>true;await aurora.closeWindow(id);
        if(!win.closing && aurora.state.windows.has(id))throw Error('Approved close did not proceed');
    }`)
	page.Timeout(10 * time.Second).MustWait(`()=>!WriterApp.instances.has(writerWindow.id)`)
	if failures := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); failures != "[]" {
		t.Fatal(failures)
	}
}
