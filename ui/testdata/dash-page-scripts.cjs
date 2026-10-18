// The dashboard's global escapeHtml must be main.js's quote-escaping helper. Reads ui/dashboard.html, takes its
// first-party scripts (/js/..., not /js/vendor/) in page order and looks for top-level definitions of escapeHtml
// without running any top-level code: a top-level function declaration is hoisted, so a wrapper that returns it before
// the script body yields exactly that file's declaration. Classic scripts share one global scope and a later
// declaration replaces an earlier one, so every definer besides main.js would decide what the page renders with.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const ui = path.join(__dirname, '..');
const html = fs.readFileSync(path.join(ui, 'dashboard.html'), 'utf8');
const scripts = Array.from(html.matchAll(/<script\b[^>]*\bsrc="([^"?]+)(?:\?[^"]*)?"[^>]*>/g), m => m[1])
    .filter(src => src.startsWith('/js/') && !src.startsWith('/js/vendor/'));
assert.ok(scripts.includes('/js/dashboard/main.js') && scripts.includes('/js/knowledge/file_sync_status.js'), 'dashboard.html script list: ' + scripts.join(', '));

const definers = [];
for (const src of scripts) {
    const file = path.join(ui, ...src.split('/').filter(Boolean));
    let source = fs.readFileSync(file, 'utf8');
    if (source.charCodeAt(0) === 0xfeff) source = source.slice(1);
    const wrapper = '(function () {\ntry { return typeof escapeHtml === \'function\' ? escapeHtml : null; } catch (_) { return \'lexical\'; }\n' + source + '\n})';
    const found = vm.runInNewContext(wrapper, {}, { filename: file })();
    if (found) definers.push({ src, fn: found });
    else if (/(?:^|[^\w$.])(?:window|globalThis|self)\.escapeHtml\s*=/m.test(source)) definers.push({ src, fn: 'assignment' });
}
assert.deepEqual(definers.map(d => d.src), ['/js/dashboard/main.js'], 'exactly one page script may define the global escapeHtml, found: ' + definers.map(d => d.src).join(', '));
assert.equal(definers[0].fn('"\'<>&'), '&quot;&#39;&lt;&gt;&amp;', 'escapeHtml escapes both quote kinds');
console.log('dashboard page scripts: escapeHtml comes from main.js only (' + scripts.length + ' first-party scripts) and escapes quotes.');
