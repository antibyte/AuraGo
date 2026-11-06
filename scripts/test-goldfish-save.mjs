import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import http from 'node:http';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const candidates = [process.env.CHROME_BIN, process.env.PROGRAMFILES && resolve(process.env.PROGRAMFILES, 'Google/Chrome/Application/chrome.exe'),
    process.env['PROGRAMFILES(X86)'] && resolve(process.env['PROGRAMFILES(X86)'], 'Microsoft/Edge/Application/msedge.exe'),
    '/usr/bin/google-chrome', '/usr/bin/chromium', '/usr/bin/chromium-browser', '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'].filter(Boolean);
const browser = candidates.find(existsSync);
assert.ok(browser, 'A real Chrome/Edge browser is required for IndexedDB transaction tests');
const scratch = resolve(root, 'disposable/_goldfish-save');
mkdirSync(scratch, { recursive: true });
const profile = mkdtempSync(resolve(scratch, 'profile-'));
const names = ['content', 'content-act1', 'content-act2', 'content-act3', 'content-act4', 'state', 'save'];
const sources = new Map(names.map(name => ['/' + name + '.js', readFileSync(resolve(root, 'ui/js/desktop/apps/goldfish-adventure', name + '.js'))]));
const page = '<!doctype html><meta charset="utf-8">' + names.map(name => '<script src="/' + name + '.js"></script>').join('');
const server = http.createServer((request, response) => {
    response.setHeader('Content-Type', request.url === '/' ? 'text/html' : 'text/javascript');
    if (request.url === '/') response.end(page);
    else if (sources.has(request.url)) response.end(sources.get(request.url));
    else { response.statusCode = 404; response.end(); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const url = 'http://127.0.0.1:' + server.address().port + '/';
const processHandle = spawn(browser, ['--headless=new', '--no-first-run', '--no-default-browser-check', '--disable-gpu', '--no-sandbox',
    '--remote-debugging-port=0', '--user-data-dir=' + profile, 'about:blank'], { windowsHide: true, stdio: ['ignore', 'ignore', 'pipe'] });
let socket;
try {
    const debuggerURL = await new Promise((resolve, reject) => {
        let output = '';
        const timer = setTimeout(() => reject(new Error('Browser did not expose its debugger')), 20000);
        processHandle.stderr.on('data', chunk => {
            output += chunk;
            const match = output.match(/DevTools listening on (ws:\/\/[^\s]+)/);
            if (match) { clearTimeout(timer); resolve(match[1]); }
        });
        processHandle.once('error', reject);
        processHandle.once('exit', code => reject(new Error('Browser exited: ' + code)));
    });
    const address = new URL(debuggerURL);
    const target = await fetch('http://' + address.host + '/json/new?about:blank', { method: 'PUT' }).then(value => value.json());
    socket = new WebSocket(target.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => { socket.addEventListener('open', resolve, { once: true }); socket.addEventListener('error', reject, { once: true }); });
    let counter = 0;
    const pending = new Map();
    socket.addEventListener('message', event => {
        const message = JSON.parse(event.data), request = pending.get(message.id);
        if (!request) return;
        pending.delete(message.id);
        message.error ? request.reject(new Error(JSON.stringify(message.error))) : request.resolve(message.result);
    });
    const call = (method, params = {}) => new Promise((resolve, reject) => {
        const id = ++counter;
        pending.set(id, { resolve, reject });
        socket.send(JSON.stringify({ id, method, params }));
    });
    await call('Page.enable');
    const loaded = new Promise(resolve => {
        const onMessage = event => {
            if (JSON.parse(event.data).method !== 'Page.loadEventFired') return;
            socket.removeEventListener('message', onMessage);
            resolve();
        };
        socket.addEventListener('message', onMessage);
    });
    await call('Page.navigate', { url });
    await loaded;
    const result = await call('Runtime.evaluate', { awaitPromise: true, returnByValue: true, expression: `
        (async () => {
            const check = (value, message) => { if (!value) throw new Error(message); };
            const rejectCode = async (operation, code) => {
                try { await operation(); } catch (error) { check(error.code === code, 'Expected '+code+', got '+error.code); return; }
                throw new Error('Expected rejection '+code);
            };
            if (!window.GoldfishSave) await new Promise(resolve => window.addEventListener('load', resolve, { once: true }));
            const one = await GoldfishSave.open(), two = await GoldfishSave.open();
            const initial = GoldfishState.create();
            const first = await one.write('manual-1', initial, { expectedRevision: 0 });
            check(first.revision === 1, 'Initial revision is one');
            const left = GoldfishState.create(), right = GoldfishState.create();
            left.settings.sound = true; right.settings.reducedMotion = true;
            const race = await Promise.allSettled([
                one.write('manual-1', left, { expectedRevision: 1 }),
                two.write('manual-1', right, { expectedRevision: 1 })
            ]);
            check(race.filter(value => value.status === 'fulfilled').length === 1, 'Exactly one concurrent CAS writer wins');
            check(race.find(value => value.status === 'rejected').reason.code === 'conflict', 'Other writer retains progress with conflict');
            check(left.settings.sound && right.settings.reducedMotion, 'Conflicting caller states stay in memory');
            const current = await one.read('manual-1'), prior = await two.read('manual-1', { prior: true });
            check(current.revision === 2 && prior.revision === 2, 'Both recovery reads carry the current CAS revision');
            check(!prior.state.settings.sound && !prior.state.settings.reducedMotion, 'Prior valid revision is retained');
            check(current.state.settings.sound !== current.state.settings.reducedMotion, 'Winner is complete, never a partial merge');
            const interrupted = GoldfishState.apply(initial, 'r01_stop').state;
            await one.write('manual-3', interrupted, { expectedRevision: 0 });
            const recovered = await two.read('manual-3');
            const resultLines = GoldfishState.pendingResults(recovered.state);
            check(recovered.state.schema === 1 && resultLines.length > 1, 'Legacy schema-one actions retain their unread result');
            recovered.state.seenLines.push(resultLines[0].text);
            await two.write('manual-3', recovered.state, { expectedRevision: 1 });
            const acknowledged = await one.read('manual-3');
            check(GoldfishState.pendingResults(acknowledged.state)[0].text === resultLines[1].text, 'Only acknowledged result lines advance on a durable round-trip');
            check(GoldfishState.apply(acknowledged.state, 'r01_stop').changed === false, 'Recovered dialogue cannot duplicate rewards');
            const nativePut = IDBObjectStore.prototype.put;
            IDBObjectStore.prototype.put = function (value) {
                if (value.current?.label === 'fault') throw new DOMException('Injected quota', 'QuotaExceededError');
                return nativePut.call(this, value);
            };
            await rejectCode(() => one.write('manual-1', initial, { expectedRevision: 2, label: 'fault' }), 'quota');
            IDBObjectStore.prototype.put = nativePut;
            check((await one.read('manual-1')).revision === 2, 'Failed transaction never replaces stored progress');
            const invalid = GoldfishState.create(); invalid.room = 'R16';
            await rejectCode(() => one.write('manual-1', invalid, { expectedRevision: 2 }), 'invalid');
            check((await one.read('manual-1')).revision === 2, 'Invalid imports never overwrite slots');
            const database = await new Promise((resolve, reject) => { const request = indexedDB.open('aurago-goldfish-adventure', 1); request.onsuccess = () => resolve(request.result); request.onerror = () => reject(request.error); });
            const seed = records => new Promise((resolve, reject) => { const tx = database.transaction('slots', 'readwrite'); records.forEach(value => tx.objectStore('slots').put(value)); tx.oncomplete = resolve; tx.onabort = () => reject(tx.error); });
            const future = GoldfishState.create(); future.schema = 99;
            await seed([{ id: 'chapter-4', revision: 1, current: { state: future, label: '', updatedAt: 1 }, prior: null }]);
            await rejectCode(() => one.read('chapter-4'), 'version');
            await rejectCode(() => one.write('chapter-4', initial, { expectedRevision: 1 }), 'version');
            await seed([{ id: 'oversized-metadata', revision: 1, current: { state: initial, label: 'x'.repeat(GoldfishSave.BUDGET), updatedAt: 1 } }]);
            await rejectCode(() => one.write('manual-2', initial, { expectedRevision: 0 }), 'quota');
            check(await one.read('manual-2') === null, 'Budget failure leaves target empty');
            check((await one.list()).length === 12, 'Only twelve named slots are presented');
            await rejectCode(() => one.remove('manual-1', { expectedRevision: 1 }), 'conflict');
            database.close(); one.close(); two.close();
            return 'Real IndexedDB: concurrent CAS, prior recovery, atomic abort, invalid/future states, quota budget and twelve slots passed.';
        })()
    ` });
    assert.equal(result.exceptionDetails, undefined, JSON.stringify(result.exceptionDetails));
    console.log(result.result.value);
} finally {
    socket?.close();
    processHandle.kill();
    await new Promise(resolve => server.close(resolve));
    if (processHandle.exitCode === null) await new Promise(resolve => processHandle.once('exit', resolve));
    // Chrome descendants may release profile handles shortly after the parent exits.
    try { rmSync(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 }); } catch (_) { /* The profile is ignored scratch. */ }
}
