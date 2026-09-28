package ui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func verifySystemWorldMotion(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{document.body.dataset.animations='false';}`)
	page.Timeout(30 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience?.society.residents.length===24`)
	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(s.motion.preference!=='auto'||!s.motion.reduced||document.querySelector('.sw-motion-status').hidden)throw Error('System pause must be visible');window.motionBefore=s;}`)
	time.Sleep(350 * time.Millisecond)
	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(s.frames<=motionBefore.frames||s.experience.machinery.time!==motionBefore.experience.machinery.time)throw Error('Quiet mode must render without advancing city life');}`)
	page.MustElement(".sw-world>summary").MustClick()
	page.MustElement("[data-world-motion]").MustSelect("Belebt")
	page.MustEval(`()=>{window.motionBefore=SysWorldApp.inspect(cityId);if(motionBefore.motion.reduced||document.querySelector('.sw-motion-status').hidden!==true)throw Error('Lively must override reduced motion');if(document.body.dataset.animations!=='false'||!matchMedia('(prefers-reduced-motion: reduce)').matches)throw Error('City control changed global preferences');}`)
	page.Timeout(15 * time.Second).MustWait(`()=>{const s=SysWorldApp.inspect(cityId);return ['resident-','patrol-','tram-','drone-'].every(prefix=>s.traffic.poses.some(p=>{const before=motionBefore.traffic.poses.find(b=>b.id===p.id);return p.id.startsWith(prefix)&&before&&Math.hypot(p.x-before.x,p.z-before.z)>.1;}))&&s.experience.machinery.time>motionBefore.experience.machinery.time;}`)
	page.MustScreenshot(filepath.Join(dir, "motion-lively.png"))
	page.MustElement("[data-world-motion]").MustSelect("Ruhig")
	page.MustEval(`()=>{window.motionBefore=SysWorldApp.inspect(cityId);if(!motionBefore.motion.reduced)throw Error('Explicit pause failed');}`)
	time.Sleep(350 * time.Millisecond)
	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId),moved=s.traffic.poses.filter(p=>{const b=motionBefore.traffic.poses.find(b=>b.id===p.id);return p.id!=='visitor'&&b&&Math.hypot(p.x-b.x,p.z-b.z)>.0001;});if(s.experience.machinery.time!==motionBefore.experience.machinery.time||moved.length)throw Error('Paused actors kept moving: '+JSON.stringify(moved));}`)
	page.MustElement("[data-world-motion]").MustSelect("Belebt")
	page.MustEval(`async()=>{fixtureCloseAll();await fixtureOpen('system-world');window.cityId=aurora.state.activeWindowId;}`)
	page.Timeout(30 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId)?.frames>4`)
	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(s.motion.preference!=='on'||s.motion.reduced)throw Error('Motion preference was not restored');document.querySelector('.sw-world').open=true;}`)
	page.MustElement("[data-world-motion]").MustSelect("Systemvorgabe")
	page.MustEval(`()=>{if(!SysWorldApp.inspect(cityId).motion.reduced)throw Error('System preference did not restore quiet mode');}`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{document.body.dataset.animations='true';}`)
	page.Timeout(5 * time.Second).MustWait(`()=>!SysWorldApp.inspect(cityId).motion.reduced`)
	page.MustSetViewport(430, 932, 2, true)
	page.MustEval(`()=>fixtureTheme('fruity-light')`)
	page.MustElement("[data-world-motion]").MustSelect("Ruhig")
	page.MustElement("[data-world-close]").MustClick()
	page.MustEval(`()=>{const n=document.querySelector('.sw-motion-status'),r=n.getBoundingClientRect();if(n.hidden||r.width<1||r.left<0||r.right>innerWidth)throw Error('Quiet-state hint is not visible on touch');}`)
	page.MustScreenshot(filepath.Join(dir, "motion-paused-touch.png"))
	page.MustEval(`()=>fixtureCloseAll()`)
	if errs := page.MustEval(`()=>JSON.stringify(cityErrors)`).Str(); errs != "[]" {
		t.Fatal(errs)
	}
}
