import assert from 'node:assert/strict'; import { ENGINE_ORDER, loadFiles } from './harness.mjs';
const FILES = ENGINE_ORDER.map(f => 'ui/js/homedash/' + f).concat(['ui/js/desktop/apps/homedash-home.js']);
function setup(extra) {
  const { window, document } = loadFiles(FILES, { dom: true }), calls = [];
  const home = window.HomeDashHome.create(Object.assign({ onImport: () => calls.push('import'), t: (k, v) => (v && v.name ? k + ':' + v.name : v && v.source ? k + ':' + v.source : k), lang: 'en', onOpen: id => calls.push('open:' + id), onStarter: () => calls.push('starter'),
    onEmpty: () => calls.push('empty'), onHa: () => calls.push('ha'), sourceName: x => 'src:' + x, onDelete: s => calls.push('delete:' + s.id), onPin: s => calls.push('pin:' + s.id) }, extra));
  document.body.appendChild(home.el);
  return { home, calls, click: node => home.el.dispatchEvent({ type: 'click', target: node }) };
}
const D = { id: 'd_aaaaaaaaaaab', name: '<b>System</b>', revision: 1, updated_at: new Date().toISOString(), saved_by: 'agent' };
export const tests = [
  ['home: first run offers starter and empty dashboard; busy marks only the pressed choice', () => {
    const { home, calls, click } = setup(); home.render({ dashboards: [], desktopReadonly: false, busy: false });
    assert.equal(home.el.getAttribute('data-hdx-mode'), 'empty'); assert.equal(home.el.querySelector('.hdx-empty').hasAttribute('hidden'), false);
    assert.ok(home.el.querySelector('.hdx-empty svg.hdx-art')); const starter = home.el.querySelector('.hdx-empty [data-action="create-starter"]');
    assert.equal(home.el.querySelector('.hdx-empty-title').textContent, 'homedash.home.empty_title'); click(starter); assert.deepEqual(calls, ['starter']);
    home.render({ dashboards: [], desktopReadonly: false, busy: true });
    assert.equal(starter.getAttribute('aria-busy'), 'true'); assert.equal(home.el.querySelector('.hdx-empty [data-action="create-empty"]').getAttribute('aria-busy'), null); assert.equal(starter.getAttribute('aria-disabled'), 'true'); assert.equal(starter.hasAttribute('disabled'), false);
    click(starter); assert.deepEqual(calls, ['starter']); /* the click guard holds while busy, focus stays on the key */
    home.render({ dashboards: [], desktopReadonly: false, busy: false }); assert.equal(starter.getAttribute('aria-disabled'), null); click(starter); assert.deepEqual(calls, ['starter', 'starter']); }],
  ['home: a read-only desktop hides creation and shows the note', () => {
    const { home } = setup(); home.render({ dashboards: [], desktopReadonly: true });
    for (const b of home.el.querySelectorAll('[data-action="create-starter"], [data-action="create-empty"]')) assert.ok(b.hasAttribute('hidden'));
    assert.equal(home.el.querySelector('.hdx-readonly-note').hasAttribute('hidden'), false); }],
  ['home: the list shows server names as text, opens and deletes, and keeps its nodes for the same list', () => {
    const { home, calls, click } = setup(); home.render({ dashboards: [D], desktopReadonly: false });
    assert.equal(home.el.getAttribute('data-hdx-mode'), 'list'); const item = home.el.querySelector('.hdx-item');
    assert.equal(item.querySelector('.hdx-card-name').textContent, '<b>System</b>'); assert.equal(item.querySelector('b'), null);
    assert.ok(item.querySelector('.hdx-card-meta').textContent.includes('homedash.home.by_agent'));
    click(item.querySelector('[data-action="open"]')); click(item.querySelector('[data-action="delete"]')); assert.deepEqual(calls, ['open:' + D.id, 'delete:' + D.id]);
    home.render({ dashboards: [D], desktopReadonly: false }); assert.equal(home.el.querySelector('.hdx-item'), item);
    home.render({ dashboards: [D], desktopReadonly: true }); assert.equal(home.el.querySelector('.hdx-card-delete'), null); }],
  ['home: notices are translated status lines', () => {
    const { home } = setup(); home.render({ dashboards: [], notice: 'limit' }); const n = home.el.querySelector('.hdx-notice');
    assert.equal(n.textContent, 'homedash.home.notice_limit'); assert.equal(n.getAttribute('role'), 'status'); assert.equal(n.getAttribute('data-tone'), 'warn');
    home.render({ dashboards: [], notice: null }); assert.equal(n.hasAttribute('hidden'), false); assert.equal(n.textContent, ''); assert.equal(n.getAttribute('role'), 'status'); }],
  ['home: the Home Assistant choice and header button follow the templates and the read-only state', () => {
    const { home, calls, click } = setup(), ha = () => Array.from(home.el.querySelectorAll('[data-action="create-ha"]'));
    home.render({ dashboards: [], templates: ['system'] }); assert.equal(ha().length, 2); assert.ok(ha().every(b => b.hasAttribute('hidden')));
    home.render({ dashboards: [], templates: ['system', 'home_assistant'] }); assert.ok(ha().every(b => !b.hasAttribute('hidden')));
    assert.ok(home.el.querySelector('.hdx-choices [data-action="create-ha"]')); assert.ok(home.el.querySelector('.hdx-home-actions [data-action="create-ha"]'));
    assert.equal(home.el.querySelectorAll('.hdx-choices .hdx-choice').length, 3); click(ha()[0]); assert.deepEqual(calls, ['ha']);
    home.render({ dashboards: [], templates: ['home_assistant'], desktopReadonly: true }); assert.ok(ha().every(b => b.hasAttribute('hidden')));
    home.render({ dashboards: [] }); assert.ok(ha().every(b => b.hasAttribute('hidden'))); }],
  ['home: template_empty and source_unavailable notices name the source', () => {
    const { home } = setup(), n = home.el.querySelector('.hdx-notice');
    home.render({ dashboards: [], notice: 'template_empty' }); assert.equal(n.textContent, 'homedash.home.notice_template_empty'); assert.equal(n.getAttribute('data-tone'), 'warn');
    home.render({ dashboards: [], notice: 'source_unavailable', noticeSource: 'ha' }); assert.equal(n.getAttribute('data-tone'), 'warn');
    assert.equal(n.textContent, 'homedash.home.notice_source_unavailable:src:ha'); }],
  ['home: choiceState is a pure table (read-only hides everything, Home Assistant needs the template, busy marks the pressed choice, import ignores templates)', () => {
    const { window } = loadFiles(FILES, { dom: true }), cs = window.HomeDashHome.choiceState, plain = v => JSON.parse(JSON.stringify(v));
    assert.deepEqual(plain(cs({ desktopReadonly: false, templates: [] }, '')), { 'create-starter': { hidden: false, disabled: false, busy: false }, 'create-empty': { hidden: false, disabled: false, busy: false }, 'create-ha': { hidden: true, disabled: false, busy: false }, import: { hidden: false, disabled: false, busy: false } });
    assert.equal(cs({ templates: ['home_assistant'] }, '')['create-ha'].hidden, false); assert.ok(Object.values(cs({ desktopReadonly: true, templates: ['home_assistant'] }, '')).every(c => c.hidden));
    const busy = plain(cs({ busy: true, templates: ['home_assistant'] }, 'create-empty')); assert.equal(busy['create-empty'].busy, true); assert.equal(busy['create-starter'].busy, false); assert.ok(Object.values(busy).every(c => c.disabled)); assert.equal(busy.import.busy, false); assert.doesNotThrow(() => cs(null, undefined)); }],
  ['home: Import is a secondary action below the first-run choices and in the list head, hidden when read-only, and runs onImport without a pending create', () => {
    const { home, calls, click } = setup(); home.render({ dashboards: [], desktopReadonly: false }); const imp = home.el.querySelector('.hdx-empty [data-action="import"]'); assert.ok(imp); assert.equal(imp.textContent, 'homedash.menu.import');
    assert.equal(imp.parentElement.getAttribute('class'), 'hdx-secondary'); click(imp); assert.deepEqual(calls, ['import']); home.render({ dashboards: [D], desktopReadonly: false }); assert.ok(home.el.querySelector('.hdx-home-head [data-action="import"]'));
    click(home.el.querySelector('.hdx-home-head [data-action="import"]')); assert.deepEqual(calls, ['import', 'import']); home.render({ dashboards: [], desktopReadonly: false, busy: true }); click(imp); assert.equal(calls.length, 2, 'the busy guard holds');
    home.render({ dashboards: [], desktopReadonly: true }); assert.ok(imp.hasAttribute('hidden')); assert.ok(home.el.querySelector('.hdx-home-head [data-action="import"]').hasAttribute('hidden')); }],
  ['home: a card pin button follows pinState (pressed, busy, repaint on change), is hidden when read-only or without pinState, and the meta segment shows the shortcut even read-only', () => {
    let ps = 'off'; const { home, calls, click } = setup({ pinState: () => ps }), q = s => home.el.querySelector(s), state = ro => ({ dashboards: [D], desktopReadonly: ro });
    home.render(state(false)); const pin = q('.hdx-card-pin'); assert.ok(pin); assert.equal(pin.getAttribute('data-action'), 'pin'); assert.equal(pin.getAttribute('data-id'), D.id); assert.equal(pin.getAttribute('aria-pressed'), 'false');
    assert.equal(pin.getAttribute('aria-label'), 'homedash.home.show_on_desktop:<b>System</b>'); assert.equal(pin.getAttribute('title'), pin.getAttribute('aria-label')); assert.equal(q('.hdx-card-pinned'), null);
    const kids = Array.from(q('.hdx-card').children).map(n => n.getAttribute('class')); assert.deepEqual(kids, ['hdx-card-open', 'hdx-card-pin', 'hdx-card-delete']);
    click(pin); assert.deepEqual(calls, ['pin:' + D.id]);
    ps = 'busy-off'; home.render(state(false)); const busy = q('.hdx-card-pin'); assert.equal(busy.getAttribute('aria-busy'), 'true'); assert.equal(busy.getAttribute('aria-disabled'), 'true'); assert.equal(busy.getAttribute('aria-pressed'), 'false'); click(busy); assert.deepEqual(calls, ['pin:' + D.id]);
    ps = 'on'; home.render(state(false)); const on = q('.hdx-card-pin'); assert.equal(on.getAttribute('aria-pressed'), 'true'); assert.equal(on.getAttribute('aria-label'), 'homedash.home.show_on_desktop:<b>System</b>'); assert.equal(on.getAttribute('aria-busy'), null);
    assert.equal(q('.hdx-card-pinned').textContent, 'homedash.home.on_desktop'); assert.ok(q('.hdx-card-meta').textContent.includes('homedash.home.by_agent')); assert.equal(q('.hdx-card-meta b'), null);
    home.render(state(true)); assert.equal(q('.hdx-card-pin'), null); assert.equal(q('.hdx-card-delete'), null); assert.equal(q('.hdx-card-pinned').textContent, 'homedash.home.on_desktop');
    const plain = setup(); plain.home.render(state(false)); assert.equal(plain.home.el.querySelector('.hdx-card-pin'), null); assert.equal(plain.home.el.querySelector('.hdx-card-pinned'), null); }]
];
