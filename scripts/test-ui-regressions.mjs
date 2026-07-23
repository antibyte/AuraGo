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
  assert.match(initPWA, /navigator\.serviceWorker\.register\(serviceWorkerURL\(\)\)/);
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
        register(url) {
          registrations.push(url);
          return Promise.resolve({ scope: '/' });
        }
      }
    },
    document: { currentScript: { dataset: { swUrl: '/sw.js?v=chat-build' } } }
  });
  assert.deepEqual(registrations, ['/sw.js?v=chat-build']);
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
  const detailSource = sourceBetween(app, 'function detailPane', 'function taskRows');
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
  assert.equal(context.formatCountdown('2026-07-16T18:01:05Z', now), '01:05');
  assert.equal(context.formatCountdown('2026-07-16T19:01:01Z', now), '01:01:01');
  assert.equal(context.formatCountdown('2026-07-17T19:01:01Z', now), '1d 01:01:01');
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

  const state = { activeSection: 'volumes', status: { capabilities: { volumes: false, agent_tasks: true } } };
  context.reconcileSection(state);
  assert.equal(state.activeSection, 'machines');
  assert.equal(disconnected, 1, 'removing an active capability must close its live workspace');

  state.activeSection = 'tasks';
  context.reconcileSection(state);
  assert.equal(state.activeSection, 'tasks', 'available capability sections must stay selected');
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
  const helperSource = sourceBetween(shell, 'function findExistingAppWindow', 'function isStandaloneWidgetPath');
  const virtualWindow = { id: 'vc-1', appId: 'virtual-computers', element: { isConnected: true }, context: {} };
  const regularWindow = { id: 'settings-1', appId: 'settings', element: { isConnected: true }, context: {} };
  const context = {
    state: { windows: new Map([[virtualWindow.id, virtualWindow], [regularWindow.id, regularWindow]]), activeWindowId: '' },
    clearWindowMenus() {},
    disposeAppWindow() {},
    normalizeDesktopPath: value => String(value || '')
  };
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.findExisting = findExistingAppWindow;`, context);

  assert.equal(context.findExisting('virtual-computers', {}), undefined, 'Virtual Computers must allow a new independent window');
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
  const helperSource = sourceBetween(app, 'function normalizeMachineList', 'function hasActiveTasks');
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
    taskRefreshTimer: 7,
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

function testVirtualComputersAgentTaskFeedbackAndPolling() {
  const app = read('ui/js/desktop/apps/virtual-computers.js');
  const helperSource = sourceBetween(app, 'function notify', 'function dispose');
  const notifications = [];
  const scheduled = [];
  const context = {
    clearTimeout() {},
    setTimeout(callback, delay) {
      scheduled.push({ callback, delay });
      return scheduled.length;
    },
    refresh() {},
    tx(_ctx, key) { return key; }
  };
  vm.createContext(context);
  vm.runInContext(
    `${helperSource}; globalThis.hasActiveTasks = hasActiveTasks; ` +
    'globalThis.scheduleTaskRefresh = scheduleTaskRefresh; globalThis.notify = notify;',
    context
  );

  assert.equal(context.hasActiveTasks([{ status: 'queued' }]), true);
  assert.equal(context.hasActiveTasks([{ status: 'running' }]), true);
  assert.equal(context.hasActiveTasks([{ status: 'failed' }, { status: 'completed' }]), false);

  const state = {
    context: { notify(payload) { notifications.push(payload); } },
    tasks: [{ status: 'running' }],
    disposed: false,
    taskRefreshTimer: null
  };
  context.notify(state, 'Cannot cancel task', 'error');
  assert.equal(JSON.stringify(notifications), JSON.stringify([{
    title: 'desktop.notification',
    message: 'Cannot cancel task',
    type: 'error'
  }]));

  context.scheduleTaskRefresh(state);
  assert.equal(scheduled.length, 1);
  assert.equal(scheduled[0].delay, 2000);
  state.tasks = [{ status: 'canceled' }];
  context.scheduleTaskRefresh(state);
  assert.equal(scheduled.length, 1, 'terminal tasks must stop polling');
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

const tests = [
  ['browser audio lease uses an exclusive Web Lock', testBrowserAudioLeaseUsesExclusiveWebLock],
  ['versioned service-worker registration', testVersionedServiceWorkerRegistration],
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
  ['Virtual Computers agent tasks report errors and poll active jobs', testVirtualComputersAgentTaskFeedbackAndPolling],
  ['local Granite multimodal observer remains idempotent', testLocalGraniteMultimodalObserverIsIdempotent],
  ['remote embedding status never renders Granite CPU state', testRemoteEmbeddingStatusNeverRendersGraniteCPUState],
  ['Network Cameras desktop contracts', testNetworkCamerasDesktopContracts],
  ['byte-exact read-only bundle check', testBundleCheckRejectsNonCanonicalBytesWithoutWriting]
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
