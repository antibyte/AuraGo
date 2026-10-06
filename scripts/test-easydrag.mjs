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

if (failures) {
    console.log(failures + ' failure(s)');
    process.exit(1);
}
console.log('all EasyDrag checks passed');
