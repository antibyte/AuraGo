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
eq('t fills in one pass: a value is never read as a placeholder', t('home.hello', { name: '{{name}} {name}' }), 'Hallo {{name}} {name}, {{name}} {name}!');
eq('t keeps placeholders it has no value for', t('designer.added', { other: 'x' }), '{name} hinzugefügt');
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
    // Ctrl+S or closing the window while the conflict choice is open: flush waits for the choice
    // instead of sending the stale version again (a second, stacked conflict dialog).
    let answer;
    let asked = 0;
    const { saver, calls } = makeSaver([{ error: new ApiError(409, 'CYD_PROJECT_CONFLICT', 'stale', { version: 8 }) }, { token: 9 }], {
        onConflict: () => { asked++; return new Promise(resolve => { answer = resolve; }); }
    });
    saver.change('mine');
    await advance(1000);
    const first = saver.flush();
    const second = saver.flush();
    await settle();
    eq('flush during an open conflict choice sends nothing and asks nothing', [calls.length, asked, first === second], [1, 1, true]);
    answer({ action: 'overwrite', token: 8 });
    eq('both flushes end with the outcome of the choice', [await first, await second, calls.map(c => c.token), saver.state()], [true, true, [1, 8], 'saved']);
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
    // A new connection after a cut-off catch-up must start where the catch-up started: the cursor
    // already counts the buffered events, which were never delivered.
    const { conn, streams } = browserConnect(3);
    streams[0].send('job_message', { message: { id: 'm4' } }, 4);
    streams[0].send('job_message', { message: { id: 'm5' } }, 5);
    eq('during a catch-up resumeFrom stays at its start while the cursor moves', [conn.after, conn.resumeFrom], [5, 3]);
    streams[0].send('stream_ready', { after: 5 });
    eq('once the backlog is delivered resumeFrom follows the cursor', conn.resumeFrom, 5);
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
            add: name => classes.add(name),
            remove: name => classes.delete(name),
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
    get open() { return this.openProp === undefined ? 'open' in this.attrs : this.openProp; }
    set open(on) { this.openProp = !!on; }
    get value() { return this.valueProp === undefined ? (this.attrs.value || '') : this.valueProp; }
    set value(v) { this.valueProp = String(v); }
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
    // Simple selectors only: tag, [attr], [attr="v"], tag[attr], :not([tabindex="-1"]), :checked, comma lists.
    matches(selector) {
        return selector.split(',').some(part => {
            const m = /^([a-z]+)?(?:\[([a-z-]+)(?:="([^"]*)")?\])?(:not\(\[tabindex="-1"\]\))?(:checked)?$/.exec(part.trim());
            if (!m) throw new Error('fake DOM: unsupported selector ' + part);
            const [, tag, attr, want, notMinus, checked] = m;
            if (tag && this.tagName !== tag.toUpperCase()) return false;
            if (checked && !this.checked) return false;
            if (!attr) return true;
            let value;
            if (attr.startsWith('data-')) value = this.dataset[attr.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase())];
            else if (attr === 'open') value = this.open ? '' : undefined;
            else value = this.attrs[attr];
            if (value === undefined) return false;
            if (notMinus && value === '-1') return false;
            return want === undefined || value === want;
        });
    }
    querySelectorAll(selector) {
        const out = [];
        const walk = node => node.childNodes.forEach(n => { if (n.tagName !== '#TEXT' && n.matches(selector)) out.push(n); walk(n); });
        walk(this);
        return out;
    }
    querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
    contains(node) { for (let n = node; n; n = n.parentNode) if (n === this) return true; return false; }
    getClientRects() { return this.hidden ? [] : [{}]; }
    getBoundingClientRect() { return { top: 0, left: 0, width: 600, height: 400 }; }
    focus() { sandbox.document.activeElement = this; }
    click() { this.dispatch('click'); }
    select() {}
    setSelectionRange() {}
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
{
    // reloadProject loads again after stream_ready {"reload":true} (a restored database, pruned
    // events): the server list replaces the window's builds, except builds reported meanwhile.
    const d = makeDock(null, [dockBuild('cb_9', 'running', 9), dockBuild('cb_8', 'succeeded', 8)], {});
    d.release();
    await settle();
    d.inst.bus.emit('build:status', dockBuild('cb_10', 'succeeded', 10));
    d.dock.builds.select('cb_9');
    d.inst.api.listBuilds = () => Promise.resolve([dockBuild('cb_8', 'succeeded', 8), dockBuild('cb_7', 'failed', 7)]);
    await d.dock.builds.load();
    const list = d.dock.builds.list();
    eq('a reload drops stale builds, so no stale running build blocks Build', [list.map(x => x.id), list.some(x => x.status === 'queued' || x.status === 'running'), d.dock.builds.selected().id],
        [['cb_8', 'cb_7'], false, 'cb_8']);
    let answer;
    d.inst.api.listBuilds = () => new Promise(resolve => { answer = resolve; });
    const reloading = d.dock.builds.load();
    d.inst.bus.emit('build:status', dockBuild('cb_12', 'running', 12));
    answer([dockBuild('cb_8', 'succeeded', 8)]);
    await reloading;
    eq('a reload keeps a build reported while it ran, and its selection', [d.dock.builds.list().map(x => x.id), d.dock.builds.selected().id], [['cb_12', 'cb_8'], 'cb_12']);
    d.dock.dispose();
}

// ── dialog (cyd-creator-core.js): only the topmost dialog takes keys; owners close theirs ──
const docKeys = [];
Object.assign(sandbox.document, {
    body: new FakeNode('body'),
    addEventListener(type, fn) { if (type === 'keydown') docKeys.push(fn); },
    removeEventListener(type, fn) { const i = docKeys.indexOf(fn); if (type === 'keydown' && i >= 0) docKeys.splice(i, 1); },
    contains: node => sandbox.document.body.contains(node)
});
sandbox.document.activeElement = sandbox.document.body;
function pressKey(name) {
    const event = { key: name, shiftKey: false, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() {} };
    docKeys.slice().forEach(fn => fn(event));
    return event;
}
function testDialog(host, title, owner) {
    return P.dialog.open(host, { title, actions: [{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }], owner });
}
function attachedHost() {
    const host = new FakeNode('div');
    sandbox.document.body.appendChild(host);
    return host;
}
{
    const host = attachedHost();
    const versions = testDialog(host, 'versions');
    const confirm = testDialog(host, 'confirm');
    const buttons = confirm.el.querySelectorAll('button');
    eq('stacked dialogs share one key listener', docKeys.length, 1);
    check('the newest dialog takes the focus', sandbox.document.activeElement === buttons[0]);
    pressKey('Tab');
    const second = sandbox.document.activeElement === buttons[1];
    pressKey('Tab');
    check('Tab cycles inside the newest dialog only', second && sandbox.document.activeElement === buttons[0]);
    pressKey('Escape');
    eq('Escape closes only the newest dialog', [confirm.closed, versions.closed], [true, false]);
    pressKey('Escape');
    eq('the next Escape closes the dialog below and no key listener stays', [versions.closed, docKeys.length], [true, 0]);
    host.remove();
}
{
    const hostA = attachedHost();
    const hostB = attachedHost();
    const a = testDialog(hostA, 'a');
    const b = testDialog(hostB, 'b');
    a.el.querySelector('button').focus();
    pressKey('Escape');
    eq('Escape closes the dialog of the window that has the focus', [a.closed, b.closed], [true, false]);
    const outside = attachedHost();
    outside.focus();
    const event = pressKey('Escape');
    eq('an Escape elsewhere on the desktop is left alone', [b.closed, event.defaultPrevented], [false, false]);
    b.close();
    [hostA, hostB, outside].forEach(n => n.remove());
}
{
    const host = attachedHost();
    const owner = P.util.bag();
    const owned = testDialog(host, 'owned', owner);
    let result = 'pending';
    owned.promise.then(v => { result = v; });
    owner.dispose();
    await settle();
    eq('disposing the owner closes its dialog', [owned.closed, result, docKeys.length, host.childNodes.length], [true, null, 0, 0]);
    const orphan = testDialog(host, 'orphan');
    host.remove();
    sandbox.document.activeElement = sandbox.document.body;
    const event = pressKey('Escape');
    eq('a dialog whose window is gone closes on the next key without taking it', [orphan.closed, event.defaultPrevented, docKeys.length], [true, false, 0]);
}
{
    const host = attachedHost();
    const owner = { hooks: [], add(fn) { this.hooks.push(fn); return fn; } };
    for (let i = 0; i < 3; i++) testDialog(host, 'closed ' + i, owner).close('a');
    const open = testDialog(host, 'open', owner);
    owner.hooks.forEach(fn => fn());
    eq('an owner keeps one hook for all its dialogs, not one per dialog', [owner.hooks.length, open.closed, docKeys.length], [1, true, 0]);
    const minimised = testDialog(host, 'minimised');
    host.hidden = true;
    sandbox.document.activeElement = sandbox.document.body;
    const hiddenEvent = pressKey('Escape');
    host.hidden = false;
    const shownEvent = pressKey('Escape');
    eq('with nothing focused, Escape reaches the newest dialog only while its window is shown',
        [hiddenEvent.defaultPrevented, shownEvent.defaultPrevented, minimised.closed], [false, true, true]);
    host.remove();
}

// ── code editor (cyd-creator-code.js) on the fake DOM with the textarea fallback ──
sandbox.requestAnimationFrame = fn => fakeSetTimeout(fn, 16);
sandbox.cancelAnimationFrame = fakeClearTimeout;
{
    const full = path.join(apps, 'cyd-creator-code.js');
    vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
}
const codeT = P.i18n({});
const consoleWarn = console.warn;
console.warn = (...args) => { if (!String(args[0]).includes('CodeMirror failed to load')) consoleWarn(...args); };
{
    const ok = P.code.isEditablePath;
    const deep = 'src/app/' + ('a'.repeat(60) + '/').repeat(2);
    eq('file names follow the server rules', [
        'src/app/main.cpp', 'src/app/a/b/c/d/x.h', 'src/app/x.y.hpp', 'src/app/_a-b.c', deep + 'b'.repeat(26) + '.cpp', 'src/app/' + 'a'.repeat(60) + '.cpp'
    ].map(ok), [true, true, true, true, true, true]);
    eq('names the server refuses are refused', [
        'src/app/a/b/c/d/e/x.h', 'src/app/con.cpp', 'src/app/COM1.h', 'src/app/lpt9.x.c', 'src/app/x.', 'src/app/.x.cpp', 'src/app/x.CPP',
        'src/app/x.txt', 'src/app/../x.cpp', deep + 'b'.repeat(27) + '.cpp', 'src/app/' + 'a'.repeat(61) + '.cpp', 'src/generated/ui.h', 'src/app/a b.cpp'
    ].map(ok), Array(13).fill(false));
    const hooks = '#include <cyd.h>\n\nvoid on_btn_clicked(lv_event_t *e) {\n  on_other_clicked(e);\n}\n// on_note_clicked(e) runs from ui.cpp\nvoid  on_spaced_pressed (lv_event_t *e);\n';
    eq('hookLine finds definitions, not calls or comments', ['on_btn_clicked', 'on_other_clicked', 'on_note_clicked', 'on_spaced_pressed', 'on_btn_click'].map(fn => P.code.hookLine(hooks, fn)), [3, 0, 0, 7, 0]);
}
// codeBackend answers like plan 2's file routes: If-Match "*" creates only, a sha must match an
// existing file, a second spelling of a name is a conflict. gates delay an answer, fail fails once
// (an error) or per call in order (an array; null lets that call through).
function codeBackend(initial) {
    const files = new Map(Object.entries(initial));
    const calls = [];
    const gates = {};
    const fail = {};
    let n = 0;
    const E = P.api.ApiError;
    const failOnce = name => {
        const f = fail[name];
        if (!f) return;
        const e = Array.isArray(f) ? f.shift() : f;
        if (!Array.isArray(f) || !f.length) delete fail[name];
        if (e) throw e;
    };
    const api = {
        async listFiles() {
            calls.push(['list']);
            const snapshot = Array.from(files, ([p, f]) => ({ path: p, sha256: f.sha, editable: f.editable !== false, generated: f.editable === false }));
            if (gates.list) await gates.list;
            return snapshot;
        },
        async readFile(id, p) {
            calls.push(['read', p]);
            if (gates.read) await gates.read;
            failOnce('read');
            const f = files.get(p);
            if (!f) throw new E(404, 'CYD_NOT_FOUND', 'file not found');
            return { text: f.text, sha: f.sha };
        },
        async writeFile(id, p, text, ifMatch) {
            calls.push(['write', p, ifMatch]);
            failOnce('write');
            const f = files.get(p);
            const clash = Array.from(files.keys()).some(k => k !== p && k.toLowerCase() === p.toLowerCase());
            if (clash || (ifMatch === '*' && f) || (ifMatch !== '*' && (!f || f.sha !== ifMatch))) throw new E(409, 'CYD_FILE_CONFLICT', 'conflict');
            const sha = 's' + (++n);
            files.set(p, { text, sha });
            return sha;
        },
        async deleteFile(id, p, ifMatch) {
            calls.push(['delete', p, ifMatch]);
            failOnce('delete');
            const f = files.get(p);
            if (!f) throw new E(404, 'CYD_NOT_FOUND', 'file not found');
            if (ifMatch !== '*' && ifMatch !== f.sha) throw new E(409, 'CYD_FILE_CONFLICT', 'conflict');
            files.delete(p);
            return true;
        }
    };
    // changes lists writes and deletes; the random temporary name of a case-only rename reads TMP.h.
    const changes = () => calls.filter(x => x[0] === 'write' || x[0] === 'delete').map(x => x[0] + ' ' + x[1].replace(/cyd_rename_[0-9a-f]{12}\.h$/, 'TMP.h'));
    return { files, calls, gates, fail, api, changes };
}
async function makeCode(initial) {
    const be = codeBackend(initial);
    const host = attachedHost();
    const seen = { state: [], notes: [], confirms: 0 };
    const inst = {
        t: P.i18n({}), bus: P.createBus(), projectId: 'ccp_1', root: host, readonly: false, isLocked: () => false,
        notify: (kind, text) => seen.notes.push(text), confirm: () => { seen.confirms++; return Promise.resolve(true); }, setMode() {}, api: be.api
    };
    inst.bus.on('code:state', s => seen.state.push(s));
    const code = P.code.create(inst);
    await code.mount(host);
    await settle();
    const c = {
        be, host, code, seen,
        area: () => findNode(code.el, n => n.dataset.ccCodeFallback !== undefined),
        dialog: () => findNode(host, n => n.attrs.role === 'dialog'),
        action: id => findNode(c.dialog(), n => n.dataset.ccDialogAction === id),
        input: () => findNode(c.dialog(), n => n.dataset.ccFileName !== undefined),
        dialogError: () => findNode(c.dialog(), n => n.attrs.role === 'alert').textContent,
        tool: (action, p) => findNode(code.el, n => n.dataset.ccCodeAction === action && (!p || n.dataset.path === p)),
        file: p => findNode(code.el, n => n.dataset.ccFile === p),
        type(text) { const a = c.area(); a.value = text; a.dispatch('input'); },
        last: () => seen.state[seen.state.length - 1],
        dispose() { code.dispose(); host.remove(); }
    };
    return c;
}
{
    const c = await makeCode({ 'src/app/a.cpp': { text: 'int a;\n', sha: 'a1' } });
    await c.code.openFile('src/app/a.cpp');
    eq('an opened file reports its saver state', c.last(), 'saved');
    c.type('int a = 1;\n');
    eq('typing marks the file dirty', c.last(), 'dirty');
    c.be.files.delete('src/app/a.cpp');
    await c.code.refreshFiles();
    await settle();
    check('a file deleted elsewhere keeps its unsaved text open', c.code.openPath() === 'src/app/a.cpp' && c.area().value === 'int a = 1;\n');
    eq('the user is told and gets the conflict choice', [c.seen.notes, !!c.dialog(), c.last()], [[codeT('cyd_creator.error.CYD_FILE_CONFLICT')], true, 'conflict']);
    await c.code.refreshFiles();
    await settle();
    eq('a second refresh does not ask again', [c.seen.notes.length, c.host.querySelectorAll('[role="dialog"]').length], [1, 1]);
    c.action('overwrite').click();
    await settle();
    eq('Overwrite recreates the deleted file with If-Match *', [c.be.files.get('src/app/a.cpp').text, c.be.calls.filter(x => x[0] === 'write').map(x => x[2]), c.last()],
        ['int a = 1;\n', ['a1', '*'], 'saved']);
    check('the recreated file is listed again', !!c.file('src/app/a.cpp'));
    c.dispose();
}
{
    const c = await makeCode({ 'src/app/a.cpp': { text: 'x\n', sha: 'a1' } });
    await c.code.openFile('src/app/a.cpp');
    c.type('y\n');
    c.be.files.delete('src/app/a.cpp');
    await advance(1000);
    check('a missed deletion ends in the conflict choice on the next save', !!c.dialog());
    c.action('reload').click();
    await settle();
    eq('loading the saved version of a deleted file closes it and reports saved', [c.code.openPath(), c.last()], ['', 'saved']);
    c.dispose();
}
{
    const c = await makeCode({ 'src/app/a.cpp': { text: 'x\n', sha: 'a1' } });
    await c.code.openFile('src/app/a.cpp');
    c.type('xy\n');
    let release;
    c.be.gates.list = new Promise(resolve => { release = resolve; });
    const refresh = c.code.refreshFiles();
    await advance(1000);
    release();
    await refresh;
    delete c.be.gates.list;
    eq('a list from before the own save does not reload the file', [c.be.calls.filter(x => x[0] === 'read').length, c.area().value], [1, 'xy\n']);
    c.be.files.set('src/app/a.cpp', { text: 'agent\n', sha: 'z9' });
    await c.code.refreshFiles();
    eq('a change made elsewhere reloads a clean file', c.area().value, 'agent\n');
    c.dispose();
}
{
    const c = await makeCode({ 'src/app/a.cpp': { text: 'a\n', sha: 'a1' }, 'src/app/b.cpp': { text: 'b\n', sha: 'b1' } });
    await c.code.openFile('src/app/a.cpp');
    let release;
    c.be.gates.read = new Promise(resolve => { release = resolve; });
    const loading = c.code.openFile('src/app/b.cpp');
    await settle();
    check('the editor is read-only while another file loads', c.area().readOnly === true && c.code.el.classList.contains('is-loading'));
    await c.code.openFile('src/app/a.cpp');
    release();
    delete c.be.gates.read;
    eq('a click on the open file wins over the file still loading', [await loading, c.code.openPath(), c.code.el.classList.contains('is-loading'), c.area().readOnly],
        [false, 'src/app/a.cpp', false, false]);
    c.dispose();
}
{
    const c = await makeCode({ 'src/app/app.cpp': { text: 'a\n', sha: 'a1' }, 'src/app/b.cpp': { text: 'b\n', sha: 'b1' } });
    c.tool('new').click();
    c.input().value = 'APP.cpp';
    c.action('ok').click();
    const clash = c.dialogError();
    c.input().value = 'sub/nul.h';
    c.action('ok').click();
    eq('the name dialog refuses a second spelling and device names', [clash, c.dialogError()], [codeT('cyd_creator.code.file_exists'), codeT('cyd_creator.code.file_name_invalid')]);
    c.action('cancel').click();
    await settle();
    c.tool('rename', 'src/app/app.cpp').click();
    c.input().value = 'App.cpp';
    c.action('ok').click();
    await settle();
    eq('a case-only rename goes through a temporary .h copy', [c.be.changes(), Array.from(c.be.files.keys()).sort()],
        [['write src/app/TMP.h', 'delete src/app/app.cpp', 'write src/app/App.cpp', 'delete src/app/TMP.h'], ['src/app/App.cpp', 'src/app/b.cpp']]);
    c.be.calls.length = 0;
    c.be.fail.delete = new P.api.ApiError(500, 'CYD_INTERNAL', 'boom');
    c.tool('rename', 'src/app/b.cpp').click();
    c.input().value = 'c.cpp';
    c.action('ok').click();
    await settle();
    eq('a rename whose delete fails removes the copy and refreshes the tree',
        [c.be.changes(), Array.from(c.be.files.keys()).sort(), c.be.calls.some(x => x[0] === 'list'), c.seen.notes.slice(-1)],
        [['write src/app/c.cpp', 'delete src/app/b.cpp', 'delete src/app/c.cpp'], ['src/app/App.cpp', 'src/app/b.cpp'], true, [codeT('cyd_creator.error.CYD_INTERNAL')]]);
    c.tool('new').click();
    check('a name dialog is open before dispose', !!c.dialog() && docKeys.length === 1);
    c.code.dispose();
    eq('dispose closes the module\'s dialogs', [!!c.dialog(), docKeys.length], [false, 0]);
    c.host.remove();
}
{
    const c = await makeCode({ 'src/generated/ui.h': { text: 'v1\n', sha: 'g1', editable: false } });
    await c.code.openFile('src/generated/ui.h');
    c.be.files.set('src/generated/ui.h', { text: 'v2\n', sha: 'g2', editable: false });
    await c.code.refreshFiles();
    eq('an open generated file follows a layout change', c.area().value, 'v2\n');
    c.be.files.set('src/generated/ui.h', { text: 'v3\n', sha: 'g3', editable: false });
    c.be.fail.read = new P.api.ApiError(503, 'CYD_INTERNAL', 'busy');
    await c.code.refreshFiles();
    eq('a failed reload keeps the old text', c.area().value, 'v2\n');
    await c.code.openFile('src/generated/ui.h');
    eq('clicking a stale generated file reloads it', c.area().value, 'v3\n');
    c.dispose();
}
{
    const c = await makeCode({ 'src/app/a.cpp': { text: 'a\n', sha: 'a1' }, 'src/app/b.cpp': { text: 'b\n', sha: 'b1' } });
    c.code.setProblems([
        { file: 'src/app/a.cpp', line: 1, severity: 'note' },
        { file: 'src/app/b.cpp', line: 2, severity: 'note' }, { file: 'src/app/b.cpp', line: 3, severity: 'warning' }
    ]);
    const tag = p => findNode(c.file(p), n => n.className === 'cc-tag');
    eq('notes count as notes in the tree', [tag('src/app/a.cpp').dataset.tone, tag('src/app/b.cpp').dataset.tone, tag('src/app/b.cpp').textContent], [undefined, 'warning', '2']);
    c.file('src/app/b.cpp').focus();
    c.code.setProblems([]);
    const active = sandbox.document.activeElement;
    check('a rebuilt tree keeps the focus on the same file', active && active.dataset.ccFile === 'src/app/b.cpp' && c.code.el.contains(active));
    c.dispose();
}
{
    // Case-only rename of the open file while the project gets locked between the requests: the new
    // name and the restore both fail, the temporary copy keeps the text on the server and opens.
    const c = await makeCode({ 'src/app/app.cpp': { text: 'int a;\n', sha: 'a1' } });
    await c.code.openFile('src/app/app.cpp');
    const locked = new P.api.ApiError(423, 'CYD_PROJECT_LOCKED', 'locked');
    c.be.fail.write = [null, locked, locked];
    c.tool('rename', 'src/app/app.cpp').click();
    c.input().value = 'App.cpp';
    c.action('ok').click();
    await settle();
    const tmp = Array.from(c.be.files.keys()).find(p => /cyd_rename_[0-9a-f]{12}\.h$/.test(p));
    eq('when both names fail the text stays on the server in the temporary copy',
        [c.be.changes(), tmp && c.be.files.get(tmp).text, c.be.files.has('src/app/app.cpp'), c.seen.notes.slice(-1)],
        [['write src/app/TMP.h', 'delete src/app/app.cpp', 'write src/app/App.cpp', 'write src/app/app.cpp'], 'int a;\n', false, [P.errors.message(codeT, locked)]]);
    eq('the kept copy opens in the editor', [c.code.openPath(), c.area().value], [tmp, 'int a;\n']);
    c.type('int a = 2;\n');
    await advance(1000);
    eq('the kept copy saves like any file', [c.be.files.get(tmp).text, c.last()], ['int a = 2;\n', 'saved']);
    c.dispose();
}
{
    // A missed deletion surfaces while another file opens: "Load saved version" closes the deleted
    // file, and the switch goes on without asking to discard anything.
    const c = await makeCode({ 'src/app/a.cpp': { text: 'a\n', sha: 'a1' }, 'src/app/b.cpp': { text: 'b\n', sha: 'b1' } });
    await c.code.openFile('src/app/a.cpp');
    c.type('a2\n');
    c.be.files.delete('src/app/a.cpp');
    const opening = c.code.openFile('src/app/b.cpp');
    await settle();
    c.action('reload').click();
    eq('after the saved version of a deleted file the switch needs no discard confirm', [await opening, c.code.openPath(), c.seen.confirms], [true, 'src/app/b.cpp', 0]);
    c.dispose();
}
{
    const c = await makeCode({ 'src/app/net/a.cpp': { text: 'a\n', sha: 'a1' } });
    c.tool('new').click();
    c.input().value = 'Net/b.cpp';
    c.action('ok').click();
    const folderClash = c.dialogError();
    c.input().value = 'net/b.cpp';
    c.action('ok').click();
    await settle();
    eq('a folder spelled differently is refused in the dialog, the same folder is fine',
        [folderClash, !!c.dialog(), c.be.files.has('src/app/net/b.cpp')], [codeT('cyd_creator.code.file_exists'), false, true]);
    c.dispose();
}
console.warn = consoleWarn;

// ── agent panel (cyd-creator-agent.js) on the fake DOM ──
sandbox.TextEncoder = TextEncoder;
{
    const full = path.join(apps, 'cyd-creator-agent.js');
    vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
}
{
    const tail = P.agent.tailBytes;
    const bytes = s => new TextEncoder().encode(s).length;
    eq('tailBytes keeps short text', tail('a\nb', 16), 'a\nb');
    const lines = Array.from({ length: 50 }, (_, i) => 'line ' + i).join('\n');
    const cut = tail(lines, 64);
    check('tailBytes keeps the end within the limit and starts at a line', bytes(cut) <= 64 && cut.startsWith('line ') && lines.endsWith(cut), JSON.stringify(cut));
    const emoji = tail('\u{1F600}'.repeat(40), 30);
    check('tailBytes never starts inside a surrogate pair', bytes(emoji) <= 30 && emoji.length > 0 && !/^[\uDC00-\uDFFF]/.test(emoji), JSON.stringify(emoji));
    const umlauts = tail('ä'.repeat(20), 9);
    check('tailBytes counts UTF-8 bytes', bytes(umlauts) <= 9 && /^ä+$/.test(umlauts), JSON.stringify(umlauts));
}
function agentJob(id, status, minute, extra) {
    return Object.assign({
        id, project_id: 'ccp_1', mode: 'change', status, phase: status === 'awaiting_confirmation' ? 'planning' : 'coding', repair_attempt: 0,
        plan: null, error_code: '', error_message: '', created_at: '2026-10-12T10:' + String(minute).padStart(2, '0') + ':00.000Z'
    }, extra || {});
}
function deferred() {
    let resolve;
    const promise = new Promise(r => { resolve = r; });
    return { promise, resolve };
}
const agentPlan = { summary: 'A clock', screens: ['home'], widgets: ['label'], data_sources: [], wifi: 'off', aurago_link: false, libraries: [] };
// makeAgent mounts an agent panel like the app; opts: messages, jobs, project, readonly, revisions, flushOk,
// confirms (answers of inst.confirm in order, default yes), startJob(body) and confirmJob(prompt) promises.
async function makeAgent(opts) {
    const o = opts || {};
    const calls = { start: [], confirm: [], restore: [], order: [], listMessages: 0, badge: [] };
    const confirms = o.confirms || [];
    let locked = false;
    const inst = {
        t: P.i18n({}), bus: P.createBus(), projectId: 'ccp_1', root: attachedHost(), readonly: !!o.readonly, caps: { libraries: [] },
        project: Object.assign({ id: 'ccp_1', has_draft: false, publish_review: false }, o.project || {}),
        isLocked: () => locked,
        api: {
            listMessages: () => { calls.listMessages++; return Promise.resolve((o.messages || []).slice()); },
            listJobs: () => Promise.resolve((o.jobs || []).slice()),
            startJob: (id, body) => { calls.start.push(body); return o.startJob ? o.startJob(body) : Promise.resolve(agentJob('ccj_new', 'queued', 30)); },
            confirmJob: (id, prompt) => { calls.confirm.push(prompt); return o.confirmJob ? o.confirmJob(prompt) : Promise.resolve(agentJob(id, 'running', 2)); },
            cancelJob: () => Promise.resolve(true),
            discardDraft: () => Promise.resolve(true),
            listRevisions: () => Promise.resolve(o.revisions || []),
            revisionDiff: () => Promise.resolve({ layout_changed: false, files: [] }),
            restoreRevision: id => { calls.order.push('restore'); calls.restore.push(id); return Promise.resolve({}); }
        },
        confirm: () => Promise.resolve(confirms.length ? confirms.shift() : true),
        flushAll: () => { calls.order.push('flush'); return Promise.resolve(o.flushOk !== false); },
        notify() {}, setPanelBadge: (id, on) => calls.badge.push(on), showPanel() {}, refreshProject() {}, reloadProject: () => Promise.resolve()
    };
    const agent = P.agent.create(inst);
    const host = attachedHost();
    await agent.mount(host);
    await settle();
    const byData = (name, value) => findNode(agent.el, n => n.dataset && n.dataset[name] !== undefined && (value === undefined || n.dataset[name] === value));
    const inDialog = (name, value) => findNode(inst.root, n => n.dataset && n.dataset[name] === value);
    return {
        agent, inst, calls, byData, inDialog, confirms,
        log: () => byData('ccAgentLog'),
        errorText: () => { const n = findNode(agent.el, x => x.className === 'cc-job-error'); return n ? n.textContent : null; },
        setLocked(v) { locked = v; },
        dispose() { agent.dispose(); host.remove(); inst.root.remove(); }
    };
}
{
    // A fast failure: the job_status event that ends the job arrives before the POST answer.
    const answer = deferred();
    const a = await makeAgent({ jobs: [agentJob('ccj_old', 'succeeded', 1)], startJob: () => answer.promise });
    a.byData('ccAgentInput').value = 'Add a clock';
    a.byData('ccComposer').dispatch('submit');
    a.inst.bus.emit('job:status', agentJob('ccj_new', 'failed', 30, { error_code: 'CYD_INTERNAL' }));
    answer.resolve(agentJob('ccj_new', 'queued', 30));
    await settle();
    eq('a stale POST answer does not reopen a job that already failed',
        [a.agent.job().status, a.agent.isBusy(), a.calls.badge[a.calls.badge.length - 1], a.byData('ccAgentAction', 'send').disabled], ['failed', false, false, false]);
    a.inst.bus.emit('job:status', agentJob('ccj_new', 'running', 30));
    eq('a replayed earlier state of a finished job is ignored', a.agent.job().status, 'failed');
    a.dispose();
}
{
    const answer = deferred();
    const a = await makeAgent({ jobs: [agentJob('ccj_p', 'awaiting_confirmation', 2, { plan: agentPlan })], confirmJob: () => answer.promise });
    a.byData('ccAgentAction', 'confirm').click();
    a.byData('ccAgentAction', 'confirm').click();
    eq('Build it sends one request and disables the plan actions but not Cancel',
        [a.calls.confirm.length, a.byData('ccAgentAction', 'confirm').disabled, a.byData('ccAgentAction', 'replan').disabled, a.byData('ccAgentAction', 'cancel').disabled],
        [1, true, true, false]);
    a.inst.bus.emit('job:status', agentJob('ccj_p', 'failed', 2, { error_code: 'CYD_INTERNAL' }));
    answer.resolve(agentJob('ccj_p', 'running', 2));
    await settle();
    eq('the confirm answer does not reopen a job that failed meanwhile', a.agent.job().status, 'failed');
    a.dispose();
    const later = deferred();
    const b = await makeAgent({ jobs: [agentJob('ccj_q', 'awaiting_confirmation', 2, { plan: agentPlan })], confirmJob: () => later.promise });
    b.byData('ccAgentAction', 'confirm').click();
    b.inst.bus.emit('job:status', agentJob('ccj_q', 'running', 2, { phase: 'building' }));
    later.resolve(agentJob('ccj_q', 'running', 2, { phase: 'layout' }));
    await settle();
    eq('the confirm answer applies only while the job still waits', b.agent.job().phase, 'building');
    b.dispose();
}
{
    const a = await makeAgent({ jobs: [agentJob('ccj_p', 'awaiting_confirmation', 2, { plan: agentPlan })] });
    a.byData('ccAgentAction', 'replan').click();
    const field = a.byData('ccReplanInput');
    field.value = '   ';
    a.byData('ccAgentAction', 'replan-send').click();
    eq('Plan again with an empty description sends nothing and focuses the field', [a.calls.confirm.length, sandbox.document.activeElement === field], [0, true]);
    field.value = 'Bigger digits';
    field.dispatch('input');
    a.inst.bus.emit('project:updated', a.inst.project);
    eq('an update keeps the edited description and the field', [a.byData('ccReplanInput') === field, field.value], [true, 'Bigger digits']);
    a.byData('ccAgentAction', 'replan-send').click();
    await settle();
    eq('Plan again sends the trimmed description', a.calls.confirm, ['Bigger digits']);
    a.dispose();
}
{
    const ro = await makeAgent({ readonly: true });
    ro.byData('ccAgentInput').value = 'x';
    ro.byData('ccAgentInput').dispatch('keydown', { key: 'Enter', ctrlKey: true });
    const busy = await makeAgent({ jobs: [agentJob('ccj_r', 'running', 3)] });
    busy.byData('ccAgentInput').value = 'x';
    busy.byData('ccAgentInput').dispatch('keydown', { key: 'Enter', ctrlKey: true });
    await settle();
    eq('Ctrl+Enter starts nothing while read-only or while a job runs', [ro.calls.start.length, busy.calls.start.length], [0, 0]);
    ro.dispose();
    busy.dispose();
}
{
    const answer = deferred();
    const first = { id: 'msg_1', role: 'agent', text: 'Hello', job_id: 'ccj_old', created_at: '2026-10-12T10:01:00.000Z' };
    const a = await makeAgent({ messages: [first], startJob: () => answer.promise });
    const log = a.log();
    const n1 = log.childNodes[0];
    let redraws = 0;
    const replaceChildren = log.replaceChildren.bind(log);
    log.replaceChildren = (...nodes) => { redraws++; replaceChildren(...nodes); };
    a.byData('ccAgentInput').value = 'Add a clock';
    a.byData('ccComposer').dispatch('submit');
    const pending = log.childNodes[1];
    a.inst.bus.emit('sse:job_message', { message: { id: 'msg_2', role: 'user', text: 'Add a clock', job_id: 'ccj_new', created_at: '2026-10-12T10:05:00.000Z' } });
    eq('the server copy replaces the pending bubble in place',
        [log.childNodes.length, log.childNodes[0] === n1, pending.parentNode, log.childNodes[1].dataset.ccMessage, pending.className.includes('is-pending'), redraws],
        [2, true, null, 'msg_2', true, 0]);
    answer.resolve(agentJob('ccj_new', 'running', 5));
    await settle();
    a.inst.bus.emit('sse:job_step', { job_id: 'ccj_new', kind: 'file', text: 'Wrote src/app/app_main.cpp' });
    a.inst.bus.emit('sse:job_message', { message: { id: 'msg_3', role: 'agent', text: 'Done', job_id: 'ccj_new', created_at: '2026-10-12T10:06:00.000Z' } });
    eq('new lines are appended without redrawing the log',
        [log.childNodes.length, log.childNodes[0] === n1, log.childNodes[2].dataset.ccStep, log.childNodes[3].dataset.ccMessage, redraws], [4, true, 'file', 'msg_3', 0]);
    a.inst.bus.emit('sse:stream_ready', { after: 9 });
    await settle();
    eq('a plain stream_ready does not reload', a.calls.listMessages, 1);
    a.inst.bus.emit('sse:stream_ready', { reload: true, after: 9 });
    await settle();
    eq('stream_ready with reload reloads without duplicating or redrawing lines', [a.calls.listMessages, log.childNodes.length, log.childNodes[0] === n1, redraws], [2, 4, true, 0]);
    a.dispose();
}
{
    const a = await makeAgent({
        jobs: [agentJob('ccj_f', 'failed', 3, { error_code: 'CYD_BUSY', error_reason: 'publication_review', error_message: 'an interrupted publication needs review' })],
        project: { has_draft: true, publish_review: true }
    });
    const resume = a.byData('ccAgentAction', 'resume');
    const review = a.inst.t('cyd_creator.error.publication_review');
    eq('a fenced publication shows the review text and blocks only Resume',
        [a.errorText(), a.byData('ccDraft').hidden, resume.disabled, resume.getAttribute('title'), a.byData('ccAgentAction', 'discard').disabled],
        [review, false, true, review, false]);
    a.dispose();
    const b = await makeAgent({ jobs: [agentJob('ccj_b', 'failed', 3, { error_code: 'CYD_BUSY', error_message: 'Another job is active.' })], project: { has_draft: true } });
    const before = b.errorText();
    b.inst.project.publish_review = true;
    b.inst.bus.emit('project:updated', b.inst.project);
    eq('without a reason CYD_BUSY keeps the server text until the summary reports the fence',
        [before, b.errorText(), b.byData('ccAgentAction', 'resume').disabled], ['Another job is active.', b.inst.t('cyd_creator.error.publication_review'), true]);
    b.dispose();
}
{
    // Server lines with a code show the translation filled with their vars; without a key (or a code,
    // or with a malformed one) they show the server's English text. Agent lines are never translated;
    // a failed job's line names the error like the job card.
    const words = {
        'cyd_creator.step.file_written': '{{path}} geschrieben (Zeilen: {{lines}})',
        'cyd_creator.msg.job_cancelled_draft': 'Job abgebrochen. Entwurf behalten.',
        'cyd_creator.msg.job_failed': '{{error}} Projekt unverändert.',
        'cyd_creator.error.CYD_BUILD_FAILED': 'Build fehlgeschlagen.'
    };
    Object.assign(sandbox.window.I18N, words);
    const at = minute => '2026-10-12T10:' + String(minute).padStart(2, '0') + ':00.000Z';
    const messages = [
        { id: 'm1', role: 'system', code: 'job_cancelled_draft', text: 'The job was cancelled.', job_id: 'ccj_s', created_at: at(1) },
        { id: 'm2', role: 'system', code: 'job_failed', vars: { error_code: 'CYD_BUILD_FAILED', error: 'The build still fails after 3 repair attempts.' }, text: 'The build still fails after 3 repair attempts. The project is unchanged.', job_id: 'ccj_s', created_at: at(2) },
        { id: 'm3', role: 'system', code: 'server_restarted', text: 'The server restarted while the job was running.', job_id: 'ccj_s', created_at: at(3) },
        { id: 'm4', role: 'system', text: 'A line from before the codes.', job_id: 'ccj_s', created_at: at(4) },
        { id: 'm5', role: 'agent', code: 'job_cancelled_draft', text: 'Agent prose', job_id: 'ccj_s', created_at: at(5) },
        { id: 'm6', role: 'system', code: 'job_failed', vars: { error_code: 'CYD_BUSY', error_reason: 'publication_review', error: 'fenced' }, text: 'fenced The project is unchanged.', job_id: 'ccj_s', created_at: at(6) }
    ];
    const a = await makeAgent({ messages, jobs: [agentJob('ccj_s', 'running', 0)] });
    a.inst.bus.emit('sse:job_step', { job_id: 'ccj_s', kind: 'file', code: 'file_written', vars: { path: 'src/app/{lines}.cpp', lines: 3 }, text: 'Wrote src/app/{lines}.cpp (lines: 3)' });
    a.inst.bus.emit('sse:job_step', { job_id: 'ccj_s', kind: 'info', code: 'plan_submitted', vars: { screens: 2 }, text: 'Plan submitted · screens: 2' });
    a.inst.bus.emit('sse:job_step', { job_id: 'ccj_s', kind: 'info', code: 'File_Written', vars: { path: 'x' }, text: 'raw {{path}}' });
    a.inst.bus.emit('sse:job_step', { job_id: 'ccj_s', kind: 'build', text: 'Build 1 succeeded' });
    const shown = a.log().childNodes.map(n => n.dataset.ccStep !== undefined ? n.childNodes[1].textContent : n.childNodes[0].textContent);
    eq('server lines show their translation or the English text', shown, [
        'Job abgebrochen. Entwurf behalten.',
        'Build fehlgeschlagen. Projekt unverändert.',
        'The server restarted while the job was running.',
        'A line from before the codes.',
        'Agent prose',
        a.inst.t('cyd_creator.error.publication_review') + ' Projekt unverändert.',
        'src/app/{lines}.cpp geschrieben (Zeilen: 3)',
        'Plan submitted · screens: 2',
        'raw {{path}}',
        'Build 1 succeeded'
    ]);
    a.inst.bus.emit('sse:job_message', { message: { id: 'm7', role: 'system', code: 'job_cancelled_draft', text: 'The job was cancelled.', job_id: 'ccj_s', created_at: at(7) } });
    eq('a live system line is translated too', a.log().childNodes.at(-1).childNodes[0].textContent, 'Job abgebrochen. Entwurf behalten.');
    a.dispose();
    Object.keys(words).forEach(key => { delete sandbox.window.I18N[key]; });
}
{
    const rev = { id: 'ccr_1', label: 'v1', reason: 'job', created_at: '2026-10-12T10:00:00.000Z' };
    const a = await makeAgent({ revisions: [rev], flushOk: false, confirms: [true, false] });
    a.agent.openRevisions();
    await settle();
    a.inDialog('ccRevision', 'ccr_1').click();
    await settle();
    const restore = a.inDialog('ccDialogAction', 'restore');
    const enabled = !restore.disabled;
    a.setLocked(true);
    a.inst.bus.emit('lock:changed', true, 'ccj_x');
    const lockedNow = restore.disabled;
    a.setLocked(false);
    a.inst.bus.emit('lock:changed', false, '');
    eq('Restore follows the selection and the lock', [enabled, lockedNow, restore.disabled], [true, true, false]);
    restore.click();
    await settle();
    eq('unsaved edits that cannot be saved stop the restore unless confirmed', [a.calls.order, restore.disabled], [['flush'], false]);
    a.confirms.push(true, true);
    restore.click();
    await settle();
    eq('the restore runs after the flush attempt', [a.calls.order, a.calls.restore], [['flush', 'flush', 'restore'], ['ccr_1']]);
    a.agent.openRevisions();
    await settle();
    const open = a.inst.root.childNodes.length;
    a.agent.dispose();
    await settle();
    eq('disposing the panel closes its revisions dialog', [open, a.inst.root.childNodes.length, docKeys.length], [1, 0, 0]);
    a.inst.root.remove();
}

// ── start screen (cyd-creator-home.js) on the fake DOM ──
{
    const full = path.join(apps, 'cyd-creator-home.js');
    vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
}
{
    const tz = P.home.posixTimezone;
    eq('posixTimezone maps IANA zones and the ICU legacy Asia/Calcutta', [tz('Europe/Berlin'), tz('Asia/Kolkata'), tz('Asia/Calcutta'), tz('Mars/Olympus')],
        ['CET-1CEST,M3.5.0,M10.5.0/3', 'IST-5:30', 'IST-5:30', 'UTC0']);
}
function homeProject(id, minute) {
    return { id, name: 'Project ' + id, board: 'cyd_2432s028r', theme: 'dark_sun', updated_at: '2026-10-12T10:' + String(minute).padStart(2, '0') + ':00Z' };
}
// makeHome mounts a start screen like the app; opts: list(n) → promise of the n-th listProjects answer,
// create(body) and del(id) → promises of the API answers.
async function makeHome(opts) {
    const o = opts || {};
    const calls = { list: 0, create: [], open: [], del: [], notify: [] };
    const inst = {
        t: P.i18n({}), bus: P.createBus(), root: attachedHost(), readonly: false,
        caps: {
            boards: [{ id: 'cyd_2432s028r', label: 'CYD 2.8"', status: 'verified', chip: 'esp32', display: { width: 320, height: 240, driver: 'ili9341' } }],
            themes: [{ id: 'dark_sun', tokens: {} }], languages: ['en', 'de']
        },
        domId: suffix => 'cc_home_test_' + suffix,
        renderToolchain() {},
        api: {
            listProjects: () => { calls.list++; return o.list ? o.list(calls.list) : Promise.resolve([]); },
            createProject: body => { calls.create.push(body); return o.create ? o.create(body) : Promise.resolve({ id: 'ccp_new' }); },
            deleteProject: id => { calls.del.push(id); return o.del ? o.del(id) : Promise.resolve(true); }
        },
        confirm: () => Promise.resolve(true),
        openProject: (id, options) => { calls.open.push(id); void options; return Promise.resolve(); },
        notify: kind => calls.notify.push(kind)
    };
    const home = P.home.create(inst);
    await home.mount(attachedHost());
    await settle();
    const inDialog = (name, value) => findNode(inst.root, n => n.dataset && n.dataset[name] === value);
    return { home, inst, calls, inDialog };
}
function rejectable() {
    let reject;
    const promise = new Promise((_, r) => { reject = r; });
    return { promise, reject };
}
{
    const pending = deferred();
    const h = await makeHome({ create: () => pending.promise });
    const result = h.home.openNewProject('agent');
    h.inDialog('ccField', 'name').value = 'Clock';
    h.inDialog('ccField', 'description').value = 'Shows the time';
    h.inDialog('ccDialogAction', 'create').click();
    await settle();
    pressKey('Escape');
    await settle();
    const lists = h.calls.list;
    pending.resolve({ id: 'ccp_late' });
    await settle();
    eq('a create dismissed in flight opens nothing, starts no job and refreshes the list',
        [h.calls.create.length, h.calls.open, h.calls.list - lists, await result, h.inst.root.childNodes.length], [1, [], 1, null, 0]);
    h.home.dispose();
}
{
    const pending = rejectable();
    const h = await makeHome({ create: () => pending.promise });
    h.home.openNewProject('designer');
    h.inDialog('ccField', 'name').value = 'Clock';
    h.inDialog('ccDialogAction', 'create').click();
    await settle();
    pressKey('Escape');
    await settle();
    pending.reject(new P.api.ApiError(503, 'CYD_DISABLED', 'off', {}));
    await settle();
    eq('a create that fails after the dialog closed becomes a notification', [h.calls.open, h.calls.notify, h.inst.root.childNodes.length], [[], ['error'], 0]);
    h.home.dispose();
}
{
    const pending = rejectable();
    const h = await makeHome({ list: () => Promise.resolve([homeProject('ccp_1', 1)]), del: () => pending.promise });
    const del = findNode(h.home.el, n => n.dataset && n.dataset.ccAction === 'delete-project');
    del.click();
    await settle();
    const during = del.disabled;
    pending.reject(new P.api.ApiError(500, 'CYD_INTERNAL', 'db', {}));
    await settle();
    eq('the delete button stays disabled while its DELETE runs and returns after a failure', [during, h.calls.del, del.disabled, h.calls.notify], [true, ['ccp_1'], false, ['error']]);
    h.home.dispose();
}
{
    const answers = [];
    const h = await makeHome({ list: n => { if (n === 1) return Promise.resolve([]); const d = deferred(); answers.push(d); return d.promise; } });
    const older = h.home.refresh();
    const newer = h.home.refresh();
    answers[1].resolve([homeProject('ccp_new', 2)]);
    await settle();
    answers[0].resolve([homeProject('ccp_old', 1)]);
    await Promise.all([older, newer]);
    await settle();
    const shown = () => h.home.el.querySelectorAll('[data-cc-project-id]').filter(n => n.dataset.ccAction === 'open').map(n => n.dataset.ccProjectId);
    eq('an older list answer that arrives late never replaces the newer one', shown(), ['ccp_new']);
    h.home.dispose();
    const lists = h.calls.list;
    await h.home.refresh();
    eq('a disposed start screen does not refresh', h.calls.list, lists);
}

// ── window (cyd-creator-app.js) on the fake DOM: the real modules against a stubbed P.api ──
{
    const full = path.join(apps, 'cyd-creator-app.js');
    vm.runInContext(fs.readFileSync(full, 'utf8'), sandbox, { filename: full });
}
sandbox.window.EventSource = BrowserES;
// The window shows real sentences: the English bundle (it also proves the keys exist).
Object.assign(sandbox.window.I18N, JSON.parse(fs.readFileSync(path.join(here, '..', 'ui', 'lang', 'cyd_creator', 'en.json'), 'utf8')));
console.warn = (...args) => { if (!String(args[0]).includes('CodeMirror failed to load')) consoleWarn(...args); };
// The desktop dispatches aurago:desktop-policy on document when read-only changes.
const policyListeners = [];
{
    const add = sandbox.document.addEventListener;
    const remove = sandbox.document.removeEventListener;
    sandbox.document.addEventListener = (type, fn, opts) => { if (type === 'aurago:desktop-policy') policyListeners.push(fn); else add(type, fn, opts); };
    sandbox.document.removeEventListener = (type, fn, opts) => {
        if (type !== 'aurago:desktop-policy') { remove(type, fn, opts); return; }
        const i = policyListeners.indexOf(fn);
        if (i >= 0) policyListeners.splice(i, 1);
    };
}
const App = sandbox.window.CYDCreatorApp;
// appBackend replaces the P.api wrappers the window and its parts call. calls lists them in order;
// fail.<name> = ApiError makes that call throw (until cleared), gates.<name> = promise holds it;
// project is the stored summary (names: other names per project id). PUT /layout checks If-Match
// like the server (409 CYD_PROJECT_CONFLICT with details.version); bump() is a change elsewhere.
function appBackend(opts) {
    const o = opts || {};
    const calls = [];
    const fail = {};
    const gates = {};
    const saved = {};
    const names = o.names || {};
    const project = Object.assign({ id: 'ccp_1', name: 'Clock', board: 'cyd_2432s028r', theme: 'dark_sun', version: 3, locked_by_job: '', has_draft: false, publish_review: false }, o.project || {});
    const layout = { name: 'Clock', orientation: 'landscape', screens: [] };
    let version = 3;
    const call = (name, fn) => async (...args) => {
        calls.push(name);
        if (gates[name]) await gates[name];
        if (fail[name]) throw fail[name];
        return fn(...args);
    };
    const answers = {
        capabilities: call('capabilities', () => ({
            enabled: true, readonly: !!o.capsReadonly, languages: ['en'],
            boards: [{ id: 'cyd_2432s028r', label: 'CYD 2.8"', status: 'verified', chip: 'esp32', display: { width: 320, height: 240, driver: 'ili9341' } }],
            themes: [{ id: 'dark_sun', tokens: {} }],
            toolchain: Object.assign({ docker_available: true, present: true, pulling: false, progress: 1, error: '' }, o.toolchain || {})
        })),
        listProjects: call('listProjects', () => []),
        getProject: call('getProject', id => Object.assign({}, project, { id: id || project.id }, names[id] ? { name: names[id] } : {})),
        getLayout: call('getLayout', () => ({ version, project: Object.assign({}, layout), issues: [] })),
        putLayout: call('putLayout', (id, next, ifMatch) => {
            if (Number(ifMatch) !== version) throw new P.api.ApiError(409, 'CYD_PROJECT_CONFLICT', 'stale', { version });
            version++;
            Object.assign(layout, next);
            return { version, issues: [] };
        }),
        uploadAsset: call('uploadAsset', () => { version++; layout.assets = [{ id: 'a_sun' }]; return { asset: { id: 'a_sun' } }; }),
        listFiles: call('listFiles', () => [{ path: 'src/app/app_main.cpp', sha256: 's1', editable: true, generated: false }]),
        readFile: call('readFile', () => ({ text: '#include <cyd.h>\n', sha: 's1' })),
        writeFile: call('writeFile', () => 's2'),
        listBuilds: call('listBuilds', () => []),
        listMessages: call('listMessages', () => []),
        listJobs: call('listJobs', () => (o.jobs || []).map(job => Object.assign({}, job))),
        startJob: call('startJob', (id, body) => ({ id: 'ccj_1', project_id: id, mode: body.mode, status: 'queued', phase: 'queued', created_at: '2026-10-12T10:00:00.000Z' }))
    };
    Object.keys(answers).forEach(name => { saved[name] = P.api[name]; P.api[name] = answers[name]; });
    return {
        calls, fail, gates, project, layout,
        version: () => version,
        bump() { version++; },
        restore() { Object.keys(saved).forEach(name => { P.api[name] = saved[name]; }); }
    };
}
let appWindows = 0;
// makeApp renders a window like the desktop shell; opts: projectId (open it right away), readonly,
// capsReadonly, project (summary fields), names, toolchain.
async function makeApp(opts) {
    const o = opts || {};
    const be = appBackend(o);
    browserStreams.length = 0;
    const host = attachedHost();
    const notes = [];
    const windowId = 'win-' + (++appWindows);
    let beforeClose = null;
    const inst = App.render(host, windowId, {
        notify: n => notes.push(n.message), readonly: !!o.readonly, projectId: o.projectId || '',
        setWindowBeforeClose(id, fn) { beforeClose = fn; }
    });
    await settle();
    const find = (name, value) => findNode(inst.root, n => n.dataset && n.dataset[name] !== undefined && (value === undefined || n.dataset[name] === value));
    return { inst, be, notes, windowId, find, beforeClose: () => beforeClose, dispose() { App.dispose(windowId); host.remove(); be.restore(); } };
}
const portrait = () => ({ name: 'Clock', orientation: 'portrait', screens: [] });
// gate holds the next calls of name until the returned release() runs.
function gate(be, name) {
    let release;
    be.gates[name] = new Promise(resolve => { release = resolve; });
    return () => { delete be.gates[name]; release(); };
}
const appT = P.i18n({});
{
    const a = await makeApp({ projectId: 'ccp_1' });
    const design = a.find('ccAction', 'mode-design');
    const code = a.find('ccAction', 'mode-code');
    eq('the Design/Code buttons keep the button frame and are segments', [design.className, code.className], ['cc-btn cc-seg', 'cc-btn cc-seg']);
    code.click();
    await settle();
    eq('the mode toggle presses one segment and mounts the editor', [design.getAttribute('aria-pressed'), code.getAttribute('aria-pressed'), a.inst.root.dataset.ccMode, !!a.find('ccCode')],
        ['false', 'true', 'code', true]);
    a.inst.bus.emit('code:state', 'conflict');
    eq('the save chip shows the worse of layout and code', a.find('ccSaveState').dataset.state, 'conflict');
    a.inst.bus.emit('code:state', 'saved');
    a.be.calls.length = 0;
    browserStreams[0].send('job_status', { job: { id: 'ccj_0', status: 'succeeded', created_at: '2026-10-12T09:00:00.000Z' } }, 7);
    browserStreams[0].send('stream_ready', { after: 7 });
    await settle();
    eq('a plain stream_ready reloads nothing', a.be.calls, []);
    browserStreams[0].send('stream_ready', { reload: true, after: 7 });
    await settle();
    eq('stream_ready with reload reloads summary, layout, files and builds (the agent reloads itself)', a.be.calls.slice().sort(),
        ['getLayout', 'getProject', 'listBuilds', 'listFiles', 'listJobs', 'listMessages']);
    a.inst.layoutSaver.change(portrait());
    a.be.bump(); // the layout changed elsewhere (or the database was restored)
    a.be.calls.length = 0;
    browserStreams[0].send('stream_ready', { reload: true, after: 7 });
    await settle();
    const kept = !a.be.calls.includes('getLayout') && a.be.calls.includes('getProject');
    const saving = a.inst.flushAll();
    await settle();
    eq('a reload keeps a layout with unsaved edits and its save meets the If-Match conflict',
        [kept, !!a.find('ccDialogAction', 'overwrite'), a.find('ccSaveState').dataset.state], [true, true, 'conflict']);
    const puts = () => a.be.calls.filter(c => c === 'putLayout').length;
    const putsBefore = puts();
    a.inst.root.dispatch('keydown', { ctrlKey: true, key: 's' });
    await settle();
    eq('Ctrl+S while the conflict choice is open sends no stale save and stacks no dialog',
        [puts(), a.inst.root.querySelectorAll('[data-cc-dialog-action="overwrite"]').length], [putsBefore, 1]);
    a.find('ccDialogAction', 'keep').click();
    eq('Keep editing leaves the edits unsaved', [await saving, a.inst.layoutSaver.dirty()], [false, true]);
    a.dispose();
}
{
    // SSE give-up: after three failures in a row the window probes GET /projects/{id}.
    const a = await makeApp({ projectId: 'ccp_1' });
    browserStreams[0].send('job_status', { job: { id: 'ccj_0', status: 'succeeded', created_at: '2026-10-12T09:00:00.000Z' } }, 9);
    browserStreams[0].send('stream_ready', { after: 9 });
    const fail = async () => { browserStreams[browserStreams.length - 1].onerror(); await settle(); };
    const probes = () => a.be.calls.filter(c => c === 'getProject').length;
    const before = probes();
    a.be.fail.getProject = new P.api.ApiError(401, 'unauthorized', 'Unauthorized', {});
    await fail();
    await advance(1500);
    await fail();
    eq('two failures in a row do not probe yet', probes(), before);
    await advance(3000);
    // The third stream is cut off during its catch-up: its buffered event 12 was never delivered.
    browserStreams[browserStreams.length - 1].send('job_message', { message: { id: 'm12' } }, 12);
    await fail();
    const banner = a.find('ccStreamStopped');
    eq('the third failure probes and an expired session stops the stream with its message', [probes(), banner.hidden, banner.childNodes[0].textContent],
        [before + 1, false, appT('cyd_creator.error.unauthorized')]);
    const streams = browserStreams.length;
    await advance(60000);
    eq('a stopped stream does not reconnect', browserStreams.length, streams);
    delete a.be.fail.getProject;
    a.find('ccAction', 'reconnect').click();
    eq('Retry resumes after the last delivered event (not a buffered one) and hides the banner', [browserStreams.length, browserStreams[browserStreams.length - 1].url, banner.hidden],
        [streams + 1, '/api/desktop/cyd-creator/projects/ccp_1/events?after=9', true]);
    a.be.fail.getProject = new P.api.ApiError(503, 'CYD_DISABLED', 'CYD Creator is not available', { problem: 'storage refused' });
    for (let i = 0; i < 3; i++) { await fail(); await advance(30000); }
    eq('CYD_DISABLED stops the stream with its text and details.problem',
        [banner.hidden, banner.childNodes.map(n => n.textContent).slice(0, 2)], [false, [appT('cyd_creator.error.CYD_DISABLED'), 'storage refused']]);
    a.dispose();
}
{
    const a = await makeApp({ projectId: 'ccp_1' });
    const fail = async () => { browserStreams[browserStreams.length - 1].onerror(); await settle(); await advance(30000); };
    a.be.fail.getProject = new P.api.ApiError(0, '', 'network error', {});
    for (let i = 0; i < 3; i++) await fail();
    eq('a network failure keeps reconnecting', [browserStreams.length, a.find('ccStreamStopped').hidden], [4, true]);
    a.be.fail.getProject = new P.api.ApiError(404, 'CYD_NOT_FOUND', 'not found', {});
    await fail();
    eq('a deleted project returns to the start screen with a notice', [a.inst.root.dataset.ccView, a.notes[a.notes.length - 1], a.inst.events],
        ['home', appT('cyd_creator.error.CYD_NOT_FOUND'), null]);
    a.dispose();
}
{
    const a = await makeApp({ projectId: 'ccp_1', project: { publish_review: true } });
    const banner = a.find('ccPublishReview');
    eq('a fenced project shows the publication review banner as an alert', [banner.hidden, banner.textContent, banner.className, banner.getAttribute('role')],
        [false, appT('cyd_creator.error.publication_review'), 'cc-banner cc-banner--danger', 'alert']);
    a.be.project.publish_review = false;
    await a.inst.refreshProject();
    eq('the banner disappears (and loses its sentence) once the summary no longer reports the fence', [banner.hidden, banner.textContent], [true, '']);
    a.be.fail.putLayout = new P.api.ApiError(409, 'CYD_BUSY', 'needs review', { reason: 'publication_review' });
    a.be.project.publish_review = true;
    a.inst.layoutSaver.change({ name: 'Clock', orientation: 'portrait', screens: [] });
    const saved = await a.inst.flushAll();
    await settle();
    eq('a save refused by the fence reloads the summary and shows the banner', [saved, a.find('ccSaveState').dataset.state, banner.hidden], [false, 'failed', false]);
    a.dispose();
}
{
    // A fenced project blocks Build with the reason. The server keeps the fence in memory: a restart
    // clears it without an event and its stream_ready has no reload, so the first stream_ready after
    // a failure re-reads the summary (a stream_ready without a failure before it reads nothing).
    const a = await makeApp({ projectId: 'ccp_1', project: { publish_review: true } });
    const build = a.find('ccAction', 'build');
    eq('a fenced project blocks Build and says why', [build.disabled, build.title], [true, appT('cyd_creator.error.publication_review')]);
    browserStreams[0].send('stream_ready', { after: 4 });
    await settle();
    a.be.calls.length = 0;
    a.be.project.publish_review = false;
    browserStreams[0].send('stream_ready', { after: 4 });
    await settle();
    eq('a stream_ready without a failure before it reads nothing', a.be.calls, []);
    browserStreams[browserStreams.length - 1].onerror();
    await settle();
    await advance(1500);
    browserStreams[browserStreams.length - 1].send('stream_ready', { after: 4 });
    await settle();
    eq('the stream back after a failure re-reads the summary: banner and Build follow the cleared fence',
        [a.be.calls.filter(c => c === 'getProject').length, a.find('ccPublishReview').hidden, build.disabled], [1, true, false]);
    a.be.calls.length = 0;
    browserStreams[browserStreams.length - 1].send('stream_ready', { after: 4 });
    await settle();
    eq('only the first stream_ready after the failure reads it', a.be.calls, []);
    a.dispose();
}
{
    // Every agent job starts from the saved project.
    const a = await makeApp({ projectId: 'ccp_1' });
    a.inst.layoutSaver.change({ name: 'Clock', orientation: 'portrait', screens: [] });
    a.be.calls.length = 0;
    a.find('ccAgentInput').value = 'Add a clock';
    a.find('ccComposer').dispatch('submit');
    await settle();
    eq('the composer saves unsaved edits before the job starts', a.be.calls.filter(c => c === 'putLayout' || c === 'startJob'), ['putLayout', 'startJob']);
    a.dispose();
    const b = await makeApp({ projectId: 'ccp_1' });
    b.be.fail.putLayout = new P.api.ApiError(0, '', 'network error', {});
    b.inst.layoutSaver.change({ name: 'Clock', orientation: 'portrait', screens: [] });
    b.find('ccAgentInput').value = 'Add a clock';
    b.find('ccComposer').dispatch('submit');
    await settle();
    eq('edits that cannot be saved stop the job with the save error', [b.be.calls.includes('startJob'), b.notes[b.notes.length - 1], b.find('ccAgentInput').value, !!b.find('ccMessage')],
        [false, appT('cyd_creator.error.network'), 'Add a clock', false]);
    b.dispose();
    const c = await makeApp({});
    c.be.calls.length = 0;
    await c.inst.openProject('ccp_1', { orientation: 'portrait', prompt: 'A clock' });
    await settle();
    eq('a new project saves its orientation, then starts the agent from the description',
        [c.be.calls.filter(x => x === 'putLayout' || x === 'startJob'), c.be.layout.orientation, c.find('ccPanel', 'agent').getAttribute('aria-selected')],
        [['putLayout', 'startJob'], 'portrait', 'true']);
    c.dispose();
}
{
    // inst.confirm belongs to the window: closing the window closes the dialog and answers no.
    const a = await makeApp({});
    let answer = 'pending';
    a.inst.confirm({ titleKey: 'cyd_creator.app.unsaved_title', bodyKey: 'cyd_creator.app.unsaved_body' }).then(v => { answer = v; });
    const open = !!a.find('ccDialogAction', 'ok');
    App.dispose(a.windowId);
    await settle();
    eq('the window owns its confirm dialogs', [open, answer, docKeys.length], [true, false, 0]);
    a.inst.notify('error', 'late');
    eq('notify still works after the window is gone', a.notes[a.notes.length - 1], 'late');
    a.dispose();
}
{
    const a = await makeApp({ projectId: 'ccp_1', toolchain: { present: false } });
    const pullBefore = a.find('ccAction', 'pull-toolchain').disabled;
    policyListeners.slice().forEach(fn => fn({ detail: { readonly: true } }));
    await settle();
    eq('a read-only policy reaches the toolbar, the toolchain banner and the agent panel',
        [pullBefore, a.inst.readonly, a.find('ccAction', 'build').disabled, a.find('ccAction', 'pull-toolchain').disabled, a.find('ccAgentAction', 'send').disabled, a.find('ccAgentInput').disabled],
        [false, true, true, true, true, true]);
    policyListeners.slice().forEach(fn => fn({ detail: { readonly: false } }));
    eq('lifting it enables them again', [a.inst.readonly, a.find('ccAgentAction', 'send').disabled, a.find('ccAction', 'pull-toolchain').disabled], [false, false, false]);
    a.dispose();
    const h = await makeApp({});
    policyListeners.slice().forEach(fn => fn({ detail: { readonly: true } }));
    await settle();
    eq('the start screen follows read-only', [h.find('ccAction', 'new-agent').disabled, h.find('ccAction', 'new-designer').disabled], [true, true]);
    const ro = await makeApp({ capsReadonly: true });
    eq('a read-scoped token (capabilities readonly) makes the window read-only', [ro.inst.readonly, ro.find('ccAction', 'new-agent').disabled], [true, true]);
    h.dispose();
    ro.dispose();
    eq('disposed windows leave no policy listener behind', policyListeners.length, 0);
}
{
    // Read-only switched by the desktop policy while a job is active reaches the agent panel at once:
    // the plan's Confirm and Change plan and the composer lock, Cancel stays usable.
    const plan = { summary: 'A clock', screens: ['home'], widgets: [], data_sources: [], wifi: 'off', aurago_link: false, libraries: [] };
    const a = await makeApp({ projectId: 'ccp_1', project: { locked_by_job: 'ccj_w' },
        jobs: [{ id: 'ccj_w', project_id: 'ccp_1', mode: 'new', status: 'awaiting_confirmation', phase: 'planning', plan, created_at: '2026-10-12T10:00:00.000Z' }] });
    const planButtons = () => ['confirm', 'replan', 'cancel'].map(id => findNode(a.find('ccPlan'), n => n.dataset && n.dataset.ccAgentAction === id).disabled);
    const before = planButtons();
    policyListeners.slice().forEach(fn => fn({ detail: { readonly: true } }));
    await settle();
    eq('a read-only policy during a waiting plan locks Confirm, Change plan and the composer, not Cancel',
        [before, planButtons(), a.find('ccAgentInput').disabled, a.find('ccAgentAction', 'send').disabled], [[false, false, false], [true, true, false], true, true]);
    policyListeners.slice().forEach(fn => fn({ detail: { readonly: false } }));
    await settle();
    eq('lifting it during the plan enables them again', [planButtons(), a.find('ccAgentInput').disabled], [[false, false, false], false]);
    a.inst.bus.emit('job:status', { id: 'ccj_w', project_id: 'ccp_1', mode: 'new', status: 'running', phase: 'coding', created_at: '2026-10-12T10:00:00.000Z' });
    policyListeners.slice().forEach(fn => fn({ detail: { readonly: true } }));
    await settle();
    const cancel = findNode(a.find('ccJob'), n => n.dataset && n.dataset.ccAgentAction === 'cancel');
    eq('a read-only policy during a running job locks the composer and keeps Cancel', [a.find('ccAgentInput').disabled, !!cancel && cancel.disabled], [true, false]);
    a.dispose();
}
{
    // uploadAsset (the designer's image upload): pending edits are saved first, and edits made while
    // the upload runs are never marked saved by the reloaded layout.
    const a = await makeApp({ projectId: 'ccp_1' });
    a.inst.layoutSaver.change(portrait());
    a.be.calls.length = 0;
    const asset = await a.inst.uploadAsset({ name: 'sun.png' });
    const order = a.be.calls.filter(c => c === 'putLayout' || c === 'uploadAsset' || c === 'getLayout');
    eq('an upload saves pending edits, then takes the new layout version', [asset.id, order, a.inst.layoutVersion, a.inst.layoutSaver.dirty(), a.inst.layout().project.assets],
        ['a_sun', ['putLayout', 'uploadAsset', 'getLayout'], a.be.version(), false, [{ id: 'a_sun' }]]);
    a.inst.layoutSaver.change(Object.assign(portrait(), { name: 'After' }));
    eq('the next edit saves on the new version', [await a.inst.flushAll(), a.find('ccSaveState').dataset.state], [true, 'saved']);
    a.be.fail.putLayout = new P.api.ApiError(0, '', 'network error', {});
    a.inst.layoutSaver.change(Object.assign(portrait(), { name: 'Offline' }));
    a.be.calls.length = 0;
    const refused = await a.inst.uploadAsset({ name: 'moon.png' }).then(() => null, err => err);
    eq('edits that cannot be saved refuse the upload with the save error and stay unsaved',
        [refused && P.errors.message(appT, refused), a.be.calls.includes('uploadAsset'), a.inst.layoutSaver.dirty()], [appT('cyd_creator.error.network'), false, true]);
    a.dispose();
    const b = await makeApp({ projectId: 'ccp_1' });
    const release = gate(b.be, 'uploadAsset');
    const upload = b.inst.uploadAsset({ name: 'sun.png' });
    await settle();
    b.inst.layoutSaver.change(portrait());
    const versionBefore = b.inst.layoutVersion;
    release();
    await upload;
    eq('an edit made during the upload stays unsaved on its version', [b.inst.layoutSaver.dirty(), b.inst.layoutVersion, b.inst.layout().project.assets], [true, versionBefore, undefined]);
    const saving = b.inst.flushAll();
    await settle();
    eq('its save meets the new version through the conflict choice', !!b.find('ccDialogAction', 'overwrite'), true);
    b.find('ccDialogAction', 'keep').click();
    await saving;
    b.dispose();
    // The layout answer of the upload is on the way (taken at version 4) while an edit meets the
    // conflict and is overwritten (version 5): the older answer must not reset the window to 4.
    const c = await makeApp({ projectId: 'ccp_1' });
    const backendLayout = P.api.getLayout;
    let releaseLayout;
    P.api.getLayout = id => backendLayout(id).then(answer => new Promise(resolve => { releaseLayout = () => resolve(answer); }));
    const late = c.inst.uploadAsset({ name: 'sun.png' });
    await settle();
    P.api.getLayout = backendLayout;
    c.inst.layoutSaver.change(portrait());
    const overwriting = c.inst.flushAll();
    await settle();
    c.find('ccDialogAction', 'overwrite').click();
    const overwritten = await overwriting;
    releaseLayout();
    await late;
    eq('an Overwrite while the upload\'s layout answer is on the way keeps the newer version',
        [overwritten, c.inst.layoutVersion, c.inst.layoutSaver.token(), c.be.version(), c.inst.layout().project.orientation], [true, 5, 5, 5, 'portrait']);
    c.inst.layoutSaver.change(Object.assign(portrait(), { name: 'Next' }));
    eq('the next edit saves on that version without a conflict', [await c.inst.flushAll(), !!c.find('ccDialogAction', 'overwrite')], [true, false]);
    c.dispose();
}
{
    // A failed code save that blocks an agent job explains itself with its own error.
    const a = await makeApp({ projectId: 'ccp_1' });
    a.find('ccAction', 'mode-code').click();
    await settle();
    await a.inst.openFile('src/app/app_main.cpp', 1);
    a.be.fail.writeFile = new P.api.ApiError(422, 'CYD_FILE_LIMIT', 'too large', {});
    const area = a.find('ccCodeFallback');
    area.value = '#include <cyd.h>\n// more\n';
    area.dispatch('input');
    a.find('ccAgentInput').value = 'Add a clock';
    a.find('ccComposer').dispatch('submit');
    await settle();
    eq('a refused code save stops the job with its sentence', [a.be.calls.includes('startJob'), a.notes[a.notes.length - 1]], [false, appT('cyd_creator.error.CYD_FILE_LIMIT')]);
    a.dispose();
}
{
    const a = await makeApp({ projectId: 'ccp_1' });
    const summaries = () => a.be.calls.filter(c => c === 'getProject').length;
    const start = summaries();
    a.inst.bus.emit('code:state', 'failed');
    a.inst.bus.emit('code:state', 'dirty');
    a.inst.bus.emit('code:state', 'failed');
    await settle();
    const once = summaries() - start;
    a.inst.bus.emit('code:state', 'saved');
    a.inst.bus.emit('code:state', 'failed');
    await settle();
    eq('a run of failed code saves asks for the summary once, the next run again', [once, summaries() - start], [1, 2]);
    a.dispose();
}
{
    // Closing the window (desktop beforeClose → leaveProject) with edits that cannot be saved asks first.
    const a = await makeApp({ projectId: 'ccp_1' });
    const clean = await a.beforeClose()();
    a.be.fail.putLayout = new P.api.ApiError(0, '', 'network error', {});
    a.inst.layoutSaver.change(portrait());
    let answer = 'pending';
    a.beforeClose()().then(v => { answer = v; });
    await settle();
    const leave = a.find('ccDialogAction', 'ok');
    a.find('ccDialogAction', 'cancel').click();
    await settle();
    const cancelled = answer;
    a.beforeClose()().then(v => { answer = v; });
    await settle();
    a.find('ccDialogAction', 'ok').click();
    await settle();
    eq('a clean window closes at once; unsaved edits ask: Cancel keeps the window, Leave anyway closes it',
        [clean, leave && leave.textContent, cancelled, answer], [true, appT('cyd_creator.app.leave_anyway'), false, true]);
    a.dispose();
}
{
    // Lifecycle A → start screen → B → close: every subscription and listener the parts add goes
    // away again, and late answers for A never reach B.
    const a = await makeApp({ names: { ccp_2: 'Second' } });
    const bus = a.inst.bus;
    const on = bus.on;
    let live = 0;
    bus.on = (type, fn) => {
        live++;
        const off = on(type, fn);
        let done = false;
        return () => { if (!done) { done = true; live--; } off(); };
    };
    const keydowns = () => (a.inst.root.listeners.keydown || []).length;
    const title = () => a.find('ccProjectTitle').textContent;
    await a.inst.openProject('ccp_1');
    await settle();
    const opened = [live, keydowns()];
    const streamA = browserStreams[browserStreams.length - 1];
    const releaseA = gate(a.be, 'getProject');
    a.inst.refreshProject();
    delete a.be.gates.getProject;
    a.find('ccAction', 'home').click();
    await settle();
    const home = [live, keydowns(), streamA.closed, a.inst.root.dataset.ccView];
    await a.inst.openProject('ccp_1');
    await settle();
    a.be.project.name = 'Stale';
    releaseA();
    await settle();
    eq('a late summary of the closed project A is dropped by the reopened one', title(), 'Clock');
    a.be.project.name = 'Clock';
    const releaseOpen = gate(a.be, 'getProject');
    const pendingA = a.inst.openProject('ccp_1');
    delete a.be.gates.getProject;
    await a.inst.openProject('ccp_2');
    releaseOpen();
    await pendingA;
    await settle();
    eq('a late open of A does not replace B', [title(), a.inst.root.querySelectorAll('[data-cc-project-title]').length, a.inst.projectId], ['Second', 1, 'ccp_2']);
    const reopened = [live, keydowns()];
    App.dispose(a.windowId);
    await settle();
    eq('A → home → B → close returns subscriptions and listeners to their baseline',
        [opened[0] > 1, home, reopened, live, keydowns(), policyListeners.length, docKeys.length],
        [true, [1, 0, true, 'home'], opened, 0, 0, 0, 0]);
    a.dispose();
}
console.warn = consoleWarn;

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('all CYD Creator shell checks passed');
