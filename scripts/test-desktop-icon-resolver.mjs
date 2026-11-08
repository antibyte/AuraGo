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
const miniManifest = JSON.parse(fs.readFileSync(path.join(root, 'ui/img/desktop-mini/manifest.json'), 'utf8'));
const symbols = fs.readFileSync(path.join(root, 'ui/img/desktop-mini/symbols.svg'), 'utf8');
const miniContext = {
  state: { miniIconManifest: miniManifest, iconThemeManifests: manifests },
  normalizeIconName: window.AuraDesktopIcons.normalize,
  esc: value => String(value),
  versionedIconAssetPath: value => value,
  settingValue: () => 'papirus',
  themeIconFamily: () => 'standard',
  iconUrlStyle: value => value,
  document: { body: { style: { setProperty() {} } } }
};
vm.createContext(miniContext);
vm.runInContext(fs.readFileSync(path.join(root, 'ui/js/desktop/core/mini-icons-runtime.js'), 'utf8'), miniContext);
const { symbols: symbolicNames, aliases } = vm.runInContext('({symbols:[...MINI_SYMBOLS],aliases:MINI_ALIASES})', miniContext);
for (const name of symbolicNames) assert.ok(symbols.includes(`id="${name}"`), `Missing SVG symbol ${name}`);
for (const [alias, target] of Object.entries(aliases)) assert.ok(miniManifest.icons[target] || symbolicNames.includes(target), `Missing mini alias ${alias} -> ${target}`);
for (const key of ['film', 'flip', 'next', 'previous', 'shuffle', 'repeat', 'history', 'flask', 'sidebar', 'zoom-fit', 'rotate']) {
  assert.ok(miniContext.miniIconMarkup(key, 'vd-window-menu-papirus-icon', 16), `App action ${key} has no icon`);
}
const verifyFiles = value => {
  if (typeof value === 'string' && value.startsWith('img/')) assert.ok(fs.existsSync(path.join(root, 'ui', value)), `Missing icon file ${value}`);
  else if (value && typeof value === 'object') Object.values(value).forEach(verifyFiles);
};
Object.values(manifests).forEach(verifyFiles);
verifyFiles(miniManifest);
const appsDir = path.join(root, 'ui/js/desktop/apps');
const easyDragSource = fs.readFileSync(path.join(appsDir, 'easydrag-core.js'), 'utf8');
const easyDragIcons = vm.runInNewContext(easyDragSource.slice(easyDragSource.indexOf('    const ICONS ='), easyDragSource.indexOf('    function icon(')) + '\nICONS');
for (const file of fs.readdirSync(appsDir).filter(file => /^easydrag.*\.js$/.test(file))) {
  for (const match of fs.readFileSync(path.join(appsDir, file), 'utf8').matchAll(/\b(?:core|ED\.core)\.icon\(\s*['"]([^'"]+)['"]/g)) {
    assert.ok(easyDragIcons[match[1]], `${file}: missing EasyDrag icon ${match[1]}`);
  }
}
console.log('Icon resolver: seven themes, full catalogs, roles, palettes, aliases and unsafe/missing paths verified.');
