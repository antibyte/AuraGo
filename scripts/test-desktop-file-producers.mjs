import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const read = name => readFile(new URL('../ui/js/desktop/core/' + name, import.meta.url), 'utf8');
const [menus, drops] = await Promise.all([read('menus-and-routing.js'), read('desktop-file-drops.js')]);
const records = [], removed = [], positions = [], notices = [];
let refreshed = 0, calls = 0;
const ctx = {
    window: { AuraDesktopFileHistory: { record: (moves, label) => records.push({ moves, label }) } },
    t: key => key, desktopReadonly: () => false,
    normalizeDesktopPath: path => String(path || '').replaceAll('\\', '/'),
    isInsideTrashPath: path => path.startsWith('Trash/'),
    removeIconPosition: path => removed.push(path),
    showDesktopNotification: value => notices.push(value), desktopSound() {},
    clampDesktopIconPosition: (x, y) => ({ x, y }), desktopIconGridEnabled: () => false,
    saveIconPosition: path => positions.push(path)
};
vm.createContext(ctx);
vm.runInContext(menus + '\n' + drops, ctx);
ctx.desktopReadonly = () => false;
ctx.isInsideTrashPath = path => path.startsWith('Trash/');
ctx.refreshDesktopAfterFileChange = async () => { refreshed++; };
ctx.refreshAfterDesktopFileDrop = async () => { refreshed++; };
const first = { from: 'Desktop/a.txt', path: 'Trash/a.txt', undo_receipt: 'opaque-a' };
ctx.api = async () => {
    calls++;
    throw Object.assign(new Error('second item failed'), { body: { code: 'partial_failure', moves: [first] } });
};
assert.deepEqual(await ctx.movePathsToTrash(['Desktop/a.txt', 'Desktop/b.txt']), [first]);
assert.equal(calls, 1, 'partial batch must not be replayed');
assert.deepEqual(removed, ['desktop-entry-Desktop/a.txt'], 'remove only confirmed icons');
assert.deepEqual(records[0].moves, [first]);
assert.equal(refreshed, 1);
const restored = { from: 'Trash/a.txt', path: 'Desktop/a.txt', undo_receipt: 'opaque-back' };
ctx.api = async () => { throw Object.assign(new Error('partial restore'), { body: { moves: [restored] } }); };
assert.equal((await ctx.restorePathsFromTrash(['Trash/a.txt', 'Trash/b.txt'])).join(','), 'Desktop/a.txt');
assert.deepEqual(records.at(-1).moves, [restored]);
ctx.api = async () => ({ moves: [first] });
ctx.refreshDesktopAfterFileChange = async () => { throw new Error('refresh failed'); };
assert.deepEqual(await ctx.movePathsToTrash(['Desktop/a.txt']), [first], 'refresh failure must not discard receipts');
ctx.desktopReadonly = () => true;
ctx.api = async () => { throw new Error('read-only mutation attempted'); };
assert.equal((await ctx.movePathsToTrash(['Desktop/a.txt'])).length, 0);
assert.equal((await ctx.restorePathsFromTrash(['Trash/a.txt'])).length, 0);
ctx.desktopReadonly = () => false;

let mutations = 0;
const moved = { path: 'Desktop/a (2).txt', undo_receipt: 'opaque-move' };
ctx.api = async (_url, options) => {
    if (!options) return { files: [] };
    if (++mutations === 1) return moved;
    throw new Error('move failed');
};
await assert.rejects(ctx.moveDraggedFilesToDesktop(['Documents/a.txt', 'Documents/b.txt'], 80, 90), /move failed/);
assert.equal(records.at(-1).moves.length, 1);
assert.equal(records.at(-1).moves[0], moved, 'already moved file stays undoable');
assert.equal(positions.at(-1), 'desktop-entry-Desktop/a (2).txt', 'use the actual server destination');

mutations = 0;
ctx.window.AuraDesktopFileClipboard = { mode: 'cut', paths: ['Documents/a.txt', 'Documents/b.txt'] };
await assert.rejects(ctx.pasteDesktopFileClipboard('Desktop'), /move failed/);
assert.equal(records.at(-1).moves.length, 1);
assert.equal(ctx.window.AuraDesktopFileClipboard.paths.join(','), 'Documents/b.txt', 'keep only unconfirmed cut sources');
let continuing = true;
const beforeCancelRefresh = refreshed;
ctx.api = async (_url, options) => {
    if (!options) return { files: [] };
    continuing = false;
    return moved;
};
ctx.window.AuraDesktopFileClipboard = { mode: 'cut', paths: ['Documents/a.txt', 'Documents/b.txt'] };
await ctx.pasteDesktopFileClipboard('Desktop', { shouldContinue: () => continuing });
assert.equal(refreshed, beforeCancelRefresh + 1, 'confirmed mutation refreshes even when the caller closes');
assert.equal(ctx.window.AuraDesktopFileClipboard.paths.join(','), 'Documents/b.txt');
ctx.listTrashEntries = async () => [{ path: 'Trash/a.txt' }, { path: 'Trash/b.txt' }];
ctx.settingBool = () => false;
ctx.refreshDesktopAfterFileChange = async () => { refreshed++; };
mutations = 0;
ctx.api = async () => { if (++mutations === 2) throw new Error('permanent delete failed'); };
const beforeEmptyRefresh = refreshed;
await ctx.emptyTrash();
assert.equal(refreshed, beforeEmptyRefresh + 1, 'partial permanent deletion must refresh confirmed changes');
assert.ok(notices.some(item => item.message === 'refresh failed'));
const foundation = await read('desktop-foundation.js');
const apiSource = foundation.slice(foundation.indexOf('    async function api('), foundation.indexOf('    function callAppDispose('));
let failureBody = { code: 'file_undo_uncertain' }, refreshes = 0, requests = 0;
const apiContext = {
    t: key => key, prepareDesktopFileMutation: () => null, resolveDesktopFileConflict: async () => false,
    refreshDesktopAfterFileChange: async () => { refreshes++; },
    fetch: async () => { requests++; return { ok: false, status: 502, headers: { get: key => key === 'content-type' ? 'application/json' : '' }, json: async () => failureBody }; }
};
vm.createContext(apiContext);
vm.runInContext(apiSource, apiContext);
await assert.rejects(apiContext.api('/api/desktop/file', { method: 'PATCH' }), /desktop.file_history_unknown/);
assert.equal(refreshes, 1, 'uncertain rename refreshes state without repeating the mutation');
assert.equal(requests, 1);
failureBody = { code: 'partial_failure', uncertain: true, moves: [first] };
await assert.rejects(apiContext.api('/api/desktop/trash', { method: 'POST' }), err => {
    assert.deepEqual(err.body.moves, [first]);
    return err.message === 'desktop.file_history_unknown';
});
assert.equal(refreshes, 2);
failureBody = { code: 'file_undo_limit' };
await assert.rejects(apiContext.api('/api/desktop/trash', { method: 'POST' }), /desktop.file_history_limit/);
assert.equal(refreshes, 2, 'preflight capacity rejection does not claim a mutation');
console.log('Desktop file producers retain partial receipts, actual paths and remaining clipboard items.');
