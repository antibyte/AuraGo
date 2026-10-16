import DOMPurify from 'dompurify';

export const limits = Object.freeze({ entries: 10000, entry: 32 * 1024 * 1024,
    expanded: 256 * 1024 * 1024, ratio: 200, pixels: 32 * 1024 * 1024 });

export function safeEntry(name) {
    if (typeof name !== 'string' || !name || name.length > 1024 || /[\\\x00-\x1f:]/.test(name)
        || name.startsWith('/') || name.split('/').some(part => part === '..' || part === '.')) {
        throw new Error('bookshelf_invalid_archive');
    }
    return name;
}

export function checkEntries(entries) {
    if (entries.length > limits.entries) throw new Error('bookshelf_archive_limit');
    const seen = new Set();
    let expanded = 0;
    for (const entry of entries) {
        const name = safeEntry(entry.filename).toLowerCase();
        if (seen.has(name) || entry.encrypted || entry.uncompressedSize > limits.entry
            || entry.uncompressedSize > Math.max(1, entry.compressedSize) * limits.ratio
            || ((entry.externalFileAttributes >>> 16) & 0xf000) === 0xa000) {
            throw new Error('bookshelf_invalid_archive');
        }
        seen.add(name);
        expanded += entry.uncompressedSize;
        if (!Number.isSafeInteger(expanded) || expanded > limits.expanded) throw new Error('bookshelf_archive_limit');
    }
}

function localResource(value) {
    return /^blob:/i.test(value) || /^data:(?:image\/(?:png|jpeg|gif|webp|avif)|font\/[\w.+-]+|application\/(?:font-woff|x-font-ttf));base64,/i.test(value);
}

export function sanitizeCSS(css) {
    // The frame CSP independently blocks all network loads, including escaped CSS URLs.
    return String(css).replace(/@import\s[^;]+;?/gi, '')
        .replace(/url\(\s*(['"]?)(.*?)\1\s*\)/gi, (_, quote, url) => localResource(url) ? `url("${url.replaceAll('"', '')}")` : 'none');
}

export function sanitizeDocument(input, type = 'text/html') {
    const doc = new DOMParser().parseFromString(String(input), type.includes('svg') ? 'image/svg+xml' : 'text/html');
    DOMPurify.sanitize(doc.documentElement, { IN_PLACE: true,
        ALLOWED_URI_REGEXP: /^(?:(?:https?|blob):|[^a-z]|[a-z+.\-]+(?:[^a-z+.\-:]|$))/i,
        ADD_TAGS: ['style', 'link', 'meta'], FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'input', 'button', 'select', 'textarea', 'base', 'foreignObject', 'animate', 'set'],
        FORBID_ATTR: ['srcdoc', 'nonce', 'integrity', 'ping', 'attributionsrc', 'formaction', 'action', 'srcset', 'target'] });
    for (const el of doc.querySelectorAll('*')) {
        for (const attr of Array.from(el.attributes)) {
            if (/^on/i.test(attr.name)) el.removeAttribute(attr.name);
            if (/^(?:src|poster|background|xlink:href)$/i.test(attr.name)
                && !localResource(attr.value) && !attr.value.startsWith('#')) el.removeAttribute(attr.name);
        }
        if (el.hasAttribute('style')) el.setAttribute('style', sanitizeCSS(el.getAttribute('style')));
        if (el.localName === 'style') el.textContent = sanitizeCSS(el.textContent);
        if (el.localName === 'meta') {
            if (el.getAttribute('name') !== 'viewport' || !/^[a-z0-9=.,\s-]{1,200}$/i.test(el.getAttribute('content') || '')) el.remove();
            else for (const attr of Array.from(el.attributes)) if (!['name', 'content'].includes(attr.name)) el.removeAttribute(attr.name);
        }
        if (el.localName === 'link' && (el.getAttribute('rel') !== 'stylesheet' || !el.getAttribute('href')?.startsWith('blob:'))) el.remove();
        if (el.localName === 'a' && /^(?:javascript|data|file|vbscript):/i.test(el.getAttribute('href') || '')) el.removeAttribute('href');
        if (el.localName === 'image' && el.hasAttribute('href') && !localResource(el.getAttribute('href'))) el.removeAttribute('href');
    }
    return new XMLSerializer().serializeToString(doc.documentElement);
}

export async function boundedImage(blob) {
    if (!blob || blob.size > limits.entry) throw new Error('bookshelf_image_limit');
    const bitmap = await createImageBitmap(blob);
    try {
        if (bitmap.width * bitmap.height > limits.pixels) throw new Error('bookshelf_image_limit');
    } finally { bitmap.close(); }
    return blob;
}
