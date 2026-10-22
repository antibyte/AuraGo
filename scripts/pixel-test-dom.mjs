// Fake DOM for node tests that run the real Pixel modules (see test-pixel-core.mjs). Not a test itself.
// Elements are permissive proxies: unknown methods are no-ops, querySelector returns a stable stub per selector,
// querySelectorAll returns []. Canvas contexts record drawImage calls in ctx.log. innerHTML keeps the markup string.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { appDir, repoRoot } from './pixel-test-util.mjs';

function makeCtx(canvas) {
    const state = { canvas, log: [] };
    return new Proxy(state, {
        get(t, p) {
            if (p in t) return t[p];
            if (p === 'getImageData' || p === 'createImageData') {
                return (x, y, w, h) => {
                    if (p === 'createImageData') { h = w; w = x; }
                    return { width: w, height: h, data: new Uint8ClampedArray(Math.max(0, w * h * 4)) };
                };
            }
            if (p === 'createLinearGradient' || p === 'createRadialGradient') return () => ({ addColorStop() {} });
            if (p === 'measureText') return () => ({ width: 10 });
            if (p === 'drawImage') return (...args) => { t.log.push(['drawImage', args[0], t.globalCompositeOperation, t.globalAlpha]); };
            return () => {};
        },
        set(t, p, v) { t[p] = v; return true; }
    });
}

export function makeEl(tag) {
    const store = {
        tagName: String(tag).toUpperCase(), style: {}, dataset: {}, hidden: false, value: '', textContent: '', innerHTML: '',
        width: 0, height: 0, scrollLeft: 0, scrollTop: 0, clientWidth: 900, clientHeight: 700, children: [], _q: new Map(),
        classList: { add() {}, remove() {}, toggle() {}, contains() { return false; } }
    };
    let ctx = null;
    const proxy = new Proxy(store, {
        get(t, p) {
            if (p in t) return t[p];
            if (p === 'parentElement') return t._parent || (t._parent = makeEl('div'));
            if (p === 'getContext') return () => ctx || (ctx = makeCtx(proxy));
            if (p === 'querySelector') return sel => { if (!t._q.has(sel)) t._q.set(sel, makeEl('div')); return t._q.get(sel); };
            if (p === 'querySelectorAll') return () => [];
            if (p === 'closest') return () => null;
            if (p === 'getBoundingClientRect') return () => ({ left: 0, top: 0, width: t.width || 100, height: t.height || 100 });
            if (p === 'append' || p === 'appendChild' || p === 'prepend') return c => { t.children.push(c); return c; };
            if (p === 'toBlob') return cb => cb(null);
            if (p === 'toDataURL') return () => 'data:image/png;base64,AAAA';
            return () => {};
        },
        set(t, p, v) { t[p] = v; return true; }
    });
    return proxy;
}

export function installFakeDom() {
    globalThis.window = globalThis;
    globalThis.addEventListener = () => {};
    globalThis.removeEventListener = () => {};
    globalThis.document = {
        createElement: tag => makeEl(tag),
        createElementNS: (_, tag) => makeEl(tag),
        body: makeEl('body'),
        addEventListener() {}, removeEventListener() {}, activeElement: null
    };
    globalThis.requestAnimationFrame = fn => setTimeout(() => fn(Date.now()), 0);
    globalThis.cancelAnimationFrame = id => clearTimeout(id);
    globalThis.Image = class { set src(v) { setTimeout(() => this.onload && this.onload(), 0); } get naturalWidth() { return 40; } get naturalHeight() { return 30; } };
    globalThis.localStorage = { getItem() { return null; }, setItem() {} };
    Object.defineProperty(globalThis, 'navigator', { value: { clipboard: {} }, configurable: true });
    globalThis.ImageData = class ImageData { constructor(d, w, h) { this.data = d; this.width = w; this.height = h; } };
    globalThis.ClipboardItem = class {};
}

// The app scripts of the pixel entry of the module loader, in loading order (shared sticker libs are skipped).
export function loaderScripts() {
    const loader = fs.readFileSync(path.join(repoRoot, 'ui', 'js', 'desktop', 'core', 'module-loader.js'), 'utf8');
    const start = loader.indexOf("'pixel': {");
    const end = loader.indexOf("'galaxa-deluxe'", start);
    const block = loader.slice(start, end);
    return Array.from(block.matchAll(/'\/js\/desktop\/apps\/(pixel[^']*\.js)'/g)).map(match => match[1]);
}

export function loaderStyles() {
    const loader = fs.readFileSync(path.join(repoRoot, 'ui', 'js', 'desktop', 'core', 'module-loader.js'), 'utf8');
    const start = loader.indexOf("'pixel': {");
    const end = loader.indexOf("scripts: [", start);
    return Array.from(loader.slice(start, end).matchAll(/'\/css\/([^']+\.css)'/g)).map(match => match[1]);
}

export function loadPixelApp() {
    loaderScripts().forEach(file => {
        vm.runInThisContext(fs.readFileSync(path.join(appDir, file), 'utf8'), { filename: file });
    });
    return globalThis.AuraPixelApp;
}

// The desktop shell's t(key, vars) for English: texts come from ui/lang/desktop/en.json (the code carries no inline
// fallbacks), a missing key returns the key, {name} and {{name}} placeholders are replaced like in ui/shared.js.
export function englishT() {
    const en = JSON.parse(fs.readFileSync(path.join(repoRoot, 'ui', 'lang', 'desktop', 'en.json'), 'utf8'));
    return (key, vars) => {
        let text = Object.prototype.hasOwnProperty.call(en, key) ? en[key] : key;
        if (vars && typeof vars === 'object') Object.keys(vars).forEach(name => { text = text.split('{{' + name + '}}').join(vars[name]).split('{' + name + '}').join(vars[name]); });
        return text;
    };
}
