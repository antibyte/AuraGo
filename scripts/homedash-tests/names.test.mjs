// G20: system entity names are localized by one helper (E.names, hd-core.js) on every surface; a search finds them by the localized and the raw name.
import assert from 'node:assert/strict'; import { eq } from './harness.mjs'; import { boot, fire, flush, fakeD, fakeStore, fakeApi, docOf } from './designer-kit.mjs';
const DE = { 'homedash.system.cpu': 'Prozessor', 'homedash.system.memory': 'Arbeitsspeicher', 'homedash.system.disk': 'Festplatte', 'homedash.system.net_rx': 'Netz Empfang', 'homedash.system.net_tx': 'Netz Senden', 'homedash.system.uptime': 'Laufzeit' };
const tDe = k => (Object.prototype.hasOwnProperty.call(DE, k) ? DE[k] : k);
const ENTS = [{ id: 'ha:light.kitchen', source: 'ha', kind: 'light', name: 'Kitchen', area: 'Kitchen', capabilities: [{ name: 'toggle' }], state: { value: 'on' } },
  { id: 'system:memory', source: 'system', kind: 'sensor', name: 'Memory', capabilities: [{ name: 'numeric' }], state: { value: 40 } }];
const VIEW = { 'system:memory': { id: 'system:memory', source: 'system', kind: 'sensor', name: 'Memory', value: 40, unit: '%', status: 'ok', available: true, icon_hint: '' } };
const WIDGETS = [{ id: 'g1', type: 'gauge', variant: 'arc', bindings: { primary: 'system:memory' }, options: {} }, { id: 's1', type: 'switch', variant: 'pill', bindings: {}, options: {} }];
const LAYOUT = [{ id: 'g1', x: 0, y: 0, w: 2, h: 2 }, { id: 's1', x: 2, y: 0, w: 2, h: 1 }];
function mountInsp(sel) { const k = boot('hd-inspector.js'), doc = docOf(k.E, k.cat, WIDGETS, LAYOUT), D = fakeD(k.E, k.cat, doc, { selection: sel }), store = fakeStore(VIEW), api = fakeApi(ENTS), host = k.document.createElement('div'); k.document.body.appendChild(host);
    const insp = k.E.inspector.create({ host, designer: D, t: tDe, lang: 'de', catalog: k.cat, store, api }); return Object.assign(k, { D, store, api, host, insp }); }
export const tests = [
  ['names (G20): display() localizes only the six system ids; every other view and missing name keep the server name', () => { const { E } = boot('hd-form.js'), N = E.names;
    assert.equal(N.display({ id: 'system:memory', name: 'Memory' }, tDe), 'Arbeitsspeicher'); assert.equal(N.display({ id: 'system:cpu', name: 'CPU' }, tDe), 'Prozessor');
    assert.equal(N.display({ id: 'ha:light.kitchen', name: 'Kitchen' }, tDe), 'Kitchen'); assert.equal(N.display({ id: 'system:other', name: 'Other' }, tDe), 'Other');
    assert.equal(N.display({ id: 'ha:x' }, tDe), ''); assert.equal(N.display(null, tDe), ''); assert.equal(N.display({ id: 'system:disk', name: 'Disk' }, k => k), 'homedash.system.disk', 'a missing key shows the key like every t()');
    eq(N.extraIds('Arbeit', tDe), ['system:memory']); eq(N.extraIds('arbeit', tDe), ['system:memory']); eq(N.extraIds('Memory', tDe), [], 'the raw name is found by the server itself'); eq(N.extraIds('', tDe), []); }],
  ['names (G20): the inspector header and the picker (bound text and list) show the German name; "Arbeit" and "Memory" both find system:memory', async () => { const m = mountInsp(['g1']);
    assert.equal(m.host.querySelector('.hd-insp-title').textContent, 'Arbeitsspeicher', 'inspector header'); m.insp.focusFirst();
    const input = m.host.querySelector('[data-hd-field="binding:primary"]'); assert.equal(input.value, 'Arbeitsspeicher', 'the bound field');
    for (const q of ['Arbeit', 'Memory']) { input.value = q; fire(input, 'input', {}); await flush(300);
        const opts = m.host.querySelectorAll('.hd-ep-opt'); eq(opts.map(o => o.getAttribute('data-entity')), ['system:memory'], q); assert.equal(opts[0].querySelector('.hd-ep-opt-name').textContent, 'Arbeitsspeicher', q); }
    assert.ok(m.api.log.some(r => r.q === 'system:memory'), 'a localized search also asks the server for the system id'); }],
  ['names (G20): the palette Devices tab shows the German row name and a search for "Arbeit" or "Memory" finds the row', async () => { const k = boot('hd-palette.js'), doc = docOf(k.E, k.cat, [], []), D = fakeD(k.E, k.cat, doc), store = fakeStore(VIEW), api = fakeApi(ENTS), host = k.document.createElement('div'); k.document.body.appendChild(host);
    const pal = k.E.palette.create({ host, designer: D, t: tDe, lang: 'de', catalog: k.cat, store, api }); pal.setTab('entities'); await flush(30);
    const names = () => host.querySelectorAll('.hd-ent-row').map(r => r.querySelector('.hd-ent-name').textContent); assert.ok(names().includes('Arbeitsspeicher'), names().join());
    const search = host.querySelector('.hd-pal-search input');
    for (const q of ['Arbeit', 'Memory']) { search.value = q; fire(search, 'input', {}); await flush(400); eq(names(), ['Arbeitsspeicher'], q); } }]
];
