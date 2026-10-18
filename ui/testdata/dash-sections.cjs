// Runs ui/js/dashboard/dash-sections.js in bare Node vm contexts and checks the section registry: it loads without
// DashRouter, its id list equals DashRouter.SECTIONS, register() rejects unknown ids, duplicates and missing loaders,
// get() returns the definition (null for anything unregistered) and ids() keeps the registration order.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const dashboard = path.join(__dirname, '..', 'js', 'dashboard');
const SECTION_IDS = ['overview', 'personality', 'memory', 'user', 'knowledge', 'filesync', 'problems', 'automations', 'system', 'audit', 'logs', 'llm', 'prompts', 'tools'];

function run(ctx, file) {
    vm.runInContext(fs.readFileSync(file, 'utf8'), ctx, { filename: file });
}

{
    const ctx = {};
    ctx.window = ctx;
    vm.createContext(ctx);
    run(ctx, path.join(dashboard, 'dash-sections.js'));
    const S = ctx.DashSections;
    assert.ok(S, 'dash-sections.js must define window.DashSections without DashRouter or the DOM at load time');
    assert.deepEqual(Array.from(S.SECTION_IDS), SECTION_IDS);
    const router = {};
    router.window = router;
    vm.createContext(router);
    run(router, path.join(dashboard, 'dash-router.js'));
    assert.deepEqual(Array.from(router.DashRouter.SECTIONS), Array.from(S.SECTION_IDS), 'the registry keeps the router section list');

    assert.throws(() => S.register({ id: 'nope', load() {} }), /unknown section id "nope"/);
    assert.throws(() => S.register({ id: '__proto__', load() {} }), /unknown section id/);
    assert.throws(() => S.register(null), /unknown section id ""/);
    assert.throws(() => S.register({ id: 'logs' }), /needs a load function/);
    const logs = () => Promise.resolve(false);
    S.register({ id: 'logs', load: logs });
    S.register({ id: 'audit', load: () => Promise.resolve() });
    assert.throws(() => S.register({ id: 'logs', load() {} }), /already registered/);
    assert.equal(S.get('logs').id, 'logs');
    assert.equal(S.get('logs').load, logs);
    assert.equal(S.get('tools'), null, 'an unregistered section has no definition');
    assert.equal(S.get('nope'), null);
    assert.deepEqual(Array.from(S.ids()), ['logs', 'audit'], 'registration order');
}

console.log('DashSections: registry checks pass.');
