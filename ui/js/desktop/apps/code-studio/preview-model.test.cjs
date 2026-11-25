const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

// package.json has "type": "module": run the UMD source in a vm with a fake module, like lsp-protocol.test.cjs.
const mod = { exports: {} };
vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'preview-model.js'), 'utf8'), { module: mod, window: {} });
const M = mod.exports;
const json = value => JSON.parse(JSON.stringify(value));

test('only HTML, HTM and SVG files are previewable', () => {
    for (const p of ['/workspace/index.html', '/workspace/a/B.HTM', '/workspace/logo.svg']) assert.equal(M.isPreviewablePath(p), true, p);
    for (const p of ['/workspace/app.js', '/workspace/html', '/workspace/.html', '', null]) assert.equal(M.isPreviewablePath(p), false, String(p));
});

test('relative paths strip the workspace root', () => {
    assert.equal(M.relativePath('/workspace'), '');
    assert.equal(M.relativePath('/workspace/site/index.html'), 'site/index.html');
    assert.equal(M.relativePath('/etc/passwd'), null);
});

test('addresses stay inside the workspace', () => {
    assert.deepEqual(json(M.parseAddress('site/index.html?x=1#top')), { path: '/workspace/site/index.html', directory: false, suffix: '?x=1#top' });
    assert.deepEqual(json(M.parseAddress('/workspace/site/')), { path: '/workspace/site', directory: true, suffix: '' });
    assert.deepEqual(json(M.parseAddress('/')), { path: '/workspace', directory: true, suffix: '' });
    assert.deepEqual(json(M.parseAddress('./docs/my%20page.html')), { path: '/workspace/docs/my page.html', directory: false, suffix: '' });
    for (const bad of ['', '   ', '../secret.html', 'a/../../b', 'https://example.com/', '//evil/x', 'javascript:alert(1)', 'a\\b.html', 'a%2Fb.html', 'a/%2e%2e/b']) {
        assert.equal(M.parseAddress(bad), null, bad);
    }
});

test('preview URLs encode every segment and the address shows the relative path', () => {
    assert.equal(M.buildUrl('/code-studio-preview/t/', '/workspace/docs/my page#1.html', false, '?q=1'), '/code-studio-preview/t/docs/my%20page%231.html?q=1');
    assert.equal(M.buildUrl('/code-studio-preview/t/', '/workspace/docs', true, ''), '/code-studio-preview/t/docs/');
    assert.equal(M.buildUrl('/code-studio-preview/t/', '/workspace', true, ''), '/code-studio-preview/t/');
    assert.equal(M.addressText('/workspace/docs', true, '?a=1'), 'docs/?a=1');
    assert.equal(M.addressText('/workspace/index.html', false, ''), 'index.html');
});

test('the pane width leaves room for the editor', () => {
    assert.equal(M.clampWidth(100, 1200), 280);
    assert.equal(M.clampWidth(5000, 1200), 880);
    assert.equal(M.clampWidth('x', 1200), 480);
    assert.equal(M.clampWidth(600, 0), 600);
});

test('device frames scale down to fit the stage', () => {
    assert.equal(M.deviceScale('responsive', 300, 300), 1);
    assert.equal(M.deviceScale('mobile', 1000, 1000), 1);
    assert.equal(M.deviceScale('mobile', 348, 1000, 24), 0.8);
    assert.equal(M.deviceScale('tablet', 2000, 560, 24), 0.5);
    assert.equal(M.deviceScale('mobile', 50, 50), 0.25);
});

test('messages from the preview need the exact shape', () => {
    assert.deepEqual(json(M.normalizeMessage({ auragoPreview: 1, kind: 'console', text: 'boom' })), { kind: 'console', text: 'boom' });
    assert.deepEqual(json(M.normalizeMessage({ auragoPreview: 1, kind: 'location' })), { kind: 'location', text: '' });
    assert.equal(M.normalizeMessage({ auragoPreview: 1, kind: 'console', text: 'x'.repeat(5000) }).text.length, 4000);
    for (const bad of [null, 'text', { kind: 'console', text: 'x' }, { auragoPreview: '1', kind: 'console', text: 'x' },
        { auragoPreview: 1, kind: 'eval', text: 'x' }, { auragoPreview: 1, kind: 'toString', text: 'x' },
        { auragoPreview: 1, kind: 'console', text: { toString() { return 'x'; } } }]) {
        assert.equal(M.normalizeMessage(bad), null);
    }
});

test('the console keeps the newest entries', () => {
    let list = [];
    for (let i = 0; i < 205; i++) list = M.pushEntry(list, { kind: 'console', text: String(i) });
    assert.equal(list.length, 200);
    assert.equal(list[0].text, '5');
    assert.equal(M.isErrorKind('resource'), true);
    assert.equal(M.isErrorKind('location'), false);
});

test('tokens are refreshed ten minutes before they expire', () => {
    const now = Date.parse('2026-11-25T10:00:00Z');
    assert.equal(M.tokenNeedsRefresh('2026-11-25T22:00:00Z', now), false);
    assert.equal(M.tokenNeedsRefresh('2026-11-25T10:05:00Z', now), true);
    assert.equal(M.tokenNeedsRefresh('', now), true);
});
