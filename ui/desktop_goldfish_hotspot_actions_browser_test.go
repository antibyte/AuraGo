package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Drive the actual bar and independently picked scene pixels; the reducer only supplies the expected result.
func goldfishPointerAction(t *testing.T, page *rod.Page, id string, profile goldfishHotspotProfile, skip bool) {
	t.Helper()
	page = page.Timeout(45 * time.Second)
	step, savedState := "opening fixture", "not checked"
	defer func() {
		failure := recover()
		if failure != nil || t.Failed() {
			if base := goldfishHotspotArtifactDir(t, "actions"); base != "" {
				if screenshot, err := page.CancelTimeout().Screenshot(true, nil); err == nil {
					_ = os.WriteFile(filepath.Join(base, profile.Name+"-"+id+"-failure.png"), screenshot, 0o600)
				}
			}
		}
		if failure != nil {
			ui := "unavailable"
			func() {
				defer func() { _ = recover() }()
				ui = page.CancelTimeout().MustEval(`()=>{const app=document.querySelector('.agfish-app');return {line:document.querySelector('.agfish-line')?.textContent,app:app?.className,room:app?.dataset.room,startHidden:document.querySelector('.agfish-start')?.hidden,speechHidden:document.querySelector('.agfish-speech')?.hidden,busy:!!document.querySelector('.agfish-busy'),artLoading:!!document.querySelector('.agfish-art-loading')}}`).JSON("", "")
			}()
			t.Errorf("%s at %s: %v; saved=%s; UI=%s", id, step, failure, savedState, ui)
		}
	}()
	page.MustSetViewport(profile.ViewportWidth, profile.ViewportHeight, profile.DPR, false)
	step = "loading saved fixture"
	page.MustEval(`async (id, profile) => await GoldfishHotspotFixture.open(GoldfishHotspotFixture.actions[id], profile)`, id, profile)
	step = "resuming saved fixture"
	page.MustElement(".agfish-start .agfish-primary:not([hidden])").MustClick()
	step = "waiting for fixture readiness"
	page.MustEval(`async () => await GoldfishHotspotFixture.ready()`)
	record := page.MustEval(`id => GoldfishContent.actions.find(a=>a.id===id)`, id)
	room, target := record.Get("room").Str(), record.Get("hotspot").Str()
	step = "waiting for resumed room"
	page.Timeout(10*time.Second).MustWait(`room => {
        const app=document.querySelector('.agfish-app');
        return !!app&&app.dataset.room===room&&document.querySelector('.agfish-start').hidden&&!document.querySelector('.agfish-busy,.agfish-in-moment,.agfish-art-loading');
    }`, room)
	step = "checking persisted fixture state"
	saved := page.MustEval(`async id => {
            const store=await GoldfishSave.open(), slot=await store.read('auto'); store.close();
            const fixture=GoldfishHotspotFixture.actions[id], action=GoldfishContent.actions.find(a=>a.id===id);
            if (!slot||!fixture||!action) return {exists:false};
            const state=slot.state, fields=['room','flags','inventory','completed','applied','routes','protocols','ending'];
            return {exists:true,room:state.room,customsClear:state.flags.includes('customs_clear'),
                liveRoom:document.querySelector('.agfish-app')?.dataset.room||'',
                available:GoldfishState.available(state,action),unapplied:action.repeatable||!state.applied.includes(id),
                matchesFixture:fields.every(k=>JSON.stringify(state[k])===JSON.stringify(fixture[k]))};
        }`, id)
	savedState = saved.JSON("", "")
	if !saved.Get("exists").Bool() || saved.Get("room").Str() != room || saved.Get("liveRoom").Str() != room || !saved.Get("available").Bool() || !saved.Get("unapplied").Bool() || !saved.Get("matchesFixture").Bool() {
		t.Fatalf("%s did not resume its reducer-built reachable save: %s", id, savedState)
	}
	if id == "r05_check_invoice" && !saved.Get("customsClear").Bool() {
		t.Fatalf("%s must start from a saved R05 state with customs_clear: %s", id, savedState)
	}
	ref := goldfishReadArtReference(t, room)
	variant := page.MustEval(`id=>{const s=GoldfishHotspotFixture.actions[id];return s.ending||s.routes.hall||'';}`, id).Str()
	var point [2]float64
	for _, entry := range ref.Targets {
		if entry.ID == target {
			point = entry.Points[0]
			if points := entry.Variants[variant]; len(points) > 0 {
				point = points[0]
			}
		}
	}
	if point == [2]float64{} {
		t.Fatalf("%s has no art reference", id)
	}
	if item := record.Get("item").Str(); !record.Get("item").Nil() && item != "" {
		step = "selecting action item"
		page.MustElement(fmt.Sprintf("[data-agfish-item=%q]", item)).MustClick()
	} else {
		step = "selecting action verb"
		verb := page.MustEval(`id=>GoldfishCommands.ownVerb(GoldfishContent.actions.find(a=>a.id===id))`, id).Str()
		page.MustElement(fmt.Sprintf("[data-agfish-verb=%q]", verb)).MustClick()
	}
	step = "moving real pointer to art reference"
	goldfishPointerMove(page, point)
	step = "waiting for art reference to hit target"
	page.Timeout(8*time.Second).MustWait(`(point,target)=>{
        const r=document.querySelector('.agfish-canvas').getBoundingClientRect();
        const hit=document.elementFromPoint(r.x+point[0]*r.width,r.y+point[1]*r.height)?.closest('[data-agfish-hotspot]');
        return hit?.dataset.agfishHotspot===target;
    }`, point, target)
	step = "dispatching real left click"
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	step = "waiting for action result"
	page.Timeout(10 * time.Second).MustWait(`() => !!document.querySelector('[data-agfish-action], [data-agfish-option], .agfish-busy') || !document.querySelector('.agfish-speech').hidden`)
	selector := fmt.Sprintf("[data-agfish-action=%q]", id)
	if page.MustHas(selector) {
		step = "choosing action menu item"
		page.MustElement(selector).MustClick()
	}
	if record.Get("challenge").Nil() == false {
		step = "answering action challenge"
		answers := page.MustEval(`id => {const a=GoldfishContent.actions.find(a=>a.id===id).challenge.answer;return Array.isArray(a)?a:[a];}`, id).Arr()
		for _, answer := range answers {
			page.Timeout(10 * time.Second).MustElement(fmt.Sprintf("[data-agfish-option=%q]", answer.Str())).MustClick()
		}
	}
	step = "waiting for action dialogue"
	moment := page.MustEval(`id=>GoldfishHotspotFixture.moments().includes(id)`, id).Bool()
	if moment {
		// Rod's layout wait can outlive a short moment; observe and activate its visible control atomically.
		page.Timeout(10*time.Second).MustWait(`skip=>{
            if(!document.querySelector('.agfish-in-moment'))return false;
            if(skip){const button=document.querySelector('.agfish-skip-scene:not([hidden])');if(!button)return false;button.click();}
            return true;
        }`, skip)
	}
	page.Timeout(15 * time.Second).MustWait(`()=>!document.querySelector('.agfish-busy,.agfish-in-moment')&&!document.querySelector('.agfish-speech').hidden`)
	step = "verifying persisted action result"
	observed := page.MustEval(`async id=>{
        const a=GoldfishContent.actions.find(a=>a.id===id), before=GoldfishHotspotFixture.actions[id];
        const result=GoldfishState.apply(before,id,{item:a.item,answer:a.challenge?.answer});
        const store=await GoldfishSave.open(), saved=await store.read('auto');store.close();
        const hotspot=GoldfishContent.rooms.find(r=>r.id===a.room).hotspots.find(h=>h.id===a.hotspot);
        const fields=['flags','inventory','completed','applied','routes','protocols','ending'];
        const walkX=Math.max(0.08,Math.min(0.92,hotspot.walk[0]));
        return {expected:GoldfishContent.texts[result.lines[0].text], actual:document.querySelector('.agfish-line').textContent,
            reading:JSON.parse(localStorage.getItem('aurago.goldfish.speech')),
            lookExpected:GoldfishContent.texts[GoldfishState.lines(result.state,hotspot.look)[0].text],
            r07Invoice:saved.state.flags.includes('r07_invoice'),readHead:saved.state.inventory.includes('read_head'),
            walkExpectedX:walkX,walkActualX:saved.state.position.x,
            walkMatches:Math.abs(saved.state.position.x-walkX)<.01&&Math.abs(saved.state.position.y-hotspot.walk[1])<.01,
            stateMatches:fields.every(k=>JSON.stringify(saved.state[k])===JSON.stringify(result.state[k]))};
    }`, id)
	if observed.Get("expected").Str() != observed.Get("actual").Str() || !observed.Get("stateMatches").Bool() || !observed.Get("walkMatches").Bool() {
		t.Errorf("%s action result: %s", id, observed.JSON("", ""))
	}
	if id == "r05_check_invoice" && (!observed.Get("r07Invoice").Bool() || !observed.Get("readHead").Bool()) {
		t.Errorf("%s did not persist r07_invoice and read_head: %s", id, observed.JSON("", ""))
	}
	step = "dismissing action dialogue"
	page.MustEval(`()=>GoldfishHotspotFixture.dismiss()`)
	step = "moving real pointer to art reference for look"
	goldfishPointerMove(page, point)
	step = "waiting for look target under pointer"
	page.Timeout(8*time.Second).MustWait(`(point,target)=>{
        const r=document.querySelector('.agfish-canvas').getBoundingClientRect();
        const hit=document.elementFromPoint(r.x+point[0]*r.width,r.y+point[1]*r.height)?.closest('[data-agfish-hotspot]');
        return hit?.dataset.agfishHotspot===target;
    }`, point, target)
	step = "dispatching real right click for look"
	page.Mouse.MustClick(proto.InputMouseButtonRight)
	step = "waiting for look response"
	page.Timeout(8*time.Second).MustWait(`text=>!document.querySelector('.agfish-speech').hidden&&document.querySelector('.agfish-line').textContent===text`, observed.Get("lookExpected").Str())
	if base := goldfishHotspotArtifactDir(t, "actions"); base != "" {
		page.MustScreenshot(filepath.Join(base, fmt.Sprintf("%s-%s-skip%v.png", profile.Name, id, skip)))
		evidence := map[string]any{
			"room": room, "action": id, "profile": profile.Name, "point": point, "moment": moment, "skip": skip,
			"expected": observed.Get("expected").Str(), "observed": observed.Get("actual").Str(), "passed": !t.Failed(), "pointerRestored": true,
		}
		if id == "r05_check_invoice" {
			evidence["persistedEffects"] = map[string]bool{"r07_invoice": observed.Get("r07Invoice").Bool(), "read_head": observed.Get("readHead").Bool()}
		}
		goldfishWriteEvidence(t, filepath.Join(base, fmt.Sprintf("%s-%s-skip%v.json", profile.Name, id, skip)), evidence)
	}
}

func TestGoldfishAdventureHotspotActionsBrowser(t *testing.T) {
	page := goldfishHotspotFixture(t)
	ids := page.MustEval(`()=>GoldfishContent.actions.map(a=>a.id)`)
	if len(ids.Arr()) != page.MustEval(`()=>Object.keys(GoldfishHotspotFixture.actions).length`).Int() {
		t.Fatal("some authored actions lack a reachable fixture")
	}
	for _, id := range ids.Arr() {
		t.Run(id.Str(), func(t *testing.T) {
			goldfishPointerAction(t, page, id.Str(), goldfishHotspotProfiles[0], false)
		})
	}
}

func TestGoldfishAdventureHotspotCinematicRecoveryBrowser(t *testing.T) {
	page := goldfishHotspotFixture(t)
	ids := page.MustEval(`()=>GoldfishHotspotFixture.moments()`)
	if len(ids.Arr()) != 14 {
		t.Fatalf("expected 14 authored cinematics, got %d", len(ids.Arr()))
	}
	for _, profile := range goldfishHotspotProfiles {
		for _, id := range ids.Arr() {
			for _, skip := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/skip%v", profile.Name, id.Str(), skip), func(t *testing.T) {
					goldfishPointerAction(t, page, id.Str(), profile, skip)
				})
			}
		}
	}
}
