import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const window = {};
vm.runInNewContext(fs.readFileSync(path.join(root, 'ui/js/desktop/icon-theme-resolver.js'), 'utf8'), { window });
const { path: resolve, themes } = window.AuraDesktopIcons;
const manifests = Object.fromEntries(themes.map(theme => [theme, JSON.parse(fs.readFileSync(path.join(root, 'ui/img', theme.replaceAll('_', '-'), 'manifest.json'), 'utf8'))]));
for (const theme of themes) {
  const settings = { 'appearance.icon_theme': theme };
  const manifest = manifests[theme];
  for (const key of Object.keys(manifest.icons)) assert.ok(resolve(manifests, settings, 'light', key), `${theme}:${key}`);
  assert.equal(resolve(manifests, settings, 'dark', 'sprite:folder'), '');
  assert.equal(resolve(manifests, settings, 'light', 'file-doc'), '/' + manifest.icons.documents);
  if (!manifest.actions) continue;
  for (const mode of ['light', 'dark']) {
    for (const key of Object.keys(manifest.actions[mode])) assert.equal(resolve(manifests, settings, mode, key, 'action'), '/' + manifest.actions[mode][key]);
    assert.equal(resolve(manifests, settings, mode, 'gallery-action-delete', 'action'), '/' + manifest.actions[mode].trash);
    assert.equal(resolve(manifests, settings, mode, 'arrow-left', 'action'), '/' + manifest.actions[mode]['chevron-left']);
  }
  const file = resolve(manifests, settings, 'light', 'documents', 'icon');
  for (const color of manifest.folder_colors) {
    settings['appearance.' + theme + '_folder_color'] = color;
    assert.equal(resolve(manifests, settings, 'light', 'documents', 'folder'), '/' + manifest.folder_variants[color].documents);
    assert.equal(resolve(manifests, settings, 'light', 'folder-documents', 'folder'), '/' + manifest.folder_variants[color].documents);
    assert.equal(resolve(manifests, settings, 'light', 'documents', 'icon'), file);
  }
  settings['appearance.' + theme + '_folder_color'] = 'invalid';
  assert.equal(resolve(manifests, settings, 'light', 'documents', 'folder'), '/' + manifest.folder_variants.standard.documents);
}
assert.equal(resolve({}, { 'appearance.icon_theme': 'candy' }, 'light', 'folder'), '');
for (const invalid of ['https://example.com/a.svg', '//example.com/a.png', 'img/../bad.svg', 'img/bad.svg?x=1', 'img/a.svg"']) {
  assert.equal(resolve({ candy: { icons: { file: invalid } } }, { 'appearance.icon_theme': 'candy' }, 'light', 'file'), '');
}
assert.equal(resolve(manifests, { 'appearance.icon_theme': 'tela' }, 'light', 'fluent:folder'), '/' + manifests.fluent.icons.folder);
console.log('Icon resolver: seven themes, full catalogs, roles, palettes, aliases and unsafe/missing paths verified.');
