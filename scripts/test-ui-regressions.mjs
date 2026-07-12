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

const tests = [
  ['versioned service-worker registration', testVersionedServiceWorkerRegistration],
  ['real skill snapshot differences', testSkillSnapshotDifferences],
  ['Python skill card ordering matches snapshots', testSkillCardOrderingMatchesSnapshots],
  ['Agent skill card ordering matches snapshots', testAgentSkillCardOrderingMatchesSnapshot],
  ['redacted marker preserves following content', testRedactedMarkerDoesNotConsumeFollowingContent],
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
