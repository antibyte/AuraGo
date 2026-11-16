import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const source = await readFile(new URL('../ui/js/desktop/core/session-runtime.js', import.meta.url), 'utf8');
const section = source.slice(source.indexOf('    function parseDefaultAppsMap()'), source.indexOf('    const RECENT_FILES_KEY'));
let stored = JSON.stringify({ pdf: 'removed-app' });
let release;
let count = 0;
const gate = new Promise(resolve => { release = resolve; });
const runtime = {
    settingValue: () => stored, desktopReadonly: () => false, t: key => key,
    appById: id => ['viewer', 'editor'].includes(id) ? { id } : undefined,
    desktopAssociationCandidates: () => [{ appId: 'viewer' }, { appId: 'editor' }],
    saveDesktopPreference: async (_key, value) => {
        if (++count === 1) await gate;
        stored = value;
    }
};
vm.runInNewContext(section + '\nglobalThis.save = setDefaultAppForExtension; globalThis.read = defaultAppForExtension;', runtime);
assert.equal(runtime.read('pdf'), '', 'an unavailable app is not launched');
assert.equal(JSON.parse(stored).pdf, 'removed-app', 'fallback never deletes the stored preference');
const a = runtime.save('txt', 'editor');
const b = runtime.save('png', 'viewer');
await Promise.resolve();
assert.equal(count, 1);
release();
await Promise.all([a, b]);
assert.deepEqual(JSON.parse(stored), { pdf: 'removed-app', txt: 'editor', png: 'viewer' });
assert.equal(runtime.read('txt'), 'editor');
await runtime.save('pdf', '');
assert.equal(JSON.parse(stored).pdf, undefined, 'reset explicitly removes only the selected mapping');
await assert.rejects(runtime.save('txt', 'unknown'), /action_unavailable/);
assert.equal(runtime.read('txt'), 'editor');
runtime.desktopReadonly = () => true;
await assert.rejects(runtime.save('txt', ''), /action_unavailable/);
console.log('Default-app concurrent writes, explicit reset and unavailable app fallback checked');
