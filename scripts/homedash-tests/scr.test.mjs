import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER } from './harness.mjs';
function hostWidget(widget, ref, history) {
  const { E, document } = loadEngine(ENGINE_ORDER, { dom: true }), parent = document.createElement('div'); parent.setAttribute('data-hd-family', 'glass'); document.body.appendChild(parent);
  const api = () => ({ entity: () => ref.current, can: () => false, command: () => Promise.reject(new Error('view only')), moreInfo() {}, history: () => Promise.resolve(history || { points: [], source: 'source' }), announce() {} });
  return { E, parent, inst: E.widgets.host({ widget, mode: 'view', size: { w: 2, h: 2 }, t: k => k, lang: 'en', api, parent }) };
}
const view = (value, extra) => Object.assign({ id: 'system:uptime', kind: 'sensor', name: 'Uptime', unit: 's', device_class: 'duration', icon_hint: 'clock', value, available: true, status: 'ok',
  readonly: false, stale: false, pending: false, attributes: { updated_at: '2026-11-25T10:00:00Z' } }, extra);
const flush = async () => { for (let i = 0; i < 10; i++) await Promise.resolve(); };
export const tests = [
  ['SCR-M2-1: the entity tile writes a duration for duration sensors and shows a known icon hint', () => {
    const ref = { current: view(7215) }, { E, parent, inst } = hostWidget({ id: 'uptime', type: 'entity', variant: 'tile', bindings: { primary: 'system:uptime' } }, ref);
    assert.equal(parent.querySelector('.hd-num-int').textContent, E.format.duration(7215, 'en')); assert.equal(parent.querySelector('.hd-num-unit').textContent, '');
    assert.ok(parent.querySelector('.hd-iconbadge [data-hd-icon="clock"]'));
    ref.current = view(12.5, { unit: '%', device_class: '', icon_hint: 'nope' }); inst.update();
    assert.equal(parent.querySelector('.hd-num-int').textContent + parent.querySelector('.hd-num-frac').textContent, '12.5');
    assert.equal(parent.querySelector('.hd-num-unit').textContent, '%'); assert.ok(parent.querySelector('.hd-iconbadge [data-hd-icon="entity"]')); }],
  ['SCR-M2-2: the sparkline extends with fresh live values after its history arrived', async () => {
    const T = Date.parse('2026-11-25T10:00:00Z'), hist = { points: [[T - 120000, 10], [T - 60000, 12], [T, 11]], source: 'source' };
    const ref = { current: view(11, { id: 'system:net_rx', unit: 'Mbit/s', device_class: 'data_rate' }) };
    const { inst, parent } = hostWidget({ id: 'net', type: 'sensor', variant: 'sparkline', bindings: { primary: 'system:net_rx' } }, ref, hist);
    await flush(); const line = parent.querySelector('.hd-spark-line'), before = line.getAttribute('d');
    ref.current = Object.assign({}, ref.current, { value: 30, attributes: { updated_at: '2026-11-25T10:00:10Z' } }); inst.update();
    const after = line.getAttribute('d'); assert.notEqual(after, before);
    ref.current = Object.assign({}, ref.current, { value: 99, stale: true, attributes: { updated_at: '2026-11-25T10:00:20Z' } }); inst.update(); assert.equal(line.getAttribute('d'), after);
    ref.current = Object.assign({}, ref.current, { value: 5, stale: false, attributes: { updated_at: '2026-11-25T09:59:00Z' } }); inst.update(); assert.equal(line.getAttribute('d'), after); }]
];
