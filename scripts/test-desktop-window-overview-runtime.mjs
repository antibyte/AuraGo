import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

function dataKey(attribute) {
    return attribute.slice(5).replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
}

function parseAttributes(source) {
    const attributes = {};
    for (const match of source.matchAll(/([\w-]+)(?:="([^"]*)")?/g)) {
        attributes[match[1]] = match[2] ?? '';
    }
    return attributes;
}

class FakeElement {
    constructor(tagName, ownerDocument) {
        this.tagName = String(tagName || 'div').toUpperCase();
        this.ownerDocument = ownerDocument;
        this.attributes = new Map();
        this.children = [];
        this.parentNode = null;
        this.listeners = new Map();
        this.style = {};
        this.hidden = false;
        this.disabled = false;
        this.dataset = {};
        this._innerHTML = '';
        this.textContent = '';
    }
    set className(value) { this.setAttribute('class', value); }
    get className() { return this.getAttribute('class') || ''; }
    get classList() {
        return {
            add: (...tokens) => this.setAttribute('class', [...new Set([...this.className.split(/\s+/).filter(Boolean), ...tokens])].join(' ')),
            remove: (...tokens) => this.setAttribute('class', this.className.split(/\s+/).filter(token => token && !tokens.includes(token)).join(' ')),
            contains: token => this.className.split(/\s+/).includes(token)
        };
    }
    get isConnected() {
        let node = this;
        while (node) {
            if (node === this.ownerDocument.body) return true;
            node = node.parentNode;
        }
        return false;
    }
    set innerHTML(value) {
        this._innerHTML = String(value || '');
        this.children = [];
        if (this.id !== 'vd-window-overview') return;
        const addButton = (attributes, parent = this) => {
            const button = new FakeElement('button', this.ownerDocument);
            for (const [name, value] of Object.entries(attributes)) button.setAttribute(name, value);
            parent.appendChild(button);
            return button;
        };
        const backdropMatch = this._innerHTML.match(/<div\b([^>]*data-overview-dismiss[^>]*)>/);
        if (backdropMatch) {
            const backdrop = new FakeElement('div', this.ownerDocument);
            for (const [name, value] of Object.entries(parseAttributes(backdropMatch[1]))) backdrop.setAttribute(name, value);
            this.appendChild(backdrop);
        }
        for (const match of this._innerHTML.matchAll(/<button\b([^>]*)>/g)) {
            const attrs = parseAttributes(match[1]);
            if (attrs['data-overview-scope'] || attrs['data-overview-dismiss'] !== undefined || attrs['data-overview-close-app'] !== undefined) addButton(attrs);
        }
        for (const match of this._innerHTML.matchAll(/<article\b([^>]*)>([\s\S]*?)<\/article>/g)) {
            const card = new FakeElement('article', this.ownerDocument);
            for (const [name, value] of Object.entries(parseAttributes(match[1]))) card.setAttribute(name, value);
            this.appendChild(card);
            for (const buttonMatch of match[2].matchAll(/<button\b([^>]*)>/g)) {
                const attrs = parseAttributes(buttonMatch[1]);
                if (attrs['data-window-activate'] !== undefined || attrs['data-window-close'] !== undefined) addButton(attrs, card);
            }
            for (const previewMatch of match[2].matchAll(/<div\b([^>]*data-overview-preview="[^"]+"[^>]*)>/g)) {
                const preview = new FakeElement('div', this.ownerDocument);
                for (const [name, value] of Object.entries(parseAttributes(previewMatch[1]))) preview.setAttribute(name, value);
                card.appendChild(preview);
            }
            for (const labelMatch of match[2].matchAll(/<span\b([^>]*data-overview-workspace-label="[^"]+"[^>]*)>/g)) {
                const label = new FakeElement('span', this.ownerDocument);
                for (const [name, value] of Object.entries(parseAttributes(labelMatch[1]))) label.setAttribute(name, value);
                card.appendChild(label);
            }
        }
    }
    get innerHTML() { return this._innerHTML; }
    setAttribute(name, value) {
        this.attributes.set(name, String(value));
        if (name === 'id') this.id = String(value);
        else if (name === 'hidden') this.hidden = true;
        else if (name === 'disabled') this.disabled = true;
        else if (name.startsWith('data-')) this.dataset[dataKey(name)] = String(value);
    }
    getAttribute(name) { return this.attributes.has(name) ? this.attributes.get(name) : null; }
    hasAttribute(name) { return this.attributes.has(name); }
    removeAttribute(name) {
        this.attributes.delete(name);
        if (name === 'hidden') this.hidden = false;
        if (name === 'disabled') this.disabled = false;
    }
    appendChild(child) { child.parentNode = this; this.children.push(child); return child; }
    remove() {
        if (this.parentNode) this.parentNode.children = this.parentNode.children.filter(child => child !== this);
        this.parentNode = null;
    }
    addEventListener(type, listener) {
        const listeners = this.listeners.get(type) || [];
        listeners.push(listener);
        this.listeners.set(type, listeners);
    }
    dispatch(type, event = {}) {
        const complete = { target: this, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.propagationStopped = true; }, ...event };
        let node = this;
        while (node) {
            complete.currentTarget = node;
            for (const listener of node.listeners.get(type) || []) listener(complete);
            if (complete.propagationStopped) break;
            node = node.parentNode;
        }
        return complete;
    }
    focus() {
        this.ownerDocument.activeElement = this;
        this.dispatch('focusin');
    }
    scrollIntoView(options) { this.lastScrollBlock = options.block; }
    getClientRects() { return this.hidden ? [] : [{}]; }
    getBoundingClientRect() { return { top: 0, left: 0, right: 200, bottom: 90 }; }
    closest(selector) {
        let node = this;
        while (node) {
            if (matchesSelector(node, selector)) return node;
            node = node.parentNode;
        }
        return null;
    }
    querySelectorAll(selector) {
        const result = [];
        const visit = node => {
            for (const child of node.children) {
                if (matchesSelector(child, selector)) result.push(child);
                visit(child);
            }
        };
        visit(this);
        return result;
    }
    querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
}

function matchesSimple(node, selector) {
    selector = selector.trim();
    if (!selector) return false;
    const notDisabled = selector.includes(':not([disabled])');
    const notTabindexMinusOne = selector.includes(':not([tabindex="-1"])');
    selector = selector.replace(/:not\([^)]*\)/g, '');
    const tag = selector.match(/^[a-z]+/i)?.[0];
    if (tag && node.tagName.toLowerCase() !== tag.toLowerCase()) return false;
    for (const match of selector.matchAll(/\[([^\]=]+)(?:="([^"]*)")?\]/g)) {
        const [, name, value] = match;
        if (!node.hasAttribute(name)) return false;
        if (value !== undefined && node.getAttribute(name) !== value) return false;
    }
    if (notDisabled && node.disabled) return false;
    if (notTabindexMinusOne && node.getAttribute('tabindex') === '-1') return false;
    return !!(tag || selector.includes('['));
}

function matchesSelector(node, selector) {
    return selector.split(',').some(group => {
        const parts = group.trim().split(/\s+/);
        if (parts.length === 1) return matchesSimple(node, parts[0]);
        if (!matchesSimple(node, parts[parts.length - 1])) return false;
        let ancestor = node.parentNode;
        for (let index = parts.length - 2; index >= 0; index--) {
            while (ancestor && !matchesSimple(ancestor, parts[index])) ancestor = ancestor.parentNode;
            if (!ancestor) return false;
            ancestor = ancestor.parentNode;
        }
        return true;
    });
}

class FakeDocument {
    constructor() {
        this.body = new FakeElement('body', this);
        this.activeElement = this.body;
    }
    createElement(tag) { return new FakeElement(tag, this); }
    getElementById(id) {
        if (this.body.id === id) return this.body;
        const visit = node => {
            for (const child of node.children) {
                if (child.id === id) return child;
                const nested = visit(child);
                if (nested) return nested;
            }
            return null;
        };
        return visit(this.body);
    }
    querySelectorAll(selector) { return this.body.querySelectorAll(selector); }
}

const document = new FakeDocument();
const observers = [];
class FakeIntersectionObserver {
    constructor(callback) { this.callback = callback; this.targets = new Set(); observers.push(this); }
    observe(target) { this.targets.add(target); }
    unobserve(target) { this.targets.delete(target); }
    disconnect() { this.targets.clear(); }
    reveal(target) { this.callback([{ target, isIntersecting: true }]); }
}
const window = { innerHeight: 800, innerWidth: 1000, IntersectionObserver: FakeIntersectionObserver };
const source = await readFile(new URL('../ui/js/desktop/core/window-overview-runtime.js', import.meta.url), 'utf8');
vm.runInContext(source, vm.createContext({ window, document, Promise, Set, Map, JSON, String, Array, Object, Number }));
const overview = window.AuraDesktopWindowOverview;

const opener = document.createElement('button');
document.body.appendChild(opener);
const wins = [
    { id: 'one', appId: 'files', title: 'Files home', spaceId: '1', element: { style: { display: '' } } },
    { id: 'two', appId: 'files', title: 'Files archive', spaceId: '1', minimizing: true, element: { style: { display: '' } } },
    { id: 'three', appId: 'files', title: 'Files other space', spaceId: '2', element: { style: { display: '' } } },
    { id: 'four', appId: 'writer', title: 'Draft', spaceId: '1', element: { style: { display: 'none' } } }
];
const secondWindow = wins[1];
let activeId = 'one';
let snapCandidates = [];
let previewCalls = [];
let activated = [];
let closed = [];
let batchIds = [];
let closeResult = 'cancelled';
let closeGuard = null;
let snapChooser;
const t = (key, variables = {}) => Object.entries(variables).reduce((value, [name, replacement]) => value.replaceAll(`{{${name}}}`, String(replacement)), key);
overview.configure({
    getWindows: options => {
        if (options.mode === 'snap-partner') return snapCandidates;
        const candidates = options.scope === 'all' ? wins : wins.filter(win => win.spaceId === '1');
        return options.appId ? candidates.filter(win => win.appId === options.appId) : candidates;
    },
    getActiveWindowId: () => activeId,
    spaceName: win => `Workspace ${win.spaceId}`,
    appName: appId => ({ files: 'Files', writer: 'Writer' })[appId] || appId,
    previewMarkup: win => { previewCalls.push(win.id); return `<span>${win.title} preview</span>`; },
    activateWindow: id => { activated.push(id); return true; },
    chooseSnapPartner: (...args) => snapChooser(...args),
    closeWindow: async id => {
        closed.push(id);
        const target = wins.find(win => win.id === id);
        if (closeGuard) {
            target.checkingClose = true;
            try {
                const result = await closeGuard(id);
                if (result === 'accepted') wins.splice(wins.findIndex(win => win.id === id), 1);
                return result;
            } finally { target.checkingClose = false; }
        }
        if (closeResult === 'accepted') wins.splice(wins.findIndex(win => win.id === id), 1);
        return closeResult;
    },
    closeBatch: async ids => { batchIds = ids; return { status: 'cancelled', closed: [], skipped: [] }; },
    t,
    mountDialog: dialog => { document.body.appendChild(dialog); return () => dialog.remove(); }
});

assert.equal(overview.open({ mode: 'switcher', opener }), true);
let dialog = document.getElementById('vd-window-overview');
assert.equal(dialog.dataset.overviewMode, 'switcher');
assert.equal(dialog.dataset.overviewScopeValue, 'current');
assert.equal(dialog.querySelectorAll('[data-overview-window]').length, 3, 'current view includes its minimized window and excludes other spaces');
assert.equal(dialog.querySelector('[data-overview-window="two"]').dataset.overviewMinimized, 'true');
assert.equal(previewCalls.length, 0, 'preview projection is deferred until a card becomes visible');
const firstPreview = dialog.querySelector('[data-overview-preview="one"]');
observers.at(-1).reveal(firstPreview);
assert.deepEqual(previewCalls, ['one']);
overview.cycle(false);
assert.equal(dialog.querySelector('[data-window-activate="two"]').getAttribute('aria-pressed'), 'true');
assert.equal(dialog.querySelector('[data-window-activate="one"]').getAttribute('aria-pressed'), 'false');
assert.equal(dialog.querySelector('[data-overview-window="two"]').lastScrollBlock, 'nearest');
assert.equal(dialog.querySelector('[data-overview-preview="one"]'), firstPreview, 'cycling preserves the mounted preview');
overview.finish({ activate: true });
assert.deepEqual(activated, ['two']);
assert.equal(overview.isOpen(), false);

assert.equal(overview.open({ mode: 'app-group', appId: 'files', scope: 'current', opener }), true);
dialog = document.getElementById('vd-window-overview');
const allScopeButton = dialog.querySelector('[data-overview-scope="all"]');
assert.equal(allScopeButton.dataset.overviewScope, 'all');
allScopeButton.dispatch('click');
assert.equal(dialog.dataset.overviewScopeValue, 'all');
assert.equal(dialog.querySelectorAll('[data-overview-window]').length, 3, 'all-workspace scope includes every Files window');
assert.ok(dialog.innerHTML.includes('Workspace 2'), 'all-workspace cards include their workspace name');
assert.ok(dialog.querySelector('[data-overview-workspace-label="three"]'), 'workspace names have a stable integration selector');
dialog.querySelector('[data-overview-close-app]').dispatch('click');
assert.equal(dialog.querySelector('[data-overview-close-app]').disabled, true, 'close-all disables while its guarded batch is running');
await new Promise(resolve => setImmediate(resolve));
assert.deepEqual(batchIds, ['one', 'two', 'three'], 'close-app snapshots all app windows across every workspace');
assert.equal(overview.isOpen('app-group'), true, 'cancelled batch close keeps the chooser open');

dialog.querySelector('[data-window-close="one"]').dispatch('click');
await new Promise(resolve => setImmediate(resolve));
assert.ok(closed.includes('one'));
assert.equal(overview.isOpen('app-group'), true, 'a cancelled single close keeps the card list available');
assert.equal(document.activeElement.getAttribute('data-window-close'), 'one', 'cancelled close returns focus to its own card action');
closeResult = 'accepted';
dialog.querySelector('[data-window-close="two"]').dispatch('click');
await new Promise(resolve => setImmediate(resolve));
assert.equal(dialog.querySelector('[data-overview-window="two"]'), null, 'accepted close removes the closed window from the refreshed list');
assert.notEqual(document.activeElement.closest('[data-overview-window]')?.dataset.overviewWindow, 'two', 'accepted close moves focus to a surviving neighbor');
wins.splice(1, 0, secondWindow);
const ctrlTab = dialog.dispatch('keydown', { key: 'Tab', ctrlKey: true });
assert.equal(!!ctrlTab.defaultPrevented, false, 'the overview leaves Ctrl+Tab handling to the shell');
dialog.dispatch('keydown', { key: 'Escape' });
assert.equal(overview.isOpen(), false);
assert.equal(document.activeElement, opener, 'dismiss returns focus to the opener');

let settleGuard;
closeGuard = () => new Promise(resolve => { settleGuard = resolve; });
overview.open({ mode: 'app-group', appId: 'files', scope: 'all', opener });
const oldDialog = document.getElementById('vd-window-overview');
oldDialog.querySelector('[data-window-close="one"]').dispatch('click');
assert.equal(wins.find(win => win.id === 'one').checkingClose, true);
overview.open({ mode: 'switcher', opener });
dialog = document.getElementById('vd-window-overview');
assert.ok(dialog.querySelector('[data-overview-window="one"]'), 'a pending guard does not hide an existing window from a reopened overview');
assert.equal(dialog.querySelector('[data-window-close="one"]').disabled, false, 'the root pending-close guard owns duplicate-close handling');
settleGuard('cancelled');
await new Promise(resolve => setImmediate(resolve));
closeGuard = null;
assert.equal(overview.isOpen('switcher'), true);
assert.equal(document.activeElement.getAttribute('data-window-activate'), 'one', 'an old close guard never restores focus over the reopened dialog');
overview.finish({ activate: false });

overview.open({ mode: 'switcher', opener });
dialog = document.getElementById('vd-window-overview');
const ordinaryFocus = dialog.querySelector('[data-window-activate="one"]');
ordinaryFocus.focus();
const originalTitle = wins[0].title;
wins[0].title = 'Files home refreshed';
overview.refresh();
assert.equal(document.activeElement.getAttribute('data-window-activate'), 'one', 'an ordinary overview refresh restores its logical focus');
const dismissButton = dialog.querySelector('button[data-overview-dismiss]');
dismissButton.focus();
wins[0].title = 'Files home refreshed while dismiss is focused';
overview.refresh();
assert.equal(document.activeElement.tagName, 'BUTTON', 'refresh keeps focus on a real dismiss button, not the backdrop marker');
assert.ok(document.activeElement.hasAttribute('data-overview-dismiss'));
wins[0].title = originalTitle;
overview.refresh();
const foregroundModal = document.createElement('section');
foregroundModal.setAttribute('aria-modal', 'true');
document.body.appendChild(foregroundModal);
const foregroundControl = document.createElement('button');
foregroundModal.appendChild(foregroundControl);
foregroundControl.focus();
wins[0].title = 'Files home changed under another modal';
overview.refresh();
assert.equal(document.activeElement, foregroundControl, 'a visible foreground modal keeps focus during taskbar-driven overview refresh');
overview.finish({ activate: false });
assert.equal(document.activeElement, foregroundControl, 'closing the overview does not return focus behind another visible modal');
foregroundModal.remove();
wins[0].title = originalTitle;

snapCandidates = [wins[1]];
let finishSnap;
snapChooser = (...args) => {
    assert.deepEqual(args, ['source', 'two', 'right-half']);
    snapCandidates = [];
    overview.refresh();
    return new Promise(resolve => { finishSnap = resolve; });
};
assert.equal(overview.open({ mode: 'snap-partner', sourceId: 'source', partnerZone: 'right-half', opener }), true);
dialog = document.getElementById('vd-window-overview');
dialog.querySelector('[data-window-activate="two"]').dispatch('click');
assert.equal(overview.isOpen('snap-partner'), true, 'taskbar refresh during a snap mutation cannot dismiss its chooser early');
finishSnap(true);
await new Promise(resolve => setImmediate(resolve));
assert.equal(overview.isOpen(), false);
assert.deepEqual(activated, ['two', 'two'], 'successful snap selection activates the mutated partner after dismissal');

snapCandidates = [wins[0]];
let finishStaleSnap;
snapChooser = () => new Promise(resolve => { finishStaleSnap = resolve; });
overview.open({ mode: 'snap-partner', sourceId: 'source', partnerZone: 'right-half', opener });
dialog = document.getElementById('vd-window-overview');
dialog.querySelector('[data-window-activate="one"]').dispatch('click');
overview.finish({ activate: false });
overview.open({ mode: 'switcher', opener });
finishStaleSnap(true);
await new Promise(resolve => setImmediate(resolve));
assert.equal(overview.isOpen('switcher'), true, 'a stale snap response cannot close a reopened overview');
assert.deepEqual(activated, ['two', 'two'], 'a dismissed snap request cannot steal focus by activating late');
overview.finish({ activate: false });

process.stdout.write('Desktop window overview: scopes, minimized entries, lazy previews, close guards, keyboard handling, and stale snap responses verified.\n');
