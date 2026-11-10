#!/usr/bin/env node
/*
 * System World visual survey harness.
 *
 * Renders the real city (ui/js/desktop/apps/sysworld-scene.js, three.js) in headless Chrome with a
 * freely placed camera and writes one PNG per scene, so models/ground/robots can be judged before
 * and after a change.
 *
 *   node scripts/system-world-survey.mjs --out <dir> [--scenes <file>] [--only a,b] [--tier low|medium|high|ultra]
 *        [--width 1600] [--height 900] [--timeout 300] [--keep-open] [--timing]
 *   --timing: after each scene's picture, runs 100 frames in one synchronous loop outside requestAnimationFrame (no yielding, so vsync
 *        and presentation cannot pace it): city.update(1/60) + near sync + render, then gl.finish() so GPU work counts, one timing per
 *        iteration. The first 10 are warm-up and dropped; frameMs {median, p95} (milliseconds, 90 samples) goes into that scene's report.json entry.
 *   --without-furniture: measuring aid for A/B runs. Patches the in-memory copy of sysworld-scene.js so kitPlacements holds only the
 *        street pieces (no furnishing() pieces); report.json gets withoutFurniture: true. Nothing on disk changes.
 *
 * How it works: a DEBUG bundle of sysworld-scene.js is built in memory with esbuild. An onLoad plugin
 * patches the source text (every patch asserted to match exactly once; nothing under ui/ is modified):
 *   - sysworld-scene.js: exposes globalThis.__sw = {THREE, camera, controls, scene, renderer, composer,
 *     experience, traffic, weather, atmosphere, surfaces, windows, sun, bloom, drones, lobbies, staticCity, nearHeight, harbour, gulls, particles, cancelFlight(), mode()}
 *   - sysworld-society.js: adds `actors` to the society object, so residents are reached through
 *     __sw.experience.society.actors (each has .id "resident-N", .body {x,y,z,heading,enabled}, .node).
 * ui/ is served as the web root; the harness page drives the city with its own animation loop.
 *
 * Near furniture: sysworld-scene.js hides meshes with userData.near above NEAR_HEIGHT by ITS camera logic. The harness
 * poses its own camera, so it sets that visibility itself from the survey camera's height (syncNear: before every
 * capture and after every frame of the city's update) and reports the applied state as stats.nearHidden.
 *
 * Scene format (JSON array; see system-world-survey.scenes.json):
 *   name           file name (sanitised to [a-z0-9-])
 *   position,target  free camera in world metres, Y up (the city is put in orbit mode, OrbitControls
 *                  limits are relaxed, damping off; the camera is re-applied right before capture)
 *   fov            vertical field of view, default 43
 *   environment    time of day, passed as setEnvironment({time}): "day" | "evening" | "night" | "local"
 *                  (local = real clock). Default "day".
 *   weather        passed as setEnvironment({weather}): "clear" | "rain" | "fog". Default "clear".
 *   frames, waitMs animation frames and extra milliseconds to let LOD/streaming/lighting settle
 *                  (defaults 40 / 400)
 *   followResident N  camera relative to resident-N (society actor N), instead of position/target:
 *                  camera = resident + offset [dx,dy,dz] (world axes; with "offsetRelative": true the
 *                  x/z offset is rotated by the resident's heading), looking at resident + lookHeight
 *                  (default 0.9). Needs tier medium+ for N>=3 and high+ for N>=10 (resident limit).
 *   sequence N, intervalMs  capture N frames named <name>-00, -01 ... every intervalMs (default 350)
 *   eval           JS source run in the page before the final camera is applied; has `city`, `__sw`,
 *                  `scene` (the scene object). If the scene has no position/followResident the camera
 *                  is left as eval set it.
 */
import { build } from 'esbuild';
import fs from 'node:fs/promises';
import { existsSync } from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { spawn, spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const uiRoot = path.join(root, 'ui');

// ---------- CLI ----------
function parseArgs(argv) {
  const o = { out: 'reports/system-world-survey/latest', scenes: path.join(here, 'system-world-survey.scenes.json'),
    only: null, tier: 'high', width: 1600, height: 900, timeout: 300, keepOpen: false, timing: false, withoutFurniture: false };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i], next = () => { if (i + 1 >= argv.length) throw Error('Missing value for ' + a); return argv[++i]; };
    if (a === '--out') o.out = next();
    else if (a === '--scenes') o.scenes = next();
    else if (a === '--only') o.only = next().split(',').map(s => s.trim()).filter(Boolean);
    else if (a === '--tier') o.tier = next();
    else if (a === '--width') o.width = Number(next());
    else if (a === '--height') o.height = Number(next());
    else if (a === '--timeout') o.timeout = Number(next());
    else if (a === '--keep-open') o.keepOpen = true;
    else if (a === '--timing') o.timing = true;
    else if (a === '--without-furniture') o.withoutFurniture = true;
    else throw Error('Unknown argument ' + a);
  }
  if (!['low', 'medium', 'high', 'ultra'].includes(o.tier)) throw Error('--tier must be low|medium|high|ultra');
  for (const k of ['width', 'height', 'timeout']) if (!Number.isFinite(o[k]) || o[k] <= 0) throw Error('--' + k + ' must be a positive number');
  return o;
}

// ---------- debug bundle ----------
function replaceOnce(text, find, replacement, label) {
  const parts = text.split(find);
  if (parts.length !== 2) throw Error('Survey patch "' + label + '" matched ' + (parts.length - 1) + ' times (expected exactly 1); the source changed, update scripts/system-world-survey.mjs');
  return parts[0] + replacement + parts[1];
}

const EXPOSE = 'globalThis.__sw = { THREE, camera, controls, scene, renderer, composer, experience, traffic, weather, atmosphere, surfaces, windows, sun, bloom, drones, '
  + 'model, catalogV3: () => catalog3, lobbies: lobbyRender, '
  + 'staticCity, nearHeight: NEAR_HEIGHT, harbour, gulls, particles, '
  + 'cancelFlight: () => { flight = null; }, mode: () => mode };\n  ';

async function buildBundle(withoutFurniture) {
  const sceneFile = path.join(uiRoot, 'js/desktop/apps/sysworld-scene.js');
  const societyFile = path.join(uiRoot, 'js/desktop/apps/sysworld-society.js');
  const same = (a, b) => path.resolve(a).toLowerCase() === path.resolve(b).toLowerCase();
  const plugin = {
    name: 'survey-expose',
    setup(b) {
      b.onLoad({ filter: /sysworld-(scene|society)\.js$/ }, async args => {
        const isScene = same(args.path, sceneFile), isSociety = same(args.path, societyFile);
        if (!isScene && !isSociety) return null;
        let text = (await fs.readFile(args.path, 'utf8')).replace(/\r\n/g, '\n');
        if (isScene) {
          if (withoutFurniture) text = replaceOnce(text, '...furnishing().map(', '...furnishing().filter(() => false).map(', 'drop the furnishing pieces from kitPlacements');
          text = replaceOnce(text, '  return {\n    districts, canvas, update, focus(id)', '  ' + EXPOSE + 'return {\n    districts, canvas, update, focus(id)', 'expose internals before createCity return');
        } else {
          text = replaceOnce(text, '    add,patrol,update,suspend,\n', '    add,patrol,update,suspend,actors,\n', 'expose society actors');
        }
        return { contents: text, loader: 'js', resolveDir: path.dirname(args.path) };
      });
    },
  };
  const pkg = JSON.parse(await fs.readFile(path.join(root, 'node_modules/three/package.json'), 'utf8'));
  if (pkg.version !== '0.186.1') throw Error('System World requires Three.js 0.186.1, found ' + pkg.version);
  const result = await build({ entryPoints: [sceneFile], bundle: true, format: 'esm', target: 'es2022', minify: false, write: false,
    absWorkingDir: root, plugins: [plugin], logLevel: 'warning', outdir: path.join(root, '.survey-unused') });
  return Buffer.from(result.outputFiles.find(f => f.path.endsWith('.js')).contents);
}

// ---------- harness page ----------
const PAGE_HTML = `<!doctype html><html><head><meta charset="utf-8"><title>System World survey</title>
<style>html,body{margin:0;height:100%;background:#000;overflow:hidden}#host{position:fixed;inset:0}#host canvas{width:100%;height:100%;display:block}</style>
</head><body><div id="host"></div><script type="module" src="/__survey/page.js"></script></body></html>`;

const PAGE_JS = `
const post = (url, body, type) => fetch(url, { method: 'POST', headers: { 'Content-Type': type || 'text/plain' }, body });
const log = text => post('/__survey/log', String(text)).catch(() => {});
const errors = [], shots = [];
const addError = (where, message) => { errors.push({ where, message: String(message).slice(0, 600) }); log('ERROR [' + where + '] ' + String(message).slice(0, 300)); };
window.addEventListener('error', e => addError('window.error', e.message + (e.filename ? ' @ ' + e.filename + ':' + e.lineno : '')));
window.addEventListener('unhandledrejection', e => addError('unhandledrejection', e.reason && e.reason.stack || e.reason));
const origError = console.error.bind(console);
console.error = (...args) => { const t = args.map(a => (a && a.message) || String(a)).join(' '); if (/THREE|WebGL|shader/i.test(t)) addError('console.error', t); origError(...args); };
const sleep = ms => new Promise(r => setTimeout(r, ms));
let tickCount = 0, timingLoop = false;
const nextFrames = n => new Promise((resolve, reject) => {
  const target = tickCount + n, started = performance.now();
  const poll = () => { if (tickCount >= target) resolve(); else if (performance.now() - started > 30000) reject(Error('animation frames stopped (page hidden?)')); else setTimeout(poll, 10); };
  poll();
});
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
const round = (n, d) => Number(n.toFixed(d || 3));

async function main() {
  const config = await (await fetch('/__survey/config.json')).json();
  const module = await import('/__survey/bundle.js');
  const host = document.getElementById('host');
  const city = await module.createCity(host, {
    label: 'survey', memoryLabel: 'memory', quality: config.tier, reducedMotion: false, signal: undefined,
    assetURL: file => '/3d/system-world/v1/' + file, resourceURL: url => url,
    busy: () => false, replaying: () => false,
    onError: e => addError('city.onError', e && e.message || e), onRobotError: () => addError('city.onRobotError', 'robot model failed'),
    onCompanionError: e => addError('city.onCompanionError', e && e.message || e),
  });
  const sw = globalThis.__sw;
  if (!sw) throw Error('__sw was not exposed');
  city.setData([], []); city.setWorld(undefined, false); city.setMode('orbit'); sw.cancelFlight();
  // Own animation loop, like the app's tick().
  // Near furniture follows the SURVEY camera's height, like the scene's own rule (camera.y > NEAR_HEIGHT hides it).
  let nearApplied = null, surveying = false;
  const syncNear = () => {
    const far = sw.camera.position.y > sw.nearHeight;
    // Always walk the near meshes: the scene's own rule may have flipped them back since the last call.
    let changed = far !== nearApplied;
    nearApplied = far;
    sw.staticCity.traverse(n => { if (n.userData && n.userData.near && n.visible === far) { n.visible = !far; changed = true; } });
    if (changed) sw.renderer.shadowMap.needsUpdate = true;
  };
  let last = 0, elapsed = 0;
  const tick = time => { const dt = last ? Math.min(0.2, (time - last) / 1000) : 1 / 60; last = time; elapsed += dt; if (!timingLoop) { city.update(dt, elapsed); if (surveying) syncNear(); }
    tickCount++; requestAnimationFrame(tick); };
  requestAnimationFrame(tick);
  // Loading settled: createCity resolved (kit loaded); the experience (v2 models, residents) streams on.
  const signature = () => { const s = city.stats(); return [s.loadedBytes, s.experience.loading, s.experience.society.residents.length, s.experience.loaded.length, s.cachedModels].join('|'); };
  let sig = signature(), since = performance.now();
  const loadStart = performance.now();
  while (performance.now() - since < 1500) {
    await sleep(250);
    if (performance.now() - loadStart > 90000) { addError('load', 'loading did not settle within 90 s'); break; }
    const now = signature();
    if (now !== sig) { sig = now; since = performance.now(); }
  }
  const loadMs = Math.round(performance.now() - loadStart);
  log('city loaded in ' + loadMs + ' ms, signature ' + sig);
  // Relax the orbit limits so any free camera is honoured.
  const c = sw.controls;
  c.minDistance = 0; c.maxDistance = Infinity; c.minPolarAngle = 0; c.maxPolarAngle = Math.PI;
  c.enableDamping = false; c.minAzimuthAngle = -Infinity; c.maxAzimuthAngle = Infinity;

  const actors = () => sw.experience.society.actors || [];
  const findResident = index => actors().find(a => a.id === 'resident-' + index && a.body && a.body.enabled);
  function applyCamera(scene) {
    sw.cancelFlight();
    sw.camera.fov = scene.fov || 43; sw.camera.updateProjectionMatrix();
    let pos, tgt;
    if (scene.followResident !== undefined) {
      const a = findResident(scene.followResident);
      if (!a) throw Error('resident-' + scene.followResident + ' not available (disabled by tier or not spawned)');
      const o = scene.offset || [2, 1, 2], h = a.body.heading || 0;
      let dx = o[0], dz = o[2];
      if (scene.offsetRelative) { const s = Math.sin(h), k = Math.cos(h); dx = o[0] * k + o[2] * s; dz = -o[0] * s + o[2] * k; }
      pos = [a.body.x + dx, a.body.y + o[1], a.body.z + dz];
      tgt = [a.body.x, a.body.y + (scene.lookHeight === undefined ? 0.9 : scene.lookHeight), a.body.z];
    } else if (scene.position && scene.target) { pos = scene.position; tgt = scene.target; }
    else return null;
    sw.camera.position.set(pos[0], pos[1], pos[2]);
    c.target.set(tgt[0], tgt[1], tgt[2]);
    c.update();
    return { position: pos, target: tgt };
  }
  const sub = s => ({ tier: s.tier, mode: s.mode, calls: s.calls, triangles: s.triangles, geometries: s.geometries, loadedBytes: s.loadedBytes, frames: s.frames,
    weather: s.weather && { time: s.weather.time, weather: s.weather.weather, daylight: round(s.weather.daylight) },
    furniture: s.furniture, nearHidden: nearApplied });

  for (const scene of config.scenes) {
    try {
      city.setEnvironment({ time: scene.environment || 'day', weather: scene.weather || 'clear' });
      let wanted = applyCamera(scene);
      surveying = true; syncNear();
      await nextFrames(scene.frames === undefined ? 40 : scene.frames);
      await sleep(scene.waitMs === undefined ? 400 : scene.waitMs);
      if (scene.eval) await new AsyncFunction('city', '__sw', 'scene', scene.eval)(city, sw, scene);
      const count = scene.sequence > 0 ? scene.sequence : 1;
      for (let i = 0; i < count; i++) {
        if (i > 0) await sleep(scene.intervalMs === undefined ? 350 : scene.intervalMs);
        const name = count > 1 ? scene.name + '-' + String(i).padStart(2, '0') : scene.name;
        // Camera, render and PNG encode start in one task.
        wanted = applyCamera(scene) || wanted;
        syncNear();
        sw.renderer.info.reset();
        const pending = city.capture();
        const stats = sub(city.stats());
        const actual = sw.camera.position.toArray().map(v => round(v));
        const blob = await pending;
        if (!blob) throw Error('capture returned no image');
        const res = await post('/__survey/shot?name=' + encodeURIComponent(name), blob, 'image/png');
        if (!res.ok) throw Error('shot upload failed ' + res.status);
        const drift = wanted ? Math.hypot(actual[0] - wanted.position[0], actual[1] - wanted.position[1], actual[2] - wanted.position[2]) : 0;
        shots.push({ name, scene: scene.name, requestedPosition: wanted && wanted.position, actualPosition: actual, target: wanted && wanted.target,
          cameraMovedByCollision: drift > 0.05 ? round(drift) : 0, stats });
      }
      if (config.timing) {
        applyCamera(scene); syncNear();
        await nextFrames(2); timingLoop = true; const gl = sw.renderer.getContext(), ms = [];
        for (let i = 0; i < 100; i++) { const t0 = performance.now(); elapsed += 1 / 60; city.update(1 / 60, elapsed); syncNear(); gl.finish(); if (i >= 10) ms.push(performance.now() - t0); }
        timingLoop = false; ms.sort((a, b) => a - b);
        shots[shots.length - 1].frameMs = { median: round(ms[Math.floor(ms.length / 2)], 2), p95: round(ms[Math.min(ms.length - 1, Math.ceil(ms.length * 0.95) - 1)], 2) };
        log('frameMs ' + scene.name + ' median ' + shots[shots.length - 1].frameMs.median + ' p95 ' + shots[shots.length - 1].frameMs.p95);
      }
    } catch (e) { addError('scene ' + scene.name, e && e.stack || e); }
  }
  const info = { tier: config.tier, withoutFurniture: !!config.withoutFurniture, canvas: [sw.renderer.domElement.width, sw.renderer.domElement.height], loadMs, renderer: city.stats().renderer };
  await post('/__survey/report', JSON.stringify({ info, shots, errors }), 'application/json');
}
main().catch(e => addError('main', e && e.stack || e)).finally(async () => {
  try { await post('/__survey/report', JSON.stringify({ info: { failed: true }, shots, errors }), 'application/json'); } catch (_) {}
  await post('/__survey/done', 'done').catch(() => {});
});
`;

// ---------- server ----------
const MIME = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.mjs': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8', '.json': 'application/json', '.glb': 'model/gltf-binary', '.gltf': 'model/gltf+json',
  '.webp': 'image/webp', '.png': 'image/png', '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg', '.svg': 'image/svg+xml', '.wasm': 'application/wasm',
  '.ktx2': 'image/ktx2', '.bin': 'application/octet-stream', '.txt': 'text/plain; charset=utf-8', '.woff2': 'font/woff2', '.ogg': 'audio/ogg', '.mp3': 'audio/mpeg' };

function readBody(req, limit = 64 * 1024 * 1024) {
  return new Promise((resolve, reject) => {
    const chunks = []; let size = 0;
    req.on('data', c => { size += c.length; if (size > limit) { reject(Error('body too large')); req.destroy(); } else chunks.push(c); });
    req.on('end', () => resolve(Buffer.concat(chunks)));
    req.on('error', reject);
  });
}

function findChrome() {
  const candidates = [process.env.CHROME_PATH, 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', 'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe'];
  const found = candidates.find(p => p && existsSync(p));
  if (!found) throw Error('No Chrome/Edge found; set CHROME_PATH');
  return found;
}

async function main() {
  const opts = parseArgs(process.argv.slice(2));
  const outDir = path.resolve(root, opts.out);
  let scenes = JSON.parse(await fs.readFile(path.resolve(root, opts.scenes), 'utf8'));
  if (!Array.isArray(scenes)) throw Error('Scenes file must be a JSON array');
  if (opts.only) {
    const missing = opts.only.filter(n => !scenes.some(s => s.name === n));
    if (missing.length) throw Error('Unknown scene(s) in --only: ' + missing.join(', '));
    scenes = scenes.filter(s => opts.only.includes(s.name));
  }
  if (!scenes.length) throw Error('No scenes to run');
  await fs.mkdir(outDir, { recursive: true });

  console.log('building debug bundle ...');
  const bundle = await buildBundle(opts.withoutFurniture);
  const config = Buffer.from(JSON.stringify({ tier: opts.tier, scenes, timing: opts.timing, withoutFurniture: opts.withoutFurniture }));

  const notFound = new Set();
  let saved = 0, report = null, finish;
  const done = new Promise(r => { finish = r; });
  const server = http.createServer(async (req, res) => {
    try {
      const url = new URL(req.url, 'http://127.0.0.1');
      const send = (code, type, body) => { res.writeHead(code, { 'Content-Type': type, 'Cache-Control': 'no-store' }); res.end(body); };
      if (req.method === 'POST') {
        const body = await readBody(req);
        if (url.pathname === '/__survey/shot') {
          const name = (url.searchParams.get('name') || '').toLowerCase().replace(/[^a-z0-9-]/g, '-').slice(0, 80);
          if (!name) return send(400, 'text/plain', 'bad name');
          await fs.writeFile(path.join(outDir, name + '.png'), body); saved++;
          console.log('saved ' + path.relative(root, path.join(outDir, name + '.png')) + ' (' + Math.round(body.length / 1024) + ' KB)');
          return send(200, 'text/plain', 'ok');
        }
        if (url.pathname === '/__survey/report') {
          const data = JSON.parse(body.toString('utf8'));
          // A later (failure-path) report must not clobber a complete one.
          if (!report || !data.info?.failed) report = { ...data, generatedAt: new Date().toISOString(), http404: [...notFound] };
          else report.http404 = [...notFound];
          await fs.writeFile(path.join(outDir, 'report.json'), JSON.stringify(report, null, 2));
          return send(200, 'text/plain', 'ok');
        }
        if (url.pathname === '/__survey/log') { console.log('[page] ' + body.toString('utf8')); return send(200, 'text/plain', 'ok'); }
        if (url.pathname === '/__survey/done') { send(200, 'text/plain', 'ok'); finish('done'); return; }
        return send(404, 'text/plain', 'not found');
      }
      if (url.pathname === '/__survey/index.html') return send(200, MIME['.html'], PAGE_HTML);
      if (url.pathname === '/__survey/page.js') return send(200, MIME['.js'], PAGE_JS);
      if (url.pathname === '/__survey/bundle.js') return send(200, MIME['.js'], bundle);
      if (url.pathname === '/__survey/config.json') return send(200, MIME['.json'], config);
      const rel = decodeURIComponent(url.pathname).replace(/^\/+/, '');
      const file = path.resolve(uiRoot, rel);
      if (file !== uiRoot && !file.startsWith(uiRoot + path.sep)) return send(403, 'text/plain', 'forbidden');
      let stat = null; try { stat = await fs.stat(file); } catch (_) { /* missing */ }
      if (!stat || !stat.isFile()) { if (!/favicon/.test(url.pathname)) notFound.add(url.pathname); return send(404, 'text/plain', 'not found'); }
      res.writeHead(200, { 'Content-Type': MIME[path.extname(file).toLowerCase()] || 'application/octet-stream', 'Content-Length': stat.size, 'Cache-Control': 'no-store' });
      (await import('node:fs')).createReadStream(file).pipe(res);
    } catch (e) { console.error('server error: ' + e.message); try { res.writeHead(500); res.end(); } catch (_) { /* gone */ } }
  });
  await new Promise(r => server.listen(0, '127.0.0.1', r));
  const port = server.address().port;
  const pageUrl = 'http://127.0.0.1:' + port + '/__survey/index.html';

  const chromePath = findChrome();
  const profile = await fs.mkdtemp(path.join(os.tmpdir(), 'sw-survey-'));
  const args = ['--headless=new', '--user-data-dir=' + profile, '--window-size=' + opts.width + ',' + opts.height, '--no-first-run',
    '--no-default-browser-check', '--disable-extensions', '--hide-scrollbars', '--use-angle=d3d11', '--enable-gpu', '--ignore-gpu-blocklist',
    '--enable-unsafe-swiftshader', pageUrl];
  console.log('server ' + pageUrl + '\nlaunching ' + chromePath);
  const chrome = spawn(chromePath, args, { stdio: 'ignore', windowsHide: true });
  let spawnError = null;
  chrome.on('error', e => { spawnError = e; finish('spawn-error'); });
  chrome.on('exit', code => { if (!report) { console.error('Chrome exited early with code ' + code); finish('chrome-exit'); } });

  const timer = setTimeout(() => finish('timeout'), opts.timeout * 1000);
  const reason = await done;
  clearTimeout(timer);

  const stopChrome = () => {
    if (!chrome.pid || chrome.exitCode !== null) return;
    if (process.platform === 'win32') spawnSync('taskkill', ['/PID', String(chrome.pid), '/T', '/F'], { stdio: 'ignore' });
    else try { process.kill(-chrome.pid, 'SIGKILL'); } catch (_) { try { chrome.kill('SIGKILL'); } catch (__) { /* gone */ } }
  };
  const cleanup = async () => {
    stopChrome();
    server.closeAllConnections?.(); server.close();
    for (let i = 0; i < 10; i++) { try { await fs.rm(profile, { recursive: true, force: true }); break; } catch (_) { await new Promise(r => setTimeout(r, 300)); } }
  };

  if (opts.keepOpen && reason === 'done') {
    console.log('--keep-open: server and Chrome stay up at ' + pageUrl + ' (Ctrl+C to stop)');
    process.on('SIGINT', async () => { await cleanup(); process.exit(0); });
    await new Promise(() => {});
  }
  await cleanup();

  if (spawnError) { console.error('Could not start Chrome: ' + spawnError.message); process.exit(1); }
  const errorCount = report?.errors?.length || 0;
  console.log('--- summary ---');
  console.log('result: ' + reason + ', shots saved: ' + saved + ', page errors: ' + errorCount + (report?.http404?.length ? ', 404s: ' + report.http404.join(' ') : ''));
  if (report?.info?.canvas) console.log('canvas ' + report.info.canvas.join('x') + ', tier ' + report.info.tier + ', ' + report.info.renderer);
  if (opts.timing) console.log('frameMs (median/p95): ' + (report?.shots || []).filter(x => x.frameMs).map(x => x.scene + ' ' + x.frameMs.median + '/' + x.frameMs.p95).join(', '));
  for (const e of report?.errors || []) console.log('  error [' + e.where + '] ' + String(e.message).split('\n')[0]);
  console.log('report: ' + path.relative(root, path.join(outDir, 'report.json')));
  process.exit(reason === 'done' && report && !report.info?.failed && errorCount === 0 ? 0 : 1);
}

main().catch(e => { console.error(e.message || e); process.exit(1); });
