#!/usr/bin/env node
// Runs the pure parts of the CYD Creator shell (cyd-creator-core.js, -api.js, -events.js) in Node:
// P.util, P.i18n, P.errors, the API error envelope and request(), the autosave/conflict state
// machine with a fake clock, and the SSE cursor, backoff, backlog coalescing and reconnect logic;
// the dock (cyd-creator-dock.js) runs on a minimal fake DOM.
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
    'cyd_creator.error.publication_review': 'Prüfung nötig',
    'cyd_creator.error.unauthorized': 'Abgemeldet',
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
// CYD_BUSY on a save: file_open (an open file blocked a rename) is retried; the publication fence
// (publication_review) lasts until an administrator resolves it, so the saver must not retry.
eq('CYD_BUSY file_open or without a reason retries', [
    P.autosave.classify(new ApiError(409, 'CYD_BUSY', 'open', { reason: 'file_open' })),
    P.autosave.classify(new ApiError(409, 'CYD_BUSY', 'busy'))
], ['offline', 'offline']);
eq('CYD_BUSY publication_review fails instead of retrying', P.autosave.classify(new ApiError(409, 'CYD_BUSY', 'fenced', { reason: 'publication_review' })), 'failed');
eq('CYD_BUSY with an unknown reason fails', P.autosave.classify(new ApiError(409, 'CYD_BUSY', 'x', { reason: 'something_new' })), 'failed');
eq('errors.message: the publication fence has its own text', P.errors.message(t, new ApiError(409, 'CYD_BUSY', 'fenced', { reason: 'publication_review' })), 'Prüfung nötig');
eq('errors.message: CYD_BUSY file_open keeps the CYD_BUSY text', P.errors.message(t, new ApiError(409, 'CYD_BUSY', 'open', { reason: 'file_open' })), 'Beschäftigt');

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
    // AuraGo's desktop auth/policy answers carry the code in "error" (no "code" field).
    const scope = toError(403, { error: 'desktop_scope_required', message: 'Desktop API scope is required for this endpoint.' });
    eq('toError takes the code from error when code is missing', [scope.code, scope.message], ['desktop_scope_required', 'Desktop API scope is required for this endpoint.']);
    const login = toError(401, { error: 'unauthorized', redirect: '/auth/login' });
    eq('an expired session is code unauthorized', [login.code, login.message, P.errors.message(t, login)], ['unauthorized', 'unauthorized', 'Abgemeldet']);
    const policy = toError(403, { error: 'desktop_readonly', code: 'desktop_readonly', message: 'The desktop does not allow this action.' });
    eq('the desktop policy answer keeps its code', [policy.code, P.autosave.classify(policy)], ['desktop_readonly', 'failed']);
    eq('a sentence in error is not a code', toError(500, { error: 'something went wrong' }).code, '');
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
    const { saver, calls } = makeSaver([{ error: new ApiError(409, 'CYD_BUSY', 'fenced', { reason: 'publication_review' }) }]);
    saver.change('a');
    await advance(1000);
    await advance(120000);
    eq('a fenced project fails once and is not retried', [saver.state(), calls.length, saver.error().details.reason], ['failed', 1, 'publication_review']);
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
    const conn = P.events.connect({ projectId: 'ccp_1', bus }, { EventSource: FakeES, random: () => 0.5 });
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
    send(type, data, id) { this.sendRaw(type, JSON.stringify(data), id); }
    sendRaw(type, raw, id) {
        if (id !== undefined) this.lastEventId = String(id);
        const ev = { data: raw, lastEventId: this.lastEventId };
        (this.listeners[type] || []).forEach(fn => fn(ev));
    }
}
function browserConnect(after) {
    browserStreams.length = 0;
    const got = [];
    const bus = P.createBus();
    P.events.TYPES.forEach(type => bus.on('sse:' + type, (data, meta) => got.push(type + ':' + JSON.stringify(data) + '@' + meta.seq)));
    const conn = P.events.connect({ projectId: 'ccp_1', bus }, { EventSource: BrowserES, after, random: () => 0.5 });
    return { got, conn, streams: browserStreams };
}
{
    const { got, conn, streams } = browserConnect(0);
    streams[0].send('project_changed', { version: 2, what: 'layout' }, 7);
    streams[0].send('stream_ready', { after: 7 });
    streams[0].send('build_log', { build_id: 'b1', lines: ['a'] });
    streams[0].send('build_log', { build_id: 'b1', lines: ['b'] });
    streams[0].send('job_step', { job_id: 'j1', text: 'x' }, 8);
    streams[0].send('build_log', { build_id: 'b1', lines: ['c'] });
    eq('events without an id line arrive although the browser keeps lastEventId', got, [
        'project_changed:{"version":2,"what":"layout"}@0',
        'stream_ready:{"after":7}@0',
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
    streams[1].send('stream_ready', { reload: true, after: 11 });
    eq('a pruned resume point delivers the backlog once, then stream_ready with reload', got, [
        'job_status:{"job":{"id":"j2","status":"running"}}@0',
        'job_message:{"message":{"id":"m10"}}@0',
        'stream_ready:{"reload":true,"after":11}@0'
    ]);
    eq('after a pruned reload the cursor is the last event sent', conn.after, 11);
    conn.close();
}
{
    const { got, conn, streams } = browserConnect(900);
    // The database was restored: its newest event is 40, so the server continues from there.
    streams[0].send('lock_changed', { locked_by_job: '' }, 41);
    streams[0].send('stream_ready', { reload: true, after: 41 });
    eq('stream_ready reload arrives although the backlog lies below the old cursor', got, ['stream_ready:{"reload":true,"after":41}@0']);
    eq('reload moves the cursor down to the last event the server sent', conn.after, 41);
    streams[0].send('job_step', { job_id: 'j3', text: 'live' }, 42);
    eq('live events after the reload are not dropped as replays', got.slice(1), ['job_step:{"job_id":"j3","text":"live"}@42']);
    streams[0].onerror();
    await advance(1000);
    eq('the next reconnect resumes after the new cursor', streams[1].url, '/api/desktop/cyd-creator/projects/ccp_1/events?after=42');
    conn.close();
}
{
    // No event since the restore: stream_ready's after (the newest event, 40) is the cursor, so a
    // reconnect does not replay the whole backlog.
    const { got, conn, streams } = browserConnect(900);
    streams[0].send('stream_ready', { reload: true, after: 40 });
    eq('a reload without any event takes the server continuation point', conn.after, 40);
    streams[0].send('build_status', { build: { id: 'ccb_1', status: 'queued' } }, 41);
    eq('the next persisted event is accepted', [got.length, conn.after], [2, 41]);
    streams[0].onerror();
    await advance(1000);
    eq('the reconnect after a reload resumes after the server position', streams[1].url, '/api/desktop/cyd-creator/projects/ccp_1/events?after=41');
    conn.close();
}
{
    const { got, conn, streams } = browserConnect(900);
    streams[0].send('stream_ready', { reload: true });
    eq('a reload without after and without any event forgets the old cursor', conn.after, 0);
    streams[0].send('build_status', { build: { id: 'ccb_1', status: 'queued' } }, 5);
    eq('then the next persisted event is accepted', [got.length, conn.after], [2, 5]);
    conn.close();
}
{
    const { got, conn, streams } = browserConnect(0);
    streams[0].send('project_changed', { version: 2, what: 'layout' }, 5);
    streams[0].send('stream_ready', { after: 5 });
    streams[0].sendRaw('job_step', '{"job_id":"j1",', 6);
    eq('a malformed frame is dropped without moving the cursor', [got.length, conn.after], [2, 5]);
    streams[0].send('build_log', { build_id: 'b1', lines: ['a'] });
    streams[0].send('job_step', { job_id: 'j1', text: 'ok' }, 7);
    eq('the stream goes on after a malformed frame', got.slice(2), ['build_log:{"build_id":"b1","lines":["a"]}@0', 'job_step:{"job_id":"j1","text":"ok"}@7']);
    conn.close();
}
{
    // Reconnect delays: the backoff ±20 % (jitter), kept within 1–30 s.
    async function reconnectDelays(random, count) {
        browserStreams.length = 0;
        const conn = P.events.connect({ projectId: 'ccp_1', bus: P.createBus() }, { EventSource: BrowserES, random });
        const out = [];
        for (let i = 0; i < count; i++) {
            browserStreams[browserStreams.length - 1].onerror();
            const delay = Array.from(timers.values()).pop().at - now;
            out.push(delay);
            await advance(delay);
        }
        conn.close();
        return out;
    }
    eq('jitter -20 % never goes below 1 s', await reconnectDelays(() => 0, 3), [1000, 1600, 3200]);
    eq('jitter +20 % never goes above 30 s', await reconnectDelays(() => 1, 7), [1200, 2400, 4800, 9600, 19200, 30000, 30000]);
    const spread = await reconnectDelays(undefined, 4);
    check('the default jitter stays within ±20 % of the backoff', spread.every((d, i) => d >= Math.max(1000, 0.8 * 1000 * 2 ** i) && d <= 1.2 * 1000 * 2 ** i), JSON.stringify(spread));
}

// ── dock (cyd-creator-dock.js) on a minimal fake DOM: only what the dock and P.util.el touch ──
class FakeNode {
    constructor(tag, text) {
        this.tagName = String(tag).toUpperCase();
        this.childNodes = [];
        this.parentNode = null;
        this.attrs = {};
        this.dataset = {};
        this.listeners = {};
        this.ownText = text || '';
        this.checkedProp = undefined;
        this.scrollTop = 0;
        this.clientHeight = 100;
        const classes = new Set();
        this.classList = {
            toggle(name, on) { const want = on === undefined ? !classes.has(name) : !!on; if (want) classes.add(name); else classes.delete(name); return want; },
            contains: name => classes.has(name)
        };
        this.style = { props: {}, setProperty(name, value) { this.props[name] = String(value); } };
    }
    get children() { return this.childNodes; }
    get firstChild() { return this.childNodes[0] || null; }
    get nextSibling() { return this.parentNode ? this.parentNode.childNodes[this.parentNode.childNodes.indexOf(this) + 1] || null : null; }
    get hidden() { return 'hidden' in this.attrs; }
    set hidden(on) { if (on) this.attrs.hidden = ''; else delete this.attrs.hidden; }
    get disabled() { return 'disabled' in this.attrs; }
    set disabled(on) { if (on) this.attrs.disabled = ''; else delete this.attrs.disabled; }
    get checked() { return this.checkedProp === undefined ? 'checked' in this.attrs : this.checkedProp; }
    set checked(on) { this.checkedProp = !!on; }
    get scrollHeight() { return this.childNodes.length * 10; }
    get textContent() { return this.ownText + this.childNodes.map(n => n.textContent).join(''); }
    set textContent(value) { this.replaceChildren(); this.ownText = String(value); }
    setAttribute(name, value) { this.attrs[name] = String(value); }
    getAttribute(name) { return name in this.attrs ? this.attrs[name] : null; }
    removeAttribute(name) { delete this.attrs[name]; }
    appendChild(node) { return this.insertBefore(node, null); }
    insertBefore(node, ref) {
        const nodes = node.tagName === '#FRAGMENT' ? node.childNodes.slice() : [node];
        nodes.forEach(n => {
            if (n.parentNode) n.parentNode.removeChild(n);
            const at = ref ? this.childNodes.indexOf(ref) : -1;
            if (at < 0) this.childNodes.push(n); else this.childNodes.splice(at, 0, n);
            n.parentNode = this;
        });
        return node;
    }
    removeChild(node) {
        const at = this.childNodes.indexOf(node);
        if (at >= 0) this.childNodes.splice(at, 1);
        node.parentNode = null;
        return node;
    }
    remove() { if (this.parentNode) this.parentNode.removeChild(this); }
    replaceChildren(...nodes) {
        this.childNodes.slice().forEach(n => this.removeChild(n));
        nodes.forEach(n => this.appendChild(n));
    }
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
    removeEventListener(type, fn) { this.listeners[type] = (this.listeners[type] || []).filter(f => f !== fn); }
    dispatch(type, extra) { (this.listeners[type] || []).slice().forEach(fn => fn(Object.assign({ preventDefault() {}, button: 0 }, extra))); }
    querySelectorAll(selector) {
        if (selector !== '[role="tab"]') throw new Error('fake DOM: unsupported selector ' + selector);
        const out = [];
        const walk = node => node.childNodes.forEach(n => { if (n.attrs.role === 'tab') out.push(n); walk(n); });
        walk(this);
        return out;
    }
    focus() {}
    setPointerCapture() {}
}
const fakeStorage = new Map();
sandbox.document = {
    createElement: tag => new FakeNode(tag),
    createTextNode: text => new FakeNode('#text', String(text)),
    createDocumentFragment: () => new FakeNode('#fragment')
};
sandbox.localStorage = { getItem: key => (fakeStorage.has(key) ? fakeStorage.get(key) : null), setItem: (key, value) => fakeStorage.set(key, String(value)) };
{
    const full = path.join(apps, 'cyd-creator-dock.js');
    vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
}
const DOCK_PREFS = 'aurago.cydcreator.dock.v1';
function findNode(node, pred) {
    if (pred(node)) return node;
    for (const child of node.childNodes) {
        const hit = findNode(child, pred);
        if (hit) return hit;
    }
    return null;
}
function dockBuild(id, status, minute) {
    return { id, status, created_at: '2026-10-12T10:' + String(minute).padStart(2, '0') + ':00.000Z', diagnostics: [] };
}
// makeDock creates a dock with stored prefs and loads its builds like the app; GET /builds
// answers when release() is called.
function makeDock(prefs, listed, logs) {
    fakeStorage.clear();
    if (prefs) fakeStorage.set(DOCK_PREFS, JSON.stringify(prefs));
    const calls = { log: [] };
    let release;
    const listedPromise = new Promise(resolve => { release = () => resolve(listed || []); });
    const inst = {
        t: P.i18n({}), bus: P.createBus(), projectId: 'ccp_1', root: { clientHeight: 1000 }, domId: s => 'cc1-' + s,
        api: {
            listBuilds: () => listedPromise,
            buildLog: id => { calls.log.push(id); return Promise.resolve((logs || {})[id] || ''); },
            buildLogUrl: id => '/log/' + id,
            cancelBuild: () => Promise.resolve(true)
        },
        reveal() {}, openFile() {}, notify() {}
    };
    const dock = P.dock.create(inst);
    dock.builds.load();
    const pre = () => findNode(dock.el, n => n.dataset.ccLog !== undefined);
    return { dock, inst, calls, release, pre, prefs: () => JSON.parse(fakeStorage.get(DOCK_PREFS) || '{}') };
}
{
    const d = makeDock({ collapsed: true, tab: 'problems', h: 300 });
    eq('a collapsed dock opens collapsed on its saved tab', [d.dock.el.classList.contains('is-collapsed'), d.dock.current(), d.prefs().collapsed], [true, 'problems', true]);
    const handle = d.dock.el.childNodes[0];
    eq('the separator states its range', [handle.getAttribute('aria-valuemin'), handle.getAttribute('aria-valuemax'), handle.getAttribute('aria-valuenow')], ['120', '700', '300']);
    d.dock.show('log');
    eq('a tab the user shows expands the dock', [d.dock.el.classList.contains('is-collapsed'), d.prefs().collapsed, d.prefs().tab], [false, false, 'log']);
    d.dock.dispose();
}
{
    const d = makeDock({ tab: 'monitor' });
    eq('the log stands in for an unregistered saved tab without replacing it', [d.dock.current(), d.prefs().tab], ['log', 'monitor']);
    d.dock.registerTab('monitor', { labelKey: 'cyd_creator.dock.log', mount() {} });
    eq('the saved tab is shown once it registers', [d.dock.current(), d.prefs().tab], ['monitor', 'monitor']);
    d.dock.dispose();
}
{
    // A new window replays build_status of builds the retention deleted (events outlive builds).
    const d = makeDock(null, [dockBuild('cb_5', 'succeeded', 5), dockBuild('cb_4', 'failed', 4)], { cb_5: 'five\n', cb_6: 'six stored\n' });
    d.inst.bus.emit('build:status', dockBuild('cb_1', 'succeeded', 1));
    d.inst.bus.emit('build:status', dockBuild('cb_6', 'running', 6));
    eq('before GET /builds a finished unknown build waits, an active one is shown', d.dock.builds.list().map(x => x.id), ['cb_6']);
    d.release();
    await settle();
    eq('the list merges the active build and drops the pruned one', [d.dock.builds.list().map(x => x.id), d.dock.builds.selected().id], [['cb_6', 'cb_5', 'cb_4'], 'cb_6']);
    d.inst.bus.emit('build:status', dockBuild('cb_2', 'failed', 2));
    eq('after GET /builds an older unknown finished build is ignored', d.dock.builds.list().length, 3);
    d.dock.trackBuild(dockBuild('cb_6', 'queued', 6));
    eq('a late POST answer does not overwrite a running build', d.dock.builds.selected().status, 'running');
    d.inst.bus.emit('sse:build_log', { build_id: 'cb_6', lines: ['live', '[… 3 lines skipped …]'] });
    const download = findNode(d.dock.el, n => n.dataset.ccLogAction === 'download');
    const logBadge = findNode(d.dock.el, n => n.dataset.ccDockTab === 'log').childNodes[1];
    eq('the download names the build and the running dot is decorative', [download.download, logBadge.getAttribute('aria-hidden')], ['cb_6-build.log', 'true']);
    d.calls.log.length = 0;
    d.inst.bus.emit('sse:stream_ready', {});
    await settle();
    eq('every stream_ready re-reads a running build', d.calls.log, ['cb_6']);
    d.inst.bus.emit('build:status', Object.assign(dockBuild('cb_6', 'succeeded', 6), { diagnostics: [{ severity: 'warning', message: 'w', file: 'src/app/app.cpp', line: 3 }] }));
    await settle();
    eq('a finished build replaces its lossy live lines with the stored log', d.pre().textContent, 'six stored\n');
    d.dock.builds.select('cb_5');
    d.inst.bus.emit('build:status', dockBuild('cb_0', 'failed', 0));
    await settle();
    eq('a pruned build cannot cancel the selected build\'s log', [d.dock.builds.selected().id, d.pre().textContent], ['cb_5', 'five\n']);
    d.dock.dispose();
}
{
    const d = makeDock(null, [dockBuild('cb_7', 'running', 7)], {});
    d.release();
    await settle();
    d.inst.bus.emit('sse:build_log', { build_id: 'cb_7', lines: ['kept'] });
    d.inst.bus.emit('build:status', dockBuild('cb_7', 'interrupted', 7));
    await settle();
    eq('an empty stored log keeps the live lines', d.pre().textContent, 'kept\n');
    const lines = Array.from({ length: 4995 }, (_, i) => 'line ' + i);
    d.inst.bus.emit('build:status', dockBuild('cb_8', 'running', 8));
    d.inst.bus.emit('sse:build_log', { build_id: 'cb_8', lines });
    const last = d.pre().childNodes[d.pre().childNodes.length - 1];
    d.inst.bus.emit('sse:build_log', { build_id: 'cb_8', lines: Array.from({ length: 10 }, (_, i) => 'more ' + i) });
    const nodes = d.pre().childNodes;
    eq('the ring drops the oldest line nodes and keeps the note first', [nodes.length, nodes[0].textContent, nodes[1].textContent, nodes[nodes.length - 1].textContent],
        [5001, 'cyd_creator.dock.log_truncated\n', 'line 5\n', 'more 9\n']);
    check('the ring does not rebuild the kept lines', last.parentNode === d.pre());
    d.dock.show('problems');
    d.pre().scrollTop = 0;
    d.dock.show('log');
    check('showing the log while following scrolls to its end', d.pre().scrollTop === d.pre().scrollHeight && d.pre().scrollTop > 0);
    d.dock.dispose();
}

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('all CYD Creator shell checks passed');
