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

async function testGameMakerEventConnectionLifecycle() {
  const app = read('ui/js/desktop/apps/game-maker-studio.js');
  const sources = [], statuses = [], cards = [], phases = {};
  const context = {
    EventSource: class { constructor() { sources.push(this); } addEventListener() {} close() { this.closed = true; } },
    eventTypes: ['job_status'], terminalStatuses: new Set(['ready', 'failed', 'cancelled']),
    updateStatus(_state, status) { statuses.push(status); }, syncJobControls() {}, finalizeStreaming() {}, stopElapsed() {},
    phaseMarkup(_state, status) { return status; }, renderProjects() {},
    async reloadProjectRecord() {}, appendResultCard(_state, kind, message) { cards.push({ kind, message }); }
  };
  vm.createContext(context);
  for (const [start, end] of [['function connectEvents(', 'function parseEvent('], ['function closeEvents(', 'function dispose('], ['function handleJobStatus(', 'function finalizeStreaming(']]) {
    vm.runInContext(sourceBetween(app, start, end), context);
  }
  const state = { project: { id: 'bo', status: 'draft' }, job: { id: 'job', status: 'building' },
    api: { eventURL: () => '/events' }, container: { querySelector: () => phases }, context: { t: key => key } };
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
  assert.equal(cards[0].kind, 'cancelled');
  assert.equal(cards[0].message, 'Game creation exceeded its time limit.');
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
  state.container = { querySelector() { return form; } };
  await context.submitChange(state);
  assert.equal(submitted.preview_diagnostics[0].message, data.message);
  assert.equal(submitted.prompt, 'Fix the snake');
  assert.deepEqual(submitted.asset_pack_ids, ['space-shooter']);
  state.project = { id: 'other-project' };
  context.handlePreviewMessage(state, { ...event, data: { ...data, message: 'late old error' } });
  assert.equal(sent.length, 2, 'a switched project must not receive old preview reports');
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
  const state = {
    project: { id: 'snake' }, diagnostics: [],
    context: { esc: String, t: String },
    container: { querySelector(selector) {
      if (!nodes.has(selector)) nodes.set(selector, { replaceChildren() {}, setAttribute() {}, querySelectorAll: () => [] });
      return nodes.get(selector);
    } },
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
  assert.match(app, /window\.NetworkCamerasApp = \{ render, dispose \}/);
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
    TabState: { active: 'audit' },
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
  ['Manus catalog failures stay isolated and actions require ready status', testManusCatalogFailuresStayIsolatedAndActionsRequireReadyStatus],
  ['Speech Lab recorder resumes audio and routes fallbacks safely', testSpeechLabRecorderLifecycleAndFallbacks],
  ['Quick Connect SFTP navigator ignores stale listings', testQuickConnectSFTPNavigatorIgnoresStaleListings],
  ['Dashboard audit search ignores late responses', testDashboardAuditIgnoresLateResponses],
  ['Dashboard cronjob search ignores late responses', testDashboardCronjobsIgnoreLateResponses],
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
