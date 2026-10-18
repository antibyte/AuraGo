package ui

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

func TestVideoStudioCanonicalProjectV2(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio canonical form checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-core.js');
const P = window.VideoStudioParts;
const asset = {id:'a',name:'a.mp4',path:'media/a',kind:'video',duration_frames:30,width:64,height:36,has_audio:true,duration_us:1000000,video_duration_us:1000000,source_fps:'30/1',codec:'h264',size_bytes:99,media_url:'/m'};
const project = {version:2,name:'x',width:1280,height:720,fps:30,ntsc:false,assets:[asset],tracks:[]};
const c = P.canonicalProject(project);
assert.equal('ntsc' in c, false, 'false ntsc is omitted like Go omitempty');
assert.deepEqual(c.assets[0], {id:'a',name:'a.mp4',path:'media/a',kind:'video',duration_frames:30,width:64,height:36,has_audio:true,duration_us:1000000,video_duration_us:1000000,source_fps:'30/1',codec:'h264',size_bytes:99});
assert.deepEqual(Object.keys(c.assets[0]), ['id','name','path','kind','duration_frames','duration_us','video_duration_us','width','height','has_audio','source_fps','codec','size_bytes'], 'asset keys follow the Go struct order');
const serverOrdered = {id:'a',name:'a.mp4',path:'media/a',kind:'video',duration_frames:30,duration_us:1000000,video_duration_us:1000000,width:64,height:36,has_audio:true,source_fps:'30/1',codec:'h264',size_bytes:99};
assert.equal(JSON.stringify(c.assets[0]), JSON.stringify(serverOrdered), 'a canonical asset stringifies like the server copy');
assert.equal(P.canonicalProject(Object.assign({}, project, {ntsc:true})).ntsc, true);
const s = {projectId:'p'};
assert.equal(P.hydrateProject(s, {version:1,name:'old',width:1280,height:720,fps:30,assets:[],tracks:[]}).version, 3, 'hydrating a 30 fps version 1 copy upgrades it through version 2 to 3 like the server');
assert.equal(P.hydrateProject(s, {version:1,fps:30,ntsc:true,assets:[],tracks:[]}).version, 1, 'an invalid version 1 copy is not upgraded');
assert.equal(P.hydrateProject(s, {version:2,fps:25,assets:[],tracks:[]}).version, 3, 'a version 2 copy becomes version 3 without data changes');
assert.equal(P.hydrateProject(s, {version:3,fps:25,assets:[],tracks:[]}).version, 3);
assert.equal(P.defaultProject('New', {ctx: {}}).version, 3, 'new projects are version 3');
const image = {id:'b',name:'b.png',path:'media/b',kind:'image',duration_frames:0,width:1,height:1,has_audio:false};
const bare = P.canonicalProject(Object.assign({}, project, {assets:[image]}));
assert.deepEqual(Object.keys(bare.assets[0]), ['id','name','path','kind','duration_frames','width','height','has_audio'], 'absent v2 fields stay absent');

// The top level follows Go's Project field order however the live keys were added: rescaleProject appends ntsc after tracks.
const GO_KEYS = ['version','name','width','height','fps','ntsc','assets','tracks'];
const live = {version:2,name:'x',width:1280,height:720,fps:30,assets:[Object.assign({}, asset)],tracks:[]};
window.VideoStudioTime.rescaleProject(live, 30, true);
assert.equal(Object.keys(live).pop(), 'ntsc', 'the live project really has ntsc after tracks');
const canonical = P.canonicalProject(live);
assert.deepEqual(Object.keys(canonical), GO_KEYS, 'ntsc sits between fps and assets like the server copy');
const serverCopy = {version:2,name:'x',width:1280,height:720,fps:30,ntsc:true,assets:canonical.assets,tracks:[]};
assert.equal(JSON.stringify(canonical), JSON.stringify(serverCopy), 'a rescaled project stringifies like the server copy');
const comparable = project => { const c = P.canonicalProject(project); c.assets = []; return JSON.stringify(c); };
assert.equal(comparable(live), comparable(serverCopy), 'refresh and 412 adoption see no false conflict');
// A draft written by persistDraft uses the same form.
const stored = {};
global.localStorage = {setItem: (key, value) => { stored[key] = value; }};
P.persistDraft({projectId: 'p', project: live});
assert.deepEqual(Object.keys(JSON.parse(stored['video-studio-draft:p']).project), GO_KEYS, 'drafts use the canonical key order');
window.VideoStudioTime.rescaleProject(live, 25, false);
assert.deepEqual(Object.keys(P.canonicalProject(live)), ['version','name','width','height','fps','assets','tracks'], 'ntsc is absent without NTSC timing');
const extra = Object.assign({}, live, {unknown: 1});
assert.deepEqual(Object.keys(P.canonicalProject(extra)), ['version','name','width','height','fps','assets','tracks'], 'keys the server does not know are not sent');
// Markers and the in/out points follow Go's omitempty JSON; an in point of 0 is still a set value.
const timed = Object.assign({}, live, {markers: [{id: 'm1', frame: 0, name: 'Start', color: 'teal', duration: 0}, {id: 'm2', frame: 30, duration: 12}], in_point: 0, out_point: 90});
assert.deepEqual(Object.keys(P.canonicalProject(timed)), ['version','name','width','height','fps','assets','tracks','markers','in_point','out_point'], 'marker and in/out keys follow the Go struct order');
assert.deepEqual(P.canonicalProject(timed).markers, [{id: 'm1', frame: 0, name: 'Start', color: 'teal'}, {id: 'm2', frame: 30, duration: 12}], 'empty marker fields are omitted');
assert.deepEqual(Object.keys(P.canonicalProject(Object.assign({}, live, {markers: []}))), ['version','name','width','height','fps','assets','tracks'], 'no markers, no key');
// Folders: asset folder and has_poster sit at their Go positions, project.folders comes after the 1c keys, and all are omitted when unset.
const owned = Object.assign({}, asset, {folder: 'Trips', has_poster: true});
const loose = Object.assign({}, asset, {id: 'l', name: 'l.mp4'});
const foldered = Object.assign({}, live, {assets: [owned, loose], folders: ['Empty'], markers: [{id: 'm1', frame: 0}], in_point: 0, out_point: 90});
const fc = P.canonicalProject(foldered);
assert.deepEqual(Object.keys(fc.assets[0]), ['id','name','folder','path','kind','duration_frames','duration_us','video_duration_us','width','height','has_audio','has_poster','source_fps','codec','size_bytes'], 'folder follows name and has_poster follows has_audio');
assert.equal(JSON.stringify(fc.assets[0]), JSON.stringify({id:'a',name:'a.mp4',folder:'Trips',path:'media/a',kind:'video',duration_frames:30,duration_us:1000000,video_duration_us:1000000,width:64,height:36,has_audio:true,has_poster:true,source_fps:'30/1',codec:'h264',size_bytes:99}), 'a canonical asset stringifies like the server copy');
assert.deepEqual(Object.keys(fc.assets[1]), ['id','name','path','kind','duration_frames','duration_us','video_duration_us','width','height','has_audio','source_fps','codec','size_bytes'], 'a top-level asset without a poster has neither key');
assert.deepEqual(Object.keys(fc), ['version','name','width','height','fps','assets','tracks','markers','in_point','out_point','folders'], 'folders is the last project key');
assert.deepEqual(fc.folders, ['Empty']);
assert.equal(P.canonicalProject(Object.assign({}, foldered, {folders: []})).folders, undefined, 'no folders, no key');
assert.equal('folders' in P.canonicalProject(Object.assign({}, foldered, {folders: undefined})), false);
assert.equal(P.canonicalProject(Object.assign({}, foldered, {assets: [Object.assign({}, owned, {folder: '', has_poster: false})]})).assets[0].folder, undefined, 'an empty folder and a false poster flag are omitted');
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("canonical form checks: %v\n%s", err, output)
	}
}

// canonicalGuardProject fills every field of Project, Asset, Track and Clip with a non-zero value, so canonicalProject
// must emit every key. Each later slice adds its new fields here with non-zero values (plan index §4.1).
func canonicalGuardProject() videostudio.Project {
	in, out := 10, 90
	return videostudio.Project{
		Version: videostudio.CurrentVersion, Name: "Guard", Width: 1280, Height: 720, FPS: 30, NTSC: true,
		Assets: []videostudio.Asset{{
			ID: "a", Name: "a.mp4", Folder: "Trips", Path: "media/a", Kind: videostudio.AssetVideo, DurationFrames: 300,
			DurationUs: 10_010_000, VideoDurationUs: 10_010_000, Width: 64, Height: 36, HasAudio: true, HasPoster: true,
			SourceFPS: "30000/1001", Codec: "h264", LUTSize: 17, SizeBytes: 99,
		}},
		Tracks: []videostudio.Track{{
			ID: "t", Kind: videostudio.TrackVideo, Name: "Video 1", Muted: true, Hidden: true, Locked: true,
			Clips: []videostudio.Clip{{
				ID: "c", AssetID: "a", Start: 1, Offset: 2, Duration: 30, X: 0.1, Y: 0.2, Width: 0.5, Height: 0.25,
				Rotation: 15, Opacity: 0.5, Volume: 0.8, FadeIn: 3, FadeOut: 4, Fit: videostudio.FitCover, Text: "Hi",
				TextStyle:     &videostudio.TextStyle{FontSize: 32},
				Transition:    &videostudio.Transition{Type: videostudio.TransitionDissolve, Duration: 5},
				Speed:         2,
				Link:          "l1",
				AudioDetached: true,
				Disabled:      true,
				Label:         "red",
				Keyframes:     map[string][]videostudio.Keyframe{"opacity": {{F: 0, V: 0, E: videostudio.EaseLinear}, {F: 25, V: 1, E: videostudio.EaseHold}}, "x": {{F: 3, V: 0.1, E: videostudio.EaseInOut}}},
				Ramp:          []videostudio.RampPoint{{F: 0, Speed: 1, E: videostudio.EaseInOut}, {F: 20, Speed: 2, E: videostudio.EaseLinear}},
				Crop:          &videostudio.Crop{L: 0.1, T: 0.05, R: 0.2, B: 0.15},
				FlipH:         true,
				FlipV:         true,
				Blend:         "multiply",
				Mask:          &videostudio.Mask{Shape: videostudio.MaskEllipse, X: 0.25, Y: 0.2, W: 0.5, H: 0.6, Rotation: 15, Feather: 0.04, Invert: true},
				Effects: []videostudio.EffectInstance{{ID: "color", On: true, Params: map[string]any{"contrast": 1.1, "exposure": 0.5}, Keyframes: map[string][]videostudio.Keyframe{
					"contrast": {{F: 0, V: 1, E: videostudio.EaseLinear}, {F: 12, V: 1.2, E: videostudio.EaseOut}},
					"exposure": {{F: 0, V: 0, E: videostudio.EaseLinear}, {F: 10, V: 1, E: videostudio.EaseHold}},
				}}},
				AnimIn:  &videostudio.ClipAnim{Preset: "pop", Duration: 13},
				AnimOut: &videostudio.ClipAnim{Preset: "slide_left", Duration: 20},
			}},
		}},
		Markers:  []videostudio.Marker{{ID: "m", Frame: 5}},
		InPoint:  &in,
		OutPoint: &out,
		Folders:  []string{"Trips"},
	}
}

// goJSONKeys lists the JSON keys of a struct type in field order, as encoding/json writes them.
func goJSONKeys(typ reflect.Type) []string {
	keys := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		keys = append(keys, name)
	}
	return keys
}

// requirePopulated fails when an exported field of the fixture struct holds its zero value: a field added in Go but
// not in canonicalGuardProject would otherwise escape the key-order check.
func requirePopulated(t *testing.T, label string, value reflect.Value) {
	t.Helper()
	for i := 0; i < value.NumField(); i++ {
		if field := value.Type().Field(i); field.IsExported() && value.Field(i).IsZero() {
			t.Fatalf("%s.%s is empty in canonicalGuardProject; give every new field a non-zero value", label, field.Name)
		}
	}
}

// The project PUT decodes with DisallowUnknownFields and the editor compares canonical copies as strings, so
// canonicalProject must emit exactly Go's JSON keys in Go's struct order (plan index §4.1).
func TestVideoStudioCanonicalKeyOrderFollowsGo(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio canonical form checks")
	}
	fixture := canonicalGuardProject()
	requirePopulated(t, "Project", reflect.ValueOf(fixture))
	requirePopulated(t, "Asset", reflect.ValueOf(fixture.Assets[0]))
	requirePopulated(t, "Track", reflect.ValueOf(fixture.Tracks[0]))
	requirePopulated(t, "Clip", reflect.ValueOf(fixture.Tracks[0].Clips[0]))
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	script := `
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-core.js');
const c = window.VideoStudioParts.canonicalProject(` + string(data) + `);
process.stdout.write(JSON.stringify({project: Object.keys(c), asset: Object.keys(c.assets[0]), track: Object.keys(c.tracks[0]), clip: Object.keys(c.tracks[0].clips[0])}));
`
	cmd := exec.Command(node, "-e", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("canonical key order: %v\n%s", err, stderr.String())
	}
	var got struct{ Project, Asset, Track, Clip []string }
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("decode %s: %v", output, err)
	}
	for _, check := range []struct {
		name string
		typ  reflect.Type
		keys []string
	}{
		{"Project", reflect.TypeOf(videostudio.Project{}), got.Project},
		{"Asset", reflect.TypeOf(videostudio.Asset{}), got.Asset},
		{"Track", reflect.TypeOf(videostudio.Track{}), got.Track},
		{"Clip", reflect.TypeOf(videostudio.Clip{}), got.Clip},
	} {
		if want := goJSONKeys(check.typ); !slices.Equal(check.keys, want) {
			t.Fatalf("canonicalProject %s keys = %v, want the Go JSON order %v", check.name, check.keys, want)
		}
	}
}

// Keyframes and ramps reach the server in Go's JSON form (plan index §4.1). The guard project's canonical copy
// stringifies exactly like json.Marshal, even when the live editor state differs: paths in insertion order, keys in
// another order, unsorted lists, an emptied path and editor-only keys.
func TestVideoStudioCanonicalKeyframesMatchGo(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio canonical form checks")
	}
	data, err := json.Marshal(canonicalGuardProject())
	if err != nil {
		t.Fatal(err)
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-core.js');
const P = window.VideoStudioParts;
const live = ` + string(data) + `;
const clip = live.tracks[0].clips[0];
clip.keyframes = {
  x: clip.keyframes.x.map(k => ({e: k.e, selected: true, v: k.v, f: k.f})),
  rotation: [],
  opacity: clip.keyframes.opacity.slice().reverse().map(k => ({v: k.v, f: k.f, e: k.e}))
};
clip.ramp = clip.ramp.slice().reverse().map(p => ({speed: p.speed, e: p.e, f: p.f, dragging: false}));
const bare = JSON.parse(JSON.stringify(live));
bare.tracks[0].clips[0].keyframes = {opacity: []};
bare.tracks[0].clips[0].ramp = [];
const bareKeys = Object.keys(P.canonicalProject(bare).tracks[0].clips[0]);
assert(!bareKeys.includes('keyframes') && !bareKeys.includes('ramp'), 'empty keyframe lists and an empty ramp are omitted like Go omitempty');
process.stdout.write(JSON.stringify(P.canonicalProject(live)));
`
	cmd := exec.Command(node, "-e", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("canonical keyframes: %v\n%s", err, stderr.String())
	}
	if string(output) != string(data) {
		t.Fatalf("canonicalProject differs from Go's JSON:\n JS %s\n Go %s", output, data)
	}
}

// Crop, flips and effects reach the server in Go's JSON form (plan 2c, index §4.1). The guard project's canonical copy
// stringifies exactly like json.Marshal, even when the live editor state differs:
//   - crop keys in another order, plus an editor-only key;
//   - params and keyframe paths in insertion order;
//   - unsorted keys and an emptied keyframe list;
//   - editor-only keys on the entry.
//
// A zero crop, false flips, an empty stack and a missing params object take Go's omitempty form.
func TestVideoStudioCanonicalEffectsMatchGo(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio canonical form checks")
	}
	data, err := json.Marshal(canonicalGuardProject())
	if err != nil {
		t.Fatal(err)
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-core.js');
const P = window.VideoStudioParts;
const live = ` + string(data) + `;
const clip = live.tracks[0].clips[0];
clip.crop = {b: clip.crop.b, dragging: true, r: clip.crop.r, t: clip.crop.t, l: clip.crop.l};
clip.effects = clip.effects.map(entry => {
  const keyframes = {gamma: []};
  Object.keys(entry.keyframes).reverse().forEach(path => { keyframes[path] = entry.keyframes[path].slice().reverse().map(k => ({e: k.e, selected: true, v: k.v, f: k.f})); });
  return {open: true, keyframes, params: Object.fromEntries(Object.entries(entry.params).reverse()), on: entry.on, id: entry.id};
});
const bare = JSON.parse(JSON.stringify(live)), bareClip = bare.tracks[0].clips[0];
Object.assign(bareClip, {crop: {l: 0, t: 0, r: 0, b: 0}, flip_h: false, flip_v: false, effects: []});
const bareKeys = Object.keys(P.canonicalProject(bare).tracks[0].clips[0]);
for (const key of ['crop', 'flip_h', 'flip_v', 'effects']) assert(!bareKeys.includes(key), key + ' is omitted like Go omitempty');
bareClip.effects = [{id: 'blur', on: false}];
assert.equal(JSON.stringify(P.canonicalProject(bare).tracks[0].clips[0].effects), '[{"id":"blur","on":false,"params":{}}]', 'params is always an object; keyframes only when there are some');
process.stdout.write(JSON.stringify(P.canonicalProject(live)));
`
	cmd := exec.Command(node, "-e", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("canonical effects: %v\n%s", err, stderr.String())
	}
	if string(output) != string(data) {
		t.Fatalf("canonicalProject differs from Go's JSON:\n JS %s\n Go %s", output, data)
	}
}

// Blend and mask reach the server in Go's JSON form (plan 2d, index §4.1). The guard project's canonical copy
// stringifies exactly like json.Marshal, even when the live mask holds its keys in another order plus an editor-only
// key. Blend normal (or an empty or null name) and a missing mask take Go's omitempty form; a mask always carries all
// eight keys, invert too, and a missing number reads 0 as Go decodes it.
func TestVideoStudioCanonicalCompositingMatchGo(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio canonical form checks")
	}
	data, err := json.Marshal(canonicalGuardProject())
	if err != nil {
		t.Fatal(err)
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-core.js');
const P = window.VideoStudioParts;
const live = ` + string(data) + `;
const clip = live.tracks[0].clips[0], m = clip.mask;
clip.mask = {invert: m.invert, dragging: true, feather: m.feather, rotation: m.rotation, h: m.h, w: m.w, y: m.y, x: m.x, shape: m.shape};
const bare = JSON.parse(JSON.stringify(live)), bareClip = bare.tracks[0].clips[0];
for (const blend of ['normal', '', null]) {
  Object.assign(bareClip, {blend, mask: null});
  const keys = Object.keys(P.canonicalProject(bare).tracks[0].clips[0]);
  assert(!keys.includes('blend') && !keys.includes('mask'), JSON.stringify(blend) + ': blend and mask are omitted like Go omitempty');
}
Object.assign(bareClip, {blend: 'screen', mask: {shape: 'rect', x: 0.1, y: 0.2, w: 0.3, h: 0.4}});
assert.equal(JSON.stringify(P.canonicalProject(bare).tracks[0].clips[0].mask), '{"shape":"rect","x":0.1,"y":0.2,"w":0.3,"h":0.4,"rotation":0,"feather":0,"invert":false}');
const keys = Object.keys(P.canonicalProject(bare).tracks[0].clips[0]);
assert.deepEqual(keys.slice(keys.indexOf('flip_v'), keys.indexOf('effects') + 1), ['flip_v', 'blend', 'mask', 'effects'], 'blend and mask sit between flip_v and effects');
process.stdout.write(JSON.stringify(P.canonicalProject(live)));
`
	cmd := exec.Command(node, "-e", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("canonical compositing: %v\n%s", err, stderr.String())
	}
	if string(output) != string(data) {
		t.Fatalf("canonicalProject differs from Go's JSON:\n JS %s\n Go %s", output, data)
	}
}

// In and out animations reach the server in Go's JSON form (plan 2e, index §4.1): {preset, duration} in that order,
// after effects, each only when set. The guard project's canonical copy stringifies exactly like json.Marshal, even
// when the live objects hold their keys in another order plus an editor-only key, and a fractional duration rounds.
// A missing, null or non-object animation is omitted like Go's omitempty.
func TestVideoStudioCanonicalAnimMatchGo(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio canonical form checks")
	}
	data, err := json.Marshal(canonicalGuardProject())
	if err != nil {
		t.Fatal(err)
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-core.js');
const P = window.VideoStudioParts;
const live = ` + string(data) + `;
const clip = live.tracks[0].clips[0];
clip.anim_in = {open: true, duration: clip.anim_in.duration - 0.4, preset: clip.anim_in.preset};
clip.anim_out = {duration: clip.anim_out.duration, preset: clip.anim_out.preset};
const bare = JSON.parse(JSON.stringify(live)), bareClip = bare.tracks[0].clips[0];
for (const value of [null, undefined, 'pop', [1]]) {
  Object.assign(bareClip, {anim_in: value, anim_out: value});
  const keys = Object.keys(P.canonicalProject(bare).tracks[0].clips[0]);
  assert(!keys.includes('anim_in') && !keys.includes('anim_out'), String(JSON.stringify(value)) + ': no animation is omitted like Go omitempty');
}
Object.assign(bareClip, {anim_in: null, anim_out: {preset: 'fade', duration: 10}});
const keys = Object.keys(P.canonicalProject(bare).tracks[0].clips[0]);
assert.deepEqual(keys.slice(-2), ['effects', 'anim_out'], 'anim_out alone follows effects');
assert.equal(JSON.stringify(P.canonicalProject(bare).tracks[0].clips[0].anim_out), '{"preset":"fade","duration":10}');
process.stdout.write(JSON.stringify(P.canonicalProject(live)));
`
	cmd := exec.Command(node, "-e", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("canonical animation: %v\n%s", err, stderr.String())
	}
	if string(output) != string(data) {
		t.Fatalf("canonicalProject differs from Go's JSON:\n JS %s\n Go %s", output, data)
	}
}
