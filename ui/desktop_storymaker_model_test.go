package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// storymakerCode removes the comments of a script, so that a guard sees only what runs.
func storymakerCode(source string) string {
	blocks := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(source, "")
	return regexp.MustCompile(`(?m)(^|\s)//[^\n]*$`).ReplaceAllString(blocks, "")
}

// The iframe of the player: sandbox allow-scripts only, no permission delegation, messages only from its own window, a fixed
// schema with caps, posts to '*' because the origin is opaque, and no requests of its own.
func TestStorymakerPlayerBridgeIsSandboxed(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/storymaker-player.js")
	for _, marker := range []string{
		"sandbox: 'allow-scripts'",
		"iframe.setAttribute('sandbox', 'allow-scripts');",
		"referrerpolicy: 'no-referrer'",
		"event.source !== iframe.contentWindow",
		"target.postMessage(Object.assign({ sm: 1 }, message), '*', transfer || []);",
		"const msg = checkIncoming(event.data);",
		"msg.sm !== 1",
		"LIMITS.keys",
		"LIMITS.scenes",
	} {
		if !strings.Contains(source, marker) {
			t.Errorf("storymaker-player.js lacks %q", marker)
		}
	}
	code := storymakerCode(source)
	for _, forbidden := range []string{"allow-same-origin", "allow-forms", "allow-modals", "allow-popups", "allow-top-navigation", "'allow',", "allow:"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("storymaker-player.js must not contain %q", forbidden)
		}
	}
	if strings.Contains(code, "fetch(") || strings.Contains(code, "XMLHttpRequest") {
		t.Error("the bridge must not make requests itself: the parent serves the assets through the resolver")
	}
}

// The pure part of the app in Node: pointer walking in JSON text, issue lists, the catalog index, story edits and the message schema.
func TestStorymakerAppModelInNode(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-app-model.js")
	if err := os.WriteFile(script, []byte(storymakerModelScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	stories, _ := filepath.Abs(filepath.Join("..", "internal", "storymaker", "testdata", "stories", "firewall.json"))
	catalog, _ := filepath.Abs(filepath.Join("img", "storymaker", "catalog.json"))
	out, err := exec.Command(node, script, dir, stories, catalog).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker app model ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerModelScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir, storyFile, catalogFile] = process.argv.slice(2);
class FakeBitmap { constructor() { this.width = 1; this.height = 1; } }
const context = { console, ImageBitmap: FakeBitmap };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-model.js', 'storymaker-player.js']) {
  // the player module reads S.el only inside createPlayer, so the pure parts load without a DOM
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts, M = S.model;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const story = JSON.parse(fs.readFileSync(storyFile, 'utf8'));
const text = M.canonicalText(story);

// locate: a pointer finds its value, a missing field falls back to the deepest value that exists
const hit = M.locate(text, '/scenes/0/actors/0/at');
eq([hit.found, text.slice(hit.from, hit.to)], [true, '"desk"'], 'pointer to a string');
eq(M.locate(text, '/title').found, true, 'title');
const missing = M.locate(text, '/scenes/1/beats/0/expression');
eq([missing.found, missing.depth], [false, 4], 'a missing field gives the beat');
eq(M.locate(text, '/scenes/9/id').found, false, 'index out of range');
eq(M.parsePointer('/a~1b/c~0d'), ['a/b', 'c~d'], 'pointer escapes');
eq(M.lineColumn(text, M.locate(text, '/title').from), { line: 3, column: 12 }, 'line and column');
eq(M.positionFromMessage('invalid character at line 3, column 14'), { line: 3, column: 14 }, 'position from a message');
eq(M.offsetOf('ab\ncd\nef', 2, 2), 4, 'offset');
// a quote, a brace and an escape inside strings must not confuse the walker
const tricky = '{"a": "x\\"}{", "b": [1, {"c": "]"}], "d": 5}';
eq(tricky.slice(M.locate(tricky, '/d').from, M.locate(tricky, '/d').to), '5', 'string with quotes and braces');
eq(tricky.slice(M.locate(tricky, '/b/1/c').from, M.locate(tricky, '/b/1/c').to), '"]"', 'nested pointer');

// issues
const merged = M.mergeIssues([{ severity: 'minor', code: 'a', path: '/x', message: 'm' }, { severity: 'error', code: 'b', path: '', message: 'e' }], [{ severity: 'minor', code: 'a', path: '/x', message: 'm' }]);
eq(merged.map(i => i.severity), ['error', 'minor'], 'errors first, duplicates gone');
eq(M.visibleIssues([{ code: 'narration_missing' }, { code: 'x' }]).length, 1, 'narration_missing is hidden');
eq(M.sceneIndexOfIssue({ path: '/scenes/2/beats/0' }, story), 2, 'scene of an issue by path');
eq(M.sceneIndexOfIssue({ scene_id: 's2', path: '' }, story), 1, 'scene of an issue by id');

// catalog and edits
const catalog = M.indexCatalog(JSON.parse(fs.readFileSync(catalogFile, 'utf8')));
eq(M.entriesOf(catalog, 'character').length, 4, 'four characters');
const draft = M.cloneStory(story);
eq(M.addScene(draft, 0, {}), 1, 'a scene after the first');
eq(draft.scenes.map(s => s.id), ['s1', 's4', 's2', 's3'], 'scene ids');
eq(draft.scenes[1].environment, 'env.office', 'a new scene keeps the environment');
const code = M.addScene(draft, 3, { kind: 'code' });
eq([draft.scenes[code].kind, draft.scenes[code].id], ['code', 'code5'], 'a code scene');
eq(M.removeScene(draft, 0), 0, 'remove');
eq(M.moveScene(draft, 0, 2), 2, 'move');
const man = catalog.byId['char.office-man-1'];
eq(M.insertActor(draft, 0, man, 'Office man').ok, true, 'first insert');
eq(M.insertActor(draft, 0, man, 'Office man').reason, 'already_in_scene', 'the same person twice');
eq(draft.cast.filter(c => c.character === 'char.office-man-1').length, 1, 'one cast member');
const props = M.insertProp(draft, 0, catalog.byId['prop.laptop'], catalog.byId['env.office']);
eq([props.ok, draft.scenes[0].props[0].at], [true, 'right'], 'a prop takes a free spot');
eq(M.insertActor(draft, M.scenesOf(draft).length - 1, man, 'x').ok, false, 'no actors in a code scene');
const one = { schema: 'storymaker/1', scenes: [{ id: 's1' }] };
eq(M.removeScene(one, 0), -1, 'the last scene stays');
eq(one.scenes[M.addScene(one, 0, { kind: 'code', taken: ['code2'] })].id, 'code3', 'a code file that outlived its scene keeps its id');
// the editor's environment edit: the server's default only reaches a scene without a transition
const worlds = { scenes: [{ id: 'a', environment: 'env.office', transition: 'cut' }, { id: 'b', environment: 'env.office', transition: 'fade' }, { id: 'c', environment: 'env.office', transition: 'fade' }, { id: 'd', environment: 'env.office', transition: 'fade' }] };
M.setEnvironment(worlds, worlds.scenes[1], 'env.home-office');
eq(worlds.scenes[1].environment, 'env.home-office', 'the environment is set');
eq(worlds.scenes.map(s => s.transition), ['cut', 'wipe', 'wipe', 'fade'], 'a fade between worlds becomes a wipe, on this scene and the next');
M.setEnvironment(worlds, worlds.scenes[1], 'env.office');
eq(worlds.scenes.map(s => s.transition), ['cut', 'wipe', 'wipe', 'fade'], 'a wipe stays when the worlds match again');
const chosen = { scenes: [{ id: 'a', environment: 'env.office', transition: 'cut' }, { id: 'b', environment: 'env.office', transition: 'slide-left' }] };
M.setEnvironment(chosen, chosen.scenes[1], 'env.home-office');
eq(chosen.scenes[1].transition, 'slide-left', 'another transition stays');
const opening = { scenes: [{ id: 'a', environment: 'env.office', transition: 'fade' }, { id: 'b', environment: 'env.office', transition: 'fade' }] };
M.setEnvironment(opening, opening.scenes[0], 'env.home-office');
eq(opening.scenes.map(s => s.transition), ['fade', 'wipe'], 'the first scene has no transition to change, the next one has');
const unsaved = { scenes: [{ id: 'a', environment: 'env.office' }, { id: 'b', environment: 'env.office' }] };
M.setEnvironment(unsaved, unsaved.scenes[1], 'env.home-office');
eq('transition' in unsaved.scenes[1], false, 'an unset transition is left to the server default');
const scene = { id: 'x', narration: 'hi' };
M.setSceneField(scene, 'narration', ''); eq('narration' in scene, false, 'an empty field leaves the document');
M.setDuration(scene, false, '7.26'); eq(scene.duration, 7.3, 'fixed duration');
M.setDuration(scene, true, 0); eq(scene.duration, 'auto', 'auto duration');
const report = { frame_count: 300, fps: 30, scenes: [{ id: 'a', start_frame: 0, end_frame: 150 }, { id: 'b', start_frame: 140, end_frame: 300 }] };
eq(M.sceneAtFrame(report, 139), 0, 'frame in the first scene'); eq(M.sceneAtFrame(report, 145), 1, 'transitions overlap, the later scene wins');
eq(M.frameForScene(report, 1), 140 + Math.floor(160 * 0.4), 'frame for a scene');
eq([{ browser: { ready: true } }, { browser: { ready: false, can_download: true } }, { browser: { ready: false } }, {}, null].map(M.browserState), ['ready', 'download', 'missing', 'missing', 'missing'], 'the render browser is ready, fetched by the first export, or missing');

// the message schema: only fixed shapes, with caps
const P = S.player.checkIncoming;
const valid = { sm: 1, type: 'compiled', report: { frame_count: 90, fps: 30, scenes: [{ id: 's1', start_frame: 0, end_frame: 90, kind: 'storyboard' }], issues: [{ severity: 'warning', code: 'x', path: '/a', message: 'm' }] } };
eq(P(valid).report.scenes.length, 1, 'a valid report');
eq(P(Object.assign({}, valid, { sm: 2 })), null, 'wrong version');
eq(P({ sm: 1, type: 'load' }), null, 'a parent-to-host type is refused');
eq(P({ sm: 1, type: 'compiled', report: { frame_count: 90, fps: 30, scenes: new Array(401).fill({ id: 's', start_frame: 0, end_frame: 1 }) } }), null, 'too many scenes');
eq(P({ sm: 1, type: 'compiled', report: { frame_count: -1, fps: 30, scenes: [] } }), null, 'negative frame count');
eq(P({ sm: 1, type: 'time', frame: 3, playing: 'yes' }), null, 'time needs a boolean');
eq(P({ sm: 1, type: 'time', frame: 3, playing: true }), { type: 'time', frame: 3, playing: true }, 'time');
eq(P({ sm: 1, type: 'need_assets', req: 'a1', keys: new Array(2001).fill('catalog:x') }), null, 'too many keys');
eq(P({ sm: 1, type: 'need_assets', req: 'a1', keys: ['catalog:rig/human.json', 7] }), null, 'a key that is not a string');
eq(P({ sm: 1, type: 'thumb', req: 't1', scene_id: 's1', bitmap: {} }), null, 'a thumb needs an ImageBitmap');
eq(P({ sm: 1, type: 'thumb', req: 't1', scene_id: 's1', bitmap: new FakeBitmap() }).scene_id, 's1', 'a thumb');
eq(P({ sm: 1, type: 'error', code: 'code_error', message: 'x'.repeat(5000), scene_id: 's3' }).message.length, 2000, 'long messages are cut');
eq(P({ sm: 1, type: 'ready', caps: { workers: true, offscreen: false }, extra: 'x' }), { type: 'ready', caps: { workers: true, offscreen: false } }, 'unknown fields are dropped');
// the compiled whitelist of phase 2: a malformed entry is dropped, the message stays
{
  const evHash = 'a'.repeat(64);
  const rich = P({ sm: 1, type: 'compiled', report: { frame_count: 90, fps: 30, cue_precision: 0.5,
    scenes: [{ id: 's1', start_frame: 0, end_frame: 90, kind: 'storyboard', cues: [{ marker: 1, at_ms: 1840, placed: 'span' }, { marker: 'x', at_ms: 1, placed: 'span' }, { marker: 2, at_ms: 5, placed: 'guess' }] }],
    audio_events: [
      { kind: 'narration', ref: evHash, start_ms: 500, scene_id: 's1', block_id: 's1:n', piece: 0, extra: 'x' },
      { kind: 'sfx', ref: 'sfx.pop', start_ms: 1836, scene_id: 's1', take: 2 },
      { kind: 'music', ref: evHash, start_ms: 1, scene_id: 's1' },
      { kind: 'narration', ref: 'r'.repeat(81), start_ms: 1, scene_id: 's1' },
      { kind: 'narration', ref: evHash, start_ms: -1, scene_id: 's1' },
      { kind: 'narration', ref: evHash, start_ms: 1, scene_id: 's1', piece: 'x' },
      'nonsense'
    ],
    captions: [
      { start_ms: 500, end_ms: 2600, lines: ['one', 'two', 'three'], scene_id: 's1', block_id: 's1:n' },
      { start_ms: 2600, end_ms: 3600, lines: ['x'.repeat(500)], scene_id: 's1', block_id: 's1:n' },
      { start_ms: 3600, end_ms: 4600, lines: [7], scene_id: 's1', block_id: 's1:n' },
      { start_ms: 4600, end_ms: 5600, lines: [], scene_id: 's1', block_id: 's1:n' }
    ]
  } });
  eq(rich.report.audio_events, [
    { kind: 'narration', ref: evHash, start_ms: 500, scene_id: 's1', block_id: 's1:n', piece: 0 },
    { kind: 'sfx', ref: 'sfx.pop', start_ms: 1836, scene_id: 's1', take: 2 }
  ], 'malformed events are dropped, unknown fields too, and the message stays');
  eq(rich.report.captions.map(c => c.lines.map(line => line.length)), [[3, 3], [400]], 'at most 2 lines, each clipped at 400 characters');
  eq(rich.report.scenes[0].cues, [{ marker: 1, at_ms: 1840, placed: 'span' }], 'only well-formed cues stay');
  eq(rich.report.cue_precision, 0.5, 'cue_precision');
  const bare = P(valid).report;
  eq([bare.audio_events, bare.captions, bare.cue_precision, bare.scenes[0].cues], [[], [], null, []], 'a report without the new fields');
  // the other side of the wire: a report from a real compile loses nothing (dir is ui/js/desktop/apps, four levels below the repo root)
  const measured = JSON.parse(fs.readFileSync(path.join(dir, '..', '..', '..', '..', 'internal', 'storymaker', 'render', 'testdata', 'report_measured.json'), 'utf8'));
  const wire = P({ sm: 1, type: 'compiled', report: measured }).report;
  eq(wire.audio_events.length, measured.audio_events.length, 'every event of a real compile passes');
  eq(wire.captions.length, (measured.captions || []).length, 'every caption of a real compile passes');
}

// the export size follows the aspect, as render.CanvasSize does (the preview compiles smaller)
eq([M.exportSize('16:9'), M.exportSize('9:16'), M.exportSize('1:1'), M.exportSize(''), M.exportSize('4:3')], [[1920, 1080], [1080, 1920], [1080, 1080], [1920, 1080], [1920, 1080]], 'export size by aspect');

console.log('storymaker app model ok');
`
