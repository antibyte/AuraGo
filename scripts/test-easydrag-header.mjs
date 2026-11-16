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
        const elsewhere = [ED.flowState.isTested(editor.ed)];
        // A restored version replaces the draft: the old flag and count go.
        editor.ed.bus.emit('restored', { flow: Object.assign(recordOf(10), { draft: flowDoc() }), issues: [] });
        await settle();
        const restored = [flow().tested, flow().changes === undefined, ED.flowState.isTested(editor.ed)];
        eq('w1c B.1 the facts follow the read, the save, a re-read and a restore', [first, dirtyTested, saved, reread, elsewhere, restored, h.logged], [
            [true, { steps: 2, connections: 1 }, { at: '2026-11-17T07:00:00+01:00', node: T1 }, true, 3],
            false,
            [false, { steps: 3, connections: 1 }, { at: '2026-11-17T07:00:00+01:00', node: T1 }, false, 4, 'saved'],
            [true, { steps: 5, connections: 0 }, true, true],
            [false],
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
            [ED.flowState.changeCount({ flow: { changes: { steps: 2, connections: 3 } } }), ED.flowState.changeCount({ flow: {} }), ED.flowState.changeCount({ flow: { changes: { steps: 'x' } } })],
            [5, null, 0]);
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

    // @@APPEND@@
}
