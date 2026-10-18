import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function read(relativePath) {
  return readFileSync(path.join(root, relativePath), 'utf8').replace(/\r\n?/g, '\n');
}

function sourceBetween(source, startMarker, endMarker) {
  const start = source.indexOf(startMarker);
  const end = source.indexOf(endMarker, start + startMarker.length);
  assert.notEqual(start, -1, `missing source marker ${startMarker}`);
  assert.notEqual(end, -1, `missing source marker ${endMarker}`);
  return source.slice(start, end);
}

function testDesktopRecentFilesExcludeDirectoryContexts() {
  const source = read('ui/js/desktop/core/session-runtime.js');
  const helperSource = source.slice(source.indexOf("const RECENT_FILES_KEY ="));
  const stored = new Map([['aurago.desktop.recentFiles.v2', [
    { path: 'desktop', name: 'desktop', appId: 'files', openedAt: 1 },
    { path: 'projects/demo', name: 'demo', appId: 'terminal', openedAt: 2 },
    { path: 'documents/report.md', name: 'stale', appId: 'notes', openedAt: 3 }
  ]]]);
  const context = {
    Set,
    Date,
    JSON,
    normalizeDesktopPath: value => String(value || '').replace(/\\/g, '/').replace(/\/+/g, '/').replace(/^\.\//, '').trim(),
    pathBaseName: value => String(value || '').split('/').filter(Boolean).pop() || '',
    readJSONStorage: (key, fallback) => stored.has(key) ? stored.get(key) : fallback,
    writeJSONStorage: (key, value) => stored.set(key, value)
  };
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.readRecentFilesForTest = readRecentFiles; globalThis.recordRecentFileForTest = recordRecentFile;`, context);

  assert.deepEqual(Array.from(context.readRecentFilesForTest(), entry => ({ path: entry.path, kind: entry.kind })), [
    { path: 'documents/report.md', kind: 'file' }
  ]);
  context.recordRecentFileForTest('documents', 'files', 'directory');
  context.recordRecentFileForTest('documents/guide.md', 'viewer');
  assert.deepEqual(Array.from(context.readRecentFilesForTest(), entry => entry.path), [
    'documents/guide.md',
    'documents/report.md'
  ]);
  assert.equal(stored.get('aurago.desktop.recentFiles.v2').some(entry => entry.path === 'documents'), false);

  const shell = read('ui/js/desktop/core/window-shell-runtime.js');
  assert.match(shell, /recordRecentFile\(context\.path, appId, context\.pathKind\)/);
  assert.match(shell, /if \(!sessionRestore && windowContext\.path\) recordRecentFile\(windowContext\.path, appId, windowContext\.pathKind\)/);
  const gameMaker = read('ui/js/desktop/apps/game-maker-studio.js');
  assert.match(gameMaker, /openApp\('code-studio', \{ path: state\.project\.project_key, pathKind: 'directory' \}\)/);
}

async function testBrowserAudioLeaseUsesExclusiveWebLock() {
  const source = read('ui/js/shared/browser-audio-lease.js');
  const values = new Map();
  let lockHeld = false;
  let nextID = 0;
  const locks = {
    request(_name, options, callback) {
      assert.equal(options.mode, 'exclusive');
      assert.equal(options.ifAvailable, true);
      if (lockHeld) return Promise.resolve(callback(null));
      lockHeld = true;
      return Promise.resolve(callback({ name: 'aurago-browser-audio' })).finally(() => {
        lockHeld = false;
      });
    }
  };
  const localStorage = {
    getItem(key) { return values.has(key) ? values.get(key) : null; },
    setItem(key, value) { values.set(key, String(value)); },
    removeItem(key) { values.delete(key); }
  };
  function loadLeaseRuntime() {
    const sessionValues = new Map();
    const window = {
      crypto: { randomUUID: () => `lease-${++nextID}` },
      addEventListener() {}
    };
    const context = {
      window,
      navigator: { locks },
      localStorage,
      sessionStorage: {
        getItem(key) { return sessionValues.has(key) ? sessionValues.get(key) : null; },
        setItem(key, value) { sessionValues.set(key, String(value)); }
      },
      BroadcastChannel: class {
        addEventListener() {}
        postMessage() {}
      },
      Uint8Array,
      Array,
      Date,
      JSON,
      String,
      Number,
      Error,
      setInterval,
      clearInterval,
      setTimeout
    };
    vm.runInNewContext(source, context);
    return window.AuraBrowserAudioLease;
  }

  const first = loadLeaseRuntime();
  const second = loadLeaseRuntime();
  const firstLease = await first.acquire('sip-phone', 'first-tab');
  await assert.rejects(
    second.acquire('realtime-speech', 'second-tab'),
    error => error && error.code === 'audio_session_busy'
  );
  first.release(firstLease.token);
  await Promise.resolve();
  await Promise.resolve();
  const secondLease = await second.acquire('realtime-speech', 'second-tab');
  second.release(secondLease.token);
}

function testVersionedServiceWorkerRegistration() {
  const shared = read('ui/js/shared/shared-core.js');
  const helperSource = sourceBetween(shared, 'function serviceWorkerURL()', 'async function initPWA()');
  const context = {
    URL,
    encodeURIComponent,
    window: { AURAGO_BUILD_VERSION: 'build 42' },
    document: { querySelector: () => null }
  };
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.result = serviceWorkerURL();`, context);
  assert.equal(context.result, '/sw.js?v=build%2042');

  const initPWA = sourceBetween(shared, 'async function initPWA()', 'async function _subscribePush');
  // Registration uses a versioned URL helper and may retry once after a transient failure.
  assert.match(initPWA, /const swURL = serviceWorkerURL\(\);/);
  assert.match(initPWA, /navigator\.serviceWorker\.register\(swURL,\s*\{\s*updateViaCache:\s*'none'\s*\}\)/);
  assert.match(initPWA, /await new Promise\(resolve => setTimeout\(resolve, 1500\)\)/);
  assert.doesNotMatch(initPWA, /register\('\/sw\.js'\)/);

  const chat = read('ui/index.html');
  assert.match(chat, /data-sw-url="\/sw\.js\?v=\{\{\.BuildVersion\}\}"/);
  const chatRegistration = read('ui/js/shared/register-sw.js');
  const registrations = [];
  vm.runInNewContext(chatRegistration, {
    console,
    location: { protocol: 'https:' },
    navigator: {
      serviceWorker: {
        register(url, options) {
          registrations.push({ url, options });
          return Promise.resolve({ scope: '/' });
        }
      }
    },
    document: { currentScript: { dataset: { swUrl: '/sw.js?v=chat-build' } } }
  });
  assert.equal(registrations.length, 1);
  assert.equal(registrations[0].url, '/sw.js?v=chat-build');
  assert.equal(registrations[0].options.updateViaCache, 'none');
}

function testAuraSSEResumesWhenVisibleAfterFrozenConnection() {
  const shared = read('ui/js/shared/shared-core.js');
  const source = sourceBetween(shared, 'window.AuraSSE = (function ()', '// TAILSCALE LOGIN WATCHER');
  const sources = [];
  const documentListeners = {};
  const windowListeners = {};
  class FakeEventSource {
    constructor(url) {
      this.url = url;
      this.readyState = FakeEventSource.CONNECTING;
      sources.push(this);
    }
    close() {
      this.readyState = FakeEventSource.CLOSED;
      this.closed = true;
    }
  }
  FakeEventSource.CONNECTING = 0;
  FakeEventSource.OPEN = 1;
  FakeEventSource.CLOSED = 2;

  const document = {
    visibilityState: 'visible',
    addEventListener(type, fn) {
      (documentListeners[type] = documentListeners[type] || []).push(fn);
    }
  };
  const window = {
    location: { pathname: '/' },
    navigator: { standalone: true },
    matchMedia(query) {
      return { matches: query === '(display-mode: standalone)' };
    },
    AuraDisposer: { add() {} },
    addEventListener(type, fn) {
      (windowListeners[type] = windowListeners[type] || []).push(fn);
    }
  };
  const context = {
    window,
    document,
    EventSource: FakeEventSource,
    localStorage: { getItem: () => 's1' },
    encodeURIComponent,
    setTimeout() { return 0; },
    clearTimeout() {},
    fetch() { return Promise.resolve({ ok: true, status: 200, json: async () => ({}) }); }
  };
  vm.runInNewContext(`${source}\nthis.AuraSSE = window.AuraSSE;`, context);

  assert.ok(documentListeners.visibilitychange, 'PWA resume must listen for visibilitychange');
  assert.ok(windowListeners.pageshow, 'PWA resume must listen for pageshow');
  assert.ok(windowListeners.online, 'PWA resume must listen for online');
  assert.ok(documentListeners.freeze, 'frozen PWAs must drop the dead EventSource');

  context.AuraSSE.connect();
  assert.equal(sources.length, 1);
  sources[0].readyState = FakeEventSource.OPEN;
  sources[0].onopen();
  assert.equal(context.AuraSSE.isConnected(), true);

  document.visibilityState = 'hidden';
  documentListeners.visibilitychange.forEach(fn => fn({ type: 'visibilitychange' }));
  assert.equal(sources.length, 1, 'backgrounding must not tear down a browser EventSource');

  document.visibilityState = 'visible';
  documentListeners.visibilitychange.forEach(fn => fn({ type: 'visibilitychange' }));
  assert.equal(sources.length, 2, 'foregrounding a standalone PWA must open a new EventSource');
  assert.equal(sources[0].closed, true);
  assert.match(sources[1].url, /session_id=s1/);
}

function gameMakerOutcomeCard(context, state) {
  let card;
  const conversation = { querySelector: () => null, appendChild(value) { card = value; } };
  context.scrollConversation = () => {};
  context.renderOutcomeCard({ ...state, container: {
    querySelector: selector => selector === '[data-gm-conversation]' ? conversation : null
  } });
  return card;
}

async function testGameMakerEventConnectionLifecycle() {
  const app = read('ui/js/desktop/apps/game-maker-studio.js');
  const sources = [], statuses = [], phases = {};
  const context = {
    EventSource: class { constructor() { sources.push(this); } addEventListener() {} close() { this.closed = true; } },
    document: { createElement: () => ({ setAttribute() {} }) },
    eventTypes: ['job_status'], terminalStatuses: new Set(['ready', 'failed', 'cancelled']),
    updateStatus(_state, status) { statuses.push(status); }, syncJobControls() {}, finalizeStreaming() {}, stopElapsed() {},
    phaseMarkup(_state, status) { return status; }, renderProjects() {},
    async reloadProjectRecord() {}, refreshWorkspaceDrift() {}
  };
  vm.createContext(context);
  vm.runInContext(sourceBetween(app, 'const outcomeFields =', 'function stepBarShown('), context);
  for (const [start, end] of [['function connectEvents(', 'function parseEvent('], ['function closeEvents(', 'function dispose('], ['function handleJobStatus(', 'function finalizeStreaming(']]) {
    vm.runInContext(sourceBetween(app, start, end), context);
  }
  const state = { project: { id: 'bo', status: 'draft' }, job: { id: 'job', status: 'building' },
    api: { eventURL: () => '/events' }, container: { querySelector: selector => selector === '[data-gm-phases]' ? phases : null }, context: { t: key => key, esc: String } };
  context.connectEvents(state);
  const first = sources[0];
  first.onerror();
  assert.equal(statuses.at(-1), 'reconnecting');
  first.onopen();
  assert.equal(statuses.at(-1), 'building', 'open without replayed events restores job state');
  assert.equal(state.reconnecting, false);
  context.handleJobStatus(state, { status: 'cancelled', error: 'Game creation exceeded its time limit.' });
  await Promise.resolve();
  assert.equal(phases.innerHTML, 'cancelled', 'terminal jobs leave the active building phase');
  assert.equal(state.latestJob.status, 'cancelled');
  assert.equal(state.latestJob.error, 'Game creation exceeded its time limit.');
  const card = gameMakerOutcomeCard(context, state);
  assert.equal(card.className, 'gm-result-card is-cancelled');
  assert.ok(card.innerHTML.includes(state.latestJob.error), 'the result card retains the cancellation reason');
  for (const status of ['cancelled', 'failed', 'ready']) {
    state.job.status = status;
    first.onerror(); first.onopen();
    assert.deepEqual(statuses.slice(-2), [status, status], 'transport cannot obscure a terminal job');
  }
  context.closeEvents(state);
  assert.equal(first.closed, true);
  assert.equal(state.reconnecting, false);
  context.connectEvents(state);
  const count = statuses.length;
  first.onerror(); first.onopen();
  assert.equal(statuses.length, count, 'old project stream cannot update the new project');
  state.disposed = true;
  sources[1].onerror(); sources[1].onopen();
  assert.equal(statuses.length, count, 'disposed windows ignore late callbacks');

  // A project refresh must retain the terminal job badge and ignore old responses.
  context.setButton = () => {};
  state.disposed = false; state.capabilities = {}; state.job.status = 'cancelled';
  const meta = new Map([['[data-gm-title]', {}], ['[data-gm-engine]', {}]]);
  state.container.querySelector = selector => meta.get(selector) || null;
  vm.runInContext(sourceBetween(app, 'function renderProjectMeta(', 'function renderConversation('), context);
  context.renderProjectMeta(state);
  assert.equal(statuses.at(-1), 'cancelled', 'draft project metadata must not overwrite cancelled job');
  let resolve;
  state.api.getProject = () => new Promise(done => { resolve = done; });
  vm.runInContext(sourceBetween(app, 'async function reloadProjectRecord(', 'function appendAgentDelta('), context);
  const pending = context.reloadProjectRecord(state);
  state.project = { id: 'new-project' };
  resolve({ project: { id: 'bo' } });
  await pending;
  assert.equal(state.project.id, 'new-project');
}

async function testGameMakerJobEventOwnership() {
  const app = read('ui/js/desktop/apps/game-maker-studio.js');
  const listeners = new Map(), phases = {};
  const context = {
    EventSource: class { addEventListener(type, fn) { listeners.set(type, fn); } },
    document: { createElement: () => ({ setAttribute() {} }) },
    eventTypes: ['job_status', 'phase', 'preview_reload', 'validation_reset', 'stream_ready'],
    terminalStatuses: new Set(['ready', 'failed', 'cancelled']),
    updateStatus(state, status) { state.displayStatus = status; },
    finalizeStreaming() {}, stopElapsed() {}, phaseMarkup: (_state, phase) => phase,
    renderProjects() {}, async reloadProjectRecord() {},
    refreshWorkspaceDrift() {},
    refreshPreview(state) { state.reloads = (state.reloads || 0) + 1; }, window: {}
  };
  context.syncJobControls = state => { state.jobActive = !context.terminalStatuses.has(state.job.status); };
  vm.createContext(context);
  vm.runInContext(sourceBetween(app, 'const outcomeFields =', 'function stepBarShown('), context);
  vm.runInContext(sourceBetween(app, 'function connectEvents(', 'function finalizeStreaming('), context);
  const job = { id: 'new', status: 'building', created_at: '2026-01-02T00:00:00Z' };
  const state = { project: { id: 'project' }, job: { ...job }, activeJob: { job_id: 'new' }, jobActive: true,
    api: { eventURL: () => '/events' }, container: { querySelector: selector => selector === '[data-gm-phases]' ? phases : null }, context: { t: String, esc: String } };
  const emit = (type, id, payload) => listeners.get(type)({ data: JSON.stringify({ type, job_id: id, payload }) });
  context.connectEvents(state);
  for (const replay of [true, false]) {
    if (!replay) emit('stream_ready', '', {});
    emit('job_status', 'old', { status: 'queued', job: { id: 'old', created_at: '2026-01-01T00:00:00Z' } });
    emit('job_status', 'old', { status: 'ready', job: { id: 'old', status: 'ready' } });
    emit('phase', 'old', { phase: 'planning' });
    emit('preview_reload', 'old', {});
    emit('validation_reset', 'old', {});
    assert.equal(state.job.id, 'new');
    assert.equal(state.job.status, 'building');
    assert.equal(state.jobActive, true);
    assert.equal(state.activeJob.job_id, 'new');
    assert.equal(state.reloads, undefined);
    assert.equal(state.latestJob, undefined);
  }
  // A job started in another Studio window still advances this window.
  emit('job_status', 'next', { status: 'queued', job: { id: 'next', status: 'queued', created_at: '2026-01-03T00:00:00Z' } });
  emit('phase', 'next', { phase: 'building' });
  emit('job_status', 'new', { status: 'cancelled' });
  assert.equal(state.job.id, 'next');
  assert.equal(state.job.status, 'building');
  assert.equal(state.activeJob.job_id, 'next');
  emit('preview_reload', '', {});
  assert.equal(state.reloads, 1, 'project revision events without a job remain valid');
  emit('job_status', 'next', { status: 'ready' });
  await Promise.resolve();
  assert.equal(state.activeJob, null);
  assert.equal(state.jobActive, false);
  assert.equal(state.latestJob.status, 'ready');
  assert.equal(state.latestJob.id, 'next');
  assert.equal(gameMakerOutcomeCard(context, state).className, 'gm-result-card is-success');
  // An active job from capabilities has no timestamp until its own replayed start.
  state.job = { id: 'active', status: 'building' };
  context.connectEvents(state);
  emit('job_status', 'old', { status: 'queued', job: { id: 'old', created_at: '2026-01-01T00:00:00Z' } });
  assert.equal(state.job.id, 'active');
  emit('job_status', 'active', { status: 'queued', job: { id: 'active', status: 'queued', created_at: '2026-01-04T00:00:00Z' } });
  emit('phase', 'active', { phase: 'building' });
  emit('stream_ready', '', {});
  assert.equal(state.job.id, 'active');
  assert.equal(state.job.status, 'building');
  state.activeJob = { job_id: 'another-project-job', project_id: 'another-project' };
  emit('job_status', 'active', { status: 'ready' });
  assert.equal(state.activeJob.job_id, 'another-project-job', 'history must not clear another project writer');
}

async function testGameMakerVisualGrantRenewal() {
  const context = { window: {}, AbortController, setTimeout, clearTimeout };
  vm.runInNewContext(read('ui/js/desktop/apps/game-maker-studio-preview.js'), context);
  // Review is private; use the same capture response path as the iframe.
  for (const outcome of ['renewed', 'revision_changed', 'validation', 'replaced', 'disposed', 'cancelled', 'failed']) {
    const frame = { contentWindow: {} }, grant = { token: 'expired', revision: 3 };
    let resolveGrant, reviewCalls = 0;
    const status = {}, state = { frame, previewGrant: grant, project: { id: 'p' }, previewProjectID: 'p', channelID: 'c',
      visualCapture: { id: 'capture', manual: true, project: 'p', grant }, context: { t: String },
      container: { querySelector: selector => selector === '[data-gm-visual-status]' ? status : null },
      addDiagnostic() {}, api: {
        previewGrant: () => new Promise((resolve, reject) => { resolveGrant = outcome === 'failed' ? () => reject(new Error('offline')) : resolve; }),
        async reviewVisual(id, body) { reviewCalls++; assert.equal(id, 'p'); assert.equal(body.token, 'fresh'); return { status: 'reviewed' }; }
      } };
    context.window.GameMakerStudioPreview.handleMessage(state, { source: frame.contentWindow, data: {
      source: 'aurago-game', type: 'capture', channel: 'c', request_id: 'capture',
      captures: [{ image: 'data:image/png;base64,AA==', width: 1, height: 1 }]
    } });
    assert.equal(typeof resolveGrant, 'function');
    if (outcome === 'replaced') state.previewGrant = { token: 'other' };
    if (outcome === 'disposed') state.disposed = true;
    if (outcome === 'cancelled') context.window.GameMakerStudioPreview.cancelVisual(state);
    resolveGrant({ token: 'fresh', revision: outcome === 'revision_changed' ? 4 : 3, validation_id: outcome === 'validation' ? 'build' : '' });
    await new Promise(setImmediate);
    assert.equal(reviewCalls, outcome === 'renewed' ? 1 : 0, outcome);
    assert.equal(state.frame, frame, 'renewal must preserve the running frame');
    if (outcome === 'renewed') assert.equal(state.previewGrant, grant, 'renewal must preserve channel/save binding');
    if (['revision_changed', 'validation', 'failed'].includes(outcome)) assert.equal(status.textContent, 'game_maker.visual_analysis_failed');
    assert.equal(state.visualBusy, false);
  }
}

function testVoxelInputOwnership() {
  const source = read('internal/gamemaker/runtime/aurago-voxel-1.js');
  const keys = new Set(), inputs = new Map();
  const context = { keys, inputs, blocked: () => false, performance: { now: () => 0 }, jumpUntil: 0 };
  vm.createContext(context);
  vm.runInContext(sourceBetween(source, 'function action(', 'function respawn('), context);
  context.action('primary', true);
  context.action('primary', false, 'pointer:4');
  assert.equal(keys.has('primary'), true, 'ending look cannot end the Mine button');
  context.action('primary', true, 'keyboard:KeyE');
  context.action('primary', true, 'keyboard:KeyE');
  context.action('primary', false);
  assert.equal(keys.has('primary'), true, 'ending touch cannot end a held keyboard key');
  context.action('primary', false, 'keyboard:KeyE');
  assert.equal(keys.has('primary'), false, 'one keyup clears repeated keydowns');
  assert.equal(inputs.size, 0);
  context.blocked = () => true;
  context.action('primary', true);
  assert.equal(keys.size, 0, 'paused games cannot acquire input');
}

async function testGameMakerPreviewDiagnosticsReachValidationAndNextRequest() {
  const app = read('ui/js/desktop/apps/game-maker-studio.js');
  const sent = [];
  const shown = [];
  let submitted;
  const state = {
    frame: { contentWindow: { postMessage() {} } }, project: { id: 'snake' }, channelID: 'channel',
    container: { querySelector: () => null },
    previewProjectID: 'snake', previewGrant: { token: 'parent-only-token', validation_id: 'build', expires_at: new Date(Date.now() + 60000).toISOString() },
    previewReported: new Set(), previewDiagnostics: [], messages: [], selectedAssetPackIDs: ['space-shooter'],
    api: {
      async reportPreview(id, report) { sent.push({ id, ...report }); },
      async startJob(id, body) { submitted = { id, ...body }; return { id: 'job', status: 'queued' }; }
    }
  };
  const context = {
    window: {}, addDiagnostic(_state, diagnostic) { shown.push(diagnostic); },
    setTimeout() { return 1; }, clearTimeout() {},
    autoGrow() {}, finalizeStreaming() {}, renderConversation() {}, scrollConversation() {}, syncJobControls() {}
  };
  vm.runInNewContext(sourceBetween(app, 'function handlePreviewMessage(', 'function addDiagnostic('), context);
  vm.runInNewContext(read('ui/js/desktop/apps/game-maker-studio-preview.js'), context);
  state.addDiagnostic = diagnostic => context.addDiagnostic(state, diagnostic);
  vm.runInNewContext(sourceBetween(app, 'async function submitChange(', 'async function stopJob('), context);
  vm.runInNewContext(sourceBetween(app, 'function postedSelection(', 'function showCreateModal('), context);
  const data = { source: 'aurago-game', channel: 'channel', type: 'runtime_error',
    message: 'Uncaught TypeError: this.scale.setSize is not a function' };
  const event = { source: state.frame.contentWindow, data };
  context.handlePreviewMessage(state, { ...event, source: {} });
  context.handlePreviewMessage(state, { ...event, data: { ...data, channel: 'old' } });
  context.handlePreviewMessage(state, { ...event, data: { ...data, source: 'foreign' } });
  assert.equal(sent.length, 0, 'foreign iframe reports must stay outside the API');
  context.handlePreviewMessage(state, { ...event, data: { ...data, type: 'ready' } });
  assert.equal(sent.length, 0, 'game-authored ready must not qualify an invisible canvas');
  context.handlePreviewMessage(state, { ...event, data: { ...data, type: 'ready', boot: true, visible: true } });
  context.handlePreviewMessage(state, event);
  context.handlePreviewMessage(state, event);
  assert.equal(sent.length, 2, 'send ready plus one deduplicated runtime error');
  assert.equal(sent[1].token, 'parent-only-token');
  assert.equal(sent[1].message, data.message);
  assert.equal(sent[0].canvas_visible, true);
  assert.equal(shown.length, 1);
  const input = { value: 'Fix the snake', disabled: false };
  const button = {};
  const form = { querySelector(selector) { return selector.includes('button') ? button : input; } };
  state.container = { querySelector: selector => selector === '[data-gm-change-form]' ? form : null };
  await context.submitChange(state);
  assert.equal(submitted.preview_diagnostics[0].message, data.message);
  assert.equal(submitted.prompt, 'Fix the snake');
  assert.deepEqual(submitted.asset_pack_ids, ['space-shooter']);
  state.project = { id: 'other-project' };
  context.handlePreviewMessage(state, { ...event, data: { ...data, message: 'late old error' } });
  assert.equal(sent.length, 2, 'a switched project must not receive old preview reports');
}

async function testGameMakerAssetSelectionReachesCreateAndEdit() {
  const app = read('ui/js/desktop/apps/game-maker-studio.js');
  for (const packs of [[], ['space-shooter', 'platformer']]) {
    const submitted = [];
    let create;
    const input = { value: 'Change the game' }, button = {};
    const fieldsBySelector = new Map([
      ['textarea', input], ['button[type="submit"]', button],
      ['select[name="provider_id"]', { addEventListener() {} }]
    ]);
    const form = {
      querySelector: selector => fieldsBySelector.get(selector) || null,
      addEventListener(type, handler) { if (type === 'submit') create = handler; }
    };
    const layer = { querySelector: selector => selector === '[data-gm-create]' ? form : null, querySelectorAll: () => [] };
    const project = { id: 'new-game', dimension: '2d' };
    const state = {
      capabilities: {}, context: { esc: String, t: String }, projects: [], messages: [],
      selectedAssetPackIDs: [...packs],
      container: { querySelector: selector => selector === '[data-gm-change-form]' ? form : null },
      api: {
        async createProject() { return project; },
        async startJob(id, body) { submitted.push({ id, body }); return { id: 'job', status: 'queued' }; }
      }
    };
    const fields = { name: 'New game', description: 'Create the game', dimension: '2d' };
    const context = {
      window: {}, FormData: class { get(key) { return fields[key] || ''; } },
      capabilityState: () => ({ status: 'ready' }), mediaToggle: () => '',
      showModal(_state, _html, mount) { mount(layer); }, closeModal() {}, setModalBusy() {},
      modalError(_layer, message) { assert.fail(message); }, fail(_state, error) { throw error; },
      async openProject(current) { current.project = project; },
      syncJobControls() {}, autoGrow() {}, finalizeStreaming() {}, renderConversation() {}, scrollConversation() {}
    };
    vm.createContext(context);
    vm.runInContext(read('ui/js/desktop/apps/game-maker-studio-assets.js'), context);
    vm.runInContext(sourceBetween(app, 'function postedSelection(', 'async function stopJob('), context);
    context.showCreateModal(state);
    await create({ preventDefault() {} });
    assert.equal(submitted.length, 1, 'create starts exactly one job');
    assert.equal(submitted[0].id, project.id);
    assert.deepEqual(Array.from(submitted[0].body.asset_pack_ids), packs, 'create sends selected packs');
    assert.equal(submitted[0].body.prompt, fields.description);

    await context.submitChange(state);
    assert.equal(submitted.length, 2, 'edit starts exactly one job');
    assert.deepEqual(Array.from(submitted[1].body.asset_pack_ids), packs, 'edit sends selected packs');
    assert.equal(submitted[1].body.prompt, 'Change the game');
    assert.equal(submitted[1].body.resume, false);
    state.selectedAssetPackIDs.push('later-selection');
    for (const { body } of submitted) {
      assert.deepEqual(Array.from(body.asset_pack_ids), packs, 'submitted selection is detached from later edits');
    }
  }
}

async function testGameMakerSpriteBrowserOwnsSelectionAndCleanup() {
  const mobile = {};
  const mobileContext = {};
  vm.runInNewContext(sourceBetween(read('ui/js/desktop/apps/game-maker-studio.js'), 'function renderMobileSelect(', 'function relativeTime('), mobileContext);
  mobileContext.renderMobileSelect({ context: { esc: value => String(value).replaceAll('<', '&lt;') }, projects: [{id:'one',name:'<Ranger>'}], project:{id:'one'},container:{querySelector:()=>mobile} });
  assert.ok(mobile.innerHTML.includes('&lt;Ranger>'));
  const nodes = new Map();
  function node(key) {
    if (!nodes.has(key)) nodes.set(key, { value: '', innerHTML: '', handlers: {},
      addEventListener(type, fn) { this.handlers[type] = fn; } });
    return nodes.get(key);
  }
  const layer = { querySelector: node };
  node('[data-asset-kind]').value = 'all';
  const selection = {};
  let finishCatalog, finishDetail, requestSignal, starts = 0;
  const state = { context: { t: key => key, esc: String }, selectedAssetPackIDs: [],
    container: { querySelectorAll: () => [selection] }, api: {
      assetPacks(options) { requestSignal = options.signal; return new Promise(resolve => { finishCatalog = resolve; }); },
      assetPack() { return new Promise(resolve => { finishDetail = resolve; }); },
      assetPackImageURL: id => '/packs/' + id, startJob() { starts++; }
    } };
  let cleared = 0;
  const context = { window: {}, AbortController, clearInterval() { cleared++; } };
  vm.runInNewContext(read('ui/js/desktop/apps/game-maker-studio-assets.js'), context);
  const assets = context.window.GameMakerStudioAssets;
  assets.show(state, { showModal(_state, _html, mount) { mount(layer); }, modalError() { assert.fail('late request touched closed modal'); } });
  finishCatalog({ packs: [{ id: 'space-shooter', kind: 'sprite2d', tags: ['space'], description: 'Ships' }] });
  await Promise.resolve();
  const cards = node('[data-asset-cards]');
  assert.match(cards.innerHTML, /data-pack="space-shooter"/);
  assert.equal(typeof finishDetail, 'function', 'the first sprite pack must load its detail');
  cards.handlers.change({ target: { dataset: { selectPack: 'space-shooter' }, checked: true } });
  assert.equal(state.selectedAssetPackIDs.join(), 'space-shooter');
  assert.ok(selection.textContent.includes('pack_space_shooter'));
  assert.equal(starts, 0, 'selection must never start a job');
  assert.ok(assets.selectionMarkup({ ...state, selectedAssetPackIDs: [] }).includes('assets_automatic'));
  state.assetBrowserCleanup();
  const closedHTML = node('[data-asset-detail]').innerHTML;
  finishDetail({ id: 'space-shooter' });
  await Promise.resolve();
  assert.equal(node('[data-asset-detail]').innerHTML, closedHTML);
  assert.equal(requestSignal.aborted, true);
  assert.ok(cleared > 0);
  assert.equal(state.assetBrowserCleanup, null);
  assets.clearSelection(state);
  assert.equal(state.selectedAssetPackIDs.length, 0);
}

async function testGameMakerDiagnosticsFollowPreviewLifetime() {
  const app = read('ui/js/desktop/apps/game-maker-studio.js');
  const nodes = new Map();
  const panel = { open: false, classList: {
    toggle(_name, value) { panel.hasErrors = value; },
    remove() { panel.hasErrors = false; }
  } };
  nodes.set('[data-gm-diagnostics]', panel);
  nodes.set('[data-gm-preview]', { replaceChildren() {}, querySelector: () => null });
  nodes.set('[data-gm-diagnostic-list]', {});
  nodes.set('[data-gm-diagnostic-count]', {});
  const state = {
    project: { id: 'snake' }, diagnostics: [],
    context: { esc: String, t: String },
    container: { querySelector: selector => nodes.get(selector) || null },
    api: { async previewGrant() { return { url: '/preview', validation_id: 'build', expires_at: new Date(Date.now() + 60000).toISOString() }; } }
  };
  let nextChannel = 0;
  const context = {
    window: {}, crypto: { getRandomValues: () => [++nextChannel] },
    setTimeout() { return 1; }, clearTimeout() {},
    document: { createElement: () => ({ contentWindow: { postMessage() {} }, setAttribute() {}, addEventListener() {} }) },
    IntersectionObserver: class { observe() {} disconnect() {} }
  };
  vm.runInNewContext(sourceBetween(app, 'async function preparePreviewReplacement(', 'function showCreateModal('), context);
  vm.runInNewContext(read('ui/js/desktop/apps/game-maker-studio-preview.js'), context);
  // Loading overlays have their own DOM/lifecycle test below.
  context.window.GameMakerStudioPreview.showLoading = () => {};
  state.addDiagnostic = diagnostic => context.addDiagnostic(state, diagnostic);
  await context.refreshPreview(state);
  assert.equal(state.diagnostics.length, 0, 'preview setup must succeed before testing late reports');
  let rejectReport;
  state.api.reportPreview = () => new Promise((_resolve, reject) => { rejectReport = reject; });
  const oldEvent = { source: state.frame.contentWindow, data: {
    source: 'aurago-game', channel: state.channelID, type: 'runtime_error', message: 'Old image error'
  } };
  context.handlePreviewMessage(state, oldEvent);
  assert.equal(panel.hasErrors, true);
  assert.equal(state.previewDiagnostics.length, 1);
  await context.refreshPreview(state);
  rejectReport(new Error('Late report failure'));
  await Promise.resolve();
  context.handlePreviewMessage(state, oldEvent);
  assert.equal(state.diagnostics.length, 0);
  assert.equal(state.previewDiagnostics.length, 0);
  assert.equal(nodes.get('[data-gm-diagnostic-list]').innerHTML, '');
  assert.equal(nodes.get('[data-gm-diagnostic-count]').textContent, '0');
  assert.equal(panel.hasErrors, false);
  assert.equal(panel.open, false);
  state.api.reportPreview = async () => {};
  const currentEvent = { ...oldEvent, source: state.frame.contentWindow,
    data: { ...oldEvent.data, channel: state.channelID } };
  context.handlePreviewMessage(state, currentEvent);
  context.handlePreviewMessage(state, { ...currentEvent,
    data: { ...currentEvent.data, type: 'ready', boot: true, visible: true } });
  assert.equal(state.diagnostics.length, 1, 'ready must preserve errors from the current run');
  assert.equal(panel.open, true);
  let rejectGrant;
  state.api.previewGrant = () => new Promise((_resolve, reject) => { rejectGrant = reject; });
  const staleRefresh = context.refreshPreview(state);
  await new Promise(setImmediate); // Let the save/flush gate obtain the old grant before replacing it.
  state.api.previewGrant = async () => ({ url: '/new-preview' });
  await context.refreshPreview(state);
  rejectGrant(new Error('Late grant failure'));
  await staleRefresh;
  assert.equal(state.diagnostics.length, 0, 'old grant failures must not contaminate the new preview');
  context.addDiagnostic(state, { level: 'runtime', message: 'Current error' });
  state.api.previewGrant = async () => { throw new Error('Current grant failure'); };
  await context.refreshPreview(state);
  assert.equal(state.diagnostics.length, 2, 'failed reload keeps existing diagnostics and reports its failure');
  context.clearDiagnostics(state);
  assert.equal(nodes.get('[data-gm-diagnostic-count]').textContent, '0');
  assert.match(sourceBetween(app, 'async function openProject(', 'function renderProject('), /clearDiagnostics\(state\)/);
}

async function testGameMakerPreviewStopsExpiredValidation() {
  const window = {};
  vm.runInNewContext(read('ui/js/desktop/apps/game-maker-studio-preview.js'), { window });
  const shown = [], sent = [];
  let reject;
  const state = {
    frame: { contentWindow: {} }, project: { id: 'breakout' }, previewProjectID: 'breakout',
    channelID: 'channel', job: { status: 'building' }, previewReported: new Set(), previewDiagnostics: [],
    previewGrant: { token: 'token', validation_id: 'build', expires_at: new Date(Date.now() + 60000).toISOString() },
    addDiagnostic: item => shown.push(item),
    api: { reportPreview: (...args) => { sent.push(args); return new Promise((_, fail) => { reject = fail; }); } }
  };
  const event = { source: state.frame.contentWindow, data: { source: 'aurago-game', channel: 'channel', type: 'runtime_error', message: 'this.paddle.body.setVelocity is not a function' } };
  window.GameMakerStudioPreview.handleMessage(state, event);
  assert.equal(sent.length, 1);
  assert.equal(shown.length, 1);
  state.previewGrant.expires_at = new Date(Date.now() - 1000).toISOString();
  reject(new Error('invalid or expired preview token'));
  await Promise.resolve();
  assert.equal(shown.length, 1, 'expiry during an in-flight report must not add a misleading game error');
  window.GameMakerStudioPreview.handleMessage(state, { ...event, data: { ...event.data, message: 'late error' } });
  window.GameMakerStudioPreview.handleMessage(state, { ...event, data: { ...event.data, type: 'gameplay', observations: [] } });
  assert.equal(sent.length, 1, 'expired validation must not send more reports');
  state.previewGrant.expires_at = new Date(Date.now() + 60000).toISOString();
  for (const status of ['failed', 'cancelled', 'ready']) {
    state.job.status = status;
    window.GameMakerStudioPreview.handleMessage(state, { ...event, data: { ...event.data, message: status } });
  }
  assert.equal(sent.length, 1, 'terminal jobs must not report against the old validation grant');
  assert.equal(shown.length, 1);
  state.previewGrant = { token: 'published', validation_id: '' };
  window.GameMakerStudioPreview.handleMessage(state, { ...event, data: { ...event.data, message: 'published gameplay error' } });
  assert.equal(shown.length, 2, 'published-game diagnostics must remain visible');
}

function testGameMakerBootDetectsInvisibleCanvasAndEngineErrors() {
  const source = sourceBetween(read('internal/gamemaker/preview.go'), '(function () {', '</script>');
  function boot() {
    const messages = [], listeners = new Map();
    let now = 0, interval = null;
    const rect = { top: 650, bottom: 1100, left: 0, right: 800, width: 800, height: 450 };
    const canvas = { width: 700, height: 504, getBoundingClientRect: () => rect };
    const events = {
      addEventListener: (type, fn) => listeners.set(type, fn),
      removeEventListener: type => listeners.delete(type)
    };
    const context = {
      window: { ...events }, console: { error() {} },
      parent: { postMessage: message => messages.push(message) },
      performance: { now: () => now },
      location: { hash: '#gm-channel=test' }, URLSearchParams,
      innerHeight: 600, innerWidth: 800,
      getComputedStyle: () => ({ display: 'block', visibility: 'visible', opacity: '1' }),
      MutationObserver: class { observe() {} disconnect() {} },
      setInterval(callback) { interval = callback; return 1; },
      clearInterval() { interval = null; },
      document: { ...events, hidden: false, readyState: 'complete', documentElement: {},
        querySelector: () => canvas,
        querySelectorAll: selector => selector === 'canvas' ? [canvas] : [] }
    };
    vm.runInNewContext(source, context);
    return { context, messages, rect, listeners,
      tick(time) { now = time; interval?.(); },
      watching: () => interval !== null };
  }
  const failed = boot();
  failed.tick(2999);
  assert.equal(failed.messages.length, 0, 'layout gets a settling period before failure');
  failed.tick(3000);
  assert.match(failed.messages[0].message, /canvas remains hidden or outside/);
  assert.equal(failed.watching(), false, 'a conclusive failure stops the layout timer');
  failed.context.console.error('Failed to process file:', 'image', '"snake"');
  assert.match(failed.messages[1].message, /Failed to process file: image "snake"/);
  Object.assign(failed.rect, { top: 0, bottom: 450 });
  failed.tick(4000);
  assert.equal(failed.messages.length, 2, 'a failed boot must not later report success');

  const resumed = boot();
  resumed.context.document.hidden = true;
  resumed.tick(4000);
  resumed.context.document.hidden = false;
  resumed.context.innerHeight = 0;
  resumed.tick(8000);
  resumed.context.innerHeight = 600;
  resumed.listeners.get('message')({ source: resumed.context.parent, data: { type: 'aurago:game:active', active: false } });
  resumed.tick(12000);
  assert.equal(resumed.messages.length, 0, 'hidden, unsized or inactive host previews are not game failures');
  resumed.listeners.get('message')({ source: resumed.context.parent, data: { type: 'aurago:game:active', active: true } });
  resumed.tick(14999);
  assert.equal(resumed.messages.length, 0, 'resuming starts a fresh layout settling period');
  Object.assign(resumed.rect, { top: 0, bottom: 450 });
  resumed.tick(15000);
  assert.equal(resumed.messages[0].type, 'ready');
  assert.equal(resumed.messages[0].visible, true);
  assert.equal(resumed.watching(), false);
}

function testDesktopMediaKeysAreInBootstrapScope() {
  let source = read('ui/js/desktop/bundles/main.bundle.js');
  source = source.slice(source.indexOf('/* ui/js/desktop/core/desktop-foundation.js */'));
  source = source.replace('(function () {', '(function () { throw new Error("scope:" + typeof initDesktopMediaKeysRuntime);');
  assert.throws(() => vm.runInNewContext(source), /scope:function/);
}

function testGameMakerPreviewLoadingIgnoresStaleFrameSettlement() {
  const source = read('ui/js/desktop/apps/game-maker-studio-preview.js');
  const timers = new Map();
  let nextTimerID = 0;
  const overlays = [];
  const document = {
    fullscreenElement: null,
    createElement() {
      const overlay = {
        className: '',
        innerHTML: '',
        attrs: {},
        removed: false,
        setAttribute(name, value) {
          this.attrs[name] = value;
        },
        remove() {
          this.removed = true;
        }
      };
      overlays.push(overlay);
      return overlay;
    }
  };
  const window = {};
  const context = {
    window,
    document,
    setTimeout(callback, delay) {
      const id = ++nextTimerID;
      timers.set(id, { callback, delay });
      return id;
    },
    clearTimeout(id) {
      timers.delete(id);
    }
  };
  vm.runInNewContext(source, context);
  const preview = window.GameMakerStudioPreview;
  const state = {
    context: {
      esc: value => value,
      t: key => key
    },
    previewLoadTimer: null,
    previewLoadClear: null,
    container: { querySelector: () => null },
    addDiagnostic() {}
  };
  const shell = {
    appendChild() {}
  };
  const frameHandlers = [];
  const newFrame = () => ({
    addEventListener(type, callback) {
      assert.equal(type, 'load');
      frameHandlers.push(callback);
    }
  });

  preview.showLoading(state, shell, newFrame());
  preview.showLoading(state, shell, newFrame());
  assert.equal(overlays[0].removed, true, 'a replacement preview must remove the previous loading overlay');
  const activeClear = state.previewLoadClear;
  const activeTimer = state.previewLoadTimer;

  frameHandlers[0]();
  const staleSettlement = [...timers.entries()].find(([, timer]) => timer.delay === 600);
  assert.ok(staleSettlement, 'the stale frame load should schedule its delayed settlement');
  timers.delete(staleSettlement[0]);
  staleSettlement[1].callback();

  assert.equal(overlays[1].removed, false, 'a stale frame must not clear the current loading overlay');
  assert.equal(state.previewLoadClear, activeClear, 'a stale frame must not clear the current cleanup callback');
  assert.equal(state.previewLoadTimer, activeTimer, 'a stale frame must not clear the current timeout');
  assert.equal(timers.has(activeTimer), true, 'the current timeout must remain armed');

  frameHandlers[1]();
  const currentSettlement = [...timers.entries()].find(([, timer]) => timer.delay === 600);
  assert.ok(currentSettlement, 'the current frame load should schedule its delayed settlement');
  currentSettlement[1].callback();
  assert.equal(overlays[1].removed, true);
  assert.equal(state.previewLoadClear, null);
  assert.equal(state.previewLoadTimer, null);
}

async function testServiceWorkerPreservesMediaRangeResponses() {
  const handlers = {};
  let cacheOpened = false;
  let networkRequests = 0;
  const context = {
    URL,
    Response,
    fetch: async request => {
      networkRequests++;
      if (request.headers.has('Range')) {
        assert.equal(request.headers.get('Range'), 'bytes=0-1023');
        return new Response('audio bytes', { status: 206 });
      }
      return new Response('script bytes', { status: 200 });
    },
    caches: {
      open: async () => {
        cacheOpened = true;
        throw new Error('range requests must bypass Cache Storage');
      },
      keys: async () => []
    },
    self: {
      location: { href: 'https://aurago.test/sw.js?v=test', origin: 'https://aurago.test' },
      addEventListener(type, handler) { handlers[type] = handler; },
      skipWaiting: async () => {},
      clients: { claim: async () => {} },
      registration: { showNotification: async () => {} }
    },
    console
  };
  vm.runInNewContext(read('ui/sw.js'), context);

  let responsePromise;
  const request = new Request('https://aurago.test/img/audio/galaga.mp3?v=test', {
    headers: { Range: 'bytes=0-1023' }
  });
  handlers.fetch({ request, respondWith(value) { responsePromise = Promise.resolve(value); } });
  assert.ok(responsePromise, 'range request must be handled directly');
  const response = await responsePromise;
  assert.equal(response.status, 206);
  assert.equal(networkRequests, 1);
  assert.equal(cacheOpened, false);

  responsePromise = null;
  handlers.fetch({ request: new Request('https://aurago.test/img/audio/galaga.mp3', { headers: { Range: 'bytes=0-1023' } }), respondWith(value) { responsePromise = value; } });
  assert.equal(responsePromise, null, 'unversioned media must remain browser network requests');
  assert.equal(networkRequests, 1);
  assert.equal(cacheOpened, false);

  let cacheWriteAttempted = false;
  context.caches.open = async () => {
    cacheOpened = true;
    return {
      match: async () => null,
      put: async () => {
        cacheWriteAttempted = true;
        throw new Error('simulated quota failure');
      }
    };
  };
  responsePromise = null;
  const scriptRequest = new Request('https://aurago.test/js/example.js?v=test');
  handlers.fetch({ request: scriptRequest, respondWith(value) { responsePromise = Promise.resolve(value); } });
  const scriptResponse = await responsePromise;
  assert.equal(scriptResponse.status, 200, 'cache write failures must not replace valid responses');
  assert.equal(cacheOpened, true);
  assert.equal(cacheWriteAttempted, true);
  assert.equal(networkRequests, 2);
}

function loadSkillSnapshotRuntime() {
  const skills = read('ui/js/skills/main.js');
  const snapshotSource = sourceBetween(skills, 'function sortedSnapshotArray', 'function shouldUpdateSkill');
  const context = {
    Array,
    JSON,
    String,
    daemonSystemEnabled: false,
    daemonStates: {}
  };
  context.getDaemonState = skillID => context.daemonStates[skillID] || null;
  vm.createContext(context);
  vm.runInContext(`${snapshotSource}; globalThis.skillSnapshot = skillStateHash;`, context);
  return context;
}

function testSkillSnapshotDifferences() {
  const context = loadSkillSnapshotRuntime();
  const skill = {
    ID: 'demo',
    Name: 'demo',
    Description: 'Example',
    IsDaemon: false,
    Tags: ['beta', 'alpha']
  };

  const baseline = context.skillSnapshot(skill);
  const reordered = context.skillSnapshot({ ...skill, Tags: ['alpha', 'beta'] });
  assert.equal(reordered, baseline, 'sorted render arrays must be deterministic');

  const daemonSkill = context.skillSnapshot({ ...skill, IsDaemon: true });
  assert.notEqual(daemonSkill, baseline, 'is_daemon must invalidate the rendered card');
  assert.equal(JSON.parse(daemonSkill).isDaemon, true);

  context.daemonSystemEnabled = true;
  const daemonSystemEnabled = context.skillSnapshot({ ...skill, IsDaemon: true });
  assert.notEqual(daemonSystemEnabled, daemonSkill, 'global daemon system state must invalidate daemon actions');
  assert.equal(JSON.parse(daemonSystemEnabled).daemonSystemEnabled, true);

  context.daemonStates.demo = { status: 'running', auto_disabled: false };
  const running = context.skillSnapshot({ ...skill, IsDaemon: true });
  context.daemonStates.demo = { status: 'disabled', auto_disabled: true };
  const autoDisabled = context.skillSnapshot({ ...skill, IsDaemon: true });
  assert.notEqual(autoDisabled, running, 'daemon status and auto-disabled state must invalidate badges and actions');
}

function loadSkillCardRuntime() {
  const skills = read('ui/js/skills/main.js');
  const snapshotSource = sourceBetween(skills, 'function sortedSnapshotArray', 'function shouldUpdateSkill');
  const renderCardSource = sourceBetween(skills, 'function renderCard', 'function agentSkillStateHash');
  const agentSnapshotSource = sourceBetween(skills, 'function agentSkillStateHash', 'function shouldUpdateAgentSkill');
  const renderAgentCardSource = sourceBetween(skills, 'function renderAgentSkillCard', 'function renderSecurityBadge');
  const context = {
    Array,
    JSON,
    String,
    daemonSystemEnabled: true,
    daemonStates: {},
    credentialMap: {},
    esc: value => String(value),
    t: key => key,
    renderSecurityBadge: () => '',
    renderDaemonBadge: () => '',
    renderDaemonActions: () => ''
  };
  context.getDaemonState = skillID => context.daemonStates[skillID] || null;
  vm.createContext(context);
  vm.runInContext(
    `${snapshotSource}\n${renderCardSource}\n${agentSnapshotSource}\n${renderAgentCardSource}\n` +
    'globalThis.renderPythonCard = renderCard; globalThis.renderAgentCard = renderAgentSkillCard;',
    context
  );
  return context;
}

function testSkillCardOrderingMatchesSnapshots() {
  const context = loadSkillCardRuntime();
  const pythonSkill = {
    ID: 'ordered',
    Name: 'Ordered',
    Description: 'Stable order',
    Dependencies: ['zeta', 'alpha'],
    Tags: ['zeta', 'alpha'],
    VaultKeys: ['zeta', 'alpha'],
    InternalTools: ['zeta', 'alpha']
  };
  const reorderedPythonSkill = {
    ...pythonSkill,
    Dependencies: ['alpha', 'zeta'],
    Tags: ['alpha', 'zeta'],
    VaultKeys: ['alpha', 'zeta'],
    InternalTools: ['alpha', 'zeta']
  };
  assert.equal(
    context.skillStateHash(pythonSkill),
    context.skillStateHash(reorderedPythonSkill),
    'equivalent Python arrays must keep the same snapshot'
  );
  assert.equal(
    context.renderPythonCard(pythonSkill),
    context.renderPythonCard(reorderedPythonSkill),
    'equivalent Python arrays must render in the same visible order'
  );
}

function testAgentSkillCardOrderingMatchesSnapshot() {
  const context = loadSkillCardRuntime();
  const agentSkill = {
    id: 'agent-order',
    name: 'Agent order',
    description: 'Stable script order',
    scripts: [{ path: 'zeta.js' }, { path: 'alpha.js' }]
  };
  const reorderedAgentSkill = {
    ...agentSkill,
    scripts: [{ path: 'alpha.js' }, { path: 'zeta.js' }]
  };
  assert.equal(
    context.agentSkillStateHash(agentSkill),
    context.agentSkillStateHash(reorderedAgentSkill),
    'equivalent Agent scripts must keep the same snapshot'
  );
  assert.equal(
    context.renderAgentCard(agentSkill),
    context.renderAgentCard(reorderedAgentSkill),
    'equivalent Agent scripts must render in the same visible order'
  );
}

function testBundleCheckRejectsNonCanonicalBytesWithoutWriting() {
  const relativeBundle = 'ui/js/chat/bundles/chat-vendor.bundle.js';
  const bundlePath = path.join(root, relativeBundle);
  const original = readFileSync(bundlePath);
  const nonCanonical = Buffer.from(original.toString('utf8').replace(/\n/g, '\r\n').replace(/\r\n/, ' \t\r\n'));
  assert.notDeepEqual(nonCanonical, original);

  try {
    writeFileSync(bundlePath, nonCanonical);
    const result = spawnSync(process.execPath, ['scripts/build-ui-bundles.js', '--check'], {
      cwd: root,
      encoding: 'utf8'
    });
    assert.notEqual(result.status, 0, `--check accepted non-canonical bytes:\n${result.stdout}${result.stderr}`);
    assert.deepEqual(readFileSync(bundlePath), nonCanonical, '--check must remain read-only on failure');
  } finally {
    writeFileSync(bundlePath, original);
  }
}

function testRedactedMarkerDoesNotConsumeFollowingContent() {
  const chatCore = read('ui/js/shared/chat-core.js');
  const helperSource = sourceBetween(
    chatCore,
    'function replaceRedactedMarkers(html, label = \'[removed]\')',
    'function isDebugOnlyHistoryMessage(msg)'
  );
  const context = {
    String,
    escapeAttr: value => String(value),
    escapeHtml: value => String(value)
  };
  vm.createContext(context);
  vm.runInContext(
    `${helperSource}; globalThis.replaceMarker = replaceRedactedMarkers;`,
    context
  );

  const rendered = context.replaceMarker(
    '<p>Source liegt unter [redacted] /tmp/llama.cpp, aber der Build fehlt.</p>',
    '[removed]'
  );
  assert.match(rendered, /<span class="redacted-badge">\[removed\]<\/span>/);
  assert.match(rendered, / \/tmp\/llama\.cpp, aber der Build fehlt\./);
  assert.doesNotMatch(rendered, /redacted-reason/);
}

// show_detection_event sends /api/go2rtc/detection/images/<frame id> as a chat image; the
// allow-list accepts exactly that same-origin shape.
function testChatMediaAllowsDetectionFrames() {
  const chatCore = read('ui/js/shared/chat-core.js');
  const helperSource = sourceBetween(chatCore, 'function isSafeMediaSource(value, tag) {', 'function decorateEmojiGlyphs(root) {');
  const context = { URL, window: { location: { origin: 'https://aurago.test' } } };
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.isSafe = isSafeMediaSource;`, context);
  for (const ok of ['/api/go2rtc/detection/images/7', '/api/go2rtc/detection/images/123456?thumb=1', 'https://aurago.test/api/go2rtc/detection/images/42']) {
    assert.equal(context.isSafe(ok, 'img'), true, ok);
  }
  for (const bad of ['/api/go2rtc/detection/images/0', '/api/go2rtc/detection/images/abc', '/api/go2rtc/detection/images/7/x',
    '/api/go2rtc/detection/events/7', 'https://evil.test/api/go2rtc/detection/images/7', '/api/go2rtc/detection/images/%2e%2e']) {
    assert.equal(context.isSafe(bad, 'img'), false, bad);
  }
  assert.equal(context.isSafe('/api/go2rtc/proxy/api/frame.jpeg?src=garden', 'img'), true, 'live camera frames stay allowed');
}

function testVirtualComputersVNCPreferencesSurviveReconnect() {
  const controller = read('ui/js/desktop/apps/virtual-computers-vnc.js');
  const helperSource = sourceBetween(controller, 'function applyVNCPreferences', 'function mount');
  const context = {};
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.applyPreferences = applyVNCPreferences;`, context);

  const rfb = {};
  context.applyPreferences(rfb, { viewOnly: true, scaleMode: 'one-to-one' });
  assert.equal(rfb.viewOnly, true, 'reconnect must preserve view-only input protection');
  assert.equal(rfb.scaleViewport, false, 'reconnect must preserve 1:1 scaling');
  assert.equal(rfb.resizeSession, false, '1:1 scaling must not resize the remote session');

  context.applyPreferences(rfb, { viewOnly: false, scaleMode: 'fit' });
  assert.equal(rfb.viewOnly, false);
  assert.equal(rfb.scaleViewport, true);
  assert.equal(rfb.resizeSession, true);
}

function testVirtualComputersVNCExpansionUsesAppContentOnly() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const helperSource = sourceBetween(app, 'function setVNCExpanded', 'function openVNC');
  const classes = new Set();
  const root = {
    classList: {
      toggle(name, active) {
        if (active) classes.add(name);
        else classes.delete(name);
      }
    }
  };
  const state = {
    host: { querySelector: selector => selector === '.vc-app' ? root : null },
    vncExpanded: false
  };
  const context = {};
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.setExpanded = setVNCExpanded;`, context);

  context.setExpanded(state, true);
  assert.equal(state.vncExpanded, true);
  assert.equal(classes.has('is-vnc-expanded'), true);
  context.setExpanded(state, false);
  assert.equal(state.vncExpanded, false);
  assert.equal(classes.has('is-vnc-expanded'), false);
  assert.doesNotMatch(helperSource, /vd-window|maximized|toggleMaximize/);
}

function testVirtualComputersTerminalBinaryLifecycle() {
  const controller = read('ui/js/desktop/apps/virtual-computers-terminal.js');
  const sent = [];
  const written = [];
  const copied = [];
  const sockets = [];
  const clearedTimers = [];
  let terminalDisposed = 0;
  let terminalInstance = null;
  let observerDisconnected = 0;
  let onCloseCalls = 0;

  class MockNode {
    constructor() {
      this.dataset = {};
      this.hidden = false;
      this.textContent = '';
      this.listeners = new Map();
      this.classList = { add() {}, toggle() {} };
    }
    addEventListener(type, callback) { this.listeners.set(type, callback); }
    click() { this.listeners.get('click')?.({ currentTarget: this }); }
    focus() {}
  }
  const nodes = new Map([
    ['[data-role="terminal-stage"]', new MockNode()],
    ['[data-role="terminal-status"]', new MockNode()],
    ['[data-terminal-action="reconnect"]', new MockNode()],
    ['[data-terminal-action="disconnect"]', new MockNode()]
  ]);
  const container = new MockNode();
  container.querySelector = selector => nodes.get(selector) || null;

  class MockFitAddon {
    fit() { this.fitCalls = (this.fitCalls || 0) + 1; }
  }
  class MockTerminal {
    constructor(options) { this.options = options; terminalInstance = this; }
    loadAddon(addon) { this.addon = addon; }
    open(node) { this.node = node; }
    onData(callback) { this.dataCallback = callback; return { dispose() { terminalDisposed += 1; } }; }
    attachCustomKeyEventHandler(callback) { this.keyCallback = callback; }
    write(data) { written.push(data); }
    focus() { this.focused = true; }
    hasSelection() { return true; }
    getSelection() { return 'selected terminal text'; }
    dispose() { terminalDisposed += 1; }
  }
  class MockWebSocket {
    static OPEN = 1;
    constructor(url) {
      this.url = url;
      this.readyState = 0;
      this.listeners = new Map();
      sockets.push(this);
    }
    addEventListener(type, callback) { this.listeners.set(type, callback); }
    emit(type, event = {}) { this.listeners.get(type)?.(event); }
    send(data) { sent.push(data); }
    close() { this.closed = true; this.readyState = 3; }
  }
  class MockResizeObserver {
    constructor(callback) { this.callback = callback; }
    observe(node) { this.node = node; }
    disconnect() { observerDisconnected += 1; }
  }

  let timerID = 0;
  const context = {
    ArrayBuffer,
    Uint8Array,
    TextDecoder,
    TextEncoder,
    ResizeObserver: MockResizeObserver,
    navigator: { clipboard: { writeText(value) { copied.push(value); return Promise.resolve(); } } },
    setTimeout(callback) { timerID += 1; callback(); return timerID; },
    clearTimeout(id) { clearedTimers.push(id); },
    window: {
      Terminal: MockTerminal,
      FitAddon: { FitAddon: MockFitAddon },
      WebSocket: MockWebSocket
    }
  };
  context.window.window = context.window;
  vm.createContext(context);
  vm.runInContext(controller, context);

  const session = context.window.VirtualComputersTerminal.mount(container, {
    url: 'wss://example.test/api/virtual-computers/machines/vm-1/tty',
    machineId: 'vm-1',
    t: key => key,
    notify() {},
    onClose() { onCloseCalls += 1; }
  });
  assert.deepEqual(Object.keys(session).sort(), ['disconnect', 'fit', 'reconnect']);
  assert.equal(sockets.length, 1);
  assert.equal(sockets[0].binaryType, 'arraybuffer');

  sockets[0].readyState = MockWebSocket.OPEN;
  sockets[0].emit('open');
  const terminal = terminalInstance;
  terminal.dataCallback('ä');
  assert.equal(Buffer.from(sent[0]).toString('utf8'), 'ä', 'terminal input must use UTF-8 binary frames');
  sockets[0].emit('message', { data: new Uint8Array([111, 107]).buffer });
  assert.equal(Buffer.from(written[0]).toString('utf8'), 'ok', 'binary TTY output must be written unchanged');

  assert.equal(terminal.keyCallback({ key: 'c', ctrlKey: true, shiftKey: true, metaKey: false }), false);
  assert.deepEqual(copied, ['selected terminal text']);
  session.fit();
  session.reconnect();
  assert.equal(sockets.length, 2);
  assert.equal(sockets[0].closed, true);
  session.disconnect();
  assert.equal(sockets[1].closed, true);
  assert.equal(observerDisconnected, 1);
  assert.ok(terminalDisposed >= 2, 'terminal data subscription and terminal must be disposed');
  assert.ok(clearedTimers.length >= 1, 'pending terminal timers must be cleared');
  assert.equal(onCloseCalls, 0, 'programmatic cleanup must not recursively close the app view');

  const disposedBeforeFailedMount = terminalDisposed;
  context.ResizeObserver = class FailingResizeObserver {
    constructor() { throw new Error('resize unavailable'); }
  };
  assert.throws(() => context.window.VirtualComputersTerminal.mount(container, {
    url: 'wss://example.test/api/virtual-computers/machines/vm-2/tty',
    machineId: 'vm-2'
  }), /resize unavailable/);
  assert.ok(terminalDisposed >= disposedBeforeFailedMount + 2, 'partial terminal mounts must dispose subscriptions and xterm');
}

function testVirtualComputersTerminalSessionReconciliation() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const permissionSource = sourceBetween(app, 'function canUseVNC', 'function isHealthy');
  const lifecycleSource = sourceBetween(app, 'function disconnectVNC', 'async function launch');
  let terminalDisconnects = 0;
  let vncDisconnects = 0;
  const context = { setVNCExpanded() {} };
  vm.createContext(context);
  vm.runInContext(
    `${permissionSource}\n${lifecycleSource}\n` +
    'globalThis.reconcileTerminalSession = reconcileTerminal; globalThis.disconnectSessions = disconnectRemoteSessions;',
    context
  );

  const state = {
    status: { enabled: true, readonly: false },
    context: { readonly: false },
    machines: [{ id: 'vm-headless', display: false }],
    detailMode: 'terminal',
    terminalMachineId: 'vm-headless',
    terminalSession: { disconnect() { terminalDisconnects += 1; } },
    vncMachineId: null,
    vncSession: null,
    selectedShot: null
  };
  context.reconcileTerminalSession(state);
  assert.equal(terminalDisconnects, 0, 'refresh must preserve an eligible active terminal');
  assert.equal(state.detailMode, 'terminal');

  state.machines = [{ id: 'vm-headless', display: true }];
  context.reconcileTerminalSession(state);
  assert.equal(terminalDisconnects, 1, 'display changes must close the headless terminal');
  assert.equal(state.detailMode, 'overview');

  const both = {
    vncSession: { disconnect() { vncDisconnects += 1; } },
    vncMachineId: 'vm-display',
    vncExpanded: false,
    terminalSession: { disconnect() { terminalDisconnects += 1; } },
    terminalMachineId: 'vm-headless'
  };
  context.disconnectSessions(both);
  assert.equal(vncDisconnects, 1);
  assert.equal(terminalDisconnects, 2);
  assert.equal(both.vncSession, null);
  assert.equal(both.terminalSession, null);
}

function testVirtualComputersTerminalActionGating() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const detailSource = sourceBetween(app, 'function detailPane', 'function workspacePane');
  const context = {
    Array,
    Number,
    encodeURIComponent,
    esc: value => String(value == null ? '' : value),
    tx: (_ctx, key) => key,
    icon: () => '',
    isMutable: () => true,
    capabilities: () => ({ agent_tasks: false }),
    canUseVNC: (_state, machine) => machine?.display === true,
    canUseTerminal: (_state, machine) => machine?.display === false,
    formatDuration: value => String(value || 0),
    formatDate: value => String(value || ''),
    expiryCountdownMarkup: () => '<span>00:30</span>'
  };
  vm.createContext(context);
  vm.runInContext(`${detailSource}; globalThis.renderDetail = detailPane;`, context);

  const state = {
    context: {},
    resourceLoading: { machines: false },
    selectedMachineId: 'vm-1',
    detailMode: 'overview',
    selectedShot: null,
    screenshotLoading: false,
    status: { enabled: true, readonly: false }
  };
  state.machines = [{ id: 'vm-1', name: 'Headless', display: false, web_ports: [] }];
  const headless = context.renderDetail(state);
  assert.match(headless, /data-action="terminal"/);
  assert.doesNotMatch(headless, /data-action="vnc"|data-action="screenshot"/);

  state.machines = [{ id: 'vm-1', name: 'Desktop', display: true, web_ports: [] }];
  const display = context.renderDetail(state);
  assert.match(display, /data-action="vnc"/);
  assert.match(display, /data-action="screenshot"/);
  assert.doesNotMatch(display, /data-action="terminal"/);
}

function testVirtualComputersExpiryCountdownFormatting() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const helperSource = sourceBetween(app, 'function formatExpiryCountdown', 'function expiryCountdownMarkup');
  const context = { Date, Number, Math, String };
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.formatCountdown = formatExpiryCountdown;`, context);

  const now = Date.parse('2026-07-16T18:00:00Z');
  const translations = JSON.parse(read('ui/lang/desktop/en.json'));
  const i18n = { t: (key, args) => translations[key].replace(/\{\{(\w+)\}\}/g, (_, name) => args[name]) };
  assert.equal(context.formatCountdown('2026-07-16T18:01:05Z', now), '01:05');
  assert.equal(context.formatCountdown('2026-07-16T19:01:01Z', now), '01:01:01');
  assert.equal(context.formatCountdown('2026-07-17T19:01:01Z', now, i18n), '1d 01:01:01');
  assert.equal(context.formatCountdown('2026-07-17T19:01:01Z', now), '1 01:01:01');
  assert.equal(context.formatCountdown('2026-07-16T17:59:59Z', now), '00:00');
  assert.equal(context.formatCountdown('', now), '—');
}

function testVirtualComputersScreenshotSettlementIgnoresStaleRequests() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const helperSource = sourceBetween(app, 'function isCurrentScreenshotRequest', 'async function screenshot');
  const context = {};
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.settleScreenshot = settleScreenshot;`, context);

  const state = {
    detailMode: 'screenshot',
    selectedMachineId: 'vm-new',
    screenshotRequestID: 2,
    screenshotLoading: true,
    selectedShot: null
  };
  assert.equal(context.settleScreenshot(state, 'vm-old', 1, { data_base64: 'old' }), false);
  assert.equal(state.screenshotLoading, true, 'stale response must not end the active loading state');
  assert.equal(state.selectedShot, null, 'stale response must not replace the active screenshot');

  const shot = { data_base64: 'new' };
  assert.equal(context.settleScreenshot(state, 'vm-new', 2, shot), true);
  assert.equal(state.screenshotLoading, false);
  assert.equal(state.selectedShot, shot);
}

function testVirtualComputersResourceFailuresStayIsolated() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const machineHelpers = sourceBetween(app, 'function normalizeMachineList', 'function isMachinePollingVisible');
  const helperSource = sourceBetween(app, 'function applyResourceResult', 'async function refresh');
  const context = {};
  vm.createContext(context);
  vm.runInContext(`${machineHelpers}; ${helperSource}; globalThis.applyResult = applyResourceResult;`, context);

  const state = {
    machines: [{ id: 'vm-existing' }],
    templates: [],
    templatesFallback: false,
    resourceErrors: { machines: '', templates: '' },
    resourceLoading: { machines: true, templates: true }
  };
  context.applyResult(state, 'machines', {
    status: 'fulfilled',
    value: { machines: [{ id: 'vm-current' }] }
  });
  context.applyResult(state, 'templates', {
    status: 'rejected',
    reason: new Error('template service unavailable')
  });

  assert.equal(state.machines[0].id, 'vm-current', 'a template failure must not discard loaded machines');
  assert.equal(state.resourceErrors.machines, '');
  assert.equal(state.resourceErrors.templates, 'template service unavailable');
  assert.equal(state.templatesFallback, true, 'template failures must enable the labeled fallback');
}

function testVirtualComputersSelectionSurvivesRefresh() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const helperSource = sourceBetween(app, 'function reconcileSelection', 'function showOverview');
  const context = {};
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.reconcile = reconcileSelection;`, context);

  const state = {
    machines: [{ id: 'vm-one' }, { id: 'vm-two' }],
    selectedMachineId: 'vm-two',
    selectedShot: { data_base64: 'shot' },
    screenshotLoading: false,
    detailMode: 'screenshot'
  };
  context.reconcile(state);
  assert.equal(state.selectedMachineId, 'vm-two');
  assert.equal(state.detailMode, 'screenshot', 'refresh must preserve the selected machine workspace');

  state.machines = [{ id: 'vm-one' }];
  context.reconcile(state);
  assert.equal(state.selectedMachineId, 'vm-one');
  assert.equal(state.detailMode, 'overview');
  assert.equal(state.selectedShot, null);
}

function testVirtualComputersHidesUnavailableCapabilitySections() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const helperSource = sourceBetween(app, 'function reconcileSection', 'function showOverview');
  let disconnected = 0;
  const context = {
    capabilities: state => state.status.capabilities,
    disconnectVNC() { disconnected += 1; }
  };
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.reconcileSection = reconcileSection;`, context);

  const state = { activeSection: 'volumes', status: { capabilities: { volumes: false, agent_control: true } } };
  context.reconcileSection(state);
  assert.equal(state.activeSection, 'machines');
  assert.equal(disconnected, 1, 'removing an active capability must close its live workspace');

  state.activeSection = 'workspaces';
  context.reconcileSection(state);
  assert.equal(state.activeSection, 'workspaces', 'available capability sections must stay selected');
}

function testVirtualComputersMutationLocksAreIdempotent() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const helperSource = sourceBetween(app, 'function isPending', 'function formatDate');
  const context = {};
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.isPending = isPending; globalThis.setPending = setPending;`, context);

  const state = { pendingActions: new Set() };
  context.setPending(state, 'destroy', true);
  context.setPending(state, 'destroy', true);
  assert.equal(context.isPending(state, 'destroy'), true);
  assert.equal(state.pendingActions.size, 1, 'double clicks must share one mutation lock');
  context.setPending(state, 'destroy', false);
  assert.equal(context.isPending(state, 'destroy'), false);
}

function testVirtualComputersCanOpenIndependentWindows() {
  const shell = read('ui/js/desktop/core/window-shell-runtime.js');
  const helperSource = sourceBetween(shell, 'function matchesExistingAppWindow', 'function isStandaloneWidgetPath');
  const virtualWindow = { id: 'vc-1', appId: 'virtual-computers', element: { isConnected: true }, context: {} };
  const serialWindow = { id: 'qc-1', appId: 'quick-connect', element: { isConnected: true }, context: {} };
  const regularWindow = { id: 'settings-1', appId: 'settings', element: { isConnected: true }, context: {} };
  const context = {
    state: { windows: new Map([[virtualWindow.id, virtualWindow], [serialWindow.id, serialWindow], [regularWindow.id, regularWindow]]), activeWindowId: '' },
    clearWindowMenus() {},
    disposeAppWindow() {},
    normalizeDesktopPath: value => String(value || ''),
    isWindowOnActiveSpace: () => true
  };
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.findExisting = findExistingAppWindow;`, context);

  assert.equal(context.findExisting('virtual-computers', {}), undefined, 'Virtual Computers must allow a new independent window');
  assert.equal(context.findExisting('quick-connect', {}), undefined, 'Quick Connect must allow independent connections in multiple windows');
  assert.equal(context.findExisting('settings', {}), regularWindow, 'other single-instance apps must keep their existing behavior');
}

function testVirtualComputersMobileLayoutUsesAvailableWindowHeight() {
  const css = read('ui/css/desktop-app-virtual-computers.css');
  const mobile = css.slice(css.indexOf('@media (max-width: 760px)'));
  assert.doesNotMatch(mobile, /min-height:\s*56vh/, 'mobile preview must not overflow the clipped desktop window');
  assert.match(mobile, /grid-template-rows:\s*minmax\(/, 'mobile rows must divide the available app height');
  assert.match(mobile, /\.vc-list\s*\{[^}]*max-height:\s*none;/s, 'mobile list must use its grid row instead of viewport height');
}

function loadVirtualComputersMachinePollingRuntime() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const helperSource = sourceBetween(app, 'function normalizeMachineList', 'function scheduleWorkspaceRefresh');
  const requests = [];
  const scheduled = [];
  const cleared = [];
  const draws = [];
  const reconciled = [];
  let requestImpl = () => Promise.resolve({ machines: [] });
  const context = {
    Array,
    JSON,
    request(path) {
      requests.push(path);
      return requestImpl(path);
    },
    setTimeout(callback, delay) {
      scheduled.push({ callback, delay });
      return scheduled.length;
    },
    clearTimeout(id) { cleared.push(id); },
    reconcileSelection() { reconciled.push('selection'); },
    reconcileVNC() { reconciled.push('vnc'); },
    reconcileTerminal() { reconciled.push('terminal'); },
    draw(state) { draws.push(state.machines.map(machine => machine.id)); }
  };
  vm.createContext(context);
  vm.runInContext(
    `const machinePollIntervalMs = 5000; ${helperSource}; globalThis.storeMachines = storeMachines; ` +
    'globalThis.pollMachines = pollMachines; globalThis.scheduleMachineRefresh = scheduleMachineRefresh;',
    context
  );
  return {
    context,
    requests,
    scheduled,
    cleared,
    draws,
    reconciled,
    setRequestImpl(impl) { requestImpl = impl; }
  };
}

function newVirtualComputersPollingState() {
  const appWindow = { hidden: false, style: { display: '' } };
  const ownerDocument = { visibilityState: 'visible' };
  const host = {
    ownerDocument,
    closest(selector) { return selector === '.vd-window' ? appWindow : null; },
    getClientRects() { return [{}]; }
  };
  return {
    state: {
      host,
      machines: [],
      machineSnapshot: JSON.stringify([]),
      machinePollTimer: null,
      machinePollInFlight: false,
      resourceLoading: { machines: false },
      resourceErrors: { machines: 'old error' },
      refreshGeneration: 1,
      disposed: false
    },
    appWindow,
    ownerDocument
  };
}

async function testVirtualComputersMachinePollingLifecycle() {
  const runtime = loadVirtualComputersMachinePollingRuntime();
  const { state, appWindow, ownerDocument } = newVirtualComputersPollingState();

  runtime.context.scheduleMachineRefresh(state);
  assert.equal(runtime.scheduled.length, 1);
  assert.equal(runtime.scheduled[0].delay, 5000);

  runtime.setRequestImpl(() => Promise.resolve({ machines: [] }));
  await runtime.scheduled.shift().callback();
  assert.deepEqual(runtime.requests, ['/api/virtual-computers/machines']);
  assert.equal(runtime.draws.length, 0, 'unchanged machines must not redraw');
  assert.equal(runtime.scheduled.length, 1, 'each completed poll must schedule its successor');

  ownerDocument.visibilityState = 'hidden';
  await runtime.scheduled.shift().callback();
  assert.equal(runtime.requests.length, 1, 'hidden tabs must not request machines');
  assert.equal(runtime.scheduled.length, 1);

  ownerDocument.visibilityState = 'visible';
  appWindow.style.display = 'none';
  await runtime.scheduled.shift().callback();
  assert.equal(runtime.requests.length, 1, 'minimized app windows must not request machines');
  assert.equal(runtime.scheduled.length, 1);

  appWindow.style.display = '';
  state.host.getClientRects = () => [];
  await runtime.scheduled.shift().callback();
  assert.equal(runtime.requests.length, 1, 'invisible app hosts must not request machines');
  assert.equal(runtime.scheduled.length, 1);

  state.host.getClientRects = () => [{}];
  runtime.setRequestImpl(() => Promise.resolve({ machines: [{ id: 'vm-1', display: false }] }));
  await runtime.scheduled.shift().callback();
  assert.equal(runtime.requests.length, 2);
  assert.deepEqual(runtime.draws, [['vm-1']]);
  assert.deepEqual(runtime.reconciled, ['selection', 'vnc', 'terminal']);
  assert.equal(state.resourceErrors.machines, '');
  assert.equal(runtime.scheduled.length, 1);

  runtime.setRequestImpl(() => Promise.reject(new Error('offline')));
  await runtime.scheduled.shift().callback();
  assert.equal(runtime.draws.length, 1, 'background failures must retain the rendered list');
  assert.equal(runtime.scheduled.length, 1, 'background failures must retry on the next cycle');

  runtime.context.storeMachines(state, [{ id: 'vm-1', display: false }]);
  runtime.setRequestImpl(() => Promise.resolve({ machines: [{ id: 'vm-1', display: false }] }));
  await runtime.context.pollMachines(state);
  assert.equal(runtime.draws.length, 1, 'a full-refresh baseline must prevent a redundant redraw');

  let releaseSlowRequest;
  runtime.setRequestImpl(() => new Promise(resolve => { releaseSlowRequest = resolve; }));
  const firstPoll = runtime.context.pollMachines(state);
  const secondPoll = runtime.context.pollMachines(state);
  assert.equal(runtime.requests.length, 5, 'an in-flight poll must suppress overlap');
  releaseSlowRequest({ machines: [{ id: 'vm-2', display: false }] });
  await Promise.all([firstPoll, secondPoll]);
  assert.deepEqual(state.machines.map(machine => machine.id), ['vm-2']);

  let releaseLateRequest;
  runtime.setRequestImpl(() => new Promise(resolve => { releaseLateRequest = resolve; }));
  const latePoll = runtime.context.pollMachines(state);
  state.disposed = true;
  releaseLateRequest({ machines: [{ id: 'vm-late', display: false }] });
  await latePoll;
  assert.deepEqual(state.machines.map(machine => machine.id), ['vm-2'], 'late responses after dispose must be ignored');
}

function testVirtualComputersMachinePollingDisposeClearsTimer() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const disposeSource = sourceBetween(app, 'function dispose(windowId)', 'window.VirtualComputersApp');
  const cleared = [];
  const state = {
    disposed: false,
    refreshGeneration: 2,
    workspaceRefreshTimer: 7,
    machinePollTimer: 8,
    expiryCountdownTimer: 9,
    clickHandler: null,
    changeHandler: null,
    keyHandler: null,
    host: {}
  };
  const context = {
    instanceMap: new Map([['vc-1', state]]),
    clearTimeout(id) { cleared.push(id); },
    disconnectRemoteSessions() {}
  };
  vm.createContext(context);
  vm.runInContext(
    `const instances = globalThis.instanceMap; ${disposeSource}; globalThis.disposeApp = dispose;`,
    context
  );
  context.disposeApp('vc-1');
  assert.deepEqual(cleared, [7, 8, 9]);
  assert.equal(state.disposed, true);
  assert.equal(state.refreshGeneration, 3);
  assert.equal(context.instanceMap.has('vc-1'), false);
}

function testVirtualComputersWorkspacePolling() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const helperSource = sourceBetween(app, 'function scheduleWorkspaceRefresh', 'function dispose');
  const scheduled = [];
  const refreshed = [];
  const context = {
    clearTimeout() {},
    setTimeout(callback, delay) {
      scheduled.push({ callback, delay });
      return scheduled.length;
    },
    capabilities: state => state.status.capabilities,
    refreshResource(_state, resource) { refreshed.push(resource); }
  };
  vm.createContext(context);
  vm.runInContext(
    `${helperSource}; globalThis.scheduleWorkspaceRefresh = scheduleWorkspaceRefresh;`,
    context
  );

  const state = {
    activeSection: 'workspaces',
    status: { capabilities: { agent_control: true } },
    workspaces: [{ state: 'ready' }],
    disposed: false,
    workspaceRefreshTimer: null
  };
  context.scheduleWorkspaceRefresh(state);
  assert.equal(scheduled.length, 1);
  assert.equal(scheduled[0].delay, 3000);
  scheduled.shift().callback();
  assert.deepEqual(refreshed, ['workspaces']);

  state.workspaces = [{ state: 'closed' }];
  context.scheduleWorkspaceRefresh(state);
  assert.equal(scheduled.length, 0, 'inactive workspaces must stop polling');
}

function testLocalGraniteMultimodalObserverIsIdempotent() {
  const main = read('ui/js/config/main.js');
  const helperSource = sourceBetween(
    main,
    'function _embeddingsBindMultimodal()',
    'let embeddingsRuntimeRefreshTimer = 0;'
  );
  const classes = new Set(['on']);
  const formatField = { style: { display: '' } };
  const provider = {
    value: 'local-granite',
    addEventListener(_event, callback) {
      this.changeListener = callback;
    }
  };
  const runtimeCardClasses = new Set();
  const runtimeCard = {
    attributes: {},
    classList: {
      toggle(name, active) {
        if (active) runtimeCardClasses.add(name);
        else runtimeCardClasses.delete(name);
      }
    },
    setAttribute(name, value) {
      this.attributes[name] = value;
    }
  };
  let observerCallbacks = 0;
  const maxObserverCallbacks = 25;
  const toggle = {
    dataset: {},
    attributes: {},
    nextElementSibling: { textContent: '' },
    title: '',
    classList: {
      contains(name) {
        return classes.has(name);
      },
      remove(name) {
        classes.delete(name);
        notifyClassMutation();
      },
      toggle(name, force) {
        const active = force === undefined ? !classes.has(name) : Boolean(force);
        if (classes.has(name) === active) return active;
        if (active) classes.add(name);
        else classes.delete(name);
        notifyClassMutation();
        return active;
      }
    },
    setAttribute(name, value) {
      this.attributes[name] = value;
    }
  };

  function notifyClassMutation() {
    if (!toggle.classObserver || observerCallbacks >= maxObserverCallbacks) return;
    observerCallbacks += 1;
    toggle.classObserver();
  }

  class FakeMutationObserver {
    constructor(callback) {
      this.callback = callback;
    }
    observe(target) {
      target.classObserver = this.callback;
    }
  }

  const context = {
    MutationObserver: FakeMutationObserver,
    document: {
      getElementById(id) {
        return id === 'emb-local-runtime-card' ? runtimeCard : null;
      },
      querySelector(selector) {
        if (selector === '[data-path="embeddings.multimodal"]') return toggle;
        if (selector === '[data-path="embeddings.multimodal_format"]') {
          return { closest: () => formatField };
        }
        if (selector === '[data-path="embeddings.provider"]') return provider;
        return null;
      }
    },
    t: key => key
  };
  vm.createContext(context);
  vm.runInContext(
    `${helperSource}; globalThis.bindMultimodal = _embeddingsBindMultimodal;`,
    context
  );

  context.bindMultimodal();
  provider.changeListener();
  assert.equal(observerCallbacks, 0, 'selecting local Granite must not create an observer feedback loop');
  assert.equal(classes.has('on'), false);
  assert.equal(toggle.dataset.disabled, 'true');
  assert.equal(formatField.style.display, 'none');
  assert.equal(runtimeCardClasses.has('is-hidden'), false);

  provider.value = 'openai';
  provider.changeListener();
  toggle.classList.toggle('on');
  assert.equal(observerCallbacks, 2, 'class observation must remain active without recursive mutations');
  assert.equal(formatField.style.display, '');
  assert.equal(runtimeCardClasses.has('is-hidden'), true);
  assert.equal(runtimeCard.attributes['aria-hidden'], 'true');
}

function testRemoteEmbeddingStatusNeverRendersGraniteCPUState() {
  const main = read('ui/js/config/main.js');
  const helperSource = sourceBetween(
    main,
    'function renderEmbeddingsRuntimeStatus(status)',
    'window.addEventListener(\'cfg:section-leave\''
  );
  const visibility = [];
  const context = {
    configData: { embeddings: { provider: 'openrouter-embeddings' } },
    document: {
      getElementById() { return null; },
      querySelector() {
        return { value: 'openrouter-embeddings' };
      }
    },
    syncEmbeddingsRuntimeVisibility(local) {
      visibility.push(local);
    }
  };
  vm.createContext(context);
  vm.runInContext(
    `${helperSource}; globalThis.renderEmbeddingStatus = renderEmbeddingsRuntimeStatus;`,
    context
  );

  context.renderEmbeddingStatus({
    provider: 'openrouter-embeddings',
    model_id: 'qwen/qwen3-embedding-8b',
    gpu: false
  });
  assert.deepEqual(visibility, [false], 'remote providers must hide the local Granite runtime card');
}

function testNetworkCamerasDesktopContracts() {
  const app = read('ui/js/desktop/apps/network-cameras.js');
  const loader = read('ui/js/desktop/core/module-loader.js');
  const routing = read('ui/js/desktop/core/menus-and-routing.js');
  const foundation = read('ui/js/desktop/core/desktop-foundation.js');
  const windowRuntime = read('ui/js/desktop/core/window-shell-runtime.js');
  const dashboard = read('ui/js/dashboard/dashboard-widgets.js');

  assert.match(loader, /'network-cameras'[\s\S]*desktop-app-network-cameras\.css[\s\S]*network-cameras\.js/);
  assert.match(routing, /appId === 'network-cameras'[\s\S]*NetworkCamerasApp/);
  assert.match(foundation, /'network-cameras': 'NetworkCamerasApp'/);
  assert.match(windowRuntime, /'network-cameras': \{ width: 1120, height: 720 \}/);
  assert.match(windowRuntime, /'network-cameras': \{ width: 680, height: 480 \}/);
  assert.match(dashboard, /href="\/desktop\?app=network-cameras"/);

  const preferenceSource = sourceBetween(app, 'function savePreferences(state)', 'function createState');
  assert.match(preferenceSource, /\{ mode: state\.mode, selected: state\.selected \}/);
  assert.doesNotMatch(preferenceSource, /password|username|source|token/i);

  const gridSource = sourceBetween(app, 'function liveGridIDs(state, streams)', 'function cardMarkup');
  const gridContext = { Set };
  vm.createContext(gridContext);
  vm.runInContext(`${gridSource}; globalThis.liveGridIDsForTest = liveGridIDs;`, gridContext);
  const streams = ['a', 'b', 'c', 'd', 'e', 'f'].map(id => ({ id, enabled: true }));
  assert.deepEqual(Array.from(gridContext.liveGridIDsForTest({ mode: 'live', visible: true, selected: 'a' }, streams)), ['a', 'b', 'c', 'd']);
  assert.equal(gridContext.liveGridIDsForTest({ mode: 'snapshots', visible: true, selected: 'a' }, streams).size, 0);
  assert.equal(gridContext.liveGridIDsForTest({ mode: 'live', visible: false, selected: 'a' }, streams).size, 0);
  assert.deepEqual(Array.from(gridContext.liveGridIDsForTest({ mode: 'live', visible: true, selected: 'hidden' }, streams)), ['a', 'b', 'c', 'd'], 'a filtered-out selection must not consume a live-grid slot');

  const thumbnailSource = sourceBetween(app, 'function visibleThumbnailNodes(state)', 'async function loadThumbnail');
  const thumbnailContext = { Array };
  vm.createContext(thumbnailContext);
  vm.runInContext(`${thumbnailSource}; globalThis.visibleThumbnailNodesForTest = visibleThumbnailNodes;`, thumbnailContext);
  const thumbnailNodes = ['front', 'garage'].map(id => ({ dataset: { thumbnail: id } }));
  const thumbnailState = {
    focus: false,
    visibleIDs: new Set(),
    host: { querySelectorAll: () => thumbnailNodes }
  };
  assert.deepEqual(Array.from(thumbnailContext.visibleThumbnailNodesForTest(thumbnailState), node => node.dataset.thumbnail), [], 'an empty visibility set must not fetch every thumbnail');
  thumbnailState.visibleIDs.add('front');
  assert.deepEqual(Array.from(thumbnailContext.visibleThumbnailNodesForTest(thumbnailState), node => node.dataset.thumbnail), ['front']);
  thumbnailState.focus = true;
  assert.deepEqual(Array.from(thumbnailContext.visibleThumbnailNodesForTest(thumbnailState), node => node.dataset.thumbnail), [], 'focus mode must stop grid thumbnail requests');
  assert.match(app, /if \(!\('IntersectionObserver' in window\)\) \{\s*cards\.forEach\(card => state\.visibleIDs\.add\(card\.dataset\.streamCard\)\)/);

  const noticeSource = sourceBetween(app, 'function mutationNoticeKey(result, successKey)', 'function readPreferences');
  const noticeContext = {};
  vm.createContext(noticeContext);
  vm.runInContext(`${noticeSource}; globalThis.mutationNoticeKeyForTest = mutationNoticeKey;`, noticeContext);
  assert.equal(noticeContext.mutationNoticeKeyForTest({ status: 'degraded' }, 'camera_saved'), 'saved_degraded');
  assert.equal(noticeContext.mutationNoticeKeyForTest({ status: 'ok' }, 'camera_saved'), 'camera_saved');

  const createSource = sourceBetween(app, 'async function createStream(state)', 'async function saveManagedStream');
  const saveSource = sourceBetween(app, 'async function saveManagedStream(state)', 'async function deleteManagedStream');
  assert.doesNotMatch(createSource, /catch \(error\) \{\s*modal\.source\s*=\s*''/, 'failed create must retain a manual source and setup token for retry');
  assert.doesNotMatch(saveSource, /catch \(error\) \{\s*modal\.source\s*=\s*''/, 'failed update must retain a replacement source for retry');

  const streamSource = sourceBetween(app, 'function activeStreams(state)', 'function filteredStreams');
  const streamContext = { Array, Object };
  vm.createContext(streamContext);
  vm.runInContext(`${streamSource}; globalThis.allStreamsForTest = allStreams;`, streamContext);
  const viewerState = { data: { can_manage: false, streams: [{ id: 'front', enabled: true }], disabled_streams: [{ id: 'garage', enabled: false }] } };
  assert.deepEqual(Array.from(streamContext.allStreamsForTest(viewerState), item => item.id), ['front']);
  viewerState.data.can_manage = true;
  assert.deepEqual(Array.from(streamContext.allStreamsForTest(viewerState), item => item.id), ['front', 'garage']);

  assert.match(app, /Math\.min\(4, nodes\.length\)/);
  assert.match(app, /state\.controllers\.forEach\(controller => controller\.abort\(\)\)/);
  assert.match(app, /frame\.src = 'about:blank'/);
  assert.match(app, /originalEnabled[\s\S]*disable_confirm_message[\s\S]*replace_confirm_message[\s\S]*confirmDialog/);
  assert.match(app, /const iconAliases[\s\S]*radar: 'search'[\s\S]*link: 'globe'/, 'camera setup methods must resolve to installed theme icons');
  assert.match(app, /nc-discovery-progress[\s\S]*role="status" aria-live="polite"[\s\S]*nc-spinner/, 'ONVIF discovery must expose a visible busy state');
  assert.match(app, /data-delete=[\s\S]*async function deleteStream[\s\S]*method: 'DELETE'/, 'admins must have a direct camera deletion path');
  assert.match(app, /const live = enabled && liveIDs\.has\(stream\.id\)/, 'selected cards must render live inside live-grid mode');
  assert.match(app, /const liveGrid = state\.mode === 'live'[\s\S]*\(liveGrid \? '' : detailMarkup\(state\)\)/, 'live-grid mode must replace the detail pane instead of duplicating it');
  assert.match(app, /if \(state\.mode === 'live'\) state\.mode = 'snapshots'[\s\S]*state\.focus = !state\.focus/, 'focus mode must return to the dedicated detail layout');
  assert.match(app, /window\.NetworkCamerasApp = \{ render, dispose, open \}/);
  assert.doesNotMatch(app, /\b(?:alert|confirm|prompt)\s*\(/);

  const languages = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
  const english = JSON.parse(read('ui/lang/desktop/en.json'));
  const expectedKeys = Object.keys(english).filter(key => key === 'desktop.app_network_cameras' || key.startsWith('desktop.network_cameras.')).sort();
  for (const language of languages) {
    const locale = JSON.parse(read(`ui/lang/desktop/${language}.json`));
    const actualKeys = Object.keys(locale).filter(key => key === 'desktop.app_network_cameras' || key.startsWith('desktop.network_cameras.')).sort();
    assert.deepEqual(actualKeys, expectedKeys, `${language} must cover every Network Cameras string`);
  }
}

function loadNetworkCamerasDetectionViews() {
  const source = read('ui/js/desktop/apps/network-cameras-detection-views.js');
  const context = { window: {} };
  vm.createContext(context);
  vm.runInContext(source, context);
  return context.window.NetworkCamerasDetectionViews;
}

function testNetworkCamerasDetectionViews() {
  const views = read('ui/js/desktop/apps/network-cameras-detection-views.js');
  assert.match(views, /const API = '\/api\/desktop\/integrations\/go2rtc\/detection\/'/);
  assert.match(views, /const POLL_MS = 5000/);
  assert.match(views, /window\.NetworkCamerasDetectionViews = Object\.freeze\(\{/);
  assert.doesNotMatch(views, /innerHTML|addEventListener|localStorage|eval\(|new Function/, 'the views module only builds strings');
  assert.doesNotMatch(views, /\b(?:alert|confirm|prompt)\s*\(/);

  const ncd = loadNetworkCamerasDetectionViews();
  const escape = value => String(value == null ? '' : value).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;').replaceAll("'", '&#39;');
  const ctx = {
    t: (key, params) => key + (params && Object.keys(params).length ? JSON.stringify(params) : ''),
    esc: escape,
    iconMarkup: () => '<i></i>'
  };
  const ns = 'desktop.network_cameras.detection.';

  assert.equal(ncd.armSummary(ctx, { mode: 'on', armed: true }), ns + 'armed_now');
  assert.equal(ncd.armSummary(ctx, { mode: 'off', armed: false }), ns + 'disarmed_now');
  assert.equal(ncd.armSummary(ctx, { mode: 'schedule', armed: false, next_change: null }), ns + 'disarmed_schedule_empty');
  assert.match(ncd.armSummary(ctx, { mode: 'schedule', armed: true, next_change: '2026-10-12T04:00:00Z' }, new Date('2026-10-11T20:00:00Z')), /armed_schedule_until\{"time":".+"\}/);
  assert.match(ncd.armSummary(ctx, { mode: 'schedule', armed: false, next_change: '2026-10-12T20:00:00Z' }, new Date('2026-10-11T08:00:00Z')), /disarmed_schedule_from\{"time":".+"\}/);

  const square = [[0.1, 0.1], [0.9, 0.1], [0.9, 0.9], [0.1, 0.9]];
  const valid = ncd.cameraPayload(Object.assign(ncd.cameraDefaults({}), { zones: [{ name: ' Einfahrt ', points: square }] }));
  assert.equal(valid.zones[0].name, ' Einfahrt ', 'stored zone names go out untouched (the server normalizes them; the editor trims edited names)');
  assert.equal(ncd.validateCameraSettings(valid), null);
  const invalid = (patch, key) => assert.equal((ncd.validateCameraSettings(Object.assign({}, valid, patch)) || {}).key, key, key);
  invalid({ interval_seconds: 0 }, 'invalid_interval');
  invalid({ interval_seconds: 31 }, 'invalid_interval');
  invalid({ min_score: 0.2 }, 'invalid_min_score');
  invalid({ cooldown_seconds: 86401 }, 'invalid_cooldown');
  invalid({ zones: [{ name: 'A', points: square.slice(0, 2) }] }, 'invalid_zone_points');
  invalid({ zones: [{ name: 'A', points: [[0.1, 0.1], [1.2, 0.1], [0.5, 0.5]] }] }, 'invalid_zone_points');
  invalid({ zones: [{ name: 'Hof', points: square }, { name: 'hof', points: square }] }, 'invalid_zone_name');
  invalid({ zones: [{ name: 'x'.repeat(49), points: square }] }, 'invalid_zone_name');
  invalid({ zones: Array.from({ length: 9 }, (_, index) => ({ name: 'Z' + index, points: square })) }, 'zone_limit');
  invalid({ actions: Object.assign({}, valid.actions, { agent_instruction: '😀'.repeat(501) }) }, 'invalid_instruction');
  assert.equal(ncd.validateCameraSettings(Object.assign({}, valid, { actions: Object.assign({}, valid.actions, { agent_instruction: '😀'.repeat(500) }) })), null, 'the instruction limit counts runes, not UTF-16 units');

  assert.deepEqual(JSON.parse(JSON.stringify(ncd.cameraDefaults(null))), { enabled: false, interval_seconds: 2, min_score: 0.5, cooldown_seconds: 300, zones: [], actions: { message: false, channel: 'all', agent: false, agent_instruction: '', desktop: true } });
  const kept = ncd.cameraDefaults({ enabled: true, interval_seconds: 5, min_score: 0.7, cooldown_seconds: 0, zones: [{ name: 'Tor', points: [[-1, 2], [0.5, 0.5], [0.2, 0.3]] }], actions: { message: true, channel: 'telegram', agent: true, agent_instruction: 'x', desktop: false } });
  assert.equal(kept.cooldown_seconds, 0);
  assert.equal(kept.actions.desktop, false);
  assert.equal(kept.actions.channel, 'telegram');
  assert.deepEqual(JSON.parse(JSON.stringify(kept.zones[0].points[0])), [0, 1], 'zone points are clamped to the frame');

  const global = ncd.settingsPayload(ncd.globalDefaults({ enabled: true, model: 'accurate', backend: 'gpu', retention_days: 7, max_storage_mb: 500, schedule: [{ days: ['sun', 'mon'], from: '22:00', to: '06:00' }] }));
  assert.deepEqual(JSON.parse(JSON.stringify(global)), { enabled: true, model: 'accurate', backend: 'gpu', retention_days: 7, max_storage_mb: 500, schedule: [{ days: ['mon', 'sun'], from: '22:00', to: '06:00' }] });
  assert.equal(ncd.validateGlobalSettings(global), null);
  assert.equal(ncd.validateGlobalSettings(Object.assign({}, global, { retention_days: 0 })).key, 'invalid_retention');
  assert.equal(ncd.validateGlobalSettings(Object.assign({}, global, { max_storage_mb: 99 })).key, 'invalid_storage');
  assert.equal(ncd.validateGlobalSettings(Object.assign({}, global, { schedule: [{ days: [], from: '22:00', to: '06:00' }] })).key, 'invalid_window');
  assert.equal(ncd.validateGlobalSettings(Object.assign({}, global, { schedule: [{ days: ['mon'], from: '24:00', to: '06:00' }] })).key, 'invalid_window');
  assert.deepEqual(JSON.parse(JSON.stringify(ncd.globalDefaults(null))), { enabled: false, model: 'standard', backend: 'auto', retention_days: 14, max_storage_mb: 2048, schedule: [] });

  const events = [
    { id: 2, camera_id: 'front', camera_name: 'Front', started_at: '2026-10-11T10:00:00Z', ended_at: '2026-10-11T10:01:05Z', max_score: 0.91, max_persons: 2, zones: [], armed: true, cover_frame_id: 21, alerts: {} },
    { id: 1, camera_id: 'garage', camera_name: 'Garage', started_at: '2026-10-11T08:00:00Z', ended_at: null, max_score: 0.6, max_persons: 1, zones: ['Tor'], armed: false, cover_frame_id: null, alerts: {} }
  ];
  const armLog = [{ id: 9, at: '2026-10-11T09:00:00Z', mode: 'on', armed: true, actor: 'user' }, { id: 8, at: '2026-10-10T07:00:00Z', mode: 'off', armed: false, actor: 'agent' }];
  assert.deepEqual(Array.from(ncd.timelineEntries(events, armLog, { camera: '', day: '' }, false), entry => entry.kind + entry.item.id), ['event2', 'arm9', 'event1', 'arm8']);
  assert.deepEqual(Array.from(ncd.timelineEntries(events, armLog, { camera: '', day: '' }, true), entry => entry.kind + entry.item.id), ['event2', 'arm9', 'event1'], 'arm rows older than the loaded page wait for the next page');
  assert.deepEqual(Array.from(ncd.timelineEntries(events, armLog, { camera: 'front', day: '' }, false), entry => entry.kind + entry.item.id), ['event2', 'event1'], 'a camera filter hides the global arm log');
  assert.deepEqual(Array.from(ncd.upsertEvents([events[1]], [Object.assign({}, events[1], { max_persons: 3 }), events[0]]), event => event.id + ':' + event.max_persons), ['2:2', '1:3']);

  const view = { status: { enabled: true, detector: { state: 'ready' }, cameras: [{ id: 'front' }] }, loaded: true, events: [], filter: { camera: '', day: '' } };
  assert.equal(ncd.mainKind(Object.assign({}, view, { status: null })), 'loading');
  assert.equal(ncd.mainKind(Object.assign({}, view, { status: null, connectionLost: true })), 'error');
  assert.equal(ncd.mainKind(Object.assign({}, view, { status: { enabled: false } })), 'off');
  assert.equal(ncd.mainKind(Object.assign({}, view, { status: { enabled: true, detector: { state: 'downloading' }, cameras: [] } })), 'preparing');
  assert.equal(ncd.mainKind(Object.assign({}, view, { status: { enabled: true, detector: { state: 'ready' }, cameras: [] } })), 'no_camera');
  assert.equal(ncd.mainKind(view), 'workspace');
  assert.equal(ncd.mainKind(Object.assign({}, view, { status: { enabled: false }, events })), 'workspace', 'past events stay visible while detection is off');

  const hostile = '<img src=x onerror=alert(1)>';
  const row = ncd.eventRowMarkup(ctx, Object.assign({}, events[0], { camera_name: hostile, zones: ['"><script>'], alerts: { message: { status: 'failed', detail: '<b>telegram</b>' }, mission: { status: 'evil"' } } }), new Date(events[0].started_at), 2);
  assert.doesNotMatch(row, /<img src=x|<script>|<b>telegram/);
  assert.match(row, /&lt;img src=x onerror=alert\(1\)&gt;/);
  assert.match(row, /class="ncd-alert is-skipped"/, 'unknown alert states fall back to skipped');
  assert.match(row, /data-event="2" data-focus-key="event-2" aria-current="true"/);
  assert.match(row, /src="\/api\/desktop\/integrations\/go2rtc\/detection\/images\/21\?thumb=1"/);
  const timeline = ncd.timelineMarkup(ctx, { events: [Object.assign({}, events[0], { id: '3" onclick="x', cover_frame_id: 'x' })], armLog: [], filter: { camera: '"><svg>', day: '' }, streams: [{ id: '"><svg>', name: hostile }], status: null, hasMore: false });
  assert.doesNotMatch(timeline, /onclick="x|<svg>|<img src=x/, 'non-numeric ids are dropped and names escaped');
  assert.doesNotMatch(ncd.chipMarkup(ctx, { id: 'a', name: hostile, state: 'active', analyses_per_minute: 29.5, last_error: '"><script>' }), /<script>|<img src=x/);

  const svg = ncd.boxesSvg([{ x1: '0.1" onload="x', y1: 0.1, x2: 0.5, y2: 0.9, score: 0.9 }, { x1: 0.1, y1: 0.2, x2: 0.3, y2: 0.9, score: 0.912, counts: true }, { x1: 0.5, y1: 0.2, x2: 0.6, y2: 0.8, score: 0.4, counts: false }], 1280, 720, 'test');
  assert.doesNotMatch(svg, /onload|NaN|undefined/);
  assert.equal((svg.match(/<rect /g) || []).length, 2);
  assert.match(svg, /<rect class="ncd-box is-counted" x="128" y="144" width="256" height="504"/);
  assert.match(svg, /class="ncd-box is-ignored"/);
  assert.match(svg, />91 %<\/text>/);
  const editor = ncd.zoneEditorSvg({ zones: [{ name: 'A', points: square }], selected: 0, drawing: null, width: 1000, height: 500, boxes: [] });
  assert.equal((editor.match(/<circle class="ncd-vertex" data-zone="0" data-point="\d"/g) || []).length, 4);
  assert.doesNotMatch(ncd.zoneEditorSvg({ zones: [{ name: 'A', points: [['x', 1], [0.2, 0.2], [0.3, 0.3]] }], selected: '0" onload="x', drawing: [[0.1, 0.1]], width: 'x', height: 720, boxes: [] }), /onload|NaN/);

  const detail = ncd.detailMarkup(ctx, { windowKey: 'w1', selectedId: 2, detail: Object.assign({}, events[0], { frames: [{ id: 21, seq: 1, width: 1280, height: 720, boxes: [{ x1: 0.1, y1: 0.1, x2: 0.2, y2: 0.5, score: 0.9 }] }, { id: 22, seq: 2, width: 1280, height: 720, boxes: [] }] }), frameIndex: 1, canManage: true });
  assert.match(detail, /src="\/api\/desktop\/integrations\/go2rtc\/detection\/images\/22"/);
  assert.match(detail, /frame_position\{&quot;current&quot;:2,&quot;total&quot;:2\}/);
  assert.match(detail, /data-ncd-action="next-frame"[^>]* disabled/);
  assert.match(detail, /data-ncd-action="delete-event"/);
  assert.doesNotMatch(ncd.detailMarkup(ctx, { windowKey: 'w1', selectedId: 2, detail: Object.assign({}, events[0], { frames: [] }), frameIndex: 0, canManage: false }), /delete-event/, 'viewers cannot delete events');

  const settings = values => ncd.settingsMarkup(ctx, { windowKey: 'w1', dialog: { values: ncd.globalDefaults(values), busy: false, error: '' }, status: { detector: { fallback_reason: 'GPU not accessible: /dev/dri/renderD128: permission denied' } } });
  assert.match(settings({}), /sudo usermod -aG render aurago/);
  assert.doesNotMatch(ncd.settingsMarkup(ctx, { windowKey: 'w1', dialog: { values: ncd.globalDefaults({}), busy: false, error: '' }, status: { detector: { fallback_reason: '' } } }), /usermod/);
  assert.match(settings({ schedule: [{ days: ['mon'], from: '22:00', to: '06:00' }] }), /data-window-day="0:mon"[^>]*aria-pressed="true"/);
  assert.match(ncd.settingsMarkup(ctx, { windowKey: 'w1', dialog: { values: null, busy: false, error: '' }, status: null }), /data-ncd-action="save-settings"[^>]* disabled/, 'without loaded settings nothing can be saved');

  const strip = ncd.stripMarkup(ctx, { canManage: false, status: { enabled: true, arm: { mode: 'on', armed: true }, detector: { state: 'degraded', backend: 'cpu', model_name: 'D-FINE-N', fallback_reason: 'self-test: <x>' }, cameras: [] } });
  assert.doesNotMatch(strip, /data-arm=/, 'viewers see the arm state read-only');
  assert.match(strip, /self-test: &lt;x&gt;/);
  assert.match(ncd.stripMarkup(ctx, { canManage: true, status: { enabled: true, arm: { mode: 'schedule', armed: true }, detector: { state: 'error', error: 'boom' }, cameras: [] } }), /data-arm="schedule"[^>]*aria-pressed="true"[\s\S]*data-ncd-action="retry"/);

  const cameraForm = ncd.cameraSettingsMarkup({ id: 'front"door', detection: { enabled: true, interval_seconds: 3, min_score: 0.6, cooldown_seconds: 60, zones: [{ name: hostile, points: square }], actions: { message: false, channel: 'all', agent: false, agent_instruction: '</textarea><script>', desktop: true } } }, ctx, { windowKey: 'win"1' });
  assert.doesNotMatch(cameraForm, /<script>|<img src=x|front"door/);
  assert.match(cameraForm, /thumbnail\/front%22door\.jpg\?width=1280&amp;height=720&amp;cache=0/);
  assert.match(cameraForm, /data-cam-field="actions.channel" disabled/, 'the channel only applies to messages');
  assert.match(cameraForm, /data-cam-field="enabled" aria-describedby="ncd-restart-hint-win1-frontdoor"[\s\S]*<p class="ncd-hint ncd-restart-hint" id="ncd-restart-hint-win1-frontdoor">desktop\.network_cameras\.detection\.cam_restart_hint<\/p>/, 'the detection switch carries the go2rtc restart hint');
  assert.deepEqual(JSON.parse(JSON.stringify(ncd.launchTarget({ event_id: 7, camera_id: 'front-door' }))), { eventId: 7, camera: 'front-door' });
  assert.deepEqual(JSON.parse(JSON.stringify(ncd.launchTarget({ event_id: 7, camera_id: 'Front Door' }))), { eventId: 7, camera: '' });
  assert.equal(ncd.launchTarget({ event_id: '7"><x' }), null);
  assert.equal(ncd.launchTarget({ event_id: 0 }), null);
  assert.equal(ncd.launchTarget(null), null);
  assert.equal(ncd.nextZoneName(ctx, [{ name: ns + 'zone_default_name{"n":1}' }]), ns + 'zone_default_name{"n":2}');
  assert.deepEqual(Array.from(ncd.normalizedPoint(150, 50, { left: 100, top: 0, width: 200, height: 100 })), [0.25, 0.5]);
  assert.deepEqual(Array.from(ncd.normalizedPoint(-50, 500, { left: 0, top: 0, width: 200, height: 100 })), [0, 1]);
  assert.equal(ncd.isNearPoint([0.5, 0.5], [0.503, 0.497]), true);
  assert.equal(ncd.isNearPoint([0.5, 0.5], [0.52, 0.5]), false);
  // Adaptations (controller notes): C5 zone-name rules, strict int64 ids, edge snap,
  // nonzero fill, flat zones, server time zone, sanitized window keys.
  assert.doesNotMatch(views, /evenodd/, 'zones use the default nonzero fill rule, like the backend');
  const zoneNameKey = name => (ncd.validateCameraSettings(Object.assign({}, valid, { zones: [{ name, points: square }] })) || {}).key || null;
  const cp = code => String.fromCodePoint(code);
  assert.equal(zoneNameKey(cp(0x200B) + 'Hof'), 'invalid_zone_name', 'format characters are rejected');
  assert.equal(zoneNameKey('Hof' + cp(0x202E)), 'invalid_zone_name', 'bidi controls are rejected');
  assert.equal(zoneNameKey('Hof' + cp(7)), 'invalid_zone_name', 'control characters are rejected');
  assert.equal(zoneNameKey('Hof' + String.fromCharCode(0xD800)), 'invalid_zone_name', 'lone surrogates are not valid UTF-8');
  assert.equal(zoneNameKey(cp(0x200D) + ' ' + cp(0x200C)), 'invalid_zone_name', 'a name needs a visible character');
  assert.equal(zoneNameKey(cp(0x1F468) + cp(0x200D) + cp(0x1F469)), null, 'the joiners ZWJ/ZWNJ are allowed');
  assert.equal(zoneNameKey('é'.repeat(48)), null, 'the name limit counts runes');
  assert.equal((ncd.validateCameraSettings(Object.assign({}, valid, { zones: [{ name: 'Cafe' + cp(0x301), points: square }, { name: 'CAFÉ', points: square }] })) || {}).key, 'invalid_zone_name', 'names compare in NFC, case-folded');
  assert.deepEqual(JSON.parse(JSON.stringify(ncd.launchTarget({ event_id: '1800000000001' }))), { eventId: 1800000000001, camera: '' }, 'int64 ids stay exact');
  for (const bad of ['0x10', '1e3', ' 7', '7.0', 1.5, -3, 2 ** 53]) assert.equal(ncd.idOf(bad), null, 'id ' + bad);
  assert.deepEqual(Array.from(ncd.normalizedPoint(198, 1, { left: 0, top: 0, width: 200, height: 100 })), [1, 0], 'points within 0.015 of a frame edge snap to it');
  assert.deepEqual(Array.from(ncd.normalizedPoint(196, 98, { left: 0, top: 0, width: 200, height: 100 })), [0.98, 0.98], 'points farther from the edge stay');
  assert.equal(ncd.isFlatZone([[0, 0], [0.5, 0.5], [1, 1]]), true, 'points on one line make a flat zone');
  assert.equal(ncd.isFlatZone(square), false);
  const at = new Date('2026-10-11T20:00:00Z');
  const localOffset = -at.getTimezoneOffset();
  const offsetText = minutes => (minutes < 0 ? '-' : '+') + String(Math.floor(Math.abs(minutes) / 60)).padStart(2, '0') + ':' + String(Math.abs(minutes) % 60).padStart(2, '0');
  const scheduleArm = offset => ({ mode: 'schedule', armed: true, next_change: '2026-10-12T04:00:00Z', time_zone: 'XST', utc_offset: offsetText(offset) });
  const stripWith = arm => ncd.stripMarkup(ctx, { canManage: false, now: at, status: { enabled: true, arm, detector: { state: 'ready' }, cameras: [] } });
  assert.match(stripWith(scheduleArm(localOffset + 60)), /arm_server_zone\{&quot;zone&quot;:&quot;XST, UTC[+-]\d{2}:\d{2}&quot;\}/, 'the arm status names the server zone when the browser offset differs');
  assert.doesNotMatch(stripWith(scheduleArm(localOffset)), /arm_server_zone/, 'no zone label when browser and server agree');
  assert.doesNotMatch(stripWith(Object.assign(scheduleArm(localOffset + 60), { time_zone: '<b>' })), /<b>|&lt;b&gt;/, 'an unexpected zone name is dropped');
  assert.match(ncd.settingsMarkup(ctx, { windowKey: 'w1', dialog: { values: ncd.globalDefaults({}), busy: false, error: '' }, status: { arm: scheduleArm(localOffset) } }), /schedule_server_zone\{&quot;zone&quot;:&quot;XST, UTC[+-]\d{2}:\d{2}&quot;\}/, 'the schedule editor names the server zone');
  assert.doesNotMatch(ncd.detailMarkup(ctx, { windowKey: 'w1"><x', selectedId: null }), /"><x/, 'window keys are sanitized');
  assert.doesNotMatch(ncd.settingsMarkup(ctx, { windowKey: 'w1"><x', dialog: { values: ncd.globalDefaults({}), busy: false, error: '' }, status: null }), /"><x/);

  // Batch 4A review add-ons (fixed in batch 4B).
  const sameInstant = '2026-10-11T10:00:00Z';
  const tieEvents = [{ id: 5, camera_id: 'a', started_at: sameInstant }, { id: 6, camera_id: 'a', started_at: sameInstant }];
  const tieArm = [{ id: 7, at: sameInstant, mode: 'on', armed: true, actor: 'user' }, { id: 8, at: sameInstant, mode: 'off', armed: false, actor: 'user' }];
  const tieOrder = (list, arm) => Array.from(ncd.timelineEntries(list, arm, { camera: '', day: '' }, false), entry => entry.kind + entry.item.id).join(',');
  assert.equal(tieOrder(tieEvents, tieArm), 'event6,event5,arm8,arm7', 'rows of the same instant sort by id (add-on 2)');
  assert.equal(tieOrder(tieEvents.slice().reverse(), tieArm.slice().reverse()), 'event6,event5,arm8,arm7', 'the order does not depend on the input order');
  assert.equal(ncd.validateCameraSettings(Object.assign({}, valid, { zones: [{ name: 'Hof', points: square }, { name: 'Hof ', points: square }] })), null, 'stored names that differ only in whitespace do not block a save (add-on 3)');
  assert.doesNotMatch(ncd.zoneListMarkup(ctx, [{ name: 'Hof', points: square }], -1), /maxlength/, 'no UTF-16 maxlength on zone names (add-on 4)');
  assert.match(ncd.zoneListMarkup(ctx, [{ name: '😀'.repeat(49), points: square }], -1), /aria-invalid="true"/, 'a name over 48 runes is marked invalid');
  assert.match(ncd.zoneListMarkup(ctx, [{ name: '😀'.repeat(48), points: square }], -1), /aria-invalid="false"/, '48 emoji are 48 runes, not 96 units');
  const perWindow = ['w1', 'w2'].map(windowKey => ncd.cameraSettingsMarkup({ id: 'front', detection: {} }, ctx, { windowKey }));
  assert.match(perWindow[0], /id="ncd-zone-help-w1-front"[\s\S]*/, 'zone help ids carry the window key (add-on 5)');
  assert.match(perWindow[1], /id="ncd-restart-hint-w2-front"/);
  assert.doesNotMatch(perWindow[1], /-w1-/);
  assert.equal(ncd.serverZone({ time_zone: 'UTC', utc_offset: '+00:00' }).label, 'UTC', 'a UTC server reads UTC (add-on 10)');
  assert.equal(ncd.serverZone({ time_zone: 'CET', utc_offset: '+01:00' }).label, 'CET, UTC+01:00');
  assert.equal(ncd.isFlatZone([[0.5, 0.5], [0.51, 0.5], [0.5, 0.51]]), true, 'a 1 % x 1 % triangle is too small (add-on 11)');
  assert.match(JSON.parse(read('ui/lang/desktop/en.json'))['desktop.network_cameras.detection.zone_flat'], /too small or flat/, 'the warning names tiny zones too');
  for (const [field, key] of [['detection.interval_seconds', 'invalid_interval'], ['detection.min_score', 'invalid_min_score'], ['detection.cooldown_seconds', 'invalid_cooldown'], ['detection.zones', 'zone_limit'], ['detection.zones[2].name', 'invalid_zone_name'], ['detection.zones[0].points[3]', 'invalid_zone_points'], ['detection.actions.agent_instruction', 'invalid_instruction'], ['detection.actions.channel', ''], [undefined, '']]) {
    assert.equal(ncd.fieldErrorKey(field), key, `400 field ${field} maps to a localized text (add-on 8)`);
  }
}

function testNetworkCamerasDetectionZones() {
  const zones = read('ui/js/desktop/apps/network-cameras-detection-zones.js');
  assert.match(zones, /window\.NetworkCamerasDetectionZones = Object\.freeze\(\{ bindCameraSettings \}\)/);
  assert.match(zones, /\} = window\.NetworkCamerasDetectionViews;/, 'zones reuse the shared views helpers');
  assert.match(zones, /stage\.setPointerCapture\(event\.pointerId\)/, 'dragging uses pointer capture for mouse and touch');
  assert.match(zones, /if \(background && JPEG_DATA_URL\.test\(image\)\) background\.src = image;/, 'only JPEG data URLs from the test endpoint become the editor background');
  assert.match(zones, /event\.key === 'Enter' && ed\.drawing[\s\S]*event\.key === 'Escape' && ed\.drawing[\s\S]*event\.key === 'Backspace'/, 'Enter closes, Escape cancels, Backspace removes the last point');
  assert.doesNotMatch(zones, /\b(?:alert|confirm|prompt)\s*\(/);
  assert.doesNotMatch(zones, /eval\(|new Function|document\.write|outerHTML|insertAdjacentHTML|localStorage/);
  for (const line of zones.split('\n').filter(entry => entry.includes('.innerHTML ='))) {
    assert.match(line.trim(), /^(?:svg\.innerHTML = zoneEditorSvg\(|if \(list\) list\.innerHTML = zoneListMarkup\()/, `unexpected innerHTML assignment: ${line.trim()}`);
  }
  // Adaptations (controller notes): edge snap through the shared helper, flat-zone warning,
  // localized test errors.
  assert.match(zones, /const pointFromEvent = event => normalizedPoint\(event\.clientX, event\.clientY, svg\.getBoundingClientRect\(\)\);/, 'every editor point passes the 0.015 edge snap');
  assert.match(zones, /if \(isFlatZone\(zone\.points\)\) setStatus\(T\(ctx, 'zone_flat', \{ name: zone\.name \}\), true\);/, 'a zone on one line gets a warning');
  assert.match(zones, /if \(code === 'detection_disabled'\) return T\(ctx, 'test_disabled'\);/, 'test mode explains a disabled detector');
  // Batch 4A review add-ons (fixed in batch 4B).
  assert.match(zones, /if \(code === 'detection_unavailable' \|\| \(error && error\.status === 503\)\) return T\(ctx, 'test_unavailable'\);/, 'a 503 test is localized (add-on 8)');
  assert.match(zones, /if \(code === 'stream_not_found'\) return T\(ctx, 'test_stream_not_found'\);/, 'a 404 test is localized');
  assert.match(zones, /const key = fieldErrorKey\(body\.field\);\s*return key \? T\(ctx, key\) : T\(ctx, 'test_failed'\);/, '400 field errors are localized');
  assert.match(zones, /if \(!svg \|\| event\.isPrimary === false \|\| \(event\.pointerType === 'mouse'/, 'only the primary pointer edits (add-on 6)');
  assert.match(zones, /if \(ed\.drag\) finishDrag\(\);/, 'a new primary press ends a drag whose pointerup got lost (4B review 3h)');
  assert.match(zones, /listen\(stage, 'pointercancel', finishDrag\);/, 'pointercancel always ends the drag');
  assert.match(zones, /moved: false, pointerId: event\.pointerId \}/);
  assert.match(zones, /listen\(stage, 'pointermove', event => \{\s*if \(!ed\.drag \|\| event\.pointerId !== ed\.drag\.pointerId\) return;/, 'a drag moves only with its own pointer');
  assert.match(zones, /const endDrag = event => \{\s*if \(ed\.drag && event\.pointerId === ed\.drag\.pointerId\) finishDrag\(\);/, 'a drag ends only with its own pointer');
  assert.match(zones, /const clearTestBoxes = \(\) => \{\s*ed\.revision \+= 1;\s*if \(!ed\.boxes\.length\) return;\s*ed\.boxes = \[\];\s*redraw\(\);/, 'stale test boxes are cleared (add-on 7)');
  assert.ok((zones.match(/clearTestBoxes\(\);/g) || []).length >= 4, 'adding, deleting and moving zones and changing the score clear the test boxes');
  assert.match(zones, /ed\.boxes = revision === ed\.revision && Array\.isArray/, 'a test that ran on older zones draws no boxes');
  assert.match(zones, /zone\.name = event\.target\.value\.trim\(\);/, 'only edited names are trimmed (add-on 3)');
  assert.match(zones, /remove\.setAttribute\('aria-label', T\(ctx, 'zone_delete'\) \+ ': ' \+ zone\.name\)/, 'the delete button follows a rename (add-on 9)');
}

function testNetworkCamerasDetectionRuntime() {
  const runtime = read('ui/js/desktop/apps/network-cameras-detection.js');
  assert.match(runtime, /window\.NetworkCamerasDetection = \{ mount, unmount, cameraSettingsMarkup, bindCameraSettings, validateCameraSettings \}/);
  assert.match(runtime, /request\(inst, 'state\?since=0'\)/);
  assert.match(runtime, /'state\?since=' \+ encodeURIComponent\(String\(inst\.cursor\)\)/);
  assert.match(runtime, /inst\.ctx\.confirmDialog\(/, 'event deletion asks through the desktop dialog');
  assert.match(runtime, /role="status" aria-live="polite" data-ncd-live/, 'status changes are announced');
  assert.match(runtime, /function unmount\(windowId, options\)[\s\S]*stopActivity\(inst\)/, 'unmount stops polling and requests');
  assert.doesNotMatch(runtime, /\b(?:alert|confirm|prompt)\s*\(/);
  assert.doesNotMatch(runtime, /eval\(|new Function|document\.write|outerHTML|insertAdjacentHTML|localStorage/);
  assert.match(runtime, /const launch = launchTarget\(opts\.launch\);[\s\S]*inst\.selectedId = launch\.eventId;/, 'a launch context selects its event');
  assert.match(runtime, /if \(inst\.selectedId && !inst\.detail && !inst\.detailError\) loadDetail\(inst, inst\.selectedId, false\);/, 'the launched event is fetched even when it is not on the loaded page');
  assert.match(runtime, /error\.status === 404 \|\| \(error\.body && error\.body\.code === 'event_not_found'\)[\s\S]*inst\.events = inst\.events\.filter[\s\S]*notify\(inst, T\(inst\.ctx, 'event_gone'\)\)/, 'a 404 drops the deleted event and says so');
  assert.doesNotMatch(runtime, /ctx\.notify\(\s*(?!\{)/, 'ctx.notify takes {title, message, type}');
  for (const line of runtime.split('\n').filter(entry => entry.includes('.innerHTML ='))) {
    assert.match(line.trim(), /^(?:region\.innerHTML = html;|inst\.host\.innerHTML = '<div class="ncd-root"[^+]*';)/, `unexpected innerHTML assignment: ${line.trim()}`);
  }

  // Batch 4B review fixes.
  const ncd = loadNetworkCamerasDetectionViews();
  const ctx = { t: (key, params) => key + (params && Object.keys(params).length ? JSON.stringify(params) : ''), esc: value => String(value), iconMarkup: () => '<i></i>' };
  // 1: busy controls stay focusable; a reloading list keeps the workspace.
  const busyStrip = ncd.stripMarkup(ctx, { canManage: true, armBusy: true, busy: true, status: { enabled: true, arm: { mode: 'on', armed: true }, detector: { state: 'error', error: 'x' }, cameras: [] } });
  assert.match(busyStrip, /data-arm="on"[^>]*aria-disabled="true"/, 'busy arm buttons keep the focus');
  assert.match(busyStrip, /data-ncd-action="retry"[^>]*aria-disabled="true"/);
  assert.doesNotMatch(busyStrip, / disabled[ >]/, 'no disabled attribute on busy strip controls');
  assert.match(runtime, /target\.disabled \|\| target\.getAttribute\('aria-disabled'\) === 'true'\) return;/, 'clicks on busy controls are ignored');
  assert.match(runtime, /const restoring = !focusKey && !!lost && \(!active \|\| active === document\.body\);/, 'a vanished control gets the focus back when it reappears');
  const status = { enabled: true, detector: { state: 'ready' }, cameras: [{ id: 'a' }] };
  assert.equal(ncd.mainKind({ status, loaded: false, events: [], filter: { camera: 'a', day: '' } }), 'workspace', 'a filtered list reloads inside the workspace');
  assert.equal(ncd.mainKind({ status, loaded: false, events: [], filter: { camera: '', day: '' }, listShown: true }), 'workspace', 'a list shown before reloads inside the workspace');
  assert.equal(ncd.mainKind({ status, loaded: false, events: [], filter: { camera: '', day: '' } }), 'loading', 'the very first load shows the spinner');
  assert.match(ncd.timelineMarkup(ctx, { events: [], armLog: [], filter: { camera: 'a', day: '' }, streams: [], status, loaded: false, hasMore: false }), /class="ncd-empty-list" role="status"/, 'the loading state sits in the list');
  // 3a–3c: change feed.
  assert.match(runtime, /const fresh = relevant\.filter\(event => !known\.has\(idOf\(event && event\.id\)\) && !inst\.announced\.has\(idOf\(event && event\.id\)\)\);/, 'only matching events are announced, once');
  assert.match(runtime, /const snapshotFeed = inst\.cursor === 0;[\s\S]*applyChanges\(inst, changes, false, snapshotFeed\);[\s\S]*if \(snapshotFeed && inst\.cursor > 0\) loadEvents\(inst, true\);/, 'since=0 after cursor 0 is a snapshot');
  assert.match(runtime, /if \(cursor != null && \(inst\.cursor === null \|\| cursor > inst\.cursor\)\) inst\.cursor = cursor;/, 'the cursor never moves back');
  // 3d, 3g: fresh settings, focus back to the opener, Tab stays in the dialog.
  assert.match(runtime, /async function freshSettings\(inst\) \{\s*const state = await request\(inst, '\/api\/desktop\/integrations\/go2rtc\/app\/state'\);/, 'fresh settings come from app/state');
  assert.match(sourceBetween(runtime, 'async function refreshDialogSettings(inst, dialog)', 'function closeSettings'), /const fresh = await freshSettings\(inst\);[\s\S]*if \(fresh && !dialog\.dirty\) dialog\.values = globalDefaults\(fresh\);/, 'the dialog loads fresh settings');
  // Final review: Activate refetches before its PUT; a save outlives a closed dialog.
  assert.match(sourceBetween(runtime, 'async function activate(inst)', 'function openSettings'), /const settings = await freshSettings\(inst\);[\s\S]*globalDefaults\(settings\)[\s\S]*method: 'PUT'/, 'Activate never PUTs a stale copy');
  assert.match(sourceBetween(runtime, 'async function saveSettings(inst)', 'function openCameraSettings'), /if \(inst\.dialog === dialog\) \{\s*inst\.dialog = null;\s*renderDialog\(inst\);\s*\}\s*notify\([\s\S]*settingsSaved\(inst, result\);/, 'a save finishes its follow-up after the dialog was closed');
  assert.match(runtime, /busy: true, error: '', dirty: false \};/, 'Save waits for the fresh settings');
  assert.match(runtime, /inst\.root\.querySelector\('\[data-focus-key="' \+ cssEscape\(inst\.dialogOpener \|\| 'settings'\) \+ '"\]'\)/, 'closing returns the focus to the opener');
  assert.match(runtime, /if \(event\.key === 'Tab' && inst\.dialog && target\.closest\('\[data-ncd-dialog\]'\)\) \{\s*trapDialogFocus\(inst, event\);/, 'Tab stays inside the dialog');
  // 3e: deleting an event deleted elsewhere.
  assert.match(sourceBetween(runtime, 'async function deleteEvent(inst)', 'async function retryDetector'), /error\.status === 404[\s\S]*notify\(inst, T\(inst\.ctx, 'event_gone'\)\)/, 'a 404 on delete drops the event with event_gone');
  // 3i: at most 50 schedule windows.
  const windows = count => ncd.validateGlobalSettings(Object.assign(ncd.globalDefaults({}), { schedule: Array.from({ length: count }, () => ({ days: ['mon'], from: '22:00', to: '06:00' })) }));
  assert.equal(windows(50), null);
  assert.equal(windows(51).key, 'schedule_limit', 'the schedule is capped like on the server');
  assert.match(runtime, /if \(values\.schedule\.length >= MAX_WINDOWS\) \{\s*inst\.dialog\.error = T\(inst\.ctx, 'schedule_limit'\);/);
  // 3j: German wording.
  const de = JSON.parse(read('ui/lang/desktop/de.json'));
  assert.match(de['desktop.network_cameras.detection.cam_restart_hint'], /Ausschalten der Erkennung/);
  assert.doesNotMatch(de['desktop.network_cameras.detection.test_result'], /\{\{counted\}\} zählen/, 'the count reads right for 1 too');
  // The CPU runtime size depends on the platform (about 8 MB on Linux, 76 MB on Windows).
  const en = JSON.parse(read('ui/lang/desktop/en.json'));
  for (const key of ['off_download', 'runtime_sizes']) assert.match(en['desktop.network_cameras.detection.' + key], /10–80 MB for the CPU, depending on the platform/, key);
  assert.doesNotMatch(read('ui/js/desktop/apps/network-cameras-detection-views.js'), /RUNTIME_MB/, 'no single runtime size');
}

function testNetworkCamerasDetectionAppContracts() {
  const app = read('ui/js/desktop/apps/network-cameras.js');
  const loader = read('ui/js/desktop/core/module-loader.js');

  assert.match(loader, /'network-cameras': \{\s*styles: appStyles\('\/css\/desktop-app-network-cameras\.css', '\/css\/desktop-app-network-cameras-detection\.css'\),\s*scripts: \[\s*'\/js\/desktop\/apps\/network-cameras-detection-views\.js',\s*'\/js\/desktop\/apps\/network-cameras-detection-zones\.js',\s*'\/js\/desktop\/apps\/network-cameras-detection\.js',\s*'\/js\/desktop\/apps\/network-cameras\.js'\s*\]/, 'the detection modules and stylesheet must load before the Network Cameras app');

  const modeSource = sourceBetween(app, 'function coerceMode(value)', 'function readPreferences');
  const modeContext = {};
  vm.createContext(modeContext);
  vm.runInContext(`${modeSource}; globalThis.coerceModeForTest = coerceMode; globalThis.launchContextForTest = launchContext;`, modeContext);
  assert.equal(modeContext.coerceModeForTest('detections'), 'detections');
  assert.equal(modeContext.coerceModeForTest('live'), 'live');
  assert.equal(modeContext.coerceModeForTest('snapshots'), 'snapshots');
  assert.equal(modeContext.coerceModeForTest('admin'), 'snapshots');
  assert.equal(modeContext.coerceModeForTest(undefined), 'snapshots');
  const launch = value => JSON.parse(JSON.stringify(modeContext.launchContextForTest(value)));
  assert.deepEqual(launch({ event_id: 7, camera_id: 'front-door' }), { event_id: 7, camera_id: 'front-door' });
  assert.deepEqual(launch({ event_id: 7, camera_id: '../x' }), { event_id: 7 }, 'an invalid camera id is dropped, the event stays');
  assert.equal(modeContext.launchContextForTest({ event_id: '7' }), null, 'event ids from the notification center are numbers');
  assert.equal(modeContext.launchContextForTest({ event_id: 2 ** 60 }), null);
  assert.equal(modeContext.launchContextForTest({ event_id: -1 }), null);
  assert.equal(modeContext.launchContextForTest(undefined), null);
  assert.match(app, /mode: coerceMode\(value\.mode\)/, 'the stored preference must accept the detection mode');
  assert.match(app, /state\.mode = coerceMode\(button\.dataset\.mode\)/, 'the segmented control must accept the detection mode');
  assert.match(app, /mode: launchContext\(ctx\) \? \(launchContext\(ctx\).event_id \? 'detections' : 'snapshots'\) : preferences\.mode/, 'event launches open detection; camera launches open snapshots');
  assert.match(app, /data-mode="detections"/);
  assert.match(app, /if \(state\.mode === 'detections' && runtime && runtime\.enabled && allStreams\(state\)\.length > 0\) return '<div class="nc-detection-host" data-role="detection"><\/div>';\s*if \(!runtime \|\| !runtime\.enabled \|\| !runtime\.api_usable/, 'the detection mode does not need a running go2rtc (4B review 2)');
  assert.match(sourceBetween(app, 'async function saveManagedStream(state)', 'async function deleteManagedStream'), /views\.fieldErrorKey\(field\)[\s\S]*modal\.error = key \? text\(state\.ctx, 'detection\.' \+ key\) : error\.message;\s*if \(field\.startsWith\('detection\.'\) && detectionTabAvailable\(state\)\) modal\.tab = 'detection';/, 'a PATCH 400 on the detection block is localized and opens its tab (4B review 3f)');
  assert.match(app, /module\.unmount\(state\.windowId, \{ keepState: true \}\)/, 'hidden windows and other modes must stop detection polling');
  assert.match(sourceBetween(app, 'function syncDetection(state)', 'function bindMainEvents'), /const launch = state\.pendingLaunch;\s*state\.pendingLaunch = null;/, 'the launch context is consumed by exactly one mount');
  assert.match(sourceBetween(app, 'function stopActivity(state)', 'function installVisibilityLifecycle'), /detection\.unmount\(state\.windowId, \{ keepState: true \}\)/);
  assert.match(sourceBetween(app, 'function dispose(windowId)', 'function render(host'), /detection\.unmount\(windowId\)/);
  assert.match(sourceBetween(app, 'function open(windowId, context)', 'function render(host'), /state\.pendingLaunch = launch;[\s\S]*state\.mode = launch.event_id \? 'detections' : 'snapshots';[\s\S]*draw\(state\)/, 'an open window takes a new event or camera launch context');
  assert.match(app, /window\.NetworkCamerasApp = \{ render, dispose, open \}/);
  const hook = "window.NetworkCamerasApp?.open?.(existing.id, context);";
  assert.ok(read('ui/js/desktop/core/window-shell-runtime.js').includes(hook), 'openApp must hand a launch context to an open Network Cameras window');
  assert.ok(read('ui/js/desktop/bundles/main.bundle.js').includes(hook), 'rebuild the main bundle (npm run build:ui)');
  assert.match(app, /modal\.detectionDirty && modal\.detection \? \{ detection: modal\.detection \} : \{\}/, 'camera detection settings are saved through PATCH streams/<id>');
  assert.match(sourceBetween(app, 'async function saveManagedStream(state)', 'async function deleteManagedStream'), /detection\.validateCameraSettings\(modal\.detection\)[\s\S]*confirmDialog/, 'invalid zones are reported before the save confirmation');
  for (const file of ['ui/js/desktop/apps/network-cameras.js', 'ui/js/desktop/apps/network-cameras-detection-views.js', 'ui/js/desktop/apps/network-cameras-detection-zones.js', 'ui/js/desktop/apps/network-cameras-detection.js']) {
    assert.doesNotMatch(read(file), /ctx\.notify\(\s*(?!\{)/, `${file}: ctx.notify is showDesktopNotification and needs {title, message, type}, not a string`);
  }
  // 4A review add-on 1: Save never drops a zone that is still being drawn.
  const zonesSource = read('ui/js/desktop/apps/network-cameras-detection-zones.js');
  assert.match(zonesSource, /const commitDrawing = \(\) => \{\s*if \(ed\.disposed \|\| !ed\.drawing\) return true;\s*if \(ed\.drawing\.length >= MIN_POINTS\) \{\s*closeZone\(\);\s*return true;\s*\}\s*setStatus\(T\(ctx, 'zone_unfinished'\), true\);\s*return false;/, 'an unfinished zone is closed, or the save is blocked with a hint');
  assert.match(zonesSource, /value: \(\) => cameraPayload\(ed\.draft\),\s*commitDrawing,/, 'the camera tab binding exposes commitDrawing');
  assert.match(sourceBetween(app, 'async function saveManagedStream(state)', 'async function deleteManagedStream'), /state\.detectionSettings\.commitDrawing\(\)\) \{\s*selectManageTab\(state, 'detection'\);[\s\S]*return;\s*\}[\s\S]*validateCameraSettings\(modal\.detection\)[\s\S]*method: 'PATCH'/, 'the drawing is committed before validation and the PATCH');
}

async function testManusCatalogFailuresStayIsolatedAndActionsRequireReadyStatus() {
  const source = read('ui/cfg/manus.js');
  const catalogSource = sourceBetween(source, 'async function manusLoadCatalogs(showErrors)', 'async function manusLoadProjectSkills(projectIDs, showErrors)');
  const loadButton = { disabled: false };
  let renderCount = 0;
  let projectLoadCount = 0;
  const catalogState = {
    projects: [],
    connectors: [],
    skills: [],
    globalSkills: [],
    projectSkills: {},
    projectSkillErrors: {},
    errors: {},
    loading: false,
    actionsEnabled: true
  };
  const catalogResponses = {
    '/api/manus/projects': { ok: true, status: 200, payload: { items: [{ id: 'project-1' }] } },
    '/api/manus/connectors': { ok: true, status: 200, payload: { items: [{ id: 'connector-1' }] } },
    '/api/manus/skills': { ok: false, status: 502, payload: { message: 'timestamp mismatch' } }
  };
  const catalogContext = {
    Promise,
    Array,
    Object,
    Error,
    manusCatalogState: catalogState,
    document: { getElementById: id => id === 'manus-load-catalogs-btn' ? loadButton : null },
    fetch: async url => {
      const response = catalogResponses[url];
      return { ok: response.ok, status: response.status, json: async () => response.payload };
    },
    t: key => key,
    manusRefreshSkillCatalog() {},
    manusConfig: () => ({ allowed_project_ids: [] }),
    async manusLoadProjectSkills() { projectLoadCount += 1; },
    manusRenderCatalogs() { renderCount += 1; },
    showToast() {}
  };
  vm.createContext(catalogContext);
  vm.runInContext(`${catalogSource}; globalThis.loadCatalogsForTest = manusLoadCatalogs;`, catalogContext);
  await catalogContext.loadCatalogsForTest(false);

  assert.deepEqual(Array.from(catalogState.projects, item => item.id), ['project-1']);
  assert.deepEqual(Array.from(catalogState.connectors, item => item.id), ['connector-1']);
  assert.equal(catalogState.errors.skills, 'timestamp mismatch (HTTP 502)');
  assert.equal(projectLoadCount, 1);
  assert.equal(renderCount, 1);
  assert.equal(loadButton.disabled, false);

  const statusSource = sourceBetween(source, 'async function manusRefreshStatus()', 'async function manusSaveAPIKey()');
  const banner = { className: '', textContent: '' };
  let ready = false;
  let actionsEnabled = null;
  const statusContext = {
    document: { getElementById: id => id === 'manus-status-banner' ? banner : null },
    fetch: async () => ({
      ok: true,
      json: async () => ready
        ? { status: 'ready', enabled: true, configured: true }
        : { status: 'missing_key', enabled: true, configured: false }
    }),
    t: key => key,
    manusConfig: () => ({}),
    manusSetActionAvailability(value) { actionsEnabled = value; }
  };
  vm.createContext(statusContext);
  vm.runInContext(`${statusSource}; globalThis.refreshStatusForTest = manusRefreshStatus;`, statusContext);
  await statusContext.refreshStatusForTest();
  assert.equal(actionsEnabled, false, 'enabled without a configured key must keep remote actions locked');
  ready = true;
  await statusContext.refreshStatusForTest();
  assert.equal(actionsEnabled, true, 'ready status must unlock remote actions');
}

function createSpeechLabRecorderHarness(options = {}) {
  const source = read('ui/js/chat/modules/speech-lab-recorder.js');
  const status = options.status || { enabled: true, chat_input_enabled: true, asr_ok: true };
  const counters = {
    contexts: 0,
    resumes: 0,
    closes: 0,
    getUserMedia: 0,
    trackStops: 0,
    statusRequests: 0,
    uploads: 0
  };
  const uploadedForms = [];
  let lastNode = null;

  class FakeAudioContext {
    constructor() {
      counters.contexts += 1;
      if (options.failContext) throw new Error('AudioContext unavailable');
      this.state = 'suspended';
      this.sampleRate = 48000;
      this.destination = {};
      this.audioWorklet = {
        addModule: async () => {
          if (options.failAddModule) throw new Error('worklet load failed');
          if (options.suspendAfterModule !== false) this.state = 'suspended';
        }
      };
    }

    async resume() {
      counters.resumes += 1;
      this.state = 'running';
    }

    async close() {
      counters.closes += 1;
      this.state = 'closed';
    }

    createMediaStreamSource() {
      return { connect() {}, disconnect() {} };
    }

    createGain() {
      return { gain: { value: 1 }, connect() {}, disconnect() {} };
    }
  }

  class FakeAudioWorkletNode {
    constructor() {
      this.port = { onmessage: null };
      lastNode = this;
    }

    connect() {}
    disconnect() {}
  }

  class FakeFormData {
    constructor() {
      this.entries = [];
    }

    append(...args) {
      this.entries.push(args);
    }
  }

  const stream = {
    getTracks: () => [{ stop: () => { counters.trackStops += 1; } }]
  };
  const navigator = {
    mediaDevices: {
      async getUserMedia() {
        counters.getUserMedia += 1;
        if (options.failGetUserMedia) {
          const error = new Error('microphone unavailable');
          error.name = options.mediaErrorName || 'NotReadableError';
          throw error;
        }
        return stream;
      }
    }
  };
  const fetch = async (url, request = {}) => {
    if (url === '/api/speech-lab/status') {
      counters.statusRequests += 1;
      if (options.statusError) throw new Error('status unavailable');
      return { ok: options.statusOK !== false, json: async () => status };
    }
    if (url === '/api/upload-voice') {
      counters.uploads += 1;
      uploadedForms.push(request.body);
      return {
        ok: true,
        headers: { get: () => 'application/json' },
        json: async () => ({ transcription: 'test transcript', speech_lab_turn_token: 'turn-token' })
      };
    }
    throw new Error(`unexpected fetch ${url}`);
  };
  const window = {
    AudioContext: FakeAudioContext,
    AudioWorkletNode: options.workletSupported === false ? null : FakeAudioWorkletNode,
    activeSessionId: () => 'session-1'
  };
  const context = {
    window,
    navigator,
    document: { body: { appendChild() {}, style: {} } },
    fetch,
    AudioContext: FakeAudioContext,
    AudioWorkletNode: FakeAudioWorkletNode,
    FormData: FakeFormData,
    Blob,
    ArrayBuffer,
    DataView,
    Float32Array,
    Math,
    Date,
    Error,
    setInterval: () => 1,
    clearInterval() {},
    setTimeout: () => 2,
    clearTimeout() {}
  };
  vm.runInNewContext(source, context);
  const recorder = window.SpeechLabRecorder;
  const errors = [];
  const transcriptions = [];
  recorder.onError = message => errors.push(message);
  recorder.onTranscription = (text, token) => transcriptions.push({ text, token });
  recorder._showUI = () => {};

  return {
    recorder,
    counters,
    errors,
    transcriptions,
    uploadedForms,
    lastNode: () => lastNode
  };
}

async function testSpeechLabRecorderLifecycleAndFallbacks() {
  const active = createSpeechLabRecorderHarness();
  const firstStart = active.recorder.start();
  assert.equal(await active.recorder.start(), 'busy', 'a concurrent start must be ignored');
  assert.equal(await firstStart, 'recording');
  assert.equal(active.recorder.state, 'recording');
  assert.equal(active.counters.contexts, 1);
  assert.equal(active.counters.getUserMedia, 1);
  assert.equal(active.counters.resumes, 2, 'the context must resume before and after worklet setup when suspended');

  active.lastNode().port.onmessage({ data: new Float32Array([0.25, -0.25, 0.5, -0.5]) });
  assert.equal(await active.recorder.send(), true);
  assert.equal(active.recorder.state, 'idle');
  assert.equal(active.counters.uploads, 1);
  assert.equal(active.counters.trackStops, 1);
  assert.equal(active.counters.closes, 1);
  assert.deepEqual(active.transcriptions, [{ text: 'test transcript', token: 'turn-token' }]);
  const upload = active.uploadedForms[0].entries[0];
  assert.equal(upload[0], 'audio');
  assert.equal(upload[1].type, 'audio/wav');
  assert.ok(upload[1].size > 44, 'the uploaded WAV must contain PCM samples');
  assert.equal(upload[2], 'speech-lab.wav');

  const empty = createSpeechLabRecorderHarness({ suspendAfterModule: false });
  assert.equal(await empty.recorder.start(), 'recording');
  assert.equal(await empty.recorder.send(), false);
  assert.equal(empty.counters.uploads, 0, 'zero samples must never be uploaded');
  assert.match(empty.errors.at(-1), /No speech audio was recorded/);

  const failed = createSpeechLabRecorderHarness({ failAddModule: true });
  assert.equal(await failed.recorder.start(), 'failed');
  assert.equal(failed.recorder.state, 'idle');
  assert.equal(failed.counters.trackStops, 1, 'a partial microphone stream must be stopped');
  assert.equal(failed.counters.closes, 1, 'a partial AudioContext must be closed');

  const unsupported = createSpeechLabRecorderHarness({ workletSupported: false });
  assert.equal(await unsupported.recorder.start(), 'browser');
  assert.equal(unsupported.counters.contexts, 0);
  assert.equal(unsupported.counters.getUserMedia, 0);

  const noContext = createSpeechLabRecorderHarness({ failContext: true });
  assert.equal(await noContext.recorder.start(), 'browser');
  assert.equal(noContext.counters.getUserMedia, 0);

  const disabled = createSpeechLabRecorderHarness({ status: { enabled: false, chat_input_enabled: false, asr_ok: false } });
  assert.equal(await disabled.recorder.start(), 'browser');

  const unavailable = createSpeechLabRecorderHarness({ statusError: true });
  unavailable.recorder.status = { enabled: true, chat_input_enabled: true, asr_ok: true };
  assert.equal(await unavailable.recorder.start(), 'failed', 'a known Speech Lab selection must fail closed');
  assert.equal(unavailable.errors.length, 1);
}

function testLocalLLMFamilySelection() {
  const saved = {};
  let renders = 0;
  const config = {
    localLLMSetDraftValue(key, value) { saved[key] = value; },
    renderLocalLLMSection() { renders++; }
  };
  vm.runInNewContext(sourceBetween(read('ui/cfg/local_llm.js'),
    'function localLLMChangeFamily(', 'function localLLMEnabledToggle('), config);
  config.localLLMChangeFamily('ling');
  assert.deepEqual(saved, {
    'local_llm.model_family': 'ling', 'local_llm.model_variant': 'q4_k_l',
    'local_llm.mtp': 'off', 'local_llm.context_size': 16384
  });
  config.localLLMChangeFamily('qwen');
  assert.equal(saved['local_llm.model_variant'], 'q4_k_m');
  assert.equal(saved['local_llm.mtp'], 'off');
  config.localLLMChangeFamily('spark');
  assert.equal(saved['local_llm.model_family'], 'spark');
  assert.equal(saved['local_llm.model_variant'], 'q4_k_m');
  assert.equal(saved['local_llm.context_size'], 65536);
  assert.equal(saved['local_llm.mtp'], 'off');
  config.localLLMChangeFamily('qwen');
  assert.equal(saved['local_llm.context_size'], 16384);
  config.localLLMChangeFamily('invalid');
  assert.equal(saved['local_llm.model_family'], 'qwen');
  assert.equal(renders, 4);

  const elements = {};
  let invalidations = 0;
  const setup = {
    Option: function(label, value) { this.text = label; this.value = value; },
    document: { getElementById(id) {
      return elements[id] ||= { value: '', disabled: false,
        classList: { toggle() {} }, replaceChildren(...options) { this.options = options; } };
    } },
    onSetupLocalLLMBackendChange() { invalidations++; }
  };
  vm.runInNewContext(sourceBetween(read('ui/js/setup/main.js'),
    'function onSetupLocalLLMFamilyChange(', "document.addEventListener('change'"), setup);
  setup.document.getElementById('setup-local-llm-family').value = 'ling';
  setup.onSetupLocalLLMFamilyChange();
  assert.equal(elements['setup-local-llm-model'].options.length, 1);
  assert.equal(elements['setup-local-llm-model'].options[0].value, 'q4_k_l');
  assert.equal(elements['setup-local-llm-mtp'].value, 'off');
  assert.equal(elements['setup-local-llm-mtp'].disabled, true);
  elements['setup-local-llm-family'].value = 'spark';
  setup.onSetupLocalLLMFamilyChange();
  assert.equal(elements['setup-local-llm-model'].options.length, 1);
  assert.equal(elements['setup-local-llm-model'].options[0].value, 'q4_k_m');
  assert.equal(elements['setup-local-llm-mtp'].disabled, true);
  elements['setup-local-llm-family'].value = 'qwen';
  setup.onSetupLocalLLMFamilyChange();
  assert.equal(elements['setup-local-llm-model'].options.length, 2);
  assert.equal(elements['setup-local-llm-mtp'].disabled, false);
  assert.equal(invalidations, 3);
}

async function testDesktopChatSeparatesStreamedToolRounds() {
  const source = sourceBetween(read('ui/js/desktop/apps/agent-chat.js'),
    '    async function sendDesktopChatStream(', '    function appendChat(');
  for (const streamed of [true, false]) {
    const frames = new Map();
    let frameID = 0;
    const scrolls = [];
    const bubbles = [];
    const announced = [];
    const log = {
      children: [],
      get lastElementChild() { return this.children.at(-1); },
      appendChild(el) {
        this.children = this.children.filter(child => child !== el);
        this.children.push(el);
        el.parentNode = this;
      }
    };
    function element() {
      return {
        dataset: {}, className: '', textContent: '',
        set innerHTML(text) { this.textContent = text; },
        classList: {
          contains(name) { return this.owner.className.split(' ').includes(name); },
          remove(name) { this.owner.className = this.owner.className.replace(name, ''); }
        },
        scrollIntoView() { scrolls.push(this); },
        remove() { log.children = log.children.filter(child => child !== this); this.parentNode = null; }
      };
    }
    function createElement() {
      const el = element();
      el.classList.owner = el;
      return el;
    }
    function flushFrames() {
      while (frames.size) {
        const pending = [...frames.values()];
        frames.clear();
        pending.forEach(callback => callback());
      }
    }
    const renderer = {
      resetDedupSets() {}, createThinkingStatus: createElement,
      appendAvatar(_log, _role, bubble) { log.appendChild(bubble); bubbles.push(bubble); },
      appendTimestamp() {}, renderMarkdown: text => text,
      processImages() {}, enhanceCodeBlocks() {},
      extractToolCallNarration: text => text, updateStatus() {}, formatAgentActionStatus: () => 'Working',
      appendRichBubble(_log, _role, text) {
        const bubble = createElement();
        bubble.textContent = text;
        log.appendChild(bubble);
        bubbles.push(bubble);
      }
    };
    const context = {
      document: { createElement }, AbortController, lastRole: 'user',
      fetch: async () => ({ ok: true }),
      desktopText: key => key, forwardAgentStreamEventToPet() {},
      announceAgentResponseToPet: text => announced.push(text), dispatchDesktopVisibleMessage() {},
      window: {
        DesktopChatRenderer: renderer,
        requestAnimationFrame(callback) { frames.set(++frameID, callback); return frameID; },
        cancelAnimationFrame: id => frames.delete(id),
        AuraChatStreamParser: {
          async readFetchEventStream(_response, { onEvent, onDone }) {
            for (const narration of ['I will inspect the printer status.', 'I will check the active print.']) {
              if (streamed) onEvent({ event: 'llm_stream_delta', content: narration });
              // Leave a pending text frame at the round boundary.
              onEvent({ event: 'tool_call', detail: narration });
              onEvent({ event: 'tool_start' });
              flushFrames();
            }
            assert.equal(bubbles.length, 2, 'tool narration must appear once per round');
            if (streamed) onEvent({ event: 'llm_stream_delta', content: 'Partial final' });
            onEvent({ event: 'final_response', detail: 'The print is 42% complete.' });
            onEvent({ event: 'done' });
            onEvent({ event: 'llm_stream_delta', content: 'late ignored text' });
            onDone();
            flushFrames();
          }
        }
      }
    };
    vm.runInNewContext(source, context);
    await context.sendDesktopChatStream({ querySelector: () => log }, 'Printer progress?', {});
    assert.deepEqual(bubbles.map(bubble => bubble.textContent), [
      'I will inspect the printer status.', 'I will check the active print.', 'The print is 42% complete.'
    ]);
    assert.deepEqual(announced, ['The print is 42% complete.']);
    assert.equal(scrolls.at(-1), log.lastElementChild, 'final scroll must target the latest message');
    assert.equal(frames.size, 0);
  }
}

async function testStoreOperationFailuresRemainVisible() {
  const source = read('ui/js/desktop/apps/software-store.js');
  const notifications = [];
  const operationErrors = new Map();
  const appID = 'gods-eye-view';
  const context = {
    operationErrors, busy: new Map(), pollingOperations: new Set(), instance: { disposed: false },
    t: key => key, renderCards() {}, scheduleLoad() {}, delay: async () => {},
    notify: message => notifications.push(message.message),
    loadBootstrap: async () => { throw new Error('bootstrap unavailable'); },
    api: async () => ({ operation: { status: 'failed', type: 'install', error: 'desktop registration failed' } })
  };
  vm.runInNewContext(sourceBetween(source, 'function showOperationError(', 'function delay('), context);
  await context.pollOperation(appID, 'install-1');
  assert.equal(operationErrors.get(appID), 'desktop registration failed');
  assert.deepEqual(notifications, ['desktop registration failed']);
  assert.equal(context.pollingOperations.size, 0);
  context.api = async () => { throw new Error('operation request unavailable'); };
  await context.pollOperation(appID, 'install-2');
  assert.equal(operationErrors.get(appID), 'operation request unavailable');
  context.api = async () => {
    assert.equal(operationErrors.has(appID), false, 'retry must clear the old error');
    return {};
  };
  await context.startOperation(appID, 'install', '/install', 'POST', {});
  assert.equal(operationErrors.has(appID), false);
  assert.match(source, /role="alert">\$\{esc\(operationError\)\}/, 'card errors must be escaped and accessible');
}

function loadSFTPNavigatorRuntime() {
  const source = read('ui/js/desktop/apps/quickconnect-sftp-navigator.js');
  const context = { AbortController };
  vm.createContext(context);
  vm.runInContext(`${source}\nglobalThis.createSFTPNavigator = createSFTPNavigator; globalThis.joinSFTPPath = joinSFTPPath; globalThis.parentSFTPPath = parentSFTPPath;`, context);
  return context;
}

function createDeferredListing() {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

async function testQuickConnectSFTPNavigatorIgnoresStaleListings() {
  const runtime = loadSFTPNavigatorRuntime();
  const pending = new Map();
  const nav = runtime.createSFTPNavigator((dirPath, signal) => {
    const listing = createDeferredListing();
    pending.set(dirPath, { ...listing, signal });
    return listing.promise;
  });

  const slow = nav.load('/x');
  const fast = nav.load('/');
  assert.equal(pending.get('/x').signal.aborted, true, 'a newer listing must abort the older request');
  pending.get('/').resolve({ entries: [{ name: 'zeta', is_dir: false }, { name: 'alpha', is_dir: true }] });
  assert.equal((await fast).status, 'ok');
  pending.get('/x').resolve({ entries: [{ name: 'data.db', is_dir: false }] });
  assert.equal((await slow).status, 'stale', 'a late listing must not win');
  assert.equal(nav.path, '/');
  assert.deepEqual(Array.from(nav.entries, entry => entry.name), ['alpha', 'zeta'], 'directories sort first');
  assert.equal(runtime.joinSFTPPath(nav.path, 'zeta'), '/zeta');
  assert.equal(runtime.joinSFTPPath('/srv/', 'a.txt'), '/srv/a.txt');

  const denied = nav.load('/root');
  pending.get('/root').reject(new Error('permission denied'));
  const failure = await denied;
  assert.equal(failure.status, 'error');
  assert.equal(failure.error.message, 'permission denied');
  assert.equal(nav.path, '/', 'a failed listing keeps the committed path');

  const late = nav.load('/late');
  nav.dispose();
  assert.equal(pending.get('/late').signal.aborted, true, 'closing the panel aborts the pending listing');
  pending.get('/late').resolve({ entries: [{ name: 'ghost', is_dir: false }] });
  assert.equal((await late).status, 'stale');
  assert.equal((await nav.load('/again')).status, 'disposed');
  assert.equal(nav.path, '/');

  assert.equal(runtime.parentSFTPPath('/a/b/'), '/a');
  assert.equal(runtime.parentSFTPPath('/a'), '/');
  assert.equal(runtime.parentSFTPPath('/'), '/');

  const panel = read('ui/js/desktop/apps/quickconnect-launchpad-chat.js');
  assert.doesNotMatch(panel, /sftpCurrentPath/, 'SFTP actions must not use a shared current path');
  assert.match(panel, /createSFTPNavigator\(\(dirPath, signal\) => api\(/);
  assert.match(panel, /data-path="\$\{esc\(entryPath\)\}"/);
  assert.match(panel, /activeSFTPNav\.dispose\(\)/);
}

function loadDashboardCardRuntime(blockStart, blockEnd, extra) {
  const source = read('ui/js/dashboard/dashboard-widgets.js');
  const gateSource = sourceBetween(source, 'function createLatestRequestGate()', '// ═══ Cronjobs');
  const blockSource = sourceBetween(source, blockStart, blockEnd);
  const pending = [];
  const cardEvents = [];
  const context = {
    AbortController,
    URLSearchParams,
    console: { warn() {} },
    window: {},
    document: { getElementById: () => null },
    t: key => key,
    showToast: () => {},
    DashRouter: { current: () => 'audit' },
    CardState: {
      setLoading: id => cardEvents.push(`loading:${id}`),
      setLoaded: id => cardEvents.push(`loaded:${id}`),
      setError: id => cardEvents.push(`error:${id}`)
    },
    fetch: (url, options) => new Promise(resolve => {
      pending.push({ url: String(url), signal: options && options.signal, resolve });
    }),
    ...extra
  };
  vm.createContext(context);
  vm.runInContext(`${gateSource}\n${blockSource}`, context);
  return { context, pending, cardEvents };
}

const dashboardJSON = body => ({ ok: true, json: async () => body });

async function testDashboardAuditIgnoresLateResponses() {
  const { context, pending, cardEvents } = loadDashboardCardRuntime('// ═══ Audit Log', '// ═══ Mission History', {});
  const rendered = [];
  context.renderAuditEvents = page => rendered.push(page.tag);

  const nextPage = context.loadAuditPage(25);
  const refresh = context.loadAuditPage(0);
  assert.equal(pending.length, 2);
  assert.match(pending[0].url, /offset=25/);
  assert.equal(pending[0].signal.aborted, true, 'a newer audit request must abort the older one');
  pending[1].resolve(dashboardJSON({ tag: 'page-1', entries: [], total: 60 }));
  await refresh;
  pending[0].resolve(dashboardJSON({ tag: 'page-2', entries: [], total: 60 }));
  await nextPage;
  assert.deepEqual(rendered, ['page-1'], 'a late audit response must not replace the newer page');
  assert.equal(vm.runInContext('auditOffset', context), 0, 'the offset belongs to the rendered page');

  const failing = context.loadAuditPage(50);
  pending[2].resolve({ ok: false, json: async () => ({}) });
  await failing;
  assert.equal(vm.runInContext('auditOffset', context), 0, 'a failed page load keeps the committed offset');
  assert.equal(cardEvents.filter(event => event.startsWith('error:')).length, 1);
}

async function testDashboardCronjobsIgnoreLateResponses() {
  const rendered = [];
  const { context, pending, cardEvents } = loadDashboardCardRuntime('// ═══ Cronjobs', 'function renderMemoryCurationPreview(plan)', {
    cronjobsQueryParams: () => new URLSearchParams(),
    renderCronjobs: data => rendered.push(data.tag)
  });

  const older = context.loadTabCronjobs();
  const newer = context.loadTabCronjobs();
  assert.equal(pending[0].signal.aborted, true, 'a newer cronjob search must abort the older one');
  pending[1].resolve(dashboardJSON({ tag: 'new' }));
  await newer;
  pending[0].resolve(dashboardJSON({ tag: 'old' }));
  await older;
  assert.deepEqual(rendered, ['new'], 'a late cronjob response must not replace the newer result');

  const staleFailure = context.loadTabCronjobs();
  const current = context.loadTabCronjobs();
  pending[3].resolve(dashboardJSON({ tag: 'current' }));
  await current;
  pending[2].resolve({ ok: false, json: async () => ({}) });
  await staleFailure;
  assert.deepEqual(rendered, ['new', 'current']);
  assert.equal(cardEvents.some(event => event.startsWith('error:')), false, 'a stale failure must not show an error over current data');
}

function createSQLConnectionsFormContext(source) {
  const elements = new Map();
  const element = id => {
    if (!elements.has(id)) {
      const classes = new Set();
      elements.set(id, {
        id, value: '', checked: false, disabled: false, readOnly: false, placeholder: '', textContent: '', files: [],
        classList: {
          add: name => classes.add(name),
          remove: name => classes.delete(name),
          contains: name => classes.has(name),
          toggle: (name, force) => {
            const on = force === undefined ? !classes.has(name) : !!force;
            if (on) classes.add(name); else classes.delete(name);
            return on;
          }
        },
        closest: () => element(`${id}::group`)
      });
    }
    return elements.get(id);
  };
  const requests = [];
  const context = {
    window: { addEventListener() {} },
    document: { getElementById: element },
    t: key => key,
    configData: {},
    fetch: async (url, opts = {}) => {
      requests.push({ url: String(url), method: opts.method || 'GET', body: typeof opts.body === 'string' ? JSON.parse(opts.body) : null });
      return { ok: true, json: async () => (opts.method ? { id: 'sql-new' } : []) };
    }
  };
  vm.createContext(context);
  vm.runInContext(`${source}\nglobalThis.setSQLConnCacheForTest = list => { sqlConnCache = list; };`, context);
  return { context, element, requests };
}

async function testSQLConnectionsRequireExplicitTLSMode() {
  const source = read('ui/cfg/sql_connections.js');
  const select = sourceBetween(source, '<select id="sqlconn-field-ssl"', '</select>');
  assert.deepEqual([...select.matchAll(/<option value="([^"]*)"/g)].map(match => match[1]),
    ['disable', 'require', 'verify-ca', 'verify-full'], 'the TLS select offers no implicit default');

  const { context, element, requests } = createSQLConnectionsFormContext(source);
  context.sqlConnShowModal();
  assert.equal(element('sqlconn-field-ssl').value, 'require', 'new connections pre-select TLS');
  element('sqlconn-field-name').value = 'Warehouse';
  element('sqlconn-field-database').value = 'warehouse';
  await context.sqlConnSave();
  const created = requests.find(request => request.method === 'POST');
  assert.equal(created?.body?.ssl_mode, 'require', 'the create request states the TLS mode');

  context.setSQLConnCacheForTest([
    { id: 'legacy', name: 'Legacy', driver: 'postgres', ssl_mode: '' },
    { id: 'pinned', name: 'Pinned', driver: 'mysql', ssl_mode: 'verify-full' }
  ]);
  context.sqlConnShowModal('legacy');
  assert.equal(element('sqlconn-field-ssl').value, 'disable', 'a stored connection without a mode shows what the server applied');
  context.sqlConnShowModal('pinned');
  assert.equal(element('sqlconn-field-ssl').value, 'verify-full');

  requests.length = 0;
  context.sqlConnShowModal();
  element('sqlconn-field-driver').value = 'sqlite';
  context.sqlConnDriverChanged();
  element('sqlconn-field-name').value = 'Local file';
  await context.sqlConnSave();
  assert.equal(requests.find(request => request.method === 'POST')?.body?.ssl_mode, '',
    'SQLite keeps the server-side default for the hidden TLS field');
}

async function testContainersListFailureStateSurvivesFiltersUntilTheListLoads() {
  const source = read('ui/js/containers/main.js');
  const nodes = {};
  const hiddenAtStart = new Set(['ct-empty', 'ct-disabled', 'ct-list-error']);
  function makeNode(id) {
    const classes = new Set(hiddenAtStart.has(id) ? ['is-hidden'] : []);
    return {
      id, style: {}, value: '', parentNode: null, children: [], cardHTML: '', textSets: 0, htmlSets: 0, _text: '', _html: '',
      classList: {
        add: name => classes.add(name),
        remove: name => classes.delete(name),
        contains: name => classes.has(name),
        toggle(name, force) { if (force === undefined ? !classes.has(name) : force) classes.add(name); else classes.delete(name); }
      },
      addEventListener() {},
      setAttribute() {},
      get firstChild() { return this.children[0] || null; },
      get textContent() { return this._text; },
      set textContent(value) { this._text = String(value); this.textSets += 1; },
      get innerHTML() { return this._html; },
      set innerHTML(value) { this._html = String(value); this.htmlSets += 1; },
      replaceChildren() { for (const child of this.children) child.parentNode = null; this.children = []; },
      appendChild(child) { child.parentNode = this; this.children.push(child); },
      insertBefore(child, ref) { child.parentNode = this; this.children.splice(ref ? this.children.indexOf(ref) : this.children.length, 0, child); },
      remove() { const parent = this.parentNode; if (parent) parent.children.splice(parent.children.indexOf(this), 1); this.parentNode = null; },
      after(child) { const parent = this.parentNode; child.parentNode = parent; parent.children.splice(parent.children.indexOf(this) + 1, 0, child); },
      replaceWith(child) { const parent = this.parentNode; child.parentNode = parent; parent.children.splice(parent.children.indexOf(this), 1, child); this.parentNode = null; }
    };
  }
  const node = id => nodes[id] || (nodes[id] = makeNode(id));

  const sseHandlers = {};
  const requests = [];
  let domReady = null;
  let reply = null;
  const timers = new Map();
  let nextTimer = 1;
  const context = {
    document: {
      getElementById: node,
      addEventListener(name, handler) { if (name === 'DOMContentLoaded') domReady = handler; },
      querySelectorAll() { return []; },
      createElement() {
        return {
          content: { firstElementChild: null },
          set innerHTML(html) {
            const card = makeNode('card');
            card.cardHTML = String(html);
            this.content = { firstElementChild: card };
          }
        };
      }
    },
    window: { AuraSSE: { on(name, handler) { sseHandlers[name] = handler; } }, location: { protocol: 'http:', host: 'aurago.test' } },
    console: { error() {}, log() {} },
    fetch: async url => {
      requests.push(String(url));
      if (reply instanceof Error) throw reply;
      return reply;
    },
    t: key => key,
    esc: value => String(value ?? ''),
    escAttr: value => String(value ?? '').replace(/'/g, '&#39;').replace(/"/g, '&quot;'),
    applyI18n() {},
    showToast() {},
    setInterval,
    clearInterval,
    setTimeout(callback, ms) { const id = nextTimer++; timers.set(id, { callback, ms }); return id; },
    clearTimeout(id) { timers.delete(id); }
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  const run = expression => vm.runInContext(expression, context);
  // fire runs the single pending retry timer the way the browser would.
  const fire = async () => {
    assert.equal(timers.size, 1, 'exactly one retry timer must be pending');
    const [id, timer] = [...timers.entries()][0];
    timers.delete(id);
    timer.callback();
    await flush();
  };
  const flush = () => new Promise(resolve => setTimeout(resolve, 0));
  const json = (status, body) => ({ status, ok: status >= 200 && status < 300, json: async () => body });
  const proxyPage = status => ({ status, ok: false, json: async () => { throw new SyntaxError('Unexpected token <'); } });
  const view = () => ({
    errorShown: !node('ct-list-error').classList.contains('is-hidden'),
    disabledShown: !node('ct-disabled').classList.contains('is-hidden'),
    message: node('ct-list-error-message').textContent,
    grid: node('ct-grid').style.display,
    statusBar: node('ct-status-bar').style.display,
    cards: node('ct-grid').children.length
  });
  const listed = [{ id: 'abc', names: ['/web'], image: 'web:1', state: 'running', status: 'Up 1 minute', protected_owner: 'go2rtc' }];
  const pushed = [{ id: 'abc', names: ['/web'], image: 'web:1', state: 'running', status: 'Up 1 minute' }];

  // Initial load: the card shows K12's protected badge.
  reply = json(200, { status: 'ok', containers: listed });
  domReady();
  await flush();
  assert.deepEqual(view(), { errorShown: false, disabledShown: false, message: '', grid: '', statusBar: '', cards: 1 });
  assert.match(node('ct-grid').children[0].cardHTML, /containers\.protected_badge/);

  // K12's SSE merge keeps the last list's flags and does not reload the list.
  const requestsBeforeMerge = requests.length;
  sseHandlers.container_update(pushed);
  await flush();
  assert.equal(requests.length, requestsBeforeMerge, 'a known container must merge without a reload');
  assert.match(node('ct-grid').children[0].cardHTML, /containers\.protected_badge/);

  // HTTP 502 with Docker's message: error state, text set as text, no stale cards.
  reply = json(502, { status: 'error', message: 'Docker error (HTTP 409): engine refused' });
  await run('loadContainers()');
  assert.deepEqual(view(), { errorShown: true, disabledShown: false, message: 'Docker error (HTTP 409): engine refused', grid: 'none', statusBar: 'none', cards: 0 });
  assert.equal(node('ct-list-error-message').htmlSets, 0, 'the message must be set as text');
  assert.equal(run('allContainers.length'), 0);
  assert.equal(timers.size, 1, 'a failed list arms exactly one retry');
  assert.equal([...timers.values()][0].ms, 10000);

  // Search and filter input keep the error state and bring no stale card back.
  node('ct-search').value = 'web';
  run('filterContainers()');
  run("setFilter('running')");
  assert.deepEqual(view(), { errorShown: true, disabledShown: false, message: 'Docker error (HTTP 409): engine refused', grid: 'none', statusBar: 'none', cards: 0 });

  assert.equal(timers.size, 1, 'filter input must not add or drop the retry');

  // A pushed update means Docker answers again: the page reloads the list.
  const requestsBeforePush = requests.length;
  reply = json(200, { status: 'ok', containers: listed });
  sseHandlers.container_update(pushed);
  await flush();
  assert.equal(requests.length, requestsBeforePush + 1, 'an update during the error state must reload the list');
  assert.deepEqual({ ...view(), message: '' }, { errorShown: false, disabledShown: false, message: '', grid: '', statusBar: '', cards: 1 });
  assert.match(node('ct-grid').children[0].cardHTML, /containers\.protected_badge/);

  assert.equal(timers.size, 0, 'a successful load clears the retry');

  // HTTP 503 keeps the Docker-disabled state, also against search input.
  reply = json(503, { status: 'error', message: 'Docker is not enabled' });
  await run('loadContainers()');
  run('filterContainers()');
  assert.deepEqual({ ...view(), message: '' }, { errorShown: false, disabledShown: true, message: '', grid: 'none', statusBar: 'none', cards: 0 });
  assert.equal(timers.size, 0, 'Docker disabled does not retry');

  // A successful load ends the unavailable state.
  node('ct-search').value = '';
  reply = json(200, { status: 'ok', containers: listed });
  await run('loadContainers()');
  assert.deepEqual({ ...view(), message: '' }, { errorShown: false, disabledShown: false, message: '', grid: '', statusBar: '', cards: 1 });

  // A proxy that replaced the 502 body with HTML still yields the error state.
  reply = proxyPage(502);
  await run('loadContainers()');
  assert.deepEqual(view(), { errorShown: true, disabledShown: false, message: 'common.error', grid: 'none', statusBar: 'none', cards: 0 });
  assert.equal(timers.size, 1, 'the unreadable-answer path retries too');

  // A JSON error without `message` (jsonError shape) shows its `error` text.
  reply = json(403, { error: 'invalid_bearer_scope' });
  await run('loadContainers()');
  assert.equal(view().message, 'invalid_bearer_scope');
  assert.equal(view().errorShown, true);
  assert.equal(timers.size, 1, 'a new failure re-arms the retry instead of adding a second timer');

  // The timer fires while Docker still fails: one new timer replaces it.
  reply = json(502, { status: 'error', message: 'still down' });
  await fire();
  assert.equal(view().message, 'still down');
  assert.equal(view().errorShown, true);
  assert.equal(timers.size, 1);

  // The retry cannot reach AuraGo at all: it keeps retrying.
  reply = new Error('offline');
  await fire();
  assert.equal(view().errorShown, true);
  assert.equal(timers.size, 1);

  // Docker is back: the retry loads the list, ends the error state and leaves no timer.
  reply = json(200, { status: 'ok', containers: listed });
  await fire();
  assert.deepEqual({ ...view(), message: '' }, { errorShown: false, disabledShown: false, message: '', grid: '', statusBar: '', cards: 1 });
  assert.equal(timers.size, 0);

  // Recovery with an empty list ends the error state and the retry too.
  reply = json(502, { status: 'error', message: 'down again' });
  await run('loadContainers()');
  assert.equal(timers.size, 1);
  reply = json(200, { status: 'ok', containers: [] });
  await fire();
  assert.equal(view().errorShown, false);
  assert.equal(view().statusBar, '');
  assert.equal(view().cards, 0);
  assert.equal(timers.size, 0);
}

// containersPage loads ui/js/containers/main.js into a fake DOM with fake
// fetch, WebSocket, xterm, timers and showModal. route({url, method}) answers
// every fetch with {status, body}.
function containersPage(route) {
  const source = read('ui/js/containers/main.js');
  const hiddenAtStart = new Set(['ct-empty', 'ct-disabled', 'ct-list-error', 'update-protected-warning', 'delete-protected-warning']);
  const nodes = {};
  function makeNode(id) {
    const classes = new Set(hiddenAtStart.has(id) ? ['is-hidden'] : []);
    return {
      id, style: {}, value: '', checked: false, disabled: false, parentNode: null, children: [], cardHTML: '', _text: '', _html: '',
      classList: {
        add: name => classes.add(name),
        remove: name => classes.delete(name),
        contains: name => classes.has(name),
        toggle(name, force) { if (force === undefined ? !classes.has(name) : force) classes.add(name); else classes.delete(name); }
      },
      addEventListener() {},
      setAttribute() {},
      get firstChild() { return this.children[0] || null; },
      get textContent() { return this._text; },
      set textContent(value) { this._text = String(value); },
      get innerHTML() { return this._html; },
      set innerHTML(value) { this._html = String(value); },
      replaceChildren() { for (const child of this.children) child.parentNode = null; this.children = []; },
      appendChild(child) { child.parentNode = this; this.children.push(child); },
      insertBefore(child, ref) { child.parentNode = this; this.children.splice(ref ? this.children.indexOf(ref) : this.children.length, 0, child); },
      remove() { const parent = this.parentNode; if (parent) parent.children.splice(parent.children.indexOf(this), 1); this.parentNode = null; },
      after(child) { const parent = this.parentNode; child.parentNode = parent; parent.children.splice(parent.children.indexOf(this) + 1, 0, child); },
      replaceWith(child) { const parent = this.parentNode; child.parentNode = parent; parent.children.splice(parent.children.indexOf(this), 1, child); this.parentNode = null; }
    };
  }
  const node = id => nodes[id] || (nodes[id] = makeNode(id));
  const requests = [];
  const sockets = [];
  const sse = {};
  const timers = [];
  const modals = [];
  let modalAnswer = false;
  let domReady = null;
  class FakeWebSocket {
    constructor(url) { this.url = String(url); this.readyState = FakeWebSocket.CONNECTING; this.sent = []; sockets.push(this); }
    send(data) { this.sent.push(data); }
    close() { this.readyState = FakeWebSocket.CLOSED; }
    open() { this.readyState = FakeWebSocket.OPEN; if (this.onopen) this.onopen(); }
    // A refused handshake (HTTP 409 etc.): browsers fire error and close, never open.
    failHandshake() { this.readyState = FakeWebSocket.CLOSED; if (this.onerror) this.onerror(); if (this.onclose) this.onclose({ code: 1006 }); }
    closeFromServer() { this.readyState = FakeWebSocket.CLOSED; if (this.onclose) this.onclose({ code: 1000 }); }
  }
  FakeWebSocket.CONNECTING = 0;
  FakeWebSocket.OPEN = 1;
  FakeWebSocket.CLOSING = 2;
  FakeWebSocket.CLOSED = 3;
  class FakeTerminal {
    constructor() { this.lines = []; this.cols = 80; this.rows = 24; }
    loadAddon() {}
    open() {}
    focus() {}
    onData(handler) { this.dataHandler = handler; }
    write(text) { this.lines.push(String(text)); }
    writeln(text) { this.lines.push(String(text)); }
    dispose() { this.disposed = true; }
  }
  const context = {
    document: {
      getElementById: node,
      addEventListener(name, handler) { if (name === 'DOMContentLoaded') domReady = handler; },
      querySelectorAll() { return []; },
      createElement() {
        return {
          content: { firstElementChild: null },
          set innerHTML(html) {
            const card = makeNode('card');
            card.cardHTML = String(html);
            this.content = { firstElementChild: card };
          }
        };
      }
    },
    window: {
      AuraSSE: { on(name, handler) { sse[name] = handler; } },
      location: { protocol: 'http:', host: 'aurago.test' },
      Terminal: FakeTerminal,
      addEventListener() {},
      removeEventListener() {}
    },
    WebSocket: FakeWebSocket,
    TextEncoder,
    TextDecoder,
    console: { error() {}, log() {} },
    fetch: async (url, options = {}) => {
      const request = { url: String(url), method: options.method || 'GET' };
      requests.push(request);
      const { status, body } = route(request);
      return { status, ok: status >= 200 && status < 300, json: async () => body };
    },
    t: key => key,
    esc: value => String(value ?? ''),
    escAttr: value => String(value ?? '').replace(/'/g, '&#39;').replace(/"/g, '&quot;'),
    applyI18n() {},
    showToast() {},
    showModal: async (title, message, isConfirm, options) => { modals.push({ title, message, isConfirm, options }); return modalAnswer; },
    setTimeout(callback, ms) { timers.push({ callback, ms, fired: false }); return timers.length; },
    clearTimeout(id) { if (timers[id - 1]) timers[id - 1].fired = true; },
    setInterval,
    clearInterval
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  const flush = () => new Promise(resolve => setTimeout(resolve, 0));
  return {
    node, requests, sockets, sse, modals, flush,
    run: expression => vm.runInContext(expression, context),
    async start() { domReady(); await flush(); },
    active: id => node(id).classList.contains('active'),
    setModalAnswer(answer) { modalAnswer = answer; },
    fireTimer(ms) {
      const timer = timers.find(entry => entry.ms === ms && !entry.fired);
      assert.ok(timer, `no pending ${ms} ms timer`);
      timer.fired = true;
      timer.callback();
    }
  };
}

async function testContainersSSEMergeKeepsFlagsAndReloadsUnknownContainers() {
  const listed = [
    { id: 'cams1', names: ['/cams'], image: 'go2rtc:1', state: 'running', status: 'Up 1 minute', protected_owner: 'go2rtc' },
    { id: 'app1', names: ['/aurago'], image: 'aurago:1', state: 'running', status: 'Up 1 minute', protected_owner: 'aurago-app', self: true }
  ];
  let list = listed;
  const page = containersPage(request => request.url === '/api/containers'
    ? { status: 200, body: { status: 'ok', containers: list } }
    : { status: 200, body: { status: 'ok' } });
  await page.start();
  const listLoads = () => page.requests.filter(r => r.url === '/api/containers').length;
  assert.equal(listLoads(), 1);

  // A push of known containers carries no flags: the last list's flags stay
  // and the changed state is shown without a reload.
  page.sse.container_update([
    { id: 'cams1', names: ['/cams'], image: 'go2rtc:1', state: 'exited', status: 'Exited (0)' },
    { id: 'app1', names: ['/aurago'], image: 'aurago:1', state: 'running', status: 'Up 2 minutes' }
  ]);
  await page.flush();
  assert.equal(listLoads(), 1, 'known containers merge without a reload');
  assert.equal(page.run("findContainer('cams1').protected_owner"), 'go2rtc');
  assert.equal(page.run("findContainer('cams1').state"), 'exited');
  assert.equal(page.run("containerProtection(findContainer('app1'))"), 'self');

  // An unknown container reloads the list, so its flags are known before any action.
  list = [...listed, { id: 'proxy1', names: ['/docker-proxy'], image: 'proxy', state: 'running', status: 'Up', docker_endpoint: true }];
  page.sse.container_update([
    ...listed.map(c => ({ id: c.id, names: c.names, image: c.image, state: c.state, status: c.status })),
    { id: 'proxy1', names: ['/docker-proxy'], image: 'proxy', state: 'running', status: 'Up' }
  ]);
  await page.flush();
  assert.equal(listLoads(), 2, 'an unknown container reloads the list');
  assert.equal(page.run("containerProtection(findContainer('proxy1'))"), 'docker-endpoint');

  // The newly known protected container asks before its shell opens.
  page.run("showTerminal('proxy1', 'docker-proxy')");
  assert.equal(page.active('protected-terminal-modal'), true);
  assert.equal(page.sockets.length, 0);
}

async function testContainersSendConfirmProtectedOnlyAfterTheModal() {
  const listed = [
    { id: 'web1', names: ['/web'], image: 'nginx:1', state: 'running', status: 'Up 1 minute' },
    { id: 'cams1', names: ['/cams'], image: 'go2rtc:1', state: 'running', status: 'Up 1 minute', protected_owner: 'go2rtc' },
    { id: 'app1', names: ['/aurago'], image: 'aurago:1', state: 'running', status: 'Up 1 minute', protected_owner: 'aurago-app', self: true }
  ];
  let actionReply = { status: 200, body: { status: 'ok' } };
  const page = containersPage(request => request.url === '/api/containers'
    ? { status: 200, body: { status: 'ok', containers: listed } }
    : actionReply);
  await page.start();
  assert.equal(page.node('ct-grid').children.length, 3);

  // Shell: unprotected opens at once without the flag.
  page.run("showTerminal('web1', 'web')");
  assert.equal(page.sockets.length, 1);
  assert.equal(page.sockets[0].url, 'ws://aurago.test/api/containers/web1/terminal');
  page.run('closeTerminalModal()');

  // Protected: the modal comes first; cancelling opens nothing.
  page.run("showTerminal('cams1', 'cams')");
  assert.equal(page.active('protected-terminal-modal'), true);
  assert.equal(page.node('protected-terminal-warning').textContent, 'containers.protected_warning');
  assert.equal(page.sockets.length, 1, 'no WebSocket before the operator confirms');
  page.run('closeProtectedTerminalModal()');
  assert.equal(page.sockets.length, 1, 'cancelling opens no WebSocket');
  page.run("showTerminal('cams1', 'cams')");
  page.run('confirmProtectedTerminal()');
  assert.equal(page.sockets.length, 2);
  assert.equal(page.sockets[1].url, 'ws://aurago.test/api/containers/cams1/terminal?confirm=protected');
  page.run('closeTerminalModal()');

  // Update: the flag only from the protected modal; self is disabled.
  page.run("showUpdateModal('web1', 'web')");
  await page.run('confirmUpdate()');
  page.run("showUpdateModal('cams1', 'cams')");
  assert.equal(page.node('update-protected-warning').classList.contains('is-hidden'), false);
  await page.run('confirmUpdate()');
  page.run("showUpdateModal('app1', 'aurago')");
  assert.equal(page.node('update-confirm-btn').disabled, true);
  await page.run('confirmUpdate()');
  page.run('closeUpdateModal()');
  assert.deepEqual(page.requests.filter(r => r.method === 'POST' && r.url.includes('/update')).map(r => r.url),
    ['/api/containers/web1/update', '/api/containers/cams1/update?confirm=protected']);

  // A stale list: the server asks (409). The page shows the warning and waits;
  // only the operator's second confirm sends the flag.
  actionReply = { status: 409, body: { status: 'error', code: 'container_protected_confirmation_required', owner: 'unverified', message: 'x' } };
  page.run("showUpdateModal('web1', 'web')");
  await page.run('confirmUpdate()');
  assert.equal(page.node('update-protected-warning').textContent, 'containers.protected_unverified_warning');
  assert.equal(page.active('update-modal'), true);
  actionReply = { status: 200, body: { status: 'ok' } };
  await page.run('confirmUpdate()');
  assert.deepEqual(page.requests.filter(r => r.method === 'POST' && r.url.startsWith('/api/containers/web1/update')).map(r => r.url),
    ['/api/containers/web1/update', '/api/containers/web1/update', '/api/containers/web1/update?confirm=protected']);

  // Remove follows the same rule.
  page.run("showDeleteModal('web1', 'web')");
  await page.run('confirmDelete()');
  page.run("showDeleteModal('cams1', 'cams')");
  await page.run('confirmDelete()');
  actionReply = { status: 409, body: { status: 'error', code: 'container_protected_confirmation_required', owner: 'go2rtc', message: 'x' } };
  page.run("showDeleteModal('web1', 'web')");
  await page.run('confirmDelete()');
  assert.equal(page.active('delete-modal'), true);
  actionReply = { status: 200, body: { status: 'ok' } };
  await page.run('confirmDelete()');
  assert.deepEqual(page.requests.filter(r => r.method === 'DELETE').map(r => r.url), [
    '/api/containers/web1?force=false',
    '/api/containers/cams1?force=false&confirm=protected',
    '/api/containers/web1?force=false',
    '/api/containers/web1?force=false&confirm=protected'
  ]);
}

async function testContainersEmptyStateFollowsTheList() {
  let list = [];
  let status = 200;
  const page = containersPage(request => {
    if (request.url !== '/api/containers') return { status: 200, body: { status: 'ok' } };
    if (status === 200) return { status, body: { status: 'ok', containers: list } };
    return { status, body: { status: 'error', message: status === 503 ? 'Docker is not enabled' : 'down' } };
  });
  const emptyShown = () => !page.node('ct-empty').classList.contains('is-hidden');
  await page.start();
  assert.equal(emptyShown(), true, 'an empty list shows the empty state');

  list = [{ id: 'web1', names: ['/web'], image: 'nginx', state: 'running', status: 'Up' }];
  await page.run('loadContainers()');
  assert.equal(emptyShown(), false);

  page.node('ct-search').value = 'nomatch';
  page.run('filterContainers()');
  assert.equal(emptyShown(), true, 'a filter without matches shows the empty state');
  page.node('ct-search').value = '';
  page.run('filterContainers()');
  assert.equal(emptyShown(), false);

  list = [];
  await page.run('loadContainers()');
  assert.equal(emptyShown(), true);
  status = 502;
  await page.run('loadContainers()');
  assert.equal(emptyShown(), false, 'the list error replaces the empty state');
  status = 200;
  await page.run('loadContainers()');
  assert.equal(emptyShown(), true, 'an empty list after the error shows the empty state');
  status = 503;
  await page.run('loadContainers()');
  assert.equal(emptyShown(), false, 'Docker disabled replaces the empty state');
}

async function testContainersResumeUnpausesAPausedContainer() {
  const listed = [{ id: 'p1', names: ['/paused'], image: 'nginx', state: 'paused', status: 'Up 1 minute (Paused)' }];
  const page = containersPage(request => request.url === '/api/containers'
    ? { status: 200, body: { status: 'ok', containers: listed } }
    : { status: 200, body: { status: 'ok', action: 'unpause' } });
  await page.start();
  const card = page.node('ct-grid').children[0].cardHTML;
  assert.match(card, /containerAction\([^)]*'unpause'\)" data-i18n="containers\.btn_unpause"/);
  assert.doesNotMatch(card, /'start'\)" data-i18n="containers\.btn_unpause"/, 'Docker refuses start on a paused container');
  await page.run("containerAction('p1', 'unpause')");
  assert.deepEqual(page.requests.filter(r => r.method === 'POST').map(r => r.url), ['/api/containers/p1/unpause']);
}

async function testContainersTerminalOffersConfirmationAfterARefusedHandshake() {
  const listed = [{ id: 'web1', names: ['/web'], image: 'nginx:1', state: 'running', status: 'Up 1 minute' }];
  let report = { status: 'ok', container_id: 'web1', owner: 'unverified', protected: true, update_unsupported: false, read_only: false, message: 'Docker did not answer the ownership check for this container. Repeat the request with confirm=protected to continue.' };
  const page = containersPage(request => {
    if (request.url === '/api/containers') return { status: 200, body: { status: 'ok', containers: listed } };
    if (request.url === '/api/containers/web1/protection') return { status: 200, body: report };
    return { status: 200, body: { status: 'ok' } };
  });
  await page.start();
  const checks = () => page.requests.filter(r => r.url === '/api/containers/web1/protection').length;

  // The list shows no protection; the server refuses the handshake (409).
  page.run("showTerminal('web1', 'web')");
  page.sockets[0].failHandshake();
  await page.flush();
  assert.equal(checks(), 1);
  assert.equal(page.active('terminal-modal'), false);
  assert.equal(page.active('protected-terminal-modal'), true);
  assert.equal(page.node('protected-terminal-warning').textContent, 'containers.protected_unverified_warning');
  assert.equal(page.sockets.length, 1, 'the page never retries with the flag on its own');

  page.run('confirmProtectedTerminal()');
  assert.equal(page.sockets[1].url, 'ws://aurago.test/api/containers/web1/terminal?confirm=protected');

  // A confirmed attempt that still fails shows the error and asks no second time.
  page.sockets[1].failHandshake();
  await page.flush();
  assert.equal(page.active('protected-terminal-modal'), false);
  assert.equal(page.node('terminal-status').textContent, 'containers.terminal_error');

  // A failure that needs no confirmation shows the error.
  page.run('closeTerminalModal()');
  report = { status: 'ok', container_id: 'web1', owner: '', protected: false, update_unsupported: false, read_only: false };
  page.run("showTerminal('web1', 'web')");
  page.sockets[2].failHandshake();
  await page.flush();
  assert.equal(page.active('protected-terminal-modal'), false);
  assert.equal(page.node('terminal-status').textContent, 'containers.terminal_error');

  // Docker read-only: no pointless confirmation.
  page.run('closeTerminalModal()');
  report = { status: 'ok', container_id: 'web1', owner: 'go2rtc', protected: true, update_unsupported: false, read_only: true };
  page.run("showTerminal('web1', 'web')");
  page.sockets[3].failHandshake();
  await page.flush();
  assert.equal(page.active('protected-terminal-modal'), false);

  // A session that opened and closes later is a normal close, not a refusal.
  page.run('closeTerminalModal()');
  page.run("showTerminal('web1', 'web')");
  page.sockets[4].open();
  page.sockets[4].closeFromServer();
  await page.flush();
  assert.equal(checks(), 4);
  assert.equal(page.node('terminal-status').textContent, 'containers.terminal_closed');
}

async function testContainersEndSessionAsksTheServerAndCloseKeepsTheShell() {
  const listed = [{ id: 'web1', names: ['/web'], image: 'nginx:1', state: 'running', status: 'Up' }];
  const page = containersPage(request => request.url === '/api/containers'
    ? { status: 200, body: { status: 'ok', containers: listed } }
    : { status: 200, body: { status: 'ok' } });
  await page.start();
  const endMessages = socket => socket.sent.filter(m => m === JSON.stringify({ type: 'end' })).length;

  // Close sends nothing to the shell (tmux and screen survive).
  page.run("showTerminal('web1', 'web')");
  assert.equal(page.node('terminal-end-btn').disabled, true, 'End session waits for the connection');
  page.sockets[0].open();
  assert.equal(page.node('terminal-end-btn').disabled, false);
  page.run('closeTerminalModal()');
  assert.equal(endMessages(page.sockets[0]), 0);

  // End session sends one control message and closes the window once the shell exited.
  page.run("showTerminal('web1', 'web')");
  page.sockets[1].open();
  page.run('endTerminalSession()');
  page.run('endTerminalSession()');
  assert.equal(endMessages(page.sockets[1]), 1);
  assert.equal(page.node('terminal-status').textContent, 'containers.terminal_ending');
  page.sockets[1].closeFromServer();
  assert.equal(page.active('terminal-modal'), false);

  // A shell that does not end in time is reported; the session stays usable.
  page.run("showTerminal('web1', 'web')");
  page.sockets[2].open();
  page.run('endTerminalSession()');
  page.fireTimer(5000);
  assert.equal(page.node('terminal-status').textContent, 'containers.terminal_end_failed');
  assert.equal(page.node('terminal-end-btn').disabled, false);
  assert.equal(page.active('terminal-modal'), true);
}

async function testContainersProtectedBadgeIsNeutralWithTheReasonAsTooltip() {
  const listed = [
    { id: 'web1', names: ['/web'], image: 'nginx', state: 'running', status: 'Up' },
    { id: 'cams1', names: ['/cams'], image: 'go2rtc', state: 'running', status: 'Up', protected_owner: 'go2rtc' },
    { id: 'app1', names: ['/aurago'], image: 'aurago', state: 'running', status: 'Up', protected_owner: 'aurago-app', self: true },
    { id: 'proxy1', names: ['/docker-proxy'], image: 'proxy', state: 'running', status: 'Up', docker_endpoint: true },
    { id: 'ts1', names: ['/tailscale'], image: 'tailscale', state: 'running', status: 'Up', shared_network: true },
    { id: 'odd1', names: ['/odd'], image: 'odd', state: 'running', status: 'Up', protected_owner: 'x"y' }
  ];
  const page = containersPage(request => request.url === '/api/containers'
    ? { status: 200, body: { status: 'ok', containers: listed } }
    : { status: 200, body: { status: 'ok' } });
  await page.start();
  const badges = Object.fromEntries(page.node('ct-grid').children.map(card => {
    const id = /data-id="([^"]+)"/.exec(card.cardHTML)[1];
    const badge = /<span class="ct-card-protected"[^>]*>[^<]*<\/span>/.exec(card.cardHTML);
    return [id, badge ? badge[0] : ''];
  }));
  assert.equal(badges.web1, '');
  for (const [id, kind, reason] of [
    ['cams1', 'go2rtc', 'containers.protected_warning (go2rtc)'],
    ['app1', 'self', 'containers.protected_self_warning'],
    ['proxy1', 'docker-endpoint', 'containers.protected_endpoint_warning'],
    ['ts1', 'shared-network', 'containers.protected_network_warning']
  ]) {
    assert.ok(badges[id].includes(`data-protection="${kind}"`), `${id}: ${badges[id]}`);
    assert.ok(badges[id].includes(`title="${reason}"`), `${id} tooltip: ${badges[id]}`);
    assert.ok(badges[id].endsWith('>containers.protected_badge</span>'), `${id} badge text: ${badges[id]}`);
  }
  // A quote in an owner label cannot end the attributes (esc covers & < >).
  assert.ok(badges.odd1.includes('data-protection="x&quot;y"'), `odd1: ${badges.odd1}`);
  assert.ok(badges.odd1.includes('title="containers.protected_warning (x&quot;y)"'), `odd1 tooltip: ${badges.odd1}`);
}

async function testContainersStopAsksOnlyForSelfEndpointAndSharedNetwork() {
  const listed = [
    { id: 'web1', names: ['/web'], state: 'running', status: 'Up', image: 'nginx' },
    { id: 'cams1', names: ['/cams'], state: 'running', status: 'Up', image: 'go2rtc', protected_owner: 'go2rtc' },
    { id: 'app1', names: ['/aurago'], state: 'running', status: 'Up', image: 'aurago', protected_owner: 'aurago-app', self: true },
    { id: 'proxy1', names: ['/docker-proxy'], state: 'running', status: 'Up', image: 'proxy', docker_endpoint: true },
    { id: 'ts1', names: ['/tailscale'], state: 'running', status: 'Up', image: 'tailscale', shared_network: true }
  ];
  const page = containersPage(request => request.url === '/api/containers'
    ? { status: 200, body: { status: 'ok', containers: listed } }
    : { status: 200, body: { status: 'ok', action: 'stop' } });
  await page.start();
  const stops = () => page.requests.filter(r => r.method === 'POST' && r.url.endsWith('/stop')).map(r => r.url);

  page.setModalAnswer(false);
  await page.run("containerAction('web1', 'stop')");
  await page.run("containerAction('cams1', 'stop')");
  await page.run("containerAction('app1', 'restart')");
  assert.equal(page.modals.length, 0, 'unprotected, managed and restart do not ask');

  for (const id of ['app1', 'proxy1', 'ts1']) await page.run(`containerAction('${id}', 'stop')`);
  assert.deepEqual(page.modals.map(m => m.message), ['containers.stop_self_warning', 'containers.stop_endpoint_warning', 'containers.stop_network_warning']);
  assert.ok(page.modals.every(m => m.title === 'containers.stop_protected_title' && m.isConfirm && m.options.confirmText === 'containers.stop_protected_confirm_btn'));
  assert.deepEqual(stops(), ['/api/containers/web1/stop', '/api/containers/cams1/stop'], 'cancelling sends nothing');

  page.setModalAnswer(true);
  await page.run("containerAction('app1', 'stop')");
  assert.equal(stops().at(-1), '/api/containers/app1/stop', 'confirming stops it; the API call is unchanged');
}

function listDesktopMainBundleParts() {
  const script = read('scripts/build-ui-bundles.js');
  const start = script.indexOf('const desktopMainParts = [');
  assert.notEqual(start, -1, 'missing desktopMainParts in scripts/build-ui-bundles.js');
  const end = script.indexOf('];', start);
  return [...script.slice(start, end).matchAll(/'(ui\/[^']+\.js)'/g)].map(match => match[1]);
}

function testDesktopMainBundlePartsEndAtFunctionBoundaries() {
  const parts = listDesktopMainBundleParts();
  const opener = 'ui/js/desktop/core/desktop-foundation.js';
  const closer = 'ui/js/desktop/core/sdk-events-bootstrap.js';
  assert.equal(parts.includes(opener), true, 'desktop-foundation.js opens the shell IIFE');
  assert.equal(parts.includes(closer), true, 'sdk-events-bootstrap.js closes the shell IIFE');
  const failures = [];
  for (const part of parts) {
    const source = read(part).replace(/^﻿/, '');
    const wrapped = part === opener ? `${source}\n})();`
      : part === closer ? `(function () {\n${source}`
      : `(function () {\n${source}\n})`;
    try {
      new vm.Script(wrapped, { filename: part });
    } catch (error) {
      failures.push(`${part}: ${error.message}`);
    }
  }
  assert.deepEqual(failures, [], `desktop main bundle parts must start and end at function boundaries:\n${failures.join('\n')}`);
}

// A window menu item's shortcutHint is drawn in the key column like a shortcut, but the desktop
// does not dispatch it: the app handles that key itself. Clicking the item still runs it.
function testWindowMenuShortcutHintIsDrawnButNotDispatched() {
  const routing = read('ui/js/desktop/core/menus-and-routing.js');
  const source = [
    read('ui/js/desktop/core/shortcut-runtime.js'),
    sourceBetween(routing, 'function contextMenuShortcutMarkup(', 'function formatDesktopDate('),
    sourceBetween(routing, 'function normalizeWindowMenuItems(', 'function normalizeWindowMenus('),
    sourceBetween(routing, 'function renderWindowMenuItems(', 'function setWindowMenus('),
    sourceBetween(routing, 'function runWindowMenuAction(', 'function renderAppContent(')
  ].join('\n');
  const ran = [];
  const context = {
    esc: value => String(value).replace(/[&<>"']/g, ch => `&#${ch.charCodeAt(0)};`),
    menuLabel: item => item.label,
    iconMarkup: () => '',
    closeWindowMenu: () => {},
    state: { activeWindowId: 'w1', windowMenus: new Map() }
  };
  vm.createContext(context);
  vm.runInContext(`${source}; globalThis.menu = { contextMenuShortcutMarkup, normalizeWindowMenuItems, renderWindowMenuItems, handleWindowMenuShortcut, runWindowMenuAction };`, context);
  const { menu } = context;
  const actions = new Map();
  const items = menu.normalizeWindowMenuItems([
    { id: 'undo', label: 'Undo', shortcutHint: 'Ctrl+Z', action: () => ran.push('undo') },
    { id: 'save', label: 'Save', shortcut: 'Ctrl+S', action: () => ran.push('save') },
    { id: 'plain', label: 'Plain', action: () => ran.push('plain') }
  ], 'edit', actions, ['w1', 'edit']);
  context.state.windowMenus.set('w1', { renderedMenus: [{ id: 'edit', items }], actions });

  const html = menu.renderWindowMenuItems(items);
  assert.match(html, /<span>Undo<\/span><kbd>Ctrl\+Z<\/kbd>/, 'a shortcutHint is drawn in the key column');
  assert.match(html, /<span>Save<\/span><kbd>Ctrl\+S<\/kbd>/, 'a shortcut is drawn in the key column');
  assert.match(html, /<span>Plain<\/span><kbd><\/kbd>/, 'an item without keys keeps an empty key column');
  assert.equal(routing.includes("contextMenuShortcutMarkup(item.shortcut || item.shortcutHint || '')"), true, 'context menus draw a shortcutHint too');
  assert.equal(menu.contextMenuShortcutMarkup('Ctrl+Z'), '<kbd class="vd-context-shortcut">Ctrl+Z</kbd>');

  const keydown = (key, extra) => Object.assign({
    key, code: 'Key' + key.toUpperCase(), ctrlKey: false, metaKey: false, altKey: false, shiftKey: false,
    defaultPrevented: false, target: { closest: () => null },
    preventDefault() { this.defaultPrevented = true; }, stopPropagation() {}
  }, extra);
  const undo = keydown('z', { ctrlKey: true });
  assert.equal(menu.handleWindowMenuShortcut(undo), false, 'the hinted key is left to the app');
  assert.equal(undo.defaultPrevented, false, 'the hinted key is not prevented');
  assert.deepEqual(ran, [], 'the hinted item does not run on its key');
  const save = keydown('s', { ctrlKey: true });
  assert.equal(menu.handleWindowMenuShortcut(save), true, 'a real shortcut is still dispatched');
  menu.runWindowMenuAction('w1', items[0].actionKey);
  assert.deepEqual(ran, ['save', 'undo'], 'a real shortcut runs its item, and a click runs the hinted item');
}

// easyDragIds builds flow and run ids of the server's shape (flows.NewFlowID / NewRunID: a prefix
// and 10 or 12 characters of a-z2-7); offset walks every character through every position.
function easyDragIds(offset) {
  const alphabet = 'abcdefghijklmnopqrstuvwxyz234567';
  const suffix = n => Array.from({ length: n }, (_, i) => alphabet[(offset + i * 7) % alphabet.length]).join('');
  return { flow: 'flow_' + suffix(10), run: 'run_' + suffix(12) };
}

// A clicked EasyDrag notification opens its flow and run; the shell keeps only ids of the server's
// shape, and an invalid run id alone still opens the flow.
function testEasyDragNotificationContextKeepsOnlyServerIds() {
  const source = sourceBetween(read('ui/js/desktop/core/shell-chrome-runtime.js'), 'function notificationContext(', 'function pushNotificationRecord(');
  const context = {};
  vm.createContext(context);
  vm.runInContext(`${source}; globalThis.notificationContext = notificationContext;`, context);
  // JSON copies the vm realm's objects into this one (deepEqual compares prototypes).
  const keep = (appId, ctx) => JSON.parse(JSON.stringify(context.notificationContext({ appId, context: ctx }) ?? null));

  for (let offset = 0; offset < 32; offset++) {
    const { flow, run } = easyDragIds(offset);
    assert.deepEqual(keep('easydrag', { flow_id: flow, run_id: run }), { flow_id: flow, run_id: run }, `server ids ${flow} ${run}`);
  }
  const { flow, run } = easyDragIds(3);
  assert.deepEqual(keep('easydrag', { flow_id: flow }), { flow_id: flow }, 'a flow without a run');
  assert.deepEqual(keep('easydrag', { flow_id: flow, run_id: run, extra: 'x', path: '/etc' }), { flow_id: flow, run_id: run }, 'other keys are dropped');
  for (const badRun of ['run_short', run + 'a', run.toUpperCase(), 'run_' + 'a'.repeat(11) + '_', 42, { id: run }]) {
    assert.deepEqual(keep('easydrag', { flow_id: flow, run_id: badRun }), { flow_id: flow }, `an invalid run id ${JSON.stringify(badRun)} keeps the flow`);
  }
  for (const badFlow of ['flow_abc', flow + 'a', flow.toUpperCase(), 'flow_ABCDEFGHIJ', 'flw_' + 'a'.repeat(10), '../flow_aaaaaaaaaa', '', 7, null]) {
    assert.equal(keep('easydrag', { flow_id: badFlow, run_id: run }), null, `an invalid flow id ${JSON.stringify(badFlow)} opens nothing`);
  }
  assert.equal(keep('easydrag', { run_id: run }), null, 'a run without a flow opens nothing');
  assert.equal(keep('easydrag', undefined), null, 'no context');
  assert.equal(keep('mission-control', { flow_id: flow, run_id: run }), null, 'another app keeps no flow ids');
  const conversation = 'ab'.repeat(32);
  assert.deepEqual(keep('meshcore', { conversation_id: conversation, flow_id: flow }), { conversation_id: conversation }, 'MeshCore keeps its conversation only');
}

// A person detection toast keeps its event (and a well-formed camera id) as launch context,
// so a click on the notification can open the event in Network Cameras.
function testNetworkCamerasNotificationContextKeepsEventIds() {
  const source = sourceBetween(read('ui/js/desktop/core/shell-chrome-runtime.js'), 'function notificationContext(', 'function pushNotificationRecord(');
  const context = {};
  vm.createContext(context);
  vm.runInContext(`${source}; globalThis.notificationContext = notificationContext;`, context);
  const keep = (appId, ctx) => JSON.parse(JSON.stringify(context.notificationContext({ appId, context: ctx }) ?? null));
  assert.deepEqual(keep('network-cameras', { event_id: 42, camera_id: 'garden' }), { event_id: 42, camera_id: 'garden' });
  assert.deepEqual(keep('network-cameras', { event_id: 42, camera_id: '../x' }), { event_id: 42 }, 'an odd camera id is dropped');
  assert.deepEqual(keep('network-cameras', { event_id: 42, camera_id: 7 }), { event_id: 42 }, 'a camera id must be a string');
  assert.deepEqual(keep('network-cameras', { event_id: 42, path: '/etc' }), { event_id: 42 }, 'other keys are dropped');
  for (const bad of [0, -1, 1.5, '42', null, Number.MAX_SAFE_INTEGER + 1]) {
    assert.equal(keep('network-cameras', { event_id: bad, camera_id: 'garden' }), null, `event id ${String(bad)} opens nothing`);
  }
  assert.equal(keep('easydrag', { event_id: 42 }), null, 'another app keeps no event ids');
}

// The session keeps an EasyDrag window's flow: a valid flowId survives save and restore; any
// other value is neither saved nor restored, so the window opens the start page.
async function testEasyDragSessionKeepsOnlyValidFlowIds() {
  const source = read('ui/js/desktop/core/session-runtime.js');
  const helperSource = [
    sourceBetween(source, 'const SESSION_SKIP_APP_IDS', 'let sessionPersistTimer'),
    sourceBetween(source, 'function sanitizeSessionContext(', 'function scheduleSessionPersist('),
    sourceBetween(source, 'function parseSessionSnapshot(', 'function parseDefaultAppsMap(')
  ].join('\n');
  const { flow, run } = easyDragIds(5);
  const settings = {};
  const opened = [];
  const windowOf = (appId, z, ctx) => ({ appId, element: { style: { left: '10px', top: '20px', width: '900px', height: '600px', zIndex: String(z), display: '' } }, context: ctx });
  const context = {
    state: {
      activeSpaceId: 0,
      windows: new Map([
        ['w1', windowOf('easydrag', 1, { flowId: flow, flow_id: flow, run_id: run })],
        ['w2', windowOf('easydrag', 2, { flowId: '..' })],
        ['w3', windowOf('easydrag', 3, { flowId: flow.toUpperCase() })],
        ['w4', windowOf('easydrag', 4, { flowId: 42 })],
        ['w5', windowOf('easydrag', 5, { flowId: null })],
        ['w6', windowOf('files', 6, { path: 'Documents' })]
      ])
    },
    window: { setTimeout: fn => fn() },
    windowSpaceId: () => 0,
    normalizeSpaceId: id => id,
    settingValue: key => settings[key],
    sessionRestoreEnabled: () => true,
    restoreActiveSpaceFromSnapshot() {},
    renderSpacePager() {},
    appById: id => ({ id }),
    openApp: (appId, ctx) => opened.push([appId, ctx.flowId === undefined ? '(none)' : ctx.flowId]),
    applySpaceVisibility() {},
    taskbarWindows: () => [],
    focusWindow() {},
    scheduleSessionPersist() {}
  };
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.capture = captureSessionSnapshot; globalThis.restore = restoreDesktopSession;`, context);

  const snapshot = JSON.parse(JSON.stringify(context.capture()));
  assert.deepEqual(snapshot.windows.map(w => [w.appId, w.context]), [
    ['easydrag', { flowId: flow }], ['easydrag', {}], ['easydrag', {}], ['easydrag', {}], ['easydrag', {}], ['files', { path: 'Documents' }]
  ], 'only a valid flowId is saved, and a notification\'s flow_id/run_id are not');

  // A stored session written before the check (or by hand) is checked again on restore.
  snapshot.windows[1].context = { flowId: '..' };
  snapshot.windows[2].context = { flowId: 'flow_' + 'a'.repeat(11) };
  settings['session.windows'] = JSON.stringify(snapshot);
  await context.restore();
  assert.deepEqual(opened, [
    ['easydrag', flow], ['easydrag', '(none)'], ['easydrag', '(none)'], ['easydrag', '(none)'], ['easydrag', '(none)'], ['files', '(none)']
  ], 'only a valid flowId is restored');
}

// flows_changed from the desktop event stream reaches EasyDrag as the DOM event aurago:flows-changed.
async function testEasyDragFlowsChangedIsForwarded() {
  const source = sourceBetween(read('ui/js/desktop/core/sdk-events-bootstrap.js'), 'async function handleDesktopEvent(', 'function showDesktopNotification(');
  const dispatched = [];
  const context = {
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } },
    document: { dispatchEvent: event => { dispatched.push([event.type, JSON.parse(JSON.stringify(event.detail))]); return true; } }
  };
  vm.createContext(context);
  vm.runInContext(`${source}; globalThis.handle = handleDesktopEvent;`, context);
  await context.handle({ type: 'flows_changed', payload: { flow_id: 'flow_aaaaaaaaaa', reason: 'saved' } });
  await context.handle({ type: 'flows_changed' });
  assert.deepEqual(dispatched, [
    ['aurago:flows-changed', { flow_id: 'flow_aaaaaaaaaa', reason: 'saved' }],
    ['aurago:flows-changed', {}]
  ]);
}

// broadcastDesktopEvent delivers a server event over the desktop WebSocket and SSE with one id:
// the shell handles it once (one toast, one bell count), while events without an id still all run.
async function testDesktopEventDeliveredTwiceIsHandledOnce() {
  const source = sourceBetween(read('ui/js/desktop/core/sdk-events-bootstrap.js'), 'async function handleDesktopEvent(', 'function showDesktopNotification(');
  const shown = [];
  const context = { showDesktopNotification: payload => shown.push(payload.message) };
  vm.createContext(context);
  vm.runInContext(`${source}; globalThis.handle = handleDesktopEvent;`, context);
  const event = { id: 'evt-1', type: 'notification', payload: { title: 'Person', message: 'Front door' } };
  await context.handle(event); // WebSocket copy
  await context.handle(JSON.parse(JSON.stringify(event))); // SSE copy
  await context.handle({ id: 'evt-2', type: 'notification', payload: { message: 'Garage' } });
  await context.handle({ type: 'notification', payload: { message: 'agent tool' } });
  await context.handle({ type: 'notification', payload: { message: 'agent tool' } });
  assert.deepEqual(shown, ['Front door', 'Garage', 'agent tool', 'agent tool']);
}

// Loads cfg/flows.js with the real config helpers it uses: escaping (utils.js), the provider choice helpers
// and toggleBool (main.js), the validation rules (catalog.js) and the field metadata (lang/meta.json).
function configFlowsContext({ flows, tools = {}, providers = [], loaded = true }) {
  const strings = { ...JSON.parse(read('ui/lang/config/common/en.json')), ...JSON.parse(read('ui/lang/config/flows/en.json')) };
  const meta = JSON.parse(read('ui/lang/meta.json'));
  const main = read('ui/js/config/main.js');
  const context = {
    window: {},
    content: { innerHTML: '' },
    configData: { flows, tools },
    providersCache: providers,
    providersLoaded: loaded,
    personalitiesCache: [],
    personalitiesLoaded: true,
    helpTexts: { 'flows.ai_provider': meta['help.flows.ai_provider'] },
    dirtyMarks: 0,
    markDirty() { context.dirtyMarks += 1; },
    t(key, params) {
      assert.ok(Object.prototype.hasOwnProperty.call(strings, key), `missing English string ${key}`);
      return strings[key].replace(/\{\{(\w+)\}\}/g, (_, name) => params[name]);
    },
    document: { getElementById: id => (id === 'content' ? context.content : null), querySelectorAll: () => [] },
    attachChangeListeners() {}
  };
  vm.createContext(context);
  vm.runInContext(read('ui/js/config/catalog.js'), context);
  vm.runInContext(sourceBetween(read('ui/js/config/utils.js'), 'function escapeAttr(', 'function formatKey('), context);
  vm.runInContext(sourceBetween(main, 'function cfgChoiceSource(', 'async function retryConfigChoiceLists('), context);
  vm.runInContext(sourceBetween(main, 'function syncToggleA11y(', 'function enhanceConfigControls('), context);
  vm.runInContext(sourceBetween(main, 'function toggleBool(', 'function togglePassword('), context);
  vm.runInContext(read('ui/cfg/flows.js'), context);
  return context;
}

const configFlowsSectionMeta = { label: 'EasyDrag flows', desc: 'Visual automations' };

function renderConfigFlowsSection(options) {
  const context = configFlowsContext(options);
  context.renderFlowsSection(configFlowsSectionMeta);
  return context.content.innerHTML;
}

function configStateForTest(rules) {
  const context = { window: {}, document: { dispatchEvent() {} }, CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } } };
  vm.createContext(context);
  vm.runInContext(read('ui/js/config/state.js'), context);
  const state = context.window.AuraConfigState;
  if (rules) state.setRules(rules);
  return state;
}

// A minimal element for a rendered .toggle, enough for toggleBool, syncToggleA11y and state.js.
function fakeConfigToggle(markup) {
  const attributes = Object.fromEntries([...markup.matchAll(/([\w-]+)="([^"]*)"/g)].map(match => [match[1], match[2]]));
  const classes = new Set(attributes.class.split(/\s+/));
  return {
    dataset: { path: attributes['data-path'] },
    nextElementSibling: null,
    classList: {
      contains: name => classes.has(name),
      toggle(name) { if (classes.has(name)) { classes.delete(name); return false; } classes.add(name); return true; }
    },
    getAttribute: name => (Object.prototype.hasOwnProperty.call(attributes, name) ? attributes[name] : null),
    setAttribute(name, value) { attributes[name] = String(value); },
    removeAttribute(name) { delete attributes[name]; }
  };
}

function decodeConfigAttr(value) {
  return value.replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&');
}

// #content as data-path controls that keep typed values until the next render,
// like the browser DOM that AuraConfigState reads.
function fakeConfigContent() {
  const content = { controls: [] };
  let html = '';
  Object.defineProperty(content, 'innerHTML', {
    get: () => html,
    set(value) {
      html = value;
      content.controls = [...value.matchAll(/<(?:input|div)\b[^>]*\bdata-path="[^"]*"[^>]*>/g)].map(([tag]) => {
        const attributes = Object.fromEntries([...tag.matchAll(/([\w-]+)="([^"]*)"/g)].map(match => [match[1], decodeConfigAttr(match[2])]));
        const classes = new Set((attributes.class || '').split(/\s+/));
        return {
          type: attributes.type || '',
          step: '',
          value: attributes.value || '',
          dataset: { path: attributes['data-path'] },
          classList: { contains: name => classes.has(name) }
        };
      });
    }
  });
  content.control = dataPath => content.controls.find(control => control.dataset.path === dataPath);
  content.querySelectorAll = () => content.controls;
  return content;
}

// An element inside #content (such as a list container) whose innerHTML replaces only its own markup.
// Controls outside it keep their typed values, like the browser DOM.
function fakeConfigRegion(content, id) {
  const bounds = () => {
    const html = content.innerHTML;
    const open = html.indexOf(`<div id="${id}"`);
    if (open < 0) return null;
    const start = html.indexOf('>', open) + 1;
    const tags = /<div\b|<\/div>/g;
    tags.lastIndex = start;
    let depth = 1;
    for (let match = tags.exec(html); match; match = tags.exec(html)) {
      depth += match[0] === '</div>' ? -1 : 1;
      if (depth === 0) return { html, start, end: match.index };
    }
    throw new Error(`#${id} is not closed`);
  };
  return {
    get innerHTML() {
      const region = bounds();
      return region ? region.html.slice(region.start, region.end) : '';
    },
    set innerHTML(inner) {
      const region = bounds();
      assert.ok(region, `#${id} is not rendered`);
      const typed = new Map(content.controls.map(control => [control.dataset.path, control.value]));
      const replaced = new Set([...inner.matchAll(/data-path="([^"]*)"/g)].map(match => decodeConfigAttr(match[1])));
      content.innerHTML = region.html.slice(0, region.start) + inner + region.html.slice(region.end);
      content.controls.forEach(control => {
        const dataPath = control.dataset.path;
        if (!replaced.has(dataPath) && typed.has(dataPath)) control.value = typed.get(dataPath);
      });
    }
  };
}

// A Config page with one section module. Like main.js, every draft change and every setDirty(true) reads
// the rendered controls back into the draft, so stale controls must never outlive the list they show.
function configSectionContext(configData, modules, { regions = [], globals = {} } = {}) {
  const content = fakeConfigContent();
  const elements = new Map(regions.map(id => [id, fakeConfigRegion(content, id)]));
  const context = {
    window: {},
    content,
    configData: JSON.parse(JSON.stringify(configData)),
    providersCache: [],
    isDirty: false,
    t: key => key,
    document: { getElementById: id => (id === 'content' ? content : (elements.get(id) || null)), dispatchEvent() {} },
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } },
    attachChangeListeners() {},
    setDirty(dirty) {
      if (dirty) context.window.AuraConfigState.syncFromDOM(content);
      context.isDirty = context.window.AuraConfigState.isDirty();
    },
    ...globals
  };
  vm.createContext(context);
  vm.runInContext(read('ui/js/config/state.js'), context);
  vm.runInContext(sourceBetween(read('ui/js/config/utils.js'), 'function escapeAttr(', 'function formatKey('), context);
  // Provider dropdown helpers that section modules (A2A) call while rendering.
  vm.runInContext(sourceBetween(read('ui/js/config/main.js'), 'function configProviderIsDecisionsOnly(', '/** Fixed and loaded options'), context);
  modules.forEach(module => vm.runInContext(read(module), context));
  const state = context.window.AuraConfigState;
  state.init(context.configData);
  state.subscribe(snapshot => context.setDirty(snapshot.dirty));
  return context;
}

function configGo2RTCContext(go2rtc) {
  return configSectionContext({ go2rtc }, ['ui/cfg/go2rtc.js']);
}

// Reads the complete lists a save would send; any other patched key fails the check.
function configListPatch(state, expectedPaths, message) {
  const plain = value => JSON.parse(JSON.stringify(value));
  const patch = plain(state.buildPatch());
  assert.deepEqual(plain(state.dirtyPaths()), [...expectedPaths].sort(), message + ': each list counts as one change ' + JSON.stringify(plain(state.dirtyPaths())));
  return expectedPaths.map(dataPath => dataPath.split('.').reduce((value, key) => value[key], patch));
}

function typeConfigControl(context, dataPath, value) {
  const control = context.content.control(dataPath);
  assert.ok(control, `missing control ${dataPath}`);
  control.value = value;
  context.window.AuraConfigState.syncFromDOM(context.content);
}

// Config section "A2A": skills and remote agents save as complete lists. Fields the rows do not show (skill
// tags, the masked agent credentials the server reports) survive, adding or removing an entry keeps typed
// edits and never shifts one entry's fields onto another, and other typed fields outside the lists stay.
function testConfigA2AListsSaveAsCompleteLists() {
  const skill = (id, name) => ({ id, name, description: name + ' help', tags: [id, 'aurago'] });
  const agent = (id, name) => ({ id, name, card_url: `https://${id}.example/.well-known/agent-card.json`, enabled: true, api_key: '••••••••', bearer_token: '••••••••' });
  const context = configSectionContext({
    providers: [],
    a2a: {
      server: {
        enabled: true, agent_name: 'Aura', agent_description: '', agent_version: '1.0.0', agent_url: '', port: 8091, base_path: '/a2a',
        skills: [skill('search', 'Search'), skill('notes', 'Notes'), skill('lights', 'Lights')]
      },
      client: { enabled: true, remote_agents: [agent('alpha', 'Alpha'), agent('beta', 'Beta'), agent('gamma', 'Gamma')] },
      auth: {},
      llm: {}
    }
  }, ['ui/cfg/a2a.js'], {
    regions: ['a2a-skills-list', 'a2a-remote-agents'],
    globals: {
      AuraConfigForm: { toggle: () => '', number: () => '' },
      cfgSecretPlaceholder: (value, fallback) => fallback,
      cfgSecretValue: () => '',
      EYE_OPEN_SVG: ''
    }
  });
  const state = context.window.AuraConfigState;
  const skills = 'a2a.server.skills';
  const agents = 'a2a.client.remote_agents';

  context.renderA2ASection({ label: 'A2A', desc: 'Agents' });
  assert.equal(state.isDirty(), false);

  typeConfigControl(context, 'a2a.server.skills.1.name', 'Notebook');
  const edited = [skill('search', 'Search'), { ...skill('notes', 'Notes'), name: 'Notebook' }, skill('lights', 'Lights')];
  assert.deepEqual(configListPatch(state, [skills], 'a skill edit'), [edited], 'a skill edit sends every skill with its tags');

  typeConfigControl(context, 'a2a.server.agent_name', 'Aura 2');
  context.a2aAddSkill();
  state.syncFromDOM(context.content); // saveConfig reads the controls again
  const addedSkill = { id: '', name: '', description: '', tags: [] };
  assert.equal(context.content.control('a2a.server.skills.1.name').value, 'Notebook', 'the re-rendered rows show the typed edit');
  assert.equal(context.content.control('a2a.server.agent_name').value, 'Aura 2', 'adding a skill keeps typed fields outside the list');
  assert.equal(state.get('a2a.server.agent_name'), 'Aura 2');
  assert.deepEqual(configListPatch(state, ['a2a.server.agent_name', skills], 'an added skill')[1], [...edited, addedSkill]);

  context.a2aRemoveSkill(0);
  state.syncFromDOM(context.content);
  assert.deepEqual(configListPatch(state, ['a2a.server.agent_name', skills], 'a removed skill')[1], [edited[1], edited[2], addedSkill],
    'a removed skill does not shift fields onto its neighbour');
  [2, 1, 0].forEach(index => context.a2aRemoveSkill(index));
  state.syncFromDOM(context.content);
  assert.deepEqual(configListPatch(state, ['a2a.server.agent_name', skills], 'no skills')[1], [], 'removing every skill sends an empty list');

  typeConfigControl(context, 'a2a.client.remote_agents.2.card_url', 'https://gamma.example/card.json');
  const gamma = { ...agent('gamma', 'Gamma'), card_url: 'https://gamma.example/card.json' };
  context.a2aRemoveRemoteAgent(0);
  state.syncFromDOM(context.content);
  assert.deepEqual(configListPatch(state, ['a2a.server.agent_name', agents, skills], 'a removed agent')[1], [agent('beta', 'Beta'), gamma],
    'a removed agent keeps the others whole');
  context.a2aAddRemoteAgent();
  state.syncFromDOM(context.content);
  assert.equal(context.content.control('a2a.client.remote_agents.1.card_url').value, 'https://gamma.example/card.json');
  assert.deepEqual(configListPatch(state, ['a2a.server.agent_name', agents, skills], 'an added agent')[1],
    [agent('beta', 'Beta'), gamma, { id: '', name: '', card_url: '', enabled: true }]);
}

// Config section "3D printers": both printer lists save as complete lists (with the masked Klipper key and
// fields like webcam_name), each list only when it changed, and adding or removing a printer re-renders
// before the controls are read back, so typed edits survive and nothing shifts between printers.
function testConfigThreeDPrinterListsSaveAsCompleteLists() {
  const elegoo = (id, name) => ({ id, name, url: `ws://${id}.local/websocket`, mainboard_id: id + '-board', timeout_seconds: 10 });
  const klipper = (id, name) => ({ id, name, url: `http://${id}.local:7125`, api_key: '••••••••', timeout_seconds: 15, webcam_name: id + '-cam' });
  const context = configSectionContext({
    three_d_printers: {
      enabled: true, readonly: true, default_printer: 'voron',
      elegoo_centauri_carbon: { enabled: true, printers: [elegoo('lab', 'Lab'), elegoo('shop', 'Shop')] },
      klipper: { enabled: true, printers: [klipper('voron', 'Voron'), klipper('ender', 'Ender'), klipper('prusa', 'Prusa')] }
    }
  }, ['ui/cfg/three_d_printers.js'], {
    globals: { SECTIONS: [{ items: [{ key: 'three_d_printers', label: '3D printers', desc: 'Printers' }] }] }
  });
  const state = context.window.AuraConfigState;
  const elegooPath = 'three_d_printers.elegoo_centauri_carbon.printers';
  const klipperPath = 'three_d_printers.klipper.printers';

  context.renderThreeDPrintersSection(null);
  assert.equal(state.isDirty(), false);

  typeConfigControl(context, klipperPath + '.1.name', 'Ender 3');
  const ender = { ...klipper('ender', 'Ender'), name: 'Ender 3' };
  assert.deepEqual(configListPatch(state, [klipperPath], 'a printer edit'), [[klipper('voron', 'Voron'), ender, klipper('prusa', 'Prusa')]],
    'a printer edit sends every Klipper printer and leaves the Elegoo list alone');

  context.threeDPrinterRemove('klipper', 0);
  state.syncFromDOM(context.content);
  assert.equal(context.content.control(klipperPath + '.0.name').value, 'Ender 3', 'the re-render shows the typed edit');
  assert.deepEqual(configListPatch(state, [klipperPath], 'a removed printer'), [[ender, klipper('prusa', 'Prusa')]],
    'a removed printer does not shift fields onto its neighbour');

  context.threeDPrinterAdd('elegoo');
  state.syncFromDOM(context.content);
  const added = { id: 'printer-3', name: 'Elegoo Centauri Carbon', url: 'ws://192.168.6.50/websocket', timeout_seconds: 10, mainboard_id: '' };
  assert.deepEqual(configListPatch(state, [elegooPath, klipperPath], 'an added printer'),
    [[elegoo('lab', 'Lab'), elegoo('shop', 'Shop'), added], [ender, klipper('prusa', 'Prusa')]]);

  context.threeDPrinterRemove('klipper', 1);
  context.threeDPrinterRemove('klipper', 0);
  state.syncFromDOM(context.content);
  assert.deepEqual(configListPatch(state, [elegooPath, klipperPath], 'no Klipper printers')[1], [], 'removing every printer sends an empty list');
}

// Config section "go2rtc": the server replaces go2rtc.streams wholesale, so a save sends the complete list,
// including fields the stream cards do not show (detection, source_configured). Adding or removing a camera
// keeps typed edits and never shifts one camera's fields onto another.
function testConfigGo2RTCStreamsSaveAsCompleteList() {
  const plain = value => JSON.parse(JSON.stringify(value));
  const stream = (id, name) => ({
    id, name, enabled: true, source: '••••••••', source_configured: true,
    detection: { enabled: true, zones: [{ name: id + '-zone', points: [[0, 0], [1, 0], [1, 1]] }] }
  });
  const context = configGo2RTCContext({
    enabled: false, auto_start: true, agent_access: true, store_media: true, web_ui_enabled: false,
    image: 'alexxit/go2rtc:test', container_name: 'aurago_go2rtc', api_host_port: 1984, webrtc: { enabled: false, port: 8555 },
    streams: [stream('front-door', 'Front door'), stream('garage', 'Garage'), stream('backyard', 'Backyard')]
  });
  const state = context.window.AuraConfigState;
  const patchStreams = message => {
    const patch = plain(state.buildPatch());
    assert.deepEqual(Object.keys(patch), ['go2rtc'], message);
    assert.deepEqual(Object.keys(patch.go2rtc), ['streams'], message);
    return patch.go2rtc.streams;
  };
  const typeName = (index, value) => {
    context.content.control(`go2rtc.streams.${index}.name`).value = value;
    state.syncFromDOM(context.content);
  };

  context.renderGo2RTCSection({ label: 'go2rtc', desc: 'Cameras' });
  assert.equal(state.isDirty(), false);

  typeName(1, 'Garage side');
  const edited = [stream('front-door', 'Front door'), stream('garage', 'Garage side'), stream('backyard', 'Backyard')];
  assert.deepEqual(patchStreams('a field edit'), edited, 'a field edit sends the complete list');
  assert.deepEqual(plain(state.dirtyPaths()), ['go2rtc.streams'], 'the list counts as one change');
  typeName(1, 'Garage');
  assert.equal(state.isDirty(), false, 'typing the saved value back clears the change');
  typeName(1, 'Garage side');

  context.go2rtcAddStream();
  state.syncFromDOM(context.content); // saveConfig reads the controls again
  const added = { id: 'camera-4', name: 'config.go2rtc.stream_name_placeholder', enabled: true, source: '', source_configured: false };
  assert.equal(context.content.control('go2rtc.streams.1.name').value, 'Garage side', 'the re-render shows the typed edit');
  assert.deepEqual(patchStreams('an added camera'), [...edited, added], 'an added camera keeps the typed edit and the other cameras');

  context.go2rtcRemoveStream(0);
  state.syncFromDOM(context.content);
  assert.deepEqual(patchStreams('a removed camera'), [edited[1], edited[2], added], 'a removed camera does not shift fields onto its neighbour');

  [2, 1, 0].forEach(index => context.go2rtcRemoveStream(index));
  state.syncFromDOM(context.content);
  assert.deepEqual(patchStreams('no cameras'), [], 'removing every camera sends an empty list');
}

// Config section "EasyDrag flows": the provider picker uses the shared choice helpers (unknown ids kept,
// hint and retry on a failed list), the limits are validated numbers, the link opens EasyDrag, and the
// agent options (not enforced yet) cannot be switched.
function testConfigFlowsSection() {
  const providers = [
    { id: 'main', name: 'Main', type: 'openai', model: 'gpt-x' },
    { id: 'p<2>', name: 'Odd "name"', type: '', model: '' }
  ];
  const flows = { enabled: true, max_parallel_runs: 12, max_parallel_nodes_per_run: 3, run_retention_days: 60, max_runs_per_flow: 500, ai_provider: 'gone', agent: { read_only: true, allow_publish: false } };
  const html = renderConfigFlowsSection({ flows, providers });
  assert.match(html, /<select class="field-select" data-path="flows\.ai_provider" data-config-choice="providers"[^>]*><option value="gone" selected data-config-choice-preserved>gone \(missing\)<\/option><option value="">Main model<\/option><option value="main">Main \[openai\] — gpt-x<\/option><option value="p&lt;2&gt;">Odd &quot;name&quot;<\/option><\/select><\/div>/);
  assert.match(renderConfigFlowsSection({ flows: { ...flows, ai_provider: 'main' }, providers }), /<option value="">Main model<\/option><option value="main" selected>/);
  assert.match(renderConfigFlowsSection({ flows: { ...flows, ai_provider: '' }, providers }), /<option value="" selected>Main model<\/option><option value="main">/);
  const shared = configFlowsContext({ flows, providers });
  assert.match(shared.cfgChoiceOptionsHTML('providers', { provider_ref: true }, ''), /^<option value="" selected>— no provider —<\/option><option value="main">/, 'other provider fields keep their empty label');

  const failed = configFlowsContext({ flows, providers: [], loaded: false });
  failed.renderFlowsSection(configFlowsSectionMeta);
  assert.match(failed.content.innerHTML, /<select class="field-select" data-path="flows\.ai_provider" data-config-choice="providers"[^>]*><option value="gone" selected data-config-choice-preserved>gone \(list unavailable\)<\/option><option value="">Main model<\/option><\/select><div class="field-help cfg-choice-unavailable" role="status" data-config-choice-hint="providers">[^<]+<button type="button" class="cfg-btn cfg-btn-sm" data-config-choice-retry="providers">[^<]+<\/button><\/div>/);
  const select = { value: 'gone', innerHTML: '', dataset: { path: 'flows.ai_provider' } };
  let hintsRemoved = 0;
  failed.document.querySelectorAll = selector => (selector === 'select[data-config-choice="providers"]' ? [select]
    : selector === '[data-config-choice-hint="providers"]' ? [{ remove() { hintsRemoved += 1; } }] : []);
  failed.providersCache = providers;
  failed.providersLoaded = true;
  failed.refreshConfigChoiceSelects('providers');
  assert.match(select.innerHTML, /^<option value="gone" selected data-config-choice-preserved>gone \(missing\)<\/option><option value="">Main model<\/option><option value="main">/, 'a retry rebuilds the select with the flows empty label');
  assert.equal(select.value, 'gone');
  assert.equal(hintsRemoved, 1);

  const numberInputs = [...html.matchAll(/<input class="field-input" type="number" min="(\d+)" max="(\d+)" value="(\d+)" data-path="(flows\.[a-z_]+)"/g)];
  assert.equal(JSON.stringify(numberInputs.map(match => [match[4], Number(match[1]), Number(match[2])])),
    JSON.stringify([['flows.max_parallel_runs', 1, 32], ['flows.max_parallel_nodes_per_run', 1, 16], ['flows.run_retention_days', 1, 365], ['flows.max_runs_per_flow', 10, 5000]]));
  const numberElements = values => numberInputs.map(([, , , value, dataPath]) => ({ type: 'number', step: '', value: values[dataPath] || value, dataset: { path: dataPath }, classList: { contains: () => false } }));
  const flowRules = Object.fromEntries(Object.entries(shared.window.AuraConfigCatalog.validationRules).filter(([path]) => path.startsWith('flows.')));
  let state = configStateForTest(flowRules);
  state.init({ flows });
  state.syncFromDOM({ querySelectorAll: () => numberElements({ 'flows.max_parallel_runs': '16', 'flows.run_retention_days': '90' }) });
  assert.equal(JSON.stringify(state.buildPatch()), JSON.stringify({ flows: { max_parallel_runs: 16, run_retention_days: 90 } }), 'number inputs reach the patch as numbers');
  assert.equal(state.validate().valid, true);
  state.syncFromDOM({ querySelectorAll: () => numberElements({ 'flows.max_parallel_runs': '40', 'flows.max_parallel_nodes_per_run': '0', 'flows.max_runs_per_flow': '9' }) });
  const validation = state.validate();
  assert.equal(validation.valid, false, 'an edit beyond the range blocks the save');
  assert.equal(JSON.stringify(Array.from(validation.errors, error => [error.path, error.code])),
    JSON.stringify([['flows.max_parallel_runs', 'max'], ['flows.max_parallel_nodes_per_run', 'min'], ['flows.max_runs_per_flow', 'min']]));
  for (const language of ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']) {
    const common = JSON.parse(read(`ui/lang/config/common/${language}.json`));
    for (const code of ['min', 'max']) assert.ok(common[`config.precision.validation_${code}`], `${language} translates validation_${code}`);
  }
  state = configStateForTest(flowRules);
  state.init({ flows: { ...flows, max_parallel_runs: 100, max_runs_per_flow: 5 } });
  assert.equal(state.validate().valid, true, 'a saved value beyond the range (the server clamps it) does not block other saves');
  state.syncFromDOM({ querySelectorAll: () => numberElements({ 'flows.max_parallel_runs': '100', 'flows.max_runs_per_flow': '5' }) });
  assert.equal(state.validate().valid, true, 'an unchanged saved value stays out of the patch and the check');

  assert.match(html, /<a class="btn-save dc-test-btn" href="\/desktop\?app=easydrag">Open EasyDrag<\/a>/);
  assert.doesNotMatch(renderConfigFlowsSection({ flows: { ...flows, enabled: false }, providers }), /href="\/desktop/, 'no link while flows are off');
  const missionsOff = renderConfigFlowsSection({ flows, providers, tools: { missions: { enabled: false } } });
  assert.match(missionsOff, /cfg-note-banner-warning">Missions are switched off\./);
  assert.doesNotMatch(missionsOff, /href="\/desktop/, 'no link while missions are off');

  const toggle = dataPath => (html.match(new RegExp(`<div class="toggle[^"]*" data-path="${dataPath.replace(/\./g, '\\.')}"[^>]*>`)) || [''])[0];
  assert.match(toggle('flows.enabled'), /onclick="toggleBool\(this\)"/);
  for (const dataPath of ['flows.agent.read_only', 'flows.agent.allow_publish']) {
    const markup = toggle(dataPath);
    assert.match(markup, /cfg-toggle-disabled/, `${dataPath} looks disabled`);
    assert.match(markup, /aria-disabled="true"/, `${dataPath} is announced as disabled`);
    assert.match(markup, /aria-describedby="flows-agent-note"/);
    assert.doesNotMatch(markup, /onclick/, `${dataPath} has no click handler`);
    const context = configFlowsContext({ flows, providers });
    const element = fakeConfigToggle(markup);
    const wasOn = element.classList.contains('on');
    context.toggleBool(element);
    assert.equal(element.classList.contains('on'), wasOn, `toggleBool leaves ${dataPath} unchanged`);
    assert.equal(context.dirtyMarks, 0, `toggleBool does not mark ${dataPath} dirty`);
    state = configStateForTest();
    state.init({ flows });
    state.syncFromDOM({ querySelectorAll: () => [element] });
    assert.equal(state.isDirty(), false, `${dataPath} leaves the draft clean`);
  }
  const control = configFlowsContext({ flows, providers });
  const enabled = fakeConfigToggle(toggle('flows.enabled'));
  control.toggleBool(enabled);
  assert.equal(enabled.classList.contains('on'), false, 'the unlocked switch does change');
  assert.equal(control.dirtyMarks, 1);
  state = configStateForTest();
  state.init({ flows });
  state.syncFromDOM({ querySelectorAll: () => [enabled] });
  assert.equal(JSON.stringify(state.dirtyPaths()), JSON.stringify(['flows.enabled']));
  assert.match(toggle('flows.agent.read_only'), /class="toggle on cfg-toggle-disabled"/, 'the saved value stays visible');
  assert.match(html, /id="flows-agent-note">The agent options take effect once the agent can work with flows\.</);
}

async function testQuickConnectSFTPMutationsBindDevice() {
  const source = read('ui/js/desktop/apps/quickconnect-launchpad-chat.js');
  const actions = sourceBetween(source, '        async function sftpUploadFiles(', '\n    function setQuickConnectMenus(');
  const requests = [], errors = [], refreshes = [];
  const context = {
    FormData, Blob, encodeURIComponent,
    t: key => key,
    showConfirmModal: async () => true,
    promptDialog: async () => 'renamed',
    joinSFTPPath: (dir, name) => `${dir}/${name}`,
    loadSFTPList: (_nav, device) => refreshes.push(device),
    showNotify: message => errors.push(message),
    api: async (url, options) => { requests.push({ url, body: JSON.parse(options.body) }); },
    fetch: async (url, options) => {
      requests.push({ url, body: { device_id: options.body.get('device_id'), path: options.body.get('remote_path') } });
      return { ok: true };
    }
  };
  // The functions are nested in renderQuickConnect; remove that enclosing close.
  vm.runInNewContext(actions.slice(0, actions.lastIndexOf('    }')), context);
  const device = 'device & secondary/ä', nav = { path: 'home' }, els = {};
  await context.sftpDelete(nav, device, 'home/file', 'file', els);
  await context.sftpRename(nav, device, 'home/file', els);
  await context.sftpMkdir(nav, device, 'home', els);
  await context.sftpCopy(nav, device, 'home/file', els);
  await context.sftpMove(nav, device, 'home/file', els);
  const file = new Blob(['upload']);
  Object.defineProperty(file, 'name', { value: 'file.txt' });
  await context.sftpUploadFiles(nav, device, 'home', [file], els);
  assert.deepEqual(errors, []);
  assert.equal(requests.length, 6);
  for (const request of requests) {
    assert.equal(new URL(request.url, 'http://desktop.test').searchParams.get('device_id'), device);
    assert.equal(request.body.device_id, device);
  }
  assert.equal(refreshes.length, 6, 'successful actions refresh their SFTP listing');
}

async function testInvasionNestFormSendsExportNestSecret() {
  const source = read('ui/js/invasion/main.js');
  const saveSource = sourceBetween(source, 'async function saveNest()', 'async function saveEgg()');
  const elements = new Map();
  for (const id of ['nest-id', 'nest-deploy-method', 'nest-name', 'nest-notes', 'nest-access-type', 'nest-host', 'nest-port',
    'nest-username', 'nest-secret', 'nest-active', 'nest-egg-id', 'nest-target-arch', 'nest-route', 'nest-route-config',
    'nest-docker-tls', 'nest-docker-tls-ca', 'nest-docker-tls-cert', 'nest-docker-tls-key', 'nest-export-secret']) {
    elements.set(id, { value: '', checked: false, focus() {} });
  }
  elements.get('nest-id').value = 'n1';
  elements.get('nest-name').value = 'Nest';
  elements.get('nest-deploy-method').value = 'ssh';
  const calls = [];
  const context = {
    document: { getElementById: id => elements.get(id) || null },
    api: async (path, options) => { calls.push(JSON.parse(options.body)); return {}; },
    closeModal() {}, showToast() {}, loadNests: async () => {}, t: key => key
  };
  vm.createContext(context);
  vm.runInContext(`${saveSource}; globalThis.saveNestForTest = saveNest;`, context);
  elements.get('nest-export-secret').checked = true;
  await context.saveNestForTest();
  elements.get('nest-export-secret').checked = false;
  await context.saveNestForTest();
  assert.deepEqual(calls.map(body => body.export_nest_secret), [true, false]);
  assert.match(source, /setChk\('nest-export-secret', isEdit && nest\?\.export_nest_secret === true\)/);
}

function testInvasionNestSecretFieldHidesOnlyWithoutEffect() {
  const source = read('ui/js/invasion/main.js');
  const helpers = sourceBetween(source, 'function setHiddenById(', 'function onDeployMethodChange()') +
    sourceBetween(source, 'function updateNestSecretField()', '// User changed the TLS mode');
  const makeEl = (props = {}) => {
    const classes = new Set();
    return { value: '', checked: false, dataset: {}, ...props,
      classList: { toggle(name, force) { if (force) classes.add(name); else classes.delete(name); }, contains: name => classes.has(name) } };
  };
  const elements = { 'nest-deploy-method': makeEl(), 'nest-secret': makeEl(), 'nest-export-secret': makeEl(), 'nest-secret-group': makeEl() };
  const context = { document: { getElementById: id => elements[id] || null } };
  vm.createContext(context);
  vm.runInContext(`${helpers}; globalThis.updateForTest = updateNestSecretField;`, context);
  const hidden = (method, { stored = false, exported = false, typed = '' } = {}) => {
    elements['nest-deploy-method'].value = method;
    elements['nest-secret'].dataset.stored = stored ? 'true' : '';
    elements['nest-secret'].value = typed;
    elements['nest-export-secret'].checked = exported;
    context.updateForTest();
    return elements['nest-secret-group'].classList.contains('is-hidden');
  };
  assert.equal(hidden('docker_remote'), true);
  assert.equal(hidden('docker_local'), true);
  assert.equal(hidden('docker_remote', { stored: true }), false, 'a stored secret stays editable');
  assert.equal(hidden('docker_remote', { exported: true }), false, 'the egg vault copy uses it');
  assert.equal(hidden('docker_local', { typed: 'pw' }), false, 'typed text never disappears');
  assert.equal(elements['nest-secret'].value, 'pw', 'switching methods must not clear the field');
  for (const method of ['ssh', 'docker_ssh']) assert.equal(hidden(method), false);
  assert.match(source, /function updateNestRemoteFields\(\) \{[\s\S]*?updateNestSecretField\(\);/);
  assert.match(source, /getElementById\('nest-export-secret'\)\?\.addEventListener\('change', updateNestSecretField\)/);
  assert.match(source, /secretInput\.dataset\.stored = isEdit && nest\?\.has_secret \? 'true' : ''/);
}

const tests = [
  ['Quick Connect SFTP mutations bind the authorized device', testQuickConnectSFTPMutationsBindDevice],
  ['Desktop recent files exclude directory contexts', testDesktopRecentFilesExcludeDirectoryContexts],
  ['Store operation failures survive rollback and bootstrap errors', testStoreOperationFailuresRemainVisible],
  ['Desktop Chat separates streamed tool rounds and final text', testDesktopChatSeparatesStreamedToolRounds],
  ['Game Maker reconnect and terminal job state remain consistent', testGameMakerEventConnectionLifecycle],
  ['Game Maker isolates old job events during replay and live updates', testGameMakerJobEventOwnership],
  ['Game Maker renews manual review grants without replacing the game', testGameMakerVisualGrantRenewal],
  ['Voxel keeps held inputs isolated by source', testVoxelInputOwnership],
  ['Game Maker asset selection reaches create and edit', testGameMakerAssetSelectionReachesCreateAndEdit],
  ['Game Maker sprite browser owns selection and cleanup', testGameMakerSpriteBrowserOwnsSelectionAndCleanup],
  ['Game Maker diagnostics belong to the current preview', testGameMakerDiagnosticsFollowPreviewLifetime],
  ['Game Maker stops reports from expired or finished validation', testGameMakerPreviewStopsExpiredValidation],
  ['Game Maker boot rejects invisible canvases and captures engine errors', testGameMakerBootDetectsInvisibleCanvasAndEngineErrors],
  ['Desktop media keys are visible to bootstrap', testDesktopMediaKeysAreInBootstrapScope],
  ['Game Maker forwards runtime diagnostics to validation and the next change', testGameMakerPreviewDiagnosticsReachValidationAndNextRequest],
  ['local LLM family selection keeps model, context and draft settings valid', testLocalLLMFamilySelection],
  ['browser audio lease uses an exclusive Web Lock', testBrowserAudioLeaseUsesExclusiveWebLock],
  ['versioned service-worker registration', testVersionedServiceWorkerRegistration],
  ['AuraSSE resumes after a frozen PWA becomes visible', testAuraSSEResumesWhenVisibleAfterFrozenConnection],
  ['Game Maker preview loading ignores stale frame settlement', testGameMakerPreviewLoadingIgnoresStaleFrameSettlement],
  ['service worker preserves media range responses', testServiceWorkerPreservesMediaRangeResponses],
  ['real skill snapshot differences', testSkillSnapshotDifferences],
  ['Python skill card ordering matches snapshots', testSkillCardOrderingMatchesSnapshots],
  ['Agent skill card ordering matches snapshots', testAgentSkillCardOrderingMatchesSnapshot],
  ['redacted marker preserves following content', testRedactedMarkerDoesNotConsumeFollowingContent],
  ['chat media allows person detection frames', testChatMediaAllowsDetectionFrames],
  ['Virtual Computers VNC preferences survive reconnect', testVirtualComputersVNCPreferencesSurviveReconnect],
  ['Virtual Computers VNC expansion stays inside app content', testVirtualComputersVNCExpansionUsesAppContentOnly],
  ['Virtual Computers terminal uses binary I/O and cleans up', testVirtualComputersTerminalBinaryLifecycle],
  ['Virtual Computers terminal survives refresh and reconciles safely', testVirtualComputersTerminalSessionReconciliation],
  ['Virtual Computers gates terminal and display actions by machine type', testVirtualComputersTerminalActionGating],
  ['Virtual Computers formats live expiry countdowns', testVirtualComputersExpiryCountdownFormatting],
  ['Virtual Computers ignores stale screenshot settlement', testVirtualComputersScreenshotSettlementIgnoresStaleRequests],
  ['Virtual Computers isolates resource failures', testVirtualComputersResourceFailuresStayIsolated],
  ['Virtual Computers preserves machine selection on refresh', testVirtualComputersSelectionSurvivesRefresh],
  ['Virtual Computers hides unavailable capability sections', testVirtualComputersHidesUnavailableCapabilitySections],
  ['Virtual Computers locks duplicate mutations', testVirtualComputersMutationLocksAreIdempotent],
  ['Virtual Computers allows independent windows', testVirtualComputersCanOpenIndependentWindows],
  ['Virtual Computers mobile layout uses available height', testVirtualComputersMobileLayoutUsesAvailableWindowHeight],
  ['Virtual Computers polls machines only when visible and changed', testVirtualComputersMachinePollingLifecycle],
  ['Virtual Computers clears machine polling on dispose', testVirtualComputersMachinePollingDisposeClearsTimer],
  ['Virtual Computers polls active agent workspaces', testVirtualComputersWorkspacePolling],
  ['local Granite multimodal observer remains idempotent', testLocalGraniteMultimodalObserverIsIdempotent],
  ['remote embedding status never renders Granite CPU state', testRemoteEmbeddingStatusNeverRendersGraniteCPUState],
  ['Network Cameras desktop contracts', testNetworkCamerasDesktopContracts],
  ['Network Cameras person detection views', testNetworkCamerasDetectionViews],
  ['Network Cameras person detection zone editor', testNetworkCamerasDetectionZones],
  ['Network Cameras person detection mode', testNetworkCamerasDetectionRuntime],
  ['Network Cameras person detection app wiring', testNetworkCamerasDetectionAppContracts],
  ['Manus catalog failures stay isolated and actions require ready status', testManusCatalogFailuresStayIsolatedAndActionsRequireReadyStatus],
  ['Speech Lab recorder resumes audio and routes fallbacks safely', testSpeechLabRecorderLifecycleAndFallbacks],
  ['Quick Connect SFTP navigator ignores stale listings', testQuickConnectSFTPNavigatorIgnoresStaleListings],
  ['Dashboard audit search ignores late responses', testDashboardAuditIgnoresLateResponses],
  ['Dashboard cronjob search ignores late responses', testDashboardCronjobsIgnoreLateResponses],
  ['SQL connections require an explicit TLS mode', testSQLConnectionsRequireExplicitTLSMode],
  ['Containers list failure stays visible until the list loads again', testContainersListFailureStateSurvivesFiltersUntilTheListLoads],
  ['Containers SSE merge keeps flags and reloads unknown containers', testContainersSSEMergeKeepsFlagsAndReloadsUnknownContainers],
  ['Containers send confirm=protected only after the modal', testContainersSendConfirmProtectedOnlyAfterTheModal],
  ['Containers empty state follows the list', testContainersEmptyStateFollowsTheList],
  ['Containers Resume unpauses a paused container', testContainersResumeUnpausesAPausedContainer],
  ['Containers terminal offers the confirmation after a refused handshake', testContainersTerminalOffersConfirmationAfterARefusedHandshake],
  ['Containers End session asks the server; Close keeps the shell', testContainersEndSessionAsksTheServerAndCloseKeepsTheShell],
  ['Containers badge is neutral with the reason as tooltip', testContainersProtectedBadgeIsNeutralWithTheReasonAsTooltip],
  ['Containers Stop asks only for self, endpoint and shared network', testContainersStopAsksOnlyForSelfEndpointAndSharedNetwork],
  ['Desktop main bundle parts end at function boundaries', testDesktopMainBundlePartsEndAtFunctionBoundaries],
  ['Window menu shortcut hints are drawn but not dispatched', testWindowMenuShortcutHintIsDrawnButNotDispatched],
  ['EasyDrag notifications keep only server-shaped flow and run ids', testEasyDragNotificationContextKeepsOnlyServerIds],
  ['Network Cameras notifications keep the detection event id', testNetworkCamerasNotificationContextKeepsEventIds],
  ['EasyDrag session keeps only valid flow ids', testEasyDragSessionKeepsOnlyValidFlowIds],
  ['EasyDrag flows_changed is forwarded as aurago:flows-changed', testEasyDragFlowsChangedIsForwarded],
  ['Desktop event delivered over WebSocket and SSE is handled once', testDesktopEventDeliveredTwiceIsHandledOnce],
  ['Config EasyDrag flows section keeps providers, numbers and locked agent options', testConfigFlowsSection],
  ['Config go2rtc streams save as the complete list', testConfigGo2RTCStreamsSaveAsCompleteList],
  ['Config A2A skills and remote agents save as complete lists', testConfigA2AListsSaveAsCompleteLists],
  ['Config 3D printer lists save as complete lists', testConfigThreeDPrinterListsSaveAsCompleteLists],
  ['byte-exact read-only bundle check', testBundleCheckRejectsNonCanonicalBytesWithoutWriting],
  ['Invasion nest form sends export_nest_secret', testInvasionNestFormSendsExportNestSecret],
  ['Invasion nest secret field hides only without effect', testInvasionNestSecretFieldHidesOnlyWithoutEffect]
];

let failures = 0;
for (const [name, test] of tests) {
  try {
    await test();
    console.log(`PASS ${name}`);
  } catch (error) {
    failures += 1;
    console.error(`FAIL ${name}: ${error.message}`);
  }
}
if (failures > 0) process.exitCode = 1;
