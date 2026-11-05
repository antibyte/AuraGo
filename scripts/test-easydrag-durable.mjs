// Default-off capability and observed durable lifecycle on the existing editor/run controller.
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    const T = 'n_aaaaaaaa', W = 'n_bbbbbbbb', RID = 'run_durable123';
    const copy = value => JSON.parse(JSON.stringify(value));
    const cap = () => ({ configured: true, effective: true, recovery_available: true, waiting_count: 1, claimed_count: 0, blocked_count: 0 });
    const phase = (state = 'waiting') => ({ run_id: RID, state, generation: 2, cancel_available: true, wake_at: '2026-11-06T12:00:00Z', deadline: '2026-11-06T12:10:00Z', ...(state === 'blocked' ? { blocked_code: 'disabled' } : {}) });
    function fixture(options = {}) {
        const doc = { schema: 1, kind: 'flow', name: 'Saved wait', settings: {}, nodes: [
            { id: T, type: 'trigger.manual', key: 'start', label: 'Start', params: {}, settings: {}, position: { x: 0, y: 0 } },
            { id: W, type: 'logic.durable_wait', key: 'wait', label: 'Saved wait', params: { seconds: 60 }, settings: {}, position: { x: 300, y: 0 } }
        ], edges: [{ id: 'e_aaaaaaaa', source: { node: T, port: 'out' }, target: { node: W, port: 'in' } }] };
        const state = { capability: cap(), run: { id: RID, flow_id: 'f1', mode: 'live', revision: 1, durable: true, status: 'waiting', trigger_node: T, trigger_type: 'manual', trigger_data: { private: 'sample' }, started_at: '2026-11-06T11:59:00Z' }, phase: phase(), steps: [{ node_id: W, node_key: 'wait', status: 'waiting' }], requests: [], hook: null };
        const detail = () => ({ run: copy(state.run), steps: copy(state.steps), doc: copy(doc), ...(state.phase ? { durable_status: copy(state.phase) } : {}) });
        const h = sandbox(req => {
            state.requests.push(req);
            if (state.hook) { const value = state.hook(req); if (value !== undefined) return value; }
            if (req.url === '/api/desktop/flow-runtime') return copy(state.capability);
            if (req.url === '/api/desktop/flows/f1/run') return { run_id: RID };
            if (req.url.startsWith('/api/desktop/flows/runs/' + RID) && req.method === 'GET') return detail();
            if (req.url.endsWith('/durable/cancel')) return { accepted: true };
            if (req.url.startsWith('/api/desktop/flows/f1/runs')) return { runs: [] };
        });
        h.catalog.types = new Map(h.catalog.types);
        h.catalog.types.set('logic.durable_wait', { type: 'logic.durable_wait', label: 'Persistent wait', category: 'logic', inputs: ['in'], outputs: ['out'], params: [{ name: 'seconds', type: 'number', required: true }], availability: { state: 'available' } });
        h.catalog.list = [...h.catalog.types.values()];
        h.contexts = []; h.ctx.updateWindowContext = (_id, context) => h.contexts.push(context);
        const editor = openEditor(h, { enabled: true, readonly: !!options.readonly, flow: { draft: doc, live: copy(doc), live_revision: 1, published_draft_revision: 3 } });
        return { h, editor, state, detail, async start() { h.menus.find(menu => menu.id === 'flow').items.find(item => item.id === 'run').action(); await settle(); return h.sources.at(-1); }, async view() { editor.showRun(RID); await settle(); }, async snapshot(es) { es.emit('snapshot', detail()); await settle(); } };
    }
    const mutations = f => f.h.requests.filter(req => req.method !== 'GET');
    const cancelPosts = f => mutations(f).filter(req => req.url.endsWith('/durable/cancel'));
    async function stop(f) {
        f.editor.el.querySelector('[data-ed-cmd="stop-viewed"]').fire('click'); await settle();
        f.h.dialogs.at(-1).el.querySelector('[data-ed-action="stop"]').fire('click'); await settle();
    }

    await guardAsync('durable bounded metadata refuses guessed types and mismatched run identities', async () => {
        const f = fixture(), D = f.h.ED.durable;
        eq('durable initial capability stays hidden until its first observation', f.editor.ed.runtime.el.hidden, true);
        await settle();
        eq('durable confirmed nonempty capability becomes visible after its read', f.editor.ed.runtime.el.hidden, false);
        eq('durable unknown counts remain null rather than zero', D.capability({ ...cap(), waiting_count: null }).waiting_count, null);
        eq('durable capability rejects string counters and unknown reasons', [D.capability({ ...cap(), blocked_count: '0' }), D.capability({ ...cap(), reason: '<private>' })], [null, null]);
        eq('durable service unavailable is capability metadata rather than a checkpoint reason', [D.capability({ ...cap(), reason: 'service_unavailable' })?.reason, D.status(f.state.run, { ...phase('blocked'), blocked_code: 'service_unavailable' })], ['service_unavailable', null]);
        eq('durable run status rejects wrong ID unsafe generation and invalid dates', [D.status(f.state.run, { ...phase(), run_id: 'other' }), D.status(f.state.run, { ...phase(), generation: -1 }), D.status(f.state.run, { ...phase(), wake_at: '<script>' })], [null, null, null]);
        eq('durable metadata stores only the agreed bounded fields', [D.capability({ ...cap(), checkpoint: 'never retained' }).checkpoint, D.status(f.state.run, { ...phase(), payload: 'never retained' }).payload], [undefined, undefined]);
        eq('durable merely opening an editor does not enable or start processing', mutations(f), []); f.editor.dispose();
    });

    await guardAsync('durable off and unknown capability prevent new wait selection without document changes', async () => {
        const f = fixture(); await settle();
        f.state.capability = { ...cap(), configured: false, effective: false, recovery_available: false, reason: 'disabled' };
        await f.editor.ed.runtime.refresh(true);
        const before = f.editor.ed.model.toJSON(), version = f.editor.ed.model.version;
        const placed = f.h.ED.palette.place(f.editor.ed, 'logic.durable_wait', { x: 600, y: 0 });
        eq('durable observed off refuses node creation while retaining stored typed waits', [placed, f.editor.ed.model.toJSON(), f.editor.ed.model.version], [null, before, version]);
        f.state.hook = req => { if (req.url === '/api/desktop/flow-runtime') throw new Error('offline'); };
        await f.editor.ed.runtime.refresh(true); await f.start();
        eq('durable unknown capability never sends a live-run admission', [f.h.sources.length, mutations(f).length], [0, 0]); f.editor.dispose();
    });

    await guardAsync('durable waiting and blocked views retain identity and never fabricate completion', async () => {
        const f = fixture(), es = await f.start(); await f.snapshot(es);
        eq('durable waiting is active and keeps its explicit window run identity', [f.editor.ed.run.status, f.h.contexts.at(-1).run_id, !!f.editor.el.querySelector('[data-ed-durable-run]')], ['waiting', RID, true]);
        f.state.phase = phase('blocked'); await f.view();
        const body = f.editor.el.querySelector('.ed-run-results-body');
        eq('durable reversible block is distinct from a final error and exposes fixed reason times', [f.editor.ed.run.status, f.editor.el.querySelector('.ed-run-results summary').textContent.includes('durable_blocked'), body.html.includes('durable_reason_disabled'), body.html.includes('durable_deadline')], ['waiting', true, true, true]);
        eq('durable body keeps the one visible title in its summary', body.html.includes('<strong>durable_title'), false);
        eq('durable opening a stored waiting run starts no replacement run', mutations(f).filter(req => req.url.endsWith('/run')).length, 1);
        f.editor.dispose();
    });

    await guardAsync('durable status disclosure starts compact and preserves an explicit user choice', async () => {
        const f = fixture(); await f.view();
        const results = f.editor.el.querySelector('.ed-run-results'), summary = results.querySelector('summary');
        eq('durable ordinary waiting status is initially collapsed', results.open, false);
        results.open = true; summary.fire('click');
        f.state.phase.generation++; f.editor.ed.bus.emit('refresh-durable-run'); await settle();
        eq('durable a metadata refresh retains the users expanded status', results.open, true);
        results.open = false; summary.fire('click');
        f.state.phase.state = 'running'; f.state.run.status = 'running';
        f.editor.ed.bus.emit('refresh-durable-run'); await settle();
        eq('durable a later phase retains the users collapsed status', results.open, false);
        f.editor.dispose();
        const blocked = fixture(); blocked.state.phase = phase('blocked'); await blocked.view();
        eq('durable an initially blocked status exposes its reason', blocked.editor.el.querySelector('.ed-run-results').open, true);
        blocked.editor.dispose();
    });

    await guardAsync('durable reconnect reads authoritative committed data before reusing the same SSE cursor', async () => {
        const f = fixture(), es = await f.start(); await f.snapshot(es);
        es.emit('event', { seq: 7, type: 'step_finished', node_id: T, step: { node_id: T, node_key: 'start', status: 'success', output: { committed: true } } });
        f.state.steps = [{ node_id: T, node_key: 'start', status: 'success', output: { committed: true } }, ...f.state.steps];
        es.onerror(); f.h.runTimers(1500); await settle();
        const next = f.h.sources.at(-1);
        eq('durable reconnection keeps run ID cursor and committed output with no run POST', [next.url, f.editor.ed.run.id, f.editor.ed.run.steps.get(T).output, mutations(f).filter(req => req.url.endsWith('/run')).length], ['/api/desktop/flows/runs/' + RID + '/events?after=7', RID, { committed: true }, 1]);
        es.listeners.snapshot[0]({ data: JSON.stringify({ ...f.detail(), run: { ...f.state.run, status: 'error' } }) });
        eq('durable queued callbacks from an old EventSource cannot rewind a new connection', f.editor.ed.run.status, 'waiting');
        next.emit('end', {}); await settle();
        eq('durable a nonterminal end snapshot never announces failure or marks finished', [f.editor.ed.run.status, f.editor.ed.run.finished, f.h.notes.some(note => note.message === 'run_failed')], ['waiting', false, false]); f.editor.dispose();
    });

    await guardAsync('durable accepted cancellation stays pending until authoritative terminal evidence', async () => {
        const f = fixture({ readonly: true }); await f.view(); await stop(f);
        eq('durable Stop uses observed generation under the existing readonly exception', cancelPosts(f).map(req => req.body), [{ generation: 2 }]);
        eq('durable accepted Stop is not optimistic cancelled and remains busy', [f.editor.ed.run.status, f.editor.ed.run.durableStopping, f.editor.el.querySelector('[data-ed-cmd="stop-viewed"]').disabled], ['waiting', true, true]);
        f.state.run.status = 'cancelled'; f.state.phase = null;
        f.editor.ed.bus.emit('refresh-durable-run'); await settle();
        eq('durable only confirmed terminal state clears the pending control', [f.editor.ed.run.status, f.editor.ed.run.durableStopping, f.h.contexts.at(-1).run_id], ['cancelled', false, null]); f.editor.dispose();
    });

    await guardAsync('durable generation conflict and failed cancellation never retry mutations', async () => {
        const f = fixture(); await f.view();
        f.state.hook = req => { if (req.url.endsWith('/durable/cancel')) { f.state.phase.generation = 3; throw Object.assign(new Error('conflict'), { status: 409, body: { code: 'FLOW_DURABLE_GENERATION_CONFLICT' } }); } };
        await stop(f);
        eq('durable stale Stop refreshes the generation and waits for a new user action', [cancelPosts(f).length, f.editor.ed.run.durableStatus.generation, f.editor.ed.run.status], [1, 3, 'waiting']);
        f.state.hook = req => { if (req.url.endsWith('/durable/cancel')) throw new TypeError('lost'); };
        await stop(f); f.h.runTimers(5000); await settle();
        eq('durable uncertain Stop is not replayed and never reports a final result', [cancelPosts(f).length, f.editor.ed.run.status], [2, 'waiting']); f.editor.dispose();
    });

    await guardAsync('durable incompatible legacy checkpoints remain independently inspectable and cancellable', async () => {
        const f = fixture(); f.state.run.durable = false; f.state.run.status = 'error'; f.state.phase = { ...phase('blocked'), blocked_code: 'incompatible' };
        await f.view();
        eq('durable terminal legacy run keeps its actual error and exposes checkpoint cleanup', [f.editor.ed.run.status, f.editor.el.querySelector('[data-ed-cmd="stop-viewed"]').hidden, f.editor.el.querySelector('.ed-run-results-body').html.includes('durable_reason_incompatible')], ['error', false, true]);
        f.state.hook = req => { if (req.url.endsWith('/durable/cancel')) { f.state.phase = null; return { accepted: true }; } };
        await stop(f);
        eq('durable legacy cleanup does not rewrite the retained terminal run', [cancelPosts(f)[0].body, f.editor.ed.run.status, f.editor.el.querySelector('[data-ed-cmd="stop-viewed"]').hidden], [{ generation: 2 }, 'error', true]); f.editor.dispose();
    });

    await guardAsync('durable refresh cadence is independent of run ticks and disposal fences late reads', async () => {
        const f = fixture(), es = await f.start(); await f.snapshot(es);
        const before = f.h.requests.filter(req => req.url.includes('/runs/' + RID)).length;
        for (let i = 1; i <= 100; i++) es.emit('event', { seq: i, type: 'step_started', node_id: T });
        eq('durable one hundred run ticks do not add metadata requests', f.h.requests.filter(req => req.url.includes('/runs/' + RID)).length, before);
        f.h.dom.document.hidden = true; f.h.runTimers(5000); await settle();
        eq('durable a hidden document does not poll run metadata', f.h.requests.filter(req => req.url.includes('/runs/' + RID)).length, before);
        f.h.dom.document.hidden = false; f.h.ctx.isActive = () => false; f.h.runTimers(5000); await settle();
        eq('durable an inactive window does not poll run metadata', f.h.requests.filter(req => req.url.includes('/runs/' + RID)).length, before);
        f.h.ctx.isActive = () => true; f.h.runTimers(5000); await settle();
        eq('durable polling resumes at its bounded cadence when the window is active', f.h.requests.filter(req => req.url.includes('/runs/' + RID)).length, before + 1);
        let resolve;
        f.state.hook = req => req.url.includes('/runs/' + RID) && req.method === 'GET' ? new Promise(done => { resolve = done; }) : undefined;
        f.editor.ed.bus.emit('refresh-durable-run'); await settle(); const run = f.editor.ed.run;
        f.editor.dispose(); resolve({ ...f.detail(), run: { ...f.state.run, status: 'success' } }); await settle();
        eq('durable disposed editor ignores late status and releases polling timers', [run.status, f.h.timers.size, f.h.logged], ['waiting', 0, []]);
    });

    await guardAsync('durable a stalled metadata read has a deadline and late replies stay inert', async () => {
        const f = fixture(); await f.view();
        let resolve;
        f.state.hook = req => req.url.includes('/runs/' + RID) && req.method === 'GET' ? new Promise(done => { resolve = done; }) : undefined;
        f.editor.ed.bus.emit('refresh-durable-run'); await settle();
        f.h.runTimers(15000); await settle();
        eq('durable a metadata read timeout leaves the run unknown without ending or stopping it', [f.editor.ed.run.durableUnknown, f.editor.ed.run.status, mutations(f)], [true, 'waiting', []]);
        f.state.hook = null; f.editor.ed.bus.emit('refresh-durable-run'); await settle();
        eq('durable an explicit read after timeout can recover the observed status', f.editor.ed.run.durableUnknown, false);
        resolve({ ...f.detail(), run: { ...f.state.run, status: 'error' } }); await settle();
        eq('durable a late timed-out reply cannot replace the recovered run', f.editor.ed.run.status, 'waiting');
        f.editor.dispose();
    });

    await guardAsync('durable wait and wake use existing committed step events without another run', async () => {
        const f = fixture(), es = await f.start(); await f.snapshot(es);
        f.state.run.status = 'waiting';
        es.emit('event', { seq: 8, type: 'step_finished', node_id: W, step: { node_id: W, node_key: 'wait', status: 'waiting' } }); await settle();
        eq('durable wait commit remains waiting and nonterminal', [f.editor.ed.run.status, f.editor.ed.run.finished || false], ['waiting', false]);
        f.state.run.status = 'running'; f.state.phase = phase('running');
        es.emit('event', { seq: 9, type: 'step_finished', node_id: W, step: { node_id: W, node_key: 'wait', status: 'success', output: { waited_seconds: 60 } } }); await settle();
        eq('durable committed wake continues the same run and keeps the increasing cursor', [f.editor.ed.run.id, f.editor.ed.run.status, mutations(f).filter(req => req.url.endsWith('/run')).length], [RID, 'running', 1]); f.editor.dispose();
    });

    await guardAsync('durable explicit restored run view keeps its immutable graph apart from local draft edits', async () => {
        const f = fixture(); f.editor.ed.model.setLabel(W, 'New draft label'); await f.view();
        eq('durable explicitly followed stored run uses its original graph and preserves draft editing state', [f.editor.ed.model.node(W).label, f.editor.ed.draftModel.node(W).label, f.h.contexts.at(-1).run_id, mutations(f).filter(req => req.url.endsWith('/run')).length], ['Saved wait', 'New draft label', RID, 0]); f.editor.dispose();
    });

    await guardAsync('durable read authority loss clears private outputs and observed cancellation metadata', async () => {
        const f = fixture(); f.state.steps.push({ node_id: T, node_key: 'start', status: 'success', output: { private: 'retained sample' } }); await f.view();
        f.state.hook = req => { if (req.method === 'GET' && req.url.includes('/runs/' + RID)) throw Object.assign(new Error('forbidden'), { status: 403 }); };
        f.editor.ed.bus.emit('refresh-durable-run'); await settle();
        eq('durable a lost read scope retains only identity and unknown state', [f.editor.ed.run.steps.size, f.editor.ed.run.record.trigger_data, f.editor.ed.run.durableStatus, f.editor.ed.runView.data.roots, f.editor.el.querySelector('[data-ed-cmd="stop-viewed"]').hidden], [0, undefined, null, {}, true]); f.editor.dispose();
    });

    await guardAsync('durable generation zero is sent only when the server explicitly supplies it', async () => {
        const f = fixture(); f.state.phase = { ...phase('running'), generation: 0 }; f.state.run.status = 'running'; await f.view(); await stop(f);
        eq('durable pre-checkpoint cancellation uses observed zero rather than a guessed fallback', cancelPosts(f)[0].body, { generation: 0 }); f.editor.dispose();
        const unavailable = fixture(); unavailable.state.phase = null; await unavailable.view();
        eq('durable missing cancellation metadata offers refresh rather than guessing zero', [unavailable.editor.el.querySelector('[data-ed-cmd="stop-viewed"]').hidden, cancelPosts(unavailable).length], [true, 0]); unavailable.editor.dispose();
    });

    await guardAsync('durable storage diagnosis preserves known legacy identity without a cancel fallback', async () => {
        const f = fixture(); f.state.run.durable = false; f.state.run.status = 'error'; f.state.phase = { ...phase('blocked'), blocked_code: 'incompatible' };
        f.state.steps.push({ node_id: T, node_key: 'start', status: 'success', output: { retained: true } }); await f.view();
        f.state.hook = req => { if (req.method === 'GET' && req.url.includes('/runs/' + RID)) { const result = f.detail(); delete result.durable_status; result.durable_status_error = 'storage_unavailable'; return result; } };
        f.editor.ed.bus.emit('refresh-durable-run'); await settle();
        eq('durable failed checkpoint diagnosis retains payload reads but clears cancellation proof', [f.editor.ed.run.durableKnown, f.editor.ed.run.durableStatus, f.editor.ed.run.steps.get(T).output, f.editor.el.querySelector('[data-ed-cmd="stop-viewed"]').hidden, cancelPosts(f).length], [true, null, { retained: true }, true, 0]);
        f.state.hook = null; f.editor.ed.bus.emit('refresh-durable-run'); await settle();
        eq('durable explicit successful refresh restores the observed legacy cleanup action', [f.editor.ed.run.durableStatus.generation, f.editor.el.querySelector('[data-ed-cmd="stop-viewed"]').hidden], [2, false]); f.editor.dispose();
    });
}
