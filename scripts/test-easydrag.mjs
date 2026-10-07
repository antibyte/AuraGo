#!/usr/bin/env node
// Runs the pure EasyDrag modules (template, model, geometry, preview, shortcut table) in Node and checks them.
// The c1d03 checks also run core and saver with fake timers, and the desktop shell's api() with a stub fetch.
// The c1d04 checks (test-easydrag-extra.mjs) run canvas, wires and interact on a small stub DOM.
// The c1d06 checks (test-easydrag-extra2.mjs) run detail, runs and publish with a stub EventSource.
// The c1d07 checks (test-easydrag-extra3.mjs) run the start page, the editor and the window shell on every module.
// The c1d14 checks (test-easydrag-extra4.mjs) run the canvas's stored-view restore and readable fit.
// The FF2 checks (test-easydrag-extra5.mjs) run the editor of the c1d07 sandbox: changes made
// elsewhere, the shared state words, pans, the saver's emergency copy, run views and focus.
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
for (const file of ['easydrag-template.js', 'easydrag-model.js', 'easydrag-geometry.js', 'easydrag-home.js', 'easydrag-dialogs.js']) {
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
// FF2 M1: the view is stored per device, never in the document; an imported viewport is kept as it came.
const withView = M.create({ schema: 1, name: 'V', nodes: [], edges: [], viewport: { x: 10, y: 20, zoom: 0.8 } }, { types });
eq('the model has no viewport command and keeps a document viewport untouched', [typeof model.setViewport, withView.toJSON().viewport, withView.version], ['undefined', { x: 10, y: 20, zoom: 0.8 }, 0]);

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
// Each block runs in guard(): a check that throws counts as one failure and the other blocks still run.
function guard(name, fn) {
    try { fn(); } catch (err) { failures++; console.log('FAIL ' + name + ' threw: ' + (err && err.message)); }
}
guard('c1d02 merge fields', () => {
    // Merge node fields follow the mode (dynamic_fields "mode"), see mergeNodeDef in internal/flows/nodes_logic.go.
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
});
guard('c1d02 filters', () => {
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
});
guard('c1d02 template syntax', () => {
    // Template syntax follows ParseTemplate and parseExpr (internal/flows/template.go); each check states Go's behaviour.
    const data = { v: { s: 'a-b-c', list: [1, 2, 3], obj: { b: 1, a: { d: 2, c: 3 } }, ik: { a: 3, 9: 2, 10: 1 } } };
    // parseExpr (template.go) accepts a negative index; stepInto (template_eval.go) counts it from the end.
    eq('c1d02 negative index counts from the end', [T.evaluate('{{v.list[-1]}}', data), T.evaluate('{{v.list[-4]}}', data)], [3, null]);
    check('c1d02 negative index is no syntax error', !T.refs('{{v.list[-1]}}')[0].error);
    // findExprEnd (template.go) skips quoted strings when it looks for "}}".
    eq('c1d02 quoted }} does not end the expression', T.evaluate('a {{v.nope | default("}}")}} b', data), 'a }} b');
    // findExprEnd (template.go): a "{{" without its "}}" fails the template.
    let unclosedThrew = false;
    try { T.evaluate('x {{v.s', data); } catch (err) { unclosedThrew = true; }
    check('c1d02 unclosed braces are an error', unclosedThrew && T.refs('x {{v.s')[0].error === 'unclosed {{' && T.refs('{{v.s | default("x}}')[0].error === 'unclosed {{');
    // exprLexer.next (template.go) skips blanks between tokens.
    eq('c1d02 blanks inside paths are allowed', [T.evaluate('{{ v . s }}', data), T.parseExpr('v [ 0 ] . x').path], ['a-b-c', [0, 'x']]);
    // parseExpr (template.go): the step after "." must be a name.
    check('c1d02 a number or dash after a dot is an error', !!T.refs('{{v.0}}')[0].error && !!T.refs('{{v.content-type}}')[0].error);
    // parseExpr (template.go) runs checkFilterCall (filters.go) while parsing.
    check('c1d02 refs flags unknown filters and argument counts', ['{{v | nope}}', '{{v | truncate}}', '{{v | upper(1)}}', '{{v | toString}}'].every(s => !!T.refs(s)[0].error));
    // Stringify (values.go) and filterJSON (filters.go) use encoding/json, which sorts object keys by their bytes.
    eq('c1d02 json and text sort object keys', [T.applyFilter('json', data.v.obj, []), T.evaluate('x {{v.ik}}', data)], ['{"a":{"c":3,"d":2},"b":1}', 'x {"10":1,"9":2,"a":3}']);
    // toTime (values.go) parses with time.ParseInLocation, which rejects impossible fields; filterDate (filters.go) says "not a date".
    const notDate = s => { try { T.applyFilter('date', s, ['DD.MM.YYYY', 'UTC']); return false; } catch (err) { return /not a date/.test(err.message); } };
    check('c1d02 date rejects impossible calendar fields', ['2026-02-30', '2026-02-29T07:30:00Z', '2026-04-31 10:00', '2026-01-01T24:00:00Z', '2026-13-01'].every(notDate));
    eq('c1d02 date keeps real leap days', T.applyFilter('date', '2028-02-29T07:30:00Z', ['DD.MM.YYYY', 'UTC']), '29.02.2028');
    // marshalCompact (values.go) uses encoding/json, which escapes U+2028 and U+2029 even without HTML escaping.
    const LS = String.fromCharCode(0x2028);
    const PS = String.fromCharCode(0x2029);
    const BS = String.fromCharCode(92);
    eq('c1d02 json escapes line and paragraph separators', [T.applyFilter('json', { ['k' + LS]: 'a' + PS + 'b' }, []), T.evaluate('x {{v.o}}', { v: { o: ['a' + LS] } })], ['{"k' + BS + 'u2028":"a' + BS + 'u2029b"}', 'x ["a' + BS + 'u2028"]']);
});
guard('c1d02 template review fixes', () => {
    const throwsWith = (fn, pattern) => { try { fn(); return false; } catch (err) { return pattern.test(err.message); } };
    // renameRoots replaces each expression root at most once, so a rename chain cannot apply twice.
    eq('c1d02 renameRoots renames each root once', T.renameRoots('{{a.r}} + {{ a_2 | json}} {{ab}}', new Map([['a', 'a_2'], ['a_2', 'a_2_2']])), '{{a_2.r}} + {{ a_2_2 | json}} {{ab}}');
    // filterStripHTML (filters.go): the preview scanner stays linear on hostile input and keeps Go's output.
    for (const unit of ['<a ', '<?', '<!D', '<script ', '<!--']) {
        const s = unit.repeat(Math.ceil(200000 / unit.length));
        const t0 = performance.now();
        T.applyFilter('strip_html', s, []);
        const ms = performance.now() - t0;
        check('c1d02 strip_html takes under 100 ms for 200 KB of ' + JSON.stringify(unit), ms < 100, ms.toFixed(1) + ' ms');
    }
    eq('c1d02 strip_html keeps Go results', ['<SCRIPT type="x">bad()</SCRIPT >ok', 'a<b and c>d', '<!DOCTYPE html><p>x', '<?xml v?>z', '<a href="x>y">link</a>'].map(s => T.applyFilter('strip_html', s, [])), ['ok', 'a d', 'x', 'z', 'y">link']);
    // toTime (values.go) accepts only Go's layouts; filterDate (filters.go) says "not a date" for the rest.
    const notDate = s => throwsWith(() => T.applyFilter('date', s, ['YYYY', 'UTC']), /not a date/);
    check('c1d02 date rejects forms Go does not parse', ['Sun, 04 Oct 2026 07:30:00 GMT', '10/04/2026', '2026-10', '2026-10-04t07:30:00z', '2026-10-04T07:30:00z', '2026-10-04 07:30:00Z', '2026-10-04T07:30Z', '1759563000'].every(notDate));
    eq('c1d02 date reads the Go layouts', ['2026-10-04T07:30:00,5+02:00', '2026-10-04T7:30:00Z', '2026-10-04T07:30:00+24:00', '0099-05-06T00:00:00Z'].map(s => T.applyFilter('date', s, ['YYYY-MM-DD HH:mm', 'UTC'])), ['2026-10-04 05:30', '2026-10-04 07:30', '2026-10-03 07:30', '0099-05-06 00:00']);
    const local = T.toDate('2026-10-04  7:30:00,25');
    check('c1d02 date reads zone-less forms in local time', !!local && local.getFullYear() === 2026 && local.getHours() === 7 && local.getMinutes() === 30 && local.getMilliseconds() === 250);
    // filterPluck (filters.go) reads map entries only, never inherited members.
    eq('c1d02 pluck reads own fields only', T.evaluate('{{a | pluck("constructor") | json}}', { a: [{ x: 1 }, { x: 2 }] }), '[null,null]');
    // An own "__proto__" key from JSON stays data in evaluate and renameInValue.
    const hostile = JSON.parse('{"__proto__": {"polluted": "yes"}, "b": "{{a.s}}"}');
    const evaluated = T.evaluate(hostile, { a: { s: 'x' } });
    const renamed = T.renameInValue(hostile, 'a', 'z');
    eq('c1d02 own __proto__ keys stay data', [Object.keys(evaluated), evaluated.polluted, JSON.stringify(evaluated), JSON.stringify(renamed), ({}).polluted], [['__proto__', 'b'], undefined, '{"__proto__":{"polluted":"yes"},"b":"x"}', '{"__proto__":{"polluted":"yes"},"b":"{{z.s}}"}', undefined]);
    // quoteForError (filters.go) echoes at most 40 characters of user input.
    check('c1d02 errors quote at most 40 characters', T.refs('{{a "' + 'x'.repeat(5000) + '"}}')[0].error.length < 60 && T.refs('{{a | ' + 'y'.repeat(5000) + '}}')[0].error.length < 60);
    // intArg (filters.go) refuses whole numbers beyond MaxInt32.
    check('c1d02 truncate refuses arguments beyond 2^31', throwsWith(() => T.applyFilter('truncate', 'abc', [2 ** 31]), /argument too large/));
    // filterJoin, filterReplace and filterSplit (filters.go), Evaluate and resolveValue (template_eval.go) cap their output.
    check('c1d02 join result is capped at 8 MiB', throwsWith(() => T.applyFilter('join', new Array(9000).fill('y'.repeat(1000)), [',']), /join result would exceed 8388608 bytes/));
    check('c1d02 replace result is capped at 8 MiB', throwsWith(() => T.applyFilter('replace', 'x'.repeat(100000), ['x', 'y'.repeat(100)]), /replace result would exceed 8388608 bytes/));
    check('c1d02 split stops at 100000 parts', T.applyFilter('split', 'x'.repeat(100000), ['']).length === 100000 && throwsWith(() => T.applyFilter('split', 'x'.repeat(100001), ['']), /more than 100000 parts/));
    check('c1d02 template text is capped at 8 MiB', throwsWith(() => T.evaluate('{{a}}{{a}}', { a: 'x'.repeat(5 << 20) }), /template output exceeds 8 MiB/));
    const nested = k => { let v = 'x'; for (let i = 0; i < k; i++) v = [v]; return v; };
    check('c1d02 parameter values nest at most 32 levels', !throwsWith(() => T.evaluate(nested(31), {}), /./) && throwsWith(() => T.evaluate(nested(32), {}), /nested deeper than 32 levels/));
    // resolvePath shares the engine's path rules (stepInto in template_eval.go).
    eq('c1d02 resolvePath follows the engine', [T.resolvePath({ a: [{ b: 1 }, { b: 2 }] }, ['a', -1, 'b']), T.resolvePath({}, ['constructor']), T.resolvePath('str', [0])], [2, null, null]);
});
guard('c1d02 bare vm context', () => {
    // The modules need no host intrinsics: they run in a context that only has window and crypto.
    const bare = vm.createContext({ window: {}, crypto: webcrypto });
    for (const file of ['easydrag-template.js', 'easydrag-model.js', 'easydrag-geometry.js']) {
        const full = path.join(apps, file);
        vm.runInContext(fs.readFileSync(full, 'utf8'), bare, { filename: full });
    }
    const BED = bare.window.EasyDrag;
    const bm = BED.model.create({ nodes: [], edges: [] }, { types });
    const id = bm.addNode('web.search', { x: 0, y: 0 });
    eq('c1d02 modules run without host intrinsics', [BED.template.evaluate('{{a.b | upper}}', { a: { b: 'x' } }), bm.node(id).key, BED.geometry.nodeHeight(1)], ['X', 'websuche', 72]);
});
guard('c1d02 model review fixes', () => {
    // Paste renames each reference once: nodes a and a_2 plus c using both, pasted into a flow that already has a.
    const f1 = M.create({ nodes: [], edges: [] }, { types });
    const na = f1.addNode('web.search', { x: 0, y: 0 });
    f1.setKey(na, 'a');
    const na2 = f1.addNode('web.search', { x: 0, y: 100 });
    f1.setKey(na2, 'a_2');
    const nc = f1.addNode('web.search', { x: 300, y: 0 });
    f1.setParam(nc, 'query', '{{a.results}} + {{a_2.results}}');
    const f2 = M.create({ nodes: [], edges: [] }, { types });
    f2.setKey(f2.addNode('web.search', { x: 0, y: 0 }), 'a');
    const chain = f2.paste(JSON.parse(JSON.stringify(f1.fragment([na, na2, nc]))), { x: 0, y: 300 });
    eq('c1d02 paste renames each reference once', [chain.slice(0, 2).map(id => f2.node(id).key), f2.node(chain[2]).params.query], [['a_2', 'a_2_2'], '{{a_2.results}} + {{a_2_2.results}}']);
    // Undo restores the exact document order, edges included.
    const u = M.create({ nodes: [], edges: [] }, { types });
    const [ua, ub, uc, ud] = [0, 1, 2, 3].map(i => u.addNode('web.search', { x: i * 300, y: 0 }));
    u.connect(ua, 'out', ub, 'in');
    u.connect(ua, 'out', ud, 'in');
    u.connect(ub, 'out', uc, 'in');
    u.connect(uc, 'out', ud, 'in');
    const docBefore = JSON.stringify(u.doc);
    u.removeNodes([ub]);
    u.undo();
    eq('c1d02 undo of a node delete restores the document', JSON.stringify(u.doc), docBefore);
    const fan = M.create({ nodes: [], edges: [] }, { types });
    const sink = fan.addNode('logic.if', { x: 900, y: 0 });
    [0, 1, 2].forEach(i => fan.connect(fan.addNode('web.search', { x: 300, y: i * 100 }), 'out', sink, 'in'));
    const fanBefore = JSON.stringify(fan.doc);
    fan.disconnect(fan.incoming(sink).slice(0, 2).map(e => e.id));
    fan.undo();
    eq('c1d02 undo of a two-edge disconnect restores the document', JSON.stringify(fan.doc), fanBefore);
    u.removeNodes([ub, uc]);
    u.undo();
    eq('c1d02 undo of a two-node delete restores the document', JSON.stringify(u.doc), docBefore);
    // Paste trusts nothing from clipboard JSON.
    const p = M.create({ nodes: [], edges: [] }, { types });
    const frag = (nodes, edges) => ({ easydrag: 1, nodes, edges: edges || [] });
    const keysOf = ids => ids.map(id => p.node(id).key);
    eq('c1d02 paste gives keyless nodes keys from their labels', keysOf(p.paste(frag([{ id: 'a', type: 'web.search', label: 'Foo' }, { id: 'b', type: 'web.search', label: 'Bar' }]), { x: 0, y: 0 })), ['foo', 'bar']);
    const keyless = p.paste(frag([{ id: 'a', type: 'web.search', label: 'Foo', params: { query: '{{x.y}}' } }]), { x: 0, y: 0 });
    eq('c1d02 paste of a keyless node keeps its references', [keysOf(keyless), p.node(keyless[0]).params.query], [['foo_2'], '{{x.y}}']);
    eq('c1d02 paste gives duplicate keys a fresh key', keysOf(p.paste(frag([{ id: 'a', key: 'k', type: 'web.search' }, { id: 'b', key: 'k', type: 'web.search' }]), { x: 0, y: 0 })), ['k', 'websuche']);
    const nodeCount = p.doc.nodes.length;
    eq('c1d02 paste drops duplicate ids', [p.paste(frag([{ id: 'a', key: 'p1', type: 'web.search' }, { id: 'a', key: 'p2', type: 'web.search' }]), { x: 0, y: 0 }).length, p.doc.nodes.length - nodeCount], [1, 1]);
    eq('c1d02 paste drops unknown types, typeless and non-object nodes', [p.paste(frag([{ id: 'a', key: 'zz', type: 'evil.type' }]), { x: 0, y: 0 }), p.paste(frag([{ id: 'a', key: 'zz2' }]), { x: 0, y: 0 }), p.paste(frag([null, 'x', 5]), { x: 0, y: 0 })], [[], [], []]);
    const protoFrag = JSON.parse('{"easydrag":1,"nodes":[{"id":"a","key":"k","type":"web.search","params":{"__proto__":{"polluted":1},"query":"{{k.results}}"}}],"edges":[]}');
    const protoNode = p.node(p.paste(protoFrag, { x: 0, y: 0 })[0]);
    eq('c1d02 paste keeps an own __proto__ param as data', [Object.keys(protoNode.params), protoNode.params.query, ({}).polluted], [['__proto__', 'query'], '{{k_2.results}}', undefined]);
    const odd = p.paste(frag([{ id: 'a', key: 'str', type: 'web.search', params: 'oops', settings: 'oops', position: { x: 'abc', y: Infinity } }]), { x: 10, y: 20 })[0];
    p.setParam(odd, 'query', 'x');
    p.toggleDisabled([odd]);
    eq('c1d02 paste coerces params, settings and positions', [p.node(odd).params, p.node(odd).settings, p.node(odd).position], [{ query: 'x' }, { disabled: true }, { x: 10, y: 20 }]);
    const wired = p.paste(frag([{ id: 'a', key: 'cy1', type: 'web.search' }, { id: 'b', key: 'cy2', type: 'web.search' }], [{}, null, { source: 'a', target: 'b' },
        { source: { node: 'a', port: 'out' }, target: { node: 'b', port: 'in' } }, { source: { node: 'a', port: 'out' }, target: { node: 'b', port: 'in' } },
        { source: { node: 'b', port: 'out' }, target: { node: 'a', port: 'in' } }, { source: { node: 'a', port: 'bogus' }, target: { node: 'b', port: 'in' } },
        { source: { node: 'a', port: 'out' }, target: { node: 'a', port: 'in' } }, { source: { node: 'a', port: 'out' }, target: { node: 'ghost', port: 'in' } }]), { x: 0, y: 0 });
    eq('c1d02 paste keeps only valid, unique, acyclic edges', [p.incoming(wired[0]).length, p.incoming(wired[1]).length], [0, 1]);
    const tooManyNodes = frag(Array.from({ length: 501 }, (_, i) => ({ id: 'n' + i, type: 'web.search' })));
    const tooManyEdges = frag([{ id: 'a', type: 'web.search' }], Array.from({ length: 2001 }, () => ({})));
    eq('c1d02 paste refuses fragments over 500 nodes or 2000 edges', [p.paste(tooManyNodes, { x: 0, y: 0 }).length, p.paste(tooManyEdges, { x: 0, y: 0 }).length], [0, 0]);
    // A bridge on delete does not duplicate an edge that already exists (no EDGE_DUPLICATE).
    const br = M.create({ nodes: [], edges: [] }, { types });
    const b1 = br.addNode('web.search', { x: 0, y: 0 });
    const b2 = br.addNode('web.search', { x: 300, y: 0 }, null, { node: b1, port: 'out' });
    const b3 = br.addNode('web.search', { x: 600, y: 0 }, null, { node: b2, port: 'out' });
    br.connect(b1, 'out', b3, 'in');
    br.removeNodes([b2], { bridge: true });
    eq('c1d02 bridge skips an existing edge', br.outgoing(b1).filter(e => e.target.node === b3).length, 1);
    // A tampered or corrupt document (emergency copy -> replaceDoc) loses malformed nodes and edges instead of throwing.
    const corrupt = () => ({ nodes: [null, 'x', [], { id: 'n1', key: 'a', type: 'web.search' }], edges: [{ id: 'bad' }, null, { id: 'e1', source: 'n1', target: {} },
        { id: 'ok', source: { node: 'n1', port: 'out' }, target: { node: 'n1', port: 'in' } }] });
    const loaded = M.create(corrupt(), { types });
    const replaced = M.create({ nodes: [], edges: [] }, { types });
    replaced.replaceDoc(corrupt());
    const emptied = M.create({ nodes: [], edges: [] }, { types });
    emptied.replaceDoc(null);
    eq('c1d02 malformed nodes and edges are dropped on load', [loaded.doc.nodes.length, loaded.doc.edges.map(e => e.id), loaded.incoming('n1').length,
        replaced.doc.nodes.length, replaced.incoming('n1').length, emptied.doc.nodes.length], [1, ['ok'], 1, 1, 1, 0]);
    eq('c1d02 clampZoom maps non-finite values to 1', [G.clampZoom(NaN), G.clampZoom(Infinity), G.clampZoom(undefined), G.zoomAt({ x: 0, y: 0, zoom: 1 }, NaN, { x: 10, y: 10 }).zoom], [1, 1, 1, 1]);
});

// ── c1d03 extras: autosave errors and the shell's api() errors ──
async function guardAsync(name, fn) {
    try { await fn(); } catch (err) { failures++; console.log('FAIL ' + name + ' threw: ' + (err && err.message)); }
}
const tick = () => new Promise(resolve => setImmediate(resolve));
const settle = async () => { for (let i = 0; i < 10; i++) await tick(); };
const deferred = () => { let resolve, reject; const p = new Promise((a, b) => { resolve = a; reject = b; }); return { p, resolve, reject }; };
// within reports whether p settled while the queued work ran, so a saver that waits forever fails a check instead of hanging.
async function within(p) {
    let out = { done: false };
    Promise.resolve(p).then(value => { out = { done: true, value }; }, err => { out = { done: true, error: err }; });
    await settle();
    return out;
}
// A rejection nobody handles is a failure, not a crash of the runner.
process.on('unhandledRejection', err => { failures++; console.log('FAIL unhandled rejection: ' + (err && err.message)); });
const apiError = code => Object.assign(new Error('text ' + code), { body: { error: 'text ' + code, code } });
const DRAFT_KEY = 'aurago.easydrag.draft.f1';
// saverHarness runs core and saver with fake timers (recorded, fired by hand), a stub localStorage and a
// stub api. With a responses array, save() answers with its next entry (an Error rejects; the last one
// repeats); with null, each save() stays open until the test settles h.saves[i].df. get() and the conflict
// dialog stay open until the test settles h.gets[i] and h.conflicts[i]. opts: store, onSaved, onState,
// onInvalid, onConflict. The sandbox's console.error lines land in h.logged.
function saverHarness(responses, opts = {}) {
    const timers = new Map();
    let nextTimer = 1;
    const store = opts.store || new Map();
    const logged = [];
    const box = vm.createContext({
        window: {}, navigator: { platform: 'Linux' }, crypto: webcrypto,
        console: { log() {}, warn() {}, error: (...args) => { logged.push(args.map(String).join(' ')); } },
        localStorage: {
            get length() { return store.size; },
            key: i => Array.from(store.keys())[i] ?? null,
            getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); }
        },
        setTimeout: (fn, ms) => { const id = nextTimer++; timers.set(id, { fn, ms }); return id; },
        clearTimeout: id => { timers.delete(id); }
    });
    for (const file of ['easydrag-core.js', 'easydrag-saver.js']) {
        const full = path.join(apps, file);
        vm.runInContext(fs.readFileSync(full, 'utf8'), box, { filename: full });
    }
    const ED = box.window.EasyDrag;
    let version = 1;
    let doc = { schema: 1, name: 'v1', nodes: [], edges: [] };
    const model = {
        get version() { return version; },
        toJSON: () => JSON.parse(JSON.stringify(doc)),
        replaceDoc(next) { doc = JSON.parse(JSON.stringify(next)); version++; }
    };
    const saves = [];
    const gets = [];
    const conflicts = [];
    const api = {
        calls: 0,
        save(id, sent, rev) {
            api.calls++;
            const entry = { doc: sent, rev, df: deferred() };
            saves.push(entry);
            if (responses) {
                const next = responses.length > 1 ? responses.shift() : responses[0];
                if (next instanceof Error) entry.df.reject(next); else entry.df.resolve(next);
            }
            return entry.df.p;
        },
        get() { const df = deferred(); gets.push(df); return df.p; }
    };
    const states = [];
    const saver = ED.saver.create({
        api, flowId: 'f1', model, revision: 1,
        onState: s => { states.push(s); if (opts.onState) opts.onState(s); },
        onSaved: opts.onSaved,
        onInvalid: opts.onInvalid,
        onConflict: opts.onConflict || (() => { const df = deferred(); conflicts.push(df); return df.p; })
    });
    return {
        saver, api, store, timers, saves, gets, conflicts, states, logged, model, ED, core: ED.core,
        change(name) { version++; doc = Object.assign({}, doc, { name: name || ('v' + version) }); saver.schedule(); },
        copy() { const raw = store.get(DRAFT_KEY); return raw ? JSON.parse(raw) : null; },
        delays: () => Array.from(timers.values()).map(entry => entry.ms),
        async fire(ms) {
            for (const [id, entry] of timers) {
                if (entry.ms === ms) { timers.delete(id); entry.fn(); await settle(); return; }
            }
            throw new Error('no timer of ' + ms + ' ms');
        }
    };
}
await guardAsync('c1d03 permanent save errors', async () => {
    for (const code of ['FLOW_TOO_LARGE', 'FLOW_NOT_FOUND', 'FLOW_PERMISSION_DENIED', 'FLOW_LOCKED', 'FLOW_BAD_REQUEST']) {
        const h = saverHarness([apiError(code)]);
        h.change();
        await h.saver.save();
        eq('c1d03 ' + code + ' fails without a retry and keeps the emergency copy', [h.saver.state, h.saver.error && h.saver.error.body.code, h.timers.size, h.api.calls, h.store.has(DRAFT_KEY)], ['failed', code, 0, 1, true]);
    }
    const h = saverHarness([apiError('FLOW_LOCKED'), apiError('FLOW_LOCKED'), { draft_revision: 2, issues: [] }]);
    h.change();
    await h.saver.save();
    check('c1d03 saver.error is read-only', !Object.getOwnPropertyDescriptor(h.saver, 'error').set);
    h.change();
    // (A 500 ms timer may hold back the copy of this second change: FF2's trailing copy write.)
    eq('c1d03 a later change schedules a new attempt', [h.saver.state, h.delays().filter(ms => ms !== 500)], ['dirty', [1000]]);
    await h.fire(1000);
    eq('c1d03 the new attempt fails again without a retry', [h.saver.state, h.api.calls, h.timers.size], ['failed', 2, 0]);
    await h.saver.save();
    eq('c1d03 an explicit save tries again and clears the error', [h.saver.state, h.saver.error, h.api.calls, h.store.has(DRAFT_KEY)], ['saved', null, 3, false]);
});
await guardAsync('c1d03 transient save errors', async () => {
    for (const code of ['FLOWS_DISABLED', 'FLOW_INTERNAL', 'FLOW_RATE_LIMITED', 'FLOW_RUN_LIMIT']) {
        const h = saverHarness([apiError(code)]);
        h.change();
        await h.saver.save();
        eq('c1d03 ' + code + ' goes offline and retries', [h.saver.state, h.saver.error && h.saver.error.body.code, h.delays(), h.store.has(DRAFT_KEY)], ['offline', code, [5000], true]);
    }
    const h = saverHarness([new TypeError('Failed to fetch')]);
    h.change();
    await h.saver.save();
    const seen = [];
    for (let i = 0; i < 6; i++) {
        seen.push(h.delays()[0]);
        await h.fire(seen[i]);
    }
    eq('c1d03 network errors retry after 5 s, doubling up to 60 s', [seen, h.api.calls, h.saver.state], [[5000, 10000, 20000, 40000, 60000, 60000], 7, 'offline']);
    const r = saverHarness([new TypeError('x'), new TypeError('x'), { draft_revision: 2, issues: [] }, new TypeError('x')]);
    r.change();
    await r.saver.save();
    await r.fire(5000);
    await r.fire(10000);
    eq('c1d03 a success clears the error and the retry', [r.saver.state, r.saver.error, r.timers.size, r.saver.revision], ['saved', null, 0, 2]);
    r.change();
    await r.saver.save();
    eq('c1d03 a success resets the backoff', [r.saver.state, r.delays()], ['offline', [5000]]);
    const d = saverHarness([new TypeError('x')]);
    d.change();
    const attempt = d.saver.save();
    d.saver.dispose();
    await attempt;
    eq('c1d03 a disposed saver schedules no retry', d.timers.size, 0);
});
// The 1d-03 quality review: emergency-copy revisions, FLOW_INVALID, conflicts, callbacks, answers and sweep.
const CONFLICT = () => apiError('FLOW_REVISION_CONFLICT');
const NETWORK = () => new TypeError('Failed to fetch');
const serverFlow = (revision, name) => ({ flow: { draft_revision: revision, draft: { schema: 1, name, nodes: [], edges: [] } } });
await guardAsync('c1d03 review emergency copy revision', async () => {
    // An edit during save #1, #1 succeeds, #2 fails: the copy must build on revision 2.
    const h = saverHarness(null);
    h.change('A');
    h.saver.save();
    h.change('B');
    h.saves[0].df.resolve({ draft_revision: 2, issues: [] });
    await settle();
    h.saves[1].df.reject(NETWORK());
    await settle();
    const c = h.copy();
    eq('c1d03 a copy written while a save raced an edit is offered again', [h.saver.state, h.saves[1].rev, c && c.revision, c && c.doc.name, !!h.ED.saver.emergencyCopy('f1', 2)], ['offline', 2, 2, 'B', true]);
    // Conflict, keep, the overwrite fails: the copy must build on the server's revision.
    const k = saverHarness(null);
    k.change('mine');
    k.saver.save();
    k.saves[0].df.reject(CONFLICT());
    await settle();
    k.conflicts[0].resolve('keep');
    await settle();
    k.gets[0].resolve(serverFlow(7, 'theirs'));
    await settle();
    k.saves[1].df.reject(NETWORK());
    await settle();
    const kc = k.copy();
    eq('c1d03 a kept draft whose overwrite fails is offered against the server revision', [k.saves[1].rev, kc && kc.revision, kc && kc.doc.name, !!k.ED.saver.emergencyCopy('f1', 7)], [7, 7, 'mine', true]);
});
await guardAsync('c1d03 review invalid documents', async () => {
    const invalid = Object.assign(new Error('bad'), { body: { error: 'bad', code: 'FLOW_INVALID', issues: [{ code: 'X' }] } });
    let reported = null;
    const h = saverHarness([NETWORK(), invalid, { draft_revision: 2, issues: [] }], { onInvalid: issues => { reported = issues; } });
    h.change('bad');
    await h.saver.save();
    await h.fire(5000);
    eq('c1d03 FLOW_INVALID keeps the draft unsaved, clears the error and reports the issues', [h.saver.state, h.saver.isDirty(), h.saver.error, h.timers.size, reported && reported[0].code], ['invalid', true, null, 0, 'X']);
    await h.saver.save();
    const flushed = await within(h.saver.flush());
    eq('c1d03 the refused document is not sent again and flush() reports it unsaved', [h.saver.state, h.api.calls, flushed.value, !!h.copy()], ['invalid', 2, false, true]);
    h.change('fixed');
    await h.fire(1000);
    eq('c1d03 a changed document is sent again', [h.saver.state, h.api.calls, h.saves[2].doc.name, h.copy()], ['saved', 3, 'fixed', null]);
});
await guardAsync('c1d03 review conflict paths', async () => {
    const r = saverHarness(null);
    r.change('mine');
    r.saver.save();
    r.saves[0].df.reject(CONFLICT());
    await settle();
    eq('c1d03 a conflict asks the user once', [r.saver.state, r.conflicts.length, r.gets.length], ['conflict', 1, 0]);
    r.conflicts[0].resolve('reload');
    await settle();
    r.gets[0].resolve(serverFlow(9, 'server'));
    await settle();
    eq('c1d03 reload takes the server draft and drops the copy', [r.saver.state, r.saver.revision, r.model.toJSON().name, r.copy(), r.saves.length, r.saver.isDirty()], ['saved', 9, 'server', null, 1, false]);
    const k = saverHarness(null);
    k.change('mine');
    k.saver.save();
    k.saves[0].df.reject(CONFLICT());
    await settle();
    k.conflicts[0].resolve('keep');
    await settle();
    k.gets[0].resolve(serverFlow(7, 'theirs'));
    await settle();
    k.saves[1].df.resolve({ draft_revision: 8, issues: [] });
    await settle();
    eq('c1d03 keep overwrites the server draft with the local one', [k.saves[1].rev, k.saves[1].doc.name, k.saver.state, k.saver.revision, k.model.toJSON().name, k.copy()], [7, 'mine', 'saved', 8, 'mine', null]);
});
await guardAsync('c1d03 review conflict fetch failure', async () => {
    // The fetch after the answer fails: offline with the error; only the fetch is retried, with the shared backoff.
    const g = saverHarness(null);
    g.change('mine');
    g.saver.save();
    g.saves[0].df.reject(CONFLICT());
    await settle();
    g.conflicts[0].resolve('keep');
    await settle();
    g.gets[0].reject(NETWORK());
    await settle();
    eq('c1d03 a failed conflict fetch goes offline with its error and a retry', [g.saver.state, g.saver.error && g.saver.error.message, g.delays(), g.saves.length], ['offline', 'Failed to fetch', [5000], 1]);
    await g.fire(5000);
    g.gets[1] && g.gets[1].reject(NETWORK());
    await settle();
    eq('c1d03 the retry fetches again without asking or saving, with the next backoff step', [g.gets.length, g.conflicts.length, g.saves.length, g.delays()], [2, 1, 1, [10000]]);
    await g.fire(10000);
    g.gets[2] && g.gets[2].resolve(serverFlow(5, 'theirs'));
    await settle();
    eq('c1d03 the remembered keep overwrites on the fetched revision', [g.conflicts.length, g.saves.length, g.saves[1] && g.saves[1].rev, g.saves[1] && g.saves[1].doc.name, g.saver.error], [1, 2, 5, 'mine', null]);
});
await guardAsync('c1d03 review conflict fetch failure with reload', async () => {
    const rr = saverHarness(null);
    rr.change('mine');
    rr.saver.save();
    rr.saves[0].df.reject(CONFLICT());
    await settle();
    rr.conflicts[0].resolve('reload');
    await settle();
    rr.gets[0].reject(NETWORK());
    await settle();
    const flushing = rr.saver.flush();
    await settle();
    rr.gets[1] && rr.gets[1].resolve(serverFlow(4, 'server'));
    const flushed = await within(flushing);
    eq('c1d03 the remembered reload applies after a failed fetch', [flushed.value, rr.conflicts.length, rr.saves.length, rr.model.toJSON().name, rr.saver.revision], [true, 1, 1, 'server', 4]);
});
await guardAsync('c1d03 review edit cancels a remembered reload', async () => {
    // Reload is chosen, the fetch fails, the user edits, then the retry: the edit survives and the user is asked again.
    const h = saverHarness(null);
    h.change('mine');
    h.saver.save();
    h.saves[0].df.reject(CONFLICT());
    await settle();
    h.conflicts[0].resolve('reload');
    await settle();
    h.gets[0].reject(NETWORK());
    await settle();
    h.change('later');
    await h.fire(5000);
    eq('c1d03 the retry after an edit saves the edit on the old revision instead of reloading', [h.gets.length, h.saves.length, h.saves[1] && h.saves[1].rev, h.saves[1] && h.saves[1].doc.name, h.model.toJSON().name, h.copy() && h.copy().doc.name], [1, 2, 1, 'later', 'later', 'later']);
    h.saves[1] && h.saves[1].df.reject(CONFLICT());
    await settle();
    eq('c1d03 the 409 asks the user again', [h.conflicts.length, h.saver.state], [2, 'conflict']);
    h.conflicts[1] && h.conflicts[1].resolve('keep');
    await settle();
    h.gets[1] && h.gets[1].resolve(serverFlow(5, 'theirs'));
    await settle();
    eq('c1d03 keeping then overwrites with the edit', [h.saves.length, h.saves[2] && h.saves[2].rev, h.saves[2] && h.saves[2].doc.name], [3, 5, 'later']);
    // The same edit while the first fetch is still open: the fetched draft must not replace it.
    const f = saverHarness(null);
    f.change('mine');
    f.saver.save();
    f.saves[0].df.reject(CONFLICT());
    await settle();
    f.conflicts[0].resolve('reload');
    await settle();
    f.change('later');
    f.gets[0].resolve(serverFlow(5, 'server'));
    await settle();
    eq('c1d03 an edit during the reload fetch survives and is saved on the old revision', [f.model.toJSON().name, f.saves.length, f.saves[1] && f.saves[1].rev, f.saves[1] && f.saves[1].doc.name, !!f.copy()], ['later', 2, 1, 'later', true]);
});
await guardAsync('c1d03 review conflict after dispose and failing dialog', async () => {
    // A conflict on a disposed saver opens no dialog, and its save settles.
    const d = saverHarness(null);
    d.change('closing');
    const closing = d.saver.save();
    d.saver.dispose();
    d.saves[0].df.reject(CONFLICT());
    const settled = await within(closing);
    eq('c1d03 a conflict after dispose opens no dialog and the save settles', [d.conflicts.length, d.gets.length, settled.done], [0, 0, true]);
    const late = saverHarness(null);
    late.change('mine');
    const answered = late.saver.save();
    late.saves[0].df.reject(CONFLICT());
    await settle();
    late.saver.dispose();
    late.conflicts[0].resolve('reload');
    const lateSettled = await within(answered);
    eq('c1d03 a dialog answered after dispose loads nothing', [late.gets.length, late.model.toJSON().name, lateSettled.done], [0, 'mine', true]);
    // A failing conflict dialog: offline with its error and a retry; the retry asks again.
    let asked = 0;
    const cr = saverHarness([CONFLICT()], { onConflict: () => { asked++; return Promise.reject(new Error('dialog failed')); } });
    cr.change('x');
    cr.saver.save();
    await settle();
    eq('c1d03 a failing conflict dialog goes offline with its error and a retry', [cr.saver.state, cr.saver.error && cr.saver.error.message, cr.delays(), cr.gets.length], ['offline', 'dialog failed', [5000], 0]);
    await cr.fire(5000);
    eq('c1d03 the retry saves and asks again', [cr.api.calls, asked, cr.delays()], [2, 2, [10000]]);
});
await guardAsync('c1d03 review callbacks and answers', async () => {
    const h = saverHarness([{ draft_revision: 2, issues: [] }], { onSaved: () => { throw new Error('render failed'); }, onState: s => { if (s === 'saved') throw new Error('state render failed'); } });
    h.change('x');
    await within(h.saver.save());
    eq('c1d03 throwing callbacks do not turn a success into offline', [h.saver.state, h.timers.size, h.copy(), h.saver.revision, h.logged.length], ['saved', 0, null, 2, 2]);
    const e = saverHarness([{}]);
    e.change('x');
    await within(e.saver.save());
    eq('c1d03 a 200 without draft_revision is transient and keeps the copy', [e.saver.state, e.saver.revision, !!e.copy(), e.delays()], ['offline', 1, true, [5000]]);
    const statusError = status => Object.assign(new Error('HTTP ' + status), { body: {}, status });
    const states = [401, 403, 404, 413, 408, 429, 500, 502].map(status => {
        const s = saverHarness([statusError(status)]);
        s.change('x');
        s.saver.save();
        return s;
    });
    await settle();
    eq('c1d03 an answer without a code fails on 4xx except 408 and 429', states.map(s => s.saver.state), ['failed', 'failed', 'failed', 'failed', 'offline', 'offline', 'offline', 'offline']);
});
await guardAsync('c1d03 review backoff and errors', async () => {
    const LOCKED = () => apiError('FLOW_LOCKED');
    const invalid = () => Object.assign(new Error('bad'), { body: { error: 'bad', code: 'FLOW_INVALID', issues: [] } });
    const p = saverHarness([NETWORK(), NETWORK(), LOCKED(), NETWORK()]);
    p.change('x');
    await within(p.saver.save());
    await p.fire(5000);
    await p.fire(10000);
    p.change('y');
    await within(p.saver.save());
    eq('c1d03 a permanent answer resets the backoff', p.delays(), [5000]);
    const v = saverHarness([NETWORK(), NETWORK(), invalid(), NETWORK()]);
    v.change('x');
    await within(v.saver.save());
    await v.fire(5000);
    await v.fire(10000);
    v.change('y');
    await within(v.saver.save());
    eq('c1d03 an invalid answer resets the backoff', v.delays(), [5000]);
    const c = saverHarness([NETWORK(), CONFLICT()]);
    c.change('x');
    await within(c.saver.save());
    await c.fire(5000);
    eq('c1d03 entering a conflict clears the error', [c.saver.state, c.saver.error], ['conflict', null]);
});
await guardAsync('c1d03 review save during a failing request', async () => {
    // A save() during a request that then fails waits for the retry timer instead of sending again at once.
    const q = saverHarness(null);
    q.change('a');
    q.saver.save();
    q.change('b');
    q.saver.save();
    q.saves[0].df.reject(NETWORK());
    await settle();
    eq('c1d03 a save() during a failing request sends no second request at once', [q.saves.length, q.saver.state, q.delays()], [1, 'offline', [5000]]);
    await q.fire(5000);
    eq('c1d03 the retry sends the latest document', [q.saves.length, q.saves[1] && q.saves[1].doc.name], [2, 'b']);
});
await guardAsync('c1d03 review flush', async () => {
    const f = saverHarness(null);
    f.change('a');
    f.saver.save();
    f.change('b');
    f.saver.save();
    const flushing = f.saver.flush();
    f.saves[0].df.resolve({ draft_revision: 2, issues: [] });
    await settle();
    f.saves[1] && f.saves[1].df.resolve({ draft_revision: 3, issues: [] });
    const flushed = await within(flushing);
    eq('c1d03 flush() waits for the queued save and reports saved', [flushed.value, f.saves.length, f.saves[1] && f.saves[1].rev, f.saver.state, f.copy()], [true, 2, 2, 'saved', null]);
    const o = saverHarness([NETWORK()]);
    o.change('a');
    const offline = await within(o.saver.flush());
    eq('c1d03 flush() reports an offline draft as unsaved', [offline.value, o.saver.state, !!o.copy()], [false, 'offline', true]);
});
await guardAsync('c1d03 review sweep', async () => {
    const day = 24 * 60 * 60 * 1000;
    const store = new Map([
        ['aurago.easydrag.draft.old', JSON.stringify({ revision: 1, at: Date.now() - 31 * day, doc: {} })],
        ['aurago.easydrag.draft.fresh', JSON.stringify({ revision: 1, at: Date.now() - 29 * day, doc: {} })],
        ['aurago.easydrag.draft.broken', '{not json'],
        ['aurago.easydrag.view.old', JSON.stringify({ x: 0, at: 0 })]
    ]);
    saverHarness([{}], { store });
    eq('c1d03 creating a saver sweeps emergency copies older than 30 days', Array.from(store.keys()).sort(), ['aurago.easydrag.draft.fresh', 'aurago.easydrag.view.old']);
});
// miniDom is just enough DOM for core.modal and the canvas modules: it parses the markup they build,
// matches the simple selectors they use (tag, .class, [attr], [attr="v"], :not(), descendant, lists),
// tracks focus and dispatches bubbling events through el.fire(type, init).
function miniDom() {
    const VOID = new Set(['input', 'br', 'img', 'hr']);
    let active = null;
    function matchCompound(node, compound) {
        const not = /:not\((.*)\)$/.exec(compound);
        if (not) {
            if (matchCompound(node, not[1])) return false;
            compound = compound.slice(0, not.index);
        }
        const re = /([a-z][a-z0-9-]*)|\.([\w-]+)|\[([\w-]+)(?:="([^"]*)")?\]/y;
        for (let pos = 0; pos < compound.length; pos = re.lastIndex) {
            re.lastIndex = pos;
            const m = re.exec(compound);
            if (!m) throw new Error('unsupported selector ' + compound);
            if (m[1] && node.localName !== m[1]) return false;
            if (m[2] && !node.className.split(/\s+/).includes(m[2])) return false;
            if (m[3] && (!node.attrs.has(m[3]) || (m[4] !== undefined && node.attrs.get(m[3]) !== m[4]))) return false;
        }
        return true;
    }
    function matches(node, list) {
        return list.split(',').some(sel => {
            const parts = sel.trim().split(/\s+/);
            if (!matchCompound(node, parts[parts.length - 1])) return false;
            let i = parts.length - 2;
            for (let x = node.parentNode; x && i >= 0; x = x.parentNode) if (matchCompound(x, parts[i])) i--;
            return i < 0;
        });
    }
    class El {
        constructor(tag, attrs) {
            this.localName = tag;
            this.attrs = new Map(Object.entries(attrs || {}));
            this.children = [];
            this.parentNode = null;
            this.listeners = {};
            this.disabled = this.attrs.has('disabled');
            this.offsetParent = {};
            this.style = {};
            const self = this;
            this.classList = {
                add(...names) { self.attrs.set('class', Array.from(new Set(self.className.split(/\s+/).filter(Boolean).concat(names))).join(' ')); },
                remove(...names) { self.attrs.set('class', self.className.split(/\s+/).filter(c => c && !names.includes(c)).join(' ')); },
                toggle(name, force) { const on = force === undefined ? !this.contains(name) : !!force; if (on) this.add(name); else this.remove(name); return on; },
                contains: name => self.className.split(/\s+/).includes(name)
            };
            const dataAttr = key => 'data-' + String(key).replace(/[A-Z]/g, c => '-' + c.toLowerCase());
            this.dataset = new Proxy({}, {
                get: (_, key) => { const v = self.attrs.get(dataAttr(key)); return v === undefined ? undefined : v; },
                set: (_, key, value) => { self.attrs.set(dataAttr(key), String(value)); return true; }
            });
        }
        get className() { return this.attrs.get('class') || ''; }
        set className(value) { this.attrs.set('class', String(value)); }
        get firstChild() { return this.children[0] || null; }
        get lastChild() { return this.children[this.children.length - 1] || null; }
        getBoundingClientRect() { return { left: 0, top: 0, right: 0, bottom: 0, width: 0, height: 0 }; }
        get id() { return this.attrs.get('id') || ''; }
        set id(value) { this.attrs.set('id', String(value)); }
        get content() { return this; }
        get firstElementChild() { return this.children[0] || null; }
        set innerHTML(html) {
            this.children.forEach(c => { c.parentNode = null; });
            this.children = [];
            const re = /<\/([a-zA-Z][\w-]*)\s*>|<([a-zA-Z][\w-]*)((?:\s+[\w:-]+(?:="[^"]*")?)*)\s*(\/?)>|[^<]+/g;
            let cur = this;
            let m;
            while ((m = re.exec(String(html)))) {
                if (m[1]) { if (cur !== this) cur = cur.parentNode; continue; }
                if (!m[2]) continue;
                const attrs = {};
                m[3].replace(/([\w:-]+)(?:="([^"]*)")?/g, (_, k, v) => { attrs[k] = v === undefined ? '' : v; return ''; });
                const node = cur.appendChild(new El(m[2].toLowerCase(), attrs));
                if (!m[4] && !VOID.has(node.localName)) cur = node;
            }
        }
        getAttribute(name) { return this.attrs.has(name) ? this.attrs.get(name) : null; }
        setAttribute(name, value) { this.attrs.set(name, String(value)); }
        appendChild(child) { child.remove(); child.parentNode = this; this.children.push(child); return child; }
        remove() { if (!this.parentNode) return; const list = this.parentNode.children; list.splice(list.indexOf(this), 1); this.parentNode = null; }
        contains(node) { for (let x = node; x; x = x.parentNode) if (x === this) return true; return false; }
        closest(sel) { for (let x = this; x; x = x.parentNode) if (matches(x, sel)) return x; return null; }
        querySelectorAll(sel) { const out = []; const walk = n => n.children.forEach(c => { if (matches(c, sel)) out.push(c); walk(c); }); walk(this); return out; }
        querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
        addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
        removeEventListener(type, fn) { const list = this.listeners[type] || []; const i = list.indexOf(fn); if (i >= 0) list.splice(i, 1); }
        setPointerCapture() {}
        matches(sel) { return sel === ':hover' ? false : matches(this, sel); }
        focus() { active = this; }
        // fire dispatches to a copy of each listener list, so a listener may remove itself.
        fire(type, init) {
            const event = Object.assign({ type, target: this, defaultPrevented: false, stopped: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; } }, init);
            for (let x = this; x && !event.stopped; x = x.parentNode) (x.listeners[type] || []).slice().forEach(fn => fn(event));
            return event;
        }
    }
    return { El, document: { get activeElement() { return active; }, createElement: tag => new El(String(tag).toLowerCase(), {}), createElementNS: (ns, tag) => new El(String(tag), {}) } };
}
// modalHarness loads core with miniDom, a frozen Date.now (dialogs open "in the same millisecond") and a
// host that holds a focused opener button.
function modalHarness() {
    const dom = miniDom();
    const logged = [];
    const FrozenDate = class extends Date {};
    FrozenDate.now = () => 1767225600000;
    const box = vm.createContext({
        window: {}, navigator: { platform: 'Linux' }, crypto: webcrypto, document: dom.document, Date: FrozenDate,
        console: { log() {}, warn() {}, error: (...args) => { logged.push(args.map(String).join(' ')); } },
        requestAnimationFrame: fn => { fn(); return 1; },
        setTimeout: () => 0, clearTimeout() {}
    });
    vm.runInContext(fs.readFileSync(path.join(apps, 'easydrag-core.js'), 'utf8'), box, { filename: 'easydrag-core.js' });
    const host = new dom.El('div', { class: 'ed-editor' });
    const opener = host.appendChild(new dom.El('button', {}));
    opener.focus();
    return { core: box.window.EasyDrag.core, host, opener, document: dom.document, logged };
}
// outcome is what dialog.done resolved to once the queued work ran, or "open".
async function outcome(dialog) {
    let result = 'open';
    dialog.done.then(value => { result = value; });
    await settle();
    return result;
}
await guardAsync('c1d03 review modal dismissal', async () => {
    const h = modalHarness();
    const actions = [{ id: 'reload', label: 'Reload' }, { id: 'keep', label: 'Keep', primary: true }];
    const locked = h.core.modal(h.host, { title: 'T', closeLabel: 'Close', dismissible: false, actions });
    const overlay = locked.el.parentNode;
    const hasClose = !!locked.el.querySelector('[data-ed-action="close"]');
    locked.el.fire('keydown', { key: 'Escape' });
    overlay.fire('click');
    eq('c1d03 a non-dismissible dialog has no close button and ignores Escape and the backdrop', [hasClose, await outcome(locked)], [false, 'open']);
    overlay.querySelector('[data-ed-action="reload"]').fire('click');
    eq('c1d03 an action still closes a non-dismissible dialog', await outcome(locked), 'reload');
    const viaEscape = h.core.modal(h.host, { title: 'T', closeLabel: 'Close', cancel: 'keep', actions });
    viaEscape.el.fire('keydown', { key: 'Escape' });
    const viaClose = h.core.modal(h.host, { title: 'T', closeLabel: 'Close', cancel: 'keep', actions });
    viaClose.el.querySelector('[data-ed-action="close"]').fire('click');
    const plain = h.core.modal(h.host, { title: 'T', closeLabel: 'Close', actions });
    plain.el.fire('keydown', { key: 'Escape' });
    eq('c1d03 cancel names what Escape and the close button resolve to', [await outcome(viaEscape), await outcome(viaClose), await outcome(plain)], ['keep', 'keep', null]);
});
await guardAsync('c1d03 review modal focus, ids and failing actions', async () => {
    const h = modalHarness();
    const actions = [{ id: 'discard', label: 'Discard', danger: true }, { id: 'restore', label: 'Restore', primary: true }];
    const locked = h.core.modal(h.host, { title: 'T', closeLabel: 'Close', dismissible: false, actions });
    const overlay = locked.el.parentNode;
    h.opener.focus(); // the browser moved focus out of the dialog
    const press = overlay.fire('mousedown');
    overlay.fire('click');
    eq('c1d03 a backdrop press keeps focus in the dialog', [press.defaultPrevented, locked.el.contains(h.document.activeElement)], [true, true]);
    const dialogs = [h.core.modal(h.host, { title: 'A', actions }), h.core.modal(h.host, { title: 'B', actions })];
    const ids = dialogs.map(d => d.el.getAttribute('aria-labelledby'));
    eq('c1d03 dialogs opened in the same millisecond get distinct title ids', [ids[0] !== ids[1], dialogs.map(d => d.el.querySelector('.ed-modal-title').id)], [true, ids]);
    const failing = h.core.modal(h.host, { title: 'T', actions, onAction: () => Promise.reject(new Error('restore failed')) });
    failing.el.querySelector('[data-ed-action="restore"]').fire('click');
    eq('c1d03 a rejected action keeps the dialog open and usable', [await outcome(failing), failing.el.querySelector('[data-ed-action="restore"]').disabled, h.logged.some(line => line.includes('restore failed'))], ['open', false, true]);
});
await guardAsync('c1d03 review core helpers', async () => {
    const core = modalHarness().core;
    eq('c1d03 duration rounds to whole seconds before splitting', [119999, 59999, 61000, 12345, 1500].map(core.fmt.duration), ['2:00 min', '1:00 min', '1:01 min', '12 s', '1.5 s']);
    eq('c1d03 icon reads own entries only and escapes the class', [core.icon('constructor').includes(core.ICONS.tool), core.icon('x', 'a"><b').includes('class="ed-icon a&quot;&gt;&lt;b"')], [true, true]);
    const em = core.emitter();
    const seen = [];
    let offSecond = null;
    em.on('x', () => { seen.push(1); offSecond(); });
    offSecond = em.on('x', () => seen.push(2));
    em.emit('x');
    eq('c1d03 a handler removed during emit does not run', seen, [1]);
    const sent = [];
    const client = core.createApi(url => { sent.push(url); return Promise.resolve({}); });
    const results = await Promise.all([client.get('..'), client.deleteSecret('.'), client.testData('f1', '..'), client.save('', {}, 1), client.get('a.b')]
        .map(p => p.then(() => 'sent', err => core.errorCode(err))));
    let urlThrew = false;
    try { client.eventsUrl('..'); } catch (err) { urlThrew = true; }
    eq('c1d03 createApi rejects empty, . and .. segments before sending', [results, sent, urlThrew], [['FLOW_BAD_REQUEST', 'FLOW_BAD_REQUEST', 'FLOW_BAD_REQUEST', 'FLOW_BAD_REQUEST', 'sent'], ['/api/desktop/flows/a.b'], true]);
});
await guardAsync('c1d03 shell api errors', async () => {
    // api() of desktop-foundation.js is the ctx.api that createApi wraps; it runs here with a stub fetch.
    const foundation = fs.readFileSync(path.join(here, '..', 'ui', 'js', 'desktop', 'core', 'desktop-foundation.js'), 'utf8').replace(/\r\n/g, '\n');
    const start = foundation.indexOf('    async function api(url, options) {');
    const end = foundation.indexOf('\n    }\n', start);
    if (start < 0 || end < 0) throw new Error('api() not found in desktop-foundation.js');
    let response = null;
    const shellApi = vm.runInContext('(' + foundation.slice(start, end + 6).trim() + ')', vm.createContext({ fetch: async () => response }));
    const respond = (status, text, headers) => {
        const h = Object.assign({}, headers);
        response = { ok: status >= 200 && status < 300, status, headers: { get: name => (name.toLowerCase() in h ? h[name.toLowerCase()] : null) }, json: async () => JSON.parse(text) };
    };
    const fail = async () => { try { await shellApi('/api/desktop/flows/f1', {}); return null; } catch (err) { return err; } };
    const JSON_TYPE = { 'content-type': 'application/json; charset=utf-8' };
    for (const [status, code] of [[413, 'FLOW_TOO_LARGE'], [429, 'FLOW_RATE_LIMITED'], [503, 'FLOWS_DISABLED']]) {
        respond(status, JSON.stringify({ error: 'text ' + code, code }), JSON_TYPE);
        const err = await fail();
        eq('c1d03 api keeps the JSON body and status of a ' + status, [err && err.message, err && err.body && err.body.code, err && err.status], ['text ' + code, code, status]);
    }
    const retryAfter = [];
    for (const value of ['60abc', '1e3', '1.5', '-5', ' 7 ', '0']) {
        respond(429, '{"error":"slow","code":"FLOW_RATE_LIMITED"}', Object.assign({ 'retry-after': value }, JSON_TYPE));
        retryAfter.push((await fail()).retryAfter);
    }
    eq('c1d03 api reads Retry-After only as whole seconds', retryAfter, [undefined, undefined, undefined, undefined, 7, 0]);
    respond(429, '{"error":"slow","code":"FLOW_RATE_LIMITED"}', Object.assign({ 'retry-after': '60' }, JSON_TYPE));
    const limited = await fail();
    respond(503, '{"error":"off","code":"FLOWS_DISABLED"}', Object.assign({ 'retry-after': '60' }, JSON_TYPE));
    const unavailable = await fail();
    respond(429, '{"error":"slow","code":"FLOW_RATE_LIMITED"}', JSON_TYPE);
    const noHeader = await fail();
    eq('c1d03 api exposes Retry-After seconds on 429 only', [limited.retryAfter, unavailable.retryAfter, noHeader.retryAfter], [60, undefined, undefined]);
    respond(502, '<html>Bad Gateway</html>', { 'content-type': 'text/html' });
    const proxy = await fail();
    const core = saverHarness([{}]).core;
    eq('c1d03 a non-JSON error body still gives a usable error', [proxy && proxy.message, JSON.stringify(proxy && proxy.body), core.errorText(t, proxy), core.errorText(t, limited)], ['HTTP 502', '{}', 'error_network', 'error_flow_rate_limited']);
});

// ── c1d04: canvas, wires and interact ──
// The checks live in test-easydrag-extra.mjs; they share this file's helpers and failure count.
await (await import('./test-easydrag-extra.mjs')).run({ apps, types, t, miniDom, check, eq, guardAsync, settle });

// ── c1d06: detail view, test runs and publishing (test-easydrag-extra2.mjs) ──
await (await import('./test-easydrag-extra2.mjs')).run({ apps, types, t, miniDom, check, eq, guardAsync, settle });

// ── c1d07: start page, dialogs, editor screen and window shell (test-easydrag-extra3.mjs) ──
const editorHarness = await (await import('./test-easydrag-extra3.mjs')).run({ apps, types, t, miniDom, check, eq, guardAsync, settle });

// ── c1d14: stored views and the readable fit (test-easydrag-extra4.mjs) ──
await (await import('./test-easydrag-extra4.mjs')).run({ apps, types, t, miniDom, check, eq, guardAsync, settle });

// ── FF2: editor sync, state, performance and accessibility (test-easydrag-extra5.mjs, on the c1d07 sandbox) ──
await (await import('./test-easydrag-extra5.mjs')).run({ apps, types, t, miniDom, check, eq, guardAsync, settle, sandbox: editorHarness.sandbox, openEditor: editorHarness.openEditor });

// ── start page preview and shortcut table ──
const H = ED.home;
const escHTML = v => String(v).replace(/[&<>"']/g, c => '&#' + c.charCodeAt(0) + ';');
const mini = H.previewSVG([{ x: 0, y: 0, category: 'trigger' }, { x: 320, y: 40, category: 'tool:files' }], escHTML);
eq('preview draws every node', (mini.match(/<rect /g) || []).length, 2);
eq('preview draws one wire', (mini.match(/<path /g) || []).length, 1);
check('preview maps tool categories', mini.includes('data-cat="tool"'));
check('empty preview placeholder', H.previewSVG([], escHTML).includes('ed-mini-empty'));
const shortcutKeys = ED.dialogs.SHORTCUTS.flatMap(([group, rows]) => [group].concat(rows.map(r => r[1])));
check('shortcut labels use full keys', shortcutKeys.every(k => /^easydrag.ui.[a-z0-9_]+$/.test(k)), shortcutKeys.join(' '));

if (failures) {
    console.log(failures + ' failure(s)');
    process.exit(1);
}
console.log('all EasyDrag checks passed');
