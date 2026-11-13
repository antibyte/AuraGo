package ui

import (
	"fmt"
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
	defer func() {
		failure := recover()
		if failure != nil || t.Failed() {
			if base := goldfishHotspotArtifactDir(t, "actions"); base != "" {
				page.CancelTimeout().MustScreenshot(filepath.Join(base, profile.Name+"-"+id+"-failure.png"))
			}
		}
		if failure != nil {
			t.Errorf("%s: %v; UI=%s", id, failure, page.CancelTimeout().MustEval(`()=>({line:document.querySelector('.agfish-line')?.textContent, app:document.querySelector('.agfish-app')?.className})`).JSON("", ""))
		}
	}()
	page.MustSetViewport(profile.ViewportWidth, profile.ViewportHeight, profile.DPR, false)
	page.MustEval(`async (id, profile) => await GoldfishHotspotFixture.open(GoldfishHotspotFixture.actions[id], profile)`, id, profile)
	page.MustElement(".agfish-start .agfish-primary:not([hidden])").MustClick()
	page.MustEval(`async () => await GoldfishHotspotFixture.ready()`)
	record := page.MustEval(`id => GoldfishContent.actions.find(a=>a.id===id)`, id)
	room, target := record.Get("room").Str(), record.Get("hotspot").Str()
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
		page.MustElement(fmt.Sprintf("[data-agfish-item=%q]", item)).MustClick()
	} else {
		verb := page.MustEval(`id=>GoldfishCommands.ownVerb(GoldfishContent.actions.find(a=>a.id===id))`, id).Str()
		page.MustElement(fmt.Sprintf("[data-agfish-verb=%q]", verb)).MustClick()
	}
	goldfishPointerMove(page, point)
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	page.Timeout(10 * time.Second).MustWait(`() => !!document.querySelector('[data-agfish-action], [data-agfish-option], .agfish-busy') || !document.querySelector('.agfish-speech').hidden`)
	selector := fmt.Sprintf("[data-agfish-action=%q]", id)
	if page.MustHas(selector) {
		page.MustElement(selector).MustClick()
	}
	if record.Get("challenge").Nil() == false {
		answers := page.MustEval(`id => {const a=GoldfishContent.actions.find(a=>a.id===id).challenge.answer;return Array.isArray(a)?a:[a];}`, id).Arr()
		for _, answer := range answers {
			page.Timeout(10 * time.Second).MustElement(fmt.Sprintf("[data-agfish-option=%q]", answer.Str())).MustClick()
		}
	}
	moment := page.MustEval(`id=>GoldfishHotspotFixture.moments().includes(id)`, id).Bool()
	if moment {
		page.Timeout(10 * time.Second).MustWait(`()=>!!document.querySelector('.agfish-in-moment')`)
		if skip {
			page.MustElement(".agfish-skip-scene:not([hidden])").MustClick()
		}
	}
	page.Timeout(15 * time.Second).MustWait(`()=>!document.querySelector('.agfish-busy,.agfish-in-moment')&&!document.querySelector('.agfish-speech').hidden`)
	observed := page.MustEval(`async id=>{
        const a=GoldfishContent.actions.find(a=>a.id===id), before=GoldfishHotspotFixture.actions[id];
        const result=GoldfishState.apply(before,id,{item:a.item,answer:a.challenge?.answer});
        const store=await GoldfishSave.open(), saved=await store.read('auto');store.close();
        const hotspot=GoldfishContent.rooms.find(r=>r.id===a.room).hotspots.find(h=>h.id===a.hotspot);
        const fields=['flags','inventory','completed','applied','routes','protocols','ending'];
        return {expected:GoldfishContent.texts[result.lines[0].text], actual:document.querySelector('.agfish-line').textContent,
            lookExpected:GoldfishContent.texts[GoldfishState.lines(result.state,hotspot.look)[0].text],
            walkMatches:Math.abs(saved.state.position.x-hotspot.walk[0])<.01&&Math.abs(saved.state.position.y-hotspot.walk[1])<.01,
            stateMatches:fields.every(k=>JSON.stringify(saved.state[k])===JSON.stringify(result.state[k]))};
    }`, id)
	if observed.Get("expected").Str() != observed.Get("actual").Str() || !observed.Get("stateMatches").Bool() || !observed.Get("walkMatches").Bool() {
		t.Errorf("%s action result: %s", id, observed.JSON("", ""))
	}
	page.MustEval(`()=>GoldfishHotspotFixture.dismiss()`)
	goldfishPointerMove(page, point)
	page.Mouse.MustClick(proto.InputMouseButtonRight)
	page.Timeout(8*time.Second).MustWait(`text=>!document.querySelector('.agfish-speech').hidden&&document.querySelector('.agfish-line').textContent===text`, observed.Get("lookExpected").Str())
	if base := goldfishHotspotArtifactDir(t, "actions"); base != "" {
		page.MustScreenshot(filepath.Join(base, fmt.Sprintf("%s-%s-skip%v.png", profile.Name, id, skip)))
		goldfishWriteEvidence(t, filepath.Join(base, fmt.Sprintf("%s-%s-skip%v.json", profile.Name, id, skip)), map[string]any{
			"room": room, "action": id, "profile": profile.Name, "point": point, "moment": moment, "skip": skip,
			"expected": observed.Get("expected").Str(), "observed": observed.Get("actual").Str(), "passed": !t.Failed(), "pointerRestored": true,
		})
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
