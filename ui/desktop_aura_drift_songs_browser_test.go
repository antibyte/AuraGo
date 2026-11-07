package ui

import (
	"testing"
	"time"
)

// Each Aura Drift biome plays one generated song: a race on two different tracks must start the
// matching looping song file (real fetch + decode of the shipped Ogg), without a console error.
func TestDesktopAuraDriftSongsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, tc := range []struct{ track, biome string }{
		{"alpenpass", "alpine"},
		{"neon-hafen", "harbour"},
	} {
		t.Run(tc.biome, func(t *testing.T) {
			page := auraDriftFixture(t)
			auraDriftReopen(page, map[string]interface{}{"muted": false, "volume": 0.05}, "")
			// Record every started buffer source (loop flag, length, channels) before the game creates its audio graph.
			page.MustEval(`()=>{
				window.__songSources=[];
				const proto=window.BaseAudioContext&&BaseAudioContext.prototype;
				const make=proto.createBufferSource;
				proto.createBufferSource=function(){
					const node=make.apply(this,arguments), start=node.start;
					node.start=function(){ try{ window.__songSources.push({loop:!!node.loop,duration:node.buffer?node.buffer.duration:0,channels:node.buffer?node.buffer.numberOfChannels:0}); }catch(e){} return start.apply(this,arguments); };
					return node;
				};
			}`)
			want := page.MustEval(`async biome=>{
				const m=await (await fetch('/3d/aura-drift/audio/manifest.json')).json();
				const path=m.music[biome];
				if(typeof path!=='string')return -1;
				return m.files.find(f=>f.path===path).seconds;
			}`, tc.biome).Num()
			if want <= 0 {
				t.Fatalf("manifest has no single-file song for %s", tc.biome)
			}
			page.MustElement(`.agdrift-tab[data-tab="track"]`).MustClick()
			auraDriftClickTrack(t, page, tc.track)
			page.MustElement(".agdrift-menu .agdrift-primary").MustClick()
			auraDriftSkipCountdown(t, page)

			deadline := time.Now().Add(75 * time.Second)
			found := false
			var raw string
			for time.Now().Before(deadline) && !found {
				auraDriftSleep(page, 300)
				found = page.MustEval(`w=>window.__songSources.some(s=>s.loop&&s.channels===2&&Math.abs(s.duration-w)<0.05)`, want).Bool()
				raw = page.MustEval(`()=>JSON.stringify(window.__songSources)`).Str()
			}
			t.Logf("%s: wanted a looping stereo source of %.3f s; started sources: %s", tc.biome, want, raw)
			if !found {
				t.Fatalf("%s: the song scene never started", tc.biome)
			}
			if page.MustEval(`()=>window.__songSources.filter(s=>s.loop&&s.channels===2&&s.duration>20).length`).Int() > 2 {
				t.Fatalf("%s: more than one long looping music source started (stems still playing?): %s", tc.biome, raw)
			}
			auraDriftCloseWindows(page)
			auraDriftNoErrors(t, page)
		})
	}
}
