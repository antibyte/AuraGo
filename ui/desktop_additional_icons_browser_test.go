package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDesktopAdditionalIconsAppearanceBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "reduce")
	s.openApps(t, "files", "settings", "writer", "sheets")
	s.eval(t, `() => {
		const panel = document.createElement('div'); panel.id='icon-role-probe';
		panel.innerHTML = auditTest.iconMarkup('documents','D','folder-probe',32,'folder') + auditTest.iconMarkup('documents','D','file-probe',32,'icon');
		panel.insertAdjacentHTML('beforeend', auditTest.iconMarkup('logo:/img/papirus/icons/folder.svg','A','logo-probe',32));
		document.body.append(panel);
		[...auditTest.state.windows.values()].find(w=>w.appId==='writer').element.querySelector('[data-window-menu]').click();
		auditTest.showContextMenu(60,160,[{id:'save',label:'Save',icon:'save'},{id:'folder',label:'Folder',icon:'folder'}]);
	}`)
	for _, theme := range []string{"tela", "candy", "fluent", "buuf_nestort"} {
		for _, scheme := range []string{"light", "dark"} {
			for _, shell := range []string{"minimal", "fruity"} {
				s.store.set(map[string]string{"appearance.theme": shell, "appearance.minimal_mode": scheme, "appearance.fruity_mode": scheme, "appearance.icon_theme": theme, "appearance.tela_folder_color": "purple", "appearance.fluent_folder_color": "teal"})
				s.pushSettings(t)
				base := "/img/" + strings.ReplaceAll(theme, "_", "-") + "/"
				s.waitFor(t, theme+" images", 15*time.Second, `(base) => {
					const images=[...document.querySelectorAll('img.vd-theme-image')];
					return images.length>30 && images.every(i=>i.complete && i.naturalWidth>0 && i.getAttribute('src').startsWith(base));
				}`, base)
				if !s.eval(t, `(scheme) => [...document.querySelectorAll('.vd-window-control-icon img')].every(i=>i.getAttribute('src').split('?')[0]===auditTest.themeIconPath(i.parentElement.dataset.vdIconKey,'action'))`, scheme).Value.Bool() {
					t.Fatal("window controls did not follow surface scheme")
				}
				if !s.eval(t, `() => {
					const folder=document.querySelector('.folder-probe img').src, file=document.querySelector('.file-probe img').src;
					return folder.endsWith(auditTest.themeIconPath('documents','folder')+'?v=dev') && file.endsWith(auditTest.themeIconPath('documents','icon')+'?v=dev') && folder!==file;
				}`).Value.Bool() {
					t.Fatal("folder/file roles mixed")
				}
				if !s.eval(t, `() => document.querySelector('.logo-probe [data-vd-icon-key]').hidden`).Value.Bool() {
					t.Fatal("theme refresh exposed the hidden app-logo fallback")
				}
				// Open menu content remains themed, including Writer and Sheets menus.
				if !s.eval(t, `() => {const icons=[...document.querySelectorAll('.vd-context-menu img')];return icons.length===2 && icons.every(i=>i.getAttribute('src').split('?')[0]===auditTest.themeIconPath(i.parentElement.dataset.vdIconKey,'action'));}`).Value.Bool() {
					t.Fatal("open context menu did not refresh")
				}
				if !s.eval(t, `(base) => [...document.querySelectorAll('.vd-window-menu-papirus-icon img')].every(i=>i.getAttribute('src').split('?')[0]===auditTest.themeIconPath(i.parentElement.dataset.vdIconKey,'action'))`, base).Value.Bool() {
					t.Fatal("menu icons did not refresh")
				}
				if shell == "minimal" {
					s.screenshot(t, filepath.Join("reports", "desktop-icon-themes", theme+"-"+scheme+".png"))
				}
			}
		}
	}
	// Exercise the real settings controls and their serialized PUTs.
	s.closeAllWindows(t)
	s.openApps(t, "settings")
	s.eval(t, `()=>{const select=document.querySelector('[data-setting-key="appearance.icon_theme"]');for(const value of ['candy','fluent','tela']){select.value=value;select.dispatchEvent(new Event('change',{bubbles:true}));}}`)
	s.waitFor(t, "queued theme saves", 10*time.Second, `()=>document.querySelector('[data-setting-key="appearance.tela_folder_color"]')!==null && auditTest.settingValue('appearance.icon_theme')==='tela'`)
	s.eval(t, `()=>{const select=document.querySelector('[data-setting-key="appearance.tela_folder_color"]');select.value='dracula';select.dispatchEvent(new Event('change',{bubbles:true}));}`)
	s.waitForStore(t, "Tela folder color saved", 10*time.Second, func(store *themeSweepStore) bool { return store.value("appearance.tela_folder_color") == "dracula" })
	// A failed save must leave the visible and persisted previous selection intact.
	s.eval(t, `()=>{const original=window.fetch;window.fetch=async(url,options)=>{if(String(url).includes('/api/desktop/settings') && options?.method==='PUT'){window.fetch=original;return new Response('{"error":"fixture save failed"}',{status:500,headers:{'Content-Type':'application/json'}});}return original(url,options);};const select=document.querySelector('[data-setting-key="appearance.icon_theme"]');select.value='candy';select.dispatchEvent(new Event('change',{bubbles:true}));}`)
	s.waitFor(t, "failed save restored", 10*time.Second, `()=>document.querySelector('[data-setting-key="appearance.icon_theme"]').value==='tela'`)
	if s.store.value("appearance.icon_theme") != "tela" {
		t.Fatal("failed save persisted a theme")
	}
	// Missing motif/file/manifest must clear the previous artwork and recover on the next switch.
	s.eval(t, `()=>{
		const a=auditTest;window.savedCandy=a.state.iconThemeManifests.candy;
		a.state.bootstrap.settings['appearance.icon_theme']='candy';
		a.state.iconThemeManifests.candy={...savedCandy,icons:{...savedCandy.icons,documents:'img/candy/icons/nonexistent.svg'}};
		a.applyDesktopSettings();
	}`)
	s.waitFor(t, "missing image fallback", 10*time.Second, `()=>document.querySelector('.file-probe img')?.hidden && !document.querySelector('.file-probe .vd-theme-image-fallback').hidden`)
	s.eval(t, `()=>{delete auditTest.state.iconThemeManifests.candy;auditTest.applyDesktopSettings();}`)
	if s.eval(t, `()=>!!document.querySelector('#icon-role-probe .file-probe img, #icon-role-probe .folder-probe img')`).Value.Bool() {
		t.Fatal("missing manifest retained old artwork")
	}
	s.eval(t, `()=>{auditTest.state.iconThemeManifests.candy=savedCandy;for(const theme of ['fluent','candy','tela','whitesur','windows_xp','papirus']){auditTest.state.bootstrap.settings['appearance.icon_theme']=theme;auditTest.applyDesktopSettings();}}`)
	s.waitFor(t, "return to Papirus", 10*time.Second, `()=>document.querySelector('.file-probe img')?.src.includes('/img/papirus/')`)
	// Mobile hit targets belong to buttons, never to the decorative image.
	s.setViewport(t, 390)
	if !s.eval(t, `()=>{
		const buttons=[...document.querySelectorAll('.vd-window-button')];
		const sizes=buttons.map(b=>[b.getBoundingClientRect().width,b.getBoundingClientRect().height]);
		buttons[0].focus();auditTest.state.bootstrap.settings['appearance.icon_theme']='candy';auditTest.applyDesktopSettings();
		return buttons.length>0 && document.activeElement===buttons[0] && buttons.every((b,i)=>b.getBoundingClientRect().width===sizes[i][0] && b.getBoundingClientRect().height===sizes[i][1]);
	}`).Value.Bool() {
		t.Fatal("theme change altered mobile hit targets or keyboard focus")
	}
	if errors := s.eval(t, `()=>fixtureErrors`).Value.String(); errors != "[]" {
		t.Logf("browser events: %s", errors)
	}
}

func TestDesktopAdditionalIconsSDKChannelBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "reduce")
	s.store.set(map[string]string{"appearance.icon_theme": "tela", "appearance.tela_folder_color": "purple"})
	s.pushSettings(t)
	s.eval(t, `async()=>{
		const frame=document.createElement('iframe');frame.dataset.appId='editor';frame.id='icon-sdk-frame';document.body.append(frame);
		window.iconSDK={ports:[],held:[],hold:false};
		window.iconSDKReady=callback=>{
			iconSDK.connect=()=>{
				const pair=new MessageChannel(),client=auditTest.sdkFrameClient(frame);client.port=pair.port1;client.generation++;const generation=client.generation;
				pair.port1.addEventListener('message',async event=>{
					if(iconSDK.hold && event.data.action==='desktop:context'){
						iconSDK.held.push({port:pair.port1,message:{type:'aurago.desktop.response',id:event.data.id,ok:true,payload:await auditTest.runSDKAction(client,'desktop:context',{})}});return;
					}
					auditTest.handleSDKMessage(client,event,pair.port1,generation);
				});pair.port1.start();iconSDK.ports.push(pair.port1);callback(pair.port2);
			};iconSDK.connect();
		};
		frame.srcdoc='<html><head><link rel="stylesheet" href="/css/desktop-sdk.css"></head><body><script>window.__AURAGO_DESKTOP_SDK_CHANNEL__={onPort:cb=>parent.iconSDKReady(cb)};<\/script><script src="/js/desktop/aura-desktop-sdk.js"><\/script><script>document.body.append(AuraDesktop.ui.icon("documents",{usage:"folder"}),AuraDesktop.ui.icon("documents"),AuraDesktop.ui.button({icon:"save",label:"Save"}));<\/script></body></html>';
	}`)
	waitSDK := func(theme string) {
		t.Helper()
		s.waitFor(t, theme+" SDK icons", 15*time.Second, `(theme)=>{const doc=document.querySelector('#icon-sdk-frame').contentDocument;const imgs=[...doc.querySelectorAll('.ad-icon img')];return imgs.length===3 && imgs.every(i=>i.complete && i.naturalWidth>0 && i.src.includes('/img/'+theme+'/'));}`, strings.ReplaceAll(theme, "_", "-"))
	}
	waitSDK("tela")
	for _, theme := range []string{"fluent", "candy", "buuf_nestort"} {
		s.store.set(map[string]string{"appearance.icon_theme": theme, "appearance.fluent_folder_color": "teal", "appearance.theme": "fruity", "appearance.fruity_mode": "dark"})
		s.pushSettings(t)
		waitSDK(theme)
	}
	if !s.eval(t, `()=>document.querySelector('#icon-sdk-frame').contentDocument.querySelector('.ad-button img').getAttribute('src').split('?')[0]===auditTest.themeIconPath('save','action')`).Value.Bool() {
		t.Fatal("SDK button did not request an action icon")
	}
	// Keep a context response in flight while appearance changes on the same port.
	s.eval(t, `()=>{iconSDK.hold=true;iconSDK.connect();}`)
	s.waitFor(t, "held context", 10*time.Second, `()=>iconSDK.held.length>0`)
	s.eval(t, `()=>{iconSDK.hold=false;}`)
	s.store.set(map[string]string{"appearance.icon_theme": "tela"})
	s.pushSettings(t)
	waitSDK("tela")
	s.eval(t, `()=>{for(const item of iconSDK.held)item.port.postMessage(item.message);iconSDK.held=[];}`)
	waitSDK("tela")
	// A revoked document's port must not change the new connection's icon state.
	s.eval(t, `()=>{const old=iconSDK.ports.at(-1);iconSDK.connect();old.postMessage({type:'aurago.desktop.appearance',payload:{revision:9999,settings:{'appearance.icon_theme':'candy'},scheme:'light'}});}`)
	waitSDK("tela")
	s.store.set(map[string]string{"appearance.icon_theme": "papirus"})
	s.pushSettings(t)
	waitSDK("papirus")
	s.screenshot(t, filepath.Join("reports", "desktop-icon-themes", "sdk-return-papirus.png"))
	t.Log(fmt.Sprintf("SDK appearance, action/folder roles, delayed contexts and replaced MessagePorts verified"))
}

func TestDesktopAdditionalIconsVisualCatalogBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "reduce")
	s.page.MustSetViewport(1280, 1650, 1, false)
	for _, theme := range []string{"tela", "candy", "fluent", "buuf_nestort"} {
		for _, scheme := range []string{"light", "dark"} {
			s.store.set(map[string]string{"appearance.icon_theme": theme, "appearance.theme": "fruity", "appearance.fruity_mode": scheme})
			s.pushSettings(t)
			s.eval(t, `(theme,scheme)=>{
				document.querySelector('#icon-catalog-proof')?.remove();
				const panel=document.createElement('section');panel.id='icon-catalog-proof';
				panel.style.cssText='position:fixed;inset:0;z-index:999999;padding:20px;overflow:auto;font:11px system-ui;background:'+(scheme==='dark'?'#191d24;color:#e7ebf4':'#faf9f6;color:#242730');
				const manifest=auditTest.state.iconThemeManifests[theme];
				const row=(key,role,sizes)=>'<div style="height:72px;text-align:center;display:flex;align-items:center;justify-content:center;flex-wrap:wrap;gap:5px">'+sizes.map(size=>auditTest.iconMarkup(key,key,'proof-icon',size,role)).join('')+'<small style="width:100%;font-size:9px">'+key+'</small></div>';
				panel.innerHTML='<h2 style="font:600 20px system-ui;margin:0 0 12px">'+manifest.name+' · '+scheme+'</h2><div style="display:grid;grid-template-columns:repeat(16,1fr);gap:4px">'+Object.keys(manifest.icons).map(key=>row(key,'icon',[42])).join('')+'</div><h3 style="font:600 16px system-ui">Controls · 14 / 20 / 24 px</h3><div style="display:grid;grid-template-columns:repeat(12,1fr);gap:4px">'+Object.keys(manifest.actions[scheme]).map(key=>row(key,'action',[14,20,24])).join('')+'</div>';
				document.body.append(panel);
			}`, theme, scheme)
			s.waitFor(t, "complete icon catalog", 15*time.Second, `()=>[...document.querySelectorAll('#icon-catalog-proof img')].every(i=>i.complete && i.naturalWidth>0)`)
			s.screenshot(t, filepath.Join("reports", "desktop-icon-themes", "catalog-"+theme+"-"+scheme+".png"))
		}
	}
}
