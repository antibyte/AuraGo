import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const source = await readFile(new URL('../ui/js/desktop/core/settings-runtime.js', import.meta.url), 'utf8');
const calls = [], events = [];
let release;
const gate = new Promise(resolve => { release = resolve; });
const runtime = {
    state: { bootstrap: { settings: {} } },
    desktopSettings: () => runtime.state.bootstrap.settings,
    desktopReadonly: () => false,
    desktopSessionEnded: false,
    settingValue: key => runtime.state.bootstrap.settings[key],
    t: key => key,
    desktopThemeById: id => id === 'fruity' ? { iconTheme: 'whitesur', wallpaper: 'aurora', soundTheme: 'crystal' } : null,
    api: async (_url, options) => {
        const updates = JSON.parse(options.body).settings;
        calls.push(updates);
        if (calls.length === 1) { await gate; throw new Error('save failed'); }
        return { settings: { ...runtime.state.bootstrap.settings, ...updates } };
    },
    applyDesktopSettings() {}, applySoundSettingsChange() {}, persistSessionSnapshot: async () => {},
    renderStartButtonIcon() {}, renderIcons() {}, renderWidgets() {}, renderStartApps() {},
    window: { dispatchEvent: event => events.push(event.detail) },
    CustomEvent: class { constructor(_type, options) { this.detail = options.detail; } }
};
vm.runInNewContext(source + '\nglobalThis.save = saveDesktopPreference;', runtime);
const first = runtime.save('sound.volume', '0.2');
const failed = assert.rejects(first, /save failed/);
const second = runtime.save('sound.volume', '0.7');
await Promise.resolve();
assert.equal(calls.length, 1, 'settings writes must be serialized');
release();
await failed;
await second;
assert.equal(runtime.state.bootstrap.settings['sound.volume'], '0.7');
assert.equal(events.length, 1, 'failed writes must not announce saved state');
await runtime.save('appearance.theme', 'fruity');
assert.deepEqual(calls.at(-1), {
    'appearance.theme': 'fruity', 'appearance.icon_theme': 'whitesur',
    'appearance.wallpaper': 'aurora', 'appearance.wallpaper_by_space': '{}',
    'appearance.theme_cursors': 'true', 'sound.theme': 'crystal'
});
assert.equal(runtime.state.bootstrap.settings['sound.volume'], '0.7', 'theme preserves volume');
runtime.desktopReadonly = () => true;
await assert.rejects(runtime.save('sound.volume', '0.1'), /action_unavailable/);
assert.equal(calls.length, 3, 'read-only settings do not start a write');
console.log('Desktop settings ordering, failure recovery and theme transaction passed');
