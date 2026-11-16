// W1c Part B checks (the editor header): the facts the editor keeps from the server (B.1), the kit popover (B.2), the pill and
// its popover (B.3, B.4) and the primary button (B.5). They run on the c1d07 sandbox (test-easydrag-extra3.mjs returns sandbox
// and openEditor); test-easydrag.mjs calls run(env) with its helpers and counts the failures. Alone this file only has to import.
import fs from 'node:fs';
import path from 'node:path';

const T1 = 'n_tttttttt';
const A = 'n_aaaaaaaa';

// flowDoc is a flow start -> alpha.
function flowDoc() {
    return {
        schema: 1, name: 'Flow',
        nodes: [
            { id: T1, key: 'start', type: 'trigger.manual', label: 'Start', position: { x: 0, y: 0 }, params: {}, settings: {} },
            { id: A, key: 'alpha', type: 'web.search', label: 'Alpha', position: { x: 300, y: 0 }, params: { query: 'x' }, settings: {} }
        ],
        edges: [{ id: 'e1', source: { node: T1, port: 'out' }, target: { node: A, port: 'in' } }]
    };
}

export async function run(env) {
    const { apps, eq, guardAsync, settle, sandbox, openEditor } = env;

    // ── B.1: the facts of the server ──

    // recordOf is the flow record the stub server answers with: published as v2, the draft at revision `rev`.
    const recordOf = rev => ({ id: 'f1', name: 'Flow', draft: flowDoc(), draft_revision: rev, published_draft_revision: 2, live: flowDoc(), live_revision: 2 });

    await guardAsync('w1c B.1 the editor keeps next_run, tested and changes from the read and from every save', async () => {
        let rev = 3;
        const get = { tested: true, changes: { steps: 2, connections: 1 }, next_run: { at: '2026-11-17T07:00:00+01:00', node: T1 } };
        const put = { tested: false, changes: { steps: 3, connections: 1 } };
        const h = sandbox(req => {
            if (req.method === 'GET' && req.url === '/api/desktop/flows/f1') return Object.assign({ flow: recordOf(rev), enabled: true, issues: [] }, get);
            if (req.method === 'PUT') { rev = 4; return Object.assign({ draft_revision: 4, issues: [] }, put); }
            return undefined;
        });
        const editor = openEditor(h, { flow: recordOf(3), enabled: true, facts: get });
        await settle();
        const ED = h.ED, flow = () => editor.ed.flow;
        const first = [flow().tested, flow().changes, flow().next_run, ED.flowState.isTested(editor.ed), ED.flowState.changeCount(editor.ed)];
        // An edit makes the draft untested at once (before the save), and the save answer brings the new facts; it says nothing of the schedule.
        editor.ed.model.setFlow({ description: 'changed' });
        const dirtyTested = ED.flowState.isTested(editor.ed);
        h.runTimers(1000);
        await settle();
        const saved = [flow().tested, flow().changes, flow().next_run, ED.flowState.isTested(editor.ed), ED.flowState.changeCount(editor.ed), editor.ed.saver.state];
        // A re-read for revision 4 that says "tested" counts; one for another revision does not.
        get.tested = true; get.changes = { steps: 5, connections: 0 }; delete get.next_run;
        h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason: 'checked' } });
        await settle();
        const reread = [flow().tested, flow().changes, flow().next_run === undefined, ED.flowState.isTested(editor.ed)];
        rev = 9;
        h.fireDoc('aurago:flows-changed', { detail: { flow_id: 'f1', reason: 'checked' } });
        await settle();
        const elsewhere = [ED.flowState.isTested(editor.ed), ED.flowState.changeCount(editor.ed)];
        // A restored version replaces the draft: the old flag and count go.
        editor.ed.bus.emit('restored', { flow: Object.assign(recordOf(10), { draft: flowDoc() }), issues: [] });
        await settle();
        const restored = [flow().tested, flow().changes === undefined, ED.flowState.isTested(editor.ed)];
        eq('w1c B.1 the facts follow the read, the save, a re-read and a restore', [first, dirtyTested, saved, reread, elsewhere, restored, h.logged], [
            [true, { steps: 2, connections: 1 }, { at: '2026-11-17T07:00:00+01:00', node: T1 }, true, 3],
            false,
            [false, { steps: 3, connections: 1 }, { at: '2026-11-17T07:00:00+01:00', node: T1 }, false, 4, 'saved'],
            [true, { steps: 5, connections: 0 }, true, true],
            [true, 5],
            [false, true, false],
            []
        ]);
        editor.dispose();
    });

    await guardAsync('w1c B.1 the editor\'s own test evidence counts for the exact revision, and a failed run does not', async () => {
        const ED = sandbox(() => undefined).ED;
        const ed = ok => ({ flow: { tested: false, draft_revision: 5 }, flowFactsRevision: 5, saver: { revision: 5, isDirty: () => false }, testEvidence: { revision: 5, status: ok } });
        const other = Object.assign(ed('success'), { testEvidence: { revision: 4, status: 'success' } });
        const dirty = Object.assign(ed('success'), { saver: { revision: 5, isDirty: () => true } });
        const staleFlag = { flow: { tested: true, draft_revision: 5 }, flowFactsRevision: 4, saver: { revision: 5, isDirty: () => false } };
        eq('w1c B.1 isTested: success of this revision, a failure, another revision, an unsaved edit, a flag for an older revision',
            [ED.flowState.isTested(ed('success')), ED.flowState.isTested(ed('error')), ED.flowState.isTested(other), ED.flowState.isTested(dirty), ED.flowState.isTested(staleFlag)],
            [true, false, false, false, false]);
        eq('w1c B.1 changeCount adds steps and connections and is null without the server\'s count',
            [ED.flowState.changeCount({ flow: { changes: { steps: 2, connections: 3 } }, flowFactsRevision: 0 }), ED.flowState.changeCount({ flow: {}, flowFactsRevision: 0 }), ED.flowState.changeCount({ flow: { changes: { steps: 'x' } }, flowFactsRevision: 0 }), ED.flowState.changeCount({ flow: { changes: { steps: 2 } }, flowFactsRevision: 1 })],
            [5, null, 0, null]);
    });

    await guardAsync('w1c B.1 the change count is null once the saver revision moved without the facts: an external acknowledgement, a conflict reload', async () => {
        const facts = { tested: true, changes: { steps: 2, connections: 1 } };
        const conflict = Object.assign(new Error('conflict'), { status: 409, body: { code: 'FLOW_REVISION_CONFLICT' } });
        let mode = '';
        const h = sandbox(req => {
            if (req.method === 'GET' && req.url === '/api/desktop/flows/f1') return Object.assign({ flow: recordOf(mode === 'conflict' ? 7 : 3), enabled: true, issues: [] }, facts);
            if (req.method === 'PUT' && mode === 'conflict') throw conflict;
            return undefined;
        });
        const editor = openEditor(h, { flow: recordOf(3), enabled: true, facts });
        await settle();
        const ED = h.ED, ed = editor.ed;
        const before = ED.flowState.changeCount(ed);
        const ticket = ed.saver.beginExternal(ed.saver.revision, ed.model.version);
        const acknowledged = ed.saver.acknowledgeExternal(ticket, ed.saver.revision + 1, () => ed.model.setFlow({ description: 'external' }), []);
        await settle();
        const afterAck = [acknowledged, ED.flowState.changeCount(ed), ED.flowState.isTested(ed)];
        mode = 'conflict';
        ed.model.setFlow({ description: 'mine' });
        h.runTimers(1000);
        await settle();
        const dialog = h.dialogs[0];
        if (dialog) dialog.el.querySelector('[data-ed-action="reload"]').fire('click');
        await settle();
        eq('w1c B.1 the count follows the revision: shown, then null after an acknowledgement and after a conflict reload',
            [before, afterAck, ed.saver.revision, ED.flowState.changeCount(ed), h.logged], [3, [true, null, false], 7, null, []]);
        editor.dispose();
    });

    // ── B.2: the kit popover ──

    await guardAsync('w1c B.2 ED.kit.popover: focus moves in; Escape, a press outside and a focus change close it; Escape gives the focus back', async () => {
        const h = sandbox(() => undefined);
        const { ED } = h, doc = h.dom.document, El = h.dom.El;
        const anchor = new El('button', {});
        const outside = new El('button', {});
        h.body.appendChild(anchor);
        h.body.appendChild(outside);
        const closed = [];
        const open = () => ED.kit.popover(anchor, {
            content: ED.core.el('<div><button type="button" data-ed-pop="a">A</button><button type="button" data-ed-pop="b">B</button></div>'),
            label: 'Status', onClose: info => closed.push(info.restore)
        });
        let pop = open();
        const opened = [pop.el.getAttribute('role'), pop.el.getAttribute('aria-label'), h.body.contains(pop.el), doc.activeElement === pop.el.querySelector('[data-ed-pop="a"]'), pop.isOpen()];
        const key = pop.el.fire('keydown', { key: 'Escape' });
        const byEscape = [h.body.contains(pop.el), doc.activeElement === anchor, closed.slice(), key.defaultPrevented, key.stopped, pop.isOpen()];
        pop = open();
        h.fireDoc('pointerdown', { target: anchor });
        const onAnchor = [pop.isOpen()];
        h.fireDoc('pointerdown', { target: pop.el.querySelector('[data-ed-pop="b"]') });
        const inside = [pop.isOpen()];
        outside.focus();
        h.fireDoc('pointerdown', { target: outside });
        const byPress = [pop.isOpen(), h.body.contains(pop.el), doc.activeElement === outside, closed.slice()];
        pop = open();
        pop.el.fire('focusout', { relatedTarget: pop.el.querySelector('[data-ed-pop="b"]') });
        const withinFocus = [pop.isOpen()];
        pop.el.fire('focusout', { relatedTarget: outside });
        const byFocus = [pop.isOpen(), closed.slice()];
        pop.close(true);
        eq('w1c B.2 popover', [opened, byEscape, onAnchor, inside, byPress, withinFocus, byFocus, h.docListeners.pointerdown.length, h.logged], [
            ['dialog', 'Status', true, true, true],
            [false, true, [true], true, true, false],
            [true], [true],
            [false, false, true, [true, false]],
            [true],
            [false, [true, false, false]],
            0, []
        ]);
    });

    await guardAsync('w1c B.2 ED.kit.popover: Tab past the last entry and Shift+Tab before the first close it and give the focus to the anchor', async () => {
        const h = sandbox(() => undefined);
        const { ED } = h, doc = h.dom.document, El = h.dom.El;
        const anchor = new El('button', {});
        h.body.appendChild(anchor);
        const open = () => ED.kit.popover(anchor, { content: ED.core.el('<div><button type="button" data-ed-pop="a">A</button><button type="button" data-ed-pop="b">B</button></div>'), label: 'Status' });
        let pop = open();
        const a = pop.el.querySelector('[data-ed-pop="a"]'), b = pop.el.querySelector('[data-ed-pop="b"]');
        const inner = [pop.el.fire('keydown', { key: 'Tab' }).defaultPrevented, pop.isOpen()];
        b.focus();
        const last = pop.el.fire('keydown', { key: 'Tab' });
        const afterTab = [last.defaultPrevented, pop.isOpen(), doc.activeElement === anchor];
        pop = open();
        const first = pop.el.fire('keydown', { key: 'Tab', shiftKey: true });
        const afterShift = [first.defaultPrevented, pop.isOpen(), doc.activeElement === anchor];
        eq('w1c B.2 popover Tab edges', [inner, afterTab, afterShift, h.logged], [[false, true], [true, false, true], [true, false, true], []]);
    });

    await guardAsync('w1c B.2 ED.kit.popover.update swaps the content and keeps the focus on the same control', async () => {
        const h = sandbox(() => undefined);
        const { ED } = h, doc = h.dom.document;
        const anchor = new h.dom.El('button', {});
        h.body.appendChild(anchor);
        const pop = ED.kit.popover(anchor, { content: ED.core.el('<div><button type="button" data-ed-pop="a">A</button><button type="button" data-ed-pop="b">B</button></div>') });
        pop.el.querySelector('[data-ed-pop="b"]').focus();
        pop.update(ED.core.el('<div><button type="button" data-ed-pop="a">A2</button><button type="button" data-ed-pop="b">B2</button></div>'));
        eq('w1c B.2 popover update', [doc.activeElement === pop.el.querySelector('[data-ed-pop="b"]'), pop.el.querySelectorAll('button').length, h.body.contains(pop.el)], [true, 2, true]);
        pop.close(false);
    });

    // ── B.3: the pill and its popover ──

    // pillOf reads the pill: [kind, status, text].
    const pillOf = editor => {
        const b = editor.el.querySelector('[data-ed-pill]');
        return [b.getAttribute('data-ed-pill-kind'), b.getAttribute('data-status'), b.querySelector('[data-ed-pill-text]').textContent];
    };
    // show sets what the editor knows and paints the header again (the bus event of a switch does that).
    const show = (editor, patch, flow) => {
        Object.assign(editor.ed, patch);
        Object.assign(editor.ed.flow, flow || {});
        editor.ed.bus.emit('enabled');
        return pillOf(editor);
    };
    // publishedFlow opens f1 as v2 with a stub server: world.enabled and world.next are what GET answers; the switch and Run now answer too.
    const publishedFlow = (world, opts) => {
        const h = sandbox(req => {
            if (req.method === 'GET' && req.url === '/api/desktop/flows/f1') return Object.assign({ flow: recordOf(2), enabled: world.enabled, issues: [] }, world.next ? { next_run: { at: world.next, node: T1 } } : {});
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/enabled') { world.enabled = !!req.body.enabled; return { enabled: world.enabled }; }
            if (req.method === 'POST' && req.url === '/api/desktop/flows/f1/run') return { run_id: 'r9' };
            if (req.method === 'PUT') return { draft_revision: 4, issues: [] };
            return undefined;
        });
        // world.parked answers the run controller's hasParked (a live run of this flow behind a review).
        const create = h.ED.pill.create;
        h.ED.pill.create = host => { if (world.parked !== undefined) host.runs.hasParked = () => !!world.parked; return create(host); };
        const editor = openEditor(h, Object.assign({ flow: recordOf(2), enabled: world.enabled }, opts));
        return { h, editor };
    };

    await guardAsync('w1c B.3 the pill says one thing, in the fixed order: run, past run, problem, pause, version', async () => {
        const world = { enabled: true, parked: false };
        const { h, editor } = publishedFlow(world);
        await settle();
        const live = { status: 'success', revision: 2, started_at: '2026-11-16T10:00:00Z' };
        const rows = [];
        rows.push(['live', pillOf(editor)]);
        rows.push(['draft ahead', show(editor, {}, { draft_revision: 5 })]);
        rows.push(['paused', show(editor, { flowEnabled: false })]);
        rows.push(['timers over paused', show(editor, { timersRunning: false })]);
        rows.push(['outdated over timers', show(editor, { liveIssues: [{ code: 'FLOW_SCHEDULE_INVALID' }], liveIssuesRevision: 2 })]);
        rows.push(['link over outdated', show(editor, { liveIssues: [{ code: 'FLOW_SCHEDULE_INVALID' }, { code: 'FLOW_MISSION_MISSING' }], liveIssuesRevision: 2 })]);
        rows.push(['partial over link', show(editor, { publishIncomplete: 'FLOW_PUBLISH_INCOMPLETE' })]);
        rows.push(['past run over problems', show(editor, { runView: { run: live } })]);
        world.parked = true;
        rows.push(['a past run with the own run parked', show(editor, {})]);
        world.parked = false;
        rows.push(['a running viewed run over everything', show(editor, { runView: { run: Object.assign({}, live, { status: 'running' }) } })]);
        rows.push(['mission missing alone', show(editor, { runView: null, liveIssues: [], publishIncomplete: 'FLOW_MISSION_MISSING' })]);
        rows.push(['all clear', show(editor, { publishIncomplete: '', timersRunning: true, flowEnabled: true }, { draft_revision: 2 })]);
        const draft = openEditor(sandbox(() => undefined));
        await settle();
        rows.push(['never published', pillOf(draft)]);
        draft.dispose();
        const dsc = editor.el.querySelector('[data-ed-pill-disc]');
        eq('w1c B.3 pill rows (kind, disc, text)', rows, [
            ['live', ['live', 'ok', 'pill_live:2']],
            ['draft ahead', ['changed', 'ok', 'pill_live_changed:2']],
            ['paused', ['paused', 'paused', 'pill_paused:2']],
            ['timers over paused', ['timers', 'uncertain', 'timers_stopped_title']],
            ['outdated over timers', ['outdated', 'blocked', 'state_live_outdated']],
            ['link over outdated', ['link', 'blocked', 'pill_link_missing']],
            ['partial over link', ['partial', 'blocked', 'state_publish_incomplete']],
            ['past run over problems', ['past', 'none', 'state_run_view']],
            ['a past run with the own run parked', ['running', 'running', 'running']],
            ['a running viewed run over everything', ['running', 'running', 'running']],
            ['mission missing alone', ['link', 'blocked', 'pill_link_missing']],
            ['all clear', ['live', 'ok', 'pill_live:2']],
            ['never published', ['draft', 'not_run', 'pill_draft']]
        ]);
        eq('w1c B.3 the pill draws a status disc (none for a past run), is a button with a popover, and the old chips and revision text are gone', [
            dsc.html.includes('data-status="ok"'), editor.el.querySelector('[data-ed-pill]').localName, editor.el.querySelector('[data-ed-pill]').getAttribute('aria-haspopup'),
            editor.el.querySelectorAll('[data-ed-state] .ed-chip').length, !!editor.el.querySelector('[data-ed-revision]'), h.logged], [true, 'button', 'dialog', 0, false, []]);
        editor.dispose();
    });

    await guardAsync('w1c B.3 the pill tooltip carries the hint of a problem, the revision facts stay on the pill', async () => {
        const { editor } = publishedFlow({ enabled: true });
        await settle();
        const pill = editor.el.querySelector('[data-ed-pill]');
        const plain = [pill.getAttribute('title'), pill.dataset.kind, pill.dataset.draft, pill.dataset.live, pill.dataset.paused];
        show(editor, { publishIncomplete: 'FLOW_PUBLISH_INCOMPLETE' });
        const partial = pill.getAttribute('title');
        show(editor, { publishIncomplete: '', flowEnabled: false }, { draft_revision: 5 });
        const paused = [pill.dataset.kind, pill.dataset.draft, pill.dataset.paused];
        eq('w1c B.3 pill title and revision facts', [plain, partial, paused], [
            ['pill_live:2', 'live', '2', '2', 'false'],
            'state_publish_incomplete ' + String.fromCodePoint(0x2013) + ' error_flow_publish_incomplete',
            ['newer', '5', 'true']
        ]);
        editor.dispose();
    });

    await guardAsync('w1c B.3 the popover: state, Active, next run, Run now and Runs; the next run is read when it opens', async () => {
        const world = { enabled: true, next: null };
        const { h, editor } = publishedFlow(world);
        await settle();
        const ED = h.ED, pill = editor.el.querySelector('[data-ed-pill]');
        const pop = () => editor.el.querySelector('[data-ed-pill-popover]');
        const entries = () => Array.from(pop().querySelectorAll('[data-ed-pop]')).map(b => b.getAttribute('data-ed-pop'));
        const gets = () => h.requests.filter(r => r.method === 'GET' && r.url === '/api/desktop/flows/f1').length;
        const getsBefore = gets();
        world.next = '2026-11-17T12:00:00Z';
        pill.fire('click');
        const opened = [!!pop(), pill.getAttribute('aria-expanded'), pop().getAttribute('role'), h.dom.document.activeElement === pop().querySelector('[data-ed-pop="active"]'), entries(),
            pop().querySelector('[data-ed-next-run]').textContent, pop().querySelector('[data-ed-pop="active"]').getAttribute('aria-checked')];
        await settle();
        // The read that came with the open brought the next run; the popover shows it as weekday and time, and keeps the focus.
        const next = pop().querySelector('[data-ed-next-run]').textContent;
        const nextOk = /^Tue/.test(next) && /\d:\d\d/.test(next);
        const later = ED.format.nextRun('2026-11-27T12:00:00Z', new Date('2026-11-16T10:00:00Z'));
        const soon = ED.format.nextRun('2026-11-17T12:00:00Z', new Date('2026-11-16T10:00:00Z'));
        eq('w1c B.3 popover contents and the read on open', [opened, gets() - getsBefore, nextOk, /Nov/.test(later) && !/Tue|Fri/.test(later), /^Tue/.test(soon), ED.format.nextRun('nope'), editor.ed.flow.next_run.at, h.dom.document.activeElement === pop().querySelector('[data-ed-pop="active"]')], [
            [true, 'true', 'dialog', true, ['active', 'run-now', 'runs'], 'pill_next_none', 'true'], 1, true, true, true, '', '2026-11-17T12:00:00Z', true
        ]);
        pill.fire('click');
        eq('w1c B.3 a second click on the pill closes the popover and gives the focus back', [!!pop(), pill.getAttribute('aria-expanded'), h.dom.document.activeElement === pill], [false, 'false', true]);
        editor.dispose();
    });

    await guardAsync('w1c B.3 the popover says paused, none and the reasons of the registry', async () => {
        const rows = [];
        const paused = publishedFlow({ enabled: false, next: '2026-11-17T12:00:00Z' });
        await settle();
        paused.editor.el.querySelector('[data-ed-pill]').fire('click');
        const p = paused.editor.el.querySelector('[data-ed-pill-popover]');
        rows.push(['paused', p.querySelector('[data-ed-next-run]').textContent, p.querySelector('[data-ed-pop="active"]').getAttribute('aria-checked'),
            p.querySelector('[data-ed-pop="run-now"]').getAttribute('aria-disabled'), p.querySelector('[data-ed-pop="run-now"]').getAttribute('aria-description'),
            p.querySelector('[data-ed-pop="run-now"]').querySelector('.ed-pop-reason') !== null]);
        paused.editor.dispose();
        const ro = openEditor(sandbox(() => undefined), { readonly: true, flow: recordOf(2), enabled: true });
        await settle();
        ro.el.querySelector('[data-ed-pill]').fire('click');
        const r = ro.el.querySelector('[data-ed-pill-popover]');
        rows.push(['read-only', r.querySelector('[data-ed-pop="active"]').getAttribute('aria-disabled'), r.querySelector('[data-ed-pop="active"]').getAttribute('aria-description'),
            r.querySelector('[data-ed-pop="runs"]').getAttribute('aria-disabled')]);
        ro.dispose();
        const draft = openEditor(sandbox(() => undefined));
        await settle();
        draft.el.querySelector('[data-ed-pill]').fire('click');
        const d = draft.el.querySelector('[data-ed-pill-popover]');
        rows.push(['never published', d.querySelector('[data-ed-next-run]').textContent, !!d.querySelector('[data-ed-pop="run-now"]'), !!d.querySelector('[data-ed-pop="active"]'), !!d.querySelector('[data-ed-pop="runs"]')]);
        draft.dispose();
        eq('w1c B.3 popover reasons', rows, [
            ['paused', 'state_inactive', 'false', 'true', 'reason_paused', true],
            ['read-only', 'true', 'reason_readonly', null],
            ['never published', 'pill_next_none', false, true, true]
        ]);
    });

    await guardAsync('w1c B.3 the entries act through the registry: Active stays open, Run now and Runs close the popover first', async () => {
        const world = { enabled: true };
        const { h, editor } = publishedFlow(world);
        await settle();
        const ran = [];
        const stop = h.ED.actions.observe(id => ran.push(id));
        const pill = editor.el.querySelector('[data-ed-pill]');
        const pop = () => editor.el.querySelector('[data-ed-pill-popover]');
        const posts = url => h.requests.filter(r => r.method === 'POST' && r.url === url).length;
        pill.fire('click');
        pop().querySelector('[data-ed-pop="active"]').fire('click');
        await settle();
        const afterActive = [!!pop(), world.enabled, pop() && pop().querySelector('[data-ed-pop="active"]').getAttribute('aria-checked'), pillOf(editor)[0], posts('/api/desktop/flows/f1/enabled')];
        pop().querySelector('[data-ed-pop="active"]').fire('click');
        await settle();
        const backOn = [world.enabled, pop().querySelector('[data-ed-pop="active"]').getAttribute('aria-checked'), pillOf(editor)[0]];
        pop().querySelector('[data-ed-pop="run-now"]').fire('click');
        await settle();
        const afterRun = [!!pop(), posts('/api/desktop/flows/f1/run'), h.dom.document.activeElement === pill];
        pill.fire('click');
        pop().querySelector('[data-ed-pop="runs"]').fire('click');
        await settle();
        const afterRuns = [!!pop(), !!editor.el.querySelector('.ed-drawer')];
        stop();
        eq('w1c B.3 popover actions', [afterActive, backOn, afterRun, afterRuns, ran, h.logged], [
            [true, false, 'false', 'paused', 1], [true, 'true', 'live'], [false, 1, true], [false, true],
            ['flow.active', 'flow.active', 'flow.run_now', 'view.runs'], []
        ]);
        editor.dispose();
    });

    await guardAsync('w1c B.3 a refused entry does nothing, and Escape closes the popover and returns the focus', async () => {
        const { h, editor } = publishedFlow({ enabled: false });
        await settle();
        const ran = [];
        const stop = h.ED.actions.observe(id => ran.push(id));
        const pill = editor.el.querySelector('[data-ed-pill]');
        pill.fire('click');
        editor.el.querySelector('[data-ed-pill-popover] [data-ed-pop="run-now"]').fire('click');
        await settle();
        const refused = [!!editor.el.querySelector('[data-ed-pill-popover]'), ran.slice(), h.requests.filter(r => r.url === '/api/desktop/flows/f1/run').length];
        const key = editor.el.querySelector('[data-ed-pill-popover]').fire('keydown', { key: 'Escape' });
        stop();
        eq('w1c B.3 refused entry and Escape', [refused, !!editor.el.querySelector('[data-ed-pill-popover]'), h.dom.document.activeElement === pill, key.defaultPrevented, pill.getAttribute('aria-expanded')],
            [[true, [], 0], false, true, true, 'false']);
        editor.dispose();
    });

    await guardAsync('w1c B.3 a partial publish and a missing link name the action that finishes them', async () => {
        const { h, editor } = publishedFlow({ enabled: true });
        await settle();
        const ran = [];
        const stop = h.ED.actions.observe(id => ran.push(id));
        const pill = editor.el.querySelector('[data-ed-pill]');
        const pop = () => editor.el.querySelector('[data-ed-pill-popover]');
        const list = () => Array.from(pop().querySelectorAll('[data-ed-pop]')).map(b => b.getAttribute('data-ed-pop'));
        const rows = [];
        show(editor, { publishIncomplete: 'FLOW_PUBLISH_INCOMPLETE' });
        pill.fire('click');
        rows.push(['partial', list(), pop().querySelector('[data-ed-fill="publish-finish"]').textContent, !!pop().querySelector('.ed-hint')]);
        pop().querySelector('[data-ed-pop="publish-finish"]').fire('click');
        await settle();
        rows.push(['finish', !!pop(), ran.slice()]);
        // The popover follows a change while it is open: the link goes missing, so Repair replaces Finish.
        pill.fire('click');
        show(editor, { publishIncomplete: 'FLOW_MISSION_MISSING' });
        rows.push(['missing', list()]);
        pop().querySelector('[data-ed-pop="repair"]').fire('click');
        await settle();
        rows.push(['repair', !!pop(), ran.slice()]);
        stop();
        eq('w1c B.3 completing actions', rows, [
            ['partial', ['active', 'run-now', 'runs', 'publish-finish'], 'pill_finish_publish', true],
            ['finish', false, ['flow.publish']],
            ['missing', ['active', 'run-now', 'runs', 'repair']],
            ['repair', false, ['flow.publish', 'flow.repair_mission']]
        ]);
        editor.dispose();
    });

    await guardAsync('w1c B.3 a block has no Active, next run or Run now: its popover shows the state only', async () => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h, { block: true, flow: { live: flowDoc(), live_revision: 3, published_draft_revision: 3 } });
        await settle();
        const kind = pillOf(editor);
        editor.el.querySelector('[data-ed-pill]').fire('click');
        const pop = editor.el.querySelector('[data-ed-pill-popover]');
        eq('w1c B.3 block pill and popover', [kind, pop.querySelectorAll('[data-ed-pop]').length, !!pop.querySelector('[data-ed-next-run]'), h.logged], [['live', 'ok', 'pill_live:3'], 0, false, []]);
        editor.dispose();
    });

    // ── B.4: the primary button ──

    // primaryOf opens f1 with the given facts and says [which button is filled, the Publish label, whether Publish shows].
    const primaryRow = async (opts) => {
        const h = sandbox(() => undefined);
        const editor = openEditor(h, opts);
        await settle();
        const filled = editor.el.querySelectorAll('[data-ed-primary]').map(b => b.getAttribute('data-ed-cmd'));
        const test = editor.el.querySelector('[data-ed-cmd="test"]'), pub = editor.el.querySelector('[data-ed-cmd="publish"]');
        const row = [filled, pub.hidden === true, pub.querySelector('span').textContent, test.className.includes('ed-btn--primary'), pub.className.includes('ed-btn--primary'), h.logged];
        return { h, editor, row };
    };
    const live = { live: flowDoc(), live_revision: 2, published_draft_revision: 2 };

    await guardAsync('w1c B.4 the primary button follows the states of the table: Test, Publish with its count, or none', async () => {
        const rows = [];
        const cases = [
            ['never published, untested', {}],
            ['never published, tested', { facts: { tested: true } }],
            ['changed, untested', { enabled: true, flow: Object.assign({ draft_revision: 3 }, live) }],
            ['changed, tested, three changes', { enabled: true, flow: Object.assign({ draft_revision: 3 }, live), facts: { tested: true, changes: { steps: 2, connections: 1 } } }],
            ['changed, tested, one change', { enabled: true, flow: Object.assign({ draft_revision: 3 }, live), facts: { tested: true, changes: { steps: 1, connections: 0 } } }],
            ['changed, tested, no count', { enabled: true, flow: Object.assign({ draft_revision: 3 }, live), facts: { tested: true } }],
            ['clean live', { enabled: true, flow: Object.assign({ draft_revision: 2 }, live), facts: { tested: true } }],
            ['read-only', { enabled: true, readonly: true, flow: Object.assign({ draft_revision: 3 }, live), facts: { tested: true } }]
        ];
        for (const [name, opts] of cases) {
            const r = await primaryRow(opts);
            rows.push([name, r.row]);
            r.editor.dispose();
        }
        eq('w1c B.4 primary rows [filled, Publish hidden (only a clean live flow or a read-only desktop), Publish text, Test filled, Publish filled, log]', rows, [
            ['never published, untested', [['test'], false, 'publish', true, false, []]],
            ['never published, tested', [['publish'], false, 'publish', false, true, []]],
            ['changed, untested', [['test'], false, 'publish', true, false, []]],
            ['changed, tested, three changes', [['publish'], false, 'publish_with_changes:3', false, true, []]],
            ['changed, tested, one change', [['publish'], false, 'publish_with_changes_one:1', false, true, []]],
            ['changed, tested, no count', [['publish'], false, 'publish', false, true, []]],
            ['clean live', [[], true, 'publish', false, false, []]],
            ['read-only', [[], true, 'publish', false, false, []]]
        ]);
    });

    await guardAsync('w1c B.4 a failed test leaves Test primary; a passed one, an edit, a save and a restore move the button', async () => {
        const h = sandbox(req => (req.method === 'PUT' ? { draft_revision: 4, issues: [], tested: false, changes: { steps: 1, connections: 0 } } : undefined));
        const editor = openEditor(h, { enabled: true, flow: Object.assign({ draft_revision: 3 }, live) });
        await settle();
        const filled = () => editor.el.querySelectorAll('[data-ed-primary]').map(b => b.getAttribute('data-ed-cmd')).join();
        const rows = [filled()];
        // The editor's own evidence: a failed run is no test, a successful one of this revision is.
        editor.ed.testEvidence = { revision: 3, status: 'error' };
        editor.ed.bus.emit('run', editor.ed.run);
        rows.push(filled());
        editor.ed.testEvidence = { revision: 3, status: 'success' };
        editor.ed.bus.emit('run', editor.ed.run);
        rows.push(filled());
        // An edit makes the draft untested at once; its save brings revision 4, which the evidence does not cover.
        editor.ed.model.setFlow({ description: 'changed' });
        rows.push(filled());
        h.runTimers(1000);
        await settle();
        rows.push(filled(), editor.el.querySelector('[data-ed-cmd="publish"]').hidden);
        // Evidence for revision 4 brings Publish with the count of the save answer.
        editor.ed.testEvidence = { revision: 4, status: 'success' };
        editor.ed.bus.emit('run', editor.ed.run);
        rows.push(filled(), editor.el.querySelector('[data-ed-cmd="publish"]').querySelector('span').textContent);
        editor.ed.bus.emit('restored', { flow: Object.assign({ id: 'f1', draft: flowDoc(), draft_revision: 9, published_draft_revision: 2 }, live), issues: [] });
        await settle();
        rows.push(filled());
        eq('w1c B.4 primary follows evidence, edits, saves and a restore', rows, ['test', 'test', 'publish', 'test', 'test', false, 'publish', 'publish_with_changes_one:1', 'test']);
        eq('w1c B.4 no log', h.logged, []);
        editor.dispose();
    });

    await guardAsync('w1c B.4 a partial publish fills Publish, a past run keeps it in place but unavailable, Test stays beside the primary and says why when it cannot run', async () => {
        const { h, editor } = await primaryRow({ enabled: true, flow: Object.assign({ draft_revision: 2 }, live) });
        const filled = () => editor.el.querySelectorAll('[data-ed-primary]').map(b => b.getAttribute('data-ed-cmd')).join();
        const rows = [filled()];
        Object.assign(editor.ed, { publishIncomplete: 'FLOW_PUBLISH_INCOMPLETE' });
        editor.ed.bus.emit('enabled');
        rows.push(filled());
        editor.ed.runView = { run: { id: 'r9', status: 'success', revision: 2 } };
        editor.ed.bus.emit('enabled');
        const test = editor.el.querySelector('[data-ed-cmd="test"]');
        rows.push(filled(), test.getAttribute('aria-disabled'), test.getAttribute('aria-description'));
        eq('w1c B.4 partial publish, past run', rows, ['', 'publish', 'publish', 'true', 'reason_run_view']);
        editor.dispose();
    });

    await guardAsync('w1c B.4 Publish stays in place while there is something to publish: untested or tested, a run going, a past run open', async () => {
        const { h, editor } = await primaryRow({ enabled: true, flow: Object.assign({ draft_revision: 3 }, live), facts: { changes: { steps: 2, connections: 1 } } });
        const pub = () => editor.el.querySelector('[data-ed-cmd="publish"]');
        const look = () => [editor.el.querySelectorAll('[data-ed-primary]').map(b => b.getAttribute('data-ed-cmd')).join(), pub().hidden === true, pub().getAttribute('aria-disabled'), pub().getAttribute('aria-description'), pub().querySelector('span').textContent];
        const rows = [look()];
        editor.ed.testEvidence = { revision: 3, status: 'success' };
        editor.ed.bus.emit('run', editor.ed.run);
        rows.push(look());
        editor.ed.runView = { run: { id: 'r9', status: 'success', revision: 2 } };
        editor.ed.bus.emit('enabled');
        rows.push(look());
        eq('w1c B.4 Publish: untested (Test filled), tested (Publish filled), a past run open (same look, unavailable)', rows, [
            ['test', false, null, null, 'publish_with_changes:3'],
            ['publish', false, null, null, 'publish_with_changes:3'],
            ['publish', false, 'true', 'reason_run_view', 'publish_with_changes:3']
        ]);
        eq('w1c B.4 Publish no log', h.logged, []);
        editor.dispose();
    });

    // ── B.5: the header row ──

    await guardAsync('w1c B.5 the header row: Back, Name and Pill on the left; Stop slot, Test, primary, Undo and Redo, More on the right', async () => {
        const h = sandbox(req => (req.method === 'POST' && req.url === '/api/desktop/flows/f1/test' ? { run_id: 'r1' } : undefined));
        const editor = openEditor(h);
        await settle();
        const head = editor.el.querySelector('.ed-head');
        const kind = c => (c.hasAttribute('data-ed-stop-slot') ? 'stop-slot' : c.hasAttribute('data-ed-history-host') ? 'history' : c.getAttribute('data-ed-cmd') || c.className);
        const row = () => Array.from(editor.el.querySelector('.ed-head-actions').children).map(kind);
        const title = editor.el.querySelector('.ed-title');
        const idle = row();
        const slot = editor.el.querySelector('[data-ed-stop-slot]');
        const stop = slot.querySelector('[data-ed-cmd="cancel"]');
        const idleStop = [stop.hidden, editor.el.querySelector('.ed-head-actions').children[0] === slot];
        editor.el.querySelector('[data-ed-cmd="test"]').fire('click');
        await settle();
        const dialog = h.dialogs[h.dialogs.length - 1];
        if (dialog) { const run = dialog.el.querySelector('[data-ed-action="run"]'); if (run) { run.fire('click'); await settle(); } }
        const running = [stop.hidden, row(), stop.parentNode === slot];
        eq('w1c B.5 header row order and the Stop slot (idle and while a test runs)', [
            Array.from(head.children).map(c => c.className), title.querySelector('[data-ed-pill]') !== null, title.querySelector('[data-ed-name]') !== null,
            idle, idleStop, running, h.logged], [
            ['ed-icon-btn ed-back', 'ed-title', 'ed-head-actions'], true, true,
            ['stop-slot', 'test', 'publish', 'history', 'more'], [true, true],
            [false, ['stop-slot', 'test', 'publish', 'history', 'more'], true], []]);
        editor.dispose();
    });

    await guardAsync('w1c B.5 the stylesheet reserves the slot, keeps the Stop label at 900 px and names no removed button', async () => {
        const css = fs.readFileSync(path.join(apps, '..', '..', '..', 'css', 'desktop-app-easydrag.css'), 'utf8');
        const slot = /\.ed-stop-slot \{([^}]*)\}/.exec(css);
        const at900 = css.slice(css.indexOf('@container (max-width: 900px)')).split(/\r?\n\}\r?\n/)[0];
        eq('w1c B.5 stylesheet', [!!slot && /flex: none/.test(slot[1]) && /width: var\(--ed-stop-slot-w, 10\.5rem\)/.test(slot[1]),
            /\[data-ed-cmd="cancel"\] > span \{ display: inline/.test(at900), /data-ed-cmd="(runs|run-now|active)"/.test(css), /\.ed-head-history \{[^}]*flex: none/.test(css)], [true, true, false, true]);
    });

    // @@APPEND@@
}
