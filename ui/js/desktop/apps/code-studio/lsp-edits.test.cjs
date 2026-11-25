// Unit tests for ui/js/desktop/apps/code-studio/lsp-edits.js (Code Studio IDE package 2C).
// Run: node --test ui/js/desktop/apps/code-studio/lsp-edits.test.cjs
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

// package.json has "type": "module": the UMD sources run in a vm with a fake module, like notes-index.test.cjs.
// lsp-edits.js maps URIs through plan 2B's CodeStudioLSPProtocol, so that module is loaded first.
function loadUmd(file, windowObject) {
    const mod = { exports: {} };
    vm.runInNewContext(fs.readFileSync(path.join(__dirname, file), 'utf8'), {
        module: mod, exports: mod.exports, window: windowObject, console
    }, { filename: file });
    return mod.exports;
}

const protocolWindow = {};
const protocolExports = loadUmd('lsp-protocol.js', protocolWindow);
const Protocol = protocolExports && typeof protocolExports.pathForUri === 'function' ? protocolExports : protocolWindow.CodeStudioLSPProtocol;
assert.equal(typeof (Protocol && Protocol.pathForUri), 'function', 'lsp-protocol.js (plan 2B) must provide pathForUri');
const editsWindow = { CodeStudioLSPProtocol: Protocol };
const E = loadUmd('lsp-edits.js', editsWindow);
const json = value => JSON.parse(JSON.stringify(value));
const uri = file => Protocol.uriForPath('/workspace/' + file);
const at = (line, character) => ({ line, character });
const edit = (startLine, startChar, endLine, endChar, newText) => ({ range: { start: at(startLine, startChar), end: at(endLine, endChar) }, newText });
const editCode = code => err => !!err && err.editError === code;

test('the module registers on window and as a CommonJS export', () => {
    assert.equal(editsWindow.CodeStudioLSPEdits, E);
    for (const name of ['workspaceEditToFileChanges', 'applyTextEdits', 'textEditsToChanges', 'planTextEdits', 'referencesToGroups', 'referencePreview']) {
        assert.equal(typeof E[name], 'function', name);
    }
});

test('changes: one entry per file with its edits and their count', () => {
    const result = json(E.workspaceEditToFileChanges({ changes: {
        [uri('a.go')]: [edit(0, 0, 0, 1, 'x'), edit(2, 0, 2, 1, 'y')],
        [uri('pkg/b.go')]: [edit(1, 4, 1, 7, 'z')]
    } }));
    assert.deepEqual(result.skipped, []);
    assert.deepEqual(result.files.map(file => [file.path, file.count, file.batches.length]), [['/workspace/a.go', 2, 1], ['/workspace/pkg/b.go', 1, 1]]);
    assert.equal(result.files[0].uri, uri('a.go'));
    assert.deepEqual(result.files[1].edits, [edit(1, 4, 1, 7, 'z')]);
});

test('documentChanges win over changes and keep the document version', () => {
    const result = json(E.workspaceEditToFileChanges({
        changes: { [uri('old.go')]: [edit(0, 0, 0, 0, 'x')] },
        documentChanges: [{ textDocument: { uri: uri('a.go'), version: 3 }, edits: [edit(0, 0, 0, 1, 'y')] }]
    }));
    assert.deepEqual(result.files.map(file => [file.path, file.version, file.count]), [['/workspace/a.go', 3, 1]]);
});

test('a file named twice in documentChanges keeps two ordered batches', () => {
    const first = [edit(0, 0, 0, 1, 'A')];
    const second = [edit(0, 2, 0, 3, 'B')];
    const result = json(E.workspaceEditToFileChanges({ documentChanges: [
        { textDocument: { uri: uri('a.go'), version: 1 }, edits: first },
        { textDocument: { uri: uri('a.go'), version: 2 }, edits: second }
    ] }));
    assert.equal(result.files.length, 1);
    assert.deepEqual(result.files[0].batches, [first, second]);
    assert.equal(result.files[0].count, 2);
});

test('file operations are skipped with a reason, and so are edits of the files they touch', () => {
    const result = json(E.workspaceEditToFileChanges({ documentChanges: [
        { kind: 'create', uri: uri('new.go') },
        { textDocument: { uri: uri('new.go'), version: null }, edits: [edit(0, 0, 0, 0, 'package x\n')] },
        { kind: 'rename', oldUri: uri('x.go'), newUri: uri('y.go') },
        { kind: 'delete', uri: uri('z.go') },
        { textDocument: { uri: uri('a.go'), version: 1 }, edits: [edit(0, 0, 0, 1, 'b')] }
    ] }));
    assert.deepEqual(result.files.map(file => file.path), ['/workspace/a.go']);
    assert.deepEqual(result.skipped.map(item => [item.reason, item.path]), [
        ['create', '/workspace/new.go'],
        ['depends', '/workspace/new.go'],
        ['rename', '/workspace/x.go'],
        ['delete', '/workspace/z.go']
    ]);
});

test('a text edit before the rename of its file is skipped as well, whatever the order', () => {
    const result = json(E.workspaceEditToFileChanges({ documentChanges: [
        { textDocument: { uri: uri('x.go'), version: 1 }, edits: [edit(0, 0, 0, 1, 'b')] },
        { kind: 'rename', oldUri: uri('x.go'), newUri: uri('y.go') }
    ] }));
    assert.deepEqual(result.files, []);
    assert.deepEqual(result.skipped.map(item => [item.reason, item.path]), [
        ['depends', '/workspace/x.go'],
        ['rename', '/workspace/x.go']
    ]);
});

test('edits outside the workspace and malformed edits are skipped; empty edit lists are ignored', () => {
    const result = json(E.workspaceEditToFileChanges({ changes: {
        'file:///etc/passwd': [edit(0, 0, 0, 0, 'x')],
        'file:///workspace/../etc/hosts': [edit(0, 0, 0, 0, 'x')],
        [uri('a.go')]: [{ range: { start: at(0, 0), end: at(0, 1) }, text: 'no newText' }],
        [uri('b.go')]: []
    } }));
    assert.deepEqual(result.files, []);
    assert.deepEqual(result.skipped.map(item => item.reason), ['outside', 'outside', 'invalid']);
    assert.deepEqual(json(E.workspaceEditToFileChanges(null)), { files: [], skipped: [] });
});

test('applyTextEdits applies unsorted edits from the end of the text', () => {
    const text = 'let a = 1;\nlet b = a + a;\n';
    const edits = [edit(1, 12, 1, 13, 'count'), edit(0, 4, 0, 5, 'count'), edit(1, 8, 1, 9, 'count')];
    assert.equal(E.applyTextEdits(text, edits), 'let count = 1;\nlet b = count + count;\n');
});

test('positions count UTF-16 code units, as LSP and JavaScript strings do', () => {
    const smile = String.fromCodePoint(0x1F600);
    assert.equal(smile.length, 2);
    assert.equal(E.applyTextEdits('x' + smile + 'y = 1', [edit(0, 3, 0, 4, 'z')]), 'x' + smile + 'z = 1');
});

test('CRLF and a lone CR end lines; a character past the line end clamps to it', () => {
    const text = 'one\r\ntwo\rthree';
    assert.equal(E.applyTextEdits(text, [edit(1, 0, 1, 3, 'TWO')]), 'one\r\nTWO\rthree');
    assert.equal(E.applyTextEdits(text, [edit(0, 99, 0, 99, '.')]), 'one.\r\ntwo\rthree');
    assert.equal(E.applyTextEdits(text, [edit(2, 99, 2, 99, '!')]), 'one\r\ntwo\rthree!');
});

test('inserts at one position keep their order and may precede one replacement', () => {
    assert.equal(E.applyTextEdits('ab', [edit(0, 1, 0, 1, 'X'), edit(0, 1, 0, 1, 'Y')]), 'aXYb');
    assert.equal(E.applyTextEdits('ab', [edit(0, 1, 0, 2, 'Z'), edit(0, 1, 0, 1, 'I')]), 'aIZ');
});

test('overlapping, out-of-range and malformed edits are rejected', () => {
    assert.throws(() => E.applyTextEdits('abcdef', [edit(0, 0, 0, 3, 'x'), edit(0, 2, 0, 4, 'y')]), editCode('overlap'));
    assert.throws(() => E.applyTextEdits('a\nb', [edit(5, 0, 5, 0, 'x')]), editCode('range'));
    assert.throws(() => E.applyTextEdits('a\nb', [edit(2, 1, 2, 1, 'x')]), editCode('range'));
    assert.throws(() => E.applyTextEdits('abc', [edit(0, 2, 0, 1, 'x')]), editCode('range'));
    assert.throws(() => E.applyTextEdits('abc', [edit(-1, 0, 0, 1, 'x')]), editCode('invalid'));
    assert.throws(() => E.applyTextEdits('abc', [{ range: { start: at(0, 0), end: at(0, 1) } }]), editCode('invalid'));
    assert.equal(E.applyTextEdits('a\nb', [edit(2, 0, 2, 0, 'X')]), 'a\nbX', 'the line after the last one means the end of the text');
});

test('textEditsToChanges gives ascending offsets for one CodeMirror transaction', () => {
    assert.deepEqual(json(E.textEditsToChanges('foo bar\nfoo', [edit(1, 0, 1, 3, 'baz'), edit(0, 0, 0, 3, 'baz')])), [
        { from: 0, to: 3, insert: 'baz' },
        { from: 8, to: 11, insert: 'baz' }
    ]);
});

test('planTextEdits applies batches one after the other', () => {
    const plan = json(E.planTextEdits('a b', [[edit(0, 0, 0, 1, 'aaa')], [edit(0, 4, 0, 5, 'B')]]));
    assert.equal(plan.text, 'aaa B');
    assert.deepEqual(plan.steps, [[{ from: 0, to: 1, insert: 'aaa' }], [{ from: 4, to: 5, insert: 'B' }]]);
});

test('references are grouped by file, sorted and de-duplicated; results outside the workspace are counted', () => {
    const loc = (file, line, character, length) => ({
        uri: file.startsWith('file:') ? file : uri(file),
        range: { start: at(line, character), end: at(line, character + length) }
    });
    const result = json(E.referencesToGroups([
        loc('b.go', 4, 1, 3),
        loc('a.go', 9, 0, 3),
        loc('a.go', 2, 5, 3),
        loc('a.go', 2, 5, 3),
        loc('file:///usr/local/go/src/fmt/print.go', 10, 5, 7),
        { uri: uri('c.go') },
        null
    ]));
    assert.deepEqual(result.groups.map(group => [group.path, group.items.map(item => item.line)]), [['/workspace/a.go', [2, 9]], ['/workspace/b.go', [4]]]);
    assert.deepEqual(result.groups[0].items[0], { line: 2, character: 5, endLine: 2, endCharacter: 8 });
    assert.equal(result.outside, 1);
    assert.equal(result.total, 3);
});

test('reference previews drop the indentation and split the line around the range', () => {
    const text = 'package main\n\n\tfmt.Println(x)\n';
    assert.deepEqual(json(E.referencePreview(text, { line: 2, character: 5, endLine: 2, endCharacter: 12 })), { before: 'fmt.', match: 'Println', after: '(x)' });
    const long = 'x'.repeat(50) + 'name()';
    const preview = json(E.referencePreview(long, { line: 0, character: 50, endLine: 0, endCharacter: 54 }));
    assert.equal(preview.before, String.fromCodePoint(0x2026) + 'x'.repeat(39));
    assert.equal(preview.match, 'name');
    assert.equal(preview.after, '()');
    assert.equal(E.referencePreview(text, { line: 9, character: 0, endLine: 9, endCharacter: 1 }), null);
});
