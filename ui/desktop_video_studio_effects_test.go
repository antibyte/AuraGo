package ui

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// videoStudioEffectsPrelude loads the DOM-free effect module the way the app does: directly after the time module.
const videoStudioEffectsPrelude = `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
const E = window.VideoStudioEffects;
`

// runVideoStudioEffectsScript runs a Node check after the prelude and returns its standard output.
func runVideoStudioEffectsScript(t *testing.T, script string) []byte {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio effect checks")
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(node, "-e", videoStudioEffectsPrelude+script)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("effect checks: %v\n%s%s", err, stdout.Bytes(), stderr.Bytes())
	}
	return stdout.Bytes()
}

// BUILTIN is the JS mirror of Go's registry (plan index §4.13): both decode to the same JSON value.
func TestVideoStudioEffectsRegistryMatchesGo(t *testing.T) {
	goJSON, err := json.Marshal(videostudio.EffectRegistry())
	if err != nil {
		t.Fatal(err)
	}
	jsJSON := runVideoStudioEffectsScript(t, `process.stdout.write(JSON.stringify(E.BUILTIN));`)
	var goValue, jsValue any
	if err := json.Unmarshal(goJSON, &goValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(jsJSON, &jsValue); err != nil {
		t.Fatalf("BUILTIN is not JSON: %v\n%s", err, jsJSON)
	}
	if !reflect.DeepEqual(goValue, jsValue) {
		t.Fatalf("VideoStudioEffects.BUILTIN differs from videostudio.EffectRegistry():\nGo: %s\nJS: %s", goJSON, jsJSON)
	}
}

func TestVideoStudioEffectsAdoptAndLookups(t *testing.T) {
	runVideoStudioEffectsScript(t, `
// A server from before 2a sends no registry: the built-in copy stays, and every transition is available.
assert.equal(E.adopt(undefined, undefined), false);
assert.equal(E.registry(), E.BUILTIN);
assert.deepEqual(E.transitions().map(spec => spec.id), ['dissolve', 'black', 'wipeleft', 'wiperight']);
assert(E.transitions().every(spec => spec.available));
assert.equal(E.transition('wipeleft').xfade, 'wiperight', 'the user-facing wipe maps to the opposite xfade name');
assert.equal(E.transition('circleopen'), null);
// Invalid input never replaces the registry.
const broken = JSON.parse(JSON.stringify(E.BUILTIN));
broken.transitions = [{id: 'x'}];
for (const bad of [null, 'registry', {version: 1}, {version: 99, base: [], effects: [], transitions: [], blend_modes: [], anim_presets: [], looks: []}, broken]) {
  assert.equal(E.adopt(bad, null), false, JSON.stringify(bad));
  assert.equal(E.registry(), E.BUILTIN);
}
// A valid server registry is adopted; the unavailable lists mark what the server's FFmpeg lacks.
const server = JSON.parse(JSON.stringify(E.BUILTIN));
server.transitions.push({id: 'circleopen', xfade: 'circleopen', category: 'shape', label_key: 'transition_circleopen'});
assert.equal(E.adopt(server, {effects: [], transitions: ['circleopen', 42], blend_modes: [], anim_presets: []}), true);
assert.equal(E.isTransition('circleopen'), true);
assert.deepEqual(E.transitions().map(spec => [spec.id, spec.available]), [['dissolve', true], ['black', true], ['wipeleft', true], ['wiperight', true], ['circleopen', false]]);
assert.equal(E.transition('circleopen').label_key, 'transition_circleopen');
server.transitions.pop();
assert.equal(E.isTransition('circleopen'), true, 'the adopted registry is a copy');
assert.equal(E.adopt(server, 'nope'), true);
assert(E.transitions().every(spec => spec.available), 'a missing unavailable object marks nothing');
assert.equal(E.isTransition('circleopen'), false);
// Base parameters and defaults.
assert.equal(E.base('opacity').keyframable, true);
assert.equal(E.base('fit').default, 'contain');
assert.equal(E.base('nope'), null);
assert.equal(E.effect('color').category, 'color', '2c fills the effect table');
assert.equal(E.effect('nope'), null);
assert.deepEqual(E.defaults('lut'), {lut: '', intensity: 1});
assert.deepEqual(E.defaults('nope'), {});
// valueAt without keyframes: the clip's own number, else the registry default, else null.
assert.equal(E.valueAt({opacity: 0.25}, 'opacity', 10, {fps: 30}), 0.25);
assert.equal(E.valueAt({}, 'opacity', 10, {fps: 30}), 1);
assert.equal(E.valueAt({rotation: null}, 'rotation', 0, {fps: 30}), 0);
assert.equal(E.valueAt({width: '0.5'}, 'width', 0, {fps: 30}), 1, 'only numbers count');
assert.equal(E.valueAt({crop: {l: 0.1}}, 'crop.l', 0, {fps: 30}), 0.1);
assert.equal(E.valueAt({effects: [{id: 'blur', params: {radius: 3}}]}, 'effects.0.radius', 0, {fps: 30}), 3);
assert.equal(E.valueAt({}, 'mask.feather', 0, {fps: 30}), null);
// clipState: the resolved per-frame state both painters draw from, with neutral 2a defaults.
const neutral = {crop: {l: 0, t: 0, r: 0, b: 0}, flipH: false, flipV: false, blend: 'normal', mask: null, effects: []};
assert.deepEqual(E.clipState({x: 0.1, y: 0.2, width: 0.5, height: 0.4, rotation: 15, opacity: 0.5, fit: 'cover'}, 0, {fps: 30}),
  Object.assign({x: 0.1, y: 0.2, width: 0.5, height: 0.4, rotation: 15, opacity: 0.5, fit: 'cover'}, neutral));
assert.deepEqual(E.clipState({}, 0, {fps: 30}), Object.assign({x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain'}, neutral), 'missing values take the base defaults');
assert.equal(E.clipState({opacity: 1.5}, 0, {fps: 30}).opacity, 1, 'opacity stays within 0..1');
console.log('ok');
`)
}

func TestVideoStudioEffectsDriveTimelineAndInspector(t *testing.T) {
	app, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio.js"))
	if err != nil {
		t.Fatal(err)
	}
	status, adopt := strings.Index(string(app), "s.status = response.body;"), strings.Index(string(app), "window.VideoStudioEffects.adopt(s.status.registry, s.status.unavailable);")
	if status < 0 || adopt < status {
		t.Fatal("initialize must adopt the status registry right after reading the status")
	}
	runVideoStudioEffectsScript(t, `
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-inspector.js');
const T = window.VideoStudioTimeline, I = window.VideoStudioInspector;
const box = {offset: 0, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fade_in: 0, fade_out: 0, fit: 'contain'};
const pair = type => ({fps: 30, assets: [{id: 'v', kind: 'video', duration_frames: 300}], tracks: [{id: 't', kind: 'video', name: 'Video', clips: [
  Object.assign({id: 'a', asset_id: 'v', start: 0, duration: 60, transition: {type, duration: 10}}, box),
  Object.assign({id: 'b', asset_id: 'v', start: 50, duration: 60, transition: null}, box)]}]});
// The timeline check reads the registry at call time.
E.adopt(undefined, undefined);
for (const type of ['dissolve', 'black', 'wipeleft', 'wiperight']) assert.equal(T.validTimeline(pair(type)), true, type);
assert.equal(T.validTimeline(pair('circleopen')), false, 'a transition outside the registry is refused');
const server = JSON.parse(JSON.stringify(E.BUILTIN));
server.transitions.push({id: 'circleopen', xfade: 'circleopen', category: 'shape', label_key: 'transition_circleopen'});
E.adopt(server, null);
assert.equal(T.validTimeline(pair('circleopen')), true, 'an adopted transition is accepted');
// The inspector lists 'none' plus the registry; unavailable transitions are disabled with a title, except the current one.
const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => '&#' + c.charCodeAt(0) + ';');
const view = (current, unavailable, registry) => {
  E.adopt(registry || E.BUILTIN, {effects: [], transitions: unavailable, blend_modes: [], anim_presets: []});
  const project = pair(current), track = project.tracks[0], host = {innerHTML: ''};
  I.render(host, {selected: {clip: track.clips[0], track}, asset: project.assets[0], project, locale: 'en', caps: {},
    transitionInfo: {next: track.clips[1]}, esc, icon: () => '', tr: (key, fallback) => key === 'transitionUnavailable' ? 'UNAVAILABLE' : '[' + key + ']'});
  return host.innerHTML;
};
const button = (html, id) => (html.match(new RegExp('<button[^>]*data-transition="' + id + '"[^>]*>')) || [''])[0];
let html = view('dissolve', ['black']);
assert.deepEqual(Array.from(html.matchAll(/data-transition="([a-z]+)"/g), m => m[1]), ['none', 'dissolve', 'black', 'wipeleft', 'wiperight'], 'none plus the registry in menu order');
assert(/ disabled/.test(button(html, 'black')) && button(html, 'black').includes('title="UNAVAILABLE"'), 'an unavailable transition is disabled with a title');
assert(!/ disabled/.test(button(html, 'dissolve')) && button(html, 'dissolve').includes('aria-pressed="true"'));
assert(!button(html, 'wipeleft').includes('title='), 'available transitions carry no title');
assert(html.includes('<i class="vs-tx-wipeleft"></i><span>[wipeleft]</span>'), 'labels come from label_key, icons stay vs-tx-<id>');
html = view('black', ['black']);
assert(!/ disabled/.test(button(html, 'black')) && button(html, 'black').includes('aria-pressed="true"'), 'the current transition stays selectable even when unavailable');
assert(button(html, 'black').includes('title="UNAVAILABLE"'));
html = view('dissolve', [], server);
assert(html.includes('<span>[transition_circleopen]</span>'), 'a registry transition is labelled by its label_key');
console.log('ok');
`)
}

// videoStudioParityRunners maps a fixture topic (a file name under internal/videostudio/testdata/parity) to its JS side:
// run(input) returns the value compared with want. Exact topics compare with deepStrictEqual (pixel geometry); the
// others compare number by number within 1e-6. A slice that adds a fixture file adds its entry here in the same task.
const videoStudioParityRunners = `{
  geometry: {exact: true, run(input) {
    const project = {width: input.canvas.width, height: input.canvas.height};
    return E.placement(E.clipState(input.clip, 0, project), project, project, null).box;
  }},
  // Fade-through-black weights and wipe edges (phase 2 index §4.13, topic "transitions"); numbers within 1e-6.
  transitions: {exact: false, run(input) {
    if (input.kind === 'fade_black') { const w = E.fadeBlackWeights(input.p); return {out: w.out, inc: w.inc, black: w.black}; }
    if (input.kind === 'wipe') return E.wipeEdge(input.type, input.p, input.width);
    throw new Error('unknown transitions input kind ' + input.kind);
  }},
  // Easing curves and keyframe sampling (phase 2 index §4.13, topics "easing" and "keyframes"); numbers within 1e-6.
  easing: {exact: false, run(input) { return {v: E.ease(input.easing, input.t)}; }},
  keyframes: {exact: false, run(input) {
    if (input.clip) return {v: E.valueAt(input.clip, input.path, input.frame, {fps: 25})};
    return {v: E.sample(input.keyframes, input.f)};
  }},
  // Speed-ramp pieces, S, its inverse and the speed (topic "ramp"); numbers within 1e-6.
  ramp: {exact: false, run(input) {
    const pieces = E.rampPieces(input.points, input.from, input.to);
    return {pieces: pieces.map(p => [p.t0, p.t1, p.s0, p.a, p.b]), source: input.t.map(t => E.rampSource(pieces, t)),
      inverse: input.s.map(s => E.rampInverse(pieces, s)), speed: input.t.map(t => E.rampSpeed(pieces, t))};
  }},
}`

// Every shared math fixture runs in JS too (plan index §4.13); a topic without a runner fails.
func TestVideoStudioEffectsParity(t *testing.T) {
	output := runVideoStudioEffectsScript(t, `
const fs = require('node:fs'), path = require('node:path');
const runners = `+videoStudioParityRunners+`;
const dir = path.join('..', 'internal', 'videostudio', 'testdata', 'parity');
function match(got, want, label) {
  if (typeof want === 'number') { assert.equal(typeof got, 'number', label); assert(Math.abs(got - want) <= 1e-6, label + ': ' + got + ' != ' + want); return; }
  if (Array.isArray(want)) { assert(Array.isArray(got) && got.length === want.length, label + ': length'); want.forEach((item, i) => match(got[i], item, label + '[' + i + ']')); return; }
  if (want && typeof want === 'object') { assert(got && typeof got === 'object', label); Object.keys(want).forEach(key => match(got[key], want[key], label + '.' + key)); return; }
  assert.equal(got, want, label);
}
const files = fs.readdirSync(dir).filter(name => name.endsWith('.json')).sort();
assert(files.length > 0, 'no parity fixtures');
let checked = 0;
for (const file of files) {
  const topic = file.slice(0, -5), runner = runners[topic];
  assert(runner, 'no JS runner for parity topic ' + topic);
  const fixture = JSON.parse(fs.readFileSync(path.join(dir, file), 'utf8'));
  assert(Array.isArray(fixture.cases) && fixture.cases.length > 0, topic + ' has no cases');
  for (const c of fixture.cases) {
    const got = runner.run(c.input), label = topic + '/' + c.name;
    if (runner.exact) assert.deepStrictEqual(got, c.want, label); else match(got, c.want, label);
    checked++;
  }
}
console.log('parity cases: ' + checked);
`)
	t.Logf("%s", output)
}

// placement: the export's even box scaled to the preview canvas, the picture rectangle FFmpeg's scale produces, and
// the rotation in radians (index §4.4). The GL compositor draws from it; the 2D painter keeps today's math in 2a.
func TestVideoStudioEffectsPlacement(t *testing.T) {
	runVideoStudioEffectsScript(t, `
const project = {width: 1280, height: 720}, half = {width: 640, height: 360};
const full = E.clipState({x: 0, y: 0, width: 1, height: 1, fit: 'contain'}, 0, project);
// No source size: the picture fills the box. The preview canvas at half quality scales the export's box.
assert.deepEqual(E.placement(full, half, project, null), {box: {x: 0, y: 0, w: 640, h: 360}, draw: {x: 0, y: 0, w: 640, h: 360}, clip: false, angle: 0});
// Contain: the largest even size inside the box (404x720 for a 720x1280 source), centred like the export's pad.
assert.deepEqual(E.placement(full, half, project, {width: 720, height: 1280}).draw, {x: 219, y: 0, w: 202, h: 360});
// Cover: the smallest even size covering the box (1280x2276), centred like crop, and clipped to the box.
const cover = E.placement(Object.assign({}, full, {fit: 'cover'}), half, project, {width: 720, height: 1280});
assert.deepEqual(cover.draw, {x: 0, y: -389, w: 640, h: 1138});
assert.equal(cover.clip, true);
// A quarter box at full quality: a 16:9 source fills it exactly; rotation is in radians around the box centre.
const quarter = E.placement(E.clipState({x: 0.25, y: 0.25, width: 0.5, height: 0.5, rotation: 30, fit: 'contain'}, 0, project), project, project, {width: 1920, height: 1080});
assert.deepEqual(quarter.box, {x: 320, y: 180, w: 640, h: 360});
assert.deepEqual(quarter.draw, quarter.box);
assert(Math.abs(quarter.angle - Math.PI / 6) < 1e-12);
console.log('ok');
`)
}

// Keyframes are sampled at the source position the clip shows (plan index §4.18): the cubic easing curves, constant
// values outside the keys, the left key's easing between them, and clipState following content at any speed.
func TestVideoStudioEffectsKeyframeSampling(t *testing.T) {
	runVideoStudioEffectsScript(t, `
const near = (got, want, label) => assert(Math.abs(got - want) < 1e-12, label + ': ' + got + ' != ' + want);
assert.deepEqual(E.keyframable(), ['x', 'y', 'width', 'height', 'rotation', 'opacity', 'crop.l', 'crop.t', 'crop.r', 'crop.b']);
near(E.ease('ease_in', 0.5), 0.125, 'ease_in');
near(E.ease('ease_out', 0.5), 0.875, 'ease_out');
near(E.ease('ease_in_out', 0.25), 0.0625, 'ease_in_out');
near(E.ease('hold', 0.99), 0, 'hold');
near(E.ease('hold', 1), 1, 'hold at the end');
near(E.ease('linear', 7), 1, 't is clamped');
near(E.ease('nope', 0.3), 0.3, 'unknown names are linear');
const keys = [{f: 10, v: 1, e: 'linear'}, {f: 20, v: 3, e: 'ease_in'}, {f: 30, v: 2, e: 'hold'}, {f: 40, v: 5, e: 'ease_out'}];
near(E.sample(keys, -100), 1, 'before the first key');
near(E.sample(keys, 15), 2, 'linear');
near(E.sample(keys, 25), 2.875, 'ease_in of the left key');
near(E.sample(keys, 39.999), 2, 'hold until the next key');
near(E.sample(keys, 40), 5, 'on the last key');
near(E.sample(keys, 1e9), 5, 'after the last key');
assert.equal(E.sample([], 3), null);
// clipState reads keyframes at the source position the clip shows (content anchor).
const kf = {x: [{f: 40, v: 0, e: 'linear'}, {f: 80, v: 0.4, e: 'linear'}], opacity: [{f: 40, v: 1, e: 'hold'}, {f: 60, v: 0, e: 'linear'}]};
const clip = {start: 100, offset: 40, duration: 50, x: 0.9, y: 0.1, width: 0.5, height: 0.5, rotation: 0, opacity: 0.7, fit: 'contain', keyframes: kf};
const project = {fps: 25};
near(E.clipState(clip, 120, project).x, 0.2, 'source 60 at speed 1');
near(E.clipState(Object.assign({}, clip, {speed: 2}), 110, project).x, 0.2, 'speed 2 shows the same content after half the frames');
near(E.clipState(Object.assign({}, clip, {speed: 0.5}), 140, project).x, 0.2, 'speed 0.5 after twice the frames');
near(E.clipState(clip, 90, project).x, 0, 'before the first key the first value holds');
near(E.clipState(clip, 149, project).x, 0.4, 'after the last key the last value holds');
near(E.clipState(clip, 119, project).opacity, 1, 'hold keeps the value until the next key');
near(E.clipState(clip, 120, project).opacity, 0, 'and switches on it');
near(E.clipState(clip, 105, project).y, 0.1, 'a path without keyframes keeps its static value');
near(E.clipState(clip, 105, project).x, 0.05, 'keyframes replace the static value');
// Effect parameters keep their keyframes inside the effect entry (spec §5.4), sampled the same way.
const fx = {start: 0, offset: 0, effects: [{id: 'blur', params: {radius: 3}, keyframes: {radius: [{f: 0, v: 0, e: 'linear'}, {f: 10, v: 10, e: 'linear'}]}}]};
near(E.valueAt(fx, 'effects.0.radius', 5, project), 5, 'effect keyframes');
near(E.valueAt({start: 50, offset: 0, duration: 75, keyframes: {opacity: [{f: 0, v: 0, e: 'linear'}, {f: 25, v: 1, e: 'linear'}]}}, 'opacity', 60, project), 0.4, 'a still uses offset 0');
near(E.valueAt({opacity: 0.25, keyframes: {opacity: []}}, 'opacity', 0, project), 0.25, 'an empty list counts as no keyframes');
console.log('ok');
`)
}

// Ramp time maps (plan index §4.18): sourceAt and timelineAt invert each other, speedAt follows the pieces, and the
// extent, maxDurationFor and headroom helpers use S. Without a ramp they are the constant-speed formulas the edit code
// used so far.
func TestVideoStudioEffectsRampTimeMaps(t *testing.T) {
	runVideoStudioEffectsScript(t, `
const near = (got, want, label) => assert(Math.abs(got - want) < 1e-9, label + ': ' + got + ' != ' + want);
const ramp = [{f: 0, speed: 1, e: 'linear'}, {f: 20, speed: 3, e: 'linear'}];
const clip = {start: 100, offset: 10, duration: 30, ramp, keyframes: {x: [{f: 10, v: 0, e: 'linear'}, {f: 50, v: 1, e: 'linear'}]}};
assert.equal(E.hasRamp(clip), true);
assert.equal(E.hasRamp({ramp: []}), false);
assert.equal(E.hasRamp({speed: 2}), false);
near(E.clipSource(clip, 10), 15, 'S(10)');
near(E.sourceAt(clip, 110), 25, 'sourceAt');
near(E.timelineAt(clip, 25), 110, 'timelineAt inverts sourceAt');
near(E.timelineAt(clip, 16.25), 105, 'inside the accelerating span');
near(E.speedAt(clip, 5), 1.5, 'the speed rises linearly');
near(E.speedAt(clip, 25), 3, 'constant after the last point');
near(E.sourceExtent(clip), 67, 'S(duration - 1)');
assert.equal(E.maxDurationFor(clip, 300, 10), 104, 'offset + S(103) = 299 fits a 300-frame asset');
assert.equal(E.maxDurationFor(clip, Infinity, 10), Infinity, 'stills are unbounded');
assert.equal(E.headroom(clip), 10, 'the head grows at the first point speed (1) until offset 0');
assert.equal(E.headroom({offset: 9, duration: 30, ramp: [{f: 0, speed: 2, e: 'linear'}, {f: 10, speed: 1, e: 'linear'}]}), 4, 'at speed 2 nine source frames hold 4.5 head frames');
near(E.clipState(clip, 110, {fps: 25}).x, 0.375, 'keyframes follow the ramp');
const fast = {start: 100, offset: 30, duration: 30, speed: 2};
near(E.sourceAt(fast, 105), 40, 'speed 2');
near(E.timelineAt(fast, 40), 105, 'speed 2 inverse');
near(E.speedAt(fast, 3), 2, 'constant speed');
near(E.sourceExtent(fast), 58, 'constant-speed extent');
assert.equal(E.maxDurationFor(fast, 300, 10), 145, 'floor(289 / 2) + 1');
assert.equal(E.headroom({offset: 9, speed: 2}), 4);
assert.equal(E.headroom({offset: 9}), 9);
// A split or head trim that shifts the points keeps the curve: S'(t) = S(t + k) - S(k).
const whole = E.rampPieces(ramp, 0, 30), cut = E.rampPieces([{f: -5, speed: 1, e: 'linear'}, {f: 15, speed: 3, e: 'linear'}], 0, 25);
for (let t = 0; t <= 25; t += 0.5) near(E.rampSource(cut, t), E.rampSource(whole, t + 5) - E.rampSource(whole, 5), 'shifted curve at ' + t);
// Sixteen eased points stay within the piece limit.
const many = Array.from({length: 16}, (_, i) => ({f: 5 + 10 * i, speed: 0.25 + i % 4, e: 'ease_in_out'}));
assert.equal(E.rampPieces(many, 0, 200).length, 62);
assert(E.rampPieces(many, 0, 200).length <= E.LIMITS.rampPieces);
console.log('ok');
`)
}

// LIMITS mirrors Go's keyframe and ramp limits (plan index §4.4). rampEaseSteps is unexported in Go; the eased spans of
// the ramp parity fixture pin it on both sides, and TestRampPiecesStayWithinLimit pins the Go value.
func TestVideoStudioEffectsLimitsMatchGo(t *testing.T) {
	output := runVideoStudioEffectsScript(t, `process.stdout.write(JSON.stringify(E.LIMITS));`)
	var got map[string]int
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("LIMITS is not JSON: %v\n%s", err, output)
	}
	want := map[string]int{
		"keyframesPerParam": videostudio.MaxKeyframesPerParam,
		"keyframesPerClip":  videostudio.MaxKeyframesPerClip,
		"rampPoints":        videostudio.MaxRampPoints,
		"rampPieces":        videostudio.MaxRampPieces,
		"rampEaseSteps":     4,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("VideoStudioEffects.LIMITS = %v, want the Go constants %v", got, want)
	}
}

// The smallest width and height a keyframe may hold is the editor's 1 % box minimum on both sides: Go MinBoxSize,
// VideoStudioEffects.MIN_BOX_SIZE and the clamp of every key the editor writes (VideoStudioKeyframeEdit).
func TestVideoStudioEffectsMinBoxSizeMatchesGo(t *testing.T) {
	output := runVideoStudioEffectsScript(t, `
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-edit.js');
require('./js/desktop/apps/video-studio-keyframe-edit.js');
const T = window.VideoStudioTimeline, K = window.VideoStudioKeyframeEdit;
const clip = Object.assign(T.makeClip({id: 'v', kind: 'video', duration_frames: 300}, 0, 'c', {fps: 30}), {duration: 60, keyframes: {width: [{f: 0, v: 0.5, e: 'linear'}]}});
const project = {version: 3, fps: 30, width: 1280, height: 720, assets: [{id: 'v', kind: 'video', has_audio: false, duration_frames: 300}], tracks: [{id: 'v1', kind: 'video', name: 'v1', clips: [clip]}]};
K.setValues(project, 'c', {width: 0.0001}, 10);
process.stdout.write(JSON.stringify({min: E.MIN_BOX_SIZE, written: clip.keyframes.width.find(key => key.f === 10).v}));
`)
	var got struct{ Min, Written float64 }
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, output)
	}
	if got.Min != videostudio.MinBoxSize || got.Written != videostudio.MinBoxSize {
		t.Fatalf("MIN_BOX_SIZE = %v and the smallest key the editor writes = %v, want Go MinBoxSize %v", got.Min, got.Written, videostudio.MinBoxSize)
	}
}
