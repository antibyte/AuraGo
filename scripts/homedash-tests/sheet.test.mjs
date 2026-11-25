import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER } from './harness.mjs';
function setup() {
  const { E, document } = loadEngine(ENGINE_ORDER.concat(['hd-sheet.js']), { dom: true });
  const box = document.createElement('div'), other = document.createElement('main'), opener = document.createElement('button');
  box.appendChild(other); other.appendChild(opener); document.body.appendChild(box); opener.focus();
  return { E, document, box, other, opener, sheet: E.sheet.create(box) };
}
const key = (k, extra) => Object.assign({ type: 'keydown', key: k, shiftKey: false, preventDefault() {}, stopPropagation() {} }, extra);
export const tests = [
  ['sheet: open shows a labelled dialog, makes the rest inert, Escape resolves null and restores the focus', async () => {
    const { box, other, opener, sheet, document } = setup();
    const done = sheet.open({ title: 'Delete dashboard?', text: ['<b>System</b> goes away.'], actions: [{ id: 'cancel', label: 'Cancel' }, { id: 'ok', label: 'Delete', tone: 'danger' }] });
    const layer = box.querySelector('.hd-sheet-layer'), dlg = layer.querySelector('.hd-sheet');
    assert.equal(layer.hasAttribute('hidden'), false); assert.equal(dlg.getAttribute('role'), 'dialog'); assert.equal(dlg.getAttribute('aria-modal'), 'true');
    assert.equal(layer.querySelector('.hd-sheet-title').textContent, 'Delete dashboard?'); assert.equal(layer.querySelector('.hd-sheet-text').textContent, '<b>System</b> goes away.');
    assert.equal(layer.querySelector('b'), null); assert.equal(other.getAttribute('inert'), ''); assert.equal(document.activeElement.getAttribute('data-action'), 'cancel');
    layer.dispatchEvent(key('Escape')); assert.equal(await done, null);
    assert.equal(layer.hasAttribute('hidden'), true); assert.equal(other.hasAttribute('inert'), false); assert.equal(document.activeElement, opener); }],
  ['sheet: a button resolves its id, confirm() gives a boolean, Tab stays inside', async () => {
    const { sheet, box, document } = setup();
    const yes = sheet.confirm({ title: 'T', text: 'x', confirmLabel: 'Delete', cancelLabel: 'Cancel', tone: 'danger' });
    const layer = box.querySelector('.hd-sheet-layer'), [cancel, ok] = layer.querySelectorAll('.hd-sheet-btn');
    assert.equal(document.activeElement, cancel); assert.equal(ok.getAttribute('data-tone'), 'danger');
    layer.dispatchEvent(key('Tab')); assert.equal(document.activeElement, ok); layer.dispatchEvent(key('Tab')); assert.equal(document.activeElement, cancel);
    layer.dispatchEvent(key('Tab', { shiftKey: true })); assert.equal(document.activeElement, ok);
    layer.dispatchEvent({ type: 'click', target: ok }); assert.equal(await yes, true); }]
];
