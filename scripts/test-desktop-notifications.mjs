import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const runtimeSource = await readFile(path.join(root, 'ui/js/desktop/core/notification-center-runtime.js'), 'utf8');
const eventSource = await readFile(path.join(root, 'ui/js/desktop/core/sdk-events-bootstrap.js'), 'utf8');
const historyKey = 'aurago.desktop.notifications.v1';
const policyKey = 'aurago.desktop.notification-policy.v1';

class Storage {
  values = new Map();
  failRead = false;
  failWrite = false;
  failRemove = false;
  getItem(key) {
    if (this.failRead) throw new Error('storage blocked');
    return this.values.has(key) ? this.values.get(key) : null;
  }
  setItem(key, value) {
    if (this.failWrite) throw new Error('quota exceeded');
    this.values.set(key, String(value));
  }
  removeItem(key) {
    if (this.failRemove) throw new Error('storage blocked');
    this.values.delete(key);
  }
}

function plain(value) { return JSON.parse(JSON.stringify(value)); }

function makeRuntime({ auth = { enabled: false, authenticated: false }, session = new Storage(), local = new Storage(), fetchError = false } = {}) {
  const handlers = new Map();
  const documentHandlers = new Map();
  const requests = [];
  let currentAuth = auth;
  let failFetch = fetchError;
  let now = Date.now();
  let nextFetchGate = null;
  let nextBodyGate = null;
  let fireNextTimerImmediately = false;
  let nextTimerID = 1;
  const timers = new Map();
  class TestDate extends Date { static now() { return now; } }
  const window = {
    sessionStorage: session,
    localStorage: local,
    addEventListener(type, fn) {
      const set = handlers.get(type) || new Set();
      set.add(fn);
      handlers.set(type, set);
    },
    emit(type, event = {}) { for (const fn of handlers.get(type) || []) fn(event); },
    async emitAsync(type, event = {}) { await Promise.all(Array.from(handlers.get(type) || [], fn => Promise.resolve(fn(event)))); },
    setTimeout(fn, delay) {
      const id = nextTimerID++;
      if (fireNextTimerImmediately) {
        fireNextTimerImmediately = false;
        Promise.resolve().then(fn);
        return id;
      }
      timers.set(id, setTimeout(fn, delay));
      return id;
    },
    clearTimeout(id) {
      const timer = timers.get(id);
      if (timer) clearTimeout(timer);
      timers.delete(id);
    },
    async fetch(url, options) {
      requests.push([url, options]);
      const gate = nextFetchGate;
      nextFetchGate = null;
      if (gate) await gate.promise;
      if (failFetch) throw new Error('offline');
      const bodyGate = nextBodyGate;
      nextBodyGate = null;
      return { ok: true, async json() { if (bodyGate) await bodyGate.promise; return currentAuth; } };
    }
  };
  const document = {
    visibilityState: 'visible',
    addEventListener(type, fn) {
      const set = documentHandlers.get(type) || new Set();
      set.add(fn);
      documentHandlers.set(type, set);
    },
    emit(type, event = {}) { for (const fn of documentHandlers.get(type) || []) fn(event); },
    async emitAsync(type, event = {}) { await Promise.all(Array.from(documentHandlers.get(type) || [], fn => Promise.resolve(fn(event)))); }
  };
  const context = { window, document, Date: TestDate, AbortController, Math, Set, Map, Object, Array, Number, String, JSON, RegExp, Error, t: () => 'Benachrichtigung' };
  vm.createContext(context);
  vm.runInContext(runtimeSource, context, { filename: 'notification-center-runtime.js' });
  return {
    api: window.AuraDesktopNotificationCenter, window, document, handlers, requests, session, local,
    setAuth(value) { currentAuth = value; },
    setFetchError(value) { failFetch = value; },
    advanceTime(ms) { now += ms; },
    timeoutNextAuthCheck() { fireNextTimerImmediately = true; },
    holdNextFetch() {
      let release;
      const promise = new Promise(resolve => { release = resolve; });
      nextFetchGate = { promise };
      return release;
    },
    holdNextBody() {
      let release;
      const promise = new Promise(resolve => { release = resolve; });
      nextBodyGate = { promise };
      return release;
    }
  };
}

function storedRecord(id, title, appId = '') {
  return { id, title, message: 'saved preview', appId, ts: Date.now() - 1000, read: false };
}

async function testPolicyDefaultsAndDelivery() {
  const env = makeRuntime();
  await env.api.init();
  assert.deepEqual(plain(env.api.policy()), { dnd: false, apps: {} });
  assert.deepEqual(plain(env.api.delivery({ appId: 'meshcore' })), { toast: true, sound: true });

  env.api.setPolicy({ apps: { 'network-cameras': { toast: false }, desktop: { sound: false } } });
  assert.deepEqual(plain(env.api.delivery({ appId: 'network-cameras' })), { toast: false, sound: true });
  assert.deepEqual(plain(env.api.delivery({})), { toast: true, sound: false });
  env.api.setPolicy({ dnd: true });
  assert.deepEqual(plain(env.api.delivery({ appId: 'network-cameras', urgent: true })), { toast: false, sound: false });

  const saved = JSON.parse(env.local.getItem(policyKey));
  assert.equal(saved.version, 1);
  assert.equal(saved.dnd, true);
  assert.equal(saved.apps['network-cameras'].toast, false);
  assert.equal(saved.apps.desktop.sound, false);
}

async function testCrossTabPolicySync() {
  const local = new Storage();
  const first = makeRuntime({ local });
  const second = makeRuntime({ local });
  await Promise.all([first.api.init(), second.api.init()]);
  const changes = [];
  second.api.subscribePolicy(value => changes.push(plain(value)));
  first.api.setPolicy({ dnd: true, apps: { desktop: { toast: false, sound: true } } });
  second.window.emit('storage', { key: policyKey, newValue: local.getItem(policyKey) });
  assert.equal(changes.at(-1).dnd, true);
  assert.deepEqual(changes.at(-1).apps.desktop, { toast: false, sound: true });
  second.window.emit('storage', { key: policyKey, newValue: null });
  assert.deepEqual(plain(second.api.policy()), { dnd: false, apps: {} });
}

async function testHistoryRestoreSanitizationAndActions() {
  const session = new Storage();
  session.setItem(historyKey, JSON.stringify({ version: 1, scope: 'local', records: [
    storedRecord('n-1-old', 'Older'),
    { ...storedRecord('bad', 'Bad id'), context: { html: '<script>bad</script>' } }
  ] }));
  const env = makeRuntime({ session });
  const snapshots = [];
  env.api.subscribe(value => snapshots.push(plain(value)));
  assert.equal(env.api.history().length, 0, 'history stays hidden before auth scope is established');
  await env.api.init();
  assert.equal(env.api.history().length, 1, 'malformed entries are discarded');
  assert.equal(env.api.history()[0].title, 'Older');

  const message = '😀'.repeat(400);
  const entry = env.api.record({ title: '  ' + 'x'.repeat(180), message, appId: 'storymaker', persist: false, html: '<b>ignored</b>' }, { project_id: '6d0e2ac2-e6ef-4ab3-988a-cb632e28fd22', unsafe: true });
  assert.equal(Array.from(entry.title).length, 160);
  assert.equal(Array.from(entry.message).length, 280);
  assert.deepEqual(plain(entry.context), { projectId: '6d0e2ac2-e6ef-4ab3-988a-cb632e28fd22' });
  assert.equal(env.api.history().length, 2, 'persist:false remains in the live center');
  let persisted = JSON.parse(session.getItem(historyKey));
  assert.equal(persisted.records.length, 1, 'persist:false does not write to sessionStorage');

  assert.equal(env.api.markRead(entry.id), true);
  assert.equal(env.api.history()[0].read, true);
  assert.equal(env.api.markAllRead(), true);
  assert.equal(env.api.history().every(item => item.read), true);
  assert.equal(env.api.remove(entry.id), true);
  assert.equal(env.api.history().length, 1);
  assert.ok(snapshots.length >= 4, 'subscribers receive history snapshots after changes');
  env.api.clear();
  assert.equal(env.api.history().length, 0);
  assert.equal(session.getItem(historyKey), null);
}

async function testInitialAuthCheckKeepsArrivingNotifications() {
  const scope = '8'.repeat(64);
  const session = new Storage();
  session.setItem(historyKey, JSON.stringify({ version: 1, scope, records: [storedRecord('n-1-saved', 'Saved before boot')] }));
  const env = makeRuntime({ auth: { enabled: true, authenticated: true, session_scope: scope }, session });
  const release = env.holdNextFetch();
  const initializing = env.api.init();
  const arriving = env.api.record({ title: 'Arrived during initial auth check', persist: false });
  assert.ok(arriving);
  assert.equal(env.api.history().length, 0, 'history stays hidden until initial scope verification completes');
  release();
  await initializing;
  assert.deepEqual(plain(env.api.history().map(item => item.title)), [
    'Arrived during initial auth check', 'Saved before boot'
  ], 'notifications arriving during initial auth verification remain in the live center');
  assert.deepEqual(JSON.parse(session.getItem(historyKey)).records.map(item => item.title), ['Saved before boot']);
}

async function testSessionScopeMismatchAndExpiredAuthClearHistory() {
  const scopeA = 'a'.repeat(64);
  const scopeB = 'b'.repeat(64);
  for (const auth of [
    { enabled: true, authenticated: true, session_scope: scopeB },
    { enabled: true, authenticated: false },
    { enabled: true, authenticated: true, session_scope: 'invalid' }
  ]) {
    const session = new Storage();
    session.setItem(historyKey, JSON.stringify({ version: 1, scope: scopeA, records: [storedRecord('n-1-private', 'Private')] }));
    const env = makeRuntime({ auth, session });
    await env.api.init();
    assert.equal(env.api.history().length, 0);
    assert.equal(session.getItem(historyKey), null);
  }

  const session = new Storage();
  session.setItem(historyKey, JSON.stringify({ version: 1, scope: scopeA, records: [storedRecord('n-1-private', 'Private')] }));
  const env = makeRuntime({ auth: { enabled: true, authenticated: true, session_scope: scopeA }, session });
  await env.api.init();
  assert.equal(env.api.history().length, 1, 'same authenticated scope restores');
  env.document.emit('aurago:auth-ended');
  assert.equal(env.api.history().length, 0);
  assert.equal(session.getItem(historyKey), null);
  assert.equal(env.api.status().authMode, 'expired');
}

async function testStorageFailureUsesMemoryAndReportsFallback() {
  const session = new Storage();
  session.failWrite = true;
  const local = new Storage();
  local.failWrite = true;
  const env = makeRuntime({ session, local });
  await env.api.init();
  env.api.record({ title: 'Temporary notice', message: 'Only in this tab' });
  assert.equal(env.api.history().length, 1);
  assert.equal(env.api.status().storage, 'memory');
  assert.equal(env.api.status().storageFallback, true);
  assert.equal(env.api.status().tabOnly, true);
  env.api.setPolicy({ dnd: true });
  assert.equal(env.api.status().policyStorage, 'memory');
  assert.equal(env.api.status().storageFallback, true);
}

async function testAuthStatusFailureNeverRestoresOldHistory() {
  const session = new Storage();
  session.setItem(historyKey, JSON.stringify({ version: 1, scope: 'local', records: [storedRecord('n-1-private', 'Old')] }));
  const env = makeRuntime({ session, fetchError: true });
  assert.equal(env.api.history().length, 0);
  await env.api.init();
  assert.equal(env.api.history().length, 0);
  const entry = env.api.record({ title: 'Fresh volatile' });
  assert.ok(entry);
  assert.equal(env.api.history().length, 1);
  assert.equal(env.api.status().authMode, 'unknown');
  assert.equal(env.api.status().storage, 'memory');
  assert.equal(session.getItem(historyKey).includes('Old'), true, 'unverified old data is left untouched and never rendered');
  assert.equal(env.requests[0][0], '/api/auth/status');
  assert.equal(env.requests[0][1].credentials, 'same-origin');
  assert.equal(env.requests[0][1].cache, 'no-store');
  assert.equal(env.requests[0][1].signal.aborted, false);
}

async function testAuthScopeRevalidationOnResumeAndVisibility() {
  const scopeA = 'a'.repeat(64);
  const scopeB = 'b'.repeat(64);
  const scopeC = 'c'.repeat(64);
  const session = new Storage();
  session.setItem(historyKey, JSON.stringify({ version: 1, scope: scopeA, records: [storedRecord('n-1-old', 'Old session')] }));
  const env = makeRuntime({ auth: { enabled: true, authenticated: true, session_scope: scopeA }, session });
  await env.api.init();
  const beforeCheck = env.api.record({ title: 'Before check', persist: false });
  assert.ok(beforeCheck);

  const release = env.holdNextFetch();
  const resumed = env.window.emitAsync('pageshow', { persisted: true });
  assert.equal(env.api.history().length, 0, 'old history stays hidden while a restored page revalidates its session');
  const arriving = env.api.record({ title: 'Arrived during check', persist: false });
  assert.ok(arriving, 'a notification arriving during revalidation is retained');
  env.setAuth({ enabled: true, authenticated: true, session_scope: scopeB });
  release();
  await resumed;
  assert.deepEqual(plain(env.api.history().map(item => item.title)), ['Arrived during check'], 'a new login receives only fresh in-flight notifications');
  assert.equal(session.getItem(historyKey), null, 'the old session envelope is removed on scope mismatch');

  env.advanceTime(3000);
  env.setAuth({ enabled: true, authenticated: true, session_scope: scopeC });
  await env.document.emitAsync('visibilitychange');
  assert.equal(env.api.history().length, 0, 'visible-session changes also discard prior-scope history');
  assert.equal(env.api.status().authMode, 'authenticated');
  assert.equal(env.requests.length, 3, 'resume and visible transitions recheck auth status');
}

async function testSameScopeRevalidationKeepsLiveOnlyEntries() {
  const scope = 'd'.repeat(64);
  const session = new Storage();
  session.setItem(historyKey, JSON.stringify({ version: 1, scope, records: [storedRecord('n-1-saved', 'Saved')] }));
  const env = makeRuntime({ auth: { enabled: true, authenticated: true, session_scope: scope }, session });
  await env.api.init();
  env.api.record({ title: 'Live only before check', persist: false });
  env.advanceTime(3000);

  const release = env.holdNextFetch();
  const resumed = env.window.emitAsync('pageshow', { persisted: true });
  env.api.record({ title: 'Arrived during same-scope check', persist: false });
  env.setAuth({ enabled: true, authenticated: true, session_scope: scope });
  release();
  await resumed;

  assert.deepEqual(plain(env.api.history().map(item => item.title)), [
    'Arrived during same-scope check', 'Live only before check', 'Saved'
  ], 'same-session revalidation preserves live-only and persisted history');
  const persisted = JSON.parse(session.getItem(historyKey));
  assert.deepEqual(persisted.records.map(item => item.title), ['Saved'], 'live-only entries stay out of sessionStorage');
}

async function testRevalidationFailureKeepsOnlyFreshMemoryEntries() {
  const scope = 'e'.repeat(64);
  const session = new Storage();
  session.setItem(historyKey, JSON.stringify({ version: 1, scope, records: [storedRecord('n-1-old', 'Old session')] }));
  const env = makeRuntime({ auth: { enabled: true, authenticated: true, session_scope: scope }, session });
  await env.api.init();
  env.advanceTime(3000);

  const release = env.holdNextFetch();
  const resumed = env.window.emitAsync('pageshow', { persisted: true });
  env.api.record({ title: 'Fresh volatile during check' });
  env.setFetchError(true);
  release();
  await resumed;

  assert.deepEqual(plain(env.api.history().map(item => item.title)), ['Fresh volatile during check']);
  assert.equal(env.api.status().authMode, 'unknown');
  assert.equal(env.api.status().storage, 'memory');
  assert.equal(session.getItem(historyKey).includes('Old session'), true, 'unverified stored data remains hidden until a later scope check');
}

async function testAuthStatusTimeoutDoesNotBlockDesktopOrRestoreHistory() {
  const scope = 'f'.repeat(64);
  const session = new Storage();
  session.setItem(historyKey, JSON.stringify({ version: 1, scope, records: [storedRecord('n-1-old', 'Old session')] }));
  const env = makeRuntime({ auth: { enabled: true, authenticated: true, session_scope: scope }, session });
  await env.api.init();
  env.advanceTime(3000);
  env.holdNextFetch();
  env.timeoutNextAuthCheck();

  await env.window.emitAsync('pageshow', { persisted: true });
  assert.equal(env.api.history().length, 0, 'a timed-out scope check never exposes the old history');
  assert.equal(env.api.status().authMode, 'unknown');
  assert.equal(env.api.status().storage, 'memory');
  assert.equal(env.requests.at(-1)[1].signal.aborted, true, 'the five-second timeout aborts the auth request');
}

async function testInitialAuthStatusTimeoutAllowsDesktopBoot() {
  const scope = '9'.repeat(64);
  const session = new Storage();
  session.setItem(historyKey, JSON.stringify({ version: 1, scope, records: [storedRecord('n-1-old', 'Old session')] }));
  const env = makeRuntime({ auth: { enabled: true, authenticated: true, session_scope: scope }, session });
  env.holdNextFetch();
  env.timeoutNextAuthCheck();

  await env.api.init();
  assert.equal(env.api.status().ready, true, 'initialization finishes after the bounded auth check times out');
  assert.equal(env.api.history().length, 0, 'initial timeout never restores unverified session history');
  assert.equal(env.api.status().authMode, 'unknown');
  assert.equal(env.api.status().storage, 'memory');
  assert.equal(env.requests[0][1].signal.aborted, true);
}

async function testInitialAuthBodyTimeoutAllowsDesktopBoot() {
  const scope = '7'.repeat(64);
  const session = new Storage();
  session.setItem(historyKey, JSON.stringify({ version: 1, scope, records: [storedRecord('n-1-old', 'Old session')] }));
  const env = makeRuntime({ auth: { enabled: true, authenticated: true, session_scope: scope }, session });
  const releaseBody = env.holdNextBody();
  env.timeoutNextAuthCheck();

  await env.api.init();
  releaseBody();
  assert.equal(env.api.status().ready, true, 'initialization finishes when auth response body decoding times out');
  assert.equal(env.api.history().length, 0, 'body timeout never restores unverified session history');
  assert.equal(env.api.status().authMode, 'unknown');
  assert.equal(env.api.status().storage, 'memory');
  assert.equal(env.requests[0][1].signal.aborted, true, 'the timeout aborts a response whose body is still pending');
}

async function testWebSocketAndSSEEventsStillDeduplicate() {
  const source = eventSource.slice(eventSource.indexOf('    async function handleDesktopEvent('), eventSource.indexOf('    function showDesktopNotification('));
  const shown = [];
  const context = { Date, Map, Set, showDesktopNotification: value => shown.push(value.message) };
  vm.createContext(context);
  vm.runInContext(`${source}; globalThis.handle = handleDesktopEvent;`, context);
  const event = { id: 'evt-1', type: 'notification', payload: { message: 'Once' } };
  await context.handle(event);
  await context.handle(JSON.parse(JSON.stringify(event)));
  await context.handle({ type: 'notification', payload: { message: 'Direct one' } });
  await context.handle({ type: 'notification', payload: { message: 'Direct two' } });
  assert.deepEqual(shown, ['Once', 'Direct one', 'Direct two']);
}

await testPolicyDefaultsAndDelivery();
await testCrossTabPolicySync();
await testHistoryRestoreSanitizationAndActions();
await testInitialAuthCheckKeepsArrivingNotifications();
await testSessionScopeMismatchAndExpiredAuthClearHistory();
await testStorageFailureUsesMemoryAndReportsFallback();
await testAuthStatusFailureNeverRestoresOldHistory();
await testAuthScopeRevalidationOnResumeAndVisibility();
await testSameScopeRevalidationKeepsLiveOnlyEntries();
await testRevalidationFailureKeepsOnlyFreshMemoryEntries();
await testAuthStatusTimeoutDoesNotBlockDesktopOrRestoreHistory();
await testInitialAuthStatusTimeoutAllowsDesktopBoot();
await testInitialAuthBodyTimeoutAllowsDesktopBoot();
await testWebSocketAndSSEEventsStillDeduplicate();
console.log('Desktop notification policy/history checks passed.');
