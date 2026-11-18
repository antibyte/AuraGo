package ui

import (
	"path/filepath"
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopAuraDriftRoadsideBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, tc := range []struct{ renderer, omit string }{{"three", ""}, {"canvas2d", "aura-drift-render3d.js"}} {
		t.Run(tc.renderer, func(t *testing.T) {
			page := auraDriftFixtureWithout(t, tc.omit)
			if tc.renderer == "three" {
				requireWebGL2(t, page)
			}
			reduced := tc.renderer == "canvas2d"
			if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
				t.Fatal(err)
			}
			settings := map[string]interface{}{"quality": "low", "voice": false, "reducedMotion": reduced}
			auraDriftReopen(page, settings, "")
			artifacts := auraDriftArtifactDir(t, "trackside-"+tc.renderer)
			const setup = `async () => {
				const d = AuraDriftApp.debug, AD = AuraDriftCore;
				aurora.state.bootstrap.settings['windows.animations'] = true;
				aurora.applyDesktopSettings();
				const pose = AD.roadsidePose;
				window.sawSalute = false;
				AD.roadsidePose = (part, site, time, out, cheerAt) => {
					pose(part, site, time, out, cheerAt);
					if (cheerAt !== undefined && out.z > pose(part, site, time, {}).z + .2) window.sawSalute = true;
					return out;
				};
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
			auraDriftKeyDown(t, page, input.KeyH)
			auraDriftKeyUp(t, page, input.KeyH)
			page.MustEval(`() => AuraDriftApp.debug.fastForward(.6, {renderEvery:12})`)
			if page.MustEval(`() => window.sawSalute`).Bool() == reduced {
				t.Fatal("Unexpected spectator response to the horn")
			}
			page.MustEval(`() => {
				const AD = AuraDriftCore, d = AuraDriftApp.debug, track = d.race().track;
				const flower = AD.vergePlants(track).find(p => p.bloom === 'daisy' && p.size > 1.35 && AD.sampleTrack(track,p.x,p.y).offset < 0);
				const here = AD.sampleTrack(track, flower.x, flower.y);
				d.warp((here.s - 18) / track.length);
				const car = d.race().cars[0].v, road = AD.pointAt(track, here.s - 18);
				car.x -= road.nx * 4; car.y -= road.ny * 4;
				d.fastForward(.25, {renderEvery:30});
			}`)
			if artifacts != "" {
				page.MustElement(".agdrift-root").MustScreenshot(filepath.Join(artifacts, "meadow.png"))
			}
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

func TestDesktopAuraDriftExtrasBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, tc := range []struct{ renderer, omit string }{{"three", ""}, {"canvas2d", "aura-drift-render3d.js"}} {
		t.Run(tc.renderer, func(t *testing.T) {
			page := auraDriftFixtureWithout(t, tc.omit)
			if tc.renderer == "three" {
				requireWebGL2(t, page)
			}
			auraDriftReopen(page, map[string]interface{}{"quality": "low", "voice": false}, "")
			page.MustEval(`async () => {
				const d = AuraDriftApp.debug, AD = AuraDriftCore;
				await d.start({mode:'quick', trackId:'alpenpass', seed:42});
				d.freeze(true); d.fastForward(4, {renderEvery:240}); d.setCamera('chase');
				window.seenDecals = new Set();
				for (const name of ['paintSuperPad', 'paintGift']) {
					const draw = AD[name];
					AD[name] = function(c) { seenDecals.add(name); return draw(c); };
				}
			}`)
			artifacts := auraDriftArtifactDir(t, "extras-"+tc.renderer)
			for _, feature := range []string{"super_pad", "gift"} {
				page.MustEval(`(kind, renderer) => {
					const d = AuraDriftApp.debug, r = d.race(), pad = kind === 'super_pad';
					const f = pad ? r.features.pads.find(p => p.superTurbo) : r.features.crates.find(c => c.gift);
					d.warp((f.s - 18) / r.track.length); r.cars[0].v.vx = 0; seenDecals.clear();
					d.fastForward(.25, {renderEvery:15});
					if (AuraDriftApp.stats()[0].renderer.kind !== renderer) throw Error('Unexpected renderer');
					if (renderer === 'canvas2d') {
						if (!seenDecals.has(pad ? 'paintSuperPad' : 'paintGift')) throw Error('Missing visible decal: ' + kind);
					} else {
						const {scene, camera, T} = d.three(), mesh = scene.getObjectByName(kind)?.children[0];
						if (!mesh?.isInstancedMesh || !mesh.material.map || !mesh.count) throw Error('Missing textured ' + kind);
						const pos = new T.Vector3(f.x, pad ? .06 : 1.8, f.y).project(camera);
						if (Math.abs(pos.x) > 1 || Math.abs(pos.y) > 1 || Math.abs(pos.z) > 1) throw Error(kind + ' outside the driving view');
					}
				}`, feature, tc.renderer)
				if artifacts != "" {
					page.MustElement(".agdrift-root").MustScreenshot(filepath.Join(artifacts, feature+".png"))
				}
				page.MustEval(`kind => {
					const d = AuraDriftApp.debug, r = d.race(), car = r.cars[0], pad = kind === 'super_pad';
					const f = pad ? r.features.pads.find(p => p.superTurbo) : r.features.crates.find(c => c.gift);
					d.give('popcorn');
					Object.assign(car.v, {x:f.x, y:f.y, vx:20, vy:0, r:0, damage:60, nitro:.1, shield:0, overdrive:0});
					if (pad) car.v.psi = f.psi;
					d.fastForward(.025, {renderEvery:1});
					if (pad && !(car.v.vx > 32 && car.v.burstPower > 3)) throw Error('No super turbo impulse');
					if (!pad && !(car.v.shield > 3 || car.v.damage < 30 || car.v.nitro > .95)) throw Error('No instant bonus');
					if (car.weapon !== 'popcorn' || car.ammo !== 2) throw Error('Pickup displaced the held weapon');
				}`, feature)
			}
			page.MustEval(`() => {
				const d = AuraDriftApp.debug, r = d.race(), car = r.cars[0], rival = r.cars[1];
				for (const other of r.cars.slice(2)) other.v.x += 300;
				Object.assign(rival.v, {x:car.v.x + 10 * Math.cos(car.v.psi), y:car.v.y + 10 * Math.sin(car.v.psi), airborne:false, shield:0, cloak:0});
				d.fire(); d.fastForward(.1, {renderEvery:3});
				if (!rival.v.airborne || car.ammo !== 1) throw Error('Popcorn missed the rival');
				d.give('bubblegum'); d.fire(); d.fastForward(.1, {renderEvery:3});
				if (!r.hazards.some(h => h.type === 'bubblegum')) throw Error('No bubblegum on the road');
			}`)
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
