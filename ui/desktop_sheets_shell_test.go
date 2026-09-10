package ui

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"path/filepath"
	"testing"
	"time"
)

func verifySheetsShell(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	page.MustEval(`async()=>{await fixtureOpen('sheets');window.sheetsWindow=[...aurora.state.windows.values()].at(-1);aurora.toggleMaximizeWindow(sheetsWindow.id);}`)
	page.Timeout(60 * time.Second).MustWait(`()=>SheetsApp.instances.get(sheetsWindow.id)?.book&&document.querySelector('[data-loading]')?.hidden`)
	page.MustEval(`async()=>{window.actualSheets=SheetsApp.instances.get(sheetsWindow.id);await actualSheets.state.load('Documents/Monatsbudget.xlsx','budget');actualSheets.session.suspend();await actualSheets.act('format');}`)
	for _, size := range [][2]int{{1920, 1080}, {1366, 768}, {390, 844}} {
		page.MustSetViewport(size[0], size[1], 1, size[0] < 821)
		if e := (proto.EmulationSetTouchEmulationEnabled{Enabled: size[0] < 821}).Call(page); e != nil {
			t.Fatal(e)
		}
		for _, theme := range []string{"standard", "fruity-dark", "fruity-light"} {
			for _, density := range []string{"comfortable", "compact"} {
				page.MustEval(`async([theme,density])=>{
    aurora.state.bootstrap.settings['appearance.density']=density;fixtureTheme(theme);await new Promise(r=>setTimeout(r,180));
    const app=document.querySelector('.sheets-app');if(app.scrollWidth>app.clientWidth+2||app.getBoundingClientRect().right>innerWidth+2)throw Error('Sheets chrome overflows: '+JSON.stringify({app:app.getBoundingClientRect().toJSON(),win:sheetsWindow.element.getBoundingClientRect().toJSON(),viewport:innerWidth,layer:document.querySelector('.vd-window-layer').getBoundingClientRect().toJSON()}));
    const menuBar=[...document.querySelectorAll('.vd-window-menubar')].find(x=>x.dataset.ownerWindow===sheetsWindow.id);
    const bad=[...menuBar.querySelectorAll('.vd-window-menu-icon:not(.empty)')].filter(x=>x.querySelector('.vd-symbol-fallback')||!x.querySelector('.vd-mini-icon,.vd-mini-symbol,.vd-theme-icon,[style*="--vd-sprite-x"]'));
    if(bad.length)throw Error('Menu icon fallback: '+JSON.stringify(bad.map(x=>x.outerHTML)));
    const canvas=document.querySelector('.sheets-canvas');if(canvas.clientWidth<200||canvas.clientHeight<180)throw Error('Spreadsheet has insufficient space');
    const notice=document.querySelector('[data-notice]');if(!notice.hidden&&notice.dataset.error==='true')throw Error(notice.textContent);
   }`, []string{theme, density})
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("sheets-%s-%s-%dx%d.png", theme, density, size[0], size[1])))
			}
		}
	}
	page.MustSetViewport(1366, 768, 1, false)
	(proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page)
	for _, theme := range []string{"standard", "fruity-dark", "fruity-light"} {
		page.MustEval(`async theme=>{fixtureTheme(theme);await actualSheets.act('closeRight');await actualSheets.act('chart');const input=document.querySelector('[data-formula]');actualSheets.book.getActiveSheet().getRange('B11').activate();input.focus({preventScroll:true});input.value='=SUM(B5:B9)';input.dispatchEvent(new Event('input',{bubbles:true}));await new Promise(r=>setTimeout(r,120));}`, theme)
		page.MustScreenshot(filepath.Join(dir, "sheets-"+theme+"-formula-chart.png"))
		page.MustEval(`()=>actualSheets.act('cancelFormula')`)
		page.MustEval(`()=>document.querySelector('.vd-window-menubar[data-owner-window="'+sheetsWindow.id+'"] [data-window-menu="file"]').click()`)
		page.MustScreenshot(filepath.Join(dir, "sheets-"+theme+"-file-menu.png"))
		page.MustEval(`()=>document.querySelector('.vd-window-menubar[data-owner-window="'+sheetsWindow.id+'"] [data-window-menu="file"]').click()`)
	}
	page.MustEval(`async()=>{actualSheets.session.dispose();sheetsWindow.beforeClose=async()=>true;await aurora.closeWindow(sheetsWindow.id);}`)
	page.Timeout(10 * time.Second).MustWait(`()=>!SheetsApp.instances.has(sheetsWindow.id)`)
	if failures := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); failures != "[]" {
		t.Fatal(failures)
	}
}
