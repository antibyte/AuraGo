import '../../../vendor/foliate/source/view.js';
import { EPUB } from '../../../vendor/foliate/source/epub.js';
import { MOBI, isMOBI } from '../../../vendor/foliate/source/mobi.js';
import { makeFB2 } from '../../../vendor/foliate/source/fb2.js';
import { makeComicBook } from '../../../vendor/foliate/source/comic-book.js';
import { Overlayer } from '../../../vendor/foliate/source/overlayer.js';
import { configure, ZipReader, BlobReader, BlobWriter } from '../../../vendor/foliate/source/vendor/zip.js';
import { limits, safeEntry, checkEntries, sanitizeDocument, sanitizeCSS, boundedImage } from './reader-security.js';

// Runs only in an opaque sandbox. It has no credentials, API URLs or network access.
let port, view, book, zip, openData, originalDirection, preferences = {}, closed = false;
const urls = new Set();
const annotations = new Map();
const emit = (type, data) => { if (!closed) port?.postMessage({ type, data }); };
const makeURL = blob => { const url = URL.createObjectURL(blob); urls.add(url); return url; };
const plain = value => typeof value === 'string' ? value : Array.isArray(value)
    ? value.map(plain).join(', ') : value && typeof value === 'object' ? plain(value.name || value.en || Object.values(value)[0]) : '';

async function makeLoader(file) {
    configure({ useWebWorkers: false, useCompressionStream: true });
    zip = new ZipReader(new BlobReader(file));
    const entries = await zip.getEntries();
    checkEntries(entries);
    const map = new Map(entries.map(entry => [entry.filename, entry]));
    async function loadBlob(name, type) {
        safeEntry(name);
        const entry = map.get(name);
        if (!entry) return null;
        let bytes = 0;
        const blob = await entry.getData(new BlobWriter(type), {
            onprogress: current => { bytes = current; if (current > limits.entry) throw new Error('bookshelf_archive_limit'); }
        });
        if (bytes > limits.entry || blob.size > limits.entry || blob.size !== entry.uncompressedSize) throw new Error('bookshelf_archive_limit');
        return blob;
    }
    return { entries, loadBlob, loadText: async name => (await loadBlob(name))?.text(),
        getSize: name => map.get(name)?.uncompressedSize || 0 };
}

function textBook(text, name, html) {
    const doc = document.implementation.createHTMLDocument(name);
    if (html) doc.documentElement.innerHTML = sanitizeDocument(text);
    else {
        const body = doc.body;
        for (const paragraph of text.split(/\r?\n\s*\r?\n/)) {
            const p = doc.createElement('p'); p.textContent = paragraph; body.append(p);
        }
    }
    const content = sanitizeDocument(doc.documentElement.outerHTML);
    const url = makeURL(new Blob([content], { type: 'text/html' }));
    return { metadata: { title: name }, sections: [{ id: 'text', size: text.length, load: async () => url,
        createDocument: async () => new DOMParser().parseFromString(content, 'text/html') }],
        toc: [{ label: name, href: 'text' }], resolveHref: () => ({ index: 0 }),
        isExternal: href => /^https?:/i.test(href),
        splitTOCHref: () => ['text', null], getTOCFragment: d => d.documentElement };
}

async function parseBook(file, format, encoding) {
    if (['txt', 'html', 'htm', 'fb2'].includes(format) && file.size > limits.entry) throw new Error('bookshelf_archive_limit');
    if (['txt', 'html', 'htm'].includes(format)) {
        const bytes = await file.arrayBuffer(); let selected = encoding;
        if (!selected || selected === 'auto') {
            const head = new Uint8Array(bytes, 0, Math.min(2, bytes.byteLength));
            selected = head[0] === 255 && head[1] === 254 ? 'utf-16le' : head[0] === 254 && head[1] === 255 ? 'utf-16be' : 'utf-8';
            try { new TextDecoder(selected, { fatal: true }).decode(bytes); } catch { selected = 'windows-1252'; }
        }
        const text = new TextDecoder(selected).decode(bytes);
        return textBook(text, file.name, format !== 'txt');
    }
    const magic = new Uint8Array(await file.slice(0, 4).arrayBuffer());
    if (magic[0] === 0x50 && magic[1] === 0x4b) {
        const loader = await makeLoader(file);
        if (format === 'cbz') {
            loader.entries = loader.entries.filter(e => /\.(png|jpe?g|webp|gif|avif)$/i.test(e.filename));
            const original = loader.loadBlob;
            loader.loadBlob = async name => boundedImage(await original(name));
            return makeComicBook(loader, file);
        }
        if (format === 'fbz' || format === 'fb2.zip') {
            const files = loader.entries.filter(entry => /\.fb2$/i.test(entry.filename));
            if (files.length !== 1) throw new Error('bookshelf_invalid_archive');
            return makeFB2(await loader.loadBlob(files[0].filename));
        }
        return new EPUB(loader).init();
    }
    if (await isMOBI(file)) return new MOBI({ unzlib: boundedInflate }).open(file);
    if (format === 'fb2') return makeFB2(file);
    throw new Error('bookshelf_unsupported_format');
}

async function boundedInflate(bytes) {
    const reader = new Blob([bytes]).stream().pipeThrough(new DecompressionStream('deflate')).getReader();
    const chunks = []; let size = 0;
    try {
        while (true) {
            const { value, done } = await reader.read(); if (done) break;
            size += value.byteLength;
            if (size > limits.entry) throw new Error('bookshelf_archive_limit');
            chunks.push(value);
        }
    } catch (error) { await reader.cancel().catch(() => {}); throw error; }
    return new Uint8Array(await new Blob(chunks).arrayBuffer());
}

function secureBook(value) {
    value.transformTarget?.addEventListener('load', ({ detail }) => { if (detail.isScript) detail.allow = false; });
    value.transformTarget?.addEventListener('data', ({ detail }) => {
        detail.data = Promise.resolve(detail.data).then(async data => {
            const type = detail.type || '';
            if (/html|xml|svg/.test(type)) return sanitizeDocument(data instanceof Blob ? await data.text() : data, type);
            if (type === 'text/css') return sanitizeCSS(data);
            return data;
        });
    });
    // Every displayed section is filtered, including formats without transform events.
    for (const section of value.sections) {
        if (typeof section.load !== 'function') continue;
        const load = section.load.bind(section), unload = section.unload?.bind(section);
        let safeURL;
        section.load = async () => {
            if (safeURL) return safeURL;
            const original = await load();
            if (!original?.startsWith('blob:')) throw new Error('bookshelf_invalid_resource');
            const response = await fetch(original);
            const text = await response.text();
            if (text.length > limits.entry) throw new Error('bookshelf_archive_limit');
            safeURL = makeURL(new Blob([sanitizeDocument(text)], { type: 'text/html' }));
            return safeURL;
        };
        section.unload = () => { if (safeURL) { URL.revokeObjectURL(safeURL); urls.delete(safeURL); safeURL = null; } unload?.(); };
    }
    return value;
}

function applyPreferences(settings) {
    preferences = { ...preferences, ...settings };
    if (!view?.renderer) return;
    const size = Math.max(12, Math.min(36, Number(preferences.fontSize) || 19));
    const line = Math.max(1.2, Math.min(2.2, Number(preferences.lineHeight) || 1.6));
    const families = { serif: 'Georgia,serif', sans: 'Arial,sans-serif', mono: 'monospace' };
    const colors = { light: ['#fffdf8', '#28231d'], sepia: ['#eee3cb', '#30281e'], dark: ['#23221f', '#eee5d6'] };
    const [background, foreground] = colors[preferences.theme] || colors.sepia;
    document.documentElement.style.background = background;
    if (view.isFixedLayout) {
        view.renderer.spread = preferences.spread === false ? 'none' : 'auto';
        view.renderer.setAttribute('zoom', Number(preferences.zoom) > 0 ? String(Math.min(4, Number(preferences.zoom))) : preferences.fit === 'width' ? 'fit-width' : 'fit-page');
        return;
    }
    view.renderer.setAttribute('flow', preferences.flow === 'scrolled' ? 'scrolled' : 'paginated');
    view.renderer.setAttribute('max-column-count', preferences.spread === false ? '1' : '2');
    view.renderer.setAttribute('max-inline-size', String(Math.max(320, Math.min(900, Number(preferences.width) || 640))));
    view.renderer.setAttribute('gap', '0.06');
    view.renderer.setStyles?.(`html{color:${foreground}!important;background:${background}!important;color-scheme:${preferences.theme === 'dark' ? 'dark' : 'light'};}body{font-family:${families[preferences.fontFamily] || families.serif}!important;font-size:${size}px!important;line-height:${line}!important;padding:12px!important;}img,svg{max-width:100%;}a{color:inherit;text-decoration:underline;}::selection{background:#e4b574;}`);
}

async function open(data) {
    openData = data;
    const file = new File([data.blob], data.name || 'book.epub');
    book = secureBook(await parseBook(file, data.format, data.settings?.encoding));
    originalDirection = book.dir;
    if (book.rendition?.layout === 'pre-paginated') {
        if (typeof data.settings?.spread === 'boolean') book.rendition.spread = data.settings.spread ? 'auto' : 'none';
        if (['rtl', 'ltr'].includes(data.settings?.direction)) book.dir = data.settings.direction;
    }
    if (closed) { book.destroy?.(); return; }
    view = document.createElement('foliate-view');
    view.style.cssText = 'display:block;width:100%;height:100%';
    document.body.append(view);
    view.addEventListener('relocate', ({ detail }) => emit('progress', {
        locator: { cfi: detail.cfi, ...(view.isFixedLayout ? { page: view.renderer.index + 1 } : {}) }, fraction: Math.max(0, Math.min(1, Number(detail.fraction) || 0)) }));
    view.addEventListener('external-link', event => { event.preventDefault(); emit('external-link', { href: event.detail?.href_ || event.detail?.href }); });
    view.addEventListener('draw-annotation', ({ detail }) => detail.draw(Overlayer.highlight, { color: '#e4b574' }));
    view.addEventListener('create-overlay', () => {
        queueMicrotask(() => { for (const annotation of annotations.values()) view.addAnnotation(annotation).catch(() => {}); });
    });
    view.addEventListener('load', ({ detail }) => {
        detail.doc.addEventListener('keydown', key);
        detail.doc.addEventListener('selectionchange', () => {
            const selection = detail.doc.getSelection();
            if (selection?.rangeCount && !selection.isCollapsed) emit('selection', {
                text: selection.toString().slice(0, 4000), locator: { cfi: view.getCFI(detail.index, selection.getRangeAt(0)) } });
        });
    });
    await view.open(book);
    applyPreferences(data.settings || {});
    await view.init({ lastLocation: data.progress?.locator?.cfi || undefined, showTextStart: !data.progress?.locator });
    const metadata = { title: plain(book.metadata?.title), author: plain(book.metadata?.author),
        description: plain(book.metadata?.description), language: plain(book.metadata?.language) };
    emit('ready', { toc: book.toc || [], isFixedLayout: view.isFixedLayout, pageCount: view.isFixedLayout ? book.sections.length : undefined, metadata });
    emit('metadata', metadata);
    if (book.getCover) {
        try {
            const cover = await boundedImage(await book.getCover());
            const bitmap = await createImageBitmap(cover);
            const canvas = document.createElement('canvas');
            const scale = Math.min(1, 640 / Math.max(bitmap.width, bitmap.height));
            canvas.width = Math.max(1, Math.round(bitmap.width * scale)); canvas.height = Math.max(1, Math.round(bitmap.height * scale));
            canvas.getContext('2d').drawImage(bitmap, 0, 0, canvas.width, canvas.height); bitmap.close();
            const coverBlob = await new Promise(resolve => canvas.toBlob(resolve, 'image/png'));
            emit('metadata', { ...metadata, coverBlob });
        } catch { /* A missing or invalid cover never prevents reading. */ }
    }
}

function key(event) {
    if (event.altKey || event.ctrlKey || event.metaKey || /input|textarea|select/i.test(event.target?.localName || '')) return;
    if (['ArrowRight', 'PageDown'].includes(event.key)) { event.preventDefault(); preferences.direction === 'rtl' && event.key === 'ArrowRight' ? view?.prev() : view?.next(); }
    if (['ArrowLeft', 'PageUp'].includes(event.key)) { event.preventDefault(); preferences.direction === 'rtl' && event.key === 'ArrowLeft' ? view?.next() : view?.prev(); }
}
document.addEventListener('keydown', key);

async function command(message) {
    const { id, action, data } = message;
    try {
        let result;
        if (action === 'open') await open(data);
        else if (action === 'next') await view?.next();
        else if (action === 'prev') await view?.prev();
        else if (action === 'goTo') await view?.goTo(data?.cfi || data?.href || (data?.page ? data.page - 1 : data));
        else if (action === 'preferences') {
            if (data.encoding && data.encoding !== preferences.encoding && ['txt', 'html', 'htm'].includes(openData.format)) {
                const saved = view.lastLocation?.cfi;
                view.close(); view.remove(); book.destroy?.(); await zip?.close(); zip = null;
                for (const url of urls) URL.revokeObjectURL(url); urls.clear();
                await open({ ...openData, settings: { ...preferences, ...data }, progress: { locator: { cfi: saved } } });
            } else if (view.isFixedLayout && (data.spread !== undefined && data.spread !== preferences.spread || data.direction !== undefined && data.direction !== preferences.direction)) {
                const saved = view.lastLocation?.cfi;
                if (typeof data.spread === 'boolean') book.rendition.spread = data.spread ? 'auto' : 'none';
                if (data.direction !== undefined) book.dir = ['rtl', 'ltr'].includes(data.direction) ? data.direction : originalDirection;
                view.close(); await view.open(book); applyPreferences(data); await view.init({ lastLocation: saved });
            } else applyPreferences(data);
        }
        else if (action === 'annotation' && data.locator?.cfi) {
            const annotation = { value: data.locator.cfi, color: '#e4b574' };
            annotations.set(annotation.value, annotation); result = await view?.addAnnotation(annotation);
        }
        else if (action === 'remove-annotation' && data.locator?.cfi) {
            annotations.delete(data.locator.cfi); await view?.deleteAnnotation({ value: data.locator.cfi });
        }
        else if (action === 'thumbnail' && view?.isFixedLayout && Number.isInteger(data) && data >= 0 && data < book.sections.length) {
            const url = await book.sections[data].load();
            const doc = new DOMParser().parseFromString(await (await fetch(url)).text(), 'text/html');
            const src = doc.querySelector('img')?.getAttribute('src');
            if (src?.startsWith('blob:')) {
                const bitmap = await createImageBitmap(await boundedImage(await (await fetch(src)).blob()));
                const canvas = document.createElement('canvas'), scale = Math.min(1, 180 / bitmap.width);
                canvas.width = Math.max(1, Math.round(bitmap.width * scale)); canvas.height = Math.max(1, Math.round(bitmap.height * scale));
                canvas.getContext('2d').drawImage(bitmap, 0, 0, canvas.width, canvas.height); bitmap.close();
                result = await new Promise(resolve => canvas.toBlob(resolve, 'image/png'));
            }
        }
        else if (action === 'search') {
            result = [];
            for await (const hit of view.search({ query: String(data).slice(0, 200) })) {
                if (closed) break;
                for (const item of hit?.subitems || (hit?.cfi ? [hit] : [])) {
                    result.push({ locator: { cfi: item.cfi }, text: typeof item.excerpt === 'string' ? item.excerpt : plain(item.excerpt?.pre) + plain(item.excerpt?.match) + plain(item.excerpt?.post) });
                    if (result.length >= 100) break;
                }
                if (result.length >= 100) break;
            }
        }
        emit('result', { id, result });
    } catch (error) { emit('error', { id, code: /^bookshelf_/.test(error.message) ? error.message : 'bookshelf_reader_failed' }); }
}

window.addEventListener('message', event => {
    if (port || event.source !== parent || event.data?.type !== 'bookshelf-connect' || !event.ports?.[0]) return;
    port = event.ports[0]; port.onmessage = event => command(event.data); port.start(); emit('connected', {});
});
window.addEventListener('pagehide', () => {
    closed = true; view?.close(); book?.destroy?.(); zip?.close();
    for (const url of urls) URL.revokeObjectURL(url);
    port?.close();
});
