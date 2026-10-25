const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
class Element {
    constructor() { this.children = []; this.options = []; this.value = ''; this.hidden = false; this.disabled = false; this.textContent = ''; }
    replaceChildren(...items) { this.children = items; this.options = items; }
    appendChild(item) { this.children.push(item); }
    add(item) { this.options.push(item); }
    set innerHTML(_) { throw new Error('unsafe HTML rendering'); }
}
const elements = new Map();
for (const id of ['status', 'summary', 'rows', 'start', 'stop', 'target', 'profile', 'report', 'results', 'bindings-panel', 'bindings']) elements.set('selftest-' + id, new Element());
elements.set('sec-tools', new Element());
elements.get('selftest-profile').value = 'read';
const listeners = new Map();
const timers = new Map(); let timerID = 0;
let posts = 0; let calls = 0; let finishStart;
const id = 'a'.repeat(32);
const run = { id, status: 'running', summary: { enabled_tools: 3, executed_tools: 1, verdicts: { FAIL: 1 } }, cases: [{ template_id: 'ONE', tool: '<img src=x onerror=alert(1)>', verdict: 'FAIL', code: '<script>bad</script>' }] };
const response = data => Promise.resolve({ ok: true, json: async () => data });
const document = { hidden: false, getElementById: id => elements.get(id), createElement: () => new Element(), addEventListener: (name, fn) => listeners.set(name, fn) };
const ctx = { document, console, AbortController, Option: class { constructor(text, value) { this.text = text; this.value = value; } }, t: key => key,
    setTimeout: fn => { timers.set(++timerID, fn); return timerID; }, clearTimeout: id => timers.delete(id), addEventListener: () => {},
    fetch: (url, options = {}) => { calls++;
        if (options.method === 'POST' && url === '/api/self-tests') { posts++; return new Promise(resolve => { finishStart = data => resolve({ ok: true, json: async () => data }); }); }
        if (url.endsWith('/cancel')) { run.status = 'cancelled'; return response({ cancelled: true }); }
        if (url.endsWith('/catalog')) return response({ inventory: [{ name: 'filesystem', enabled: true }] });
        if (url === '/api/self-tests') return response({ runs: posts ? [run] : [] });
        return response(run);
    }
};
ctx.window = ctx;
vm.runInNewContext(fs.readFileSync(path.join(__dirname, '..', 'js', 'dashboard', 'self-tests.js'), 'utf8'), ctx);
const flush = () => new Promise(resolve => setImmediate(resolve));
const click = action => listeners.get('click')({ target: { closest: () => ({ dataset: { selftestAction: action } }) } });
(async () => {
    await ctx.AuraSelfTests.load();
    assert.match(elements.get('selftest-status').textContent, /empty/);
    click('start'); click('start'); await flush();
    assert.equal(posts, 1, 'duplicate start'); assert.equal(elements.get('selftest-start').disabled, true);
    finishStart(run); await flush(); await flush();
    assert.equal(elements.get('selftest-stop').disabled, false);
    assert.equal(elements.get('selftest-rows').children[0].children[1].textContent, run.cases[0].tool);
    assert.equal(elements.get('selftest-report').href, '/api/self-tests/' + id + '/report.md');
    elements.get('sec-tools').hidden = true;
    const count = calls; for (const fn of [...timers.values()]) await fn();
    assert.equal(calls, count, 'hidden section polled');
    elements.get('sec-tools').hidden = false;
    click('stop'); await flush(); await flush();
    assert.equal(elements.get('selftest-stop').disabled, true);
    assert.equal(elements.get('selftest-start').disabled, false);
    assert.match(elements.get('selftest-status').textContent, /cancelled/);
    console.log('Self-test dashboard: duplicate prevention, safe rendering, polling, stop and downloads passed.');
})().catch(error => { console.error(error); process.exitCode = 1; });
