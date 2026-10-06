#!/usr/bin/env node
// Runs the pure EasyDrag modules (template, model, geometry) in Node and checks their behaviour.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';
import { webcrypto } from 'node:crypto';

const here = path.dirname(fileURLToPath(import.meta.url));
const apps = path.join(here, '..', 'ui', 'js', 'desktop', 'apps');
const sandbox = { window: {}, Intl, Number, String, Array, Object, Math, Set, Map, RegExp, Date, JSON, console, Uint8Array, crypto: webcrypto };
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
for (const file of ['easydrag-template.js', 'easydrag-model.js', 'easydrag-geometry.js']) {
    const full = path.join(apps, file);
    vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
}
const ED = sandbox.window.EasyDrag;
const T = ED.template;
const M = ED.model;
const G = ED.geometry;

let failures = 0;
function check(name, cond, detail) {
    if (cond) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + (detail ? ' — ' + detail : ''));
}
function eq(name, got, want) {
    check(name, JSON.stringify(got) === JSON.stringify(want), 'got ' + JSON.stringify(got) + ' want ' + JSON.stringify(want));
}
const t = (key, params) => key.replace('easydrag.ui.', '') + (params ? ':' + Object.values(params).join(',') : '');

// ── template ──
eq('segments literal+expr', T.segments('Hi {{a.b}}!').map(s => s.literal !== undefined ? 'L' : 'E'), ['L', 'E', 'L']);
eq('escaped braces stay literal', T.segments('\\{{x}}'), [{ literal: '{{x}}' }]);
eq('parse path', T.parseExpr('search.results[0]["title"] | truncate(10)').path, ['results', 0, 'title']);
eq('parse filters', T.parseExpr('a | join(", ") | upper').filters.map(f => f.name), ['join', 'upper']);
eq('refs roots', T.refs('{{a.x}} and {{trigger.data}}').map(r => r.root), ['a', 'trigger']);
check('refs report syntax errors', !!T.refs('{{1bad}}')[0].error);
eq('rename root', T.renameRoot('{{web.results}} {{webby.x}} {{ web | json }}', 'web', 'search'), '{{search.results}} {{webby.x}} {{ search | json }}');
eq('rename keeps escapes', T.renameRoot('\\{{web}} {{web}}', 'web', 's'), '\\{{web}} {{s}}');
eq('rename in value', T.renameInValue({ a: ['{{x.y}}'], b: 1 }, 'x', 'z'), { a: ['{{z.y}}'], b: 1 });
const roots = { search: { results: [{ title: 'Alpha' }, { title: 'Beta' }], count: 2 }, trigger: { data: { name: 'Welt', html: '<b>Hi</b> &amp; bye' } }, run: { started_at: '2026-10-04T07:30:00Z' } };
eq('type preserving list', T.evaluate('{{search.results}}', roots).length, 2);
eq('mixed text', T.evaluate('Hallo {{trigger.data.name}}!', roots), 'Hallo Welt!');
eq('missing path is null', T.evaluate('{{search.nope.deep}}', roots), null);
eq('pluck join', T.evaluate('{{search.results | pluck("title") | join(" / ")}}', roots), 'Alpha / Beta');
eq('count', T.evaluate('{{search.results | count}}', roots), 2);
eq('default', T.evaluate('{{search.nope | default("x")}}', roots), 'x');
eq('truncate', T.evaluate('{{trigger.data.name | truncate(2)}}', roots), 'We…');
eq('strip_html', T.evaluate('{{trigger.data.html | strip_html}}', roots), 'Hi & bye');
eq('round', T.applyFilter('round', 3.14159, [2]), 3.14);
eq('replace', T.applyFilter('replace', 'a-b-c', ['-', '+']), 'a+b+c');
eq('date utc', T.evaluate('{{run.started_at | date("DD.MM.YYYY HH:mm", "UTC")}}', roots), '04.10.2026 07:30');
eq('objects recurse', T.evaluate({ x: '{{search.count}}' }, roots), { x: 2 });
let threw = false;
try { T.evaluate('{{search | nope}}', roots); } catch (err) { threw = true; }
check('unknown filter throws', threw);
eq('describe list', T.describe([1, 2, 3], t), 'preview_list:3');
eq('describe text', T.describe('abc', t), 'preview_text:3');
eq('describe empty', T.describe(null, t), 'preview_empty');

// ── model ──
eq('key from label', M.keyFromLabel('Web-Suche Ärger', new Set()), 'web_suche_aerger');
eq('key unique', M.keyFromLabel('PDF', new Set(['pdf', 'pdf_2'])), 'pdf_3');
eq('key reserved', M.keyFromLabel('Trigger', new Set()), 'trigger_node');
eq('key digits', M.keyFromLabel('123', new Set()), 'n_123');
eq('key empty', M.keyFromLabel('!!!', new Set()), 'node');
check('node ids', /^n_[a-z2-7]{8}$/.test(M.newNodeID()));

const types = new Map([
    ['trigger.manual', { type: 'trigger.manual', label: 'Manueller Start', trigger: true, inputs: [], outputs: ['out'], params: [], output_fields: [{ name: 'data', type: 'object', primary: true }] }],
    ['web.search', { type: 'web.search', label: 'Websuche', inputs: ['in'], outputs: ['out'], primary_input: 'query', params: [{ name: 'query', kind: 'text' }, { name: 'count', kind: 'number', default: 5 }], output_fields: [{ name: 'results', type: 'list', primary: true }] }],
    ['logic.if', { type: 'logic.if', label: 'Wenn', inputs: ['in'], outputs: ['true', 'false'], params: [] }],
    ['logic.switch', { type: 'logic.switch', label: 'Verzweigung', inputs: ['in'], outputs: ['default'], dynamic_outputs: 'cases', params: [] }],
    ['ai.step', { type: 'ai.step', label: 'KI-Schritt', inputs: ['in'], outputs: ['out'], primary_input: 'prompt', dynamic_fields: 'fields', params: [{ name: 'prompt', kind: 'textarea' }], output_fields: [{ name: 'text', type: 'text', primary: true }] }],
    ['documents.pdf', { type: 'documents.pdf', label: 'PDF', inputs: ['in'], outputs: ['out'], primary_input: 'content', params: [{ name: 'content', kind: 'textarea' }], output_fields: [{ name: 'file', type: 'file', primary: true }] }],
    ['notify.telegram', { type: 'notify.telegram', label: 'Telegram', inputs: ['in'], outputs: ['out'], primary_input: 'message', params: [{ name: 'message', kind: 'textarea' }, { name: 'file', kind: 'file' }] }]
]);
const model = M.create({ schema: 1, name: 'Test', nodes: [], edges: [] }, { types });
const start = model.addNode('trigger.manual', { x: 0, y: 0 });
const search = model.addNode('web.search', { x: 300, y: 0 });
eq('defaults applied', model.node(search).params, { count: 5 });
eq('key derived', model.node(search).key, 'websuche');
const c1 = model.connect(start, 'out', search, 'in');
check('connect ok', c1.ok);
eq('smart prefill', model.node(search).params.query, '{{manueller_start.data}}');
const pdfNode = model.addNode('documents.pdf', { x: 600, y: 0 });
const tgNode = model.addNode('notify.telegram', { x: 900, y: 0 }, null, { node: pdfNode, port: 'out' });
eq('file prefill goes to the file parameter', model.node(tgNode).params, { file: '{{pdf.file}}' });
model.removeNodes([pdfNode, tgNode]);
eq('duplicate edge refused', model.connect(start, 'out', search, 'in').reason, 'duplicate');
eq('trigger has no input', model.connect(search, 'out', start, 'in').reason, 'port');
const ai = model.addNode('ai.step', { x: 600, y: 0 });
model.connect(search, 'out', ai, 'in');
eq('cycle refused', model.connect(ai, 'out', search, 'in').reason, 'cycle');
eq('upstream', Array.from(model.upstream(ai)).sort(), [search, start].sort());
model.undo();
eq('undo removes edge', model.incoming(ai).length, 0);
model.redo();
eq('redo restores edge', model.incoming(ai).length, 1);
model.moveNodes([ai], 10, 0);
model.moveNodes([ai], 10, 0);
model.undo();
eq('moves coalesce into one undo step', model.node(ai).position.x, 600);
model.removeNodes([search], { bridge: true });
eq('remove bridges neighbours', model.incoming(ai).map(e => e.source.node), [start]);
model.undo();
eq('undo restores node and edges', [!!model.node(search), model.incoming(ai).length, model.outgoing(start).length], [true, 1, 1]);
model.setParam(ai, 'prompt', 'Fasse {{websuche.results}} zusammen');
const renamed = model.setKey(search, 'suche');
check('set key ok', renamed.ok);
eq('key rename rewrites references', model.node(ai).params.prompt, 'Fasse {{suche.results}} zusammen');
eq('reserved key refused', model.setKey(search, 'run').reason, 'reserved');
eq('taken key refused', model.setKey(search, model.node(ai).key).reason, 'taken');
const sw = model.addNode('logic.switch', { x: 900, y: 0 });
model.setParam(sw, 'cases', [{ label: 'a' }, { label: 'b' }]);
eq('switch outputs follow cases', model.outputs(model.node(sw)), ['case_1', 'case_2', 'default']);
model.setSettings(ai, { on_error: 'error_port' });
eq('error port added', model.outputs(model.node(ai)), ['out', 'error']);
model.setParam(ai, 'output_mode', 'fields');
model.setParam(ai, 'fields', [{ name: 'title' }, { name: 'summary' }]);
eq('dynamic fields', model.fieldsOf(model.node(ai)).map(f => f.name), ['title', 'summary', 'tokens', 'model']);
const frag = model.fragment([search, ai]);
const pasted = model.paste(frag, { x: 0, y: 400 });
eq('paste creates nodes', pasted.length, 2);
const pastedAI = model.node(pasted[1]);
check('paste remaps references', /\{\{suche_2\.results\}\}/.test(pastedAI.params.prompt), pastedAI.params.prompt);
eq('paste keeps inner edges', model.incoming(pasted[1]).map(e => e.source.node), [pasted[0]]);
const chained = model.addNode('web.search', { x: 0, y: 800 }, null, { node: start, port: 'out' });
eq('add connected in one undo step', model.incoming(chained).map(e => e.source.node), [start]);
model.undo();
eq('undo removes node and edge together', [!!model.node(chained), model.outgoing(start).some(e => e.target.node === chained)], [false, false]);
const ins = model.addNode('logic.if', { x: 0, y: 0 });
const edgeID = model.incoming(ai)[0].id;
const placed = model.insertOnEdge(edgeID, ins, { x: 450, y: 0 });
eq('insert on edge', [model.incoming(placed).length, model.incoming(ai)[0].source.node], [1, ins]);
model.toggleDisabled([ins]);
eq('disable', model.node(ins).settings.disabled, true);
model.toggleDisabled([ins]);
eq('enable', model.node(ins).settings.disabled, undefined);
const versionBefore = model.version;
model.setViewport({ x: 10.4, y: 20.6, zoom: 0.8 });
eq('viewport stored without undo step', [model.doc.viewport.x, model.version > versionBefore], [10, true]);

// ── geometry ──
eq('single port in middle', G.portPoint({ x: 0, y: 0 }, 'out', 0, 1), { x: 232, y: 36 });
eq('stacked ports', G.portPoint({ x: 0, y: 0 }, 'out', 1, 2), { x: 232, y: 60 });
eq('height grows with ports', G.nodeHeight(3), 36 + 48 + 26);
check('wire path', /^M0 0 C/.test(G.wirePath({ x: 0, y: 0 }, { x: 300, y: 100 })));
check('distance to wire small on curve', G.distanceToWire(G.wireMidpoint({ x: 0, y: 0 }, { x: 300, y: 100 }), { x: 0, y: 0 }, { x: 300, y: 100 }) < 1);
const view = G.zoomAt({ x: 0, y: 0, zoom: 1 }, 2, { x: 100, y: 100 });
eq('zoom keeps point under cursor', G.toWorld(view, { x: 100, y: 100 }), { x: 100, y: 100 });
eq('zoom clamps', G.zoomAt({ x: 0, y: 0, zoom: 1 }, 9, { x: 0, y: 0 }).zoom, G.MAX_ZOOM);
const fitted = G.fit({ x: 0, y: 0, w: 1000, h: 500 }, { w: 800, h: 600 }, 50);
check('fit shows box', fitted.zoom > 0.6 && fitted.zoom < 0.8, JSON.stringify(fitted));
eq('rect normalize', G.normalizeRect({ x: 10, y: 10 }, { x: 0, y: 5 }), { x: 0, y: 5, w: 10, h: 5 });
eq('free spot avoids overlap', G.freeSpot({ x: 0, y: 0 }, [{ x: 0, y: 0, w: 232, h: 72 }]).y > 72, true);

// ── c1d02 extras (controller requirements beyond the plan) ──
{
    // Merge node fields follow the mode (dynamic_fields "mode"), see internal/flows/nodes_logic.go mergeNodeDef.
    const mergeTypes = new Map(types);
    mergeTypes.set('logic.merge', { type: 'logic.merge', label: 'Zusammenführen', inputs: ['in'], outputs: ['out'], dynamic_fields: 'mode', params: [{ name: 'mode', kind: 'segmented', default: 'wait_all' }, { name: 'field', kind: 'text', default: 'items' }], output_fields: [] });
    const mm = M.create({ schema: 1, name: 'Merge', nodes: [], edges: [] }, { types: mergeTypes });
    const trg = mm.addNode('trigger.manual', { x: 0, y: 0 });
    const s1 = mm.addNode('web.search', { x: 300, y: 0 }, null, { node: trg, port: 'out' });
    const s2 = mm.addNode('web.search', { x: 300, y: 200 }, null, { node: trg, port: 'out' });
    const merge = mm.addNode('logic.merge', { x: 600, y: 0 });
    eq('c1d02 merge without upstream nodes has no fields', mm.fieldsOf(mm.node(merge)), []);
    mm.connect(s1, 'out', merge, 'in');
    mm.connect(s2, 'out', merge, 'in');
    eq('c1d02 merge wait_all lists upstream keys in edge order', mm.fieldsOf(mm.node(merge)), [{ name: 'websuche', type: 'object' }, { name: 'websuche_2', type: 'object' }]);
    mm.setParam(merge, 'mode', undefined);
    eq('c1d02 merge without mode param lists upstream keys', mm.fieldsOf(mm.node(merge)).map(f => f.name), ['websuche', 'websuche_2']);
    mm.setParam(merge, 'mode', 'append');
    eq('c1d02 merge append outputs items and count', mm.fieldsOf(mm.node(merge)), [{ name: 'items', type: 'list', primary: true }, { name: 'count', type: 'number' }]);
    const branch = mm.addNode('logic.if', { x: 300, y: 400 }, null, { node: trg, port: 'out' });
    const merge2 = mm.addNode('logic.merge', { x: 600, y: 400 });
    mm.connect(branch, 'true', merge2, 'in');
    mm.connect(branch, 'false', merge2, 'in');
    eq('c1d02 merge skips duplicate upstream nodes', mm.fieldsOf(mm.node(merge2)).map(f => f.name), ['wenn']);
}
{
    // Preview filters follow the Go engine (internal/flows/filters.go); the expectations are Go's results.
    eq('c1d02 round halves away from zero', [T.applyFilter('round', -2.5, []), T.applyFilter('round', 2.5, []), T.applyFilter('round', -0.125, [2])], [-3, 3, -0.13]);
    check('c1d02 round has no negative zero', Object.is(T.applyFilter('round', -0.4, []), 0));
    let tooManyDecimals = false;
    try { T.applyFilter('round', 3.14159, [11]); } catch (err) { tooManyDecimals = true; }
    check('c1d02 round allows at most 10 decimals', tooManyDecimals);
    eq('c1d02 replace with empty search keeps the text', T.applyFilter('replace', 'a-b-c', ['', '+']), 'a-b-c');
    eq('c1d02 strip_html separates blocks', T.applyFilter('strip_html', '<p>a</p><p>b</p><br/>c', []), 'a b c');
    eq('c1d02 strip_html drops script and style', T.applyFilter('strip_html', '<script>x()</script>Hi<style>p{}</style>', []), 'Hi');
    eq('c1d02 strip_html keeps comparisons', T.applyFilter('strip_html', 'x < 5 and y > 3', []), 'x < 5 and y > 3');
    eq('c1d02 strip_html drops comments and collapses whitespace', T.applyFilter('strip_html', 'a <!-- c -->\n\n b', []), 'a b');
    eq('c1d02 strip_html decodes numeric entities', T.applyFilter('strip_html', '&#228;&#x263A; &lt;b&gt;', []), 'ä☺ <b>');
    eq('c1d02 upper keeps sharp s', T.applyFilter('upper', 'straße', []), 'STRAßE');
    eq('c1d02 lower maps one letter at a time', [T.applyFilter('lower', 'ΟΔΟΣ', []), T.applyFilter('lower', 'İstanbul', [])], ['οδοσ', 'istanbul']);
}
{
    // Template syntax follows ParseTemplate and parseExpr (internal/flows/template.go); each check states Go's behaviour.
    const data = { v: { s: 'a-b-c', list: [1, 2, 3], obj: { b: 1, a: { d: 2, c: 3 } }, ik: { a: 3, 9: 2, 10: 1 } } };
    // template.go:282-291 accepts a negative index; template_eval.go:101-107 counts it from the end.
    eq('c1d02 negative index counts from the end', [T.evaluate('{{v.list[-1]}}', data), T.evaluate('{{v.list[-4]}}', data)], [3, null]);
    check('c1d02 negative index is no syntax error', !T.refs('{{v.list[-1]}}')[0].error);
    // template.go:114-137 (findExprEnd) skips quoted strings when it looks for "}}".
    eq('c1d02 quoted }} does not end the expression', T.evaluate('a {{v.nope | default("}}")}} b', data), 'a }} b');
    // template.go:136: a "{{" without its "}}" fails the template.
    let unclosedThrew = false;
    try { T.evaluate('x {{v.s', data); } catch (err) { unclosedThrew = true; }
    check('c1d02 unclosed braces are an error', unclosedThrew && T.refs('x {{v.s')[0].error === 'unclosed {{' && T.refs('{{v.s | default("x}}')[0].error === 'unclosed {{');
    // template.go:167-169: the lexer skips blanks between tokens.
    eq('c1d02 blanks inside paths are allowed', [T.evaluate('{{ v . s }}', data), T.parseExpr('v [ 0 ] . x').path], ['a-b-c', [0, 'x']]);
    // template.go:264-270: the step after "." must be a name.
    check('c1d02 a number or dash after a dot is an error', !!T.refs('{{v.0}}')[0].error && !!T.refs('{{v.content-type}}')[0].error);
    // template.go:327-329 runs checkFilterCall (filters.go:67-79) while parsing.
    check('c1d02 refs flags unknown filters and argument counts', ['{{v | nope}}', '{{v | truncate}}', '{{v | upper(1)}}', '{{v | toString}}'].every(s => !!T.refs(s)[0].error));
    // values.go:57-66 and filters.go:285-291 use encoding/json, which sorts object keys by their bytes.
    eq('c1d02 json and text sort object keys', [T.applyFilter('json', data.v.obj, []), T.evaluate('x {{v.ik}}', data)], ['{"a":{"c":3,"d":2},"b":1}', 'x {"10":1,"9":2,"a":3}']);
    // values.go:138-146 parses with time.ParseInLocation, which rejects impossible fields (filters.go:337-340 "not a date").
    const notDate = s => { try { T.applyFilter('date', s, ['DD.MM.YYYY', 'UTC']); return false; } catch (err) { return /not a date/.test(err.message); } };
    check('c1d02 date rejects impossible calendar fields', ['2026-02-30', '2026-02-29T07:30:00Z', '2026-04-31 10:00', '2026-01-01T24:00:00Z', '2026-13-01'].every(notDate));
    eq('c1d02 date keeps real leap days', T.applyFilter('date', '2028-02-29T07:30:00Z', ['DD.MM.YYYY', 'UTC']), '29.02.2028');
    // values.go:72-85 (encoding/json) escapes U+2028 and U+2029 even without HTML escaping.
    const LS = String.fromCharCode(0x2028);
    const PS = String.fromCharCode(0x2029);
    const BS = String.fromCharCode(92);
    eq('c1d02 json escapes line and paragraph separators', [T.applyFilter('json', { ['k' + LS]: 'a' + PS + 'b' }, []), T.evaluate('x {{v.o}}', { v: { o: ['a' + LS] } })], ['{"k' + BS + 'u2028":"a' + BS + 'u2029b"}', 'x ["a' + BS + 'u2028"]']);
}

if (failures) {
    console.log(failures + ' failure(s)');
    process.exit(1);
}
console.log('all EasyDrag checks passed');
