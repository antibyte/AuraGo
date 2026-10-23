import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import '../ui/js/desktop/apps/ziro-files.js';
const { Project, relative, shared, filesChecked } = globalThis.AuraZiroFiles;
const bytes = text => new TextEncoder().encode(text);
const etag = data => '"' + createHash('sha256').update(data).digest('hex') + '"';
for (const path of ['../x', '/x', 'a/../b', 'a\\b', 'C:/x', 'x//y', 'x/./y', 'x./y', 'x\0y']) assert.throws(() => relative(path));
assert.throws(() => shared('Documents/x'));
assert.throws(() => filesChecked([{ name: 'a', bytes: bytes('') }, { name: 'A', bytes: bytes('') }]));
assert.throws(() => filesChecked(Array.from({ length: 2001 }, (_, i) => ({ name: String(i), bytes: bytes('') }))));
assert.equal(relative('sub/Schaltplan.kicad_sch'), 'sub/Schaltplan.kicad_sch');
const folder = 'Shared/Ziro Designer/Board';
const disk = new Map([
    [folder + '/Board.kicad_pro', bytes('{}')], [folder + '/Board.kicad_sch', bytes('(schematic)')],
    [folder + '/sub/model.bin', Uint8Array.of(0, 255, 128, 0, 1)]
]);
let failPath = '', readonly = false, writes = 0;
const request = async (input, options = {}) => {
    const url = new URL(input, 'http://localhost');
    const json = (body, status = 200) => Response.json(body, { status });
    if (url.pathname === '/api/desktop/files') {
        const path = url.searchParams.get('path');
        return json({ files: [...disk].filter(([name]) => name.startsWith(path + '/')).map(([path, bytes]) => ({ path, size: bytes.length, type: 'file' })) });
    }
    if (url.pathname === '/api/desktop/download') {
        const data = disk.get(url.searchParams.get('path'));
        return new Response(data, { headers: { ETag: etag(data) } });
    }
    if (readonly) return json({ code: 'desktop_readonly' }, 403);
    if (url.pathname === '/api/desktop/directory') return json({ status: 'ok' });
    assert.equal(url.pathname, '/api/desktop/upload');
    const file = options.body.get('file');
    const path = options.body.get('path') + '/' + file.name;
    if (path === failPath) return json({ error: 'disk full' }, 500);
    const old = disk.get(path);
    const headers = new Headers(options.headers);
    if ((old && headers.get('If-Match') !== etag(old)) || (!old && headers.get('If-None-Match') !== '*')) {
        return json({ code: 'file_conflict', conflict: { path, version: old ? etag(old) : '' } }, 412);
    }
    writes++;
    const data = new Uint8Array(await file.arrayBuffer());
    disk.set(path, data);
    return json({ path, version: etag(data) });
};
const project = new Project(request);
const opened = await project.open(folder + '/Board.kicad_pro');
const linked = new Project(async (input, options) => {
    if (input.startsWith('/api/desktop/files?')) return Response.json({ files: [{ path: folder + '/escape.kicad_pro', type: 'file', mode: 'Lrwxrwxrwx' }] });
    throw new Error('a symlink must be rejected before reading it');
});
await assert.rejects(linked.open(folder + '/escape.kicad_pro'), /invalid_path/);
const byNewest = new Project(async (input, options) => {
    if (input.startsWith('/api/desktop/files?')) return Response.json({ files: [...disk].reverse().map(([path, data]) => ({ path, type: 'file', size: data.length })) });
    return request(input, options);
});
assert.equal((await byNewest.open(folder + '/Board.kicad_pro'))[0].name, 'Board.kicad_pro', 'the selected project file must lead the upstream file set');
await project.save(opened, () => { throw Error('unexpected conflict'); });
assert.equal(writes, 0);
assert.deepEqual(opened.find(file => file.name.endsWith('.bin')).bytes, Uint8Array.of(0, 255, 128, 0, 1));
const edited = opened.map(file => file.name.endsWith('.bin') ? file : { ...file, bytes: bytes(file.name) });
failPath = folder + '/Board.kicad_sch';
await assert.rejects(project.save(edited, () => 'replace'), /disk full/);
assert.equal(writes, 1);
failPath = '';
await project.save(edited, () => 'replace');
assert.equal(writes, 2, 'a retry keeps completed file versions');
let disconnected = false;
const reconnect = new Project((input, options) => {
    if (disconnected && input === '/api/desktop/upload') throw new TypeError('Failed to fetch');
    return request(input, options);
});
const reconnectFiles = await reconnect.open(folder + '/Board.kicad_pro');
const observed = reconnect.baseline.get('Board.kicad_pro');
reconnectFiles[0] = { ...reconnectFiles[0], bytes: bytes('network retry') };
disconnected = true;
await assert.rejects(reconnect.save(reconnectFiles, () => 'replace'), /Failed to fetch/);
assert.equal(reconnect.baseline.get('Board.kicad_pro'), observed, 'a lost connection cannot advance the confirmed version');
disconnected = false;
await reconnect.save(reconnectFiles, () => 'replace');
assert.equal(new TextDecoder().decode(disk.get(folder + '/Board.kicad_pro')), 'network retry');
const concurrent = new Project(request);
const old = await concurrent.open(folder + '/Board.kicad_pro');
disk.set(folder + '/Board.kicad_pro', bytes('another window'));
old[0] = { ...old[0], bytes: bytes('my changes') };
await assert.rejects(concurrent.save(old, () => 'copy'), error => error.copyProject);
assert.equal(new TextDecoder().decode(disk.get(folder + '/Board.kicad_pro')), 'another window');
await concurrent.save(old, () => 'replace');
assert.equal(new TextDecoder().decode(disk.get(folder + '/Board.kicad_pro')), 'my changes');
readonly = true;
old[0] = { ...old[0], bytes: bytes('denied') };
await assert.rejects(concurrent.save(old, () => 'replace'), error => error.status === 403);
const controller = new AbortController(); controller.abort();
await assert.rejects(new Project(request, controller.signal).open(folder + '/Board.kicad_pro'), error => error.name === 'AbortError');
const shell = await readFile(new URL('../ui/js/desktop/core/window-shell-runtime.js', import.meta.url), 'utf8');
const windowMatchSource = shell.slice(shell.indexOf('function matchesExistingAppWindow('), shell.indexOf('function findExistingAppWindow('));
const matchesWindow = new Function('normalizeDesktopPath', 'return ' + windowMatchSource)(value => String(value).replaceAll('\\', '/'));
const ziroWindow = { appId: 'store-ziro-designer', context: { path: 'Shared/A/Board/Board.kicad_pro' } };
assert.equal(matchesWindow(ziroWindow, ziroWindow.appId, { path: 'Shared/A/Board/Board.kicad_pro' }), true);
assert.equal(matchesWindow(ziroWindow, ziroWindow.appId, { path: 'Shared/B/Board/Board.kicad_pro' }), false, 'opening a different project needs its own window');
for (const locale of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']) {
    const translations = JSON.parse(await readFile(new URL('../ui/lang/desktop/' + locale + '.json', import.meta.url)));
    for (const key of ['description','new','open','save','save_as','ready','saved','saving','unsaved_status','unsaved','discard','recover','recover_action','project_name','conflict','replace','copy','invalid_path','shared_only','project_limit','invalid_project','version_missing','disconnected','readonly','folder_exists','origin_required','save_failed']) {
        assert.ok(translations['desktop.ziro.' + key], locale + ': ' + key);
    }
}
console.log('Ziro: paths, byte preservation, conditional writes, partial failures, conflicts, readonly, cancellation and 16 locales passed.');
