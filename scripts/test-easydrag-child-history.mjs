// Invocation identity, immutable context and bounded, read-only child-record inspection.
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    const RID = 'run_childhistory', P = 'n_bbbbbbbb', B = 'flow_childblock', H = 'a'.repeat(64);
    const copy = v => JSON.parse(JSON.stringify(v));
    const deferred = () => { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b; }); return { promise, resolve, reject }; };
    const meta = (state = 'complete', count = 2) => ({ parent_seq: 12, state, retained_invocations: count, total_invocations: count, retained_steps: count, total_steps: count });
    const frame = (id = 'i_1', extra = {}) => ({ invocation_id: id, call_node_id: P, call_attempt: 1, block_id: B, revision: 1, hash: H, entered: true, status: 'success', steps: [{ ordinal: 1, step: { node_id: P, node_key: 'inside', attempt: 1, status: 'success', output: { text: id }, params: { input: false } } }], ...extra });
    const recorded = (frames = [frame(), frame('i_2', { parent_invocation_id: 'i_1', item_index: 0 })], extra = {}) => ({ schema: 1, run_id: RID, parent_seq: 12, history: meta('complete', frames.length), frames, ...extra });
    const doc = (name = 'Pinned v1') => ({ block_id: B, revision: 1, hash: H, doc: { id: B, kind: 'block', schema: 1, name, nodes: [{ id: P, key: 'inside', type: 'logic.set', label: 'Pinned child step', params: {}, settings: {}, position: { x: 0, y: 0 } }], edges: [] } });
    function fixture(options = {}) {
        const state = { trace: recorded(), doc: doc(), hook: null };
        const h = sandbox(req => {
            if (state.hook) { const answer = state.hook(req); if (answer !== undefined) return answer; }
            if (req.url.endsWith('/children')) return copy(state.trace);
            if (req.url.endsWith('/document')) return copy(state.doc);
            if (req.url === '/api/desktop/flows/runs/' + RID + '?include=doc') return { run: { id: RID, revision: 1, status: 'success' }, steps: [step()], doc_missing: 'version_pruned' };
        });
        const editor = openEditor(h, { readonly: !!options.readonly });
        // NamedNodeMap is browser DOM; the shared minimal harness stores the same attributes in Map.
        Object.defineProperty(h.dom.El.prototype, 'attributes', { configurable: true, get() { return Array.from(this.attrs, ([name, value]) => ({ name, value })); } });
        const ed = editor.ed;
        ed.run = { id: RID, record: { id: RID, status: 'success', revision: 1 }, status: 'success', steps: new Map([[P, step()]]) };
        return { h, ed, editor, state, viewer: () => editor.el.querySelector('[data-ed-child-history]'), open() { return ed.childHistory.open(step()); } };
    }
    const step = () => ({ node_id: P, node_key: 'beta', status: 'success', attempt: 1, child_history: meta() });
    const click = (el, selector) => { const target = el.querySelector(selector); if (!target) throw Error('missing ' + selector); target.fire('click'); };
    const getCount = f => f.h.requests.filter(r => /\/children/.test(r.url)).length;

    await guardAsync('child-history schema and scoped identity', async () => {
        const f = fixture(), C = f.h.ED.childHistory;
        const parsed = C.trace(recorded(), { seq: 12, runId: RID, simulation: false });
        eq('child-history equal node IDs preserve distinct frames, local ordinal and item zero', [parsed.parent_seq, parsed.frames[0].steps[0].ordinal, parsed.frames[1].item_index, parsed.frames.map(row => row.steps[0].step.output.text)], [12, 1, 0, ['i_1', 'i_2']]);
        const bad = [recorded([], { schema: 2 }), recorded([], { run_id: 'other' }), recorded([], { parent_seq: 1 }), recorded([frame(), frame()]), recorded([frame('i_01')]), recorded([frame('i_1', { parent_invocation_id: 'i_1' })])];
        eq('child-history rejects wrong schema, owner, inferred sequence, duplicate identity and cycles', bad.map(value => { try { C.trace(value, { seq: 12, runId: RID, simulation: false }); return false; } catch { return true; } }), bad.map(() => true));
        const sim = recorded(); delete sim.run_id;
        eq('child-history simulation has explicit sequence and never accepts a real-run identity', [C.trace(sim, { seq: 12, simulation: true }).parent_seq, (() => { try { C.trace(recorded(), { seq: 12, simulation: true }); return false; } catch { return true; } })()], [12, true]);
        f.editor.dispose();
    });

    await guardAsync('child-history complete values and omission privacy', async () => {
        const f = fixture(), C = f.h.ED.childHistory, marker = { _preview: '', _omitted: 'child_history_omitted' };
        eq('child-history never retains a prefix from partial or oversized values', [C.fullValue({ text: 'PRIVATE-PREFIX' }, true), C.fullValue({ _preview: 'PRIVATE-PREFIX', _truncated: true }), C.fullValue({ text: 'x'.repeat(16384) })], [marker, marker, marker]);
        eq('child-history full null, false, zero and escaped strings survive unchanged', [C.fullValue(null), C.fullValue(false), C.fullValue(0), C.fullValue({ text: '\r\n"\\😀' })], [null, false, 0, { text: '\r\n"\\😀' }]);
        eq('child-history preflight counts encoded escapes, rejects unpaired Unicode and oversized traversal', [C.fits('\\'.repeat(8192), 16384, 64), C.fits('\ud800', 16384, 64), C.fits(Array(50001).fill(0), 262144, 64)], [false, false, false]);
        const html = f.h.ED.detail.recordValueMarkup(f.ed, { $type: 'file', web_path: '/files/private.png', text: '<img src=x>' }, 'tree');
        eq('child-history inert renderer escapes data and offers no file/media requests', [/&lt;img/.test(html), /<(img|audio|video|iframe)|href=/.test(html), getCount(f)], [true, false, 0]);
        f.editor.dispose();
    });

    await guardAsync('child-history lazy Inspector integration stays read-only', async () => {
        const f = fixture({ readonly: true }), before = JSON.stringify(f.ed.model.doc), version = f.ed.model.version;
        f.h.ED.detail.open(f.ed, P, { column: 'output' });
        eq('child-history Inspector entry does not eagerly fetch a trace', [!!f.editor.el.querySelector('[data-ed-child-open]'), getCount(f)], [true, 0]);
        click(f.editor.el, '[data-ed-child-open]'); await settle();
        click(f.viewer(), '[data-ed-child-frame="i_1"]'); await settle();
        click(f.viewer(), '[data-ed-child-record="1"]');
        eq('child-history opens the exact persisted sequence and pinned context without draft mutations', [f.h.requests.filter(r => /\/children/.test(r.url)).map(r => r.url), JSON.stringify(f.ed.model.doc) === before, f.ed.model.version === version, f.h.puts().length, f.viewer().html.includes('Pinned child step')], [['/api/desktop/flows/runs/' + RID + '/steps/12/children', '/api/desktop/flows/runs/' + RID + '/steps/12/children/i_1/document'], true, true, 0, true]);
        click(f.viewer(), '[data-ed-child-subject="params"]');
        eq('child-history params are independent of output', f.viewer().html.includes('false'), true);
        f.ed.detail.close(); await settle();
        eq('child-history closing the parent Inspector removes its child viewer', !!f.viewer(), false);
        f.editor.dispose();
    });

    await guardAsync('child-history frame changes and late document replies', async () => {
        const f = fixture(), a = deferred(), b = deferred();
        f.state.trace = recorded([frame(), frame('i_2', { block_id: 'flow_other' })]);
        f.state.hook = req => req.url.endsWith('i_1/document') ? a.promise : req.url.endsWith('i_2/document') ? b.promise : undefined;
        f.open(); await settle();
        click(f.viewer(), '[data-ed-child-frame="i_1"]');
        click(f.viewer(), '[data-ed-child-root]'); click(f.viewer(), '[data-ed-child-frame="i_2"]');
        click(f.viewer(), '[data-ed-child-root]'); click(f.viewer(), '[data-ed-child-frame="i_1"]');
        a.resolve(doc()); await settle(); b.resolve({ block_id: 'flow_other', revision: 1, hash: H, doc_missing: 'redacted' }); await settle();
        eq('child-history A–B–A coalesces a pending immutable document and ignores other-frame painting', [f.h.requests.filter(r => r.url.endsWith('i_1/document')).length, f.viewer().querySelector('[data-ed-child-heading]').textContent.includes('Pinned v1'), !f.viewer().querySelector('[data-ed-child-document-missing]').hidden], [1, true, false]);
        const late = deferred(); f.state.hook = req => req.url.endsWith('/children') ? late.promise : undefined;
        click(f.viewer(), '[data-ed-child-reload]'); f.editor.dispose(); late.resolve(recorded()); await settle();
        eq('child-history a response after disposal cannot recreate protected data', [!!f.viewer(), f.h.logged], [false, []]);
    });

    await guardAsync('child-history simulation identity never creates persistent URLs', async () => {
        const f = fixture(), simulated = recorded(); delete simulated.run_id;
        f.ed.run = { id: 'simulation', mode: 'simulation', record: { mode: 'simulation' }, steps: new Map([[P, step()]]) };
        f.h.ED.childHistory.rememberSimulation(f.ed.run, { child_traces: [simulated], child_documents: [doc()] });
        f.open(); await settle(); click(f.viewer(), '[data-ed-child-frame="i_1"]'); await settle(); click(f.viewer(), '[data-ed-child-record="1"]');
        eq('child-history simulation uses only inline trace and verified document snapshots', [getCount(f), f.viewer().html.includes('Pinned child step')], [0, true]);
        f.ed.run = { ...f.ed.run }; f.ed.bus.emit('run');
        eq('child-history a new simulation generation closes the old record view', !!f.viewer(), false);
        f.editor.dispose();
    });

    await guardAsync('child-history completeness, missing documents and permission loss', async () => {
        const f = fixture();
        f.state.trace = recorded([], { history: meta('complete', 0) }); f.open(); await settle();
        eq('child-history a proved empty trace is complete rather than unavailable', f.viewer().html.includes('child_history_empty'), true);
        f.state.trace = recorded([frame('i_1', { entered: false, status: 'error', error_code: 'FLOW_PARAM_INVALID', steps: [] })], { history: { ...meta('incomplete', 1), truncated: true, total_invocations: null, total_steps: null } });
        f.state.doc = { block_id: B, revision: 1, hash: H, doc_missing: 'budget' };
        click(f.viewer(), '[data-ed-child-reload]'); await settle(); click(f.viewer(), '[data-ed-child-frame="i_1"]'); await settle();
        eq('child-history incomplete and truncated remain visible with pre-entry failure and missing doc', [['child_history_incomplete', 'child_history_truncated', 'child_history_not_started'].every(key => f.viewer().html.includes(key)), f.viewer().querySelector('[data-ed-child-document-missing]').textContent.includes('child_document_budget')], [true, true]);
        f.state.hook = req => { if (req.url.endsWith('/children')) throw Object.assign(new Error('denied'), { status: 403 }); };
        click(f.viewer(), '[data-ed-child-reload]'); await settle();
        eq('child-history read denial clears prior values and document labels', [f.viewer().html.includes('error_flow_child_history_unavailable'), f.viewer().html.includes('Pinned'), f.viewer().querySelectorAll('[data-ed-child-frame]').length], [true, false, 0]);
        f.editor.dispose();
    });

    await guardAsync('child-history bounded local paging', async () => {
        const f = fixture(), frames = Array.from({ length: 120 }, (_, i) => frame('i_' + (i + 1), { steps: [] }));
        frames[0].steps = Array.from({ length: 240 }, (_, i) => ({ ordinal: i + 1, step: { node_id: P, attempt: i + 1, status: 'success', output: { value: i } } }));
        f.state.trace = recorded(frames); f.open(); await settle();
        const first = f.viewer().querySelectorAll('[data-ed-child-frame]').length;
        click(f.viewer(), '[data-ed-child-next-page]'); const second = f.viewer().querySelectorAll('[data-ed-child-frame]').length;
        click(f.viewer(), '[data-ed-child-prev-page]'); click(f.viewer(), '[data-ed-child-frame="i_1"]'); await settle();
        const records = f.viewer().querySelectorAll('[data-ed-child-record]').length;
        click(f.viewer(), '[data-ed-child-next-page]');
        eq('child-history pages locally at 50 calls and 100 records without more trace requests', [first, second, records, f.viewer().querySelectorAll('[data-ed-child-record]').length, f.h.requests.filter(r => r.url.endsWith('/children')).length, f.viewer().querySelectorAll('button').length < 250], [50, 50, 100, 100, 1, true]);
        f.editor.dispose();
    });

    await guardAsync('child-history retained evidence without the parent document', async () => {
        const f = fixture(); f.editor.showRun(RID); await settle();
        eq('child-history missing parent document uses the same modal viewer without swapping the draft', [!!f.viewer(), !!f.ed.runView, f.h.notes.length, f.h.requests.some(r => r.url.endsWith('/steps/12/children'))], [true, false, 0, true]);
        f.editor.dispose();
    });

    await guardAsync('child-history pending and historical attempts are not live progress', async () => {
        const f = fixture();
        f.ed.childHistory.open({ node_id: P, status: 'running' }); await settle();
        eq('child-history unfinished calls without a committed sequence stay pending and issue no guessed GET', [f.viewer().html.includes('child_history_pending'), getCount(f)], [true, 0]);
        f.state.trace = recorded([frame('i_1', { steps: [
            { ordinal: 1, step: { node_id: P, node_key: 'same', attempt: 1, status: 'error', error_code: 'FLOW_PARAM_INVALID' } },
            { ordinal: 2, step: { node_id: P, node_key: 'same', attempt: 2, status: 'success', started_at: '2026-11-05T10:00:00Z', finished_at: '2026-11-05T10:00:05Z', duration_ms: 5000, output: { ok: true } } }
        ] })]);
        f.open(); await settle(); click(f.viewer(), '[data-ed-child-frame="i_1"]'); await settle(); click(f.viewer(), '[data-ed-child-record="1"]');
        eq('child-history early failed attempts never inherit fabricated timing or free error text', [f.viewer().querySelectorAll('time').length, f.viewer().html.includes('error_flow_param_invalid')], [0, true]);
        click(f.viewer(), '[data-ed-child-back]'); click(f.viewer(), '[data-ed-child-record="2"]');
        eq('child-history final attempt exposes only its actual node-lifetime timestamps and explanation', [f.viewer().querySelectorAll('time').length, f.viewer().html.includes('child_history_time_hint')], [2, true]);
        f.editor.dispose();
    });

    await guardAsync('child-history fallback, document identity and bounded rendering', async () => {
        const f = fixture();
        f.state.doc = { ...doc('WRONG VERSION'), revision: 2 };
        f.state.trace = recorded([frame('i_1', { parent_invocation_id: 'i_99' })]);
        f.open(); await settle();
        eq('child-history retained frames stay reachable even if their ancestor is missing', !!f.viewer().querySelector('[data-ed-child-frame="i_1"]'), true);
        click(f.viewer(), '[data-ed-child-frame="i_1"]'); await settle();
        eq('child-history mismatched document revision is never used as context', [f.viewer().querySelector('[data-ed-child-heading]').textContent.includes('WRONG VERSION'), f.viewer().querySelector('[data-ed-child-document-missing]').textContent.includes('child_document_unavailable')], [false, true]);
        click(f.viewer(), '[data-ed-child-back]');
        eq('child-history Back from a retained frame with a missing ancestor returns to calls', [!!f.viewer().querySelector('.ed-child-row[data-ed-child-frame="i_1"]'), f.viewer().querySelectorAll('[data-ed-child-record]').length], [true, 0]);
        const html = f.h.ED.detail.recordValueMarkup(f.ed, Array.from({ length: 1000 }, (_, i) => i), 'tree');
        eq('child-history selected JSON tree has a separate 100-entry DOM budget', (html.match(/<li>/g) || []).length <= 101, true);
        const gate = deferred(); f.state.hook = req => req.url.endsWith('/children') ? gate.promise : undefined;
        f.open(); await settle(); f.h.dialogs.at(-1).close(null); await settle(); gate.resolve(recorded()); await settle();
        eq('child-history core-modal close invalidates a still-pending response', !!f.viewer(), false);
        f.editor.dispose();
    });

    await guardAsync('child-history navigation and cache bounds', async () => {
        const f = fixture(), pending = [], controllers = [], original = f.ed.api.childDocument;
        f.state.trace = recorded(Array.from({ length: 4 }, (_, i) => frame('i_' + (i + 1), { block_id: 'flow_' + i })));
        f.ed.api.childDocument = (_run, _seq, _invocation, signal) => { const d = deferred(); pending.push(d); controllers.push(signal); return d.promise; };
        f.open(); await settle();
        for (let i = 1; i <= 4; i++) { if (i > 1) click(f.viewer(), '[data-ed-child-root]'); click(f.viewer(), '[data-ed-child-frame="i_' + i + '"]'); }
        eq('child-history document navigation aborts evicted reads and retains at most two active document requests', controllers.map(c => c.aborted), [true, true, false, false]);
        f.ed.run = { id: 'other', record: { id: 'other' }, steps: new Map() }; f.ed.bus.emit('run');
        pending.forEach(d => d.resolve(doc())); await settle();
        eq('child-history switching run closes the viewer, aborts reads and ignores late context', [!!f.viewer(), controllers.every(c => c.aborted), f.h.logged], [false, true, []]);
        f.ed.api.childDocument = original; f.editor.dispose();
    });

    await guardAsync('child-history keyboard focus and denial revoke every read', async () => {
        const f = fixture(), waiting = deferred(), signals = [];
        const original = f.ed.api.childDocument;
        f.ed.api.childDocument = (...args) => { signals.push(args[3]); return waiting.promise; };
        f.open(); await settle();
        eq('child-history initial open focuses its heading rather than the hidden opener', f.h.dom.document.activeElement === f.viewer().querySelector('[data-ed-child-heading]'), true);
        click(f.viewer(), '[data-ed-child-frame="i_1"]');
        const recordButton = f.viewer().querySelector('[data-ed-child-record="1"]'); recordButton.focus();
        waiting.resolve(doc()); await settle();
        eq('child-history async document context preserves the actual focused record and its DOM identity', [f.h.dom.document.activeElement === recordButton, f.viewer().querySelector('[data-ed-child-record="1"]') === recordButton, recordButton.querySelector('strong').textContent], [true, true, 'Pinned child step']);
        click(f.viewer(), '[data-ed-child-record="1"]');
        const subject = f.viewer().querySelector('[data-ed-child-subject="params"]'); subject.focus(); subject.fire('click');
        eq('child-history subject change retains valid keyboard focus', f.h.dom.document.activeElement === f.viewer().querySelector('[data-ed-child-subject="params"]'), true);
        const late = deferred(); f.ed.api.childDocument = (...args) => { signals.push(args[3]); return late.promise; };
        f.open(); await settle(); click(f.viewer(), '[data-ed-child-frame="i_1"]');
        f.state.hook = req => { if (req.url.endsWith('/children')) throw Object.assign(new Error('denied'), { status: 401 }); };
        click(f.viewer(), '[data-ed-child-reload]'); await settle();
        late.resolve(doc('PRIVATE AFTER DENIAL')); await settle();
        eq('child-history read denial aborts all pending document reads, disables reload and rejects late context', [signals.at(-1).aborted, f.viewer().querySelector('[data-ed-child-reload]').disabled, f.viewer().html.includes('PRIVATE AFTER DENIAL'), f.viewer().querySelectorAll('[data-ed-child-record]').length], [true, true, false, 0]);
        f.ed.api.childDocument = original; f.editor.dispose();
    });

    await guardAsync('child-history busy guard and mobile list detail', async () => {
        const f = fixture(), pending = deferred(); let reads = 0, signal;
        const original = f.ed.api.childTrace;
        f.ed.api.childTrace = (_run, _seq, sig) => { reads++; signal = sig; return pending.promise; };
        f.open(); await settle();
        const disabled = f.viewer().querySelector('[data-ed-child-reload]').disabled;
        click(f.viewer(), '[data-ed-child-reload]'); await settle();
        eq('child-history initial read disables Reload synchronously and a synthetic click cannot abort it', [disabled, reads, signal.aborted], [true, 1, false]);
        pending.resolve(recorded()); await settle();
        click(f.viewer(), '[data-ed-child-frame="i_1"]'); await settle(); click(f.viewer(), '[data-ed-child-record="1"]');
        eq('child-history selecting a record replaces lists and pagers with the value', [!!f.viewer().querySelector('[data-ed-child-value]'), f.viewer().querySelectorAll('.ed-child-list').length, f.viewer().querySelectorAll('.ed-child-pager').length, !!f.viewer().querySelector('[data-ed-child-back]')], [true, 0, 0, true]);
        click(f.viewer(), '[data-ed-child-back]');
        eq('child-history Back restores the record list with retained invocation identity', [!!f.viewer().querySelector('[data-ed-child-record="1"]'), f.viewer().html.includes('child_history_invocation')], [true, true]);
        f.ed.api.childTrace = original;
        const host = f.h.ED.core.el('<div></div>'); f.editor.el.appendChild(host);
        f.editor.el.getBoundingClientRect = () => ({ width: 1200 });
        const view = f.ed.childHistory.open(step(), { host }); await settle();
        f.editor.el.getBoundingClientRect = () => ({ width: 320 });
        (f.h.winListeners.resize || []).slice().forEach(fn => fn()); await settle();
        eq('child-history desktop to phone keeps the same viewer in the existing modal', [f.viewer() === view, !!view.closest('.ed-modal--children'), !!view.closest('.ed-modal--children').querySelector('[data-ed-action="close"]')], [true, true, true]);
        f.ed.childHistory.detach(host);
        eq('child-history closing the originating Inspector also closes its promoted mobile viewer', !!f.viewer(), false);
        f.editor.dispose();
    });
}
