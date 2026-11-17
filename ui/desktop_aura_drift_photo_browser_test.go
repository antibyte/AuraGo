package ui

import (
	"path/filepath"
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopAuraDriftTrackPhotoBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, tc := range []struct {
		renderer, omit string
		width          int
		reduced, honk  bool
	}{
		{"three", "", 1280, false, true},
		{"canvas2d", "aura-drift-render3d.js", 560, true, false},
	} {
		t.Run(tc.renderer, func(t *testing.T) {
			page := auraDriftFixtureWithout(t, tc.omit)
			if tc.renderer == "three" {
				requireWebGL2(t, page)
			}
			page.MustSetViewport(tc.width, 720, 1, false)
			if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
				t.Fatal(err)
			}
			auraDriftReopen(page, map[string]interface{}{"quality": "low", "voice": false, "reducedMotion": tc.reduced}, "")
			page.MustEval(`async()=>{await AuraDriftApp.debug.start({mode:'drift_attack',trackId:'alpenpass',seed:2});}`)
			artifacts := auraDriftArtifactDir(t, "track-photo-"+tc.renderer)
			if artifacts != "" {
				page.MustElement(".agdrift-root").MustScreenshot(filepath.Join(artifacts, "start.png"))
			}
			if tc.honk {
				auraDriftKeyDown(t, page, input.KeyH)
				auraDriftKeyUp(t, page, input.KeyH)
			}
			auraDriftSkipCountdown(t, page)
			auraDriftKeyDown(t, page, input.ArrowUp)
			auraDriftDrive(page, 3)
			page.MustEval(`(renderer,honk,reduced)=>{
				const d=AuraDriftApp.debug;
				d.launch(12);
				d.fastForward(.25,{renderEvery:15});
				const card=document.querySelector('.agdrift-root > .agdrift-photo');
				if(!card || (getComputedStyle(card).animationName==='none')!==reduced) throw Error('Missing print or incorrect motion preference');
				if(AuraDriftApp.stats()[0].renderer.kind!==renderer) throw Error('Wrong photo source');
				const image=card.querySelector('canvas'), pixels=image.getContext('2d').getImageData(0,0,image.width/2,image.height).data;
				let bright=0;
				for(let i=0;i<pixels.length;i+=4) if(pixels[i]+pixels[i+1]+pixels[i+2]>70 && pixels[i+3]>0) bright++;
				if(bright<pixels.length/40) throw Error('Empty race photo');
				const key='aura_drift.photo.'+(honk?'duck':'air');
				if(!window.t(key).split('\n').includes(card.querySelector('figcaption span').textContent)) throw Error('Wrong photo caption');
				window.testRacePrint=card;
			}`, tc.renderer, tc.honk, tc.reduced)
			auraDriftKeyUp(t, page, input.ArrowUp)
			if artifacts != "" {
				page.MustElement(".agdrift-root").MustScreenshot(filepath.Join(artifacts, "race.png"))
			}
			page.MustEval(`()=>{
				const d=AuraDriftApp.debug;
				d.fastForward(5,{renderEvery:200});
				if(document.querySelector('.agdrift-photo')) throw Error('Photo stayed on track');
				d.fastForward(95,{renderEvery:100000});
				const card=document.querySelector('.agdrift-results .agdrift-photo');
				if(card!==window.testRacePrint || document.querySelectorAll('.agdrift-photo').length!==1) throw Error('Lost race print');
				const box=card.getBoundingClientRect(), root=document.querySelector('.agdrift-root').getBoundingClientRect();
				if(box.left<root.left || box.right>root.right || box.bottom>root.bottom || box.top<root.top) throw Error('Photo outside the window');
				const rows=document.querySelector('.agdrift-result-body');
				if(rows.scrollWidth>rows.clientWidth+1) throw Error('Photo stretches the result rows');
			}`)
			if artifacts != "" {
				page.MustElement(".agdrift-root").MustScreenshot(filepath.Join(artifacts, "results.png"))
			}
			page.MustEval(`async()=>{
				await AuraDriftApp.debug.start({mode:'time_trial',trackId:'alpenpass'});
				if(document.querySelector('.agdrift-photo')) throw Error('Last race print survived the restart');
			}`)
			auraDriftCloseWindows(page)
			auraDriftNoErrors(t, page)
		})
	}
}
