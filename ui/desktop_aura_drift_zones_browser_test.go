package ui

import (
	"path/filepath"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopAuraDriftZonePaintBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, tc := range []struct {
		renderer, omit string
		width          int
	}{{"three", "", 1280}, {"canvas2d", "aura-drift-render3d.js", 560}} {
		t.Run(tc.renderer, func(t *testing.T) {
			page := auraDriftFixtureWithout(t, tc.omit)
			if tc.renderer == "three" {
				requireWebGL2(t, page)
			}
			page.MustSetViewport(tc.width, 720, 1, false)
			if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
				t.Fatal(err)
			}
			auraDriftReopen(page, map[string]interface{}{"quality": "low", "voice": false, "reducedMotion": tc.renderer == "canvas2d"}, "")
			page.MustEval(`async renderer => {
				const d = AuraDriftApp.debug, AD = AuraDriftCore;
				aurora.state.bootstrap.settings['windows.animations'] = true;
				aurora.applyDesktopSettings();
				await d.start({mode:'drift_attack', trackId:'alpenpass', seed:42});
				d.freeze(true); d.fastForward(6, {renderEvery:120}); d.setCamera('chase');
				if (AuraDriftApp.stats()[0].renderer.kind !== renderer) throw Error('Wrong renderer');
				const track = d.race().track, zone = track.driftZones.find(z => z.to - z.from > 40 && z.from > 120);
				const hud = document.querySelector('.agdrift-hud').getContext('2d'), text = hud.fillText;
				const label = '×' + new Intl.NumberFormat(document.documentElement.lang).format(zone.multiplier);
				window.zoneLabel = false;
				hud.fillText = function(s,...args) { if (s === label) window.zoneLabel = true; return text.call(this,s,...args); };
				const world = document.querySelector('.agdrift-world').getContext('2d'), fill = world.fill;
				window.roadInk = new Set();
				world.fill = function(...args) { if (args[0] instanceof Path2D) roadInk.add(this.fillStyle); return fill.apply(this,args); };
				d.warp((zone.from - 65) / track.length);
				d.fastForward(.18, {renderEvery:4});
				if (zoneLabel) throw Error('Bonus shown for a straight approach');
				if (renderer === 'canvas2d') {
					if (!roadInk.has('#ac78e6') || !roadInk.has('#eee2ff')) throw Error('Missing cached road paint');
				} else {
					const {scene,camera,T} = d.three(), geo = scene.getObjectByName('kerbs').geometry;
					const pos = geo.getAttribute('position'), col = geo.getAttribute('color'), tint = AD.world.hexLin('#ac78e6');
					let visible = false;
					for (let i=0;i<pos.count;i++) {
						if (Math.abs(col.getX(i)-tint[0]) > .001 || Math.abs(col.getZ(i)-tint[2]) > .001) continue;
						const p = new T.Vector3().fromBufferAttribute(pos,i).project(camera);
						if (Math.abs(p.x)<1 && Math.abs(p.y)<1 && Math.abs(p.z)<1) {visible=true;break;}
					}
					if (!visible) throw Error('No approach paint in the driving view');
				}
			}`, tc.renderer)
			dir := auraDriftArtifactDir(t, "drift-zone-"+tc.renderer)
			if dir != "" {
				page.MustElement(".agdrift-root").MustScreenshot(filepath.Join(dir, "approach.png"))
			}
			page.MustEval(`() => {
				const d = AuraDriftApp.debug, race = d.race(), car = race.cars[0];
				const zone = race.track.driftZones.find(z => z.to - z.from > 40 && z.from > 120);
				d.warp((zone.from + 12) / race.track.length);
				Object.assign(car.v,{vx:32,vy:14});
				d.fastForward(.1,{renderEvery:2});
				if (!zoneLabel || AuraDriftCore.earnedDriftZone(car) !== zone.multiplier || car.style.raw <= 0) throw Error('Missing scored drift or multiplier');
			}`)
			if dir != "" {
				page.MustElement(".agdrift-root").MustScreenshot(filepath.Join(dir, "slide.png"))
			}
			page.MustEval(`() => {
				const d = AuraDriftApp.debug;
				d.warp(.05); window.zoneLabel = false; d.fastForward(.1,{renderEvery:2});
				if (zoneLabel) throw Error('Bonus lingered after leaving the corner');
			}`)
			auraDriftNoErrors(t, page)
		})
	}
}
