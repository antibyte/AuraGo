package ui

import (
	"os/exec"
	"testing"
)

// videoStudioFoldersPrelude loads the DOM-free folder model with the fixtures the checks use.
const videoStudioFoldersPrelude = `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-folders.js');
const F = window.VideoStudioFolders;
const asset = (id, name, extra) => Object.assign({id, name, kind: 'video'}, extra || {});
const project = (assets, folders, tracks) => ({version: 2, assets, folders: folders || [], tracks: tracks || []});
`

func TestVideoStudioFolderModel(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio folder checks")
	}
	script := videoStudioFoldersPrelude + `
// Paths: trimmed segments, at most four levels, no control characters, no empty or relative segments.
assert.equal(F.normalizeFolder(' Trips / 2024 '), 'Trips/2024');
assert.equal(F.normalizeFolder(''), '');
const longSegment = 'x'.repeat(64);
for (const bad of ['a//b', '/a', 'a/', '.', 'a/..', 'a/b/c/d/e', 'a\x07b', 'x'.repeat(65), [longSegment, longSegment, longSegment, longSegment].join('/')]) {
	assert.equal(F.normalizeFolder(bad), null, String(bad));
}
// Folder tree: the project's own folders, every asset folder and their ancestors.
const p = project([asset('a1', 'A', {folder: 'Trips/2024'}), asset('a2', 'B', {folder: 'Music'}), asset('a3', 'C')], ['Empty/Folder']);
assert.deepEqual(F.folderPaths(p), ['Empty', 'Empty/Folder', 'Music', 'Trips', 'Trips/2024']);
assert.deepEqual(F.childFolders(p, ''), ['Empty', 'Music', 'Trips']);
assert.deepEqual(F.childFolders(p, 'Trips'), ['Trips/2024']);
assert.deepEqual(F.assetsIn(p, '', false).map(a => a.id), ['a3']);
assert.deepEqual(F.assetsIn(p, 'Trips', true).map(a => a.id), ['a1']);
// Usage counts clips per asset across tracks.
const used = project([asset('a1', 'A')], [], [{id: 'v1', clips: [{id: 'c1', asset_id: 'a1'}, {id: 'c2', asset_id: 'a1'}]}, {id: 'v2', clips: [{id: 'c3', asset_id: 'a1'}]}]);
assert.equal(F.usageCounts(used).get('a1'), 3);
// Sorting by the list columns; ties keep the name order.
const s = [
	asset('x', 'Beta', {duration_us: 2e6, width: 1920, height: 1080, source_fps: '30000/1001', size_bytes: 10}),
	asset('y', 'alpha', {duration_us: 1e6, width: 1280, height: 720, source_fps: '25/1', size_bytes: 30}),
	asset('z', 'Gamma', {duration_frames: 90, width: 640, height: 360, codec: 'vp9', size_bytes: 20})
];
assert.deepEqual(F.sortAssets(s, 'name', 'asc', {}).map(a => a.name), ['alpha', 'Beta', 'Gamma']);
assert.deepEqual(F.sortAssets(s, 'duration', 'asc', {fps: 30}).map(a => a.id), ['y', 'x', 'z']);
assert.deepEqual(F.sortAssets(s, 'resolution', 'desc', {}).map(a => a.id), ['x', 'y', 'z']);
assert.deepEqual(F.sortAssets(s, 'frame_rate', 'desc', {}).map(a => a.id), ['x', 'y', 'z']);
assert.deepEqual(F.sortAssets(s, 'size', 'asc', {}).map(a => a.id), ['x', 'z', 'y']);
assert.deepEqual(F.sortAssets(s, 'usage', 'desc', {usage: new Map([['z', 4]])}).map(a => a.id), ['z', 'y', 'x']);
// Folder edits: creation, collisions, the folder limit, renames into a subtree, delete to the parent.
const e = project([asset('a1', 'A', {folder: 'Trips/2024'})], ['Trips']);
assert.equal(F.createFolder(e, 'Trips/2024'), F.REASON.exists);
assert.equal(F.createFolder(e, 'a//b'), F.REASON.invalid);
assert.equal(F.createFolder(e, 'New'), true);
assert.ok(e.folders.includes('New'));
const limited = project([], Array.from({length: F.LIMITS.folders}, (_, i) => 'f' + i));
assert.equal(F.createFolder(limited, 'more'), F.REASON.limit);
assert.equal(F.renameFolder(e, 'Trips', 'Trips/Sub'), F.REASON.inside);
assert.equal(F.renameFolder(e, 'Trips', 'New'), F.REASON.exists);
assert.equal(F.renameFolder(e, 'Trips', 'Journeys'), true);
assert.equal(e.assets[0].folder, 'Journeys/2024');
assert.ok(e.folders.includes('Journeys') && !e.folders.includes('Trips'));
const deep = project([asset('a1', 'A', {folder: 'A/B/C'})], ['A/B/C/D']);
assert.equal(F.renameFolder(deep, 'A', 'W/X/Y'), F.REASON.invalid, 'a rename that pushes a subfolder past four levels is refused');
assert.equal(F.renameFolder(deep, 'A', 'W/X'), F.REASON.invalid, 'the listed empty subfolder counts too');
assert.equal(deep.assets[0].folder, 'A/B/C', 'a refused rename changes nothing');
assert.equal(F.renameFolder(deep, 'Nope', 'Z'), F.REASON.nothing);
assert.equal(F.renameFolder(deep, 'A', 'X'), true);
assert.deepEqual(deep.folders, ['X/B/C/D']);
const d = project([asset('a1', 'A', {folder: 'Trips/2024'}), asset('a2', 'B', {folder: 'Trips'})], ['Trips/2024/Old']);
assert.equal(F.deleteFolder(d, 'Trips/2024'), true);
assert.equal(d.assets[0].folder, 'Trips');
assert.equal(d.assets[1].folder, 'Trips');
assert.deepEqual(d.folders, ['Trips/Old']);
assert.equal(F.deleteFolder(d, 'Nope'), F.REASON.nothing);
// Moving and renaming assets.
const m = project([asset('a1', 'A'), asset('a2', 'B', {folder: 'X'})], []);
assert.equal(F.moveAssets(m, ['a1', 'a2'], 'X/Y'), true);
assert.equal(m.assets[0].folder, 'X/Y');
assert.equal(m.assets[1].folder, 'X/Y');
assert.equal(F.moveAssets(m, ['a1'], 'X/Y'), F.REASON.nothing);
assert.equal(F.moveAssets(m, ['a1'], 'a//b'), F.REASON.invalid);
assert.equal(F.moveAssets(m, ['a1'], ''), true);
assert.equal('folder' in m.assets[0], false);
assert.equal(F.renameAsset(m, 'a1', '  Hero  '), true);
assert.equal(m.assets[0].name, 'Hero');
assert.equal(F.renameAsset(m, 'a1', '   '), F.REASON.name);
assert.equal(F.renameAsset(m, 'nope', 'x'), F.REASON.name);
assert.equal(F.joinFolder('Trips', 'Sub'), 'Trips/Sub');
console.log('ok');
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("folder checks: %v\n%s", err, output)
	}
}
