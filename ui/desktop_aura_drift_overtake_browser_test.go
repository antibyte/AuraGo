package ui

import (
	"path/filepath"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopAuraDriftSlingshotBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, tc := range []struct {
		renderer, omit string
		width          int
		reduced        bool
	}{{"three", "", 1280, false}, {"canvas2d", "aura-drift-render3d.js", 560, true}} {
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
			page.MustEval(`async renderer => {
				aurora.state.bootstrap.settings['windows.animations'] = true;
				aurora.applyDesktopSettings();
				const d = AuraDriftApp.debug;
				await d.start({mode:'quick', trackId:'alpenpass', seed:42});
				d.freeze(true); d.fastForward(7, {renderEvery:240}); d.warp(.10);
				if (AuraDriftApp.stats()[0].renderer.kind !== renderer) throw Error('Wrong renderer');
				const [player, rival] = d.race().cars, v = player.v;
				v.vx = 38; v.draftTime = .45;
				Object.assign(rival.v, {x:v.x + 11*Math.cos(v.psi), y:v.y + 11*Math.sin(v.psi), psi:v.psi, vx:38, vy:0, r:0});
				const ctx = document.querySelector('.agdrift-hud').getContext('2d'), fill = ctx.fill;
				window.draftPaint = new Set();
				ctx.fill = function(...args) { if(this.globalAlpha > .02) draftPaint.add(this.fillStyle); return fill.apply(this,args); };
			}`, tc.renderer)
			dir := auraDriftArtifactDir(t, "slingshot-"+tc.renderer)
			for _, phase := range []string{"charging", "ready", "release", "clear"} {
				page.MustEval(`(phase,reduced) => {
					const d = AuraDriftApp.debug, [player,rival] = d.race().cars, v = player.v;
					if (phase === 'ready') v.draftTime = 1.19;
					if (phase === 'release') { rival.v.x -= 5.5*Math.sin(v.psi); rival.v.y += 5.5*Math.cos(v.psi); }
					if (phase === 'clear') { rival.v.out = true; d.fastForward(.65,{renderEvery:60}); }
					draftPaint.clear(); d.fastForward(.06,{renderEvery:3});
					const gauge = phase === 'charging' ? '#73d8e7' : '#ffcd60';
					if (draftPaint.has(gauge) !== (phase === 'ready' || phase === 'charging')) throw Error('Wrong gauge: '+phase);
					if (phase === 'release' && !(player.tricks.counts.slingshot === 1 && v.burst > 0)) throw Error('Missing boost');
					if (draftPaint.has('#b9eafa') !== (!reduced && ['charging','ready'].includes(phase)) ||
						draftPaint.has('#ffe2a2') !== (!reduced && phase === 'release')) {
						throw Error('Wrong wind: '+JSON.stringify({phase,reduced,time:d.race().time,draft:v.draftTime,wake:draftPaint.has('#b9eafa'),release:draftPaint.has('#ffe2a2')}));
					}
				}`, phase, tc.reduced)
				if dir != "" && (phase == "ready" || phase == "release") {
					page.MustElement(".agdrift-root").MustScreenshot(filepath.Join(dir, phase+".png"))
				}
			}
			auraDriftNoErrors(t, page)
		})
	}
}
