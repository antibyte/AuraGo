package ui

import (
	"path/filepath"
	"testing"
)

func TestDesktopAllAppIconsBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "reduce")
	for _, theme := range []string{"papirus", "whitesur", "windows_xp", "tela", "candy", "fluent", "buuf_nestort"} {
		for _, scheme := range []string{"light", "dark"} {
			t.Run(theme+"/"+scheme, func(t *testing.T) {
				s.store.set(map[string]string{"appearance.theme": "fruity", "appearance.fruity_mode": scheme, "appearance.icon_theme": theme})
				s.pushSettings(t)
				result := s.eval(t, `async()=>{
                    const a=auditTest;
                    a.state.startCategory='all';a.state.startQuery='';a.renderStartApps();
                    if(document.getElementById('vd-start-menu').hidden)document.getElementById('vd-start-button').click();
                    document.getElementById('app-icon-probe')?.remove();
                    const probe=document.createElement('div');probe.id='app-icon-probe';
                    Object.assign(probe.style,{position:'fixed',right:'24px',top:'60px',padding:'16px',zIndex:99999,background:'var(--vd-theme-app-bg)',color:'var(--vd-theme-text)',display:'grid',gap:'10px'});
                    const keys=['film','flip','next','previous','shuffle','repeat','history','flask','sidebar','zoom-fit','rotate'];
                    probe.innerHTML=keys.map(key=>'<div data-probe-key="'+key+'">'+a.iconMarkup(key,key,'vd-window-menu-papirus-icon',16)+' '+key+'</div>').join('');
                    document.body.append(probe);
                    const menu=document.getElementById('vd-start-menu');
                    const apps=a.state.bootstrap.builtin_apps;
                    const missing=[];
                    for(const app of apps){
                        if(app.internal===true||app.start_visible===false){
                            const row=document.createElement('div');row.dataset.probeKey=app.id;
                            row.innerHTML=a.iconMarkup(app.icon,app.name,'vd-window-header-icon',24)+' '+app.id;probe.append(row);
                            continue;
                        }
                        const item=[...menu.querySelectorAll('[data-app-id]')].find(el=>el.dataset.appId===app.id);
                        if(!item||item.querySelector('.vd-icon-letter,.vd-symbol-fallback'))missing.push(app.id);
                    }
                    for(const slot of probe.children){
                        if(slot.querySelector('.vd-icon-letter,.vd-symbol-fallback')||!slot.querySelector('svg,img,.vd-mini-icon'))missing.push(slot.dataset.probeKey);
                    }
                    const images=[...menu.querySelectorAll('img'),...probe.querySelectorAll('img')];
                    images.forEach(img=>img.loading='eager');
                    await Promise.all(images.map(async img=>{try{await img.decode();}catch{missing.push(img.getAttribute('src'));}}));
                    return {missing,apps:apps.length,actions:keys.length,images:images.length};
                }`)
				if len(result.Value.Get("missing").Arr()) != 0 {
					t.Fatalf("missing app/action icons: %s", result.Value.JSON("", "  "))
				}
				t.Logf("checked %d apps and %d shared actions", result.Value.Get("apps").Int(), result.Value.Get("actions").Int())
				if scheme == "dark" {
					s.screenshot(t, filepath.Join("reports", "desktop-icon-audit-20261108", theme+".png"))
				}
			})
		}
	}
}
