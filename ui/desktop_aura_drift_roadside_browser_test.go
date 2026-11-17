package ui

import (
	"path/filepath"
	"testing"
)

func TestDesktopAuraDriftRoadsideBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, tc := range []struct{ renderer, omit string }{{"three", ""}, {"canvas2d", "aura-drift-render3d.js"}} {
		t.Run(tc.renderer, func(t *testing.T) {
			page := auraDriftFixtureWithout(t, tc.omit)
			if tc.renderer == "three" {
				requireWebGL2(t, page)
			}
			settings := map[string]interface{}{"quality": "low", "voice": false, "reducedMotion": true}
			auraDriftReopen(page, settings, "")
			artifacts := auraDriftArtifactDir(t, "trackside-"+tc.renderer)
			const setup = `async () => {
				const d = AuraDriftApp.debug, AD = AuraDriftCore;
				const draw = AD.drawRoadside, paint = AD.paintPuddle;
				window.seenShows = new Set(); window.poolFrames = 0;
				AD.drawRoadside = function(c, show, ...rest) { seenShows.add(show.story); return draw(c, show, ...rest); };
				AD.paintPuddle = function(c) { poolFrames++; return paint(c); };
				await d.start({ mode: 'drift_attack', trackId: 'alpenpass', seed: 42 });
				d.fastForward(7, {renderEvery: 300}); d.freeze(true);
			}`
			const inspect = `(kind, renderer) => {
				const AD = AuraDriftCore, d = AuraDriftApp.debug, track = d.race().track;
				if (AuraDriftApp.stats()[0].renderer.kind !== renderer) throw Error('Wrong active renderer');
				const show = AD.roadsideScenes(track, AD.readFinishes()).find(o => o.story === kind);
				const pool = track.puddles.find(p => p.permanent), spot = kind === 'water' ? pool : show.site;
				d.warp((spot.s - (kind === 'water' ? 14 : 26)) / track.length);
				seenShows.clear(); window.poolFrames = 0;
				d.fastForward(.25, {renderEvery: 30});
				if (renderer === 'canvas2d') {
					if (kind === 'water' ? !poolFrames : !seenShows.has(kind)) throw Error('Missing Canvas drawing: ' + kind);
					return;
				}
				const {scene, camera, T} = d.three();
				const mesh = scene.getObjectByName(kind === 'water' ? 'standing-water' : 'roadside-' + kind);
				if (!mesh || !mesh.visible) throw Error('Missing live 3D scene: ' + kind);
				if (kind === 'water') {
					if (mesh.position.x !== pool.x || mesh.position.z !== pool.y || !mesh.material.map) throw Error('Water footprint mismatch');
				} else {
					if (mesh.children.length !== show.parts.length) throw Error('Missing members of the cast');
					for (const actor of mesh.children) if (!actor.getObjectByProperty('isMesh', true)) throw Error('Empty actor');
				}
				const p = new T.Box3().setFromObject(mesh).getCenter(new T.Vector3()).project(camera);
				if (Math.abs(p.x) > 1 || Math.abs(p.y) > 1 || p.z < -1 || p.z > 1) throw Error(kind + ' outside the driving view: ' + p.toArray());
			}`
			capture := func(kind string) {
				t.Helper()
				page.MustEval(inspect, kind, tc.renderer)
				if artifacts != "" {
					page.MustElement(".agdrift-root").MustScreenshot(filepath.Join(artifacts, kind+".png"))
				}
			}
			page.MustEval(setup)
			capture("podium")
			capture("water")
			page.MustEval(`async () => {
				const d = AuraDriftApp.debug;
				const start = () => d.start({mode:'drift_attack', trackId:'alpenpass', seed:42});
				await start();
				if (AuraDriftCore.readFinishes() !== 0) throw Error('Restart counted as a finish');
				for (let n = 1; n <= 2; n++) {
					d.fastForward(100, {renderEvery:100000});
					if (!d.view().results || AuraDriftCore.readFinishes() !== n) throw Error('Race result count: ' + n);
					if (n === 1) await start();
				}
			}`)
			auraDriftReopen(page, settings, "")
			page.MustEval(setup)
			capture("taxi")
			auraDriftNoErrors(t, page)
		})
	}
}

func TestDesktopAuraDriftHandlingReplayBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := auraDriftFixtureWithout(t, "aura-drift-render3d.js")
	settings := map[string]interface{}{"showGhost": true, "voice": false}
	auraDriftReopen(page, settings, "")
	page.MustEval(`() => AuraDriftCore.saveBests({alpenpass:{time:1, carId:'tundra', tape:[0,1]}}, false)`)
	auraDriftReopen(page, settings, "")
	page.MustEval(`async () => {
		const d = AuraDriftApp.debug, AD = AuraDriftCore;
		const start = () => d.start({mode:'time_trial', trackId:'alpenpass', carId:'vektor'});
		await start();
		if (d.race().ghost) throw Error('Legacy input tape played with new handling');
		d.freeze(true); d.autopilot(true, {skill:1, ramps:false});
		d.fastForward(340, {renderEvery:100000});
		const best = AD.loadBests(true).alpenpass;
		if (best.time !== 1 || best.carId !== 'tundra' || best.handling !== AD.GHOST_HANDLING ||
			best.ghostCarId !== 'vektor' || !(best.ghostTime > 1) || !AD.decodeTape(best.tape).length)
			throw Error('Historical record or new ghost missing');
		d.autopilot(false); await start();
		if (!d.race().ghost || d.race().ghost.v.car.id !== 'vektor') throw Error('Compatible ghost did not reload');
	}`)
	auraDriftNoErrors(t, page)
}
