package ui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopMobileLayoutBrowser(t *testing.T) {
	shellCSS := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	codeStudioMux := http.NewServeMux()
	newCodeStudioFixtureBackend().register(codeStudioMux)
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/css/desktop-shell.bundle.css": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			_, _ = w.Write([]byte(shellCSS))
		}),
		"/api/desktop/integrations/contacts": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if q := strings.ToLower(r.URL.Query().Get("q")); q != "" && !strings.Contains("ada lovelace", q) {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":"fixture-person-1","name":"Ada Lovelace","relationship":"friend","email":"ada@example.test","phone":"+49123456789"}]`))
		}),
		"/api/desktop/integrations/people/": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(r.URL.Path, "/lookup") {
				_, _ = w.Write([]byte(`{"nodes":[]}`))
				return
			}
			_, _ = w.Write([]byte(`[]`))
		}),
		"/api/desktop/viewer/content": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"content":"# Mobile viewer\n\nA short fixture document."}`))
		}),
		"/api/code-studio/": codeStudioMux,
	})
	page = page.CancelTimeout().Timeout(5 * time.Minute)
	responsiveApps := []string{"todo", "gallery", "calendar", "quick-connect", "settings", "people", "findit", "viewer", "openscad"}
	wideApps := []string{"code-studio"}
	viewports := []struct {
		name          string
		width, height int
		mobile        bool
	}{
		{name: "phone-360", width: 360, height: 800, mobile: true},
		{name: "phone-390", width: 390, height: 844, mobile: true},
		{name: "tablet-768", width: 768, height: 1024, mobile: true},
		{name: "tablet-1024", width: 1024, height: 768, mobile: true},
		{name: "desktop", width: 1440, height: 900},
		{name: "rotated-phone", width: 844, height: 390, mobile: true},
	}

	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`async theme => {
			const a = auditTest, settings = a.state.bootstrap.settings;
			settings['appearance.theme'] = theme;
			settings['appearance.fruity_mode'] = 'light';
			a.applyDesktopSettings();
			// Opening and closing a lightweight app exercises the same taskbar render path
			// used after a live theme change, so the Fruity dock is present for shell checks.
			a.openApp('calculator');
			const id = a.state.activeWindowId;
			if (id) await a.closeWindow(id);
		}`, theme)
		page.Timeout(10 * time.Second).MustWait(`() => ![...auditTest.state.windows.values()].some(win => win.appId === 'calculator')`)
		for _, viewport := range viewports {
			viewport := viewport
			t.Run(fmt.Sprintf("%s/%s/shell", theme, viewport.name), func(t *testing.T) {
				setDesktopMobileViewport(t, page, viewport.width, viewport.height, viewport.mobile)
				assertDesktopMobileShell(t, page, viewport.width, viewport.height)
			})
			appIDs := responsiveApps
			if viewport.name == "rotated-phone" {
				appIDs = []string{"people", "viewer", "code-studio", "openscad"}
			}
			for _, appID := range appIDs {
				appID := appID
				t.Run(fmt.Sprintf("%s/%s/%s", theme, viewport.name, appID), func(t *testing.T) {
					setDesktopMobileViewport(t, page, viewport.width, viewport.height, viewport.mobile)
					openDesktopMobileApp(t, page, appID)
					defer page.MustEval(`() => { const id=auditTest.state.activeWindowId; if(id) auditTest.closeWindow(id); }`)
					wide := appID == "code-studio"
					assertDesktopMobileWindow(t, page, appID, viewport.width, viewport.height, wide)
					if appID == "people" && viewport.width <= 390 {
						assertDesktopPeopleMobileInteraction(t, page)
					}
					if appID == "viewer" && viewport.width <= 390 {
						assertDesktopViewerMobileInteraction(t, page)
					}
					if (viewport.width == 390 || viewport.width == 768) && (appID == "people" || appID == "code-studio" || appID == "openscad") {
						captureDesktopMobileScreenshot(t, page, theme, viewport.width, appID)
					}
				})
			}
			if viewport.width != 844 {
				for _, appID := range wideApps {
					appID := appID
					t.Run(fmt.Sprintf("%s/%s/%s", theme, viewport.name, appID), func(t *testing.T) {
						setDesktopMobileViewport(t, page, viewport.width, viewport.height, viewport.mobile)
						openDesktopMobileApp(t, page, appID)
						defer page.MustEval(`() => { const id=auditTest.state.activeWindowId; if(id) auditTest.closeWindow(id); }`)
						assertDesktopMobileWindow(t, page, appID, viewport.width, viewport.height, appID == "code-studio")
						if appID == "code-studio" && viewport.width <= 390 {
							assertDesktopMobileCodeStudioWorkspaceScrolling(t, page)
						}
						if (viewport.width == 390 || viewport.width == 768) && appID == "code-studio" {
							captureDesktopMobileScreenshot(t, page, theme, viewport.width, appID)
						}
					})
				}
			}
		}
		t.Run(theme+"/code-studio/live-resize", func(t *testing.T) {
			setDesktopMobileViewport(t, page, 768, 1024, true)
			openDesktopMobileApp(t, page, "code-studio")
			defer page.MustEval(`() => { const id=auditTest.state.activeWindowId; if(id) auditTest.closeWindow(id); }`)
			for _, viewport := range []struct {
				width, height int
				mobile        bool
			}{{1440, 900, false}, {390, 844, true}} {
				setDesktopMobileViewport(t, page, viewport.width, viewport.height, viewport.mobile)
				assertDesktopMobileWindow(t, page, "code-studio", viewport.width, viewport.height, true)
			}
		})
		t.Run(theme+"/code-studio/persist-resize", func(t *testing.T) {
			setDesktopMobileViewport(t, page, 1440, 900, false)
			openDesktopMobileApp(t, page, "code-studio")
			defer page.MustEval(`() => {
				auditTest.state.bootstrap.settings['windows.restore_session']='false';
				const id=auditTest.state.activeWindowId; if(id) auditTest.closeWindow(id);
			}`)
			page.MustEval(`() => {
				const a=auditTest, w=a.state.windows.get(a.state.activeWindowId), settings=a.state.bootstrap.settings;
				Object.assign(w.element.style,{left:'600px',top:'300px',width:'800px',height:'500px'});
				window.mobileSessionBeforeResize=JSON.stringify(a.captureSessionSnapshot());
				settings['session.windows']=window.mobileSessionBeforeResize;
				settings['windows.restore_session']='true';
			}`)
			setDesktopMobileViewport(t, page, 1024, 768, false)
			page.Timeout(10 * time.Second).MustWait(`() => auditTest.state.bootstrap.settings['session.windows']!==window.mobileSessionBeforeResize`)
			if !page.MustEval(`() => {
				const a=auditTest, current=a.captureSessionSnapshot().windows.find(w=>w.appId==='code-studio');
				const saved=JSON.parse(a.state.bootstrap.settings['session.windows']).windows.find(w=>w.appId==='code-studio');
				return current && saved && ['left','top','width','height'].every(key=>current[key]===saved[key]);
			}`).Bool() {
				t.Error("viewport clamp must persist corrected bounds for the next session restore")
			}
			assertDesktopMobileWindow(t, page, "code-studio", 1024, 768, true)
		})
	}
}

func captureDesktopMobileScreenshot(t *testing.T, page *rod.Page, theme string, width int, appID string) {
	t.Helper()
	dir := filepath.Join("..", "reports", "desktop-mobile-browser")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("create mobile browser report directory: %v", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%d-%s.png", theme, width, appID))
	if err := os.WriteFile(path, page.MustScreenshot(), 0644); err != nil {
		t.Fatalf("save mobile browser screenshot: %v", err)
	}
}

func setDesktopMobileViewport(t *testing.T, page *rod.Page, width, height int, mobile bool) {
	t.Helper()
	page.MustSetViewport(width, height, 1, mobile)
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: mobile}).Call(page); err != nil {
		t.Fatalf("set touch emulation at %dx%d: %v", width, height, err)
	}
	page.MustEval(`async () => { await new Promise(requestAnimationFrame); await new Promise(requestAnimationFrame); }`)
}

func openDesktopMobileApp(t *testing.T, page *rod.Page, appID string) {
	t.Helper()
	page = page.Timeout(10 * time.Second)
	if appID == "viewer" {
		page.MustEval(`() => auditTest.openApp('viewer', {path:'Documents/mobile-fixture.md'})`)
	} else {
		page.MustEval(`id => auditTest.openApp(id)`, appID)
	}
	page.MustWait(fmt.Sprintf(`() => {
		const a=auditTest, w=a.state.windows.get(a.state.activeWindowId), c=w?.element.querySelector('[data-window-content]');
		return w?.appId === %q && !!c && !!window.AuraDesktopModules?.appAssetsReady?.(%q) &&
			!!c.firstElementChild && !c.firstElementChild.classList.contains('vd-empty');
	}`, appID, appID))
	if appID == "code-studio" {
		page.MustWait(`() => !!document.querySelector('[data-code-studio] .cs-tree-item[data-file-path="/workspace/main.go"]')`)
		page.MustEval(`async () => { await window.CodeStudioApp.openPath('/workspace/main.go', false); }`)
		page.MustWait(`() => !!document.querySelector('[data-code-studio] .cm-editor')`)
	}
}

func assertDesktopMobileShell(t *testing.T, page *rod.Page, width, height int) {
	t.Helper()
	page = page.Timeout(10 * time.Second)
	assertDesktopMobileFixtureStyles(t, page, width, height)
	page.MustWait(`() => {
		const bar=document.querySelector('.vd-taskbar');
		const visible=el=>el && (typeof el.checkVisibility==='function' ? el.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}) : !!el.getClientRects().length && getComputedStyle(el).visibility!=='hidden' && Number(getComputedStyle(el).opacity||1)>.05);
		return visible(document.getElementById('vd-start-button')) || visible(bar?.querySelector('.vd-dock-orb')) || visible(document.querySelector('.vd-global-brand'));
	}`)
	result := page.MustEval(`() => {
		const errors=[]; const bar=document.querySelector('.vd-taskbar');
		const rect=el=>{const r=el.getBoundingClientRect();return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:r.width,height:r.height};};
		const visible=el=>typeof el.checkVisibility==='function' ? el.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}) : (()=>{const s=getComputedStyle(el);return !!el.getClientRects().length && s.visibility!=='hidden' && s.display!=='none' && Number(s.opacity||1)>.05;})();
		const scrollCanReach=(el)=>{for(let p=el.parentElement;p&&p!==bar;p=p.parentElement){const s=getComputedStyle(p),r=rect(p),e=rect(el),localLeft=e.left-r.left+p.scrollLeft,localRight=e.right-r.left+p.scrollLeft,localTop=e.top-r.top+p.scrollTop,localBottom=e.bottom-r.top+p.scrollTop,x=['auto','scroll'].includes(s.overflowX)&&p.scrollWidth>p.clientWidth+1&&localLeft>=-1&&localRight<=p.scrollWidth+1,y=['auto','scroll'].includes(s.overflowY)&&p.scrollHeight>p.clientHeight+1&&localTop>=-1&&localBottom<=p.scrollHeight+1;if(x||y)return true;}return false;};
		if(document.documentElement.scrollWidth>innerWidth+1 || document.body.scrollWidth>innerWidth+1) errors.push('page has horizontal overflow');
		if(!bar || !bar.getClientRects().length) errors.push('taskbar is not visible');
		if(bar){const r=rect(bar);if(r.left< -1 || r.right>innerWidth+1 || r.top< -1 || r.bottom>innerHeight+1) errors.push('taskbar is outside the viewport: '+JSON.stringify(r));
			const launchers=[document.getElementById('vd-start-button'),bar.querySelector('.vd-dock-orb'),document.querySelector('.vd-global-brand')].filter(el=>el&&visible(el));
			if(!launchers.length)errors.push('no visible taskbar launcher');
			for(const el of bar.querySelectorAll('button')){if(!visible(el))continue;const b=rect(el),outside=b.left< -1 || b.right>innerWidth+1 || b.top< -1 || b.bottom>innerHeight+1;if(outside&&!scrollCanReach(el))errors.push('taskbar control outside viewport without a scrollable ancestor: '+JSON.stringify({id:el.id,class:el.className,aria:el.getAttribute('aria-label'),rect:b}));}}
		return errors.join('; ') || 'ok';
	}`).Str()
	if result != "ok" {
		t.Errorf("shell controls clipped at %dx%d: %s", width, height, result)
	}
	launcher := page.MustEval(`() => ['#vd-start-button','.vd-dock-orb','.vd-global-brand'].find(selector=>document.querySelector(selector)?.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}))`).Str()
	page.MustElement(launcher).MustClick()
	page.MustWait(`() => !document.getElementById('vd-start-menu').hidden`)
	if !page.MustEval(`() => { const r=document.getElementById('vd-start-menu').getBoundingClientRect(); return r.width>0 && r.left>=-1 && r.right<=innerWidth+1 && r.top>=-1 && r.bottom<=innerHeight+1; }`).Bool() {
		t.Errorf("app menu exceeds viewport at %dx%d", width, height)
	}
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`() => document.getElementById('vd-start-menu').hidden`)
}

func assertDesktopMobileWindow(t *testing.T, page *rod.Page, appID string, width, height int, wide bool) {
	t.Helper()
	page = page.Timeout(10 * time.Second)
	assertDesktopMobileFixtureStyles(t, page, width, height)
	result := page.MustEval(`(appID,wide) => {
		const a=auditTest,w=a.state.windows.get(a.state.activeWindowId), errors=[];
		const rect=el=>{const r=el.getBoundingClientRect();return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:r.width,height:r.height};};
		if(!w || w.appId!==appID) return 'app window did not open';
		const wr=rect(w.element), title=w.element.querySelector('.vd-window-titlebar'), content=w.element.querySelector('[data-window-content]');
		if(wr.width<1 || wr.height<1 || wr.left< -1 || wr.right>innerWidth+1 || wr.top< -1 || wr.bottom>innerHeight+1) errors.push('window outside viewport: '+JSON.stringify(wr));
		if(!title || !content) return errors.concat('window title or content missing').join('; ');
		const tr=rect(title); if(tr.left< -1 || tr.right>innerWidth+1 || tr.top< -1 || tr.bottom>innerHeight+1) errors.push('titlebar outside viewport: '+JSON.stringify(tr));
		const visible=el=>typeof el.checkVisibility==='function' ? el.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}) : (()=>{const s=getComputedStyle(el);return !!el.getClientRects().length && s.visibility!=='hidden' && s.display!=='none' && Number(s.opacity||1)>.05;})();
		const close=title.querySelector('[data-action="close"]');
		if(!close || !visible(close)) errors.push('window close control is not visible');
		for(const button of title.querySelectorAll('.vd-window-button')) { if(!visible(button))continue; const r=rect(button); if(r.left<tr.left-1 || r.right>tr.right+1 || r.top<tr.top-1 || r.bottom>tr.bottom+1) errors.push('title control clipped: '+JSON.stringify({id:button.id,class:button.className,action:button.dataset.action,aria:button.getAttribute('aria-label'),rect:r})); }
		if(document.documentElement.scrollWidth>innerWidth+1 || document.body.scrollWidth>innerWidth+1) errors.push('page has horizontal overflow');
		if(appID==='openscad' && innerWidth<=768){
			const app=content.matches('.openscad-app') ? content : content.querySelector('.openscad-app'), contentRect=content.getBoundingClientRect(), appRect=app?.getBoundingClientRect();
			if(!app || appRect.width>content.clientWidth+2 || appRect.left<contentRect.left-1 || appRect.right>contentRect.right+1) errors.push('OpenSCAD app root exceeds its window content width: '+JSON.stringify({app:appRect&&{left:appRect.left,right:appRect.right,width:appRect.width},content:{left:contentRect.left,right:contentRect.right,width:content.clientWidth}}));
		}
		if(wide && innerWidth<=768){
			if(content.scrollWidth<=content.clientWidth+2 || !['auto','scroll'].includes(getComputedStyle(content).overflowX)) errors.push('wide workspace does not scroll inside its window content');
		}
		if(window.useMobileDesktopMode?.() && document.body.classList.contains('fruity-dock-collapsed')) errors.push('mobile dock hides app switching controls');
		return errors.join('; ') || 'ok';
	}`, appID, wide).Str()
	if result != "ok" {
		t.Errorf("%s at %dx%d: %s", appID, width, height, result)
	}
}

func assertDesktopMobileFixtureStyles(t *testing.T, page *rod.Page, width, height int) {
	t.Helper()
	page = page.Timeout(10 * time.Second)
	page.MustWait(`() => { const link=[...document.querySelectorAll('link[rel="stylesheet"]')].find(el=>new URL(el.href).pathname==='/css/desktop-shell.bundle.css'); return !!link?.sheet && (link.sheet.cssRules?.length || 0)>100; }`)
	result := page.MustEval(`() => {
		const link=[...document.querySelectorAll('link[rel="stylesheet"]')].find(el=>new URL(el.href).pathname==='/css/desktop-shell.bundle.css');
		let rules=0, cssError='';
		try { rules=link?.sheet?.cssRules?.length || 0; } catch(error) { cssError=String(error); }
		const shell=document.querySelector('.vd-shell'), taskbar=document.querySelector('.vd-taskbar');
		const styles={shell:shell&&getComputedStyle(shell).display,taskbar:taskbar&&getComputedStyle(taskbar).display,position:taskbar&&getComputedStyle(taskbar).position};
		const assets=performance.getEntriesByType('resource').filter(entry=>entry.name.includes('desktop-shell.bundle.css')).map(entry=>({name:entry.name,status:entry.responseStatus,duration:Math.round(entry.duration)}));
		if(!link || !link.sheet || rules<100 || styles.shell!=='grid' || styles.taskbar!=='flex' || !['fixed','absolute'].includes(styles.position)) return JSON.stringify({error:'desktop shell CSS was not applied',href:link?.href||null,rules,cssError,styles,assets});
		return 'ok';
	}`).Str()
	if result != "ok" {
		t.Fatalf("desktop mobile browser fixture styles unavailable at %dx%d: %s", width, height, result)
	}
}

func assertDesktopPeopleMobileInteraction(t *testing.T, page *rod.Page) {
	t.Helper()
	page = page.Timeout(10 * time.Second)
	page.MustWait(`() => !!document.querySelector('.vd-people-card[data-contact-id="fixture-person-1"], .vd-people-list-row[data-contact-id="fixture-person-1"]')`)
	visible := page.MustEval(`() => {
		const host=document.querySelector('.vd-people');
		const search=host?.querySelector('.vd-people-search'), filter=host?.querySelector('.vd-people-sidebar-item[data-filter="friend"]'), list=host?.querySelector('.vd-people-view-btn[data-view="list"]');
		return !!host && [search,filter,list].every(el=>el && el.getClientRects().length && el.getBoundingClientRect().width>0);
	}`).Bool()
	if !visible {
		t.Error("People search, relationship filter, and view controls must remain reachable on a phone")
		return
	}
	page.MustElement(`.vd-people-sidebar-item[data-filter="friend"]`).MustClick()
	if !page.MustEval(`() => document.querySelector('.vd-people-sidebar-item[data-filter="friend"]')?.classList.contains('active') && !!document.querySelector('.vd-people-card[data-contact-id="fixture-person-1"], .vd-people-list-row[data-contact-id="fixture-person-1"]')`).Bool() {
		t.Error("People compact relationship navigation did not filter the contacts")
		return
	}
	page.MustElement(`.vd-people-view-btn[data-view="grid"]`).MustClick()
	page.MustWait(`() => document.querySelector('.vd-people-view-btn[data-view="grid"]')?.classList.contains('active')`)
	search := page.MustElement(`.vd-people-search`)
	search.MustInput("__missing_contact__")
	page.MustWait(`() => !!document.querySelector('.vd-people-content .vd-people-empty')`)
	search.MustSelectAllText().MustInput("Ada")
	page.MustWait(`() => !!document.querySelector('.vd-people-card[data-contact-id="fixture-person-1"]')`)
	page.MustElement(`.vd-people-card[data-contact-id="fixture-person-1"]`).MustClick()
	page.MustWait(`() => document.querySelector('.vd-people-detail')?.classList.contains('open') && document.querySelector('.vd-people-detail-name')?.textContent==='Ada Lovelace'`)
	page.MustElement(`.vd-people-detail-close`).MustClick()
	page.MustWait(`() => !document.querySelector('.vd-people-detail')?.classList.contains('open')`)
	page.MustElement(`.vd-people-view-btn[data-view="list"]`).MustClick()
	page.MustWait(`() => !!document.querySelector('.vd-people-list-row[data-contact-id="fixture-person-1"]')`)
	if !page.MustEval(`() => document.querySelector('.vd-people-view-btn[data-view="list"]')?.classList.contains('active')`).Bool() {
		t.Error("People list-view action did not respond to a mobile click")
	}
	result := page.MustEval(`() => {
		const content=document.querySelector('.vd-people-content'), row=content?.querySelector('.vd-people-list-row');
		if(!content || !row)return 'mobile list row did not render';
		const bounds=content.getBoundingClientRect(), columns=[...row.querySelectorAll('.vd-people-list-col')].filter(el=>el.getClientRects().length);
		const clipped=columns.filter(el=>{const r=el.getBoundingClientRect();return r.width<1 || r.left<bounds.left-1 || r.right>bounds.right+1;});
		return clipped.length || row.scrollWidth>row.clientWidth+1 ? JSON.stringify({columns:columns.map(el=>({text:el.textContent,left:el.getBoundingClientRect().left,right:el.getBoundingClientRect().right})),rowWidth:row.clientWidth,rowScrollWidth:row.scrollWidth,contentWidth:content.clientWidth}) : 'ok';
	}`).Str()
	if result != "ok" {
		t.Errorf("People mobile list fields are clipped: %s", result)
	}
}

func assertDesktopMobileCodeStudioWorkspaceScrolling(t *testing.T, page *rod.Page) {
	t.Helper()
	page = page.Timeout(10 * time.Second)
	result := page.MustEval(`() => {
		const w=auditTest.state.windows.get(auditTest.state.activeWindowId), title=w?.element.querySelector('.vd-window-titlebar'), scrollport=w?.element.querySelector('.vd-window-content');
		if(!w || !title || !scrollport)return JSON.stringify({error:'Code Studio window content scrollport or titlebar missing'});
		const beforeRect=title.getBoundingClientRect(), windowRect=w.element.getBoundingClientRect(), metrics={clientWidth:scrollport.clientWidth,scrollWidth:scrollport.scrollWidth,before:scrollport.scrollLeft,after:0,titleLeftBefore:beforeRect.left,titleTopBefore:beforeRect.top,titleLeftAfter:0,titleTopAfter:0,windowLeftBefore:windowRect.left,windowLeftAfter:0};
		scrollport.scrollLeft=0;metrics.before=scrollport.scrollLeft;scrollport.scrollLeft=scrollport.scrollWidth;metrics.after=scrollport.scrollLeft;
		const titleAfter=title.getBoundingClientRect(), windowAfter=w.element.getBoundingClientRect();metrics.titleLeftAfter=titleAfter.left;metrics.titleTopAfter=titleAfter.top;metrics.windowLeftAfter=windowAfter.left;
		const fixed=Math.abs(metrics.titleLeftAfter-metrics.titleLeftBefore)<1 && Math.abs(metrics.titleTopAfter-metrics.titleTopBefore)<1 && Math.abs(metrics.windowLeftAfter-metrics.windowLeftBefore)<1;
		scrollport.scrollLeft=0;
		return metrics.scrollWidth>metrics.clientWidth+2 && metrics.after>metrics.before+2 && fixed ? 'ok' : JSON.stringify(metrics);
	}`).Str()
	if result != "ok" {
		t.Errorf("Code Studio window content must overflow and scroll while the titlebar stays fixed: %s", result)
	}
}

func assertDesktopViewerMobileInteraction(t *testing.T, page *rod.Page) {
	t.Helper()
	page = page.Timeout(10 * time.Second)
	toolbar := page.MustEval(`() => {
		const host=document.querySelector('.vd-viewer'), bar=host?.querySelector('.vd-viewer-toolbar');
		if(!bar || !bar.getClientRects().length)return false;
		const r=bar.getBoundingClientRect();
		return r.left>=-1 && r.right<=innerWidth+1 && [...bar.querySelectorAll('button')].filter(el=>el.getClientRects().length).every(el=>{const b=el.getBoundingClientRect();return b.left>=r.left-1 && b.right<=r.right+1;});
	}`).Bool()
	if !toolbar {
		t.Error("Viewer toolbar actions must stay visible inside the phone window")
		return
	}
	page.MustElement(`.vd-viewer-toolbar [data-action="download"]`).MustClick()
	if !page.MustEval(`() => !!auditTest.state.windows.get(auditTest.state.activeWindowId)?.element.querySelector('.vd-viewer')`).Bool() {
		t.Error("Viewer download action unexpectedly closed the viewer")
	}
}
