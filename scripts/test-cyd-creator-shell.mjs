#!/usr/bin/env node
// Runs the pure parts of the CYD Creator shell (cyd-creator-core.js, -api.js, -events.js) in Node:
// P.util, P.i18n, P.errors, the API error envelope and request(), the autosave/conflict state
// machine with a fake clock, and the SSE cursor, backoff, backlog coalescing and reconnect logic.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';
import { webcrypto } from 'node:crypto';

const here = path.dirname(fileURLToPath(import.meta.url));
const apps = path.join(here, '..', 'ui', 'js', 'desktop', 'apps');

// ── fake clock shared by the sandbox ──
let now = 0;
let timerSeq = 0;
const timers = new Map();
function fakeSetTimeout(fn, ms) { const id = ++timerSeq; timers.set(id, { at: now + (Number(ms) || 0), fn }); return id; }
function fakeClearTimeout(id) { timers.delete(id); }
async function settle() { for (let i = 0; i < 20; i++) await new Promise(r => setImmediate(r)); }
async function advance(ms) {
    const end = now + ms;
    for (;;) {
        let next = null;
        timers.forEach((t, id) => { if (t.at <= end && (!next || t.at < next.t.at)) next = { id, t }; });
        if (!next) break;
        timers.delete(next.id);
        now = next.t.at;
        next.t.fn();
        await settle();
    }
    now = end;
    await settle();
}

const sandbox = {
    window: { I18N: {}, SYSTEM_LANG: 'de' },
    console, JSON, Math, Date, Number, String, Array, Object, Set, Map, Promise, Error, RegExp, Uint8Array, Intl,
    crypto: webcrypto, structuredClone, Response, Headers, FormData, Blob,
    setTimeout: fakeSetTimeout, clearTimeout: fakeClearTimeout, setInterval: fakeSetTimeout, clearInterval: fakeClearTimeout,
    encodeURIComponent, fetch: null
};
sandbox.globalThis = sandbox;
vm.createContext(sandbox);
for (const file of ['cyd-creator-core.js', 'cyd-creator-api.js', 'cyd-creator-events.js']) {
    const full = path.join(apps, file);
    vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
}
const P = sandbox.window.CYDCreatorParts;

let failures = 0;
function check(name, cond, detail) {
    if (cond) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + (detail ? ' — ' + detail : ''));
}
function eq(name, got, want) {
    check(name, JSON.stringify(got) === JSON.stringify(want), 'got ' + JSON.stringify(got) + ' want ' + JSON.stringify(want));
}

// ── util ──
check('uid has the Go RandomIDs shape', /^w_[0-9a-f]{12}$/.test(P.util.uid('w')));
check('uid differs between calls', P.util.uid('w') !== P.util.uid('w'));
eq('clamp limits', [P.util.clamp(5, 0, 3), P.util.clamp(-1, 0, 3), P.util.clamp('x', 2, 3)], [3, 0, 2]);
const frozen = P.util.deepFreeze({ a: { b: [1, 2] } });
check('deepFreeze freezes nested values', Object.isFrozen(frozen.a.b));
const copy = P.util.deepClone(frozen);
copy.a.b.push(3);
eq('deepClone is mutable and independent', [copy.a.b.length, frozen.a.b.length], [3, 2]);
eq('escapeText escapes markup', P.util.escapeText('<a href="x">&\'</a>'), '&lt;a href=&quot;x&quot;&gt;&amp;&#39;&lt;/a&gt;');
{
    let calls = 0;
    const d = P.util.debounce(v => { calls += v; return calls; }, 100);
    d(1); d(2);
    await advance(50);
    eq('debounce waits', calls, 0);
    eq('debounce flush runs the last call at once', d.flush(), 2);
    d(5); d.cancel();
    await advance(200);
    eq('debounce cancel drops the call', calls, 2);
}

// ── i18n and errors ──
sandbox.window.I18N = {
    'cyd_creator.app.title': 'CYD Creator',
    'cyd_creator.home.hello': 'Hallo {{name}}, {{name}}!',
    'cyd_creator.designer.added': '{name} hinzugefügt',
    'cyd_creator.error.CYD_BUSY': 'Beschäftigt',
    'cyd_creator.error.CYD_INTERNAL': 'Interner Fehler',
    'cyd_creator.error.network': 'Netzwerk',
    'cyd_creator.error.generic': 'Fehler'
};
const t = P.i18n({});
eq('t adds the section prefix', t('app.title'), 'CYD Creator');
eq('t accepts the full key', t('cyd_creator.app.title'), 'CYD Creator');
eq('t interpolates every placeholder', t('home.hello', { name: 'Ada' }), 'Hallo Ada, Ada!');
eq('a missing key shows its full name', t('home.nope'), 'cyd_creator.home.nope');
eq('t fills single-brace placeholders of plan 4b', t('designer.added', { name: 'btn_ok' }), 'btn_ok hinzugefügt');
eq('t.has', [t.has('app.title'), t.has('home.nope')], [true, false]);
eq('t.or uses the key when it exists', t.or('designer.added', 'x', { name: 'a' }), 'a hinzugefügt');
eq('t.or falls back for a missing key', t.or('cyd_creator.theme.aurora', 'Aurora'), 'Aurora');
const { ApiError, toError } = P.api;
eq('errors.message uses error.<CODE>', P.errors.message(t, new ApiError(409, 'CYD_BUSY', 'busy')), 'Beschäftigt');
eq('errors.message: status 0 is a network error', P.errors.message(t, new ApiError(0, '', 'offline')), 'Netzwerk');
eq('errors.message: unknown codes are generic', P.errors.message(t, new ApiError(500, 'CYD_SOMETHING_NEW', 'x')), 'Fehler');
eq('errors.message: CYD_INTERNAL (plan 2, HTTP 500) has its own text', P.errors.message(t, new ApiError(500, 'CYD_INTERNAL', 'db')), 'Interner Fehler');
eq('a CYD_INTERNAL save failure retries like other 5xx', P.autosave.classify(new ApiError(500, 'CYD_INTERNAL', 'db')), 'offline');
eq('errors.message: plain errors are generic', P.errors.message(t, new Error('boom')), 'Fehler');

// ── API envelope ──
{
    const e = toError(409, { error: 'conflict', code: 'CYD_PROJECT_CONFLICT', details: { version: 14 } });
    eq('toError keeps status, code, message and details', [e.name, e.status, e.code, e.message, e.details.version], ['ApiError', 409, 'CYD_PROJECT_CONFLICT', 'conflict', 14]);
    const plain = toError(502, '<html>Bad gateway</html>');
    eq('toError of a text body', [plain.code, plain.message], ['', '<html>Bad gateway</html>']);
    eq('toError without a body', toError(500, '').message, 'HTTP 500');
    // Real server (plan 2 Task 13): CYD_DISABLED says why in details (problem or restart_required).
    const disabled = toError(503, { error: 'CYD Creator was switched on; restart AuraGo to start it', code: 'CYD_DISABLED', details: { restart_required: true } });
    eq('toError keeps the CYD_DISABLED details', [disabled.status, disabled.code, disabled.details.restart_required], [503, 'CYD_DISABLED', true]);
}
{
    const seen = [];
    sandbox.fetch = async (url, init) => {
        seen.push({ url, method: init.method, headers: init.headers, body: init.body, credentials: init.credentials });
        if (url.endsWith('/layout') && init.method === 'PUT') {
            if (init.headers['If-Match'] !== '12') return new Response(JSON.stringify({ error: 'stale', code: 'CYD_PROJECT_CONFLICT', details: { version: 13 } }), { status: 409, headers: { 'Content-Type': 'application/json' } });
            return new Response(JSON.stringify({ version: 13, issues: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } });
        }
        if (url.includes('/files/src/app/app_main.cpp') && init.method === 'GET') return new Response('void app_setup() {}\n', { status: 200, headers: { 'Content-Type': 'text/plain; charset=utf-8', 'X-CYD-SHA256': 'abc' } });
        if (url.includes('/files/') && init.method === 'DELETE') return new Response(null, { status: 204 });
        if (url.endsWith('/projects')) return new Response(JSON.stringify({ projects: [{ id: 'ccp_1' }] }), { status: 200, headers: { 'Content-Type': 'application/json' } });
        if (url.endsWith('/capabilities')) throw new TypeError('Failed to fetch');
        // Real server (plan 3 Task 9): job cancel answers 202 {"job"}, draft discard 204.
        if (url.endsWith('/jobs/ccj_1/cancel') && init.method === 'POST') return new Response(JSON.stringify({ job: { id: 'ccj_1', status: 'running' } }), { status: 202, headers: { 'Content-Type': 'application/json; charset=utf-8' } });
        if (url.endsWith('/draft/discard') && init.method === 'POST') return new Response(null, { status: 204 });
        return new Response(JSON.stringify({ error: 'nope', code: 'CYD_NOT_FOUND' }), { status: 404, headers: { 'Content-Type': 'application/json' } });
    };
    eq('listProjects unwraps the payload', (await P.api.listProjects()).map(p => p.id), ['ccp_1']);
    eq('requests are same-origin under the API base', [seen[0].url, seen[0].credentials], ['/api/desktop/cyd-creator/projects', 'same-origin']);
    eq('putLayout sends If-Match and the project envelope', await P.api.putLayout('ccp_1', { name: 'x' }, 12), { version: 13, issues: [] });
    eq('putLayout body', JSON.parse(seen[1].body), { project: { name: 'x' } });
    try {
        await P.api.putLayout('ccp_1', {}, 11);
        check('a stale layout throws', false);
    } catch (err) {
        eq('a stale layout throws CYD_PROJECT_CONFLICT with the server version', [err.name, err.status, err.code, err.details.version], ['ApiError', 409, 'CYD_PROJECT_CONFLICT', 13]);
    }
    eq('readFile returns text and X-CYD-SHA256', await P.api.readFile('ccp_1', 'src/app/app_main.cpp'), { text: 'void app_setup() {}\n', sha: 'abc' });
    check('readFile keeps the slashes of the path', seen.some(s => s.url === '/api/desktop/cyd-creator/projects/ccp_1/files/src/app/app_main.cpp'));
    eq('deleteFile accepts 204', await P.api.deleteFile('ccp_1', 'src/app/x.cpp', 'sha1'), true);
    eq('deleteFile sends If-Match', seen[seen.length - 1].headers['If-Match'], 'sha1');
    try {
        await P.api.capabilities();
        check('a failed fetch throws', false);
    } catch (err) {
        eq('a failed fetch is ApiError status 0', [err.name, err.status], ['ApiError', 0]);
    }
    try {
        await P.api.getBuild('ccb_x');
        check('a 404 throws', false);
    } catch (err) {
        eq('a 404 carries its code', err.code, 'CYD_NOT_FOUND');
    }
    eq('eventsUrl adds after only when positive', [P.api.eventsUrl('ccp_1', 0), P.api.eventsUrl('ccp_1', 42)],
        ['/api/desktop/cyd-creator/projects/ccp_1/events', '/api/desktop/cyd-creator/projects/ccp_1/events?after=42']);
    eq('cancelJob accepts the 202 {"job"} answer', await P.api.cancelJob('ccj_1'), true);
    eq('cancelJob posts an empty JSON object', [seen[seen.length - 1].url, seen[seen.length - 1].method, seen[seen.length - 1].body], ['/api/desktop/cyd-creator/jobs/ccj_1/cancel', 'POST', '{}']);
    eq('discardDraft accepts 204', await P.api.discardDraft('ccp_1'), true);
    eq('discardDraft path', seen[seen.length - 1].url, '/api/desktop/cyd-creator/projects/ccp_1/draft/discard');
}

// ── autosave ──
function makeSaver(answers, extra) {
    const calls = [];
    const states = [];
    const saver = P.autosave.create(Object.assign({
        token: 1,
        timers: { setTimeout: fakeSetTimeout, clearTimeout: fakeClearTimeout },
        save: (payload, token) => {
            calls.push({ payload, token });
            const answer = answers.shift();
            if (!answer) return Promise.resolve(token + 1);
            if (answer.deferred) return answer.deferred;
            if (answer.error) return Promise.reject(answer.error);
            return Promise.resolve(answer.token);
        },
        onState: s => states.push(s)
    }, extra || {}));
    return { saver, calls, states };
}
{
    const { saver, calls, states } = makeSaver([]);
    saver.change('a');
    saver.change('b');
    eq('a change marks the document dirty', saver.state(), 'dirty');
    await advance(999);
    eq('nothing is saved before the delay', calls.length, 0);
    await advance(1);
    eq('one save with the latest payload and token', calls, [{ payload: 'b', token: 1 }]);
    eq('saved with the new token', [saver.state(), saver.token(), saver.dirty()], ['saved', 2, false]);
    eq('state sequence', states, ['dirty', 'saving', 'saved']);
}
{
    let release;
    const deferred = new Promise(r => { release = r; });
    const { saver, calls } = makeSaver([{ deferred }, { token: 9 }]);
    saver.change('a');
    await advance(1000);
    saver.change('b');
    await advance(5000);
    eq('a change during a save waits for it', calls.length, 1);
    release(5);
    await settle();
    eq('the newer payload follows at once with the new token', calls.map(c => [c.payload, c.token]), [['a', 1], ['b', 5]]);
    eq('then saved', [saver.state(), saver.token()], ['saved', 9]);
}
{
    const { saver, calls } = makeSaver([{ error: new ApiError(0, '', 'offline') }, { error: new ApiError(503, 'CYD_BUSY', 'busy') }, { token: 7 }]);
    saver.change('a');
    await advance(1000);
    eq('a network error goes offline', saver.state(), 'offline');
    await advance(4999);
    eq('the retry waits 5 s', calls.length, 1);
    await advance(1);
    eq('first retry', calls.length, 2);
    await advance(9999);
    eq('the second retry waits 10 s', calls.length, 2);
    await advance(1);
    eq('second retry saves', [calls.length, saver.state(), saver.token()], [3, 'saved', 7]);
}
{
    const { saver, calls } = makeSaver([{ error: new ApiError(400, 'CYD_BAD_REQUEST', 'bad') }, { token: 3 }]);
    saver.change('a');
    await advance(1000);
    eq('a permanent error fails without retry', saver.state(), 'failed');
    await advance(120000);
    eq('no automatic retry after failed', calls.length, 1);
    saver.change('b');
    await advance(1000);
    eq('the next change saves again', [calls.length, saver.state()], [2, 'saved']);
}
{
    const conflicts = [];
    const { saver, calls } = makeSaver([{ error: new ApiError(409, 'CYD_PROJECT_CONFLICT', 'stale', { version: 20 }) }, { token: 21 }], {
        onConflict: err => { conflicts.push(err.details.version); return Promise.resolve({ action: 'overwrite', token: err.details.version }); }
    });
    saver.change('mine');
    await advance(1000);
    eq('a conflict asks once and overwrites with the server version', [conflicts, calls.map(c => c.token), saver.state(), saver.token()], [[20], [1, 20], 'saved', 21]);
}
{
    const { saver, calls } = makeSaver([{ error: new ApiError(409, 'CYD_FILE_CONFLICT', 'stale') }], {
        onConflict: () => Promise.resolve({ action: 'reload', token: 'server-sha' })
    });
    saver.change('mine');
    await advance(1000);
    eq('reload drops the local change and takes the server token', [calls.length, saver.state(), saver.dirty(), saver.token()], [1, 'saved', false, 'server-sha']);
}
{
    let asked = 0;
    const { saver, calls } = makeSaver([{ error: new ApiError(409, 'CYD_FILE_CONFLICT', 'stale') }, { error: new ApiError(409, 'CYD_FILE_CONFLICT', 'stale') }], {
        onConflict: () => { asked++; return Promise.resolve({ action: 'keep' }); }
    });
    saver.change('mine');
    await advance(1000);
    eq('keep stays in conflict', saver.state(), 'conflict');
    saver.change('more');
    await advance(5000);
    eq('a kept conflict does not autosave', calls.length, 1);
    await saver.flush();
    eq('flush tries again and asks again', [calls.length, asked, saver.state()], [2, 2, 'conflict']);
}
{
    const { saver, calls } = makeSaver([{ error: new ApiError(423, 'CYD_PROJECT_LOCKED', 'locked') }, { token: 4 }]);
    saver.change('a');
    await advance(1000);
    eq('423 parks the saver', saver.state(), 'locked');
    saver.change('b');
    await advance(5000);
    eq('no save while locked', calls.length, 1);
    eq('flush reports unsaved while locked', await saver.flush(), false);
    saver.unlock();
    await settle();
    eq('unlock saves the latest payload', [calls.length, calls[1].payload, saver.state()], [2, 'b', 'saved']);
}
{
    const { saver, calls } = makeSaver([]);
    saver.change('a');
    const ok = await saver.flush();
    eq('flush saves at once', [ok, calls.length, saver.state()], [true, 1, 'saved']);
    eq('flush of a clean document', await saver.flush(), true);
    saver.change('b');
    saver.reset(99);
    await advance(2000);
    eq('reset drops the pending change', [calls.length, saver.token(), saver.dirty()], [1, 99, false]);
}

// ── events: cursor, backoff, catch-up ──
{
    const c = P.events.createCursor(0);
    eq('cursor accepts new seqs and drops replays', [c.accept('1'), c.accept('2'), c.accept('2'), c.accept('1'), c.accept(''), c.accept('3')], [true, true, false, false, true, true]);
    eq('cursor after', c.after, 3);
    c.reset(1);
    eq('cursor reset may move it down', [c.after, c.accept('2')], [1, true]);
    eq('backoff doubles to 30 s', [0, 1, 2, 3, 4, 5, 9].map(P.events.backoff), [1000, 2000, 4000, 8000, 16000, 30000, 30000]);
}
{
    const cu = P.events.createCatchup();
    cu.add('job_status', { job: { id: 'j1', status: 'running' } });
    cu.add('project_changed', { version: 3, what: 'layout' });
    cu.add('job_step', { job_id: 'j0', text: 'old' });
    cu.add('job_step', { job_id: 'j1', text: 'one' });
    cu.add('project_changed', { version: 5, what: 'layout' });
    cu.add('project_changed', { version: 4, what: 'files' });
    cu.add('job_message', { message: { id: 'm1' } });
    cu.add('lock_changed', { locked_by_job: 'j1' });
    cu.add('job_status', { job: { id: 'j1', status: 'succeeded' } });
    cu.add('lock_changed', { locked_by_job: '' });
    eq('the backlog collapses to the latest state', cu.drain().map(p => p[0] + ':' + JSON.stringify(p[1])), [
        'lock_changed:{"locked_by_job":""}',
        'project_changed:{"version":5,"what":"layout"}',
        'project_changed:{"version":4,"what":"files"}',
        'job_status:{"job":{"id":"j1","status":"succeeded"}}',
        'job_message:{"message":{"id":"m1"}}',
        'job_step:{"job_id":"j1","text":"one"}'
    ]);
}
{
    const streams = [];
    class FakeES {
        constructor(url) { this.url = url; this.listeners = {}; this.closed = false; this.onerror = null; streams.push(this); }
        addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
        close() { this.closed = true; }
        send(type, data, id) { (this.listeners[type] || []).forEach(fn => fn({ data: JSON.stringify(data), lastEventId: id ? String(id) : '' })); }
    }
    const got = [];
    const bus = P.createBus();
    P.events.TYPES.forEach(type => bus.on('sse:' + type, data => got.push(type + ':' + JSON.stringify(data))));
    const states = [];
    bus.on('events:state', s => states.push(s));
    const conn = P.events.connect({ projectId: 'ccp_1', bus }, { EventSource: FakeES });
    eq('the first stream starts without after', streams[0].url, '/api/desktop/cyd-creator/projects/ccp_1/events');
    streams[0].send('job_status', { job: { id: 'j1', status: 'running' } }, 1);
    streams[0].send('job_status', { job: { id: 'j1', status: 'awaiting_confirmation' } }, 2);
    eq('the backlog waits for stream_ready', got.length, 0);
    streams[0].send('stream_ready', {}, 0);
    eq('stream_ready replays the coalesced backlog, then announces itself', got, ['job_status:{"job":{"id":"j1","status":"awaiting_confirmation"}}', 'stream_ready:{}']);
    streams[0].send('build_log', { build_id: 'b1', lines: ['x'] }, 0);
    streams[0].send('job_step', { job_id: 'j1', text: 'live' }, 3);
    eq('live events pass through', got.slice(2), ['build_log:{"build_id":"b1","lines":["x"]}', 'job_step:{"job_id":"j1","text":"live"}']);
    streams[0].onerror();
    eq('an error closes the stream', [streams[0].closed, states[states.length - 1]], [true, 'reconnecting']);
    await advance(999);
    eq('the reconnect waits 1 s', streams.length, 1);
    await advance(1);
    eq('the reconnect resumes after the last seq', streams[1].url, '/api/desktop/cyd-creator/projects/ccp_1/events?after=3');
    streams[1].send('job_step', { job_id: 'j1', text: 'live' }, 3);
    streams[1].send('job_step', { job_id: 'j1', text: 'next' }, 4);
    streams[1].onerror();
    await advance(2000);
    // The next stream resumes from the same point as the interrupted one, so the server can still
    // detect pruned events and answer stream_ready {"reload":true}; the replay is dropped by seq.
    eq('an interrupted backlog is kept and the next stream resumes from the same point', streams[2].url, '/api/desktop/cyd-creator/projects/ccp_1/events?after=3');
    streams[2].send('job_step', { job_id: 'j1', text: 'next' }, 4);
    streams[2].send('stream_ready', {}, 0);
    eq('replayed seqs are dropped, the kept backlog is delivered once', got.slice(4), ['job_step:{"job_id":"j1","text":"next"}', 'stream_ready:{}']);
    eq('state follows the stream', states, ['connecting', 'live', 'reconnecting', 'reconnecting', 'reconnecting', 'reconnecting', 'live']);
    conn.close();
    eq('close closes the source', streams[2].closed, true);
}

// ── events against the real server and a real EventSource ──
// A browser EventSource keeps lastEventId from the previous event when an event has no `id:` line
// (the server sends build_log and stream_ready without one), and stream_ready may carry
// {"reload":true} (plan 2 Task 13: events after the resume point were pruned, or the resume point
// lies above the project's newest event after a database restore).
const browserStreams = [];
class BrowserES {
    constructor(url) { this.url = url; this.listeners = {}; this.closed = false; this.onerror = null; this.lastEventId = ''; browserStreams.push(this); }
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
    close() { this.closed = true; }
    send(type, data, id) {
        if (id !== undefined) this.lastEventId = String(id);
        const ev = { data: JSON.stringify(data), lastEventId: this.lastEventId };
        (this.listeners[type] || []).forEach(fn => fn(ev));
    }
}
function browserConnect(after) {
    browserStreams.length = 0;
    const got = [];
    const bus = P.createBus();
    P.events.TYPES.forEach(type => bus.on('sse:' + type, (data, meta) => got.push(type + ':' + JSON.stringify(data) + '@' + meta.seq)));
    const conn = P.events.connect({ projectId: 'ccp_1', bus }, { EventSource: BrowserES, after });
    return { got, conn, streams: browserStreams };
}
{
    const { got, conn, streams } = browserConnect(0);
    streams[0].send('project_changed', { version: 2, what: 'layout' }, 7);
    streams[0].send('stream_ready', {});
    streams[0].send('build_log', { build_id: 'b1', lines: ['a'] });
    streams[0].send('build_log', { build_id: 'b1', lines: ['b'] });
    streams[0].send('job_step', { job_id: 'j1', text: 'x' }, 8);
    streams[0].send('build_log', { build_id: 'b1', lines: ['c'] });
    eq('events without an id line arrive although the browser keeps lastEventId', got, [
        'project_changed:{"version":2,"what":"layout"}@0',
        'stream_ready:{}@0',
        'build_log:{"build_id":"b1","lines":["a"]}@0',
        'build_log:{"build_id":"b1","lines":["b"]}@0',
        'job_step:{"job_id":"j1","text":"x"}@8',
        'build_log:{"build_id":"b1","lines":["c"]}@0'
    ]);
    eq('the cursor follows id lines only', conn.after, 8);
    conn.close();
}
{
    const { got, conn, streams } = browserConnect(3);
    eq('a resumed window asks for the events after its cursor', streams[0].url, '/api/desktop/cyd-creator/projects/ccp_1/events?after=3');
    // Events 4..9 were pruned; the stream is cut off before stream_ready.
    streams[0].send('job_message', { message: { id: 'm10' } }, 10);
    streams[0].onerror();
    await advance(1000);
    eq('the cut-off catch-up resumes from the same point', streams[1].url, '/api/desktop/cyd-creator/projects/ccp_1/events?after=3');
    streams[1].send('job_message', { message: { id: 'm10' } }, 10);
    streams[1].send('job_status', { job: { id: 'j2', status: 'running' } }, 11);
    streams[1].send('stream_ready', { reload: true });
    eq('a pruned resume point delivers the backlog once, then stream_ready with reload', got, [
        'job_status:{"job":{"id":"j2","status":"running"}}@0',
        'job_message:{"message":{"id":"m10"}}@0',
        'stream_ready:{"reload":true}@0'
    ]);
    eq('after a pruned reload the cursor is the last event sent', conn.after, 11);
    conn.close();
}
{
    const { got, conn, streams } = browserConnect(900);
    // The database was restored: its newest event is 40, so the server continues from there.
    streams[0].send('lock_changed', { locked_by_job: '' }, 41);
    streams[0].send('stream_ready', { reload: true });
    eq('stream_ready reload arrives although the backlog lies below the old cursor', got, ['stream_ready:{"reload":true}@0']);
    eq('reload moves the cursor down to the last event the server sent', conn.after, 41);
    streams[0].send('job_step', { job_id: 'j3', text: 'live' }, 42);
    eq('live events after the reload are not dropped as replays', got.slice(1), ['job_step:{"job_id":"j3","text":"live"}@42']);
    streams[0].onerror();
    await advance(1000);
    eq('the next reconnect resumes after the new cursor', streams[1].url, '/api/desktop/cyd-creator/projects/ccp_1/events?after=42');
    conn.close();
}
{
    const { got, conn, streams } = browserConnect(900);
    streams[0].send('stream_ready', { reload: true });
    eq('a reload without any event forgets the old cursor', conn.after, 0);
    streams[0].send('build_status', { build: { id: 'ccb_1', status: 'queued' } }, 5);
    eq('the next persisted event is accepted', [got.length, conn.after], [2, 5]);
    conn.close();
}

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('all CYD Creator shell checks passed');
