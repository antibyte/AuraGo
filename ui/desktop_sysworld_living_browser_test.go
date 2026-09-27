package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func verifySystemWorldLiving(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	page.MustSetViewport(1366, 900, 1, false)
	page.Timeout(40 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.society.places.length===6`)
	page.MustEval(`()=>{document.body.dataset.animations='true';document.querySelector('[data-sw-action="close"]').click();const time=document.querySelectorAll('.sw-world select')[0];time.value='day';time.dispatchEvent(new Event('change'));}`)
	for _, id := range []string{"repair-bay", "parcel-sorter", "relay-mast", "kinetic-fountain", "glass-garden", "meeting-charge"} {
		page.MustEval(`id=>{document.querySelector('.sw-world').open=true;document.querySelector('[data-world-place="'+id+'"]').click();}`, id)
		page.Timeout(20*time.Second).MustWait(`id=>SysWorldApp.inspect(cityId).experience.details.some(p=>p.id===id&&p.level===0)`, id)
		page.Timeout(10 * time.Second).MustWait(`()=>!document.querySelector('.sw-interaction').hidden`)
		page.MustElement(".sw-interaction").MustClick()
		page.MustEval(`()=>{if(/sysworld\.world\./.test(document.querySelector('.sw-world').innerText))throw Error('Untranslated local interaction');}`)
		time.Sleep(600 * time.Millisecond)
		page.MustScreenshot(filepath.Join(dir, "living-"+id+".png"))
	}
	// Meet a real resident using the same E/touch controls as the user.
	page.MustEval(`()=>{document.querySelector('.sw-world').open=true;document.querySelector('[data-world-place="meeting-charge"]').click();}`)
	page.Timeout(70 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.interactions?.kind==='resident'`)
	page.MustElement(".sw-interaction").MustClick()
	page.MustElement(".sw-society .sw-world-verbs button").MustClick()
	page.MustEval(`()=>{if(!SysWorldApp.inspect(cityId).experience.society.residents.some(r=>r.state==='greet'))throw Error('Greeting did not stop a resident');}`)
	page.MustScreenshot(filepath.Join(dir, "living-greeting.png"))
	page.MustEval(`()=>{const s=document.querySelector('.sw-society select');if(!s)throw Error('Missing guide destinations');s.value='missions';document.querySelector('.sw-society>button').click();window.livingCamera=JSON.stringify(SysWorldApp.inspect(cityId).position);}`)
	page.Timeout(5 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.society.residents.some(r=>r.guided)`)
	time.Sleep(700 * time.Millisecond)
	page.MustEval(`()=>{if(JSON.stringify(SysWorldApp.inspect(cityId).position)!==livingCamera)throw Error('Guide took camera control');}`)
	page.MustEval(`()=>{document.querySelector('[data-sw-mode="orbit"]').click();document.querySelector('.sw-world').open=true;document.querySelector('[data-world-place="kinetic-fountain"]').click();fixtureTheme('fruity-light');}`)
	page.MustSetViewport(430, 932, 2, true)
	page.MustScreenshot(filepath.Join(dir, "living-touch.png"))
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{window.livingStill=SysWorldApp.inspect(cityId).experience.machinery.time;}`)
	time.Sleep(400 * time.Millisecond)
	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(s.experience.machinery.time!==livingStill||s.experience.society.signals.active!==0)throw Error('Reduced motion did not freeze local effects');}`)
	page.MustElement(".sw-interaction").MustClick()
	page.MustScreenshot(filepath.Join(dir, "living-reduced.png"))
	if errs := page.MustEval(`()=>JSON.stringify(cityErrors)`).Str(); errs != "[]" {
		t.Fatal(errs)
	}
	report := page.MustEval(`()=>JSON.stringify(SysWorldApp.inspect(cityId),null,2)`).Str()
	if err := os.WriteFile(filepath.Join(dir, "living-acceptance.json"), []byte(report), 0600); err != nil {
		t.Fatal(err)
	}
}
