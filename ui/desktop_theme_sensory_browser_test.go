package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopThemeSensoryBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "reduce")
	s.eval(t, `() => {
        Object.defineProperty(document, 'hidden', {configurable:true, get:()=>false});
        Object.defineProperty(document, 'visibilityState', {configurable:true, get:()=>'visible'});
    }`)
	if s.eval(t, `()=>DesktopSounds.inspect().enabled || DesktopSounds.inspect().unlocked || DesktopSounds.inspect().bundleLoaded`).Value.Bool() {
		t.Fatal("sound must remain off, locked and unloaded at boot")
	}
	s.openApps(t, "settings")
	for _, id := range []string{"amiga", "cyberpunk", "black_gloss", "crazy", "lollipop", "organic"} {
		t.Run(id, func(t *testing.T) {
			s.store.set(map[string]string{"appearance.theme": id, "appearance.theme_cursors": "true", "sound.theme": "desktop"})
			s.pushSettings(t)
			got := s.eval(t, `()=>({cursor:document.body.dataset.cursorTheme,sound:DesktopSounds.inspect().theme,css:getComputedStyle(document.body).cursor})`)
			if got.Value.Get("cursor").Str() != id || got.Value.Get("sound").Str() != id || !strings.Contains(got.Value.Get("css").Str(), "/cursors/"+id+"/default.svg") {
				t.Fatalf("theme identity mismatch: %s", got.Value.String())
			}
			result := s.eval(t, `async(id)=>{
                const roles=['default','pointer','text','grab','grabbing','move','ew-resize','ns-resize','nesw-resize','nwse-resize','not-allowed','wait'];
                for (const role of roles) {
                    const img=new Image(); img.src='/img/cursors/'+id+'/'+role+'.svg'; await img.decode();
                    if(img.naturalWidth!==32 || img.naturalHeight!==32) return 'invalid size: '+role;
                }
                const cover=new Image(); cover.src='/img/themes/sounds/'+id+'.svg'; await cover.decode();
                const cases=[['button','pointer'],['textarea','text'],['div','grab','vd-window-titlebar'],['div','ew-resize','vd-resize-e'],['div','ns-resize','vd-resize-n'],['div','nesw-resize','vd-resize-ne'],['div','nwse-resize','vd-resize-nw']];
                for (const [tag,role,cls] of cases) {
                    const el=document.createElement(tag); el.className=cls||''; document.body.append(el);
                    const value=getComputedStyle(el).cursor; el.remove();
                    if(!value.includes('/'+id+'/'+role+'.svg')) return role+': '+value;
                }
                const disabled=document.createElement('button'); disabled.disabled=true; document.body.append(disabled);
                const blocked=getComputedStyle(disabled).cursor;
                disabled.setAttribute('aria-busy','true');
                const waiting=getComputedStyle(disabled).cursor; disabled.remove();
                if(!blocked.includes('/not-allowed.svg')) return 'disabled: '+blocked;
                if(!waiting.includes('/wait.svg')) return 'busy: '+waiting;
                const canvas=document.createElement('canvas'); canvas.style.cursor='crosshair'; document.body.append(canvas);
                const tool=getComputedStyle(canvas).cursor; canvas.remove();
                return tool==='crosshair'?'ok':'tool cursor overwritten';
            }`, id).Value.Str()
			if result != "ok" {
				t.Fatal(result)
			}
		})
	}
	// Off survives a theme switch; manually selected sounds remain independent.
	s.store.set(map[string]string{"appearance.theme_cursors": "false", "appearance.theme": "cyberpunk", "sound.theme": "wood"})
	s.pushSettings(t)
	if got := s.eval(t, `()=>({cursor:getComputedStyle(document.body).cursor,theme:DesktopSounds.inspect().theme})`).Value; strings.Contains(got.Get("cursor").Str(), "/cursors/") || got.Get("theme").Str() != "wood" {
		t.Fatalf("disabled cursor/manual sound mismatch: %s", got.String())
	}
	s.store.set(map[string]string{"appearance.theme_cursors": "true", "sound.theme": "desktop"})
	s.pushSettings(t)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "forced-colors", Value: "active"}}}).Call(s.page); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s.eval(t, `()=>getComputedStyle(document.body).cursor`).Value.Str(), "/cursors/") {
		t.Fatal("forced colors must keep the system cursor")
	}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(s.page); err != nil {
		t.Fatal(err)
	}
	// Drive the real persisted toggle and keyboard sound selection.
	s.closeAllWindows(t)
	s.openApps(t, "settings")
	s.eval(t, `()=>document.querySelector('[data-setting-key="appearance.theme_cursors"]').click()`)
	s.waitForStore(t, "cursor toggle saved", 5*time.Second, func(store *themeSweepStore) bool { return store.value("appearance.theme_cursors") == "false" })
	s.waitFor(t, "system cursor applied", 5*time.Second, `()=>document.body.dataset.cursorTheme==='system'`)
	s.eval(t, `()=>auditTest.openApp('settings',{category:'sound'})`)
	s.waitFor(t, "sound picker", 5*time.Second, `()=>!!document.querySelector('[data-sound-theme="desktop"]')`)
	s.eval(t, `()=>{ const el=document.querySelector('[data-sound-theme="organic"]'); el.focus(); el.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true})); }`)
	s.waitForStore(t, "keyboard sound selection", 5*time.Second, func(store *themeSweepStore) bool { return store.value("sound.theme") == "organic" })
	s.waitFor(t, "sound card focus restored", 5*time.Second, `()=>document.activeElement?.dataset.soundTheme==='organic' && document.activeElement.getAttribute('aria-pressed')==='true'`)
	// Reload proves both off switches and manual sound selection persist.
	s.page.MustReload().MustWaitLoad()
	s.waitFor(t, "reload", 10*time.Second, `()=>window.auditTest?.state.bootstrap && document.body.dataset.cursorTheme==='system'`)
	if s.eval(t, `()=>DesktopSounds.inspect().theme!=='organic' || DesktopSounds.inspect().enabled`).Value.Bool() {
		t.Fatal("saved sensory settings lost after reload")
	}
	s.eval(t, `()=>Object.defineProperty(document,'hidden',{configurable:true,get:()=>false})`)
	// Delay actual synthesis, mute mid-render, then release it. No late audio may start.
	s.eval(t, `async()=>{
        await AuraDesktopModules.loadBundle('desktop-sounds');
        auditTest.state.bootstrap.settings['sound.enabled']='true'; auditTest.applyDesktopSettings();
        document.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));
        const original=DesktopSoundSynth.renderLayers;
        let release;
        const gate=new Promise(resolve=>release=resolve);
        DesktopSoundSynth.renderLayers=async(...args)=>{await gate; return original(...args);};
        const before=DesktopSounds.inspect().plays;
        const pending=desktopSound('window.open');
        auditTest.state.bootstrap.settings['sound.enabled']='false'; auditTest.applyDesktopSettings();
        release(); await pending; DesktopSoundSynth.renderLayers=original;
        if(DesktopSounds.inspect().plays!==before || DesktopSounds.inspect().unlocked) throw Error('late audio after mute');
        auditTest.state.bootstrap.settings['sound.enabled']='true'; auditTest.applyDesktopSettings();
        document.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));
        if(!DesktopSounds.inspect().unlocked) throw Error('sound did not unlock after re-enable');
        await new Promise(r=>setTimeout(r,100)); await desktopSound('notify.message');
        if(DesktopSounds.inspect().plays===before) throw Error('re-enabled sound did not play');
        auditTest.state.bootstrap.settings['sound.enabled']='false'; auditTest.applyDesktopSettings();
        const previewBefore=DesktopSounds.inspect().plays;
        await previewDesktopSound('amiga');
        if(DesktopSounds.inspect().enabled || DesktopSounds.inspect().unlocked || DesktopSounds.inspect().plays<=previewBefore) throw Error('preview must play and release audio without enabling sounds');
    }`)
	// Original artwork contact sheet at native and enlarged sizes for visual inspection.
	artifacts := filepath.Join("..", "reports", "desktop-sensory")
	if err := os.MkdirAll(artifacts, 0755); err != nil {
		t.Fatal(err)
	}
	s.setViewport(t, 1280)
	s.eval(t, `()=>{
        const sheet=document.createElement('div'); sheet.id='sensory-artwork-sheet';
        sheet.style.cssText='position:fixed;inset:0;z-index:2147483647;background:#101827;color:#edf2fa;padding:32px;overflow:auto;font:16px system-ui;display:grid;grid-template-columns:repeat(3,1fr);gap:24px';
        for(const id of ['amiga','cyberpunk','black_gloss','crazy','lollipop','organic']){
            const card=document.createElement('section'); card.innerHTML='<h2>'+id.replace('_',' ')+'</h2><img src="/img/cursors/'+id+'/preview.svg" width="288" height="88">';
            for(const bg of ['#f3f4f6','#121a26']){
                const row=document.createElement('div'); row.style.cssText='background:'+bg+';border-radius:8px;padding:12px;margin-top:12px;display:flex;gap:14px';
                for(const role of ['default','pointer','text','grab','ew-resize']) row.innerHTML+='<img src="/img/cursors/'+id+'/'+role+'.svg" width="32" height="32">';
                card.append(row);
            } sheet.append(card);
        } document.body.append(sheet);
    }`)
	s.screenshot(t, filepath.Join(artifacts, "cursor-artwork.png"))
	s.eval(t, `()=>document.getElementById('sensory-artwork-sheet').remove()`)
	for _, width := range []int{1280, 390} {
		s.closeAllWindows(t)
		s.setViewport(t, width)
		s.store.set(map[string]string{"appearance.theme": "organic", "appearance.theme_cursors": "true"})
		s.pushSettings(t)
		s.openApps(t, "settings")
		s.eval(t, `()=>auditTest.openApp('settings',{category:'sound'})`)
		s.screenshot(t, filepath.Join(artifacts, fmt.Sprintf("sound-settings-%d.png", width)))
	}
	if rejected := s.store.rejections(); len(rejected) > 0 {
		t.Fatalf("rejected settings: %v", rejected)
	}
}
