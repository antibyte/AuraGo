package ui

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDesktopAppIdentityIconsBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "reduce")
	want := map[string]string{
		"findit":            "/img/findit/icon.webp",
		"detective":         "/img/desktop-icons/detective.svg",
		"kaggle":            "/img/desktop-icons/kaggle.svg",
		"pet-picker":        "/img/desktop-icons/pet-picker.svg",
		"retro-games":       "/img/desktop-icons/retro-games.svg",
		"store-bytestash":   "/img/desktop-icons/bytestash.svg",
		"store-code-server": "/img/desktop-icons/code-server.svg",
	}
	s.eval(t, `() => {
		const a = auditTest;
		a.state.bootstrap = {...a.state.bootstrap, installed_apps: [
			{id:'store-bytestash',name:'ByteStash',icon:'code',category:'dev'},
			{id:'store-code-server',name:'code-server',icon:'code',category:'dev'},
			{id:'fixture-space',name:'Space Invaders',icon:'space-invaders'},
			{id:'fixture-logo',name:'Custom logo',icon:'search',metadata:{logo_path:'/img/desktop-icons/kaggle.svg'}}
		]};
		const ids = ['findit','detective','kaggle','pet-picker','retro-games','store-bytestash','store-code-server'];
		a.state.bootstrap.shortcuts = ids.map(id => ({id:'identity-'+id,target_type:'app',target_id:id,icon:'code'}));
		a.state.startCategory = 'all';
		a.state.startQuery = '';
		localStorage.setItem('aurago.desktop.recentApps.v1',JSON.stringify(['findit']));
		a.renderIcons(); a.renderStartApps();
		document.getElementById('vd-start-button').click();
	}`)
	for _, iconTheme := range []string{"papirus", "whitesur"} {
		for _, surface := range []struct{ theme, scheme string }{{"standard", "dark"}, {"fruity", "light"}, {"fruity", "dark"}} {
			s.store.set(map[string]string{"appearance.theme": surface.theme, "appearance.fruity_mode": surface.scheme, "appearance.icon_theme": iconTheme})
			s.pushSettings(t)
			s.eval(t, `() => {
				auditTest.renderIcons(); auditTest.renderStartApps();
				if (document.getElementById('vd-start-menu').hidden) document.getElementById('vd-start-button').click();
			}`)
			s.waitFor(t, "distinct app artwork in the menu and shortcuts", 15*time.Second, `(want,size) => {
				if (document.getElementById('vd-start-menu').hidden) return false;
				const paths = new Set();
				for (const [id,path] of Object.entries(want)) {
					const menu = document.querySelector('.vd-start-item[data-app-id="'+id+'"] img');
					const shortcut = document.querySelector('.vd-icon[data-id="identity-'+id+'"] img');
					for (const img of [menu,shortcut]) {
						if (!img || !img.complete || !img.naturalWidth || img.hidden || new URL(img.src).pathname !== path || new URL(img.src).searchParams.get('v') !== 'dev') return false;
					}
					const frame = menu.parentElement;
					if (Math.abs(frame.getBoundingClientRect().width-size)>1 || getComputedStyle(frame).padding !== '0px') return false;
					paths.add(new URL(menu.src).pathname);
				}
				return paths.size === Object.keys(want).length;
			}`, want, map[string]int{"standard": 30, "fruity": 38}[surface.theme])
			if !s.eval(t, `() => {
				const custom = document.querySelector('.vd-start-item[data-app-id="fixture-logo"] img');
				const studio = document.querySelector('.vd-start-item[data-app-id="code-studio"] img');
				return new URL(custom.src).pathname === '/img/desktop-icons/kaggle.svg' && studio.src.includes('/icons/code-studio.svg');
			}`).Value.Bool() {
				t.Fatal("app artwork lost to a generic icon or a dedicated theme mapping changed")
			}
			s.waitFor(t, "visible launcher and recent icons", 10*time.Second, `async () => {
				const icons = [document.querySelector('.vd-start-item[data-app-id="fixture-space"] img'), document.querySelector('[data-category="recent"] img')];
				for (const img of icons) {
					if (!img?.complete || !img.naturalWidth) return false;
					const canvas = document.createElement('canvas'); canvas.width = canvas.height = 32;
					const ctx = canvas.getContext('2d'); ctx.drawImage(img,0,0,32,32);
					const pixels = ctx.getImageData(0,0,32,32).data;
					let colored = 0;
					for (let i=0;i<pixels.length;i+=4) if (pixels[i+3]>100 && pixels[i+2]-pixels[i]>50) colored++;
					if (colored < 25) return false;
				}
				return true;
			}`)
			// Capture the same categories as the report, using the real menu and theme assets.
			for _, category := range []string{"office", "ai", "dev", "games", "installed"} {
				s.eval(t, `(category) => { auditTest.state.startCategory=category; auditTest.renderStartApps(); }`, category)
				s.screenshot(t, filepath.Join("reports", "desktop-app-identity-icons", iconTheme+"-"+surface.theme+"-"+surface.scheme+"-"+category+".png"))
			}
			s.eval(t, `() => { auditTest.state.startCategory='all'; auditTest.renderStartApps(); }`)
		}
	}
}
