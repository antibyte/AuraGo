// Runs ui/js/dashboard/dash-router.js in a bare Node vm context (no DOM) and checks the pure
// section resolver: section ids, '#'-prefixed hashes, every legacy tab id, unknown -> stored -> overview.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const file = path.join(__dirname, '..', 'js', 'dashboard', 'dash-router.js');
const ctx = { window: {} };
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(file, 'utf8'), ctx, { filename: file });
const router = ctx.window.DashRouter;
assert.ok(router, 'dash-router.js must define window.DashRouter without touching the DOM at load time');

const sections = ['overview', 'personality', 'memory', 'user', 'knowledge', 'filesync', 'problems', 'automations', 'system', 'audit', 'logs', 'llm', 'prompts', 'tools'];
const legacy = { overview: 'overview', agent: 'personality', user: 'user', knowledge: 'knowledge', filesync: 'filesync', audit: 'audit', cronjobs: 'automations', system: 'system' };
assert.equal(JSON.stringify(router.SECTIONS), JSON.stringify(sections), 'SECTIONS in sidebar order');
assert.equal(JSON.stringify(router.LEGACY_SECTION_MAP), JSON.stringify(legacy), 'legacy map covers the eight old tabs');
assert.equal(router.current(), null, 'no active section before init');

const resolve = router.resolveSection;
assert.equal(resolve('', null), 'overview', 'empty hash, nothing stored');
assert.equal(resolve(undefined, undefined), 'overview', 'no arguments');
for (const id of sections) {
    assert.equal(resolve(id, null), id, 'plain id ' + id);
    assert.equal(resolve('#' + id, null), id, 'hash #' + id);
}
for (const [old, id] of Object.entries(legacy)) {
    assert.equal(resolve('#' + old, null), id, 'legacy hash #' + old);
    assert.equal(resolve('', old), id, 'stored legacy value ' + old);
}
assert.equal(resolve('#tools', 'memory'), 'tools', 'a valid hash wins over the stored section');
assert.equal(resolve('#nope', 'logs'), 'logs', 'unknown hash falls back to the stored section');
assert.equal(resolve('#main-content', 'cronjobs'), 'automations', 'in-page anchors fall back to the stored (legacy) value');
assert.equal(resolve('#nope', 'nope'), 'overview', 'unknown hash and unknown stored value');
assert.equal(resolve('#nope', null), 'overview', 'unknown hash, nothing stored');
assert.equal(resolve('#constructor', '__proto__'), 'overview', 'object prototype keys are not sections');
console.log('DashRouter: resolveSection covers ' + sections.length + ' sections and ' + Object.keys(legacy).length + ' legacy ids.');
