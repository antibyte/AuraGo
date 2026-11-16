import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const runtime = await readFile(path.join(root, 'ui/js/desktop/core/accessibility-runtime.js'), 'utf8');
const css = await readFile(path.join(root, 'ui/css/desktop-accessibility.css'), 'utf8');
const style = new Map();
const body = { dataset: {}, style: { setProperty: (key, value) => style.set(key, value) } };
const context = { window: {}, document: { body }, Set, String, Number, Object };
vm.createContext(context);
vm.runInContext(runtime, context, { filename: 'accessibility-runtime.js' });
const accessibility = context.window.AuraDesktopAccessibility;

assert.deepEqual(JSON.parse(JSON.stringify(accessibility.normalize({ 'accessibility.text_size': '125', 'accessibility.high_contrast': true }))), {
  textSize: '125', highContrast: true
});
assert.deepEqual(JSON.parse(JSON.stringify(accessibility.normalize({ textSize: '175', highContrast: 'false' }))), {
  textSize: '100', highContrast: false
});
assert.deepEqual(JSON.parse(JSON.stringify(accessibility.apply({ 'accessibility.text_size': 150, 'accessibility.high_contrast': 'true' }))), {
  textSize: '150', highContrast: true
});
assert.deepEqual(body.dataset, { textSize: '150', highContrast: 'true' });
assert.equal(style.get('--vd-accessibility-text-scale'), '1.5');

for (const required of [
  'data-text-size="125"', 'data-text-size="150"', 'data-high-contrast="true"',
  ':focus-visible', 'forced-colors: active', 'prefers-reduced-motion: reduce',
  'span, div, summary', '.vd-notification-center', '.vd-settings-app',
  '.vd-file-dialog-backdrop', '.desktop-body > .vd-modal-backdrop',
  'max-width: calc(100vw - 16px)', 'overflow: auto'
]) {
  assert.ok(css.includes(required), `missing accessibility contract: ${required}`);
}
assert.ok(!/\btransform\s*:/.test(css), 'text accessibility must not scale the desktop with a transform');
assert.ok(!css.includes('.vd-window-content'), 'accessibility shell rules must not be applied to embedded app content');
assert.ok(css.includes('.vd-window-titlebar'), 'window chrome remains in the scoped shell contract');

console.log('Desktop accessibility settings/styles checks passed.');
