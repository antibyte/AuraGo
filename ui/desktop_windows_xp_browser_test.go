package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// Reproduce the Luna titlebar gap and broken shortcut column with real shell
// menus, rather than checking only that the XP classes were added.
func TestDesktopWindowsXPChromeBrowser(t *testing.T) {
	for _, viewport := range []struct {
		name   string
		width  int
		height int
		touch  bool
	}{
		{"desktop", 1920, 1080, false},
		{"touch", 420, 900, true},
	} {
		t.Run(viewport.name, func(t *testing.T) {
			s := newThemeSweepSession(t, "reduce")
			s.page.MustSetViewport(viewport.width, viewport.height, 1, viewport.touch)
			if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: viewport.touch}).Call(s.page); err != nil {
				t.Fatal(err)
			}
			s.store.set(map[string]string{"appearance.theme": "windows_xp", "appearance.icon_theme": "windows_xp", "appearance.wallpaper": "windows_xp"})
			s.pushSettings(t)
			s.eval(t, `()=>document.querySelectorAll('[data-i18n-placeholder]').forEach(el=>el.setAttribute('placeholder',t(el.dataset.i18nPlaceholder)))`)
			s.openApps(t, "infra", "files", "calculator")
			s.eval(t, `()=>{window.xpFiles=[...auditTest.state.windows.values()].find(w=>w.appId==='files');if(innerWidth>820){const infra=[...auditTest.state.windows.values()].find(w=>w.appId==='infra');if(!infra.maximized)auditTest.toggleMaximizeWindow(infra.id);Object.assign(xpFiles.element.style,{left:'380px',top:'100px',width:'920px',height:'650px'});}auditTest.focusWindow(xpFiles.id);}`)
			s.settle(t)
			dir := filepath.Join("..", "reports", "desktop-experience-themes")
			s.screenshot(t, filepath.Join(dir, "windows_xp-chrome-"+viewport.name+".png"))
			if !viewport.touch {
				checkDesktopXPRenderedFont(t, s, `.vd-window[data-app-id="files"] .vd-window-title`)
			}
			for _, mode := range []string{"normal", "maximized"} {
				if mode == "maximized" {
					s.eval(t, `()=>auditTest.toggleMaximizeWindow(xpFiles.id)`)
					s.settle(t)
				}
				result := s.eval(t, `()=>[...document.querySelectorAll('.vd-window')].filter(w=>w.getBoundingClientRect().height>0).map(w=>{
					const bar=w.querySelector('.vd-window-titlebar'),menu=w.querySelector('.vd-window-menubar'),
						group=w.querySelector('.vd-window-title-group'),content=w.querySelector('.vd-window-content');
					const b=bar.getBoundingClientRect(),g=group.getBoundingClientRect(),m=menu?.getBoundingClientRect(),c=content.getBoundingClientRect();
					const buttons=[...bar.querySelectorAll('.vd-window-button')].map(x=>x.getBoundingClientRect());
					return {app:w.dataset.appId,bar:b.height,menuTop:m?.top,barTop:b.top,group:g.height,contentGap:c.top-b.bottom,
						font:getComputedStyle(w.querySelector('.vd-window-title')).fontFamily,
						contained:buttons.every(r=>r.top>=b.top && r.bottom<=(m?.top||b.bottom)+1),
						joined:Math.abs(c.top-b.bottom)<=1 && (!m || Math.abs(m.top-(g.bottom))<=1),
						touch:!matchMedia('(pointer:coarse)').matches || buttons.every(r=>r.width>=44 && r.height>=44)};
				})`).Value
				t.Log(mode, result.String())
				for _, entry := range result.Arr() {
					if !entry.Get("contained").Bool() || !entry.Get("joined").Bool() || !entry.Get("touch").Bool() {
						t.Errorf("%s titlebar leaves a gap, clips controls or shrinks touch targets: %s", mode, entry.String())
					}
					if !strings.Contains(entry.Get("font").Str(), "Tahoma") || !strings.Contains(entry.Get("font").Str(), "sans-serif") {
						t.Errorf("XP shell lost its font or portable fallback: %s", entry.String())
					}
				}
			}
			s.page.MustElement(`.vd-window[data-app-id="files"] .vd-window-menu-button`).MustClick()
			s.waitFor(t, "reachable window menu", 3*time.Second, `()=>{const menu=xpFiles.element.querySelector('.vd-window-menu.open .vd-window-menu-popover');if(!menu)return false;const r=menu.getBoundingClientRect();return r.height>0 && r.top>=0 && r.bottom<=innerHeight && menu.contains(document.elementFromPoint(r.left+20,r.top+20));}`)
			s.closeAllWindows(t)
			s.eval(t, `()=>document.getElementById('vd-start-button').click()`)
			s.waitFor(t, "XP Start menu", 3*time.Second, `()=>!document.getElementById('vd-start-menu').hidden`)
			s.eval(t, `()=>document.querySelector('.vd-start-category[data-category="office"]')?.click()`)
			s.settle(t)
			s.screenshot(t, filepath.Join(dir, "windows_xp-start-"+viewport.name+".png"))
			if !viewport.touch {
				checkDesktopXPRenderedFont(t, s, `.vd-start-xp-header strong`)
			}
			result := s.eval(t, `()=>{
				const menu=document.querySelector('.vd-start-menu'),links=menu.querySelector('.vd-start-xp-links'),rail=menu.querySelector('.vd-start-rail');
				const l=links.getBoundingClientRect(),r=rail.getBoundingClientRect(),last=links.lastElementChild.getBoundingClientRect();
				return {fallbacks:[...menu.querySelectorAll('.vd-start-xp-header .vd-symbol-fallback,.vd-start-xp-links .vd-symbol-fallback')].map(x=>x.textContent),
					font:getComputedStyle(menu).fontFamily,headerFont:getComputedStyle(menu.querySelector('strong')).fontFamily,
					bodyFont:getComputedStyle(document.body).fontFamily,gap:r.top-last.bottom,
					noHorizontalOverflow:menu.scrollWidth<=menu.clientWidth && rail.scrollWidth<=rail.clientWidth,
					compact:r.top-last.bottom<=22 && Math.abs(r.top-l.bottom)<=1,
					aligned:[...links.children].every(b=>{const i=b.firstElementChild.getBoundingClientRect(),t=b.lastElementChild.getBoundingClientRect();return t.left>=i.right+3 && Math.abs(i.top+i.height/2-t.top-t.height/2)<=2;}),
					touch:!matchMedia('(pointer:coarse)').matches || [...links.children,...rail.querySelectorAll('button')].every(b=>b.getBoundingClientRect().height>=44)};
			}`).Value
			t.Log("start", result.String())
			if len(result.Get("fallbacks").Arr()) != 0 || !result.Get("compact").Bool() || !result.Get("aligned").Bool() || !result.Get("noHorizontalOverflow").Bool() || !result.Get("touch").Bool() {
				t.Errorf("broken XP Start layout or icons: %s", result.String())
			}
			if !strings.Contains(result.Get("headerFont").Str(), "Tahoma") {
				t.Errorf("XP header lost its theme typography: %s", result.String())
			}
			// Prove that the rebuilt shortcut column still opens its actual apps.
			for _, id := range []string{"system-info", "settings"} {
				s.page.MustElement(`[data-xp-app="` + id + `"]`).MustClick()
				s.waitFor(t, "XP shortcut opens "+id, 10*time.Second, `(id)=>document.getElementById('vd-start-menu').hidden && !!document.querySelector('.vd-window[data-app-id="'+id+'"] .vd-window-content > *')`, id)
				s.eval(t, `()=>document.getElementById('vd-start-button').click()`)
			}
			s.store.set(map[string]string{"appearance.theme": "standard", "appearance.icon_theme": "papirus"})
			s.pushSettings(t)
			if s.eval(t, `()=>document.querySelectorAll('.vd-start-xp-header,.vd-start-xp-links,.vd-start-xp-label').length`).Value.Int() != 0 {
				t.Fatal("XP-specific Start menu elements survived the theme change")
			}
		})
	}
}

func checkDesktopXPRenderedFont(t *testing.T, s *themeSweepSession, selector string) {
	t.Helper()
	if err := (proto.DOMEnable{}).Call(s.page); err != nil {
		t.Fatal(err)
	}
	if err := (proto.CSSEnable{}).Call(s.page); err != nil {
		t.Fatal(err)
	}
	doc, err := (proto.DOMGetDocument{}).Call(s.page)
	if err != nil {
		t.Fatal(err)
	}
	node, err := (proto.DOMQuerySelector{NodeID: doc.Root.NodeID, Selector: selector}).Call(s.page)
	if err != nil {
		t.Fatal(err)
	}
	fonts, err := (proto.CSSGetPlatformFontsForNode{NodeID: node.NodeID}).Call(s.page)
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts.Fonts) == 0 {
		t.Fatalf("XP text did not render: %s", selector)
	}
	for _, font := range fonts.Fonts {
		t.Logf("%s rendered font: %+v", selector, font)
		for _, serif := range []string{"Times", "Georgia", "Cambria", "Liberation Serif"} {
			if strings.Contains(font.FamilyName, serif) {
				t.Errorf("XP text fell back to a serif face: %s, %s", selector, font.FamilyName)
			}
		}
	}
}
