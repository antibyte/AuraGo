import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const drafts = new Map();
const files = new Map();
const listeners = new Map();
const document = {
    addEventListener(name, handler) { listeners.set(name, handler); },
    removeEventListener(name, handler) { if (listeners.get(name) === handler) listeners.delete(name); },
    dispatchEvent(event) { listeners.get(event.type)?.(event); }
};
const context = {
    AbortController,
    document,
    location: { origin: 'http://aurago.test' },
    setTimeout,
    clearTimeout,
    window: null
};
context.window = context;
vm.createContext(context);
vm.runInContext(readFileSync('ui/js/desktop/apps/writer-session.js', 'utf8'), context);
context.OfficeSession.draft = async (operation, key, value) => {
    if (operation === 'put') { drafts.set(key, { ...value, key }); return value; }
    if (operation === 'get') return drafts.get(key) || null;
    if (operation === 'deleteIf') {
        if (drafts.get(key)?.id === value.id) drafts.delete(key);
        return true;
    }
    throw new Error('unsupported draft operation: ' + operation);
};
vm.runInContext(readFileSync('ui/js/desktop/apps/synth-studio-storage.js', 'utf8'), context);
vm.runInContext(readFileSync('ui/js/desktop/apps/synth-studio-model.js', 'utf8'), context);

const makeProject = name => ({ version: 1, name });
const rawValidationInputs = [];
const validate = value => {
    let project = value;
    if (typeof project === 'string') {
        rawValidationInputs.push(project);
        if (project.length > 5 * 1024 * 1024) throw new Error('project too large');
        project = JSON.parse(project);
    }
    if (!project || project.version !== 1 || typeof project.name !== 'string') throw new Error('invalid project');
    return { version: 1, name: project.name };
};
assert.throws(
    () => context.SynthStudioModel.validate(' '.repeat(5 * 1024 * 1024 + 1)),
    error => error.code === 'E_PROJECT_SIZE',
    'the real project validator rejects oversized raw JSON before parsing'
);
const makeContext = ({ windowId, sessionKey = windowId, path = '', readonly = false, savePath = 'Documents/Synth Studio/Untitled.aurasynth', onLeaveDraft = async () => true } = {}) => ({
    windowId,
    sessionKey,
    path,
    readonly,
    openFileDialog: async () => ({ path: savePath }),
    saveFileDialog: async () => ({ path: savePath }),
    confirmDialog: async () => true,
    updateWindowContext() {},
    loadBootstrap() {},
    api: async (url, options = {}) => {
        if (options.signal?.aborted) throw new DOMException('Aborted', 'AbortError');
        if (!options.method || options.method === 'GET') {
            const target = decodeURIComponent(String(url).split('path=')[1] || '');
            const file = files.get(target);
            if (!file) throw new Error('not found');
            return { path: target, content: file.content, version: file.version };
        }
        if (options.method !== 'PUT') throw new Error('unsupported API call');
        const body = JSON.parse(options.body);
        const headers = options.headers || {};
        const existing = files.get(body.path);
        if (headers['If-Match'] && (!existing || headers['If-Match'] !== existing.version)) {
            const error = new Error('file_conflict');
            error.status = 412;
            throw error;
        }
        if (headers['If-None-Match'] === '*' && existing) {
            const error = new Error('file_conflict');
            error.status = 412;
            throw error;
        }
        const version = '"v' + ((existing?.counter || 0) + 1) + '"';
        files.set(body.path, { content: body.content, version, counter: (existing?.counter || 0) + 1 });
        return { path: body.path, version };
    },
    onLeaveDraft
});
const makeStorage = (ctx, project, extra = {}) => context.SynthStudioStorage.create(ctx, {
    serialize: () => project.current,
    validate,
    onLoaded: (loaded, detail) => {
        project.loads.push({ loaded, detail });
        if (loaded) project.current = loaded;
    },
    onLeaveDraft: ctx.onLeaveDraft,
    ...extra
});

const draftProject = { current: makeProject('Latest recoverable edit'), loads: [] };
const first = makeStorage(makeContext({ windowId: 'w-first', sessionKey: 'restore-key-1' }), draftProject);
await first.ready;
assert.equal(draftProject.loads[0].loaded, null, 'new windows start from the editor default');
draftProject.current = makeProject('Latest recoverable edit');
first.changed();
assert.equal(await first.guard(), true, 'pathless close can retain a recoverable draft');
const stableKey = 'http://aurago.test:synth-studio:window:restore-key-1';
assert.equal(drafts.get(stableKey)?.content, JSON.stringify(draftProject.current), 'guard flushes the latest serialized project');
first.dispose();

const recoveredProject = { current: makeProject('Default'), loads: [] };
const recovered = makeStorage(makeContext({ windowId: 'w-after-reload', sessionKey: 'restore-key-1' }), recoveredProject, {
    onRecovery: async () => true
});
await recovered.ready;
assert.equal(recoveredProject.current.name, 'Latest recoverable edit', 'session restoration reuses the durable unnamed draft key');
assert.ok(rawValidationInputs.includes(JSON.stringify(makeProject('Latest recoverable edit'))), 'recovery content reaches validation as raw JSON');
assert.equal(recovered.state.recovered, true);
assert.equal(await recovered.newDocument(makeProject('New project')), true, 'replacing an unnamed dirty project first offers Save As');
assert.equal(files.get('Documents/Synth Studio/Untitled.aurasynth')?.content, JSON.stringify(makeProject('Latest recoverable edit')));
assert.equal(recovered.state.path, '', 'the replacement starts as a fresh local document');
recovered.dispose();

const cleanProject = { current: makeProject('Save As clean'), loads: [] };
const clean = makeStorage(makeContext({ windowId: 'w-clean', savePath: 'Documents/Synth Studio/Clean.aurasynth' }), cleanProject);
await clean.ready;
assert.equal(await clean.save(true), true, 'Save As writes even when the current revision was clean');
assert.equal(clean.state.path, 'Documents/Synth Studio/Clean.aurasynth');
assert.equal(files.get(clean.state.path)?.content, JSON.stringify(cleanProject.current));
clean.dispose();

const failingContext = makeContext({ windowId: 'w-failing', path: 'Documents/Synth Studio/Unavailable.aurasynth' });
files.set('Documents/Synth Studio/Unavailable.aurasynth', { content: JSON.stringify(makeProject('Old disk')), version: '"v1"', counter: 1 });
failingContext.api = async (url, options = {}) => {
    if (!options.method || options.method === 'GET') return { content: files.get('Documents/Synth Studio/Unavailable.aurasynth').content, version: '"v1"' };
    throw new Error('disk unavailable');
};
const failingProject = { current: makeProject('Default'), loads: [] };
const failing = makeStorage(failingContext, failingProject);
await failing.ready;
failingProject.current = makeProject('Latest after failed save');
failing.changed();
assert.equal(await failing.guard(), true, 'failed saves can leave the latest state as a recovery draft');
const failedSaveKey = 'http://aurago.test:synth-studio:Documents/Synth Studio/Unavailable.aurasynth';
assert.equal(drafts.get(failedSaveKey)?.content, JSON.stringify(failingProject.current));
assert.equal(failing.state.dirty, true, 'failed writes retain dirty state');
failing.dispose();

files.set('Documents/Synth Studio/Stale.aurasynth', { content: JSON.stringify(makeProject('Disk revision')), version: '"v2"', counter: 2 });
const staleKey = 'http://aurago.test:synth-studio:Documents/Synth Studio/Stale.aurasynth';
drafts.set(staleKey, {
    key: staleKey,
    id: randomUUID(),
    path: 'Documents/Synth Studio/Stale.aurasynth',
    etag: '"v1"',
    content: JSON.stringify(makeProject('Recovery revision'))
});
const staleProject = { current: makeProject('Default'), loads: [] };
const stale = makeStorage(makeContext({ windowId: 'w-stale', path: 'Documents/Synth Studio/Stale.aurasynth' }), staleProject, {
    onRecovery: async () => true
});
await stale.ready;
assert.equal(staleProject.current.name, 'Recovery revision');
assert.equal(stale.state.path, '', 'stale recovery must not overwrite the changed disk file');
assert.ok(stale.state.error, 'stale recovery exposes the save-as warning');
stale.dispose();

const readonlyProject = { current: makeProject('Default'), loads: [] };
const readonly = makeStorage(makeContext({ windowId: 'w-readonly', path: clean.state.path, readonly: true }), readonlyProject);
await readonly.ready;
assert.equal(readonlyProject.current.name, 'Save As clean', 'read-only mode still permits opening projects');
assert.equal(await readonly.save(), false);
assert.equal(listeners.has('aurago:desktop-policy'), true, 'storage observes policy changes independently of app context copies');
document.dispatchEvent({ type: 'aurago:desktop-policy', detail: { readonly: false } });
assert.equal(readonly.state.readonly, false);
readonly.dispose();
assert.equal(listeners.has('aurago:desktop-policy'), false, 'policy listener is removed on dispose');

const loadingContext = makeContext({ windowId: 'w-loading' });
const loadingProject = { current: makeProject('Before open'), loads: [] };
const loadingStates = [];
const loading = makeStorage(loadingContext, loadingProject, { onState: state => loadingStates.push(state.loading) });
await loading.ready;
assert.equal(loading.state.loading, false, 'initialization clears its loading fence before opening');
assert.equal(loading.state.dirty, false, 'the new local document starts clean');
files.set('Documents/Synth Studio/Deferred.aurasynth', { content: JSON.stringify(makeProject('Deferred project')), version: '"v1"', counter: 1 });
let finishRead;
loadingContext.api = () => new Promise(resolve => { finishRead = resolve; });
const opening = loading.open('Documents/Synth Studio/Deferred.aurasynth');
assert.equal(loading.state.loading, true, 'opening freezes editor controls while the file read is pending');
assert.equal(loadingStates.at(-1), true, 'loading state is published to the app shell');
await new Promise(resolve => setTimeout(resolve, 0));
assert.equal(typeof finishRead, 'function', 'the deferred file request has started');
finishRead({ content: JSON.stringify(makeProject('Deferred project')), version: '"v1"' });
assert.equal(await opening, true);
assert.ok(rawValidationInputs.includes(JSON.stringify(makeProject('Deferred project'))), 'file content reaches validation as raw JSON');
assert.equal(loading.state.loading, false, 'loading clears after the project has been installed');
assert.equal(loadingStates.at(-1), false);
loading.dispose();

console.log('Synth Studio storage: stable draft recovery, latest guard flush, clean Save As, stale-version protection, read-only open, and policy lifecycle pass');
