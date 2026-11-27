// Final-review fixes: a focused select widget keeps focus when it switches between the segmented and the list control (both directions), and the
// palette Devices tab pages the server by its own offset when localized system hits were put in front of the first page.
import assert from 'node:assert/strict'; import { eq } from './harness.mjs'; import { env } from './w1.test.mjs'; import { boot, fire, flush, fakeD, fakeStore, docOf } from './designer-kit.mjs';
const ID = 'ha:select.heater_mode';
const withOptions = (view, options) => Object.assign({}, view, { capabilities: view.capabilities.map(c => (c.name === 'select_option' ? Object.assign({}, c, { options }) : c)) });
const four = ['eco', 'comfort', 'boost', 'away'], three = ['eco', 'comfort', 'boost'];

const DE = { 'homedash.system.memory': 'Arbeitsspeicher' };
const tDe = k => (Object.prototype.hasOwnProperty.call(DE, k) ? DE[k] : k);
// A server that honours q (name or id, like the real search), limit and offset, and records every request.
function pagedApi(rows) {
    const log = [];
    return { log, entities: async p => { log.push(Object.assign({}, p)); const hit = rows.filter(e => !p.q || (e.name + ' ' + e.id).toLowerCase().includes(String(p.q).toLowerCase()));
        return { entities: hit.slice(p.offset || 0, (p.offset || 0) + p.limit), total: hit.length, sources: [{ source: 'ha' }, { source: 'system' }] }; },
        entity: async () => ({ entity: null }) };
}

export const tests = [
  ['select focus: list -> segmented (width grows) and segmented -> list (width shrinks) keep focus on the equivalent control', () => {
    const x = env({ [ID]: c => { c.capabilities.find(k => k.name === 'select_option').options = four; } }), doc = x.document;
    const w = x.mount('select', 'segmented', ID, { size: { w: 2, h: 1 } }); assert.ok(w.q('button.hd-pick'), 'four options at width 2 render the list');
    w.q('button.hd-pick').focus(); assert.equal(doc.activeElement, w.q('button.hd-pick'));
    w.h.resize({ w: 3, h: 1 }); assert.ok(w.q('.hd-seg')); assert.ok(doc.activeElement && doc.activeElement.closest && doc.activeElement.closest('.hd-seg'), 'focus is inside the segmented control');
    assert.equal(doc.activeElement.getAttribute('tabindex'), '0');
    w.h.resize({ w: 2, h: 1 }); assert.ok(w.q('button.hd-pick')); assert.equal(doc.activeElement, w.q('button.hd-pick'), 'focus returns to the picker'); }],
  ['select focus: the options dropping to three switch the focused list picker to the segmented control and keep focus', () => {
    const x = env({ [ID]: c => { c.capabilities.find(k => k.name === 'select_option').options = four; } }), doc = x.document, v = x.view(ID);
    const w = x.mount('select', 'segmented', ID, { size: { w: 2, h: 1 } }); w.q('button.hd-pick').focus();
    w.set(withOptions(v, three)); assert.ok(w.q('.hd-seg'), 'three options fit the segmented control at width 2'); assert.equal(w.q('button.hd-pick'), null);
    assert.ok(doc.activeElement && doc.activeElement.closest && doc.activeElement.closest('.hd-seg'), 'focus landed in the segmented control, not on the body');
    w.set(withOptions(v, four)); assert.ok(w.q('button.hd-pick')); assert.equal(doc.activeElement, w.q('button.hd-pick')); }],
  ['select focus: an unfocused widget never steals focus when it switches', () => {
    const x = env({ [ID]: c => { c.capabilities.find(k => k.name === 'select_option').options = four; } }), doc = x.document;
    const w = x.mount('select', 'segmented', ID, { size: { w: 2, h: 1 } }); const other = doc.createElement('button'); doc.body.appendChild(other); other.focus();
    w.h.resize({ w: 3, h: 1 }); assert.ok(w.q('.hd-seg')); assert.equal(doc.activeElement, other); }],
  ['palette (final review): localized system hits in front of the first page do not skip server rows on the next page', async () => {
    const k = boot('hd-palette.js'), rows = Array.from({ length: 250 }, (_, i) => ({ id: 'ha:sensor.a' + String(i).padStart(3, '0'), source: 'ha', kind: 'sensor', name: 'Arbeit ' + i, capabilities: [{ name: 'numeric' }], state: { value: i } }));
    rows.push({ id: 'system:memory', source: 'system', kind: 'sensor', name: 'Memory', capabilities: [{ name: 'numeric' }], state: { value: 40 } });
    const doc = docOf(k.E, k.cat, [], []), D = fakeD(k.E, k.cat, doc), api = pagedApi(rows), host = k.document.createElement('div'); k.document.body.appendChild(host);
    const pal = k.E.palette.create({ host, designer: D, t: tDe, lang: 'de', catalog: k.cat, store: fakeStore(), api }); pal.setTab('entities'); await flush(30);
    const search = host.querySelector('.hd-pal-search input'); search.value = 'Arbeit'; fire(search, 'input', {}); await flush(400);
    const ids = () => host.querySelectorAll('.hd-ent-row').map(r => r.getAttribute('data-entity'));
    assert.equal(ids()[0], 'system:memory', 'the localized system hit is in front'); assert.equal(ids().length, 201);
    const more = host.querySelector('.hd-ent-moreitem button'); assert.ok(more && !host.querySelector('.hd-ent-moreitem').hasAttribute('hidden'), 'a second page is offered');
    fire(more, 'click', {}); await flush(60);
    const paged = api.log.filter(r => r.q === 'Arbeit' && r.offset > 0); eq(paged.map(r => r.offset), [200], 'the next page asks the server for row 200, not 201');
    const got = new Set(ids()); assert.equal(got.size, 251, 'all 250 server rows and the system hit show'); for (let i = 0; i < 250; i += 1) assert.ok(got.has('ha:sensor.a' + String(i).padStart(3, '0')), 'row ' + i);
    assert.ok(host.querySelector('.hd-ent-moreitem').hasAttribute('hidden'), 'paging ends'); pal.destroy(); }],
  ['palette (final review): the reducer keeps a separate server offset and takes the extra count of an answer', () => {
    const { E } = boot('hd-palette.js'), M = E.palette.model, row = i => ({ id: 'ha:x.' + i, source: 'ha', kind: 'sensor', name: 'n' + i });
    let s = M.entReduce(M.entInit(), { type: 'filter', q: 'a' });
    s = M.entReduce(s, { type: 'loaded', seq: s.seq, answer: { entities: [{ id: 'system:memory', source: 'system', kind: 'sensor', name: 'Memory' }].concat(Array.from({ length: 200 }, (_, i) => row(i))), total: 251, extra: 1 } });
    assert.equal(s.items.length, 201); s = M.entReduce(s, { type: 'more' }); assert.equal(s.offset, 200); assert.equal(M.entRequest(s).offset, 200);
    s = M.entReduce(s, { type: 'loaded', seq: s.seq, answer: { entities: Array.from({ length: 50 }, (_, i) => row(200 + i)), total: 251 } }); assert.equal(s.items.length, 251); assert.equal(M.entReduce(s, { type: 'more' }), s); }]
];
