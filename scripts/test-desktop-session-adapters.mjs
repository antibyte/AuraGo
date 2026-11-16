import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const source = await readFile(new URL('../ui/js/desktop/apps/infra-inspector-forms.js', import.meta.url), 'utf8');
const context = { window: {} };
vm.runInNewContext(source, context, { filename: 'infra-inspector-forms.js' });
const saveSessionDrafts = context.window.InfraAppInspectorForms.saveSessionDrafts;

for (const path of [
    '../ui/js/desktop/apps/cyd-creator-app.js',
    '../ui/js/desktop/apps/code-studio/core.js',
    '../ui/js/desktop/apps/easydrag-editor.js',
    '../ui/js/desktop/apps/game-maker-studio.js',
    '../ui/js/desktop/apps/infra.js',
    '../ui/js/desktop/apps/layerling.js',
    '../ui/js/desktop/apps/mission-control.js',
    '../ui/js/desktop/apps/video-studio.js'
]) {
    const adapter = await readFile(new URL(path, import.meta.url), 'utf8');
    assert.match(adapter, /registerSessionAction/, path + ' must register a logout participant');
    assert.match(adapter, /sessionActionOff/, path + ' must unregister on disposal');
}

function sourceBetween(source, start, end, label) {
    const first = source.indexOf(start);
    assert.notEqual(first, -1, `${label}: missing start marker`);
    const last = source.indexOf(end, first + start.length);
    assert.notEqual(last, -1, `${label}: missing end marker`);
    return source.slice(first, last);
}

// Exercise Code Studio's real adapter and saveAllFiles implementation with two
// modified tabs. The test harness only supplies the editor persistence boundary.
{
    const source = await readFile(new URL('../ui/js/desktop/apps/code-studio/core.js', import.meta.url), 'utf8');
    const chunks = [
        sourceBetween(source, '    function isLiveInstance(instance) {', '    function sessionRevision(instance) {', 'Code Studio isLiveInstance'),
        sourceBetween(source, '    function sessionRevision(instance) {', '    function runWithInstance(instance, fn) {', 'Code Studio participant'),
        sourceBetween(source, '    function runWithInstance(instance, fn) {', '    async function runAsyncStep(instance, fn) {', 'Code Studio runWithInstance'),
        sourceBetween(source, '    async function saveAllFiles() {', '    async function mutatePath(target, file, operation) {', 'Code Studio saveAllFiles'),
        sourceBetween(source, '    function canWrite(instance = state) {', '    function policyDisables(id, tab = activeTab()) {', 'Code Studio canWrite'),
        sourceBetween(source, '    function editorValue(tab) {', '    function activeTab() {', 'Code Studio editorValue')
    ];
    const harness = `
        const instances = new Map();
        let state = null;
        let nextSaveID = 0;
        const saves = [];
        const completeSave = new Map();
        function saveTab(target, tab) {
            saves.push(tab.path);
            const pending = new Promise(resolve => completeSave.set(tab.path, ok => {
                if (ok) tab.modified = false;
                tab.pendingSave = null;
                resolve(ok);
            }));
            tab.pendingSave = pending;
            return pending;
        }
        ${chunks.join('\n')}
        window.__test = { instances, registerSessionAction, saves, completeSave };
    `;
    const context = { window: {} };
    vm.runInNewContext(harness, context, { filename: 'code-studio-session-participant.mjs' });
    const h = context.window.__test;
    let participant = null;
    const instance = {
        windowId: 'code-tabs',
        context: { registerSessionAction: (_id, value) => { participant = value; return () => { participant = null; }; } },
        readOnly: false,
        openTabs: [
            { path: '/workspace/one.go', kind: 'go', content: 'package one', modified: true },
            { path: '/workspace/two.py', kind: 'python', content: 'print(2)', modified: true }
        ],
        activeTabIndex: 0
    };
    h.instances.set(instance.windowId, instance);
    context.window.__test.registerSessionAction(instance);
    const snapshot = participant.inspect();
    assert.equal(snapshot.dirty, true, 'Code Studio reports modified tabs as dirty');
    assert.equal(snapshot.label, '/workspace/one.go');
    const saving = participant.save(snapshot.revision);
    await new Promise(resolve => setImmediate(resolve));
    assert.deepEqual(Array.from(h.saves), ['/workspace/one.go', '/workspace/two.py'], 'save-all starts persistence for every modified tab');
    h.completeSave.get('/workspace/two.py')(true);
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(participant.inspect().dirty, true, 'one completed tab cannot acknowledge the other pending tab');
    h.completeSave.get('/workspace/one.go')(true);
    const result = await saving;
    assert.equal(result.revision, snapshot.revision, 'Code Studio acknowledges the exact content revision only after all tabs save');
    assert.equal(participant.inspect().dirty, false);
}

// Execute Infra's actual inline participant callbacks without mounting its canvas UI.
{
    const source = await readFile(new URL('../ui/js/desktop/apps/infra.js', import.meta.url), 'utf8');
    const revision = sourceBetween(source, '        const sessionRevision = () => JSON.stringify([', '\n        sessionActionOff =', 'Infra session revision');
    const registration = sourceBetween(source, '        sessionActionOff = ctx.registerSessionAction?.(windowId, {', '\n        ctx.setWindowBeforeClose?.', 'Infra session participant');
    const code = `
        let sessionActionOff = null;
        ${revision}
        ${registration}
        return ctx.participant;
    `;
    const run = new Function('ctx', 'windowId', 't', 'store', 'state', 'app', 'disposed', code);
    let dirty = true;
    let readonly = false;
    let releaseSave;
    let saveCalls = 0;
    let participant = null;
    const ctx = { registerSessionAction: (_id, value) => { participant = value; return () => { participant = null; }; } };
    const state = {
        draftRevision: new Map([['node-a', 3]]),
        sourceRevision: new Map(),
        nodes: [{ id: 'node-a' }]
    };
    const store = {
        hasUnsavedWork: () => dirty,
        canWrite: () => !readonly,
        effectivePlacement: () => ({ x: 20, y: 30 }),
        effectiveParent: () => ''
    };
    const app = { forms: { saveAllDrafts: () => { saveCalls++; return new Promise(resolve => { releaseSave = resolve; }); } } };
    run(ctx, 'infra-test', key => key, store, state, app, false);
    const registered = participant;
    const snapshot = registered.inspect();
    assert.equal(snapshot.dirty, true);
    assert.match(snapshot.label, /infra/);
    const saving = registered.save(snapshot.revision);
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(saveCalls, 1, 'Infra delegates to its awaited multi-draft persistence helper');
    dirty = false;
    releaseSave(true);
    const result = await saving;
    assert.equal(result.revision, snapshot.revision, 'Infra acknowledges only after persistence leaves the same revision clean');
    assert.equal(registered.inspect().dirty, false);

    dirty = true;
    const changedSnapshot = registered.inspect();
    app.forms.saveAllDrafts = async () => {
        state.draftRevision.set('node-a', 4);
        dirty = false;
        return true;
    };
    assert.equal(await registered.save(changedSnapshot.revision), false, 'an edit made during Infra persistence invalidates the acknowledgement');
}

// Exercise EasyDrag's real participant callback against a saver whose server
// revision advances after each successful PUT acknowledgement.
{
    const source = await readFile(new URL('../ui/js/desktop/apps/easydrag-editor.js', import.meta.url), 'utf8');
    const registration = sourceBetween(source,
        '        if (typeof ctx.registerSessionAction === \'function\') {',
        '        // ── start ──',
        'EasyDrag session participant');
    let participant = null;
    let serverRevision = 8;
    let saverDirty = true;
    let releaseFlush = null;
    let flushCalls = 0;
    const draftModel = { doc: { name: 'Scene draft' }, toJSON() { return { ...this.doc, nodes: [{ id: 'node-a', x: 10 }] }; } };
    const saver = {
        get revision() { return serverRevision; },
        isDirty: () => saverDirty,
        flush: () => {
            flushCalls++;
            const submittedContent = JSON.stringify(draftModel.toJSON());
            return new Promise(resolve => { releaseFlush = () => {
                serverRevision++;
                saverDirty = JSON.stringify(draftModel.toJSON()) !== submittedContent;
                resolve(true);
            }; });
        }
    };
    const ed = {
        windowId: 'easy-drag-session', flow: { id: 'flow-a' }, saver,
        readonly: false, runView: false,
        detail: { hasPendingEdits: () => false },
        flushPendingEdits: () => true
    };
    const ctx = { registerSessionAction: (_id, value) => { participant = value; return () => { participant = null; }; } };
    const create = new Function('ctx', 'ed', 'draftModel', 't', `
        let disposed = false;
        let deleting = false;
        let contentDirty = false;
        let sessionEditRevision = 0;
        let sessionActionFlushing = false;
        ${registration}
        return {
            setContentDirty: value => { contentDirty = value; },
            markEdit: () => { sessionEditRevision++; contentDirty = true; },
            markModelChange: () => { if (!sessionActionFlushing) sessionEditRevision++; contentDirty = true; }
        };
    `);
    const controller = create(ctx, ed, draftModel, key => key);
    const savedSnapshot = participant.inspect();
    assert.equal(savedSnapshot.dirty, true);
    const saving = participant.save(savedSnapshot.revision);
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(flushCalls, 1);
    releaseFlush();
    const result = await saving;
    assert.equal(serverRevision, 9, 'the successful PUT advances the saver revision');
    assert.equal(result.revision, savedSnapshot.revision, 'server revision advancement does not invalidate an unchanged content save');
    assert.equal(participant.inspect().dirty, false);

    saverDirty = true;
    const editedSnapshot = participant.inspect();
    const savingWithEdit = participant.save(editedSnapshot.revision);
    await new Promise(resolve => setImmediate(resolve));
    draftModel.doc.name = 'Edited during save';
    controller.markModelChange();
    releaseFlush();
    assert.equal(await savingWithEdit, false, 'a real edit during the request cannot be acknowledged as saved');
    assert.equal(participant.inspect().dirty, true);
    assert.notEqual(participant.inspect().revision, editedSnapshot.revision);
}

// The pure EasyDrag detail snapshot sees edits before debounce/blur without
// flushing them; the phase-two save callback alone commits them.
{
    const source = await readFile(new URL('../ui/js/desktop/apps/easydrag-detail.js', import.meta.url), 'utf8');
    const helpers = sourceBetween(source,
        '        function hasPendingEdits() {',
        '        function renderNote() {',
        'EasyDrag pending edit detection');
    const node = { id: 'node-a', label: 'Step', key: 'step', settings: { notes: 'saved note' }, params: { title: 'saved title' } };
    const noteInput = { value: 'saved note' };
    const keyInput = { value: 'step', dataset: { edNode: 'node-a', edDirty: '' } };
    const paramControl = {
        id: 'title-input', name: '', type: 'text', tagName: 'INPUT', value: 'saved title', checked: false,
        closest: () => ({ dataset: { param: 'title' } })
    };
    const el = { querySelector: selector => selector.includes('textarea') ? noteInput : selector.includes('ed-code') ? keyInput : null };
    const model = { node: id => id === node.id ? node : null };
    const run = new Function('ed', 'el', 'node', 'form', 'labelInput', 'pendingNote', 'pendingFormEdits', helpers + `
        return { hasPendingEdits };
    `);
    const detail = run({ model }, el, node, { el: { querySelectorAll: () => [paramControl] } }, { value: 'Step' }, null, new Set());
    assert.equal(detail.hasPendingEdits(), false);
    noteInput.value = 'typed before debounce';
    assert.equal(detail.hasPendingEdits(), true, 'an immediate logout sees a typed note before its debounce fires');
    noteInput.value = 'saved note';
    keyInput.value = 'renamed-step'; keyInput.dataset.edDirty = '1';
    assert.equal(detail.hasPendingEdits(), true, 'an uncommitted key edit is visible without calling commitKey');
    keyInput.value = 'step'; keyInput.dataset.edDirty = '';
    paramControl.value = 'typed title';
    const formDetail = run({ model }, el, node, { el: { querySelectorAll: () => [paramControl] } }, { value: 'Step' }, null, new Set(['title']));
    assert.equal(formDetail.hasPendingEdits(), true, 'form input remains dirty until phase-two flush');
    assert.equal(node.params.title, 'saved title', 'checking pending edits does not mutate the draft model');
}

// Immediate logout sees the detail pane's uncommitted note, then Save commits
// it only in phase two and keeps the edit generation stable across that flush.
{
    const source = await readFile(new URL('../ui/js/desktop/apps/easydrag-editor.js', import.meta.url), 'utf8');
    const detailSource = await readFile(new URL('../ui/js/desktop/apps/easydrag-detail.js', import.meta.url), 'utf8');
    const registration = sourceBetween(source,
        '        if (typeof ctx.registerSessionAction === \'function\') {',
        '        // ── start ──',
        'EasyDrag pending-note participant');
    const flushNoteSource = sourceBetween(detailSource,
        '        function flushNote() {',
        '        // commitKey writes',
        'EasyDrag note debounce');
    let participant = null;
    let choice = 'cancel';
    let pendingValue = 'typed before debounce';
    let saverDirty = true;
    let flushStarted = false;
    let releaseSave;
    const draftModel = { doc: { name: 'Flow', note: 'saved note' }, toJSON() { return { ...this.doc }; } };
    const saver = {
        isDirty: () => saverDirty,
        flush: () => {
            flushStarted = true;
            return new Promise(resolve => { releaseSave = () => { saverDirty = false; controller.saved(); resolve(true); }; });
        }
    };
    const ed = {
        flow: { id: 'flow-note' }, readonly: false, runView: false, saver,
        model: {
            node: id => id === 'node-a' ? { id } : null,
            setSettings: (_id, settings) => { draftModel.doc.note = settings.notes || ''; controller.markModelChange(); return true; }
        },
        detail: { hasPendingEdits: () => pendingValue !== null },
        flushPendingEdits: () => {
            if (pendingValue !== null) draftModel.doc.note = pendingValue;
            pendingValue = null;
            controller.markModelChange();
            return true;
        }
    };
    const ctx = {
        registerSessionAction: (_id, value) => { participant = value; return () => {}; },
        decideSessionAction: () => choice
    };
    const create = new Function('ctx', 'ed', 'draftModel', 't', `
        let disposed = false;
        let deleting = false;
        let contentDirty = false;
        let sessionEditRevision = 1;
        let sessionActionFlushing = false;
        ${registration}
        return {
            markModelChange: () => { if (!sessionActionFlushing) sessionEditRevision++; contentDirty = true; },
            markInput: () => { if (!sessionActionFlushing) sessionEditRevision++; },
            sessionActionCommit: fn => { const wasFlushing = sessionActionFlushing; sessionActionFlushing = true; try { return fn(); } finally { sessionActionFlushing = wasFlushing; } },
            saved: () => { contentDirty = false; },
            get revision() { return sessionEditRevision; }
        };
    `);
    const controller = create(ctx, ed, draftModel, key => key);
    ed.sessionActionCommit = controller.sessionActionCommit;
    const noteFlusher = new Function('ed', 'saveNote', 'node', `
        let pendingNote = null;
        ${flushNoteSource}
        return { flushNote, setPending: value => { pendingNote = value === null ? null : { id: node.id, value }; } };
    `)(ed, { cancel() {} }, { id: 'node-a' });
    const snapshot = participant.inspect();
    assert.equal(snapshot.dirty, true, 'pending note is dirty even before the debounce commits it');
    assert.equal(participant.decide(snapshot), 'cancel', 'Cancel preserves the editor draft without flushing');
    assert.equal(draftModel.doc.note, 'saved note');
    assert.equal(flushStarted, false);

    choice = 'save';
    const saving = participant.save(snapshot.revision);
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(draftModel.doc.note, 'typed before debounce', 'Save flushes the visible note into the model');
    assert.equal(controller.revision, 1, 'phase-two pending-input commit is not counted as a new edit');
    assert.equal(flushStarted, true);
    releaseSave();
    const saved = await saving;
    assert.equal(saved.revision, snapshot.revision);
    assert.equal(participant.inspect().dirty, false);

    // The detail pane's real 400 ms timer may commit while the logout decision is open.
    // That transfer into the model is still the same user edit, while a later input is not.
    saverDirty = true;
    pendingValue = 'note committed by natural debounce';
    noteFlusher.setPending(pendingValue);
    controller.markInput();
    const debounceSnapshot = participant.inspect();
    await new Promise(resolve => setImmediate(resolve));
    noteFlusher.flushNote();
    pendingValue = null;
    assert.equal(draftModel.doc.note, 'note committed by natural debounce');
    assert.equal(controller.revision, JSON.parse(debounceSnapshot.revision)[1], 'the natural note timer does not turn a transfer into a second edit');
    const savingDebounced = participant.save(debounceSnapshot.revision);
    await new Promise(resolve => setImmediate(resolve));
    releaseSave();
    assert.equal((await savingDebounced).revision, debounceSnapshot.revision, 'a timer commit during the decision can still be saved');

    saverDirty = true;
    pendingValue = 'another pending note';
    controller.markInput();
    let releaseLate;
    saver.flush = () => new Promise(resolve => { releaseLate = () => { saverDirty = false; controller.saved(); resolve(true); }; });
    const lateSnapshot = participant.inspect();
    const savingLate = participant.save(lateSnapshot.revision);
    await new Promise(resolve => setImmediate(resolve));
    pendingValue = 'edited while save is pending';
    controller.markInput();
    releaseLate();
    assert.equal(await savingLate, false, 'a real note edit after phase-two flush prevents acknowledgement');

    saverDirty = true;
    pendingValue = 'save will fail';
    controller.markInput();
    saver.flush = async () => false;
    const failed = participant.inspect();
    assert.equal(await participant.save(failed.revision), false, 'failed persistence keeps logout blocked');
}

// Video Studio's edit generation subtracts only the revision caused by its
// own derived artwork commit. Ordinary gestures and edits during either await
// still invalidate the acknowledgement.
{
    const source = await readFile(new URL('../ui/js/desktop/apps/video-studio.js', import.meta.url), 'utf8');
    const stageSource = await readFile(new URL('../ui/js/desktop/apps/video-studio-stage.js', import.meta.url), 'utf8');
    const gestureTracker = sourceBetween(source,
        '    function trackTimelineDragging(s) {',
        '    function appRender(host, id, ctx) {',
        'Video Studio timeline gesture tracking');
    const liveValues = sourceBetween(stageSource,
        '    function liveValues(s, found, values, holder) {',
        '    function liveField(s, input) {',
        'Video Studio live transform values');
    const tracking = new Function('s', gestureTracker + '\nreturn trackTimelineDragging(s);');
    const liveValueUpdate = new Function('window', 's', 'found', 'values', 'holder', liveValues + '\nreturn liveValues(s, found, values, holder);');
    const registration = sourceBetween(source,
        '        const sessionRevision = () => JSON.stringify([s.projectId, s.projectEpoch, s.revision - s.sessionDerivedRevision, s.sessionActionRevision]);',
        '        if (ctx.setWindowMenus)',
        'Video Studio session participant');
    let participant = null;
    const ctx = { registerSessionAction: (_id, value) => { participant = value; ctx.participant = value; return () => {}; } };
    const s = {
        projectId: 'project-a', projectEpoch: 2, revision: 5, sessionActionRevision: 0, sessionDerivedRevision: 0,
        project: { assets: [] }, dirty: false, saving: false, savePromise: null,
        pendingArtwork: new Map(), textTimers: new Map(), staleArtwork: new Set(), artworkInFlight: new Set(),
        readonly: false, timelineDragging: false, disposed: false
    };
    tracking(s);
    const transformClip = { id: 'clip-transform', x: 0.1, y: 0.2, width: 0.4, height: 0.3 };
    const transformProject = { tracks: [{ clips: [transformClip] }] };
    const transformWindow = { VideoStudioKeyframeEdit: { setValues: (_project, _clipId, values) => { Object.assign(transformClip, values); return true; } } };
    const revisionBeforeTransform = s.revision;
    const sessionRevisionBeforeTransform = s.sessionActionRevision;
    liveValueUpdate(transformWindow, s, { clip: transformClip }, { x: 0.25 }, {});
    assert.equal(transformClip.x, 0.25, 'a live transform changes the clip before pointerup commits history');
    assert.equal(s.revision, revisionBeforeTransform, 'live transform movement does not alter the project save generation');
    assert.equal(s.sessionActionRevision, sessionRevisionBeforeTransform + 1, 'live transform movement advances the logout revision');
    liveValueUpdate(transformWindow, s, { clip: transformClip }, { x: 0.25 }, {});
    assert.equal(s.sessionActionRevision, sessionRevisionBeforeTransform + 1, 'a no-op transform update does not advance the session revision');
    const register = new Function('s', 'ctx', 'id', 'P', 'saveUntilClean', 'tr', registration + '\nreturn ctx.participant;');
    let flushArtwork = async () => {};
    let saveImpl = async current => { current.dirty = false; return true; };
    const getParticipant = () => register(s, ctx, 'video-studio-test', { flushArtwork: (...args) => flushArtwork(...args) }, current => saveImpl(current), key => key);
    const action = getParticipant();
    const cleanSnapshot = action.inspect();
    s.timelineDragging = true;
    assert.equal(action.inspect().dirty, true, 'a live gesture before its debounced save remains visible');
    const activeGestureSnapshot = action.inspect();
    assert.notEqual(activeGestureSnapshot.revision, cleanSnapshot.revision, 'starting a gesture invalidates an earlier snapshot');
    assert.equal(await action.save(action.inspect().revision), false, 'logout cannot acknowledge an unfinished transform gesture');
    assert.equal(s.revision, revisionBeforeTransform, 'gesture state stays separate from the project persistence revision');
    s.timelineDragging = false;
    assert.notEqual(action.inspect().revision, cleanSnapshot.revision, 'ending a gesture also advances the session revision');

    s.dirty = true;
    let finishFlush;
    flushArtwork = async current => {
        await new Promise(resolve => { finishFlush = resolve; });
        current.project.assets.push({ id: 'generated-title' });
        current.revision++;
        current.sessionDerivedRevision++;
    };
    const artworkSnapshot = action.inspect();
    const savingArtwork = action.save(artworkSnapshot.revision);
    await new Promise(resolve => setImmediate(resolve));
    finishFlush();
    const artworkResult = await savingArtwork;
    assert.equal(artworkResult.revision, artworkSnapshot.revision, 'the participant accepts a committed generated asset as part of the same user edit');
    assert.equal(action.inspect().dirty, false);

    s.dirty = true;
    flushArtwork = async current => {
        await new Promise(resolve => { finishFlush = resolve; });
        current.project.userTitle = 'late edit';
        current.revision++;
    };
    const editDuringFlush = action.inspect();
    const savingDuringFlush = action.save(editDuringFlush.revision);
    await new Promise(resolve => setImmediate(resolve));
    s.revision++;
    finishFlush();
    assert.equal(await savingDuringFlush, false, 'a real edit during artwork flush blocks logout');

    s.dirty = true;
    flushArtwork = async () => {};
    let finishSave;
    saveImpl = async () => new Promise(resolve => { finishSave = resolve; });
    const editDuringSave = action.inspect();
    const savingWhileEditing = action.save(editDuringSave.revision);
    await new Promise(resolve => setImmediate(resolve));
    s.revision++;
    finishSave(true);
    assert.equal(await savingWhileEditing, false, 'a real edit during server persistence blocks logout');

    s.dirty = true;
    saveImpl = async () => false;
    const failedSnapshot = action.inspect();
    assert.equal(await action.save(failedSnapshot.revision), false, 'a failed Video Studio write cannot acknowledge logout');
}

// Layerling permits a new untitled document to use its existing Save As flow.
// Choosing the first path changes the queue revision, but not the edit identity.
{
    const source = await readFile(new URL('../ui/js/desktop/apps/layerling.js', import.meta.url), 'utf8');
    const registration = sourceBetween(source,
        '        const sessionRevision = () => JSON.stringify([s.sessionDocumentEpoch, s.sessionEditRevision]);',
        '        ctx.registerWindowCleanup?.',
        'Layerling session participant');
    let participant = null;
    const ctx = { registerSessionAction: (_id, value) => { participant = value; ctx.participant = value; return () => {}; } };
    const signal = { aborted: false };
    const s = {
        sessionDocumentEpoch: 0, sessionEditRevision: 4, path: '', loading: false, readonly: false,
        queue: { revision: 3, dirty: true }
    };
    let selectedTarget = 'Documents/Untitled.lyl';
    let chooseSaveImpl = async () => selectedTarget;
    let saveCalls = [];
    let saveImpl = async target => {
        saveCalls.push(target);
        s.path = target;
        s.queue.revision++;
        s.queue.dirty = false;
        return true;
    };
    const register = new Function('s', 'ctx', 'id', 'signal', 't', 'chooseSave', 'save', registration + '\nreturn ctx.participant;');
    const action = register(s, ctx, 'layerling-test', signal, key => key, (...args) => chooseSaveImpl(...args), (...args) => saveImpl(...args));
    const fresh = action.inspect();
    assert.equal(fresh.dirty, true);
    const saved = await action.save(fresh.revision);
    assert.equal(saved.revision, fresh.revision, 'Save As path assignment is not treated as a document edit');
    assert.deepEqual(saveCalls, ['Documents/Untitled.lyl'], 'the participant uses the normal Save As writer');
    assert.equal(action.inspect().revision, fresh.revision);

    s.path = ''; s.queue.dirty = true; s.queue.revision++;
    let chooseTarget;
    chooseSaveImpl = () => new Promise(resolve => { chooseTarget = resolve; });
    const duringPicker = action.inspect();
    const choosing = action.save(duringPicker.revision);
    await new Promise(resolve => setImmediate(resolve));
    s.sessionEditRevision++;
    chooseTarget('Documents/Untitled.lyl');
    assert.equal(await choosing, false, 'edits made while the destination picker is open invalidate Save');

    s.path = 'Documents/Scene.lyl'; s.queue.dirty = true; s.sessionEditRevision++;
    let finishSave;
    saveImpl = async () => new Promise(resolve => { finishSave = resolve; });
    const duringWrite = action.inspect();
    const writing = action.save(duringWrite.revision);
    await new Promise(resolve => setImmediate(resolve));
    s.sessionEditRevision++;
    s.dirty = false;
    finishSave(true);
    assert.equal(await writing, false, 'edits made while persistence is pending invalidate acknowledgement');

    s.queue.dirty = true; s.sessionEditRevision++;
    saveImpl = async () => { throw new Error('write failed'); };
    const failed = action.inspect();
    assert.equal(await action.save(failed.revision), false, 'a failed Layerling write blocks logout');
}

{
    let releaseFirst;
    let dirty = true;
    const calls = [];
    const pendingFirst = new Promise(resolve => { releaseFirst = resolve; });
    const saving = saveSessionDrafts({
        nodeIDs: ['node-a', 'node-b'],
        sourceIDs: ['source-a'],
        views: ['atlas', 'topology'],
        saveNode: async id => { calls.push('node:start:' + id); if (id === 'node-a') await pendingFirst; calls.push('node:done:' + id); return true; },
        saveSource: async id => { calls.push('source:' + id); return true; },
        retryLayout: async view => { calls.push('retry:' + view); return true; },
        whenLayoutIdle: async view => { calls.push('idle:' + view); return true; },
        hasUnsavedWork: () => dirty
    });
    await new Promise(resolve => setImmediate(resolve));
    assert.deepEqual(calls, ['node:start:node-a'], 'the coordinator waits for a pending draft write before starting the next one');
    dirty = false;
    releaseFirst();
    assert.equal(await saving, true);
    assert.deepEqual(calls, [
        'node:start:node-a', 'node:done:node-a', 'node:start:node-b', 'node:done:node-b',
        'source:source-a', 'retry:atlas', 'idle:atlas', 'retry:topology', 'idle:topology'
    ]);
}

{
    const calls = [];
    const saved = await saveSessionDrafts({
        nodeIDs: ['node-a', 'node-b'],
        sourceIDs: ['source-a'],
        views: ['atlas'],
        saveNode: async id => { calls.push('node:' + id); if (id === 'node-b') throw new Error('write rejected'); return true; },
        saveSource: async id => { calls.push('source:' + id); return true; },
        retryLayout: async view => { calls.push('retry:' + view); return true; },
        whenLayoutIdle: async () => true,
        hasUnsavedWork: () => false
    });
    assert.equal(saved, false, 'a rejected save must keep logout blocked');
    assert.deepEqual(calls, ['node:node-a', 'node:node-b'], 'later drafts and layout writes must not run after a failed save');
}

console.log('Desktop session adapter persistence tests passed');
