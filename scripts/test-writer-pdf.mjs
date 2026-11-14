import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

const context = vm.createContext({window: {}, btoa, setTimeout});
vm.runInContext(readFileSync('ui/js/desktop/apps/writer-pdf.js', 'utf8'), context);
const P = context.window.WriterPdf;
const same = (actual, expected, message) => assert.equal(JSON.stringify(actual), JSON.stringify(expected), message);

// Faces: one key per family, weight class and slant; the first family of a font list counts.
same(P.used([['Calibri, Carlito, sans-serif', '400', 'normal'], ['"Courier New", Calibri', '700', 'italic'], ['Calibri', 'bold', 'normal'], ['calibri', '400', 'normal'], ['Calibri', '600', 'oblique']]),
    ['calibri|r|n', 'courier new|b|i', 'calibri|b|n', 'calibri|b|i']);
const bytes = text => new TextEncoder().encode(text);
const fonts = {
    sources: [
        {request: {family: 'Carlito', weight: 400, style: 'normal'}, bytes: bytes('carlito-regular')},
        {request: {family: 'Carlito', weight: 700, style: 'normal'}, bytes: bytes('carlito-bold')},
        {request: {family: 'Liberation Mono', weight: 700, style: 'italic'}, bytes: bytes('mono-bold-italic')}],
    substitutions: [
        {from: {family: 'Calibri', weight: 400, style: 'normal'}, to: {family: 'Carlito', weight: 400, style: 'normal'}},
        {from: {family: 'Calibri', weight: 700, style: 'normal'}, to: {family: 'Carlito', weight: 700, style: 'normal'}},
        {from: {family: 'Courier New', weight: 700, style: 'italic'}, to: {family: 'Liberation Mono', weight: 700, style: 'italic'}},
        {from: {family: 'Symbol', weight: 400, style: 'normal'}, to: {family: 'Missing', weight: 400, style: 'normal'}}]
};
assert.equal(P.source(fonts, 'carlito|r|n').bytes, fonts.sources[0].bytes, 'a family with its own file');
assert.equal(P.source(fonts, 'calibri|b|n').bytes, fonts.sources[1].bytes, 'a substituted family takes the file it is laid out with');
assert.equal(P.source(fonts, 'calibri|b|i'), null, 'no file for a face nobody has');
assert.equal(P.source(fonts, 'symbol|r|n'), null, 'a substitute without a file');
assert.equal(P.faces(fonts, ['calibri|r|n', 'courier new|b|i', 'system-ui|r|n']),
    '@font-face{font-family:"calibri";font-weight:400;font-style:normal;src:url(data:font/ttf;base64,' + btoa('carlito-regular') + ')}'
    + '@font-face{font-family:"courier new";font-weight:700;font-style:italic;src:url(data:font/ttf;base64,' + btoa('mono-bold-italic') + ')}');
const big = {sources: [{request: {family: 'Big', weight: 400, style: 'normal'}, bytes: new Uint8Array(70000).fill(65)}], substitutions: []};
assert.equal(P.faces(big, ['big|r|n']), '@font-face{font-family:"big";font-weight:400;font-style:normal;src:url(data:font/ttf;base64,' + btoa('A'.repeat(70000)) + ')}', 'long files survive the chunks');

// The page: no script, no link, the title escaped.
const html = P.page({title: 'Q3 <Report> & more', engine: '.docx-page{color:black}', faces: '@font-face{}', sheets: '@page{size:1px 2px}', body: '<div id="pages"></div>'});
assert.equal(html, '<!doctype html><html><head><meta charset="utf-8"><title>Q3 &lt;Report&gt; &amp; more</title><style>.docx-page{color:black}</style><style>@font-face{}@page{size:1px 2px}</style></head><body><div id="pages"></div></body></html>');
assert.ok(!/<script|<link/i.test(html));

// File names.
assert.equal(P.fileName('Documents/Reports/Q3.docx'), 'Q3.pdf');
assert.equal(P.fileName('Q3.DOCX'), 'Q3.pdf');
assert.equal(P.fileName('Documents/notes'), 'notes.pdf');
assert.equal(P.fileName(''), 'document.pdf');

// Pictures that are still loading.
assert.equal(P.pending({pages: [{fragments: [{drawings: [{resource: {kind: 'ready'}}, {resource: {kind: 'pending'}}]}]}]}), true);
assert.equal(P.pending({pages: [{fragments: [{drawings: [{resource: {kind: 'ready'}}, {resource: {kind: 'invalid'}}]}]}]}), false);
assert.equal(P.pending(vm.runInContext('new Map([["a", {resource: {kind: "pending"}}]])', context)), true, 'maps are walked');
const loop = {resource: {kind: 'ready'}};
loop.self = loop;
assert.equal(P.pending(loop), false, 'a cycle ends');
assert.equal(P.pending({bytes: new Uint8Array(1000), resource: null}), false);
let calls = 0;
await P.pictures({surface: {layout: () => ({drawing: {resource: {kind: ++calls < 3 ? 'pending' : 'ready'}}})}}, 2000);
assert.equal(calls, 3, 'pictures waits until nothing is pending');
const started = Date.now();
await P.pictures({surface: {layout: () => ({drawing: {resource: {kind: 'pending'}}})}}, 120);
assert.ok(Date.now() - started < 1000, 'pictures gives up after its limit');

console.log('Writer PDF: faces, font sources, page, file names and the wait for pictures pass');
