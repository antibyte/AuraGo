// Flagship journeys use the existing editor sandbox and API transport.
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    await guardAsync('flagship sample provenance and readable safe previews', async () => {
        const h = sandbox(() => undefined);
        const parse = text => { try { return h.ED.testing.parseSample(text); } catch (_) { return 'refused'; } };
        eq('flagship incomplete samples refuse actual markers, preserving ordinary text and meaningful null', [
            parse('{"token":"[redacted]"}'), parse('{"data":{"_preview":"partial","_omitted":true}}'),
            parse('{"message":"Write [redacted] literally","value":null}'), parse('{"_preview":"a real field","other":1}')
        ], ['refused', 'refused', { message: 'Write [redacted] literally', value: null }, { _preview: 'a real field', other: 1 }]);
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
}
