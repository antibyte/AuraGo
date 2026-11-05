import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const box = vm.createContext({ window: {}, console, Intl, Date, Set, Map });
for (const name of ['easydrag-template.js', 'easydrag-testing.js', 'easydrag-transform.js']) {
    const file = new URL('../ui/js/desktop/apps/' + name, import.meta.url);
    vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
}
const ED = box.window.EasyDrag, T = ED.transform;
const local = vm.runInContext('text => JSON.parse(text)', box);
const check = (name, args) => ({ name, args });
for (const value of ['plain', 'quote"back\\slash', 'new\nline\ttab\rcarriage', 'literal\\u0041', '☃😀', '}} {{unsafe}}', '\u0000', '\u2028\u2029']) {
    const expression = T.build(ED.template.pathJoin('source', value), [check('default', [value])]);
    const parsed = ED.template.parseExpr(expression);
    assert.equal(parsed.path[0], value);
    assert.equal(parsed.filters[0].args[0], value);
}
for (const value of [null, true, false, -0, 1e21, 1e-9, 1e308, 5e-324]) {
    const parsed = ED.template.parseExpr(T.build('source.value', [check('default', [value])]));
    assert.equal(parsed.filters[0].args[0], value);
}
assert.throws(() => T.build('source.value', [check('default', ['\uD800'])]));
assert.throws(() => T.build('source.value | count', [check('trim', [])]));
assert.throws(() => T.build('source.value', Array.from({ length: 9 }, () => check('trim', []))));
assert.throws(() => T.build('source.value', [check('round', [11])]));
assert.throws(() => T.build('source.value', [check('default', [{ unrepresentable: true }])]));
assert.equal(T.preview('source.value', [check('trim', []), check('upper', []), check('split', [',']), check('join', [' · '])], ' a,b ').text, 'A · B');
assert.equal(T.preview('source.value', [check('split', [''])], 'A😀B').value.length, 3);
assert.equal(T.preview('source.value', [check('default', ['Write [redacted] literally'])], null).text, 'Write [redacted] literally');
assert.throws(() => T.preview('source.value', [check('trim', [])], '[redacted]'), err => err.transformKind === 'incomplete');
assert.throws(() => T.preview('source.value', [check('date', ['YYYY-MM-DD', 'UTC'])], '2026-11-05T12:30:00'), err => err.transformKind === 'date');
assert.equal(T.preview('source.value', [check('date', ['YYYY-MM-DD', 'UTC'])], '2026-11-05T12:30:00Z').text, '2026-11-05');
let evaluations = 0;
const evaluate = ED.template.evaluate;
ED.template.evaluate = (...args) => { evaluations++; return evaluate(...args); };
assert.throws(() => T.preview('source.value', [check('replace', ['a', 'x'.repeat(4096)])], 'a'.repeat(64000)), err => err.transformKind === 'budget');
assert.throws(() => T.preview('source.value', [check('join', ['x'.repeat(4096)])], Array.from({ length: 1000 }, () => 'a')), err => err.transformKind === 'budget');
assert.throws(() => T.preview('source.value', [check('split', [''])], 'a'.repeat(20000)), err => err.transformKind === 'budget');
assert.equal(evaluations, 0, 'expansion is refused before invoking the evaluator');
const shown = T.preview('source.value', [check('trim', [])], '😀'.repeat(4000));
assert.equal(Buffer.byteLength(shown.text), 8192);
assert.equal(shown.limited, true);
assert.equal(shown.value.length, 8000);
const large = { value: '\\'.repeat(40000) };
box.large = large;
vm.runInContext('globalThis.rawParses=0;globalThis.largeSerializations=0;const parse=JSON.parse,stringify=JSON.stringify;JSON.parse=(...args)=>{rawParses++;return parse(...args)};JSON.stringify=(value,...args)=>{if(value===large)largeSerializations++;return stringify(value,...args)};', box);
assert.throws(() => T.readSample(' '.repeat(1000000) + '{}'), err => err.transformKind === 'budget');
assert.equal(box.rawParses, 0, 'oversized raw text is refused before JSON.parse');
assert.throws(() => T.preview('source.value', [check('json', [])], large), err => err.transformKind === 'budget');
assert.equal(box.largeSerializations, 0, 'escaped string expansion is budgeted before JSON.stringify');
const cyclic = {}; cyclic.self = cyclic;
assert.throws(() => T.preview('source.value', [check('json', [])], cyclic));
assert.equal(T.available().length, ED.template.FILTER_LIST.length);
assert.deepEqual(JSON.parse(JSON.stringify(T.preview('source.value', [check('pick', ['name', 'title']), check('rename', ['title', 'heading'])], local('{"name":"Ada","title":"Hello","private":"omit"}')).value)), { name: 'Ada', heading: 'Hello' });
assert.equal(T.preview('source.value', [check('where', ['active', 'eq', true]), check('sort', ['price', 'desc']), check('pluck', ['price']), check('sum', [''])], local('[{"active":true,"price":3},{"active":false,"price":99},{"active":true,"price":2}]')).value, 5);
assert.deepEqual(JSON.parse(JSON.stringify(T.preview('source.value', [check('unique', ['']), check('sort', ['', 'asc'])], [3, 2, 3, 1]).value)), [1, 2, 3]);
assert.throws(() => T.build('source.value', [check('where', ['price', 'not_supported', 1])]));
assert.throws(() => T.build('source.value', [check('pick', Array.from({ length: 17 }, () => 'x'))]));
console.log('All EasyDrag transform expression and preview checks passed.');

export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    const B = 'n_bbbbbbbb';
    function fixture() {
        const h = sandbox(() => undefined), types = new Map(h.catalog.types);
        const info = types.get('web.search');
        types.set('web.search', Object.assign({}, info, { params: info.params.map(param => param.name === 'query' ? Object.assign({}, param, { templatable: true }) : param) }));
        h.catalog = Object.assign({}, h.catalog, { types, list: Array.from(types.values()) });
        return { h, editor: openEditor(h) };
    }
    function sourceButton(editor, ref = 'alpha.text') { return editor.el.querySelectorAll('[data-ed-transform-ref]').find(button => button.dataset.edTransformRef === ref); }
    function chooseTarget(editor) { const select = editor.el.querySelector('[data-ed-mapping-target]'); select.value = 'query'; select.fire('change'); }
    await guardAsync('transform canvas double-click follows captured mouse or touch hits without overriding controls', async () => {
        const { h, editor } = fixture(); await settle();
        eq('transform move-up uses a registered direction icon', h.ED.core.icon('chevron-up').includes(h.ED.core.ICONS['chevron-up']), true);
        const canvas = editor.el.querySelector('.ed-canvas'), card = editor.el.querySelector('.ed-node'), label = card.querySelector('.ed-node-label');
        const port = card.querySelector('.ed-port'), tool = card.querySelector('.ed-node-tools'), wire = editor.el.querySelector('.ed-edge');
        const wireActions = canvas.appendChild(new h.dom.El('div', { class: 'ed-wire-actions' }));
        const wireButton = wireActions.appendChild(new h.dom.El('button', {}));
        const emitted = [], emit = editor.ed.bus.emit;
        editor.ed.bus.emit = (type, data) => ['open-detail', 'quick-add'].includes(type) ? emitted.push({ type, data }) : emit(type, data);
        let hit = label, lookups = 0;
        h.dom.document.elementFromPoint = () => { lookups++; return hit; };
        const fire = (target, pointerType = 'mouse') => canvas.fire('dblclick', { target, pointerType, clientX: 90, clientY: 80 });
        fire(label);
        eq('ordinary double-click keeps its real target', [emitted.pop().type, lookups], ['open-detail', 0]);
        fire(canvas); fire(canvas, 'touch');
        eq('captured double-click and generated touch double-click open the visible card', emitted.splice(0).map(event => [event.type, event.data.nodeId]), [['open-detail', card.dataset.nodeId], ['open-detail', card.dataset.nodeId]]);
        for (hit of [port, tool, canvas.querySelector('.ed-zoom'), canvas.querySelector('.ed-minimap'), wire, wireButton].filter(Boolean)) fire(canvas);
        fire(wire); fire(wireButton);
        eq('captured and ordinary ports, tools, wires and canvas controls remain inert for double-click', emitted.length, 0);
        hit = canvas; fire(canvas);
        eq('empty background keeps quick-add', emitted.splice(0).map(event => event.type), ['quick-add']);
        hit = h.body; fire(canvas); hit = null; fire(canvas);
        eq('capture outside this canvas or without a hit never acts', emitted.length, 0);
        editor.ed.readonly = true; hit = canvas; fire(canvas); hit = label; fire(canvas);
        eq('read-only still inspects cards and never adds', emitted.splice(0).map(event => event.type), ['open-detail']);
        editor.ed.readonly = false; editor.ed.runView = { id: 'run' }; hit = canvas; fire(canvas); hit = label; fire(canvas, 'touch');
        eq('run view preserves the same boundary', emitted.splice(0).map(event => event.type), ['open-detail']);
        editor.dispose();
    });
    await guardAsync('transform helper requires an explicit target and preserves the insertion range in one undo', async () => {
        const { h, editor } = fixture(); await settle();
        editor.ed.lastRunData = { label: 'Observed test', roots: { alpha: { text: ' hello ' } } };
        editor.ed.model.setParam(B, 'query', 'before AFTER');
        editor.ed.bus.emit('open-detail', { nodeId: B });
        const count = h.dialogs.length;
        sourceButton(editor).fire('click');
        eq('transform helper never guesses a target', [h.dialogs.length, editor.ed.model.node(B).params.query], [count, 'before AFTER']);
        chooseTarget(editor);
        const field = editor.el.querySelector('.ed-field[data-param="query"]');
        field.querySelector('.ed-tpl-view').fire('click');
        const input = field.querySelector('.ed-tpl-input'); input.setSelectionRange(7, 12); input.fire('focusin');
        const before = editor.ed.model.version, requests = h.requests.length;
        sourceButton(editor).fire('click'); await settle();
        const d = h.dialogs[h.dialogs.length - 1];
        eq('transform preview reads the observed source and performs no requests', [d.body.querySelector('[data-ed-transform-preview]').textContent, h.requests.length === requests, d.el.parentNode.html.includes('Observed test')], ['hello', true, true]);
        eq('transform sample has a name and trim hides irrelevant parity and argument notices', [d.body.querySelector('[data-ed-transform-sample]').getAttribute('aria-label'), d.body.querySelector('[data-ed-transform-parity]').hidden, d.body.querySelector('[data-ed-transform-literals]').hidden], ['transform_sample', true, true]);
        d.el.querySelector('[data-ed-action="apply"]').fire('click'); await settle();
        const applied = editor.ed.model.node(B).params.query, version = editor.ed.model.version;
        editor.ed.model.setParam(B, 'query', applied + ' typed'); editor.ed.model.undo();
        const afterTypingUndo = editor.ed.model.node(B).params.query;
        editor.ed.model.undo();
        eq('transform helper replaces only the selected range as one reversible edit isolated from surrounding typing', [applied, version - before, afterTypingUndo, editor.ed.model.node(B).params.query, h.logged], ['before {{alpha.text | trim}}', 1, applied, 'before AFTER', []]);
        editor.dispose();
    });
    await guardAsync('transform helper distinguishes missing schema samples from explicit null', async () => {
        const { h, editor } = fixture(); await settle();
        editor.ed.bus.emit('open-detail', { nodeId: B }); chooseTarget(editor);
        const schema = editor.el.querySelectorAll('[data-ed-transform-ref]').find(button => button.dataset.edTransformRef.startsWith('alpha.'));
        schema.fire('click'); await settle();
        let d = h.dialogs[h.dialogs.length - 1];
        eq('transform schema references stay authorable without inventing real missing values', [d.body.querySelector('[data-ed-transform-preview-error]').textContent, d.body.querySelector('[data-ed-transform-sample]').value, d.el.querySelector('[data-ed-action="apply"]').disabled], ['transform_no_sample', '', false]);
        d.close(); await settle();
        editor.ed.detail.close(); h.runTimers(160); editor.ed.lastRunData = { label: 'Observed null', roots: { alpha: { text: null } } };
        editor.ed.bus.emit('open-detail', { nodeId: B }); chooseTarget(editor); sourceButton(editor).fire('click'); await settle();
        d = h.dialogs[h.dialogs.length - 1];
        eq('transform explicit null is an available JSON value with its own provenance', [d.body.querySelector('[data-ed-transform-sample]').value, d.body.querySelector('[data-ed-transform-preview-error]').hidden, d.el.parentNode.html.includes('Observed null')], ['null', true, true]);
        d.close(); editor.dispose();
    });
    await guardAsync('transform helper fences local changes readonly and IME without losing input', async () => {
        const { h, editor } = fixture(); await settle();
        editor.ed.lastRunData = { label: 'Observed', roots: { alpha: { text: ' hello ' } } };
        editor.ed.bus.emit('open-detail', { nodeId: B }); chooseTarget(editor);
        const field = editor.el.querySelector('.ed-field[data-param="query"]'); field.querySelector('.ed-tpl-view').fire('click');
        const input = field.querySelector('.ed-tpl-input'); input.value = '未確定'; input.edComposing = true;
        const count = h.dialogs.length; sourceButton(editor).fire('click');
        eq('transform opening declines an unfinished target composition and retains its buffer', [h.dialogs.length, input.value, editor.ed.model.node(B).params.query], [count, '未確定', '']);
        input.edComposing = false; input.value = ''; sourceButton(editor).fire('click'); await settle();
        let d = h.dialogs[h.dialogs.length - 1];
        editor.ed.model.setLabel(B, 'New local edit');
        d.el.querySelector('[data-ed-action="apply"]').fire('click'); await settle();
        eq('transform stale draft cannot overwrite a newer local edit', [editor.ed.model.node(B).params.query, d.body.querySelector('.ed-error').textContent], ['', 'transform_stale']);
        d.close(); await settle(); sourceButton(editor).fire('click'); await settle();
        d = h.dialogs[h.dialogs.length - 1]; editor.ed.readonly = true;
        d.el.querySelector('[data-ed-action="apply"]').fire('click'); await settle();
        eq('transform Apply rechecks readonly even after opening', editor.ed.model.node(B).params.query, '');
        d.close(); editor.ed.readonly = false; await settle(); sourceButton(editor).fire('click'); await settle();
        d = h.dialogs[h.dialogs.length - 1];
        const area = d.body.querySelector('[data-ed-transform-sample]'); area.fire('compositionstart'); area.value = '"新しい値"'; area.fire('input');
        d.el.querySelector('[data-ed-action="apply"]').fire('click'); await settle();
        eq('transform sample composition blocks Apply without clearing text', [editor.ed.model.node(B).params.query, area.value], ['', '"新しい値"']);
        area.fire('compositionend'); h.runTimers(180);
        area.value = ' '.repeat(1000000); area.fire('input'); h.runTimers(180);
        eq('transform oversized raw whitespace is refused before parsing', d.body.querySelector('[data-ed-transform-preview-error]').textContent, 'transform_budget');
        editor.ed.detail.close(); await settle();
        eq('transform dialogs close with their inspector and log no disposal errors', [editor.ed.detail, h.logged], [null, []]);
        editor.dispose();
    });
    await guardAsync('transform helper keyboard entry retains a single tree tab stop', async () => {
        const { h, editor } = fixture(); await settle();
        editor.ed.lastRunData = { label: 'Observed', roots: { alpha: { text: 'x' } } };
        editor.ed.bus.emit('open-detail', { nodeId: B }); chooseTarget(editor);
        const row = editor.el.querySelectorAll('.ed-tree-row').find(item => item.dataset.ref === 'alpha.text');
        row.fire('keydown', { key: 'F10', shiftKey: true }); await settle();
        eq('transform Shift+F10 opens the same dialog without extra tree stops', [h.dialogs.length, editor.el.querySelectorAll('.ed-tree-row[tabindex="0"]').length], [1, 1]);
        editor.dispose();
    });
    await guardAsync('transform mapping references retain NUL CR percent and quotes across every path', async () => {
        const { h, editor } = fixture(); await settle();
        const keys = ['nul\u0000key', 'carriage\rkey', 'percent%2f', 'quote"\\key'], values = {};
        keys.forEach((key, i) => { values[key] = 'value' + i; });
        const inserts = [], transformed = [];
        const mapping = h.ED.mapping.create({ ed: editor.ed, sources: [{ id: 'data', label: 'Observed', roots: { alpha: values } }], sourceId: 'data', upstream: [{ key: 'alpha', label: 'Alpha', fields: [] }], onInsert: ref => inserts.push(ref), onTransform: item => transformed.push(item) });
        editor.el.appendChild(mapping.el);
        const drag = [], expected = keys.map(key => h.ED.template.pathJoin('alpha', key));
        for (const ref of expected) {
            const row = mapping.el.querySelectorAll('.ed-tree-row').find(item => item.dataset.ref === ref);
            row.fire('click');
            row.fire('dragstart', { dataTransfer: { setData(type, value) { if (type === 'application/x-easydrag-ref') drag.push(value); }, effectAllowed: '' } });
            h.fireDoc('dragend');
            row.querySelector('[data-ed-transform-ref]').fire('click');
        }
        const stale = mapping.el.querySelectorAll('.ed-tree-row').find(item => item.dataset.ref === expected[0]);
        mapping.refresh({ sources: [{ id: 'data', label: 'New source', roots: { alpha: values } }] });
        mapping.el.fire('click', { target: stale });
        mapping.el.fire('click', { target: stale.querySelector('[data-ed-transform-ref]') });
        eq('transform opaque DOM refs preserve original bytes for insert drag and helper; stale rows are ignored', [inserts, drag, transformed.map(item => item.ref), transformed.map(item => item.value)], [expected, expected, expected, keys.map((_, i) => 'value' + i)]);
        mapping.el.remove(); editor.dispose();
    });
}
