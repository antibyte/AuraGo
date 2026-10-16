package ui

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const videoStudioNavPrelude = `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-navigation.js');
const T = window.VideoStudioTimeline, N = window.VideoStudioNavigation, Time = window.VideoStudioTime;
const box = {x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fit: 'contain'};
`

func runVideoStudioNavScript(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio navigation checks")
	}
	if output, err := exec.Command(node, "-e", videoStudioNavPrelude+script).CombinedOutput(); err != nil {
		t.Fatalf("navigation checks: %v\n%s", err, output)
	}
}

func TestVideoStudioRescaleKeepsMarkersAndInOut(t *testing.T) {
	runVideoStudioNavScript(t, `
assert.equal(Time.frameAtRate(1000, {fps: 30}, {fps: 25}), 833, 'the same time at the new rate, rounded');
assert.equal(Time.frameAtRate(833, {fps: 25}, {fps: 30}), 1000, 'undo maps the playhead back');
assert.equal(Time.frameAtRate(100, {fps: 24}, {fps: 24, ntsc: true}), 100);
assert.equal(Time.clockSeconds(3725), '1:02:05', 'clock text carries hours');
assert.equal(Time.clockSeconds(75), '1:15');
// Markers and in/out follow the clips when the rate changes.
const project = {fps: 30, assets: [{id: 'v', kind: 'video', duration_frames: 300}], tracks: [{id: 't', kind: 'video', clips: [
  {...box, id: 'c', asset_id: 'v', start: 0, offset: 0, duration: 300, transition: null}]}],
  markers: [{id: 'm', frame: 31}, {id: 'r', frame: 10, duration: 15, name: 'range'}], in_point: 1, out_point: 31};
Time.rescaleProject(project, 25, false);
assert.deepEqual(project.markers.map(m => [m.id, m.frame, m.duration || 0]), [['m', 26, 0], ['r', 8, 13]], 'the array keeps its order; the app sorts on save');
assert.deepEqual([project.in_point, project.out_point], [1, 26]);
// A range keeps at least one frame; an out point that rounds onto the in point is dropped.
const tiny = {fps: 60, assets: [], tracks: [], markers: [{id: 'z', frame: 2, duration: 1}], in_point: 2, out_point: 3};
Time.rescaleProject(tiny, 24, false);
assert.equal(tiny.markers[0].duration, 1);
assert.equal(tiny.in_point, 1);
assert.equal('out_point' in tiny, false, 'a collapsed out point is dropped');
// A 60-minute project cannot change to 23.976 fps: its last frame would be past the limit. Refuse, do not clamp.
const long = {fps: 24, assets: [{id: 'img', kind: 'image'}], tracks: [{id: 't', kind: 'video', clips: [
  {...box, id: 'c', asset_id: 'img', start: 0, offset: 0, duration: 86400, transition: null}]}]};
assert.equal(Time.rescaleFits(long, 24, true), false, 'one frame past the 60-minute limit');
assert.equal(Time.rescaleFits(long, 24, false), true, 'the same rate always fits');
assert.equal(Time.rescaleFits(long, 25, false), true, '3600 s is exactly 90000 frames at 25 fps and fits');
`)
}

func TestVideoStudioNavigationHelpers(t *testing.T) {
	runVideoStudioNavScript(t, `
const log = [];
let frame = 100, rate = 1, playing = false;
const shuttle = N.createShuttle({
  getFrame: () => frame,
  seek: value => { frame = value; },
  play: () => { playing = true; },
  pause: () => { playing = false; },
  setRate: value => { rate = value; },
  maxFrame: () => 500
});
shuttle.press('l'); assert.equal(rate, 1); assert.equal(playing, true);
shuttle.press('l'); assert.equal(rate, 2);
shuttle.press('l'); assert.equal(rate, 4);
shuttle.press('l'); shuttle.press('l'); assert.equal(rate, 8, 'each press doubles the rate, up to 8x');
shuttle.press('j'); assert.equal(rate, -1, 'a direction change restarts at 1x backward');
shuttle.press('j'); assert.equal(rate, -2);
shuttle.press('k'); assert.equal(playing, false, 'K stops');
assert.equal(rate, 1, 'stopping returns the preview to 1x');
assert.equal(shuttle.rate(), 0);
shuttle.press('k');
shuttle.press('l'); assert.equal(frame, 101, 'K held with L steps one frame');
assert.equal(playing, false, 'a step does not play');
shuttle.press('j'); assert.equal(frame, 100, 'K held with J steps back');
shuttle.release('k');
shuttle.press('l'); assert.equal(rate, 1, 'released K shuttles again'); assert.equal(playing, true);
shuttle.reset(); assert.equal(shuttle.rate(), 0, 'reset forgets the rate without calling the preview');
shuttle.press('k'); frame = 500; shuttle.press('l'); assert.equal(frame, 500, 'steps stop at the timeline end');
shuttle.release('k');
assert.equal(shuttle.press('x'), null, 'other keys are not the shuttle');

const p = {tracks: [
  {id: 'a', kind: 'video', hidden: false, locked: false, clips: [{start: 30, duration: 40}]},
  {id: 'b', kind: 'video', hidden: false, locked: true, clips: [{start: 50, duration: 10}]},
  {id: 'c', kind: 'overlay', hidden: true, locked: false, clips: [{start: 5, duration: 5}]}]};
assert.equal(N.editPoints(p, 0, 1), 30, 'unlocked, visible tracks only');
assert.equal(N.editPoints(p, 30, 1), 70);
assert.equal(N.editPoints(p, 70, 1), null);
assert.equal(N.editPoints(p, 40, -1), 30);
assert.equal(N.editPoints(p, 30, -1), 0, 'frame 0 counts as an edit point');
assert.equal(N.markerStep([10, 20, 20, 35], 20, 1), 35, 'the next marker strictly after the frame');
assert.equal(N.markerStep([10, 20, 20, 35], 20, -1), 10);
assert.equal(N.markerStep([10], 10, 1), null);
assert.deepEqual(N.loopRange({in: 30, out: 90}, 300), {start: 30, end: 90});
assert.deepEqual(N.loopRange({in: null, out: null}, 300), {start: 0, end: 300}, 'no points loops the whole timeline');
assert.equal(N.loopRange({in: 100, out: 50}, 300), null, 'an empty range is refused');
assert.deepEqual(N.overviewBox(1000, 250, 400, 200), {left: 50, width: 80});
assert.deepEqual(N.overviewBox(1000, 900, 400, 200), {left: 120, width: 80}, 'the window stays on the strip');
assert.equal(N.laneForStrip(60, 1000, 200, 400), 300);
assert.equal(N.laneForStrip(199, 1000, 200, 400), 600, 'clamped to the last full view');
assert.equal(N.zoomForSpan(10, 520, {min: 0.005, max: 2.8, basePx: 52}), 1);
assert.equal(N.zoomForSpan(1e9, 520, {min: 0.005, max: 2.8, basePx: 52}), 0.005);

const tracks = [{id: 'v1', kind: 'video'}, {id: 'o1', kind: 'overlay'}, {id: 'a1', kind: 'audio'}];
assert.equal(N.trackInsertIndex(tracks, 'v1', 'above'), 1, 'visual tracks list front-most first: above is after it');
assert.equal(N.trackInsertIndex(tracks, 'v1', 'below'), 0);
assert.equal(N.trackInsertIndex(tracks, 'a1', 'above'), 2, 'audio lists in array order: above is before it');
assert.equal(N.trackInsertIndex(tracks, 'a1', 'below'), 3);
assert.equal(N.trackInsertIndex(tracks, 'missing', 'above'), 3);
assert.deepEqual(['video', 'audio'].map(kind => N.arraySideFor(kind, 'above')), ['after', 'before']);
const four = [{id: 'v1', kind: 'video'}, {id: 'v2', kind: 'video'}, {id: 'o1', kind: 'overlay'}, {id: 'v3', kind: 'video'}];
assert.deepEqual(N.moveWithinKind(four, 'v1', 'v3', 'after').map(track => track.id), ['v2', 'o1', 'v3', 'v1']);
assert.equal(N.moveWithinKind(four, 'v1', 'o1', 'after'), null, 'a different kind does not move');
assert.equal(N.moveWithinKind(four, 'v1', 'v1', 'after'), null, 'the same track does not move');
assert.equal(N.moveWithinKind(four, 'v1', 'v2', 'before'), null, 'dropping a track just before the one it already precedes changes nothing');
assert.equal(N.moveWithinKind(four, 'v2', 'v1', 'after'), null, 'dropping a track just after the one it already follows changes nothing');
assert.equal(N.moveWithinKind(four, 'v1', 'v3', 'before').map(track => track.id).join(), 'v2,o1,v1,v3', 'a real move keeps the other tracks in order');
assert.deepEqual(four.map(track => track.id), ['v1', 'v2', 'o1', 'v3'], 'the input list is never changed');
`)
}

func TestVideoStudioReviewFixesWiring(t *testing.T) {
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join("js", "desktop", "apps", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return string(data)
	}
	app := read("video-studio.js")
	// Tab: dialogs first, then an open marker popover (it lives outside the drawer), then the narrow inspector drawer.
	trapAt := strings.Index(app, "const trap = ")
	if trapAt < 0 {
		t.Fatal("the app shell has no Tab trap")
	}
	trap := app[trapAt : trapAt+strings.Index(app[trapAt:], "\n")]
	last := -1
	for _, want := range []string{"[data-conflict-host] .vs-modal", "[data-modal-host] .vs-modal", "s.markerPop", ".vs-inspector"} {
		at := strings.Index(trap, want)
		if at < 0 || at < last {
			t.Fatalf("the Tab trap does not check %q in order: %s", want, trap)
		}
		last = at
	}
	// The time button takes its name from its content (the time), so it carries no fixed aria-label.
	button := regexp.MustCompile(`<button[^>]*data-action="goto-time"[^>]*>`).FindString(app)
	if button == "" || strings.Contains(button, "aria-label") || !strings.Contains(app, `class="vs-sr-only"> – `) {
		t.Errorf("the time button must be named by its content plus a visually hidden hint: %s", button)
	}
	if !strings.Contains(app, `<div class="vs-time" aria-live="off"><button type="button" class="vs-time-button"`) {
		t.Error("the time display must stay a silent live region")
	}
	inspector := read("video-studio-inspector.js")
	for _, want := range []string{`data-action="mark-in"`, `data-action="mark-out"`, "v.tr('setIn'", "v.tr('setOut'"} {
		if !strings.Contains(inspector, want) {
			t.Errorf("the inspector's in/out row lacks %s", want)
		}
	}
	timeline := read("video-studio-timeline.js")
	press := timeline[strings.Index(timeline, "function pointerDown(event)"):]
	if commit, drag := strings.Index(press, "commitOpenRename(event)"), strings.Index(press, "timelineDragging = true"); commit < 0 || drag < 0 || commit > drag {
		t.Error("a press must commit an open track rename before any gesture state is set")
	}
	// Edits that change nothing return before mutate.
	for file, wants := range map[string][]string{
		"video-studio-transport.js": {"if (io[which] === frame) return false;"},
		"video-studio-markers.js":   {"marker.frame === io.in && (marker.duration || 0) === io.out - io.in", "marker.frame === clamp(Math.round(s.frame), 0, maxFrames(s))"},
		"video-studio-tracks.js":    {"if (!Nav().moveWithinKind(s.project.tracks, fromId, toId, side)) return false;"},
	} {
		src := read(file)
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Errorf("%s does not skip the no-op edit (%s)", file, want)
			}
		}
	}
	// Every locale carries the two new button labels.
	files, err := filepath.Glob(filepath.Join("lang", "desktop", "*.json"))
	if err != nil || len(files) != 16 {
		t.Fatalf("desktop locales: %d files, %v", len(files), err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{`"videoStudio.setIn"`, `"videoStudio.setOut"`} {
			if !bytes.Contains(data, []byte(key)) {
				t.Errorf("%s lacks %s", file, key)
			}
		}
	}
}

func TestVideoStudioMarkerValidation(t *testing.T) {
	runVideoStudioNavScript(t, `
const base = {fps: 30, assets: [{id: 'v', kind: 'video', duration_frames: 300, width: 1, height: 1}], tracks: []};
const ok = extra => T.validTimeline(Object.assign({}, base, extra));
assert(ok({markers: [{id: 'm', frame: 10}], in_point: 5, out_point: 50}), 'valid markers and in/out');
assert(ok({markers: [{id: 'm', frame: 10, duration: 20, color: 'teal', name: 'x', note: 'y'}]}));
assert.equal(ok({markers: [{id: 'm', frame: -1}]}), false);
assert.equal(ok({markers: [{id: 'm', frame: 10}, {id: 'm', frame: 12}]}), false, 'duplicate ids');
assert.equal(ok({markers: [{id: 'm', frame: 10, color: 'mauve'}]}), false, 'colours come from the palette');
assert.equal(ok({markers: [{id: 'm', frame: 108000, duration: 1}]}), false, 'a range past the limit');
assert.equal(ok({in_point: 50, out_point: 50}), false, 'in must be before out');
assert.equal(ok({out_point: 0}), false, 'out starts at frame 1');
assert.equal(ok({in_point: 0}), true, 'an in point at 0 is valid');
assert.equal(ok({markers: Array.from({length: 501}, (_, i) => ({id: 'm' + i, frame: i}))}), false, 'at most 500 markers');
assert.deepEqual(T.LABEL_COLORS, ['red', 'orange', 'yellow', 'green', 'teal', 'blue', 'purple', 'pink']);
`)
}

func TestVideoStudioPreviewKeepsSpeedAndDisabledRules(t *testing.T) {
	preview, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-preview.js"))
	if err != nil {
		t.Fatalf("preview part: %v", err)
	}
	for _, want := range []string{
		"function speedOf(clip) { const speed = Number(clip.speed); return speed > 0 ? speed : 1; }",
		"return !clip.disabled && at >= Number(clip.start || 0)",
		"(Number(clip.offset || 0) + (at - Number(clip.start || 0)) * speedOf(clip)) / fps()",
		"if (index <= 0 || sorted[index - 1].disabled) return null;",
		"if (!after || after.disabled) return null;",
		"near = !current && !clip.disabled && clip.start > at",
		"!clip.audio_detached && speedOf(clip) <= 4 && soloed && !shuttleMuted()",
		"const playRate = clamp(speedOf(clip) * (forwardPlayback ? rate : 1), 0.0625, 16);",
		"flatMap(track => track.clips.filter(clip => !clip.disabled).map(clip => clip.start + clip.duration))",
	} {
		if !bytes.Contains(preview, []byte(want)) {
			t.Errorf("preview part lost a 1b rule: %s", want)
		}
	}
}

func TestVideoStudioRulerMarkup(t *testing.T) {
	runVideoStudioNavScript(t, `
require('./js/desktop/apps/video-studio-ruler.js');
const R = window.VideoStudioRuler;
assert.equal(typeof R.markup, 'function'); assert.equal(typeof R.overviewInner, 'function'); assert.equal(typeof R.syncOverview, 'function'); assert.equal(typeof R.attachOverview, 'function');
assert.equal(typeof R.markerDrag, 'function'); assert.equal(typeof R.markerMove, 'function');
const project = {fps: 30, markers: [{id: 'm', frame: 30, duration: 0, name: '', color: 'teal'}, {id: 'r', frame: 60, duration: 30, color: 'mauve'}], in_point: 30, out_point: 90};
const options = {esc: value => String(value), label: (key, fallback) => fallback};
const html = R.markup(options, project, 52, 1000);
assert(html.includes('class="vs-range-shade"') && html.includes('data-marker-id="m"') && html.includes('--vs-marker-color:var(--vs-label-teal)'));
assert(html.includes('is-range') && html.includes('vs-marker-span'), 'a range marker draws its span');
assert(!html.includes('mauve'), 'a colour outside the palette is not drawn');
const inner = R.overviewInner(options, project, 52, 1000);
assert(inner.includes('vs-overview-shade') && inner.includes('vs-overview-mark'));
`)
}

func TestVideoStudioNavigationWiring(t *testing.T) {
	transport, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-transport.js"))
	if err != nil {
		t.Fatalf("transport part: %v", err)
	}
	app, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio.js"))
	if err != nil {
		t.Fatalf("app shell: %v", err)
	}
	for _, want := range []string{"handleNavKeys", "createShuttle", "togglePlay", "soloTrackIds", "snapExtras"} {
		if !bytes.Contains(transport, []byte(want)) {
			t.Errorf("transport part does not export %s", want)
		}
	}
	for _, want := range []string{"P.handleNavKeys(s, event)", "P.isNavShortcut(s, event)", "P.createShuttle({", "P.togglePlay(s)", `data-action="goto-time"`, `data-action="loop"`, "P.syncTransport(s)", "P.snapExtras(s, excludeMarkerId)"} {
		if !bytes.Contains(app, []byte(want)) {
			t.Errorf("app shell does not wire %s", want)
		}
	}
}

func TestVideoStudioMarkerWiring(t *testing.T) {
	markers, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-markers.js"))
	if err != nil {
		t.Fatalf("marker part: %v", err)
	}
	for _, want := range []string{"addMarkerAtPlayhead", "openMarkerPopover", "openMarkerMenu", "pointOf", "updateMarker", "markerRangeFromInOut", "closeMarkerPopover", "syncMarkerPopover"} {
		if !bytes.Contains(markers, []byte(want)) {
			t.Errorf("marker part does not define %s", want)
		}
	}
	inspector, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-inspector.js"))
	if err != nil {
		t.Fatalf("inspector: %v", err)
	}
	if !bytes.Contains(inspector, []byte("data-marker-jump")) || !bytes.Contains(inspector, []byte(`step="1" value`)) {
		t.Error("inspector lacks the marker list or the one-frame transition step")
	}
	app, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio.js"))
	if err != nil {
		t.Fatalf("app shell: %v", err)
	}
	if !bytes.Contains(app, []byte("P.openMarkerMenu(s, id, x, y)")) || !bytes.Contains(app, []byte("P.jumpToMarker(s, id)")) {
		t.Error("the app shell does not wire the marker callbacks")
	}
}

func TestVideoStudioTrackWiring(t *testing.T) {
	tracks, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-tracks.js"))
	if err != nil {
		t.Fatalf("track part: %v", err)
	}
	for _, want := range []string{"targetTracks", "pasteTargets", "trackHeight", "setTrackHeight", "toggleTarget", "toggleSolo", "trackState", "renameTrack", "addTrackBeside", "deleteTrack", "moveTrack", "openTrackMenu", "runTrackAction"} {
		if !bytes.Contains(tracks, []byte(want)) {
			t.Errorf("track part does not define %s", want)
		}
	}
	app, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio.js"))
	if err != nil {
		t.Fatalf("app shell: %v", err)
	}
	for _, want := range []string{"P.runTrackAction(s, id, action, element)", "P.moveTrack(s, from, to, side)", "P.trackState(s, track)", "P.renameTrack(s, id, value)", "P.openTrackMenu(s, id, x, y)"} {
		if !bytes.Contains(app, []byte(want)) {
			t.Errorf("app shell does not wire %s", want)
		}
	}
	editing, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-editing.js"))
	if err != nil {
		t.Fatalf("editing part: %v", err)
	}
	if !bytes.Contains(editing, []byte("targets: P.pasteTargets(s)")) {
		t.Error("paste does not pass the target tracks")
	}
}

func TestVideoStudioRateWiring(t *testing.T) {
	projects, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-projects.js"))
	if err != nil {
		t.Fatalf("projects part: %v", err)
	}
	for _, want := range []string{"Time().rescaleFits(s.project, next.fps, next.ntsc)", "rateChangeLimit", "Time().frameAtRate(s.frame, before, next)"} {
		if !bytes.Contains(projects, []byte(want)) {
			t.Errorf("settings do not use %s", want)
		}
	}
	core, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-core.js"))
	if err != nil {
		t.Fatalf("core: %v", err)
	}
	if !bytes.Contains(core, []byte("frameAtRate(s.frame, previous, s.project)")) {
		t.Error("undo and redo do not keep the playhead's time")
	}
	jobs, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-jobs.js"))
	if err != nil {
		t.Fatalf("jobs: %v", err)
	}
	if !bytes.Contains(jobs, []byte("VideoStudioTime.clockSeconds")) {
		t.Error("failure ranges do not use the hour-aware clock text")
	}
}

func TestVideoStudioNavigationLoadOrder(t *testing.T) {
	loader, err := os.ReadFile(filepath.Join("js", "desktop", "core", "module-loader.js"))
	if err != nil {
		t.Fatalf("module loader: %v", err)
	}
	fixture, err := os.ReadFile(filepath.Join("desktop_video_studio_browser_test.go"))
	if err != nil {
		t.Fatalf("browser fixture: %v", err)
	}
	server, err := os.ReadFile(filepath.Join("..", "internal", "server", "video_studio_browser_test.go"))
	if err != nil {
		t.Fatalf("server browser fixture: %v", err)
	}
	order := []string{"video-studio-timeline.js", "video-studio-ruler.js", "video-studio-edit.js", "video-studio-navigation.js", "video-studio-editing.js", "video-studio-tracks.js", "video-studio-markers.js", "video-studio-transport.js", "video-studio-derive.js", "video-studio-projects.js", "video-studio-menus.js", "video-studio.js"}
	for name, source := range map[string][]byte{"module loader": loader, "ui browser fixture": fixture, "server browser fixture": server} {
		last := -1
		for _, file := range order {
			index := bytes.Index(source, []byte(file))
			if index < 0 {
				t.Errorf("%s does not load %s", name, file)
				continue
			}
			if index < last {
				t.Errorf("%s loads %s before the parts that come before it", name, file)
			}
			last = index
		}
	}
}
