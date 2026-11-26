import assert from 'node:assert/strict'; import vm from 'node:vm'; import { loadEngine, ENGINE_ORDER, read, eq, noStyleAttr } from './harness.mjs';
const FILES = ENGINE_ORDER.concat(['hd-tone.js', 'hd-api.js', 'hd-catalog.js', 'hd-render.js']);
const STARTER = JSON.parse(read('ui/testdata/homedash-m2/starter-dashboard.json')), CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json'));
const V = (id, value) => ({ id, kind: 'sensor', name: id, unit: '%', value, available: true, status: 'ok', readonly: false, stale: false, pending: false, attributes: { updated_at: '2026-11-25T10:00:00Z' } });
function setup(rawDoc, tr, extra) {
  const { E, document } = loadEngine(FILES, { dom: true }), catalog = E.catalog.create(CATALOG), doc = E.doc.normalise(rawDoc || STARTER, { sizeOf: catalog.defaultSize });
  const root = E.el('div', { class: 'hd-root', 'data-hd-family': 'glass' }), stage = E.el('div', { class: 'hd-stage' }); root.appendChild(stage); document.body.appendChild(root);
  const views = new Map(), bus = E.core.emitter(), histories = [], frames = [], layouts = []; let width = 1180;
  const store = { view: id => views.get(id) || null, on: bus.on, off: bus.off, history(id, range, points) { histories.push([id, range, points]); return Promise.resolve({ points: [[1, 2], [2, 3]], source: 'source' }); } };
  const page = E.render.page(Object.assign({ doc, page: doc.pages[0], stage, store, catalog, t: tr || (k => k), lang: 'en', raf: fn => { frames.push(fn); return frames.length; }, caf: () => {},
    measure: () => ({ width, height: 700 }), onLayout: m => layouts.push(m) }, extra));
  return { E, doc, stage, views, bus, histories, page, layouts, run: () => { while (frames.length) frames.shift()(); }, setWidth: w => { width = w; } };
}
// Command-enabled page: a fake commands client (decorate passthrough plus a settable overlay, a send recorder, an emitter) and a spy on the widget host that records each widget api.
const CAPS = ['toggle', { name: 'turn_on' }, { name: 'turn_off' }], LIGHT_CAPS = CAPS.concat([{ name: 'brightness', min: 0, max: 100, step: 1 }]);
const CV = (id, kind, extra) => Object.assign({ id, kind, name: id, value: false, available: true, status: 'ok', readonly: false, stale: false, pending: false, attributes: {}, capabilities: CAPS }, extra);
const CMD_DOC = { id: 'd_cmdcmdcmdc1', name: 'Cmd', pages: [{ id: 'main', widgets: [
  { id: 'sw', type: 'switch', variant: 'pill', bindings: { primary: 'ha:switch.coffee' } }, { id: 'lt', type: 'light', variant: 'tile', bindings: { primary: 'ha:light.kitchen' } },
  { id: 'cpu', type: 'gauge', variant: 'arc', bindings: { primary: 'system:cpu' } }],
  layouts: { lg: [{ id: 'sw', x: 0, y: 0, w: 2, h: 1 }, { id: 'lt', x: 2, y: 0, w: 2, h: 2 }, { id: 'cpu', x: 4, y: 0, w: 2, h: 2 }] } }] };
function setupCmd(withCommands, rawDoc, extraViews, extra) {
  const { E, document } = loadEngine(FILES, { dom: true }), catalog = E.catalog.create(CATALOG), doc = E.doc.normalise(rawDoc || CMD_DOC, { sizeOf: catalog.defaultSize });
  const root = E.el('div', { class: 'hd-root', 'data-hd-family': 'glass' }), stage = E.el('div', { class: 'hd-stage' }); root.appendChild(stage); document.body.appendChild(root);
  const views = new Map(), bus = E.core.emitter(), cbus = E.core.emitter(), frames = [], sent = [], overlay = new Map(), announced = [], sheets = [], apis = new Map();
  const real = E.widgets.host; E.widgets = Object.assign({}, E.widgets, { host(o) { const inst = real(o); apis.set(o.widget.id, o.api({ widget: o.widget, mode: 'view' })); return inst; } });
  const commands = { sent, decorate: (id, v) => (overlay.has(id) ? Object.freeze(Object.assign({}, v, overlay.get(id))) : v), send: spec => { sent.push(spec); return Promise.resolve({ status: 'accepted' }); }, on: cbus.on, off: cbus.off };
  const store = { view: id => views.get(id) || null, on: bus.on, off: bus.off, history: () => Promise.resolve({ points: [], source: 'none' }) };
  views.set('ha:switch.coffee', CV('ha:switch.coffee', 'switch')); views.set('ha:light.kitchen', CV('ha:light.kitchen', 'light', { capabilities: LIGHT_CAPS }));
  for (const v of extraViews || []) views.set(v.id, v);
  const page = E.render.page(Object.assign({ doc, page: doc.pages[0], stage, store, catalog, t: k => k, lang: 'en', raf: fn => { frames.push(fn); return frames.length; }, caf: () => {}, measure: () => ({ width: 1180, height: 700 }),
    commands: withCommands === false ? null : commands, moreInfo: req => sheets.push(req), announce: x => announced.push(x) }, extra));
  return { E, document, stage, views, bus, cbus, sent, overlay, announced, sheets, apis, page, run: () => { while (frames.length) frames.shift()(); }, text: id => stage.querySelector('[data-hd-wid="' + id + '"] .hd-state-text') };
}
const vars = (el, ...names) => names.map(n => el.style.getPropertyValue(n)).join(',');
export const tests = [
  ['render: one host per widget, placed in cells, sized in pixels, history in the widget hours', () => {
    const { stage, histories, layouts } = setup();
    assert.equal(stage.querySelectorAll('.hd-page > .hd-widget').length, 6);
    const rx = stage.querySelector('[data-hd-wid="net-rx"]'); assert.equal(vars(rx, '--hd-x', '--hd-y', '--hd-w', '--hd-h'), '0,4,5,2');
    assert.equal(rx.querySelector('svg.hd-spark').getAttribute('width'), '473');
    assert.ok(histories.some(h => h[0] === 'system:net_rx' && h[1] === 3600000 && h[2] === 120)); assert.equal(layouts[0].bp, 'lg'); assert.equal(layouts[0].fit, 'scroll'); }],
  ['render: tone from warn/alert reaches the wrapper and zones are drawn; only bound widgets update', () => {
    const { stage, views, bus } = setup(), cpu = stage.querySelector('[data-hd-wid="cpu"]');
    assert.ok(cpu.querySelector('path.hd-gauge-zone[data-tone="crit"]')); assert.ok(cpu.querySelector('path.hd-gauge-zone[data-tone="warn"]'));
    views.set('system:cpu', V('system:cpu', 97)); views.set('system:memory', V('system:memory', 99)); bus.emit('change', new Set(['system:cpu']));
    assert.equal(cpu.getAttribute('data-hd-tone'), 'crit'); assert.equal(stage.querySelector('[data-hd-wid="memory"]').getAttribute('data-hd-tone'), null);
    views.set('system:cpu', V('system:cpu', 85)); bus.emit('change', null); assert.equal(cpu.getAttribute('data-hd-tone'), 'warn'); assert.equal(stage.querySelector('[data-hd-wid="memory"]').getAttribute('data-hd-tone'), 'crit'); }],
  ['render: a type the engine does not register gets a labelled cell; an empty page gets its note', () => {
    const doc = { name: 'X', pages: [{ id: 'main', widgets: [{ id: 'clock', type: 'clock', variant: 'digital', title: 'Clock' }], layouts: { lg: [{ id: 'clock', x: 0, y: 0, w: 2, h: 1 }] } }] };
    const { stage } = setup(doc), cell = stage.querySelector('.hd-unsupported');
    assert.equal(cell.querySelector('.hd-title').textContent, 'Clock'); assert.equal(cell.querySelector('.hd-state-text').textContent, 'homedash.view.unsupported');
    const empty = setup({ name: 'E', pages: [{ id: 'main', widgets: [], layouts: { lg: [] } }] });
    assert.equal(empty.stage.querySelector('.hd-page-empty .hd-page-empty-title').textContent, 'homedash.view.empty_title'); }],
  ['render: built-in system entities get localized names; other entities keep the server name', () => {
    const { stage, views, bus } = setup(null, k => 'T:' + k), title = id => stage.querySelector('[data-hd-wid="' + id + '"] .hd-title').textContent;
    views.set('system:memory', Object.assign(V('system:memory', 40), { source: 'system', name: 'Memory' }));
    views.set('system:cpu', Object.assign(V('system:cpu', 10), { source: 'other', name: 'Custom CPU' }));
    bus.emit('change', null);
    assert.equal(title('memory'), 'T:homedash.system.memory');
    assert.equal(views.get('system:memory').name, 'Memory');
    assert.equal(title('cpu'), 'Custom CPU'); }],
  ['render: breakpoints follow the stage width with derived, constrained layouts; destroy cleans up', () => {
    const { stage, page, run, setWidth, layouts, bus } = setup();
    setWidth(500); page.layout(); run(); assert.equal(layouts[layouts.length - 1].bp, 'sm');
    assert.equal(stage.querySelector('[data-hd-wid="cpu"]').style.getPropertyValue('--hd-w'), '2');
    page.destroy(); assert.equal(stage.querySelectorAll('.hd-widget').length, 0); bus.emit('change', null); }],
  ['render: without commands the page is view only (can false, command rejects view_only, no more button, no sheet)', async () => {
    const { apis, sheets, stage } = setupCmd(false), api = apis.get('sw');
    assert.equal(api.can('primary', 'turn_on'), false); assert.equal(api.moreInfo('primary'), false); assert.equal(apis.get('lt').moreInfo('primary'), false); assert.equal(sheets.length, 0);
    await assert.rejects(api.command('primary', 'turn_on', {}), e => e.code === 'view_only'); assert.equal(stage.querySelectorAll('.hd-more').length, 0);
    assert.equal(stage.querySelector('[data-hd-wid="cpu"] .hd-title').textContent, ''); }],
  ['render: can follows capability, availability, read-only and mode; command sends the full spec with the widget key', async () => {
    const { apis, views, sent } = setupCmd(), api = apis.get('sw');
    assert.equal(api.can('primary', 'turn_on'), true); assert.equal(api.can('primary', 'brightness'), false); assert.equal(api.can('nope', 'turn_on'), false);
    views.set('ha:switch.coffee', CV('ha:switch.coffee', 'switch', { capabilities: ['toggle', { name: 'turn_on', allowed: false }] })); assert.equal(api.can('primary', 'turn_on'), false); assert.equal(api.can('primary', 'toggle'), true);
    eq(await api.command('primary', 'turn_on', {}), { status: 'refused' }); assert.equal(sent.length, 0);
    views.set('ha:switch.coffee', CV('ha:switch.coffee', 'switch')); await api.command('primary', 'turn_on', { x: 1 }, { phase: 'commit' });
    eq(sent, [{ entity: 'ha:switch.coffee', command: 'turn_on', args: { x: 1 }, phase: 'commit', widgetKey: 'w|d_cmdcmdcmdc1|sw', widgetId: 'sw', dashboardId: 'd_cmdcmdcmdc1', confirm: false }]);
    for (const v of [{ readonly: true }, { available: false }, { status: 'unavailable' }, { status: 'unknown' }]) { views.set('ha:switch.coffee', CV('ha:switch.coffee', 'switch', v)); assert.equal(api.can('primary', 'turn_on'), false, JSON.stringify(v)); } }],
  ['render: a cue from the client reaches the state line as its localized text in one frame; cues and armings are announced', () => {
    const { cbus, overlay, run, text, announced, apis } = setupCmd();
    overlay.set('ha:switch.coffee', { status: 'error', error: { code: 'timeout' } }); cbus.emit('change', ['ha:switch.coffee']);
    assert.notEqual(text('sw').textContent, 'homedash.cue.timeout'); run(); assert.equal(text('sw').textContent, 'homedash.cue.timeout');
    assert.equal(apis.get('sw').entity('primary').error.text, 'homedash.cue.timeout');
    cbus.emit('cue', { id: 'ha:switch.coffee', cue: { code: 'timeout' } }); cbus.emit('cue', { id: 'ha:other', cue: { code: 'timeout' } });
    cbus.emit('armed', { id: 'ha:switch.coffee', widgetKey: 'w|d_cmdcmdcmdc1|sw' }); cbus.emit('armed', { id: 'ha:switch.coffee', widgetKey: 'w|other|sw' });
    eq(announced, ['homedash.cue.timeout', 'homedash.arm.announce']); }],
  ['render: a widget whose entity has no view shows the localized system name as its title; only a light-kind entity shows the more button', () => {
    const { stage } = setupCmd(); assert.equal(stage.querySelector('[data-hd-wid="cpu"] .hd-title').textContent, 'homedash.system.cpu');
    const shown = Array.from(stage.querySelectorAll('.hd-more')).filter(b => !b.hasAttribute('hidden'));
    assert.equal(shown.length, 1); assert.ok(stage.querySelector('[data-hd-wid="lt"] .hd-more') === shown[0]); }],
  ['render: the more button follows the bound entity kind, not the widget type (pill on a light shows it, sensor widget hides it, no commands: none)', () => {
    const raw = { id: 'd_cmdcmdcmdc2', name: 'K', pages: [{ id: 'main', widgets: [
      { id: 'pl', type: 'switch', variant: 'pill', bindings: { primary: 'ha:light.sofa' } }, { id: 'cpu', type: 'gauge', variant: 'arc', bindings: { primary: 'system:cpu' } }],
      layouts: { lg: [{ id: 'pl', x: 0, y: 0, w: 2, h: 1 }, { id: 'cpu', x: 2, y: 0, w: 2, h: 2 }] } }] };
    const extra = [CV('ha:light.sofa', 'light', { capabilities: LIGHT_CAPS }), V('system:cpu', 40)];
    const { stage, apis, sheets, views, bus } = setupCmd(true, raw, extra), more = id => stage.querySelector('[data-hd-wid="' + id + '"] .hd-more');
    assert.ok(more('pl')); assert.equal(more('pl').hasAttribute('hidden'), false); assert.ok(more('cpu')); assert.equal(more('cpu').hasAttribute('hidden'), true);
    more('pl').dispatchEvent({ type: 'click', preventDefault() {}, stopPropagation() {} }); assert.equal(sheets.length, 1); assert.equal(sheets[0].api.entity('primary').id, 'ha:light.sofa');
    views.set('ha:light.sofa', CV('ha:light.sofa', 'switch')); bus.emit('change', new Set(['ha:light.sofa'])); assert.equal(more('pl').hasAttribute('hidden'), true, 'follows the kind when it changes');
    const bare = setupCmd(false, raw, extra); assert.equal(bare.stage.querySelectorAll('.hd-more').length, 0); }],
  ['render: the sheet opener is the focused element inside the widget, else its visible more button, else the widget with a temporary tabindex', () => {
    const raw = { id: 'd_cmdcmdcmdc3', name: 'K', pages: [{ id: 'main', widgets: [{ id: 'lt', type: 'light', variant: 'tile', bindings: { primary: 'ha:light.kitchen' } }], layouts: { lg: [{ id: 'lt', x: 0, y: 0, w: 2, h: 2 }] } }] };
    const { stage, apis, sheets, document } = setupCmd(true, raw), wrap = stage.querySelector('[data-hd-wid="lt"]'), more = wrap.querySelector('.hd-more'), title = wrap.querySelector('.hd-title');
    document.activeElement = document.body; apis.get('lt').moreInfo('primary'); assert.ok(sheets[0].opener === more, 'body is not inside the widget: the more button');
    document.activeElement = title; apis.get('lt').moreInfo('primary'); assert.ok(sheets[1].opener === title, 'an element inside the widget wins');
    document.activeElement = document.body; more.setAttribute('hidden', ''); assert.equal(wrap.hasAttribute('tabindex'), false); apis.get('lt').moreInfo('primary');
    assert.ok(sheets[2].opener === wrap, 'a hidden more button is no opener'); assert.equal(wrap.getAttribute('tabindex'), '-1');
    wrap.dispatchEvent({ type: 'blur' }); assert.equal(wrap.hasAttribute('tabindex'), false, 'the temporary tabindex is dropped again'); }],
  ['render: moreInfo opens the sheet for light entities only, with a sheet api and a subscription that follows the entity', () => {
    const { apis, sheets, cbus, bus, run, sent } = setupCmd();
    assert.equal(apis.get('sw').moreInfo('primary'), false); assert.equal(sheets.length, 0);
    assert.equal(apis.get('lt').moreInfo('primary'), true); assert.equal(sheets.length, 1);
    const req = sheets[0], hits = []; assert.equal(req.family, 'glass'); assert.equal(req.api.entity('primary').id, 'ha:light.kitchen');
    const off = req.subscribe(() => hits.push(1)); cbus.emit('change', ['ha:light.kitchen']); run(); assert.equal(hits.length, 1);
    bus.emit('change', new Set(['ha:light.kitchen'])); assert.equal(hits.length, 2); bus.emit('change', new Set(['ha:switch.coffee'])); assert.equal(hits.length, 2);
    off(); bus.emit('change', null); assert.equal(hits.length, 2);
    req.api.command('primary', 'turn_on', {}); eq(sent.map(x => x.widgetKey), ['sheet|d_cmdcmdcmdc1|lt']); }],
  ['render: destroy detaches the command listeners', () => {
    const { page, cbus, run, stage, announced } = setupCmd(); page.destroy(); cbus.emit('change', ['ha:switch.coffee']); cbus.emit('cue', { id: 'ha:switch.coffee', cue: { code: 'timeout' } }); run();
    eq(announced, []); assert.equal(stage.querySelectorAll('.hd-widget').length, 0); }],
  ['render (M4): design mode makes a listbox of options named by describe, hides the more button and refuses every command and more-info', async () => {
    const describe = (w, it, ent) => w.id + '@' + it.x + ',' + it.y + (ent ? '#' + ent.id : '');
    const { page, stage, apis, sheets, sent } = setupCmd(true, null, null, { mode: 'design', describe }), pg = page.el;
    assert.equal(pg.getAttribute('role'), 'listbox'); assert.equal(pg.getAttribute('aria-multiselectable'), 'true'); assert.equal(pg.getAttribute('aria-label'), 'Cmd'); assert.equal(pg.getAttribute('aria-labelledby'), null);
    const lt = stage.querySelector('[data-hd-wid="lt"]'); assert.equal(lt.getAttribute('role'), 'option'); assert.equal(lt.getAttribute('aria-selected'), 'false'); assert.equal(lt.getAttribute('aria-labelledby'), null);
    assert.equal(lt.getAttribute('aria-label'), 'lt@2,0#ha:light.kitchen'); assert.equal(lt.getAttribute('tabindex'), '-1');
    for (const m of stage.querySelectorAll('.hd-more')) assert.equal(m.hasAttribute('hidden'), true);
    assert.equal(apis.get('lt').can('primary', 'turn_on'), false); eq(await apis.get('lt').command('primary', 'turn_on', {}), { status: 'refused' });
    assert.equal(apis.get('lt').moreInfo('primary'), false); assert.equal(sheets.length, 0); assert.equal(sent.length, 0); noStyleAttr(pg); }],
  ['render (M4): setMode switches roles, labels, the more button and command rights both ways; a view frame gets its title reference back', async () => {
    const { page, stage, apis, sent } = setupCmd(true), lt = stage.querySelector('[data-hd-wid="lt"]'), ref = lt.getAttribute('aria-labelledby');
    assert.ok(ref); assert.equal(lt.getAttribute('role'), 'group'); assert.equal(apis.get('lt').can('primary', 'turn_on'), true);
    page.setMode('design'); assert.equal(lt.getAttribute('role'), 'option'); assert.equal(lt.getAttribute('aria-labelledby'), null); assert.equal(lt.getAttribute('aria-label'), 'light');
    assert.equal(lt.querySelector('.hd-more').hasAttribute('hidden'), true); assert.equal(apis.get('lt').can('primary', 'turn_on'), false);
    eq(await apis.get('lt').command('primary', 'turn_on', {}), { status: 'refused' }); assert.equal(sent.length, 0);
    page.setMode('view'); assert.equal(lt.getAttribute('role'), 'group'); assert.equal(lt.getAttribute('aria-labelledby'), ref); assert.equal(lt.getAttribute('aria-label'), null); assert.equal(lt.getAttribute('aria-selected'), null);
    assert.equal(lt.getAttribute('tabindex'), null); assert.equal(lt.querySelector('.hd-more').hasAttribute('hidden'), false); assert.equal(apis.get('lt').can('primary', 'turn_on'), true);
    assert.equal(page.el.getAttribute('role'), 'region'); assert.equal(page.el.getAttribute('aria-multiselectable'), null); }],
  ['render (M4): update keeps the nodes of unchanged widgets, replaces a changed one, mounts new ones and rebuilds the entity index', () => {
    const { doc, page, stage, run } = setup(), node = id => stage.querySelector('[data-hd-wid="' + id + '"]'), before = new Map(['cpu', 'memory', 'net-rx'].map(id => [id, node(id)]));
    const next = JSON.parse(JSON.stringify(doc)), widgets = next.pages[0].widgets; widgets.find(w => w.id === 'cpu').title = 'Processor';
    page.update(next, next.pages[0]);
    assert.ok(node('memory') === before.get('memory')); assert.ok(node('net-rx') === before.get('net-rx')); assert.ok(node('cpu') !== before.get('cpu')); assert.equal(node('cpu').querySelector('.hd-title').textContent, 'Processor');
    assert.equal(stage.querySelectorAll('.hd-page > .hd-widget').length, 6);
    const cut = Object.assign({}, next.pages[0], { widgets: widgets.filter(w => w.id !== 'memory') }); page.update(Object.assign({}, next, { pages: [cut] }), cut);
    assert.equal(node('memory'), null); assert.equal(stage.querySelectorAll('.hd-page > .hd-widget').length, 5); assert.ok(!page.entityIds().includes('system:memory')); assert.ok(page.entityIds().includes('system:cpu'));
    const none = Object.assign({}, next.pages[0], { widgets: [], layouts: { lg: [], md: null, sm: null } }); page.update(Object.assign({}, next, { pages: [none] }), none);
    assert.equal(stage.querySelectorAll('.hd-page > .hd-widget').length, 0); assert.ok(stage.querySelector('.hd-page-empty')); eq(page.entityIds(), []); run(); }],
  ['render (M4): the empty note follows mode and action, is rebuilt only when they change, and a new widget removes it', () => {
    const raw = { name: 'E', pages: [{ id: 'main', widgets: [], layouts: { lg: [] } }] }, { doc, page, stage } = setup(raw, null, { mode: 'design', emptyAction: { cmd: 'add_widget', label: 'Add' } });
    const note = () => stage.querySelector('.hd-page-empty'), btn = () => stage.querySelector('.hd-page-empty-action');
    assert.equal(note().getAttribute('data-hd-mode'), 'design'); assert.equal(note().querySelector('.hd-page-empty-title').textContent, 'homedash.designer.empty_title'); assert.equal(btn().getAttribute('data-hd-cmd'), 'add_widget'); assert.equal(btn().textContent, 'Add');
    const same = note(); page.update(doc, doc.pages[0]); assert.ok(note() === same, 'unchanged inputs keep the note');
    page.setEmptyAction(null); assert.equal(btn(), null); page.setMode('view'); assert.equal(note().getAttribute('data-hd-mode'), 'view'); assert.equal(note().querySelector('.hd-page-empty-title').textContent, 'homedash.view.empty_title');
    const next = JSON.parse(JSON.stringify(doc)); next.pages[0].widgets = [{ id: 'u', type: 'entity', variant: 'tile', bindings: { primary: 'system:uptime' } }]; next.pages[0].layouts = { lg: [{ id: 'u', x: 0, y: 0, w: 2, h: 1 }], md: null, sm: null };
    page.update(next, next.pages[0]); assert.equal(note(), null); assert.ok(stage.querySelector('[data-hd-wid="u"]')); }],
  ['render (M4): preview writes only changed items, animates a move with a FLIP the next frame clears, honours skip and rows, and null restores the document layout', () => {
    const { page, stage, run } = setup(), items = page.items().map(i => Object.assign({}, i)), cell = id => stage.querySelector('[data-hd-wid="' + id + '"]');
    assert.equal(page.preview(items), 0); items[0].x += 1; items[1].y += 1; const a = items[0].id, b = items[1].id;
    assert.equal(page.preview(items, { skip: new Set([b]) }), 1); assert.equal(cell(a).style.getPropertyValue('--hd-x'), String(items[0].x)); assert.notEqual(cell(b).style.getPropertyValue('--hd-y'), String(items[1].y));
    assert.equal(page.preview(items, { animate: true, rows: 30 }), 1); assert.equal(cell(b).getAttribute('data-hd-flip'), 'from'); assert.ok(cell(b).style.getPropertyValue('--hd-e-fy').endsWith('px'));
    assert.equal(page.el.style.getPropertyValue('--hd-rows'), '30'); run(); assert.equal(cell(b).getAttribute('data-hd-flip'), null); assert.equal(cell(b).style.getPropertyValue('--hd-e-fy'), '');
    assert.equal(page.preview(null), 2, 'null restores the document layout'); assert.equal(page.preview(null), 0); }],
  ['render (G2): zoomFor clamps stage / preview width to 0.5..1 and floors at three decimals', () => { const { E } = setup(), z = E.render.zoomFor;
    assert.equal(E.render.ZOOM_MIN, 0.5); assert.equal(z(1180, 960), 1); assert.equal(z(960, 960), 1); assert.equal(z(580, 960), 0.604); assert.equal(z(720, 960), 0.75); assert.equal(z(400, 960), 0.5, 'never below 0.5: the stage scrolls'); assert.equal(z(300, 600), 0.5);
    assert.equal(z(0, 960), 1); assert.equal(z(-5, 960), 1); assert.equal(z(NaN, 960), 1); assert.ok(960 * z(581, 960) <= 581, 'the scaled page never overflows the stage'); }],
  ['render (G2): in design mode a forced preview wider than the stage is zoomed (--hd-e-zoom, data-hd-zoomed, metrics().zoom); view mode and wide stages stay at 1', () => {
    const { page, run, setWidth } = setup(null, null, { mode: 'design' }), pg = page.el; assert.equal(page.metrics().zoom, 1, 'unforced: no zoom'); assert.equal(pg.getAttribute('data-hd-zoomed'), null);
    setWidth(580); page.setBreakpoint('lg'); assert.equal(page.metrics().width, 960); assert.equal(page.metrics().zoom, 0.604); assert.equal(pg.style.getPropertyValue('--hd-e-zoom'), '0.604'); assert.equal(pg.getAttribute('data-hd-zoomed'), 'true'); assert.equal(pg.style.getPropertyValue('--hd-e-pw'), '960px', 'the metrics stay the preview width (WYSIWYG)');
    page.setBreakpoint('md'); assert.equal(page.metrics().width, 600); assert.equal(page.metrics().zoom, 0.966); setWidth(300); page.setBreakpoint('lg'); assert.equal(page.metrics().zoom, 0.5);
    setWidth(1180); page.setBreakpoint('lg'); assert.equal(page.metrics().zoom, 1); assert.equal(pg.style.getPropertyValue('--hd-e-zoom'), ''); assert.equal(pg.getAttribute('data-hd-zoomed'), null);
    setWidth(580); page.setBreakpoint('lg'); assert.equal(page.metrics().zoom, 0.604); page.setMode('view'); page.setBreakpoint(null); assert.equal(page.metrics().zoom, 1, 'view mode is never zoomed'); assert.equal(pg.getAttribute('data-hd-zoomed'), null); run(); }],
  ['render (M4): a forced breakpoint previews at its own width and columns; metrics, items, frame and order follow; design forces fit to scroll', () => {
    const { page, stage, run } = setup(null, null, { mode: 'design' }), pg = page.el, m0 = page.metrics();
    assert.equal(m0.bp, 'lg'); assert.equal(m0.cols, 12); assert.equal(m0.mode, 'scroll'); assert.equal(pg.style.getPropertyValue('--hd-e-pw'), '');
    page.setBreakpoint('sm'); const m = page.metrics(); assert.equal(pg.style.getPropertyValue('--hd-e-pw'), '390px'); assert.equal(pg.getAttribute('data-hd-preview'), 'sm'); assert.equal(pg.style.getPropertyValue('--hd-cols'), '4'); assert.equal(m.bp, 'sm'); assert.equal(m.cols, 4); assert.equal(m.width, 390);
    assert.equal(page.items().length, 6); assert.ok(page.items().every(i => i.x + i.w <= 4)); assert.ok(page.frame('cpu') === stage.querySelector('[data-hd-wid="cpu"]')); assert.equal(page.frame('nope'), null);
    eq(page.order(), page.items().slice().sort((a, b) => a.y - b.y || a.x - b.x || (a.id < b.id ? -1 : 1)).map(i => i.id));
    page.setBreakpoint('md'); assert.equal(page.metrics().width, 768); assert.equal(pg.style.getPropertyValue('--hd-e-pw'), '768px'); page.setBreakpoint('lg'); assert.equal(page.metrics().width, 1180);
    page.setBreakpoint(null); assert.equal(pg.getAttribute('data-hd-preview'), null); assert.equal(pg.style.getPropertyValue('--hd-e-pw'), ''); page.setFit('fit_screen'); assert.equal(page.metrics().mode, 'scroll', 'design ignores the fit setting');
    page.setMode('view'); assert.equal(page.metrics().mode, 'fit', 'view mode applies the override'); run(); noStyleAttr(pg); }],
  ['render (M4): destroy cancels the FLIP frame; update and preview after destroy do nothing', () => {
    const { page, stage, run } = setup(), items = page.items().map(i => Object.assign({}, i)); items[0].x += 1; page.preview(items, { animate: true }); page.destroy(); run();
    page.update(null, null); assert.equal(stage.querySelectorAll('.hd-widget').length, 0); assert.equal(page.preview(items), 0); }],
  ['render (M4): the more-info sheet api reads the live mode: after the page goes to design mode it refuses commands and capabilities', () => {
    const { apis, sheets, sent, page } = setupCmd(); assert.equal(apis.get('lt').moreInfo('primary'), true); const req = sheets[0];
    assert.equal(req.api.can('primary', 'turn_on'), true); req.api.command('primary', 'turn_on', {}); assert.equal(sent.length, 1);
    page.setMode('design'); assert.equal(req.api.can('primary', 'turn_on'), false); return Promise.resolve(req.api.command('primary', 'turn_on', {})).then(r => { assert.equal(r.status, 'refused'); assert.equal(sent.length, 1, 'nothing was sent in design mode'); }); }],
  ['render (M4 F1): contentBox subtracts the stage padding so metrics and zoom work on the inner width', () => { const { E } = setup(), cb = E.render.contentBox, node = { clientWidth: 600, clientHeight: 400 };
    eq(cb(node, { paddingLeft: '16px', paddingRight: '16px', paddingTop: '16px', paddingBottom: '16px' }), { width: 568, height: 368 }); eq(cb(node, { paddingLeft: '0px', paddingRight: '0px', paddingTop: '0px', paddingBottom: '0px' }), { width: 600, height: 400 });
    eq(cb(node, null), { width: 600, height: 400 }); eq(cb({ clientWidth: 10, clientHeight: 10 }, { paddingLeft: '8px', paddingRight: '8px', paddingTop: '9px', paddingBottom: '9px' }), { width: 0, height: 0 });
    assert.equal(E.render.zoomFor(cb({ clientWidth: 600, clientHeight: 0 }, { paddingLeft: '16px', paddingRight: '16px' }).width, 960), 0.591, 'the zoom fits the inner width'); }],
  ['render (D11): the page height is written only when it changes, the placement variables are registered as non-inherited, and update({ keepBox }) reuses the last stage measure', () => {
    let reads = 0; const { page, stage, doc, run } = setup(null, null, { measure: () => { reads += 1; return { width: 1180, height: 700 }; } }), items = page.items().map(i => Object.assign({}, i)), pg = page.el, writes = [];
    const set0 = pg.style.setProperty.bind(pg.style); pg.style.setProperty = (n, v) => { writes.push(n + '=' + v); set0(n, v); };
    items[0].x += 1; page.preview(items, { rows: 30 }); page.preview(items, { rows: 30 }); items[1].y += 1; page.preview(items, { rows: 30 });
    eq(writes, ['--hd-rows=30'], 'three previews with the same height write it once'); page.preview(items, { rows: 31 }); eq(writes, ['--hd-rows=30', '--hd-rows=31']); page.preview(null); assert.equal(writes.length, 3, 'null restores the document height once');
    const before = reads; page.update(doc, doc.pages[0], { keepBox: true }); assert.equal(reads, before, 'a keepBox update does not read the stage again'); page.update(doc, doc.pages[0]); assert.equal(reads, before + 1, 'a plain update measures'); run(); }],
  ['render (D11): the engine registers the placement and gesture variables as non-inherited (syntax *) and survives a duplicate or missing CSS.registerProperty', () => {
    const calls = [], names = () => calls.map(c => c.name), load = css => { const window = {}, context = { window, console, setTimeout, clearTimeout, AbortController, Intl, Map, Set, JSON, Math, Date, URL, CSS: css }; vm.createContext(context);
      for (const f of FILES) vm.runInContext(read('ui/js/homedash/' + f), context, { filename: f }); return window.HomeDashEngine; };
    load({ registerProperty: d => calls.push(d) }); eq(names(), ['--hd-x', '--hd-y', '--hd-w', '--hd-h', '--hd-rows', '--hd-e-fx', '--hd-e-fy', '--hd-e-dx', '--hd-e-dy']); assert.ok(calls.every(c => c.syntax === '*' && c.inherits === false));
    assert.ok(load({ registerProperty() { throw new Error('already registered'); } }).render, 'a duplicate registration is ignored'); assert.ok(load({}).render, 'no registerProperty: plain custom properties'); assert.ok(load(undefined).render); }]
];
