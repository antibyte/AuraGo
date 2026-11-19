package ui

import (
	"path/filepath"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopAuraDriftSurfaceEffectsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, tc := range []struct{ renderer, omit string }{{"three", ""}, {"canvas2d", "aura-drift-render3d.js"}} {
		t.Run(tc.renderer, func(t *testing.T) {
			page := auraDriftFixtureWithout(t, tc.omit)
			if tc.renderer == "three" {
				requireWebGL2(t, page)
			}
			if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
				t.Fatal(err)
			}
			auraDriftReopen(page, map[string]interface{}{"quality": "low", "voice": false, "reducedMotion": false}, "")
			page.MustEval(`async renderer => {
				const d = AuraDriftApp.debug, AD = AuraDriftCore, art = AD.surfacePrintCanvas;
				aurora.state.bootstrap.settings['windows.animations'] = true; aurora.applyDesktopSettings();
				AD.surfacePrintCanvas = kind => { const c = art(kind); c.dataset.print = kind; return c; };
				await d.start({mode:'time_trial', trackId:'neon-hafen', seed:42});
				d.freeze(true); d.fastForward(9, {renderEvery:60}); d.setCamera('chase');
				if (AuraDriftApp.stats()[0].renderer.kind !== renderer) throw Error('Unexpected renderer');
				const r = d.race(), v = r.cars[0].v, drawn = new Set();
				if (renderer === 'canvas2d') {
					const c = document.querySelector('.agdrift-world').getContext('2d'), original = c.drawImage;
					c.drawImage = function(img,...args) { if (img.dataset?.print && this.globalAlpha > .05) drawn.add(img.dataset.print); return original.call(this,img,...args); };
				}
				window.driveEffect = kind => {
					drawn.clear();
					if (kind === 'wake') {
						const p = r.track.puddles.find(p => p.permanent);
						d.warp(p.s/r.track.length);
						Object.assign(v,{x:p.x-p.tx*5, y:p.y-p.ty*5, vx:38, vy:0});
					} else {
						const pad = r.features.pads.find(p => !p.superTurbo);
						d.warp(pad.s/r.track.length); Object.assign(v,{x:pad.x,y:pad.y,vx:34,vy:0});
					}
					d.fastForward(kind === 'wake' ? .22 : .43,{renderEvery:2});
					if (kind === 'wake' ? v.planing <= .1 : v.burst <= 0) throw Error('Driving effect was not activated');
					if (renderer === 'canvas2d') {
						if (!drawn.has(kind)) throw Error('Missing painted '+kind);
					} else {
						const {scene,camera,T} = d.three(), name = kind === 'wake' ? 'tyre-wake' : 'boost-reflection';
						const meshes = scene.getObjectsByProperty('name',name).filter(m => m.visible && m.material.opacity > .05 && m.material.map.image.dataset.print === kind);
						const positions = meshes.map(m => m.getWorldPosition(new T.Vector3()).project(camera));
						if (!positions.some(p => Math.max(Math.abs(p.x),Math.abs(p.y),Math.abs(p.z)) <= 1)) throw Error(name+' outside chase camera: '+JSON.stringify(positions));
					}
				};
			}`, tc.renderer)
			dir := auraDriftArtifactDir(t, "surface-effects-"+tc.renderer)
			for _, kind := range []string{"wake", "cyan"} {
				page.MustEval(`kind => driveEffect(kind)`, kind)
				if dir != "" {
					page.MustElement(".agdrift-root").MustScreenshot(filepath.Join(dir, kind+".png"))
				}
			}
			auraDriftNoErrors(t, page)
		})
	}
}
