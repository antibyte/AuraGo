// Flagship journeys use the existing editor sandbox and API transport.
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    await guardAsync('flagship comparison exports only explicit bounded metadata and rejects stale responses', async () => {
        const records = ['r1', 'r2', 'r3'].map(id => ({ id, flow_id: 'f1', status: 'success', revision: 3, started_at: '2026-11-04T12:00:00Z' }));
        let delayed = false;
        const pending = [];
        const detail = id => ({ run: { ...records.find(r => r.id === id), trigger_data: { password: 'private-input' } }, steps: [{ node_id: 'n_aaaaaaaa', node_key: 'private-label', status: 'success', params: { query: 'private-query' }, output: { text: id + '-private-output' }, error_message: 'private-error' }] });
        const h = sandbox(req => {
            if (req.url.startsWith('/api/desktop/flows/f1/runs?limit=50')) return { runs: records, has_more: false };
            if (req.url.startsWith('/api/desktop/flows/runs/')) {
                const id = req.url.split('/').pop();
                if (delayed) return new Promise(resolve => pending.push({ id, resolve }));
                return detail(id);
            }
        });
        const editor = openEditor(h);
        await settle();
        editor.el.querySelector('[data-ed-cmd="runs"]').fire('click'); await settle();
        editor.el.querySelector('[data-ed-runs-compare]').fire('click'); await settle();
        const dialog = h.dialogs[h.dialogs.length - 1];
        const exportButton = dialog.el.querySelector('[data-ed-action="export"]');
        eq('flagship comparison itself only reads and never downloads', [exportButton.disabled, h.downloads.length, h.requests.filter(req => req.method !== 'GET').length], [false, 0, 0]);
        exportButton.fire('click');
        const json = JSON.parse(await h.downloads[0].blob.text());
        eq('flagship explicit diagnostic export excludes every payload and human label', [h.downloads[0].name, json.format, JSON.stringify(json).includes('private-'), json.runs.map(r => r.run.id)], ['easydrag-diagnostic.json', 'easydrag-diagnostic', false, ['r2', 'r1']]);
        h.runTimers(1000);
        eq('flagship diagnostic object URLs are released', h.objects.size, 0);
        delayed = true;
        const after = dialog.body.querySelector('[data-ed-compare="after"]');
        after.value = 'r3'; after.fire('change'); await settle();
        const stale = pending.splice(0);
        after.value = 'r2'; after.fire('change'); await settle();
        const latest = pending.splice(0);
        latest.forEach(p => p.resolve(detail(p.id))); await settle();
        stale.forEach(p => p.resolve(detail(p.id))); await settle();
        exportButton.fire('click');
        eq('flagship late comparisons cannot replace the selected pair', JSON.parse(await h.downloads[1].blob.text()).runs.map(r => r.run.id), ['r2', 'r2']);
        after.value = 'r3'; after.fire('change'); await settle();
        pending.splice(0).forEach(p => p.resolve({ ...detail(p.id), run: { ...detail(p.id).run, flow_id: 'another-flow' } })); await settle();
        eq('flagship another flow response disables diagnostics and shows an error', [exportButton.disabled, dialog.body.querySelector('[data-ed-compare-error]').hidden], [true, false]);
        editor.dispose(); await settle();
        eq('flagship comparison closes with the editor and causes no writes', [dialog.el.isConnected, h.requests.filter(req => req.method !== 'GET').length, h.logged], [false, 0, []]);
    });

    await guardAsync('flagship sample provenance and readable safe previews', async () => {
        const h = sandbox(() => undefined);
        const parse = text => { try { return h.ED.testing.parseSample(text); } catch (_) { return 'refused'; } };
        eq('flagship incomplete samples refuse actual markers, preserving ordinary text and meaningful null', [
            parse('{"token":"[redacted]"}'), parse('{"data":{"_preview":"","_omitted":"run_storage_budget"}}'),
            parse('{"message":"Write [redacted] literally","value":null}'), parse('{"_preview":"a real field","other":1}'),
            parse('{"data":{"_preview":"partial","_truncated":true}}'), parse('{"data":{"[redacted]":"hidden"}}'), parse('{"_preview":"ordinary","_omitted":"user value"}')
        ], ['refused', 'refused', { message: 'Write [redacted] literally', value: null }, { _preview: 'a real field', other: 1 }, 'refused', 'refused', { _preview: 'ordinary', _omitted: 'user value' }]);
        eq('flagship result preview keeps data as bounded plain text', [h.ED.detail.previewText({ text: '<script>bad()</script>' }, []), h.ED.detail.previewText({ body: 'a'.repeat(9000) }, [{ name: 'body', primary: true }]).length], ['<script>bad()</script>', 8000]);
        const model = h.ED.model.create({ schema: 1, name: 'Local first result', nodes: [
            { id: 'n_greeting', key: 'greeting', type: 'logic.set', label: 'Greeting', params: { fields: [{ name: 'greeting', value: 'Hallo Welt' }] } },
            { id: 'n_check', key: 'check', type: 'logic.if', label: 'Check', params: {} }
        ], edges: [] }, { types: new Map([['logic.set', {}], ['logic.if', { output_fields: [{ name: 'result', type: 'bool', primary: true }] }]]) });
        eq('flagship introductory recipe shows the greeting before the final boolean check', h.ED.testing.contentResult({ model }, [
            { node_id: 'n_greeting', status: 'success', output: { greeting: 'Hallo Welt' } }, { node_id: 'n_check', status: 'success', output: { result: true } }
        ]), { text: 'Hallo Welt', label: 'Greeting' });
    });

    await guardAsync('flagship run history reaches retained older pages without shifting inspected rows', async () => {
        const records = Array.from({ length: 55 }, (_, i) => ({ id: 'r' + i, status: 'success', mode: 'test', started_at: '2026-11-04T10:00:00Z', revision: 3 }));
        const h = sandbox(req => {
            if (req.url.startsWith('/api/desktop/flows/f1/runs?limit=50')) {
                const offset = new URL(req.url, 'https://fixture.test').searchParams.get('offset') === '50' ? 50 : 0;
                return { runs: records.slice(offset, offset + 50), has_more: !offset, next_offset: offset + Math.min(50, records.length - offset) };
            }
        });
        const editor = openEditor(h);
        await settle();
        editor.el.querySelector('[data-ed-cmd="runs"]').fire('click');
        await settle();
        editor.el.querySelector('[data-ed-runs-more]').fire('click');
        await settle();
        const rows = () => editor.el.querySelectorAll('[data-ed-run]').map(b => b.dataset.edRun);
        const loaded = rows();
        editor.ed.runs?.runFinished?.();
        const search = editor.el.querySelector('[data-ed-runs-search]');
        search.value = 'r54'; search.fire('input');
        eq('flagship retained page and loaded-search preserve distinct IDs', [loaded.length, new Set(loaded).size, rows(), editor.el.querySelector('[data-ed-runs-more]').hidden], [55, 55, ['r54'], true]);
        editor.dispose();
    });

    await guardAsync('flagship explicit simulation never starts a real test or an event stream', async () => {
        const h = sandbox(req => {
            if (req.url === '/api/desktop/flows/f1/simulate') return { mode: 'simulation', revision: 3, status: 'success', started_at: '2026-11-04T12:00:00Z', steps: [{ node_id: 'n_aaaaaaaa', node_key: 'alpha', status: 'success', output: { text: 'Local result' }, ports: ['out'] }] };
        });
        const editor = openEditor(h);
        await settle();
        editor.el.querySelector('[data-ed-cmd="test"]').fire('click');
        await settle();
        const dialog = h.dialogs[h.dialogs.length - 1];
        const mode = dialog.body.querySelector('[data-ed-test-mode]');
        mode.value = 'simulation'; mode.fire('change');
        dialog.body.querySelector('[data-ed-test-data]').value = '{}';
        dialog.body.querySelector('[data-ed-simulation-fixtures]').value = '{}';
        dialog.el.querySelector('[data-ed-action="run"]').fire('click');
        await settle();
        eq('flagship simulation paints the result with no real POST, remembered data or SSE', [
            editor.ed.run.mode, editor.ed.lastRunData.roots.alpha.text, h.sources.length,
            h.requests.filter(req => req.method === 'POST' && req.url.endsWith('/test')).length,
            h.requests.find(req => req.url.endsWith('/simulate')).body, h.logged
        ], ['simulation', 'Local result', 0, 0, { trigger_node: 'n_tttttttt', trigger_data: {}, base_revision: 3, fixtures: {} }, []]);
        editor.dispose();
    });

    await guardAsync('flagship publish review names fields and preserves paused activation', async () => {
        let previewRevision = 3;
        const h = sandbox(req => {
            if (req.url.endsWith('/publish-preview')) return { draft_revision: previewRevision, issues: [], effects: [], diff: { changed_nodes: 1, details: [{ kind: 'node_changed', node_id: 'n_aaaaaaaa', label: 'Alpha', field: 'params.query', before: 'old', after: '<new>' }] } };
        });
        const draft = { schema: 1, name: 'Flow', nodes: [], edges: [] };
        const editor = openEditor(h, { enabled: false, flow: { live: draft, live_revision: 2 } });
        await settle();
        editor.el.querySelector('[data-ed-cmd="publish"]').fire('click');
        await settle();
        const dialog = h.dialogs[h.dialogs.length - 1];
        const html = dialog.el.parentNode.html;
        eq('flagship publish shows escaped before/after, absent exact-test evidence and paused-by-default', [html.includes('Alpha'), html.includes('&lt;new&gt;'), html.includes('publish_not_tested'), dialog.body.querySelector('[data-ed-activate]').checked], [true, true, true, false]);
        dialog.close(); await settle();
        editor.ed.testEvidence = { revision: 3, status: 'success' };
        previewRevision = 4;
        editor.el.querySelector('[data-ed-cmd="publish"]').fire('click');
        await settle();
        const changed = h.dialogs[h.dialogs.length - 1];
        eq('flagship another window revision never inherits this editors test evidence', changed.el.parentNode.html.includes('publish_not_tested'), true);
        changed.close(); editor.dispose();
    });

    await guardAsync('flagship trigger changes preserve simulation disclosure', async () => {
        const h = sandbox(req => req.method === 'PUT' && req.url === '/api/desktop/flows/f1' ? { draft_revision: 4, issues: [] } : undefined);
        const editor = openEditor(h);
        await settle();
        editor.ed.model.addNode('trigger.manual', { x: 0, y: 200 });
        editor.ed.bus.emit('node-test', { nodeId: 'n_aaaaaaaa' });
        await settle();
        const dialog = h.dialogs[h.dialogs.length - 1];
        const mode = dialog.body.querySelector('[data-ed-test-mode]');
        mode.value = 'simulation'; mode.fire('change');
        const trigger = dialog.body.querySelector('[data-ed-test-trigger]');
        trigger.value = editor.ed.model.doc.nodes[editor.ed.model.doc.nodes.length - 1].id;
        trigger.fire('change');
        await settle();
        eq('flagship renewed effect checks stay hidden in explicit simulation', [dialog.body.querySelector('.ed-simulation-fields').hidden, dialog.body.querySelector('[data-ed-test-remember]').disabled,
            dialog.body.querySelectorAll('[data-ed-test-effects], [data-ed-effects-unchecked]').every(node => node.hidden)], [false, true, true]);
        dialog.close(); editor.dispose();
    });

    await guardAsync('flagship asynchronous validation preserves inspector controls', async () => {
        const h = sandbox(() => undefined), editor = openEditor(h);
        await settle();
        editor.ed.bus.emit('open-detail', { nodeId: 'n_aaaaaaaa' });
        const input = editor.el.querySelector('.ed-field[data-param="query"]');
        editor.ed.bus.emit('issues', []);
        const retained = editor.el.querySelector('.ed-field[data-param="query"]') === input;
        editor.ed.model.setParam('n_aaaaaaaa', 'query', 'An external edit');
        const updated = editor.el.querySelector('.ed-field[data-param="query"]');
        editor.ed.model.node('n_aaaaaaaa').params.query = 'Same object, changed content';
        editor.ed.bus.emit('issues', []);
        const mutated = editor.el.querySelector('.ed-field[data-param="query"]');
        eq('flagship issue refresh keeps targets; immutable and same-object content edits refresh controls', [retained, updated !== input, mutated !== updated, editor.ed.model.node('n_aaaaaaaa').params.query], [true, true, true, 'Same object, changed content']);
        editor.dispose();
    });

    await guardAsync('flagship authoring sends only an explicit goal and applies one undo command', async () => {
        const h = sandbox(req => {
            if (req.url.endsWith('/ai-proposal')) return { base_revision: 3, doc: { schema: 1, name: 'Proposed flow', nodes: [{ id: 'n_new', key: 'start', type: 'trigger.manual', label: 'New start', params: {}, settings: {}, position: { x: 80, y: 80 } }], edges: [] }, issues: [], effects: [], diff: { details: [] } };
        });
        const editor = openEditor(h);
        await settle();
        let menu;
        h.ctx.showContextMenu = (_x, _y, items) => { menu = items; };
        editor.el.querySelector('[data-ed-cmd="more"]').fire('click');
        await menu.find(item => item.label === 'authoring_title').action();
        const dialog = h.dialogs[h.dialogs.length - 1];
        dialog.body.querySelector('[data-ed-authoring-goal]').value = 'Create a local greeting';
        dialog.el.querySelector('[data-ed-action="generate"]').fire('click');
        await settle();
        const request = h.requests.find(req => req.url.endsWith('/ai-proposal'));
        eq('flagship AI proposal transmits no draft, fixtures or run data and writes nothing before Apply', [request.body, h.puts().length, editor.ed.model.doc.name, dialog.el.querySelector('[data-ed-action="apply"]').hidden], [{ goal: 'Create a local greeting', base_revision: 3 }, 0, 'Flow', false]);
        const before = JSON.stringify(Object.assign({}, editor.ed.model.toJSON(), { groups: [] }));
        dialog.el.querySelector('[data-ed-action="apply"]').fire('click');
        await settle();
        const name = editor.ed.model.doc.name;
        editor.ed.model.undo();
        eq('flagship explicit Apply is one reversible draft edit without execution or publication', [name, JSON.stringify(editor.ed.model.toJSON()) === before, editor.ed.model.canUndo(), h.requests.filter(req => /\/(test|simulate|run|publish)$/.test(req.url) && req.method === 'POST').length], ['Proposed flow', true, false, 0]);
        editor.dispose();
    });

    await guardAsync('flagship in-flight authoring cannot overwrite unsaved local edits', async () => {
        let respond;
        const response = new Promise(resolve => { respond = resolve; });
        const h = sandbox(req => req.url.endsWith('/ai-proposal') ? response : undefined), editor = openEditor(h);
        await settle();
        let menu; h.ctx.showContextMenu = (_x, _y, items) => { menu = items; };
        editor.el.querySelector('[data-ed-cmd="more"]').fire('click');
        await menu.find(item => item.label === 'authoring_title').action();
        const d = h.dialogs[h.dialogs.length - 1];
        d.body.querySelector('[data-ed-authoring-goal]').value = 'Create a greeting';
        d.el.querySelector('[data-ed-action="generate"]').fire('click'); await settle();
        editor.ed.model.setLabel('n_aaaaaaaa', 'Keep this local edit');
        respond({ base_revision: 3, doc: { schema: 1, name: 'Proposal', nodes: [{ id: 'n_new', key: 'start', type: 'trigger.manual' }], edges: [] }, issues: [], effects: [] });
        await settle();
        eq('flagship model-version fence rejects a proposal before Apply even when server revision matches', [d.el.querySelector('[data-ed-action="apply"]').hidden, d.body.querySelector('.ed-error').textContent, editor.ed.model.node('n_aaaaaaaa').label, h.puts().length], [true, 'authoring_stale', 'Keep this local edit', 0]);
        d.close(); editor.dispose();
    });

    await guardAsync('flagship panel width can be chosen by a single pointer', async () => {
        const h = sandbox(() => undefined), editor = openEditor(h);
        await settle();
        editor.el.getBoundingClientRect = () => ({ x: 0, y: 0, width: 1600, height: 900, left: 0, right: 1600, top: 0, bottom: 900 });
        let choices;
        h.ctx.showContextMenu = (_x, _y, items) => { choices = items; };
        editor.ed.bus.emit('open-detail', { nodeId: 'n_aaaaaaaa' });
        const before = editor.ed.model.version;
        editor.el.querySelector('[data-ed-detail-size]').fire('click');
        choices[choices.length - 1].action();
        eq('flagship width presets use the same bounds and never change flow history', [choices.length > 1, editor.el.style['--ed-inspector-width'], editor.ed.model.version === before], [true, '800px', true]);
        editor.dispose();
    });

    await guardAsync('flagship new proposals create a flow only after explicit Apply', async () => {
        const doc = { schema: 1, name: 'Proposed flow', nodes: [{ id: 'n_new', key: 'start', type: 'trigger.manual', label: 'Start', params: {}, settings: {}, position: { x: 80, y: 80 } }], edges: [] };
        const h = sandbox(req => req.url.endsWith('/ai-proposal') ? { doc, issues: [], effects: [] } : req.url === '/api/desktop/flows' && req.method === 'POST' ? { flow: { id: 'flow_created' } } : undefined);
        const root = h.ED.core.el('<div></div>'); h.body.appendChild(root);
        const owner = h.ED.authoring.createNew({ ctx: h.ctx, t: h.ctx.t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, openFlow: id => h.opened.push(id) }, root, () => true);
        async function generate() {
            await owner.open();
            const d = h.dialogs[h.dialogs.length - 1];
            d.body.querySelector('[data-ed-authoring-goal]').value = 'A local greeting';
            d.el.querySelector('[data-ed-action="generate"]').fire('click'); await settle();
            return d;
        }
        let d = await generate();
        eq('flagship goal-first preview sends the goal without creating a blank draft', h.requests.map(req => [req.url, req.body]), [['/api/desktop/flows/ai-proposal', { goal: 'A local greeting' }]]);
        d.close(); await settle();
        eq('flagship cancelled goal-first proposal leaves no flow', [h.requests.length, h.opened], [1, []]);
        d = await generate(); d.el.querySelector('[data-ed-action="apply"]').fire('click'); await settle();
        eq('flagship new proposal Apply imports once and opens the created draft', [h.requests.filter(req => req.url === '/api/desktop/flows').map(req => req.body), h.opened], [[{ import: doc }], ['flow_created']]);
        owner.dispose();
    });
}
