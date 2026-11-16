package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func desktopEverydaySettingsHandler() http.Handler {
	settings := desktop.DesktopSettingDefaults()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			var body struct {
				Key      string            `json:"key"`
				Value    string            `json:"value"`
				Settings map[string]string `json:"settings"`
			}
			if json.NewDecoder(r.Body).Decode(&body) == nil {
				updates := body.Settings
				if updates == nil {
					updates = map[string]string{body.Key: body.Value}
				}
				for key, value := range updates {
					settings[key] = value
				}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "settings": settings})
	})
}

func TestDesktopEverydayWindowLayoutsBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`()=>{
        auditTest.openApp('calculator');window.fixedWindowId=auditTest.state.activeWindowId;
        const fixed=auditTest.state.windows.get(fixedWindowId);
        window.fixedWindowBefore={left:fixed.element.style.left,top:fixed.element.style.top,width:fixed.element.style.width,height:fixed.element.style.height,maxWidth:fixed.element.style.maxWidth,maxHeight:fixed.element.style.maxHeight};
        window.fixedMenuResult=auditTest.openWindowLayoutMenu(fixedWindowId);
        window.fixedSnapResult=auditTest.applyWindowSnap(fixed.element,'left-half');
        auditTest.openApp('files');window.layoutWindowId=auditTest.state.activeWindowId;
        const item=auditTest.state.windows.get(layoutWindowId);
        window.layoutOriginal={left:item.element.style.left,top:item.element.style.top,width:item.element.style.width,height:item.element.style.height};
        const control=item.element.querySelector('[data-action="maximize"]');
        const contextEvent=new MouseEvent('contextmenu',{bubbles:true,cancelable:true,button:2});
        window.layoutContextDispatchResult=control.dispatchEvent(contextEvent);
        window.layoutContextProbe={
            appId:item.appId,windowId:item.id,activeId:auditTest.state.activeWindowId,
            controlConnected:control.isConnected,hasPopup:control.getAttribute('aria-haspopup'),
            defaultPrevented:contextEvent.defaultPrevented,
            menuPresent:!!document.querySelector('#vd-window-layout-menu'),
            activeTag:document.activeElement?.tagName||'',activeIdAttr:document.activeElement?.id||'',
            activeClass:document.activeElement?.className||'',fixtureErrors:window.fixtureErrors.slice()
        };
    }`)
	if !page.MustEval(`()=>{
        const fixed=auditTest.state.windows.get(fixedWindowId),style=fixed.element.style,b=fixedWindowBefore;
        return fixedMenuResult===false&&fixedSnapResult===false&&!fixed.snapped
            &&style.left===b.left&&style.top===b.top&&style.width===b.width&&style.height===b.height
            &&style.maxWidth===b.maxWidth&&style.maxHeight===b.maxHeight&&!fixed.element.querySelector('[data-action="maximize"]');
    }`).Bool() {
		t.Fatal("fixed calculator bounds must not expose a layout menu or accept an impossible snap")
	}
	if !page.MustEval(`()=>!!document.querySelector('#vd-window-layout-menu[role="menu"]')`).Bool() {
		probe := page.MustEval(`()=>JSON.stringify(layoutContextProbe)`).Str()
		t.Fatalf("maximize contextmenu did not open the layout menu; dispatch=%t probe=%s", page.MustEval(`()=>layoutContextDispatchResult`).Bool(), probe)
	}
	// Finishing a background app's lazy render must not take focus from this menu.
	page.MustEval(`async()=>{await AuraDesktopModules.loadAppAssets('calculator');}`)
	if err := page.Timeout(5 * time.Second).Wait(rod.Eval(`()=>!!document.activeElement.closest('#vd-window-layout-menu')`)); err != nil {
		active := page.MustEval(`()=>{
        const element=document.activeElement,win=element?.closest('.vd-window');
        return JSON.stringify({
            tag:element?.tagName||'',id:element?.id||'',className:element?.className||'',
            menuPresent:!!document.querySelector('#vd-window-layout-menu'),
            closestWindow:win?{id:win.id,appId:win.dataset.appId||'',title:win.querySelector('.vd-window-title')?.textContent||'',className:win.className}:null
        });
    }`).Str()
		t.Fatalf("layout menu did not retain keyboard focus within 5s: %v; current focus=%s", err, active)
	}
	if !page.MustEval(`()=>{
        const menu=document.getElementById('vd-window-layout-menu');
        return menu.getAttribute('aria-label')===window.t('desktop.window_layouts')&&menu.querySelectorAll('[role="menuitemradio"]').length>=8;
    }`).Bool() {
		t.Fatal("maximize context menu must expose accessible layout choices")
	}
	page.MustEval(`()=>{
        const button=[...document.querySelectorAll('#vd-window-layout-menu button')].find(node=>node.querySelector('.vd-context-label')?.textContent.trim()===window.t('desktop.snap_left'));
        if(!button||button.disabled)throw new Error('left-half layout should be available');button.click();
    }`)
	page.MustWait(`()=>auditTest.state.windows.get(layoutWindowId)?.snapped==='left-half'`)
	if !page.MustEval(`()=>{
        const item=auditTest.state.windows.get(layoutWindowId),r=item.element.getBoundingClientRect(),b=auditTest.workspaceBoundsForWindow();
        return Math.abs(r.left)<1&&Math.abs(r.width-b.width/2)<2&&Math.abs(r.height-b.height)<2;
    }`).Bool() {
		t.Fatal("left-half snap must match the shared preview rectangle")
	}
	page.MustEval(`()=>{
        const item=auditTest.state.windows.get(layoutWindowId),handle=item.element.querySelector('[data-resize="e"]'),r=handle.getBoundingClientRect();
        window.restoreBoundsBeforeNoopResize={...item.restoreBounds};
        window.noopResizePoint={x:r.left+r.width/2,y:r.top+r.height/2};
    }`)
	point := page.MustEval(`()=>noopResizePoint`)
	page.Mouse.MustMoveTo(point.Get("x").Num(), point.Get("y").Num()).MustDown(proto.InputMouseButtonLeft).MustUp(proto.InputMouseButtonLeft)
	if !page.MustEval(`()=>{
        const item=auditTest.state.windows.get(layoutWindowId),actual=item.restoreBounds,expected=restoreBoundsBeforeNoopResize;
        return item.snapped==='left-half'&&actual&&actual.left===expected.left&&actual.top===expected.top
            &&actual.width===expected.width&&actual.height===expected.height&&!item.minimizing&&item.element.style.display!=='none';
    }`).Bool() {
		t.Fatal("a no-movement resize click must preserve the snapped layout and its restore bounds")
	}
	page.MustEval(`()=>{
        window.resizeArrowEvent=new KeyboardEvent('keydown',{key:'ArrowDown',metaKey:true,bubbles:true,cancelable:true});
        document.dispatchEvent(resizeArrowEvent);
    }`)
	page.MustWait(`()=>auditTest.state.windows.get(layoutWindowId)?.snapped==='bottom-left'`)
	if !page.MustEval(`()=>{
        const item=auditTest.state.windows.get(layoutWindowId),actual=item.restoreBounds,expected=restoreBoundsBeforeNoopResize;
        return resizeArrowEvent.defaultPrevented&&actual&&actual.left===expected.left&&actual.top===expected.top
            &&actual.width===expected.width&&actual.height===expected.height&&!item.minimizing&&item.element.style.display!=='none';
    }`).Bool() {
		t.Fatal("ArrowDown after a no-op resize must snap to the lower quarter and retain the original restore bounds")
	}
	page.MustEval(`()=>auditTest.openWindowLayoutMenu(layoutWindowId)`)
	page.MustWait(`()=>!!document.querySelector('#vd-window-layout-menu')`)
	page.MustEval(`()=>{
        const button=[...document.querySelectorAll('#vd-window-layout-menu button')].find(node=>node.querySelector('.vd-context-label')?.textContent.trim()===window.t('desktop.restore'));
        if(!button)throw new Error('Restore must be visible after snap');button.click();
    }`)
	page.MustWait(`()=>!auditTest.state.windows.get(layoutWindowId)?.snapped`)
	if !page.MustEval(`()=>{
        const s=auditTest.state.windows.get(layoutWindowId).element.style,b=layoutOriginal;
        return s.left===b.left&&s.top===b.top&&s.width===b.width&&s.height===b.height;
    }`).Bool() {
		t.Fatal("Restore after snapping must return to the original bounds")
	}
	page.MustEval(`()=>{
        auditTest.state.windows.get(layoutWindowId).element.style.maxWidth='500px';
        auditTest.openWindowLayoutMenu(layoutWindowId);
    }`)
	page.MustWait(`()=>!!document.querySelector('#vd-window-layout-menu')`)
	if !page.MustEval(`()=>{
        const button=[...document.querySelectorAll('#vd-window-layout-menu button')].find(node=>node.querySelector('.vd-context-label')?.textContent.trim()===window.t('desktop.snap_left'));
        return !!button&&button.disabled&&button.getAttribute('aria-description')===window.t('desktop.snap_layout_too_small');
    }`).Bool() {
		t.Fatal("a half wider than the app maximum must be disabled with an explanation")
	}
	page.MustEval(`()=>{
        document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true}));
        auditTest.state.windows.get(layoutWindowId).element.style.maxWidth='';
    }`)

	page.MustSetViewport(400, 740, 1, false)
	page.MustEval(`()=>auditTest.openWindowLayoutMenu(layoutWindowId)`)
	page.MustWait(`()=>!!document.querySelector('#vd-window-layout-menu')`)
	if !page.MustEval(`()=>{
        const button=[...document.querySelectorAll('#vd-window-layout-menu button')].find(node=>node.querySelector('.vd-context-label')?.textContent.trim()===window.t('desktop.snap_left'));
        return !!button&&button.disabled&&button.getAttribute('aria-description')===window.t('desktop.snap_layout_too_small');
    }`).Bool() {
		t.Fatal("a half too narrow for the app minimum must be disabled with an explanation")
	}
	page.MustEval(`()=>document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true}))`)

	page.MustEval(`async()=>{
        auditTest.openApp('editor');window.dirtyWindowId=auditTest.state.activeWindowId;
        auditTest.state.windows.get(dirtyWindowId).beforeClose=async()=>false;
        auditTest.openApp('calendar');window.laterWindowId=auditTest.state.activeWindowId;
        window.closeBatch=await auditTest.closeDesktopWindowBatch([layoutWindowId,dirtyWindowId,laterWindowId]);
    }`)
	if got := page.MustEval(`()=>JSON.stringify(closeBatch)`).Str(); !strings.Contains(got, `"status":"cancelled"`) || !strings.Contains(got, `"closed":["`+page.MustEval(`()=>layoutWindowId`).Str()+`"]`) {
		t.Fatalf("close batch must stop after the dirty-window guard cancels: %s", got)
	}
	if !page.MustEval(`()=>auditTest.state.windows.has(dirtyWindowId)&&auditTest.state.windows.has(laterWindowId)`).Bool() {
		t.Fatal("cancelled batch must keep the guarded window and later targets open")
	}
}

func TestDesktopEverydayStartSpotlightSettingsFocusAndPagingBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`()=>{
        window.searchRequests=[];window.originalFetch=window.fetch.bind(window);
        window.fetch=(url,options)=>{
            const parsed=new URL(url,location.href);
            if(parsed.pathname!=='/api/desktop/search')return originalFetch(url,options);
            const offset=Number(parsed.searchParams.get('offset')||0),query=parsed.searchParams.get('query');
            searchRequests.push({query,offset,type:parsed.searchParams.get('type')||'',minSize:parsed.searchParams.get('min_size')||''});
            const files=query.includes('needle')
                ? (offset===0?Array.from({length:50},(_,i)=>({name:'needle-'+String(i).padStart(2,'0')+'.png',path:'Pictures/needle-'+String(i).padStart(2,'0')+'.png'})):[{name:'needle-50.png',path:'Pictures/needle-50.png'}])
                : [];
            return Promise.resolve(new Response(JSON.stringify({files,has_more:query.includes('needle')&&offset===0,truncated:false,next_offset:offset+files.length}),{headers:{'Content-Type':'application/json'}}));
        };
        document.getElementById('vd-start-button').click();
        const input=document.getElementById('vd-start-search');input.value='needle';input.dispatchEvent(new Event('input',{bubbles:true}));
    }`)
	page.MustWait(`()=>document.querySelector('#vd-start-apps [data-start-result]')&&searchRequests.some(item=>item.query==='needle'&&item.offset===0)`)
	page.MustWait(`()=>!!document.querySelector('#vd-start-apps [data-search-more]')`)
	page.MustEval(`()=>document.querySelector('#vd-start-apps [data-search-more]').click()`)
	page.MustWait(`()=>searchRequests.some(item=>item.query==='needle'&&item.offset===50)&&document.querySelector('#vd-start-apps').textContent.includes('needle-50.png')`)

	page.MustEval(`()=>{
        const input=document.getElementById('vd-start-search');input.value=window.t('desktop.settings_text_size');input.dispatchEvent(new Event('input',{bubbles:true}));
    }`)
	page.MustWait(`()=>[...document.querySelectorAll('#vd-start-apps [data-start-result]')].some(button=>button.textContent.includes(window.t('desktop.settings_text_size')))`)
	page.MustEval(`()=>{
        const button=[...document.querySelectorAll('#vd-start-apps [data-start-result]')].find(node=>node.textContent.includes(window.t('desktop.settings_text_size')));
        if(!button)throw new Error('Start search settings result missing');button.click();
    }`)
	page.MustWait(`()=>document.querySelector('.vd-window[data-app-id="settings"] [data-setting-key="accessibility.text_size"]')&&document.activeElement.dataset.settingKey==='accessibility.text_size'`)

	page.MustEval(`()=>{
        auditTest.openSpotlight();const input=document.querySelector('.vd-spotlight-input');input.value=window.t('desktop.settings_text_size');input.dispatchEvent(new Event('input',{bubbles:true}));
    }`)
	page.MustWait(`()=>[...document.querySelectorAll('[data-spotlight-index]')].some(button=>button.textContent.includes(window.t('desktop.settings_text_size')))`)
	page.MustEval(`()=>{
        const button=[...document.querySelectorAll('[data-spotlight-index]')].find(node=>node.textContent.includes(window.t('desktop.settings_text_size')));
        if(!button)throw new Error('Spotlight settings result missing');button.click();
    }`)
	page.MustWait(`()=>document.querySelector('.vd-window[data-app-id="settings"] [data-setting-key="accessibility.text_size"]')&&document.activeElement.dataset.settingKey==='accessibility.text_size'`)

	page.MustEval(`()=>{
        auditTest.openSpotlight();const backdrop=document.getElementById('vd-spotlight-backdrop');
        backdrop.querySelector('[data-spotlight-filter-type]').value='image';
        backdrop.querySelector('[data-spotlight-min-size]').value='12';
        const input=backdrop.querySelector('.vd-spotlight-input');input.value='needlepage';input.dispatchEvent(new Event('input',{bubbles:true}));
    }`)
	page.MustWait(`()=>searchRequests.some(item=>item.query==='needlepage'&&item.offset===0)`)
	if !page.MustEval(`()=>{
        const request=searchRequests.find(item=>item.query==='needlepage'&&item.offset===0);
        return request&&request.type==='image'&&request.minSize==='12';
    }`).Bool() {
		t.Fatal("Spotlight must send filters before retrieving the first result page")
	}
	page.MustWait(`()=>!document.querySelector('[data-spotlight-load-more]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-spotlight-load-more]').click()`)
	page.MustWait(`()=>searchRequests.some(item=>item.query==='needlepage'&&item.offset===50)&&document.querySelector('[data-spotlight-results]').textContent.includes('needle-50.png')`)
	if !page.MustEval(`()=>searchRequests.filter(item=>item.query==='needlepage').every(item=>item.type==='image'&&item.minSize==='12')`).Bool() {
		t.Fatal("Spotlight must carry the same filters through later result pages")
	}
	page.MustEval(`()=>{
        auditTest.closeSpotlight();auditTest.openSpotlight();const input=document.querySelector('.vd-spotlight-input');
        input.value='2+2';input.dispatchEvent(new Event('input',{bubbles:true}));
        [...document.querySelectorAll('[data-spotlight-index]')].find(button=>button.textContent.includes('2+2 = 4')).click();
    }`)
	page.MustWait(`()=>document.querySelector('.vd-window[data-app-id="calculator"] [data-expression]')?.textContent==='2+2'&&document.querySelector('.vd-window[data-app-id="calculator"] [data-result]')?.textContent==='4'`)
	page.MustEval(`()=>{
        window.searchCalculatorId=auditTest.state.activeWindowId;
        auditTest.openSpotlight();const input=document.querySelector('.vd-spotlight-input');
        input.value='9*9';input.dispatchEvent(new Event('input',{bubbles:true}));
        [...document.querySelectorAll('[data-spotlight-index]')].find(button=>button.textContent.includes('9*9 = 81')).click();
    }`)
	page.MustWait(`()=>auditTest.state.activeWindowId===searchCalculatorId&&document.querySelector('.vd-window[data-app-id="calculator"] [data-result]')?.textContent==='81'`)
}

func TestDesktopEverydayDefaultAppsFilesChooserBrowser(t *testing.T) {
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/settings": desktopEverydaySettingsHandler()})
	page.MustEval(`async()=>{
        await AuraDesktopModules.loadAppAssets('settings');
        auditTest.openApp('settings',{category:'files'});
    }`)
	page.MustWait(`()=>!!document.querySelector('.vd-window[data-app-id="settings"] [data-default-new]')`)
	page.MustEval(`()=>{
        const input=document.querySelector('.vd-window[data-app-id="settings"] [data-default-new]');input.value='txt';input.dispatchEvent(new Event('input',{bubbles:true}));
    }`)
	page.MustWait(`()=>{
        const select=document.querySelector('.vd-window[data-app-id="settings"] [data-default-choice]');
        return select&&!select.disabled&&[...select.options].some(option=>option.value==='editor');
    }`)
	page.MustEval(`()=>{
        const select=document.querySelector('.vd-window[data-app-id="settings"] [data-default-choice]');select.value='editor';
        document.querySelector('.vd-window[data-app-id="settings"] [data-default-add]').click();
    }`)
	page.MustWait(`()=>document.querySelector('.vd-window[data-app-id="settings"] [data-default-extension="txt"]')?.value==='editor'&&auditTest.defaultAppForExtension('txt')==='editor'`)
	if !page.MustEval(`()=>!!document.querySelector('.vd-window[data-app-id="settings"] [data-default-reset="txt"]')`).Bool() {
		t.Fatal("saved default-app association must offer an individual reset control")
	}
}

func TestDesktopEverydayNotificationsKeepReadStateAndDND(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustWait(`()=>window.AuraDesktopNotificationCenter?.status().ready`)
	page.MustEval(`()=>{
        const center=window.AuraDesktopNotificationCenter;
        window.openedNotification=center.record({appId:'calculator',title:'Open notification',message:'Open a calculator'});
        auditTest.toggleNotificationCenter(document.getElementById('vd-notification-button'));
    }`)
	if !page.MustEval(`()=>{
        const item=window.AuraDesktopNotificationCenter.history().find(entry=>entry.id===openedNotification.id);
        return item&&!item.read;
    }`).Bool() {
		t.Fatal("opening notification history must not mark entries read")
	}
	page.MustEval(`()=>document.querySelector('[data-action="open-notification"][data-notification-id="'+openedNotification.id+'"]').click()`)
	page.MustWait(`()=>window.AuraDesktopNotificationCenter.history().find(entry=>entry.id===openedNotification.id)?.read===true`)

	page.MustEval(`()=>{
        const center=window.AuraDesktopNotificationCenter;
        window.explicitReadNotification=center.record({appId:'desktop',title:'Explicit read'});
        window.dismissedNotification=center.record({appId:'desktop',title:'Dismiss me'});
        auditTest.toggleNotificationCenter(document.getElementById('vd-notification-button'));
    }`)
	page.MustEval(`()=>document.querySelector('[data-action="open-notification"][data-notification-id="'+explicitReadNotification.id+'"]').click()`)
	page.MustWait(`()=>window.AuraDesktopNotificationCenter.history().find(entry=>entry.id===explicitReadNotification.id)?.read===true`)
	page.MustEval(`()=>document.querySelector('[data-action="dismiss-notification"][data-notification-id="'+dismissedNotification.id+'"]').click()`)
	page.MustWait(`()=>!window.AuraDesktopNotificationCenter.history().some(entry=>entry.id===dismissedNotification.id)`)
	page.MustEval(`()=>{
        const checkbox=document.getElementById('vd-notification-dnd');checkbox.checked=true;checkbox.dispatchEvent(new Event('change',{bubbles:true}));
    }`)
	page.MustWait(`()=>window.AuraDesktopNotificationCenter.policy().dnd`)
	if !page.MustEval(`()=>{
        const delivery=window.AuraDesktopNotificationCenter.delivery({appId:'calculator'});
        return !delivery.toast&&!delivery.sound;
    }`).Bool() {
		t.Fatal("Do Not Disturb must suppress toast and sound delivery")
	}
}

func TestDesktopEverydayAccessibilitySettingsSmallViewportBrowser(t *testing.T) {
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/settings": desktopEverydaySettingsHandler()})
	page.MustEval(`async()=>{await AuraDesktopModules.loadAppAssets('settings');auditTest.openApp('settings',{category:'appearance'});}`)
	page.MustWait(`()=>!!document.querySelector('.vd-window[data-app-id="settings"] [data-setting-key="accessibility.text_size"]')`)
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`async theme=>{await auditTest.saveDesktopPreference('appearance.theme',theme);}`, theme)
		page.MustWait(`theme=>document.body.dataset.theme===theme`, theme)
		for _, viewport := range []struct{ width, height int }{{390, 700}, {640, 780}} {
			page.MustSetViewport(viewport.width, viewport.height, 1, false)
			page.MustEval(`()=>{
                const size=document.querySelector('.vd-window[data-app-id="settings"] [data-setting-key="accessibility.text_size"]');
                size.value='150';size.dispatchEvent(new Event('change',{bubbles:true}));
            }`)
			page.MustWait(`()=>document.body.dataset.textSize==='150'`)
			page.MustEval(`()=>{
                const contrast=document.querySelector('.vd-window[data-app-id="settings"] [data-setting-key="accessibility.high_contrast"]');
                if(!contrast.checked){contrast.checked=true;contrast.dispatchEvent(new Event('change',{bubbles:true}));}
            }`)
			page.MustWait(`()=>document.body.dataset.highContrast==='true'`)
			if !page.MustEval(`()=>{
                const win=document.querySelector('.vd-window[data-app-id="settings"]'),select=win?.querySelector('[data-setting-key="accessibility.text_size"]'),toggle=win?.querySelector('[data-setting-key="accessibility.high_contrast"]');
                const target=toggle?.closest('.vd-switch')?.querySelector('span');
                if(!win||!select||!toggle||!target||getComputedStyle(win.querySelector('.vd-settings-sidebar')).visibility!=='hidden')return false;
                const wr=win.getBoundingClientRect(),sr=select.getBoundingClientRect(),tr=target.getBoundingClientRect();
                return wr.left>=0&&wr.right<=innerWidth+1&&wr.top>=0&&wr.bottom<=innerHeight+1
                    &&sr.width>80&&sr.height>24&&tr.width>=40&&tr.height>=24;
            }`).Bool() {
				t.Fatalf("text size/high contrast controls must stay reachable at 150%% in %s on a %dpx viewport", theme, viewport.width)
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				name := filepath.Join(dir, fmt.Sprintf("desktop-everyday-accessibility-%s-%d.png", theme, viewport.width))
				if err := os.WriteFile(name, page.MustScreenshot(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestDesktopEverydayForcedColorsReducedMotionZoomBrowser(t *testing.T) {
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/settings": desktopEverydaySettingsHandler()})
	// A 1280x900 display at 200% zoom has a 640x450 CSS viewport.
	page.MustSetViewport(640, 450, 2, false)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{
		{Name: "forced-colors", Value: "active"}, {Name: "prefers-reduced-motion", Value: "reduce"},
	}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`async()=>{await AuraDesktopModules.loadAppAssets('settings');auditTest.openApp('settings',{category:'appearance',settingKey:'accessibility.text_size'});}`)
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`async theme=>{await auditTest.saveDesktopPreference('appearance.theme',theme);}`, theme)
		page.MustEval(`()=>{const input=document.querySelector('[data-setting-key="accessibility.text_size"]');input.focus();input.scrollIntoView({block:'nearest'});}`)
		if !page.MustEval(`()=>{
            const input=document.querySelector('[data-setting-key="accessibility.text_size"]'),r=input.getBoundingClientRect(),style=getComputedStyle(input);
            return matchMedia('(forced-colors: active)').matches&&matchMedia('(prefers-reduced-motion: reduce)').matches
                &&document.activeElement===input&&r.left>=0&&r.right<=innerWidth+1&&r.top>=0&&r.bottom<=innerHeight+1
                &&style.forcedColorAdjust!=='none'&&parseFloat(style.transitionDuration)<0.01;
        }`).Bool() {
			t.Fatalf("settings control must remain visible and respect forced colors/reduced motion at 200%% CSS viewport in %s", theme)
		}
	}
}
