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
  const helperSource = sourceBetween(app, 'function applyResourceResult', 'async function refresh');
  const context = {};
  vm.createContext(context);
  vm.runInContext(`${helperSource}; globalThis.applyResult = applyResourceResult;`, context);

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

const tests = [
  ['versioned service-worker registration', testVersionedServiceWorkerRegistration],
  ['real skill snapshot differences', testSkillSnapshotDifferences],
  ['Python skill card ordering matches snapshots', testSkillCardOrderingMatchesSnapshots],
  ['Agent skill card ordering matches snapshots', testAgentSkillCardOrderingMatchesSnapshot],
  ['redacted marker preserves following content', testRedactedMarkerDoesNotConsumeFollowingContent],
  ['Virtual Computers VNC preferences survive reconnect', testVirtualComputersVNCPreferencesSurviveReconnect],
  ['Virtual Computers VNC expansion stays inside app content', testVirtualComputersVNCExpansionUsesAppContentOnly],
  ['Virtual Computers ignores stale screenshot settlement', testVirtualComputersScreenshotSettlementIgnoresStaleRequests],
  ['Virtual Computers isolates resource failures', testVirtualComputersResourceFailuresStayIsolated],
  ['Virtual Computers preserves machine selection on refresh', testVirtualComputersSelectionSurvivesRefresh],
  ['Virtual Computers hides unavailable capability sections', testVirtualComputersHidesUnavailableCapabilitySections],
  ['Virtual Computers locks duplicate mutations', testVirtualComputersMutationLocksAreIdempotent],
  ['Virtual Computers allows independent windows', testVirtualComputersCanOpenIndependentWindows],
  ['Virtual Computers mobile layout uses available height', testVirtualComputersMobileLayoutUsesAvailableWindowHeight],
  ['Virtual Computers agent tasks report errors and poll active jobs', testVirtualComputersAgentTaskFeedbackAndPolling],
  ['byte-exact read-only bundle check', testBundleCheckRejectsNonCanonicalBytesWithoutWriting]
];

let failures = 0;
for (const [name, test] of tests) {
  try {
    test();
    console.log(`PASS ${name}`);
  } catch (error) {
    failures += 1;
    console.error(`FAIL ${name}: ${error.message}`);
  }
}
if (failures > 0) process.exitCode = 1;
