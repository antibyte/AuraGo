const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

// package.json has "type": "module": run the UMD source in a vm with a fake module, like notes-index.test.cjs.
const mod = { exports: {} };
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'lsp-protocol.js'), 'utf8'), { module: mod, window: {} });
const P = mod.exports;
// Objects from the vm realm have another Object.prototype, so deep comparisons go through JSON.
const json = value => JSON.parse(JSON.stringify(value));

// A stand-in for CodeMirror's Text: lines, length, line(n) and lineAt(offset) with number, from, to, text.
function fakeText(content) {
    const lines = content.split('\n');
    const starts = [];
    let pos = 0;
    for (const line of lines) {
        starts.push(pos);
        pos += line.length + 1;
    }
    return {
        length: content.length,
        lines: lines.length,
        line(n) {
            const text = lines[n - 1];
            return { number: n, from: starts[n - 1], to: starts[n - 1] + text.length, text };
        },
        lineAt(offset) {
            let n = lines.length;
            while (n > 1 && starts[n - 1] > offset) n -= 1;
            return this.line(n);
        }
    };
}

test('languageForPath maps the four servers and nothing else', () => {
    assert.equal(P.languageForPath('/workspace/cmd/main.go'), 'go');
    for (const ext of ['ts', 'tsx', 'js', 'jsx', 'mjs', 'cjs']) assert.equal(P.languageForPath('/workspace/web/app.' + ext), 'typescript');
    assert.equal(P.languageForPath('/workspace/tool.PY'), 'python');
    for (const ext of ['c', 'cc', 'cpp', 'cxx', 'h', 'hpp']) assert.equal(P.languageForPath('/workspace/src/x.' + ext), 'cpp');
    for (const name of ['/workspace/README.md', '/workspace/Makefile', '/workspace/.bashrc', '/workspace/lib.rs', '', null]) {
        assert.equal(P.languageForPath(name), null, String(name));
    }
    assert.deepEqual(json(P.LANGUAGES), ['go', 'typescript', 'python', 'cpp']);
    assert.equal(P.ROOT_URI, 'file:///workspace');
});

test('languageIdForPath and sourceForLanguage', () => {
    assert.equal(P.languageIdForPath('/workspace/a.go'), 'go');
    assert.equal(P.languageIdForPath('/workspace/a.ts'), 'typescript');
    assert.equal(P.languageIdForPath('/workspace/a.tsx'), 'typescriptreact');
    assert.equal(P.languageIdForPath('/workspace/a.jsx'), 'javascriptreact');
    assert.equal(P.languageIdForPath('/workspace/a.mjs'), 'javascript');
    assert.equal(P.languageIdForPath('/workspace/a.py'), 'python');
    assert.equal(P.languageIdForPath('/workspace/a.h'), 'c');
    assert.equal(P.languageIdForPath('/workspace/a.hpp'), 'cpp');
    assert.equal(P.languageIdForPath('/workspace/a.txt'), null);
    assert.deepEqual(['go', 'typescript', 'python', 'cpp'].map(P.sourceForLanguage), ['gopls', 'typescript', 'pyright', 'clangd']);
    assert.deepEqual(json(P.SOURCES), { go: 'gopls', typescript: 'typescript', python: 'pyright', cpp: 'clangd' });
    assert.equal(P.sourceForLanguage('rust'), '');
});

test('uriForPath percent-encodes segments and stays inside /workspace', () => {
    assert.equal(P.uriForPath('/workspace'), 'file:///workspace');
    assert.equal(P.uriForPath('/workspace/cmd/main.go'), 'file:///workspace/cmd/main.go');
    assert.equal(P.uriForPath("/workspace/my dir/it's #1 (copy).go"), 'file:///workspace/my%20dir/it%27s%20%231%20%28copy%29.go');
    assert.equal(P.uriForPath('/workspace/' + String.fromCodePoint(0xe4) + '.py'), 'file:///workspace/%C3%A4.py');
    assert.equal(P.uriForPath('//workspace///a.go'), 'file:///workspace/a.go');
    for (const bad of ['/etc/passwd', '/workspacex/a.go', '/workspace/../etc', '/workspace/./a.go', 'workspace/a.go', '/workspace/a\\b.go', '', null]) {
        assert.equal(P.uriForPath(bad), null, String(bad));
    }
});

test('pathForUri decodes workspace URIs and rejects everything else', () => {
    assert.equal(P.pathForUri('file:///workspace/my%20dir/it%27s%20%231.go'), "/workspace/my dir/it's #1.go");
    assert.equal(P.pathForUri('file://localhost/workspace/a.go'), '/workspace/a.go');
    assert.equal(P.pathForUri('FILE:///workspace/a.go'), '/workspace/a.go');
    assert.equal(P.pathForUri(P.uriForPath('/workspace/x y/z.ts')), '/workspace/x y/z.ts');
    for (const bad of [
        'file:///etc/passwd', 'file:///workspace/../etc/passwd', 'file:///workspace/a%2Fb.go', 'file:///workspace/%E0%A4%A.go',
        'https://example.com/workspace/a.go', 'file:///workspace/a.go?x=1', 'file:///C:/workspace/a.go', 'untitled:Untitled-1', '', null
    ]) {
        assert.equal(P.pathForUri(bad), null, String(bad));
    }
});

test('diagnosticsToProblems maps severity, 1-based positions, the source and keeps the raw diagnostic', () => {
    const problems = P.diagnosticsToProblems('/workspace/main.go', [
        { range: { start: { line: 8, character: 1 }, end: { line: 8, character: 5 } }, severity: 1, code: 'UndeclaredName', message: 'undefined: fmtx', source: 'compiler', data: { fix: 1 } },
        { range: { start: { line: 0, character: 0 }, end: { line: 0, character: 7 } }, severity: 2, message: '  unused import  ' },
        { range: { start: { line: 3, character: 2 }, end: { line: 4, character: 0 } }, severity: 3, code: 2304, message: 'info' },
        { range: { start: { line: 5, character: 4 }, end: { line: 5, character: 4 } }, severity: 4, message: 'hint' },
        { range: { start: { line: 6, character: 0 }, end: { line: 6, character: 1 } }, message: 'no severity' },
        { range: { start: { line: 7, character: 0 } }, severity: 1, message: '' },
        { message: 'no range' },
        null
    ], 'gopls');
    const range = (a, b, c, d) => ({ start: { line: a, character: b }, end: { line: c, character: d } });
    assert.deepEqual(json(problems), [
        { file: '/workspace/main.go', line: 9, column: 2, endLine: 9, endColumn: 6, severity: 'error', message: 'undefined: fmtx', source: 'gopls',
            lsp: { range: range(8, 1, 8, 5), message: 'undefined: fmtx', severity: 1, code: 'UndeclaredName', source: 'compiler', data: { fix: 1 } } },
        { file: '/workspace/main.go', line: 1, column: 1, endLine: 1, endColumn: 8, severity: 'warning', message: 'unused import', source: 'gopls',
            lsp: { range: range(0, 0, 0, 7), message: '  unused import  ', severity: 2 } },
        { file: '/workspace/main.go', line: 4, column: 3, endLine: 5, endColumn: 1, severity: 'info', message: 'info', source: 'gopls',
            lsp: { range: range(3, 2, 4, 0), message: 'info', severity: 3, code: 2304 } },
        { file: '/workspace/main.go', line: 6, column: 5, endLine: 6, endColumn: 5, severity: 'info', message: 'hint', source: 'gopls',
            lsp: { range: range(5, 4, 5, 4), message: 'hint', severity: 4 } },
        { file: '/workspace/main.go', line: 7, column: 1, endLine: 7, endColumn: 2, severity: 'error', message: 'no severity', source: 'gopls',
            lsp: { range: range(6, 0, 6, 1), message: 'no severity' } }
    ]);
    assert.deepEqual(json(P.diagnosticsToProblems('/workspace/a.py', 'not a list', 'pyright')), []);
});

test('positionToOffset and offsetToPosition use UTF-16 units and clamp', () => {
    // Line 1 starts with a surrogate pair: the emoji takes two UTF-16 units, as in LSP and CodeMirror.
    const doc = fakeText('ab\n' + String.fromCodePoint(0x1F600) + 'x\n\nend');
    assert.equal(doc.length, 11);
    assert.equal(P.positionToOffset(doc, { line: 1, character: 2 }), 5);
    assert.deepEqual(json(P.offsetToPosition(doc, 5)), { line: 1, character: 2 });
    assert.equal(P.positionToOffset(doc, { line: 0, character: 99 }), 2);
    assert.equal(P.positionToOffset(doc, { line: 99, character: 0 }), 11);
    assert.equal(P.positionToOffset(doc, { line: -1, character: 3 }), 0);
    assert.equal(P.positionToOffset(doc, { line: 3, character: 3 }), 11);
    assert.deepEqual(json(P.offsetToPosition(doc, 0)), { line: 0, character: 0 });
    assert.deepEqual(json(P.offsetToPosition(doc, 7)), { line: 2, character: 0 });
    assert.deepEqual(json(P.offsetToPosition(doc, 999)), { line: 3, character: 3 });
    assert.deepEqual(json(P.offsetToPosition(doc, -4)), { line: 0, character: 0 });
    for (let offset = 0; offset <= doc.length; offset += 1) {
        assert.equal(P.positionToOffset(doc, P.offsetToPosition(doc, offset)), offset, 'round trip ' + offset);
    }
});

test('binaryForLanguage names the server binary the bridge probes', () => {
    assert.equal(P.binaryForLanguage('go'), 'gopls');
    assert.equal(P.binaryForLanguage('typescript'), 'typescript-language-server');
    assert.equal(P.binaryForLanguage('python'), 'pyright-langserver');
    assert.equal(P.binaryForLanguage('cpp'), 'clangd');
    assert.equal(P.binaryForLanguage('rust'), '');
});
