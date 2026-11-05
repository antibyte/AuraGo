package ui

import (
	"fmt"
	"testing"
	"time"
)

// A window maximized on a wide desktop must follow later workspace changes
// without overwriting the geometry used when the user restores it.
func TestDesktopMaximizedWindowFollowsViewportResizeBrowser(t *testing.T) {
	page := desktopAuditBrowserWithAPI(t, nil).CancelTimeout().Timeout(90 * time.Second)
	for _, theme := range []string{"standard", "fruity"} {
		t.Run(theme, func(t *testing.T) {
			setDesktopMobileViewport(t, page, 1440, 900, false)
			page.MustEval(`theme=>{auditTest.state.bootstrap.settings['appearance.theme']=theme;auditTest.applyDesktopSettings();}`, theme)
			openDesktopMobileApp(t, page, "settings")
			id := page.MustEval(`()=>auditTest.state.activeWindowId`).Str()
			defer page.MustEval(`async id=>{await auditTest.closeWindow(id)}`, id)
			page.MustEval(`id=>{
				const a=auditTest,w=a.state.windows.get(id),e=w.element;
				if(w.maximized)e.querySelector('[data-action="maximize"]').click();
				a.applyResize(e,'se',{left:70,top:60,width:800,height:600},0,0);
				a.applyWindowSnap(e,'maximize');
				window.expectedMaximizedRestore=JSON.stringify(w.restoreBounds);
				window.expectedMaximizedMin=JSON.stringify([e.style.minWidth,e.style.minHeight]);
			}`, id)
			for _, viewport := range []struct {
				width, height int
				mobile        bool
			}{{320, 844, false}, {320, 844, true}, {640, 180, false}, {1440, 900, false}} {
				setDesktopMobileViewport(t, page, viewport.width, viewport.height, viewport.mobile)
				result := page.MustEval(`id=>{
					const a=auditTest,w=a.state.windows.get(id),e=w.element,r=e.getBoundingClientRect(),b=a.workspaceBoundsForWindow();
					return {maximized:w.maximized,restore:JSON.stringify(w.restoreBounds),width:r.width,height:r.height,
						minWidth:parseFloat(e.style.minWidth),minHeight:parseFloat(e.style.minHeight),availableWidth:b.width,availableHeight:b.height};
				}`, id)
				label := fmt.Sprintf("%s %dx%d mobile=%t", theme, viewport.width, viewport.height, viewport.mobile)
				if !result.Get("maximized").Bool() || result.Get("restore").Str() != page.MustEval(`()=>window.expectedMaximizedRestore`).Str() {
					t.Fatalf("%s: lost maximized state or restore bounds: %s", label, result.JSON("", ""))
				}
				if result.Get("width").Num() > result.Get("availableWidth").Num()+1 || result.Get("minWidth").Num() > result.Get("availableWidth").Num()+1 || result.Get("minHeight").Num() > result.Get("availableHeight").Num()+1 {
					t.Fatalf("%s: maximized window exceeds workspace limits: %s", label, result.JSON("", ""))
				}
				if !viewport.mobile && result.Get("height").Num() > result.Get("availableHeight").Num()+1 {
					t.Fatalf("%s: maximized window exceeds workspace height: %s", label, result.JSON("", ""))
				}
			}
			if !page.MustEval(`id=>{
				const e=auditTest.state.windows.get(id).element;
				return JSON.stringify([e.style.minWidth,e.style.minHeight])===window.expectedMaximizedMin;
			}`, id).Bool() {
				t.Fatal("returning to the wide desktop must restore the application's logical minimum")
			}
			page.MustEval(`async id=>{
				const e=auditTest.state.windows.get(id).element;
				e.querySelector('[data-action="maximize"]').click();
				await new Promise(requestAnimationFrame);await new Promise(requestAnimationFrame);
			}`, id)
			if !page.MustEval(`id=>{
				const w=auditTest.state.windows.get(id),e=w.element,b=JSON.parse(window.expectedMaximizedRestore);
				return !w.maximized && parseInt(e.style.left,10)===b.left && parseInt(e.style.top,10)===b.top &&
					parseInt(e.style.width,10)===b.width && parseInt(e.style.height,10)===b.height;
			}`, id).Bool() {
				t.Fatal("restoring on the wide desktop must recover the original usable geometry")
			}
		})
	}
}
