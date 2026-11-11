// System World window interiors: lit-share curve, levels, material flags, shader injection
// against the pinned Three.js standard material, uniform plumbing, NaN guards, and routing of
// the real kit v3 panes. Needs no GPU.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import crypto from 'node:crypto';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
import {WINDOW_MATERIAL, WINDOW_SHADER, WINDOW_TONES, WINDOW_ENV, WINDOW_LIFE_ATLAS, WINDOW_LIFE_LOOK, windowLighting, windowLevel, createWindows, addPaneAxes} from '../ui/js/desktop/apps/sysworld-windows.js';
import {createSurfaces} from '../ui/js/desktop/apps/sysworld-surfaces.js';

const near = (a, b, e = .005) => Math.abs(a - b) <= e;

// 1. Lit share by time of day (daylight(): day 1/0, evening 0/1, night 0/0).
const day = windowLighting(1, 0), evening = windowLighting(0, 1), night = windowLighting(0, 0);
assert.ok(near(day.share, .12) && near(evening.share, .6) && near(night.share, .5), 'lit share ' + JSON.stringify([day.share, evening.share, night.share]));
assert.ok(windowLighting(1, 0, 1).share > day.share, 'overcast days light more rooms');
for (let d = 0, last = Infinity; d <= 1.0001; d += .05) { const s = windowLighting(d, 0).share; assert.ok(s <= last + 1e-9, 'share must fall as daylight grows'); last = s; }
assert.ok(day.lobby === 0 && night.lobby === 1 && evening.lobby === 1, 'lobbies are lit from dusk to dawn');
assert.ok(day.ambient > night.ambient * 5 && night.ambient < .02, 'unlit rooms: dim by day, near black at night');
assert.ok(night.glow > day.glow, 'lit rooms glow brighter at night');
for (const v of Object.values(windowLighting(NaN, -3, 7))) assert.ok(Number.isFinite(v), 'lighting stays finite on bad input');

// 2. Levels follow the scene tiers.
assert.deepEqual(['low', 'medium', 'high', 'ultra', 'bogus'].map(windowLevel), [0, 1, 2, 2, 2]);

// 3. One shared material: glossy, reflective, vertex colours on, opaque.
const windows = createWindows(), m = windows.material;
assert.ok(m.isMeshStandardMaterial && m.name === WINDOW_MATERIAL && m.vertexColors === true && m.transparent === false, 'material flags');
assert.ok(near(m.roughness, .08) && m.metalness === 0 && m.envMap === null, 'glass reflects the scene environment');

// 4. Injection: every hook matched once per tier, defines first, uniforms shared by reference.
const compile = tier => {
  windows.setTier(tier);
  const shader = {uniforms: {}, vertexShader: THREE.ShaderLib.standard.vertexShader, fragmentShader: THREE.ShaderLib.standard.fragmentShader};
  m.onBeforeCompile(shader);
  return shader;
};
const keys = new Set();
for (const [tier, level] of [['low', 0], ['medium', 1], ['high', 2], ['ultra', 2]]) {
  const s = compile(tier);
  assert.ok(s.vertexShader.startsWith('#define WIN_LEVEL ' + level) && s.fragmentShader.startsWith('#define WIN_LEVEL ' + level), tier + ': level define');
  for (const part of [WINDOW_SHADER.vertexPars, WINDOW_SHADER.vertexMain]) assert.equal(s.vertexShader.split(part).length, 2, tier + ': vertex hook');
  for (const part of [WINDOW_SHADER.fragmentPars, WINDOW_SHADER.fragmentMain, WINDOW_SHADER.roughness, WINDOW_SHADER.emissive])
    assert.equal(s.fragmentShader.split(part).length, 2, tier + ': fragment hook');
  assert.ok(s.fragmentShader.indexOf(WINDOW_SHADER.fragmentMain) > s.fragmentShader.indexOf('#include <color_fragment>'), 'interior after the colour chunk');
  assert.ok(s.fragmentShader.indexOf(WINDOW_SHADER.emissive) > s.fragmentShader.indexOf('#include <emissivemap_fragment>'), 'glow after the emissive chunk');
  for (const name of ['winTime', 'winState', 'winSky', 'winLife', 'winLifeOn']) assert.equal(s.uniforms[name], windows.uniforms[name], name + ' shared by reference');
  keys.add(m.customProgramCacheKey());
}
assert.equal(keys.size, 3, 'one program per level');
assert.ok(WINDOW_SHADER.vertexMain.includes('#ifdef USE_INSTANCING') && WINDOW_SHADER.vertexMain.includes('modelMatrix*instanceMatrix'), 'instancing-safe world position and seed');

// 5. Tier switches recompile only when the level changes.
windows.setTier('high'); const version = m.version;
windows.setTier('ultra'); assert.equal(m.version, version, 'high and ultra share a program');
windows.setTier('medium'); assert.ok(m.version > version, 'a level change recompiles');

// 6. Lighting and time plumbing; reduced motion freezes the life layer.
windows.setLighting(0, 1, 0, new THREE.Color(0x2a2638));
assert.ok(near(windows.uniforms.winState.value.x, .6) && windows.uniforms.winState.value.z === 1, 'evening state');
assert.equal(windows.uniforms.winSky.value.getHex(), 0x2a2638, 'sky sheen follows the fog colour');
// Own environment: without a texture envMap stays null; with one the glass uses it, with its own intensity by day and night.
assert.equal(m.envMap, null, 'no texture, no envMap');
windows.setLighting(0, 1, 0); assert.ok(near(m.envMapIntensity, WINDOW_ENV.night), 'night env intensity');
windows.setLighting(1, 0, 0); assert.ok(near(m.envMapIntensity, WINDOW_ENV.day), 'day env intensity');
const stubTexture = new THREE.Texture(); windows.setEnvironment(stubTexture); assert.equal(m.envMap, stubTexture, 'glass gets the scene texture');
windows.setLighting(0, 1, 0, new THREE.Color(0x2a2638)); assert.ok(near(m.envMapIntensity, WINDOW_ENV.night));
windows.setEnvironment(null); assert.equal(m.envMap, null, 'texture removed');
windows.setLighting(0, 1, 0, new THREE.Color(0x2a2638));
const t0 = windows.uniforms.winTime.value;
windows.update(.05, true); assert.ok(near(windows.uniforms.winTime.value - t0, .05, 1e-9), 'time advances');
windows.update(5, true); assert.ok(near(windows.uniforms.winTime.value - t0, .15, 1e-9), 'dt is clamped to 0.1 s');
windows.update(.05, false); windows.setReducedMotion(true); windows.update(.05, true); windows.update(NaN, true);
assert.ok(near(windows.uniforms.winTime.value - t0, .15, 1e-9), 'no motion: time frozen');
windows.setReducedMotion(false);
assert.deepEqual(Object.keys(windows.stats()).sort(), ['glow', 'level', 'life', 'lifeFailures', 'lobby', 'meshes', 'reduced', 'share', 'tier', 'time']);

// 7. NaN guards: no normalize() at all, every pow() base clamped, every division guarded,
// balanced preprocessor blocks and parentheses.
const glsl = Object.values(WINDOW_SHADER).join('\n'), code = glsl.replace(/\/\/[^\n]*/g, '');
assert.ok(!code.includes('normalize('), 'use x/max(length(x),eps) instead of normalize()');
for (const hit of code.matchAll(/pow\(/g)) assert.ok(code.startsWith('pow(clamp(', hit.index), 'unclamped pow() base');
for (const [, token] of code.matchAll(/\/\s*([A-Za-z_][\w.]*|\()/g))
  assert.ok(['max', 'winDiv', 'winSize', 'winS', 'winD.z', '('].includes(token), 'unguarded division by ' + token);
for (const [name, part] of Object.entries(WINDOW_SHADER)) {
  const strip = part.replace(/\/\/[^\n]*/g, '');
  assert.equal((strip.match(/^#if/gm) || []).length, (strip.match(/^#endif/gm) || []).length, name + ': #if/#endif');
  assert.equal((strip.match(/\(/g) || []).length, (strip.match(/\)/g) || []).length, name + ': parentheses');
}
assert.ok(code.includes('max(-dot(winV,winN),0.02)') && code.includes('max(abs(winD),vec3(1e-4))') && code.includes('max(winLu,1e-4)'), 'guards present');
assert.ok(!/dFdx\(vWinPos\)|dFdx\(vWinRoom\)|winTrust/.test(code), 'frame and size no longer come from screen-space derivatives');

// Phase 6: at least five wall tones, cumulative shares ending at 1, tones distinct and spread in brightness; generated once
// into the shader; darker furniture; daylight falls to 12 % at the back of a room.
assert.ok(WINDOW_TONES.length>=5,'five or more wall tones');
WINDOW_TONES.forEach((t,i)=>{assert.ok(t.upTo>(i?WINDOW_TONES[i-1].upTo:0)&&t.upTo<=1,'ascending shares');assert.equal(t.color.length,3);});
assert.equal(WINDOW_TONES.at(-1).upTo,1,'the last tone closes the range');
for(let i=0;i<WINDOW_TONES.length;i++)for(let j=i+1;j<WINDOW_TONES.length;j++)assert.ok(Math.hypot(...WINDOW_TONES[i].color.map((c,k)=>c-WINDOW_TONES[j].color[k]))>=.1,'tones '+i+' and '+j+' too close');
const lum=c=>.2126*c[0]+.7152*c[1]+.0722*c[2],l=WINDOW_TONES.map(t=>lum(t.color));assert.ok(Math.max(...l)-Math.min(...l)>=.25,'tones spread in brightness');
assert.equal(WINDOW_SHADER.fragmentPars.split('vec3 winTone(float h)').length,2);assert.equal(WINDOW_SHADER.fragmentMain.split('winTone(winH3)').length,2);
assert.ok(WINDOW_SHADER.fragmentPars.includes('fill*(0.12+0.88*(1.0-z)*(1.0-z))'),'daylight falls off with depth');
assert.ok(WINDOW_SHADER.fragmentMain.includes('vec3(0.07,0.065,0.06)'),'darker furniture');
// Class B: lit rooms are capped before the Fresnel term so they cannot clip on their own.
assert.ok(compile('high').fragmentShader.includes('#define WIN_CAP')&&WINDOW_SHADER.fragmentMain.includes('min(winRoomCol,vec3(WIN_CAP))'),'room radiance cap');

// Resident sprites: the exported atlas layout equals the manifest, the PNG equals its hash, the life layer exists only at WIN_LEVEL>1.
const manifest = JSON.parse(await fs.readFile('ui/3d/system-world/textures/v1/window-life.json', 'utf8'));
assert.equal(WINDOW_LIFE_ATLAS.columns, manifest.cells.columns); assert.equal(WINDOW_LIFE_ATLAS.rows, manifest.cells.rows);
assert.deepEqual(WINDOW_LIFE_ATLAS.pixels, manifest.cells.pixels); assert.deepEqual(WINDOW_LIFE_ATLAS.metres, manifest.cells.metres);
assert.equal(WINDOW_LIFE_ATLAS.feet, manifest.cells.feet); assert.equal(WINDOW_LIFE_ATLAS.stride, manifest.stride);
assert.deepEqual(JSON.parse(JSON.stringify(WINDOW_LIFE_ATLAS.sets)), manifest.sets, 'atlas sets match the manifest');
assert.ok(Object.isFrozen(WINDOW_LIFE_ATLAS) && Object.isFrozen(WINDOW_LIFE_ATLAS.sets.walk) && Object.isFrozen(WINDOW_LIFE_LOOK));
assert.equal(manifest.size[0], manifest.cells.columns * manifest.cells.pixels[0]); assert.equal(manifest.size[1], manifest.cells.rows * manifest.cells.pixels[1]);
const png = await fs.readFile('ui/3d/system-world/textures/v1/' + manifest.file);
assert.equal(png.length, manifest.bytes, 'atlas bytes'); assert.equal(crypto.createHash('sha256').update(png).digest('hex'), manifest.sha256, 'atlas sha256');
assert.ok(!glsl.includes('winRobot') && !glsl.includes('winVisor') && !glsl.includes('winBots'), 'the flat robot shapes are gone');
{
  const pars = WINDOW_SHADER.fragmentPars, main = WINDOW_SHADER.fragmentMain;
  const lifeStart = pars.indexOf('#if WIN_LEVEL>1\nuniform sampler2D winLife;');
  assert.ok(lifeStart > 0 && pars.indexOf('textureGrad(') > lifeStart, 'life layer in the pars');
  const lifeEnd = pars.indexOf('\n#endif', lifeStart);
  assert.ok(lifeEnd > pars.indexOf('winFigure(') && !/winLife|winSprite|winFigure|winPose/.test(pars.slice(0, lifeStart) + pars.slice(lifeEnd)), 'sampler and sprite code only inside WIN_LEVEL>1');
  const at = main.indexOf('winFigure('), open = main.lastIndexOf('#if WIN_LEVEL>1', at), close = main.indexOf('#endif', at);
  assert.ok(at > 0 && open > 0 && close > at && main.slice(open, at).indexOf('#endif') < 0, 'figures drawn only inside #if WIN_LEVEL>1');
  assert.ok(main.includes('mod(winCell.x+2.0*winCell.y+winOff,4.0)') && main.includes('mod(winCell.x+winOff,3.0)'), 'roles follow the cell, no worker in lobbies');
  assert.ok(main.includes('+0.6180339*winCell.x+0.7548777*winCell.y)'), 'the timing hash is decorrelated between neighbouring cells (winHash collides in 32-bit floats)');
  assert.ok(main.includes('winHi=mix(winS.x-0.45,max(0.5*winS.x-0.575,winLo),winTwo)') && main.includes('winPo2.x=winS.x-winPo2.x'), 'lobby pair keeps to its own halves, 1.15 m between centres');
  assert.ok(main.includes('winPu=vWinAxU') && main.includes('winS=mix(winS,vec2(winLu,winLv),winOk)'), 'frame and room size from the exact pane axes');
  assert.ok(main.includes('winTwo=winLobby*step(2.05,winS.x)') && main.includes('if(winTwo>0.5)'), 'lobbies narrower than 2.05 m show one figure');
  assert.ok(main.includes('winPres=winAlive*step(winH2,mix(0.4,0.85,winLobby))*winLifeOn'), 'presence rule unchanged, gated by winLifeOn');
}
for (const tier of ['low', 'medium', 'high']) { windows.setTier(tier); assert.equal(m.customProgramCacheKey(), 'sw-windows|' + windowLevel(tier), 'cache key stays per level'); }
windows.setTier('high');
assert.equal(windows.uniforms.winLifeOn.value, 0, 'no atlas: no figures'); assert.ok(windows.uniforms.winLife.value?.isTexture, 'placeholder texture keeps the program valid');
windows.update(.5, true); assert.equal(windows.uniforms.winLifeOn.value, 0, 'winLifeOn stays 0 without a texture');
{
  const closed = []; let disposedTex = 0;
  const mk = () => { const t = new THREE.Texture(); t.image = {close: () => closed.push(1)}; t.addEventListener('dispose', () => disposedTex++); return t; };
  const a = mk(), b = mk(); const own = createWindows();
  assert.equal(own.setLife(null), false); assert.equal(own.stats().lifeFailures, 1); assert.equal(own.stats().life, 0);
  assert.equal(own.setLife(a), true); assert.equal(own.stats().life, 1); assert.equal(own.uniforms.winLife.value, a); assert.equal(own.uniforms.winLifeOn.value, 0);
  own.update(.5, true); assert.ok(near(own.uniforms.winLifeOn.value, .1, 1e-9), 'eases in at a clamped step');
  for (let i = 0; i < 12; i++) own.update(.1, true); assert.equal(own.uniforms.winLifeOn.value, 1, 'fully on after about a second');
  const frozen = createWindows(); frozen.setReducedMotion(true); frozen.setLife(mk()); frozen.update(.05, true);
  assert.equal(frozen.uniforms.winLifeOn.value, 1, 'reduced motion shows the figures at once'); frozen.dispose();
  const still = createWindows(); still.setLife(mk()); still.update(0, false); assert.equal(still.uniforms.winLifeOn.value, 1, 'frozen time: figures at once'); still.dispose();
  disposedTex = 0; closed.length = 0;
  own.setLife(b); assert.ok(disposedTex === 1 && closed.length === 1, 'replacing releases the old texture and bitmap');
  own.dispose(); assert.ok(disposedTex === 2 && closed.length === 2, 'dispose releases the texture and bitmap');
  const late = mk(); disposedTex = 0; assert.equal(own.setLife(late), false); assert.equal(disposedTex, 1, 'a late texture is released at once');
}

// Exact pane axes: a 3.8 m x 3 m pane (u 0..1, uv.y 1..0 for rows upwards) gives axU (3.8,0,0) and axV (0,3,0), once per geometry.
{
  const g = new THREE.BufferGeometry();
  g.setAttribute('position', new THREE.Float32BufferAttribute([0, 0, 0, 3.8, 0, 0, 3.8, 3, 0, 0, 3, 0], 3));
  g.setAttribute('uv', new THREE.Float32BufferAttribute([0, 1, 1, 1, 1, 0, 0, 0], 2)); g.setIndex([0, 1, 2, 0, 2, 3]);
  assert.equal(addPaneAxes(g), true); assert.equal(addPaneAxes(g), false, 'once per geometry');
  const u = g.getAttribute('winAxU'), v = g.getAttribute('winAxV');
  for (let i = 0; i < 4; i++) { assert.ok(near(u.getX(i), 3.8, 1e-5) && near(u.getY(i), 0) && near(v.getY(i), 3, 1e-5) && near(v.getX(i), 0), 'axes of vertex ' + i); }
  assert.equal(addPaneAxes(new THREE.BufferGeometry()), false, 'no attributes, nothing to do');
  const shared = new THREE.BufferGeometry(); shared.copy(g); shared.deleteAttribute('winAxU'); shared.deleteAttribute('winAxV');
  const a = new THREE.Mesh(g, new THREE.MeshStandardMaterial({name: WINDOW_MATERIAL})), b = new THREE.Mesh(g, new THREE.MeshStandardMaterial({name: WINDOW_MATERIAL}));
  windows.adopt(a); const attr = g.getAttribute('winAxU'); windows.adopt(b); assert.equal(g.getAttribute('winAxU'), attr, 'a shared geometry gets its axes exactly once');
}

// 8. Routing: adopt() swaps city.window for the shared material; surfaces never patch it.
const original = new THREE.MeshStandardMaterial({name: WINDOW_MATERIAL, vertexColors: true});
let disposals = 0; original.addEventListener('dispose', () => disposals++);
const pane = new THREE.Mesh(new THREE.PlaneGeometry(), original);
assert.equal(windows.adopt(pane), true); assert.equal(pane.material, m); assert.equal(disposals, 1);
assert.equal(windows.adopt(pane), false, 'adopting twice is a no-op');
const other = new THREE.Mesh(new THREE.PlaneGeometry(), new THREE.MeshStandardMaterial({name: 'city.graphite'}));
assert.equal(windows.adopt(other), false); assert.notEqual(other.material, m);
const multi = new THREE.Mesh(new THREE.PlaneGeometry(), [new THREE.MeshStandardMaterial({name: 'city.titanium'}), new THREE.MeshStandardMaterial({name: WINDOW_MATERIAL})]);
assert.equal(windows.adopt(multi), true); assert.equal(multi.material[1], m); assert.notEqual(multi.material[0], m);
assert.ok(windows.owns(m) && !windows.owns(original));
const surfaces = createSurfaces({capabilities: {getMaxAnisotropy: () => 8}}, {});
const hook = m.onBeforeCompile; surfaces.apply(m); surfaces.prepare(pane);
assert.equal(m.onBeforeCompile, hook, 'sysworld-surfaces.js must leave city.window alone');
assert.equal(pane.geometry.getAttribute('swSeed'), undefined, 'no pane seeds on window panes');
surfaces.dispose();

// 9. Real kit v3 buildings: every pane at every LOD routes and carries room coordinates and AO.
await MeshoptDecoder.ready;
const loader = new GLTFLoader().setMeshoptDecoder(MeshoptDecoder);
const kit = JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json', 'utf8'));
let routed = 0;
for (const id of ['data-tower-a', 'agent-spire', 'knowledge-atrium', 'operations-beacon']) {
  const entry = kit.assets.find(a => a.id === id); assert.ok(entry, 'kit v3 lacks ' + id);
  for (const lod of entry.lods) {
    const data = await fs.readFile('ui/3d/system-world/v3/' + lod.file);
    const gltf = await loader.parseAsync(data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength), '');
    let panes = 0;
    gltf.scene.traverse(n => {
      if (!n.isMesh || !windows.adopt(n)) return;
      panes++;
      assert.ok(n.geometry.getAttribute('uv') && n.geometry.getAttribute('color'), lod.file + ': pane lacks uv or colour');
      const au = n.geometry.getAttribute('winAxU'), av = n.geometry.getAttribute('winAxV');
      assert.ok(au && av, lod.file + ': pane lacks its room axes');
      for (let i = 0; i < au.count; i++) { const lu = Math.hypot(au.getX(i), au.getY(i), au.getZ(i)), lv = Math.hypot(av.getX(i), av.getY(i), av.getZ(i)); assert.ok(Number.isFinite(lu + lv) && (lu > 0) === (lv > 0), lod.file + ': degenerate axes'); }
    });
    gltf.scene.traverse(n => { if (n.isMesh) assert.ok(n.material === m || n.material.name !== WINDOW_MATERIAL, lod.file + ': unrouted pane'); });
    assert.ok(panes > 0, lod.file + ': no window panes'); routed += panes;
  }
}
assert.equal(windows.stats().meshes, routed + 4, 'stats count every routed mesh');
windows.dispose();
console.log(`System World windows: share day/evening/night ${day.share.toFixed(2)}/${evening.share.toFixed(2)}/${night.share.toFixed(2)}, 3 programs, guards verified, ${routed} kit v3 pane meshes routed.`);
