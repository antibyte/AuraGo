import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import vm from 'node:vm';

const loadedScripts = [];
const loadedStyles = [];
const requested = [];
const context = {
    console: { warn() {}, error() {} },
    document: { documentElement: { lang: 'en' } },
    fetch: async url => {
        requested.push(String(url));
        return { ok: true, json: async () => ({ data: { synthStudio: {} } }) };
    },
    window: null
};
context.window = context;
context.AuraLazyAssets = {
    loadScript: async src => { loadedScripts.push(src); },
    loadStyle: async href => { loadedStyles.push(href); }
};
vm.createContext(context);
vm.runInContext(readFileSync('ui/js/desktop/core/module-loader.js', 'utf8'), context);

const modules = context.AuraDesktopModules;
const assets = modules.DESKTOP_APP_ASSETS['synth-studio'];
assert.ok(assets, 'Synth Studio is registered for lazy loading');
assert.ok(assets.styles.includes('/css/desktop-app-synth-studio.css'));
assert.deepEqual(Array.from(assets.scripts), [
    '/js/desktop/apps/writer-session.js',
    '/js/vendor/synth-studio/gm-data.js',
    '/js/desktop/apps/synth-studio-presets.js',
    '/js/vendor/synth-studio/tone-midi-2.0.28.bundle.js',
    '/js/desktop/apps/synth-studio-model.js',
    '/js/desktop/apps/synth-studio-audio.js',
    '/js/desktop/apps/synth-studio-midi.js',
    '/js/desktop/apps/synth-studio-storage.js',
    '/js/desktop/apps/synth-studio-editor.js',
    '/js/desktop/apps/synth-studio.js'
]);
for (const url of assets.scripts) {
    assert.ok(existsSync('ui' + url), 'registered app script exists: ' + url);
}
for (const url of assets.styles) {
    assert.ok(existsSync('ui' + url), 'registered app stylesheet exists: ' + url);
}
await modules.loadAppAssets('synth-studio');
assert.deepEqual(loadedScripts, Array.from(assets.scripts), 'scripts load in dependency order only when the app opens');
assert.equal(loadedStyles.length, assets.styles.length);
assert.ok(requested[0].includes('sections=synthStudio'), 'app translation section loads before app scripts');
assert.equal(modules.appAssetsReady('synth-studio'), true);

const routeFiles = [
    'ui/js/desktop/apps/editor-filemenu.js',
    'ui/js/desktop/core/menus-and-routing.js',
    'ui/js/desktop/core/desktop-window-file-drops.js',
    'ui/js/desktop/core/desktop-foundation.js'
].map(path => readFileSync(path, 'utf8')).join('\n');
assert.match(routeFiles, /\\\.aurasynth/);
assert.match(routeFiles, /'synth-studio'/);
assert.match(routeFiles, /aurasynth:\s*'synth-studio'/);
const locales = ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh'];
const english = JSON.parse(readFileSync('ui/lang/desktop/en.json', 'utf8'));
const translatedKeys = Object.keys(english).filter(key => key.startsWith('synthStudio.')).sort();
assert.equal(translatedKeys.length, 103, 'all current Synth Studio labels and pattern names are translated');
for (const locale of locales) {
    const dictionary = JSON.parse(readFileSync('ui/lang/desktop/' + locale + '.json', 'utf8'));
    assert.deepEqual(Object.keys(dictionary).filter(key => key.startsWith('synthStudio.')).sort(), translatedKeys, locale + ' has the complete Synth Studio section');
    assert.equal(dictionary['desktop.app_synth_studio'], 'Synth Studio', locale + ' preserves the app brand');
}
console.log('Synth Studio registration: lazy assets, dependency order, i18n section, and .aurasynth routes pass');
