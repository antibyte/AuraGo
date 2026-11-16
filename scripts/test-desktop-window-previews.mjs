import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

let customCloneCount = 0;
const escapePreviewText = value => String(value).replace(/[&<>]/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' })[char]);

class PreviewNode {
    constructor(tag, attributes = {}, children = []) {
        this.tagName = String(tag).toUpperCase();
        this.localName = this.tagName === '#TEXT' ? '' : this.tagName.toLowerCase();
        this.nodeType = this.tagName === '#TEXT' ? 3 : 1;
        this.values = new Map(Object.entries(attributes));
        this.children = children;
        this.parentNode = null;
        this.offsetHeight = 240;
        this.style = { setProperty() {} };
        children.forEach(child => { child.parentNode = this; });
    }
    get childNodes() { return this.children; }
    get nodeValue() { return this.tagName === '#TEXT' ? this.text : null; }
    get attributes() { throw new Error('preview sanitizer must not materialize all source attributes'); }
    get innerHTML() { return this.children.map(child => child.tagName === '#TEXT' ? escapePreviewText(child.text) : child.outerHTML).join(''); }
    get outerHTML() {
        if (this.tagName === '#TEXT') return escapePreviewText(this.text);
        const attrs = Array.from(this.values, ([name, value]) => ` ${name}="${String(value)}"`).join('');
        return `<${this.tagName.toLowerCase()}${attrs}>${this.innerHTML}</${this.tagName.toLowerCase()}>`;
    }
    setAttribute(name, value) { this.values.set(name, String(value)); }
    removeAttribute(name) { this.values.delete(name); }
    hasAttribute(name) { return this.values.has(name); }
    getAttribute(name) { return this.values.has(name) ? this.values.get(name) : null; }
    appendChild(child) { child.parentNode = this; this.children.push(child); return child; }
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
        if (this.tagName.includes('-')) customCloneCount++;
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
const createdTags = [];
const document = {
    createElement(tag) { createdTags.push(String(tag).toLowerCase()); return new PreviewNode(tag); },
    createTextNode(value) { const node = text(value); createdTags.push('#text'); return node; }
};
const context = vm.createContext({
    window: {},
    document,
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
    ['people', makeContent([text('private contact')])],
    ['quick-connect', makeContent([text('private connection')])],
    ['contacts', makeContent([text('private legacy contact')])],
    ['quickconnect', makeContent([text('private legacy connection')])],
    ['calculator', makeContent([new PreviewNode('canvas')])],
    ['calculator', makeContent([new PreviewNode('iframe')])],
    ['calculator', makeContent([new PreviewNode('input', { type: 'password', value: 'secret' })])],
    ['calculator', makeContent([new PreviewNode('form', {}, [new PreviewNode('input', { value: 'hidden secret' })])])],
    ['calculator', makeContent([new PreviewNode('div', { hidden: '' }, [new PreviewNode('iframe', { src: 'https://invalid.test/private' })])])],
    ['calculator', makeContent([new PreviewNode('p', { contenteditable: 'true' }, [text('draft')])])],
    ['calculator', makeContent([new PreviewNode('aurago-live-widget', {}, [text('live state')])])],
    ['virtual-computers', makeContent([text('private machine inventory')])]
]) {
    createdTags.length = 0;
    customCloneCount = 0;
    const markup = preview(appId, content);
    assert.match(markup, /desktop\.taskbar_thumb_live/, `${appId} unsafe view should use an icon fallback`);
    assert.doesNotMatch(markup, /private message body|secret|draft|live state/);
    assert.equal(customCloneCount, 0, 'fallback selection never clones a custom element');
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
createdTags.length = 0;
const sanitized = preview('calculator', activeContent);
assert.match(sanitized, /safe visible text/);
assert.match(sanitized, /inert aria-hidden="true"/);
assert.doesNotMatch(sanitized, /source-id|field-name|background:url|onclick|data-action|tabindex|javascript:|invalid\.test|live control|steal\(\)|foreignObject|animate|embedded html|embedded math|<svg|<math/);
assert.ok(!createdTags.some(tag => ['a', 'button', 'img', 'link', 'script', 'svg', 'math'].includes(tag)), 'resource, navigation and active nodes are skipped before creating preview nodes');

const oversizedNodes = makeContent(Array.from({ length: 520 }, () => new PreviewNode('span', {}, [text('x')])));
createdTags.length = 0;
assert.match(preview('calculator', oversizedNodes), /desktop\.taskbar_thumb_live/, 'more than 512 source nodes use the bounded fallback');
assert.equal(createdTags.length, 0, 'preflight detects oversized source before creating projected nodes');

let iteratedWideNodes = 0;
const wideChildren = {
    *[Symbol.iterator]() {
        for (let index = 0; index < 2000; index++) {
            iteratedWideNodes++;
            yield new PreviewNode('span', {}, [text('x')]);
        }
    }
};
const wideContent = makeContent([]);
Object.defineProperty(wideContent, 'childNodes', { get: () => wideChildren });
createdTags.length = 0;
assert.match(preview('calculator', wideContent), /desktop\.taskbar_thumb_live/, 'a wide live source falls back before full traversal');
assert.ok(iteratedWideNodes <= 512, 'preflight stops iterating a wide source at the fixed node budget');
assert.equal(createdTags.length, 0, 'wide source preflight creates no projected nodes');

const oversizedText = makeContent([text('x'.repeat(32 * 1024 + 1))]);
createdTags.length = 0;
assert.match(preview('calculator', oversizedText), /desktop\.taskbar_thumb_live/, 'text above 32 KiB uses the bounded fallback');
assert.ok(createdTags.length <= 1, 'oversized text is rejected without copying its text node');

process.stdout.write('Desktop window previews: bounded passive projections and sensitive/live fallbacks verified.\n');
