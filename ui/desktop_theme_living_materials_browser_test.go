package ui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestDesktopLivingMaterialsIdleBrowser(t *testing.T) {
	for _, id := range []string{"green_slime", "electronic_dreams"} {
		t.Run(id, func(t *testing.T) {
			s := newThemeSweepSession(t, "no-preference")
			s.page.MustSetViewport(1600, 1000, 1, false)
			s.store.set(map[string]string{"appearance.theme": id, "appearance.wallpaper": id, "windows.animations": "true"})
			s.pushSettings(t)
			s.openApps(t, "calendar", "calculator")
			s.eval(t, `()=>{let i=0;for(const w of auditTest.state.windows.values()){Object.assign(w.element.style,i++?{left:'1080px',top:'260px',width:'380px',height:'530px'}:{left:'130px',top:'120px',width:'820px',height:'650px'});}dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
			s.waitFor(t, "living material renderer", 10*time.Second, `()=>AuraDesktopThemeFx?.inspect().motion && AuraDesktopThemeFx.inspect().windowCount===2`)
			s.eval(t, `()=>{const canvas=document.querySelector('.vd-theme-fx-layer canvas');window.idleMaterialSample=canvas.getContext('2d').getImageData(0,0,canvas.width,canvas.height).data;window.idleMaterialClock=AuraDesktopThemeFx.inspect().frameCount;}`)
			s.waitFor(t, "visible idle material change", 5*time.Second, `()=>{if(AuraDesktopThemeFx.inspect().frameCount-idleMaterialClock<30)return false;const canvas=document.querySelector('.vd-theme-fx-layer canvas'),next=canvas.getContext('2d').getImageData(0,0,canvas.width,canvas.height).data;let changed=0;for(let i=0;i<next.length;i+=4)if(next[i]!==idleMaterialSample[i]||next[i+1]!==idleMaterialSample[i+1]||next[i+3]!==idleMaterialSample[i+3])changed++;window.idleMaterialChanged=changed;return changed>700;}`)
			if id == "green_slime" {
				s.waitFor(t, "growing drips detach without input", 8*time.Second, `()=>AuraDesktopThemeFx.inspect().dropCount>=3 && AuraDesktopThemeFx.inspect().currentMaxDisplacement>.2`)
				if !s.eval(t, `()=>[...document.querySelectorAll('.vd-window-button')].every(button=>getComputedStyle(button,'::before').display!=='none')`).Value.Bool() {
					t.Fatal("native control icons suppress the decorative slime surface")
				}
			} else {
				if !s.eval(t, `()=>[...document.querySelectorAll('.vd-window-titlebar')].every(bar=>{const r=bar.getBoundingClientRect(),controls=[...bar.querySelectorAll('.vd-window-button,.vd-window-menubar')].map(el=>el.getBoundingClientRect().bottom),content=bar.parentElement.querySelector('.vd-window-content')?.getBoundingClientRect();return r.height>=60 && Math.max(...controls)<=r.bottom-23 && (!content||content.top>=r.bottom-.5);})`).Value.Bool() {
					t.Fatal("electronic component strip overlaps menu, controls or application content")
				}
			}
			s.screenshot(t, filepath.Join("..", "reports", "desktop-experience-themes", id+"-living.png"))
			if errors := s.eval(t, `()=>window.fixtureErrors`).Value.String(); errors != "[]" {
				t.Fatalf("material errors: %s", errors)
			}
		})
	}
}

func TestDesktopVoxelRestingPileQuietBrowser(t *testing.T) {
	s := newThemeSweepSessionWithBrowser(t, "no-preference", func(l *launcher.Launcher) { l.Delete("disable-gpu").Set("enable-unsafe-swiftshader") })
	s.store.set(map[string]string{"appearance.theme": "voxel_world", "windows.animations": "true"})
	s.pushSettings(t)
	s.waitFor(t, "Rapier", 30*time.Second, `()=>AuraDesktopThemeFx?.inspect().physicsReady`)
	s.openApps(t, "calendar")
	s.eval(t, `()=>{window.pileImpacts=[];const original=DesktopSounds.play;DesktopSounds.play=function(event,options){if(event==='window.impact')pileImpacts.push({at:performance.now(),strength:options.strength});return original.apply(this,arguments);};const w=[...auditTest.state.windows.values()][0];Object.assign(w.element.style,{left:'120px',top:'150px',width:'900px',height:'540px'});dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));window.pileClosedAt=performance.now();void auditTest.closeWindow(w.id);}`)
	s.waitFor(t, "dense physical fragments", 3*time.Second, `()=>AuraDesktopThemeFx.inspect().fragmentCount>30`)
	s.waitFor(t, "falling fragments have settled", 12*time.Second, `()=>AuraDesktopThemeFx.inspect().fragmentCount===0`)
	if got := s.eval(t, `()=>({count:pileImpacts.length,late:pileImpacts.filter(hit=>hit.at-pileClosedAt>4500).length,max:Math.max(0,...pileImpacts.map(hit=>hit.strength))})`).Value; got.Get("count").Int() < 1 || got.Get("count").Int() > 16 || got.Get("late").Int() != 0 || got.Get("max").Num() > .65 {
		t.Fatalf("resting/contact-pressure sound regression: %s", got.String())
	}
}
