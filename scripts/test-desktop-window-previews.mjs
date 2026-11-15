import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

class PreviewNode {
    constructor(tag, attributes = {}, children = []) {
        this.tagName = String(tag).toUpperCase();
        this.values = new Map(Object.entries(attributes));
        this.children = children;
        this.parentNode = null;
        this.offsetHeight = 240;
        this.style = { setProperty() {} };
        children.forEach(child => { child.parentNode = this; });
    }
    get attributes() { return Array.from(this.values, ([name, value]) => ({ name, value })); }
    get innerHTML() { return this.children.map(child => child.tagName === '#TEXT' ? child.text : child.outerHTML).join(''); }
    get outerHTML() {
        if (this.tagName === '#TEXT') return this.text;
        const attrs = Array.from(this.values, ([name, value]) => ` ${name}="${String(value)}"`).join('');
        return `<${this.tagName.toLowerCase()}${attrs}>${this.innerHTML}</${this.tagName.toLowerCase()}>`;
    }
    setAttribute(name, value) { this.values.set(name, String(value)); }
    removeAttribute(name) { this.values.delete(name); }
    matches(selector) {
        if (this.tagName === '#TEXT') return false;
        return selector.split(',').some(raw => {
            const part = raw.trim();
            if (part === '*') return true;
            if (part.startsWith('.')) return String(this.values.get('class') || '').split(/\s+/).includes(part.slice(1));
            const attr = part.match(/^\[([^=\]]+)(?:=["']?([^\]"']+)["']?)?\]$/);
            if (attr) return this.values.has(attr[1]) && (attr[2] === undefined || this.values.get(attr[1]) === attr[2]);
            return this.tagName === part.toUpperCase();
        });
    }
    querySelectorAll(selector) {
        const matches = [];
        const visit = node => node.children.forEach(child => {
            if (child.matches(selector)) matches.push(child);
            visit(child);
        });
        visit(this);
        return matches;
    }
    querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
    cloneNode(deep) {
        const clone = new PreviewNode(this.tagName, Object.fromEntries(this.values));
        if (this.tagName === '#TEXT') clone.text = this.text;
        if (deep) clone.children = this.children.map(child => child.cloneNode(true));
        clone.children.forEach(child => { child.parentNode = clone; });
        clone.offsetHeight = this.offsetHeight;
        return clone;
    }
    remove() {
        if (this.parentNode) this.parentNode.children = this.parentNode.children.filter(child => child !== this);
    }
}

const text = value => Object.assign(new PreviewNode('#text'), { text: value });
const makeContent = children => new PreviewNode('div', { class: 'vd-window-content' }, children);
const source = await readFile(new URL('../ui/js/desktop/core/taskbar-thumbnails-runtime.js', import.meta.url), 'utf8');
const context = vm.createContext({
    window: {},
    appById: id => ({ id }),
    iconForApp: () => 'app-icon',
    iconGlyph: () => 'A',
    iconMarkup: () => '<span class="icon">A</span>',
    esc: value => String(value).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]),
    t: key => key
});
vm.runInContext(source, context);

function preview(appId, content) {
    context.win = {
        appId,
        title: 'Private preview title',
        element: { offsetWidth: 640, querySelector: () => content }
    };
    return vm.runInContext('windowPreviewMarkup(win, {})', context);
}

for (const [appId, content] of [
    ['mailer', makeContent([text('private message body')])],
    ['calculator', makeContent([new PreviewNode('canvas')])],
    ['calculator', makeContent([new PreviewNode('iframe')])],
    ['calculator', makeContent([new PreviewNode('input', { type: 'password', value: 'secret' })])],
    ['calculator', makeContent([new PreviewNode('p', { contenteditable: 'true' }, [text('draft')])])],
    ['calculator', makeContent([new PreviewNode('aurago-live-widget', {}, [text('live state')])])]
]) {
    const markup = preview(appId, content);
    assert.match(markup, /desktop\.taskbar_thumb_live/, `${appId} unsafe view should use an icon fallback`);
    assert.doesNotMatch(markup, /private message body|secret|draft|live state/);
}

const activeContent = makeContent([
    new PreviewNode('p', {
        id: 'source-id', name: 'field-name', style: 'background:url(https://invalid.test/a)',
        onclick: 'steal()', 'data-action': 'navigate', tabindex: '0', href: 'javascript:steal()'
    }, [text('safe visible text')]),
    new PreviewNode('button', {}, [text('live control')]),
    new PreviewNode('img', { src: 'https://invalid.test/image.png' }),
    new PreviewNode('script', {}, [text('steal()')]),
    new PreviewNode('link', { href: 'https://invalid.test/style.css' }),
    new PreviewNode('svg', {}, [
        new PreviewNode('foreignObject', {}, [text('embedded html')]),
        new PreviewNode('animate', { attributeName: 'href', to: 'https://invalid.test/payload' })
    ]),
    new PreviewNode('math', {}, [new PreviewNode('mi', {}, [text('embedded math')])])
]);
const sanitized = preview('calculator', activeContent);
assert.match(sanitized, /safe visible text/);
assert.match(sanitized, /inert aria-hidden="true"/);
assert.doesNotMatch(sanitized, /source-id|field-name|background:url|onclick|data-action|tabindex|javascript:|invalid\.test|live control|steal\(\)|foreignObject|animate|embedded html|embedded math|<svg|<math/);

process.stdout.write('Desktop window previews: sensitive/live fallbacks and inert sanitized clones verified.\n');
