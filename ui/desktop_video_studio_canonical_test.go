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
			SourceFPS: "30000/1001", Codec: "h264", SizeBytes: 99,
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
