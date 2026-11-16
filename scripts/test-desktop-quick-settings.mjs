import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

class FakeElement {
    constructor(document, id = '') {
        this.document = document;
        this.id = id;
        this.dataset = {};
        this.attributes = {};
        this.listeners = {};
        this.parentNode = null;
        this.disabled = false;
        this.hidden = false;
        this.value = '';
        this.checked = false;
        this.type = '';
        this.textContent = '';
        this.innerHTML = '';
        this.isConnected = true;
        const classes = new Set();
        this.classList = {
            add: name => classes.add(name),
            remove: name => classes.delete(name),
            contains: name => classes.has(name),
            toggle: (name, force) => { const enabled = force === undefined ? !classes.has(name) : !!force; if (enabled) classes.add(name); else classes.delete(name); return enabled; }
        };
    }
    addEventListener(type, handler) { this.listeners[type] = handler; }
    setAttribute(key, value) { this.attributes[key] = value; }
    getAttribute(key) { return this.attributes[key]; }
    focus() { this.document.activeElement = this; }
    matches(selector) { return selector === '[data-quick-dnd]' && !!this.isDnd; }
    contains(element) { for (let current = element; current; current = current.parentNode) if (current === this) return true; return false; }
    getSetting(key) {
        if (!this.nodes) this.nodes = new Map();
        const selector = `[data-quick-setting="${key}"]`;
        if (!this.nodes.has(selector)) {
            const node = new FakeElement(this.document);
            node.parentNode = this;
            node.dataset.quickSetting = key;
            node.type = key === 'sound.enabled' || key === 'windows.animations' ? 'checkbox' : key === 'sound.volume' ? 'range' : 'select-one';
            this.nodes.set(selector, node);
        }
        return this.nodes.get(selector);
    }
    querySelector(selector) {
        if (!this.nodes) this.nodes = new Map();
        const action = selector.match(/^\[data-media-action="([^"]+)"\]$/);
        if (action) {
            const node = new FakeElement(this.document);
            node.parentNode = this;
            node.dataset.mediaAction = action[1];
            if (['play', 'pause'].includes(action[1])) node.dataset.mediaToggle = 'true';
            return node;
        }
        if (selector === '[data-media-toggle="true"]') {
            const node = new FakeElement(this.document);
            node.parentNode = this;
            node.dataset.mediaToggle = 'true';
            node.dataset.mediaAction = this.innerHTML.match(/data-media-action="(play|pause)"/)?.[1] || '';
            return node;
        }
        const setting = selector.match(/^\[data-quick-setting="([^"]+)"\]$/);
        if (setting) return this.getSetting(setting[1]);
        if (!this.nodes.has(selector)) {
            const node = new FakeElement(this.document);
            node.parentNode = this;
            if (selector === '[data-quick-scheme-slot]') node.dataset.quickSchemeSlot = 'true';
            if (selector === '[data-quick-dnd]') node.isDnd = true;
            if (selector === '[data-quick-media-content]') node.dataset.quickMediaContent = 'true';
            this.nodes.set(selector, node);
        }
        return this.nodes.get(selector);
    }
    querySelectorAll(selector) {
        if (selector !== '[data-quick-setting]') return [];
        const slot = this.nodes?.get('[data-quick-scheme-slot]');
        const html = `${this.innerHTML} ${slot?.innerHTML || ''}`;
        return [...new Set([...html.matchAll(/data-quick-setting="([^"]+)"/g)].map(match => slot?.innerHTML.includes(match[1]) ? slot.getSetting(match[1]) : this.getSetting(match[1])))];
    }
    closest(selector) { return selector.includes('data-quick') ? this : null; }
}

const elements = new Map();
const document = {
    activeElement: null,
    listeners: {},
    body: { appendChild(element) { element.isConnected = true; element.parentNode = this; elements.set(element.id, element); } },
    createElement() { return new FakeElement(document); },
    getElementById(id) { return elements.get(id) || null; },
    addEventListener(type, listener) { this.listeners[type] = listener; },
    removeEventListener(type) { delete this.listeners[type]; },
    dispatchEvent(event) { this.listeners[event.type]?.(event); }
};
const window = {
    listeners: {},
    addEventListener(type, listener) { this.listeners[type] = listener; },
    removeEventListener(type) { delete this.listeners[type]; },
    AuraDesktopNotificationCenter: {
        current: { dnd: false },
        policy() { return this.current; },
        setPolicy(value) { this.current = { ...this.current, ...value }; },
        subscribePolicy(listener) { listener(this.current); return () => {}; }
    }
};
let readonly = false;
let theme = { id: 'standard', schemeSetting: '' };
let failSave = false;
let deferSave = false;
let finishDeferredSave = null;
const settings = { 'sound.enabled': 'false', 'sound.volume': '0.6', 'windows.animations': 'true', 'appearance.fruity_mode': 'light', 'appearance.minimal_mode': 'dark' };
const saveCalls = [];
const context = vm.createContext({
    window,
    document,
    currentDesktopTheme: () => theme,
    settingValue: key => settings[key] ?? 'true',
    settingBool: key => !['false', '0'].includes(String(settings[key] ?? 'true')),
    desktopReadonly: () => readonly,
    t: key => key,
    esc: value => String(value).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;'),
    saveDesktopPreference: async (key, value) => {
        saveCalls.push([key, value]);
        if (deferSave) await new Promise(resolve => { finishDeferredSave = resolve; });
        if (failSave) throw new Error('write failed');
        settings[key] = String(value);
        return {};
    },
    placeTrayPopover() {}
});
const runtime = fileURLToPath(new URL('../ui/js/desktop/core/quick-settings-runtime.js', import.meta.url));
vm.runInContext(await readFile(runtime, 'utf8'), context, { filename: runtime });

const anchor = new FakeElement(document, 'quick-anchor');
document.activeElement = anchor;
theme = { id: 'standard', schemeSetting: '' };
context.toggleQuickSettingsPopup(anchor);
const panel = document.getElementById('vd-quick-settings');
assert.ok(panel, 'quick settings panel is created lazily');
assert.equal(panel.hidden, false);
assert.ok(!panel.querySelectorAll('[data-quick-setting]').some(control => control.dataset.quickSetting.includes('_mode')), 'themes without a scheme setting do not show an unsupported mode control');
assert.equal(panel.querySelectorAll('[data-quick-setting]').find(control => control.dataset.quickSetting === 'sound.enabled').disabled, false);

readonly = true;
document.dispatchEvent({ type: 'aurago:desktop-policy' });
assert.ok(panel.querySelectorAll('[data-quick-setting]').every(control => control.disabled), 'server-backed settings are disabled when the desktop is readonly');
const dnd = panel.querySelector('[data-quick-dnd]');
dnd.checked = true;
panel.listeners.change({ target: dnd });
assert.equal(window.AuraDesktopNotificationCenter.policy().dnd, true, 'browser-local DND remains usable in readonly mode');
assert.equal(saveCalls.length, 0, 'DND does not write backend settings');
context.closeQuickSettingsPopup(true);
assert.equal(panel.hidden, true);
assert.equal(document.activeElement, anchor, 'closing can restore focus to its anchor');
assert.equal(document.listeners['aurago:desktop-policy'], undefined, 'closing unsubscribes from live policy updates');
readonly = false;

context.toggleQuickSettingsPopup(anchor);
const volumeControl = panel.querySelector('[data-quick-setting="sound.volume"]');
volumeControl.value = '85';
deferSave = true;
panel.listeners.change({ target: volumeControl });
await Promise.resolve();
assert.equal(volumeControl.value, '85', 'pending writes keep the user’s requested slider value visible');
assert.equal(volumeControl.disabled, true, 'a preference cannot be written twice while pending');
assert.equal(panel.querySelector('[data-quick-status]').textContent, 'desktop.saving');
finishDeferredSave();
deferSave = false;
await new Promise(resolve => setImmediate(resolve));
assert.equal(volumeControl.value, '85');
assert.deepEqual(saveCalls.at(-1), ['sound.volume', '0.85']);
context.closeQuickSettingsPopup();

theme = { id: 'fruity', schemeSetting: 'appearance.fruity_mode' };
context.toggleQuickSettingsPopup(anchor);
theme = { id: 'minimal', schemeSetting: 'appearance.minimal_mode' };
let scheme = panel.querySelectorAll('[data-quick-setting]').find(control => control.dataset.quickSetting === 'appearance.fruity_mode');
assert.ok(scheme, 'Fruity exposes its supported scheme selector');
scheme.focus();
context.syncQuickSettings();
assert.equal(document.activeElement.dataset.quickSetting, 'appearance.minimal_mode', 'theme mode refresh preserves focus on the replacement setting');
scheme = panel.querySelectorAll('[data-quick-setting]').find(control => control.dataset.quickSetting === 'appearance.minimal_mode');
assert.ok(scheme, 'Minimal exposes its supported scheme selector');
theme = { id: 'standard', schemeSetting: '' };
context.syncQuickSettings();
assert.equal(document.activeElement.dataset.quickSetting, 'windows.animations', 'removing an unsupported mode control moves focus nearby');
assert.ok(!panel.querySelectorAll('[data-quick-setting]').some(control => control.dataset.quickSetting.includes('_mode')));
theme = { id: 'custom', schemeSetting: 'appearance.unsupported_mode' };
context.syncQuickSettings();
assert.ok(!panel.querySelectorAll('[data-quick-setting]').some(control => control.dataset.quickSetting === 'appearance.unsupported_mode'), 'unknown scheme settings are not written by the quick panel');
const beforeUnsupportedWrite = saveCalls.length;
await context.saveQuickSetting('appearance.unsupported_mode', 'dark');
assert.equal(saveCalls.length, beforeUnsupportedWrite, 'unsupported theme settings never reach the preference writer');
theme = { id: 'minimal', schemeSetting: 'appearance.minimal_mode' };
context.syncQuickSettings();

const animation = panel.querySelectorAll('[data-quick-setting]').find(control => control.dataset.quickSetting === 'windows.animations');
animation.checked = false;
failSave = true;
panel.listeners.change({ target: animation });
await new Promise(resolve => setImmediate(resolve));
const status = panel.querySelector('[data-quick-status]');
assert.equal(status.textContent, 'desktop.quick_settings_save_failed', 'failed preference writes remain visible');
assert.equal(status.classList.contains('is-error'), true);
assert.deepEqual(saveCalls.at(-1), ['windows.animations', 'false']);
failSave = false;
context.closeQuickSettingsPopup();

let mediaListener;
let finishPause = null;
const mediaDispatches = [];
let mediaSnapshot = { available: true, metadata: { title: '<Track>', artist: 'Artist' }, playbackState: 'playing', status: 'playing', actions: ['play', 'pause', 'nexttrack'] };
window.AuraDesktopMediaSession = {
    snapshot: () => mediaSnapshot,
    subscribe(listener) { mediaListener = listener; listener(this.snapshot()); return () => { mediaListener = null; }; },
    async dispatch(action) {
        mediaDispatches.push(action);
        if (action === 'pause') {
            mediaSnapshot = { ...mediaSnapshot, playbackState: 'paused', status: 'paused' };
            mediaListener(mediaSnapshot);
            await new Promise(resolve => { finishPause = resolve; });
            return true;
        }
        assert.equal(action, 'nexttrack');
        throw new Error('media unavailable');
    }
};
theme = { id: 'standard', schemeSetting: '' };
context.toggleQuickSettingsPopup(anchor);
const mediaHost = panel.querySelector('[data-quick-media-content]');
assert.ok(mediaHost.innerHTML.includes('&lt;Track&gt;'), 'media metadata is escaped');
assert.ok(mediaHost.innerHTML.includes('data-media-action="pause"'));
assert.ok(!mediaHost.innerHTML.includes('data-media-action="play"'), 'only the correct playback toggle is shown');
const pauseTarget = mediaHost.querySelector('[data-media-action="pause"]');
pauseTarget.focus();
panel.listeners.click({ target: pauseTarget });
assert.ok(mediaHost.innerHTML.includes('aria-busy="true"'), 'pending media commands mark the action group busy');
assert.equal(panel.querySelector('[data-quick-media-status]').textContent, 'desktop.loading');
assert.equal(document.activeElement.dataset.mediaAction, 'play', 'focus stays on the playback toggle while pending');
panel.listeners.click({ target: document.activeElement });
assert.deepEqual(mediaDispatches, ['pause'], 'a pending command cannot be dispatched twice');
finishPause();
await new Promise(resolve => setImmediate(resolve));
assert.ok(mediaHost.innerHTML.includes('data-media-action="play"'), 'the playback action follows the current state');
assert.equal(document.activeElement.dataset.mediaAction, 'play', 'play/pause refresh keeps focus on the logical toggle');
const mediaTarget = mediaHost.querySelector('[data-media-action="nexttrack"]');
mediaTarget.focus();
panel.listeners.click({ target: mediaTarget });
await new Promise(resolve => setImmediate(resolve));
assert.equal(panel.querySelector('[data-quick-media-status]').textContent, 'desktop.quick_settings_media_action_failed');
assert.equal(document.activeElement.dataset.mediaAction, 'nexttrack', 'media refresh preserves focus on the same supported action');
context.closeQuickSettingsPopup();

console.log('Desktop quick settings checks passed.');
