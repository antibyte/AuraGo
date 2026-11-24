import fs from 'node:fs'; import path from 'node:path'; import vm from 'node:vm'; import assert from 'node:assert/strict';
import { createDocument, allElements } from './dom.mjs';
export { allElements };
export const root = path.resolve(import.meta.dirname, '..', '..');
export const ENGINE_DIR = 'ui/js/homedash';
export const ENGINE_ORDER = ['hd-core.js', 'hd-format.js', 'hd-colour.js', 'hd-grid.js', 'hd-layout.js', 'hd-doc.js', 'hd-history.js', 'hd-icons.js', 'hd-geometry.js', 'hd-prim-displays.js', 'hd-prim-controls.js', 'hd-widgets.js', 'widgets/hd-w-misc.js'];
export const read = rel => fs.readFileSync(path.join(root, rel), 'utf8');
export const plain = v => (v === undefined ? undefined : JSON.parse(JSON.stringify(v)));
export const eq = (a, e, m) => assert.deepStrictEqual(plain(a), plain(e), m);
export const fc = (...cps) => String.fromCodePoint(...cps);
export function loadEngine(files, { dom = false } = {}) {
  const window = {};
  const context = { window, console, setTimeout, clearTimeout, AbortController, Intl, Map, Set, JSON, Math, Date, URL };
  if (dom) { context.document = createDocument(); window.document = context.document; }
  vm.createContext(context);
  for (const f of files) vm.runInContext(read(`${ENGINE_DIR}/${f}`), context, { filename: f });
  return { E: window.HomeDashEngine, window, document: context.document };
}
export function noStyleAttr(el) { for (const n of allElements(el)) assert.equal(n.getAttribute('style'), null, `style attribute on ${n.localName}`); }
