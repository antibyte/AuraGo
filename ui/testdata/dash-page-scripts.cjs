// The dashboard's global escapeHtml must be main.js's quote-escaping helper. Reads ui/dashboard.html, takes its
// first-party scripts (/js/..., not /js/vendor/) in page order, plus the bodies of its inline <script> elements and of
// the inline scripts in the section partials (ui/dashboard/**/*.html), and looks for every way such a script can
// define or replace the global escapeHtml, without running any top-level code. Classic scripts share one global scope
// and a later definition replaces an earlier one, so every definer besides main.js would decide what the page renders
// with. Two detections per source:
//   - Declarations: the source is compiled (never run) inside a function whose first statement returns escapeHtml,
//     in a context whose global escapeHtml is a sentinel. A top-level function or var (including a function in a
//     block, which creates a var binding) shadows the sentinel and is read as that function or undefined; let, const
//     and class throw (temporal dead zone); IIFE-local helpers leave the sentinel untouched and do not count.
//   - Assignments, by a static scan (comments are not stripped, so a commented-out override is reported too): a bare
//     escapeHtml = ... (also ??=, ||=, &&= and the like) that is not a declaration, window/globalThis/self/this
//     .escapeHtml or ['escapeHtml'] assigned to, and Object.assign/defineProperty/defineProperties or Reflect.set/
//     defineProperty on window/globalThis/self/this whose arguments mention escapeHtml.
// The detector is proven first on in-memory sources (every form must be reported, local helpers and plain uses must
// not), then applied to the real page.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const ui = path.join(__dirname, '..');

const ASSIGN = /\s*(?:\*\*|<<|>>>?|&&|\|\||\?\?|[-+*\/%&|^])?=(?![=>])/.source;
const BARE_ASSIGN = new RegExp(/(^|[^\w$.])escapeHtml(?![\w$])/.source + ASSIGN, 'g');
const MEMBER_ASSIGN = new RegExp(/(?:^|[^\w$.])(?:window|globalThis|self|this)\s*(?:\.\s*escapeHtml(?![\w$])|\[\s*(['"`])escapeHtml\1\s*\])/.source + ASSIGN, 'g');
const GLOBAL_CALL = /(?:^|[^\w$.])(?:Object\s*\.\s*(?:assign|defineProperty|defineProperties)|Reflect\s*\.\s*(?:set|defineProperty))\s*\(\s*(?:window|globalThis|self|this)(?![\w$])/g;

// The text of the call whose opening parenthesis is at source[open] (to the end of the source when it never closes).
function callArguments(source, open) {
    let depth = 0;
    let quote = '';
    for (let i = open; i < source.length; i++) {
        const c = source[i];
        if (quote) {
            if (c === '\\') i++;
            else if (c === quote) quote = '';
        } else if (c === '"' || c === "'" || c === '`') quote = c;
        else if (c === '(') depth++;
        else if (c === ')' && --depth === 0) return source.slice(open, i + 1);
    }
    return source.slice(open);
}

// Every way one script source defines or replaces the global escapeHtml:
// { kind: 'declaration', fn } | { kind: 'var' } | { kind: 'lexical' } | { kind: 'assignment', detail }.
function escapeHtmlDefinitions(source, label) {
    if (source.charCodeAt(0) === 0xfeff) source = source.slice(1);
    const sentinel = function sentinelEscapeHtml() {};
    const wrapper = '(function () {\ntry { return escapeHtml; } catch (_) { return \'lexical\'; }\n' + source + '\n})';
    let value;
    try {
        value = vm.runInNewContext(wrapper, { escapeHtml: sentinel }, { filename: label })();
    } catch (err) {
        throw new Error('cannot analyse ' + label + ': ' + err.message);
    }
    const found = [];
    if (value === 'lexical') found.push({ kind: 'lexical' });
    else if (typeof value === 'function' && value !== sentinel) found.push({ kind: 'declaration', fn: value });
    else if (value === undefined) found.push({ kind: 'var' });

    for (const m of source.matchAll(BARE_ASSIGN)) {
        const before = source.slice(0, m.index + m[1].length);
        if (!/\b(?:var|let|const)\s*$/.test(before)) found.push({ kind: 'assignment', detail: m[0].trim() });
    }
    for (const m of source.matchAll(MEMBER_ASSIGN)) found.push({ kind: 'assignment', detail: m[0].trim() });
    for (const m of source.matchAll(GLOBAL_CALL)) {
        const args = callArguments(source, m.index + m[0].indexOf('('));
        if (/escapeHtml(?![\w$])/.test(args)) found.push({ kind: 'assignment', detail: m[0].trim() + ' ... escapeHtml' });
    }
    return found;
}

// Bodies of the inline JavaScript <script> elements of an HTML file (no src; JSON and other data types are skipped).
function inlineScriptBodies(html) {
    const bodies = [];
    for (const m of html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script\s*>/gi)) {
        if (/\bsrc\s*=/i.test(m[1])) continue;
        const type = /\btype\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))/i.exec(m[1]);
        const kind = type ? (type[1] ?? type[2] ?? type[3]).trim().toLowerCase() : '';
        if (kind && kind !== 'module' && !/^(?:text|application)\/(?:x-)?(?:java|ecma)script$/.test(kind)) continue;
        bodies.push(m[2]);
    }
    return bodies;
}

function htmlFilesUnder(dir) {
    return fs.readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) return htmlFilesUnder(full);
        return entry.name.endsWith('.html') ? [full] : [];
    });
}

// Detector self-test: every form must be reported, local helpers and plain uses must not.
const mustDefine = {
    'function declaration': 'function escapeHtml(s) { return s; }',
    'top-level var': 'var escapeHtml = function (s) { return s; };',
    'top-level let': 'let escapeHtml = s => s;',
    'top-level const': 'const escapeHtml = s => s;',
    'top-level class': 'class escapeHtml {}',
    'block-level function (Annex B)': 'if (window.flag) {\n  function escapeHtml(s) { return s; }\n}',
    'bare assignment': 'escapeHtml = function (s) { return s; };',
    'bare assignment inside an IIFE': '(function () { escapeHtml = function (s) { return s; }; })();',
    'bare ??= assignment': 'escapeHtml ??= function (s) { return s; };',
    'bare ||= assignment': 'escapeHtml ||= function (s) { return s; };',
    'window. assignment': 'window.escapeHtml = function (s) { return s; };',
    'window. ??= assignment': 'window.escapeHtml ??= function (s) { return s; };',
    'globalThis[] assignment': 'globalThis[\'escapeHtml\'] = function (s) { return s; };',
    'self[] assignment': 'self["escapeHtml"] = function (s) { return s; };',
    'self. assignment': 'self.escapeHtml = function (s) { return s; };',
    'this. assignment': 'this.escapeHtml = function (s) { return s; };',
    'Object.assign(window, ...)': 'Object.assign(window, {\n  other: 1,\n  escapeHtml: function (s) { return s; },\n});',
    'Object.defineProperty(globalThis, ...)': 'Object.defineProperty(globalThis, \'escapeHtml\', { value: function (s) { return s; } });',
    'Reflect.set(window, ...)': 'Reflect.set(window, \'escapeHtml\', function (s) { return s; });',
};
for (const [name, source] of Object.entries(mustDefine)) {
    assert.ok(escapeHtmlDefinitions(source, name).length > 0, 'the detector must report: ' + name);
}
const mustNotDefine = {
    'IIFE-local function': '(function () { function escapeHtml(s) { return s; } return escapeHtml(\'x\'); })();',
    'IIFE-local var': '(function () { var escapeHtml = function (s) { return s; }; return escapeHtml(\'x\'); })();',
    'function-local let': 'function render(v) { let escapeHtml = s => s; return escapeHtml(v); }',
    'call': 'const out = escapeHtml(\'x\');',
    'comparison': 'const same = a === escapeHtml || escapeHtml === a || escapeHtml != null || escapeHtml <= 1;',
    'property read': 'const fn = obj.escapeHtml;',
    'property assignment on another object': 'obj.escapeHtml = function (s) { return s; };',
    'arrow parameter': 'const pick = escapeHtml => 1;',
    'Object.assign onto another target': 'Object.assign(target, { escapeHtml: 1 });',
    'Object.assign(window, ...) without escapeHtml': 'Object.assign(window, { other: 1 });',
};
for (const [name, source] of Object.entries(mustNotDefine)) {
    assert.deepEqual(escapeHtmlDefinitions(source, name).map(f => f.kind), [], 'the detector must not report: ' + name);
}
const inlineHtml = [
    '<script type="application/json" id="data">{"escapeHtml": 1}</script>',
    '<script src="/js/x.js"></script>',
    '<script>function escapeHtml(s) { return s; }</script>',
    '<script type="text/javascript">window.escapeHtml = null;</script>',
].join('\n');
assert.deepEqual(inlineScriptBodies(inlineHtml), ['function escapeHtml(s) { return s; }', 'window.escapeHtml = null;'], 'inline scripts: JSON blocks and external scripts are skipped, JavaScript bodies are returned');
assert.ok(escapeHtmlDefinitions(inlineScriptBodies(inlineHtml)[0], 'inline').length > 0, 'the detector must report an inline script body');

// The real page.
const dashboardHtml = path.join(ui, 'dashboard.html');
const html = fs.readFileSync(dashboardHtml, 'utf8');
const scripts = Array.from(html.matchAll(/<script\b[^>]*\bsrc="([^"?]+)(?:\?[^"]*)?"[^>]*>/g), m => m[1])
    .filter(src => src.startsWith('/js/') && !src.startsWith('/js/vendor/'));
assert.ok(scripts.includes('/js/dashboard/main.js') && scripts.includes('/js/knowledge/file_sync_status.js'), 'dashboard.html script list: ' + scripts.join(', '));

const sources = scripts.map(src => ({ label: src, source: fs.readFileSync(path.join(ui, ...src.split('/').filter(Boolean)), 'utf8') }));
for (const file of [dashboardHtml, ...htmlFilesUnder(path.join(ui, 'dashboard'))]) {
    const rel = path.relative(ui, file).split(path.sep).join('/');
    inlineScriptBodies(fs.readFileSync(file, 'utf8')).forEach((body, i) => sources.push({ label: rel + ' inline <script> #' + (i + 1), source: body }));
}

const definers = [];
for (const { label, source } of sources) {
    const found = escapeHtmlDefinitions(source, label);
    if (found.length) definers.push({ label, found });
}
const describe = d => d.label + ' (' + d.found.map(f => f.kind + (f.detail ? ': ' + f.detail : '')).join(', ') + ')';
assert.deepEqual(definers.map(d => d.label), ['/js/dashboard/main.js'], 'exactly one page script may define the global escapeHtml, found: ' + definers.map(describe).join('; '));
assert.ok(definers[0].found.length === 1 && definers[0].found[0].kind === 'declaration', 'main.js must define escapeHtml once, as a top-level function: ' + describe(definers[0]));
assert.equal(definers[0].found[0].fn('"\'<>&'), '&quot;&#39;&lt;&gt;&amp;', 'escapeHtml escapes both quote kinds');
console.log('dashboard page scripts: escapeHtml comes from main.js only (' + scripts.length + ' first-party scripts, ' + (sources.length - scripts.length) + ' inline scripts) and escapes quotes.');
